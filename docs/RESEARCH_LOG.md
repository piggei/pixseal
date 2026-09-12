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

## build17 — coded-bit-group-disjoint repetition cross-fit

### Hypothesis

Build16 showed that repetition scores contain information about the geometric top-1 vs
runner-up, but its fold scores were not truly held out because the primary repetition
controls had already been estimated from all repetition pairs. The build17 hypothesis
was that a proposal constructed from one independent subset of Format-v3 repetition
structure could be validated by the other subset strongly enough to discriminate the
integer cycle without expected bits, key material or HMAC.

### Predeclared construction

- Keep Format v3, encoder, production decoder, exact build16 unwrap objective and
  HMAC/smooth-resample budgets frozen.
- Partition by coded-bit group, not by individual pair. All repeated positions belonging
  to one coded bit stay in one fold. The two folds therefore share no logical repetition
  position.
- Direction A->B: estimate global/local repetition controls using fold A only, combine
  with the existing key-independent fractional lattice phase, enumerate exact top-1/
  top-2, then score only those two assignments with fold B.
- Direction B->A repeats the experiment with roles reversed.
- Compare the two independently proposed recentered local integer-cycle fields. Do not
  compare fold-specific absolute global tile anchors.
- `crossfit_supports_best` requires both held-out directions to favor their own top-1,
  at least three comparable cells and complete local-cycle agreement. This flag is
  diagnostic only and cannot override all-pairs ambiguity.
- Run cross-fit only when the normal all-pairs exact solver is already `ambiguous` and
  has an exact runner-up. This bounds duplicate search and prevents the experiment from
  becoming a general candidate multiplier.

### Synthetic regressions

`make crossfit-unwrap-test` verifies:

1. robust and balanced repetition groups partition into two non-empty folds;
2. no logical repetition position appears in both folds;
3. fold-specific repetition estimation uses the bounded proposal subset;
4. a held-out fold can prefer a known correct phase over a one-cycle alternative;
5. both symmetric directions execute through the exact solver on a bounded synthetic
   field.

### Physical evidence (four available acquisitions)

| acquisition | all-pairs result | A->B delta | B->A delta | cycle agreement | overall cross-fit |
|---|---|---:|---:|---:|---|
| `foto stampa.jpg` | not-applicable, best hard 2/24 | n/a | n/a | n/a | not run |
| `foto stampa storta.jpg` | ambiguous, margin 0.02113 | +0.00465 | +0.20522 | 2/9 | reject |
| `0270_001.jpg` | ambiguous, margin 0.02144 | -0.18283 | -0.02287 | 0/9 | reject |
| `0270_002.jpg` | ambiguous, margin 0.01780 | +0.05639 | -0.24050 | 0/9 | reject |

The best-candidate fold profile is balanced in all three ambiguous cases; the group
partition yields 220 proposal pairs in fold A and 228 in fold B. Exact cross-fit state
counts remain below the declared worst-case ceiling. Measured end-to-end diagnostic
runtime remains around 10--15 seconds for the reported best cases once cross-fit is
restricted to already-ambiguous candidates.

### Interpretation

The hypothesis is only partially supported. Truly held-out repetition evidence is
clearly informative: scanner 001 independently rejects the geometric top-1 twice, while
scanner 002 exposes directional instability. However, even the inclined smartphone case
where both held-out directions are positive does not reproduce the same integer-cycle
field across folds. Positive top-1 validation alone is therefore insufficient.

Build17 does **not** promote any unwrap, does not consume an extra HMAC slot and does not
reinterpret lower known-header errors as success. The next problem is candidate
stability/reconciliation across independent evidence partitions, or an additional
key-independent signal that can anchor the cycle field itself.



## build18 — lazy cross-fit and cell-level instability localization

### Hypothesis

Build17 showed useful held-out information but unstable local cycle fields. Before
adding another observer, determine whether agreement is concentrated in high-quality
controls, and remove duplicated cross-fit work from candidates that cannot become the
reported best bit-channel result.

### Implementation

- Freeze Format v3, production extraction, all-pairs exact objective/thresholds,
  build17 fold partition and held-out scoring.
- Run all-pairs exact unwrap on every existing bit diagnostic as before.
- Defer A->B/B->A cross-fit until bit diagnostics are ranked; run it at most once on the
  final best candidate and only for `ambiguous` + exact-runner-up cases.
- Export cross-fit milliseconds, attempts and skipped candidates.
- Export per-cell all-pairs proposed shifts, A/B local cycles, A/B confidence and cycle
  agreement. Summarize agreeing/disagreeing joint fold confidence and lattice confidence.

### Four-case physical result

