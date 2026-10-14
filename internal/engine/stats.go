package engine

import (
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"appmodbus/internal/modbus"
)

// recentSamples is how many response times are kept for percentiles,
// the histogram and the response-time chart.
const recentSamples = 2000

// Outcome kinds of one transaction.
const (
	OutcomeOK        = "ok"
	OutcomeTimeout   = "timeout"
	OutcomeCRC       = "crc"
	OutcomeException = "exception"
	OutcomeInvalid   = "invalid"
	OutcomeIO        = "io"
)

// Classify sorts a request error into an outcome kind.
func Classify(err error) string {
	var ex *modbus.ExceptionError
	switch {
	case err == nil:
		return OutcomeOK
	case errors.Is(err, modbus.ErrTimeout):
		return OutcomeTimeout
	case errors.As(err, &ex):
		return OutcomeException
	case strings.Contains(err.Error(), "CRC error"):
		return OutcomeCRC
	case errors.Is(err, modbus.ErrInvalidResponse):
		return OutcomeInvalid
	}
	return OutcomeIO
}

// HistBin is one bar of the response-time histogram: responses that took
// From <= ms < To (To = 0 means "and slower").
type HistBin struct {
	From  float64 `json:"from"`
	To    float64 `json:"to"`
	Count int     `json:"count"`
}

// RTSample is one recent transaction for the response-time chart.
type RTSample struct {
	T  int64   `json:"t"`  // unix ms
	Ms float64 `json:"ms"` // response time; timeouts are reported at the timeout
	OK bool    `json:"ok"` // a response arrived (data or exception)
}

// Stats summarises the transactions of a connection or poll window.
type Stats struct {
	Since      int64             `json:"since"` // unix ms of the last reset
	Requests   uint64            `json:"requests"`
	OK         uint64            `json:"ok"`
	Timeouts   uint64            `json:"timeouts"`
	CRC        uint64            `json:"crc"`
	Exceptions uint64            `json:"exceptions"`
	Invalid    uint64            `json:"invalid"`
	IOErrors   uint64            `json:"ioErrors"`
	ByCode     map[string]uint64 `json:"byCode"` // exception code (hex) -> count
	MinMs      float64           `json:"minMs"`
	AvgMs      float64           `json:"avgMs"`
	MaxMs      float64           `json:"maxMs"`
	P50Ms      float64           `json:"p50Ms"`
	P95Ms      float64           `json:"p95Ms"`
	LastError  string            `json:"lastError"`
	LastAt     int64             `json:"lastAt"`
	Histogram  []HistBin         `json:"histogram,omitempty"`
	Recent     []RTSample        `json:"recent,omitempty"`
}

// statsAcc accumulates Stats. Response times count only when the slave
// answered (data or exception).
type statsAcc struct {
	mu     sync.Mutex
	s      Stats
	sum    float64
	n      int
	recent []RTSample // ring
	next   int
}

func newStatsAcc() *statsAcc {
	a := &statsAcc{}
	a.reset()
	return a
}

func (a *statsAcc) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.s = Stats{Since: time.Now().UnixMilli(), ByCode: map[string]uint64{}}
	a.sum, a.n = 0, 0
	a.recent = a.recent[:0]
	a.next = 0
}

func (a *statsAcc) add(elapsed time.Duration, err error) {
	ms := float64(elapsed.Microseconds()) / 1000
	kind := Classify(err)
	now := time.Now().UnixMilli()
	a.mu.Lock()
	defer a.mu.Unlock()
	s := &a.s
	s.Requests++
	s.LastAt = now
	answered := false
	switch kind {
	case OutcomeOK:
		s.OK++
		answered = true
	case OutcomeTimeout:
		s.Timeouts++
	case OutcomeCRC:
		s.CRC++
	case OutcomeException:
		s.Exceptions++
		answered = true
		var ex *modbus.ExceptionError
		if errors.As(err, &ex) {
			s.ByCode[hex2(ex.Code)]++
		}
	case OutcomeInvalid:
		s.Invalid++
	default:
		s.IOErrors++
	}
	if err != nil {
		s.LastError = describeErr(err)
	}
	if answered {
		if a.n == 0 || ms < s.MinMs {
			s.MinMs = ms
		}
		if ms > s.MaxMs {
			s.MaxMs = ms
		}
		a.sum += ms
		a.n++
	}
	smp := RTSample{T: now, Ms: ms, OK: answered}
	if len(a.recent) < recentSamples {
		a.recent = append(a.recent, smp)
	} else {
		a.recent[a.next] = smp
	}
	a.next = (a.next + 1) % recentSamples
}

