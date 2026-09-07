package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/pj/pixseal/internal/buildinfo"
	"github.com/pj/pixseal/watermark"
)

var usageHeader = fmt.Sprintf(`PixSeal %s - robust image watermarking

Usage:
  pixseal <command> [options]

Commands:
  embed      Embed an authenticated message in an image
  extract    Extract and authenticate a message from an image
  capacity   Show the usable payload capacity of an image

Supported image formats:
  Input       PNG (.png), JPEG (.jpg, .jpeg)
  Output      PNG (.png) only

Embed options:
  -in FILE           Input JPEG or PNG (required)
  -out FILE          Output PNG (required)
  -key TEXT          Secret key, minimum 8 bytes (required)
  -message TEXT      Message to embed (required)
  -repetition N      Odd repetition count, 1 to 31 (default 5)
  -strength N        DCT embedding strength, 4 to 120 (default 34)

Extract options:
  -in FILE           Marked JPEG or PNG (required)
  -key TEXT          Secret key, minimum 8 bytes (required)
  -repetition N      Same value used by embed (default 5)
  -strength N        Accepted for compatibility (default 34)

Capacity options:
  -in FILE           Input JPEG or PNG (required)
  -repetition N      Odd repetition count, 1 to 31 (default 5)

Run "pixseal <command> -help" to show the options for a command.

Examples:
  pixseal embed -in photo.png -out marked.png -key "a long secret" -message "hello"
  pixseal extract -in marked.png -key "a long secret"
  pixseal capacity -in photo.png -repetition 5
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

func openImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func pngOutputPath(path string) string {
	ext := filepath.Ext(path)
	if strings.EqualFold(ext, ".png") {
		return path
	}
	if ext == "" {
		return path + ".png"
	}
	return strings.TrimSuffix(path, ext) + ".png"
}

func common(fs *flag.FlagSet) (*string, *int, *float64) {
	key := fs.String("key", "", "secret key (required, minimum 8 bytes)")
	repetition := fs.Int("repetition", 5, "odd repetition count from 1 to 31")
	strength := fs.Float64("strength", 34, "DCT embedding strength from 4 to 120")
	return key, repetition, strength
}

func embed(args []string) error {
	fs := newFlagSet("embed", "Embed a message; the output image is always PNG.")
	in := fs.String("in", "", "input JPEG or PNG file (required)")
	out := fs.String("out", "", "output PNG file (required)")
	message := fs.String("message", "", "message to embed, up to the image capacity (required)")
	key, repetition, strength := common(fs)

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

	img, err := openImage(*in)
	if err != nil {
		return err
	}
	marked, err := watermark.Embed(img, []byte(*message), []byte(*key), watermark.Options{
		Strength:   *strength,
		Repetition: *repetition,
	})
	if err != nil {
		return err
	}

	outputPath := pngOutputPath(*out)
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := png.Encode(f, marked); err != nil {
		return err
	}
	if outputPath != *out {
		fmt.Printf("output renamed to %s (PixSeal output is PNG)\n", outputPath)
	}
	fmt.Printf("embedded %d bytes in %s\n", len([]byte(*message)), outputPath)
	return nil
}

func extract(args []string) error {
	fs := newFlagSet("extract", "Extract and authenticate a PixSeal message.")
	in := fs.String("in", "", "marked JPEG or PNG file (required)")
	key, repetition, strength := common(fs)

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
	payload, confidence, err := watermark.Extract(img, []byte(*key), watermark.Options{
		Strength:   *strength,
		Repetition: *repetition,
	})
	if err != nil {
		return err
	}
	fmt.Printf("%s\nconfidence-margin: %.2f\n", payload, confidence)
	return nil
}

func capacity(args []string) error {
	fs := newFlagSet("capacity", "Show usable payload capacity in bytes.")
	in := fs.String("in", "", "input JPEG or PNG file (required)")
	repetition := fs.Int("repetition", 5, "odd repetition count from 1 to 31")

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

	img, err := openImage(*in)
	if err != nil {
		return err
	}
	fmt.Printf("%d bytes\n", watermark.Capacity(img, *repetition))
	return nil
}
