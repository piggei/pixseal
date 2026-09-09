# PixSeal TODO

This file contains open work only. Completed milestones belong in `HISTORY.md`
and `CHANGELOG.md`.

## v0.2.0 release closeout

- [ ] Run `make release-check` on PJ's final private corpus with v0.2.0-rc3.
- [ ] Run `make all-test ALL_TEST_REPORT=report_v0.2.0-rc3.txt` and compare with
      the RC1 qualification reference.
- [ ] Confirm encoder golden fingerprints are unchanged.
- [ ] Review the RC3 source archive for binaries, private images,
      `Zone.Identifier` and temporary/debug artifacts.
- [ ] If no release-gate regression appears, change only version/release metadata
      from `v0.2.0-rc3` to `v0.2.0` and create the final archive/tag.

## v0.3.0 — geometry research

- [ ] Build a diagnostic direct/local lattice estimator separated from
      `ExtractWithInfo` until it demonstrates better discrimination on real
      carriers.
- [ ] Estimate transformed local lattice vectors `u` and `v` rather than adding
      open-ended angle/scale/shear Cartesian searches.
- [ ] Fit affine/projective geometry from multiple local lattice measurements.
- [ ] Reduce the negative-case cost of arbitrary geometry; `geometry-test`
      currently dominates `all-test` runtime.
- [ ] Generalize projective recovery beyond the two fixed vertical-keystone
      hypotheses.
- [ ] Preserve explicit maximum probe/candidate/full-decode budgets for every
      new search stage.

## v0.3.0 — print-camera research

- [ ] Use the original full-resolution private smartphone photographs as a
      regression corpus; never include them in source archives.
- [ ] Determine whether the v3 DCT signal remains statistically measurable after
      print -> paper -> smartphone capture before changing the encoder.
- [ ] Attempt general homography recovery through virtual sampling.
- [ ] Add controlled synthetic camera-channel stages: perspective, resize, blur,
      gamma/illumination variation, JPEG and crop.
- [ ] Add an optional private `print-camera-test` target that SKIPs cleanly when
      the private corpus is absent.
- [ ] Keep the hidden print-camera message unknown to the decoder-development
      workflow; only HMAC-authenticated recovery counts as success.

## Image pipeline / metadata

- [ ] Evaluate explicit EXIF Orientation normalization in v0.3.
- [ ] Decide whether/how to preserve ICC/color-management information across
      embedding.
- [ ] Investigate a lazy/tiled/sparse extraction plane for very large images to
      reduce peak memory below the v0.2 full-plane design.

## Frontends

- [ ] Keep `watermark` independent from CLI/UI dependencies.
- [ ] After CLI/core stabilization, evaluate a GUI architecture for Windows and
      Linux with Android as a first-class target.
- [ ] Preserve core compile checks for Linux/amd64, Windows/amd64, Android/arm64
      and iOS/arm64.
