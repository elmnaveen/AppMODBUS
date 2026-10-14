package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"modbusstudio/internal/datafmt"
	"modbusstudio/internal/engine"
	"modbusstudio/internal/profile"
	"modbusstudio/internal/settings"
)

// ImportPreview is shown in the import dialog after picking a file.
type ImportPreview struct {
	Path      string                `json:"path"`
	FileName  string                `json:"fileName"`
	Rows      [][]string            `json:"rows"` // first rows, for the preview grid
	TotalRows int                   `json:"totalRows"`
	Options   profile.ImportOptions `json:"options"`
	Fields    []profile.Field       `json:"fields"`
}

// ImportResult is the profile built from a table with the chosen options.
type ImportResult struct {
	Profile  profile.Profile `json:"profile"`
	Warnings []string        `json:"warnings"`
}

// ProfileFolders tells the UI where profiles are stored.
type ProfileFolders struct {
	User   string `json:"user"`
	Shared string `json:"shared"`
}

var importCache sync.Map // path -> [][]string

func (a *App) library() *profile.Library {
	return profile.NewLibrary(filepath.Join(settings.DefaultDir(), "profiles"), a.prefs().Get().SharedProfileDir)
}

// ListProfiles lists bundled, shared and user profiles.
func (a *App) ListProfiles() []profile.Summary { return a.library().List() }

// GetProfile loads one profile.
func (a *App) GetProfile(id string) (profile.Profile, error) { return a.library().Get(id) }

// SaveProfile stores a profile in "user" or "shared"; id replaces an
// existing profile in that folder. Returns the new id.
func (a *App) SaveProfile(p profile.Profile, dest string, id string) (string, error) {
	return a.library().Save(p, profile.Source(dest), id)
}

// DeleteProfile removes a user or shared profile.
func (a *App) DeleteProfile(id string) error { return a.library().Delete(id) }

// ProfileFolders returns the user and shared profile folders.
func (a *App) ProfileFolders() ProfileFolders {
	l := a.library()
	return ProfileFolders{User: l.UserDir, Shared: l.SharedDir}
}

// ChooseSharedProfileFolder asks for a team profile folder ("" = cancelled).
func (a *App) ChooseSharedProfileFolder() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose a shared profile folder"})
	if err != nil || dir == "" {
		return "", err
	}
	_, err = a.prefs().Update(func(s *settings.Settings) { s.SharedProfileDir = dir })
	return dir, err
}

// ClearSharedProfileFolder stops using the shared folder.
func (a *App) ClearSharedProfileFolder() error {
	_, err := a.prefs().Update(func(s *settings.Settings) { s.SharedProfileDir = "" })
	return err
}

// ApplyProfile creates labelled poll windows for one slave from a profile.
func (a *App) ApplyProfile(id, connectionID string, slaveID, scanRateMs int) ([]engine.Poll, error) {
	p, err := a.library().Get(id)
	if err != nil {
		return nil, err
	}
	if slaveID < 0 || slaveID > 255 {
		return nil, errors.New("slave ID must be 0..255")
	}
	cfgs, err := profile.ToPolls(p, connectionID, slaveID, scanRateMs)
	if err != nil {
		return nil, err
	}
	var out []engine.Poll
	for _, c := range cfgs {
		created, err := a.engine.CreatePoll(c)
		if err != nil {
			return out, err
		}
		out = append(out, created)
	}
	return out, nil
}

// ProfileFromPoll turns a poll window's formats and tags into a profile.
func (a *App) ProfileFromPoll(pollID, name string) (profile.Profile, error) {
	for _, p := range a.engine.Polls() {
		if p.Config.ID == pollID {
			if name == "" {
				name = p.Config.Name
			}
			return profile.FromPoll(p.Config, name), nil
		}
	}
	return profile.Profile{}, errors.New("unknown poll window")
}

// SetTag names/scales one register of a poll window; nil removes the tag.
func (a *App) SetTag(pollID string, address int, tag *engine.Tag) error {
	return a.engine.SetTag(pollID, address, tag)
}

// ImportTable asks for a CSV/Excel register map and previews it.
func (a *App) ImportTable() (*ImportPreview, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import register map",
		Filters: []runtime.FileFilter{
			{DisplayName: "Register maps (*.csv, *.xlsx, *.txt)", Pattern: "*.csv;*.xlsx;*.xlsm;*.txt;*.tsv"},
		},
	})
	if err != nil || path == "" {
		return nil, err
	}
	return a.PreviewTable(path)
}

// PreviewTable reads a register map file and guesses the column mapping.
func (a *App) PreviewTable(path string) (*ImportPreview, error) {
	rows, err := profile.ReadTable(path)
	if err != nil {
		return nil, err
	}
	importCache.Store(path, rows)
	hdr, mapping := profile.DetectHeader(rows)
	opt := profile.ImportOptions{HeaderRow: hdr, Mapping: mapping, DefaultFunction: 3, ByteOrder: datafmt.ABCD}
	opt.AddressMode = profile.SuggestAddressMode(rows, opt)
	preview := rows
	if len(preview) > 300 {
		preview = preview[:300]
	}
	return &ImportPreview{Path: path, FileName: filepath.Base(path), Rows: preview, TotalRows: len(rows), Options: opt, Fields: profile.Fields}, nil
}

// BuildImport converts the previewed table with the chosen options.
func (a *App) BuildImport(path string, opt profile.ImportOptions) (ImportResult, error) {
	v, ok := importCache.Load(path)
	if !ok {
		rows, err := profile.ReadTable(path)
		if err != nil {
			return ImportResult{}, err
		}
		v = rows
	}
	p, warnings := profile.BuildProfile(v.([][]string), opt)
	if warnings == nil {
		warnings = []string{}
	}
	if p.Registers == nil {
		p.Registers = []profile.Register{}
	}
	return ImportResult{Profile: p, Warnings: warnings}, nil
}

// ImportProfileFile copies a .json profile into the user library.
func (a *App) ImportProfileFile() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Import device profile",
		Filters: []runtime.FileFilter{{DisplayName: "Modbus Studio profile (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	p, err := profile.Decode(b)
	if err != nil {
		return "", err
	}
	return a.library().Save(p, profile.User, "")
}

// ExportProfile saves a copy of a profile to a file of the user's choice.
func (a *App) ExportProfile(id string) (string, error) {
	p, err := a.library().Get(id)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export device profile",
		DefaultFilename: profile.FileName(p),
		Filters:         []runtime.FileFilter{{DisplayName: "Modbus Studio profile (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	b, err := profile.Encode(p)
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(b, '\n'), 0o644)
}
