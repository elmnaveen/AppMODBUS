package profile

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

//go:embed bundled/*.json
var bundledFS embed.FS

// Source says where a profile lives.
type Source string

const (
	Bundled Source = "bundled" // shipped with the app, read-only
	User    Source = "user"    // this user's profile folder
	Shared  Source = "shared"  // team folder, e.g. on a network drive
)

// Summary describes a profile in the library list.
type Summary struct {
	ID           string `json:"id"` // "<source>:<file name>"
	Source       Source `json:"source"`
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	Registers    int    `json:"registers"`
	Path         string `json:"path"`
	Error        string `json:"error,omitempty"` // file present but unreadable
}

// Library reads and writes profiles from the three sources.
type Library struct {
	UserDir   string
	SharedDir string // optional
	bundled   fs.FS
}

// NewLibrary creates a library with the bundled profiles.
func NewLibrary(userDir, sharedDir string) *Library {
	sub, _ := fs.Sub(bundledFS, "bundled")
	return &Library{UserDir: userDir, SharedDir: sharedDir, bundled: sub}
}

func (l *Library) dir(s Source) (string, error) {
	switch s {
	case User:
		return l.UserDir, nil
	case Shared:
		if l.SharedDir == "" {
			return "", errors.New("no shared profile folder is configured")
		}
		return l.SharedDir, nil
	}
	return "", fmt.Errorf("profiles in %s cannot be changed", s)
}

// List returns every profile, bundled first, sorted by name.
func (l *Library) List() []Summary {
	var out []Summary
	add := func(src Source, fsys fs.FS, base string) {
		entries, _ := fs.ReadDir(fsys, ".")
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".json") {
				continue
			}
			s := Summary{ID: string(src) + ":" + e.Name(), Source: src}
			if base != "" {
				s.Path = filepath.Join(base, e.Name())
			}
			b, err := fs.ReadFile(fsys, e.Name())
			if err == nil {
				var p Profile
				if p, err = Decode(b); err == nil {
					s.Name, s.Manufacturer, s.Model, s.Registers = p.Name, p.Manufacturer, p.Model, len(p.Registers)
				}
			}
			if err != nil {
				s.Name, s.Error = e.Name(), err.Error()
			}
			out = append(out, s)
		}
	}
	add(Bundled, l.bundled, "")
	if l.UserDir != "" {
		add(User, os.DirFS(l.UserDir), l.UserDir)
	}
	if l.SharedDir != "" {
		add(Shared, os.DirFS(l.SharedDir), l.SharedDir)
	}
	rank := map[Source]int{Bundled: 0, Shared: 1, User: 2}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return rank[out[i].Source] < rank[out[j].Source]
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	if out == nil {
		out = []Summary{}
	}
	return out
}

func splitID(id string) (Source, string, error) {
	src, name, ok := strings.Cut(id, ":")
	if !ok || name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return "", "", fmt.Errorf("invalid profile id %q", id)
	}
	return Source(src), name, nil
}

// Get loads a profile by ID.
func (l *Library) Get(id string) (Profile, error) {
	src, name, err := splitID(id)
	if err != nil {
		return Profile{}, err
	}
	var b []byte
	if src == Bundled {
		b, err = fs.ReadFile(l.bundled, name)
	} else {
		dir, derr := l.dir(src)
		if derr != nil {
			return Profile{}, derr
		}
		b, err = os.ReadFile(filepath.Join(dir, name))
	}
	if err != nil {
		return Profile{}, err
	}
	return Decode(b)
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._ -]+`)

// FileName derives a file name from a profile's name.
func FileName(p Profile) string {
	n := strings.TrimSpace(unsafeChars.ReplaceAllString(p.Name, "-"))
	if n == "" {
		n = "profile"
	}
	return n + ".json"
}

// Save writes a profile into a source folder. When id names an existing
// profile in that folder it is replaced, otherwise a new file is created.
// It returns the profile's ID.
func (l *Library) Save(p Profile, dest Source, id string) (string, error) {
	b, err := Encode(p)
	if err != nil {
		return "", err
	}
	dir, err := l.dir(dest)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := ""
	if src, n, err := splitID(id); err == nil && src == dest {
		name = n
	} else {
		base := strings.TrimSuffix(FileName(p), ".json")
		name = base + ".json"
		for i := 2; ; i++ {
			if _, err := os.Stat(filepath.Join(dir, name)); errors.Is(err, os.ErrNotExist) {
				break
			}
			name = fmt.Sprintf("%s (%d).json", base, i)
		}
	}
	tmp := filepath.Join(dir, "."+name+".tmp")
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, filepath.Join(dir, name)); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return string(dest) + ":" + name, nil
}

// Delete removes a user or shared profile.
func (l *Library) Delete(id string) error {
	src, name, err := splitID(id)
	if err != nil {
		return err
	}
	dir, err := l.dir(src)
	if err != nil {
		return err
	}
	return os.Remove(filepath.Join(dir, name))
}
