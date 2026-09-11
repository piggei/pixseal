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
	if evidence.BitDiagnosticAttempts != 1 || len(evidence.BitDiagnostics) != 1 {
		t.Fatalf("bit diagnostic usage attempts=%d entries=%d, want 1/1", evidence.BitDiagnosticAttempts, len(evidence.BitDiagnostics))
	}
	bit := evidence.BitDiagnostics[0]
	if bit.KnownHeaderCodedErrors != 0 || bit.KnownHeaderMultiWords != 0 || bit.PostECCHdrErrors != 0 {
		t.Fatalf("exact projective grid has unexpected bit-channel errors: %+v", bit)
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

func TestDiagnosticFundamentalSelectionRejectsHigherFrequencyAliases(t *testing.T) {
	candidates := []DiagnosticScaleCandidate{
		{CanonicalWidthPixels: 1500, CanonicalHeightPixels: 1125, Confidence: .80, SupportingRegions: 5, SupportingLevels: 2},
		{CanonicalWidthPixels: 1000, CanonicalHeightPixels: 750, Confidence: .30, SupportingRegions: 2, SupportingLevels: 2},
		// A still lower frequency observed at only one pyramid level is not
		// sufficient evidence for the fundamental family.
		{CanonicalWidthPixels: 650, CanonicalHeightPixels: 488, Confidence: .50, SupportingRegions: 4, SupportingLevels: 1},
	}
	markDiagnosticFundamental(candidates)
	if !candidates[1].Fundamental {
		t.Fatalf("expected 1000x750 multi-level family to be fundamental: %+v", candidates)
	}
	if candidates[0].Fundamental || candidates[2].Fundamental {
		t.Fatalf("unexpected alias marked fundamental: %+v", candidates)
	}
}

func TestDiagnosticSubpixelOffsetCorrectsBoundaryPhase(t *testing.T) {
	base := diagnosticTestImage(640, 480)
	key := []byte("subpixel-boundary-phase-key")
	marked, err := Embed(base, []byte("offset"), key, Options{Profile: ProfileRobust, Strength: 32})
	if err != nil {
		t.Fatal(err)
	}
	boundary := PrintBoundaryEstimate{
		Detected: true, Confidence: 1,
		TopLeft: ImagePoint{X: 0, Y: 0}, TopRight: ImagePoint{X: 639, Y: 0},
		BottomRight: ImagePoint{X: 639, Y: 479}, BottomLeft: ImagePoint{X: 0, Y: 479},
	}
	h, ok := homographyForPrintBoundary(640, 480, boundary)
	if !ok {
		t.Fatal("subpixel test homography failed")
	}
	candidate := DiagnosticScaleCandidate{CanonicalWidthPixels: 640, CanonicalHeightPixels: 480, Confidence: 1, Fundamental: true}
	decoder := newDecoder(key)
	// Start from a deliberately wrong intra-block phase. The bounded search has
	// access only to the modulo-8 neighbourhood and should never make the sparse
	// sync evidence worse after full-probe verification.
	baseMapper := diagnosticProjectiveMapper{h: h, canonicalOffsetX: 3, canonicalOffsetY: -3}
	baseProbe := diagnosticProbeProjectiveSyncWithMapper(marked, candidate, baseMapper, decoder)
	mapper, probe, attempts := refineDiagnosticSubpixelOffset(marked, candidate, baseMapper, decoder)
	if attempts > 40 {
		t.Fatalf("subpixel probe budget=%d, want <=40", attempts)
	}
	if probe.z < baseProbe.z {
		t.Fatalf("subpixel refinement degraded z %.3f -> %.3f", baseProbe.z, probe.z)
	}
	if mapper.canonicalOffsetX < -4 || mapper.canonicalOffsetX >= 4 || mapper.canonicalOffsetY < -4 || mapper.canonicalOffsetY >= 4 {
		t.Fatalf("subpixel offset escaped modulo-8 cell: (%.2f,%.2f)", mapper.canonicalOffsetX, mapper.canonicalOffsetY)
	}
}

func TestDiagnosticResidualWarpFitsSmoothSubBlockField(t *testing.T) {
	controls := make([]diagnosticResidualControl, 0, 9)
	for gy := 0; gy < 3; gy++ {
		for gx := 0; gx < 3; gx++ {
			x := float64(gx) * 500
			y := float64(gy) * 400
			nx := 2*x/1000 - 1
			ny := 2*y/800 - 1
			dx := 0.75 + 0.6*nx - 0.25*ny + 0.2*nx*ny
			dy := -0.5 + 0.2*nx + 0.55*ny - 0.15*nx*ny
			controls = append(controls, diagnosticResidualControl{x: x, y: y, dx: dx, dy: dy, weight: 1})
		}
	}
	warp, ok := fitDiagnosticResidualWarp(1000, 800, controls)
	if !ok {
		t.Fatal("smooth residual warp fit failed")
	}
	if warp.rmsPixels > 1e-6 {
		t.Fatalf("residual warp RMS=%.8f, want near zero", warp.rmsPixels)
	}
	dx, dy := warp.correction(250, 600)
	nx, ny := -0.5, 0.5
	wantX := 0.75 + 0.6*nx - 0.25*ny + 0.2*nx*ny
	wantY := -0.5 + 0.2*nx + 0.55*ny - 0.15*nx*ny
	if math.Abs(dx-wantX) > 1e-6 || math.Abs(dy-wantY) > 1e-6 {
		t.Fatalf("residual correction=(%.6f,%.6f), want (%.6f,%.6f)", dx, dy, wantX, wantY)
	}
}

func TestDiagnosticPhotometricBankAuthenticatesPristineCarrier(t *testing.T) {
	base := diagnosticTestImage(640, 480)
	key := []byte("photometric-bank-test-key")
	message := []byte("photo-bank")
	marked, err := Embed(base, message, key, Options{Profile: ProfileRobust, Strength: 32})
	if err != nil {
		t.Fatal(err)
	}
	boundary := PrintBoundaryEstimate{
		Detected: true, Confidence: 1,
		TopLeft: ImagePoint{X: 0, Y: 0}, TopRight: ImagePoint{X: 639, Y: 0},
		BottomRight: ImagePoint{X: 639, Y: 479}, BottomLeft: ImagePoint{X: 0, Y: 479},
	}
	h, ok := homographyForPrintBoundary(640, 480, boundary)
	if !ok {
		t.Fatal("photometric test homography failed")
	}
	mapper := diagnosticProjectiveMapper{h: h}
	decoder := newDecoder(key)
	for _, mode := range diagnosticPhotometricModes() {
		grid, ok := diagnosticProjectiveGridWithMapperPhotometric(marked, 640, 480, mapper, mode)
		if !ok {
			t.Fatalf("%s grid unavailable", mode.String())
		}
		payload, info, _, found := decoder.decodeGrid(grid)
		if !found {
			t.Fatalf("%s did not authenticate pristine carrier", mode.String())
		}
		if string(payload) != string(message) || info.Profile != ProfileRobust {
			t.Fatalf("%s payload=%q profile=%s", mode.String(), payload, info.Profile)
		}
	}
}

func TestDiagnosticPhotometricBankIsBounded(t *testing.T) {
	if got := len(diagnosticPhotometricModes()); got != diagnosticPhotometricModeCount {
		t.Fatalf("photometric modes=%d, want %d", got, diagnosticPhotometricModeCount)
	}
	if diagnosticMaxPhotometricProbeAttempts != diagnosticPhotometricModeCount*diagnosticMaxPhotometricGeometries {
		t.Fatalf("photometric probe budget=%d is inconsistent", diagnosticMaxPhotometricProbeAttempts)
	}
	if diagnosticMaxPhotometricProbeAttempts > 9 {
		t.Fatalf("photometric probe budget=%d, want <=9", diagnosticMaxPhotometricProbeAttempts)
	}
}

func TestDiagnosticAdaptiveEscalationAddsOneFinerLevel(t *testing.T) {
	levels := []DiagnosticLevel{{Divisor: 8, Width: 2040, Height: 1536}, {Divisor: 16, Width: 1020, Height: 768}}
	divisor, reason, ok := diagnosticAdaptiveDivisor(levels, false, false, 0.53, 16320, 12288)
	if !ok || divisor != 4 {
		t.Fatalf("adaptive divisor=%d ok=%t, want 4/true", divisor, ok)
	}
	if reason == "" {
		t.Fatal("adaptive escalation reason is empty")
	}
	if _, _, ok := diagnosticAdaptiveDivisor(levels, true, false, 0.53, 16320, 12288); ok {
		t.Fatal("strong boundary unexpectedly triggered adaptive escalation")
	}
	if _, _, ok := diagnosticAdaptiveDivisor(levels, false, false, 0.30, 16320, 12288); ok {
		t.Fatal("low-consistency noise unexpectedly triggered adaptive escalation")
	}
}

func TestDiagnosticWeakBoundaryRequiresPlausibleQuad(t *testing.T) {
	weak := PrintBoundaryEstimate{
		Detected: false, Confidence: .30,
		TopLeft: ImagePoint{X: 80, Y: 70}, TopRight: ImagePoint{X: 920, Y: 50},
		BottomRight: ImagePoint{X: 940, Y: 720}, BottomLeft: ImagePoint{X: 60, Y: 700},
	}
	if !diagnosticBoundaryCandidateUsable(weak, 1000, 800) {
		t.Fatal("plausible weak quadrilateral was rejected")
	}
	if _, ok := homographyForPrintBoundary(640, 480, weak); !ok {
		t.Fatal("weak-but-plausible boundary could not initialize homography")
	}
	weak.Confidence = .05
	if diagnosticBoundaryCandidateUsable(weak, 1000, 800) {
		t.Fatal("very-low-confidence quadrilateral was accepted")
	}
}

func TestDiagnosticSmoothPhaseFieldRecoversAffinePhaseDrift(t *testing.T) {
	base := diagnosticTestImage(840, 768)
	key := []byte("smooth-phase-field-key")
	message := []byte("smooth-field")
	marked, err := Embed(base, message, key, Options{Profile: ProfileRobust, Strength: 32})
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, 920, 848))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(40, 40, 880, 808), marked, image.Point{}, draw.Src)
	boundary := PrintBoundaryEstimate{
		Detected: true, Confidence: 1,
		TopLeft: ImagePoint{X: 40, Y: 40}, TopRight: ImagePoint{X: 879, Y: 40},
		BottomRight: ImagePoint{X: 879, Y: 807}, BottomLeft: ImagePoint{X: 40, Y: 807},
	}
	h, ok := homographyForPrintBoundary(840, 768, boundary)
	if !ok {
		t.Fatal("smooth-phase test homography failed")
	}

	// Simulate a smooth sampling drift of +/- one whole 8px block at the
	// outer 3x3 cell centers. These idealized local-phase observations are
	// independent of the hidden payload and define the inverse correction.
	spatial := &DiagnosticSpatialBitEvidence{}
	for ry := 0; ry < 3; ry++ {
		for rx := 0; rx < 3; rx++ {
			spatial.CellsEvidence = append(spatial.CellsEvidence, DiagnosticSpatialCellEvidence{
				RegionX: rx, RegionY: ry,
				LocalPhaseOffsetX:      rx - 1,
				LocalPhaseOffsetY:      ry - 1,
				LocalPhaseSyncFraction: 1,
			})
		}
	}
	warp, smooth, ok := diagnosticFitSmoothPhaseField(spatial, 840, 768)
	if !ok || !smooth.Available || !smooth.Eligible || !smooth.DecodeEligible {
		t.Fatalf("smooth-phase field not accepted: %+v", smooth)
	}
	if smooth.FitRMSBlocks > 1e-9 || smooth.LeaveOneOutRMSBlocks > 1e-9 {
		t.Fatalf("exact affine field fit is not exact: %+v", smooth)
	}

	badWarp := &diagnosticResidualWarp{width: 840, height: 768, maxPixels: 16}
	badWarp.dx[1] = 12 // nx=+/-2/3 -> +/-8 px at outer cell centers
	badWarp.dy[2] = 12 // ny=+/-2/3 -> +/-8 px
	correctedMapper := diagnosticProjectiveMapper{h: h, warp: badWarp, smoothPhaseWarp: warp}
	correctedGrid, _, ok := diagnosticProjectiveGridWithMapperPhotometricStats(canvas, 840, 768, correctedMapper, diagnosticPhotometricRaw)
	if !ok {
		t.Fatal("smooth-phase corrected grid unavailable")
	}
	decoder := newDecoder(key)
	payload, info, _, found := decoder.decodeGrid(correctedGrid)
	if !found {
		corrected, _ := diagnosticAnalyzeBitChannel(correctedGrid, key, decoder, "corrected", diagnosticPhotometricRaw)
		t.Fatalf("smooth-phase correction did not authenticate; fit=%+v corrected=%+v", smooth, corrected)
	}
	if string(payload) != string(message) || info.Profile != ProfileRobust {
		t.Fatalf("smooth-phase payload=%q profile=%s, want %q/robust", payload, info.Profile, message)
	}
}

