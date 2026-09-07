# PixSeal algorithm specification

This document describes the adaptive **format v3** used by PixSeal
v0.2.0 build 4. Runtime extraction is intentionally v3-only; formats v1 and v2
are retained only as historical development context. Builds 3 and 4 add bounded
geometric recovery around the unchanged v3 on-image format.

PixSeal is an experimental robust image-steganography system for short
authenticated payloads. The frequency-domain embedding mechanism is related to
digital watermarking techniques, but ownership marking is not the required use
case.

## 1. Fixed geometric carrier

PixSeal format v3 uses the following logical DCT geometry:

- source embedding block: 8x8 pixels;
- logical tile: 35x32 blocks;
- positions per logical tile: 1120;
- aligned source tile: 280x256 pixels;
- DCT coefficients compared: `(u=3,v=2)` and `(u=2,v=3)` in the implementation's
  row/column convention.

Keeping the 1120-position tile unchanged lets v3 reuse and refine the geometric
synchronization and scale-normalization work developed during v0.1.

## 2. Adaptive v3 profiles

v3 defines three concrete on-image profiles. `auto` is only an encoder policy;
it is never stored as a fourth profile.

| Profile | ID | Maximum payload | Frame bytes | Raw bits | Hamming-coded bits | Tile redundancy |
|---|---:|---:|---:|---:|---:|---:|
| `robust` | 1 | 16 | 32 | 256 | 448 | 1120/448 = 2.50x |
| `balanced` | 2 | 32 | 48 | 384 | 672 | 1120/672 = 1.6667x |
| `capacity` | 3 | 64 | 80 | 640 | 1120 | 1.00x |

Automatic selection is deterministic:

```text
1..16 bytes  -> robust
17..32 bytes -> balanced
33..64 bytes -> capacity
>64 bytes    -> reject
```

The core can represent a zero-length frame, but the current CLI deliberately
rejects an empty `-message`; normal user payloads therefore begin at one byte.

The 16/32/64 thresholds are not arbitrary padding targets. With the existing
8-byte header, 8-byte authentication tag and Hamming(7,4), they create protected
frame sizes that fit the existing 1120-position tile at useful redundancy
ratios without changing geometry.

## 3. v3 frame layout

The v3 frame keeps an 8-byte header:

| Byte(s) | Meaning |
|---|---|
| 0..1 | ASCII magic `P`, `S` |
| 2 | packed format/profile byte |
| 3 | actual payload length in bytes |
| 4..7 | CRC32-IEEE of the actual payload |
| 8.. | payload bytes, followed immediately by the authentication tag |
| remaining bytes | zero padding to the fixed profile frame size |

### 3.1 Packed format/profile byte

Byte 2 packs the format version and adaptive profile into one byte:

```text
high nibble = 0x3   (format v3)
low nibble  = profile ID
```

Concrete values are therefore:

```text
0x31 = v3 robust
0x32 = v3 balanced
0x33 = v3 capacity
```

This avoids increasing the header from 8 to 9 bytes and is the reason the
16/32/64 profile capacities retain the clean 32/48/80-byte frame sizes.

### 3.2 CRC32

Bytes 4..7 contain CRC32-IEEE over the **actual payload bytes only**. CRC is used
as an inexpensive damage check and is not an authentication mechanism.

### 3.3 HMAC

The tag is the first 8 bytes (64 bits) of HMAC-SHA256 using the user key.

The HMAC input is:

```text
header || actual payload
```

The tag is placed immediately after the actual payload. Bytes after the tag are
padding and are not authenticated or required by `parseV3Frame` once the
payload/tag have decoded correctly. This intentionally avoids making damage to
unused profile capacity a reason to reject an otherwise intact message.

The packed format/profile byte and payload length are inside the authenticated
header.

A 64-bit truncated tag is a format/design trade-off of the current
short-message prototype. PixSeal has not received a cryptographic audit.

## 4. Whitening

The complete fixed-size profile frame, including unused trailing padding, is
converted to bits and XORed with a deterministic SHA-256-derived bit stream.

The v3 domain-separation label is:

```text
pixseal-whiten-v3
```

The stream is derived from:

```text
SHA256(label || key || big_endian_counter)
```

with counter values starting at zero until enough stream bits have been
produced.

Whitening is keyed randomization of the embedded representation. **It is not
encryption and does not provide a reviewed confidentiality guarantee.**
Sensitive content should be encrypted before embedding.


## 5. Hamming(7,4)

Whitened frame bits are protected with Hamming(7,4). Each four data bits become
seven coded bits. The current decoder corrects a single flipped bit in each
7-bit codeword.

The resulting coded lengths are exactly:

