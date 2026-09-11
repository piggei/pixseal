# PixSeal v0.2.0 — qualification and measured results

This document separates the **stable release baseline** from experimental
geometry measurements. Results are corpus-specific observations, not universal
recovery guarantees.

## RC2 qualification reference — 2026-09-09

The preserved `v0.2.0-rc2` all-test report was produced on Linux/WSL2 with Go
1.25.1 and ImageMagick, using strict mode for pass/fail baseline and geometry
suites. It reported:

```text
Release baseline: PASS
Research suites: ATTENTION (2 experimental targets failed)
Overall: FAIL (2 experimental targets failed)
```

Release baseline:

```text
version-check          PASS
vet                    PASS
release-unit           PASS
test-images            PASS
deep-test              72/72 PASS
core-target-check      PASS
```

Research measurements:

```text
extreme-test           completed progressive limit mapping
research-unit          PASS
geometry-test          65/120 PASS, 55 FAIL
affine-test            22/24 PASS, 2 FAIL
composition-test       2/2 PASS
lattice-test           8/8 PASS
perspective-test       4/4 PASS
```

The geometry/affine FAIL rows are intentionally visible measurements of the
experimental search bank. They are not failures of the stable authenticated
JPEG/resize/crop release baseline.

## Baseline digital robustness

On the RC2 qualification corpus, `deep-test` recovered all 72 tested cases
across two real images and all three profiles. The suite covered JPEG quality 82,
resize 95/85/75/65/55/50%, center crop 90/75/50% and deterministic random crop
75/50%. `extreme-test` is intentionally non-strict and maps the progressive
failure envelope.

## Adaptive profiles

| Profile | Max payload | Protected bits | Tile redundancy |
|---|---:|---:|---:|
| robust | 16 B | 448 | 2.50x |
| balanced | 32 B | 672 | 1.67x |
| capacity | 64 B | 1120 | 1.00x |

## Resize recovery budget

```text
13 fixed non-direct scale hypotheses
<= 3 phases per scale
<= 6 full-grid shortlisted scales
<= 18 full-carrier virtual aggregations
```

A decisive scale may receive one physical bilinear fallback over the expected
inverse size and its eight ±1-pixel neighbours.

## Experimental lattice composition budget

The frozen bank contains 110x90, 90x110, 105x95 and 95x105 basis shapes over
-45..+45 degrees at 0.25-degree steps. The search is evaluated in two sequential
shape groups (±10% anchors, then ±5% moderates):

```text
1444 sparse probes maximum
192 stronger coherence evaluations maximum
12 full authenticated grids maximum per group
24 full authenticated grids maximum overall
```

The RC2 qualification recovered 8/8 configured robust-profile lattice cases.

## Experimental perspective

The frozen v0.2 bank contains only `top-narrow-4` and `bottom-narrow-4`, one
aligned full projective aggregation each. RC2 recovered 4/4 configured cases.
RC2 already contained an end-to-end Go recovery regression; RC3 strengthened the
synthetic warp implementation while retaining explicit correction-path assertion.
General homography inference and physical print-camera recovery are not claimed.

## Final v0.2.0 qualification — 2026-09-09

The RC4 source candidate was exercised on the same private two-image corpus used
for the RC2 reference. The complete all-test report recorded:

```text
version-check          PASS
vet                    PASS
release-unit           PASS
test-images            PASS
deep-test              72/72 PASS
extreme-test           completed progressive limit mapping
research-unit          PASS
geometry-test          63 PASS, 55 FAIL, 2 TIMEOUT (60-second run)
affine-test            22/24 PASS, 2 FAIL
composition-test       2/2 PASS
lattice-test           8/8 PASS
perspective-test       4/4 PASS
core-target-check      PASS

Release baseline: PASS
Research suites: ATTENTION
```

The two RC4 geometry timeouts were both `PJ_lingua.PNG`, profile `capacity`, at
-10 and -5 degrees. A targeted rerun with:

```text
STRICT=1 EXTRACT_TIMEOUT=120 make geometry-test
```

recovered both cases and produced:

```text
65 passed, 55 failed, 0 timeouts, 0 skipped, 0 errors, 120 total
```

This exactly reproduces the RC2 experimental geometry reference. The timeout
difference is therefore treated as a wall-clock harness sensitivity, not a
decoder capability regression. The final source keeps the same decoder search
banks and uses a 120-second default only for the geometry research harness.

Final release conclusions:

- stable release baseline fully qualified;
- Format v3 golden fingerprints unchanged;
- no encoder or decoder-search functional change after RC4;
- experimental affine/composition/lattice/perspective counts match the RC2
  reference;
- geometry capability matches the RC2 65/120 reference when given the validated
  120-second research timeout;
- reusable core portability checks pass for linux/amd64, windows/amd64,
  android/arm64 and ios/arm64.

## Large images

The v0.2 line has round-tripped a private ~200 MP carrier. v0.2.0 retains the
300,000,000-pixel CLI/core source policy from RC2 while keeping RC3's
overflow-safe arithmetic. Geometry suites independently use lower research
limits (typically 50/100 MP). The 300 MP guard is a safety policy, not a promise
that every machine has sufficient RAM for every image below it.

