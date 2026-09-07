# PixSeal v0.1 robustness results

## Test context

The progressive measurements below were reported on 7 September 2026 during the
final development cycle. The final release candidate subsequently completed the
Linux regression and progressive test suites and was promoted without algorithm
changes to release v0.1.0.

The tests used the repository's local `original pics` corpus, the default
strength of 24, a short authenticated test message, ImageMagick transformations
and five-percentage-point steps. The source images are intentionally excluded
from the repository and release archive.

Results describe this corpus only. Image dimensions, texture, resampling filter,
payload settings and later processing can change the outcome.

## Progressive limits

| Source | Smallest recovered resize | Smallest center crop | Smallest random crop |
|---|---:|---:|---:|
| Immagine LQ | 45% | 40% | 40% |
| Immagine MQ | 40% | 20% | 20% |
| Immagine HQ | Not tested | Not tested | Not tested |

`Immagine HQ` was skipped because its approximately 201 megapixels exceeded the
default 100-megapixel robustness-test limit.

For LQ, resize extraction passed at every tested step from 95% through 45% and
failed from 40% through 10%. For MQ, it passed from 95% through 40%; an earlier
development build timed out at 35% and 30%, then reported failures from 25%
through 10%. Those two timeouts were inconclusive and must not be interpreted as
extraction failures.

Center and deterministic off-center crop results matched for this run: LQ passed
at 40% and failed at 35%, while MQ passed at 20% and failed at 15%.

## Regression expectations

The default `make deep-test` matrix is the release regression baseline:

- JPEG recompression at quality 82;
- resize at 95%, 85%, 75%, 65%, 55% and 50%;
- centered crop at 90%, 75% and 50%;
- deterministic off-center crop at 75% and 50%.

All applicable LQ and MQ transformations in this baseline were recovered during
the final development cycle. The HQ image remained skipped by the configured
pixel limit.

Run `make extreme-test` to measure a particular local corpus. `PASS` proves
recovery for that transformation; `FAIL` means extraction completed without a
valid authenticated frame; `TIMEOUT` is an inconclusive bounded result.
