
## Build 10 regression gates

- [x] Restore fractional pure-resize recovery before speculative rotation/lattice paths.
- [x] Add deterministic Format-v3 encoder fingerprints.
- [x] Make huge-PNG dimension probing independent of ImageMagick resource limits.
# PixSeal TODO

This is the live engineering roadmap for PixSeal. Items describe intended work,
not guaranteed future features. Completed milestones belong in `HISTORY.md` and
release-facing changes belong in `CHANGELOG.md`.

## v0.2.0 release-candidate checklist

- [x] Freeze Format v3 and deterministic encoder fingerprints.
- [x] Add a release-baseline gate separate from research-suite results.
- [x] Audit README, ALGORITHM, RESULTS, HISTORY, TODO and CHANGELOG for RC scope.
- [ ] Run `make release-check` on PJ's Linux corpus.
- [ ] Run the complete `make all-test ALL_TEST_REPORT=report_v0.2.0-rc1.txt` and review all research-limit changes.
- [ ] Remove the `-rc1` suffix only after final validation.


## Post-v0.2.0 research (planned v0.3.0 line)

### Geometry and synchronization

- [x] Introduce the first bounded arbitrary-rotation + anisotropic-scale composition
      without evaluating the full angle x affine Cartesian product (build 6: robust,
      110%x90% -> rotation baseline).
- [x] Replace the build-6 rotation-first composition heuristic with direct repeated
      DCT-lattice scoring after real-corpus timeout/failure results (build 7).
- [x] Use the two private failing photographs as local regression cases without
      including either source image in distributed archives.
- [x] Generalize the direct composed search from the build-7 110%x90% case to a
      symmetric build-8 lattice-basis bank covering both 110%x90% and 90%x110%
      before rotation, while preserving a fixed search budget.
- [ ] Move from the build-9 discrete basis bank to continuous or locally refined
      inference of the transformed horizontal and vertical lattice basis vectors
      (`u`, `v`) instead of growing an edit-specific table indefinitely. A first
      build-9 prototype recovered positives but was rejected because negative-case
      runtime was too high.
- [x] Extend the direct lattice bank to 105%x95% and 95%x105% before rotation
      using real-corpus evidence (build 9).
- [ ] Extend composed recovery to balanced/capacity and the reverse transformation
      order only when real-corpus evidence supports each promotion.
- [ ] Compose arbitrary rotation with X/Y shear using the same bounded-candidate
      principle.
- [ ] Add regression cases for rotation + affine combinations on all adaptive
      profiles.
- [ ] Preserve a predictable upper bound for every new geometric search stage.
- [ ] Keep wrong-key and unmarked-image failure times as explicit regression
      metrics.
- [ ] Investigate the 50% resize + arbitrary-rotation failure envelope and
      determine whether strength, interpolation strategy or synchronization can
      improve it without compromising visual quality.

### Perspective / projective recovery

- [ ] Design a bounded projective synchronization model without brute-forcing
      homography parameters.
- [ ] Estimate perspective from the repeated PixSeal DCT lattice itself, without
      visible corner markers.
- [ ] Rectify only a small ranked set of projective candidates before HMAC
      validation.
- [x] Add the first deterministic synthetic perspective test (build 11: bounded 4% vertical keystone bank).
- [ ] Generalize projective recovery beyond the build-11 two-hypothesis vertical-keystone baseline before claiming perspective support.

### Print-camera experiment

- [ ] Define a reproducible print-camera benchmark and record printer, paper,
      phone, lighting, distance and viewing angle.
- [ ] Establish progressive milestones: frontal capture, moderate rotation,
      perspective, partial crop, uneven illumination, then casual handheld
      capture.
- [ ] Evaluate robustness to printer halftoning/dithering, RGB-CMYK conversion,
      lens blur/distortion, sensor noise, sharpening/HDR and phone JPEG output.
- [ ] Attempt authenticated recovery from a printed PixSeal carrier photographed
      with a smartphone.

### Adaptive format and signal quality

- [ ] Continue measuring `robust`, `balanced` and `capacity` separately instead
      of assuming that nominal redundancy predicts real-world robustness.
- [ ] Revisit profile thresholds only if corpus measurements show a better
      capacity/robustness trade-off.
- [ ] Evaluate whether some robust-profile redundancy should eventually be
      dedicated to geometric synchronization rather than payload repetition.
- [ ] Keep format v3 unchanged unless measured evidence justifies a format-level
      change.

### Analyze / diagnostics

- [ ] Extend `analyze` with geometry-oriented diagnostics when the underlying
      metrics are reliable enough to be useful.
- [ ] Consider reporting experimental digital-geometry and print-camera
      suitability separately from deterministic capacity information.
- [ ] Never present heuristic or corpus-derived estimates as recovery guarantees.

### Performance

- [ ] Continue profiling negative extraction: unmarked images, wrong keys and
      transformed wrong-key carriers.
- [ ] Prefer sparse/key-independent geometry ranking followed by a very small
      number of authenticated probes.
- [ ] Avoid any unbounded or combinatorial search over angle, scale, shear or
      perspective parameters.
- [ ] Keep large-image pixel/memory safety limits explicit and documented.

### Testing and release hardening

- [x] Add `make all-test` with sequential execution, strict experimental-suite
      semantics, comparative timing/status summary and optional full report capture.
- [ ] Run `go test ./...` uncached on PJ's Linux system after every substantial
      geometry change.
- [ ] Run `make test`, `make deep-test`, `make extreme-test`,
      `STRICT=1 make geometry-test`, `STRICT=1 make affine-test`,
      `STRICT=1 make composition-test` and `STRICT=1 make lattice-test` on
      `original pics/` before release candidates; `make all-test` is the preferred
      one-command comparative wrapper for the complete sequence.
- [ ] Record relevant corpus results in `docs/RESULTS.md` without implying that
      observed thresholds are universal guarantees.
- [ ] Audit README, CLI help, algorithm specification, HISTORY, TODO and
      CHANGELOG before the final v0.2.0 release.

## GUI and application frontends

- [ ] Keep the steganography/geometry core independent from CLI parsing, stdout/stderr
      and desktop-specific APIs so it can be embedded in other frontends.
- [ ] Preserve compileability of the reusable core for Android/arm64 while the CLI
      remains the reference implementation.
- [ ] After the CLI/decoder is experimentally stable, design a GUI for Windows and
      Linux around the public core API rather than shelling out to the executable.
- [ ] Select a GUI stack only after comparing Android support, binary size, camera/file
      integration, maintenance burden and the amount of platform-specific code required.
- [ ] Treat Android as a first-class GUI target, including image selection/capture and
      eventual on-device extraction from camera photographs.
- [ ] Keep iOS compatibility feasible where practical, without letting it delay the
      Windows/Linux/Android path.

## Longer-term ideas

- [ ] Provide a first-class arbitrary-byte CLI input path in addition to current
      UTF-8 `-message` handling.
- [ ] Evaluate convenient pre-encryption workflows while continuing to state
      clearly that PixSeal whitening is **not encryption**.
- [ ] Explore mobile capture/extraction only after the desktop geometric decoder
      is experimentally stable.
