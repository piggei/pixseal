package watermark

import (
	"encoding/json"
	"image"
	"image/draw"
	"math"
	"testing"
)

func diagnosticTestGlobalUnwrapControls() ([]diagnosticSpatialGridCell, diagnosticBlindPhaseResult, diagnosticBlindPhaseResult) {
	cells := diagnosticTestSpatialCells9()
	primary := diagnosticBlindPhaseResult{method: "intra-tile-repetition", controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{method: "local-lattice-fractional-phase", controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.82, minConfidence: 0.82}
	for i, cell := range cells {
		nx := float64(cell.RegionX - 1)
		ny := float64(cell.RegionY - 1)
		trueX := 0.82*nx + 0.18*ny
		trueY := -0.15*nx + 0.76*ny
		primary.controls[i] = diagnosticBlindCellControl{available: true, offsetX: trueX, offsetY: trueY, confidence: 0.48}
		lattice.controls[i] = diagnosticBlindCellControl{available: true, offsetX: diagnosticWrapHalf(trueX), offsetY: diagnosticWrapHalf(trueY), confidence: 0.82}
	}
	// Two low-confidence integer-cycle mistakes. The fractional lattice phase
	// remains correct because +/-1 block is invisible modulo one block.
	primary.controls[0].offsetX += 1
	primary.controls[0].confidence = 0.08
	primary.controls[8].offsetY -= 1
	primary.controls[8].confidence = 0.07
	return cells, primary, lattice
}

func TestDiagnosticGlobalDiscreteUnwrapRecoversSmoothKnownCycleSlips(t *testing.T) {
	cells, primary, lattice := diagnosticTestGlobalUnwrapControls()
	xr := diagnosticSolveDiscreteUnwrapAxis(primary.controls, lattice.controls, cells, true)
	yr := diagnosticSolveDiscreteUnwrapAxis(primary.controls, lattice.controls, cells, false)
	t.Logf("x accepted=%v amb=%v base=%.5f best=%.5f second=%.5f imp=%.5f margin=%.5f shifts=%v", xr.accepted, xr.ambiguous, xr.baselineObjective, xr.bestObjective, xr.secondObjective, xr.improvement, xr.margin, xr.shifts)
	t.Logf("y accepted=%v amb=%v base=%.5f best=%.5f second=%.5f imp=%.5f margin=%.5f shifts=%v", yr.accepted, yr.ambiguous, yr.baselineObjective, yr.bestObjective, yr.secondObjective, yr.improvement, yr.margin, yr.shifts)
	got, ok := diagnosticGlobalDiscreteUnwrap(primary, lattice, cells)
	if !ok {
		t.Fatalf("global unwrap rejected: states=%d baseline=%.4f best=%.4f margin=%.4f ambiguous=%v", got.unwrapEvaluatedStates, got.unwrapBaselineObjective, got.unwrapObjective, got.unwrapMargin, got.unwrapAmbiguous)
	}
	if got.unwrapChangedCells < 2 || got.unwrapAcceptedAxes == 0 {
		t.Fatalf("changed=%d acceptedAxes=%d shiftsX=%v shiftsY=%v", got.unwrapChangedCells, got.unwrapAcceptedAxes, got.unwrapShiftX, got.unwrapShiftY)
	}
	if got.unwrapShiftX[0] != -1 {
		t.Fatalf("x slip shift=%d want=-1; all=%v", got.unwrapShiftX[0], got.unwrapShiftX)
	}
	if got.unwrapShiftY[8] != 1 {
		t.Fatalf("y slip shift=%d want=+1; all=%v", got.unwrapShiftY[8], got.unwrapShiftY)
	}
	if got.unwrapEvaluatedStates > 2*diagnosticDiscreteUnwrapMaxAssignmentsPerAxis {
		t.Fatalf("evaluated states=%d exceeds declared budget", got.unwrapEvaluatedStates)
	}
}

func TestDiagnosticGlobalDiscreteUnwrapLocksHighConfidencePrimary(t *testing.T) {
	cells, primary, lattice := diagnosticTestGlobalUnwrapControls()
	primary.controls[0].confidence = 0.90
	got, _ := diagnosticGlobalDiscreteUnwrap(primary, lattice, cells)
	if len(got.unwrapShiftX) > 0 && got.unwrapShiftX[0] != 0 {
		t.Fatalf("high-confidence control changed by %d", got.unwrapShiftX[0])
	}
}

func TestDiagnosticGlobalDiscreteUnwrapRejectsAmbiguousSymmetricChoice(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	primary := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.8, minConfidence: 0.8}
	for i := range cells {
		primary.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.12}
		lattice.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.8}
	}
	// Two symmetric alternatives: moving either outer control can reduce the
	// same non-affine kink. A safe solver must not pretend that one is unique.
	primary.controls[3].offsetX = 0.52
	primary.controls[5].offsetX = -0.52
	got, ok := diagnosticGlobalDiscreteUnwrap(primary, lattice, cells)
	if ok && !got.unwrapAmbiguous {
		t.Fatalf("ambiguous layout was accepted: shiftsX=%v objective=%.4f margin=%.4f", got.unwrapShiftX, got.unwrapObjective, got.unwrapMargin)
	}
}

