package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pj/pixseal/internal/buildinfo"
	"github.com/pj/pixseal/watermark"
)

const maxDecodedPixels int64 = 300_000_000

var usageHeader = fmt.Sprintf(`PixSeal %s - robust image steganography

Usage:
  pixseal <command> [options]

Commands:
  embed      Hide an authenticated message in an image
  extract    Recover and authenticate a hidden message with bounded geometric recovery
  capacity   Show the usable payload capacity of an image
  analyze    Recommend a v3 profile and embedding settings

Supported image formats:
  Input       PNG (.png), JPEG (.jpg, .jpeg)
  Output      PNG (.png) only

Embed options:
  -in FILE           Input JPEG or PNG (required)
  -out FILE          Output PNG (required)
  -key TEXT          Secret key, minimum 8 bytes (required)
  -message TEXT      Message to hide (required)
  -profile NAME      auto, robust, balanced or capacity (default auto)
  -strength N        DCT embedding strength, 4 to 120 (default 24)
  -force             Allow replacing an existing output file

Extract options:
  -in FILE           Carrier JPEG or PNG (required)
  -key TEXT          Secret key, minimum 8 bytes (required)
  -raw               Write only exact payload bytes to stdout; diagnostics go to stderr

Capacity options:
  -in FILE           Input JPEG or PNG (required)
  -details           Show per-profile capacities and image dimensions

Analyze options:
  -in FILE           Input JPEG or PNG (required)
  -message TEXT      Message whose UTF-8 byte length should be analyzed
  -bytes N           Payload byte count to analyze instead of -message

Run "pixseal <command> -help" to show the options for a command.

Examples:
  pixseal embed -in photo.png -out sealed.png -key "a long secret" -message "hello"
  pixseal extract -in sealed.png -key "a long secret"
  pixseal capacity -in photo.png -details
  pixseal analyze -in photo.png -message "hidden message"
`, buildinfo.String())

func main() {
	if len(os.Args) < 2 {
		rootUsage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "embed":
		err = embed(os.Args[2:])
	case "extract":
		err = extract(os.Args[2:])
	case "capacity":
		err = capacity(os.Args[2:])
	case "analyze":
		err = analyze(os.Args[2:])
	case "help", "-help", "--help", "-h":
		rootUsage()
		return
	default:
		if len(os.Args[1]) > 0 && os.Args[1][0] == '-' {
			fmt.Fprintf(os.Stderr, "Missing command before option %q; did you mean \"pixseal embed %s\"?\n\n", os.Args[1], os.Args[1])
		} else {
			fmt.Fprintf(os.Stderr, "Unknown command %q.\n\n", os.Args[1])
		}
		rootUsage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func rootUsage() {
	fmt.Fprint(os.Stderr, usageHeader)
}

func newFlagSet(command, summary string) *flag.FlagSet {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: pixseal %s [options]\n\n%s\n\nOptions:\n", command, summary)
		fs.PrintDefaults()
	}
	return fs
}

func validateDecodedDimensions(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid image dimensions %dx%d", width, height)
	}
	pixels := int64(width) * int64(height)
	if pixels > maxDecodedPixels {
		return fmt.Errorf("image has %.1f MP; maximum decoded input is %.1f MP", float64(pixels)/1_000_000, float64(maxDecodedPixels)/1_000_000)
	}
	return nil
}

func openImageWithFormat(path string) (image.Image, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	config, configFormat, err := image.DecodeConfig(f)
	if err != nil {
		return nil, "", err
	}
	if err := validateDecodedDimensions(config.Width, config.Height); err != nil {
		return nil, "", err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, "", err
	}
	img, format, err := image.Decode(f)
	if err != nil {
		return nil, "", err
	}
	if format == "" {
		format = configFormat
	}
	return img, format, nil
}

func openImage(path string) (image.Image, error) {
	img, _, err := openImageWithFormat(path)
	return img, err
}

