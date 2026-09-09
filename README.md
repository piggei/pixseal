# PixSeal

![PixSeal — Hide messages. Keep the picture.](docs/assets/pixseal-banner.png)

PixSeal is an experimental, pure-Go **robust image steganography** tool for
hiding short authenticated messages inside images. It embeds protected payload
bits in luminance DCT coefficients while trying to keep the visual change small
under normal viewing conditions.

Current release candidate: **v0.2.0-rc4**.

PixSeal is a hidden-data channel, not an ownership-marking product. Digital
watermarking is the robustness mechanism; the project goal is robust
steganography for short messages. PixSeal does **not** claim statistical
undetectability and has not undergone a professional cryptographic or
steganalytic audit.

Format v3 is the interoperability baseline for the v0.2.0 line. Its on-image
layout and deterministic encoder fingerprints are frozen during release
hardening.

Project history and future work are kept in [`HISTORY.md`](HISTORY.md) and
[`TODO.md`](TODO.md). Release-facing changes are in
[`CHANGELOG.md`](CHANGELOG.md).

## What v0.2.0 provides

The release baseline covers:

- adaptive Format v3 profiles: `robust`, `balanced`, `capacity`, `auto`;
- authenticated payloads up to 64 bytes;
- CRC32, truncated HMAC-SHA256, key-derived whitening and Hamming(7,4);
- JPEG/PNG input and PNG output;
- robust digital JPEG, resize and crop recovery on the qualification corpus;
- v3-only extraction with automatic profile detection;
- a reusable pure-Go core compile-checked for Linux, Windows, Android and iOS.

The decoder also contains **experimental**, bounded recovery for arbitrary
rotation, axis-aligned affine transforms, a fixed direct lattice-basis bank and
two mild projective/keystone hypotheses. These are measured research
capabilities, not universal guarantees. General homography estimation and the
physical print-camera channel are outside v0.2.0 scope.

## Build

```sh
go build -o pixseal ./cmd/pixseal
```

or:

```sh
make
```

`make` builds only.

The project has no external runtime dependencies. ImageMagick and GNU
`timeout` are required only by the shell robustness suites.

## Basic usage

```sh
pixseal embed \
  -in photo.png \
  -out sealed.png \
  -key "a long secret" \
  -message "hidden message" \
  -profile auto
```

A successful embed reports the selected profile:

```text
embedded 14 bytes using profile robust in sealed.png
```

Extraction does not require a profile:

```sh
pixseal extract -in sealed.png -key "a long secret"
```

The authenticated payload is written to stdout. Human-readable diagnostics such
as confidence, recovered profile and geometry correction are written to stderr.
For scripts or multiline payloads, use:

```sh
pixseal extract -in sealed.png -key "a long secret" -raw
```

`-raw` writes **only** the authenticated payload bytes to stdout, without an
added newline; decoder diagnostics remain available separately on stderr.

### Strength

The public CLI accepts finite strength values from **4 through 120**. The
default is 24 when `-strength` is omitted. Explicit `0`, `NaN` and infinities
are rejected. The Go API retains `Options.Strength == 0` as the internal
"use default" sentinel for compatibility.

### Profiles

| Profile | Maximum payload | Protected frame | Hamming-coded bits | Tile redundancy |
|---|---:|---:|---:|---:|
| `robust` | 16 B | 32 B | 448 | 2.50x |
| `balanced` | 32 B | 48 B | 672 | 1.67x |
| `capacity` | 64 B | 80 B | 1120 | 1.00x |

`auto` chooses the most robust profile capable of containing the actual payload.
Payload limits are measured in bytes, not Unicode characters.

## Format v3 frame

Every profile has a fixed frame size, but the authentication tag follows the
**actual payload**, not the end of the profile capacity region:

```text
magic(2)
+ version/profile(1)
+ payload length(1)
+ CRC32(4)
+ payload(actual length)
+ HMAC-SHA256 tag(8, truncated)
+ zero padding to the profile frame size
```

The tag offset is therefore:

```text
8 + actual_payload_length
```