func TestDiagnosticGlobalDiscreteUnwrapOffsetsRemainModuloLatticeConsistent(t *testing.T) {
	cells, primary, lattice := diagnosticTestGlobalUnwrapControls()
	got, ok := diagnosticGlobalDiscreteUnwrap(primary, lattice, cells)
	if !ok {
		t.Fatal("global unwrap unavailable")
	}
	for i, control := range got.controls {
		if !control.available || !lattice.controls[i].available {
			continue
		}
		if math.Abs(diagnosticWrapHalf(control.offsetX-lattice.controls[i].offsetX)) > diagnosticDiscreteUnwrapMaxFractionDistance+1e-9 ||
			math.Abs(diagnosticWrapHalf(control.offsetY-lattice.controls[i].offsetY)) > diagnosticDiscreteUnwrapMaxFractionDistance+1e-9 {
			t.Fatalf("control %d lost modulo-lattice agreement", i)
		}
	}
}

func TestDiagnosticGlobalDiscreteUnwrapRejectedProposalRollsBackFractionalPhase(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	primary := diagnosticBlindPhaseResult{method: "intra-tile-repetition", controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{method: "local-lattice-fractional-phase", controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.8, minConfidence: 0.8}
	for i := range cells {
		primary.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.12}
		lattice.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.8}
	}
	primary.controls[3].offsetX = 0.52
	primary.controls[5].offsetX = -0.52
	before := append([]diagnosticBlindCellControl(nil), primary.controls...)
	got := diagnosticFuseBlindLatticePhase(primary, lattice, cells)
	if got.unwrapChangedCells != 0 || got.latticeCycleSlipCorrections != 0 {
		t.Fatalf("ambiguous proposal leaked changes: changed=%d legacy=%d", got.unwrapChangedCells, got.latticeCycleSlipCorrections)
	}
	for i := range before {
		if math.Abs(got.controls[i].offsetX-before[i].offsetX) > 1e-12 || math.Abs(got.controls[i].offsetY-before[i].offsetY) > 1e-12 {
			t.Fatalf("control %d changed despite rollback: got=(%.4f,%.4f) before=(%.4f,%.4f)", i, got.controls[i].offsetX, got.controls[i].offsetY, before[i].offsetX, before[i].offsetY)
		}
	}
}

