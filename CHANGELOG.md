# Changelog

## v0.3.0-build16 — 2026-09-11

Sixteenth print-acquisition research checkpoint. Format v3, deterministic encoding,
production extraction and the existing HMAC/smooth-resample ceilings remain frozen.

### Fixed

- Replaced build15's width-64 beam search with exact bounded per-axis enumeration,
  yielding a true top-1/top-2 over at most `3^9 = 19683` assignments per axis.
- Added a deterministic regression for a build15 beam false accept: the beam missed a
  closer runner-up and overstated uniqueness; build16 correctly reports ambiguity.
- Removed non-finite `+Inf` second-objective reporting from no-eligible/degenerate
  unwrap paths and added a JSON-serialization regression.
- Made mixed-axis ambiguity atomic: if either X or Y is ambiguous, no otherwise
  acceptable axis is partially committed and the global status remains `ambiguous`.
- Removed the private physical-corpus key from Makefile/script/documentation defaults.
- Added `/print-scan private/` and local private-manifest/config artifacts to `.gitignore`.
- Updated the two historical smartphone filenames to canonical `.jpg` names matching
  their actual JPEG content.

### Added

- Explicit X/Y unwrap statuses and eligibility counts plus second-solution availability.
- Deterministic `split-repetition-top2` diagnostics comparing exact top-1/top-2 cycle
  assignments on two disjoint key-independent repetition-pair folds. This signal is
  diagnostic-only and cannot override exact ambiguity in build16.
- `docs/PRIVATE_CORPUS.md` and `make private-corpus-manifest` for local corpus integrity.

### Research result

- `foto stampa storta.jpg` remains conservatively ambiguous with exact proposed/second
  objectives about 0.80446 / 0.82560; no cycle is committed.
- Scanner `0270_002.jpg` tightens from the build15 beam margin ~0.02634 to an exact
  margin ~0.01780 while remaining ambiguous. This directly validates the audit finding
  without changing the safe decision.
- No real physical acquisition authenticated in the four-case corpus available during
  the build16 session. The two bicycle cases were unavailable and remain required for
  full 6/6 validation.


## v0.3.0-build15 — 2026-09-11

Fifteenth print-acquisition research checkpoint. Format v3, deterministic encoding,
production extraction and the maximum four complete/HMAC decode slots remain frozen.

### Added

- Key-independent global discrete integer phase-unwrapping solver for the 3x3 blind
  control field. X/Y are solved independently with a deterministic bounded beam search.
- Candidate cycle shifts are limited to `-1, 0, +1` block. The objective combines
  robust affine residual, confidence-weighted cycle-change cost and second-difference
  curvature; no key, known header bit, CRC or HMAC result participates.
- Explicit ambiguity reporting: baseline/proposed/applied/second-best objectives,
  absolute/relative improvement, margin, evaluated states, eligible/changed cells and
  accepted axes.
- Transactional lattice fusion: fractional lattice phase and integer unwrap are applied
  together only when the global unwrap is accepted. Ambiguous/rejected proposals roll
  back completely to the pre-lattice blind controls.
- `make global-unwrap-test`, including known-slip recovery, high-confidence locking,
  ambiguous-layout rejection, rollback, budget checks and end-to-end synthetic HMAC.

### Changed

- `make all-test` now contains 24 targets.
- `make test-list` documents the new global unwrap target separately from fractional
  lattice-phase measurement.

### Research result

- The build14 per-cell unwrap can yield large same-candidate gains, but build15 shows
  that those cycle choices are often not uniquely identifiable by a key-independent
  global objective.
- On `foto stampa storta.jpg`, a representative final global proposal changes all nine
  cells and greatly lowers the geometric objective, but the first/second solution margin
  is only about 0.021; the proposal is therefore marked ambiguous and rolled back. The
  safe blind path returns to the build12/13-like 5 -> 4 diagnostic result, with no HMAC.
- Representative `foto bici dritta.jpg` and scanner `0270_002.jpg` proposals are also
  rejected as ambiguous. This is evidence against promoting build14's local unwrap
  merely because it improves the known prefix.
- No production decoder path or HMAC budget is changed.

## v0.3.0-build14 — 2026-09-11

Fourteenth print-acquisition research checkpoint. Format v3, deterministic encoding,
production extraction and the maximum four complete/HMAC decode slots remain frozen.

### Added

