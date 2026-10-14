package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotWritable means the app's location cannot be written by this user
// (e.g. Program Files); on Windows the installer is used instead.
var ErrNotWritable = errors.New("the application folder is not writable")

const (
	binaryName  = "AppMODBUS"
	bundleName  = "AppMODBUS.app"
	oldSuffix   = ".old"
	newSuffix   = ".new"
	macOldName  = ".AppMODBUS-old.app"
	macStageDir = ".AppMODBUS-update"
)

// Launch describes how to start the updated application.
type Launch struct {
	Path string
	Args []string
}

// Install replaces the running application with the downloaded package.
// exe is the path of the running executable.
func Install(goos, pkgPath, exe string) (Launch, error) {
	if goos == "linux" && strings.HasSuffix(pkgPath, ".deb") {
		// A .deb install is owned by dpkg: install the new package with
		// admin rights (pkexec shows the desktop password prompt), then
		// start the updated app as the current user.
		return Launch{Path: "sh", Args: []string{"-c", `pkexec dpkg -i "$0" && exec "$1"`, pkgPath, exe}}, nil
	}
	switch goos {
	case "linux":
		return installBinary(exe, func(dst string) error { return extractTarGz(pkgPath, binaryName, dst) })
	case "windows":
		return installBinary(exe, func(dst string) error { return copyFile(pkgPath, dst, 0o755) })
	case "darwin":
		return installMacBundle(pkgPath, exe)
	}
	return Launch{}, fmt.Errorf("self-update is not supported on %s", goos)
}

// installBinary swaps a single executable. Renaming a running executable is
// allowed on Windows, Linux and macOS, so the old file is moved aside and
// deleted on the next start by Cleanup.
func installBinary(exe string, write func(dst string) error) (Launch, error) {
	next, old := exe+newSuffix, exe+oldSuffix
	if err := write(next); err != nil {
		os.Remove(next)
		if errors.Is(err, fs.ErrPermission) {
			return Launch{}, ErrNotWritable
		}
		return Launch{}, err
	}
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		os.Remove(next)
		if errors.Is(err, fs.ErrPermission) {
			return Launch{}, ErrNotWritable
		}
		return Launch{}, err
	}
	if err := os.Rename(next, exe); err != nil {
		_ = os.Rename(old, exe) // roll back
		os.Remove(next)
		return Launch{}, err
	}
	return Launch{Path: exe}, nil
}

// bundleRoot finds the enclosing .app directory of a macOS executable.
func bundleRoot(exe string) (string, error) {
	for dir := filepath.Dir(exe); dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
		if strings.HasSuffix(dir, ".app") {
			return dir, nil
		}
	}
	return "", errors.New("AppMODBUS is not running from an .app bundle")
}

func installMacBundle(pkgPath, exe string) (Launch, error) {
	bundle, err := bundleRoot(exe)
	if err != nil {
		return Launch{}, err
	}
	parent := filepath.Dir(bundle)
	stage := filepath.Join(parent, macStageDir)
	os.RemoveAll(stage)
	if err := extractZip(pkgPath, stage); err != nil {
		os.RemoveAll(stage)
		if errors.Is(err, fs.ErrPermission) {
			return Launch{}, ErrNotWritable
		}
		return Launch{}, err
	}
	defer os.RemoveAll(stage)
	fresh := filepath.Join(stage, bundleName)
	if _, err := os.Stat(filepath.Join(fresh, "Contents", "MacOS", binaryName)); err != nil {
		return Launch{}, errors.New("update package does not contain AppMODBUS.app")
	}
	old := filepath.Join(parent, macOldName)
	os.RemoveAll(old)
	if err := os.Rename(bundle, old); err != nil {
		if errors.Is(err, fs.ErrPermission) {
			return Launch{}, ErrNotWritable
		}
		return Launch{}, err
	}
	if err := os.Rename(fresh, bundle); err != nil {
		_ = os.Rename(old, bundle)
		return Launch{}, err
	}
	return Launch{Path: "open", Args: []string{"-n", bundle}}, nil
}

// Cleanup removes leftovers of a previous update. Call it at startup.
func Cleanup(goos, exe string) {
	os.Remove(exe + oldSuffix)
	os.Remove(exe + newSuffix)
	if goos == "darwin" {
		if bundle, err := bundleRoot(exe); err == nil {
			parent := filepath.Dir(bundle)
			os.RemoveAll(filepath.Join(parent, macOldName))
			os.RemoveAll(filepath.Join(parent, macStageDir))
		}
	}
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	return writeFile(dst, in, mode)
}

func writeFile(dst string, r io.Reader, mode os.FileMode) error {
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, r); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// extractTarGz writes the single file called name from a .tar.gz to dst.
func extractTarGz(src, name, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%s not found in update package", name)
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeReg && filepath.Base(h.Name) == name {
			return writeFile(dst, tr, 0o755)
		}
	}
}

// extractZip unpacks a zip (as made by `ditto -c -k`) into dir, keeping
// file modes and symlinks and rejecting paths that escape dir.
func extractZip(src, dir string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, "__MACOSX/") {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(f.Name))
		if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path %q in update package", f.Name)
		}
		mode := f.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		if mode&os.ModeSymlink != 0 {
			link, err := io.ReadAll(io.LimitReader(rc, 4096))
			rc.Close()
			if err != nil {
				return err
			}
			resolved := filepath.Join(filepath.Dir(target), string(link))
			if filepath.IsAbs(string(link)) || !strings.HasPrefix(resolved, root+string(os.PathSeparator)) {
				return fmt.Errorf("unsafe symlink %q in update package", f.Name)
			}
			if err := os.Symlink(string(link), target); err != nil {
				return err
			}
			continue
		}
		perm := mode.Perm()
		if perm == 0 {
			perm = 0o644
		}
		err = writeFile(target, rc, perm)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
