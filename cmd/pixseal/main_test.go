package main

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestPNGOutputPath(t *testing.T) {
	tests := map[string]string{
		"marked.png":  "marked.png",
		"marked.PNG":  "marked.PNG",
		"marked.jpg":  "marked.png",
		"marked.jpeg": "marked.png",
		"marked":      "marked.png",
	}
	for input, want := range tests {
		if got := pngOutputPath(input); got != want {
			t.Errorf("pngOutputPath(%q) = %q; want %q", input, got, want)
		}
	}
}

func TestEmbedRequiresMessage(t *testing.T) {
	err := embed([]string{"-in", "input.png", "-out", "output.png", "-key", "12345678"})
	if err == nil {
		t.Fatal("embed accepted a missing -message")
	}
}

func TestEmbedRejectsPositionalMessage(t *testing.T) {
	err := embed([]string{"-in", "input.png", "-out", "output.png", "-key", "12345678", "message"})
	if err == nil {
		t.Fatal("embed accepted an unlabelled positional message")
	}
}

func TestWritePNGAtomicProtectsExistingOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "carrier.png")
	original := []byte("do not overwrite")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})

	if err := writePNGAtomic(path, img, false); err == nil {
		t.Fatal("writePNGAtomic overwrote an existing file without -force")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatal("existing output changed after rejected write")
	}

	if err := writePNGAtomic(path, img, true); err != nil {
		t.Fatalf("forced replacement failed: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, _, err := image.Decode(f); err != nil {
		t.Fatalf("forced output is not a valid image: %v", err)
	}
}
