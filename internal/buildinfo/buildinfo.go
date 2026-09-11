// Package buildinfo contains the version displayed by PixSeal.
package buildinfo

const (
	Version    = "0.3.0"
	Prerelease = "build16"
)

// String returns the human-readable version.
func String() string {
	if Prerelease != "" {
		return "v" + Version + "-" + Prerelease
	}
	return "v" + Version
}
