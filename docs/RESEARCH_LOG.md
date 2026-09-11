# PixSeal research log

This file is an **append-only engineering/research notebook** for the v0.3 physical
print-acquisition work. `HISTORY.md` records milestones; `docs/RESULTS.md` records
checkpoint measurements; this file additionally preserves hypotheses, rejected
variants, threshold decisions and negative results so later work can reconstruct why a
path was accepted or abandoned.

Rules for future entries:

- never call a known-header improvement authentication; only Format-v3 HMAC is success;
- distinguish global-best hard values from same-candidate before/after comparisons;
- state whether a measurement is key-independent, key-assisted oracle, or HMAC-gated;
- record failed/rejected variants instead of deleting them from the historical record;
- record bounded search budgets and threshold changes, especially when a threshold is
  deliberately *not* relaxed after a promising result;
- private print-camera/scanner images are referenced by regression-case name only and
  are never copied into source archives.

## Compact chronology

| Build | Main question | Outcome |
|---|---|---|
| build1 | Can local lattice evidence be measured separately from production decode? | Yes; bounded diagnostic lattice path established. |
| build2 | Can print boundary + homography + virtual DCT sampling recover synthetic perspective? | Yes synthetically; real HMAC still absent. |
| build3 | Does spatial phase consensus improve projective geometry? | Useful evidence, insufficient real authentication. |
| build4 | Are scale aliases/harmonics hiding the fundamental carrier period? | Fundamental-scale and modulo-8 refinement added. |
| build5 | Is residual photometric variation the main blocker? | Small bounded bank helps some scores, no real HMAC. |
| build6 | Are large no-boundary photos blocked too early? | Adaptive finer lattice and weak-boundary lattice-first fallback added. |
| build7 | Is the failure before or after ECC? | Six known Hamming words expose protected-bit failure without revealing payload. |
| build8 | Can reliability-aware Hamming rescue weak words? | Synthetic yes; real cases no. |
| build9 | Are errors systematic or spatially local? | Mostly mixed/local; global inversion hypothesis rejected. |
| build10 | Can one low-DOF affine phase field remove spatial drift? | Some candidate improvements, no general solution. |
| build11 | Were integer local maxima/control quality causing false fields? | Yes; confidence/boundary-aware controls reject prior weak promotion. |
| build12 | Can controls be estimated without key/header? | Repetition-coherence blind observer established; first real blind 5 -> 4. |
| build13 | Can guided cross-cell correlation resolve cycle slips? | No; real pair score 0.053--0.065 vs ~0.61 synthetic. Kept diagnostic-only. |
| build14 | Can independent local lattice phase supply fractional registration and assist unwrap? | Stronger signal and large same-candidate gains, but unwrap remains mixed and unpromoted. |

## Build14 experiment notebook — local lattice fractional phase

### Hypothesis

The repetition observer can often identify an approximate integer tile/block phase but
retains cycle ambiguity. The pre-existing local lattice estimator measures physical
8-pixel periodicity independently of the key. Projecting a regional lattice
intersection back into the candidate canonical plane should therefore provide the
**fractional** phase inside one block. Combining integer repetition phase with this
modulo-block measurement may reduce spatial registration error without looking at the
known Format-v3 prefix.

### Fixed constraints

- Format v3 and encoder unchanged.
- Production `ExtractWithInfo` unchanged.
- Four complete/HMAC decode slots maximum.
- Two smooth corrected-grid resamples maximum.
- Nine lattice-phase controls maximum (one per 3x3 region).
- Lattice phase sees no key, magic, header bits, payload, CRC or HMAC result.
- Key-assisted local phase is retained only for ex-post oracle distance.

### Attempt A — build13 guided cross-cell retained as a check

The build13 guided cross-cell observer was not removed, because its weakness is useful
negative evidence. It remains an independent check but does not substitute for the
new lattice phase. Real pair scores previously measured around 0.053--0.065 remain far
below the 0.10 build13 consensus-strength gate.

Decision: **retain for diagnostics, do not tune further**.

### Attempt B — initial residual-as-inverse-correction sign

The first lattice implementation treated the canonical modulo-grid residual as if the
observer should directly encode the inverse sampler correction. On
`foto stampa storta.jpg`, the lattice controls were measurable and the smooth fit
improved geometrically, but the best same-candidate bit result was only **5 -> 5**.

Reviewing the internal smooth-phase convention showed that local controls represent
*observed phase drift*; `diagnosticFitBlindSmoothPhaseField` performs the sign inversion
when constructing the correction. The lattice observer was therefore changed to report
the observed canonical drift directly.

Decision: **reject inverse-correction sign; preserve this trial as convention history**.
The final sign choice is defined by internal phase semantics, not by selecting whichever
known-header result is numerically better.

### Attempt C — observed-drift sign, fractional phase only

With the final sign convention but integer unwrap disabled:

