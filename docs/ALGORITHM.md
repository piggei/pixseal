# PixSeal v0.2.0 — Format v3 and decoder specification

This document describes the **current v0.2.0 implementation** used by
`v0.2.0-rc2`. Historical build strategies belong in `HISTORY.md` and are not
part of this normative description.

PixSeal is an experimental robust-steganography system for short messages. The
on-image Format v3 and encoder are frozen for the v0.2.0 line.

## 1. Carrier geometry

PixSeal operates on luminance DCT blocks.

Constants:

```text
native block size:        8 x 8 pixels
logical tile:             35 x 32 blocks
positions per tile:       1120
native minimum carrier:   280 x 256 pixels
maximum payload:          64 bytes
```

Only two DCT coefficients are used for one embedded bit:

```text
C(2,3)
C(3,2)
```

The sign of the coefficients is preserved. PixSeal controls the difference
between their absolute magnitudes.

## 2. Adaptive profiles

Format v3 has three concrete profiles:

| Profile | ID | Max payload | Fixed frame | Hamming-protected bits | Tile redundancy |
|---|---:|---:|---:|---:|---:|
| `robust` | 1 | 16 B | 32 B | 448 | 2.50× |
| `balanced` | 2 | 32 B | 48 B | 672 | 1120/672 ≈ 1.67× |
| `capacity` | 3 | 64 B | 80 B | 1120 | 1.00× |

`auto` is not an on-image profile. It selects the most redundant concrete
profile that can contain the requested payload.

## 3. Exact Format v3 frame layout

Every profile has a fixed frame size. The meaningful fields are serialized as:

```text
+------------------+------------------------+------------------+------------------+
| header (8 bytes) | actual payload (N B)   | HMAC tag (8 B)   | zero padding      |
+------------------+------------------------+------------------+------------------+
```

The tag offset is:

```text
tagOffset = headerSize + payloadLength
```

It is **not** fixed at the end of the profile's maximum payload region.

The remaining bytes of the fixed profile frame remain zero before whitening.

### 3.1 Header

```text
offset  size  field
0       2     magic: 'P', 'S'
2       1     version/profile byte
3       1     actual payload length
4       4     CRC32(payload), big endian
```

The version/profile byte is:

```text
0x30 | profileID
```

where the high nibble identifies v3 and the low bits identify the concrete
profile.

### 3.2 Authentication

The authentication tag is the first 8 bytes of HMAC-SHA256:

```text
HMAC-SHA256(key, header || actualPayload)[0:8]
```

The HMAC therefore covers the header and the actual payload only. The zero
padding after the tag is not part of the HMAC input, although it is part of the
fixed frame that is subsequently whitened and ECC protected.

CRC32 is used as damage detection inside the authenticated frame. It is not a
security primitive.

## 4. Whitening

The complete fixed frame is converted to bits and XORed with a deterministic
key-derived bit stream before ECC.

The bit stream is generated from repeated SHA-256 blocks over:

```text
label || key || counter_be64
```

using the v3 label:

```text
pixseal-whiten-v3
```

Whitening reduces obvious fixed-bit structure. It is **not encryption** and does
not provide confidentiality.

## 5. Hamming(7,4)

Whitened frame bits are encoded with Hamming(7,4).

Frame sizes therefore become:

```text
robust:    32 bytes = 256 bits -> 448 protected bits
balanced:  48 bytes = 384 bits -> 672 protected bits
capacity:  80 bytes = 640 bits -> 1120 protected bits
```

The decoder corrects one bit error per seven-bit Hamming codeword.

## 6. Mapping into the 1120-position tile

Protected bits are mapped into the logical 35×32 tile with:

```text
codeIndex = (tilePosition * 251) mod codedBits
```

251 is coprime with all current protected-frame sizes (448, 672 and 1120).

When `codedBits < 1120`, multiple tile positions map to the same protected bit.
That is the source of profile redundancy.

The tile repeats across the whole usable carrier. Crop robustness comes from
this periodic repetition rather than from storing one unique frame at one fixed
location.

## 7. DCT embedding

For each complete native 8×8 block, PixSeal computes luminance and the DCT.
Let:

```text
a = abs(C(2,3))
b = abs(C(3,2))
```

For bit 1, PixSeal ensures approximately:

```text
a - b >= strength
```

For bit 0:

```text
b - a >= strength
```

Coefficient sign is retained with `math.Copysign`. The image is inverse-DCT
reconstructed and converted back to RGB.

The CLI default strength is 24 and accepts only finite values in 4..120. The Go
API treats `Strength == 0` as its internal request for the default value.

Alpha is composited onto white before embedding.

## 8. Automatic profile recognition

The decoder knows, for each profile, the whitened/Hamming-protected bits that
correspond to the fixed magic/version/profile header prefix. These known bits
form a key-derived sync pattern.

