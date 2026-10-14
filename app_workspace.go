package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"appmodbus/internal/modbus"
	"appmodbus/internal/settings"
	"appmodbus/internal/version"
	"appmodbus/internal/workspace"
)

const eventWorkspace = "workspace:state"

// WorkspaceState is shown in the toolbar and title bar.
type WorkspaceState struct {
	Path   string   `json:"path"`
	Name   string   `json:"name"`
	Dirty  bool     `json:"dirty"`
	Recent []string `json:"recent"`
}

// MissingPort is a saved serial port that does not exist on this machine.
type MissingPort struct {
	ConnectionID string `json:"connectionId"` // "" for the simulator
	Name         string `json:"name"`
	Port         string `json:"port"`
}

// LoadResult is returned after opening a workspace.
type LoadResult struct {
	State        WorkspaceState   `json:"state"`
	Layout       workspace.Layout `json:"layout"`
	Charts       any              `json:"charts"`
	Warnings     []string         `json:"warnings"`
	MissingPorts []MissingPort    `json:"missingPorts"`
	Ports        []string         `json:"ports"` // serial ports available now
}

type workspaceManager struct {
	mu       sync.Mutex
	path     string
	dirty    bool
	layout   workspace.Layout
	charts   []byte
	sessTime *time.Timer
}

func (a *App) prefs() *settings.Store { return a.updates.settings }

func sessionPath() string { return filepath.Join(settings.DefaultDir(), "session.json") }

// markDirty records an unsaved change and schedules a session autosave.
func (a *App) markDirty() {
	a.ws.mu.Lock()
	was := a.ws.dirty
	a.ws.dirty = true
	a.ws.mu.Unlock()
	if !was {
		a.emitWorkspace()
	}
	a.scheduleSession()
}

// scheduleSession autosaves the session shortly after the last change.
func (a *App) scheduleSession() {
	a.ws.mu.Lock()
	defer a.ws.mu.Unlock()
	if a.ws.sessTime != nil {
		a.ws.sessTime.Stop()
	}
	a.ws.sessTime = time.AfterFunc(1500*time.Millisecond, func() { _ = a.saveSession() })
}

// snapshot captures the current workspace contents.
func (a *App) snapshot() workspace.File {
	f := workspace.New()
	f.App = version.Version
	f.Connections, f.Polls = a.engine.Definitions()
	a.ws.mu.Lock()
	f.Layout = a.ws.layout
	if len(a.ws.charts) > 0 {
		f.Charts = append([]byte(nil), a.ws.charts...)
	}
	a.ws.mu.Unlock()
	st := a.engine.Sim.Status()
	cfg := st.Config
	if !st.Running {
		cfg = a.simConfig
	}
	f.Simulator = &workspace.Simulator{Config: cfg, Running: st.Running, Data: a.engine.Sim.Store.Snapshot()}
	return f
}

func (a *App) saveSession() error {
	a.ws.mu.Lock()
	path, dirty := a.ws.path, a.ws.dirty
	a.ws.mu.Unlock()
	return workspace.WriteSession(sessionPath(), workspace.Session{Path: path, Dirty: dirty, File: a.snapshot()})
}

func (a *App) workspaceState() WorkspaceState {
	a.ws.mu.Lock()
	path, dirty := a.ws.path, a.ws.dirty
	a.ws.mu.Unlock()
	recent := []string{}
	for _, p := range a.prefs().Get().RecentWorkspaces {
		if _, err := os.Stat(p); err == nil {
			recent = append(recent, p)
		}
	}
	return WorkspaceState{Path: path, Name: workspace.DisplayName(path), Dirty: dirty, Recent: recent}
}

func (a *App) emitWorkspace() {
	st := a.workspaceState()
	title := st.Name + " — AppMODBUS " + version.Version
	if st.Dirty {
		title = "● " + title
	}
	runtime.WindowSetTitle(a.ctx, title)
	runtime.EventsEmit(a.ctx, eventWorkspace, st)
}

// apply replaces the running state with a workspace document.
func (a *App) apply(f workspace.File, path string, dirty bool) LoadResult {
	a.engine.Sim.Stop()
	warnings := a.engine.Load(f.Connections, f.Polls)
	if f.Simulator != nil {
		a.engine.Sim.Store.Restore(f.Simulator.Data)
		a.simConfig = f.Simulator.Config
	} else {
		a.engine.Sim.Store.Restore(nil)
	}

	ports, _ := modbus.ListSerialPorts()
	if ports == nil {
		ports = []string{}
	}
	var missing []MissingPort
	for _, c := range f.Connections {
		if c.Mode == modbus.ModeRTU && c.SerialPort != "" && !slices.Contains(ports, c.SerialPort) {
			missing = append(missing, MissingPort{ConnectionID: c.ID, Name: c.Name, Port: c.SerialPort})
		}
	}
	if s := f.Simulator; s != nil && s.Running {
		if s.Config.Mode == modbus.ModeRTU && !slices.Contains(ports, s.Config.SerialPort) {
			missing = append(missing, MissingPort{Name: "Simulator", Port: s.Config.SerialPort})
		} else if err := a.engine.Sim.Start(s.Config); err != nil {
			warnings = append(warnings, "simulator: "+err.Error())
		}
	}

	a.ws.mu.Lock()
	a.ws.path, a.ws.dirty, a.ws.layout = path, dirty, f.Layout
	a.ws.charts = append([]byte(nil), f.Charts...)
	a.ws.mu.Unlock()
	a.emitWorkspace()

	var charts any = []any{}
	if len(f.Charts) > 0 {
		charts = f.Charts
	}
	if warnings == nil {
		warnings = []string{}
	}
	if missing == nil {
		missing = []MissingPort{}
	}
	return LoadResult{State: a.workspaceState(), Layout: f.Layout, Charts: charts, Warnings: warnings, MissingPorts: missing, Ports: ports}
}