## Interpretation

- **Deterministic:** format sizes, profile capacities, search-space caps and
  authentication rules.
- **Experimental:** success rates on particular images/transforms.
- **Heuristic:** `analyze` detail score and recommended strength.

No experimental PASS guarantees recovery on an arbitrary future image, and no
failed experimental geometry case invalidates the authenticated digital baseline.

## v0.3.0-build1 research checkpoint — 2026-09-09

The build1 change is isolated from Format v3, embedding and `ExtractWithInfo`; the
v0.2 qualification numbers above therefore remain the stable comparison
baseline. New diagnostic tests add two explicit properties:

- a deterministic marked synthetic carrier must produce bounded local-lattice
  evidence while its corresponding unmarked source remains negative;
- optional correct-key authentication uses the existing HMAC path, while a wrong
  key cannot turn lattice evidence into an authenticated payload.

Local manual qualification on the private digital corpus is recorded only as a
research observation, not a release guarantee. With robust-profile marked copies
of the two qualification images, the 2026-09-09 build1 run measured:

| Input | Lattice evidence | Global consistency | Global `u` | Global `v` |
|---|---:|---:|---:|---:|
| marked `PJ_piccolo.png` | true | 0.939 | (7.95, -0.02) | (-0.02, 8.03) |
| unmarked `PJ_piccolo.png` | false | 0.414 | (5.50, -5.30) | (4.70, 5.23) |
| marked `PJ_lingua.PNG` | true | 0.981 | (8.07, 0.00) | (0.00, 8.00) |
| unmarked `PJ_lingua.PNG` | false | 0.442 | (6.31, -1.91) | (1.76, 6.50) |

The marked canonical carriers therefore resolve very close to the native 8-pixel
lattice while the corresponding unmarked originals remain below the evidence
gate. Exact values are content dependent and are not frozen as interoperability
requirements.

The real `foto stampa.jpg` / `foto stampa storta.jpg` corpus is intentionally not
distributed. `make print-camera-test` therefore SKIPs when that private corpus is
not installed. When it is installed, only an authenticated Format v3 HMAC can
produce PASS; useful lattice evidence without HMAC is reported as research, not
success.

Known build1 limit: automatic arbitrary-angle local-basis estimation is not yet
reliable enough to fit a general homography. Synthetic probing shows measurable
repetition evidence near the true transformed lattice, but candidate generation
and bounded refinement need further work before the homography milestone.
## v0.3.0-build2 projective research checkpoint — 2026-09-09

Build2 remains isolated from Format v3, embedding and production
`ExtractWithInfo`. Its deterministic additions are tested separately through the
research targets. The synthetic projective test verifies that the virtual
homography sampler can reach a valid existing Format-v3 HMAC with the correct
key and that a wrong key fails.

On the two undistributed original smartphone photographs, the current automatic
diagnostic measured:

| Input | Boundary confidence | Global lattice consistency | Lattice evidence | Real HMAC |
|---|---:|---:|---:|---:|
| `foto stampa.jpg` | 0.728 | 0.760 | true | not recovered |
| `foto stampa storta.jpg` | 0.652 | 0.812 | true | not recovered |

For comparison, build1 measured about 0.637/true on the frontal capture and
0.505/false on the inclined capture. Build2 therefore materially improves the
geometric initializer on the harder perspective case. These values are research
observations on two images, not frozen thresholds or interoperability promises.

The bounded projective path probes at most eight canonical-size candidates and
permits at most four full virtual v3 decodes. On the real captures it reaches
that path without materializing a full rectified image, but no candidate yet
authenticates. Key-known header fractions/z-scores are deliberately not reported
as watermark detection because the bounded candidate search still creates a
multiple-testing effect. The hidden message remains unknown.

The qualified v0.2 release baseline remains the reference: build2 changes only
research diagnostics and does not promote these hypotheses into
`ExtractWithInfo`.


## v0.3.0-build3 phase-refinement checkpoint — 2026-09-10

Build3 keeps the build2 boundary/lattice results and adds spatial phase
consensus plus bounded scale/DLT refinement. On the two private ~200 MP
smartphone captures, the automatic diagnostic still reports global lattice
consistency about 0.760 (frontal) and 0.812 (inclined), with lattice evidence on
both.

A notable cross-capture observation is a closely matching coarse scale family:
about 1668x1250 on the frontal capture and 1653x1254 on the inclined capture.
These values are produced independently by the estimator and are not coded as
target dimensions. The frontal member has phase consistency about 0.716; on the
inclined capture the corresponding family is about 0.575 and a bounded height
refinement near 1653x1241 reaches about 0.615. Other local maxima remain and are
retained rather than silently discarded.

For the frontal family, a four-point phase-DLT refinement raises its sparse
known-header probe from roughly z=3.50 to z=4.45. On the inclined capture, the
bounded refinement bank raises the best sparse probe from the build2 value near
z=3.70 to about z=3.97. These are multiple-testing-affected research scores, not
watermark detection.

Both photographs still exhaust at most four virtual full-grid attempts without
a valid Format-v3 HMAC. `print-camera-test` therefore remains a deliberate
research failure and the hidden message remains unknown.


