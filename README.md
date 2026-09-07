# PixSeal

PixSeal is an experimental, pure-Go **robust image steganography** CLI. It hides
small authenticated payloads inside the luminance DCT coefficients of an image
while keeping the resulting changes visually unobtrusive under normal viewing.

Current release: **v0.1.0**.

The goal is not limited to ownership marks or provenance identifiers. PixSeal's
core format carries an arbitrary byte payload; the v0.1 CLI exposes that payload
as a text message through `-message`. Future front ends can use the same format
for identifiers, hashes, compact structured data or other application-specific
content.

PixSeal v2 adds geometric synchronization for the transformations in the current
test matrix: JPEG recompression, centered or off-grid cropping, and automatic
resize normalization from 95% down to 25% in 5% steps. The decoder retains read
compatibility with the original v1 format.

The project direction was initially inspired by the general idea demonstrated by
systems such as Google SynthID: information can be embedded imperceptibly in
media while retaining useful robustness after transformations. PixSeal is an
independent classical DCT implementation; it does **not** implement or reproduce
Google's SynthID algorithm.

PixSeal does not claim statistical steganographic undetectability. The current
implementation has not undergone a steganalytic or security audit.

## Build

```sh
go build -o pixseal ./cmd/pixseal
```

Or use the Makefile to build a native executable in `dist/`:

```sh
make
```

## Usage

```sh
pixseal embed -in photo.png -out sealed.png -key "a long secret" -message "hello"
pixseal extract -in sealed.png -key "a long secret"
pixseal capacity -in photo.png
```

`-message` is mandatory for `embed`; empty payloads and unlabelled positional
arguments are rejected. Run PixSeal without arguments for the command overview,
or use `pixseal <command> -help` for all options of a specific command.

Output from `embed` is always PNG so the newly hidden payload is not immediately
subjected to a lossy re-encoding step. The resulting PNG may subsequently be
converted to JPEG.

Supported formats:

- input: PNG (`.png`) and JPEG (`.jpg`, `.jpeg`);
- output from `embed`: PNG (`.png`) only.

If the requested output name has another extension, PixSeal replaces it with
`.png`; if it has no extension, `.png` is appended. Existing destination files
are protected by default. Use `-force` only when replacement is intentional:

```sh
pixseal embed -in photo.jpg -out sealed.png -force \
  -key "a long secret" -message "hello"
```

The image is fully encoded to a temporary file before the destination is
replaced, reducing the risk of destroying an existing file after a failed PNG
encode.

## Payload protection

Payloads are authenticated with HMAC-SHA256 truncated to 64 bits, whitened with
a key-derived stream, protected by Hamming(7,4), and repeated as a periodic
two-dimensional DCT tile. During extraction, PixSeal searches block scale, pixel
alignment and tile phase before accepting a payload only after CRC and HMAC
validation.

This provides authentication and error detection; it is **not encryption**.
Encrypt sensitive content before embedding it if confidentiality matters.

The current CLI receives the key as a command-line argument. Depending on the
operating system and shell, command-line arguments can be exposed through shell
history or process inspection. Do not treat `-key` as a complete secret-management
solution for high-security workflows.

## Geometry and scaling

The v2 embedding grid uses 8x8 blocks. Extraction has direct paths for apparent
8-, 6- and 4-pixel blocks, corresponding to 100%, 75% and 50% scaling. It then
applies bounded bicubic luminance normalization to search the remaining scales
from 95% down to 25% in 5% increments.

A crop does not need to start on an 8-pixel boundary. One complete logical tile
is 280x256 pixels at the original scale. Because an arbitrary crop can begin
between block boundaries, the dimensions that **guarantee** a complete tile for
any crop alignment are larger:

| Apparent scale | Aligned tile | Guaranteed for any pixel offset |
|---:|---:|---:|
| 100% | 280x256 | 287x263 |
| 75% | 210x192 | 215x197 |
| 50% | 140x128 | 143x131 |

Smaller off-grid crops can still succeed when their particular alignment retains
one complete logical tile; the guaranteed dimensions above are worst-case bounds.

See [the algorithm specification](docs/ALGORITHM.md) and
[the measured robustness results](docs/RESULTS.md).

## Current limitations

- Maximum payload: 64 bytes.
- A source image used for v2 embedding must be at least 280x256 pixels.
- The v0.1 CLI embeds text through `-message`; the underlying format stores bytes.
- Automatic normalization searches supported scales from 25% to 100% at 5%
  intervals, with direct paths at 100%, 75% and 50%.