Candidate grids are scored against all concrete profile sync patterns. Only a
frame that subsequently passes frame parsing, CRC and HMAC is accepted.
Geometry scores and sync scores are therefore ranking evidence, not success
criteria.

## 9. Pixel working plane and source-size guard

The v0.2 decoder converts the decoded image into a compact 8-bit RGB
`pixelPlane` for repeated geometric probes.

Before that allocation:

- the CLI calls `image.DecodeConfig` and rejects decoded inputs above 300 MP;
- the public embed/extract core checks the same 300 MP source bound before
  creating PixSeal working images/planes.

The 300 MP source guard is separate from lower geometry-search limits such as
`maxSearchPixels = 50,000,000` used when a particular inverse transform would
materialize a large corrected plane.

The v0.2 implementation is not tiled/lazy; that is future work.

## 10. Decoder order

The implementation order in `watermark/decoder.go` is:

1. direct 8/6/4-pixel integer-grid search;
2. gated exact quarter turns;
3. pure isotropic fractional-scale search;
4. optional one-scale physical normalization fallback;
5. early axis-aligned affine search when zero-degree evidence remains;
6. two-hypothesis mild projective search;
7. fixed direct lattice-basis composition search;
8. arbitrary-angle rotation search;
9. final axis-aligned affine fallback if rotation evidence is weak.

This ordering is deliberate. After earlier development regressions, the project
adopted the rule:

> An unauthenticated advanced geometry heuristic must not suppress an established
> recovery path unless deterministic or authenticated evidence makes that path
> inapplicable.

## 11. Direct integer-grid recovery

The first search evaluates block sizes:

```text
8 pixels  -> native 100% scale
6 pixels  -> exact 75% lattice
4 pixels  -> exact 50% lattice
```

These are cheap paths because no continuous transform needs to be estimated.

## 12. Exact quarter turns

90/180/270-degree rotations preserve an integer lattice and can be corrected
without interpolation.

Tile-period coherence is used as a key-independent gate. The native period is
35×32 blocks; a 90/270-degree carrier exposes the swapped 32×35 period.

Small one-tile carriers for which repetition cannot be measured remain eligible
rather than being interpreted as negative evidence.

## 13. Pure isotropic fractional scale

The fixed hypotheses are:

```text
95, 90, 85, 80, 70, 65, 60, 55, 45, 40, 35, 30, 25 percent
```

100/75/50% are omitted because direct 8/6/4-pixel paths already cover them.

Each scale is evaluated through a **virtual affine sampler**; PixSeal does not
materialize thirteen full normalized images.

Bounded behavior:

- 13 fixed scale hypotheses;
- up to 3 candidate phases per hypothesis for coherence/single-tile probing;
- full-carrier virtual decoding is limited to the best 6 scale hypotheses;
- therefore at most **18 full-carrier virtual aggregations** in that stage;
- if virtual sampling produces decisive sync evidence but no authenticated frame,
  at most one scale can enter physical normalization.

The physical fallback uses bilinear normalization for the selected scale and
tries the rounded target dimensions plus the eight ±1-pixel neighbours. It is a
single targeted fallback, not the old multi-image bicubic sweep.

## 14. Axis-aligned affine recovery

The fixed affine bank contains 36 hypotheses:

- 20 anisotropic X/Y scale pairs built from 90, 95, 100, 105 and 110 percent,
  excluding equal X/Y pairs;
- 16 single-axis shear hypotheses: X or Y at ±3°, ±5°, ±8° and ±10°.

The decoder samples these transforms virtually.

After coherence ranking:

- at most 6 matrices survive the first full candidate stage;
- up to 3 phases per surviving matrix may receive single-tile authenticated
  probes;
- only the best 4 matrix/phase combinations reach full-carrier aggregation.

HMAC remains the final acceptance criterion.

## 15. Mild projective recovery

v0.2 contains exactly two projective hypotheses:

```text
top-narrow-4
bottom-narrow-4
```

They model a 4% vertical keystone with either the top or bottom edge narrowed.
Each uses one aligned phase, so this stage performs at most **2 projective
full-grid aggregations**.

The projective sampler evaluates a homography virtually; no corrected full-size
bitmap is produced.

This is experimental bounded projective recovery. It is **not** general
perspective estimation, arbitrary four-corner homography inference, lens
correction or print-camera support.

## 16. Direct lattice-basis composition bank

The fixed bank describes four anisotropic lattice bases:

```text
110% x 90%
 90% x 110%
105% x 95%
 95% x 105%
```

Each basis is rotated as a unit from -45° to +45° in 0.25° increments.

Deterministic bounds:

```text
4 shapes x 361 angles = 1444 sparse lattice probes
48 shortlisted candidates per shape = 192 stronger coherence evaluations max
```