func pngOutputPath(path string) string {
	ext := filepath.Ext(path)
	if strings.EqualFold(ext, ".png") {
		return path
	}
	// filepath.Ext(".sealed") returns ".sealed". A basename consisting only
	// of a leading-dot name has no user-visible extension and should become
	// ".sealed.png", not ".png".
	if ext == "" || ext == filepath.Base(path) {
		return path + ".png"
	}
	return strings.TrimSuffix(path, ext) + ".png"
}

func validateOutputTarget(path string, force bool) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !force {
		return nil, fmt.Errorf("output path %s already exists; use -force to replace a regular file", path)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to replace non-regular output path %s", path)
	}
	return info, nil
}

func commitNoClobber(temporaryPath, path string) error {
	if runtime.GOOS == "windows" {
		// Windows os.Rename does not replace an existing destination. After the
		// Lstat preflight this is therefore an atomic no-clobber commit: a target
		// created by a racing process causes Rename to fail.
		if err := os.Rename(temporaryPath, path); err != nil {
			if _, existsErr := os.Lstat(path); existsErr == nil {
				return fmt.Errorf("output path %s already exists; use -force to replace a regular file", path)
			}
			return err
		}
		return nil
	}

	// On POSIX, rename(2) would replace a racing destination. link(2) instead
	// creates the destination only if no directory entry already exists, while
	// keeping the fully encoded temporary inode intact until commit succeeds.
	if err := os.Link(temporaryPath, path); err != nil {
		if _, existsErr := os.Lstat(path); existsErr == nil {
			return fmt.Errorf("output path %s already exists; use -force to replace a regular file", path)
		}
		return fmt.Errorf("atomic no-clobber output commit failed: %w", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		return fmt.Errorf("remove committed temporary link: %w", err)
	}
	return nil
}

func writePNGAtomic(path string, img image.Image, force bool) error {
	initialInfo, err := validateOutputTarget(path, force)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	temporary, err := os.CreateTemp(dir, "."+base+".pixseal-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	// New files intentionally keep CreateTemp's private 0600-style mode, which
	// never weakens the caller's umask. Replacements preserve the permissions of
	// the regular file being replaced.
	if initialInfo != nil {
		if err := temporary.Chmod(initialInfo.Mode().Perm()); err != nil {
			_ = temporary.Close()
			return err
		}
	}
	if err := png.Encode(temporary, img); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}

	if initialInfo == nil {
		if err := commitNoClobber(temporaryPath, path); err != nil {
			return err
		}
		committed = true
		return nil
	}

	// Re-check immediately before a forced replacement. This cannot make a
	// forced replace fully race-free on every OS using only portable stdlib
	// primitives, but it prevents PixSeal from intentionally replacing a
	// directory, symlink, FIFO or device.
	current, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("re-check output before replacement: %w", err)
	}
	if current.Mode()&os.ModeSymlink != 0 || !current.Mode().IsRegular() {
		return fmt.Errorf("refusing to replace non-regular output path %s", path)
	}
	if err := os.Chmod(temporaryPath, current.Mode().Perm()); err != nil {
		return err
	}

	if err := os.Rename(temporaryPath, path); err == nil {
		committed = true
		return nil
	} else if runtime.GOOS != "windows" {
		return err
	}

	// Windows does not replace an existing file with os.Rename. Keep this
	// backup-and-restore fallback Windows-only. The encoded temporary file has
	// already been fsync'd; a crash between the two renames can still leave the
	// backup path behind, so this is atomic replacement in the logical/no-partial
	// sense, not a filesystem durability guarantee.
	backup, err := os.CreateTemp(dir, "."+base+".backup-*")
	if err != nil {
		return err
	}
	backupPath := backup.Name()
	if err := backup.Close(); err != nil {
		_ = os.Remove(backupPath)
		return err
	}
	if err := os.Remove(backupPath); err != nil {
		return err
	}
	if err := os.Rename(path, backupPath); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		_ = os.Rename(backupPath, path)
		return err
	}
	committed = true
	_ = os.Remove(backupPath)
	return nil
}

