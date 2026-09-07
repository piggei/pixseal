# Changelog

All notable changes to PixSeal are documented here.

## v0.1.0 - 2026-09-07

Initial public release.

### Features

- Pure-Go command-line application with `embed`, `extract` and `capacity`.
- Robust image steganography for hiding a small authenticated payload in DCT
  luminance coefficients.
- PNG and JPEG input, with lossless PNG output for newly sealed carrier images.
- Authenticated payloads up to 64 bytes using CRC32 and truncated HMAC-SHA256.
- Key-derived whitening and Hamming(7,4) error correction.
- Periodic two-dimensional DCT tile with geometric synchronization.
- Recovery after JPEG recompression, supported resizing and off-grid cropping.
- Legacy v1 extraction compatibility, now covered by an explicit regression test.
- Protected output writes: existing files require `-force`, and PNG encoding is
  completed through a temporary file before replacement.
- Cached pixel/luminance extraction path to reduce repeated work during failed
  synchronization searches without changing the v2 payload format.
- Linux, Windows and macOS cross-compilation targets.
- Self-contained public tests plus optional local-corpus robustness suites.

### Documentation and release hardening

- Project terminology now describes PixSeal as robust image steganography rather
  than limiting it to watermarking.
- The relationship to the original SynthID inspiration is clarified: PixSeal is
  independent and does not implement Google's SynthID algorithm.
- Arbitrary byte payload capability of the core format is distinguished from the
  v0.1 CLI's text-only `-message` interface.
- Crop geometry now distinguishes aligned tile dimensions from worst-case
  off-grid containment guarantees.
- The decoder's 50-million-pixel normalization-candidate bound is documented.
- 8-bit output conversion, alpha flattening and metadata loss are documented.
- `make test` is self-contained; private `original pics/` data is no longer
  required for the public regression target.
- Progressive crop tests now continue after failures so non-monotonic recovery
  is not hidden by a first-failure stopping rule.

### Tested boundaries

- Scale normalization is searched at five-percentage-point intervals from 95%
  down to 25%, with direct paths for 100%, 75% and 50%, subject to the internal
  normalization-size bound.
- On the recorded local corpus, resize recovery reached 45% for LQ and 40% for MQ.
- In the historical first-failure crop run, the last passing centered and
  deterministic random crops were 40% for LQ and 20% for MQ.

These measurements are corpus-specific and are not guarantees for other images.
See `docs/RESULTS.md` for the complete context and methodology notes.