The two ±10% shapes are evaluated as the anchor group first. If they do not
authenticate, the ±5% group is evaluated.

Per group, at most 4 candidates × 3 phases reach full-carrier authenticated
decoding. Therefore:

```text
positive anchor path:  <= 12 full aggregations
overall worst case:    <= 24 full aggregations across both groups
```

The bank is discrete. v0.2 does not claim continuous/local inference of arbitrary
basis vectors `u` and `v`.

## 17. Arbitrary digital rotation

The standalone rotation detector searches two useful lattice sizes:

```text
8 pixels
6 pixels
```

The coarse search covers -45°..+45° at 0.25° while excluding the near-zero band,
for at most 720 non-zero coarse probes across both sizes.

It keeps at most 3 distinct coarse candidates, refines each over ±0.25° at 0.05°
steps, and returns at most 2 distinct decode candidates.

A selected candidate is rectified into an expanded white-canvas plane with
bilinear interpolation, then tested across the four quarter-turn quadrants.
Large corrected planes are rejected by the geometry search pixel bound.

Arbitrary rotation is experimental and remains texture-dependent on some real
carriers.

## 18. Crop and phase

Because the 35×32 tile repeats, cropping mainly changes which logical tile phase
is visible. Grid decoding searches profile sync phases and aggregates repeated
observations by logical tile position.

This is why crop can remain robust even when a substantial part of the original
image is removed, provided enough complete block/tile information remains.

## 19. `analyze`

`AnalyzeImage` combines:

- deterministic payload/profile/capacity facts;
- deterministic average observations per protected bit on an untransformed
  carrier;
- a bounded local-gradient detail heuristic;
- a heuristic strength recommendation.

The detail estimator sparsely samples at most roughly 256×256 anchor locations.
Transparent pixels are composited onto white exactly as embedding does before
luminance is measured.

The result is advisory and must not be presented as a recovery guarantee.

## 20. Output-file semantics

CLI embedding writes to a temporary file first, encodes the PNG completely,
`Sync()`s it, then commits it.

Without `-force`:

- any existing directory entry is treated as occupied, including symlinks;
- POSIX uses hard-link creation for atomic no-clobber commit;
- Windows relies on `os.Rename` to a non-existing target, which does not replace
  an existing destination.

With `-force`:

- only a regular file may be replaced;
- symlinks, directories, FIFOs and devices are refused;
- permissions of the regular file are preserved;
- the Windows replacement fallback uses backup + rename + restore-on-error.

New files keep the private mode produced by `os.CreateTemp` (normally 0600 on
Unix), rather than being forced to 0644.

The Windows backup sequence is not claimed to be fully crash-durable: a crash
between renames can leave the backup path behind. Directory fsync durability is
also outside the v0.2 contract.

## 21. Raw extraction output

Human-readable `extract` output remains payload followed by diagnostics.

`extract -raw` writes exact payload bytes to stdout and diagnostics to stderr.
Test harnesses use this mode when comparing payloads so embedded newlines do not
become ambiguous with diagnostic lines.

## 22. Metadata and color limitations

Go decodes stored JPEG pixels; EXIF Orientation is not automatically applied.
PixSeal therefore processes stored orientation rather than the orientation a
viewer may display after EXIF interpretation.

Output is a newly encoded PNG and source metadata is not preserved. ICC/color
profiles are not carried through, so losing color management can change visual
appearance as well as metadata.

## 23. Security boundaries

Format v3 provides authenticated recovery, not secrecy.

- HMAC-SHA256 is truncated to 64 bits.
- Whitening is deterministic and is not encryption.
- The minimum accepted key length is 8 bytes, which is a project policy rather
  than a claim that all eight-byte passwords are secure.
- No claim is made of statistical steganographic indistinguishability.
- No professional cryptographic or steganalytic audit has been performed.

Sensitive payloads should be encrypted before embedding.

## 24. Historical formats

Early unreleased engineering versions used formats referred to as v1 and v2.
The v0.2 runtime is intentionally v3-only because those formats had no external
installed base. Their history is retained in `HISTORY.md`; they are not runtime
compatibility formats.

## 25. Release boundary and future research

The v0.2.0 release baseline is digital JPEG/resize/crop recovery plus the Format
v3 core. Rotation, affine, lattice composition and mild projective recovery are
reported research capabilities with corpus-dependent limits.

The planned v0.3 research line focuses on:

- continuous/local lattice-basis inference;
- general bounded homography estimation;
- print → paper → smartphone recovery;
- sparse/tiled processing for very large images;
- explicit EXIF/color-management decisions.

Format v3 remains frozen unless experimental evidence shows that the print-camera
channel cannot be solved decoder-side with the existing signal.