## v0.3.0-build4 fundamental-scale checkpoint — 2026-09-10

Build4 keeps the build3 lattice evidence (global consistency about 0.760 frontal
and 0.812 inclined) and adds an explicit fundamental-scale selector before
key-assisted phase refinement. The selector independently promotes about
1668.02x1250.43 on the frontal capture and 1653.11x1253.88 on the inclined
capture. These values come from multilevel/multiregion support and are not
private-corpus constants.

On the frontal capture, bounded fundamental-scale refinement does not beat the
seed (best fundamental z≈3.50). Modulo-8 refinement also keeps an unshifted best
origin for the strongest refined branch, while the overall known-header maximum
remains balanced z≈4.454, unchanged from build3.

On the inclined capture, the fundamental branch refines to about
1686.17x1278.96 and reaches z≈3.732. The overall known-header maximum remains
balanced z≈3.973, also unchanged from build3. The single lattice-derived residual
fits use nine controls, with RMS residual about 2.28 canonical pixels frontal and
1.24 inclined; neither fit authenticates.

Both real photographs still exhaust at most four complete virtual Format-v3
decodes without a valid HMAC. Therefore `print-camera-test` remains a deliberate
research failure and the hidden message remains unknown. Header z-scores are
reported only as bounded search diagnostics and are not watermark-detection
claims.


## v0.3.0-build5 photometric checkpoint — 2026-09-10

Build5 preserves build4 geometry and adds a three-view photometric experiment.
On `foto stampa.jpg`, the retained fundamental phase-DLT geometry improves from
raw balanced z≈4.454 / fraction≈0.768 to mild-highpass balanced z≈4.695 /
fraction≈0.783. Its normalized signed sync margin also rises from about 0.621 to
0.651. No HMAC is recovered.

On `foto stampa storta.jpg`, the photometric views do not exceed the build4 raw
global best z≈3.973. This is kept as a negative result rather than triggering
additional tuning. The full-decode ceiling remains four and the hidden payload
remains unknown.

Qualification-corpus handling is reported separately from release invariants in
`all-test`, allowing different workstations to add low/medium/high-resolution
images without redefining the stable Format-v3 release gate.


## v0.3.0-build6 lattice-first held-out checkpoint — 2026-09-10

Build6 was frozen after build5 and then evaluated on the new private bicycle
print-camera pair. No thresholds or target dimensions were derived from that pair
before the test.

| Input | Strong boundary | Adaptive 1/4 | Lattice consistency | Projective source | Best photometric sync z | HMAC |
|---|---:|---:|---:|---|---:|---:|
| `foto bici dritta.jpg` | false (0.337) | yes | 0.615 | lattice+weak-boundary | 4.454 (raw) | not recovered |
| `foto bici storta.jpg` | true (0.780) | no | 0.781 | print-boundary+lattice | 4.079 (mild-highpass) | not recovered |
| `foto stampa.jpg` | true (0.728) | no | 0.760 | print-boundary+lattice | 4.695 (mild-highpass) | not recovered |
| `foto stampa storta.jpg` | true (0.652) | no | 0.812 | print-boundary+lattice | 3.973 (raw) | not recovered |

The new frontal bicycle capture is the key build6 result: build5 stopped before
projective authentication at consistency≈0.533. The adaptive finer level raises
consistency to ≈0.615 and the lattice-positive weak quadrilateral opens the same
bounded projective/HMAC path used by strong-boundary captures. Four complete
decode attempts are made, but none authenticate.

Negative controls remain essential. The unmarked 16320x12288 qualification image
can itself reach diagnostic lattice evidence after adaptive escalation
(consistency≈0.592), but its boundary confidence is only ≈0.082 and therefore it
does not open the lattice-first projective fallback. A wrong-key run on the new
frontal capture can also produce a high bounded header z-score while still
failing HMAC. These observations are why lattice, phase and sync scores remain
research evidence only and never constitute watermark detection.


## v0.3.0-build7 protected-bit channel checkpoint — 2026-09-10

Build7 keeps the build6 geometry, build5 photometric bank and four-full-decode
budget fixed, then measures the protected-bit/ECC state of those same decode
candidates. Only the fixed three-byte v3 prefix is used as known data; the hidden
payload remains unknown.

| Input | Best known coded errors (42) | Known Hamming words >1 error (6) | Known header errors after ECC (24) | Wrong/correct margin ratio | HMAC |
|---|---:|---:|---:|---:|---:|
| `foto stampa.jpg` | 7 | 1 | 2 | 0.670 | not recovered |
| `foto stampa storta.jpg` | 11 | 3 | 6 | 1.088 | not recovered |
| `foto bici dritta.jpg` | 12 | 3 | 4 | 0.876 | not recovered |
| `foto bici storta.jpg` | 8 | 2 | 4 | 1.567 | not recovered |

The original frontal capture is the most promising reliability-aware case: five
of six known header codewords are already inside the hard Hamming correction
radius and its wrong known bits have materially lower DCT margin than correct
bits. The inclined bicycle capture is different: despite only eight known coded
errors, wrong bits are stronger on average than correct known bits, suggesting
that geometry/phase or optical distortion is still a more plausible limitation
than simple low-confidence channel noise.