- Key-independent local fractional lattice-phase observer derived from the already
  selected 3x3 `LocalLatticeEstimate` records and current candidate geometry.
- Approximate inverse mapping through projective/canonical-offset/residual-warp state,
  followed by modulo-8 canonical phase measurement.
- Confidence-gated fractional fusion with the repetition observer.
- Bounded smooth integer unwrap: at most +/-1 block per axis, two passes, only for
  low-confidence primary controls and only when an affine robust prediction provides
  a large residual improvement.
- JSON evidence for lattice-phase availability, confidence, modulo-phase distance,
  consensus cells and cycle-slip corrections.
- `make lattice-phase-test`.
- `docs/RESEARCH_LOG.md`, an append-only record of hypotheses, rejected variants and
  negative results.

### Changed

- `make test-list` now reports each target's input dependency (`source`, `pics`,
  `private`, or aggregate) and warns that corpus-sensitive research/qualification
  failures are not automatically production regressions.
- `make all-test` now contains 23 targets.

### Research result

- The build13 guided cross-cell observer remains diagnostic; its weak real score is
  not used to drive the lattice-phase fusion.
- The local lattice observer is materially stronger on the current real corpus, with
  mean lattice confidence about 0.38--0.52 on the reported best-hard spatial cases.
- An initial inverse-correction sign interpretation and a fractional-only/no-unwrap
  variant were explicitly tested and rejected. The final convention reports observed
  lattice drift; the smooth fitter applies the inverse correction.
- With bounded unwrap, same-candidate diagnostics include 5 -> 2 on
  `foto stampa storta.jpg`, 7 -> 3 on `foto bici dritta.jpg`, and 9 -> 4 on scanner
  `0270_001.jpg`. Other candidates worsen, including scanner `0270_002` 7 -> 10.
- No real field passes all decode-promotion gates; smooth HMAC attempts remain zero and
  no physical acquisition authenticates.

## v0.3.0-build13 — 2026-09-11

Thirteenth print-acquisition research checkpoint. Build13 tests whether an independent
guided cross-cell image-domain observation can disambiguate integer cycle slips in the
build12 repetition-coherence controls without using key/header evidence or increasing
HMAC slots.

### Added

- Guided cross-cell correlation around the repetition-predicted relative shift: +/-1
  block, at most 324 pair probes for nine cells.
- Explicit secondary-observer, observer-distance, consensus-cell and cycle-slip
  reporting.
- Independent secondary-strength gate before any fusion or +/-1 cycle-slip correction.
- Synthetic regressions proving that strong guided evidence participates in consensus
  and that cycle-slip correction remains bounded and cannot override a high-confidence
  primary control.
- `make test-list`, with categorized target descriptions and RELEASE / QUALIFICATION /
  RESEARCH / PRIVATE / PORTABILITY / AGGREGATE labels.

### Changed

- Full +/-4 cross-cell correlation is now fallback-only when repetition evidence is
  unavailable; normal dual-observer cost is capped at 324 pair probes.
- Weak secondary evidence is diagnostic-only and cannot veto or modify the primary
  repetition controls.
- The complete `all-test` qualification matrix remains 22 targets; `test-list` is
  informational.

### Real-corpus result

- No smartphone or scanner acquisition authenticates.
- Guided cross-cell mean pair peaks are only about 0.053--0.065 on the six reported
  best-hard cases, versus about 0.61 on the controlled synthetic consensus regression.
- No real case clears the 0.10 consensus-strength gate; zero cycle-slip fixes are
  applied and no secondary observation changes an HMAC candidate.
- `foto stampa storta.jpg` therefore preserves the build12 blind 5 -> 4 measured
  same-candidate improvement and still fails the strict LOO decode gate.
- `foto stampa.jpg` remains the best hard real case at 2/24.

## v0.3.0-build12 — 2026-09-11

Twelfth print-acquisition research checkpoint. Build12 replaces key-assisted local
phase controls in the smooth-field path with a key-independent observer derived from
Format-v3 structural repetition. Format v3, the encoder, production extraction and
the four complete/HMAC decode slots remain frozen.

### Added

- Key-independent intra-tile repetition-coherence phase scoring for robust and
  balanced profiles; capacity intentionally supplies no repetition evidence.
- Blind aggregate phase search followed by bounded +/-2-block per-cell refinement
  with fractional peaks and confidence inherited from the build11 surface logic.
