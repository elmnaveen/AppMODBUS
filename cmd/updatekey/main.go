// Command updatekey manages the Ed25519 key that signs AppMODBUS releases.
//
//	go run ./cmd/updatekey generate          # print a new key pair
//	go run ./cmd/updatekey pubkey            # public key of $UPDATE_SIGNING_KEY
//	go run ./cmd/updatekey sign FILE         # write FILE.sig using $UPDATE_SIGNING_KEY
//	go run ./cmd/updatekey verify FILE PUB   # check FILE.sig against a public key
//
// The private key is the base64 Ed25519 seed (32 bytes). Keep it only in the
// UPDATE_SIGNING_KEY GitHub Actions secret.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "updatekey:", err)
		os.Exit(1)
	}
}

func privateKey() (ed25519.PrivateKey, error) {
	v := strings.TrimSpace(os.Getenv("UPDATE_SIGNING_KEY"))
	if v == "" {
		return nil, errors.New("UPDATE_SIGNING_KEY is not set")
	}
	seed, err := base64.StdEncoding.DecodeString(v)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, errors.New("UPDATE_SIGNING_KEY must be a base64 32-byte Ed25519 seed")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: updatekey generate | pubkey | sign FILE | verify FILE PUBKEY")
	}
	enc := base64.StdEncoding
	switch args[0] {
	case "generate":
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		fmt.Println("UPDATE_SIGNING_KEY (secret, store as a GitHub Actions secret):")
		fmt.Println(enc.EncodeToString(priv.Seed()))
		fmt.Println()
		fmt.Println("Public key (embedded into builds automatically by CI):")
		fmt.Println(enc.EncodeToString(pub))
	case "pubkey":
		k, err := privateKey()
		if err != nil {
			return err
		}
		fmt.Println(enc.EncodeToString(k.Public().(ed25519.PublicKey)))
	case "sign":
		if len(args) != 2 {
			return errors.New("usage: updatekey sign FILE")
		}
		k, err := privateKey()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		sig := enc.EncodeToString(ed25519.Sign(k, data))
		return os.WriteFile(args[1]+".sig", []byte(sig+"\n"), 0o644)
	case "verify":
		if len(args) != 3 {
			return errors.New("usage: updatekey verify FILE PUBKEY")
		}
		pub, err := enc.DecodeString(args[2])
		if err != nil || len(pub) != ed25519.PublicKeySize {
			return errors.New("invalid public key")
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			return err
		}
		sigText, err := os.ReadFile(args[1] + ".sig")
		if err != nil {
			return err
		}
		sig, err := enc.DecodeString(strings.TrimSpace(string(sigText)))
		if err != nil || !ed25519.Verify(pub, data, sig) {
			return errors.New("signature does NOT verify")
		}
		fmt.Println("signature OK")
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}
