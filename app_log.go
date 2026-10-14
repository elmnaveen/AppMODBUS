package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"appmodbus/internal/applog"
	"appmodbus/internal/engine"
	"appmodbus/internal/settings"
	"appmodbus/internal/simulator"
	"appmodbus/internal/version"
)

const eventAppLog = "applog"

// Log sources shown in the Application log window.
const (
	srcApp        = "app"
	srcConnection = "connection"
	srcPoll       = "poll"
	srcWrite      = "write"
	srcAlarm      = "alarm"
	srcSimulator  = "simulator"
	srcWorkspace  = "workspace"
	srcUpdate     = "update"
	srcRecording  = "recording"
	srcScan       = "scan"
	srcTest       = "test"
	srcUI         = "ui"
)

// pollErrorRepeat limits how often a poll that keeps failing with
// different errors is logged; going from OK to failing and back is always logged.
const pollErrorRepeat = 10 * time.Second

// logWatcher turns engine events into log entries by remembering the last
// state of each connection, poll window, alarm, test run and scan.
type logWatcher struct {
	mu     sync.Mutex
	conns  map[string]engine.ConnectionInfo
	polls  map[string]pollMark
	alarms map[string]engine.Alarm
	seqRun map[string]bool
	scan   map[string]bool
	update string
}

type pollMark struct {
	running bool
	err     string
	errAt   time.Time
}

func newLogWatcher() *logWatcher {
	return &logWatcher{
		conns:  map[string]engine.ConnectionInfo{},
		polls:  map[string]pollMark{},
		alarms: map[string]engine.Alarm{},
		seqRun: map[string]bool{},
		scan:   map[string]bool{},
	}
}

func (a *App) startLog() {
	a.log = applog.Open(filepath.Join(settings.DefaultDir(), "logs"), func(e applog.Entry) {
		runtime.EventsEmit(a.ctx, eventAppLog, e)
	})
	a.logw = newLogWatcher()
	a.log.Infof(srcApp, "Modbus Studio %s started (%s/%s)", version.Version, goruntime.GOOS, goruntime.GOARCH)
}

// observe is called for every engine event before it goes to the UI.
func (a *App) observe(event string, data any) {
	if a.log == nil {
		return
	}
	w := a.logw
	w.mu.Lock()
	defer w.mu.Unlock()
	switch v := data.(type) {
	case []engine.ConnectionInfo:
		a.observeConnections(v)
	case engine.Poll:
		a.observePoll(v)
	case []engine.Alarm:
		a.observeAlarms(v)
	case engine.SequenceRun:
		if v.Running && !w.seqRun[v.SequenceID] {
			a.log.Infof(srcTest, "Test %q started on %s", v.Name, v.Connection)
		} else if !v.Running && w.seqRun[v.SequenceID] {
			level := applog.Info
			if v.Verdict != "pass" {
				level = applog.Warn
			}
			msg := fmt.Sprintf("Test %q finished: %s (%d passed, %d failed)", v.Name, strings.ToUpper(v.Verdict), v.Passed, v.Failed)
			if v.Error != "" {
				msg += ": " + v.Error
			}
			a.log.Add(level, srcTest, "%s", msg)
		}
		w.seqRun[v.SequenceID] = v.Running
	case engine.ScanState:
		if v.Running && !w.scan[v.Kind] {
			a.log.Infof(srcScan, "%s scan started", scanName(v.Kind))
		} else if !v.Running && w.scan[v.Kind] {
			if v.Error != "" {
				a.log.Errorf(srcScan, "%s scan failed: %s", scanName(v.Kind), v.Error)
			} else {
				a.log.Infof(srcScan, "%s scan finished: %s", scanName(v.Kind), scanSummary(v))
			}
		}
		w.scan[v.Kind] = v.Running
	}
}

func (a *App) observeConnections(list []engine.ConnectionInfo) {
	w := a.logw
	seen := map[string]bool{}
	for _, c := range list {
		seen[c.ID] = true
		old, known := w.conns[c.ID]
		w.conns[c.ID] = c
		if !known {
			if c.Connected {
				a.log.Infof(srcConnection, "Connected to %s (%s)", c.Name, c.Summary)
			}
			continue
		}
		switch {
		case c.Connected && !old.Connected:
			a.log.Infof(srcConnection, "Connected to %s (%s)", c.Name, c.Summary)
		case !c.Connected && old.Connected && c.Error == "":
			a.log.Infof(srcConnection, "Disconnected from %s", c.Name)
		}
		if c.Error != "" && c.Error != old.Error {
			a.log.Errorf(srcConnection, "%s (%s): %s", c.Name, c.Summary, c.Error)
		}
	}
	for id := range w.conns {
		if !seen[id] {
			delete(w.conns, id)
		}
	}
}

func (a *App) observePoll(p engine.Poll) {
	w := a.logw
	id, name, st := p.Config.ID, p.Config.Name, p.State
	old, known := w.polls[id]
	m := old
	m.running = st.Running
	if known && st.Running != old.running {
		if st.Running {
			a.log.Infof(srcPoll, "%s: polling started (slave %d, F%02d, %d × %d ms)", name, p.Config.SlaveID, p.Config.Function, p.Config.Quantity, p.Config.ScanRateMs)
		} else {
			a.log.Infof(srcPoll, "%s: polling stopped", name)
		}
	}
	now := time.Now()
	switch {
	case st.LastError != "" && old.err == "":
		a.log.Warnf(srcPoll, "%s: %s", name, st.LastError)
		m.err, m.errAt = st.LastError, now
	case st.LastError != "" && st.LastError != old.err && now.Sub(old.errAt) >= pollErrorRepeat:
		a.log.Warnf(srcPoll, "%s: %s", name, st.LastError)
		m.err, m.errAt = st.LastError, now
	case st.LastError == "" && old.err != "":
		a.log.Infof(srcPoll, "%s: communication OK again", name)
		m.err = ""
	}
	w.polls[id] = m
}