var histEdges = []float64{0, 1, 2, 5, 10, 20, 50, 100, 200, 500, 1000}

// snapshot returns the statistics; detail adds the histogram and the
// recent samples in time order.
func (a *statsAcc) snapshot(detail bool) Stats {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.s
	s.ByCode = make(map[string]uint64, len(a.s.ByCode))
	for k, v := range a.s.ByCode {
		s.ByCode[k] = v
	}
	if a.n > 0 {
		s.AvgMs = a.sum / float64(a.n)
	}
	ordered := make([]RTSample, 0, len(a.recent))
	if len(a.recent) == recentSamples {
		ordered = append(ordered, a.recent[a.next:]...)
		ordered = append(ordered, a.recent[:a.next]...)
	} else {
		ordered = append(ordered, a.recent...)
	}
	var times []float64
	for _, r := range ordered {
		if r.OK {
			times = append(times, r.Ms)
		}
	}
	sort.Float64s(times)
	s.P50Ms, s.P95Ms = percentile(times, 0.5), percentile(times, 0.95)
	if detail {
		s.Recent = ordered
		s.Histogram = make([]HistBin, len(histEdges))
		for i, from := range histEdges {
			to := 0.0
			if i+1 < len(histEdges) {
				to = histEdges[i+1]
			}
			s.Histogram[i] = HistBin{From: from, To: to}
		}
		for _, t := range times {
			i := sort.SearchFloat64s(histEdges, t)
			if i == len(histEdges) || histEdges[i] > t {
				i--
			}
			s.Histogram[max(i, 0)].Count++
		}
	}
	return s
}

func percentile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(q*float64(len(sorted)))) - 1
	return sorted[min(max(i, 0), len(sorted)-1)]
}

func hex2(b byte) string {
	const digits = "0123456789ABCDEF"
	return string([]byte{digits[b>>4], digits[b&15]})
}

// ConnectionStats is the statistics view of one connection.
type ConnectionStats struct {
	ID    string      `json:"id"`
	Name  string      `json:"name"`
	Stats Stats       `json:"stats"`
	Polls []PollStats `json:"polls"`
}

// PollStats is the statistics of one poll window.
type PollStats struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Stats Stats  `json:"stats"`
}

// Statistics returns every connection with its poll windows. detailID
// names the connection whose histogram and recent samples are included.
func (e *Engine) Statistics(detailID string) []ConnectionStats {
	e.mu.Lock()
	conns := make([]*connection, 0, len(e.conns))
	for _, c := range e.conns {
		conns = append(conns, c)
	}
	polls := make([]*poll, 0, len(e.polls))
	for _, p := range e.polls {
		polls = append(polls, p)
	}
	e.mu.Unlock()
	sort.Slice(conns, func(i, j int) bool { return idLess(conns[i].cfg.ID, conns[j].cfg.ID) })
	sort.Slice(polls, func(i, j int) bool { return idLess(polls[i].cfg.ID, polls[j].cfg.ID) })
	out := make([]ConnectionStats, 0, len(conns))
	for _, c := range conns {
		cs := ConnectionStats{ID: c.cfg.ID, Name: c.cfg.Name, Stats: c.stats.snapshot(c.cfg.ID == detailID), Polls: []PollStats{}}
		for _, p := range polls {
			p.mu.Lock()
			cfg := p.cfg
			p.mu.Unlock()
			if cfg.ConnectionID == c.cfg.ID {
				cs.Polls = append(cs.Polls, PollStats{ID: cfg.ID, Name: cfg.Name, Stats: p.stats.snapshot(false)})
			}
		}
		out = append(out, cs)
	}
	return out
}

// ResetStatistics clears the counters of one connection and its poll
// windows ("" = all connections).
func (e *Engine) ResetStatistics(connID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, c := range e.conns {
		if connID == "" || id == connID {
			c.stats.reset()
		}
	}
	for _, p := range e.polls {
		p.mu.Lock()
		owner := p.cfg.ConnectionID
		p.mu.Unlock()
		if connID == "" || owner == connID {
			p.stats.reset()
		}
	}
}
