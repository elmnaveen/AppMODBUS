package main

import (
	"context"
	"encoding/base64"
	"os"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"modbusstudio/internal/applog"
	"modbusstudio/internal/datafmt"
	"modbusstudio/internal/engine"
	"modbusstudio/internal/legacy"
	"modbusstudio/internal/modbus"
	"modbusstudio/internal/recorder"
	"modbusstudio/internal/simulator"
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

	alarmLog *engine.AlarmLog

	log  *applog.Log
	logw *logWatcher

	// migrated describes data carried over from AppMODBUS on this start.
	migrated []legacy.Note
}

// NewApp creates the application.
func NewApp() *App { return &App{updates: newUpdater()} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.startLog()
	a.engine = engine.New(func(event string, data any) {
		a.observe(event, data)
		runtime.EventsEmit(ctx, event, data)
	})
	a.engine.OnChange = a.markDirty
	a.startRecorder()
	a.startAlarmLog()
	a.startUpdater()
}

func (a *App) shutdown(context.Context) {
	_ = a.saveSession()
	a.engine.Shutdown()
	if a.rec != nil {
		a.rec.Close()
	}
	a.log.Infof(srcApp, "Modbus Studio stopped")
	a.log.Close()
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
	info, err := a.engine.SaveConnection(c)
	a.logResult(srcConnection, err, "Connection %q saved (%s)", c.Name, info.Summary)
	return info, err
}

func (a *App) DeleteConnection(id string) {
	name := id
	for _, c := range a.engine.Connections() {
		if c.ID == id {
			name = c.Name
		}
	}
	a.engine.DeleteConnection(id)
	a.log.Infof(srcConnection, "Connection %q deleted", name)
}
func (a *App) Connect(id string) error { return a.engine.Connect(id) }
func (a *App) Disconnect(id string)    { a.engine.Disconnect(id) }

// ---- polls ----

func (a *App) Polls() []engine.Poll { return a.engine.Polls() }

func (a *App) CreatePoll(c engine.PollConfig) (engine.Poll, error) {
	p, err := a.engine.CreatePoll(c)
	a.logResult(srcPoll, err, "Poll window %q opened", c.Name)
	return p, err
}
func (a *App) UpdatePoll(c engine.PollConfig) (engine.Poll, error) { return a.engine.UpdatePoll(c) }
func (a *App) DeletePoll(id string) {
	name := id
	for _, p := range a.engine.Polls() {
		if p.Config.ID == id {
			name = p.Config.Name
		}
	}
	a.engine.DeletePoll(id)
	a.log.Infof(srcPoll, "Poll window %q closed", name)
}
func (a *App) StartPoll(id string) error { return a.engine.StartPoll(id) }
func (a *App) StopPoll(id string)        { a.engine.StopPoll(id) }
func (a *App) StartAll() error           { return a.engine.StartAll() }
func (a *App) StopAll()                  { a.engine.StopAll() }
func (a *App) ReadOnce(id string) error  { return a.engine.ReadOnce(id) }

func (a *App) SetCellFormat(id string, address int, format string) error {
	return a.engine.SetCellFormat(id, address, datafmt.Format(format))
}

// ---- writes ----

func (a *App) Write(w engine.WriteRequest) error {
	err := a.engine.Write(w)
	a.logResult(srcWrite, err, "F%02d write to %s, slave %d, address %d: %s", w.Function, a.connName(w.ConnectionID), w.SlaveID, w.Address, valuesText(w.Values))
	return err
}

// ReadWrite performs function 23 and returns the registers read.
func (a *App) ReadWrite(r engine.ReadWriteRequest) ([]int, error) {
	out, err := a.engine.ReadWrite(r)
	a.logResult(srcWrite, err, "F23 read/write on %s, slave %d: wrote %s at %d", a.connName(r.ConnectionID), r.SlaveID, valuesText(r.Values), r.WriteAddress)
	return out, err
}

func (a *App) WriteCell(pollID string, address int, value string, forceMultiple bool) error {
	err := a.engine.WriteCell(pollID, address, value, forceMultiple)
	a.logResult(srcWrite, err, "%s: wrote %s to address %d", a.pollName(pollID), value, address)
	return err
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
		a.log.Errorf(srcSimulator, "Simulator failed to start: %v", err)
		return err
	}
	a.log.Infof(srcSimulator, "Simulator started (%s)", simText(c))
	a.simConfig = c
	a.markDirty()
	return nil
}

func (a *App) SimStop() {
	a.engine.Sim.Stop()
	a.log.Infof(srcSimulator, "Simulator stopped")
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
