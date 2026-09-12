# Private physical corpus

The physical print/camera and scanner acquisitions are research material and are
**not distributed with PixSeal source archives**. Their test key is intentionally
public and reproducible: `Piccotti`. Build17 keeps it as the default
`PRINT_CAMERA_KEY` so the historical corpus can be decoded without extra setup.
It is a project test constant, not a personal or production credential. If PixSeal is
ever promoted to a production deployment, replace this Makefile/script default with a
different production key. The private research corpus itself is not part of production
artifacts, so production compatibility with `Piccotti` is not required.

Canonical smartphone filenames:

```text
foto stampa.jpg
foto stampa storta.jpg
foto bici dritta.jpg
foto bici storta.jpg
```

Canonical scanner filenames:

```text
0270_001.jpg
0270_002.jpg
```

The two historical `foto stampa*.PNG` files contained JPEG data despite their
extension. The canonical corpus names use `.jpg`; renaming changes only the filename,
not the image bytes.

Generate a local SHA-256 manifest after placing the private files in the default
folders:

```bash
make private-corpus-manifest
```

The output `private-corpus-manifest.sha256` is ignored by Git and records canonical
relative labels rather than machine-specific absolute paths. Keep the manifest with
the private corpus if you need to verify that two research runs used byte-identical
acquisitions.

Physical regression examples:

```bash
make print-camera-test        # defaults to PRINT_CAMERA_KEY=Piccotti
make print-scan-test           # defaults to PRINT_CAMERA_KEY=Piccotti
PRINT_CAMERA_KEY='other-test-key' make print-camera-test  # optional override
```

Only a valid Format-v3 HMAC counts as a PASS. Geometry, lattice, error-count and
unwrap diagnostics remain research evidence.

## Build20

Build20 continues to use `Piccotti` as the intentional public/reproducible research key. The
private acquisition files remain excluded from source archives. The independent pairwise cycle
anchor was evaluated on the same four available physical acquisitions; the two bicycle photos
are still absent from the current corpus. A future production release must replace the default
research key and will not distribute this private corpus.
