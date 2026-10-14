package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistAndDefaults(t *testing.T) {
	dir := t.TempDir()
	st := Open(dir)
	if s := st.Get(); s.UpdateChannel != "stable" || !s.AutoCheckUpdates {
		t.Fatalf("defaults %+v", s)
	}
	if _, err := st.Update(func(s *Settings) { s.UpdateChannel = "beta"; s.SkippedVersion = "1.2.3" }); err != nil {
		t.Fatal(err)
	}
	if s := Open(dir).Get(); s.UpdateChannel != "beta" || s.SkippedVersion != "1.2.3" || !s.AutoCheckUpdates {
		t.Fatalf("reloaded %+v", s)
	}
	// Unknown channels fall back to stable; corrupt files to defaults.
	st.Update(func(s *Settings) { s.UpdateChannel = "nightly" })
	if s := Open(dir).Get(); s.UpdateChannel != "stable" {
		t.Fatalf("channel %q", s.UpdateChannel)
	}
	os.WriteFile(filepath.Join(dir, "settings.json"), []byte("{not json"), 0o644)
	if s := Open(dir).Get(); s != Defaults() {
		t.Fatalf("corrupt file: %+v", s)
	}
}
