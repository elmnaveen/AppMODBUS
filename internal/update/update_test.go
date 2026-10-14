package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionOrdering(t *testing.T) {
	// Ascending order from the semver 2.0 spec plus our own cases.
	order := []string{
		"0.9.9", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "v1.0.1", "1.10.0",
	}
	for i := 0; i+1 < len(order); i++ {
		a, _ := ParseVersion(order[i])
		b, err := ParseVersion(order[i+1])
		if err != nil {
			t.Fatal(err)
		}
		if a.Compare(b) != -1 || b.Compare(a) != 1 {
			t.Errorf("%s should be < %s", order[i], order[i+1])
		}
	}
	if v, _ := ParseVersion("v2.3.4+build.7"); v.String() != "2.3.4" {
		t.Errorf("build metadata: %s", v)
	}
	for _, bad := range []string{"", "1.2", "1.2.x", "1.2.3-", "a.b.c"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("%q should not parse", bad)
		}
	}
}

// fakeRelease is a release served by the fake GitHub.
type fakeRelease struct {
	tag        string
	prerelease bool
	draft      bool
	files      map[string][]byte // asset name -> content
}

type fixture struct {
	srv  *httptest.Server
	pub  string
	priv ed25519.PrivateKey
}

// signedFiles builds package + checksums + signature for a release.
func (fx *fixture) signedFiles(pkgName string, pkg []byte) map[string][]byte {
	sum := sha256.Sum256(pkg)
	sums := []byte(fmt.Sprintf("%s  %s\n%s  other.zip\n", hex.EncodeToString(sum[:]), pkgName, strings.Repeat("0", 64)))
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(fx.priv, sums))
	return map[string][]byte{pkgName: pkg, ChecksumsFile: sums, SignatureFile: []byte(sig + "\n")}
}

func newFixture(t *testing.T, releases func(fx *fixture) []fakeRelease) *fixture {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	fx := &fixture{pub: base64.StdEncoding.EncodeToString(pub), priv: priv}
	mux := http.NewServeMux()
	fx.srv = httptest.NewServer(mux)
	t.Cleanup(fx.srv.Close)
	rels := releases(fx)
	mux.HandleFunc("/repos/acme/app/releases", func(w http.ResponseWriter, r *http.Request) {
		var out []map[string]any
		for _, rel := range rels {
			var assets []map[string]any
			for name, data := range rel.files {
				assets = append(assets, map[string]any{
					"name": name, "size": len(data),
					"browser_download_url": fx.srv.URL + "/dl/" + rel.tag + "/" + name,
				})
			}
			out = append(out, map[string]any{
				"tag_name": rel.tag, "name": "Release " + rel.tag, "body": "notes for " + rel.tag,
				"html_url": "https://example/" + rel.tag, "draft": rel.draft, "prerelease": rel.prerelease,
				"assets": assets,
			})
		}
		json.NewEncoder(w).Encode(out)
	})
	mux.HandleFunc("/dl/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/dl/"), "/", 2)
		for _, rel := range rels {
			if rel.tag == parts[0] {
				if data, ok := rel.files[parts[1]]; ok {
					w.Write(data)
					return
				}
			}
		}
		http.NotFound(w, r)
	})
	return fx
}

func (fx *fixture) updater(current string) *Updater {
	return &Updater{APIBase: fx.srv.URL, Repo: "acme/app", Current: current, PublicKey: fx.pub, GOOS: "linux", GOARCH: "amd64"}
}

const linuxPkg = "ModbusStudio-linux-amd64.tar.gz"

func TestCheckChannels(t *testing.T) {
	fx := newFixture(t, func(fx *fixture) []fakeRelease {
		return []fakeRelease{
			{tag: "v0.3.0", draft: true, files: fx.signedFiles(linuxPkg, []byte("draft"))},
			{tag: "v0.2.1-beta.1", prerelease: true, files: fx.signedFiles(linuxPkg, []byte("beta"))},
			{tag: "v0.2.0", files: fx.signedFiles(linuxPkg, []byte("stable"))},
			{tag: "v0.2.5", files: map[string][]byte{linuxPkg: []byte("unsigned")}}, // incomplete
			{tag: "v0.1.0", files: fx.signedFiles(linuxPkg, []byte("old"))},
		}
	})
	ctx := context.Background()
	rel, err := fx.updater("0.1.0").Check(ctx, Stable)
	if err != nil || rel == nil || rel.Version != "0.2.0" {
		t.Fatalf("stable: %+v %v", rel, err)
	}
	rel, err = fx.updater("0.1.0").Check(ctx, Beta)
	if err != nil || rel == nil || rel.Version != "0.2.1-beta.1" || !rel.Prerelease {
		t.Fatalf("beta: %+v %v", rel, err)
	}
	rel, err = fx.updater("0.2.0").Check(ctx, Stable)
	if err != nil || rel != nil {
		t.Fatalf("up to date: %+v %v", rel, err)
	}
	// A beta user on a newer beta is not offered the older stable.
	rel, _ = fx.updater("0.2.1-beta.1").Check(ctx, Beta)
	if rel != nil {
		t.Fatalf("expected none, got %s", rel.Version)
	}
	u := fx.updater("0.1.0")
	u.GOOS, u.GOARCH = "linux", "arm64"
	if _, err := u.Check(ctx, Stable); err == nil {
		t.Fatal("linux/arm64 has no package")
	}
}

