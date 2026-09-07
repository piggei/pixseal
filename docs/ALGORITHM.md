# PixSeal watermark format

## Version 2 overview

PixSeal v2 separates payload protection from geometric synchronization. The
encoder repeats one complete protected frame as a two-dimensional DCT tile. The
decoder searches the possible block-grid offset and tile phase, so neither the
original image dimensions nor the crop origin are required.

The implementation remains pure Go and does not require a neural model.

## Authenticated frame

The unencoded frame is always 80 bytes (640 bits):

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
bits and corrects one damaged bit in each seven-bit codeword.

CRC detects accidental corruption. HMAC authenticates the recovered payload and
prevents a false synchronization candidate from being accepted.

## Periodic DCT tile

The 1120 protected bits fill a row-major 35x32 tile. Every complete 8x8 block in
the source image embeds the bit at:

```text
tileX = blockX mod 35
tileY = blockY mod 32
bit   = protected[tileY*35 + tileX]
```

Each bit is represented by the relative magnitudes of DCT coefficients `(3,2)`
and `(2,3)` in the luminance channel. Repeating the tile gives multiple spatial
observations of each protected bit and makes the layout independent of the full
image width and height.

## Extraction and synchronization

The decoder first searches apparent block sizes 8, 6 and 4, corresponding to
100%, 75% and 50% scaling of the embedded 8x8 grid. This direct path searches
all pixel offsets inside the apparent block. It then tries bicubic scale
normalization from 95% down to 25% in five-percentage-point increments. For each
nominal normalized candidate it:

1. assumes the top-left grid origin preserved by a pure resize;
2. accumulates coefficient differences modulo the 35x32 tile;
3. evaluates every cyclic tile phase against the keyed, protected magic/version
   prefix;
4. fully decodes the strongest phase candidates;
5. accepts a candidate only after CRC and HMAC validation.

Since image tools round width and height independently, the decoder ranks the
nominal scale candidates by synchronization-prefix agreement. It checks the
eight immediately adjacent dimensions only for the three strongest candidates.
This handles one-pixel rounding differences while bounding failed-extraction
work. It deliberately does not run an exhaustive normalized offset search.

A crop may therefore start at an arbitrary pixel. It must retain at least one
complete logical tile: 280x256 pixels at the original scale, 210x192 at 75%, or
140x128 at 50%.

## Compatibility

New embeds use format v2. Extraction tries v2 first and falls back to the
dimension-dependent v1 decoder. The CLI `-repetition` option is retained for v1
decoding and is not part of v2 capacity or error correction.

## Current boundaries

- Maximum payload is 64 bytes.
- Automatic scale normalization currently searches 25%-100% in 5% increments.
- Scaling factors between search steps, a fractional resize combined with an
  off-grid crop, rotation, perspective correction and very small crops require
  a stronger synchronization layer.
- Robustness trades against visibility. The default v2 strength is 24.
- This experimental format has not received a cryptographic or steganalytic
  audit.

## Design background

The separation between payload embedding and a synchronization/template signal
is a standard approach for geometric robustness. PixSeal v2 uses a deliberately
small classical implementation rather than reproducing a proprietary or neural
watermarking system.

Measured results for the current implementation and test corpus are recorded in
[RESULTS.md](RESULTS.md). They are empirical regression data, not guarantees for
unseen images.

- S. Pereira and T. Pun, "Rotation, scale, and translation resilient watermarking
  for images," IEEE Transactions on Image Processing, 2001.
- H.-L. Li et al., "Resampling-Detection-Network-Based Robust Image Watermarking
  against Scaling and Cutting," Sensors 23(19), 8195, 2023,
  doi:10.3390/s23198195.
