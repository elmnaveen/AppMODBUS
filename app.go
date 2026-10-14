package main

import (
	"context"
	"encoding/base64"
	"os"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"appmodbus/internal/datafmt"
	"appmodbus/internal/engine"
	"appmodbus/internal/modbus"
	"appmodbus/internal/recorder"
	"appmodbus/internal/simulator"
)

// App exposes the engine to the frontend. Every exported method becomes a
// JavaScript binding under window.go.main.App.
type App struct {
	ctx     context.Context
	engine  *engine.Engine
	updates *updater
	ws      workspaceManager

	// simConfig is the last simulator configuration, saved with the
	// workspace even while the simulator is stopped.
	simConfig simulator.Config

	rec    *recorder.Recorder
	recErr atomic.Value // string
}

// NewApp creates the application.
func NewApp() *App { return &App{updates: newUpdater()} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.engine = engine.New(func(event string, data any) { runtime.EventsEmit(ctx, event, data) })
	a.engine.OnChange = a.markDirty
	a.startRecorder()
	a.startUpdater()
}

func (a *App) shutdown(context.Context) {
	_ = a.saveSession()
	a.engine.Shutdown()
	if a.rec != nil {
		a.rec.Close()
	}
}

// ---- metadata ----

// Formats lists the display formats.
func (a *App) Formats() []datafmt.Info { return datafmt.All() }

// SerialPorts lists the serial ports on this machine.
func (a *App) SerialPorts() ([]string, error) {
	ports, err := modbus.ListSerialPorts()
	if ports == nil {
		ports = []string{}
	}
	return ports, err
}

// ---- connections ----

func (a *App) Connections() []engine.ConnectionInfo { return a.engine.Connections() }

func (a *App) SaveConnection(c engine.ConnectionConfig) (engine.ConnectionInfo, error) {
	return a.engine.SaveConnection(c)
}

func (a *App) DeleteConnection(id string) { a.engine.DeleteConnection(id) }
func (a *App) Connect(id string) error    { return a.engine.Connect(id) }
func (a *App) Disconnect(id string)       { a.engine.Disconnect(id) }

// ---- polls ----

func (a *App) Polls() []engine.Poll { return a.engine.Polls() }

func (a *App) CreatePoll(c engine.PollConfig) (engine.Poll, error) { return a.engine.CreatePoll(c) }
func (a *App) UpdatePoll(c engine.PollConfig) (engine.Poll, error) { return a.engine.UpdatePoll(c) }
func (a *App) DeletePoll(id string)                                { a.engine.DeletePoll(id) }
func (a *App) StartPoll(id string) error                           { return a.engine.StartPoll(id) }
func (a *App) StopPoll(id string)                                  { a.engine.StopPoll(id) }
func (a *App) StartAll() error                                     { return a.engine.StartAll() }
func (a *App) StopAll()                                            { a.engine.StopAll() }
func (a *App) ReadOnce(id string) error                            { return a.engine.ReadOnce(id) }

func (a *App) SetCellFormat(id string, address int, format string) error {
	return a.engine.SetCellFormat(id, address, datafmt.Format(format))
}

// ---- writes ----

func (a *App) Write(w engine.WriteRequest) error { return a.engine.Write(w) }

func (a *App) WriteCell(pollID string, address int, value string, forceMultiple bool) error {
	return a.engine.WriteCell(pollID, address, value, forceMultiple)
}

// ParseValue encodes a value in a format, for the multi-register write dialog.
func (a *App) ParseValue(format, value string) ([]int, error) {
	regs, err := datafmt.ParseValue(datafmt.Format(format), value)
	out := make([]int, len(regs))
	for i, r := range regs {
		out[i] = int(r)
	}
	return out, err
}

// ---- traffic ----

func (a *App) SetTrafficPaused(paused bool) { a.engine.SetTrafficPaused(paused) }

// SaveTrafficLog asks for a file name and writes the given text to it.
func (a *App) SaveTrafficLog(text string) (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save communication traffic",
		DefaultFilename: "traffic.txt",
		Filters:         []runtime.FileFilter{{DisplayName: "Text files", Pattern: "*.txt;*.log"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, []byte(text), 0o644)
}

// SaveExportFile asks for a file name and writes data to it. data is
// base64 when binary is set (e.g. a PNG image), otherwise text.
func (a *App) SaveExportFile(title, defaultName, filterName, pattern, data string, binary bool) (string, error) {
	b := []byte(data)
	if binary {
		var err error
		if b, err = base64.StdEncoding.DecodeString(data); err != nil {
			return "", err
		}
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultName,
		Filters:         []runtime.FileFilter{{DisplayName: filterName, Pattern: pattern}},
	})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, b, 0o644)
}

// ---- simulator ----

func (a *App) SimStart(c simulator.Config) error {
	if err := a.engine.Sim.Start(c); err != nil {
		return err
	}
	a.simConfig = c
	a.markDirty()
	return nil
}

func (a *App) SimStop() {
	a.engine.Sim.Stop()
	a.markDirty()
}

// SimConfig returns the configuration to show in the simulator panel.
func (a *App) SimConfig() simulator.Config {
	if st := a.engine.Sim.Status(); st.Running {
		return st.Config
	}
	return a.simConfig
}
func (a *App) SimStatus() simulator.Status { return a.engine.Sim.Status() }

func (a *App) SimRead(table string, address, quantity int) []int {
	return a.engine.Sim.Store.Read(simulator.Table(table), address, quantity)
}

func (a *App) SimWrite(table string, address int, values []int) {
	a.engine.Sim.Store.Write(simulator.Table(table), address, values)
	a.markDirty()
}
