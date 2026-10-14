package engine

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"
)

// EventAlarms carries the alarm list whenever it changes.
const EventAlarms = "alarms"

// Alarm levels. HiHi, LoLo and Comm are critical; the others warnings.
const (
	LevelHiHi  = "hihi"
	LevelHi    = "hi"
	LevelLo    = "lo"
	LevelLoLo  = "lolo"
	LevelState = "state" // a bit in its alarm state
	LevelComm  = "comm"  // the poll window keeps failing
)

// commAlarmAfter is how many failed reads in a row raise a comm alarm.
const commAlarmAfter = 3

// AlarmLimits configures alarms for one tag. Nil limits are off.
type AlarmLimits struct {
	Enabled  bool     `json:"enabled"`
	HiHi     *float64 `json:"hiHi"`
	Hi       *float64 `json:"hi"`
	Lo       *float64 `json:"lo"`
	LoLo     *float64 `json:"loLo"`
	Deadband float64  `json:"deadband"` // a limit clears only this far back inside
	OnState  *int     `json:"onState"`  // bits: alarm while the bit equals this (0 or 1)
}

func normalizeAlarm(a *AlarmLimits) error {
	if a == nil {
		return nil
	}
	for _, p := range []*float64{a.HiHi, a.Hi, a.Lo, a.LoLo} {
		if p != nil && (math.IsNaN(*p) || math.IsInf(*p, 0)) {
			return errors.New("alarm limits must be numbers")
		}
	}
	if a.Deadband < 0 {
		return errors.New("the deadband cannot be negative")
	}
	order := []*float64{a.LoLo, a.Lo, a.Hi, a.HiHi}
	last := math.Inf(-1)
	for _, p := range order {
		if p == nil {
			continue
		}
		if *p < last {
			return errors.New("limits must be in order: low-low ≤ low ≤ high ≤ high-high")
		}
		last = *p
	}
	if a.OnState != nil && *a.OnState != 0 && *a.OnState != 1 {
		return errors.New("a bit alarm state must be 0 or 1")
	}
	return nil
}

// evalLevel returns the alarm level of v. prev is the current level, so a
// limit that is active clears only once v is Deadband back inside it.
func (a *AlarmLimits) evalLevel(v float64, bit bool, prev string) string {
	if a == nil || !a.Enabled {
		return ""
	}
	if bit {
		if a.OnState != nil && int(v) == *a.OnState {
			return LevelState
		}
		return ""
	}
	db := a.Deadband
	high := func(th *float64, active bool) bool {
		if th == nil {
			return false
		}
		if active {
			return v > *th-db
		}
		return v >= *th
	}
	low := func(th *float64, active bool) bool {
		if th == nil {
			return false
		}
		if active {
			return v < *th+db
		}
		return v <= *th
	}
	switch {
	case high(a.HiHi, prev == LevelHiHi):
		return LevelHiHi
	case high(a.Hi, prev == LevelHi || prev == LevelHiHi):
		return LevelHi
	case low(a.LoLo, prev == LevelLoLo):
		return LevelLoLo
	case low(a.Lo, prev == LevelLo || prev == LevelLoLo):
		return LevelLo
	}
	return ""
}

func (a *AlarmLimits) limitFor(level string) float64 {
	pick := map[string]*float64{LevelHiHi: a.HiHi, LevelHi: a.Hi, LevelLo: a.Lo, LevelLoLo: a.LoLo}[level]
	if pick != nil {
		return *pick
	}
	if level == LevelState && a.OnState != nil {
		return float64(*a.OnState)
	}
	return 0
}

// Severity of a level: "critical" or "warning".
func Severity(level string) string {
	switch level {
	case LevelHiHi, LevelLoLo, LevelComm:
		return "critical"
	}
	return "warning"
}

var levelRank = map[string]int{"": 0, LevelState: 1, LevelHi: 1, LevelLo: 1, LevelHiHi: 2, LevelLoLo: 2, LevelComm: 2}

