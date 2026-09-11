# PixSeal

![PixSeal — Hide messages. Keep the picture.](docs/assets/pixseal-banner.png)

PixSeal is an experimental, pure-Go **robust image steganography** tool for
hiding short authenticated messages inside images. It embeds protected payload
bits in luminance DCT coefficients while trying to keep the visual change small
under normal viewing conditions.

Current development snapshot: **v0.3.0-build16**.

Stable release baseline: **v0.2.0**.

PixSeal is a hidden-data channel, not an ownership-marking product. Digital
watermarking is the robustness mechanism; the project goal is robust
steganography for short messages. PixSeal does **not** claim statistical
undetectability and has not undergone a professional cryptographic or
steganalytic audit.

Format v3 remains the interoperability baseline. Its on-image layout and
deterministic encoder fingerprints are **frozen** during the first v0.3 research
phase: the new work is decoder/diagnostic geometry only unless oracle evidence
demonstrates that the physical channel destroys the v3 signal.

Project history and future work are kept in [`HISTORY.md`](HISTORY.md) and
[`TODO.md`](TODO.md). Release-facing changes are in [`CHANGELOG.md`](CHANGELOG.md).
The append-only research notebook in [`docs/RESEARCH_LOG.md`](docs/RESEARCH_LOG.md)
records hypotheses, rejected variants, threshold decisions and negative results so
future builds do not silently repeat abandoned experiments.

## What v0.2.0 provides

The release baseline covers:

- adaptive Format v3 profiles: `robust`, `balanced`, `capacity`, `auto`;
- authenticated payloads up to 64 bytes;
- CRC32, truncated HMAC-SHA256, key-derived whitening and Hamming(7,4);
- JPEG/PNG input and PNG output;
- robust digital JPEG, resize and crop recovery on the qualification corpus;
- v3-only extraction with automatic profile detection;
- a reusable pure-Go core compile-checked for Linux, Windows, Android and iOS.

The decoder also contains **experimental**, bounded recovery for arbitrary
rotation, axis-aligned affine transforms, a fixed direct lattice-basis bank and
two mild projective/keystone hypotheses. These are measured research
capabilities, not universal guarantees. General homography estimation and the
physical print-camera channel are outside v0.2.0 scope.

## What v0.3.0-build16 adds

Build16 hardens the global integer-cycle barrier introduced in build15. The former
width-64 beam search could discard a partial state that later became the true optimum
or runner-up, so its reported first/second margin was not a global uniqueness
certificate. Build16 removes that approximation: for each axis it enumerates the full
bounded `{-1,0,+1}` state space over eligible cells, at most `3^9 = 19683`
assignments per axis (`39366` across X/Y), and ranks the exact top-1/top-2 using a
deterministic tie break. The objective, thresholds and transactional rollback remain
key-independent and unchanged in meaning.

The exact solver closes a real false-accept hole found during the build15 audit. A
fixed regression reproduces a field for which the old beam margin would have accepted
an unwrap while the exact runner-up makes it ambiguous. Build16 also reports separate
X/Y eligibility and status, distinguishes `not-applicable`, `no-change`,
`rejected-improvement`, `ambiguous` and `accepted`, and never exports `Inf`/`NaN` for
the no-eligible path. A JSON regression protects that invariant. Mixed-axis cases are
also atomic: ambiguity on either axis prevents a partial integer-cycle commit and the
global result remains `ambiguous`.

As a second research diagnostic, build16 compares the exact geometric top-1/top-2
assignments using two deterministic disjoint subsets of the existing key-independent
Format-v3 repetition pairs. This `split-repetition-top2` signal is intentionally
**diagnostic-only**: the primary repetition controls were estimated from the complete
pair set, so the split is consistency evidence, not a sufficiently independent
held-out certificate and cannot override an exact ambiguous result.

The physical-corpus contract is also hardened. `PRINT_CAMERA_KEY` no longer has a
source-tree default, both smartphone and scanner directories are ignored by Git, and
`make private-corpus-manifest` can generate a local SHA-256 manifest. The canonical
smartphone filenames now use `.jpg`, matching their actual JPEG encoding; see
[`docs/PRIVATE_CORPUS.md`](docs/PRIVATE_CORPUS.md).

Format v3, deterministic encoding, the production extractor, profiles, whitening,
ECC, tile mapping and HMAC remain frozen. No HMAC or smooth-resample budget is
increased, and no diagnostic error-count improvement is treated as authentication.

## What v0.3.0-build14 adds

