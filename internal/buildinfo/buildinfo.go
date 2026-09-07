// Package buildinfo contains the development version displayed by PixSeal.
package buildinfo

import "fmt"

const (
	Version = "0.2.0"
	Build   = 3
)

// String returns the human-readable development version.
func String() string {
	return fmt.Sprintf("v%s build %d", Version, Build)
}