- Pairwise cross-cell DCT-pattern correlation as an explicitly bounded fallback.
- Ex-post blind-vs-key-assisted-oracle distance reporting; the oracle never feeds the
  blind fit.
- Explicit blind search budgets: 2240 global structural probes, 225 local probes and
  2916 pairwise-fallback probes, all on already-collected grids.
- `make blind-phase-test`.

### Changed

- The real smooth-field path now fits only `blind-self-registration` controls.
- The build11 key-assisted local phase remains diagnostic/oracle evidence only.
- `make all-test` now contains 22 targets.

### Real-corpus result

- No current smartphone or scanner acquisition authenticates.
- Direct cross-cell blind correlation is weak on the real corpus; intra-tile
  structural repetition is materially stronger and is used as the primary observer.
- Only `foto stampa storta.jpg` produces blind fields that pass the diagnostic
  resample gate. The best measured same-candidate correction changes 5 -> 4 known
  post-ECC header errors (fit RMS about 1.27 blocks, LOO about 2.04 blocks).
- The strict decode gate rejects that field because LOO remains above 2.0 blocks, so
  no blind/smooth field consumes an HMAC slot.
- `foto stampa.jpg` remains the best hard real case at 2/24 and is left untouched.
- Wrong-key real control remains unauthenticated; blind structure scores are not
  watermark detection.

## v0.3.0-build11 — 2026-09-10

Eleventh print-acquisition research checkpoint. Build11 improves spatial phase-control
quality before increasing model freedom. Format v3, the encoder, production extraction
and the four complete/HMAC decode slots remain frozen.

### Added

- Fractional-block local phase estimates from a bounded clipped correlation surface
  with parabolic peak interpolation.
- Per-cell phase confidence from correlation strength, peak prominence and curvature.
- Explicit detection and confidence penalty for local phase maxima at the +/-2-block
  search boundary.
- Confidence-weighted Huber-robust smooth-field fitting.
- Ridge-regularized quadratic comparison (six basis terms per axis) with strict
  leave-one-out model selection; affine remains the default.
- JSON reporting for phase confidence, robust outliers, affine/quadratic fit metrics
  and the selected smooth model.
- `make phase-surface-test`.

### Changed

- The build10 smooth decode gate keeps its RMS/LOO limits but now also requires a
  minimum mean control confidence.
- Boundary-truncated controls are down-weighted instead of being treated as precise
  integer observations.
- Quadratic fields are research-only unless cross-validation improves by both an
  absolute and relative margin and all existing correction/budget gates pass.

### Real-corpus result

- No current smartphone or scanner acquisition authenticates.
- No real case consumes a smooth-field HMAC slot in the final build11 code.
- The build10 `foto bici dritta` promoted field becomes 6->8 after confidence-aware
  control estimation and is therefore rejected, indicating that its earlier 6->5
  improvement was not robust to better control modelling.
- `foto bici storta` retains the strongest same-candidate diagnostic improvement,
  4->3, but leave-one-out RMS is about 2.18 blocks and fails the decode gate.
- One `0270_002` candidate selects the regularized quadratic model by cross-validation,
  but its maximum predicted correction exceeds the bounded cap, so it is rejected.

## v0.3.0-build10 — 2026-09-10

Tenth print-acquisition research checkpoint. Build10 tests whether the spatial
phase drift measured in build9 can be represented by one bounded, smooth affine
field without changing Format v3 or increasing the four HMAC-decode slots.

### Added

- Six-parameter affine phase-field fit over the existing 3x3 spatial cells.
- Separate diagnostic and strict decode gates using fit RMS and leave-one-out RMS.
- At most two bounded smooth-field resamples on already-ranked decode candidates.
- Same-candidate before/after known-prefix metrics and explicit smooth HMAC-slot count.
- `make smooth-phase-test` with exact affine recovery, HMAC authentication,
  non-smooth rejection and budget checks.

### Changed

- Spatial diagnostic cells now ignore incomplete edge tiles while the ordinary
  aggregate decode grid still uses those edge blocks.
- A smooth corrected grid can replace an existing full-decode grid only when the
  strict fit gate passes and known-prefix hard-ECC errors improve without adding
  known coded-bit errors. No new HMAC slot is created.

### Real-corpus result

- Same-candidate smooth-field measurements improve four of six current private
  acquisitions and worsen two; no acquisition authenticates.