func (a *App) observeAlarms(list []engine.Alarm) {
	w := a.logw
	seen := map[string]bool{}
	for _, al := range list {
		seen[al.ID] = true
		old, known := w.alarms[al.ID]
		w.alarms[al.ID] = al
		what := al.Window
		if al.Name != "" {
			what += " · " + al.Name
		}
		switch {
		case al.Active && (!known || !old.Active):
			level := applog.Warn
			if al.Severity == "critical" {
				level = applog.Error
			}
			a.log.Add(level, srcAlarm, "%s: %s", what, al.Message)
		case !al.Active && known && old.Active:
			a.log.Infof(srcAlarm, "%s: cleared", what)
		}
		if al.Acked && known && !old.Acked {
			a.log.Infof(srcAlarm, "%s: acknowledged", what)
		}
	}
	for id := range w.alarms {
		if !seen[id] {
			delete(w.alarms, id)
		}
	}
}

// observeUpdate logs update checks and installs when their status changes.
func (a *App) observeUpdate(st UpdateState) {
	if a.log == nil {
		return
	}
	w := a.logw
	w.mu.Lock()
	defer w.mu.Unlock()
	if st.Status == w.update {
		return
	}
	w.update = st.Status
	switch st.Status {
	case "available":
		if st.Release != nil {
			a.log.Infof(srcUpdate, "Version %s is available", st.Release.Version)
		}
	case "uptodate":
		a.log.Infof(srcUpdate, "Up to date (%s)", version.Version)
	case "downloading":
		a.log.Infof(srcUpdate, "Downloading update")
	case "installing":
		a.log.Infof(srcUpdate, "Installing update")
	case "error":
		a.log.Warnf(srcUpdate, "Update check failed: %s", st.Error)
	}
}

func scanName(kind string) string {
	switch kind {
	case "ids":
		return "Slave ID"
	case "map":
		return "Register map"
	case "network":
		return "Network"
	}
	return kind
}

func scanSummary(s engine.ScanState) string {
	switch s.Kind {
	case "ids":
		ok := 0
		for _, r := range s.IDs {
			if r.Status == "ok" || r.Status == "exception" {
				ok++
			}
		}
		return fmt.Sprintf("%d slave(s) answered", ok)
	case "map":
		return fmt.Sprintf("%d range(s) found", len(s.Ranges))
	case "network":
		return fmt.Sprintf("%d host(s) found", len(s.Hosts))
	}
	return "done"
}

// logResult logs a user action: info when it worked, error when it failed.
func (a *App) logResult(source string, err error, format string, args ...any) {
	if a.log == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if err != nil {
		a.log.Errorf(source, "%s failed: %v", msg, err)
	} else {
		a.log.Infof(source, "%s", msg)
	}
}

func (a *App) connName(id string) string {
	for _, c := range a.engine.Connections() {
		if c.ID == id {
			return c.Name
		}
	}
	return id
}

func (a *App) pollName(id string) string {
	for _, p := range a.engine.Polls() {
		if p.Config.ID == id {
			return p.Config.Name
		}
	}
	return id
}

// valuesText shows the first values of a write, e.g. "[1, 2, 3, … 10 values]".
func valuesText(v []int) string {
	const show = 8
	parts := make([]string, 0, show)
	for i, x := range v {
		if i == show {
			break
		}
		parts = append(parts, fmt.Sprint(x))
	}
	s := strings.Join(parts, ", ")
	if len(v) > show {
		s += fmt.Sprintf(", … %d values", len(v))
	}
	return "[" + s + "]"
}

func simText(c simulator.Config) string {
	mode := strings.ToUpper(string(c.Mode))
	if c.Mode.IsSerial() {
		return fmt.Sprintf("%s on %s, %d baud, slave %d", mode, c.SerialPort, c.BaudRate, c.SlaveID)
	}
	return fmt.Sprintf("%s %s:%d, slave %d", mode, c.Host, c.Port, c.SlaveID)
}

// ---- bindings for the Application log window ----

// AppLog returns the newest entries, oldest first.
func (a *App) AppLog(limit int) []applog.Entry { return a.log.Recent(limit) }

// ClearAppLog empties the log window. The daily files are kept.
func (a *App) ClearAppLog() { a.log.Clear() }

// AppLogFolder is where the daily log files are written.
func (a *App) AppLogFolder() string { return a.log.Dir() }

// LogUI records a message shown by the UI, e.g. an error toast. It is
// skipped when the backend logged the same problem moments ago (a failed
// write is already in the log as a write error).
func (a *App) LogUI(level, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	cutoff := time.Now().Add(-3 * time.Second).UnixMilli()
	for _, e := range a.log.Recent(20) {
		if e.Time >= cutoff && e.Source != srcUI && strings.Contains(e.Message, message) {
			return
		}
	}
	a.log.Add(level, srcUI, "%s", message)
}

// OpenAppLogFolder opens the log folder in the file manager.
func (a *App) OpenAppLogFolder() error {
	dir := a.log.Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	// explorer exits with status 1 even when it opened the folder.
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// ExportAppLog saves the entries in memory to a text file the user chooses.
func (a *App) ExportAppLog() (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export application log",
		DefaultFilename: "modbus-studio-" + time.Now().Format("2006-01-02_1504") + ".log",
		Filters:         []runtime.FileFilter{{DisplayName: "Log files", Pattern: "*.log;*.txt"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, []byte(applog.Format(a.log.Recent(0))), 0o644)
}
