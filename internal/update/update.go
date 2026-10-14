package update

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Channel selects which releases are offered.
type Channel string

const (
	Stable Channel = "stable"
	Beta   Channel = "beta" // stable releases plus GitHub pre-releases
)

// Release files every published release must contain.
const (
	ChecksumsFile = "checksums.txt"
	SignatureFile = "checksums.txt.sig"
)

// Asset is a downloadable release file.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// Release is an available update.
type Release struct {
	Version     string `json:"version"`
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	Notes       string `json:"notes"`
	PageURL     string `json:"pageUrl"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"publishedAt"`
	Asset       Asset  `json:"asset"`     // package for this platform
	Installer   *Asset `json:"installer"` // Windows installer fallback

	checksums Asset
	signature Asset
}

// Updater talks to GitHub and verifies downloads.
type Updater struct {
	HTTP      *http.Client
	APIBase   string // default https://api.github.com
	Repo      string // "owner/repo"
	Current   string // running version
	PublicKey string // base64 Ed25519 public key; empty disables installs
	GOOS      string // defaults to runtime.GOOS (overridable for tests)
	GOARCH    string
}

func (u *Updater) client() *http.Client {
	if u.HTTP != nil {
		return u.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func (u *Updater) platform() (string, string) {
	goos, arch := u.GOOS, u.GOARCH
	if goos == "" {
		goos = runtime.GOOS
	}
	if arch == "" {
		arch = runtime.GOARCH
	}
	return goos, arch
}

// PackageName returns the release asset holding the app for a platform.
func PackageName(goos, arch string) (pkg, installer string, err error) {
	switch goos {
	case "windows":
		if arch == "amd64" {
			return "AppMODBUS-windows-amd64.exe", "AppMODBUS-windows-amd64-installer.exe", nil
		}
	case "darwin":
		return "AppMODBUS-macos-universal.zip", "", nil
	case "linux":
		if arch == "amd64" {
			return "AppMODBUS-linux-amd64.tar.gz", "", nil
		}
	}
	return "", "", fmt.Errorf("no update package is published for %s/%s", goos, arch)
}

// CanInstall reports whether this build can verify (and so install) updates.
func (u *Updater) CanInstall() bool { _, err := u.publicKey(); return err == nil }

func (u *Updater) publicKey() (ed25519.PublicKey, error) {
	if u.PublicKey == "" {
		return nil, errors.New("this build has no update signing key, so updates must be installed manually")
	}
	k, err := base64.StdEncoding.DecodeString(u.PublicKey)
	if err != nil || len(k) != ed25519.PublicKeySize {
		return nil, errors.New("invalid update signing key in this build")
	}
	return k, nil
}

type ghRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	HTMLURL     string `json:"html_url"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// Check returns the newest release newer than the running version on the
// given channel, or nil when the app is up to date.
func (u *Updater) Check(ctx context.Context, ch Channel) (*Release, error) {
	cur, err := ParseVersion(u.Current)
	if err != nil {
		return nil, fmt.Errorf("running version: %w", err)
	}
	pkgName, instName, err := PackageName(u.platform())
	if err != nil {
		return nil, err
	}
	base := u.APIBase
	if base == "" {
		base = "https://api.github.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/releases?per_page=30", base, u.Repo), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "AppMODBUS/"+u.Current)
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("no releases found for %s (is the repository public?)", u.Repo)
	case resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests:
		return nil, errors.New("GitHub rate limit reached; try again later")
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("GitHub returned %s", resp.Status)
	}
	var list []ghRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&list); err != nil {
		return nil, fmt.Errorf("invalid release list: %w", err)
	}

	var best *Release
	var bestV Version
	for _, r := range list {
		if r.Draft || (r.Prerelease && ch != Beta) {
			continue
		}
		v, err := ParseVersion(r.TagName)
		if err != nil || v.Compare(cur) <= 0 || (v.IsPrerelease() && ch != Beta) {
			continue
		}
		if best != nil && v.Compare(bestV) <= 0 {
			continue
		}
		rel := &Release{
			Version: v.String(), Tag: r.TagName, Name: r.Name, Notes: r.Body, PageURL: r.HTMLURL,
			Prerelease: r.Prerelease || v.IsPrerelease(), PublishedAt: r.PublishedAt,
		}
		found := 0
		for _, a := range r.Assets {
			asset := Asset{Name: a.Name, URL: a.URL, Size: a.Size}
			switch a.Name {
			case pkgName:
				rel.Asset = asset
				found++
			case ChecksumsFile:
				rel.checksums = asset
				found++
			case SignatureFile:
				rel.signature = asset
				found++
			case instName:
				ia := asset
				rel.Installer = &ia
			}
		}
		if found != 3 {
			continue // incomplete release (still uploading, or no package for us)
		}
		best, bestV = rel, v
	}
	return best, nil
}

// Progress reports download progress in bytes (total may be 0 if unknown).
type Progress func(done, total int64)

// Download fetches an asset of rel into dir, verifying the signed checksum
// list first and the file's SHA-256 afterwards. It returns the file path.
func (u *Updater) Download(ctx context.Context, rel *Release, asset Asset, dir string, progress Progress) (string, error) {
	key, err := u.publicKey()
	if err != nil {
		return "", err
	}
	if rel.checksums.URL == "" || rel.signature.URL == "" {
		return "", errors.New("release is missing its signed checksums")
	}
	sums, err := u.fetchSmall(ctx, rel.checksums.URL)
	if err != nil {
		return "", fmt.Errorf("download checksums: %w", err)
	}
	sigText, err := u.fetchSmall(ctx, rel.signature.URL)
	if err != nil {
		return "", fmt.Errorf("download signature: %w", err)
	}
	if err := VerifySignature(key, sums, sigText); err != nil {
		return "", err
	}
	want, err := lookupChecksum(sums, asset.Name)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "AppMODBUS/"+u.Current)
	// Large downloads must not hit the short default client timeout.
	c := *u.client()
	c.Timeout = 0
	resp, err := c.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", resp.Status)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(asset.Name))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	total := resp.ContentLength
	if total <= 0 {
		total = asset.Size
	}
	var done int64
	buf := make([]byte, 64<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return "", werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(path)
			return "", rerr
		}
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		os.Remove(path)
		return "", fmt.Errorf("checksum mismatch for %s: the download is corrupted or was tampered with", asset.Name)
	}
	return path, nil
}

func (u *Updater) fetchSmall(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "AppMODBUS/"+u.Current)
	resp, err := u.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// VerifySignature checks a base64 Ed25519 signature over data.
func VerifySignature(key ed25519.PublicKey, data, sigText []byte) error {
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sigText)))
	if err != nil || len(sig) != ed25519.SignatureSize || !ed25519.Verify(key, data, sig) {
		return errors.New("update signature is invalid: refusing to install")
	}
	return nil
}

// lookupChecksum finds name in a sha256sum-style list ("<hex>  <name>").
func lookupChecksum(sums []byte, name string) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(sums))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("%s is not listed in the signed checksums", name)
}