// RestoreSession reopens the last session (polls stopped). Called by the
// UI once on start-up.
func (a *App) RestoreSession() LoadResult {
	s, err := workspace.ReadSession(sessionPath())
	if err != nil {
		return a.apply(workspace.New(), "", false)
	}
	return a.apply(s.File, s.Path, s.Dirty)
}

// WorkspaceState returns the current file name and unsaved-changes flag.
func (a *App) WorkspaceState() WorkspaceState { return a.workspaceState() }

// NewWorkspace clears everything. The UI asks about unsaved changes first.
func (a *App) NewWorkspace() LoadResult {
	r := a.apply(workspace.New(), "", false)
	_ = a.saveSession()
	return r
}

// OpenWorkspace shows a file dialog and opens the chosen workspace.
// It returns nil when the dialog is cancelled.
func (a *App) OpenWorkspace() (*LoadResult, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Open workspace",
		Filters: []runtime.FileFilter{{DisplayName: "AppMODBUS workspace (*.mbw)", Pattern: "*.mbw"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	return a.OpenWorkspacePath(path)
}

// OpenWorkspacePath opens a workspace file, e.g. from the recent list.
func (a *App) OpenWorkspacePath(path string) (*LoadResult, error) {
	f, err := workspace.Read(path)
	if err != nil {
		return nil, err
	}
	_, _ = a.prefs().Update(func(s *settings.Settings) { s.AddRecent(path) })
	r := a.apply(f, path, false)
	_ = a.saveSession()
	return &r, nil
}

// SaveWorkspace saves to the current file, or asks for one.
// It returns the saved path ("" if cancelled).
func (a *App) SaveWorkspace() (string, error) {
	a.ws.mu.Lock()
	path := a.ws.path
	a.ws.mu.Unlock()
	if path == "" {
		return a.SaveWorkspaceAs()
	}
	return path, a.saveTo(path)
}

// SaveWorkspaceAs asks for a file name and saves there.
func (a *App) SaveWorkspaceAs() (string, error) {
	st := a.workspaceState()
	dir := ""
	if st.Path != "" {
		dir = filepath.Dir(st.Path)
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:            "Save workspace",
		DefaultDirectory: dir,
		DefaultFilename:  st.Name + workspace.Extension,
		Filters:          []runtime.FileFilter{{DisplayName: "AppMODBUS workspace (*.mbw)", Pattern: "*.mbw"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	path = workspace.WithExtension(path)
	return path, a.saveTo(path)
}

func (a *App) saveTo(path string) error {
	if path == "" {
		return errors.New("no file name")
	}
	if err := workspace.Write(path, a.snapshot()); err != nil {
		return err
	}
	a.ws.mu.Lock()
	a.ws.path, a.ws.dirty = path, false
	a.ws.mu.Unlock()
	_, _ = a.prefs().Update(func(s *settings.Settings) { s.AddRecent(path) })
	a.emitWorkspace()
	return a.saveSession()
}

// SetLayout records the UI layout. Layout changes are saved with the
// workspace but do not by themselves count as unsaved changes.
func (a *App) SetLayout(l workspace.Layout) {
	a.ws.mu.Lock()
	a.ws.layout = l
	a.ws.mu.Unlock()
	a.scheduleSession()
}

// SetCharts stores the chart definitions (owned by the UI).
func (a *App) SetCharts(charts any) error {
	b, err := jsonMarshal(charts)
	if err != nil {
		return err
	}
	a.ws.mu.Lock()
	same := string(a.ws.charts) == string(b)
	a.ws.charts = b
	a.ws.mu.Unlock()
	if !same {
		a.markDirty()
	}
	return nil
}

// RemapSerialPort points a connection (or, with an empty id, the
// simulator) at a different serial port after opening a workspace.
func (a *App) RemapSerialPort(connectionID, port string) error {
	if connectionID == "" {
		a.simConfig.SerialPort = port
		a.markDirty()
		return a.engine.Sim.Start(a.simConfig)
	}
	for _, c := range a.engine.Connections() {
		if c.ID == connectionID {
			cfg := c.ConnectionConfig
			cfg.SerialPort = port
			_, err := a.engine.SaveConnection(cfg)
			return err
		}
	}
	return errors.New("unknown connection")
}

func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
