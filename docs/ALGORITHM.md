# PixSeal algorithm specification

This document describes the adaptive **format v3** used by PixSeal
v0.2.0 build 9. Runtime extraction is intentionally v3-only; formats v1 and v2
are retained only as historical development context. Builds 3 through 8 add
bounded geometric recovery around the unchanged v3 on-image format.

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

Build 7 retains the explicitly bounded rotation stage for the native 8-pixel
lattice and the 6-pixel lattice produced by a 75% resize, and adds a separate
bounded axis-aligned affine stage. None of these decoder additions changes the
on-image format.

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
high, the decoder tests lossless 90, 180 and 270 degree corrections. Quarter turns
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

Build 9 additionally gates exact quarter turns with key-independent tile-period
coherence: native/180-degree geometry repeats as `35x32` blocks, while 90/270
degrees swap the periods to `32x35`. The coherence gate is used only when the
carrier is large enough to contain **two complete logical periods** along at
least one axis. A smaller carrier can still contain one decodable tile; in that
case repetition is classified as *not measurable* rather than as evidence
against the quarter-turn candidate, and the bounded direct-score gate remains
available.

### 10.3 Arbitrary-angle multi-lattice orientation probe

A naive full decode for every angle and every scale would multiply failed-
extraction cost. The rotation stage therefore performs sparse signal estimation first and
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

Builds 4 and 5 have experimentally recovered, using ImageMagick-generated transforms:

```text
rotate -> resize 75%
resize 75% -> rotate
rotate -> crop 80%
crop 80% -> rotate
rotate -> resize 75% -> crop 80%
```

These are measured development baselines, not universal guarantees for every
image or interpolation filter.

### 10.5 Bounded axis-aligned affine recovery

Build 5 adds a separate affine stage after direct/rotation probing and before the
historical pure-resize inverse-normalization path. The stage is intentionally
**axis-aligned**: it does not compose arbitrary rotation with affine distortion.
This avoids an angle x affine Cartesian search while the affine model is still
experimental.

The hypothesis set is finite and fixed:

```text
anisotropic scale values per axis: 0.90, 0.95, 1.00, 1.05, 1.10
all X/Y combinations except X == Y: 20 matrices

shear X or shear Y: +/-3, +/-5, +/-8, +/-10 degrees
2 axes x 8 signed values: 16 matrices

total: 36 affine matrices
```

Uniform X/Y scale pairs are omitted because uniform scaling is already handled
by the established resize paths. Each affine matrix maps coordinates in a
rectified virtual carrier into the observed image. PixSeal samples luminance
through that inverse mapping with bilinear interpolation and computes the normal
DCT coefficient pair directly; it does **not** allocate a corrected full-size
image for every hypothesis.

Before the affine set is explored, the decoder computes native v3 repetition
coherence. A strongly coherent native lattice (`>= 0.82`) means the geometry is
already aligned; if authenticated direct decoding failed, the affine stage is
skipped so wrong-key/native inputs do not pay unnecessary affine work.

For each of the 36 hypotheses, a sparse phase/contrast probe is followed by a
**periodic tile-coherence** check. Format v3 repeats the same 35x32 logical tile
across the carrier, so corresponding logical DCT signs in adjacent tiles should
agree when the candidate geometry is close to correct. Coherence is
key-independent and is used only as a gate/ranking signal; it cannot authenticate
a payload. Candidates below `0.72` coherence are discarded.

The expensive authenticated work is explicitly capped:

```text
36 affine matrices evaluated geometrically
 6 matrices retained after coherence ranking maximum
 3 pixel phases retained per matrix maximum
18 authenticated single-tile probes maximum
 4 strongest failed sync candidates receive full-carrier aggregation maximum
```

For shear hypotheses, axis-preserving pixel phases are added to the phase
candidate set before coherence ranking (X phases for shear X, Y phases for shear
Y). Only the best three phases survive, so this improves phase coverage without
raising the authenticated-probe cap.

Single-tile decoding is attempted first. If HMAC does not validate, at most four
strong candidates aggregate repeated observations across every complete virtual
tile before one final v3 decode. This second level is particularly useful for
the `capacity` profile, where a single transformed tile may be too damaged while
spatial repetition across a larger carrier still contains enough evidence.

When affine extraction succeeds, `ExtractInfo` reports the inverse correction:
`ScaleXCorrection` / `ScaleYCorrection` for anisotropic scale or the relevant
shear correction component. HMAC-SHA256 authentication of the v3 frame remains
the sole acceptance criterion.

The development ImageMagick matrix currently exercises 110%x90%, 90%x110%,
shear X 8 degrees and shear Y 8 degrees across all three profiles. These are
measured regression cases, not continuous guarantees over the complete +/-10%
or +/-10-degree hypothesis range.

### 10.6 Direct lattice-basis estimation for composed anisotropy + rotation

Build 9 keeps the v3 on-image format unchanged and expands the explicit
**lattice-basis bank** introduced in build 8.

A candidate linear carrier geometry is represented by two column vectors:

```text
u = transformed horizontal source-pixel basis
v = transformed vertical source-pixel basis

      [ ux  vx ]
M  =  [        ]
      [ uy  vy ]
```

This representation is independent of CLI edit names. Build 9 still uses a
discrete bank rather than continuously estimating the matrix, but it evaluates
the basis vectors directly instead of trusting a standalone rotation estimate
followed by a scale guess.