The scientific decisions are identical to build17. The frontal photograph remains
`2/24` and not-applicable. The inclined photograph remains ambiguous with both held-out
deltas positive and only 2/9 cycle agreement. Scanner 001 remains 4/24 with both
held-out deltas negative and 0/9 agreement. Scanner 002 remains 5/24, directionally
split, with 0/9 agreement.

The new localization falsifies a simple partial-consensus hypothesis. On the inclined
photo, the two agreeing cells are `(0,1)` and `(1,2)`, but their mean joint fold
confidence is only `0.1337`; the seven disagreeing cells average `0.3576`. Lattice
confidence is `0.4059` for agreeing versus `0.3772` for disagreeing cells, not enough to
rescue the weak repetition consensus. The agreement subset is therefore not a
high-confidence core that can be promoted safely.

On the development host the selected held-out experiment costs only tens of milliseconds
(26/48/73 ms on the three ambiguous best candidates), with three lower-ranked bit
candidates skipped. Absolute end-to-end timings are host-dependent; the invariant to
preserve is one cross-fit attempt maximum per image candidate ranking.

### Status

Retain build18 telemetry and lazy scheduling. Do not build a decoder correction from the
2/9 inclined-photo agreement. The next experiment should add or derive a genuinely
independent cycle anchor, or demonstrate stability across additional deterministic
partitions/acquisitions, before relaxing any ambiguity gate.


## build19 — multi-partition integer-cycle stability

### Hypothesis

Build17/18 may have observed an unlucky two-way split. If repetition evidence contains a
real cycle anchor, the same recentered local integer field should recur when coded-bit
groups are repartitioned several deterministic ways, especially among proposals that are
positively validated by their held-out complement.

### Frozen experimental design

- 8 deterministic binary partitions, fixed before physical evaluation.
- Each coded-bit group is indivisible; proposal and validation evidence never share a
  repetition group.
- Both directions are evaluated for each partition: 16 maximum trials.
- Partition 0 is exactly the historical build17 A/B split; the other seven are distinct
  modulo complement.
- The experiment runs only on the final best bit candidate, only after all-pairs exact
  unwrap is ambiguous with an exact runner-up.
- No modal vote, supported-trial filter or stability statistic can alter sampling, smooth
  resampling, full-decode count or HMAC attempts.

### Four-case result

| case | trials | held-out supported | unique complete fields | supported unique fields | mean cell modal | supported mean modal | mean pairwise agreement |
|---|---:|---:|---:|---:|---:|---:|---:|
| frontal photo | not run | n/a | n/a | n/a | n/a | n/a | n/a |
| inclined photo | 16/16 | 9 | 16/16 | 9/9 | 0.2153 | 0.2716 | 0.0731 |
| scanner 001 | 16/16 | 6 | 16/16 | 6/6 | 0.2153 | 0.2963 | 0.0759 |
| scanner 002 | 16/16 | 12 | 16/16 | 12/12 | 0.2361 | 0.2593 | 0.0824 |

No cell is unanimous across all 16 trials in any ambiguous case. Even the most frequent
per-cell cycle receives only a small minority of votes. Scanner 002 is particularly
instructive: 75% of trials positively validate their own top-1, yet all 12 supported
complete fields are different. Positive held-out validation therefore does not imply a
reproducible field.

### Interpretation / decision

The hypothesis is rejected on the current corpus. The build17 A/B instability is not an
artifact of one unlucky split: repetition-derived cycle fields are highly partition
sensitive. Additional repetition voting or threshold relaxation would convert instability
into confidence rather than add independent information. Keep exact rollback and all HMAC
budgets unchanged. The next research branch must add a genuinely independent cycle anchor
or physical model.

## build20 — independent cross-cell cycle-anchor experiment

Hypothesis: unguided cross-cell image-domain registration, which uses neither coded-bit
repetition nor any key/header oracle, may supply an independent absolute-relative anchor for
the exact integer-cycle top-2 ambiguity.

Predeclared construction: run only on the final best candidate when the exact all-pairs solver
is ambiguous and has a runner-up. Build the existing unguided pairwise correlation graph,
solve its zero-mean relative offsets, and score exact top-1 and runner-up after removing the
best common integer x/y gauge. No threshold can promote a field in build20; sign and agreement
are telemetry only.

Physical result: inclined smartphone delta(second-top1)=+0.5083, mean confidence 0.0575;
scanner001 +0.2277, confidence 0.0168; scanner002 +0.0956, confidence 0.0798. All three prefer
top-1 continuously, but both top-1 and runner-up have 0/9 rounded-cycle agreement with the
anchor. Frontal smartphone is not-applicable and the anchor is not run. Scanner001 contradicts
the repetition held-out preference; scanner002 negative control also prefers top-1.

Conclusion: pairwise registration contains continuous relative-shape information but is not an
absolute integer-cycle anchor on the current corpus. No unwrap, HMAC, encoder or format rule is
changed.