func embed(args []string) error {
	fs := newFlagSet("embed", "Hide an authenticated message; the output image is always PNG.")
	in := fs.String("in", "", "input JPEG or PNG file (required)")
	out := fs.String("out", "", "output PNG file (required)")
	message := fs.String("message", "", "message to hide, up to 64 bytes (required)")
	profileName := fs.String("profile", "auto", "v3 profile: auto, robust, balanced or capacity")
	force := fs.Bool("force", false, "replace an existing output file")
	key := fs.String("key", "", "secret key (required, minimum 8 bytes)")
	strength := fs.Float64("strength", 24, "DCT embedding strength from 4 to 120")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("unexpected positional argument %q; use -message \"text\"", fs.Arg(0))
	}
	if *in == "" || *out == "" || *key == "" || *message == "" {
		fs.Usage()
		return fmt.Errorf("-in, -out, -key and -message are required")
	}
	strengthExplicit := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "strength" {
			strengthExplicit = true
		}
	})
	if math.IsNaN(*strength) || math.IsInf(*strength, 0) {
		return fmt.Errorf("-strength must be a finite number between 4 and 120")
	}
	if strengthExplicit && *strength == 0 {
		return fmt.Errorf("-strength 0 is invalid; omit -strength to use the default value 24")
	}

	outputPath := pngOutputPath(*out)
	if _, err := validateOutputTarget(outputPath, *force); err != nil {
		return err
	}

	profile, err := watermark.ParseProfile(*profileName)
	if err != nil {
		return err
	}
	img, err := openImage(*in)
	if err != nil {
		return err
	}
	marked, embedInfo, err := watermark.EmbedWithInfo(img, []byte(*message), []byte(*key), watermark.Options{
		Strength: *strength,
		Profile:  profile,
	})
	if err != nil {
		return err
	}

	if err := writePNGAtomic(outputPath, marked, *force); err != nil {
		return err
	}
	if outputPath != *out {
		fmt.Printf("output renamed to %s (PixSeal output is PNG)\n", outputPath)
	}
	fmt.Printf("embedded %d bytes using profile %s in %s\n", len([]byte(*message)), embedInfo.Profile, outputPath)
	return nil
}

func extract(args []string) error {
	fs := newFlagSet("extract", "Recover and authenticate a hidden PixSeal message; bounded rotation and supported combined geometry correction are automatic.")
	in := fs.String("in", "", "carrier JPEG or PNG file (required)")
	key := fs.String("key", "", "secret key (required, minimum 8 bytes)")
	raw := fs.Bool("raw", false, "write only the exact recovered payload bytes to stdout")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("unexpected positional argument %q", fs.Arg(0))
	}
	if *in == "" || *key == "" {
		fs.Usage()
		return fmt.Errorf("-in and -key are required")
	}

	img, err := openImage(*in)
	if err != nil {
		return err
	}
	payload, info, err := watermark.ExtractWithInfo(img, []byte(*key))
	if err != nil {
		return err
	}
	if *raw {
		if _, err := os.Stdout.Write(payload); err != nil {
			return err
		}
		printExtractDiagnostics(os.Stderr, info)
		return nil
	}
	fmt.Printf("%s\n", payload)
	printExtractDiagnostics(os.Stdout, info)
	return nil
}

func printExtractDiagnostics(w io.Writer, info watermark.ExtractInfo) {
	fmt.Fprintf(w, "confidence-margin: %.2f\nprofile: %s\n", info.Confidence, info.Profile)
	if info.RotationCorrectionDegrees != 0 {
		fmt.Fprintf(w, "rotation-correction: %.2f degrees\n", info.RotationCorrectionDegrees)
	}
	if info.ScaleXCorrection != 0 || info.ScaleYCorrection != 0 {
		fmt.Fprintf(w, "scale-correction: x=%.4f y=%.4f\n", info.ScaleXCorrection, info.ScaleYCorrection)
	}
	if info.ShearXCorrection != 0 {
		fmt.Fprintf(w, "shear-x-correction: %.2f degrees\n", math.Atan(info.ShearXCorrection)*180/math.Pi)
	}
	if info.ShearYCorrection != 0 {
		fmt.Fprintf(w, "shear-y-correction: %.2f degrees\n", math.Atan(info.ShearYCorrection)*180/math.Pi)
	}
	if info.PerspectiveCorrection != "" {
		fmt.Fprintf(w, "perspective-correction: %s\n", info.PerspectiveCorrection)
	}
}