func TestDiagnosticGlobalDiscreteUnwrapFieldRestoresHMAC(t *testing.T) {
	base := diagnosticTestImage(840, 768)
	key := []byte("global-unwrap-field-key")
	message := []byte("global-unwrap")
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
		t.Fatal("global unwrap homography failed")
	}

	cells := diagnosticTestSpatialCells9()
	primary := diagnosticBlindPhaseResult{method: "intra-tile-repetition", controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{method: "local-lattice-fractional-phase", controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.85, minConfidence: 0.85}
	for i, cell := range cells {
		trueX := float64(cell.RegionX - 1)
		trueY := float64(cell.RegionY - 1)
		primary.controls[i] = diagnosticBlindCellControl{available: true, offsetX: trueX, offsetY: trueY, confidence: 0.48}
		lattice.controls[i] = diagnosticBlindCellControl{available: true, offsetX: 0, offsetY: 0, confidence: 0.85}
	}
	primary.controls[0].offsetX += 1
	primary.controls[0].confidence = 0.07
	primary.controls[8].offsetY -= 1
	primary.controls[8].confidence = 0.07
	unwrapped, ok := diagnosticGlobalDiscreteUnwrap(primary, lattice, cells)
	if !ok || unwrapped.unwrapAmbiguous {
		t.Fatalf("known cycle slips not unwrapped: ok=%v ambiguous=%v shiftsX=%v shiftsY=%v", ok, unwrapped.unwrapAmbiguous, unwrapped.unwrapShiftX, unwrapped.unwrapShiftY)
	}

	spatial := &DiagnosticSpatialBitEvidence{}
	for i, cell := range cells {
		spatial.CellsEvidence = append(spatial.CellsEvidence, DiagnosticSpatialCellEvidence{
			RegionX: cell.RegionX, RegionY: cell.RegionY,
			BlindPhaseAvailable:  true,
			BlindPhaseOffsetX:    unwrapped.controls[i].offsetX,
			BlindPhaseOffsetY:    unwrapped.controls[i].offsetY,
			BlindPhaseConfidence: 1,
		})
	}
	warp, smooth, ok := diagnosticFitBlindSmoothPhaseField(spatial, 840, 768)
	if !ok || !smooth.DecodeEligible {
		t.Fatalf("unwrapped field not decode-eligible: %+v", smooth)
	}
	badWarp := &diagnosticResidualWarp{width: 840, height: 768, maxPixels: 16}
	badWarp.dx[1] = 12
	badWarp.dy[2] = 12
	mapper := diagnosticProjectiveMapper{h: h, warp: badWarp, smoothPhaseWarp: warp}
	grid, _, ok := diagnosticProjectiveGridWithMapperPhotometricStats(canvas, 840, 768, mapper, diagnosticPhotometricRaw)
	if !ok {
		t.Fatal("global unwrap corrected grid unavailable")
	}
	payload, info, _, found := newDecoder(key).decodeGrid(grid)
	if !found {
		t.Fatal("global discrete unwrap field did not restore HMAC authentication")
	}
	if string(payload) != string(message) || info.Profile != ProfileRobust {
		t.Fatalf("payload=%q profile=%s", payload, info.Profile)
	}
}

func TestDiagnosticGlobalDiscreteUnwrapExactTop2RejectsFormerBeamFalseAccept(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	primary := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.80, minConfidence: 0.80}
	// Deterministic build15 counterexample. The former width-64 beam accepted
	// X with best/second 0.027408951002 / 0.062494434795. Complete enumeration
	// finds the same optimum but a missed runner-up at 0.035129429298, making
	// the assignment ambiguous under the fixed margin gate.
	values := []struct {
		x, y, confidence float64
	}{
		{0.565770166273271, 0.986795330305221, 0.378418283561948},
		{0.353193575603702, -0.689690731410825, 0.438723262035439},
		{-0.098574355815257, 0.491686834114028, 0.234844300703030},
		{0.140941323661953, 0.069157454349167, 0.426940330403423},
		{-0.193327792460868, -0.273414770019478, 0.310600032387832},
		{0.651059916756997, 0.276482152392212, 0.447843811864446},
		{0.927318484491350, -0.256576361987041, 0.115599121926027},
		{-0.237891476888321, -0.031012479358238, 0.160467034653154},
		{0.656729986540776, 0.045653707979348, 0.149209482193893},
	}
	for i, v := range values {
		primary.controls[i] = diagnosticBlindCellControl{available: true, offsetX: v.x, offsetY: v.y, confidence: v.confidence}
		lattice.controls[i] = diagnosticBlindCellControl{available: true, offsetX: diagnosticWrapHalf(v.x), offsetY: diagnosticWrapHalf(v.y), confidence: 0.80}
	}

	x := diagnosticSolveDiscreteUnwrapAxis(primary.controls, lattice.controls, cells, true)
	if x.accepted || !x.ambiguous || x.status != "ambiguous" {
		t.Fatalf("exact top-2 must reject former beam false accept: status=%s best=%.12f second=%.12f", x.status, x.bestObjective, x.secondObjective)
	}
	if math.Abs(x.baselineObjective-0.254935832754) > 1e-9 ||
		math.Abs(x.bestObjective-0.027408951002) > 1e-9 ||
		math.Abs(x.secondObjective-0.035129429298) > 1e-9 {
		t.Fatalf("unexpected exact X objectives: base=%.12f best=%.12f second=%.12f", x.baselineObjective, x.bestObjective, x.secondObjective)
	}
	got, ok := diagnosticGlobalDiscreteUnwrap(primary, lattice, cells)
	if ok || got.unwrapChangedCells != 0 || got.unwrapStatus != "ambiguous" {
		t.Fatalf("former false accept escaped exact ambiguity gate: ok=%v status=%s changed=%d", ok, got.unwrapStatus, got.unwrapChangedCells)
	}
}