Build14 changes the second blind signal rather than expanding the HMAC or geometry
candidate bank. It reuses the already-selected 3x3 `LocalLatticeEstimate` records and
projects one measured lattice intersection per region back into each candidate's
canonical plane. The residual position modulo the frozen 8-pixel Format-v3 block grid
is reported as a **local fractional lattice phase**. It is key-independent: no magic,
header bits, payload data, whitening sequence or HMAC result enters the measurement.

The lattice observer cannot know the absolute integer block cycle by itself. Build14
therefore keeps repetition coherence as the cycle-index prior and uses lattice phase
only as modulo-one-block evidence. Fractional agreement can refine a control. A
bounded +/-1 integer unwrap is considered only for a low-confidence repetition
control when an affine robust prediction of the other cells shows at least 0.45 block
residual improvement and leaves at most 0.55 block residual. At most two unwrap passes
are allowed. No additional source-image DCT sampling is introduced.

The real result is promising but not promotable. Lattice-phase confidence is much
stronger than the build13 cross-cell score (roughly 0.38--0.52 mean confidence on the
reported best-hard spatial cases). With the final observed-drift sign convention and
bounded unwrap, several **same-candidate** diagnostic corrections improve sharply:
`foto stampa storta.jpg` 5 -> 2, `foto bici dritta.jpg` 7 -> 3 and scanner
`0270_001.jpg` 9 -> 4. The global hard baselines remain 7, 6 and 4 respectively; these
numbers must not be compared as if they were the same candidate.

None of those fields consumes an HMAC slot. Their mean fitted-control confidence stays
below the strict 0.20 decode gate and/or leave-one-out remains above 2.0 blocks. Other
candidates also worsen (for example scanner `0270_002` 7 -> 10), so the bounded unwrap
is retained as research evidence rather than promoted. All six physical acquisitions
remain unauthenticated.

Build14 adds `make lattice-phase-test` and expands `make test-list` with an `INPUT`
column and explicit notes that `pics/` qualification/research suites are corpus-sensitive
while `release-check` remains the production gate. `all-test` now contains 23 targets.

## What v0.3.0-build13 adds

Build13 keeps the build12 repetition-coherence observer as the primary key-independent
source of spatial controls and adds a second **guided cross-cell image-domain observer**.
For each pair of populated 3x3 cells, cross-correlation is searched only in a +/-1
integer-block neighbourhood around the relative offset predicted by repetition
coherence. This costs at most 36*9 = 324 pair probes when the primary observer exists;
the older +/-4 / 2916-probe pairwise search is retained only as fallback if repetition
evidence is unavailable. No additional source-image DCT sampling is introduced.

The secondary observer is deliberately unable to override the primary merely because a
local cross-correlation maximum exists. Its mean pair peak must exceed an independent
strength gate before the two observers can be fused or a bounded +/-1 cycle-slip
correction can be applied. Weak secondary evidence is still reported, including
observer distance and per-cell confidence, but it cannot lower the primary confidence,
change the smooth field or consume an HMAC slot.

On the six current physical acquisitions the guided observer follows the primary much
more closely than the old unrestricted pairwise search (mean observer distances about
0.51--0.85 blocks on the best hard candidates), but its mean pair peak is only about
0.053--0.065. A controlled synthetic field produces about 0.61, so no real case clears
the 0.10 consensus-strength gate. Consequently build13 applies zero cycle-slip fixes
on the real corpus and preserves the build12 blind result: `foto stampa storta.jpg`
still measures a same-candidate 5 -> 4 post-ECC improvement, remains above the strict
LOO decode gate and consumes no HMAC slot. No real acquisition authenticates.

Build13 also adds `make test-list`, a categorized index of release, qualification,
research, private physical-channel, portability and aggregate test targets. It is an
informational target and is not itself part of the 22-target `all-test` matrix.

## What v0.3.0-build12 adds

Build12 changes the **source of the spatial phase controls**, not the Format-v3
layout or the smooth-field model. The projective diagnostic now has a
key-independent blind registration observer. Its primary score uses the structural
redundancy already present in the robust and balanced Format-v3 profiles: positions
that map to the same coded bit should have coherent observed DCT margins at the
correct tile phase even when the bit value, key and payload are unknown. Capacity
has no repetition and is intentionally not treated as structural evidence.

