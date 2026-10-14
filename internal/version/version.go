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

// UpdatePublicKey is the base64 Ed25519 key that release checksums must be
// signed with. Builds without it can check for updates but not install them.
var UpdatePublicKey = ""
