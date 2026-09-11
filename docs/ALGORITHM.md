# PixSeal — Format v3 stable specification and v0.3 research diagnostics

Sections 1-19 describe the stable Format v3 and production decoder shipped in
**v0.2.0**; those rules remain unchanged in **v0.3.0-build7**. Historical
strategies from intermediate builds belong in `HISTORY.md`. The final appendix
documents the separate v0.3 research diagnostic and is not part of the on-image
format or authenticated extraction contract.

PixSeal is an experimental robust-steganography system for short authenticated
messages. DCT watermarking is the carrier mechanism; it is not an ownership
protocol and is not claimed to be statistically undetectable.

## 1. Constants

```text
DCT source block                 8 x 8 pixels
logical tile                    35 x 32 blocks
logical tile positions          1120
minimum aligned embed geometry  280 x 256 pixels
maximum payload                 64 bytes
header                          8 bytes
authentication tag              8 bytes (HMAC-SHA256 truncated)
ECC                             Hamming(7,4)
```

The two luminance DCT coefficients used for each embedded bit are `(u=3,v=2)`
and `(u=2,v=3)` in the implementation's coefficient indexing. Embedding forces a
minimum absolute-magnitude separation according to the desired bit and selected
strength.

## 2. Adaptive profiles

| Profile | ID | Max payload | Frame bytes | Protected bits | Tile redundancy |
|---|---:|---:|---:|---:|---:|
| robust | 1 | 16 | 32 | 448 | 2.50x |
| balanced | 2 | 32 | 48 | 672 | 1.67x |
| capacity | 3 | 64 | 80 | 1120 | 1.00x |

`auto` is not encoded as a profile. The encoder resolves it to the smallest
capacity class that can hold the actual payload.

## 3. Frame layout

The v3 frame has a fixed size per profile but the tag immediately follows the
**actual payload**:

```text
offset  size  field
0       2     magic "PS"
2       1     version/profile byte (0x30 | profile ID)
3       1     actual payload length in bytes
4       4     CRC32(payload), big endian
8       N     actual payload
8+N     8     HMAC-SHA256(header || payload), truncated to 8 bytes
after   ...   zero padding to the profile frame size
```

Thus:

```text
tagOffset = headerSize + payloadLength
```

The tag authenticates the header and actual payload, including format/profile
identity and payload length. Padding is not included in the MAC input.

CRC32 is an error-detection aid. HMAC is the authentication mechanism. The
64-bit truncated tag provides at most approximately 64 bits of forgery
resistance.

## 4. Whitening and ECC

The complete fixed-size profile frame is converted to bits and XOR-whitened with
a deterministic SHA-256-derived stream labeled `pixseal-whiten-v3` and keyed by
the supplied key. Whitening is not encryption.

The whitened bits are encoded with Hamming(7,4):

```text
32-byte robust frame    -> 448 coded bits
48-byte balanced frame  -> 672 coded bits
80-byte capacity frame  -> 1120 coded bits
```

## 5. Tile mapping

The logical tile always contains 1120 DCT positions. A tile position maps to a
protected-bit index using:

```text
codeIndex = (tilePosition * 251) mod codedBits
```

251 is coprime with all three coded-bit counts. Profiles shorter than 1120 bits
therefore receive repeated observations distributed across the same fixed tile.

The tile itself repeats spatially across the carrier. Extraction aggregates
observations at equal logical tile positions before ECC/frame validation.

## 6. Embedding

1. Validate key length (minimum 8 bytes), profile and payload size.
2. Normalize embedding options; API strength 0 means the default 24.
3. Build and authenticate the v3 frame.
4. Whiten and Hamming-encode it.
5. Flatten source alpha against white and convert to 8-bit NRGBA.
6. For every complete 8x8 block, map its repeated tile position to the protected
   bit and modify the selected DCT coefficient pair.
7. Output remains a lossless in-memory NRGBA image; the CLI writes PNG.

The public CLI accepts only finite strengths from 4 through 120. `NaN`,
infinities and explicit 0 are rejected.

## 7. Image-size safety

The CLI uses `image.DecodeConfig` before decoding and rejects sources above
300,000,000 decoded pixels. The Go core applies the same working-image bound
before creating the compact extraction pixel plane or an embed copy.

The extraction plane stores 3 bytes per pixel in addition to the decoded Go
image. The explicit limit prevents unbounded extra allocation and integer-size
overflow. v0.3 may replace this full plane with sparse/tiled access.

Generated inverse-normalization and rotation candidates additionally obey the
50,000,000-pixel geometric-search bound.

## 8. Direct extraction

The decoder first tests apparent integer DCT block sizes:

```text
8 px -> native / 100%
6 px -> 75%
4 px -> 50%
```

For each size it searches all pixel phases inside one block. Every candidate is
processed through v3 sync/profile probing, Hamming decode, unwhitening, CRC and
HMAC. The profile is inferred from the authenticated v3 header; the user never
provides it during extraction.

## 9. Exact quarter turns

90/180/270-degree rotations preserve the integer pixel lattice and are corrected
with lossless pixel-plane reorientation. Key-independent repeated-tile
coherence gates the expensive quarter-turn attempts on sufficiently large
carriers; small one-tile carriers remain eligible when coherence cannot be
measured.

## 10. Pure fractional resize recovery

The fixed non-direct scale list is:

```text
95 90 85 80 70 65 60 55 45 40 35 30 25 percent
```