The observer first searches the complete 35x32 tile phase on the already-built
aggregate grid for robust and balanced repetition coherence (at most 2240 score
probes). Each populated 3x3 spatial cell then searches only +/-2 blocks around that
blind global phase (at most 225 local probes total), with the same sub-block peak
interpolation and boundary-aware confidence logic introduced in build11. The common
translation is removed before fitting so the smooth field models only residual
spatial drift. If structural repetition is unavailable, a separately reported
pairwise cross-cell correlation fallback is bounded to at most 2916 score probes.
None of these blind probes resamples the source image or uses the key, expected
header bits, CRC or HMAC.

The build11 key-assisted local phase remains in the report only as an ex-post oracle
comparison. The smooth fitter used by the real projective path now accepts controls
only from `blind-self-registration`; oracle controls cannot influence the blind fit.
The affine/regularized-quadratic gates, two-resample ceiling and four complete/HMAC
decode slots remain unchanged.

On the six current real acquisitions the direct cross-cell correlation control is
weak, while intra-tile repetition is consistently measurable. Only
`foto stampa storta.jpg` produces blind fields that pass the diagnostic resample
gate in the final code. Its best measured blind correction changes the **same
candidate** from 5 to 4 known post-ECC header errors, with fit RMS about 1.27 blocks
and leave-one-out RMS about 2.04 blocks. That LOO value remains just above the strict
2.0 decode limit, so no smooth HMAC slot is consumed. The best frontal hard candidate
remains untouched at 2/24. No real acquisition authenticates.

`make blind-phase-test` covers key-independent relative registration, structural
repetition phase recovery, the deliberate absence of capacity repetition evidence,
blind-control isolation from the key-assisted oracle, explicit blind budgets and a
controlled blind-control affine correction that restores a valid Format-v3 HMAC.
The latter verifies the downstream blind-control fitting/correction path; it does not
claim that the real blind observer itself has already recovered a complete physical
print-acquisition warp.

## What v0.3.0-build11 adds

Build11 improves the quality of the build10 spatial phase controls before changing
model order. Format v3, the deterministic encoder, production `ExtractWithInfo`,
photometric/reliability logic and the maximum four complete/HMAC decode slots remain
unchanged.

Each bounded 3x3 spatial cell still searches only the existing +/-2-block phase
neighbourhood, but build11 now evaluates a clipped signed-sync correlation surface
around the integer maximum and uses parabolic interpolation to estimate a fractional
block offset. Every control gets an explicit confidence derived from local correlation
strength, peak prominence and curvature. A maximum that lands on the +/-2 search
boundary is flagged and automatically down-weighted because the true peak may lie
outside the bounded window.

The smooth fitter is now confidence-weighted and Huber-robust. It always fits the
six-parameter affine field first. With at least eight controls it also fits a
ridge-regularized quadratic field (six basis terms per axis), but the quadratic can
win only when leave-one-out RMS improves by both an absolute and relative margin.
The decode gate remains as strict as build10 (fit RMS <= 1.25 blocks and leave-one-out
RMS <= 2.0 blocks), now with an additional mean-control-confidence requirement and
the existing bounded-correction check. At most two corrected-grid resamples are still
allowed and no fifth HMAC slot can be created.

The six real acquisitions provide a useful negative result. The build10 promoted
`foto bici dritta` field no longer improves once boundary-truncated controls are
down-weighted: its eligible field changes 6->8 known post-ECC header errors and is
therefore rejected before HMAC. No real acquisition consumes a smooth-field HMAC slot
in build11. The strongest remaining diagnostic improvement is `foto bici storta`,
4->3 on one same-candidate field, but leave-one-out RMS remains about 2.18 blocks so
it is not decode-eligible. One scanner candidate selects the regularized quadratic
model by cross-validation, but its predicted correction exceeds the bounded correction
cap and is rejected. No real acquisition authenticates.

`make phase-surface-test` covers fractional peak interpolation, search-boundary
confidence reduction, confidence weighting, robust outlier rejection and the
quadratic cross-validation rule. `make smooth-phase-test` continues to verify exact
synthetic affine recovery through ordinary Format-v3 HMAC and the no-extra-slot budget.

## What v0.3.0-build10 adds

Build10 tests the build9 hypothesis that residual print-acquisition errors contain a
smooth spatial phase component. Format v3, the deterministic encoder, production
`ExtractWithInfo`, the photometric bank and the maximum four complete/HMAC decode
slots remain unchanged.

For each already-retained full-decode candidate, the bounded 3x3 spatial diagnostic
can fit a six-parameter affine field in canonical block coordinates:

```text
dx(nx,ny) = a0 + a1*nx + a2*ny
dy(nx,ny) = b0 + b1*nx + b2*ny
```