func TestDiagnosticSmoothPhaseFieldRejectsNonSmoothLocalChoices(t *testing.T) {
	spatial := &DiagnosticSpatialBitEvidence{}
	for ry := 0; ry < 3; ry++ {
		for rx := 0; rx < 3; rx++ {
			sign := 1
			if (rx+ry)%2 == 0 {
				sign = -1
			}
			spatial.CellsEvidence = append(spatial.CellsEvidence, DiagnosticSpatialCellEvidence{
				RegionX: rx, RegionY: ry,
				LocalPhaseOffsetX:      2 * sign,
				LocalPhaseOffsetY:      -2 * sign,
				LocalPhaseSyncFraction: 0.75,
			})
		}
	}
	_, smooth, ok := diagnosticFitSmoothPhaseField(spatial, 1800, 1400)
	if !ok || !smooth.Available {
		t.Fatal("non-smooth field should still be measurable")
	}
	if smooth.DecodeEligible {
		t.Fatalf("non-smooth local choices unexpectedly decode-eligible: %+v", smooth)
	}
}

func TestDiagnosticSmoothPhaseBudgetsStayBounded(t *testing.T) {
	if diagnosticMaxSmoothPhaseResamples > diagnosticMaxProjectiveFullDecodes {
		t.Fatalf("smooth resamples=%d exceed full-decode budget=%d", diagnosticMaxSmoothPhaseResamples, diagnosticMaxProjectiveFullDecodes)
	}
	if diagnosticSmoothPhaseMinControls < 6 {
		t.Fatalf("smooth minimum controls=%d, want >=6", diagnosticSmoothPhaseMinControls)
	}
	if diagnosticSmoothPhaseDecodeMaxFitRMSBlocks >= diagnosticSmoothPhaseMaxFitRMSBlocks ||
		diagnosticSmoothPhaseDecodeMaxLOORMSBlocks >= diagnosticSmoothPhaseMaxLOORMSBlocks {
		t.Fatal("smooth decode gate is not stricter than diagnostic resample gate")
	}
}

