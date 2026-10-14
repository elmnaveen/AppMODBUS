// Package settings persists user preferences as JSON in the OS config
// directory (e.g. %AppData%\AppMODBUS on Windows).
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Settings are the persisted user preferences.
type Settings struct {
	UpdateChannel     string `json:"updateChannel"` // "stable" or "beta"
	AutoCheckUpdates  bool   `json:"autoCheckUpdates"`
	SkippedVersion    string `json:"skippedVersion"`
	LastUpdateCheckAt int64  `json:"lastUpdateCheckAt"` // unix ms

	RecentWorkspaces []string `json:"recentWorkspaces"` // most recent first

	SharedProfileDir string `json:"sharedProfileDir"` // optional team profile folder

	// Data management for recordings.
	RecordingsDir string `json:"recordingsDir"` // "" = platform default
	RetentionDays int    `json:"retentionDays"` // 0 = keep forever
	MaxStorageMB  int    `json:"maxStorageMB"`  // 0 = no limit
}

// MaxRecent is the length of the recent workspaces list.
const MaxRecent = 10

// AddRecent moves path to the front of the recent workspaces list.
func (s *Settings) AddRecent(path string) {
	list := []string{path}
	for _, p := range s.RecentWorkspaces {
		if p != path && len(list) < MaxRecent {
			list = append(list, p)
		}
	}
	s.RecentWorkspaces = list
}

// Defaults are used when no settings file exists.
func Defaults() Settings {
	return Settings{UpdateChannel: "stable", AutoCheckUpdates: true, RecentWorkspaces: []string{}, RetentionDays: 30, MaxStorageMB: 2048}
}

// Store loads and saves settings to one file.
type Store struct {
	mu   sync.Mutex
	path string
	s    Settings
}

// Open loads settings from dir (created if needed); missing or corrupt
// files fall back to defaults.
func Open(dir string) *Store {
	st := &Store{path: filepath.Join(dir, "settings.json"), s: Defaults()}
	if b, err := os.ReadFile(st.path); err == nil {
		s := Defaults()
		if json.Unmarshal(b, &s) == nil {
			if s.UpdateChannel != "beta" {
				s.UpdateChannel = "stable"
			}
			st.s = s
		}
	}
	return st
}

// DefaultDir returns the per-user config directory for Modbus Studio.
func DefaultDir() string {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "AppMODBUS")
	}
	return "."
}

// Get returns a copy of the current settings.
func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

// Update applies fn and writes the result atomically.
func (st *Store) Update(fn func(*Settings)) (Settings, error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	next := st.s
	fn(&next)
	if next.UpdateChannel != "beta" {
		next.UpdateChannel = "stable"
	}
	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return st.s, err
	}
	if err := os.MkdirAll(filepath.Dir(st.path), 0o755); err != nil {
		return st.s, err
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return st.s, err
	}
	if err := os.Rename(tmp, st.path); err != nil {
		return st.s, err
	}
	st.s = next
	return next, nil
}
