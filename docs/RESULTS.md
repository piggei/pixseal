# PixSeal v0.1 robust-steganography results

## Test context

The measurements below were recorded on 7 September 2026 during the final v0.1
development cycle. They describe recovery of an authenticated hidden payload
after image transformations; they are not claims of general steganographic
undetectability or guarantees for unseen images.

The tests used the repository's local `original pics` corpus, the default
strength of 24, a short authenticated test message, ImageMagick transformations
and five-percentage-point steps. The source images are intentionally excluded
from Git and release archives.

Image dimensions, texture, resampling filter, payload settings and later
processing can change the outcome.

The release cleanup after these measurements did not change the v2 frame, DCT
coefficients, synchronization rules or acceptance criteria. Extraction was
optimized by caching decoded pixel/luminance data and avoiding repeated image
interface conversions. The bicubic normalized luminance path remains equivalent
to the previous channel-wise bicubic reconstruction used for decoding.

## Progressive measurements

| Source | Smallest recovered resize tested | Last passing center crop before first failure | Last passing random crop before first failure |
|---|---:|---:|---:|
| Immagine LQ | 45% | 40% | 40% |
| Immagine MQ | 40% | 20% | 20% |
| Immagine HQ | Not tested | Not tested | Not tested |

`Immagine HQ` was skipped because its approximately 201 megapixels exceeded the
default 100-megapixel **test-harness** limit.

For LQ, resize extraction passed at every tested step from 95% through 45% and
failed from 40% through 10%. For MQ, it passed from 95% through 40%; an earlier
development build timed out at 35% and 30%, then reported failures from 25%
through 10%. Those two timeouts were inconclusive and must not be interpreted as
proof that the hidden payload was absent.

Center and deterministic off-center crop results matched at the observed
boundary in that run: LQ passed at 40% and first failed at 35%, while MQ passed
at 20% and first failed at 15%.

### Important crop-methodology note

The recorded crop run used the earlier progressive script, which stopped each
crop sequence at its first failed extraction. Therefore the 40% and 20% crop
figures above mean **last passing step before the first observed failure**. They
do not prove that every smaller crop would fail; crop alignment can make recovery
non-monotonic.

The release version of `make extreme-test` corrects this methodology and now
continues through every configured centered-crop and random-crop percentage,
just as it already did for resize percentages. New measurements should therefore
report the complete pass/fail/timeout set rather than infer a hard boundary from
the first failure.

## Regression expectations

The default `make deep-test` matrix is the release regression baseline:

- JPEG recompression at quality 82;
- resize at 95%, 85%, 75%, 65%, 55% and 50%;
- centered crop at 90%, 75% and 50%;
- deterministic off-center crop at 75% and 50%.

All applicable LQ and MQ transformations in this baseline were recovered during
the final development cycle. The HQ image remained skipped by the configured
100-MP test-harness limit.

That harness limit is separate from the decoder's internal rule that skips any
single reconstructed normalization candidate above 50,000,000 pixels.

Run `make extreme-test` to measure a particular local corpus. `PASS` proves
recovery for that exact transformation; `FAIL` means extraction completed
without a valid authenticated payload; `TIMEOUT` is an inconclusive bounded
result. None of these outcomes establishes statistical detectability or
undetectability of the steganographic channel.