- The strongest diagnostic change is `foto bici dritta` 7->3 known header errors
  after ECC, but that field does not pass the strict leave-one-out gate.
- One different `foto bici dritta` candidate passes the strict gate, changes 6->5
  and consumes one existing HMAC slot; HMAC still fails.
- The result supports a spatially varying residual component but rejects a single
  global affine field as a general solution.

## v0.3.0-build9 — 2026-09-10

Ninth print-acquisition research checkpoint. Build9 keeps Format v3, the encoder,
production extraction, geometry budgets and the four complete HMAC-decode slots
unchanged. It measures whether protected carrier errors are spatially local or
systematic across repeated tiles.

### Added

- Zero-extra-read 3x3 spatial buckets collected during the existing full-grid
  sampling pass.
- Key-independent sign-agreement metrics over all 1120 v3 tile positions.
- Known-prefix classification of the 42 protected header bits as stable-correct,
  stable-wrong or mixed across spatial cells, plus diagnostic majority results.
- A bounded +/-2-block local phase oracle (max 25 phase positions per cell, max
  225 across 9 cells) that never changes the decode grid or consumes HMAC slots.
- Candidate-paired hard/soft reporting so hard-to-soft changes are never compared
  across different geometry/photometric candidates.
- `make spatial-channel-test`.

### Six-acquisition findings

- Mean key-independent tile-position sign agreement is only about 0.635--0.683
  across the four smartphone photographs and two 600-dpi scanner JPEGs.
- For the best hard bit-channel candidate, 38--42 of the 42 known protected header
  bits are mixed across spatial cells in every case. Stable-wrong bits are zero in
  five cases and one in the remaining case.
- Scanner lattice consistency remains about 0.87, yet spatial sign agreement is
  only about 0.655--0.659, proving that a strong macro-lattice estimate does not
  imply spatially stable protected-bit observations.
- The bounded local phase check usually prefers offsets of roughly 1--1.8 blocks,
  but its known-header majority improves some cases and degrades others. It is
  retained only as evidence for a future smooth low-DOF phase/warp model.
- No real acquisition authenticates; HMAC remains the sole success criterion.

### Preserved

- Maximum four complete diagnostic Format-v3/HMAC decode slots.
- Build8 scanner/full-grid/reliability budgets.
- Frozen Format v3, deterministic encoder fingerprints and production extractor.

## v0.3.0-build8 — 2026-09-10

Eighth print-acquisition research checkpoint. Build8 keeps Format v3, the encoder
and production extraction unchanged, makes diagnostic authentication truly bounded
on scanner-sized inputs, and evaluates a deterministic reliability-aware Hamming
decoder without increasing the four full-decode/HMAC-slot ceiling.

### Added

- A 16 Mi-pixel diagnostic-only budget for the mature baseline extractor. Larger
  scanner/camera inputs skip that pre-pass and enter the bounded projective path.
- Full-grid work bounding: candidates above 50,000 canonical blocks use at most 25
  spatially distributed complete v3 tiles.
- Authentication sub-stage timings and explicit JSON counters for baseline skips,
  bounded grid use, sampled blocks/tiles and reliability-decode attempts.
- Deterministic soft Hamming(7,4) maximum-likelihood decoding from signed DCT
  margins, gated only by the fixed known v3 prefix and consuming an existing decode
  slot rather than adding a candidate.
- `make reliability-test` and optional `make print-scan-test`.

### Scanner/real-corpus findings

- The two 600-dpi 4960x7015 scanner JPEGs complete full diagnostic authentication
  in about 10--11 s after previously exceeding 120 s. Their lattice consistency is
  about 0.869/0.875, but their best known protected headers still contain roughly
  8--9 coded errors and 2--3 multi-error Hamming words.
- The original frontal smartphone capture gets one reliability decode, but soft
  Hamming does not improve its two remaining known-header errors and does not pass
  HMAC. Other real captures do not satisfy the conservative reliability gate.
- No smartphone or scanner acquisition authenticates. High lattice consistency and
  header z-score remain research evidence only.

### Preserved

- Format v3 and deterministic encoder fingerprints.
- Stable `ExtractWithInfo` implementation/search behavior.
- Maximum four complete diagnostic decode/HMAC slots and HMAC-only success.

## v0.3.0-build7 — 2026-09-10

