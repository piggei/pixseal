package main

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func TestAnalyzeRejectsInvalidPayloadOptions(t *testing.T) {
	if err := analyze([]string{"-in", "carrier.png"}); err == nil {
		t.Fatal("analyze accepted neither -message nor -bytes")
	}
	if err := analyze([]string{"-in", "carrier.png", "-message", "hello", "-bytes", "5"}); err == nil {
		t.Fatal("analyze accepted both -message and -bytes")
	}
	if err := analyze([]string{"-in", "carrier.png", "-bytes", "0"}); err == nil {
		t.Fatal("analyze accepted zero bytes")
	}
}

func TestEmbedRejectsInvalidProfile(t *testing.T) {
	err := embed([]string{
		"-in", "input.png", "-out", "output.png", "-key", "12345678",
		"-message", "hello", "-profile", "unknown",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid profile") {
		t.Fatalf("embed invalid profile error = %v", err)
	}
}

func TestRemovedCompatibilityFlagsAreRejected(t *testing.T) {
	if err := extract([]string{"-in", "carrier.png", "-key", "12345678", "-repetition", "5"}); err == nil {
		t.Fatal("extract still accepts removed -repetition flag")
	}
	if err := extract([]string{"-in", "carrier.png", "-key", "12345678", "-strength", "24"}); err == nil {
		t.Fatal("extract still accepts removed -strength flag")
	}
	if err := capacity([]string{"-in", "carrier.png", "-repetition", "5"}); err == nil {
		t.Fatal("capacity still accepts removed -repetition flag")
	}
}

func TestAnalyzeCapacityAndEmbedAgreeOnProfiles(t *testing.T) {
	dir := t.TempDir()
	carrier := filepath.Join(dir, "carrier.png")
	output := filepath.Join(dir, "sealed.png")
	img := image.NewNRGBA(image.Rect(0, 0, 320, 288))
	for y := 0; y < 288; y++ {
		for x := 0; x < 320; x++ {
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8((x + y) % 256),
				G: uint8((2*x + y) % 256),
				B: uint8((x + 2*y) % 256),
				A: 255,
			})
		}
	}
	if err := writePNGAtomic(carrier, img, true); err != nil {
		t.Fatal(err)
	}

	capacityOutput := captureStdout(t, func() error {
		return capacity([]string{"-in", carrier, "-details"})
	})
	for _, want := range []string{"robust:      16 bytes", "balanced:    32 bytes", "capacity:    64 bytes", "maximum:     64 bytes"} {
		if !strings.Contains(capacityOutput, want) {
			t.Fatalf("capacity output missing %q:\n%s", want, capacityOutput)
		}
	}

	analyzeOutput := captureStdout(t, func() error {
		return analyze([]string{"-in", carrier, "-bytes", "18"})
	})
	if !strings.Contains(analyzeOutput, "Recommended profile:       balanced") ||
		!strings.Contains(analyzeOutput, "Profile capacity:          32 bytes") {
		t.Fatalf("analyze did not recommend balanced/32:\n%s", analyzeOutput)
	}

	embedOutput := captureStdout(t, func() error {
		return embed([]string{
			"-in", carrier, "-out", output, "-key", "12345678",
			"-message", strings.Repeat("x", 18),
		})
	})
	if !strings.Contains(embedOutput, "embedded 18 bytes using profile balanced") {
		t.Fatalf("embed did not auto-select balanced:\n%s", embedOutput)
	}
}

func TestExplicitProfileErrorNamesMinimumCompatibleProfile(t *testing.T) {
	dir := t.TempDir()
	carrier := filepath.Join(dir, "carrier.png")
	img := image.NewNRGBA(image.Rect(0, 0, 320, 288))
	if err := writePNGAtomic(carrier, img, true); err != nil {
		t.Fatal(err)
	}
	err := embed([]string{
		"-in", carrier, "-out", filepath.Join(dir, "out.png"), "-key", "12345678",
		"-message", strings.Repeat("x", 17), "-profile", "robust",
	})
	if err == nil || !strings.Contains(err.Error(), "payload is 17 bytes") ||
		!strings.Contains(err.Error(), "profile robust capacity is 16 bytes") ||
		!strings.Contains(err.Error(), "minimum compatible profile is balanced") {
		t.Fatalf("unexpected explicit-profile error: %v", err)
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	old := os.Stdout
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writeEnd
	callErr := fn()
	_ = writeEnd.Close()
	os.Stdout = old
	data, readErr := io.ReadAll(readEnd)
	_ = readEnd.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if callErr != nil {
		t.Fatalf("captured command failed: %v", callErr)
	}
	return string(data)
}
