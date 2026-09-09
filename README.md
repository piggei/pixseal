# PixSeal

![PixSeal — Hide messages. Keep the picture.](docs/assets/pixseal-banner.png)

PixSeal is an experimental, pure-Go **robust image steganography** tool for
hiding short authenticated messages inside images. It embeds protected bits in
luminance DCT coefficients and repeats the protected frame across the image so
that digital transforms such as JPEG recompression, resize and crop can often be
survived.

Current package: **v0.2.0-rc2**. This release candidate keeps the on-image
**Format v3** and encoder bit-for-bit compatible with the qualified v0.2.0 RC1
line while hardening CLI validation, output-file safety, test harnesses and the
technical documentation.

PixSeal is a hidden-data channel, not an ownership-marking product. Classical
watermarking techniques are part of the mechanism; the project goal is robust
steganography for short messages. PixSeal does **not** claim statistical
undetectability and has not received a professional cryptographic or
steganalytic audit.

The project was inspired by the general idea that machine-readable information
can remain embedded in transformed media. PixSeal is an independent classical
DCT implementation and does not implement or claim compatibility with Google
SynthID.

See also:

- [`docs/ALGORITHM.md`](docs/ALGORITHM.md) — current Format v3 and decoder specification;
- [`docs/RESULTS.md`](docs/RESULTS.md) — release qualification and research results;
- [`HISTORY.md`](HISTORY.md) — chronological engineering history;
- [`CHANGELOG.md`](CHANGELOG.md) — release/build changes;
- [`TODO.md`](TODO.md) — open work only.

## Release scope

The v0.2.0 release line considers the following the stable digital baseline:

- adaptive Format v3 with `robust`, `balanced`, `capacity` and `auto` profiles;
- authenticated payloads up to 64 bytes;
- deterministic keyed whitening, Hamming(7,4), CRC32 and truncated HMAC-SHA256;
- PNG/JPEG input and PNG output;
- JPEG, pure resize and crop recovery used by `deep-test`;
- reusable Go core with Linux/Windows/Android/iOS compile checks.

The decoder also contains **experimental** bounded recovery for:

- exact 90/180/270-degree rotations;
- arbitrary digital rotation;
- selected axis-aligned affine transforms;
- a fixed direct lattice-basis bank for selected anisotropic-scale + rotation cases;
- two mild vertical-keystone projective hypotheses.

These research paths are intentionally not universal recovery promises. The
physical print → paper → smartphone channel and general homography inference are
outside the v0.2.0 release contract.

## Build

The core and CLI use only the Go standard library at runtime.

```sh
go build -o pixseal ./cmd/pixseal
```

or:

```sh
make
```

`make` builds only. ImageMagick and GNU `timeout` are required by the shell test
suites, not by PixSeal itself.

