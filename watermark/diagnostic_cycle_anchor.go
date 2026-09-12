package watermark

import "math"

type diagnosticCycleAnchorResult struct {
	method                  string
	available               bool
	pairs                   int
	cells                   int
	meanConfidence          float64
	top1Objective           float64
	secondObjective         float64
	deltaSecondMinusTop1    float64
	prefersTop1             bool
	top1AgreementCells      int
	top1AgreementFraction   float64
	secondAgreementCells    int
	secondAgreementFraction float64
	anchor                  diagnosticBlindPhaseResult
	top1X                   []int
	top1Y                   []int
	secondX                 []int
	secondY                 []int
}

// diagnosticIndependentCycleAnchor is the build20 diagnostic-only experiment.
// It uses the unguided cross-cell image-domain registration observer as an
// independent absolute-relative cycle anchor. Unlike repetition proposal,
// cross-fit and partition stability, this observer never uses coded-bit
// repetition groups. It sees only normalized per-cell image-domain margin
// grids. The returned preference is telemetry only and cannot alter sampling,
// HMAC candidates or production decoding.
func diagnosticIndependentCycleAnchor(spatial *DiagnosticSpatialBitEvidence, cells []diagnosticSpatialGridCell) diagnosticCycleAnchorResult {
	result := diagnosticCycleAnchorResult{method: "cross-cell-pairwise-cycle-anchor"}
	if spatial == nil || spatial.BlindGlobalUnwrapStatus != "ambiguous" || !spatial.BlindGlobalUnwrapSecondAvailable || len(cells) < 3 {
		return result
	}
	anchor, ok := diagnosticEstimateBlindSpatialPhasePairwise(cells)
	if !ok || len(anchor.controls) != len(cells) {
		return result
	}
	primary, lattice, ok := diagnosticControlsFromSpatialEvidence(spatial, cells)
	if !ok {
		return result
	}
	x := diagnosticSolveDiscreteUnwrapAxis(primary, lattice, cells, true)
	y := diagnosticSolveDiscreteUnwrapAxis(primary, lattice, cells, false)
	if !isFiniteObjective(x.bestObjective) || !isFiniteObjective(y.bestObjective) {
		return result
	}
	bestX := append([]int(nil), x.shifts...)
	bestY := append([]int(nil), y.shifts...)
	secondObjective := math.Inf(1)
	secondX := append([]int(nil), bestX...)
	secondY := append([]int(nil), bestY...)
	if x.secondAvailable && isFiniteObjective(y.bestObjective) {
		secondObjective = x.secondObjective + y.bestObjective
		secondX = append([]int(nil), x.secondShifts...)
	}
	if y.secondAvailable && isFiniteObjective(x.bestObjective) {
		candidate := x.bestObjective + y.secondObjective
		if candidate < secondObjective || (math.Abs(candidate-secondObjective) <= 1e-12 && diagnosticDiscreteShiftLexLess(y.secondShifts, secondY)) {
			secondObjective = candidate
			secondX = append([]int(nil), x.shifts...)
			secondY = append([]int(nil), y.secondShifts...)
		}
	}
	if !isFiniteObjective(secondObjective) {
		return result
	}

	top1Score, top1Agree, compared1, ok1 := diagnosticCycleAnchorScore(primary, anchor.controls, bestX, bestY)
	secondScore, secondAgree, compared2, ok2 := diagnosticCycleAnchorScore(primary, anchor.controls, secondX, secondY)
	if !ok1 || !ok2 || compared1 == 0 || compared1 != compared2 {
		return result
	}
	result.available = true
	result.pairs = anchor.pairs
	result.cells = compared1
	result.meanConfidence = anchor.meanConfidence
	result.top1Objective = top1Score
	result.secondObjective = secondScore
	result.deltaSecondMinusTop1 = secondScore - top1Score
	result.prefersTop1 = result.deltaSecondMinusTop1 > 0
	result.top1AgreementCells = top1Agree
	result.secondAgreementCells = secondAgree
	result.top1AgreementFraction = float64(top1Agree) / float64(compared1)
	result.secondAgreementFraction = float64(secondAgree) / float64(compared1)
	result.anchor = anchor
	result.top1X, result.top1Y = bestX, bestY
	result.secondX, result.secondY = secondX, secondY
	return result
}