For each scale PixSeal uses a virtual affine sampler rather than reconstructing a
full inverse-resized bitmap. `bestAffineCoherence` retains at most three pixel
phases.

Bounded stages:

```text
13 scale hypotheses
<= 3 single-tile authenticated probes per scale
<= 6 shortlisted scale hypotheses
<= 3 full-grid phases per shortlisted scale
=> <= 18 full-carrier virtual aggregations
```

If virtual evidence is decisive but HMAC still fails, exactly one scale may use
a physical **bilinear** normalization fallback. The expected inverse size plus
its eight ±1-pixel neighbours are tried, and any normalization exceeding the
50-million-pixel geometry bound is skipped.

This pure-resize stage precedes speculative arbitrary rotation/lattice recovery.
That ordering is a deliberate regression rule: an unauthenticated advanced
geometry heuristic must not suppress an established recovery path unless there
is deterministic or authenticated evidence that the established path cannot
apply.

## 11. Axis-aligned affine recovery

The fixed affine bank contains 36 hypotheses:

- anisotropic X/Y scale pairs from `{0.90, 0.95, 1.00, 1.05, 1.10}`, excluding
  equal X/Y pairs: 20 matrices;
- X or Y shear at ±3, ±5, ±8 and ±10 degrees: 16 matrices.

The decoder measures key-independent periodic coherence, keeps at most six
matrices for authenticated single-tile probing, retains at most three phases per
matrix and promotes at most four matrix/phase pairs to full-carrier aggregation.

This is a discrete research bank, not general affine inference.

## 12. Mild projective recovery

The v0.2.0 decoder contains exactly two fixed vertical-keystone hypotheses:

```text
top-narrow-4
bottom-narrow-4
```

Each corresponds to a 4% narrowing of one horizontal edge. A homography maps
coordinates in a rectified 8-pixel DCT lattice directly into the observed image;
luminance is sampled bilinearly. No full rectified image is created.

Only phase `(0,0)` is attempted for each hypothesis:

```text
2 homographies x 1 phase = 2 full projective grid aggregations maximum
```

A deterministic Go end-to-end regression verifies that a warped v3 carrier is
recovered **through the projective path** and reports the expected
`PerspectiveCorrection` value. The shell suite additionally verifies the same
path on ImageMagick-generated JPEG/PNG corpus cases.

This is not general perspective support. Horizontal keystone, arbitrary corner
motion, perspective+rotation composition, lens distortion and camera-pose
inference remain future work.

## 13. Direct lattice-basis composition

The fixed bank contains four anisotropic basis shapes:

```text
110%x90%
90%x110%
105%x95%
95%x105%
```

Each is rotated over -45..+45 degrees at 0.25-degree spacing.

Bounded search:

```text
4 shapes x 361 angles = 1444 sparse lattice probes
<= 48 candidates per shape = <= 192 stronger coherence evaluations
<= 4 matrices x <= 3 phases = <= 12 full authenticated grids per group
2 sequential shape groups = <= 24 full authenticated grids overall
```

The ±10% anchor pair is evaluated before the ±5% pair to preserve the fast path
for the earlier validated cases. HMAC remains the sole acceptance criterion.

## 14. Arbitrary rotation

The orientation detector probes 8- and 6-pixel apparent lattices:

```text
2 zero-degree checks
<= 720 non-zero 0.25-degree coarse probes
<= 3 coarse peaks
<= 33 local 0.05-degree fine probes
<= 2 refined candidates reach rectification
```

Each refined candidate may be tested in four quarter-turn quadrants and all
pixel phases for its selected apparent block size. Rectified canvases exceeding
50,000,000 pixels are skipped.

The arbitrary-rotation path is experimental and remains image-content dependent
on the qualification corpus.

## 15. Decoder ordering

The implemented v3 ordering is:

1. direct 8/6/4 grids;
2. gated lossless quarter turns;
3. virtual isotropic fractional scales;
4. optional single-scale physical bilinear fallback;
5. early axis-aligned affine when applicable;
6. two-hypothesis mild perspective;
7. direct lattice-basis bank;
8. arbitrary rotation;
9. final affine fallback when rotation evidence is weak.

The ordering is part of the v0.2.0 performance/regression contract, not part of
the on-image Format v3 interoperability format.

## 16. CLI output and files

Normal extraction writes the payload to stdout and diagnostics to stderr.
`extract -raw` writes only the authenticated payload bytes to stdout without an
added newline; decoder diagnostics remain on stderr.

Output publication rules protect existing directory entries. `-force` applies
to regular files only. New files use mode 0666 subject to the process umask;
replacement preserves existing regular-file permissions. The temporary image is
synced before publication. No-clobber publication uses a hard-link commit when
possible and exclusive-create fallback otherwise.

## 17. Alpha, metadata, EXIF and ICC

Both embedding and `analyze` flatten transparency against white. Output is 8-bit
NRGBA PNG and container metadata is not preserved.

Go's JPEG decoder does not automatically apply EXIF Orientation, and ICC
profiles are not carried into the output. EXIF orientation normalization and
color-management preservation are deliberately deferred to v0.3.

## 18. Security properties and non-properties

PixSeal provides:

- keyed frame whitening;
- CRC32 damage detection;
- HMAC authentication with a 64-bit truncated tag;
- ECC against bounded bit errors.

PixSeal does not provide reviewed encryption, strong key management, statistical
undetectability or resistance to a determined steganalyst. Sensitive messages
should be encrypted before embedding.

