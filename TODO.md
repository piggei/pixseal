# PixSeal TODO

This file contains open work only. Completed milestones belong in `HISTORY.md`
and `CHANGELOG.md`.

## v0.3.0 — geometry research

- [ ] Build a diagnostic direct/local lattice estimator separated from
      `ExtractWithInfo` until it demonstrates better discrimination on real carriers.
- [ ] Estimate transformed local lattice vectors `u` and `v` rather than adding
      open-ended angle/scale/shear Cartesian searches.
- [ ] Fit affine/projective geometry from multiple local lattice measurements.
- [ ] Reduce the negative-case cost of arbitrary geometry; `geometry-test`
      currently dominates `all-test` runtime.
- [ ] Generalize projective recovery beyond the two fixed vertical-keystone
      hypotheses.
- [ ] Preserve explicit maximum probe/candidate/full-decode budgets for every new
      search stage.

## v0.3.0 — print-camera research

- [ ] Use the original full-resolution private smartphone photographs as a
      regression corpus; never include them in source archives.
- [ ] Determine whether the v3 DCT signal remains statistically measurable after
      print -> paper -> smartphone capture before changing the encoder.
- [ ] Attempt general homography recovery through virtual sampling.
- [ ] Add controlled synthetic camera-channel stages: perspective, resize, blur,
      gamma/illumination variation, JPEG recompression and sensor noise.

## Future platform work

- [ ] Run the no-clobber CLI regressions on a real Windows host when convenient;
      v0.2.0 qualification already includes Windows/amd64 cross-compilation.
- [ ] Decide explicit EXIF Orientation normalization policy.
- [ ] Decide ICC/color-management preservation policy.
- [ ] Consider tiled/lazy pixel access to reduce peak memory on very large images.