The observations are the existing +/-2-block local-phase measurements; no hidden
payload bit is used. A field needs at least six populated cells. Diagnostic resampling
requires fit RMS <= 2 blocks and leave-one-out RMS <= 3 blocks. Promotion into an
existing HMAC slot is stricter (RMS <= 1.25 and leave-one-out RMS <= 2), and the
corrected known prefix must improve without increasing known coded-bit errors. At
most two smooth-field resamples are allowed, in the existing photometric candidate
order, and a corrected grid replaces rather than adds a full decode slot.

A deterministic synthetic regression injects a known affine phase drift and verifies
that the fitted inverse field restores an authenticated v3 payload. A deliberately
non-smooth checkerboard field is measurable but rejected by the strict decode gate.

On the six current real acquisitions, the best measured same-candidate smooth changes
are mixed: `foto bici dritta` 7->3 post-ECC known-prefix errors, `foto stampa` 7->5,
`foto stampa storta` 8->6 and scanner `0270_002` 8->6, while scanner `0270_001`
changes 4->9 and `foto bici storta` 3->8. Only one real candidate passes the strict
decode gate: the mild-highpass fundamental candidate on `foto bici dritta`, which
changes 6->5 and consumes one of the existing four HMAC slots. It still does not
authenticate. Thus a smooth component is measurable, but a single global affine
field is not an adequate general correction model.

Build10 also makes spatial-cell collection ignore incomplete edge tiles for the 3x3
diagnostic only. Those edge blocks still contribute to the ordinary aggregate decode
grid, so this change does not alter Format v3 or the production decoder.

`make smooth-phase-test` covers affine-field fitting, exact synthetic recovery,
non-smooth rejection and budget invariants.

## What v0.3.0-build9 adds

Build9 keeps every build8 decoder budget and the production Format-v3 path
unchanged. It instruments spatial repetition already present in the virtual grid
so the same protected carrier positions can be compared across coarse 3x3 image
regions without re-reading a single DCT block. The spatial buckets are collected
during the existing full-grid sampling pass and are research evidence only.

For the best bit-channel candidate, JSON now reports a key-independent sign
agreement over all 1120 tile positions, plus known-prefix stability using the six
fixed v3 header Hamming words. A known coded bit is classified as stable-correct,
stable-wrong, or mixed across spatial cells. A simple per-cell majority is measured
only as an oracle diagnostic; it does not receive an HMAC slot and is not promoted
to extraction.

Build9 also measures a tightly bounded local phase check. Each populated spatial
cell may test only a +/-2-block neighbourhood around the already-selected global
phase (at most 25 phase positions per cell, 225 positions across 9 cells). This
uses the known v3 prefix and is therefore explicitly key-assisted/multiple-tested
research evidence. It is never used to authenticate or to change a decode grid.

Across the six current real acquisitions, mean key-independent tile-position sign
agreement is only about 0.635--0.683 even though scanner lattice consistency is
about 0.87. Most known header bits are mixed across regions and stable-wrong bits
are nearly absent. This strongly favors spatially varying misregistration/channel
damage over a small fixed set of systematically inverted bits. Small local phase
changes help some cases but hurt others, so independent per-cell re-phasing is not
promoted as a decoder.

The compact soft-Hamming report is also disambiguated: the soft result on the
best-hard candidate and the hard result on the best-soft candidate are now exposed
separately, avoiding comparisons between different geometry/view candidates.

`make spatial-channel-test` covers the spatial classification, key-independent tile
agreement, bounded local-phase neighbourhood and no-extra-read accounting.

## What v0.3.0-build8 adds

Build8 keeps Format v3, the deterministic encoder and production `ExtractWithInfo`
unchanged. It addresses two research findings from build7: scanner inputs below the
production 50 MP search limit could spend more than two minutes in the mature
baseline extractor before projective diagnostics started, and the original frontal
smartphone capture suggested that low-margin errors might benefit from a soft
Hamming experiment.

`diagnose` now has its own 16 Mi-pixel baseline-authentication budget. Larger
acquisition images skip the production baseline only inside diagnostics and proceed
directly to the already-bounded projective path. Complete virtual grids are also
area-bounded: candidates requiring more than 50,000 canonical blocks are aggregated
from at most 25 spatially distributed complete v3 tiles. The JSON report exposes
which baseline/full-grid budgets fired plus coarse/refinement/photometric/full-decode
stage timings. The production extractor itself is untouched.

