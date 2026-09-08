# PixSeal History

This file records the technical evolution of PixSeal across public releases and
intermediate development builds. It complements `CHANGELOG.md`: the changelog
is release-oriented, while this history preserves the main architectural and
experimental steps that led to the current design.

## v0.1.0 — First public release

PixSeal was first published as a pure-Go robust image steganography CLI for
hiding short authenticated messages in images.

The first stable release established the basic DCT carrier model, keyed
whitening, CRC32, truncated HMAC-SHA256 authentication, Hamming(7,4) error
correction, periodic two-dimensional embedding, and bounded recovery after
JPEG recompression, resizing and off-grid cropping.

The project terminology was also clarified during release hardening: PixSeal is
primarily a robust steganography project for short messages. Digital
watermarking is the embedding mechanism, not the end goal. The original
inspiration from systems such as Google SynthID is historical and conceptual;
PixSeal is an independent implementation and does not reproduce SynthID.

The v0.1 line used experimental format generations v1/v2 internally. They were
never deployed to an external carrier population and are no longer supported at
runtime, but remain part of the engineering history.

## v0.2.0 build 1 — Adaptive format v3

The v0.2 development line introduced the adaptive **format v3** while keeping
the maximum payload at 64 bytes.

Three concrete profiles were added:

- `robust`: up to 16 bytes, 2.50x mean per-tile coded-bit observations;
- `balanced`: up to 32 bytes, 1.67x mean observations;
- `capacity`: up to 64 bytes, 1.00x mean observations;
- `auto`: selects the most robust compatible profile automatically.

Format v3 retained CRC32, truncated HMAC-SHA256, key-derived whitening and
Hamming(7,4), but reused unused tile capacity as additional spatially dispersed
redundancy for shorter frames. The CLI gained `analyze`, detailed capacity
reporting and explicit adaptive-profile support.

## v0.2.0 build 2 — v3-only runtime

Because v1 and v2 had never been distributed in practice, runtime compatibility
was removed before the v0.2 format was frozen. Format numbering was deliberately
kept at v3 to preserve the real engineering history.

The decoder became v3-only, obsolete legacy flags and code paths were removed,
and failed-extraction behavior became simpler and easier to bound. Repository
hygiene was also improved, including ignore rules for Windows/WSL
`Zone.Identifier` artifacts.

This build also documented the next geometry roadmap: digital rotation,
combined transforms, affine recovery, perspective correction and eventually a
print-camera channel.

## v0.2.0 build 3 — Digital rotation recovery

Build 3 added the first real geometric recovery layer without changing format
v3.

Exact 90/180/270-degree rotations gained a direct path, while arbitrary-angle
rotation used a bounded DCT-lattice orientation estimator. Rotation is estimated
coarsely and refined only around a very small number of candidates before
authenticated v3 decoding.

The new `geometry-test` target made rotation limits reproducible instead of
anecdotal.

## v0.2.0 build 4 — Rotation combined with resize and crop

The geometric detector was generalized to operate on the DCT lattice at the
main baseline scales. This enabled controlled combined transforms including:

- rotation followed by 75% resize;
- 75% resize followed by rotation;
- rotation followed by crop;
- crop followed by rotation;
- rotation + 75% resize + crop.

The 50% resize combined with arbitrary rotation was intentionally not claimed as
supported after tests showed that the limiting factor was signal degradation
from repeated interpolation rather than angle detection alone.

## v0.2.0 build 5 — Bounded affine recovery

Build 5 introduced the first bounded affine recovery stage, again without
changing format v3.

The decoder gained 36 explicit axis-aligned affine hypotheses:

- 20 anisotropic X/Y scale combinations derived from 90/95/100/105/110%,
  excluding uniform pairs already handled by normal resize recovery;
- 16 X/Y shear hypotheses at +/-3, +/-5, +/-8 and +/-10 degrees.

Instead of rendering a corrected image for every hypothesis, PixSeal evaluates
the DCT lattice through a virtual inverse-affine sampler. A key-independent
periodic tile-coherence score ranks hypotheses, while HMAC-authenticated v3
recovery remains the only final success condition.

A dedicated `affine-test` target was added. Development tests using ImageMagick
recovered all three profiles under 110%x90%, 90%x110%, shear X 8 degrees and
shear Y 8 degrees.