func TestDownloadVerifies(t *testing.T) {
	var good, tampered *fixture
	good = newFixture(t, func(fx *fixture) []fakeRelease {
		return []fakeRelease{{tag: "v1.0.0", files: fx.signedFiles(linuxPkg, []byte("real package"))}}
	})
	tampered = newFixture(t, func(fx *fixture) []fakeRelease {
		files := fx.signedFiles(linuxPkg, []byte("real package"))
		files[linuxPkg] = []byte("malware") // checksum no longer matches
		return []fakeRelease{{tag: "v1.0.0", files: files}}
	})
	ctx := context.Background()
	dir := t.TempDir()

	u := good.updater("0.9.0")
	rel, err := u.Check(ctx, Stable)
	if err != nil || rel == nil {
		t.Fatal(rel, err)
	}
	var last int64
	path, err := u.Download(ctx, rel, rel.Asset, dir, func(done, total int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "real package" || last != int64(len(b)) {
		t.Fatalf("content %q progress %d", b, last)
	}

	// Signed by a different key: rejected before downloading the package.
	other := good.updater("0.9.0")
	other.PublicKey = tampered.pub
	if _, err := other.Download(ctx, rel, rel.Asset, dir, nil); err == nil || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("wrong key: %v", err)
	}

	// Valid signature but altered package: hash mismatch.
	tu := tampered.updater("0.9.0")
	trel, _ := tu.Check(ctx, Stable)
	if _, err := tu.Download(ctx, trel, trel.Asset, dir, nil); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("tampered: %v", err)
	}

	// No key in the build: installs are refused.
	nokey := good.updater("0.9.0")
	nokey.PublicKey = ""
	if nokey.CanInstall() {
		t.Fatal("CanInstall without key")
	}
	if _, err := nokey.Download(ctx, rel, rel.Asset, dir, nil); err == nil {
		t.Fatal("expected refusal without key")
	}
}

func tarGz(t *testing.T, name string, data []byte) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg})
	tw.Write(data)
	tw.Close()
	gz.Close()
	p := filepath.Join(t.TempDir(), "pkg.tar.gz")
	os.WriteFile(p, buf.Bytes(), 0o644)
	return p
}

func TestInstallLinuxBinary(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "ModbusStudio")
	os.WriteFile(exe, []byte("v1"), 0o755)
	l, err := Install("linux", tarGz(t, "ModbusStudio", []byte("v2")), exe)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "v2" || l.Path != exe {
		t.Fatalf("got %q launch %+v", b, l)
	}
	if fi, _ := os.Stat(exe); fi.Mode().Perm()&0o100 == 0 {
		t.Fatal("not executable")
	}
	if _, err := os.Stat(exe + ".old"); err != nil {
		t.Fatal("old binary should be kept until cleanup")
	}
	Cleanup("linux", exe)
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Fatal("cleanup did not remove .old")
	}

	// A package without the binary leaves the install untouched.
	if _, err := Install("linux", tarGz(t, "README", []byte("x")), exe); err == nil {
		t.Fatal("expected error")
	}
	if b, _ := os.ReadFile(exe); string(b) != "v2" {
		t.Fatalf("install damaged: %q", b)
	}
}

func TestInstallWindowsBinary(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "ModbusStudio.exe")
	os.WriteFile(exe, []byte("old"), 0o755)
	pkg := filepath.Join(t.TempDir(), "ModbusStudio-windows-amd64.exe")
	os.WriteFile(pkg, []byte("new"), 0o644)
	if _, err := Install("windows", pkg, exe); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new" {
		t.Fatalf("got %q", b)
	}
}

func makeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0o755)
		w, _ := zw.CreateHeader(h)
		w.Write([]byte(body))
	}
	zw.Close()
	p := filepath.Join(t.TempDir(), "pkg.zip")
	os.WriteFile(p, buf.Bytes(), 0o644)
	return p
}

