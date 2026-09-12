package watermark

import (
	"math"
	"testing"
)

func TestDiagnosticCycleAnchorScoreIsGaugeInvariantAndDiscriminates(t *testing.T) {
	primary := []diagnosticBlindCellControl{
		{available: true, offsetX: -1.0, offsetY: 0.0, confidence: 0.8},
		{available: true, offsetX: 0.0, offsetY: 0.0, confidence: 0.8},
		{available: true, offsetX: 1.0, offsetY: 0.0, confidence: 0.8},
	}
	anchor := []diagnosticBlindCellControl{
		{available: true, offsetX: -1.05, offsetY: 0.02, confidence: 0.7},
		{available: true, offsetX: 0.03, offsetY: -0.01, confidence: 0.7},
		{available: true, offsetX: 1.02, offsetY: 0.00, confidence: 0.7},
	}
	bestX := []int{0, 0, 0}
	bestY := []int{0, 0, 0}
	wrongX := []int{0, 1, 0}
	wrongY := []int{0, 0, 0}
	best, bestAgree, compared, ok := diagnosticCycleAnchorScore(primary, anchor, bestX, bestY)
	if !ok || compared != 3 {
		t.Fatalf("best score unavailable: ok=%v compared=%d", ok, compared)
	}
	wrong, _, _, ok := diagnosticCycleAnchorScore(primary, anchor, wrongX, wrongY)
	if !ok || !(best < wrong) {
		t.Fatalf("independent anchor did not prefer structurally matching field: best=%g wrong=%g", best, wrong)
	}
	if bestAgree != 3 {
		t.Fatalf("expected all rounded cycles to agree, got %d/3", bestAgree)
	}

	// Add a common +2/-1 gauge to the candidate. The objective must remain
	// unchanged after integer gauge removal.
	shiftedPrimary := append([]diagnosticBlindCellControl(nil), primary...)
	for i := range shiftedPrimary {
		shiftedPrimary[i].offsetX += 2
		shiftedPrimary[i].offsetY -= 1
	}
	shifted, shiftedAgree, _, ok := diagnosticCycleAnchorScore(shiftedPrimary, anchor, bestX, bestY)
	if !ok || math.Abs(shifted-best) > 1e-12 {
		t.Fatalf("gauge changed anchor objective: base=%g shifted=%g", best, shifted)
	}
	if shiftedAgree != bestAgree {
		t.Fatalf("gauge changed cycle agreement: base=%d shifted=%d", bestAgree, shiftedAgree)
	}
}
