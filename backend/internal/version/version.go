// Package version holds build-time identity for L5S1.
// Values are injected via -ldflags at release builds.
package version

// Version is the semver-ish product version (e.g. 0.0.1-beta.18).
var Version = "0.0.1-beta.18"

// Commit is the short git SHA when built from CI; empty for local.
var Commit = "dev"

// BuildTime is RFC3339 UTC when the binary was built.
var BuildTime = "unknown"

// String returns a human-readable version line.
// Examples: "v0.0.1-beta.18" (local) or "v0.0.1-beta.18+167ff35" (CI).
// Version must NOT already include the commit — that produced doubled
// strings like "v0.0.1-beta.18-g167ff35+167ff35".
func String() string {
	if Commit == "" || Commit == "dev" {
		return "v" + Version
	}
	return "v" + Version + "+" + Commit
}