Seventh print-camera research checkpoint. Build6 geometry/fallback behavior,
Build5 photometric modes and all Format-v3/production extraction rules remain
unchanged. Build7 adds bounded protected-bit/ECC diagnostics and makes the
private print-camera harness corpus-driven.

### Added

- `DiagnosticBitChannelEvidence` on the same candidates already selected for at
  most four complete virtual v3 decodes. It reports the strongest profile/phase,
  protected coded-bit margins, 42 known coded bits derived from the fixed first
  three v3 header bytes, the six exactly-known Hamming(7,4) words, post-ECC
  known-header bit errors, Hamming syndrome occupancy and correct-vs-wrong known
  bit margin ratios.
- `make bit-channel-test` with exact synthetic and deliberate two-errors-in-one-
  codeword regressions.
- Corpus-driven `make print-camera-test`: every `.png`, `.jpg` or `.jpeg` file
  in the private directory is tested in deterministic order and summarized.

### Real-corpus research status

- Original frontal capture: best known-header channel has 7/42 protected-bit
  errors, 1/6 known Hamming words beyond the one-bit correction radius and 2/24
  known-header errors after ECC; wrong-bit margin ratio ≈0.67.
- Original inclined capture: 11/42, 3/6 and 6/24 respectively; ratio ≈1.09.
- Held-out frontal bicycle capture: 12/42, 3/6 and 4/24; ratio ≈0.88.
- Held-out inclined bicycle capture: 8/42, 2/6 and 4/24; ratio ≈1.57.
- No real capture produces a valid Format-v3 HMAC. These measurements do not
  authenticate or reveal the hidden payload.

### Preserved

- Four-full-decode ceiling; bit diagnostics reuse the already materialized grids
  and do not multiply geometry/photometric search.
- Format v3, deterministic encoder fingerprints, stable production extractor and
  HMAC-only success semantics.

## v0.3.0-build6 — 2026-09-10

Sixth print-camera research checkpoint. Build5 photometric views and all frozen
Format-v3/production-decoder behavior are preserved. The diagnostic geometry no
longer requires a strong detected print boundary before a lattice-supported
projective attempt can be made.

### Added

- One bounded adaptive pyramid escalation for large, no-boundary, borderline
  lattice cases. The default 1/8 + 1/16 analysis may add only one finer level,
  normally 1/4, capped at a 4096-pixel diagnostic dimension.
- Lattice-first projective fallback: after independent lattice evidence exists,
  a sane weak print quadrilateral may seed the existing bounded projective
  estimator even when `print_boundary.detected=false`.
- Explicit `adaptive_escalated`, `adaptive_divisor`, `adaptive_reason` and
  `lattice_first_fallback_used` diagnostics.
- Regression tests proving the adaptive trigger remains bounded and that weak
  quadrilaterals require minimum geometric/confidence sanity before homography
  use.

### Held-out research status

- `foto bici dritta.jpg`: build5 stopped at consistency≈0.533 with no projective
  decode. Build6 adds 1/4, reaches consistency≈0.615, promotes lattice evidence,
  uses the weak boundary only after that evidence, and performs four projective
  decodes. Best bounded sync is balanced z≈4.454; no HMAC authenticates.
- `foto bici storta.jpg`: strong-boundary path is unchanged (consistency≈0.781,
  best photometric z≈4.079); no HMAC authenticates.
- The two original private photographs retain their build5 best results
  (frontal mild-highpass z≈4.695; inclined raw z≈3.973).
- A 200 MP unmarked qualification image can produce diagnostic lattice evidence
  after adaptive escalation, but its boundary confidence is below the weak-seed
  gate and no projective fallback is opened. This reinforces HMAC-only success
  semantics.

### Preserved

- Maximum four complete projective/HMAC decode attempts.
- Build5 three-view photometric bank and build4 geometry/refinement budgets.
- Format v3, encoder fingerprints, production `ExtractWithInfo` and HMAC-only
  authentication semantics.

## v0.3.0-build5 — 2026-09-10

Fifth print-camera research checkpoint. Build4 geometry is preserved while a
bounded photometric experiment is added after geometry selection. Format v3,
the deterministic encoder and production `ExtractWithInfo` remain unchanged.

### Added

- Fixed three-view photometric bank: `raw`, `local-normalize` and
  `mild-highpass`.
- At most three previously selected geometries are evaluated, for a maximum of
  nine photometric header probes; the complete HMAC decode budget remains four.