// Alarm is one entry of the alarm list. It stays until it has both
// returned to normal and been acknowledged.
type Alarm struct {
	ID        string  `json:"id"`
	PollID    string  `json:"pollId"`
	Address   int     `json:"address"` // -1 for comm alarms
	Window    string  `json:"window"`
	Name      string  `json:"name"`
	Unit      string  `json:"unit"`
	Level     string  `json:"level"`
	Severity  string  `json:"severity"`
	Value     float64 `json:"value"` // value when the alarm (or its last level change) was raised
	Limit     float64 `json:"limit"`
	Message   string  `json:"message"`
	Active    bool    `json:"active"`
	Acked     bool    `json:"acked"`
	Since     int64   `json:"since"` // unix ms of activation
	ClearedAt int64   `json:"clearedAt"`
	AckedAt   int64   `json:"ackedAt"`
}

// AlarmEvent is one line of the alarm history.
type AlarmEvent struct {
	Time    int64   `json:"t"`
	Kind    string  `json:"kind"` // active, level, cleared, ack
	ID      string  `json:"id"`
	Window  string  `json:"window"`
	Name    string  `json:"name"`
	Level   string  `json:"level"`
	Value   float64 `json:"value"`
	Limit   float64 `json:"limit"`
	Unit    string  `json:"unit"`
	Message string  `json:"message"`
}

type alarmTable struct {
	mu   sync.Mutex
	list map[string]*Alarm
}

// transition is a level change found while evaluating a poll read.
type transition struct {
	id, window, name, unit, level, message string
	pollID                                 string
	address                                int
	value, limit                           float64
}

func alarmID(pollID string, address int) string {
	if address < 0 {
		return pollID + ":comm"
	}
	return pollID + ":" + strconv.Itoa(address)
}

// evaluateAlarms updates the poll's alarm levels from its cells (p.mu held)
// and returns the changes. read is false when no request was made (limits
// changed), so the communication alarm is left alone.
func (p *poll) evaluateAlarms(readErr error, read bool) []transition {
	if p.levels == nil {
		p.levels = map[int]string{}
	}
	var out []transition
	set := func(addr int, level, name, unit, msg string, v, limit float64) {
		if p.levels[addr] == level {
			return
		}
		if level == "" {
			delete(p.levels, addr)
		} else {
			p.levels[addr] = level
		}
		out = append(out, transition{id: alarmID(p.cfg.ID, addr), pollID: p.cfg.ID, address: addr, window: p.cfg.Name,
			name: name, unit: unit, level: level, message: msg, value: v, limit: limit})
	}
	// Communication alarm.
	switch {
	case !read:
	case p.cfg.CommAlarm && readErr != nil:
		p.failStreak++
		if p.failStreak >= commAlarmAfter {
			set(-1, LevelComm, "Communication", "", describeErr(readErr), 0, 0)
		}
	default:
		p.failStreak = 0
		set(-1, "", "Communication", "", "", 0, 0)
	}
	if readErr != nil {
		return out // keep value alarms as they are until data arrives
	}
	bit := isBitPoll(p.cfg)
	for i := range p.st.Cells {
		c := &p.st.Cells[i]
		if c.Span == 0 {
			continue
		}
		t, ok := p.cfg.Tags[c.Address]
		if !ok || t.Alarm == nil || !t.Alarm.Enabled || !c.HasNum {
			set(c.Address, "", "", "", "", 0, 0)
			continue
		}
		lvl := t.Alarm.evalLevel(c.Num, bit, p.levels[c.Address])
		name := t.Name
		if name == "" {
			name = fmt.Sprintf("Address %d", c.Address)
		}
		set(c.Address, lvl, name, t.Unit, "", c.Num, t.Alarm.limitFor(lvl))
	}
	p.applyLevels()
	return out
}

// applyLevels copies alarm levels into the cells (p.mu held).
func (p *poll) applyLevels() {
	for i := range p.st.Cells {
		p.st.Cells[i].Alarm = p.levels[p.st.Cells[i].Address]
	}
}