## v0.2.0 build 6 — First bounded composed affine geometry

Build 6 kept format v3 unchanged and introduced the first controlled composition
of previously separate geometry stages. The validated baseline is anisotropic
110%x90% scaling followed by arbitrary digital rotation on the `robust` profile.

Rather than evaluating the full angle-by-affine Cartesian product, PixSeal uses
at most two rotation peaks, refines each within a fixed +/-2 degree window at
0.25-degree spacing, and combines them with four discrete anisotropic scale
pairs. This caps the new stage at 136 composed matrix probes and at most 18
full-carrier phase aggregates. HMAC authentication remains the final success
criterion.

A dedicated `composition-test` target was added. The intentionally narrow first
baseline makes the next research questions explicit: broader angle stability,
both anisotropic orientations, balanced/capacity profiles, the reverse edit
order, and rotation combined with shear.

This build also introduced `HISTORY.md`, `TODO.md` and the repository banner as
first-class project documentation. The project also recorded a future frontend
direction: keep the pure-Go codec independent from CLI/UI concerns so it can later
back Windows/Linux graphical applications and, in particular, an Android-capable
frontend. No GUI dependency was introduced in this build.

## v0.2.0 build 7 — Direct lattice estimation from real-corpus failures

Build 7 was driven by the first important negative result from PJ's private
photograph corpus. The build-6 composed path passed a generated carrier but, under
the same `110%x90% -> 12.3-degree rotation` transform, one real image timed out and
another failed outright.

The failure showed that the composition could not reliably be decomposed as
"estimate rotation first, then try anisotropic scale": the scale transform itself
can move or distort the orientation peak seen by the standalone rotation detector.

The decoder was therefore changed to score the **repeated v3 DCT lattice directly**
under the composed matrix. The promoted build-7 baseline deliberately remains
`robust` and 110%x90% scaling followed by arbitrary rotation. A sparse 361-angle
stage ranks candidate matrices using key-independent tile repetition, at most 16
candidates receive stronger coherence analysis, and at most four matrices with
three phases each can reach full authenticated aggregation.

The two private photographs that exposed build 6 were rerun end-to-end through
ImageMagick and `make composition-test`; both recovered successfully. The private
source images remain external regression material and are not distributed with
PixSeal.

This build marks a conceptual shift in the geometric roadmap: future work should
estimate transformed lattice basis vectors directly rather than continually adding
separate rotation/scale/shear heuristics. That representation is also the natural
bridge toward projective geometry and the eventual print-camera channel.

## v0.2.0 build 8 — First symmetric lattice-basis bank

Build 8 kept format v3 unchanged and generalized the build-7 direct composed
search from one hard-coded anisotropic geometry into the first explicit bank of
transformed lattice bases.

Each candidate is represented by the observed horizontal and vertical basis
vectors (`u`, `v`) of the PixSeal DCT lattice. The first promoted bank contains
two symmetric anisotropic shapes, 110%x90% and 90%x110%, each searched over the
same bounded quarter-degree orientation range.

Real-corpus testing again shaped the implementation. A sparse 20-position score
was sufficient for the original positive-angle baseline but could under-rank the
correct geometry at negative angles. Build 8 therefore changed the ranking to
retain at most 48 quick candidates per basis shape before stronger repetition
analysis. The full authenticated budget remains only four matrices with three
phases each.

The two private photographs that exposed build 6 were rerun through ImageMagick
at 12.3 degrees under both anisotropic orientations. `STRICT=1 make lattice-test`
completed with 4/4 recovered messages and no timeout. Additional development
spot checks recovered positive and negative rotations on the promoted basis
shapes.

This build deliberately stops short of calling the bank a general affine
estimator. The next architectural step remains continuous or locally refined
inference of `u` and `v`, including non-orthogonal bases for shear, rather than
adding an ever-growing list of edit-specific cases.

## v0.2.0 build 10 - regression recovery

The build-9 `all-test` report exposed a clear regression pattern on a small real carrier: direct 8/6/4-pixel grids at 100/75/50% survived while fractional resize levels were often cut off by false rotation/lattice candidates. Build 10 deliberately stopped adding geometry features and restored baseline ordering.