These measurements are deliberately not extrapolated to unknown payload bits and
are not a detection criterion. None of the four captures authenticates.

`make print-camera-test` in build7 discovers every PNG/JPEG directly inside the
private corpus directory and reports a corpus summary. The private photographs
remain excluded from source archives.


## v0.3.0-build8 scanner/reliability checkpoint — 2026-09-10

Build8 keeps the four smartphone cases and adds two 600-dpi Canon/JFIF scanner JPEGs
(4960x7015, about 34.8 MP each). The supplied scans are physically rotated 90 degrees
in their pixel data and are intentionally tested as supplied. One scan corresponds to
the bicycle print and the other to the original lake/bicycle print.

| Acquisition | Lattice consistency | Best sync z | Known coded errors /42 | Known words >1 error /6 | Hard post-ECC errors /24 | HMAC |
|---|---:|---:|---:|---:|---:|---|
| original smartphone frontal | 0.760 | 4.695 photometric | 7 | 1 | 2 | not recovered |
| original smartphone inclined | 0.812 | 3.973 | 10 | 3 | 7 | not recovered |
| bicycle smartphone frontal | 0.615 | 4.454 | 10 | 3 | 6 | not recovered |
| bicycle smartphone inclined | 0.781 | 4.079 photometric | 8 | 2 | 4 | not recovered |
| scanner bicycle print (`0270_001`) | 0.869 | 4.320 | 8 | 2 | 4 | not recovered |
| scanner original print (`0270_002`) | 0.875 | 3.732 | 9 | 3 | 5 | not recovered |

The build8 full-grid block cap changes some large-candidate bit-channel aggregates
relative to build7 while leaving the underlying geometry/probe search and Format-v3
semantics unchanged. This is expected: large full grids now use a fixed spatial sample
instead of every canonical block.

The original frontal smartphone capture is the only current case that passes the
conservative reliability-decode gate. One of its four complete decode slots therefore
uses weighted Hamming, but the best known prefix remains two bits wrong after both
hard and soft decoding and HMAC still fails. On the scanner cases, weighted Hamming
does not improve the known prefix. Strong scanner lattice consistency therefore does
not imply a clean protected-bit channel; the remaining problem cannot be attributed
only to smartphone perspective/ISP processing.

Performance is materially bounded: both scanner images previously exceeded a 120 s
full diagnostic run because build7 entered the mature baseline extractor. Build8 skips
that pre-pass above 16 Mi pixels and records full diagnostic wall times around 10--11 s
on the development environment, with projective authentication itself about 3.4--3.6
s. No private acquisition file is part of the source archive.


## v0.3.0-build9 spatial-stability checkpoint — 2026-09-10

Build9 was evaluated without changing any of the build8 geometry, photometric,
full-grid or HMAC budgets. The six private acquisitions are not distributed. The
figures below refer to the spatial evidence attached to the candidate selected by
the existing hard bit-channel ordering, not a new spatially optimized candidate.

| Acquisition | Lattice consistency | Cells | key-independent tile sign agreement | unstable tile positions | hard post-ECC known header | known stable-wrong / mixed | global-phase majority post-ECC | local-phase cells matching global | mean local phase offset (blocks) | bounded local-phase majority post-ECC | HMAC |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `foto stampa.jpg` | 0.760 | 4 | 0.683 | 0.389 | 2/24 | 0 / 38 | 5/24 | 1/4 | 1.37 | 10/24 | no |
| `foto stampa storta.jpg` | 0.812 | 6 | 0.669 | 0.743 | 7/24 | 0 / 39 | 6/24 | 1/6 | 1.82 | 5/24 | no |
| `foto bici dritta.jpg` | 0.615 | 9 | 0.635 | 0.821 | 6/24 | 0 / 42 | 10/24 | 2/9 | 1.35 | 4/24 | no |
| `foto bici storta.jpg` | 0.781 | 4 | 0.682 | 0.397 | 4/24 | 1 / 41 | 12/24 | 1/4 | 1.06 | 10/24 | no |
| scanner `0270_001.jpg` | 0.869 | 6 | 0.659 | 0.773 | 4/24 | 0 / 39 | 10/24 | 3/6 | 1.04 | 8/24 | no |
| scanner `0270_002.jpg` | 0.875 | 6 | 0.655 | 0.783 | 5/24 | 0 / 41 | 9/24 | 1/6 | 1.48 | 5/24 | no |

The key-independent result is the central build9 observation: repeated tile
positions do not preserve a common sign reliably across the print surface. Mean
agreement remains only about 0.64--0.68 even on the two scanner acquisitions whose
macro-lattice consistency is about 0.87. This rules out the simple model in which
a small fixed subset of coded bits is systematically inverted everywhere.

Known-prefix evidence points in the same direction: almost every known protected
header bit is mixed across cells, whereas stable-wrong bits are essentially absent.
A simple cell majority is not a solution and often performs worse than the existing
DCT-margin aggregation, showing that spatial cells are neither equally reliable nor
perfectly registered.