- A normalized search candidate is skipped if reconstructing it would exceed
  **50,000,000 pixels**. This bounds memory and failed-extraction work on very
  large images.
- Fractional resizing combined with an off-grid crop is not guaranteed in v0.1;
  the normalized scale path assumes that a pure resize retained the grid origin.
- Rotation, perspective changes and arbitrary resampling factors are not yet
  synchronized.
- Failed extraction is inherently more expensive than successful extraction,
  because the decoder must exhaust more scale/alignment candidates before it can
  conclude that no authenticated payload was found.
- `-repetition` is a legacy-v1 fallback option. It is ignored by v2 embedding and
  cannot prevent extraction of an otherwise valid v2 payload.
- PixSeal operates on decoded pixels and writes an 8-bit PNG. Higher PNG sample
  depth is therefore reduced to 8 bits.
- Image metadata such as EXIF, XMP, comments and ICC/profile information is not
  preserved by the current pixel decode/re-encode pipeline.
- Alpha is flattened against white.
- Visual unobtrusiveness is not equivalent to statistical undetectability.
- This is experimental software and has not received a cryptographic or
  steganalytic audit.

## Tests

The public regression suite is self-contained and does not require the private
image corpus:

```sh
make test
```

It builds PixSeal and runs `go test ./...`, including v2 transformation tests,
legacy-v1 extraction compatibility and output-overwrite protection.

The development corpus lives locally in `original pics/` and is intentionally
excluded from Git and release archives. To run a simple CLI round trip on every
local JPEG or PNG in that directory:

```sh
make test-images
```

`make deep-test` runs the configured JPEG, resize and crop robustness matrix:

```sh
make deep-test
```

`make all` runs the self-contained tests plus `test-images` and `deep-test`, so
it requires the local `original pics/` corpus:

```sh
make all
```

All image-corpus tests scan only `original pics/`; generated files are temporary
and removed automatically.

### Robustness tests

The shell robustness suites require:

- Bash 4 or newer (`mapfile` is used);
- GNU `timeout`;
- ImageMagick (`magick` or `convert`).

These requirements matter in particular on a stock macOS installation, where
the bundled Bash and command set may not satisfy them without additional tools.
They are not required for `go test ./...`.

`make deep-test` prints a non-blocking robustness report for each image after:

- JPEG recompression at quality 82;
- resizing to 95%, 85%, 75%, 65%, 55% and 50%;
- centered cropping to 90%, 75% and 50%;
- reproducible off-center crops to 75% and 50%.

Payload-recovery failures are reported without failing the default deep-test
target. To make every failed recovery or extraction timeout return a non-zero
exit status, use strict mode:

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

`make extreme-test` evaluates every configured resize, centered-crop and random-
crop percentage. It intentionally continues after failures because robustness
need not be monotonic across different resampling or crop alignments:

```sh
make extreme-test
```

The default sequence starts at 95%, descends by 5 percentage points and ends at
10%. It is configurable:

```sh
make extreme-test LIMIT_START=90 LIMIT_MIN=20 LIMIT_STEP=10 RANDOM_SEED=42
```

This target is intentionally not included in `make all`, because it can take
considerably longer than the repeatable regression suite.

Transformation tests limit each extraction to 60 seconds. Override this when
needed, for example:

```sh
make extreme-test EXTRACT_TIMEOUT=120
```

To avoid ImageMagick resource exhaustion, images larger than 100 megapixels are
skipped by default during corpus robustness tests. Change or disable that test
harness limit with:

```sh
make deep-test ROBUST_MAX_MPIX=250
make deep-test ROBUST_MAX_MPIX=0
```

This 100-MP **test-harness** limit is separate from the decoder's internal
50,000,000-pixel cap on reconstructed normalization candidates.

Reports distinguish recovered messages (`PASS`), completed extraction without a
valid authenticated payload (`FAIL`), extraction time limits (`TIMEOUT`), images
over the configured test limit (`SKIP`) and transformation-tool failures
(`ERROR`). A timeout is inconclusive, not evidence that no hidden payload exists.
Tool errors always make the target fail; payload failures and timeouts do so only
with `STRICT=1`.

## License

PixSeal is distributed under the [MIT License](LICENSE).