Pure isotropic resize is now a first-class bounded recovery stage. It samples known scale hypotheses virtually and permits at most one targeted physical normalization when sync evidence is strong. Advanced unauthenticated geometry heuristics are no longer allowed to suppress this established path. A deterministic encoder fingerprint test was also added to freeze Format-v3 embedding while decoder research continues.

The same report revealed false errors on the private ~201 MP HQ PNG because several scripts asked ImageMagick to decode enough of the image merely to obtain dimensions. Build 10 added PNG-IHDR probing so those suites now skip the image by policy instead of reporting an error.

## v0.2.0 build 9 — Broader lattice bank and unified test reports

Build 9 kept format v3 unchanged and expanded the build-8 transformed-lattice
bank from two to four anisotropic basis shapes. In addition to the 110%x90% and
90%x110% anchors, the decoder now promotes 105%x95% and 95%x105% before
arbitrary rotation. These moderate anisotropies are important because they show
that the lattice representation generalizes beyond one deformation magnitude.

Real-image experimentation again influenced the search budget. The moderate
shapes showed stronger sensitivity to pixel phase than the +/-10% anchors;
shrinking their quick shortlist caused a private regression carrier to be missed.
Build 9 therefore keeps a larger but still finite shortlist for those shapes
while preserving a maximum of four matrices and three phases for full
authenticated aggregation.

The final end-to-end lattice regression on the two private photographs exercised
all four promoted basis shapes and completed 8/8 authenticated recoveries with no
failures or timeouts. The photographs remain external test material and are not
distributed.

A prototype attempted more continuous/local refinement of the observed lattice
vectors `u` and `v`. It recovered positive composed cases but pushed negative
extraction toward unacceptable runtimes. The prototype was not promoted. This is
recorded intentionally: future continuous estimation needs **more information per
probe**, not merely a denser search.

Build 9 also introduced `make all-test`, a sequential orchestrator that runs all
distinct test/check suites, continues after individual failures and emits one
summary containing status and elapsed time. Optional report capture makes the
output suitable for comparing geometric robustness and runtime across builds.

Final closeout also exposed a small-carrier edge case in the new quarter-turn
periodicity gate: carriers with one decodable tile but insufficient area for two
full periods were being treated as low-coherence. The gate now distinguishes
unmeasurable repetition from negative evidence, preserving bounded 90/180/270
degree recovery on small carriers.

## Current direction

The next controlled step is to move beyond the expanded discrete build-9 basis bank and
infer or locally refine the transformed horizontal/vertical lattice vectors
directly, without paying for a dense negative-case search. That should cover broader affine cases, including shear, without
multiplying independent angle/scale/shear searches. After that, the project will move toward
bounded projective/perspective recovery.

The long-term experimental target is a physical-channel test:

```text
embed message
    -> print image on paper
    -> photograph it with a smartphone
    -> recover and authenticate the PixSeal payload
```

Achieving this requires robust geometric synchronization as well as tolerance to
printing, optics, illumination, sensor processing and recompression. It remains
a research goal rather than a current capability.

## v0.2.0 build 11 — First projective step

Build 11 preserved the frozen Format-v3 encoder and introduced the first projective decoder experiment. Instead of brute-forcing homography parameters, PixSeal gained a virtual projective sampler with two fixed 4% vertical-keystone hypotheses. This established that the existing v3 DCT signal can survive a mild synthetic projective warp on both the private LQ and MQ carriers without changing embedding. The result is intentionally recorded as a bounded research baseline, not as general perspective support.
## v0.2.0-rc1 — Release consolidation

The release candidate returns to the build-11 code line after the build-12 local-consensus experiment failed to improve the real corpus and increased runtime. No new recovery algorithm is added in the RC. Format v3 and its deterministic encoder fingerprints are frozen while release-facing behavior, documentation and test semantics are consolidated.

The RC explicitly separates the stable release baseline from research suites. JPEG, pure resize, crop, authenticated round trips and reusable-core portability form the release gate. Arbitrary rotation, affine/lattice composition and mild projective recovery remain measured experimental capabilities. General homography estimation and the physical print-camera channel move to the next research line.

Build 12 remains part of engineering history as a non-promoted experiment: the reconstructed local-consensus fallback reproduced build-11 functional results on the PJ corpus while increasing total test runtime, so it was not used as the RC base.