## 19. Format history

Formats v1 and v2 were internal experimental formats. Runtime extraction is
v3-only because no external v1/v2 compatibility population existed when the
v0.2 line was consolidated. Historical details belong in `HISTORY.md` and
`CHANGELOG.md`.

## 20. v0.3.0-build1 local-lattice diagnostic (non-normative)

`DiagnoseGeometry` is intentionally separate from `ExtractWithInfo`. Its output
is research evidence only; no confidence or lattice score can authenticate a
watermark. Only the existing Format v3 frame/HMAC path can do that.

The build1 pipeline is:

```text
decoded source image
  -> bounded luminance diagnostic level(s)
  -> spatial regions (3x3 by default)
  -> coarse orthogonal u/v candidates
  -> distributed block/phase probing across each region
  -> quick 35x32 tile-repetition ranking
  -> bounded local refinement and full-repetition shortlist
  -> per-region candidate pools
  -> 90-degree-equivalence alignment
  -> cross-region consensus
  -> diagnostic lattice evidence
```

For very large inputs, the diagnostic luminance plane is sampled directly from
the decoded image at a power-of-two divisor chosen to keep the analysis level
within `MaxAnalysisDimension` (2048 by default). This avoids another full RGB
search copy, but the standard Go decoder still materializes the source image.

Default build1 budgets per region are deterministic and exposed in JSON:

```text
coarse basis candidates       117
refined basis candidates      243
quick repetition candidates   117
full repetition shortlist      12 per stage
max full repetition candidates 25 per region
max phase hypotheses/basis     16
sample blocks/phase            25
regions                         9 (default; bounded to 16)
```

The estimator reports `u` and `v` in native source-pixel coordinates even when a
downsampled diagnostic level is used. Local regions do not independently decide
the global geometry; a consensus stage aligns the square lattice's quarter-turn
equivalent bases before measuring consistency.

Dev1 is not yet a general projective decoder. Arbitrary-angle candidate
generation is still insufficiently reliable on transformed carriers, so no
homography is fitted and no diagnostic candidate is promoted into the production
extractor. This separation is deliberate to avoid overfitting and unbounded
search growth.
## 21. v0.3.0-build2 bounded projective diagnostic (non-normative)

Build2 extends the research-only `DiagnoseGeometry` path; it does **not** modify
Format v3, embedding or the production `ExtractWithInfo` search order. The new
flow is:

```text
decoded source image
  -> bounded diagnostic pyramid
  -> coarse visible-print quadrilateral (optional initializer)
  -> local u/v candidates in multiple regions and levels
  -> normalize bases in print-boundary coordinates
  -> cross-region + cross-level native-scale support
  -> bounded canonical-size clusters (max 8)
  -> canonical-to-photo homography
  -> sparse key-known header probe for candidate ordering
  -> max 4 virtual projective full-grid decodes
  -> existing Format-v3 frame decoder
  -> HMAC
```

The print quadrilateral is never evidence of PixSeal. Likewise, a high lattice
score or key-known header correlation is not detection. Header probing is an
oracle/research ranking aid and is exposed because searching several geometries
creates a multiple-testing effect. Only a valid Format-v3 HMAC can set
`authenticated_payload=true`.

### 21.1 Homography and virtual sampling

For each retained canonical width/height candidate, build2 fits an 8-DOF
homography from the canonical rectangle corners to the four observed print
corners. The diagnostic decoder then maps requested canonical lattice samples
directly into source-image coordinates through that homography and bilinearly
interpolates luminance. It therefore constructs DCT blocks on demand instead of
materializing a rectified ~200 MP bitmap.

The projective diagnostic budgets are explicit:

```text
max canonical-size/projective candidates   8
max complete virtual Format-v3 decodes     4
```

A synthetic regression embeds a real Format-v3 frame, places the carrier inside
a larger image with a known print boundary and verifies authenticated recovery
through this virtual path. The same regression verifies that a wrong key cannot
authenticate.

### 21.2 Remaining research gap

On real print-camera photographs, local periodic texture can still create
harmonics and aliases of the fundamental DCT lattice period. Build2 uses
cross-region/cross-level support rather than a photo-specific target scale, but
sub-pixel period/phase refinement and stronger deterministic scale
disambiguation remain necessary before real HMAC recovery. No Format-v3 change
is justified by this geometry-stage failure.


## 22. v0.3.0-build3 spatial phase refinement (non-normative)

Build3 adds a bounded key-assisted geometric refinement after the build2
projective shortlist. It does not change Format v3 or production extraction.

For each retained canonical scale, four spatially separated complete tiles are
used when available. For every v3 profile, the strongest local sync phase is
measured independently. Phase coherence is the circular concentration of phase
X and phase Y across those tiles, weighted by their local known-header z-score.
The combined score is diagnostic geometry evidence only.

The maximum refinement budget is:

```text
coarse projective scales            8
phase-refinement seeds              3
scale evaluations per seed          9 (base + ±1%/±2% X and Y)
phase-correspondence homographies   3
complete virtual v3 decodes         4
```

When exactly four spatial phase observations are available, their circular mean
phase defines a common lattice origin. Local phase drift is unwrapped modulo the
35x32 tile and converted to canonical 8-pixel block offsets. Corrections larger
than 64 canonical pixels on either axis are rejected. The remaining four
canonical/source correspondences are fitted with the same bounded 4-point DLT
solver used for projective geometry.

