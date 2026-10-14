package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"appmodbus/internal/engine"
	"appmodbus/internal/recorder"
	"appmodbus/internal/settings"
)

const eventRecording = "recording:status"

// DataSettings are the data-management preferences shown in the UI.
type DataSettings struct {
	Folder        string `json:"folder"`
	DefaultFolder string `json:"defaultFolder"`
	RetentionDays int    `json:"retentionDays"`
	MaxStorageMB  int    `json:"maxStorageMB"`
	UsedBytes     int64  `json:"usedBytes"`
	Error         string `json:"error"` // last recording error, if any
}

// startRecorder opens the recording store and its housekeeping loops.
func (a *App) startRecorder() {
	s := a.prefs().Get()
	root := s.RecordingsDir
	if root == "" {
		root = recorder.DefaultRoot()
	}
	rec, err := recorder.Open(root)
	if err != nil {
		// Fall back to the default location if a custom folder vanished.
		if rec, err = recorder.Open(recorder.DefaultRoot()); err != nil {
			a.recErr.Store("recording disabled: " + err.Error())
			return
		}
		a.recErr.Store(fmt.Sprintf("recording folder %s unavailable, using the default", root))
	}
	a.rec = rec
	a.engine.OnSample = a.recordSample
	go a.recorderLoop()
}

func (a *App) recorderLoop() {
	flush := time.NewTicker(time.Second)
	clean := time.NewTicker(10 * time.Minute)
	defer flush.Stop()
	defer clean.Stop()
	a.enforceLimits()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-flush.C:
			if err := a.rec.Flush(); err != nil {
				a.setRecErr(err)
			}
		case <-clean.C:
			a.enforceLimits()
		}
	}
}

func (a *App) enforceLimits() {
	if a.rec == nil {
		return
	}
	s := a.prefs().Get()
	if a.alarmLog != nil {
		a.alarmLog.Prune(s.RetentionDays)
	}
	if _, err := a.rec.Enforce(s.RetentionDays, int64(s.MaxStorageMB)<<20, time.Now()); err != nil {
		a.setRecErr(err)
	}
}

func (a *App) setRecErr(err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	if old, _ := a.recErr.Load().(string); old != msg {
		a.recErr.Store(msg)
		runtime.EventsEmit(a.ctx, eventRecording, msg)
	}
}

// recordSample stores every value of a recorded poll read.
func (a *App) recordSample(cfg engine.PollConfig, conn string, cells []engine.Cell, t time.Time) {
	if a.rec == nil {
		return
	}
	for _, c := range cells {
		if c.Span == 0 || !c.HasNum {
			continue
		}
		m := recorder.Meta{
			ID:         recorder.SeriesID(conn, cfg.SlaveID, cfg.Function, c.Address, c.Format),
			Connection: conn, SlaveID: cfg.SlaveID, Function: cfg.Function, Address: c.Address,
			Format: c.Format, Name: c.Name, Unit: c.Unit, Window: cfg.Name,
		}
		if err := a.rec.Record(m, t, c.Num); err != nil {
			a.setRecErr(err)
			return
		}
	}
}

// SetPollRecording turns recording on or off for a poll window.
func (a *App) SetPollRecording(pollID string, on bool) error {
	if on && a.rec == nil {
		return errors.New("recording is unavailable: " + a.recErrText())
	}
	return a.engine.SetRecord(pollID, on)
}

func (a *App) recErrText() string { s, _ := a.recErr.Load().(string); return s }

// GetDataSettings returns the data-management settings and usage.
func (a *App) GetDataSettings() DataSettings {
	s := a.prefs().Get()
	d := DataSettings{DefaultFolder: recorder.DefaultRoot(), RetentionDays: s.RetentionDays, MaxStorageMB: s.MaxStorageMB, Error: a.recErrText()}
	if a.rec != nil {
		d.Folder = a.rec.Root()
		d.UsedBytes = a.rec.Usage()
	}
	return d
}

// SetDataLimits changes retention and the storage cap, then applies them.
func (a *App) SetDataLimits(retentionDays, maxStorageMB int) (DataSettings, error) {
	if retentionDays < 0 || retentionDays > 3650 {
		return DataSettings{}, errors.New("retention must be 0 (forever) to 3650 days")
	}
	if maxStorageMB < 0 || (maxStorageMB > 0 && maxStorageMB < 50) {
		return DataSettings{}, errors.New("maximum size must be 0 (no limit) or at least 50 MB")
	}
	if _, err := a.prefs().Update(func(s *settings.Settings) {
		s.RetentionDays, s.MaxStorageMB = retentionDays, maxStorageMB
	}); err != nil {
		return DataSettings{}, err
	}
	a.enforceLimits()
	return a.GetDataSettings(), nil
}

