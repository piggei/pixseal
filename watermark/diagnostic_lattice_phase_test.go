package watermark

import (
	"math"
	"testing"
)

func diagnosticTestIdentityMapper() diagnosticProjectiveMapper {
	return diagnosticProjectiveMapper{h: homography{h: [9]float64{1, 0, 0, 0, 1, 0, 0, 0, 1}}}
}

func diagnosticTestSpatialCells9() []diagnosticSpatialGridCell {
	cells := make([]diagnosticSpatialGridCell, 0, 9)
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			cells = append(cells, diagnosticSpatialGridCell{RegionX: x, RegionY: y})
		}
	}
	return cells
}

func diagnosticTestPhase01(value float64) float64 {
	value = math.Mod(value, 1)
	if value < 0 {
		value += 1
	}
	return value
}

func TestDiagnosticLocalLatticeFractionalPhaseRecoversKnownModuloOffsets(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	regions := make([]LocalLatticeEstimate, 0, len(cells))
	wantX := make([]float64, len(cells))
	wantY := make([]float64, len(cells))
	for i, cell := range cells {
		nx := float64(cell.RegionX - 1)
		ny := float64(cell.RegionY - 1)
		wantX[i] = 0.18*nx + 0.07*ny
		wantY[i] = -0.05*nx + 0.16*ny
		regions = append(regions, LocalLatticeEstimate{
			RegionX: cell.RegionX, RegionY: cell.RegionY,
			NativeX: cell.RegionX * 240, NativeY: cell.RegionY * 240,
			NativeWidth: 220, NativeHeight: 220,
			U: LatticeVector{X: 8}, V: LatticeVector{Y: 8},
			PhaseU:            diagnosticTestPhase01(wantX[i]),
			PhaseV:            diagnosticTestPhase01(wantY[i]),
			PeriodicCoherence: 0.85, TileRepetitionCoherence: 0.82, Confidence: 0.80,
		})
	}
	candidate := DiagnosticScaleCandidate{CanonicalWidthPixels: 720, CanonicalHeightPixels: 720}
	got, ok := diagnosticEstimateLocalLatticeFractionalPhase(candidate, diagnosticTestIdentityMapper(), cells, regions)
	if !ok {
		t.Fatal("lattice fractional phase unavailable")
	}
	if len(got.controls) != len(cells) {
		t.Fatalf("controls=%d want=%d", len(got.controls), len(cells))
	}
	for i, control := range got.controls {
		if !control.available {
			t.Fatalf("control %d unavailable", i)
		}
		if dx := math.Abs(diagnosticWrapHalf(control.offsetX - wantX[i])); dx > 0.03 {
			t.Fatalf("control %d dx=%.4f got=%.4f want=%.4f", i, dx, control.offsetX, wantX[i])
		}
		if dy := math.Abs(diagnosticWrapHalf(control.offsetY - wantY[i])); dy > 0.03 {
			t.Fatalf("control %d dy=%.4f got=%.4f want=%.4f", i, dy, control.offsetY, wantY[i])
		}
	}
}

func TestDiagnosticBlindLatticeFusionCorrectsOnlyLowConfidenceCycleSlip(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	primary := diagnosticBlindPhaseResult{method: "intra-tile-repetition", controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{method: "local-lattice-fractional-phase", controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.8, minConfidence: 0.8}
	for i, cell := range cells {
		nx := float64(cell.RegionX - 1)
		ny := float64(cell.RegionY - 1)
		trueX := 0.20*nx + 0.08*ny
		trueY := -0.06*nx + 0.15*ny
		primary.controls[i] = diagnosticBlindCellControl{available: true, offsetX: trueX, offsetY: trueY, confidence: 0.55}
		lattice.controls[i] = diagnosticBlindCellControl{available: true, offsetX: diagnosticWrapHalf(trueX), offsetY: diagnosticWrapHalf(trueY), confidence: 0.80}
	}
	center := 4
	primary.controls[center].offsetX += 1
	primary.controls[center].confidence = 0.08
	got := diagnosticFuseBlindLatticePhase(primary, lattice, cells)
	if got.latticeCycleSlipCorrections == 0 || center >= len(got.latticeCycleAdjusted) || !got.latticeCycleAdjusted[center] {
		t.Fatalf("expected bounded cycle-slip correction; corrections=%d adjusted=%v", got.latticeCycleSlipCorrections, got.latticeCycleAdjusted)
	}

	high := primary
	high.controls = append([]diagnosticBlindCellControl(nil), primary.controls...)
	high.controls[center].confidence = 0.90
	gotHigh := diagnosticFuseBlindLatticePhase(high, lattice, cells)
	if center < len(gotHigh.latticeCycleAdjusted) && gotHigh.latticeCycleAdjusted[center] {
		t.Fatal("high-confidence primary cycle was overridden")
	}
}

func TestDiagnosticBlindLatticeFusionDoesNotUseKeyMaterial(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	primary := diagnosticBlindPhaseResult{method: "intra-tile-repetition", controls: make([]diagnosticBlindCellControl, len(cells))}
	lattice := diagnosticBlindPhaseResult{method: "local-lattice-fractional-phase", controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.7, minConfidence: 0.7}
	for i := range cells {
		primary.controls[i] = diagnosticBlindCellControl{available: true, offsetX: 0.1 * float64(cells[i].RegionX-1), offsetY: 0.1 * float64(cells[i].RegionY-1), confidence: 0.4}
		lattice.controls[i] = diagnosticBlindCellControl{available: true, offsetX: primary.controls[i].offsetX, offsetY: primary.controls[i].offsetY, confidence: 0.7}
	}
	got := diagnosticFuseBlindLatticePhase(primary, lattice, cells)
	if got.latticeConsensusCells != 9 {
		t.Fatalf("consensus cells=%d want=9", got.latticeConsensusCells)
	}
	if got.method != "intra-tile-repetition+lattice-global-unwrap-nochange" {
		t.Fatalf("method=%q", got.method)
	}
}
