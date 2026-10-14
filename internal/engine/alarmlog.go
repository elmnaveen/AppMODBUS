package engine

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// AlarmLog keeps the alarm history as one JSON-lines file per day (UTC).
type AlarmLog struct {
	mu  sync.Mutex
	dir string
}

// NewAlarmLog stores history in dir.
func NewAlarmLog(dir string) *AlarmLog { return &AlarmLog{dir: dir} }

func (l *AlarmLog) file(t time.Time) string {
	return filepath.Join(l.dir, "alarms-"+t.UTC().Format("2006-01-02")+".jsonl")
}

// Append writes one event.
func (l *AlarmLog) Append(ev AlarmEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.file(time.UnixMilli(ev.Time)), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, _ := json.Marshal(ev)
	_, err = f.Write(append(b, '\n'))
	return err
}

// Query returns events with from <= t <= to, newest first, at most limit
// (0 = all).
func (l *AlarmLog) Query(from, to int64, limit int) ([]AlarmEvent, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	files, _ := filepath.Glob(filepath.Join(l.dir, "alarms-*.jsonl"))
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	firstDay := time.UnixMilli(from).UTC().Format("2006-01-02")
	lastDay := time.UnixMilli(to).UTC().Format("2006-01-02")
	out := []AlarmEvent{}
	for _, name := range files {
		day := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(name), "alarms-"), ".jsonl")
		if day < firstDay || day > lastDay {
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			return out, err
		}
		var dayEvents []AlarmEvent
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			var ev AlarmEvent
			if json.Unmarshal(sc.Bytes(), &ev) == nil && ev.Time >= from && ev.Time <= to {
				dayEvents = append(dayEvents, ev)
			}
		}
		f.Close()
		for i := len(dayEvents) - 1; i >= 0; i-- {
			out = append(out, dayEvents[i])
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

// Prune deletes days older than keepDays.
func (l *AlarmLog) Prune(keepDays int) {
	if keepDays <= 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	cut := time.Now().UTC().AddDate(0, 0, -keepDays).Format("2006-01-02")
	files, _ := filepath.Glob(filepath.Join(l.dir, "alarms-*.jsonl"))
	for _, name := range files {
		day := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(name), "alarms-"), ".jsonl")
		if day < cut {
			os.Remove(name)
		}
	}
}