The bounded local-phase oracle finds that most cells prefer a phase within one to
two blocks of the global phase. It can reduce known-prefix majority errors in some
cases (for example `foto bici dritta.jpg` from 10 to 4 post-ECC errors and the
original inclined photograph from 6 to 5), while making other cases worse (the
original frontal photograph from 5 to 10). Because 25 key-known phase positions are
compared per cell, these improvements are not authentication evidence and are not
used for HMAC attempts. The result motivates a future smooth, low-degree-of-freedom
spatial phase/warp model rather than independent per-cell re-phasing.

Build9 also makes the build8 soft-Hamming summary candidate-paired. In particular,
`best_hard_candidate_soft_*` refers to the same candidate that produced the best
hard bit-channel result, while `best_soft_candidate_hard_*` reports the hard error
count on the candidate independently selected by the soft metric. This removes the
ambiguous cross-candidate comparison exposed by the build8 report.

No private acquisition produces a valid Format-v3 HMAC.


## v0.3.0-build10 smooth-field checkpoint — 2026-09-10

Build10 was evaluated on the same six private print-acquisition cases used by
build9. No private image is distributed with the source archive. All figures below
refer to the final build10 code and the unchanged the private physical-corpus key. `Best hard` is the
best hard bit-channel candidate overall. `Smooth before -> after` compares the same
candidate on both sides and therefore must not be confused with `Best hard`.

| Acquisition | Best hard post-ECC /24 | Best measured smooth same-candidate /24 | Smooth fit RMS | LOO RMS | Smooth resamples | Smooth HMAC slots | HMAC |
|---|---:|---:|---:|---:|---:|---:|---|
| scanner `0270_001.jpg` | 4 | 4 -> 9 | 1.362 | 1.969 | 2 | 0 | no |
| scanner `0270_002.jpg` | 5 | 8 -> 6 | 1.536 | 2.435 | 2 | 0 | no |
| `foto bici dritta.jpg` | 6 | 7 -> 3 | 1.500 | 2.461 | 2 | 1 | no |
| `foto bici storta.jpg` | 4 | 3 -> 8 | 1.555 | 2.432 | 2 | 0 | no |
| `foto stampa storta.jpg` | 7 | 8 -> 6 | 1.432 | 2.211 | 2 | 0 | no |
| `foto stampa.jpg` | 2 | 7 -> 5 | 1.525 | 2.389 | 2 | 0 | no |

The diagnostic field improves at least one measured candidate in four of six cases
and worsens the selected measured candidate in two. The strongest raw numerical
improvement is the bicycle frontal photograph, 7 -> 3 known post-ECC header errors,
but that field fails the strict leave-one-out decode gate and is not used for HMAC.

A different bicycle-frontal candidate (`fundamental-sync-subpixel`, mild-highpass)
has fit RMS about 1.215 blocks and leave-one-out RMS about 1.910 blocks. It passes
the strict gate, improves the same candidate from 6 -> 5 known post-ECC errors and
therefore replaces that candidate's ordinary grid in one of the existing four HMAC
slots. Authentication still fails.

The real-corpus conclusion is deliberately narrower than “smooth correction works”.
A continuous component is present strongly enough to improve selected candidates,
but one global affine field is not sufficiently predictive across the corpus. The
best-hard frontal photograph remains at 2/24 known post-ECC errors because its best
candidate contains only four complete spatial cells and therefore cannot support a
six-control field. Scanner cases likewise show that a very strong macro lattice does
not guarantee that one affine phase field describes the protected-bit channel.

Runtime remains bounded. On the development environment the two scanner cases
complete in about 10.2--10.3 s, the smartphone cases in about 12.7--17.4 s, and no
case exceeds the two-smooth-resample or four-HMAC-slot ceilings. No real case
produces a valid Format-v3 HMAC.

## v0.3.0-build11 continuous-control checkpoint — 2026-09-10

Build11 was evaluated on the same six private print-acquisition cases as builds 8--10
with the private physical-corpus key. The images remain private and are not distributed. The table uses
the final boundary-aware confidence implementation. `Best hard` is the existing hard
bit-channel summary. `Best measured smooth` compares the same candidate before and
after a build11 corrected-grid resample; `--` means no candidate passed the diagnostic
resample gate. A resampled field still receives no HMAC unless it also passes the
strict decode gate and improves the known prefix without increasing known coded-bit
errors.

| Acquisition | Best hard post-ECC /24 | Best measured smooth same-candidate /24 | Mean control confidence (measured field) | Fit RMS | LOO RMS | Smooth resamples | Smooth HMAC slots | HMAC |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| `foto stampa.jpg` | 2 | -- | -- | -- | -- | 0 | 0 | no |
| `foto stampa storta.jpg` | 7 | 7 -> 7 | 0.346 | 1.477 | 2.223 | 2 | 0 | no |
| `foto bici dritta.jpg` | 6 | 6 -> 8 | 0.475 | 1.067 | 1.766 | 1 | 0 | no |
| `foto bici storta.jpg` | 4 | **4 -> 3** | 0.354 | 1.175 | 2.182 | 2 | 0 | no |
| scanner `0270_001.jpg` | 4 | 4 -> 6 | 0.330 | 1.036 | 1.698 | 1 | 0 | no |
| scanner `0270_002.jpg` | 5 | 7 -> 7 | 0.314 | 1.293 | 2.103 | 2 | 0 | no |

