package workspace

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"modbusstudio/internal/engine"
	"modbusstudio/internal/modbus"
	"modbusstudio/internal/simulator"
)

func TestRoundTripThroughEngine(t *testing.T) {
	e := engine.New(nil)
	defer e.Shutdown()
	c, err := e.SaveConnection(engine.ConnectionConfig{Name: "Meter bus", Mode: modbus.ModeTCP, Host: "10.0.0.5", Port: 502})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := e.CreatePoll(engine.PollConfig{Name: "Energy", ConnectionID: c.ID, SlaveID: 3, Function: 3, Address: 100, Quantity: 8, ScanRateMs: 500, Format: "unsigned"})
	_ = e.SetCellFormat(p.Config.ID, 102, "float_cdab")
	e.Sim.Store.Write(simulator.HoldingRegisters, 40, []int{1, 0, 0, 2})
	e.Sim.Store.Write(simulator.Coils, 9, []int{1})

	f := New()
	f.Connections, f.Polls = e.Definitions()
	f.Layout = Layout{View: "tile", ActiveID: p.Config.ID, ShowTraffic: false}
	f.Charts = []byte(`[{"id":"k1"}]`)
	f.Simulator = &Simulator{Config: simulator.Config{Mode: modbus.ModeTCP, Port: 5020, SlaveID: 1}, Data: e.Sim.Store.Snapshot()}

	path := filepath.Join(t.TempDir(), "plant"+Extension)
	if err := Write(path, f); err != nil {
		t.Fatal(err)
	}
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}

	e2 := engine.New(nil)
	defer e2.Shutdown()
	if w := e2.Load(got.Connections, got.Polls); len(w) != 0 {
		t.Fatalf("warnings %v", w)
	}
	e2.Sim.Store.Restore(got.Simulator.Data)

	polls := e2.Polls()
	if len(polls) != 1 || polls[0].Config.ID != p.Config.ID || polls[0].Config.CellFormats[102] != "float_cdab" || polls[0].State.Running {
		t.Fatalf("polls %+v", polls)
	}
	if cs := e2.Connections(); len(cs) != 1 || cs[0].ID != c.ID || cs[0].Host != "10.0.0.5" {
		t.Fatalf("connections %+v", cs)
	}
	if v := e2.Sim.Store.Read(simulator.HoldingRegisters, 40, 4); v[0] != 1 || v[3] != 2 {
		t.Fatalf("sim holding %v", v)
	}
	if v := e2.Sim.Store.Read(simulator.Coils, 9, 1); v[0] != 1 {
		t.Fatalf("sim coils %v", v)
	}
	var compact bytes.Buffer
	_ = json.Compact(&compact, got.Charts)
	if got.Layout.View != "tile" || compact.String() != `[{"id":"k1"}]` {
		t.Fatalf("layout/charts %+v %s", got.Layout, got.Charts)
	}
	// New objects must not reuse saved IDs.
	np, _ := e2.CreatePoll(engine.PollConfig{ConnectionID: c.ID, SlaveID: 1, Function: 3, Quantity: 1})
	if np.Config.ID == p.Config.ID {
		t.Fatalf("id reused: %s", np.Config.ID)
	}
}

func TestDecodeRejects(t *testing.T) {
	if _, err := Decode([]byte(`{"format":"other","version":1}`)); err == nil {
		t.Error("foreign file accepted")
	}
	if _, err := Decode([]byte(`{"format":"appmodbus-workspace","version":99}`)); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Errorf("newer version: %v", err)
	}
	if _, err := Decode([]byte(`not json`)); err == nil {
		t.Error("garbage accepted")
	}
}

func TestLoadSkipsInvalidEntries(t *testing.T) {
	e := engine.New(nil)
	defer e.Shutdown()
	warn := e.Load(
		[]engine.ConnectionConfig{{ID: "c1", Mode: modbus.ModeTCP, Host: "", Port: 502}},
		[]engine.PollConfig{{ID: "p1", Function: 3, Quantity: 500}},
	)
	if len(warn) != 2 || len(e.Polls()) != 0 || len(e.Connections()) != 0 {
		t.Fatalf("warnings %v", warn)
	}
}

func TestSessionAndAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	sp := filepath.Join(dir, "session.json")
	if err := WriteSession(sp, Session{Path: "/x/a.mbw", Dirty: true, File: New()}); err != nil {
		t.Fatal(err)
	}
	s, err := ReadSession(sp)
	if err != nil || s.Path != "/x/a.mbw" || !s.Dirty {
		t.Fatalf("%+v %v", s, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp files left: %v", entries)
	}
	if WithExtension("a") != "a.mbw" || WithExtension("b.MBW") != "b.MBW" || DisplayName("/q/plant.mbw") != "plant" || DisplayName("") != "Untitled" {
		t.Fatal("name helpers")
	}
}
