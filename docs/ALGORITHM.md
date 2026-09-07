# PixSeal algorithm specification

This document describes the adaptive **format v3** used by PixSeal
v0.2.0 build 2. Build 2 is intentionally v3-only at runtime; formats v1 and v2
are retained only as historical development context.

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

### 10.1 Direct block-size/offset search

The decoder directly tests apparent block sizes:

```text
8 pixels
6 pixels
4 pixels
```

These provide fast direct paths for the original scale and the important 75%
and 50% cases. For each size, every pixel offset inside one apparent block is
bounded and tested:

```text
8x8 offsets = 64 grids
6x6 offsets = 36 grids
4x4 offsets = 16 grids
maximum direct grids = 116
```

A size is skipped if the candidate image cannot contain a 35x32 logical tile.

For every aggregated grid, the decoder checks the three v3 profile patterns.
The profile probes are fixed and bounded and do not require separate DCT grids.

### 10.2 Inverse scale normalization

If direct extraction fails, the image is bicubically reconstructed toward the
nominal original dimensions for these scale hypotheses:

```text
95, 90, 85, 80, 70, 65, 60, 55, 45, 40, 35, 30, 25 percent
```

There are therefore at most 13 aligned normalization candidates.

Each normalized candidate uses the aligned 8x8 fast path. The three nominal
scales with the strongest synchronization evidence may additionally receive the
eight neighboring `(width +/- 1, height +/- 1)` dimension combinations, for at
most 24 further reconstructions.

Before size-bound skips, the v3 search is consequently bounded by:

```text
116 direct grids
+ 13 aligned normalized candidates
+ 24 neighboring normalized candidates
= 153 geometric candidates maximum
```

Profile detection adds fixed scoring work inside a grid but does not multiply
the 153-candidate geometric space.

A normalized candidate is skipped if reconstruction would exceed
**50,000,000 pixels**. This bounds memory and failed-extraction work for very
large images.

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
carrier population or interoperability commitment. Build 2 removes their runtime
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

Build 2 uses the mean absolute luminance-gradient score:

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

Build 2 does not add synchronization for:

- arbitrary rotation;
- perspective transforms;
- arbitrary affine warps;
- all possible resize factors;
- exhaustive combined fractional resize plus off-grid crop.

The next research step is arbitrary digital rotation, then combined
rotation/resize/crop, affine deformation and perspective correction. The intended
approach is staged, bounded synchronization and geometric rectification rather
than multiplying brute-force dimensions. A later experimental target is recovery
through a print-camera channel (print the carrier, photograph it with a phone,
then recover the authenticated v3 payload). None of those capabilities is part of
build 2. Any implementation must keep failed extraction explicitly bounded.