This stage is deliberately key-assisted research: the key determines the known
v3 header pattern, but neither the hidden message nor an expected payload is
used. A phase score, z-score or fitted homography can never set
`authenticated_payload=true`; only the existing Format-v3 frame parser/HMAC can.


## 23. v0.3.0-build4 fundamental-scale and modulo-8 refinement (non-normative)

Build4 remains entirely outside the Format-v3 wire/image format and production
`ExtractWithInfo` search path. It refines only the research diagnostic.

### Fundamental-scale selection

Each clustered projective scale retains the number of independent regions and
pyramid levels that support it. A fundamental candidate must have support from at
least two regions and two levels. Candidates whose multilevel confidence is less
than 30% of the strongest multilevel candidate are discarded for this purpose;
among the remaining supported families, the lowest spatial frequency (smallest
canonical area, equivalently the largest observed native block pitch) is selected
as the fundamental hypothesis. The rule contains no dimensions learned from the
private print-camera corpus.

### Fundamental-only scale refinement

When a key is available to `diagnose`, only the already selected fundamental
family receives a small sync-assisted scale refinement. Width and height are
searched independently at:

```text
-2%, -1%, -0.5%, 0, +0.5%, +1%, +2%
```

The search is capped at 16 probes including full-probe verification and cannot
jump to another coarse scale family. Sync evidence ranks candidates only; it can
never authenticate a payload.

### Modulo-8 block-origin refinement

The detected physical print boundary is not assumed to coincide with the original
DCT-block origin. Up to three candidates therefore search one periodic 8-pixel
canonical cell. Coarse offsets use two-pixel spacing and the local fine stage uses
0.5-pixel spacing. The budget is 35 probes per refinement. The sparse winner and
the unshifted seed are both re-evaluated with the ordinary projective probe, and
the refinement is reverted if the seed is better.

### Residual local warp

A single optional residual hypothesis is derived without the key from the 3x3
local lattice field. For each valid local basis, the nearest lattice point is
mapped back into canonical coordinates and its residual is reduced modulo the
8-pixel DCT block grid. These bounded controls feed a six-term quadratic smooth
warp. This remains diagnostic and is not promoted unless later evidence shows a
repeatable benefit.

The global research budgets remain deterministic: no more than eight coarse
projective candidates, three phase/subpixel refinement seeds, one residual-warp
fit and four complete virtual Format-v3 decode/HMAC attempts. Only a valid HMAC
can set `authenticated_payload=true`.


## 24. v0.3.0-build5 bounded photometric views (non-normative)

Build5 does not alter Format v3. After build4 geometry has produced a bounded
set of candidate mappers, at most three geometries are observed through exactly
three luminance views: raw; a 5x5 local mean/variance normalization computed
from a mapped 12x12 neighbourhood around each 8x8 DCT block; and a mild 3x3
high-pass blend using the same bounded mapped support. The DCT coefficient pair
and all v3 framing/ECC/HMAC rules are unchanged.

The bank is diagnostic and bounded: at most nine header probes are generated,
then at most four geometry/view combinations reach a complete existing v3
decode. The raw view gets a preserved opportunity, so photometric experimentation
cannot silently remove the build4 path. HMAC remains the only success criterion.


## 25. v0.3.0-build6 adaptive lattice-first initialization (non-normative)

Build6 preserves all Format-v3 and production extraction rules. The change is
confined to the diagnostic research path. A reliable print boundary remains the
preferred geometric prior, but absence of a strong boundary no longer prevents
all projective research.

For large images, if the default bounded lattice pass has no strong boundary, no
lattice evidence yet, and normalized consistency is at least 0.45, the diagnostic
may add exactly one finer pyramid level. The finer level is capped so its largest
dimension is at most 4096 pixels. On the current ~200 MP captures this means an
optional 1/4 plane in addition to the default 1/8 and 1/16 planes. No additional
angle/period bank is created.

After recomputing multilevel consensus, a weak print quadrilateral can initialize
the projective estimator only when lattice evidence is independently positive,
the quadrilateral is finite and geometrically sane, covers a substantial image
area, and has non-trivial boundary confidence. The projective source is reported
as `lattice+weak-boundary`; the boundary itself is still not watermark evidence.
If these conditions are not met, the decoder remains closed.

The downstream phase, fundamental-scale, residual-warp, photometric and complete
HMAC decode budgets are unchanged. In particular, no more than four full virtual
Format-v3 decodes are attempted and only a valid HMAC authenticates a payload.


## 26. v0.3.0-build7 protected-bit/ECC diagnostics (non-normative)

Build7 does not modify encoding, Format v3, Hamming decoding or HMAC. After the
existing build6 geometry and build5 photometric ranking have selected at most
four complete-decode candidates, the diagnostic reuses each already-built tile
grid to measure the protected-bit channel before invoking the unchanged decoder.
No extra geometry candidate, photometric view or HMAC attempt is created.

For the strongest bounded profile/phase of each candidate, tile repetitions are
combined exactly as in `readV3ProtectedFrame`, while retaining the signed average
DCT margin for every protected code bit. Format v3 fixes the first three frame
bytes (`magic[0]`, `magic[1]`, version/profile). With the supplied key, whitening
is deterministic; Hamming encoding those 24 known bits therefore yields 42
exactly-known protected bits, grouped as six known Hamming(7,4) words.

