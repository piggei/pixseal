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

## Current direction

The next controlled step is to generalize direct lattice estimation beyond the
single 110%x90% baseline and infer the transformed horizontal/vertical lattice
basis more directly. That should cover broader affine cases without multiplying
independent angle/scale/shear searches. After that, the project will move toward
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
