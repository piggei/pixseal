package watermark

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func diagnosticTestImage(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	var state uint32 = 0x9e3779b9
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			state = state*1664525 + 1013904223
			noise := uint8(state >> 24)
			base := uint8((x*7 + y*11 + (x*y)%251) & 0xff)
			img.SetNRGBA(x, y, color.NRGBA{
				R: uint8((uint16(base) + uint16(noise)/3) & 0xff),
				G: uint8((uint16(base)*3 + uint16(noise)/2 + uint16(y)) & 0xff),
				B: uint8((uint16(base)*5 + uint16(noise)/4 + uint16(x)) & 0xff),
				A: 255,
			})
		}
	}
	return img
}

func TestDiagnosticEstimatorSeparatesCanonicalCarrierFromUnmarked(t *testing.T) {
	base := diagnosticTestImage(640, 640)
	key := []byte("diagnostic-test-key")
	marked, err := Embed(base, []byte("diagnostic"), key, Options{Profile: ProfileRobust, Strength: 24})
	if err != nil {
		t.Fatal(err)
	}
	options := DefaultDiagnosticOptions()
	options.MaxLevels = 1
	markedReport, err := DiagnoseGeometry(marked, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if !markedReport.LatticeEvidence {
		t.Fatalf("marked carrier lattice evidence=false; consistency=%.3f u=(%.2f,%.2f) v=(%.2f,%.2f)",
			markedReport.GlobalConsistency, markedReport.GlobalU.X, markedReport.GlobalU.Y, markedReport.GlobalV.X, markedReport.GlobalV.Y)
	}
	if got := math.Hypot(markedReport.GlobalU.X, markedReport.GlobalU.Y); got < 6 || got > 10.5 {
		t.Fatalf("marked global |u|=%.2f; want a bounded estimate near the native 8-pixel lattice", got)
	}

	unmarkedReport, err := DiagnoseGeometry(base, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if unmarkedReport.LatticeEvidence {
		t.Fatalf("unmarked image produced lattice evidence; consistency=%.3f", unmarkedReport.GlobalConsistency)
	}
}

func TestDiagnosticAuthenticationIsIndependentFromLatticeEvidence(t *testing.T) {
	base := diagnosticTestImage(640, 640)
	key := []byte("diagnostic-test-key")
	marked, err := Embed(base, []byte("authenticated"), key, Options{Profile: ProfileRobust, Strength: 24})
	if err != nil {
		t.Fatal(err)
	}
	options := DefaultDiagnosticOptions()
	options.MaxLevels = 1
	options.AttemptAuthentication = true
	report, err := DiagnoseGeometry(marked, key, options)
	if err != nil {
		t.Fatal(err)
	}
	if !report.AuthenticatedPayload || report.AuthenticationStatus != "baseline-authenticated" {
		t.Fatalf("authentication status=%q authenticated=%t", report.AuthenticationStatus, report.AuthenticatedPayload)
	}
	wrongReport, err := DiagnoseGeometry(marked, []byte("definitely-wrong-key"), options)
	if err != nil {
		t.Fatal(err)
	}
	if wrongReport.AuthenticatedPayload || wrongReport.AuthenticationStatus != "baseline-failed" {
		t.Fatalf("wrong-key status=%q authenticated=%t", wrongReport.AuthenticationStatus, wrongReport.AuthenticatedPayload)
	}
}