The diagnostic reports coded-bit errors within those 42 bits and classifies each
known word as clean, one-error (inside the Hamming correction radius), or
multi-error (outside the one-error design radius). It also runs the unchanged
hard Hamming decoder and counts residual errors in the 24 known raw header bits.
Across the entire selected profile it reports non-zero syndrome occupancy and
absolute coded-margin statistics. Finally, correct and wrong known coded bits
are compared by mean absolute margin. A wrong/correct ratio below one indicates
that current mistakes tend to be lower-confidence than correct bits, which is
useful evidence for a future bounded reliability-aware experiment; it is not an
authentication result.

Only the 24 fixed known header bits participate in these comparisons. Payload,
length-dependent CRC and HMAC contents are not treated as known. No unauthenticated
payload bytes are surfaced. A valid existing Format-v3 HMAC remains the sole
definition of successful recovery.

When a compact `best_bit_*` summary is required across the at-most-four decode
candidates, candidates are ordered first by fewer known multi-error Hamming
words, then by fewer post-ECC known-header errors, then by fewer known coded-bit
errors. Whole-frame syndrome occupancy and margin ratios are tie-breakers. This
ordering is diagnostic only and does not influence which candidates are decoded
or authenticated.


## 27. v0.3.0-build8 bounded full-grid and reliability decode (non-normative)

Build8 remains a diagnostic-only extension. Format v3, its Hamming(7,4) code and the
production extractor are unchanged. Two additional bounds apply inside `diagnose`:

```text
diagnostic baseline ExtractWithInfo     <= 16 Mi decoded pixels
full virtual grid before sampling       <= 50,000 canonical 8x8 blocks
large-grid spatial sampler              <= 5 x 5 complete v3 tiles (25)
complete diagnostic decode slots        <= 4
```

When a canonical candidate exceeds the block budget, complete 35x32 v3 tiles are
selected at deterministic, evenly spaced tile indices. Each selected tile contributes
one observation for every logical tile position, so the resulting 1120-value grid
keeps the same logical mapping while work no longer grows with canonical area. Small
and medium candidates retain the previous all-block aggregation.

For reliability research, the signed average margin of each protected code bit is
interpreted as a soft observation. For each seven-bit Hamming word the diagnostic
enumerates the 16 valid codewords and chooses the one maximizing the signed-margin
score. This produces exactly one decoded four-bit word; no list decoding, payload
guessing or candidate multiplication occurs.

A decode slot uses this reliability path only when the fixed, message-independent
three-byte v3 prefix shows residual hard-ECC errors, no more than one known Hamming
word outside the one-error radius, and wrong known bits with mean absolute margin less
than 90% of correct known bits. Otherwise the existing hard decoder is used. Either
method can report success only after the ordinary CRC and HMAC checks.


## 28. v0.3.0-build9 spatial protected-bit stability (non-normative)

Build9 does not alter Format v3 or any production extraction path. While the
existing full-grid sampler is already reading DCT margins for a candidate, each
observation is also accumulated into at most nine coarse spatial cells arranged
as a 3x3 grid. No additional image or DCT sample is taken for this bookkeeping.
Only cells containing every logical tile position participate.

The primary spatial metric is key-independent: for each of the 1120 logical tile
positions, the sign observed in each populated cell is compared and the majority
agreement fraction is measured. The report exposes the mean agreement and the
fraction of positions whose agreement is below 0.75. This measure does not use
the password, profile, hidden message or known header.

After the ordinary bounded profile/global-phase choice has already been made, the
six known Hamming(7,4) words from the fixed three-byte v3 prefix are also evaluated
per spatial cell. Each known protected bit is classified as correct in every cell,
wrong in every cell, or mixed. A majority-coded and post-ECC known-prefix result is
reported for diagnosis only; it is not decoded through HMAC.

A second, explicitly key-assisted oracle probes local phase drift. Each populated
cell tests only the 5x5 phase neighbourhood centered on the selected global phase
(+/-2 blocks on each axis). Thus the maximum is 25 phase probes per cell or 225
for nine cells, with no additional DCT sampling. Because this search scores the
known v3 prefix and performs multiple comparisons, any apparent header improvement
is research evidence only. Local phases are never used to form a production or
full-HMAC decode candidate in build9.

The build8 four-full-decode ceiling, photometric bank, reliability gate, residual
warp and all projective search budgets are unchanged. HMAC remains the only
authentication criterion.


## 29. v0.3.0-build10 smooth affine phase field (non-normative)

Build10 tests whether the bounded per-cell phase offsets introduced for diagnosis in
build9 contain a continuous low-degree-of-freedom component. Format v3, the
production encoder and `ExtractWithInfo` are unchanged.

For each already-retained full-decode candidate, complete v3 tile observations are
accumulated into at most nine 3x3 spatial cells. Incomplete edge tiles are excluded
from these cell statistics only; they still contribute to the ordinary aggregate
decode grid. Each populated cell performs the existing bounded +/-2-block phase
check around the candidate's selected global phase.

The local phase offsets are then fitted by weighted least squares to two affine
fields in normalized canonical coordinates `(nx, ny)`:

```text
dx(nx, ny) = a0 + a1*nx + a2*ny
dy(nx, ny) = b0 + b1*nx + b2*ny
```

The six coefficients represent a canonical sampling correction in whole-block
units. The fitted field is converted to pixels only when the virtual projective
sampler maps corrected canonical coordinates back into the source image. A local
positive phase offset therefore contributes the opposite canonical sampling
correction.

The field is measured only with at least six controls. Build10 deliberately uses
two gates:

