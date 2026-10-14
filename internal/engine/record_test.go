package engine

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"modbusstudio/internal/datafmt"
	"modbusstudio/internal/modbus"
	"modbusstudio/internal/simulator"
)

func TestOnSampleOnlyWhenRecording(t *testing.T) {
	e := New(nil)
	defer e.Shutdown()
	port := startSim(t, e, modbus.ModeTCP)
	e.Sim.Store.Write(simulator.HoldingRegisters, 0, []int{2305, 0x42F6, 0xE979})
	e.Sim.Store.Write(simulator.Coils, 0, []int{1, 0})
	var mu sync.Mutex
	var got []Cell
	var conn string
	e.OnSample = func(cfg PollConfig, c string, cells []Cell, _ time.Time) {
		mu.Lock()
		defer mu.Unlock()
		got, conn = cells, c
	}
	cn, _ := e.SaveConnection(ConnectionConfig{Mode: modbus.ModeTCP, Host: "127.0.0.1", Port: port})
	p, _ := e.CreatePoll(PollConfig{ConnectionID: cn.ID, SlaveID: 1, Function: 3, Quantity: 3, Format: "unsigned",
		CellFormats: map[int]datafmt.Format{1: "float_abcd"},
		Tags:        map[int]Tag{0: {Name: "V", Gain: 0.1, Unit: "V", Decimals: -1}}})
	if err := e.ReadOnce(p.Config.ID); err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatal("sample delivered while not recording")
	}
	_ = e.SetRecord(p.Config.ID, true)
	if err := e.ReadOnce(p.Config.ID); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 3 || !got[0].HasNum || got[0].Num < 230.49 || got[0].Num > 230.51 || got[0].Name != "V" ||
		got[1].Span != 2 || got[1].Num < 123.455 || got[1].Num > 123.457 || got[2].Span != 0 {
		t.Fatalf("cells %+v", got)
	}
	if conn != "TCP 127.0.0.1:"+strconv.Itoa(port) {
		t.Fatalf("connection %q", conn)
	}
	if !e.Polls()[0].Config.Record {
		t.Fatal("record flag not saved")
	}
}
