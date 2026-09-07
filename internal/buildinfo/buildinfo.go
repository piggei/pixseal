// Package buildinfo contains the release version displayed by PixSeal.
package buildinfo

const Version = "0.1.0"

// String returns the human-readable release version.
func String() string {
	return "v" + Version
}