func TestDiagnosticGlobalDiscreteUnwrapInsufficientCoverageIsNotApplicable(t *testing.T) {
	cells := diagnosticTestSpatialCells9()[:4]
	primary := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells))}
	for i := range cells {
		primary.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.4}
		lattice.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.8}
	}
	got, ok := diagnosticGlobalDiscreteUnwrap(primary, lattice, cells)
	if ok || got.unwrapStatus != "not-applicable" || got.unwrapAmbiguous {
		t.Fatalf("insufficient coverage should be a safe non-applicable case: ok=%v status=%s ambiguous=%v", ok, got.unwrapStatus, got.unwrapAmbiguous)
	}
}

func TestDiagnosticGlobalDiscreteUnwrapPartialLatticeIsNotApplicable(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	primary := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, 4)}
	for i := range primary.controls {
		primary.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.4}
	}
	for i := range lattice.controls {
		lattice.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.8}
	}
	got, ok := diagnosticGlobalDiscreteUnwrap(primary, lattice, cells)
	if ok || got.unwrapStatus != "not-applicable" || got.unwrapStatusX != "not-applicable" || got.unwrapStatusY != "not-applicable" {
		t.Fatalf("partial lattice should be a safe non-applicable case: ok=%v status=%s x=%s y=%s", ok, got.unwrapStatus, got.unwrapStatusX, got.unwrapStatusY)
	}
	if got.unwrapAmbiguous || got.unwrapChangedCells != 0 {
		t.Fatalf("partial lattice must not be mislabeled ambiguous or alter controls: ambiguous=%v changed=%d", got.unwrapAmbiguous, got.unwrapChangedCells)
	}
}

func TestDiagnosticGlobalDiscreteUnwrapMixedAxisAmbiguityRollsBackAtomically(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	primary := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.80, minConfidence: 0.80}
	xValues := []struct {
		x, confidence float64
	}{
		{0.565770166273271, 0.378418283561948},
		{0.353193575603702, 0.438723262035439},
		{-0.098574355815257, 0.234844300703030},
		{0.140941323661953, 0.426940330403423},
		{-0.193327792460868, 0.310600032387832},
		{0.651059916756997, 0.447843811864446},
		{0.927318484491350, 0.115599121926027},
		{-0.237891476888321, 0.160467034653154},
		{0.656729986540776, 0.149209482193893},
	}
	for i, cell := range cells {
		y := float64(cell.RegionY - 1)
		confidence := xValues[i].confidence
		if i == 8 {
			y -= 1
			confidence = 0.07
		}
		primary.controls[i] = diagnosticBlindCellControl{available: true, offsetX: xValues[i].x, offsetY: y, confidence: confidence}
		lattice.controls[i] = diagnosticBlindCellControl{available: true, offsetX: diagnosticWrapHalf(xValues[i].x), offsetY: 0, confidence: 0.80}
	}
	x := diagnosticSolveDiscreteUnwrapAxis(primary.controls, lattice.controls, cells, true)
	y := diagnosticSolveDiscreteUnwrapAxis(primary.controls, lattice.controls, cells, false)
	if !x.ambiguous || !y.accepted {
		t.Fatalf("test vector must exercise mixed ambiguity: x=%s y=%s", x.status, y.status)
	}
	got, ok := diagnosticGlobalDiscreteUnwrap(primary, lattice, cells)
	if ok || got.unwrapStatus != "ambiguous" || !got.unwrapAmbiguous {
		t.Fatalf("mixed accepted/ambiguous axes must be globally ambiguous: ok=%v status=%s ambiguous=%v", ok, got.unwrapStatus, got.unwrapAmbiguous)
	}
	if got.unwrapChangedCells != 0 || got.unwrapAcceptedAxes != 0 {
		t.Fatalf("mixed ambiguity must roll back atomically: changed=%d acceptedAxes=%d", got.unwrapChangedCells, got.unwrapAcceptedAxes)
	}
	if got.unwrapStatusY != "accepted" {
		t.Fatalf("per-axis evidence should remain visible, y=%s", got.unwrapStatusY)
	}
}

