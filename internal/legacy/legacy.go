// Package legacy carries a user's data over from AppMODBUS, the app's name
// up to v2.0.0, on the first start of Modbus Studio.
package legacy

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"modbusstudio/internal/applog"
	"modbusstudio/internal/settings"
)

// Note describes one migration step, for the application log.
type Note struct {
	Level string // applog level
	Text  string
}

// Migrate carries a user's data over from the AppMODBUS folders on the first
// start of Modbus Studio:
//
//   - the config folder (settings, session, profiles, alarm history, logs) is
//     copied, so nothing is lost if the copy is interrupted;
//   - the default recordings folder, which can be large, is moved. If it
//     cannot be moved (e.g. another drive), it stays where it is and the
//     settings point to it.
//
// Nothing happens when the new folders already exist or there is nothing old.
func Migrate(oldConfig, newConfig, oldRec, newRec string) []Note {
	var notes []Note
	if oldConfig != "" && isDir(oldConfig) && !exists(newConfig) {
		if err := copyTree(oldConfig, newConfig); err != nil {
			notes = append(notes, Note{applog.Warn, fmt.Sprintf("Could not copy settings from %s: %v", oldConfig, err)})
		} else {
			notes = append(notes, Note{applog.Info, fmt.Sprintf("Settings, profiles and history copied from %s", oldConfig)})
		}
	}
	if oldRec != "" && isDir(oldRec) && !exists(newRec) {
		err := os.MkdirAll(filepath.Dir(newRec), 0o755)
		if err == nil {
			err = os.Rename(oldRec, newRec)
		}
		if err == nil {
			notes = append(notes, Note{applog.Info, fmt.Sprintf("Recordings moved from %s to %s", oldRec, newRec)})
		} else {
			st := settings.Open(newConfig)
			if st.Get().RecordingsDir == "" {
				_, _ = st.Update(func(s *settings.Settings) { s.RecordingsDir = oldRec })
			}
			notes = append(notes, Note{applog.Warn, fmt.Sprintf("Recordings stay in %s (could not move them: %v)", oldRec, err)})
		}
	}
	return notes
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
func isDir(p string) bool  { st, err := os.Stat(p); return err == nil && st.IsDir() }

// copyTree copies the regular files and folders under src to dst.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case d.Type().IsRegular():
			return copyFile(p, target)
		}
		return nil // skip links and special files
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