Cross-platform helper targets are documented under [Testing](#testing).

## CLI

### Embed

```sh
pixseal embed \
  -in photo.png \
  -out sealed.png \
  -key "a long secret" \
  -message "hidden message" \
  -profile auto
```

Profiles:

| Profile | Maximum payload | Protected frame | Hamming-protected bits | Tile redundancy |
|---|---:|---:|---:|---:|
| `robust` | 16 B | 32 B | 448 | 2.50× |
| `balanced` | 32 B | 48 B | 672 | 1.67× |
| `capacity` | 64 B | 80 B | 1120 | 1.00× |
| `auto` | automatic | automatic | automatic | most robust compatible |

The default embedding strength is 24. CLI values must be finite and in the
inclusive range **4..120**. Explicit `-strength 0`, `NaN` and infinities are
rejected. The Go API retains `Options.Strength == 0` as the internal sentinel
for the default value.

A successful embed reports the selected profile:

```text
embedded 14 bytes using profile robust in sealed.png
```

PixSeal output is always PNG. If the supplied output name has another extension,
it is normalized to `.png`. A dotfile such as `.sealed` becomes
`.sealed.png`.

### Output safety and `-force`

Without `-force`, PixSeal uses a no-clobber commit and refuses any existing
directory entry, including dangling symlinks. With `-force`, only an existing
**regular file** may be replaced; directories, symlinks, FIFOs and devices are
rejected.

New output files are created with private temporary-file permissions (normally
`0600` on Unix), so PixSeal never weakens a restrictive umask. Replacing an
existing regular file preserves its permission bits.

The encoded temporary file is synced before commit. On POSIX, no-force commit
uses an atomic hard-link creation so a racing destination is not overwritten.
On Windows, forced replacement requires a backup/restore rename sequence because
Go's `os.Rename` cannot replace an existing destination; a process crash in that
small window can leave the backup name behind. PixSeal therefore does not claim
full filesystem-durability semantics for forced Windows replacement.

### Extract

```sh
pixseal extract -in sealed.png -key "a long secret"
```

Normal extraction prints the payload followed by diagnostics such as profile,
confidence and any geometric correction used.

For scripts or payloads containing newlines, use:

```sh
pixseal extract -raw -in sealed.png -key "a long secret"
```

`-raw` writes **only the exact recovered payload bytes to stdout**. Diagnostics
are written to stderr. This avoids the ambiguity of parsing a multiline payload
from human-readable output.

The profile is recovered automatically from the authenticated v3 frame; users do
not provide a profile to `extract`.

### Capacity

```sh
pixseal capacity -in photo.png
```

Detailed output:

```sh
pixseal capacity -in photo.png -details
```

A complete Format v3 tile requires at least **280×256** pixels at the native
8-pixel DCT grid.

### Analyze

```sh
pixseal analyze -in photo.png -message "hidden message"
```

or:

```sh
pixseal analyze -in photo.png -bytes 18
```

Exactly one of `-message` or `-bytes` is required. `analyze` reports deterministic
format facts separately from heuristic image-detail and strength recommendations.
Its detail estimator uses the same white alpha compositing as the encoder.
Recommendations are not recovery guarantees.

## Format v3 summary

Format v3 uses a fixed-size frame selected by profile. The logical frame is:

```text
+------------------+------------------------+----------+------------------+
| header (8 bytes) | actual payload (N B)   | tag (8B) | zero padding      |
+------------------+------------------------+----------+------------------+
```

The tag is placed **immediately after the actual payload**, at offset
`8 + payloadLength`; it is not placed at the end of the profile's payload
capacity region.

Header:

```text
magic[2] | version/profile[1] | payloadLength[1] | CRC32[4]
```

The HMAC covers `header + actual payload`. The remainder of the fixed profile
frame stays zero before whitening. The full fixed frame is whitened, encoded
with Hamming(7,4), permuted into a 35×32 tile (1120 positions), and repeated
across the carrier.

Whitening is deterministic key-derived masking. **It is not encryption.** If the
message itself is sensitive, encrypt it before embedding.

The complete technical specification is in [`docs/ALGORITHM.md`](docs/ALGORITHM.md).

## Decoder overview

The v0.2.0 decoder tries established recovery paths before more speculative
geometry. The current order is:

1. direct v3 decoding on 8/6/4-pixel integer grids;
2. gated lossless 90/180/270-degree quarter turns;
3. pure isotropic fractional-scale recovery through virtual affine sampling;
4. at most one targeted physical bilinear resize fallback for a decisive scale;
5. axis-aligned affine recovery when zero-degree evidence remains;
6. two mild vertical-keystone projective hypotheses;
7. fixed direct lattice-basis composition search;
8. bounded arbitrary-angle rotation search;
9. final axis-aligned affine fallback when rotation evidence is weak.

A project rule introduced after earlier regressions is preserved:

> An unauthenticated advanced geometry heuristic must not suppress an established
> recovery path unless deterministic or authenticated evidence makes that path
> inapplicable.

All successful payloads must pass Format v3 authentication. Geometry scores are
ranking evidence only.

## Image size and memory policy

The CLI calls `image.DecodeConfig` before full decode and rejects decoded inputs
above **300 MP**. The public embed/extract core applies the same source-size guard
before building PixSeal's RGB working plane. This is a pre-allocation safety
policy, not the 50 MP geometry-search limit used by some research paths.

Images around 200 MP remain within the v0.2.0 source policy and have been used in
round-trip testing. Geometry scripts may independently skip large sources (for
example at 50 MP) to keep research runtimes bounded.

A future v0.3 line may replace the full RGB working plane with tiled/sparse
processing; v0.2.0 does not claim that optimization.

## Metadata, orientation and color

PixSeal decodes image pixels and writes a new PNG. It does not preserve source
metadata.

Known consequences:

- EXIF Orientation is not automatically applied by Go's JPEG decoder; a JPEG that
  a viewer auto-rotates may be processed in its stored pixel orientation.
- ICC/color profiles are not preserved. Loss of color management can change
  appearance, not merely metadata.
- transparency is flattened against white during embedding; `analyze` uses the
  same assumption for its detail heuristic.

These are documented v0.2.0 limitations rather than silent guarantees.

## Testing

Local test images live in `original pics/` and are deliberately excluded from the
source archive.

Core targets:

```sh
make test              # complete Go tests + local image round trips
make release-unit      # release-gate Go tests only
make research-unit     # deterministic experimental-geometry Go regressions
make test-images       # local image round trips only
make deep-test         # baseline JPEG / resize / crop transformations
make extreme-test      # progressive limit map; non-strict by design
make geometry-test     # experimental rotation / combined geometry
make affine-test       # experimental axis-aligned affine
make composition-test  # build-7 composition regression
make lattice-test      # direct lattice-basis bank
make perspective-test  # two mild vertical-keystone hypotheses
make core-target-check # linux/windows/android/ios core compile checks
make version-check     # VERSION and buildinfo consistency
make release-check     # release baseline gate
make all-test          # sequential comparative report
```

`extreme-test` is intentionally a **non-strict limit exploration**: individual
FAIL entries describe the measured boundary and do not fail the target.

When `STRICT=1` is used, `deep-test`, `geometry-test`, `affine-test`,
`composition-test`, `lattice-test` and `perspective-test` fail their target on a
reported FAIL or timeout.

`make all-test` applies `ALL_TEST_STRICT=1` to those strict-capable suites,
continues after failures and prints separate release-baseline and research-suite
status. `extreme-test` remains non-strict inside `all-test`.

Example report capture:

```sh
make all-test ALL_TEST_REPORT=report.txt
```

The release baseline is:

```text
version-check
vet
release-unit
test-images
deep-test
core-target-check
```

`make test` retains the historical semantics of running the complete Go test
suite plus local image round trips. For release qualification, `release-unit`
and `research-unit` split deterministic Go tests so an experimental geometry
regression cannot make the stable release baseline red. Private-corpus research
suites are
reported separately.

## Security boundaries

PixSeal provides authenticated hidden payload recovery, not confidentiality.

- Minimum key length is 8 bytes; this is a project trade-off, not a claim of
  password strength.
- The HMAC-SHA256 tag is truncated to 64 bits.
- Whitening is deterministic and does not encrypt the payload.
- No claim is made that a carrier is statistically indistinguishable from an
  unmodified image.
- No professional cryptographic or steganalytic audit has been performed.

## Portability and future GUI

The `watermark` package is independent of terminal I/O. `make core-target-check`
compile-checks the reusable core for:

```text
linux/amd64
windows/amd64
android/arm64
ios/arm64
```

A future GUI is planned for Windows/Linux with Android as a first-class target.
The GUI is not part of v0.2.0.

## License

MIT. See [`LICENSE`](LICENSE).