func TestDiagnosticPhaseSurfaceParabolicSubblockPeak(t *testing.T) {
	peak := 0.25
	score := func(x float64) float64 { return 1 - (x-peak)*(x-peak) }
	offset, curvature := diagnosticParabolicPeakOffset(score(-1), score(0), score(1))
	if math.Abs(offset-peak) > 1e-9 {
		t.Fatalf("subblock peak=%.6f, want %.6f", offset, peak)
	}
	if curvature <= 0 {
		t.Fatalf("curvature=%.6f, want positive", curvature)
	}
}

func TestDiagnosticSmoothPhaseConfidenceDownweightsOutlier(t *testing.T) {
	spatial := &DiagnosticSpatialBitEvidence{}
	for ry := 0; ry < 3; ry++ {
		for rx := 0; rx < 3; rx++ {
			nx := 2*(float64(rx)+0.5)/3 - 1
			ny := 2*(float64(ry)+0.5)/3 - 1
			correctionX := 0.15 + 0.55*nx - 0.20*ny
			correctionY := -0.10 + 0.25*nx + 0.45*ny
			confidence := 0.9
			if rx == 2 && ry == 0 {
				// One deliberately bad local maximum. Its low confidence must keep
				// the robust weighted fit close to the eight coherent controls.
				correctionX = -2.0
				correctionY = 2.0
				confidence = 0.03
			}
			spatial.CellsEvidence = append(spatial.CellsEvidence, DiagnosticSpatialCellEvidence{
				RegionX: rx, RegionY: ry,
				LocalPhaseSyncFraction:      0.9,
				LocalPhaseSubblockAvailable: true,
				LocalPhaseSubblockOffsetX:   -correctionX,
				LocalPhaseSubblockOffsetY:   -correctionY,
				LocalPhaseConfidence:        confidence,
			})
		}
	}
	_, smooth, ok := diagnosticFitSmoothPhaseField(spatial, 1600, 1200)
	if !ok || !smooth.Available {
		t.Fatalf("confidence-weighted field unavailable: %+v", smooth)
	}
	if math.Abs(smooth.CorrectionXCoefficients[0]-0.15) > 0.12 ||
		math.Abs(smooth.CorrectionXCoefficients[1]-0.55) > 0.12 ||
		math.Abs(smooth.CorrectionXCoefficients[2]+0.20) > 0.12 {
		t.Fatalf("x coefficients chased low-confidence outlier: %+v", smooth.CorrectionXCoefficients)
	}
	if smooth.MeanControlConfidence <= 0 || smooth.MinControlConfidence > 0.05 {
		t.Fatalf("unexpected confidence summary: mean=%.3f min=%.3f", smooth.MeanControlConfidence, smooth.MinControlConfidence)
	}
}