- diagnostic resample gate: fit RMS <= 2 blocks and leave-one-out RMS <= 3 blocks;
- decode gate: fit RMS <= 1.25 blocks and leave-one-out RMS <= 2 blocks.

At most two smooth-field resamples are allowed, and only on candidates already in
the four-slot full-decode shortlist. A corrected grid can replace the original grid
inside the same HMAC slot only when the stricter gate passes, the known v3 prefix
has fewer hard post-ECC errors, and the known coded-bit error count does not
increase. This selection is key-assisted research logic and is not part of the
production decoder.

The total number of complete/HMAC decode slots remains four. The smooth field never
creates a fifth candidate and HMAC remains the sole authentication criterion.

A deterministic synthetic regression constructs exact affine local-phase controls,
applies the corresponding opposite drift to a marked image, and verifies that the
fitted inverse field restores ordinary Format-v3 HMAC authentication. A checkerboard
non-smooth control pattern is explicitly rejected by the strict decode gate.

## 30. v0.3.0-build11 confidence-weighted continuous phase controls (non-normative)

Build11 does not change Format v3, embedding, production `ExtractWithInfo`, the
photometric bank, reliability decoding, or the maximum four complete/HMAC decode
slots. It changes only the research estimate used to fit the build10 smooth phase
field.

### 30.1 Fractional local phase surface

Each complete 3x3 spatial cell keeps the build9 bounded local phase neighbourhood:
`dx,dy in [-2,+2]` blocks around the already-selected global phase. The integer
maximum remains the authoritative bounded phase candidate. Around that point,
build11 evaluates a signed correlation score over only the fixed Format-v3 sync
prefix. DCT margins are clipped at three times the local median absolute sync margin
before normalization, limiting domination by isolated high-energy coefficients.

When the integer maximum has neighbours on an axis, a three-sample parabola estimates
that axis' sub-block vertex:

```text
fraction = 0.5 * (left - right) / (left - 2*center + right)
```

The result is clamped to `[-0.5,+0.5]` block. Thus a cell can report a phase offset
such as `(0.73,-1.36)` blocks instead of only integer offsets. This interpolation does
not add image/DCT reads and does not enlarge the +/-2 integer search.

### 30.2 Control confidence and search-boundary penalty

Each local control receives a bounded confidence assembled from three diagnostic
quantities: normalized signed-correlation strength, peak prominence over the next
best sampled phase, and local parabola curvature. The confidence is a weighting
heuristic, not watermark evidence and never authenticates data.

An integer maximum that lands at `+/-2` on either axis is explicitly marked as being
at the search boundary. Such a maximum may continue outside the deliberately bounded
window, so its confidence is multiplied by `0.65` for each boundary axis. Build11
therefore does not treat a budget-truncated local maximum as a precise control point.

### 30.3 Confidence-weighted robust affine field

The build10 affine correction remains the default model:

```text
dx(nx,ny) = a0 + a1*nx + a2*ny
dy(nx,ny) = b0 + b1*nx + b2*ny
```

where `nx,ny` are normalized canonical coordinates. Each observation is weighted by
its phase confidence and local sync fraction. The fit then performs three Huber
reweighting iterations (`k=1.5`), using the median residual as a robust scale. This
allows an isolated geometrically inconsistent control to be down-weighted even when
its initial phase confidence is high.

### 30.4 Regularized quadratic comparison

With at least eight controls, build11 also evaluates the six-term basis

```text
1, nx, ny, nx*ny, nx^2, ny^2
```

independently for X and Y correction. The three quadratic terms receive ridge
regularization (`lambda=0.60`, scaled by mean control weight). The quadratic is not
selected merely because its in-sample RMS is lower. It must improve leave-one-out RMS
relative to the robust affine model by both:

- at least `0.15` block absolute RMS; and
- at least `12%` relative RMS.

It must also have no worse in-sample fit RMS. This makes the higher-order field a
cross-validated alternative rather than an automatic expansion in model freedom.

### 30.5 Bounded promotion rules

A field is diagnostic-resample eligible only with at least six controls, mean control
confidence >= `0.12`, fit RMS <= `1.75` blocks, leave-one-out RMS <= `2.50` blocks,
and maximum predicted correction <= `2.5` blocks. The stricter decode gate retains
the build10 geometry thresholds while adding confidence:

- mean control confidence >= `0.20`;
- fit RMS <= `1.25` blocks;
- leave-one-out RMS <= `2.00` blocks;
- maximum predicted correction <= `2.5` blocks.

As before, no more than two smooth corrected grids may be resampled. A corrected grid
can replace its original grid inside the same full-decode slot only if the known
post-ECC prefix improves and known coded-bit errors do not increase. The total number
of complete/HMAC decode slots remains four. HMAC is still the only success criterion.

### 30.6 Interpretation limits

The local phase surface is key-assisted because the fixed Format-v3 header/sync
pattern is key-whitened. It does not know the hidden payload, but its per-cell phase
choices are still multiple-tested research evidence and must not be interpreted as
detection. Build11 reports confidence, affine/quadratic fits and corrected known-prefix
errors only to diagnose the print-acquisition channel. A wrong key can still create
apparently structured scores; only a valid Format-v3 HMAC authenticates a payload.


## 31. v0.3.0-build12 key-independent structural phase observer (non-normative)

Build12 leaves Format v3, embedding, production `ExtractWithInfo`, the existing
projective candidate order and the four complete/HMAC slots unchanged. Its purpose is
to estimate spatial phase controls without expected Format-v3 bit values.