A diagnostic reliability decoder performs one deterministic maximum-likelihood
choice among the 16 valid Hamming(7,4) codewords for each seven-bit word using the
signed DCT margins. It never creates a list of payload candidates. A full-decode
slot uses this path only when the fixed known v3 prefix shows at most one multi-error
word and wrong known bits are materially weaker than correct ones; the total remains
maximum four complete/HMAC decode slots. Synthetic regression proves that two weak
errors in one Hamming word can be recovered and authenticated by this path while the
ordinary hard decoder fails.

On the real corpus, the simple soft-Hamming model does not authenticate any capture.
The original frontal photograph activates one reliability decode, but its best known
header remains two bits wrong after both hard and soft decoding. Scanner acquisition
adds an important control: the two supplied 600-dpi 4960x7015 JPEG scans have very
strong lattice consistency (about 0.87) yet still show substantial protected-bit
errors. After the diagnostic baseline budget fix, both complete in about 10--11 s
instead of exceeding 120 s. This shows that strong lattice geometry alone does not
guarantee a clean v3 protected-bit channel.

`make reliability-test` covers soft-Hamming and bounded full-grid behavior. An
optional `make print-scan-test` uses the same HMAC-only corpus harness with
`PRINT_SCAN_DIR` (default `print-scan private`) so scanner and smartphone corpora can
remain separate.

## What v0.3.0-build7 adds

Build7 keeps the build6 lattice-first geometry and build5 photometric bank
unchanged, but instruments the exact protected-bit channel seen by the same
four candidates that already reach a complete virtual Format-v3 decode. No new
geometry candidates and no additional HMAC attempts are introduced.

For each full-decode candidate, the diagnostic selects the strongest bounded v3
profile/phase and reconstructs the protected coded bits together with their
signed DCT margins. The first three Format-v3 frame bytes (magic plus
version/profile) are known before payload recovery, so their 24 raw bits and six
Hamming(7,4) codewords can be compared exactly without knowing the hidden
message. JSON reports known protected-bit errors, how many of those six known
codewords contain 0, 1 or more than 1 error, post-ECC known-header bit errors,
global Hamming syndrome occupancy, coded-margin quantiles, and the mean margin of
correct versus wrong known bits. These are research diagnostics only; HMAC is
still the sole authentication criterion.

On the four current private smartphone captures, the best bounded candidate has
1/6, 3/6, 3/6 and 2/6 known header codewords respectively with more than one bit
error. The original frontal capture is closest to Hamming capacity: its best
candidate has 7/42 known coded-bit errors, only one known multi-error word and
2/24 known-header errors after ordinary Hamming correction. Its wrong known bits
also have substantially lower DCT margins than its correct bits, making it the
strongest future candidate for a bounded soft-decoding experiment. No real
capture authenticates in build7.

`make print-camera-test` is now corpus-driven: every PNG/JPEG directly inside the
private print-camera directory is tested in deterministic filename order. The
test no longer hardcodes the two original filenames, reports a corpus summary,
cleanly SKIPs an absent/empty corpus, and still marks each image PASS only after
a valid Format-v3 HMAC.

## What v0.3.0-build6 adds

Build6 keeps the build5 photometric bank and build4 geometry budgets but removes
the strong print-boundary detector as a mandatory gateway to projective
analysis. The boundary remains the preferred initializer when reliable; it is
never watermark evidence.

On large images with no strong boundary, a borderline lattice result can trigger
one bounded adaptive escalation to a finer diagnostic pyramid level (normally
1/4 after the default 1/8 + 1/16 pass). If that second look produces lattice
evidence, a geometrically sane weak boundary quadrilateral may be used only as a
projective seed. The report explicitly records `adaptive_escalated`, the added
divisor/reason and `lattice_first_fallback_used`.

This changes the held-out `foto bici dritta.jpg` case from a build5 early stop
(consistency≈0.533, no projective attempts) to an adaptive 1/4+1/8+1/16 result
with consistency≈0.615, lattice evidence and four bounded projective/HMAC decode
attempts. No HMAC authenticates. The companion held-out inclined capture and the
two original private captures keep their previous build5 geometry/sync results.

A large unmarked qualification image can still show diagnostic lattice evidence
after adaptive escalation; its weak-boundary confidence is too low to open the
projective fallback. This is intentional evidence that lattice/sync scores are
not watermark detection. Only the existing Format-v3 HMAC can authenticate.

## What v0.3.0-build5 adds

Build5 keeps the build4 geometry fixed and adds the first small deterministic
print-camera photometric bank. Format v3, the deterministic encoder and the
production `ExtractWithInfo` search order remain unchanged.

