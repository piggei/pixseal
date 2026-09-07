# PixSeal

PixSeal is an experimental, pure-Go robust image watermarking CLI. It embeds a
small authenticated payload in the luminance DCT coefficients of an image.

Current development version: **v0.1.0 build 5**. During development, functional
version `0.1.0` remains fixed and the build number increases after each revision.

This first MVP is intended to establish a measurable baseline. It survives PNG
round-trips and ordinary JPEG recompression. Cropping, rotation and arbitrary
resizing are not yet supported because they require geometric synchronization.

## Build

```sh
go build -o pixseal ./cmd/pixseal
```

## Usage

```sh
pixseal embed -in photo.png -out marked.png -key "a long secret" -message "hello"
pixseal extract -in marked.png -key "a long secret"
pixseal capacity -in photo.png
```

`-message` is mandatory for `embed`; empty payloads and unlabelled positional
arguments are rejected. Run PixSeal without arguments for the command overview,
or use `pixseal <command> -help` for all options of a specific command.

Output from `embed` is always PNG, so the watermark is not damaged before it is
delivered. The marked PNG may subsequently be converted to JPEG.

Supported formats:

- input: PNG (`.png`) and JPEG (`.jpg`, `.jpeg`);
- output from `embed`: PNG (`.png`) only.

If the output name has another extension, PixSeal replaces it with `.png`; if it
has no extension, `.png` is appended.

Payloads are authenticated with HMAC-SHA256 (truncated to 64 bits), randomized
with a key-derived stream, and repeated across DCT blocks. This is watermarking,
not encryption: encrypt sensitive payloads before embedding them.

## Current limitations

- Maximum payload: 64 bytes, subject to image capacity.
- Minimum useful image size depends on payload and repetition setting.
- The decoder must use the same key and repetition setting.
- Alpha is flattened against white.
- This is experimental software and has not received a security audit.

## Tests

`make` only builds the Linux executable in `dist/pixseal`:

```sh
make
```

`make test` first builds the executable, then runs the Go unit tests and a simple
embed/extract round trip on every JPEG or PNG in `original pics`. It never scans
other repository directories:

```sh
make test
```

`make deep-test` builds the executable when needed and runs only the advanced
JPEG, resize and crop tests:

```sh
make deep-test
```

`make all` runs the complete sequence: build, simple tests and advanced tests:

```sh
make all
```

All image tests use only `original pics`; generated files are temporary and are
removed automatically.

### Robustness tests

`make deep-test` prints a non-blocking robustness report for each image after:

- JPEG recompression at quality 82;
- resizing to 75% and 50%;
- centered cropping to 90% and 75%.

Crop and resize failures are expected in the current algorithm and are reported
without failing the normal test target. To make every robustness failure return a
non-zero exit status, use strict mode:

```sh
make deep-test STRICT=1
```

The transformation matrix is configurable, for example:

```sh
make deep-test ROBUST_RESIZES="80 60" ROBUST_CROPS="95 80" JPEG_QUALITY=75
```

To avoid ImageMagick resource exhaustion, images larger than 100 megapixels are
skipped by default during deep tests. Change or disable the limit with:

```sh
make deep-test ROBUST_MAX_MPIX=250
make deep-test ROBUST_MAX_MPIX=0
```

The report distinguishes watermark failures (`FAIL`), images over the configured
limit (`SKIP`) and transformation-tool failures (`ERROR`). Tool errors always make
the target fail; watermark failures do so only with `STRICT=1`.

The robustness suite requires ImageMagick (`magick` or `convert`). Generated test
files are kept in a temporary directory and removed automatically.
