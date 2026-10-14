package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"appmodbus/internal/modbus"
	"appmodbus/internal/simulator"
)

// TestSerialRTU runs master and simulator over a virtual serial cable made
// by socat. It is skipped where socat is not installed (e.g. Windows CI).
func TestSerialRTU(t *testing.T)   { serialRoundTrip(t, modbus.ModeRTU, 8) }
func TestSerialASCII(t *testing.T) { serialRoundTrip(t, modbus.ModeASCII, 7) }

func serialRoundTrip(t *testing.T, mode modbus.Mode, dataBits int) {
	if _, err := exec.LookPath("socat"); err != nil {
		t.Skip("socat not installed")
	}
	dir := t.TempDir()
	a, b := filepath.Join(dir, "ttyA"), filepath.Join(dir, "ttyB")
	cmd := exec.Command("socat", "pty,raw,echo=0,link="+a, "pty,raw,echo=0,link="+b)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(b); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	e := New(nil)
	defer e.Shutdown()
	if err := e.Sim.Start(simulator.Config{Mode: mode, SerialPort: b, BaudRate: 19200, DataBits: dataBits, Parity: "E", StopBits: 1, SlaveID: 7}); err != nil {
		t.Fatal(err)
	}
	e.Sim.Store.Write(simulator.HoldingRegisters, 100, []int{11, 22, 33})

	conn, err := e.SaveConnection(ConnectionConfig{Mode: mode, SerialPort: a, BaudRate: 19200, DataBits: dataBits, Parity: "E", TimeoutMs: 500})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 7, Function: 3, Address: 100, Quantity: 3, ScanRateMs: 50})
	if err := e.ReadOnce(p.Config.ID); err != nil {
		t.Fatal(err)
	}
	if c := e.Polls()[0].State.Cells; c[0].Value != "11" || c[2].Value != "33" {
		t.Fatalf("cells %+v", c)
	}
	if err := e.WriteCell(p.Config.ID, 101, "-5", false); err != nil {
		t.Fatal(err)
	}
	if v := e.Sim.Store.Read(simulator.HoldingRegisters, 101, 1); v[0] != 65531 {
		t.Fatalf("got %v", v)
	}
	// Wrong slave on a serial line: no answer -> timeout, line stays usable.
	p2, _ := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 8, Function: 3, Address: 0, Quantity: 1, ScanRateMs: 50})
	if err := e.ReadOnce(p2.Config.ID); err == nil {
		t.Fatal("expected timeout")
	}
	if err := e.ReadOnce(p.Config.ID); err != nil {
		t.Fatalf("after timeout: %v", err)
	}
}