```text
robust:   256 raw bits * 7/4 = 448 coded bits
balanced: 384 raw bits * 7/4 = 672 coded bits
capacity: 640 raw bits * 7/4 = 1120 coded bits
```

## 6. Mapping the protected frame into the tile

Earlier experimental formats used a direct position mapping. v3 instead distributes
short-profile repetitions over the complete tile.

For logical tile position `p` in `[0,1119]`, the v3 coded-bit index is:

```text
codeIndex = (p * 251) mod codedBits(profile)
```

The fixed stride **251** is coprime with all three coded lengths:

```text
gcd(251, 448)  = 1
gcd(251, 672)  = 1
gcd(251, 1120) = 1
```

This has two useful properties:

1. every coded bit is visited before a coded-length cycle repeats;
2. the extra observations of short profiles are spatially dispersed through
   the 35x32 tile instead of being placed as contiguous frame copies.

Observation counts in one complete logical tile are deterministic:

- `robust`: every coded bit has 2 or 3 tile observations, average 2.50;
- `balanced`: every coded bit has 1 or 2 observations, average 1.6667;
- `capacity`: every coded bit has exactly 1 observation.

The mapping is not a new search variable. It is a fixed v3 format rule.

## 7. DCT embedding

For each complete 8x8 source block, PixSeal converts RGB to luminance/chroma and
computes an 8x8 DCT of luminance. Two mid-frequency coefficients are compared.

Let:

```text
a = abs(DCT[2][3])
b = abs(DCT[3][2])
```

For protected bit `1`, embedding enforces approximately:

```text
a - b >= strength
```

For protected bit `0`, it enforces approximately:

```text
b - a >= strength
```

Only the necessary coefficient magnitude is increased; its existing sign is
preserved. The default strength remains **24**. Valid explicit values are 4 to
120.

The inverse DCT reconstructs luminance and the original per-pixel chroma is used
to reconstruct 8-bit RGB output. Alpha is flattened by the current pixel
pipeline.

## 8. Periodicity across the image

For source block coordinates `(blockX, blockY)`, logical tile position is:

```text
tileX = blockX mod 35
tileY = blockY mod 32
p     = tileY * 35 + tileX
```

v3 then applies the modular coded-bit mapping from section 6.

Images larger than one logical tile therefore provide additional observations
of each logical tile position. The decoder sums those observations before
making bit decisions.

For analysis purposes, the arithmetic mean number of source-block observations
per protected coded bit in an untransformed carrier is:

```text
floor(width/8) * floor(height/8) / codedBits(profile)
```

This is a deterministic geometric average, not a guarantee that all coded bits
have equal signal quality after image processing.

## 9. v3 synchronization and automatic profile recognition

Extraction is given the key but not a profile.

For each concrete v3 profile, the decoder knows the first three logical bytes:

```text
'P', 'S', packed-v3/profile-byte
```

Those 24 known bits are whitened with the v3 stream and Hamming-encoded to 42
known coded bits. The profile's tile mapping tells the decoder which logical
tile positions should carry those known coded bits.

In one complete tile, the current sync scorer therefore obtains:

- 106 known tile observations for `robust`;
- 69 for `balanced`;
- 42 for `capacity`.

For each possible 35x32 tile phase, agreement with those known observations is
scored. Only the strongest four phase candidates for a profile are retained,
and a candidate must have at least 75% sync agreement before full frame parsing
is attempted.

For an accepted v3 phase:

1. all 1120 logical tile values are remapped to the profile's coded indices;
2. repeated values for the same coded bit are summed (soft combining);
3. the sign of each combined value becomes the coded bit decision;
4. Hamming(7,4) is decoded;
5. whitening is reversed;
6. magic/version/profile/length are validated;
7. CRC32 is checked;
8. HMAC is verified.

A profile is accepted only after the authenticated frame validates. The user
does not supply profile information during extraction.

## 10. v3 geometric search is bounded

Adaptive profile detection does **not** add another geometric search dimension.
Each expensive grid aggregation is performed once and then scored against the
three fixed v3 profile patterns.

Build 4 extends the explicitly bounded rotation stage to recognize the native
8-pixel lattice and the 6-pixel lattice produced by a 75% resize. It does not
change the on-image format.

### 10.1 Direct block-size/offset search

The decoder first tests apparent block sizes:

```text
8 pixels
6 pixels
4 pixels
```

These provide fast direct paths for the original scale and the important 75%
and 50% cases. For each size, every pixel offset inside one apparent block is
tested:

```text
8x8 offsets = 64 grids
6x6 offsets = 36 grids
4x4 offsets = 16 grids
maximum direct grids = 116
```

A size is skipped if the candidate image cannot contain a 35x32 logical tile.
For every aggregated grid, the decoder checks the three v3 profile patterns.

### 10.2 Exact quarter-turn recovery