- `foto stampa storta.jpg`: best measured same candidate **5 -> 6**;
- `foto bici dritta.jpg`: best measured same candidate **7 -> 11**.

This is a useful negative result. A lattice phase reduced modulo one block cannot know
which integer block cycle is correct. Replacing/refining only the fractional component
without solving the cycle index is insufficient and can move the sampler in the wrong
periodic basin.

Decision: **fractional-only fusion is insufficient**.

### Attempt D — bounded affine-guided +/-1 unwrap (final build14 experiment)

The final research path keeps repetition as the initial integer-cycle estimate and
lattice phase as independent fractional evidence. A low-confidence primary control may
move by at most +/-1 block per axis only when a robust affine fit to all controls shows:

- lattice confidence >= 0.22;
- primary confidence <= 0.24;
- residual improvement >= 0.45 block;
- final residual <= 0.55 block;
- at most two unwrap passes.

These conditions are evaluated before known-header diagnostics.

Observed final same-candidate results include:

| Acquisition | Best hard /24 | Same candidate before -> after | Lattice consensus / cycle fixes | Smooth mean confidence | Fit / LOO RMS | Promoted to HMAC? |
|---|---:|---:|---:|---:|---:|---|
| `foto stampa.jpg` | 2 | -- | 3 / 0 | -- | -- | no |
| `foto stampa storta.jpg` | 7 | **5 -> 2** | 6 / 1 | 0.174 | 1.240 / 1.952 | no |
| `foto bici dritta.jpg` | 6 | **7 -> 3** | 5 / 2 | 0.145 | 1.387 / 2.032 | no |
| `foto bici storta.jpg` | 4 | 9 -> 8 | 3 / 3 | 0.196 | 0.727 / 1.279 | no |
| scanner `0270_001.jpg` | 4 | **9 -> 4** | 7 / 3 | 0.125 | 1.168 / 2.262 | no |
| scanner `0270_002.jpg` | 5 | **7 -> 10** | 6 / 2 | 0.201 | 1.557 / 2.121 | no |

The storta 5 -> 2 result is especially interesting because it reaches the same error
count as the key-assisted local oracle on that acquisition. It is **not** an HMAC
success and it is **not** the global hard candidate moving from 7 -> 2. It is one
separately ranked candidate moving 5 -> 2 after a blind field.

The method is not ready for promotion. The three large improvements all fail at least
one unchanged decode gate (especially the 0.20 mean-control-confidence threshold and,
in some cases, the 2.0 LOO threshold). Scanner 002 demonstrates a substantial
same-candidate degradation. Therefore no threshold was relaxed, zero build14 smooth
fields receive an HMAC slot, and all six physical acquisitions remain unauthenticated.

### Wrong-key control

On `foto stampa storta.jpg` with an unrelated key, final build14 remains
`projective-failed`, performs four ordinary full-decode attempts, zero smooth decode
attempts and authenticates no payload. Lattice-phase confidence remains measurable,
which is correct because it is key-independent geometry evidence rather than watermark
detection.

### Research conclusion

Build14 supports three statements and rejects a fourth:

1. **Supported:** local lattice phase is a materially stronger independent measurement
   than direct cross-cell DCT-pattern correlation on this corpus.
2. **Supported:** combining fractional lattice phase with a bounded integer unwrap can
   move some real same-candidate protected-bit channels dramatically closer to the
   known-header oracle.
3. **Supported:** a low geometric residual/LOO alone still does not guarantee a better
   bit channel, so confidence and HMAC gates must remain independent.
4. **Rejected:** the current affine-guided +/-1 unwrap is not yet reliable enough for
   decode promotion across acquisitions.

Next experiments should target the **discrete unwrap problem**, preferably with a third
key-independent signal (gradient/spectral phase, neighbourhood consistency, or a global
small-state smoothness objective). Do not increase HMAC slots, lower the 0.20 confidence
gate, lower the LOO gate, or introduce Format v4 solely to admit the current results.


## build15 — global discrete integer unwrap and transactional rollback

### Hypothesis
The build14 fractional lattice observer appears informative, while the integer cycle
assignment is the unstable component. Test whether a global smoothness objective can
choose the integer cycles independently of known Format-v3 bits.

### Attempt A — axis-separable bounded beam search
Each populated cell may retain its repetition cycle or move by exactly one block in
either direction. X and Y are solved separately with beam width 64. The score is a
combination of robust affine residual, confidence-weighted cycle-change penalty and a
second-difference curvature term. The search is deterministic and bounded; the declared
worst-case evaluation budget is 3456 states across both axes.

### Attempt B — accept geometric optimum without ambiguity margin (rejected)
Early real runs showed that a dramatically lower global objective can still correspond
to many nearly equivalent integer assignments. Treating the best state alone as
authoritative would recreate the build14 over-selection problem in a more complicated
form. The solver was therefore changed to retain the second-best state and require both
absolute and relative first/second margins.