- Per-mode known-header fraction/z-score, mean absolute DCT margin and normalized
  signed sync margin in JSON diagnostics and `print-camera-test`.
- `make photometric-test` with pristine-carrier authentication and explicit bank
  budget regressions.
- Release/qualification/research classification in `make all-test` and uniform
  large-image SKIP behavior in bounded geometry scripts.

### Real-corpus research status

- Frontal: `mild-highpass` on the retained fundamental phase-DLT geometry raises
  the known-header score from z≈4.454 / fraction≈0.768 to z≈4.695 /
  fraction≈0.783.
- Inclined: the photometric bank does not beat the build4 raw global maximum
  z≈3.973; this negative result is retained rather than tuned away.
- Neither photograph produces a valid Format-v3 HMAC. The hidden payload remains
  unknown.

### Preserved

- Build4 geometry candidate generation/refinement and four-full-decode ceiling.
- Format v3, encoder fingerprints, production extraction order and HMAC-only
  success semantics.

## v0.3.0-build4 — 2026-09-10

Fourth print-camera research checkpoint. Format v3, the deterministic encoder and
the production `ExtractWithInfo` path remain unchanged from v0.2.0.

### Added

- Explicit fundamental-scale selection after projective scale clustering. A
  candidate must have multi-level and multi-region support; among sufficiently
  supported candidates the lowest spatial frequency is preferred, preventing a
  high-frequency phase alias from automatically becoming the carrier scale.
- A bounded key-assisted refinement around only the selected fundamental family:
  width and height are probed independently at -2%, -1%, -0.5%, 0, +0.5%, +1%
  and +2% (maximum 16 sync probes including full-resolution verification).
- Modulo-8 DCT block-origin refinement. At most three candidates search one
  periodic 8-pixel cell with coarse 2-pixel and fine 0.5-pixel offsets; sparse
  winners are rechecked with the ordinary probe and cannot silently degrade the
  starting candidate.
- One bounded residual-warp diagnostic derived from the 3x3 key-independent
  local lattice field. Control residuals are reduced modulo the 8-pixel block
  grid before fitting a smooth quadratic correction.
- Explicit fundamental/subpixel/residual counters, budgets and evidence in
  `diagnose -json` and `make print-camera-test`.
- Regression coverage for harmonic rejection, modulo-8 subpixel refinement and
  smooth residual-warp fitting.

### Real-corpus research status

- The selector independently promotes the matching low-frequency family near
  1668x1250 on `foto stampa.jpg` and 1653x1254 on `foto stampa storta.jpg`; no
  private-corpus dimensions are hard-coded.
- On the frontal photograph the fundamental branch remains near z=3.50 and does
  not beat the build3 global best z=4.454. The best subpixel result is the
  unshifted origin, so build4 does not claim a synthetic improvement there.
- On the inclined photograph the fundamental-only bounded scale refinement moves
  to about 1686x1279 and reaches z=3.732, while the global best remains the
  build3 value near z=3.973.
- The lattice-derived residual fits use nine controls (RMS about 2.28 canonical
  pixels frontal and 1.24 inclined) but do not yet produce a valid HMAC.
- Neither photograph authenticates. The real hidden payload therefore remains
  unknown and `print-camera-test` correctly remains a research failure.

### Preserved

- Format v3, encoder fingerprints and production extraction search order.
- Eight coarse projective candidates, three bounded refinement seeds and at most
  four complete virtual Format-v3/HMAC decodes.
- HMAC-only success semantics and the private-corpus exclusion policy.

## v0.3.0-build3 — 2026-09-10

Third print-camera research checkpoint. Format v3, the deterministic encoder and
the production `ExtractWithInfo` path remain unchanged from v0.2.0.

### Added

- Spatial Format-v3 phase-consensus measurement across independent complete
  tiles for every retained projective scale candidate. Aggregate sync peaks no
  longer decide geometry by themselves.
- Bounded phase-aware scale refinement: at most three seeds, each evaluated at
  the base scale plus ±1%/±2% width and height perturbations (maximum 27 scale
  evaluations total).
- Deterministic phase-correspondence DLT refinement. Four local phase
  observations can refine the coarse homography, with canonical corrections
  bounded to 64 pixels per axis.
- Phase coherence, refinement budgets, refined candidates and phase-DLT evidence
  in `diagnose -json`.
