package engine

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"modbusstudio/internal/datafmt"
	"modbusstudio/internal/modbus"
)

// EventSequence carries SequenceRun updates.
const EventSequence = "sequence"

// SeqStep is one step of a test sequence.
type SeqStep struct {
	Kind  string `json:"kind"` // read, write, wait, prompt
	Label string `json:"label"`

	// read / write
	SlaveID  int    `json:"slaveId"` // 0 = the sequence's slave
	Function int    `json:"function"`
	Address  int    `json:"address"`
	Format   string `json:"format"`

	// read: scaling and the check applied to the scaled value
	Gain      float64 `json:"gain"`
	Offset    float64 `json:"offset"`
	Check     string  `json:"check"` // "" (record only), eq, ne, lt, le, gt, ge, range
	Value     float64 `json:"value"`
	Min       float64 `json:"min"`
	Max       float64 `json:"max"`
	Tolerance float64 `json:"tolerance"` // for eq / ne

	Values string `json:"values"` // write: comma separated values in Format
	WaitMs int    `json:"waitMs"` // wait
	Text   string `json:"text"`   // prompt: instruction for the operator
}

// Sequence is a test sequence.
type Sequence struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ConnectionID string    `json:"connectionId"`
	SlaveID      int       `json:"slaveId"`
	Repeat       int       `json:"repeat"`     // runs (1 = once)
	StopOnFail   bool      `json:"stopOnFail"` // stop at the first failed step
	StepDelayMs  int       `json:"stepDelayMs"`
	Steps        []SeqStep `json:"steps"`
}

// StepResult is the outcome of one step in one run.
type StepResult struct {
	Run     int     `json:"run"` // 1-based
	Index   int     `json:"index"`
	Label   string  `json:"label"`
	Status  string  `json:"status"` // pass, fail, error, done (no check), skipped
	Value   string  `json:"value"`  // value read, as shown
	Expect  string  `json:"expect"` // the check, readable
	Message string  `json:"message"`
	Ms      float64 `json:"ms"`
	Time    int64   `json:"time"`
}

// SequenceRun is the progress and report of the current or last run.
type SequenceRun struct {
	SequenceID string       `json:"sequenceId"`
	Name       string       `json:"name"`
	Connection string       `json:"connection"`
	Running    bool         `json:"running"`
	Prompt     string       `json:"prompt"` // waiting for the operator when set
	Run        int          `json:"run"`
	Runs       int          `json:"runs"`
	Step       int          `json:"step"` // index being executed
	Started    int64        `json:"started"`
	Finished   int64        `json:"finished"`
	Verdict    string       `json:"verdict"` // pass, fail, stopped, error
	Error      string       `json:"error"`
	Passed     int          `json:"passed"`
	Failed     int          `json:"failed"`
	Errors     int          `json:"errors"`
	Results    []StepResult `json:"results"`
	Seq        uint64       `json:"seq"`
}

type sequencer struct {
	mu     sync.Mutex
	run    SequenceRun
	cancel context.CancelFunc
	answer chan bool // operator answer to a prompt: true = continue, false = fail
}

// SequenceState returns the current run.
func (e *Engine) SequenceState() SequenceRun {
	e.seq.mu.Lock()
	defer e.seq.mu.Unlock()
	return e.seq.copyLocked()
}

func (s *sequencer) copyLocked() SequenceRun {
	r := s.run
	r.Results = append([]StepResult{}, s.run.Results...)
	return r
}

func (e *Engine) seqUpdate(fn func(*SequenceRun)) {
	e.seq.mu.Lock()
	fn(&e.seq.run)
	e.seq.run.Seq++
	r := e.seq.copyLocked()
	e.seq.mu.Unlock()
	e.emit(EventSequence, r)
}

// StopSequence cancels the running sequence.
func (e *Engine) StopSequence() {
	e.seq.mu.Lock()
	c := e.seq.cancel
	e.seq.mu.Unlock()
	if c != nil {
		c()
	}
}

// AnswerPrompt continues (ok) or fails (not ok) a waiting prompt step.
func (e *Engine) AnswerPrompt(ok bool) {
	e.seq.mu.Lock()
	ch := e.seq.answer
	e.seq.mu.Unlock()
	if ch != nil {
		select {
		case ch <- ok:
		default:
		}
	}
}