The HMAC covers the 8-byte header and actual payload. The completed frame is
whitened, Hamming(7,4)-protected and mapped onto the fixed 35x32 = 1120-position
DCT tile with a deterministic modular stride.

Whitening is **not encryption**. Encrypt sensitive content before embedding if
confidentiality is required.

See [`docs/ALGORITHM.md`](docs/ALGORITHM.md) for the precise specification.

## Analyze and capacity

```sh
pixseal analyze -in photo.png -message "hidden message"
pixseal analyze -in photo.png -bytes 18
pixseal capacity -in photo.png
pixseal capacity -in photo.png -details
```

`analyze` clearly labels deterministic facts and heuristic recommendations. Its
image-detail estimator uses the same white-background alpha flattening as the
encoder. Recommendations are advisory and are not recovery guarantees.

## Image pipeline and safety

Supported decoded input formats are PNG and JPEG; detection is based on file
content rather than filename extension. Output is always PNG.

If the requested output has no extension, `.png` is appended. If it has another
extension, it is replaced with `.png`. Unix dotfiles are handled as basenames:
`.sealed` becomes `.sealed.png`, not `.png`.

Output safety rules:

- without `-force`, any existing directory entry is protected;
- `-force` can replace **regular files only**;
- directories, symlinks and other special files are rejected;
- no-clobber publication uses an atomic hard-link commit when supported, with an
  exclusive-create fallback where hard links are unavailable;
- newly created files respect the process umask;
- replacing an existing regular file preserves its permission bits;
- the temporary file is `Sync()`ed before commit.

On Windows the fallback replacement path uses backup-and-restore because
`os.Rename` cannot always replace an existing file. This reduces replacement
risk but is not claimed to provide filesystem-level crash durability; directory
`fsync` is not attempted.

### Image-size policy

Before decoding pixel data, the CLI uses `image.DecodeConfig` and rejects inputs
above **300,000,000 decoded pixels**. The core applies the same working-image
limit before allocating its compact extraction pixel plane. This keeps the
verified ~200 MP class usable while avoiding obviously unbounded allocations.

Some **generated geometry candidates** use the stricter 50,000,000-pixel search
bound. Such candidates are skipped rather than materialized.

The v0.3 research line may replace the full pixel plane with a lazy/tiled/sparse
representation; that architectural change is intentionally not part of v0.2.0.

## Decoder order

The current v3 extractor is bounded and ordered so speculative geometry does not
suppress established recovery paths:

1. direct grids at apparent block sizes 8, 6 and 4 pixels, including pixel phase;
2. gated lossless quarter turns (90/180/270 degrees);
3. pure isotropic fractional-scale recovery through virtual affine sampling;
4. one targeted physical bilinear normalization fallback when the scale evidence
   is decisive;
5. early axis-aligned affine recovery when zero-degree evidence supports it;
6. two mild projective/vertical-keystone hypotheses;
7. fixed direct lattice-basis composition bank;
8. arbitrary-angle rotation estimation and rectification;
9. final axis-aligned affine fallback when rotation evidence is weak.

HMAC authentication is always the final success criterion.

### Pure fractional resize budget

The non-integer direct-scale path has:

- 13 fixed scale hypotheses: 95, 90, 85, 80, 70, 65, 60, 55, 45, 40, 35, 30, 25%;
- up to 3 candidate phases per scale for single-tile probing;
- at most 6 shortlisted scales x 3 phases = **18 full-carrier virtual grids**;
- at most one decisive physical normalization candidate, with the base target
  size plus its eight ±1-pixel neighbours.

100%, 75% and 50% are already handled directly by apparent block sizes 8, 6 and
4 and are not duplicated in that list.

### Experimental geometry budgets

Arbitrary rotation:

- 2 zero-degree probes (8/6 px);
- at most 720 non-zero coarse probes;
- at most 33 fine probes;
- at most 2 refined angle candidates reach rectification.

Fixed direct lattice bank:

