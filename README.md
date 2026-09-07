# PixSeal

PixSeal is an experimental, pure-Go robust image watermarking CLI. It embeds a
small authenticated payload in the luminance DCT coefficients of an image.

Current release: **v0.1.0**.

Watermark format v2 adds geometric synchronization for the transformations in
the current test matrix: JPEG recompression, centered or off-grid cropping, and
automatic resize normalization from 95% down to 25% in 5% steps. The decoder
retains read compatibility with v1 images.

Transformation tests limit each extraction to 60 seconds. Override this when
needed, for example with `make extreme-test EXTRACT_TIMEOUT=120`.

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

Payloads are authenticated with HMAC-SHA256 (truncated to 64 bits), whitened
with a key-derived stream, protected by Hamming(7,4), and repeated as a periodic
two-dimensional DCT tile. During extraction, PixSeal searches block scale, pixel
alignment and tile phase before validating the payload. This is watermarking, not
encryption: encrypt sensitive payloads before embedding them.

The v2 synchronization grid uses 8x8 blocks when embedding. Extraction has fast
paths for 100%, 75% and 50%, then applies bounded bicubic normalization to search
scaling from 95% down to 25% in 5% increments. A crop does not need to start on
an 8-pixel boundary. See [the algorithm specification](docs/ALGORITHM.md) and
[the measured robustness results](docs/RESULTS.md).

## Current limitations

- Maximum payload: 64 bytes.
- A v2 source image must be at least 280x256 pixels.
- The decoder searches scales from 25% to 100% at 5% intervals; intermediate
  fractions, rotation and perspective changes are not yet supported.
- Fractional resizing combined with an off-grid crop is not guaranteed in v0.1;
  the bounded scale path assumes that the resized image retains its grid origin.
- Crops must retain enough area to contain the complete 35x32 synchronization
  tile at one of the supported scales.
- `-repetition` is used only when reading legacy v1 watermarks.
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
- resizing to 95%, 85%, 75%, 65%, 55% and 50%;
- centered cropping to 90%, 75% and 50%;
- reproducible off-center crops to 75% and 50%.

Watermark failures are reported without failing the default deep-test target. To
make every robustness failure return a non-zero exit status, use strict mode:

```sh
make deep-test STRICT=1
```

The transformation matrix is configurable, for example:

```sh
make deep-test ROBUST_RESIZES="80 60" ROBUST_CROPS="95 80" JPEG_QUALITY=75
```

Random crops use a fixed seed by default. Use a different seed or add more
positions per percentage with:

```sh
make deep-test RANDOM_SEED=42 RANDOM_CROP_COUNT=3
```

### Progressive limit tests

`make extreme-test` tests every configured resize percentage, because scale
support may be non-monotonic, then reduces centered and random crops until their
first extraction failure:

```sh
make extreme-test
```

The default sequence starts at 95%, descends by 5 percentage points and stops at
10%. It is configurable:

```sh
make extreme-test LIMIT_START=90 LIMIT_MIN=20 LIMIT_STEP=10 RANDOM_SEED=42
```

This target is intentionally not included in `make all`, because it can take
considerably longer than the repeatable regression suite.

To avoid ImageMagick resource exhaustion, images larger than 100 megapixels are
skipped by default during deep tests. Change or disable the limit with:

```sh
make deep-test ROBUST_MAX_MPIX=250
make deep-test ROBUST_MAX_MPIX=0
```

The report distinguishes recovered messages (`PASS`), watermark failures
(`FAIL`), extraction time limits (`TIMEOUT`), images over the configured limit
(`SKIP`) and transformation-tool failures (`ERROR`). A timeout is inconclusive,
not evidence that the watermark is absent. Tool errors always make the target
fail; watermark failures and timeouts do so only with `STRICT=1`.

The robustness suite requires ImageMagick (`magick` or `convert`). Generated test
files are kept in a temporary directory and removed automatically.

## License

PixSeal is distributed under the [MIT License](LICENSE).
