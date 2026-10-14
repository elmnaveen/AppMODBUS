package engine

import (
	"testing"
	"time"

	"modbusstudio/internal/modbus"
	"modbusstudio/internal/simulator"
)

func f(v float64) *float64 { return &v }

func TestAlarmLevels(t *testing.T) {
	a := &AlarmLimits{Enabled: true, LoLo: f(0), Lo: f(10), Hi: f(100), HiHi: f(200), Deadband: 5}
	cases := []struct {
		v          float64
		prev, want string
	}{
		{50, "", ""},
		{100, "", LevelHi},
		{97, LevelHi, LevelHi}, // inside the deadband: stays
		{95, LevelHi, ""},
		{250, "", LevelHiHi},
		{197, LevelHiHi, LevelHiHi},
		{150, LevelHiHi, LevelHi},
		{10, "", LevelLo},
		{14, LevelLo, LevelLo},
		{15, LevelLo, ""},
		{-1, LevelLo, LevelLoLo},
	}
	for _, c := range cases {
		if got := a.evalLevel(c.v, false, c.prev); got != c.want {
			t.Errorf("v=%v prev=%q: got %q, want %q", c.v, c.prev, got, c.want)
		}
	}
	if err := normalizeAlarm(&AlarmLimits{Lo: f(10), Hi: f(5)}); err == nil {
		t.Error("expected an order error")
	}
}

func TestAlarmLifecycle(t *testing.T) {
	var events []AlarmEvent
	e := New(nil)
	e.OnAlarm = func(ev AlarmEvent) { events = append(events, ev) }
	defer e.Shutdown()
	port := startSimCfg(t, e, simulator.Config{Mode: modbus.ModeTCP})
	conn, _ := e.SaveConnection(ConnectionConfig{Mode: modbus.ModeTCP, Host: "127.0.0.1", Port: port, TimeoutMs: 500})
	p, _ := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 1, Function: 3, Address: 0, Quantity: 2, ScanRateMs: 100})
	id := p.Config.ID
	if err := e.SetTag(id, 0, &Tag{Name: "Temp", Gain: 1, Decimals: -1, Unit: "°C", Alarm: &AlarmLimits{Enabled: true, Hi: f(80), HiHi: f(90), Deadband: 2}}); err != nil {
		t.Fatal(err)
	}
	read := func(v int) {
		t.Helper()
		e.Sim.Store.Write(simulator.HoldingRegisters, 0, []int{v})
		if err := e.ReadOnce(id); err != nil {
			t.Fatal(err)
		}
	}
	read(50)
	if len(e.Alarms()) != 0 {
		t.Fatalf("no alarm expected: %+v", e.Alarms())
	}
	read(85)
	al := e.Alarms()
	if len(al) != 1 || al[0].Level != LevelHi || !al[0].Active || al[0].Acked || al[0].Name != "Temp" || al[0].Limit != 80 {
		t.Fatalf("hi alarm %+v", al)
	}
	if c := e.Polls()[0].State.Cells[0]; c.Alarm != LevelHi {
		t.Fatalf("cell alarm %q", c.Alarm)
	}
	e.AckAlarms(nil)
	read(95) // escalates: needs a new acknowledgement
	if al := e.Alarms(); al[0].Level != LevelHiHi || al[0].Acked {
		t.Fatalf("hihi %+v", al)
	}
	read(70) // back to normal but not acknowledged: stays listed
	if al := e.Alarms(); len(al) != 1 || al[0].Active {
		t.Fatalf("cleared %+v", al)
	}
	e.AckAlarms([]string{al[0].ID})
	if len(e.Alarms()) != 0 {
		t.Fatalf("should be gone: %+v", e.Alarms())
	}
	kinds := ""
	for _, ev := range events {
		kinds += ev.Kind + " "
	}
	if kinds != "active ack level cleared ack " {
		t.Fatalf("events %q", kinds)
	}

	// Removing the alarm config clears a standing alarm.
	read(85)
	if err := e.SetTag(id, 0, &Tag{Name: "Temp", Gain: 1, Decimals: -1}); err != nil {
		t.Fatal(err)
	}
	if al := e.Alarms(); len(al) != 1 || al[0].Active {
		t.Fatalf("after removing limits %+v", al)
	}

	// Bit alarm and comm alarm.
	bp, _ := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 1, Function: 1, Address: 0, Quantity: 1, ScanRateMs: 100})
	on := 1
	_ = e.SetTag(bp.Config.ID, 0, &Tag{Name: "Trip", Gain: 1, Decimals: -1, Alarm: &AlarmLimits{Enabled: true, OnState: &on}})
	e.Sim.Store.Write(simulator.Coils, 0, []int{1})
	_ = e.ReadOnce(bp.Config.ID)
	found := false
	for _, a := range e.Alarms() {
		found = found || a.Name == "Trip" && a.Level == LevelState && a.Severity == "warning"
	}
	if !found {
		t.Fatalf("bit alarm missing: %+v", e.Alarms())
	}
	cp, _ := e.CreatePoll(PollConfig{ConnectionID: conn.ID, SlaveID: 9, Function: 3, Address: 0, Quantity: 1, ScanRateMs: 100, CommAlarm: true})
	for i := 0; i < commAlarmAfter; i++ {
		_ = e.ReadOnce(cp.Config.ID)
	}
	found = false
	for _, a := range e.Alarms() {
		found = found || a.Level == LevelComm && a.Severity == "critical" && a.Message != ""
	}
	if !found {
		t.Fatalf("comm alarm missing: %+v", e.Alarms())
	}
	e.DeletePoll(cp.Config.ID)
	for _, a := range e.Alarms() {
		if a.Level == LevelComm {
			t.Fatal("comm alarm should go with its window")
		}
	}
}