If direct decoding fails but the best v3 synchronization score is sufficiently
high, build 4 tests lossless 90, 180 and 270 degree corrections. Quarter turns
preserve pixel values, so no interpolation is needed.

Each corrected orientation now reuses all three direct apparent block sizes:

```text
8x8 offsets = 64 grids
6x6 offsets = 36 grids
4x4 offsets = 16 grids
116 grids/orientation
```

The gated quarter-turn branch therefore adds at most:

```text
3 orientations x 116 grids = 348 grids
```

This preserves quarter-turn compatibility with the existing 100%, 75% and 50%
direct resize paths without adding a new scale search.

### 10.3 Arbitrary-angle multi-lattice orientation probe

A naive full decode for every angle and every scale would multiply failed-
extraction cost. Build 4 therefore performs sparse signal estimation first and
limits arbitrary-angle probing to the two lattice sizes that remained useful in
development tests after interpolation:

```text
8 pixels -> native / 100% lattice
6 pixels -> 75% lattice
```

The 4-pixel / 50% lattice is intentionally not part of arbitrary-angle probing.
At default strength, a 50% resize plus arbitrary rotation introduces enough
interpolation loss that exact-angle rectification still failed in development
checks. Spending search time on that hypothesis would therefore increase cost
without producing a working path.

For each candidate lattice size, embedding's separation between `C(2,3)` and
`C(3,2)` provides an orientation signal. Candidate contrast is divided by the
expected block-area scale before 8-pixel and 6-pixel hypotheses are ranked; this
prevents the naturally larger DCT magnitudes of an 8-pixel block from dominating
a genuine 6-pixel signal.

Before the full sweep, one zero-degree probe per supported lattice checks whether
the image is already strongly aligned. If either is strongly aligned, arbitrary-
angle search is skipped.

The coarse range is rotation modulo 90 degrees:

```text
-45.00 .. +45.00 degrees
step 0.25 degrees
360 non-zero probes per lattice
2 lattices
720 non-zero coarse probes maximum
```

The three strongest distinct coarse peaks are refined locally:

```text
coarse angle +/- 0.25 degrees
step 0.05 degrees
11 refinements per peak
33 refinement probes maximum
```

At most two refined candidates are passed to authenticated decoding. Including
the two zero-degree alignment checks, the orientation stage performs at most
**755 sparse probes**. These are DCT-margin samples, not full payload decodes.

### 10.4 Rectification, scale choice and crop phase

Each refined candidate carries both its correction angle and its detected
apparent block size. The image is rectified once with bilinear interpolation and
the decoder searches all pixel offsets only for that selected lattice size.

The probe angle is modulo 90 degrees, so each rectified candidate may be tested
in up to four lossless quarter-turn orientations to resolve its quadrant. The
worst full-grid contribution remains the native 8-pixel case:

```text
2 refined angles x 4 quadrants x 64 grids = 512 grids maximum
```

For a 75% candidate the corresponding cost is smaller because the selected
6-pixel lattice has only 36 offsets. Crop phase is naturally recovered by the
same offset and 35x32 logical-phase machinery after rectification, which is why
rotation can now be combined with crop without adding a separate crop search
dimension.

The reported `RotationCorrectionDegrees` value is the correction applied to the
carrier, normalized to `[-180, 180)` degrees. A rectification candidate is
skipped if its expanded canvas would exceed **50,000,000 pixels**.

Build 4 has experimentally recovered, using ImageMagick-generated transforms:

```text
rotate -> resize 75%
resize 75% -> rotate
rotate -> crop 80%
crop 80% -> rotate
rotate -> resize 75% -> crop 80%
```

These are measured development baselines, not universal guarantees for every
image or interpolation filter.

### 10.5 Pure-resize inverse normalization

If direct and rotation recovery fail, the original pure-resize path bicubically
reconstructs the image toward the nominal original dimensions for these scale
hypotheses:

```text
95, 90, 85, 80, 70, 65, 60, 55, 45, 40, 35, 30, 25 percent
```

There are therefore at most 13 aligned normalization candidates. The three
nominal scales with the strongest synchronization evidence may additionally
receive the eight neighboring `(width +/- 1, height +/- 1)` dimension
combinations, for at most 24 further reconstructions.

The original crop/resize path remains bounded to:

```text
116 direct grids
+ 13 aligned normalized candidates
+ 24 neighboring normalized candidates
= 153 crop/resize candidates maximum
```

Including every optional build-4 branch, the theoretical maximum number of full
grid aggregations is:

```text
116 native direct grids
+ 348 gated quarter-turn grids
+ 512 arbitrary-rotation rectified grids
+  13 aligned normalized candidates
+  24 neighboring normalized candidates
= 1013 full grid candidates maximum
```

