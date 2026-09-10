package watermark

import (
	"image"
	"image/draw"
	"math"
	"testing"
)

func TestDiagnosticScaleClusteringRewardsCrossRegionSupport(t *testing.T) {
	proposals := []diagnosticScaleProposal{
		{width: 2020, height: 1515, score: .40, region: 0, divisor: 8},
		{width: 2030, height: 1520, score: .35, region: 1, divisor: 16},
		{width: 2010, height: 1508, score: .30, region: 2, divisor: 8},
		{width: 3300, height: 2400, score: .70, region: 0, divisor: 8},
	}
	got := clusterDiagnosticScales(proposals)
	if len(got) == 0 {
		t.Fatal("no diagnostic scale clusters")
	}
	if math.Abs(got[0].CanonicalWidthPixels-2020) > 40 || math.Abs(got[0].CanonicalHeightPixels-1515) > 40 {
		t.Fatalf("best scale=(%.1f,%.1f), want cross-region cluster near 2020x1515", got[0].CanonicalWidthPixels, got[0].CanonicalHeightPixels)
	}
	if got[0].SupportingRegions != 3 || got[0].SupportingLevels != 2 {
		t.Fatalf("best scale support=%d regions/%d levels, want 3/2", got[0].SupportingRegions, got[0].SupportingLevels)
	}
}

func TestDiagnosticProjectiveVirtualAuthentication(t *testing.T) {
	base := diagnosticTestImage(640, 480)
	key := []byte("projective-diagnostic-key")
	message := []byte("virtual")
	marked, err := Embed(base, message, key, Options{Profile: ProfileRobust, Strength: 32})
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, 800, 620))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	rect := image.Rect(80, 70, 720, 550)
	draw.Draw(canvas, rect, marked, image.Point{}, draw.Src)
	boundary := PrintBoundaryEstimate{
		Detected: true, Confidence: 1,
		TopLeft: ImagePoint{X: 80, Y: 70}, TopRight: ImagePoint{X: 719, Y: 70},
		BottomRight: ImagePoint{X: 719, Y: 549}, BottomLeft: ImagePoint{X: 80, Y: 549},
	}
	h, ok := homographyForPrintBoundary(640, 480, boundary)
	if !ok {
		t.Fatal("projective test homography failed")
	}
	estimate := DiagnosticProjectiveEstimate{
		Available: true, BoundaryConfidence: 1, CanonicalWidthPixels: 640, CanonicalHeightPixels: 480,
		Matrix:          h.h,
		ScaleCandidates: []DiagnosticScaleCandidate{{CanonicalWidthPixels: 640, CanonicalHeightPixels: 480, Confidence: 1, SupportingRegions: 9, SupportingLevels: 2}},
	}
	payload, info, evidence, found := attemptDiagnosticProjectiveAuthentication(canvas, key, estimate)
	if !found {
		t.Fatalf("virtual projective authentication failed; evidence=%+v", evidence)
	}
	if string(payload) != string(message) || info.Profile != ProfileRobust {
		t.Fatalf("payload=%q profile=%s, want %q/%s", payload, info.Profile, message, ProfileRobust)
	}
	if evidence.FullDecodeAttempts != 1 || evidence.CandidatesProbed != 1 {
		t.Fatalf("projective budget usage probes=%d decodes=%d, want 1/1", evidence.CandidatesProbed, evidence.FullDecodeAttempts)
	}

	if _, _, _, found := attemptDiagnosticProjectiveAuthentication(canvas, []byte("definitely-wrong-key"), estimate); found {
		t.Fatal("wrong key authenticated through projective diagnostic")
	}
}

func TestDiagnosticPhaseConsensusPrefersExactScale(t *testing.T) {
	base := diagnosticTestImage(840, 768)
	key := []byte("phase-consensus-test-key")
	marked, err := Embed(base, []byte("phase"), key, Options{Profile: ProfileRobust, Strength: 32})
	if err != nil {
		t.Fatal(err)
	}
	boundary := PrintBoundaryEstimate{
		Detected: true, Confidence: 1,
		TopLeft: ImagePoint{X: 0, Y: 0}, TopRight: ImagePoint{X: 839, Y: 0},
		BottomRight: ImagePoint{X: 839, Y: 767}, BottomLeft: ImagePoint{X: 0, Y: 767},
	}
	decoder := newDecoder(key)
	exact := diagnosticProjectivePhaseConsensus(marked, boundary, DiagnosticScaleCandidate{
		CanonicalWidthPixels: 840, CanonicalHeightPixels: 768, Confidence: 1,
	}, decoder)
	perturbed := diagnosticProjectivePhaseConsensus(marked, boundary, DiagnosticScaleCandidate{
		CanonicalWidthPixels: 856.8, CanonicalHeightPixels: 768, Confidence: 1,
	}, decoder)
	if exact.tiles < 4 {
		t.Fatalf("exact phase tiles=%d, want at least 4", exact.tiles)
	}
	if exact.score <= perturbed.score {
		t.Fatalf("exact phase score %.3f <= 2%% width-error score %.3f", exact.score, perturbed.score)
	}
}

func TestDiagnosticPhaseDifferenceIsBoundedModuloTile(t *testing.T) {
	if got := diagnosticPhaseDifference(34, 1, tileWidth); got != -2 {
		t.Fatalf("wrapped X phase difference=%v, want -2", got)
	}
	if got := diagnosticPhaseDifference(1, 31, tileHeight); got != 2 {
		t.Fatalf("wrapped Y phase difference=%v, want 2", got)
	}
}