func TestDiagnosticGlobalDiscreteUnwrapNoEligibleIsJSONSafe(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	primary := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.8, minConfidence: 0.8}
	for i := range cells {
		primary.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.90}
		lattice.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.80}
	}
	got, ok := diagnosticGlobalDiscreteUnwrap(primary, lattice, cells)
	if ok {
		t.Fatal("no-eligible case unexpectedly produced a correction")
	}
	if got.unwrapSecondAvailable {
		t.Fatal("no-eligible case unexpectedly reports a second solution")
	}
	if !isFiniteObjective(got.unwrapBaselineObjective) || !isFiniteObjective(got.unwrapObjective) || !isFiniteObjective(got.unwrapSecondObjective) || !isFiniteObjective(got.unwrapMargin) {
		t.Fatalf("non-finite no-eligible diagnostics: base=%v best=%v second=%v margin=%v", got.unwrapBaselineObjective, got.unwrapObjective, got.unwrapSecondObjective, got.unwrapMargin)
	}
	evidence := DiagnosticSpatialBitEvidence{
		BlindGlobalUnwrapMethod:            got.unwrapMethod,
		BlindGlobalUnwrapStatus:            got.unwrapStatus,
		BlindGlobalUnwrapSecondObjective:   got.unwrapSecondObjective,
		BlindGlobalUnwrapSecondAvailable:   got.unwrapSecondAvailable,
		BlindGlobalUnwrapBaselineObjective: got.unwrapBaselineObjective,
		BlindGlobalUnwrapObjective:         got.unwrapObjective,
		BlindGlobalUnwrapMargin:            got.unwrapMargin,
	}
	if _, err := json.Marshal(evidence); err != nil {
		t.Fatalf("no-eligible diagnostics are not JSON serializable: %v", err)
	}
}

func TestDiagnosticGlobalUnwrapSplitRepetitionSupportsCorrectPhase(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	spec, ok := profileSpecFor(ProfileRobust)
	if !ok {
		t.Fatal("robust profile unavailable")
	}
	for i := range cells {
		cells[i].grid = make([]float64, eccBits)
		for pos := 0; pos < eccBits; pos++ {
			if v3CodeIndex(pos, spec.codedBits)%2 == 0 {
				cells[i].grid[pos] = 1
			} else {
				cells[i].grid[pos] = -1
			}
		}
	}
	primary := diagnosticBlindPhaseResult{
		profile:  ProfileRobust,
		controls: make([]diagnosticBlindCellControl, len(cells)),
	}
	for i := range primary.controls {
		primary.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.5}
	}
	bestX, bestY := make([]int, len(cells)), make([]int, len(cells))
	secondX, secondY := make([]int, len(cells)), make([]int, len(cells))
	secondX[4] = 1
	got := diagnosticValidateGlobalUnwrapSplitRepetition(primary, cells, bestX, bestY, secondX, secondY)
	if !got.available || got.cells != 1 {
		t.Fatalf("validation unavailable: %+v", got)
	}
	if !got.supportsBest || got.fold0Delta <= 0 || got.fold1Delta <= 0 {
		t.Fatalf("split repetition did not support the known phase: fold0=%f fold1=%f", got.fold0Delta, got.fold1Delta)
	}
	if got.pairsFold0 == 0 || got.pairsFold1 == 0 || got.pairsFold0+got.pairsFold1 != diagnosticBlindRepetitionPairCount(ProfileRobust) {
		t.Fatalf("invalid deterministic pair partition: %d + %d", got.pairsFold0, got.pairsFold1)
	}
}