### 31.1 Blind repetition coherence

For robust and balanced profiles, more than one tile position maps to some of the same
protected coded bits. Let `G[p]` be the observed DCT-margin value at logical tile
position `p`. For every pair `(p0,p1)` that maps to the same coded-bit index, the blind
score at candidate phase `(px,py)` accumulates

```text
G[shift(p0,px,py)] * G[shift(p1,px,py)]
```

and normalizes by the corresponding sum of absolute products. The score therefore
rewards agreement among structural repetitions without knowing whether the repeated
bit should be positive or negative. It uses no key, whitening stream, magic bytes,
payload, CRC or HMAC. Capacity maps one tile position per coded bit and intentionally
has no repetition score.

The aggregate grid evaluates robust and balanced over all `35*32=1120` phases: at
most `2240` score probes. The selected blind profile/phase becomes the center of each
cell's existing +/-2-block search, at most `9*25=225` additional score probes. Local
peaks use the build11 fractional interpolation and boundary-aware confidence. The
weighted mean cell offset is removed before smooth fitting so this field represents
residual spatial drift rather than a second global phase estimator.

### 31.2 Pairwise fallback

If aggregate repetition evidence is unavailable, a fallback compares normalized
1120-position observed margin patterns between spatial cells. With at most nine cells
there are at most 36 pairs, each searched over a `9x9` relative-shift window
(`+/-4` blocks), for a hard ceiling of `2916` pairwise score probes. A robust graph
solve recovers zero-mean relative offsets. Real acquisition peaks from this method are
weak, so it is reported as a fallback/control rather than treated as strong evidence.

### 31.3 Oracle separation and smooth-field promotion

The build11 key-assisted local phase is still computed for research comparison, but
only **after** blind controls are independently available. Blind and oracle offsets are
centered separately before their distance is reported. Oracle offsets, expected header
bits and hidden payload information never enter the blind fit.

`diagnosticFitBlindSmoothPhaseField` accepts only cells with blind controls and sets
`control_source=blind-self-registration`. The affine/quadratic fit, confidence gates,
maximum correction, at-most-two corrected-grid resamples and four-HMAC-slot ceiling
remain those of build11. A corrected blind grid can be promoted only through the same
strict validation path; a blind score by itself is never detection.

### 31.4 Validation scope

Build12 has separate component regressions: (1) exact structural phase recovery on a
synthetic repeated robust tile without a key; (2) rejection of capacity as structural
repetition evidence; (3) pairwise relative-drift recovery; (4) proof that blind smooth
fit ignores contradictory key-assisted controls; and (5) a controlled affine blind
control field whose correction restores a valid ordinary Format-v3 HMAC. The final
test validates the downstream blind-control fit/correction path, not complete blind
observer recovery from a camera-warped synthetic image.

All blind fields and scores remain RESEARCH evidence. Only a valid Format-v3 HMAC is
a successful recovery.


## 32. v0.3.0-build13 guided dual-observer blind registration (non-normative)

Build13 leaves Format v3 and all production extraction logic unchanged. The build12
repetition-coherence controls remain the primary blind observation. A secondary
image-domain observer normalizes each populated spatial-cell 1120-position DCT-margin
grid and evaluates cross-correlation only around the pairwise shift predicted by the
primary controls. For nine cells there are at most 36 pairs and each pair evaluates a
3x3 integer-shift surface, so the normal secondary budget is at most 324 score probes.
The older 9x9 / 2916-probe search is invoked only if the repetition observer itself is
unavailable.

The guided pair constraints are solved with the same robust zero-mean graph used by the
fallback. Secondary controls are always reported. They influence the primary controls
only if their mean absolute pair peak is at least 0.10. If that gate passes, a primary
control with low confidence may receive at most one integer block of correction per
axis when the secondary observer reduces the disagreement by at least 0.35 blocks and
leaves at most 0.70 blocks residual disagreement. High-confidence primary controls are
not cycle-slip corrected. Close controls may then be weakly confidence-weighted; wide
disagreement reduces confidence. All thresholds are independent of known Format-v3
header values and HMAC outcomes.

A synthetic repeated-carrier field exercises the strong-consensus path with guided
pair score around 0.61. On the current six real acquisitions the corresponding score
is about 0.053--0.065, below the 0.10 gate, so secondary evidence remains diagnostic
and no real cycle-slip correction is applied. This negative result prevents a weak
image-domain measurement from overfitting the known-prefix diagnostics.

The complete/HMAC decode ceiling remains four and the smooth corrected-grid resample
ceiling remains two. Only an ordinary valid Format-v3 HMAC is success.

## 33. v0.3.0-build14 local fractional lattice phase (non-normative)

Build14 adds a candidate-specific, key-independent phase measurement from the already
selected local lattice field. For each populated 3x3 spatial cell, one source-space
lattice intersection is reconstructed from the region's `U`, `V`, `PhaseU` and
`PhaseV`. The current projective mapper is approximately inverted back to canonical
coordinates, including canonical offsets and any existing bounded residual warp.

For canonical coordinate `(cx, cy)`, the observer stores the measured drift modulo the
frozen 8-pixel block grid:

```text
fx = wrap_half(cx / 8)
fy = wrap_half(cy / 8)
```

