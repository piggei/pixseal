# PixSeal robust steganographic format

## Version 2 overview

PixSeal v2 is a small, classical robust image-steganography format. It separates
payload protection from geometric synchronization, then repeats one complete
protected frame as a two-dimensional DCT tile.

The decoder searches the possible block-grid offset and tile phase, so it does
not need the original image dimensions or crop origin. The implementation is
pure Go and does not require a neural model.

The objective is to make a small hidden payload visually unobtrusive and
recoverable after a limited set of common image transformations. This is not a
claim of statistical steganographic undetectability.

## Authenticated frame

The unencoded v2 frame is always 80 bytes (640 bits):

| Offset | Size | Field |
|---:|---:|---|
| 0 | 2 | Magic `PS` |
| 2 | 1 | Format version (`2`) |
| 3 | 1 | Payload length, 0-64 |
| 4 | 4 | Payload CRC32, big endian |
| 8 | 0-64 | Payload |
| variable | 8 | Truncated HMAC-SHA256 |
| remaining | variable | Zero padding before whitening |

The frame bits are XOR-whitened with a SHA-256 stream derived from the key and
the domain label `pixseal-whiten-v2`. Hamming(7,4) then expands 640 bits to 1120
protected bits and corrects one damaged bit in each seven-bit codeword.

CRC detects accidental corruption. HMAC authenticates the recovered payload and
prevents a synchronization candidate from being accepted unless the protected
frame validates with the supplied key.

Whitening is not encryption. A confidential payload should be encrypted before
it is passed to PixSeal.

## Periodic DCT tile

The 1120 protected bits fill a row-major 35x32 tile. Every complete 8x8 block in
the source image embeds the bit at:

```text
tileX = blockX mod 35
tileY = blockY mod 32
bit   = protected[tileY*35 + tileX]
```

Each bit is represented by the relative magnitudes of DCT coefficients `(3,2)`
and `(2,3)` in the luminance channel. The default strength enforces a minimum
magnitude difference of 24 between the selected pair.

Repeating the tile gives multiple spatial observations of each protected bit and
makes the payload layout independent of the complete source dimensions.

One aligned v2 tile occupies:

```text
35 * 8 = 280 pixels horizontally
32 * 8 = 256 pixels vertically
```

A source image therefore needs to be at least 280x256 pixels for embedding.

## Extraction and synchronization

The decoder first converts the source into a compact 8-bit RGB pixel plane. This
conversion is performed once and reused by the offset searches, avoiding repeated
`image.Image` traversal and color-model conversion for every candidate grid.
Luminance is then computed directly from the cached bytes while evaluating DCT
coefficients.

It then searches apparent block sizes 8, 6 and 4, corresponding to 100%, 75% and
50% scaling of the embedded 8x8 grid. For each apparent block size it searches
all possible pixel offsets within the block.

For each viable grid it:

1. accumulates DCT coefficient differences modulo the 35x32 periodic tile;
2. evaluates every cyclic tile phase against a keyed protected prefix containing
   the magic bytes and v2 format version;
3. keeps the strongest phase candidates;
4. Hamming-decodes and unwhitens complete frame candidates;
5. accepts a payload only after CRC and HMAC validation.

If the direct 8/6/4-pixel searches do not recover a v2 payload, the decoder tries
bicubic normalization for the remaining supported nominal scales from 95% down
to 25% in five-percentage-point increments. The 75% and 50% cases are omitted
from this second list because they already have direct 6- and 4-pixel paths.

Normalization operates on the cached RGB plane and writes another compact pixel
plane rather than constructing an intermediate Go image. Its channel-wise bicubic
interpolation and clamping are equivalent to the earlier image reconstruction path,
then luminance is evaluated directly from the normalized cached bytes.

For each normalized candidate the decoder assumes that a pure resize preserved
the top-left grid origin, aggregates an aligned 8x8 grid and ranks the nominal
scale by synchronization-prefix agreement. Image tools can round width and
height independently, so the decoder also checks the eight adjacent dimension
pairs for the three strongest nominal scale candidates.

This deliberately bounds failed-extraction work instead of performing an
exhaustive normalized offset search.

### Internal normalization bound

A reconstructed normalization candidate is skipped when its dimensions would
exceed 50,000,000 pixels. This is an implementation safety bound for memory and
failed-extraction cost; it is separate from any limit used by the shell test
harness.

