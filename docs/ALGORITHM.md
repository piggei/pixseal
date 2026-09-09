# PixSeal v0.2.0 — Format v3 and decoder specification

This document describes the implementation shipped in **v0.2.0-rc4**. Historical
strategies from intermediate builds belong in `HISTORY.md` and are not normative
for the current decoder.

PixSeal is an experimental robust-steganography system for short authenticated
messages. DCT watermarking is the carrier mechanism; it is not an ownership
protocol and is not claimed to be statistically undetectable.

## 1. Constants

```text
DCT source block                 8 x 8 pixels
logical tile                    35 x 32 blocks
logical tile positions          1120
minimum aligned embed geometry  280 x 256 pixels
maximum payload                 64 bytes
header                          8 bytes
authentication tag              8 bytes (HMAC-SHA256 truncated)
ECC                             Hamming(7,4)
```

The two luminance DCT coefficients used for each embedded bit are `(u=3,v=2)`
and `(u=2,v=3)` in the implementation's coefficient indexing. Embedding forces a
minimum absolute-magnitude separation according to the desired bit and selected
strength.

## 2. Adaptive profiles

| Profile | ID | Max payload | Frame bytes | Protected bits | Tile redundancy |
|---|---:|---:|---:|---:|---:|
| robust | 1 | 16 | 32 | 448 | 2.50x |
| balanced | 2 | 32 | 48 | 672 | 1.67x |
| capacity | 3 | 64 | 80 | 1120 | 1.00x |

`auto` is not encoded as a profile. The encoder resolves it to the smallest
capacity class that can hold the actual payload.

## 3. Frame layout

The v3 frame has a fixed size per profile but the tag immediately follows the
**actual payload**:

```text
offset  size  field
0       2     magic "PS"
2       1     version/profile byte (0x30 | profile ID)
3       1     actual payload length in bytes
4       4     CRC32(payload), big endian
8       N     actual payload
8+N     8     HMAC-SHA256(header || payload), truncated to 8 bytes
after   ...   zero padding to the profile frame size
```

Thus:

```text
tagOffset = headerSize + payloadLength
```

The tag authenticates the header and actual payload, including format/profile
identity and payload length. Padding is not included in the MAC input.

CRC32 is an error-detection aid. HMAC is the authentication mechanism. The
64-bit truncated tag provides at most approximately 64 bits of forgery
resistance.

## 4. Whitening and ECC

The complete fixed-size profile frame is converted to bits and XOR-whitened with
a deterministic SHA-256-derived stream labeled `pixseal-whiten-v3` and keyed by
the supplied key. Whitening is not encryption.

The whitened bits are encoded with Hamming(7,4):

```text
32-byte robust frame    -> 448 coded bits
48-byte balanced frame  -> 672 coded bits
80-byte capacity frame  -> 1120 coded bits
```

## 5. Tile mapping

The logical tile always contains 1120 DCT positions. A tile position maps to a
protected-bit index using:

```text
codeIndex = (tilePosition * 251) mod codedBits
```

251 is coprime with all three coded-bit counts. Profiles shorter than 1120 bits
therefore receive repeated observations distributed across the same fixed tile.

The tile itself repeats spatially across the carrier. Extraction aggregates
observations at equal logical tile positions before ECC/frame validation.

## 6. Embedding

1. Validate key length (minimum 8 bytes), profile and payload size.
2. Normalize embedding options; API strength 0 means the default 24.
3. Build and authenticate the v3 frame.
4. Whiten and Hamming-encode it.
5. Flatten source alpha against white and convert to 8-bit NRGBA.
6. For every complete 8x8 block, map its repeated tile position to the protected
   bit and modify the selected DCT coefficient pair.
7. Output remains a lossless in-memory NRGBA image; the CLI writes PNG.

The public CLI accepts only finite strengths from 4 through 120. `NaN`,
infinities and explicit 0 are rejected.

## 7. Image-size safety

The CLI uses `image.DecodeConfig` before decoding and rejects sources above
300,000,000 decoded pixels. The Go core applies the same working-image bound
before creating the compact extraction pixel plane or an embed copy.

The extraction plane stores 3 bytes per pixel in addition to the decoded Go
image. The explicit limit prevents unbounded extra allocation and integer-size
overflow. v0.3 may replace this full plane with sparse/tiled access.

Generated inverse-normalization and rotation candidates additionally obey the
50,000,000-pixel geometric-search bound.

## 8. Direct extraction

The decoder first tests apparent integer DCT block sizes:

```text
8 px -> native / 100%
6 px -> 75%
4 px -> 50%
```

For each size it searches all pixel phases inside one block. Every candidate is
processed through v3 sync/profile probing, Hamming decode, unwhitening, CRC and
HMAC. The profile is inferred from the authenticated v3 header; the user never
provides it during extraction.

## 9. Exact quarter turns

90/180/270-degree rotations preserve the integer pixel lattice and are corrected
with lossless pixel-plane reorientation. Key-independent repeated-tile
coherence gates the expensive quarter-turn attempts on sufficiently large
carriers; small one-tile carriers remain eligible when coherence cannot be
measured.

## 10. Pure fractional resize recovery

The fixed non-direct scale list is:

```text
95 90 85 80 70 65 60 55 45 40 35 30 25 percent
```

For each scale PixSeal uses a virtual affine sampler rather than reconstructing a
full inverse-resized bitmap. `bestAffineCoherence` retains at most three pixel
phases.

Bounded stages:

```text
13 scale hypotheses
<= 3 single-tile authenticated probes per scale
<= 6 shortlisted scale hypotheses
<= 3 full-grid phases per shortlisted scale
=> <= 18 full-carrier virtual aggregations
```

If virtual evidence is decisive but HMAC still fails, exactly one scale may use
a physical **bilinear** normalization fallback. The expected inverse size plus
its eight ±1-pixel neighbours are tried, and any normalization exceeding the
50-million-pixel geometry bound is skipped.

This pure-resize stage precedes speculative arbitrary rotation/lattice recovery.
That ordering is a deliberate regression rule: an unauthenticated advanced
geometry heuristic must not suppress an established recovery path unless there
is deterministic or authenticated evidence that the established path cannot
apply.

## 11. Axis-aligned affine recovery

The fixed affine bank contains 36 hypotheses:

- anisotropic X/Y scale pairs from `{0.90, 0.95, 1.00, 1.05, 1.10}`, excluding
  equal X/Y pairs: 20 matrices;
- X or Y shear at ±3, ±5, ±8 and ±10 degrees: 16 matrices.

The decoder measures key-independent periodic coherence, keeps at most six
matrices for authenticated single-tile probing, retains at most three phases per
matrix and promotes at most four matrix/phase pairs to full-carrier aggregation.

This is a discrete research bank, not general affine inference.

## 12. Mild projective recovery

The v0.2.0 decoder contains exactly two fixed vertical-keystone hypotheses:

```text
top-narrow-4
bottom-narrow-4
```

Each corresponds to a 4% narrowing of one horizontal edge. A homography maps
coordinates in a rectified 8-pixel DCT lattice directly into the observed image;
luminance is sampled bilinearly. No full rectified image is created.

Only phase `(0,0)` is attempted for each hypothesis:

```text
2 homographies x 1 phase = 2 full projective grid aggregations maximum
```

A deterministic Go end-to-end regression verifies that a warped v3 carrier is
recovered **through the projective path** and reports the expected
`PerspectiveCorrection` value. The shell suite additionally verifies the same
path on ImageMagick-generated JPEG/PNG corpus cases.

This is not general perspective support. Horizontal keystone, arbitrary corner
motion, perspective+rotation composition, lens distortion and camera-pose
inference remain future work.

## 13. Direct lattice-basis composition

The fixed bank contains four anisotropic basis shapes:

```text
110%x90%
90%x110%
105%x95%
95%x105%
```

Each is rotated over -45..+45 degrees at 0.25-degree spacing.

Bounded search:

```text
4 shapes x 361 angles = 1444 sparse lattice probes
<= 48 candidates per shape = <= 192 stronger coherence evaluations
<= 4 matrices x <= 3 phases = <= 12 full authenticated grids per group
2 sequential shape groups = <= 24 full authenticated grids overall
```

The ±10% anchor pair is evaluated before the ±5% pair to preserve the fast path
for the earlier validated cases. HMAC remains the sole acceptance criterion.

## 14. Arbitrary rotation

The orientation detector probes 8- and 6-pixel apparent lattices:

```text
2 zero-degree checks
<= 720 non-zero 0.25-degree coarse probes
<= 3 coarse peaks
<= 33 local 0.05-degree fine probes
<= 2 refined candidates reach rectification
```

Each refined candidate may be tested in four quarter-turn quadrants and all
pixel phases for its selected apparent block size. Rectified canvases exceeding
50,000,000 pixels are skipped.

The arbitrary-rotation path is experimental and remains image-content dependent
on the qualification corpus.

## 15. Decoder ordering

The implemented v3 ordering is:

1. direct 8/6/4 grids;
2. gated lossless quarter turns;
3. virtual isotropic fractional scales;
4. optional single-scale physical bilinear fallback;
5. early axis-aligned affine when applicable;
6. two-hypothesis mild perspective;
7. direct lattice-basis bank;
8. arbitrary rotation;
9. final affine fallback when rotation evidence is weak.

The ordering is part of the v0.2.0 performance/regression contract, not part of
the on-image Format v3 interoperability format.

## 16. CLI output and files

Normal extraction writes the payload to stdout and diagnostics to stderr.
`extract -raw` writes only the authenticated payload bytes to stdout without an
added newline; decoder diagnostics remain on stderr.

Output publication rules protect existing directory entries. `-force` applies
to regular files only. New files use mode 0666 subject to the process umask;
replacement preserves existing regular-file permissions. The temporary image is
synced before publication. No-clobber publication uses a hard-link commit when
possible and exclusive-create fallback otherwise.

## 17. Alpha, metadata, EXIF and ICC

Both embedding and `analyze` flatten transparency against white. Output is 8-bit
NRGBA PNG and container metadata is not preserved.

Go's JPEG decoder does not automatically apply EXIF Orientation, and ICC
profiles are not carried into the output. EXIF orientation normalization and
color-management preservation are deliberately deferred to v0.3.

## 18. Security properties and non-properties

PixSeal provides:

- keyed frame whitening;
- CRC32 damage detection;
- HMAC authentication with a 64-bit truncated tag;
- ECC against bounded bit errors.

PixSeal does not provide reviewed encryption, strong key management, statistical
undetectability or resistance to a determined steganalyst. Sensitive messages
should be encrypted before embedding.

## 19. Format history

Formats v1 and v2 were internal experimental formats. Runtime extraction is
v3-only because no external v1/v2 compatibility population existed when the
v0.2 line was consolidated. Historical details belong in `HISTORY.md` and
`CHANGELOG.md`.
