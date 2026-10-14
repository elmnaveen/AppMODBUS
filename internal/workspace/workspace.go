// Package workspace defines the Modbus Studio workspace file (.mbw): a JSON
// document holding connections, poll windows, layout, charts and the
// simulator setup, so a test session can be saved and reopened.
package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"modbusstudio/internal/engine"
	"modbusstudio/internal/simulator"
)

// File identification.
const (
	FormatID       = "modbus-studio-workspace"
	CurrentVersion = 1
	// LegacyFormatID marks workspaces saved before the rename to Modbus
	// Studio (AppMODBUS, up to v2.0.0); they open unchanged.
	LegacyFormatID = "appmodbus-workspace"
	Extension      = ".mbw"
)

// Layout is the UI arrangement saved with a workspace.
type Layout struct {
	View          string `json:"view"` // "tabs" or "tile"
	ActiveID      string `json:"activeId"`
	ShowTraffic   bool   `json:"showTraffic"`
	ShowSimulator bool   `json:"showSimulator"`
}

// Simulator is the saved simulator configuration and data.
type Simulator struct {
	Config  simulator.Config                      `json:"config"`
	Running bool                                  `json:"running"`
	Data    map[simulator.Table][]simulator.Block `json:"data"`
}

// File is the on-disk workspace document.
type File struct {
	Format      string                    `json:"format"`
	Version     int                       `json:"version"`
	App         string                    `json:"app,omitempty"` // Modbus Studio version that saved it
	Connections []engine.ConnectionConfig `json:"connections"`
	Polls       []engine.PollConfig       `json:"polls"`
	Layout      Layout                    `json:"layout"`
	Charts      json.RawMessage           `json:"charts,omitempty"`
	Sequences   json.RawMessage           `json:"sequences,omitempty"` // test sequences (owned by the UI)
	Simulator   *Simulator                `json:"simulator,omitempty"`
}

// New returns an empty workspace document.
func New() File {
	return File{Format: FormatID, Version: CurrentVersion, Connections: []engine.ConnectionConfig{}, Polls: []engine.PollConfig{}, Layout: Layout{View: "tabs", ShowTraffic: true}}
}

// Encode serialises a workspace as indented JSON.
func Encode(f File) ([]byte, error) {
	f.Format, f.Version = FormatID, CurrentVersion
	return json.MarshalIndent(f, "", "  ")
}

// Decode parses and validates a workspace document.
func Decode(b []byte) (File, error) {
	var head struct {
		Format  string `json:"format"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(b, &head); err != nil {
		return File{}, fmt.Errorf("not a workspace file: %w", err)
	}
	if head.Format != FormatID && head.Format != LegacyFormatID {
		return File{}, errors.New("not a Modbus Studio workspace file")
	}
	if head.Version > CurrentVersion {
		return File{}, fmt.Errorf("this workspace was saved by a newer Modbus Studio (format %d); please update the app", head.Version)
	}
	f := New()
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("corrupt workspace file: %w", err)
	}
	if f.Layout.View != "tile" {
		f.Layout.View = "tabs"
	}
	return f, nil
}

// Read loads a workspace from disk.
func Read(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	return Decode(b)
}

// WriteAtomic writes data to path via a temporary file and rename, so a
// crash mid-save never leaves a truncated file behind.
func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mbw-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_ = tmp.Chmod(0o644) // CreateTemp uses 0600; workspaces are meant to be shared
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Write saves a workspace to disk.
func Write(path string, f File) error {
	b, err := Encode(f)
	if err != nil {
		return err
	}
	return WriteAtomic(path, b)
}

// WithExtension appends .mbw unless the path already has it.
func WithExtension(path string) string {
	if strings.EqualFold(filepath.Ext(path), Extension) {
		return path
	}
	return path + Extension
}

// DisplayName is the file name without extension, or "Untitled".
func DisplayName(path string) string {
	if path == "" {
		return "Untitled"
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// Session is the autosaved state of the running app: the current workspace
// contents plus which file it belongs to and whether it has unsaved changes.
type Session struct {
	Path  string `json:"path"`
	Dirty bool   `json:"dirty"`
	File  File   `json:"workspace"`
}

// ReadSession loads the autosaved session, if any.
func ReadSession(path string) (*Session, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(s.File)
	if s.File, err = Decode(raw); err != nil {
		return nil, err
	}
	return &s, nil
}

// WriteSession autosaves the session.
func WriteSession(path string, s Session) error {
	s.File.Format, s.File.Version = FormatID, CurrentVersion
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return WriteAtomic(path, b)
}