func TestInstallMacBundle(t *testing.T) {
	apps := t.TempDir()
	exe := filepath.Join(apps, "ModbusStudio.app", "Contents", "MacOS", "ModbusStudio")
	os.MkdirAll(filepath.Dir(exe), 0o755)
	os.WriteFile(exe, []byte("old"), 0o755)
	pkg := makeZip(t, map[string]string{
		"ModbusStudio.app/Contents/MacOS/ModbusStudio": "new",
		"ModbusStudio.app/Contents/Info.plist":         "<plist/>",
	})
	l, err := Install("darwin", pkg, exe)
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exe); string(b) != "new" {
		t.Fatalf("got %q", b)
	}
	if l.Path != "open" || l.Args[1] != filepath.Join(apps, "ModbusStudio.app") {
		t.Fatalf("launch %+v", l)
	}
	Cleanup("darwin", exe)
	entries, _ := os.ReadDir(apps)
	if len(entries) != 1 || entries[0].Name() != "ModbusStudio.app" {
		t.Fatalf("leftovers: %v", entries)
	}
}

func TestZipSlipRejected(t *testing.T) {
	pkg := makeZip(t, map[string]string{"../evil": "x"})
	if err := extractZip(pkg, t.TempDir()); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("expected unsafe path error, got %v", err)
	}
}

func TestNotWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "ModbusStudio")
	os.WriteFile(exe, []byte("v1"), 0o755)
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)
	if _, err := Install("linux", tarGz(t, "ModbusStudio", []byte("v2")), exe); err != ErrNotWritable {
		t.Fatalf("got %v", err)
	}
}

func TestLinuxFlavors(t *testing.T) {
	for flavor, want := range map[LinuxFlavor]string{LinuxPortable: linuxPkg, LinuxDeb: DebAsset, LinuxDebLegacy: DebLegacyAsset} {
		if got, _, err := PackageName("linux", "amd64", flavor); err != nil || got != want {
			t.Errorf("flavor %d: %s %v", flavor, got, err)
		}
	}
	fx := newFixture(t, func(fx *fixture) []fakeRelease {
		files := fx.signedFiles(DebLegacyAsset, []byte("deb"))
		for k, v := range fx.signedFiles(linuxPkg, []byte("tgz")) {
			if k == linuxPkg {
				files[k] = v
			}
		}
		return []fakeRelease{{tag: "v1.0.0", files: files}}
	})
	u := fx.updater("0.9.0")
	u.Linux = LinuxDebLegacy
	rel, err := u.Check(context.Background(), Stable)
	if err != nil || rel == nil || rel.Asset.Name != DebLegacyAsset {
		t.Fatalf("deb check %+v %v", rel, err)
	}
	// Only the portable install is offered when its flavor's file is missing.
	u.Linux = LinuxDeb
	if rel, _ := u.Check(context.Background(), Stable); rel != nil {
		t.Fatalf("expected no update without %s, got %+v", DebAsset, rel.Asset)
	}
	if DetectLinuxFlavor("/home/me/ModbusStudio", true) != LinuxPortable {
		t.Fatal("a copy outside /opt is portable")
	}
}

func TestInstallDebUsesPkexec(t *testing.T) {
	l, err := Install("linux", "/tmp/x/ModbusStudio-ubuntu-24.04-amd64.deb", DebInstallPath)
	if err != nil || l.Path != "sh" || len(l.Args) != 4 || !strings.Contains(l.Args[1], "pkexec dpkg -i") ||
		l.Args[2] != "/tmp/x/ModbusStudio-ubuntu-24.04-amd64.deb" || l.Args[3] != DebInstallPath {
		t.Fatalf("%+v %v", l, err)
	}
}

// A release that only has the pre-rename AppMODBUS- package is not offered.
func TestAssetNames(t *testing.T) {
	fx := newFixture(t, func(fx *fixture) []fakeRelease {
		return []fakeRelease{{tag: "v1.1.0", files: fx.signedFiles("AppMODBUS-linux-amd64.tar.gz", []byte("tgz"))}}
	})
	if rel, err := fx.updater("1.0.0").Check(context.Background(), Stable); err != nil || rel != nil {
		t.Fatalf("check %+v %v", rel, err)
	}
	for _, goos := range []string{"windows", "darwin", "linux"} {
		pkg, inst, err := PackageName(goos, "amd64", LinuxPortable)
		if err != nil || !strings.HasPrefix(pkg, AssetPrefix) || (inst != "" && !strings.HasPrefix(inst, AssetPrefix)) {
			t.Errorf("%s: %q %q %v", goos, pkg, inst, err)
		}
	}
}