// CheckText describes a step's check, e.g. "229 … 231" or "= 1 ± 0.5".
func (s SeqStep) CheckText() string {
	n := func(v float64) string { return strconv.FormatFloat(v, 'g', 10, 64) }
	tol := ""
	if s.Tolerance > 0 {
		tol = " ± " + n(s.Tolerance)
	}
	switch s.Check {
	case "eq":
		return "= " + n(s.Value) + tol
	case "ne":
		return "≠ " + n(s.Value) + tol
	case "lt":
		return "< " + n(s.Value)
	case "le":
		return "≤ " + n(s.Value)
	case "gt":
		return "> " + n(s.Value)
	case "ge":
		return "≥ " + n(s.Value)
	case "range":
		return n(s.Min) + " … " + n(s.Max)
	}
	return ""
}

func (s SeqStep) passes(v float64) bool {
	switch s.Check {
	case "eq":
		return math.Abs(v-s.Value) <= s.Tolerance
	case "ne":
		return math.Abs(v-s.Value) > s.Tolerance
	case "lt":
		return v < s.Value
	case "le":
		return v <= s.Value
	case "gt":
		return v > s.Value
	case "ge":
		return v >= s.Value
	case "range":
		return v >= s.Min && v <= s.Max
	}
	return true
}

func validateSequence(s *Sequence) error {
	if len(s.Steps) == 0 {
		return errors.New("the sequence has no steps")
	}
	if s.Repeat < 1 {
		s.Repeat = 1
	}
	if s.Repeat > 10000 {
		return errors.New("repeat at most 10000 times")
	}
	for i := range s.Steps {
		st := &s.Steps[i]
		where := fmt.Sprintf("step %d", i+1)
		switch st.Kind {
		case "read":
			if st.Function < 1 || st.Function > 4 {
				return fmt.Errorf("%s: read with function 1-4", where)
			}
			if st.Gain == 0 {
				st.Gain = 1
			}
			if st.Function > 2 && !datafmt.Valid(datafmt.Format(st.Format)) {
				st.Format = string(datafmt.Signed)
			}
		case "write":
			switch st.Function {
			case 5, 6, 15, 16:
			default:
				return fmt.Errorf("%s: write with function 5, 6, 15 or 16", where)
			}
			if strings.TrimSpace(st.Values) == "" {
				return fmt.Errorf("%s: enter the value to write", where)
			}
			if !datafmt.Valid(datafmt.Format(st.Format)) {
				st.Format = string(datafmt.Signed)
			}
		case "wait":
			if st.WaitMs < 0 || st.WaitMs > 3_600_000 {
				return fmt.Errorf("%s: wait 0 to 3600000 ms", where)
			}
		case "prompt":
		default:
			return fmt.Errorf("%s: unknown step type %q", where, st.Kind)
		}
	}
	return nil
}

