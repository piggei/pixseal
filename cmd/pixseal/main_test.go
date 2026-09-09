package main

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pj/pixseal/watermark"
)

func TestPNGOutputPath(t *testing.T) {
	tests := map[string]string{
		"marked.png":  "marked.png",
		"marked.PNG":  "marked.PNG",
		"marked.jpg":  "marked.png",
		"marked.jpeg": "marked.png",
		"marked":      "marked.png",
		".sealed":     ".sealed.png",
	}
	for input, want := range tests {
		if got := pngOutputPath(input); got != want {
			t.Errorf("pngOutputPath(%q) = %q; want %q", input, got, want)
		}
	}
}

func TestOpenImageRejectsOversizedPNGFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.png")
	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:4], 20_000)
	binary.BigEndian.PutUint32(ihdr[4:8], 20_000) // 400 MP > 300 MP policy
	ihdr[8], ihdr[9], ihdr[10], ihdr[11], ihdr[12] = 8, 2, 0, 0, 0
	data := append([]byte("\x89PNG\r\n\x1a\n"), 0, 0, 0, 13)
	data = append(data, []byte("IHDR")...)
	data = append(data, ihdr[:]...)
	crc := crc32.ChecksumIEEE(append([]byte("IHDR"), ihdr[:]...))
	var crcBytes [4]byte
	binary.BigEndian.PutUint32(crcBytes[:], crc)
	data = append(data, crcBytes[:]...)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err := openImageWithFormat(path)
	if err == nil || !strings.Contains(err.Error(), "maximum decoded input") {
		t.Fatalf("oversized DecodeConfig guard error=%v", err)
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
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("forced replacement permissions=%v err=%v; want 0600", info, err)
	}
}

func TestWritePNGAtomicRejectsDirectoryAndSymlinkTargets(t *testing.T) {
	dir := t.TempDir()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))

	directoryTarget := filepath.Join(dir, "target.png")
	if err := os.Mkdir(directoryTarget, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writePNGAtomic(directoryTarget, img, true); err == nil {
		t.Fatal("writePNGAtomic replaced a directory with -force")
	}
	if info, err := os.Stat(directoryTarget); err != nil || !info.IsDir() {
		t.Fatalf("directory target changed after rejected write: info=%v err=%v", info, err)
	}

	if runtime.GOOS == "windows" {
		return
	}
	symlinkTarget := filepath.Join(dir, "dangling.png")
	if err := os.Symlink(filepath.Join(dir, "missing.png"), symlinkTarget); err != nil {
		t.Fatal(err)
	}
	if err := writePNGAtomic(symlinkTarget, img, false); err == nil {
		t.Fatal("writePNGAtomic replaced a dangling symlink without -force")
	}
	if info, err := os.Lstat(symlinkTarget); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink target changed after rejected write: info=%v err=%v", info, err)
	}
}

func TestEmbedRejectsExplicitZeroAndNonFiniteStrength(t *testing.T) {
	base := []string{"-in", "missing.png", "-out", filepath.Join(t.TempDir(), "out.png"), "-key", "12345678", "-message", "hello"}
	for _, value := range []string{"0", "NaN", "+Inf", "-Inf"} {
		args := append(append([]string{}, base...), "-strength", value)
		err := embed(args)
		if err == nil || !strings.Contains(err.Error(), "strength") {
			t.Fatalf("embed -strength %s error = %v", value, err)
		}
	}
}

func TestCommitNoClobberRejectsRacingTarget(t *testing.T) {
	dir := t.TempDir()
	temporary := filepath.Join(dir, "temporary")
	target := filepath.Join(dir, "target.png")
	if err := os.WriteFile(temporary, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("racer"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := commitNoClobber(temporary, target); err == nil {
		t.Fatal("commitNoClobber replaced a racing target")
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "racer" {
		t.Fatalf("racing target changed to %q", got)
	}
}

func TestExtractRawPreservesMultilinePayload(t *testing.T) {
	dir := t.TempDir()
	carrier := filepath.Join(dir, "carrier.png")
	img := image.NewNRGBA(image.Rect(0, 0, 560, 512))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8((x + y) % 256), G: uint8((2*x + y) % 256), B: uint8((x + 2*y) % 256), A: 255})
		}
	}
	message := "line1\nline2"
	marked, err := watermark.Embed(img, []byte(message), []byte("12345678"), watermark.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if err := writePNGAtomic(carrier, marked, false); err != nil {
		t.Fatal(err)
	}
	got := captureStdout(t, func() error { return extract([]string{"-raw", "-in", carrier, "-key", "12345678"}) })
	if got != message {
		t.Fatalf("raw extract=%q, want %q", got, message)
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
