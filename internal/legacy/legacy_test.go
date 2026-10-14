package legacy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"modbusstudio/internal/settings"
)

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMigrateCopiesConfigAndMovesRecordings(t *testing.T) {
	root := t.TempDir()
	oldCfg, newCfg := filepath.Join(root, "cfg", "AppMODBUS"), filepath.Join(root, "cfg", "ModbusStudio")
	oldRec, newRec := filepath.Join(root, "data", "AppMODBUS", "recordings"), filepath.Join(root, "data", "ModbusStudio", "recordings")
	write(t, filepath.Join(oldCfg, "settings.json"), `{"retentionDays":7}`)
	write(t, filepath.Join(oldCfg, "profiles", "meter.json"), "profile")
	write(t, filepath.Join(oldRec, "2026-10-14", "s1.bin"), "samples")

	notes := Migrate(oldCfg, newCfg, oldRec, newRec)
	if len(notes) != 2 || notes[0].Level != "info" || notes[1].Level != "info" {
		t.Fatalf("notes %+v", notes)
	}
	if read(t, filepath.Join(newCfg, "settings.json")) != `{"retentionDays":7}` || read(t, filepath.Join(newCfg, "profiles", "meter.json")) != "profile" {
		t.Fatal("config not copied")
	}
	if _, err := os.Stat(filepath.Join(oldCfg, "settings.json")); err != nil {
		t.Fatal("old config must be kept (copied, not moved)")
	}
	if read(t, filepath.Join(newRec, "2026-10-14", "s1.bin")) != "samples" {
		t.Fatal("recordings not moved")
	}
	if _, err := os.Stat(oldRec); !os.IsNotExist(err) {
		t.Fatal("old recordings folder should be gone after the move")
	}

	// Second start: new folders exist, nothing more happens.
	write(t, filepath.Join(oldCfg, "settings.json"), `{"retentionDays":99}`)
	if notes := Migrate(oldCfg, newCfg, oldRec, newRec); len(notes) != 0 {
		t.Fatalf("second run did something: %+v", notes)
	}
	if !strings.Contains(read(t, filepath.Join(newCfg, "settings.json")), `7`) {
		t.Fatal("existing new settings were overwritten")
	}
}

func TestMigrateNothingOld(t *testing.T) {
	root := t.TempDir()
	if notes := Migrate(filepath.Join(root, "a"), filepath.Join(root, "b"), filepath.Join(root, "c"), filepath.Join(root, "d")); len(notes) != 0 {
		t.Fatalf("notes %+v", notes)
	}
	if notes := Migrate("", filepath.Join(root, "b"), "", filepath.Join(root, "d")); len(notes) != 0 {
		t.Fatalf("notes %+v", notes)
	}
}

func TestMigrateKeepsRecordingsWhenMoveFails(t *testing.T) {
	root := t.TempDir()
	newCfg := filepath.Join(root, "cfg")
	oldRec := filepath.Join(root, "old", "recordings")
	write(t, filepath.Join(oldRec, "x.bin"), "x")
	// newRec's parent is a file, so MkdirAll/Rename fail.
	write(t, filepath.Join(root, "blocker"), "")
	newRec := filepath.Join(root, "blocker", "recordings")

	notes := Migrate("", newCfg, oldRec, newRec)
	if len(notes) != 1 || notes[0].Level != "warn" {
		t.Fatalf("notes %+v", notes)
	}
	if got := settings.Open(newCfg).Get().RecordingsDir; got != oldRec {
		t.Fatalf("recordings dir setting = %q, want %q", got, oldRec)
	}
	if read(t, filepath.Join(oldRec, "x.bin")) != "x" {
		t.Fatal("recordings were touched")
	}
}
