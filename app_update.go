package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"modbusstudio/internal/settings"
	"modbusstudio/internal/update"
	"modbusstudio/internal/version"
)

// Update events sent to the frontend.
const (
	eventUpdateState    = "update:state"
	eventUpdateProgress = "update:progress"
)

// AppInfo describes this build.
type AppInfo struct {
	Version    string `json:"version"`
	Commit     string `json:"commit"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	Repo       string `json:"repo"`
	CanInstall bool   `json:"canInstall"`
}

// UpdateState is the updater status shown in the UI.
type UpdateState struct {
	// Status is one of: idle, checking, uptodate, available, downloading,
	// installing, error.
	Status    string          `json:"status"`
	Release   *update.Release `json:"release"`
	Error     string          `json:"error"`
	CheckedAt int64           `json:"checkedAt"`
	Skipped   bool            `json:"skipped"` // the available version was skipped by the user
}

type updater struct {
	mu       sync.Mutex
	u        *update.Updater
	settings *settings.Store
	state    UpdateState
	busy     bool
}

func newUpdater() *updater {
	linux := update.LinuxPortable
	if exe, err := executablePath(); err == nil && goruntime.GOOS == "linux" {
		linux = update.DetectLinuxFlavor(exe, version.WebKit41)
	}
	return &updater{
		u: &update.Updater{
			Linux: linux,
			// MODBUS_STUDIO_UPDATE_API points the updater at a GitHub-compatible
			// mirror (used for testing); the signature check still applies.
			APIBase:   os.Getenv("MODBUS_STUDIO_UPDATE_API"),
			Repo:      version.UpdateRepo,
			Current:   version.Version,
			PublicKey: version.UpdatePublicKey,
		},
		settings: settings.Open(settings.DefaultDir()),
		state:    UpdateState{Status: "idle"},
	}
}

func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

// startUpdater removes leftovers of a previous update and, if enabled,
// checks for a new version shortly after startup.
func (a *App) startUpdater() {
	if exe, err := executablePath(); err == nil {
		update.Cleanup(goruntime.GOOS, exe)
	}
	if a.updates.settings.Get().AutoCheckUpdates {
		go func() {
			time.Sleep(3 * time.Second)
			_, _ = a.CheckForUpdates()
		}()
	}
}

func (a *App) setUpdateState(fn func(*UpdateState)) UpdateState {
	a.updates.mu.Lock()
	fn(&a.updates.state)
	st := a.updates.state
	a.updates.mu.Unlock()
	a.observeUpdate(st)
	runtime.EventsEmit(a.ctx, eventUpdateState, st)
	return st
}

// AppInfo returns version and platform details.
func (a *App) AppInfo() AppInfo {
	return AppInfo{
		Version: version.Version, Commit: version.Commit, OS: goruntime.GOOS, Arch: goruntime.GOARCH,
		Repo: version.UpdateRepo, CanInstall: a.updates.u.CanInstall(),
	}
}

// UpdateSettings returns the persisted preferences.
func (a *App) UpdateSettings() settings.Settings { return a.updates.settings.Get() }

// SetUpdateSettings changes the channel and automatic checking.
func (a *App) SetUpdateSettings(channel string, autoCheck bool) (settings.Settings, error) {
	return a.updates.settings.Update(func(s *settings.Settings) {
		s.UpdateChannel = channel
		s.AutoCheckUpdates = autoCheck
	})
}

// GetUpdateState returns the current updater state.
func (a *App) GetUpdateState() UpdateState {
	a.updates.mu.Lock()
	defer a.updates.mu.Unlock()
	return a.updates.state
}

// CheckForUpdates queries GitHub for a newer release on the chosen channel.
func (a *App) CheckForUpdates() (UpdateState, error) {
	a.updates.mu.Lock()
	if a.updates.busy {
		st := a.updates.state
		a.updates.mu.Unlock()
		return st, nil
	}
	a.updates.busy = true
	a.updates.mu.Unlock()
	defer func() {
		a.updates.mu.Lock()
		a.updates.busy = false
		a.updates.mu.Unlock()
	}()

	a.setUpdateState(func(s *UpdateState) { s.Status, s.Error = "checking", "" })
	set := a.updates.settings.Get()
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	rel, err := a.updates.u.Check(ctx, update.Channel(set.UpdateChannel))
	now := time.Now().UnixMilli()
	_, _ = a.updates.settings.Update(func(s *settings.Settings) { s.LastUpdateCheckAt = now })
	if err != nil {
		st := a.setUpdateState(func(s *UpdateState) { s.Status, s.Error, s.CheckedAt = "error", err.Error(), now })
		return st, err
	}
	st := a.setUpdateState(func(s *UpdateState) {
		s.CheckedAt, s.Release, s.Error = now, rel, ""
		s.Status = "uptodate"
		s.Skipped = false
		if rel != nil {
			s.Status = "available"
			s.Skipped = rel.Version == set.SkippedVersion
		}
	})
	return st, nil
}

// SkipVersion stops the update banner for one version.
func (a *App) SkipVersion(v string) error {
	_, err := a.updates.settings.Update(func(s *settings.Settings) { s.SkippedVersion = v })
	a.setUpdateState(func(s *UpdateState) { s.Skipped = s.Release != nil && s.Release.Version == v })
	return err
}

// OpenReleasePage opens the release notes in the default browser.
func (a *App) OpenReleasePage() {
	if rel := a.GetUpdateState().Release; rel != nil && rel.PageURL != "" {
		runtime.BrowserOpenURL(a.ctx, rel.PageURL)
	}
}

// InstallUpdate downloads, verifies and installs the available release,
// then restarts the application.
func (a *App) InstallUpdate() error {
	a.updates.mu.Lock()
	rel := a.updates.state.Release
	if a.updates.busy || rel == nil {
		a.updates.mu.Unlock()
		return errors.New("no update is ready to install")
	}
	a.updates.busy = true
	a.updates.mu.Unlock()
	defer func() {
		a.updates.mu.Lock()
		a.updates.busy = false
		a.updates.mu.Unlock()
	}()

	fail := func(err error) error {
		a.setUpdateState(func(s *UpdateState) { s.Status, s.Error = "error", err.Error() })
		return err
	}
	exe, err := executablePath()
	if err != nil {
		return fail(err)
	}
	dir, err := os.MkdirTemp("", "ModbusStudio-update-")
	if err != nil {
		return fail(err)
	}
	progress := func(done, total int64) {
		runtime.EventsEmit(a.ctx, eventUpdateProgress, map[string]int64{"done": done, "total": total})
	}
	a.setUpdateState(func(s *UpdateState) { s.Status, s.Error = "downloading", "" })
	pkg, err := a.updates.u.Download(a.ctx, rel, rel.Asset, dir, progress)
	if err != nil {
		return fail(err)
	}
	a.setUpdateState(func(s *UpdateState) { s.Status = "installing" })
	launch, err := update.Install(goruntime.GOOS, pkg, exe)
	if errors.Is(err, update.ErrNotWritable) && rel.Installer != nil {
		// Installed for all users (e.g. Program Files): run the signed
		// installer, which asks for administrator rights.
		a.setUpdateState(func(s *UpdateState) { s.Status = "downloading" })
		inst, derr := a.updates.u.Download(a.ctx, rel, *rel.Installer, dir, progress)
		if derr != nil {
			return fail(derr)
		}
		a.setUpdateState(func(s *UpdateState) { s.Status = "installing" })
		launch, err = update.Launch{Path: inst}, nil
	}
	if err != nil {
		if errors.Is(err, update.ErrNotWritable) {
			err = errors.New("Modbus Studio cannot replace itself in " + filepath.Dir(exe) + "; download the new version from the release page instead")
		}
		return fail(err)
	}
	cmd := exec.Command(launch.Path, launch.Args...)
	if err := cmd.Start(); err != nil {
		return fail(err)
	}
	_ = cmd.Process.Release()
	a.engine.Shutdown()
	runtime.Quit(a.ctx)
	return nil
}
