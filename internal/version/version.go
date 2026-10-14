// Package version holds build metadata injected at link time, e.g.
//
//	-ldflags "-X appmodbus/internal/version.Version=1.2.0"
package version

// Version is the semantic version of this build (without a leading "v").
var Version = "0.0.0-dev"

// Commit is the git commit this build was made from.
var Commit = ""

// UpdateRepo is the GitHub "owner/repo" whose releases provide updates.
var UpdateRepo = "elmnaveen/AppMODBUS"

// PinnedUpdatePublicKey is the base64 Ed25519 public key of the release
// signing key (the UPDATE_SIGNING_KEY Actions secret). It is public and
// safe to commit. CI refuses to build or sign a release if the secret does
// not match it, so a wrong or replaced secret can never ship a build that
// rejects your own updates. Change it only when deliberately rotating the
// key (see docs/UPDATES.md).
const PinnedUpdatePublicKey = "d0b+dzEpjMR/f9fcuDxWZpfsJ01/YYvt2tR+H8K0OpI="

// UpdatePublicKey is the key release checksums must be signed with.
// Builds whose key is empty can check for updates but not install them.
var UpdatePublicKey = PinnedUpdatePublicKey