func capacity(args []string) error {
	fs := newFlagSet("capacity", "Show usable v3 payload capacity in bytes.")
	in := fs.String("in", "", "input JPEG or PNG file (required)")
	details := fs.Bool("details", false, "show per-profile capacities and image dimensions")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("unexpected positional argument %q", fs.Arg(0))
	}
	if *in == "" {
		fs.Usage()
		return fmt.Errorf("-in is required")
	}

	img, format, err := openImageWithFormat(*in)
	if err != nil {
		return err
	}
	maximum := watermark.Capacity(img)
	if !*details {
		fmt.Printf("%d bytes\n", maximum)
		return nil
	}

	fmt.Printf("Image:       %s\n", *in)
	fmt.Printf("Format:      %s\n", strings.ToUpper(format))
	fmt.Printf("Dimensions:  %d x %d\n", img.Bounds().Dx(), img.Bounds().Dy())
	for _, profile := range watermark.Profiles() {
		value := profile.MaximumPayload
		if maximum == 0 {
			value = 0
		}
		fmt.Printf("%-12s %d bytes\n", string(profile.Name)+":", value)
	}
	fmt.Printf("maximum:     %d bytes\n", maximum)
	return nil
}

func analyze(args []string) error {
	fs := newFlagSet("analyze", "Analyze carrier geometry and recommend a v3 adaptive profile. Recommendations are advisory.")
	in := fs.String("in", "", "input JPEG or PNG file (required)")
	message := fs.String("message", "", "message whose UTF-8 byte length should be analyzed")
	payloadBytes := fs.Int("bytes", -1, "payload byte count to analyze instead of -message")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return fmt.Errorf("unexpected positional argument %q", fs.Arg(0))
	}
	if *in == "" {
		fs.Usage()
		return fmt.Errorf("-in is required")
	}
	hasMessage := *message != ""
	hasBytes := *payloadBytes >= 0
	if hasMessage == hasBytes {
		fs.Usage()
		return fmt.Errorf("exactly one of -message or -bytes is required")
	}
	requested := *payloadBytes
	if hasMessage {
		requested = len([]byte(*message))
	}
	if requested <= 0 {
		return fmt.Errorf("requested payload must be at least 1 byte")
	}

	img, format, err := openImageWithFormat(*in)
	if err != nil {
		return err
	}
	result := watermark.AnalyzeImage(img, requested)

	profile := "none"
	if result.RecommendedProfile != "" {
		profile = string(result.RecommendedProfile)
	}
	fmt.Printf("Image:                     %s\n", *in)
	fmt.Printf("Format:                    %s\n", strings.ToUpper(format))
	fmt.Printf("Dimensions:                %d x %d\n", result.Width, result.Height)
	fmt.Printf("Requested payload:         %d bytes\n", result.RequestedBytes)
	fmt.Printf("Recommended profile:       %s  [deterministic]\n", profile)
	fmt.Printf("Profile capacity:          %d bytes  [deterministic]\n", result.ProfileCapacity)
	fmt.Printf("Tile redundancy:           %.2fx  [deterministic]\n", result.ProfileTileRedundancy)
	fmt.Printf("Average observations/bit:  %.2fx  [deterministic, untransformed carrier]\n", result.AverageObservations)
	fmt.Printf("Image detail:              %s (score %.2f)  [heuristic]\n", result.Detail, result.DetailScore)
	fmt.Printf("Recommended strength:      %.0f  [heuristic]\n", result.RecommendedStrength)
	fmt.Printf("Status:                    %s\n", result.Status)
	for _, warning := range result.Warnings {
		fmt.Printf("Warning:                   %s\n", warning)
	}
	fmt.Println("Note:                      robustness results are experimental; this analysis is not a recovery guarantee")
	return nil
}
