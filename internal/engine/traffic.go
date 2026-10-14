package engine

import (
	"sync"
	"time"

	"modbusstudio/internal/modbus"
)

// TrafficEntry is one raw frame shown in the communication traffic window.
type TrafficEntry struct {
	Seq    uint64 `json:"seq"`
	Time   int64  `json:"time"` // unix ms
	Source string `json:"source"`
	Dir    string `json:"dir"`
	Hex    string `json:"hex"`
	Len    int    `json:"len"`
}

// trafficLog batches frames and flushes them every 100 ms so a fast poll
// cannot flood the UI with events.
type trafficLog struct {
	mu      sync.Mutex
	seq     uint64
	pending []TrafficEntry
	paused  bool
	flush   func([]TrafficEntry)
	stop    chan struct{}
	done    chan struct{}
}

const maxPending = 2000

func newTrafficLog(flush func([]TrafficEntry)) *trafficLog {
	t := &trafficLog{flush: flush, stop: make(chan struct{}), done: make(chan struct{})}
	go t.run()
	return t
}

func (t *trafficLog) run() {
	defer close(t.done)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-t.stop:
			return
		case <-tick.C:
			t.mu.Lock()
			batch := t.pending
			t.pending = nil
			t.mu.Unlock()
			if len(batch) > 0 {
				t.flush(batch)
			}
		}
	}
}

func (t *trafficLog) close() {
	select {
	case <-t.stop:
	default:
		close(t.stop)
		<-t.done
	}
}

func (t *trafficLog) observer(source string) modbus.TrafficFunc {
	return func(dir modbus.Direction, frame []byte) {
		now := time.Now().UnixMilli()
		hex := modbus.HexString(frame)
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.paused {
			return
		}
		t.seq++
		if len(t.pending) >= maxPending {
			t.pending = t.pending[1:]
		}
		t.pending = append(t.pending, TrafficEntry{Seq: t.seq, Time: now, Source: source, Dir: string(dir), Hex: hex, Len: len(frame)})
	}
}

// SetTrafficPaused stops or resumes capturing traffic.
func (e *Engine) SetTrafficPaused(paused bool) {
	e.traffic.mu.Lock()
	e.traffic.paused = paused
	e.traffic.mu.Unlock()
}