// ChooseRecordingsFolder asks for a new storage folder. New recordings go
// there; existing ones stay in the old folder.
func (a *App) ChooseRecordingsFolder() (DataSettings, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a folder for recordings", CanCreateDirectories: true})
	if err != nil || dir == "" || a.rec == nil {
		return a.GetDataSettings(), err
	}
	return a.setRecordingsFolder(dir)
}

// ResetRecordingsFolder goes back to the default folder.
func (a *App) ResetRecordingsFolder() (DataSettings, error) { return a.setRecordingsFolder("") }

func (a *App) setRecordingsFolder(dir string) (DataSettings, error) {
	root := dir
	if root == "" {
		root = recorder.DefaultRoot()
	}
	if err := a.rec.SetRoot(root); err != nil {
		return a.GetDataSettings(), err
	}
	if _, err := a.prefs().Update(func(s *settings.Settings) { s.RecordingsDir = dir }); err != nil {
		return a.GetDataSettings(), err
	}
	a.setRecErr(nil)
	return a.GetDataSettings(), nil
}

// RecordedSeries lists every recorded series with its size and time span.
func (a *App) RecordedSeries() []recorder.SeriesInfo {
	if a.rec == nil {
		return []recorder.SeriesInfo{}
	}
	return a.rec.Series()
}

// QuerySeries returns history for a chart, reduced to about maxPoints.
func (a *App) QuerySeries(id string, from, to int64, maxPoints int) ([]recorder.Sample, error) {
	if a.rec == nil {
		return []recorder.Sample{}, nil
	}
	s, err := a.rec.Query(id, from, to, maxPoints)
	if s == nil {
		s = []recorder.Sample{}
	}
	return s, err
}

// ExportResult reports a finished export.
type ExportResult struct {
	Path string `json:"path"` // "" when the dialog was cancelled
	Rows int    `json:"rows"`
}

// ExportRecording saves the selected series between from and to (unix ms)
// as "csv" or "xlsx".
func (a *App) ExportRecording(ids []string, from, to int64, format string) (ExportResult, error) {
	if a.rec == nil {
		return ExportResult{}, errors.New("recording is unavailable")
	}
	ext, label := ".csv", "CSV (*.csv)"
	if format == "xlsx" {
		ext, label = ".xlsx", "Excel workbook (*.xlsx)"
	}
	name := "AppMODBUS-" + time.UnixMilli(from).Format("2006-01-02_1504") + ext
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title: "Export recording", DefaultFilename: name,
		Filters: []runtime.FileFilter{{DisplayName: label, Pattern: "*" + ext}},
	})
	if err != nil || path == "" {
		return ExportResult{}, err
	}
	if !strings.EqualFold(filepath.Ext(path), ext) {
		path += ext
	}
	var n int
	if format == "xlsx" {
		n, err = a.rec.ExportXLSX(path, ids, from, to, time.Local)
	} else {
		n, err = a.rec.ExportCSV(path, ids, from, to, time.Local)
	}
	return ExportResult{Path: path, Rows: n}, err
}

// DeleteRecordedSeries removes series completely.
func (a *App) DeleteRecordedSeries(ids []string) error {
	if a.rec == nil {
		return nil
	}
	return a.rec.DeleteSeries(ids)
}

// DeleteRecordedRange removes samples between from and to (unix ms) from
// the given series, or from all series when ids is empty.
func (a *App) DeleteRecordedRange(ids []string, from, to int64) error {
	if a.rec == nil {
		return nil
	}
	return a.rec.DeleteRange(ids, from, to)
}

// SeriesRef identifies where a charted value comes from.
type SeriesRef struct {
	Connection string `json:"connection"` // connection summary, as shown in the UI
	SlaveID    int    `json:"slaveId"`
	Function   int    `json:"function"`
	Address    int    `json:"address"`
	Format     string `json:"format"`
}

// SeriesIDs returns the recording ID of each reference, so charts can load
// recorded history for the registers they show.
func (a *App) SeriesIDs(refs []SeriesRef) []string {
	ids := make([]string, len(refs))
	for i, r := range refs {
		ids[i] = recorder.SeriesID(r.Connection, r.SlaveID, r.Function, r.Address, r.Format)
	}
	return ids
}
