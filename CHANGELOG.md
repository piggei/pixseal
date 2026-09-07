# Changelog

All notable changes to PixSeal are documented here.

## v0.1.0 - 2026-09-07

Initial public release.

### Features

- Pure-Go command-line application with `embed`, `extract` and `capacity`.
- PNG and JPEG input, with lossless PNG output for newly marked images.
- Authenticated payloads up to 64 bytes using CRC32 and truncated HMAC-SHA256.
- Key-derived whitening and Hamming(7,4) error correction.
- Periodic two-dimensional watermark tile embedded in luminance DCT coefficients.
- Recovery after JPEG recompression, supported resizing and off-grid cropping.
- Legacy v1 extraction compatibility.
- Linux-native build plus cross-compilation targets for Windows and macOS.
- Simple, advanced and progressive image-test targets.

### Tested boundaries

- Scale normalization is searched at five-percentage-point intervals from 95%
  down to 25%, with direct paths for 100%, 75% and 50%.
- On the recorded local corpus, resize recovery reached 45% for LQ and 40% for MQ.
- Centered and deterministic random crop recovery reached 40% for LQ and 20% for MQ.

These measurements are corpus-specific and are not guarantees for other images.
See `docs/RESULTS.md` for the complete test context and limitations.