The promoted basis shapes are:

```text
A: u=(1.10,0.00), v=(0.00,0.90)   # 110%x90%
B: u=(0.90,0.00), v=(0.00,1.10)   #  90%x110%
C: u=(1.05,0.00), v=(0.00,0.95)   # 105%x95%
D: u=(0.95,0.00), v=(0.00,1.05)   #  95%x105%
```

For an orientation angle `theta`, both vectors are rotated together before the
matrix is scored. The angle range is fixed:

```text
-45 .. +45 degrees, step 0.25 degrees
361 angles per basis shape
4 basis shapes
1444 sparse lattice probes maximum
```

For each matrix the first stage uses 20 deterministic logical tile positions and
compares DCT-sign agreement between adjacent repeated v3 tiles. Pixel phase is
searched on the even 4x4 grid and locally refined by one pixel. The score is
key-independent and can only rank geometry.

Real-corpus testing shows different phase sensitivity for the two anisotropy
magnitudes. Build 9 therefore uses two bounded shortlist sizes:

```text
110%x90% / 90%x110%: 48 quick candidates per shape maximum
105%x95% / 95%x105%: 240 quick candidates per shape maximum

stronger repetition/coherence evaluations maximum:
48 + 48 + 240 + 240 = 576

4 matrices retained for authenticated aggregation maximum
3 pixel phases per retained matrix maximum
12 full-carrier authenticated probes maximum
```

The stronger stage uses the virtual affine sampler from build 5 and measures
periodicity over a larger deterministic subset of the repeated tile. Candidates
below the stronger coherence threshold are discarded before full-carrier
aggregation. HMAC-SHA256 authentication of the recovered v3 frame remains the
sole success criterion. When a basis candidate succeeds, the CLI reports the
inverse rotation and inverse X/Y scale corrections.

The larger moderate-anisotropy shortlist is not an arbitrary performance trade:
reducing it during development caused a real private regression carrier to miss
the correct 105%x95% geometry. The bound is therefore explicit and is tracked as
a negative-case performance cost.

A denser/local refinement prototype for `u` and `v` was also evaluated in build
9. It recovered positive cases but made negative extraction too expensive, so it
was deliberately not promoted. Future continuous estimation must extract richer
geometric information per probe rather than simply increasing candidate density.

The private real photographs used for regression are external development
material and are not part of distributed archives.

### 10.7 Pure-resize inverse normalization

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

Ignoring the separate affine virtual-sampler budget described above, the legacy/rotation branch has the following theoretical maximum number of full
grid aggregations:

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

Build 7 retains the mean absolute luminance-gradient score introduced in build 1:

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

Build 9 retains experimental recovery for digital rotation, the measured
75%-resize/crop combinations, the discrete axis-aligned affine bank and the
first symmetric anisotropic lattice-basis bank. It does not yet synchronize:

- arbitrary-angle + 50% resize at default strength;
- arbitrary fractional scales beyond the detected 75% lattice;
- continuous anisotropic scale + rotation outside the four promoted build-9
  `robust` basis shapes (110%x90%, 90%x110%, 105%x95% and 95%x105%);
- arbitrary rotation composed with X/Y shear;
- continuous affine transforms outside the discrete build-5/build-9 banks;
- perspective transforms;
- print-camera distortion.

The next research step is continuous or locally refined estimation of the two
transformed lattice basis vectors **without a dense negative-case search**, followed
by bounded projective/perspective recovery. The intended approach remains staged, bounded synchronization and
geometric rectification rather than multiplying open-ended brute-force
parameters.

A later experimental target is recovery through a print-camera channel: print
the carrier, photograph it with a phone, geometrically rectify the photograph,
then recover the authenticated v3 payload. Digital rotation, combined
resize/crop and basis-bank tests are controlled validation stages; they are not
evidence that the physical print-camera channel already works.


## Decoder ordering and regression rule (build 10)

Format v3 and the encoder are unchanged. The decoder treats established recovery paths as baselines: an unauthenticated advanced geometry heuristic must not suppress an older path unless deterministic/authenticated evidence makes that path inapplicable.

After direct 8/6/4-pixel grids and quarter-turn handling, pure isotropic resize is tested with a bounded virtual sampler over 95, 90, 85, 80, 70, 65, 60, 55, 45, 40, 35, 30 and 25 percent hypotheses. Strong sync may select one scale for a targeted physical normalization fallback. Affine, lattice-bank and arbitrary-rotation recovery remain later stages.

The encoder is protected by deterministic pixel fingerprints for all three adaptive profiles. These tests are implementation regression guards, not cryptographic commitments.


## Build 11 experimental projective recovery

Build 11 does not alter Format v3 or embedding. After direct, isotropic-resize and axis-aligned-affine recovery, the decoder may test a fixed bank of **two** mild vertical-keystone homographies: 4% top-edge narrowing or 4% bottom-edge narrowing. Each homography maps coordinates in the rectified 8-pixel DCT lattice directly into the observed carrier and samples luminance bilinearly; no intermediate rectified image is allocated. Only phase `(0,0)` is authenticated for each hypothesis, so the new projective stage is bounded at two full projective grid aggregations. CRC32/HMAC validation remains the sole success criterion.

This is an experimental print-camera stepping stone, not general homography estimation. Horizontal keystone, arbitrary corner displacement, projective+rotation composition and camera-lens effects are not claimed by build 11.