- 4 basis shapes: 110x90, 90x110, 105x95, 95x105%;
- 361 angles per shape (-45..+45 at 0.25 degree);
- **1444 sparse probes** maximum;
- at most **48 candidates per shape / 192 total** receive stronger coherence
  evaluation;
- at most 4 matrices x 3 phases per shape group = **12 full authenticated grids per group**,
  with two sequential groups (±10% then ±5%) for a **24-grid overall worst case**.

Mild perspective:

- exactly 2 fixed vertical-keystone hypotheses (`top-narrow-4`,
  `bottom-narrow-4`);
- one aligned phase each;
- at most **2 projective full-grid aggregations**.

These budgets describe the implemented search space; they are not statements of
universal recovery probability.

## Metadata and color/orientation limitations

PixSeal works on decoded pixels and outputs 8-bit NRGBA flattened against white.
Consequently:

- 16-bit PNG input becomes 8-bit output;
- EXIF, XMP, comments and ICC/profile metadata are not preserved;
- Go's JPEG decoder does not automatically apply EXIF Orientation, so a JPEG
  shown rotated by a metadata-aware viewer may be processed according to its
  stored pixel orientation;
- losing an ICC profile can change appearance in color-managed workflows, not
  merely remove descriptive metadata.

EXIF-orientation normalization and color-management preservation are deferred to
v0.3 because they require an explicit image-pipeline policy.

## Tests

```sh
make                 # build only
make test            # complete Go tests + local image round trips
make release-unit    # release-gate Go regressions only
make research-unit   # experimental geometry Go regressions
make deep-test       # stable JPEG/resize/crop baseline
make extreme-test    # non-strict progressive resize/crop limit map
make geometry-test   # experimental rotation/combined geometry
make affine-test     # experimental axis-aligned affine matrix
make composition-test # historical composed regression
make lattice-test     # direct lattice-basis regression
make perspective-test # two-hypothesis mild projective regression
make core-target-check # reusable core: Linux/Windows/Android/iOS
make version-check    # VERSION vs buildinfo consistency
make release-check    # release baseline gate
make all-test         # every suite sequentially + summary
```

`extreme-test` is intentionally **non-strict**: individual FAIL rows identify
measured limits and do not make the target fail. `deep-test`, `geometry-test`,
`affine-test`, `composition-test`, `lattice-test` and `perspective-test` support
strict mode, and `make all-test` invokes those suites with strict mode by default.

`make release-check` is self-contained and forces strict semantics for the stable
`deep-test` baseline. It includes `release-unit`, private-corpus round trips and
core portability while excluding `research-unit`. `make all-test` keeps research
failures visible separately from the release baseline and reports partial target
sets as `PARTIAL`/`NOT RUN` rather than as a full qualification PASS.

The local shell suites use `original pics/`, which is private and excluded from
source archives. An empty test corpus is an error rather than a false PASS.

The full shell qualification harness is intended for **Linux/WSL with Bash >= 4,
GNU-compatible userland (including `timeout` and `sort -z`) and ImageMagick**.
This harness requirement is separate from reusable-core portability.

## Current limitations

- Maximum v3 payload: 64 bytes.
- Minimum aligned embed geometry: 280x256 pixels.
- CLI embedding currently accepts text via `-message`; the core stores bytes.
- General affine inference, arbitrary homography estimation and print-camera
  recovery are not v0.2.0 guarantees.
- Experimental rotation/affine performance is image-content dependent.
- Failed extraction can be substantially more expensive than successful
  extraction because bounded candidates must be exhausted.
- No neural model is used.
- No professional cryptographic or steganalytic audit has been performed.

## Release status

**v0.2.0-rc4** is a reconciliation/hardening candidate for the v0.2.0 final
release. It preserves the Format v3 encoder and RC3 filesystem hardening while
restoring RC2 release/research separation, strict release qualification, raw
stdout/stderr scripting semantics and accurate RC1→RC2→RC3 history.

Qualification results are recorded in [`docs/RESULTS.md`](docs/RESULTS.md).

## License

MIT. See [`LICENSE`](LICENSE).
