package engine

import (
	"testing"

	"appmodbus/internal/modbus"
	"appmodbus/internal/simulator"
)

func TestTagsScaleDisplayAndWrites(t *testing.T) {
	e := New(nil)
	defer e.Shutdown()
	port := startSim(t, e, modbus.ModeTCP)
	e.Sim.Store.Write(simulator.HoldingRegisters, 0, []int{2305, 0xFFF6, 1500, 0x42F6, 0xE979})
	conn, _ := e.SaveConnection(ConnectionConfig{Mode: modbus.ModeTCP, Host: "127.0.0.1", Port: port})
	p, err := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 1, Function: 3, Address: 0, Quantity: 5, ScanRateMs: 100,
		Format: "unsigned", Tags: map[int]Tag{
			0: {Name: "Voltage", Gain: 0.1, Unit: "V", Decimals: -1},
			2: {Name: "Temp", Gain: 0.1, Offset: -100, Unit: "°C", Decimals: -1},
			3: {Name: "Power", Gain: 1000, Unit: "W", Decimals: 0},
		}})
	if err != nil {
		t.Fatal(err)
	}
	_ = e.SetCellFormat(p.Config.ID, 1, "signed")
	_ = e.SetCellFormat(p.Config.ID, 3, "float_abcd")
	if err := e.ReadOnce(p.Config.ID); err != nil {
		t.Fatal(err)
	}
	cells := e.Polls()[0].State.Cells
	check := func(i int, value, raw, name, unit string) {
		t.Helper()
		c := cells[i]
		if c.Value != value || c.RawText != raw || c.Name != name || c.Unit != unit {
			t.Errorf("cell %d = %q raw %q name %q unit %q", i, c.Value, c.RawText, c.Name, c.Unit)
		}
	}
	check(0, "230.5", "2305", "Voltage", "V")
	check(1, "-10", "", "", "")            // untagged signed
	check(2, "50.0", "1500", "Temp", "°C") // 1500*0.1-100
	check(3, "123456", "123.456", "Power", "W")

	// Writes take engineering units, with or without the unit suffix.
	if err := e.WriteCell(p.Config.ID, 0, "231.2 V", false); err != nil {
		t.Fatal(err)
	}
	if err := e.WriteCell(p.Config.ID, 2, "-12.5", false); err != nil {
		t.Fatal(err)
	}
	if err := e.WriteCell(p.Config.ID, 3, "2500", false); err != nil {
		t.Fatal(err)
	}
	got := e.Sim.Store.Read(simulator.HoldingRegisters, 0, 5)
	if got[0] != 2312 || got[2] != 875 || got[3] != 0x4020 || got[4] != 0 {
		t.Fatalf("store %v", got)
	}
	// Out of range after scaling is rejected, not truncated.
	if err := e.WriteCell(p.Config.ID, 0, "7000", false); err == nil {
		t.Fatal("70000 raw must not fit unsigned 16-bit")
	}
	// Removing a tag restores the raw display.
	if err := e.SetTag(p.Config.ID, 0, nil); err != nil {
		t.Fatal(err)
	}
	_ = e.ReadOnce(p.Config.ID)
	if c := e.Polls()[0].State.Cells[0]; c.Value != "2312" || c.Name != "" {
		t.Fatalf("after remove %+v", c)
	}
}
