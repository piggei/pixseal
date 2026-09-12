package watermark

import "math"

const diagnosticUnwrapCrossfitFolds = 2

type diagnosticUnwrapCrossfitDirection struct {
	available       bool
	proposalFold    int
	validationFold  int
	profile         Profile
	proposalPairs   int
	validationPairs int
	cells           int
	evaluatedStates int
	margin          float64
	delta           float64
	supportsBest    bool
	primary         diagnosticBlindPhaseResult
	bestX           []int
	bestY           []int
}

type diagnosticUnwrapCrossfitResult struct {
	method            string
	available         bool
	aToB              diagnosticUnwrapCrossfitDirection
	bToA              diagnosticUnwrapCrossfitDirection
	comparedCells     int
	agreementCells    int
	agreementFraction float64
	supportsBest      bool
}

// diagnosticGlobalUnwrapCrossfit is the build17 held-out repetition experiment.
// Each direction estimates the repetition controls from one deterministic
// coded-bit-group fold only, combines those proposal controls with the
// key-independent fractional lattice observer, ranks the bounded integer cycle
// space exactly, then evaluates the exact top-1/top-2 distinction using only
// the other repetition fold. The folds are disjoint by coded-bit group, so a
// Format-v3 repetition constraint can participate in proposal or validation,
// never both. This remains research telemetry and does not alter the production
// decode/HMAC candidate budget.
func diagnosticGlobalUnwrapCrossfit(cells []diagnosticSpatialGridCell, aggregate []float64, lattice diagnosticBlindPhaseResult) diagnosticUnwrapCrossfitResult {
	result := diagnosticUnwrapCrossfitResult{method: "crossfit-repetition-top2"}
	result.aToB = diagnosticGlobalUnwrapCrossfitDirection(cells, aggregate, lattice, 0, 1)
	result.bToA = diagnosticGlobalUnwrapCrossfitDirection(cells, aggregate, lattice, 1, 0)
	if !result.aToB.available || !result.bToA.available {
		return result
	}
	for i := range cells {
		ax, ay, aok := diagnosticCrossfitBestLocalCycle(result.aToB, i)
		bx, by, bok := diagnosticCrossfitBestLocalCycle(result.bToA, i)
		if !aok || !bok {
			continue
		}
		result.comparedCells++
		if ax == bx && ay == by {
			result.agreementCells++
		}
	}
	if result.comparedCells > 0 {
		result.agreementFraction = float64(result.agreementCells) / float64(result.comparedCells)
	}
	result.available = true
	// Deliberately strict diagnostic definition: both independently proposed
	// candidates must be supported by the opposite held-out fold and must agree
	// on every comparable absolute local phase. This flag is not an unwrap gate.
	result.supportsBest = result.aToB.supportsBest && result.bToA.supportsBest && result.comparedCells >= 3 && result.agreementCells == result.comparedCells
	return result
}

func diagnosticGlobalUnwrapCrossfitDirection(cells []diagnosticSpatialGridCell, aggregate []float64, lattice diagnosticBlindPhaseResult, proposalFold, validationFold int) diagnosticUnwrapCrossfitDirection {
	result := diagnosticUnwrapCrossfitDirection{proposalFold: proposalFold, validationFold: validationFold}
	primary, ok := diagnosticEstimateBlindRepetitionPhaseCrossfitFold(cells, aggregate, proposalFold)
	if !ok || len(primary.controls) != len(cells) || len(lattice.controls) != len(cells) {
		return result
	}
	prepared, ok := diagnosticPrepareCrossfitLatticeProposal(primary, lattice)
	if !ok {
		return result
	}
	x := diagnosticSolveDiscreteUnwrapAxis(prepared.controls, lattice.controls, cells, true)
	y := diagnosticSolveDiscreteUnwrapAxis(prepared.controls, lattice.controls, cells, false)
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
	validationPairs := diagnosticCrossfitRepetitionPairs(primary.profile, validationFold)
	if len(validationPairs) == 0 {
		return result
	}
	cellsUsed, delta, ok := diagnosticValidateCrossfitTop2(primary, cells, bestX, bestY, secondX, secondY, validationPairs)
	if !ok {
		return result
	}
	result.available = true
	result.profile = primary.profile
	result.proposalPairs = primary.pairs
	result.validationPairs = len(validationPairs)
	result.cells = cellsUsed
	result.evaluatedStates = x.evaluatedStates + y.evaluatedStates
	result.margin = secondObjective - (x.bestObjective + y.bestObjective)
	result.delta = delta
	result.supportsBest = delta > 0
	result.primary = primary
	result.bestX = bestX
	result.bestY = bestY
	return result
}