where `wrap_half()` maps to `[-0.5, +0.5)`. The field reports **observed drift**; the
existing smooth-phase fitter later negates the observation to obtain the sampler
correction. Regional confidence combines the selected lattice estimate's confidence,
periodic coherence and tile-repetition coherence. Fractional controls are circularly
recentered so the common modulo phase does not become a false spatial warp.

The lattice measurement is periodic and therefore cannot determine the integer block
cycle. Fusion follows two bounded stages:

1. If lattice confidence is at least 0.16 and modulo-phase distance from the repetition
   control is at most 0.42 block, the fractional components are weakly combined while
   preserving the repetition observer's integer cycle.
2. At most two robust affine unwrap passes may move a low-confidence repetition control
   by only `-1`, `0` or `+1` block per axis. Lattice confidence must be at least 0.22,
   primary confidence at most 0.24, predicted residual must improve by at least 0.45
   block, and final residual must be at most 0.55 block.

The unwrap objective uses only control geometry and confidence. Known header errors are
computed strictly after the field is frozen and are not part of phase selection. The
build13 guided cross-cell observer remains separately reported but is not used as a
strength substitute for lattice phase.

The new observer adds no source-image DCT reads. At most nine lattice controls and two
unwrap passes are evaluated per existing full-grid candidate. The existing ceilings of
two smooth corrected-grid resamples and four complete/HMAC decode attempts remain
unchanged. Only a valid Format-v3 HMAC is an authenticated result.

The build14 real-corpus experiment is intentionally mixed: large same-candidate gains
exist, but so do degradations. Therefore the lattice/unwrap model remains research-only
and no threshold is relaxed for promotion. Detailed variant history is retained in
`docs/RESEARCH_LOG.md`.



## 34. v0.3.0-build15 global discrete phase unwrap (non-normative)

For each axis, cells with sufficient lattice confidence, non-locked primary confidence
and modulo-lattice agreement may receive an integer shift `k_i in {-1,0,+1}`. A
deterministic beam search of width 64 processes the least-certain eligible controls first.
Each complete state is scored by a robust affine residual plus a confidence-weighted
cycle-change penalty and a second-difference curvature penalty. X and Y are independent,
so the search does not enumerate the full Cartesian `3^(2N)` field.

An axis is accepted only when the proposed state improves the baseline by fixed absolute
and relative amounts and is separated from the second-best state by fixed absolute and
relative margins. High-confidence repetition controls are locked. The maximum shift is
one block per axis. The declared state budget is 3456 evaluations across both axes for
a full 3x3 field.

Lattice fusion is transactional. The fractional lattice proposal is not committed until
the integer unwrap is accepted. If the global solution is ambiguous, rejected, or has no
accepted cycle change, all lattice-derived fractional changes are discarded and the
original repetition controls are restored. This rule follows directly from build14's
fractional-only degradation experiment.

No score term reads expected Format-v3 header bits, key material, CRC or HMAC results.
Known-prefix diagnostics are computed only after the field is frozen. The existing limits
of two smooth resamples and four complete/HMAC decode attempts remain unchanged.

## 35. v0.3.0-build16 exact discrete top-2 and split-repetition validation (non-normative)

Build16 keeps the build15 axis-separable integer-cycle formulation and the same
`k_i in {-1,0,+1}` bound, but removes the width-64 beam approximation. For an axis
with `N <= 9` eligible controls, every one of the `3^N` assignments is scored by the
existing key-independent objective:

```text
E = robust affine residual
  + confidence-weighted cycle-change penalty
  + spatial second-difference curvature penalty
```

The fixed maximum is therefore 19683 complete assignments per axis and 39366 across
X/Y. Ranking uses the exact first and second complete assignments, with deterministic
tie-breaking that prefers no change, then -1, then +1 in spatial order. This makes the
reported top-2 gap a property of the declared bounded state space rather than a beam
survivor margin.

Axis status is explicit: `not-applicable`, `no-change`, `rejected-improvement`,
`ambiguous`, or `accepted`. X/Y eligibility and status are exported separately. If
there are too few usable controls to define the objective, the axis is
`not-applicable`; finite zero-valued placeholders plus an availability flag are used
for absent second-solution metrics so JSON output never contains `Inf` or `NaN`.

The build15 transactional rule is strengthened at the reporting boundary. Fractional
lattice refinement and integer cycle changes are committed only when every nontrivial
axis needed by the proposal is non-ambiguous and each applied axis passes the fixed
improvement and exact top-2 margin gates. Ambiguity on either axis makes the complete
integer-cycle transaction `ambiguous`, applies zero axes, rolls back to the pre-lattice
repetition controls and consumes no additional HMAC attempt.

Build16 also computes `split-repetition-top2` diagnostics. The Format-v3 structural
repetition pairs are deterministically divided into two disjoint folds. For spatial
cells where the exact geometric top-1 and top-2 assignments differ, each fold scores
which integer phase has greater same-coded-bit agreement. The report includes fold
pair counts, per-fold top1-minus-top2 deltas, their mean and whether both folds support
the geometric top-1.

This split score uses no key and no expected bit value. It is deliberately not an
acceptance term in build16, because the primary repetition controls were estimated
from the complete pair set before the split. It is therefore additional consistency
evidence, not a genuinely held-out discriminator. An exact ambiguous geometry result
cannot be promoted by this diagnostic.

Format v3, encoder behavior, the production decoder, whitening, Hamming ECC, mapping,
HMAC, the four full/HMAC-attempt ceiling and the two smooth-resample ceiling remain
unchanged.