The build11 control estimator materially changes the interpretation of build10. The
single build10 real field that entered an HMAC slot (`foto bici dritta`, 6 -> 5) no
longer improves when phase maxima at the edge of the bounded +/-2 search are explicitly
down-weighted. In final build11 the corresponding confidence-aware field is still
geometrically decode-eligible, but changes the known prefix from 6 -> 8; the existing
improvement requirement therefore prevents it from replacing the hard grid. No real
case consumes a smooth-field HMAC slot.

`foto bici storta.jpg` contains the strongest remaining same-candidate diagnostic
change: 4 -> 3 post-ECC known-header errors. Its robust affine fit RMS is about 1.175
blocks, but leave-one-out RMS is about 2.182 blocks, above the 2.0 decode limit. It is
therefore measured but not promoted. This is evidence that useful local correction
exists without evidence that the fitted global field predicts unseen cells reliably.

The regularized quadratic model is computed only with at least eight controls and is
selected only after an explicit leave-one-out gain. Across the six real acquisitions it
wins model selection on one `0270_002` candidate. That model has fit RMS about 0.775
blocks and leave-one-out RMS about 1.915 blocks, but predicts a maximum correction of
about 2.73 blocks, above the 2.5-block bound. It is rejected before resampling. The
corpus therefore does not currently justify promoting the quadratic model.

The front smartphone capture remains the closest hard channel case at 2/24 known
post-ECC errors. Its best hard candidate has only four complete spatial cells; its
leave-one-out behavior is unstable and no build11 smooth field is resampled. This
preserves the good hard candidate rather than forcing a higher-order correction.

Approximate diagnostic wall times in the development environment remain bounded:
about 9.2--11.3 s for the scanner cases and 11.5--17.4 s for the four smartphone cases.
No image authenticates. The negative result is intentional: build11 improves control
quality and rejects the only build10 real HMAC-field substitution instead of claiming
a fragile gain.


## v0.3.0-build12 blind-registration checkpoint — 2026-09-11