func diagnosticControlsFromSpatialEvidence(spatial *DiagnosticSpatialBitEvidence, cells []diagnosticSpatialGridCell) ([]diagnosticBlindCellControl, []diagnosticBlindCellControl, bool) {
	primary := make([]diagnosticBlindCellControl, len(cells))
	lattice := make([]diagnosticBlindCellControl, len(cells))
	found := 0
	for i, cell := range cells {
		for _, evidence := range spatial.CellsEvidence {
			if evidence.RegionX != cell.RegionX || evidence.RegionY != cell.RegionY {
				continue
			}
			if evidence.BlindPhaseAvailable {
				primary[i] = diagnosticBlindCellControl{available: true, offsetX: evidence.BlindPhaseOffsetX, offsetY: evidence.BlindPhaseOffsetY, confidence: evidence.BlindPhaseConfidence, pairs: evidence.BlindPhasePairSupport, residual: evidence.BlindPhaseResidualBlocks}
			}
			if evidence.BlindLatticePhaseAvailable {
				lattice[i] = diagnosticBlindCellControl{available: true, offsetX: evidence.BlindLatticePhaseOffsetX, offsetY: evidence.BlindLatticePhaseOffsetY, confidence: evidence.BlindLatticePhaseConfidence}
			}
			if primary[i].available && lattice[i].available {
				found++
			}
			break
		}
	}
	return primary, lattice, found >= 3
}

// diagnosticCycleAnchorScore compares a candidate integer-cycle field with the
// independent pairwise observer after removing the best common x/y translation.
// This makes the comparison invariant to the arbitrary zero-mean/global-cycle
// gauge while preserving relative integer-cycle structure.
func diagnosticCycleAnchorScore(primary, anchor []diagnosticBlindCellControl, shiftsX, shiftsY []int) (float64, int, int, bool) {
	if len(primary) == 0 || len(primary) != len(anchor) {
		return 0, 0, 0, false
	}
	meanDX, meanDY, weightSum := 0.0, 0.0, 0.0
	for i := range primary {
		if !primary[i].available || !anchor[i].available {
			continue
		}
		sx, sy := 0, 0
		if i < len(shiftsX) {
			sx = shiftsX[i]
		}
		if i < len(shiftsY) {
			sy = shiftsY[i]
		}
		w := math.Max(math.Min(primary[i].confidence, anchor[i].confidence), 0.01)
		meanDX += w * ((primary[i].offsetX + float64(sx)) - anchor[i].offsetX)
		meanDY += w * ((primary[i].offsetY + float64(sy)) - anchor[i].offsetY)
		weightSum += w
	}
	if weightSum <= 0 {
		return 0, 0, 0, false
	}
	meanDX /= weightSum
	meanDY /= weightSum
	gaugeX := int(math.Round(meanDX))
	gaugeY := int(math.Round(meanDY))

	score, norm := 0.0, 0.0
	agreement, compared := 0, 0
	for i := range primary {
		if !primary[i].available || !anchor[i].available {
			continue
		}
		sx, sy := 0, 0
		if i < len(shiftsX) {
			sx = shiftsX[i]
		}
		if i < len(shiftsY) {
			sy = shiftsY[i]
		}
		candidateX := primary[i].offsetX + float64(sx)
		candidateY := primary[i].offsetY + float64(sy)
		dx := candidateX - anchor[i].offsetX - float64(gaugeX)
		dy := candidateY - anchor[i].offsetY - float64(gaugeY)
		w := math.Max(math.Min(primary[i].confidence, anchor[i].confidence), 0.01)
		score += w * (dx*dx + dy*dy)
		norm += w
		candidateCycleX := int(math.Round(candidateX)) - gaugeX
		candidateCycleY := int(math.Round(candidateY)) - gaugeY
		anchorCycleX := int(math.Round(anchor[i].offsetX))
		anchorCycleY := int(math.Round(anchor[i].offsetY))
		if candidateCycleX == anchorCycleX && candidateCycleY == anchorCycleY {
			agreement++
		}
		compared++
	}
	if norm <= 0 || compared == 0 {
		return 0, 0, 0, false
	}
	return score / norm, agreement, compared, true
}