The at-most 755 sparse orientation probes are additional fixed work and are not
included in that full-grid count. Normal positive cases return from an early
candidate and do not exhaust this theoretical maximum.

Arbitrary-angle recovery is currently scale-aware only for the native and 75%
lattices. The inverse-normalization branch is not recursively chained across
arbitrary-angle hypotheses; arbitrary fractional scales and arbitrary-angle +
50% resize remain research work.

## 11. Grid aggregation and crop phase

For a selected apparent block size/offset, DCT evidence from all complete blocks
is folded into the 35x32 logical tile by block coordinates modulo tile width and
height. This allows multiple spatial repetitions in a large carrier to vote on
the same logical positions.

Cropping can change the tile phase. The 35x32 phase search recovers that phase
using the keyed known prefix described above.

One aligned original-scale tile is 280x256 pixels. A crop beginning between
8-pixel boundaries can lose partial leading blocks. The dimensions that
**guarantee** one complete logical tile for any pixel offset are:

| Apparent scale | Aligned tile | Guaranteed for any offset |
|---:|---:|---:|
| 100% | 280x256 | 287x263 |
| 75% | 210x192 | 215x197 |
| 50% | 140x128 | 143x131 |

These are geometry bounds, not robustness guarantees.

## 12. Historical formats v1 and v2

Formats v1 and v2 were experimental development formats with no known external
carrier population or interoperability commitment. Build 2 removed their runtime
decoders and the legacy CLI controls associated with them. The extractor therefore
accepts **format v3 only**.

The version number is intentionally not renumbered. v1 and v2 remain part of
the engineering history that led to the current design:

- v1 explored keyed block ordering and configurable repetition but did not have
  the periodic geometric synchronization used later;
- v2 established the fixed 35x32 / 1120-position periodic tile and the 80-byte
  authenticated frame used by the v0.1 development line;
- v3 introduced authenticated profile identity plus adaptive 16/32/64-byte
  frames whose unused tile capacity becomes additional observations.

Because no known external compatibility requirement exists, carrying v1/v2 decoding
would add code paths, tests and failed-extraction work without protecting user
data. Their behavior remains documented in the changelog and historical v0.1
results rather than in the runtime implementation.

## 13. `analyze` model

`analyze` deliberately distinguishes deterministic format math from heuristics.

### Deterministic fields

The following come directly from dimensions and v3 format constants:

- profile selected from payload byte length;
- profile maximum payload;
- per-tile redundancy ratio;
- average source-block observations per protected bit;
- minimum carrier geometry;
- 50-million-pixel normalization warning.

### Heuristic fields

Image detail is estimated from immediate horizontal/vertical luminance gradients
at a bounded set of sample anchors (approximately no more than 256x256 anchors).
Large images are sampled sparsely, but each gradient remains a one-pixel local
difference.

Build 4 retains the mean absolute luminance-gradient score introduced in build 1:

```text
score < 4      -> low detail    -> recommended strength 20
4 <= score <12 -> medium detail -> recommended strength 24
score >= 12    -> high detail   -> recommended strength 28
```

The rationale is visual masking: highly textured imagery can generally hide a
stronger DCT perturbation than very flat imagery. These thresholds are a first
engineering heuristic, **not a calibrated perceptual model and not a recovery
guarantee**.

### Experimental evidence

Measured crop/resize/JPEG results belong in `RESULTS.md`. They describe exact
corpus/transformation observations and must not be promoted to deterministic
claims.

## 14. Security and steganalysis boundaries

PixSeal provides keyed embedding and authentication, but the following must not
be inferred:

- whitening is not encryption;
- visual unobtrusiveness does not prove statistical undetectability;
- truncated HMAC and the surrounding protocol have not been independently
  audited;
- robustness against the documented regression transformations does not imply
  robustness against arbitrary editing or adversarial removal;
- the v3 profile layout is public and should not be treated as a secret.

Sensitive messages should be encrypted separately before embedding.

## 15. Current unsupported geometry and direction

Build 4 adds experimental recovery for digital rotation plus a first combined
75%-resize/crop baseline. It does not yet synchronize:

- arbitrary-angle + 50% resize at default strength;
- arbitrary fractional scales beyond the detected 75% lattice;
- perspective transforms;
- arbitrary affine warps;
- print-camera distortion.

The next research step is affine deformation and then perspective correction.
The intended approach remains staged, bounded synchronization and geometric
rectification rather than multiplying open-ended brute-force dimensions.

A later experimental target is recovery through a print-camera channel: print
the carrier, photograph it with a phone, geometrically rectify the photograph,
then recover the authenticated v3 payload. Digital rotation and combined
resize/crop are controlled validation stages for orientation, scale and phase;
they are not evidence that the print-camera channel already works.