func diagnosticEstimateBlindRepetitionPhaseCrossfitFold(cells []diagnosticSpatialGridCell, aggregate []float64, fold int) (diagnosticBlindPhaseResult, bool) {
	result := diagnosticBlindPhaseResult{method: "intra-tile-repetition-crossfit"}
	if fold < 0 || fold >= diagnosticUnwrapCrossfitFolds || len(cells) < 3 || len(aggregate) < eccBits {
		return result, false
	}
	global := diagnosticBlindRepetitionPeak{peak: math.Inf(-1)}
	var proposalPairs [][2]int
	for _, profile := range []Profile{ProfileRobust, ProfileBalanced} {
		pairs := diagnosticCrossfitRepetitionPairs(profile, fold)
		peak, ok := diagnosticBlindBestRepetitionFull(aggregate, profile, pairs)
		if ok && peak.peak > global.peak {
			global = peak
			proposalPairs = pairs
		}
	}
	if math.IsInf(global.peak, -1) || global.peak < 0.05 || len(proposalPairs) == 0 {
		return result, false
	}
	result.profile = global.profile
	result.globalPhaseX = global.phaseX
	result.globalPhaseY = global.phaseY
	result.globalScore = global.peak
	result.pairs = len(proposalPairs)
	result.meanPairPeak = global.peak
	result.controls = make([]diagnosticBlindCellControl, len(cells))
	result.minConfidence = 1
	available := 0
	for i, cell := range cells {
		peak, ok := diagnosticBlindBestRepetitionNearPairs(cell.grid, global.profile, global.phaseX, global.phaseY, diagnosticSpatialPhaseRadius, proposalPairs)
		if !ok {
			continue
		}
		control := diagnosticBlindCellControl{available: true, offsetX: peak.offsetX, offsetY: peak.offsetY, confidence: peak.confidence, pairs: len(proposalPairs)}
		result.controls[i] = control
		available++
		result.meanConfidence += control.confidence
		if control.confidence < result.minConfidence {
			result.minConfidence = control.confidence
		}
	}
	if available < 3 {
		return diagnosticBlindPhaseResult{}, false
	}
	result.meanConfidence /= float64(available)
	diagnosticRecenterBlindControls(&result)
	return result, true
}

func diagnosticBlindBestRepetitionNearPairs(grid []float64, profile Profile, referenceX, referenceY, radius int, pairs [][2]int) (diagnosticBlindRepetitionPeak, bool) {
	result := diagnosticBlindRepetitionPeak{profile: profile, peak: math.Inf(-1), second: math.Inf(-1)}
	if len(grid) < eccBits || radius < 0 || len(pairs) == 0 {
		return result, false
	}
	scores := make(map[[2]int]float64, (2*radius+1)*(2*radius+1))
	bestDX, bestDY := 0, 0
	bestDistance := math.Inf(1)
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			phaseX := positiveMod(referenceX+dx, tileWidth)
			phaseY := positiveMod(referenceY+dy, tileHeight)
			score := diagnosticBlindRepetitionScorePairs(grid, phaseX, phaseY, pairs)
			scores[[2]int{dx, dy}] = score
			distance := math.Hypot(float64(dx), float64(dy))
			if score > result.peak || (score == result.peak && distance < bestDistance) {
				result.second = result.peak
				result.peak = score
				result.phaseX = phaseX
				result.phaseY = phaseY
				bestDX, bestDY = dx, dy
				bestDistance = distance
			} else if score > result.second {
				result.second = score
			}
		}
	}
	if math.IsInf(result.peak, -1) {
		return result, false
	}
	center := scores[[2]int{bestDX, bestDY}]
	fracX, curvatureX := 0.0, 0.0
	if bestDX > -radius && bestDX < radius {
		fracX, curvatureX = diagnosticParabolicPeakOffset(scores[[2]int{bestDX - 1, bestDY}], center, scores[[2]int{bestDX + 1, bestDY}])
	}
	fracY, curvatureY := 0.0, 0.0
	if bestDY > -radius && bestDY < radius {
		fracY, curvatureY = diagnosticParabolicPeakOffset(scores[[2]int{bestDX, bestDY - 1}], center, scores[[2]int{bestDX, bestDY + 1}])
	}
	result.offsetX = float64(bestDX) + fracX
	result.offsetY = float64(bestDY) + fracY
	prominence := math.Max(0, center-result.second)
	scoreQuality := diagnosticClamp01((center - 0.02) / 0.45)
	prominenceQuality := diagnosticClamp01(prominence / 0.10)
	curvatureQuality := diagnosticClamp01((curvatureX + curvatureY) / 0.30)
	result.confidence = 0.60*scoreQuality + 0.25*prominenceQuality + 0.15*curvatureQuality
	boundaryAxes := 0
	if bestDX == -radius || bestDX == radius {
		boundaryAxes++
	}
	if bestDY == -radius || bestDY == radius {
		boundaryAxes++
	}
	for i := 0; i < boundaryAxes; i++ {
		result.confidence *= 0.65
	}
	return result, true
}