### Attempt C — reporting baseline as best after rejection (bug, fixed)
An intermediate diagnostic path rolled the applied controls back correctly but mixed the
rolled-back baseline objective with the second-best proposed objective, allowing a
negative-looking margin in JSON. Reporting now separates baseline, proposed, applied and
second-best objectives. No decoding behavior depended on the reporting bug.

### Attempt D — fractional lattice survives ambiguous unwrap (rejected)
This was methodologically unsafe. Build14's explicit ablation already showed that
fractional-only correction can degrade the bit channel. Build15 therefore makes lattice
fusion transactional: if the integer unwrap is ambiguous/rejected, fractional lattice
changes are rolled back as well. A dedicated regression prevents future leakage.

### Representative real evidence
`foto stampa storta.jpg`: baseline objective about 5.480, proposed about 0.804,
second-best about 0.826, margin about 0.021. The proposal changes nine cells but is
ambiguous, so zero global changes are applied and the safe same-candidate diagnostic
returns to 5 -> 4 rather than build14's 5 -> 2.

`foto bici dritta.jpg`: baseline about 6.159, proposed about 1.267, margin about 0.011;
proposal rejected as ambiguous. Scanner `0270_002.jpg`: baseline about 7.318, proposed
about 1.723, margin about 0.026; proposal likewise rejected. These examples demonstrate
that geometric improvement alone does not establish a uniquely recoverable cycle field.

### Status
The global unwrap machinery is retained as RESEARCH. It improves the falsifiability of
the experiment by refusing non-unique cycle assignments, but it does not yet justify a
new real HMAC attempt. Format v3 and the production decoder remain frozen.

## build16 — exact top-2 certification and conservative structural validation

### Hypothesis
The build15 global formulation is useful, but a width-64 beam cannot certify the
first/second solution margin. First remove that approximation. Then ask whether a
key-independent structural comparison of the exact top-1/top-2 assignments provides
additional evidence without turning HMAC or the known header into a search oracle.

### Audit finding A — beam top-2 is not globally certified
A deterministic synthetic field was found for which the build15 beam accepted an X
unwrap with objective 0.027408951002 and beam runner-up 0.062494434795. Exact bounded
enumeration finds the same optimum but a better runner-up at 0.035129429298. The exact
margin falls below the fixed gate, so the correct result is ambiguous. This is a
false-accept hole in the beam uniqueness certificate, not merely a numerical change.

### Attempt A — exact axis enumeration (retained)
Build16 enumerates all `3^N` assignments for the `N <= 9` eligible controls of each
axis. The objective and thresholds are unchanged; only search completeness changes.
Maximum work is 19683 assignments per axis / 39366 across X/Y. A permanent regression
contains the deterministic former beam false-accept field. Exact top-1/top-2 ranking,
no-slip behavior, high-confidence locking, ambiguous rollback and HMAC restoration on
a controlled synthetic field remain covered.

### Audit finding B — mixed-axis status could be misleading (fixed)
The outer lattice fusion already refused an ambiguous global result, but the inner
solver could temporarily apply an accepted X or Y axis while the other axis was
ambiguous and report the global state as accepted. Build16 makes the transaction
atomic at the solver boundary too: one ambiguous axis means global `ambiguous`, zero
applied axes and baseline applied objective. A dedicated regression preserves the
per-axis evidence while asserting zero committed changes.

### Audit finding B — non-finite no-eligible reporting (fixed)
Build15 could leave `secondObjective=+Inf` when no control was eligible. Go's JSON
encoder rejects non-finite floats. Build16 reports the axis as `not-applicable`, keeps
numeric placeholders finite, and exports whether a second solution is actually
available. A regression serializes the public diagnostic structure for this path.

### Attempt B — split repetition top-2 validator (diagnostic-only)
The exact top-1/top-2 combined assignments are additionally compared using two
deterministic disjoint subsets of the existing Format-v3 repetition pairs. Each fold
measures same-coded-bit agreement only; no expected value, key, CRC or HMAC is used.
The signal is exported as `split-repetition-top2` with per-fold deltas and agreement.
It is not allowed to override an exact ambiguous result because the primary repetition
controls were originally estimated from the complete pair set. Treating this split as
fully held-out would overstate its independence.

### Physical evidence available in this build16 session
Four of the six canonical acquisitions were available. The two bicycle photographs
were absent, so no six-case promotion claim is made. On the available cases exact
ranking preserves the conservative build15 behavior. In particular, `foto stampa
storta.jpg` remains ambiguous with the same exact top-2 margin (~0.02113), while
scanner `0270_002.jpg` exposes the beam approximation clearly: its reported build15
margin ~0.02634 becomes exact ~0.01780. The proposal is still rejected, so the safe
behavior is unchanged.

### Status
Exact bounded enumeration is retained and replaces beam ranking. Split-repetition
validation is retained as research telemetry only. The next integer-cycle experiment
must use proposal and validation evidence that are disjoint by construction and must be
evaluated on the complete six-image corpus before it can relax an ambiguity decision.
