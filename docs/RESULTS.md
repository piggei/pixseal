# PixSeal results

## v0.2.0-rc2 hardening status

`v0.2.0-rc2` is a hardening candidate derived from the qualified v0.2.0 RC1
algorithm. Format v3 and the encoder mapping are unchanged; the deterministic
encoder fingerprints remain the compatibility gate.

RC2 addresses issues found by an independent pre-release audit:

- finite/explicit strength validation;
- output-path and no-clobber safety;
- private permissions for new outputs;
- dotfile output naming;
- raw multiline-safe extraction output;
- pre-decode/source-size guards;
- alpha-consistent analyzer sampling;
- Windows amd64 build metadata;
- perspective harness correctness and a real Go end-to-end perspective test;
- VERSION/buildinfo consistency checking;
- removal of dead bicubic normalization code;
- complete documentation realignment.

RC2 still requires a full private-corpus qualification run before the `-rc2`
suffix can be removed.

### Local RC2 verification performed during packaging

The release package is expected to pass at least:

```text
gofmt
go vet ./...
make release-unit
make research-unit
go test ./cmd/pixseal
targeted watermark hardening/perspective tests
bash -n scripts/*.sh
native build
core-target-check
version-check
```

The full uncached `go test ./...` remains available through `make test`, but
RC2 release qualification uses `release-unit` and keeps deterministic
experimental-geometry Go regressions in `research-unit`. Private-corpus
`release-check`/`all-test` are distinguished from local packaging checks because
the private images are not part of the source archive.

## v0.2.0 RC1 qualification — reference baseline

The v0.2.0 RC1 qualification run on PJ's Linux/WSL2 corpus is the algorithmic
baseline from which RC2 was hardened.

Environment recorded by the run:

```text
Linux 6.18.33.2-microsoft-standard-WSL2 x86_64
Go 1.25.1 linux/amd64
ImageMagick 6.9.11-60 Q16
```

Summary:

| Target | RC1 result | Notes |
|---|---:|---|
| `vet` | PASS | static analysis |
| `test` | PASS | unit tests + 2 images × 3 profiles round trip |
| `deep-test` | PASS | **72/72** JPEG/resize/crop cases |
| `extreme-test` | PASS | non-strict progressive boundary map |
| `geometry-test` | ATTENTION | **65/120** experimental cases |
| `affine-test` | ATTENTION | **22/24** experimental cases |
| `composition-test` | PASS | **2/2** |
| `lattice-test` | PASS | **8/8** |
| `perspective-test` | PASS | **4/4** mild vertical-keystone cases |
| `core-target-check` | PASS | linux/windows/android/ios core builds |

The all-test summary was:

```text
Release baseline: PASS
Research suites: ATTENTION (2 experimental targets failed)
Overall: FAIL (2 targets failed)
```

The overall strict failure is deliberate: research limits remain visible rather
than being hidden behind the release-baseline classification.

### Baseline transformation qualification

On both private qualification images and all three profiles, RC1 passed:

```text
JPEG quality 82
resize 95%
resize 85%
resize 75%
resize 65%
resize 55%
resize 50%
center crop 90%
center crop 75%
center crop 50%
random crop 75%
random crop 50%
```

Total: **72/72 PASS**.

This is the v0.2 stable digital recovery baseline.

### Progressive limits

`extreme-test` is a boundary exploration rather than a strict success matrix.
Individual FAIL entries below the recoverable boundary are expected data.

On the RC1 qualification corpus, larger/more redundant carriers generally
survived more aggressive resize/crop than smaller carriers. The precise boundary
is image- and profile-dependent and is not an API guarantee.

### Experimental arbitrary/combined geometry

RC1 recorded **65/120** passes in `geometry-test`. One private carrier handled
arbitrary rotations very well while another was strongly texture-dependent and
mostly retained only exact quarter turns. This is why arbitrary rotation remains
an experimental capability rather than part of the release guarantee.

Combined edit ordering also matters. Some rotate→resize/crop cases pass where
the reverse order does not.

### Experimental axis-aligned affine

RC1 recorded **22/24** passes. The two failures were corpus/profile-specific
shear cases. Anisotropic scale cases were generally strong on the qualification
corpus.

### Fixed lattice-basis composition

RC1 passed **8/8** cases for the fixed direct lattice bank:

```text
110%x90% + rotation
90%x110% + rotation
105%x95% + rotation
95%x105% + rotation
```

These are discrete supported research hypotheses, not continuous affine-basis
inference.

### Mild projective recovery

RC1 passed **4/4** ImageMagick-generated cases for:

```text
top-narrow-4
bottom-narrow-4
```

The v0.2 decoder contains exactly those two vertical-keystone hypotheses. The
result demonstrates bounded projective sampling, not general perspective
estimation.

## Encoder compatibility fingerprints

The v0.2 line freezes deterministic pixel fingerprints for a known image/key/
payload input:

```text
robust
f78b3f97824780cbbd2a303d25b0d36c94f845cdd8213291fc76b263f14a08bb

balanced
9b548bd4da9befab4d4ab97fb0d2fe758d79cd0c7ef30c0c32079f8c03874029

capacity
cdf7007a900c05247b532e327b549607cdb7e8c2d677c71b46481b835edfc577
```

A change to one of these values is treated as an encoder/Format-v3 regression
unless deliberately accompanied by a new format decision.

## Large-image observations

A private 16320×12288 image (~200.5 MP) has been used successfully for basic
round-trip testing. Research geometry scripts intentionally use lower megapixel
limits to control runtime.

RC2 adds a 300 MP pre-decode/source guard so absurdly large images fail before
PixSeal builds its complete RGB working plane. This does not make the v0.2
implementation memory-sparse; it simply prevents unbounded pre-allocation.

## Print-camera research status

Two ~200 MP original smartphone captures of a printed v3 carrier have been kept
as private research inputs. The v0.2 decoder has **not** authenticated the hidden
payload from those photographs.

This result is outside the v0.2 release baseline. The next research line is
expected to study local lattice-basis and general homography inference before
considering any encoder change.

## Interpreting results

Three kinds of statement must remain separate:

1. **Deterministic format facts** — capacities, frame sizes, search bounds.
2. **Heuristics** — texture/detail scores, geometric ranking and confidence.
3. **Experimental measurements** — PASS/FAIL on a specific image,
   transformation and software environment.

A PASS on one corpus is not a universal recovery guarantee; a heuristic score is
not proof that a PixSeal payload is present. Only a valid authenticated Format v3
frame is a successful extraction.
