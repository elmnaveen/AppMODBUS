// Package applog is the application log: what the app did and what went
// wrong (connections, polling, writes, alarms, workspace, updates…), as
// opposed to the traffic log of raw Modbus frames.
//
// Entries are kept in memory for the Application log window and appended to
// one text file per day in the log folder; old files are deleted after
// KeepDays.
package applog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Levels.
const (
	Info  = "info"
	Warn  = "warn"
	Error = "error"
)

const (
	// MaxEntries is how many entries stay in memory.
	MaxEntries = 5000
	// KeepDays is how long daily log files are kept.
	KeepDays   = 14
	filePrefix = "modbus-studio-"
	fileSuffix = ".log"
)

// Entry is one log line.
type Entry struct {
	ID      int64  `json:"id"`
	Time    int64  `json:"time"` // unix ms
	Level   string `json:"level"`
	Source  string `json:"source"` // app, connection, poll, write, alarm, simulator, workspace, update, recording, scan, test, ui
	Message string `json:"message"`
}

// Log is safe for concurrent use.
type Log struct {
	mu     sync.Mutex
	dir    string
	buf    []Entry // ring of at most MaxEntries
	start  int     // index of the oldest entry in buf
	nextID int64
	day    string
	file   *os.File
	fileOK bool
	emit   func(Entry)
	now    func() time.Time
}

// Open creates a log writing daily files to dir (created if needed). emit,
// if set, is called for every new entry. File errors do not stop logging to
// memory.
func Open(dir string, emit func(Entry)) *Log {
	l := &Log{dir: dir, emit: emit, now: time.Now, nextID: 1}
	l.fileOK = os.MkdirAll(dir, 0o755) == nil
	if l.fileOK {
		l.prune()
	}
	return l
}

// Dir is the folder holding the daily log files.
func (l *Log) Dir() string { return l.dir }

// Add records an entry. Unknown levels are treated as info.
func (l *Log) Add(level, source, format string, args ...any) Entry {
	if level != Warn && level != Error {
		level = Info
	}
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	msg = strings.TrimSpace(msg)
	now := l.now()

	l.mu.Lock()
	e := Entry{ID: l.nextID, Time: now.UnixMilli(), Level: level, Source: source, Message: msg}
	l.nextID++
	if len(l.buf) < MaxEntries {
		l.buf = append(l.buf, e)
	} else {
		l.buf[l.start] = e
		l.start = (l.start + 1) % MaxEntries
	}
	l.writeFile(now, e)
	emit := l.emit
	l.mu.Unlock()

	if emit != nil {
		emit(e)
	}
	return e
}

// Infof, Warnf and Errorf are shorthands for Add.
func (l *Log) Infof(source, format string, args ...any)  { l.Add(Info, source, format, args...) }
func (l *Log) Warnf(source, format string, args ...any)  { l.Add(Warn, source, format, args...) }
func (l *Log) Errorf(source, format string, args ...any) { l.Add(Error, source, format, args...) }

// Recent returns up to limit entries, oldest first (limit <= 0 means all).
func (l *Log) Recent(limit int) []Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(l.buf)
	if limit <= 0 || limit > n {
		limit = n
	}
	out := make([]Entry, 0, limit)
	for i := n - limit; i < n; i++ {
		out = append(out, l.buf[(l.start+i)%n])
	}
	return out
}

// Clear empties the in-memory log. Files on disk are kept.
func (l *Log) Clear() {
	l.mu.Lock()
	l.buf, l.start = nil, 0
	l.mu.Unlock()
}

// Close flushes and closes the current file.
func (l *Log) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		_ = l.file.Close()
		l.file = nil
	}
}

// Format renders entries as text lines, the same layout as the files.
func Format(entries []Entry) string {
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(line(time.UnixMilli(e.Time), e))
	}
	return b.String()
}

func line(t time.Time, e Entry) string {
	return fmt.Sprintf("%s %-5s [%s] %s\n", t.Format("2006-01-02 15:04:05.000"), strings.ToUpper(e.Level), e.Source, e.Message)
}

// writeFile appends to today's file, switching files at midnight. Called with mu held.
func (l *Log) writeFile(now time.Time, e Entry) {
	if !l.fileOK {
		return
	}
	day := now.Format("2006-01-02")
	if l.file == nil || day != l.day {
		if l.file != nil {
			_ = l.file.Close()
			l.file = nil
			l.prune()
		}
		f, err := os.OpenFile(filepath.Join(l.dir, filePrefix+day+fileSuffix), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			l.fileOK = false
			return
		}
		l.file, l.day = f, day
	}
	_, _ = l.file.WriteString(line(now, e))
}

// prune deletes daily files older than KeepDays.
func (l *Log) prune() {
	names, err := filepath.Glob(filepath.Join(l.dir, filePrefix+"*"+fileSuffix))
	if err != nil {
		return
	}
	sort.Strings(names)
	cutoff := l.now().AddDate(0, 0, -KeepDays).Format("2006-01-02")
	for _, p := range names {
		day := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(p), filePrefix), fileSuffix)
		if day < cutoff {
			_ = os.Remove(p)
		}
	}
}