func TestDiagnosticSmoothPhaseQuadraticRequiresCrossValidatedGain(t *testing.T) {
	spatial := &DiagnosticSpatialBitEvidence{}
	for ry := 0; ry < 3; ry++ {
		for rx := 0; rx < 3; rx++ {
			nx := 2*(float64(rx)+0.5)/3 - 1
			ny := 2*(float64(ry)+0.5)/3 - 1
			correctionX := 0.10 + 0.15*nx - 0.10*ny + 0.70*nx*ny + 0.55*nx*nx
			correctionY := -0.05 + 0.10*nx + 0.20*ny - 0.60*nx*ny + 0.50*ny*ny
			spatial.CellsEvidence = append(spatial.CellsEvidence, DiagnosticSpatialCellEvidence{
				RegionX: rx, RegionY: ry,
				LocalPhaseSyncFraction:      1,
				LocalPhaseSubblockAvailable: true,
				LocalPhaseSubblockOffsetX:   -correctionX,
				LocalPhaseSubblockOffsetY:   -correctionY,
				LocalPhaseConfidence:        1,
			})
		}
	}
	_, smooth, ok := diagnosticFitSmoothPhaseField(spatial, 1800, 1400)
	if !ok || !smooth.QuadraticAvailable {
		t.Fatalf("quadratic comparison unavailable: %+v", smooth)
	}
	if !smooth.QuadraticSelected {
		t.Fatalf("clear curved field did not win cross-validation: affine loo=%.3f quadratic loo=%.3f", smooth.AffineLeaveOneOutRMSBlocks, smooth.QuadraticLeaveOneOutRMS)
	}
	if smooth.QuadraticLeaveOneOutRMS >= smooth.AffineLeaveOneOutRMSBlocks {
		t.Fatalf("quadratic selected without LOO gain: %+v", smooth)
	}
}