func TestAlarmLog(t *testing.T) {
	l := NewAlarmLog(t.TempDir())
	base := int64(1_700_000_000_000)
	for i := 0; i < 5; i++ {
		if err := l.Append(AlarmEvent{Time: base + int64(i)*86_400_000/2, Kind: "active", Name: "T", Value: float64(i)}); err != nil {
			t.Fatal(err)
		}
	}
	all, err := l.Query(base, base+3*86_400_000, 0)
	if err != nil || len(all) != 5 || all[0].Value != 4 || all[4].Value != 0 {
		t.Fatalf("query %v %v", all, err)
	}
	if two, _ := l.Query(base, base+3*86_400_000, 2); len(two) != 2 || two[1].Value != 3 {
		t.Fatalf("limit %v", two)
	}
	if some, _ := l.Query(base+86_400_000/2, base+86_400_000, 0); len(some) != 2 {
		t.Fatalf("range %v", some)
	}
}

func TestSequence(t *testing.T) {
	e := New(nil)
	defer e.Shutdown()
	port := startSimCfg(t, e, simulator.Config{Mode: modbus.ModeTCP})
	e.Sim.Store.Write(simulator.InputRegisters, 3, []int{2301})
	conn, _ := e.SaveConnection(ConnectionConfig{Mode: modbus.ModeTCP, Host: "127.0.0.1", Port: port, TimeoutMs: 500})
	seq := Sequence{ID: "s1", Name: "Meter check", ConnectionID: conn.ID, SlaveID: 1, Repeat: 2, Steps: []SeqStep{
		{Kind: "read", Label: "Voltage L1", Function: 4, Address: 3, Format: "unsigned", Gain: 0.1, Check: "range", Min: 225, Max: 235},
		{Kind: "write", Label: "Set CT ratio", Function: 6, Address: 0, Format: "signed", Values: "500"},
		{Kind: "read", Label: "CT ratio", Function: 3, Address: 0, Format: "signed", Check: "eq", Value: 500},
		{Kind: "write", Label: "Relay on", Function: 5, Address: 2, Values: "on"},
		{Kind: "read", Label: "Relay", Function: 1, Address: 2, Check: "eq", Value: 1},
		{Kind: "wait", WaitMs: 10},
		{Kind: "prompt", Text: "Check the LED"},
		{Kind: "read", Label: "Too high", Function: 4, Address: 3, Format: "unsigned", Gain: 0.1, Check: "lt", Value: 200},
	}}
	if err := e.RunSequence(seq); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		st := e.SequenceState()
		if st.Prompt != "" {
			e.AnswerPrompt(true)
		}
		if !st.Running {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	st := e.SequenceState()
	if st.Running || st.Verdict != "fail" || len(st.Results) != 16 || st.Passed != 8 || st.Failed != 2 || st.Errors != 0 {
		t.Fatalf("run %+v", st)
	}
	if r := st.Results[0]; r.Status != "pass" || r.Value != "230.1" || r.Expect != "225 … 235" {
		t.Fatalf("first step %+v", r)
	}
	if r := st.Results[15]; r.Run != 2 || r.Status != "fail" {
		t.Fatalf("last step %+v", r)
	}

	// Stop on the first failure.
	seq.StopOnFail, seq.Repeat = true, 1
	seq.Steps = []SeqStep{seq.Steps[7], seq.Steps[0]}
	_ = e.RunSequence(seq)
	for e.SequenceState().Running {
		time.Sleep(5 * time.Millisecond)
	}
	if st := e.SequenceState(); len(st.Results) != 1 || st.Verdict != "fail" {
		t.Fatalf("stop on fail %+v", st)
	}
}
