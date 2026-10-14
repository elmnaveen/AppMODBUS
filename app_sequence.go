package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"modbusstudio/internal/engine"
)

// RunSequence starts a test sequence.
func (a *App) RunSequence(s engine.Sequence) error { return a.engine.RunSequence(s) }

// StopSequence stops the running sequence.
func (a *App) StopSequence() { a.engine.StopSequence() }

// AnswerPrompt continues (ok) or fails a waiting operator prompt.
func (a *App) AnswerPrompt(ok bool) { a.engine.AnswerPrompt(ok) }

// SequenceState returns the current or last run.
func (a *App) SequenceState() engine.SequenceRun { return a.engine.SequenceState() }

var verdictText = map[string]string{"pass": "PASS", "fail": "FAIL", "stopped": "STOPPED", "error": "ERROR"}

// ExportSequenceReport saves the last run as CSV.
func (a *App) ExportSequenceReport() (string, error) {
	r := a.engine.SequenceState()
	if r.Started == 0 {
		return "", fmt.Errorf("no sequence has run yet")
	}
	stamp := time.UnixMilli(r.Started)
	name := strings.Map(func(c rune) rune {
		if strings.ContainsRune(`\/:*?"<>|`, c) {
			return '_'
		}
		return c
	}, r.Name)
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save test report",
		DefaultFilename: fmt.Sprintf("%s %s.csv", name, stamp.Format("2006-01-02 1504")),
		Filters:         []runtime.FileFilter{{DisplayName: "CSV (*.csv)", Pattern: "*.csv"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	var b strings.Builder
	b.WriteString("\xef\xbb\xbf")
	w := csv.NewWriter(&b)
	finished := ""
	if r.Finished > 0 {
		finished = time.UnixMilli(r.Finished).Format("2006-01-02 15:04:05")
	}
	for _, row := range [][]string{
		{"Test", r.Name},
		{"Connection", r.Connection},
		{"Started", stamp.Format("2006-01-02 15:04:05")},
		{"Finished", finished},
		{"Result", verdictText[r.Verdict]},
		{"Passed", fmt.Sprint(r.Passed)},
		{"Failed", fmt.Sprint(r.Failed)},
		{"Errors", fmt.Sprint(r.Errors)},
		{},
		{"Run", "Step", "Label", "Result", "Value", "Expected", "Message", "Time (ms)", "Timestamp"},
	} {
		_ = w.Write(row)
	}
	for _, s := range r.Results {
		_ = w.Write([]string{
			fmt.Sprint(s.Run), fmt.Sprint(s.Index + 1), s.Label, strings.ToUpper(s.Status), s.Value, s.Expect, s.Message,
			fmt.Sprintf("%.1f", s.Ms), time.UnixMilli(s.Time).Format("15:04:05.000"),
		})
	}
	w.Flush()
	return path, os.WriteFile(path, []byte(b.String()), 0o644)
}