func TestDiagnosticSmoothPhaseHuberRejectsHighConfidenceOutlier(t *testing.T) {
	spatial := &DiagnosticSpatialBitEvidence{}
	for ry := 0; ry < 3; ry++ {
		for rx := 0; rx < 3; rx++ {
			nx := 2*(float64(rx)+0.5)/3 - 1
			ny := 2*(float64(ry)+0.5)/3 - 1
			correctionX := 0.10 + 0.40*nx - 0.15*ny
			correctionY := -0.10 + 0.20*nx + 0.35*ny
			if rx == 2 && ry == 0 {
				correctionX = -2.2
				correctionY = 2.2
			}
			spatial.CellsEvidence = append(spatial.CellsEvidence, DiagnosticSpatialCellEvidence{
				RegionX: rx, RegionY: ry,
				LocalPhaseSyncFraction:      1,
				LocalPhaseSubblockAvailable: true,
				LocalPhaseSubblockOffsetX:   -correctionX,
				LocalPhaseSubblockOffsetY:   -correctionY,
				LocalPhaseConfidence:        0.9,
			})
		}
	}
	_, smooth, ok := diagnosticFitSmoothPhaseField(spatial, 1600, 1200)
	if !ok || !smooth.Available {
		t.Fatalf("robust field unavailable: %+v", smooth)
	}
	if smooth.RobustOutliers == 0 {
		t.Fatalf("high-confidence geometric outlier was not downweighted: %+v", smooth)
	}
	if math.Abs(smooth.CorrectionXCoefficients[1]-0.40) > 0.25 {
		t.Fatalf("robust fit chased outlier: %+v", smooth.CorrectionXCoefficients)
	}
}

