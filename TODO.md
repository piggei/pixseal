# PixSeal TODO

This file contains open work only. Completed milestones belong in `HISTORY.md`
and `CHANGELOG.md`.

## v0.2.0 release closeout

- [ ] Run `make release-check` on PJ's final private corpus with v0.2.0-rc4.
- [ ] Run `make all-test ALL_TEST_REPORT=report_v0.2.0-rc4.txt` and compare with
      the **RC2 qualification reference** in `docs/RESULTS.md`.
- [ ] Confirm `deep-test` remains 72/72 on the same two-image, three-profile
      qualification corpus.
- [ ] Confirm research measurements do not regress below RC2 reference:
      geometry >=65/120, affine >=22/24, composition 2/2, lattice 8/8,
      perspective 4/4.
- [ ] Confirm Format v3 encoder golden fingerprints are unchanged.
- [ ] Preferably execute the no-clobber CLI regressions on a real Windows host in
      addition to cross-compilation.
- [ ] Review the RC4 source archive for binaries, private images, reports,
      `Zone.Identifier` and temporary/debug artifacts.
- [ ] If the release baseline is fully green, change only version/release metadata
      from `v0.2.0-rc4` to `v0.2.0` and create the final archive/tag.

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

- [ ] Decide explicit EXIF Orientation normalization policy.
- [ ] Decide ICC/color-management preservation policy.
- [ ] Consider tiled/lazy pixel access to reduce peak memory on very large images.
