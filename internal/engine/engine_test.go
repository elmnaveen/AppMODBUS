package engine

import (
	"errors"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"modbusstudio/internal/modbus"
	"modbusstudio/internal/simulator"
)

func startSim(t *testing.T, e *Engine, mode modbus.Mode) int {
	t.Helper()
	if err := e.Sim.Start(simulator.Config{Mode: mode, Host: "127.0.0.1", Port: 0, SlaveID: 1}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Sim.Stop)
	_, port, _ := net.SplitHostPort(e.Sim.Addr())
	p, _ := strconv.Atoi(port)
	return p
}

func TestEndToEndAllNetworkModes(t *testing.T) {
	for _, mode := range []modbus.Mode{modbus.ModeTCP, modbus.ModeRTUOverTCP, modbus.ModeRTUOverUDP, modbus.ModeASCIITCP, modbus.ModeUDP} {
		t.Run(string(mode), func(t *testing.T) {
			e := New(nil)
			defer e.Shutdown()
			port := startSim(t, e, mode)
			e.Sim.Store.Write(simulator.HoldingRegisters, 10, []int{0x42F6, 0xE979, 7})
			e.Sim.Store.Write(simulator.Coils, 3, []int{1})

			conn, err := e.SaveConnection(ConnectionConfig{Mode: mode, Host: "127.0.0.1", Port: port, TimeoutMs: 500})
			if err != nil {
				t.Fatal(err)
			}
			p, err := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 1, Function: 3, Address: 10, Quantity: 3, ScanRateMs: 100, Format: "signed"})
			if err != nil {
				t.Fatal(err)
			}
			if err := e.SetCellFormat(p.Config.ID, 10, "float_abcd"); err != nil {
				t.Fatal(err)
			}
			if err := e.ReadOnce(p.Config.ID); err != nil {
				t.Fatal(err)
			}
			cells := e.Polls()[0].State.Cells
			if cells[0].Value != "123.456" || cells[0].Span != 2 || cells[1].Span != 0 || cells[2].Value != "7" {
				t.Fatalf("cells %+v", cells)
			}

			// Write through a cell: float spans two registers -> FC16.
			if err := e.WriteCell(p.Config.ID, 10, "-2.5", false); err != nil {
				t.Fatal(err)
			}
			if got := e.Sim.Store.Read(simulator.HoldingRegisters, 10, 2); got[0] != 0xC020 || got[1] != 0 {
				t.Fatalf("store after write % X", got)
			}

			// Coils + FC05.
			cp, _ := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 1, Function: 1, Address: 0, Quantity: 8, ScanRateMs: 100})
			if err := e.WriteCell(cp.Config.ID, 5, "on", false); err != nil {
				t.Fatal(err)
			}
			if err := e.ReadOnce(cp.Config.ID); err != nil {
				t.Fatal(err)
			}
			for _, c := range e.Polls()[1].State.Cells {
				want := "0"
				if c.Address == 3 || c.Address == 5 {
					want = "1"
				}
				if c.Value != want {
					t.Fatalf("coil %d = %s", c.Address, c.Value)
				}
			}
		})
	}
}

func TestExceptionAndTimeout(t *testing.T) {
	e := New(nil)
	defer e.Shutdown()
	port := startSim(t, e, modbus.ModeRTUOverTCP)
	conn, _ := e.SaveConnection(ConnectionConfig{Mode: modbus.ModeRTUOverTCP, Host: "127.0.0.1", Port: port, TimeoutMs: 200})

	p, _ := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 1, Function: 3, Address: 65530, Quantity: 6, ScanRateMs: 100})
	var ex *modbus.ExceptionError
	// 65530+6 is a valid range, so read a valid poll, then ask the
	// simulator for an unsupported function to get an exception.
	if err := e.ReadOnce(p.Config.ID); err != nil {
		t.Fatal(err)
	}
	c := e.conn(conn.ID)
	_, err := c.client.Do(1, modbus.PDU{Function: 0x14, Data: []byte{0x07, 0x06, 0, 1, 0, 0, 0, 1}}, 200*time.Millisecond)
	if !errors.As(err, &ex) || ex.Code != modbus.ExIllegalFunction {
		t.Fatalf("expected illegal function, got %v", err)
	}

	// Wrong slave ID: RTU slaves stay silent -> timeout.
	p2, _ := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 9, Function: 3, Address: 0, Quantity: 1, ScanRateMs: 100})
	if err := e.ReadOnce(p2.Config.ID); !errors.Is(err, modbus.ErrTimeout) {
		t.Fatalf("expected timeout, got %v", err)
	}
	if st := e.Polls()[1].State; st.LastError != "Timeout Error" || st.Errors != 1 {
		t.Fatalf("state %+v", st)
	}
	// The link must still work after a timeout.
	if err := e.ReadOnce(p.Config.ID); err != nil {
		t.Fatalf("after timeout: %v", err)
	}
}

func TestPollLoopEmitsUpdatesAndTraffic(t *testing.T) {
	var mu sync.Mutex
	updates, traffic := 0, 0
	e := New(func(ev string, data any) {
		mu.Lock()
		defer mu.Unlock()
		switch ev {
		case EventPollUpdate:
			updates++
		case EventTraffic:
			traffic += len(data.([]TrafficEntry))
		}
	})
	defer e.Shutdown()
	port := startSim(t, e, modbus.ModeTCP)
	conn, _ := e.SaveConnection(ConnectionConfig{Mode: modbus.ModeTCP, Host: "127.0.0.1", Port: port})
	p, _ := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 1, Function: 4, Address: 0, Quantity: 10, ScanRateMs: 20})
	if err := e.StartPoll(p.Config.ID); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	e.StopPoll(p.Config.ID)
	st := e.Polls()[0].State
	if st.Running || st.Valid < 5 || st.Errors != 0 {
		t.Fatalf("state %+v", st)
	}
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if updates < 5 || traffic < 10 {
		t.Fatalf("updates=%d traffic=%d", updates, traffic)
	}
}

func TestSharedConnectionConcurrentPolls(t *testing.T) {
	e := New(nil)
	defer e.Shutdown()
	port := startSim(t, e, modbus.ModeRTUOverTCP)
	conn, _ := e.SaveConnection(ConnectionConfig{Mode: modbus.ModeRTUOverTCP, Host: "127.0.0.1", Port: port})
	for i := 0; i < 5; i++ {
		p, _ := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 1, Function: 3, Address: i * 10, Quantity: 10, ScanRateMs: 10})
		_ = e.StartPoll(p.Config.ID)
	}
	time.Sleep(400 * time.Millisecond)
	e.StopAll()
	for _, p := range e.Polls() {
		if p.State.Errors != 0 || p.State.Valid == 0 {
			t.Fatalf("%s: %+v", p.Config.Name, p.State)
		}
	}
}
