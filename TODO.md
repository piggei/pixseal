# PixSeal TODO

This file contains **open work only**. Completed milestones belong in
`HISTORY.md` and release-facing changes belong in `CHANGELOG.md`.

## v0.2.0 release qualification

- [ ] Run `make release-check` on the private qualification corpus for RC2.
- [ ] Run `make all-test ALL_TEST_REPORT=report_v0.2.0-rc2.txt` and compare the
      research results with RC1.
- [ ] Re-run the independent audit checks against the RC2 archive.
- [ ] If the release baseline remains green and no new blocker appears, remove
      the `-rc2` suffix and publish v0.2.0 final.

## v0.3.0 research line

### Geometry and synchronization

- [ ] Develop a diagnostic lattice estimator separately from `Extract()` before
      integrating another recovery heuristic.
- [ ] Infer local/continuous horizontal and vertical lattice basis vectors
      (`u`, `v`) instead of extending the discrete basis bank indefinitely.
- [ ] Require bounded candidate counts and explicit negative-case runtime for
      every new geometry stage.
- [ ] Improve texture-independence of arbitrary rotation without regressing the
      v0.2 resize/crop baseline.
- [ ] Investigate reverse-order resize→rotation and the 50%+rotation failure
      envelope.
- [ ] Study rotation + shear composition only after the estimator has a reliable
      geometry signal.

### Perspective and print-camera

- [ ] Fit a bounded general homography from local lattice measurements rather
      than enumerating edit-specific perspective shapes.
- [ ] Build a reproducible synthetic camera channel: perspective, scale, crop,
      blur, gamma/illumination and JPEG.
- [ ] Define a private `print-camera-test` convention that SKIPs cleanly when the
      private corpus is absent.
- [ ] Use the two original ~200 MP smartphone photographs as regression cases,
      never as distributed fixtures.
- [ ] Determine whether Format v3 signal remains measurably correlated after
      print → paper → camera before proposing any encoder change.
- [ ] Attempt authenticated recovery from the real print-camera captures without
      revealing the expected hidden message in advance.

### Large-image architecture

- [ ] Replace the full RGB `pixelPlane` with tiled/lazy/sparse processing where
      appropriate.
- [ ] Keep DecodeConfig/source-size policy configurable or formally specified.
- [ ] Reduce geometry runtime on large negative/wrong-key inputs.

### Metadata and image fidelity

- [ ] Decide how a future API should handle EXIF Orientation explicitly.
- [ ] Evaluate ICC/color-profile preservation or normalization.
- [ ] Define metadata-copy policy for future GUI/frontends.

### Frontends

- [ ] Keep the Go core independent of terminal I/O.
- [ ] Evaluate a GUI architecture for Windows/Linux with Android as a first-class
      target.
- [ ] Avoid a GUI design that shells out to the CLI when the Go API can be called
      directly.

## Security/research

- [ ] Consider independent cryptographic review before making stronger security
      claims.
- [ ] Perform steganalysis/detectability experiments; PixSeal currently makes no
      indistinguishability claim.
- [ ] Revisit key policy and 64-bit authentication-tag trade-offs only through an
      explicit format/security decision, not incidental refactoring.
