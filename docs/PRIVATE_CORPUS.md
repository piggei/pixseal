# Private physical corpus

The physical print/camera and scanner acquisitions are research material and are
**not distributed with PixSeal source archives**. Build16 also removes the physical
corpus key from source defaults; provide it only through local configuration or the
environment when running physical tests.

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
PRINT_CAMERA_KEY='local-secret' make print-camera-test
PRINT_CAMERA_KEY='local-secret' make print-scan-test
```

Only a valid Format-v3 HMAC counts as a PASS. Geometry, lattice, error-count and
unwrap diagnostics remain research evidence.