func TestDiagnosticPhaseSurfacePenalizesSearchBoundary(t *testing.T) {
	key := []byte("phase-surface-boundary-key")
	spec, ok := profileSpecFor(ProfileRobust)
	if !ok {
		t.Fatal("robust profile unavailable")
	}
	pattern := newV3SyncPattern(key, spec)
	makeGrid := func(phaseX, phaseY int) []float64 {
		grid := make([]float64, eccBits)
		for _, point := range pattern.points {
			logicalX := point.tilePosition % tileWidth
			logicalY := point.tilePosition / tileWidth
			x := positiveMod(logicalX-phaseX, tileWidth)
			y := positiveMod(logicalY-phaseY, tileHeight)
			margin := -1.0
			if point.expected != 0 {
				margin = 1
			}
			grid[y*tileWidth+x] = margin
		}
		return grid
	}
	interior, ok := diagnosticBestV3PhaseSurfaceNear(makeGrid(0, 0), pattern, 0, 0, diagnosticSpatialPhaseRadius)
	if !ok || interior.atSearchBoundary {
		t.Fatalf("interior peak misclassified: %+v", interior)
	}
	boundary, ok := diagnosticBestV3PhaseSurfaceNear(makeGrid(2, 0), pattern, 0, 0, diagnosticSpatialPhaseRadius)
	if !ok || !boundary.atSearchBoundary {
		t.Fatalf("boundary peak not flagged: %+v", boundary)
	}
	if boundary.confidence >= interior.confidence {
		t.Fatalf("boundary confidence %.3f not below interior %.3f", boundary.confidence, interior.confidence)
	}
}

func TestDiagnosticBlindControlFieldAuthenticatesKnownAffineDrift(t *testing.T) {
	base := diagnosticTestImage(840, 768)
	key := []byte("blind-control-field-key")
	message := []byte("blind-control")
	marked, err := Embed(base, message, key, Options{Profile: ProfileRobust, Strength: 32})
	if err != nil {
		t.Fatal(err)
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, 920, 848))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(40, 40, 880, 808), marked, image.Point{}, draw.Src)
	boundary := PrintBoundaryEstimate{Detected: true, Confidence: 1,
		TopLeft: ImagePoint{X: 40, Y: 40}, TopRight: ImagePoint{X: 879, Y: 40},
		BottomRight: ImagePoint{X: 879, Y: 807}, BottomLeft: ImagePoint{X: 40, Y: 807}}
	h, ok := homographyForPrintBoundary(840, 768, boundary)
	if !ok {
		t.Fatal("blind control homography failed")
	}

	spatial := &DiagnosticSpatialBitEvidence{}
	for ry := 0; ry < 3; ry++ {
		for rx := 0; rx < 3; rx++ {
			spatial.CellsEvidence = append(spatial.CellsEvidence, DiagnosticSpatialCellEvidence{
				RegionX: rx, RegionY: ry,
				BlindPhaseAvailable:  true,
				BlindPhaseOffsetX:    float64(rx - 1),
				BlindPhaseOffsetY:    float64(ry - 1),
				BlindPhaseConfidence: 1,
			})
		}
	}
	warp, smooth, ok := diagnosticFitBlindSmoothPhaseField(spatial, 840, 768)
	if !ok || !smooth.Available || !smooth.Eligible || !smooth.DecodeEligible {
		t.Fatalf("blind control smooth field not accepted: %+v", smooth)
	}
	if smooth.ControlSource != "blind-self-registration" {
		t.Fatalf("control source=%q", smooth.ControlSource)
	}

	badWarp := &diagnosticResidualWarp{width: 840, height: 768, maxPixels: 16}
	badWarp.dx[1] = 12
	badWarp.dy[2] = 12
	mapper := diagnosticProjectiveMapper{h: h, warp: badWarp, smoothPhaseWarp: warp}
	grid, _, ok := diagnosticProjectiveGridWithMapperPhotometricStats(canvas, 840, 768, mapper, diagnosticPhotometricRaw)
	if !ok {
		t.Fatal("blind control corrected grid unavailable")
	}
	payload, info, _, found := newDecoder(key).decodeGrid(grid)
	if !found {
		t.Fatal("blind control field did not restore HMAC authentication")
	}
	if string(payload) != string(message) || info.Profile != ProfileRobust {
		t.Fatalf("payload=%q profile=%s", payload, info.Profile)
	}
}
