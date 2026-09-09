//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package main

import (
	"image"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestWritePNGAtomicRespectsUmaskForNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "private.png")
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	if err := writePNGAtomic(path, image.NewNRGBA(image.Rect(0, 0, 2, 2)), false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("new output mode = %04o; want 0600 under umask 077", got)
	}
}