Build12 was evaluated on the same six private acquisitions with the private physical-corpus key; the
key is used only by the ordinary bit/HMAC diagnostic after the blind observer has
constructed its controls. `Local oracle` is the build11 key-assisted local-phase
majority result and is shown only for comparison. `Blind score/confidence/oracle
distance` come from the best hard candidate's independent blind observation. `Blind
smooth` compares the same candidate before/after a corrected-grid resample; `--` means
no blind field passed the diagnostic resample gate.

| Acquisition | Best hard /24 | Local oracle /24 | Blind structural profile | Blind global score | Mean blind conf. | Mean blind-oracle distance (blocks) | Best blind smooth same-candidate /24 | Smooth HMAC slots | HMAC |
|---|---:|---:|---|---:|---:|---:|---|---:|---|
| `foto stampa.jpg` | **2** | 10 | balanced | 0.193 | 0.080 | 1.625 | -- | 0 | no |
| `foto stampa storta.jpg` | 7 | **2** | balanced | **0.410** | **0.138** | 2.393 | **5 -> 4** | 0 | no |
| `foto bici dritta.jpg` | 6 | 4 | balanced | 0.209 | 0.092 | 2.528 | -- | 0 | no |
| `foto bici storta.jpg` | 4 | 3 | balanced | 0.121 | 0.073 | 1.992 | -- | 0 | no |
| scanner `0270_001.jpg` | 4 | 6 | balanced | 0.189 | 0.067 | 2.043 | -- | 0 | no |
| scanner `0270_002.jpg` | 5 | **3** | robust | 0.107 | 0.035 | 2.587 | -- | 0 | no |

Only the inclined historical smartphone acquisition produces blind fields that pass
the diagnostic resample gate. Its best measured blind field uses nine controls and
`control_source=blind-self-registration`; on a mild-highpass capacity candidate it
changes the same-candidate known post-ECC prefix from 5 to 4 errors. The robust affine
fit RMS is about `1.271` blocks and leave-one-out RMS about `2.041` blocks. The latter
remains above the strict `2.0` decode threshold, so the correction is not promoted to
an HMAC slot. A second field on the same image is also diagnostic-eligible but not
decode-eligible.

The blind-vs-oracle distances remain large (roughly 1.6--2.6 blocks on the reported
best-hard candidates), showing that the two observers do not yet identify the same
local phase consistently. This is particularly important because the key-assisted
oracle can reach 2/24 on `foto stampa storta.jpg` and 3/24 on scanner `0270_002`, while
the independent blind controls cannot yet reproduce those gains. The direct pairwise
cross-cell observer was also measured during development; real pair peaks were around
0.09 and often several blocks from the oracle, so intra-tile repetition became the
primary build12 observer.

A wrong-key run on `foto stampa storta.jpg` remains `projective-failed`, performs the
normal bounded four full-decode attempts and authenticates nothing. Blind structural
scores can still be nonzero under a wrong key because the observer itself is
key-independent; that score is therefore explicitly not a watermark-detection result.
No real acquisition in build12 produces a valid Format-v3 HMAC.


## v0.3.0-build13 guided dual-observer checkpoint — 2026-09-11

Build13 was evaluated on the same six private physical acquisitions. The guided
cross-cell observer uses no key or expected header values. `Observer distance` compares
its zero-mean controls with the primary repetition controls; `blind-oracle distance`
remains the ex-post comparison against the build11 key-assisted local-phase oracle.

| Acquisition | Best hard /24 | Repetition score | Guided pair score | Mean observer distance (blocks) | Mean blind-oracle distance (blocks) | Consensus / cycle fixes | Best blind smooth | HMAC |
|---|---:|---:|---:|---:|---:|---|---|---|
| `foto stampa.jpg` | **2** | 0.193 | 0.065 | 0.853 | 1.625 | 0 / 0 | -- | no |
| `foto stampa storta.jpg` | 7 | **0.410** | 0.062 | 0.587 | 2.393 | 0 / 0 | **5 -> 4** | no |
| `foto bici dritta.jpg` | 6 | 0.209 | 0.058 | 0.744 | 2.528 | 0 / 0 | -- | no |
| `foto bici storta.jpg` | 4 | 0.121 | 0.053 | **0.515** | 1.992 | 0 / 0 | -- | no |
| scanner `0270_001.jpg` | 4 | 0.189 | 0.060 | 0.652 | 2.043 | 0 / 0 | -- | no |
| scanner `0270_002.jpg` | 5 | 0.107 | 0.055 | 0.664 | 2.587 | 0 / 0 | -- | no |

The guided search is substantially better behaved than the unrestricted build12
pairwise control: its offsets stay within roughly one block of the primary rather than
wandering several blocks. However, absolute pair correlation remains weak. The
controlled synthetic consensus regression gives about 0.61 mean pair score, whereas
all six real best-hard observations remain below 0.07. The independent 0.10 strength
gate therefore keeps every real secondary observation in check-only mode. No real
control is cycle-slip adjusted.

Because weak secondary evidence is forbidden from vetoing the primary observer, the
build12 `foto stampa storta.jpg` result is preserved: two blind fields reach the
diagnostic resample stage and the best same-candidate result is 5 -> 4 post-ECC known
header errors. Its leave-one-out error remains above the strict decode gate, so it
receives no HMAC slot. The frontal image remains hard-best at 2/24 and is untouched.
No acquisition authenticates in build13.

## v0.3.0-build14 local lattice-phase checkpoint — 2026-09-11

Build14 was evaluated on the same four private smartphone captures and two private
scanner acquisitions with the private physical-corpus key. Originals remain outside source archives.
`Best smooth` is always a same-candidate before/after comparison; it must not be
compared directly with `Best hard` when the base value differs.

| Acquisition | Best hard /24 | Oracle local /24 | Lattice mean conf. | Mean fractional disagreement (blocks) | Lattice consensus / cycle fixes | Best smooth same-candidate /24 | Smooth mean conf. | Fit / LOO RMS | Smooth HMAC | HMAC |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| `foto stampa.jpg` | **2** | 10 | 0.419 | 0.314 | 3 / 0 | -- | -- | -- | 0 | no |
| `foto stampa storta.jpg` | 7 | **2** | 0.384 | 0.367 | 6 / 1 | **5 -> 2** | 0.174 | 1.240 / 1.952 | 0 | no |
| `foto bici dritta.jpg` | 6 | 4 | 0.407 | 0.398 | 5 / 2 | **7 -> 3** | 0.145 | 1.387 / 2.032 | 0 | no |
| `foto bici storta.jpg` | 4 | 3 | 0.438 | 0.447 | 3 / 3 | 9 -> 8 | 0.196 | 0.727 / 1.279 | 0 | no |
| scanner `0270_001.jpg` | **4** | 6 | 0.404 | 0.375 | 7 / 3 | **9 -> 4** | 0.125 | 1.168 / 2.262 | 0 | no |
| scanner `0270_002.jpg` | 5 | 3 | **0.522** | 0.372 | 6 / 2 | 7 -> 10 | 0.201 | 1.557 / 2.121 | 0 | no |

The lattice phase is substantially stronger than build13 guided cross-cell correlation,
but high geometric confidence is still not equivalent to a correct protected-bit grid.
The storta 5 -> 2 result is especially informative: its fit and LOO both pass the old
numeric geometry limits, yet mean fitted-control confidence is only about 0.174, below
the unchanged 0.20 decode gate. It therefore receives no HMAC slot. The straight-bike
7 -> 3 field misses both confidence and (slightly) the 2.0 LOO limit. Scanner 001 reaches
9 -> 4 but starts from a candidate worse than the global hard 4/24 and also fails the
confidence/LOO promotion gates.

Two explicitly recorded ablations prevent over-interpreting these gains. With the
opposite residual/correction sign interpretation the storta candidate remains 5 -> 5.
With the final observed-drift sign but cycle unwrap disabled, storta becomes 5 -> 6 and
straight-bike 7 -> 11. The strong final improvements therefore depend on the bounded
integer unwrap, which is also the least validated part of the method. Scanner 002's
7 -> 10 degradation confirms that the current unwrap is not ready for promotion.

Wrong-key control on `foto stampa storta.jpg` remains `projective-failed`, executes four
ordinary full-decode attempts, zero smooth decode attempts, and authenticates nothing.
Lattice phase remains measurable under a wrong key, as expected for key-independent
geometry evidence.



## v0.3.0-build15 global discrete unwrap checkpoint — 2026-09-11

Build15 changes the question from "can a local +/-1 unwrap improve the known prefix?"
to "is the integer cycle field uniquely selectable from key-independent geometry?".
Representative final diagnostics answer the second question conservatively: multiple
real cases admit much better geometric objectives but insufficient first/second margins.
Those proposals are rolled back transactionally and receive no HMAC slot.

For `foto stampa storta.jpg`, the global solver evaluates 1584 states, proposes changes
in nine cells and reduces its objective from about 5.480 to 0.804, but the alternative
solution is about 0.826 (margin about 0.021), so the unwrap is ambiguous. Zero changes
are committed. The safe pre-lattice blind field still measures 5 -> 4 on the same
candidate and remains above the strict smooth LOO gate.

This checkpoint intentionally does not reinterpret the build14 5 -> 2 / 7 -> 3 / 9 -> 4
results as successes. They remain valuable evidence that useful cycle assignments exist,
but build15 shows they are not yet independently identifiable with sufficient uniqueness.
Only a valid Format-v3 HMAC is success.

## v0.3.0-build16 exact top-2 checkpoint — 2026-09-11

Build16 first corrects the uniqueness certificate itself. Build15's width-64 beam could
prune a partial assignment that later became the true runner-up, so its first/second
margin was not guaranteed to be global. The new solver enumerates the complete bounded
`{-1,0,+1}` state space independently for X and Y (at most `3^9 = 19683` assignments
per axis), preserves the existing key-independent objective and acceptance thresholds,
and obtains the exact top-1/top-2. A deterministic regression captures an actual
false-accept pattern from the old beam: the optimum remains `0.027408951002`, while the
runner-up changes from the beam's `0.062494434795` to the exact `0.035129429298`, which
correctly makes the field ambiguous under the unchanged gate.

The no-eligible, insufficient-coverage and partial-lattice paths are now explicit
`not-applicable` outcomes rather than malformed/ambiguous solutions, and public numeric
diagnostics remain finite/JSON-serializable. Per-axis status and eligibility are
reported separately. These changes do not consume additional HMAC attempts or smooth
resamples.

Build16 also adds `split-repetition-top2`, a key-independent consistency diagnostic that
compares the exact geometric top-1/top-2 with two deterministic disjoint partitions of
the existing repeated-position evidence. It cannot override ambiguity: the primary
repetition controls were estimated from the complete pair set, so this is useful
structural telemetry but not a fully held-out certificate.

Only four of the six canonical physical acquisitions were available in this build16
session. The two bicycle photographs were absent. The two historical smartphone files
are now canonically named `.jpg`, matching their JPEG encoding; only their filenames
changed.

| Acquisition | Best hard /24 | Exact states | Eligible X/Y | Baseline -> best | Exact second | Margin | Split fold deltas | Split supports best | Same-candidate smooth | Smooth HMAC | HMAC |
|---|---:|---:|---:|---:|---:|---:|---|---|---|---:|---|
| `foto stampa.jpg` | **2** | 0 | 0/0 | n/a | n/a | n/a | n/a | n/a | -- | 0 | no |
| `foto stampa storta.jpg` | 7 | 8748 | 7/8 | 5.4804 -> 0.8045 | 0.8256 | 0.02113 | +0.1002 / -0.0361 | no | **5 -> 4** | 0 | no |
| scanner `0270_001.jpg` | **4** | 26244 | 9/8 | 5.7797 -> 0.8083 | 0.8297 | 0.02144 | -0.0828 / -0.0343 | no | -- | 0 | no |
| scanner `0270_002.jpg` | 5 | **39366** | 9/9 | 7.3178 -> 1.7229 | 1.7407 | **0.01780** | +0.0410 / -0.0929 | no | -- | 0 | no |

`foto stampa storta.jpg` therefore preserves the conservative build15 result: a large
geometric objective improvement exists, but the exact competitor is too close and the
two split-repetition folds disagree. Scanner `0270_001.jpg` is even stronger negative
evidence for the selected geometric top-1 because both split folds prefer its runner-up.
Scanner `0270_002.jpg`, the historical counterexample, tightens from the build15 beam
margin of roughly `0.02634` to an exact margin of `0.01780`; its split folds also
disagree. No proposal is promoted and no smooth HMAC slot is consumed.

No available real acquisition authenticates in build16. The next useful experiment must
construct proposal and validation evidence that are disjoint by design and then be
frozen before the complete six-image corpus is evaluated. A lower known-header error
count remains research evidence only; success still requires an ordinary valid Format-v3
HMAC.