// RunSequence starts a sequence in the background.
func (e *Engine) RunSequence(s Sequence) error {
	if err := validateSequence(&s); err != nil {
		return err
	}
	c := e.conn(s.ConnectionID)
	if c == nil {
		return errors.New("choose a connection for the sequence")
	}
	e.seq.mu.Lock()
	if e.seq.run.Running {
		e.seq.mu.Unlock()
		return errors.New("a sequence is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.seq.cancel = cancel
	e.seq.answer = make(chan bool, 1)
	e.seq.run = SequenceRun{SequenceID: s.ID, Name: s.Name, Connection: c.cfg.Name, Running: true, Runs: s.Repeat,
		Started: time.Now().UnixMilli(), Results: []StepResult{}, Seq: e.seq.run.Seq + 1}
	e.seq.mu.Unlock()
	e.seqUpdate(func(*SequenceRun) {})
	go func() {
		defer cancel()
		verdict, err := e.runSequence(ctx, c, s)
		e.seqUpdate(func(r *SequenceRun) {
			r.Running, r.Prompt, r.Finished = false, "", time.Now().UnixMilli()
			r.Verdict = verdict
			if err != nil {
				r.Error = err.Error()
			}
		})
	}()
	return nil
}

func (e *Engine) runSequence(ctx context.Context, c *connection, s Sequence) (string, error) {
	timeout := time.Duration(c.cfg.TimeoutMs) * time.Millisecond
	failed := false
	for run := 1; run <= s.Repeat; run++ {
		e.seqUpdate(func(r *SequenceRun) { r.Run = run })
		for i, st := range s.Steps {
			if ctx.Err() != nil {
				return "stopped", nil
			}
			e.seqUpdate(func(r *SequenceRun) { r.Step = i })
			res := e.runStep(ctx, c.client, timeout, s, st)
			res.Run, res.Index, res.Time = run, i, time.Now().UnixMilli()
			if res.Label == "" {
				res.Label = st.Label
			}
			if ctx.Err() != nil && res.Status != "pass" && res.Status != "done" {
				return "stopped", nil
			}
			e.seqUpdate(func(r *SequenceRun) {
				r.Results = append(r.Results, res)
				switch res.Status {
				case "pass":
					r.Passed++
				case "fail":
					r.Failed++
				case "error":
					r.Errors++
				}
			})
			if res.Status == "fail" || res.Status == "error" {
				failed = true
				if s.StopOnFail {
					return "fail", nil
				}
			}
			if s.StepDelayMs > 0 && i < len(s.Steps)-1 {
				if !sleepCtx(ctx, time.Duration(s.StepDelayMs)*time.Millisecond) {
					return "stopped", nil
				}
			}
		}
	}
	if failed {
		return "fail", nil
	}
	return "pass", nil
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func (e *Engine) runStep(ctx context.Context, cl *modbus.Client, timeout time.Duration, s Sequence, st SeqStep) StepResult {
	unit := s.SlaveID
	if st.SlaveID > 0 {
		unit = st.SlaveID
	}
	start := time.Now()
	done := func(r StepResult) StepResult {
		r.Ms = float64(time.Since(start).Microseconds()) / 1000
		return r
	}
	switch st.Kind {
	case "wait":
		if !sleepCtx(ctx, time.Duration(st.WaitMs)*time.Millisecond) {
			return done(StepResult{Status: "skipped", Message: "stopped"})
		}
		return done(StepResult{Status: "done", Message: fmt.Sprintf("waited %d ms", st.WaitMs)})

	case "prompt":
		e.seq.mu.Lock()
		ch := e.seq.answer
		e.seq.mu.Unlock()
		select { // drop a stale answer
		case <-ch:
		default:
		}
		e.seqUpdate(func(r *SequenceRun) { r.Prompt = st.Text })
		defer e.seqUpdate(func(r *SequenceRun) { r.Prompt = "" })
		select {
		case <-ctx.Done():
			return done(StepResult{Status: "skipped", Message: "stopped"})
		case ok := <-ch:
			if ok {
				return done(StepResult{Status: "pass", Message: "confirmed by the operator"})
			}
			return done(StepResult{Status: "fail", Message: "failed by the operator"})
		}

	case "write":
		w := WriteRequest{ConnectionID: s.ConnectionID, SlaveID: unit, Function: st.Function, Address: st.Address}
		for _, item := range strings.FieldsFunc(st.Values, func(r rune) bool { return r == ',' || r == ';' || r == ' ' }) {
			if st.Function == 5 || st.Function == 15 {
				on, err := parseBool(item)
				if err != nil {
					return done(StepResult{Status: "error", Message: err.Error()})
				}
				w.Values = append(w.Values, b2i(on))
				continue
			}
			regs, err := datafmt.ParseValue(datafmt.Format(st.Format), item)
			if err != nil {
				return done(StepResult{Status: "error", Message: err.Error()})
			}
			for _, r := range regs {
				w.Values = append(w.Values, int(r))
			}
		}
		if st.Function == 5 || st.Function == 6 {
			if len(w.Values) != 1 {
				return done(StepResult{Status: "error", Message: fmt.Sprintf("function %d writes one value", st.Function)})
			}
		}
		if err := e.Write(w); err != nil {
			return done(StepResult{Status: "error", Message: err.Error()})
		}
		return done(StepResult{Status: "done", Value: st.Values, Message: fmt.Sprintf("wrote %d value(s)", len(w.Values))})

	case "read":
		fc := byte(st.Function)
		qty := 1
		if !modbus.IsBitFunction(fc) {
			qty = datafmt.Registers(datafmt.Format(st.Format))
		}
		bits, regs, _, err := cl.Read(byte(unit), fc, st.Address, qty, timeout)
		if err != nil {
			return done(StepResult{Status: "error", Expect: st.CheckText(), Message: describeErr(err)})
		}
		var v float64
		var shown string
		if modbus.IsBitFunction(fc) {
			v = float64(b2i(bits[0]))
			shown = strconv.Itoa(int(v))
		} else {
			n, ok := datafmt.Numeric(datafmt.Format(st.Format), regs)
			if !ok {
				return done(StepResult{Status: "error", Message: "the value is not a number"})
			}
			v = n*st.Gain + st.Offset
			digits := 10
			if strings.HasPrefix(st.Format, "float") {
				digits = 7 // a float32 has about 7 significant digits
			}
			shown = strconv.FormatFloat(v, 'g', digits, 64)
			v, _ = strconv.ParseFloat(shown, 64) // check what is shown, without float noise
		}
		r := StepResult{Value: shown, Expect: st.CheckText(), Status: "done"}
		if st.Check != "" {
			r.Status = "fail"
			if st.passes(v) {
				r.Status = "pass"
			}
		}
		return done(r)
	}
	return done(StepResult{Status: "error", Message: "unknown step"})
}
