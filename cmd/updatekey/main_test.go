package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"modbusstudio/internal/version"
)

func TestPinnedKeyIsValid(t *testing.T) {
	k, err := base64.StdEncoding.DecodeString(version.PinnedUpdatePublicKey)
	if err != nil || len(k) != ed25519.PublicKeySize {
		t.Fatalf("pinned key is not a base64 Ed25519 public key: %v", err)
	}
	if version.UpdatePublicKey != version.PinnedUpdatePublicKey {
		t.Fatal("builds must default to the pinned key")
	}
}

func TestWrongSecretIsRefused(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	t.Setenv("UPDATE_SIGNING_KEY", base64.StdEncoding.EncodeToString(priv.Seed()))
	if err := run([]string{"check"}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("check: %v", err)
	}
	f := filepath.Join(t.TempDir(), "checksums.txt")
	os.WriteFile(f, []byte("x"), 0o644)
	if err := run([]string{"sign", f}); err == nil {
		t.Fatal("sign must refuse an unpinned key")
	}
	if _, err := os.Stat(f + ".sig"); err == nil {
		t.Fatal("no signature may be written")
	}
}
