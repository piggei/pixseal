package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
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

const maxCLISourcePixels int64 = 300_000_000

var usageHeader = fmt.Sprintf(`PixSeal %s - robust image steganography

Usage:
  pixseal <command> [options]

Commands:
  embed      Hide an authenticated message in an image
  extract    Recover and authenticate a hidden message with bounded geometric recovery
  capacity   Show the usable payload capacity of an image
  analyze    Recommend a v3 profile and embedding settings
  diagnose   Experimental bounded local-lattice diagnostics (v0.3 research)

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
  -raw               Write only authenticated payload bytes to stdout

Capacity options:
  -in FILE           Input JPEG or PNG (required)
  -details           Show per-profile capacities and image dimensions

Analyze options:
  -in FILE           Input JPEG or PNG (required)
  -message TEXT      Message whose UTF-8 byte length should be analyzed
  -bytes N           Payload byte count to analyze instead of -message

Diagnose options:
  -in FILE           Input JPEG or PNG (required)
  -key TEXT          Optional key for an independent baseline HMAC attempt
  -json              Emit machine-readable JSON
  -regions N         Diagnostic region grid, N x N (default 3, maximum 4)
  -max-dim N         Maximum diagnostic pyramid dimension (default 2048)
  -levels N          Maximum diagnostic pyramid levels (default 2, maximum 3)

Run "pixseal <command> -help" to show the options for a command.

Examples:
  pixseal embed -in photo.png -out sealed.png -key "a long secret" -message "hello"
  pixseal extract -in sealed.png -key "a long secret"
  pixseal capacity -in photo.png -details
  pixseal analyze -in photo.png -message "hidden message"
  pixseal diagnose -in captured.jpg -json
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
	case "diagnose":
		err = diagnose(os.Args[2:])
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
		if errors.Is(err, flag.ErrHelp) {
			return
		}
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

func validateSourceDimensions(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid image dimensions %dx%d", width, height)
	}
	if int64(width) > (1<<63-1)/int64(height) {
		return errors.New("image dimensions overflow the PixSeal pixel-count calculation")
	}
	pixels := int64(width) * int64(height)
	if pixels > maxCLISourcePixels {
		return fmt.Errorf("image has %d pixels; PixSeal CLI safety limit is %d pixels", pixels, maxCLISourcePixels)
	}
	return nil
}

func openImageConfig(path string) (image.Config, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, "", err
	}
	defer f.Close()
	config, format, err := image.DecodeConfig(f)
	if err != nil {
		return image.Config{}, "", err
	}
	if err := validateSourceDimensions(config.Width, config.Height); err != nil {
		return image.Config{}, "", err
	}
	return config, format, nil
}

func openImageWithFormat(path string) (image.Image, string, error) {
	config, format, err := openImageConfig(path)
	if err != nil {
		return nil, "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	img, decodedFormat, err := image.Decode(f)
	if err != nil {
		return nil, "", err
	}
	if decodedFormat != "" {
		format = decodedFormat
	}
	if got := img.Bounds(); got.Dx() != config.Width || got.Dy() != config.Height {
		return nil, "", fmt.Errorf("decoded dimensions %dx%d differ from image config %dx%d", got.Dx(), got.Dy(), config.Width, config.Height)
	}
	return img, format, nil
}

type boundsOnlyImage struct{ rectangle image.Rectangle }

func (img boundsOnlyImage) ColorModel() color.Model { return color.NRGBAModel }
func (img boundsOnlyImage) Bounds() image.Rectangle { return img.rectangle }
func (img boundsOnlyImage) At(x, y int) color.Color { return color.NRGBA{A: 255} }

func openImage(path string) (image.Image, error) {
	img, _, err := openImageWithFormat(path)
	return img, err
}

func pngOutputPath(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	// filepath.Ext(".sealed") is ".sealed". A Unix dotfile with no second dot
	// is a basename, not an extension-only filename.
	if strings.HasPrefix(base, ".") && strings.Count(base, ".") == 1 {
		ext = ""
	}
	if strings.EqualFold(ext, ".png") {
		return path
	}
	if ext == "" {
		return path + ".png"
	}
	return strings.TrimSuffix(path, ext) + ".png"
}

func outputTargetInfo(path string, force bool) (bool, os.FileMode, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, 0, fmt.Errorf("output path %s exists but is not a regular file", path)
	}
	if !force {
		return true, info.Mode().Perm(), fmt.Errorf("output file %s already exists; use -force to replace it", path)
	}
	return true, info.Mode().Perm(), nil
}

func createTempForOutput(dir, base string) (*os.File, string, error) {
	for attempt := 0; attempt < 32; attempt++ {
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, "", err
		}
		path := filepath.Join(dir, "."+base+".pixseal-"+hex.EncodeToString(random[:]))
		// 0666 is intentionally subject to the process umask. This avoids making a
		// newly-created carrier more permissive than the caller requested.
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
		if err == nil {
			return f, path, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("could not create a unique temporary output file")
}

func commitNoClobber(temporaryPath, path string) error {
	// A hard link publishes the already-synced temporary inode atomically and
	// fails if any directory entry already exists at the destination.
	if err := os.Link(temporaryPath, path); err == nil {
		_ = os.Remove(temporaryPath)
		return nil
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("output file %s appeared while writing; refusing to overwrite it", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Some filesystems/platforms do not support hard links. Fall back to an
	// exclusive destination create. This preserves no-clobber semantics, though
	// it is not crash-atomic while bytes are copied.
	src, err := os.Open(temporaryPath)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("output file %s appeared while writing; refusing to overwrite it", path)
		}
		return err
	}
	ok := false
	defer func() {
		_ = dst.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	if err := dst.Sync(); err != nil {
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	ok = true
	_ = os.Remove(temporaryPath)
	return nil
}

func commitForcedRegular(temporaryPath, path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// The original target disappeared during encoding. Do not turn -force into
			// permission to clobber a newly-created path in a race; publish no-clobber.
			return commitNoClobber(temporaryPath, path)
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("output path %s changed and is no longer a regular file", path)
	}
	if err := os.Chmod(temporaryPath, mode); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err == nil {
		return nil
	} else if runtime.GOOS != "windows" {
		return err
	}

	// Windows cannot always replace an existing destination with os.Rename.
	// Restrict the backup-and-restore fallback to Windows and only to a regular
	// file that has just been revalidated above.
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	backup, backupPath, err := createTempForOutput(dir, base+".backup")
	if err != nil {
		return err
	}
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
	_ = os.Remove(backupPath)
	return nil
}

func writePNGAtomic(path string, img image.Image, force bool) error {
	existed, mode, err := outputTargetInfo(path, force)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	temporary, temporaryPath, err := createTempForOutput(dir, base)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = temporary.Close()
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := png.Encode(temporary, img); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}

	if existed {
		err = commitForcedRegular(temporaryPath, path, mode)
	} else {
		err = commitNoClobber(temporaryPath, path)
	}
	if err != nil {
		return err
	}
	committed = true
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

	if math.IsNaN(*strength) || math.IsInf(*strength, 0) || *strength < 4 || *strength > 120 {
		return fmt.Errorf("-strength must be a finite number from 4 to 120")
	}
	profile, err := watermark.ParseProfile(*profileName)
	if err != nil {
		return err
	}
	outputPath := pngOutputPath(*out)
	// Fast-fail before decoding or embedding a potentially huge input. The final
	// writer repeats the check and commits race-safely.
	if _, _, err := outputTargetInfo(outputPath, *force); err != nil {
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
	raw := fs.Bool("raw", false, "write only the authenticated payload bytes to stdout")

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
	printExtractDiagnostics(os.Stderr, info)
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

	config, format, err := openImageConfig(*in)
	if err != nil {
		return err
	}
	geometry := boundsOnlyImage{rectangle: image.Rect(0, 0, config.Width, config.Height)}
	maximum := watermark.Capacity(geometry)
	if !*details {
		fmt.Printf("%d bytes\n", maximum)
		return nil
	}

	fmt.Printf("Image:       %s\n", *in)
	fmt.Printf("Format:      %s\n", strings.ToUpper(format))
	fmt.Printf("Dimensions:  %d x %d\n", config.Width, config.Height)
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

func diagnose(args []string) error {
	fs := newFlagSet("diagnose", "Experimental v0.3 local-lattice diagnostics. Lattice evidence is not watermark authentication.")
	in := fs.String("in", "", "input JPEG or PNG file (required)")
	key := fs.String("key", "", "optional key for an independent baseline v3 authentication attempt")
	jsonOutput := fs.Bool("json", false, "emit machine-readable JSON")
	regions := fs.Int("regions", 3, "diagnostic region grid, N x N (1 to 4)")
	maxDimension := fs.Int("max-dim", 2048, "maximum diagnostic pyramid dimension (512 to 4096)")
	levels := fs.Int("levels", 2, "maximum diagnostic pyramid levels (1 to 3)")

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
	if *regions < 1 || *regions > 4 {
		return fmt.Errorf("-regions must be from 1 to 4")
	}
	if *maxDimension < 512 || *maxDimension > 4096 {
		return fmt.Errorf("-max-dim must be from 512 to 4096")
	}
	if *levels < 1 || *levels > 3 {
		return fmt.Errorf("-levels must be from 1 to 3")
	}

	img, format, err := openImageWithFormat(*in)
	if err != nil {
		return err
	}
	options := watermark.DefaultDiagnosticOptions()
	options.RegionsX = *regions
	options.RegionsY = *regions
	options.MaxAnalysisDimension = *maxDimension
	options.MaxLevels = *levels
	options.AttemptAuthentication = *key != ""
	report, err := watermark.DiagnoseGeometry(img, []byte(*key), options)
	if err != nil {
		return err
	}
	if *jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}

	fmt.Printf("Image:                    %s\n", *in)
	fmt.Printf("Format:                   %s\n", strings.ToUpper(format))
	fmt.Printf("Dimensions:               %d x %d\n", report.Width, report.Height)
	fmt.Printf("Pyramid levels:           ")
	for index, level := range report.Levels {
		if index > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("1/%d=%dx%d", level.Divisor, level.Width, level.Height)
	}
	fmt.Println()
	fmt.Printf("Print boundary:           detected=%t confidence=%.3f prior=%t\n", report.PrintBoundary.Detected, report.PrintBoundary.Confidence, report.BoundaryPriorUsed)
	if report.ProjectiveEstimate.Available {
		fmt.Printf("Projective initializer:   confidence=%.3f canonical≈%.1fx%.1f candidates=%d\n",
			report.ProjectiveEstimate.Confidence, report.ProjectiveEstimate.CanonicalWidthPixels,
			report.ProjectiveEstimate.CanonicalHeightPixels, len(report.ProjectiveEstimate.ScaleCandidates))
	}
	fmt.Printf("Local regions:            %d\n", len(report.Regions))
	for _, region := range report.Regions {
		fmt.Printf("  [%d,%d] div=%d u=(%.2f,%.2f) v=(%.2f,%.2f) period=(%.2f,%.2f) angle=%.2f axis=%.2f coherence=%.3f repeat=%.3f confidence=%.3f\n",
			region.RegionX, region.RegionY, region.AnalysisDivisor,
			region.U.X, region.U.Y, region.V.X, region.V.Y,
			region.PeriodU, region.PeriodV, region.OrientationDegrees,
			region.InterAxisDegrees, region.PeriodicCoherence, region.TileRepetitionCoherence, region.Confidence)
	}
	fmt.Printf("Global basis:             u=(%.2f,%.2f) v=(%.2f,%.2f)\n", report.GlobalU.X, report.GlobalU.Y, report.GlobalV.X, report.GlobalV.Y)
	fmt.Printf("Global consistency:       %.3f\n", report.GlobalConsistency)
	fmt.Printf("Consensus regions:        %d/%d (%.3f)\n", report.ConsensusRegions, len(report.Regions), report.ConsensusFraction)
	fmt.Printf("Lattice evidence:         %t  [diagnostic only]\n", report.LatticeEvidence)
	fmt.Printf("Authentication status:    %s\n", report.AuthenticationStatus)
	fmt.Printf("Authenticated payload:    %t\n", report.AuthenticatedPayload)
	if report.ProjectiveAuthentication.CandidatesProbed > 0 {
		fmt.Printf("Projective auth probe:    candidates=%d full-decodes=%d best-sync=%s %.3f (z=%.2f)\n",
			report.ProjectiveAuthentication.CandidatesProbed, report.ProjectiveAuthentication.FullDecodeAttempts,
			report.ProjectiveAuthentication.BestSyncProfile, report.ProjectiveAuthentication.BestSyncFraction,
			report.ProjectiveAuthentication.BestSyncZScore)
	}
	if report.AuthenticatedPayload {
		fmt.Printf("Authenticated profile:    %s\n", report.AuthenticatedProfile)
		fmt.Printf("Authentication confidence: %.2f\n", report.AuthenticationConfidence)
		fmt.Printf("Authenticated message:    %s\n", report.AuthenticatedMessage)
	}
	fmt.Printf("Timing:                   pyramid=%dms boundary=%dms lattice=%dms projective=%dms auth=%dms total=%dms\n",
		report.Timings.PyramidMilliseconds, report.Timings.PrintBoundaryMilliseconds, report.Timings.LocalLatticeMilliseconds,
		report.Timings.ProjectiveFitMilliseconds, report.Timings.AuthenticationMilliseconds, report.Timings.TotalMilliseconds)
	fmt.Printf("Note:                     %s\n", report.Note)
	return nil
}
