// Package version holds build-time identity for L5S1.
// Values are injected via -ldflags at release builds.
package version

// Version is the semver-ish product version (e.g. 0.0.1-beta.14).
var Version = "0.0.1-beta.14"

// Commit is the short git SHA when built from CI; empty for local.
var Commit = "dev"

// BuildTime is RFC3339 UTC when the binary was built.
var BuildTime = "unknown"

// String returns a human-readable version line.
func String() string {
	if Commit == "" || Commit == "dev" {
		return "v" + Version
	}
	return "v" + Version + "+" + Commit
}