- Full virtual Format-v3 decodes remain capped at four and HMAC remains the only
  success criterion.

### Real-corpus research status

- The build2 lattice/boundary results are preserved: about 0.760 global
  consistency on `foto stampa.jpg` and 0.812 on `foto stampa storta.jpg`, both
  with lattice evidence.
- Two independent captures contain a closely matching coarse scale family near
  1668x1250 / 1653x1254. Build3 discovers and measures that family from local
  phase consistency; it is not hard-coded and is not assumed to be the true
  carrier size.
- On the frontal capture, phase-DLT refinement of that family raises its sparse
  known-header probe from z≈3.50 to z≈4.45. On the inclined capture the bounded
  phase search also improves the best sparse probe (to about z≈3.97).
- Neither photograph authenticates yet. The hidden real payload therefore
  remains unknown and `print-camera-test` correctly remains a research failure.

### Preserved

- Format v3 and deterministic encoder fingerprints.
- Production `ExtractWithInfo` behavior/search order.
- Eight coarse projective candidates and at most four complete virtual v3
  decodes.
- Private-corpus policy: real smartphone photographs are never distributed.

## v0.3.0-build2 — 2026-09-09

Second print-camera research checkpoint. Format v3, the deterministic encoder and
the production `ExtractWithInfo` search banks remain unchanged from v0.2.0.

### Added

- Pure-Go coarse print-boundary estimator with explicit confidence and corners.
  The quadrilateral is an initializer only and never watermark evidence.
- Boundary-aware normalization of local lattice bases and cross-level native-scale
  support to reduce scene-texture and single-pyramid-level harmonics.
- Bounded projective scale clustering and explicit canonical-to-photo homography.
- Virtual projective DCT sampler that avoids materializing a full rectified copy.
- Diagnostic key-known Format-v3 header probes for candidate ordering, capped at
  eight geometry probes and four complete virtual Format-v3 decodes.
- Synthetic end-to-end projective authentication regression, including wrong-key
  rejection.
- `make homography-test`; it is included in `make all-test` as a research target.
- Additional projective-fit/authentication timings and budgets in diagnose JSON.

### Real-corpus research status

- `foto stampa.jpg`: automatic print boundary, global lattice consistency about
  0.760, `lattice_evidence=true`.
- `foto stampa storta.jpg`: automatic print boundary, global lattice consistency
  about 0.812, `lattice_evidence=true`; build1 was about 0.505/false.
- Both 200 MP captures now reach the bounded virtual projective authentication
  path without a full rectified image. No Format-v3 HMAC is valid yet, therefore
  **the hidden real payload remains unrecovered and print-camera PASS is not
  claimed**.
- Key-known header scores are explicitly treated as multiple-testing-affected
  research evidence, not as proof that a watermark is present.

### Preserved

- Format v3 and deterministic encoder fingerprints.
- Production `ExtractWithInfo` behavior/search order.
- Private-corpus policy: real smartphone photographs are never distributed.

## v0.3.0-build1 — 2026-09-09

First research checkpoint toward print -> paper -> smartphone recovery. Format
v3, the deterministic encoder and the production `ExtractWithInfo` decoder are
unchanged from the qualified v0.2.0 baseline.

### Added

- Separate `watermark.DiagnoseGeometry` API and `pixseal diagnose` CLI command.
- Bounded sampled-luminance diagnostic pyramid for large inputs.
- Local `u/v` basis, phase, DCT differential margin, periodic coherence and
  35x32 v3 tile-repetition measurements.
- Per-region candidate pools with cross-region geometric consensus and explicit
  handling of 90-degree square-lattice basis equivalence.
- Explicit coarse/refined/repetition/phase/sample budgets and per-stage timings
  in JSON output.
- Optional independent baseline HMAC attempt when a key is supplied. Diagnostic
  lattice evidence cannot by itself authenticate a payload.
- `make lattice-estimator-test`.
- `make print-camera-test`, which uses the two private original smartphone
  photographs when present, SKIPs cleanly when absent, and reports PASS only for
  an authenticated Format v3 payload.

### Research status

- Canonical marked synthetic/digital carriers can produce coherent local lattice
  evidence while corresponding unmarked inputs remain negative in the build1
  regression path.
- Arbitrary-angle automatic local-basis estimation is not yet reliable enough to
  feed homography fitting; this remains the next geometry milestone.
- No claim of real print-camera payload recovery is made by build1.

