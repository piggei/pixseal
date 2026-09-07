# PixSeal TODO

This is the live engineering roadmap for PixSeal. Items describe intended work,
not guaranteed future features. Completed milestones belong in `HISTORY.md` and
release-facing changes belong in `CHANGELOG.md`.

## v0.2.0 development

### Geometry and synchronization

- [x] Introduce the first bounded arbitrary-rotation + anisotropic-scale composition
      without evaluating the full angle x affine Cartesian product (build 6: robust,
      110%x90% -> rotation baseline).
- [x] Replace the build-6 rotation-first composition heuristic with direct repeated
      DCT-lattice scoring after real-corpus timeout/failure results (build 7).
- [x] Use the two private failing photographs as local regression cases without
      including either source image in distributed archives.
- [ ] Generalize direct lattice composition beyond the current 110%x90% / robust
      baseline before claiming broader affine invariance.
- [ ] Estimate the transformed horizontal and vertical lattice basis vectors
      directly instead of adding separate angle/scale/shear searches indefinitely.
- [ ] Generalize rotation + anisotropic scale to 90%x110%, 105%x95%, 95%x105%,
      broader angles, balanced/capacity and the reverse transformation order.
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
- [ ] Add deterministic synthetic perspective tests before any physical-camera
      experiments.

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

- [ ] Run `go test ./...` uncached on PJ's Linux system after every substantial
      geometry change.
- [ ] Run `make test`, `make deep-test`, `make extreme-test`,
      `STRICT=1 make geometry-test`, `STRICT=1 make affine-test` and
      `STRICT=1 make composition-test` on
      `original pics/` before release candidates.
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