The diagnostic path now evaluates at most three already-selected projective
geometries through exactly three photometric views: `raw`, `local-normalize` and
`mild-highpass`. This adds at most nine known-header probes and still permits at
most four complete Format-v3/HMAC decodes. Photometric scoring never creates new
geometry candidates and never counts as watermark detection.

On the private frontal capture, `mild-highpass` improves the strongest retained
fundamental/phase-DLT known-header probe from z≈4.454 / fraction≈0.768 to
z≈4.695 / fraction≈0.783. On the inclined capture no photometric mode beats the
build4 raw global maximum z≈3.973. Neither photograph authenticates, so the
hidden real payload remains unknown.

Build5 also separates release invariants from qualification-corpus observations
in `make all-test`. Additional images in `original pics/` remain useful and are
reported, but a content-specific robustness miss no longer rewrites the meaning
of the release baseline. Very large/undimensionable inputs are consistently
SKIPped by bounded geometry suites instead of being reported as decoder errors.

## Build

```sh
go build -o pixseal ./cmd/pixseal
```

or:

```sh
make
```

`make` builds only.

The project has no external runtime dependencies. ImageMagick and GNU
`timeout` are required only by the shell robustness suites.

## Basic usage

```sh
pixseal embed \
  -in photo.png \
  -out sealed.png \
  -key "a long secret" \
  -message "hidden message" \
  -profile auto
```

A successful embed reports the selected profile:

```text
embedded 14 bytes using profile robust in sealed.png
```

Extraction does not require a profile:

```sh
pixseal extract -in sealed.png -key "a long secret"
```

The authenticated payload is written to stdout. Human-readable diagnostics such
as confidence, recovered profile and geometry correction are written to stderr.
For scripts or multiline payloads, use:

```sh
pixseal extract -in sealed.png -key "a long secret" -raw
```

`-raw` writes **only** the authenticated payload bytes to stdout, without an
added newline; decoder diagnostics remain available separately on stderr.

### Strength

The public CLI accepts finite strength values from **4 through 120**. The
default is 24 when `-strength` is omitted. Explicit `0`, `NaN` and infinities
are rejected. The Go API retains `Options.Strength == 0` as the internal
"use default" sentinel for compatibility.

### Profiles

| Profile | Maximum payload | Protected frame | Hamming-coded bits | Tile redundancy |
|---|---:|---:|---:|---:|
| `robust` | 16 B | 32 B | 448 | 2.50x |
| `balanced` | 32 B | 48 B | 672 | 1.67x |
| `capacity` | 64 B | 80 B | 1120 | 1.00x |

`auto` chooses the most robust profile capable of containing the actual payload.
Payload limits are measured in bytes, not Unicode characters.

## Format v3 frame

Every profile has a fixed frame size, but the authentication tag follows the
**actual payload**, not the end of the profile capacity region:

```text
magic(2)
+ version/profile(1)
+ payload length(1)
+ CRC32(4)
+ payload(actual length)
+ HMAC-SHA256 tag(8, truncated)
+ zero padding to the profile frame size
```

The tag offset is therefore:

```text
8 + actual_payload_length
```

The HMAC covers the 8-byte header and actual payload. The completed frame is
whitened, Hamming(7,4)-protected and mapped onto the fixed 35x32 = 1120-position
DCT tile with a deterministic modular stride.

Whitening is **not encryption**. Encrypt sensitive content before embedding if
confidentiality is required.

See [`docs/ALGORITHM.md`](docs/ALGORITHM.md) for the precise specification.

## Geometry diagnostics (v0.3 research)

```sh
pixseal diagnose -in capture.jpg
pixseal diagnose -in capture.jpg -json
pixseal diagnose -in capture.jpg -key "a long secret" -json
```

The diagnostic command reports local/global lattice evidence, the optional print
boundary initializer, bounded projective candidates, budgets and timings. When a
key is supplied, small ordinary carriers still use the mature baseline extractor;
the research path may additionally rank bounded projective hypotheses with the
key-known Format-v3 header and run at most four complete virtual projective
decodes. Those probe scores are diagnostic only. A valid Format v3 HMAC remains
the only success criterion.

## Analyze and capacity

```sh
pixseal analyze -in photo.png -message "hidden message"
pixseal analyze -in photo.png -bytes 18
pixseal capacity -in photo.png
pixseal capacity -in photo.png -details
```

`analyze` clearly labels deterministic facts and heuristic recommendations. Its
image-detail estimator uses the same white-background alpha flattening as the
encoder. Recommendations are advisory and are not recovery guarantees.