func diagnosticCrossfitRepetitionPairs(profile Profile, fold int) [][2]int {
	if fold < 0 || fold >= diagnosticUnwrapCrossfitFolds {
		return nil
	}
	spec, ok := profileSpecFor(profile)
	if !ok {
		return nil
	}
	all := diagnosticBlindRepetitionPairs(profile)
	result := make([][2]int, 0, len(all)/diagnosticUnwrapCrossfitFolds+1)
	for _, pair := range all {
		codeIndex := v3CodeIndex(pair[0], spec.codedBits)
		if diagnosticCrossfitGroupFold(codeIndex) == fold {
			result = append(result, pair)
		}
	}
	return result
}

func diagnosticCrossfitGroupFold(codeIndex int) int {
	x := uint64(codeIndex+1) * 0x9e3779b185ebca87
	x ^= x >> 33
	x *= 0xc2b2ae3d27d4eb4f
	x ^= x >> 29
	return int(x & 1)
}

func diagnosticPrepareCrossfitLatticeProposal(primary, lattice diagnosticBlindPhaseResult) (diagnosticBlindPhaseResult, bool) {
	result := primary
	if len(result.controls) != len(lattice.controls) || len(result.controls) < diagnosticSmoothPhaseMinControls {
		return result, false
	}
	for i := range result.controls {
		p := &result.controls[i]
		l := lattice.controls[i]
		if !p.available || !l.available || l.confidence < diagnosticBlindLatticeConsensusMinConfidence {
			continue
		}
		pFracX := diagnosticWrapHalf(p.offsetX)
		pFracY := diagnosticWrapHalf(p.offsetY)
		distance := math.Hypot(diagnosticWrapHalf(pFracX-l.offsetX), diagnosticWrapHalf(pFracY-l.offsetY))
		if distance > diagnosticBlindLatticeConsensusMaxFractionDistance {
			continue
		}
		baseX := p.offsetX - pFracX
		baseY := p.offsetY - pFracY
		wp := math.Max(p.confidence, 0.03)
		wl := 0.55 * math.Max(l.confidence, 0.03)
		fracX := diagnosticWrapHalf((wp*pFracX + wl*l.offsetX) / (wp + wl))
		fracY := diagnosticWrapHalf((wp*pFracY + wl*l.offsetY) / (wp + wl))
		p.offsetX = baseX + fracX
		p.offsetY = baseY + fracY
		agreement := math.Exp(-2 * distance * distance)
		p.confidence = diagnosticClamp01(p.confidence*(0.90+0.10*agreement) + 0.12*l.confidence*agreement)
	}
	return result, true
}

func diagnosticValidateCrossfitTop2(primary diagnosticBlindPhaseResult, cells []diagnosticSpatialGridCell, bestX, bestY, secondX, secondY []int, validationPairs [][2]int) (int, float64, bool) {
	if len(validationPairs) == 0 || len(primary.controls) != len(cells) {
		return 0, 0, false
	}
	delta, weightSum := 0.0, 0.0
	cellsUsed := 0
	for i, cell := range cells {
		if len(cell.grid) < eccBits || !primary.controls[i].available {
			continue
		}
		bx, by := shiftAt(bestX, i), shiftAt(bestY, i)
		sx, sy := shiftAt(secondX, i), shiftAt(secondY, i)
		if bx == sx && by == sy {
			continue
		}
		control := primary.controls[i]
		bestPhaseX := positiveMod(primary.globalPhaseX+int(math.Round(control.offsetX+float64(bx))), tileWidth)
		bestPhaseY := positiveMod(primary.globalPhaseY+int(math.Round(control.offsetY+float64(by))), tileHeight)
		secondPhaseX := positiveMod(primary.globalPhaseX+int(math.Round(control.offsetX+float64(sx))), tileWidth)
		secondPhaseY := positiveMod(primary.globalPhaseY+int(math.Round(control.offsetY+float64(sy))), tileHeight)
		bestScore := diagnosticBlindRepetitionScorePairs(cell.grid, bestPhaseX, bestPhaseY, validationPairs)
		secondScore := diagnosticBlindRepetitionScorePairs(cell.grid, secondPhaseX, secondPhaseY, validationPairs)
		weight := math.Max(control.confidence, 0.03)
		delta += weight * (bestScore - secondScore)
		weightSum += weight
		cellsUsed++
	}
	if cellsUsed == 0 || weightSum <= 0 {
		return 0, 0, false
	}
	return cellsUsed, delta / weightSum, true
}

func diagnosticCrossfitBestLocalCycle(direction diagnosticUnwrapCrossfitDirection, i int) (int, int, bool) {
	if !direction.available || i < 0 || i >= len(direction.primary.controls) || !direction.primary.controls[i].available {
		return 0, 0, false
	}
	control := direction.primary.controls[i]
	// The fold-specific global tile phase is intentionally excluded here. Each
	// repetition subset can choose a different equivalent global anchor; the
	// integer-cycle question is whether the independently estimated, recentered
	// local fields assign the same block cycle to each spatial control.
	cycleX := int(math.Round(control.offsetX + float64(shiftAt(direction.bestX, i))))
	cycleY := int(math.Round(control.offsetY + float64(shiftAt(direction.bestY, i))))
	return cycleX, cycleY, true
}