// applyTransitions updates the alarm list and returns history events.
func (e *Engine) applyTransitions(ts []transition) []AlarmEvent {
	if len(ts) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	e.alarms.mu.Lock()
	defer e.alarms.mu.Unlock()
	if e.alarms.list == nil {
		e.alarms.list = map[string]*Alarm{}
	}
	var events []AlarmEvent
	for _, t := range ts {
		a := e.alarms.list[t.id]
		ev := AlarmEvent{Time: now, ID: t.id, Window: t.window, Name: t.name, Level: t.level, Value: t.value, Limit: t.limit, Unit: t.unit, Message: t.message}
		switch {
		case t.level == "" && a != nil && a.Active:
			a.Active, a.ClearedAt = false, now
			ev.Kind, ev.Level, ev.Name = "cleared", a.Level, a.Name
			if a.Acked {
				delete(e.alarms.list, t.id)
			}
		case t.level == "":
			continue
		case a == nil || !a.Active:
			if a == nil {
				a = &Alarm{ID: t.id, PollID: t.pollID, Address: t.address}
				e.alarms.list[t.id] = a
			}
			*a = Alarm{ID: t.id, PollID: t.pollID, Address: t.address, Window: t.window, Name: t.name, Unit: t.unit,
				Level: t.level, Severity: Severity(t.level), Value: t.value, Limit: t.limit, Message: t.message, Active: true, Since: now}
			ev.Kind = "active"
		default: // level change while active
			if levelRank[t.level] > levelRank[a.Level] {
				a.Acked, a.AckedAt = false, 0
			}
			a.Level, a.Severity, a.Value, a.Limit, a.Message = t.level, Severity(t.level), t.value, t.limit, t.message
			ev.Kind = "level"
		}
		events = append(events, ev)
	}
	return events
}

// Alarms returns the alarm list, most severe and newest first.
func (e *Engine) Alarms() []Alarm {
	e.alarms.mu.Lock()
	defer e.alarms.mu.Unlock()
	out := make([]Alarm, 0, len(e.alarms.list))
	for _, a := range e.alarms.list {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Active != b.Active {
			return a.Active
		}
		if levelRank[a.Level] != levelRank[b.Level] {
			return levelRank[a.Level] > levelRank[b.Level]
		}
		return a.Since > b.Since
	})
	return out
}

// AckAlarms acknowledges the given alarms (none = all).
func (e *Engine) AckAlarms(ids []string) {
	now := time.Now().UnixMilli()
	var events []AlarmEvent
	e.alarms.mu.Lock()
	for id, a := range e.alarms.list {
		if len(ids) > 0 && !contains(ids, id) || a.Acked {
			continue
		}
		a.Acked, a.AckedAt = true, now
		events = append(events, AlarmEvent{Time: now, Kind: "ack", ID: id, Window: a.Window, Name: a.Name, Level: a.Level, Value: a.Value, Limit: a.Limit, Unit: a.Unit})
		if !a.Active {
			delete(e.alarms.list, id)
		}
	}
	e.alarms.mu.Unlock()
	e.alarmsChanged(events)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// alarmsChanged logs events and tells the UI.
func (e *Engine) alarmsChanged(events []AlarmEvent) {
	if len(events) == 0 {
		return
	}
	if e.OnAlarm != nil {
		for _, ev := range events {
			e.OnAlarm(ev)
		}
	}
	e.emit(EventAlarms, e.Alarms())
}

// dropAlarms removes the alarms of a poll window (when it is deleted).
func (e *Engine) dropAlarms(pollID string) {
	e.alarms.mu.Lock()
	changed := false
	for id, a := range e.alarms.list {
		if a.PollID == pollID {
			delete(e.alarms.list, id)
			changed = true
		}
	}
	e.alarms.mu.Unlock()
	if changed {
		e.emit(EventAlarms, e.Alarms())
	}
}

func isBitPoll(c PollConfig) bool { return c.Function == 1 || c.Function == 2 }