## v0.2.0 — 2026-09-09

Final release of the Format v3 line, promoted from RC4 after qualification on
the private two-image corpus. No Format v3, encoder fingerprint or decoder
search-bank change was made after RC4.

### Qualification

- `release-unit`: PASS.
- `test-images`: PASS on both private qualification images and all three explicit
  profiles.
- `deep-test`: 72/72 PASS with zero failures, timeouts, skips or errors.
- `core-target-check`: PASS for linux/amd64, windows/amd64, android/arm64 and
  ios/arm64.
- Experimental reference results: affine 22/24, composition 2/2, lattice 8/8,
  perspective 4/4.
- The first RC4 all-test run produced two 60-second geometry timeouts; a targeted
  rerun with a 120-second geometry extraction timeout recovered both and restored
  the RC2 reference result of 65/120 with zero timeouts.

### Finalization

- Promoted version metadata from `v0.2.0-rc4` to `v0.2.0`.
- Recorded final qualification and retained the release/research separation.
- Set the geometry research harness default extraction timeout to 120 seconds to
  avoid misclassifying known recoverable cases as wall-clock regressions on the
  qualification host.

## v0.2.0-rc4 — 2026-09-09

Release-engineering reconciliation candidate. **Format v3, deterministic encoder
fingerprints and decoder search banks are unchanged.**

### Fixed

- Restored the RC2 `release-unit` / `research-unit` split so experimental Go
  geometry regressions cannot make the stable release baseline red.
- Made `make release-check` self-contained with target-specific `STRICT=1`.
- Made strict baseline qualification fail when every configured transformation
  is skipped and zero baseline transformations are executed.
- Made partial `ALL_TEST_TARGETS` runs report `PARTIAL` / `NOT RUN` instead of a
  misleading full qualification PASS.
- Handle subcommand `flag.ErrHelp` as successful CLI help (exit 0).
- Make `capacity` use `image.DecodeConfig` only; it no longer decodes the full
  bitmap merely to calculate geometry capacity.
- Restored `extract -raw` contract: exact payload bytes on stdout, diagnostics on
  stderr; geometric shell harnesses again capture the two streams separately.
- Restored numeric validation of `PERSPECTIVE_MAX_MPIX`.
- Restored RC2 CLI oversized-config and forced-permission regression coverage and
  added a true 64-bit pixel-count overflow regression.
- Treat post-publication temporary-file removal as best-effort cleanup rather
  than converting a successful no-clobber commit into a false write failure.
- Restore the v0.2 public/core source-size policy to 300,000,000 pixels while
  retaining RC3 overflow-safe arithmetic.
- Correct direct-lattice documentation to a 24 full-grid overall worst case
  (12 per sequential shape group), not 12 globally.
- Restore accurate RC1 → RC2 → RC3 chronology and attribute the qualification
  report to RC2.

### Preserved from RC3

- Cross-platform no-clobber publication with hard-link first and exclusive-create
  copy fallback when hard links are unavailable.
- Regular-file-only forced replacement, umask-safe new files and permission
  preservation on replacement.
- Shared alpha-on-white flattening between encoder and analyzer.
- Overflow-safe source-size checks and strengthened projective E2E regression.

## v0.2.0-rc3 — 2026-09-09

Second hardening candidate. RC3 improved cross-platform output publication,
overflow checking, alpha flattening and the synthetic perspective regression.
A subsequent audit found that assembly of RC3 had also reverted several valid
RC2 release-engineering and shell-harness changes; RC4 reconciles those changes.

## v0.2.0-rc2 — 2026-09-09

Implemented the first independent audit findings without changing Format v3:
finite strength validation, explicit CLI strength contract, dotfile handling,
`extract -raw`, large-image preflight, safer output permissions/no-clobber logic,
perspective harness fixes, an end-to-end projective regression, release/research
Go-test separation and major documentation cleanup.

PJ's RC2 qualification run reported release baseline PASS, `deep-test` 72/72,
composition 2/2, lattice 8/8 and perspective 4/4. Experimental geometry and
affine limits remained visible at 65/120 and 22/24 respectively.

## v0.2.0-rc1 — 2026-09-08

Consolidated the qualified build-11 algorithmic line and froze Format v3 encoder
fingerprints for release hardening.

## Development builds 1–12

See [`HISTORY.md`](HISTORY.md) for the complete technical progression.
