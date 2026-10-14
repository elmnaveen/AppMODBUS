package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"modbusstudio/internal/engine"
	"modbusstudio/internal/settings"
)

// startAlarmLog keeps the alarm history next to the settings.
func (a *App) startAlarmLog() {
	a.alarmLog = engine.NewAlarmLog(filepath.Join(settings.DefaultDir(), "alarms"))
	a.engine.OnAlarm = func(ev engine.AlarmEvent) { _ = a.alarmLog.Append(ev) }
	a.alarmLog.Prune(a.prefs().Get().RetentionDays)
}

// Alarms returns the current alarm list.
func (a *App) Alarms() []engine.Alarm { return a.engine.Alarms() }

// AckAlarms acknowledges alarms (none = all).
func (a *App) AckAlarms(ids []string) { a.engine.AckAlarms(ids) }

// AlarmHistory returns logged events between from and to (unix ms), newest first.
func (a *App) AlarmHistory(from, to int64, limit int) ([]engine.AlarmEvent, error) {
	return a.alarmLog.Query(from, to, limit)
}

var alarmLevelNames = map[string]string{
	engine.LevelHiHi: "High-high", engine.LevelHi: "High", engine.LevelLo: "Low", engine.LevelLoLo: "Low-low",
	engine.LevelState: "State", engine.LevelComm: "Communication",
}

// ExportAlarmHistory saves the history between from and to as CSV.
func (a *App) ExportAlarmHistory(from, to int64) (ExportResult, error) {
	events, err := a.alarmLog.Query(from, to, 0)
	if err != nil {
		return ExportResult{}, err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export alarm history",
		DefaultFilename: "alarms-" + time.Now().Format("2006-01-02") + ".csv",
		Filters:         []runtime.FileFilter{{DisplayName: "CSV (*.csv)", Pattern: "*.csv"}},
	})
	if err != nil || path == "" {
		return ExportResult{}, err
	}
	var b strings.Builder
	b.WriteString("\xef\xbb\xbf")
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"Time", "Event", "Window", "Tag", "Level", "Value", "Limit", "Unit", "Message"})
	for i := len(events) - 1; i >= 0; i-- { // oldest first in the file
		ev := events[i]
		num := func(v float64) string { return strconv.FormatFloat(v, 'g', 10, 64) }
		value, limit := num(ev.Value), num(ev.Limit)
		if ev.Level == engine.LevelComm {
			value, limit = "", ""
		}
		_ = w.Write([]string{
			time.UnixMilli(ev.Time).Format("2006-01-02 15:04:05.000"), ev.Kind, ev.Window, ev.Name,
			alarmLevelNames[ev.Level], value, limit, ev.Unit, ev.Message,
		})
	}
	w.Flush()
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return ExportResult{}, fmt.Errorf("write %s: %w", path, err)
	}
	return ExportResult{Path: path, Rows: len(events)}, nil
}
