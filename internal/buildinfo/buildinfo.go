// Package buildinfo contains the version displayed by PixSeal.
package buildinfo

const (
	Version    = "0.2.0"
	Prerelease = "rc4"
)

// String returns the human-readable version.
func String() string {
	if Prerelease != "" {
		return "v" + Version + "-" + Prerelease
	}
	return "v" + Version
}