## Image pipeline and safety

Supported decoded input formats are PNG and JPEG; detection is based on file
content rather than filename extension. Output is always PNG.

If the requested output has no extension, `.png` is appended. If it has another
extension, it is replaced with `.png`. Unix dotfiles are handled as basenames:
`.sealed` becomes `.sealed.png`, not `.png`.

Output safety rules:

- without `-force`, any existing directory entry is protected;
- `-force` can replace **regular files only**;
- directories, symlinks and other special files are rejected;
- no-clobber publication uses an atomic hard-link commit when supported, with an
  exclusive-create fallback where hard links are unavailable;
- newly created files respect the process umask;
- replacing an existing regular file preserves its permission bits;
- the temporary file is `Sync()`ed before commit.

On Windows the fallback replacement path uses backup-and-restore because
`os.Rename` cannot always replace an existing file. This reduces replacement
risk but is not claimed to provide filesystem-level crash durability; directory
`fsync` is not attempted.

### Image-size policy

Before decoding pixel data, the CLI uses `image.DecodeConfig` and rejects inputs
above **300,000,000 decoded pixels**. The core applies the same working-image
limit before allocating its compact extraction pixel plane. This keeps the
verified ~200 MP class usable while avoiding obviously unbounded allocations.

Some **generated geometry candidates** use the stricter 50,000,000-pixel search
bound. Such candidates are skipped rather than materialized.

The v0.3 diagnostic path now builds sampled luminance analysis planes rather than
an additional full RGB extraction plane. The Go image decoder still materializes
the decoded source image; build4 avoids additional full-size rectified/projective
working copies but is not yet a streaming/tiled source decoder.

## Decoder order

The current v3 extractor is bounded and ordered so speculative geometry does not
suppress established recovery paths:

1. direct grids at apparent block sizes 8, 6 and 4 pixels, including pixel phase;
2. gated lossless quarter turns (90/180/270 degrees);
3. pure isotropic fractional-scale recovery through virtual affine sampling;
4. one targeted physical bilinear normalization fallback when the scale evidence
   is decisive;
5. early axis-aligned affine recovery when zero-degree evidence supports it;
6. two mild projective/vertical-keystone hypotheses;
7. fixed direct lattice-basis composition bank;
8. arbitrary-angle rotation estimation and rectification;
9. final axis-aligned affine fallback when rotation evidence is weak.

HMAC authentication is always the final success criterion.

### Pure fractional resize budget

The non-integer direct-scale path has:

- 13 fixed scale hypotheses: 95, 90, 85, 80, 70, 65, 60, 55, 45, 40, 35, 30, 25%;
- up to 3 candidate phases per scale for single-tile probing;
- at most 6 shortlisted scales x 3 phases = **18 full-carrier virtual grids**;
- at most one decisive physical normalization candidate, with the base target
  size plus its eight ±1-pixel neighbours.

100%, 75% and 50% are already handled directly by apparent block sizes 8, 6 and
4 and are not duplicated in that list.

### Experimental geometry budgets

Arbitrary rotation:

- 2 zero-degree probes (8/6 px);
- at most 720 non-zero coarse probes;
- at most 33 fine probes;
- at most 2 refined angle candidates reach rectification.

Fixed direct lattice bank:

- 4 basis shapes: 110x90, 90x110, 105x95, 95x105%;
- 361 angles per shape (-45..+45 at 0.25 degree);
- **1444 sparse probes** maximum;
- at most **48 candidates per shape / 192 total** receive stronger coherence
  evaluation;
- at most 4 matrices x 3 phases per shape group = **12 full authenticated grids per group**,
  with two sequential groups (±10% then ±5%) for a **24-grid overall worst case**.

Mild perspective:

- exactly 2 fixed vertical-keystone hypotheses (`top-narrow-4`,
  `bottom-narrow-4`);
- one aligned phase each;
- at most **2 projective full-grid aggregations**.

These budgets describe the implemented search space; they are not statements of
universal recovery probability.

## Metadata and color/orientation limitations

PixSeal works on decoded pixels and outputs 8-bit NRGBA flattened against white.
Consequently:

- 16-bit PNG input becomes 8-bit output;
- EXIF, XMP, comments and ICC/profile metadata are not preserved;
- Go's JPEG decoder does not automatically apply EXIF Orientation, so a JPEG
  shown rotated by a metadata-aware viewer may be processed according to its
  stored pixel orientation;
- losing an ICC profile can change appearance in color-managed workflows, not
  merely remove descriptive metadata.