Consequently, the statement that PixSeal searches every supported nominal scale
has an important qualification: a scale is searched only when its reconstructed
candidate fits inside this 50-million-pixel bound.

## Cropping and block alignment

A crop may start at an arbitrary source pixel. The tile phase search compensates
for losing whole blocks, while the pixel-offset search compensates for a crop
starting between block boundaries.

The aligned tile dimensions are:

| Apparent block size | Approx. scale | Aligned tile |
|---:|---:|---:|
| 8 | 100% | 280x256 |
| 6 | 75% | 210x192 |
| 4 | 50% | 140x128 |

Those dimensions are sufficient only when the retained crop happens to align so
that a complete block grid begins immediately. To **guarantee** a complete tile
for every possible pixel offset, the crop must also allow up to `blockSize - 1`
pixels to be discarded before the first complete block:

| Apparent block size | Guaranteed width | Guaranteed height |
|---:|---:|---:|
| 8 | 287 | 263 |
| 6 | 215 | 197 |
| 4 | 143 | 131 |

For example, a 280x256 crop at 100% can succeed when aligned to the original
8-pixel grid, while a 280x256 crop beginning one pixel off-grid does not contain
35x32 complete 8x8 blocks. A 287x263 crop guarantees enough room regardless of
that offset.

These are geometric containment bounds, not robustness guarantees: image
content and previous transformations can still prevent recovery.

## Compatibility with v1

New embeds use format v2. Extraction always tries v2 first and then falls back
to the original dimension-dependent v1 decoder.

The CLI `-repetition` setting belongs only to the v1 fallback. It is ignored by
v2 embedding and v2 capacity, and an invalid v1 repetition value cannot prevent
an otherwise valid v2 payload from being recovered. The value is validated only
if v2 extraction fails and the decoder actually needs to attempt v1.

The test suite includes a generated v1 fixture path to ensure that legacy
extraction remains functional as v2 evolves.

## Pixel pipeline

PixSeal embeds into decoded image pixels and writes a new PNG carrier. The
current output representation is 8-bit NRGBA with alpha flattened against white.
As a result:

- 16-bit PNG input is reduced to 8-bit output;
- EXIF, XMP, PNG comments, ICC/profile metadata and similar container metadata
  are not preserved by the current decode/re-encode path;
- output from `embed` is always PNG to avoid an immediate lossy encoding step.

These are format-pipeline properties, not properties of the v2 payload frame.

## Current boundaries

- Maximum payload is 64 bytes.
- Automatic scale normalization currently covers nominal 25%-100% scaling in
  5% increments, subject to the internal 50-million-pixel reconstruction bound.
- Scaling factors between search steps are not guaranteed.
- Fractional resizing combined with an off-grid crop is not guaranteed in v0.1,
  because the normalized path does not exhaustively search pixel offsets.
- Rotation and perspective correction require a stronger synchronization layer.
- Very small crops can no longer contain a complete protected tile.
- Failed extraction can take materially longer than successful extraction because
  more candidate grids and normalization scales must be exhausted.
- Robustness trades against visual impact. The default v2 strength is 24.
- The format has not received a cryptographic or steganalytic audit.

## Design background

PixSeal began from an interest in the general capability illustrated by systems
such as Google SynthID: hiding machine-readable information in visual media while
retaining useful robustness after transformations. PixSeal does not implement,
clone or reverse-engineer SynthID. Its current format is an independent,
classical frequency-domain design built from DCT coefficient modulation,
periodicity, keyed synchronization, Hamming error correction, CRC and HMAC.

The synchronization and robustness literature around digital watermarking is
still directly relevant to this engineering problem even though PixSeal's
product goal is framed as robust steganography rather than ownership marking.
Useful background includes:

- S. Pereira and T. Pun, "Rotation, scale, and translation resilient watermarking
  for images," IEEE Transactions on Image Processing, 2001.
- H.-L. Li et al., "Resampling-Detection-Network-Based Robust Image Watermarking
  against Scaling and Cutting," Sensors 23(19), 8195, 2023,
  doi:10.3390/s23198195.

Measured results for the current implementation and test corpus are recorded in
[RESULTS.md](RESULTS.md). They are empirical regression data, not guarantees for
unseen images.