EXIF-orientation normalization and color-management preservation are deferred to
v0.3 because they require an explicit image-pipeline policy.

## Tests

```sh
make                 # build only
make test            # complete Go tests + local image round trips
make release-unit    # release-gate Go regressions only
make research-unit   # experimental geometry Go regressions
make lattice-estimator-test # v0.3 bounded local-lattice diagnostics
make homography-test   # v0.3 boundary/homography/virtual decoder regressions
make photometric-test  # v0.3 bounded raw/normalize/high-pass diagnostics
make bit-channel-test  # v0.3 protected-bit/Hamming channel diagnostics
make reliability-test  # v0.3 bounded soft-Hamming/full-grid diagnostics
make spatial-channel-test # v0.3 spatial/tile protected-bit diagnostics
make print-camera-test # private real print-camera corpus; SKIP when absent
make print-scan-test   # optional separate private scanner corpus
make private-corpus-manifest # local SHA-256 manifest for both private physical corpora
make deep-test       # stable JPEG/resize/crop baseline
make extreme-test    # non-strict progressive resize/crop limit map
make geometry-test   # experimental rotation/combined geometry
make affine-test     # experimental axis-aligned affine matrix
make composition-test # historical composed regression
make lattice-test     # direct lattice-basis regression
make perspective-test # two-hypothesis mild projective regression
make core-target-check # reusable core: Linux/Windows/Android/iOS
make version-check    # VERSION vs buildinfo consistency
make release-check    # release baseline gate
make all-test         # every suite sequentially + summary
```

`extreme-test` is intentionally **non-strict**: individual FAIL rows identify
measured limits and do not make the target fail. `deep-test`, `geometry-test`,
`affine-test`, `composition-test`, `lattice-test` and `perspective-test` support
strict mode, and `make all-test` invokes those suites with strict mode by default.

`make release-check` is self-contained and forces strict semantics for the stable
`deep-test` baseline. It includes `release-unit`, private-corpus round trips and
core portability while excluding `research-unit`. `make all-test` keeps research
failures visible separately from the release baseline and reports partial target
sets as `PARTIAL`/`NOT RUN` rather than as a full qualification PASS.

The local shell suites use `original pics/`, which is private and excluded from
source archives. An empty qualification corpus is an error rather than a false
PASS. `print-camera-test` separately scans every PNG/JPEG directly inside the
private print-camera directory and cleanly reports `SKIP` when that corpus is
absent or empty; when present, `PRINT_CAMERA_KEY` must be supplied locally and each
image can report `PASS` only if the authenticated Format v3 payload is recovered.
The same key policy applies to `print-scan-test`. No physical-corpus key is stored in
the source archive.

The full shell qualification harness is intended for **Linux/WSL with Bash >= 4,
GNU-compatible userland (including `timeout` and `sort -z`) and ImageMagick**.
This harness requirement is separate from reusable-core portability.

## Current limitations

- Maximum v3 payload: 64 bytes.
- Minimum aligned embed geometry: 280x256 pixels.
- CLI embedding currently accepts text via `-message`; the core stores bytes.
- General affine/projective print-camera recovery is not a v0.2.0 guarantee.
- v0.3.0-build16 retains the bounded lattice/projective/photometric/reliability
  research path, build12 repetition controls, build14 fractional lattice phase and
  build15 transactional global unwrap, but certifies the bounded discrete top-2 by
  exact enumeration. No real smartphone or scanner acquisition has authenticated;
  lattice/sync/ECC/spatial/blind/unwrap/smooth-field evidence remains diagnostic only.
- Experimental rotation/affine performance is image-content dependent.
- Failed extraction can be substantially more expensive than successful
  extraction because bounded candidates must be exhausted.
- No neural model is used.
- No professional cryptographic or steganalytic audit has been performed.

## Release status

**v0.3.0-build16** is an experimental development snapshot. It preserves the
qualified v0.2.0 encoder/Format-v3/production-extractor baseline and changes only the
separate diagnostic research path. Build16 replaces the heuristic build15 beam top-2
with exact bounded enumeration and adds diagnostic split-repetition top-2 consistency.

**v0.2.0** remains the qualified stable release of the Format v3 line. It was
promoted from RC4 after the stable release baseline passed on the private
qualification corpus with 72/72 baseline transformations recovered. The
experimental geometry measurements remain explicitly non-normative and are
recorded separately from the release gate.

Qualification results are recorded in [`docs/RESULTS.md`](docs/RESULTS.md).

## License

MIT. See [`LICENSE`](LICENSE).

