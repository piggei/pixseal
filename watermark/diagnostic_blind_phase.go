package watermark

import (
	"math"
	"sort"
)

const (
	diagnosticBlindPairRadius                = 4
	diagnosticBlindMinPairPeak               = 0.08
	diagnosticBlindMinPairConfidence         = 0.04
	diagnosticBlindGuidedPairRadius          = 1
	diagnosticBlindGuidedMinPairPeak         = 0.04
	diagnosticBlindGuidedMinPairConfidence   = 0.015
	diagnosticBlindHuberK                    = 1.50
	diagnosticBlindStructuralProfiles        = 2 // robust + balanced; capacity has no repetition
	diagnosticMaxBlindGlobalPhaseProbes      = diagnosticBlindStructuralProfiles * eccBits
	diagnosticMaxBlindLocalPhaseProbes       = diagnosticSpatialGridAxis * diagnosticSpatialGridAxis * (2*diagnosticSpatialPhaseRadius + 1) * (2*diagnosticSpatialPhaseRadius + 1)
	diagnosticMaxBlindGuidedPairwiseProbes   = (diagnosticSpatialGridAxis * diagnosticSpatialGridAxis * (diagnosticSpatialGridAxis*diagnosticSpatialGridAxis - 1) / 2) * (2*diagnosticBlindGuidedPairRadius + 1) * (2*diagnosticBlindGuidedPairRadius + 1)
	diagnosticMaxBlindPairwiseFallbackProbes = (diagnosticSpatialGridAxis * diagnosticSpatialGridAxis * (diagnosticSpatialGridAxis*diagnosticSpatialGridAxis - 1) / 2) * (2*diagnosticBlindPairRadius + 1) * (2*diagnosticBlindPairRadius + 1)
)

type diagnosticBlindPairConstraint struct {
	a, b       int
	dx, dy     float64 // observed offset[a] - observed offset[b], in blocks
	peak       float64
	confidence float64
	boundary   bool
	robust     float64
}

type diagnosticBlindCellControl struct {
	available  bool
	offsetX    float64
	offsetY    float64
	confidence float64
	pairs      int
	residual   float64
}

type diagnosticBlindPhaseResult struct {
	controls       []diagnosticBlindCellControl
	method         string
	profile        Profile
	globalPhaseX   int
	globalPhaseY   int
	globalScore    float64
	pairs          int
	meanPairPeak   float64
	meanConfidence float64
	minConfidence  float64
	meanOffset     float64
	maxOffset      float64
	robustOutliers int

	secondaryMethod       string
	secondaryCells        int
	secondaryMeanPairPeak float64
	consensusCells        int
	cycleSlipCorrections  int
	meanObserverDistance  float64
	maxObserverDistance   float64
	secondaryControls     []diagnosticBlindCellControl
	cycleSlipAdjusted     []bool

	latticeMethod               string
	latticeCells                int
	latticeMeanConfidence       float64
	latticeMinConfidence        float64
	latticeMeanFractionDistance float64
	latticeMaxFractionDistance  float64
	latticeConsensusCells       int
	latticeCycleSlipCorrections int
	latticeControls             []diagnosticBlindCellControl
	latticeCycleAdjusted        []bool

	unwrapMethod                 string
	unwrapStatus                 string
	unwrapStatusX                string
	unwrapStatusY                string
	unwrapEvaluatedStates        int
	unwrapEligibleCells          int
	unwrapEligibleCellsX         int
	unwrapEligibleCellsY         int
	unwrapChangedCells           int
	unwrapProposedChanged        int
	unwrapAcceptedAxes           int
	unwrapBaselineObjective      float64
	unwrapObjective              float64
	unwrapAppliedObjective       float64
	unwrapSecondObjective        float64
	unwrapSecondAvailable        bool
	unwrapImprovement            float64
	unwrapMargin                 float64
	unwrapAmbiguous              bool
	unwrapValidationMethod       string
	unwrapValidationAvailable    bool
	unwrapValidationCells        int
	unwrapValidationPairsFold0   int
	unwrapValidationPairsFold1   int
	unwrapValidationFold0Delta   float64
	unwrapValidationFold1Delta   float64
	unwrapValidationMeanDelta    float64
	unwrapValidationSupportsBest bool
	unwrapShiftX                 []int
	unwrapShiftY                 []int
}

const (
	diagnosticBlindConsensusMaxDistance      = 0.80
	diagnosticBlindCycleSlipMaxResidual      = 0.70
	diagnosticBlindCycleSlipMinGain          = 0.35
	diagnosticBlindCycleSlipMinSecondaryConf = 0.05
	diagnosticBlindCycleSlipMaxPrimaryConf   = 0.35
	diagnosticBlindSecondaryWeight           = 0.35
	diagnosticBlindConsensusMinSecondaryPeak = 0.10
)

// diagnosticEstimateBlindSpatialPhase combines two key-independent observers.
// Intra-tile repetition coherence remains primary. Cross-cell image-domain
// correlation is always evaluated as an independent secondary observer when
// possible. The secondary observer may confirm a control, reduce confidence on
// disagreement, or correct a bounded +/-1 integer cycle slip; it never sees
// the key, Format-v3 magic/header bits, payload, or HMAC result.
func diagnosticEstimateBlindSpatialPhase(cells []diagnosticSpatialGridCell, aggregate []float64) (diagnosticBlindPhaseResult, bool) {
	primary, primaryOK := diagnosticBlindPhaseResult{}, false
	if len(aggregate) >= eccBits {
		primary, primaryOK = diagnosticEstimateBlindRepetitionPhase(cells, aggregate)
	}
	if primaryOK {
		if secondary, secondaryOK := diagnosticEstimateBlindSpatialPhaseGuided(cells, primary); secondaryOK {
			secondary.method = "cross-cell-guided-correlation"
			return diagnosticFuseBlindObservers(primary, secondary), true
		}
		return primary, true
	}
	secondary, secondaryOK := diagnosticEstimateBlindSpatialPhasePairwise(cells)
	if secondaryOK {
		secondary.method = "cross-cell-correlation-fallback"
	}
	return secondary, secondaryOK
}

// diagnosticEstimateBlindSpatialPhaseWithLattice adds the build14 fractional
// local-lattice observer to the build12/13 blind registration chain. The
// lattice result is candidate-specific and already key-independent; the older
// guided cross-cell observer remains attached as a diagnostic control.
func diagnosticEstimateBlindSpatialPhaseWithLattice(cells []diagnosticSpatialGridCell, aggregate []float64, lattice diagnosticBlindPhaseResult, latticeOK bool) (diagnosticBlindPhaseResult, bool) {
	base, ok := diagnosticEstimateBlindSpatialPhase(cells, aggregate)
	if !ok {
		return base, false
	}
	if !latticeOK {
		return base, true
	}
	return diagnosticFuseBlindLatticePhase(base, lattice, cells), true
}

func diagnosticFuseBlindObservers(primary, secondary diagnosticBlindPhaseResult) diagnosticBlindPhaseResult {
	result := primary
	result.method = "intra-tile-repetition+guided-cross-cell-check"
	result.secondaryMethod = secondary.method
	result.secondaryMeanPairPeak = secondary.meanPairPeak
	result.secondaryControls = append([]diagnosticBlindCellControl(nil), secondary.controls...)
	result.cycleSlipAdjusted = make([]bool, len(result.controls))

	distanceSum := 0.0
	for i := range result.controls {
		if i >= len(secondary.controls) || !result.controls[i].available || !secondary.controls[i].available {
			continue
		}
		result.secondaryCells++
		distance := math.Hypot(secondary.controls[i].offsetX-result.controls[i].offsetX, secondary.controls[i].offsetY-result.controls[i].offsetY)
		distanceSum += distance
		if distance > result.maxObserverDistance {
			result.maxObserverDistance = distance
		}
	}
	if result.secondaryCells > 0 {
		result.meanObserverDistance = distanceSum / float64(result.secondaryCells)
	}
	// The build12 full pairwise observer produced ~0.09 correlations on the
	// real corpus. Build13 therefore treats the guided cross-cell observer as
	// diagnostic unless its mean pair peak clears an independent strength gate.
	// Weak secondary evidence is reported but cannot veto a stronger repetition
	// control or consume a smooth-field decode slot.
	if secondary.meanPairPeak < diagnosticBlindConsensusMinSecondaryPeak {
		return result
	}

	result.method = "intra-tile-repetition+guided-cross-cell-consensus"
	for i := range result.controls {
		if i >= len(secondary.controls) || !result.controls[i].available || !secondary.controls[i].available {
			continue
		}
		result.consensusCells++
		p := &result.controls[i]
		s := secondary.controls[i]
		rawDistance := math.Hypot(s.offsetX-p.offsetX, s.offsetY-p.offsetY)

		shiftX := math.Max(-1, math.Min(1, math.Round(s.offsetX-p.offsetX)))
		shiftY := math.Max(-1, math.Min(1, math.Round(s.offsetY-p.offsetY)))
		shiftedDistance := math.Hypot(s.offsetX-(p.offsetX+shiftX), s.offsetY-(p.offsetY+shiftY))
		if (shiftX != 0 || shiftY != 0) &&
			s.confidence >= diagnosticBlindCycleSlipMinSecondaryConf &&
			p.confidence <= diagnosticBlindCycleSlipMaxPrimaryConf &&
			shiftedDistance <= diagnosticBlindCycleSlipMaxResidual &&
			rawDistance-shiftedDistance >= diagnosticBlindCycleSlipMinGain {
			p.offsetX += shiftX
			p.offsetY += shiftY
			result.cycleSlipAdjusted[i] = true
			result.cycleSlipCorrections++
		}

		distance := math.Hypot(s.offsetX-p.offsetX, s.offsetY-p.offsetY)
		agreement := math.Exp(-0.5 * distance * distance)
		if distance <= diagnosticBlindConsensusMaxDistance {
			wp := math.Max(p.confidence, 0.02)
			ws := diagnosticBlindSecondaryWeight * math.Max(s.confidence, 0.02)
			p.offsetX = (wp*p.offsetX + ws*s.offsetX) / (wp + ws)
			p.offsetY = (wp*p.offsetY + ws*s.offsetY) / (wp + ws)
			p.confidence = diagnosticClamp01(p.confidence*(0.80+0.20*agreement) + 0.15*s.confidence*agreement)
		} else {
			p.confidence *= math.Max(0.35, agreement)
		}
	}
	diagnosticRecenterBlindControls(&result)
	return result
}

func diagnosticRecenterBlindControls(result *diagnosticBlindPhaseResult) {
	if result == nil {
		return
	}
	meanX, meanY, weightSum := 0.0, 0.0, 0.0
	for _, control := range result.controls {
		if !control.available {
			continue
		}
		w := math.Max(control.confidence, 0.02)
		meanX += w * control.offsetX
		meanY += w * control.offsetY
		weightSum += w
	}
	if weightSum > 0 {
		meanX /= weightSum
		meanY /= weightSum
	}
	result.meanOffset = 0
	result.maxOffset = 0
	result.meanConfidence = 0
	result.minConfidence = 1
	available := 0
	for i := range result.controls {
		control := &result.controls[i]
		if !control.available {
			continue
		}
		control.offsetX -= meanX
		control.offsetY -= meanY
		offset := math.Hypot(control.offsetX, control.offsetY)
		result.meanOffset += offset
		if offset > result.maxOffset {
			result.maxOffset = offset
		}
		result.meanConfidence += control.confidence
		if control.confidence < result.minConfidence {
			result.minConfidence = control.confidence
		}
		available++
	}
	if available > 0 {
		result.meanOffset /= float64(available)
		result.meanConfidence /= float64(available)
	} else {
		result.minConfidence = 0
	}
}

type diagnosticBlindRepetitionPeak struct {
	profile    Profile
	phaseX     int
	phaseY     int
	offsetX    float64
	offsetY    float64
	peak       float64
	second     float64
	confidence float64
}

func diagnosticEstimateBlindRepetitionPhase(cells []diagnosticSpatialGridCell, aggregate []float64) (diagnosticBlindPhaseResult, bool) {
	result := diagnosticBlindPhaseResult{method: "intra-tile-repetition"}
	if len(cells) < 3 || len(aggregate) < eccBits {
		return result, false
	}
	global, ok := diagnosticBlindBestRepetitionGlobal(aggregate)
	if !ok || global.peak < 0.05 {
		return result, false
	}
	result.profile = global.profile
	result.globalPhaseX = global.phaseX
	result.globalPhaseY = global.phaseY
	result.globalScore = global.peak
	result.pairs = diagnosticBlindRepetitionPairCount(global.profile)
	result.meanPairPeak = global.peak
	result.controls = make([]diagnosticBlindCellControl, len(cells))
	confSum := 0.0
	result.minConfidence = 1
	available := 0
	for i, cell := range cells {
		peak, ok := diagnosticBlindBestRepetitionNear(cell.grid, global.profile, global.phaseX, global.phaseY, diagnosticSpatialPhaseRadius)
		if !ok {
			continue
		}
		control := diagnosticBlindCellControl{
			available: true,
			offsetX:   peak.offsetX, offsetY: peak.offsetY,
			confidence: peak.confidence,
			pairs:      result.pairs,
		}
		result.controls[i] = control
		available++
		confSum += control.confidence
		if control.confidence < result.minConfidence {
			result.minConfidence = control.confidence
		}
	}
	if available < 3 {
		return diagnosticBlindPhaseResult{}, false
	}
	result.meanConfidence = confSum / float64(available)

	// Remove the common blind offset: only spatial residual drift belongs in
	// the smooth field. The global v3 phase path keeps responsibility for any
	// common tile translation. This centering is still entirely key-independent.
	meanX, meanY, weightSum := 0.0, 0.0, 0.0
	for _, control := range result.controls {
		if !control.available {
			continue
		}
		w := math.Max(control.confidence, 0.02)
		meanX += w * control.offsetX
		meanY += w * control.offsetY
		weightSum += w
	}
	if weightSum > 0 {
		meanX /= weightSum
		meanY /= weightSum
	}
	for i := range result.controls {
		if !result.controls[i].available {
			continue
		}
		result.controls[i].offsetX -= meanX
		result.controls[i].offsetY -= meanY
		offset := math.Hypot(result.controls[i].offsetX, result.controls[i].offsetY)
		result.meanOffset += offset
		if offset > result.maxOffset {
			result.maxOffset = offset
		}
	}
	result.meanOffset /= float64(available)
	return result, true
}

func diagnosticBlindPhaseMedoid(peaks []diagnosticBlindRepetitionPeak) (int, int, float64) {
	if len(peaks) == 0 {
		return 0, 0, 0
	}
	bestIndex := 0
	bestCost := math.Inf(1)
	for i, anchor := range peaks {
		cost, weightSum := 0.0, 0.0
		for _, peak := range peaks {
			dx := float64(diagnosticSignedPhaseDelta(peak.phaseX, anchor.phaseX, tileWidth))
			dy := float64(diagnosticSignedPhaseDelta(peak.phaseY, anchor.phaseY, tileHeight))
			w := math.Max(peak.confidence, 0.05)
			cost += w * math.Hypot(dx, dy)
			weightSum += w
		}
		if weightSum > 0 {
			cost /= weightSum
		}
		if cost < bestCost {
			bestCost = cost
			bestIndex = i
		}
	}
	return peaks[bestIndex].phaseX, peaks[bestIndex].phaseY, bestCost
}

func diagnosticBlindBestRepetitionFull(grid []float64, profile Profile, pairs [][2]int) (diagnosticBlindRepetitionPeak, bool) {
	result := diagnosticBlindRepetitionPeak{profile: profile, peak: math.Inf(-1), second: math.Inf(-1)}
	if len(grid) < eccBits || len(pairs) == 0 {
		return result, false
	}
	scores := make([]float64, eccBits)
	for y := 0; y < tileHeight; y++ {
		for x := 0; x < tileWidth; x++ {
			score := diagnosticBlindRepetitionScorePairs(grid, x, y, pairs)
			scores[y*tileWidth+x] = score
			if score > result.peak {
				result.second = result.peak
				result.peak = score
				result.phaseX = x
				result.phaseY = y
			} else if score > result.second {
				result.second = score
			}
		}
	}
	if math.IsInf(result.peak, -1) {
		return result, false
	}
	center := scores[result.phaseY*tileWidth+result.phaseX]
	left := scores[result.phaseY*tileWidth+positiveMod(result.phaseX-1, tileWidth)]
	right := scores[result.phaseY*tileWidth+positiveMod(result.phaseX+1, tileWidth)]
	down := scores[positiveMod(result.phaseY-1, tileHeight)*tileWidth+result.phaseX]
	up := scores[positiveMod(result.phaseY+1, tileHeight)*tileWidth+result.phaseX]
	result.offsetX, _ = diagnosticParabolicPeakOffset(left, center, right)
	result.offsetY, _ = diagnosticParabolicPeakOffset(down, center, up)
	_, curvatureX := diagnosticParabolicPeakOffset(left, center, right)
	_, curvatureY := diagnosticParabolicPeakOffset(down, center, up)
	prominence := math.Max(0, center-result.second)
	scoreQuality := diagnosticClamp01((center - 0.02) / 0.45)
	prominenceQuality := diagnosticClamp01(prominence / 0.10)
	curvatureQuality := diagnosticClamp01((curvatureX + curvatureY) / 0.30)
	result.confidence = 0.60*scoreQuality + 0.25*prominenceQuality + 0.15*curvatureQuality
	return result, true
}

func diagnosticBlindBestRepetitionGlobal(grid []float64) (diagnosticBlindRepetitionPeak, bool) {
	best := diagnosticBlindRepetitionPeak{peak: math.Inf(-1)}
	for _, profile := range []Profile{ProfileRobust, ProfileBalanced} {
		peak, ok := diagnosticBlindBestRepetitionFull(grid, profile, diagnosticBlindRepetitionPairs(profile))
		if ok && peak.peak > best.peak {
			best = peak
		}
	}
	return best, !math.IsInf(best.peak, -1)
}

func diagnosticBlindBestRepetitionNear(grid []float64, profile Profile, referenceX, referenceY, radius int) (diagnosticBlindRepetitionPeak, bool) {
	result := diagnosticBlindRepetitionPeak{profile: profile, peak: math.Inf(-1), second: math.Inf(-1)}
	if len(grid) < eccBits || radius < 0 {
		return result, false
	}
	scores := make(map[[2]int]float64, (2*radius+1)*(2*radius+1))
	bestDX, bestDY := 0, 0
	bestDistance := math.Inf(1)
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			phaseX := positiveMod(referenceX+dx, tileWidth)
			phaseY := positiveMod(referenceY+dy, tileHeight)
			score := diagnosticBlindRepetitionScore(grid, profile, phaseX, phaseY)
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

// diagnosticBlindRepetitionScore measures whether positions that Format v3
// maps to the same coded bit agree with one another. It uses neither the key
// nor the value of that coded bit. Capacity has no repetition and is therefore
// intentionally unsupported as a blind phase source.
func diagnosticBlindRepetitionScore(grid []float64, profile Profile, phaseX, phaseY int) float64 {
	return diagnosticBlindRepetitionScorePairs(grid, phaseX, phaseY, diagnosticBlindRepetitionPairs(profile))
}

func diagnosticBlindRepetitionScorePairs(grid []float64, phaseX, phaseY int, pairs [][2]int) float64 {
	if len(grid) < eccBits || len(pairs) == 0 {
		return 0
	}
	numerator, denominator := 0.0, 0.0
	for _, pair := range pairs {
		p0, p1 := pair[0], pair[1]
		x0, y0 := p0%tileWidth, p0/tileWidth
		x1, y1 := p1%tileWidth, p1/tileWidth
		o0x := positiveMod(x0-phaseX, tileWidth)
		o0y := positiveMod(y0-phaseY, tileHeight)
		o1x := positiveMod(x1-phaseX, tileWidth)
		o1y := positiveMod(y1-phaseY, tileHeight)
		v0 := grid[o0y*tileWidth+o0x]
		v1 := grid[o1y*tileWidth+o1x]
		product := v0 * v1
		numerator += product
		denominator += math.Abs(product)
	}
	if denominator <= 1e-9 {
		return 0
	}
	return numerator / denominator
}

func diagnosticBlindRepetitionPairs(profile Profile) [][2]int {
	spec, ok := profileSpecFor(profile)
	if !ok || spec.codedBits >= eccBits {
		return nil
	}
	groups := make([][]int, spec.codedBits)
	for logicalPosition := 0; logicalPosition < eccBits; logicalPosition++ {
		index := v3CodeIndex(logicalPosition, spec.codedBits)
		groups[index] = append(groups[index], logicalPosition)
	}
	pairs := make([][2]int, 0, diagnosticBlindRepetitionPairCount(profile))
	for _, group := range groups {
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				pairs = append(pairs, [2]int{group[i], group[j]})
			}
		}
	}
	return pairs
}

func diagnosticBlindRepetitionPairCount(profile Profile) int {
	spec, ok := profileSpecFor(profile)
	if !ok || spec.codedBits >= eccBits {
		return 0
	}
	counts := make([]int, spec.codedBits)
	for logicalPosition := 0; logicalPosition < eccBits; logicalPosition++ {
		counts[v3CodeIndex(logicalPosition, spec.codedBits)]++
	}
	pairs := 0
	for _, count := range counts {
		pairs += count * (count - 1) / 2
	}
	return pairs
}

// diagnosticEstimateBlindSpatialPhaseGuided performs the build13 secondary
// image-domain observation. It searches only a +/-1 integer-block neighborhood
// around the relative offsets predicted by the repetition observer, then solves
// the resulting pair graph with the same robust zero-mean gauge. It is blind:
// no key, expected bit value, header byte or HMAC result enters this score.
func diagnosticEstimateBlindSpatialPhaseGuided(cells []diagnosticSpatialGridCell, primary diagnosticBlindPhaseResult) (diagnosticBlindPhaseResult, bool) {
	result := diagnosticBlindPhaseResult{}
	if len(cells) < 3 || len(primary.controls) != len(cells) {
		return result, false
	}
	normalized := make([][]float64, len(cells))
	for i := range cells {
		normalized[i] = diagnosticNormalizeBlindGrid(cells[i].grid)
		if len(normalized[i]) != eccBits {
			return result, false
		}
	}
	constraints := make([]diagnosticBlindPairConstraint, 0, len(cells)*(len(cells)-1)/2)
	peakSum := 0.0
	for i := 0; i < len(cells); i++ {
		if !primary.controls[i].available {
			continue
		}
		for j := i + 1; j < len(cells); j++ {
			if !primary.controls[j].available {
				continue
			}
			centerX := int(math.Round(primary.controls[i].offsetX - primary.controls[j].offsetX))
			centerY := int(math.Round(primary.controls[i].offsetY - primary.controls[j].offsetY))
			pair, ok := diagnosticBlindPairSurfaceAround(normalized[i], normalized[j], centerX, centerY, diagnosticBlindGuidedPairRadius)
			if !ok || pair.peak < diagnosticBlindGuidedMinPairPeak || pair.confidence < diagnosticBlindGuidedMinPairConfidence {
				continue
			}
			pair.a = i
			pair.b = j
			pair.robust = 1
			constraints = append(constraints, pair)
			peakSum += pair.peak
		}
	}
	if len(constraints) < len(cells)-1 {
		return result, false
	}
	offsetsX, offsetsY, outliers, ok := diagnosticSolveBlindGraph(len(cells), constraints)
	if !ok {
		return result, false
	}
	result.controls = make([]diagnosticBlindCellControl, len(cells))
	result.pairs = len(constraints)
	result.robustOutliers = outliers
	result.meanPairPeak = peakSum / float64(len(constraints))
	result.minConfidence = 1
	available := 0
	for i := range cells {
		weightedConf, weightedResidual, weightSum := 0.0, 0.0, 0.0
		pairs := 0
		for _, p := range constraints {
			if p.a != i && p.b != i {
				continue
			}
			residual := math.Hypot((offsetsX[p.a]-offsetsX[p.b])-p.dx, (offsetsY[p.a]-offsetsY[p.b])-p.dy)
			w := math.Max(p.confidence*p.robust, 0.01)
			weightedConf += w * p.confidence
			weightedResidual += w * residual
			weightSum += w
			pairs++
		}
		if pairs == 0 || weightSum <= 0 {
			continue
		}
		meanPairConf := weightedConf / weightSum
		meanResidual := weightedResidual / weightSum
		consistency := math.Exp(-0.5 * meanResidual * meanResidual)
		confidence := diagnosticClamp01(meanPairConf * consistency)
		result.controls[i] = diagnosticBlindCellControl{
			available: true, offsetX: offsetsX[i], offsetY: offsetsY[i],
			confidence: confidence, pairs: pairs, residual: meanResidual,
		}
		result.meanConfidence += confidence
		if confidence < result.minConfidence {
			result.minConfidence = confidence
		}
		offset := math.Hypot(offsetsX[i], offsetsY[i])
		result.meanOffset += offset
		if offset > result.maxOffset {
			result.maxOffset = offset
		}
		available++
	}
	if available < 3 {
		return diagnosticBlindPhaseResult{}, false
	}
	result.meanConfidence /= float64(available)
	result.meanOffset /= float64(available)
	return result, true
}

func diagnosticBlindPairSurfaceAround(a, b []float64, centerX, centerY, radius int) (diagnosticBlindPairConstraint, bool) {
	result := diagnosticBlindPairConstraint{}
	if len(a) < eccBits || len(b) < eccBits || radius < 0 {
		return result, false
	}
	type sample struct {
		dx, dy int
		score  float64
	}
	samples := make([]sample, 0, (2*radius+1)*(2*radius+1))
	peak, second := math.Inf(-1), math.Inf(-1)
	bestX, bestY := centerX, centerY
	for dy := centerY - radius; dy <= centerY+radius; dy++ {
		for dx := centerX - radius; dx <= centerX+radius; dx++ {
			score := diagnosticBlindPairScore(a, b, dx, dy)
			samples = append(samples, sample{dx: dx, dy: dy, score: score})
			if score > peak {
				second = peak
				peak = score
				bestX, bestY = dx, dy
			} else if score > second {
				second = score
			}
		}
	}
	if math.IsInf(peak, -1) {
		return result, false
	}
	lookup := func(dx, dy int) (float64, bool) {
		for _, s := range samples {
			if s.dx == dx && s.dy == dy {
				return s.score, true
			}
		}
		return 0, false
	}
	fracX, curvatureX := 0.0, 0.0
	if left, okL := lookup(bestX-1, bestY); okL {
		if right, okR := lookup(bestX+1, bestY); okR {
			fracX, curvatureX = diagnosticParabolicPeakOffset(left, peak, right)
		}
	}
	fracY, curvatureY := 0.0, 0.0
	if down, okD := lookup(bestX, bestY-1); okD {
		if up, okU := lookup(bestX, bestY+1); okU {
			fracY, curvatureY = diagnosticParabolicPeakOffset(down, peak, up)
		}
	}
	result.dx = float64(bestX) + fracX
	result.dy = float64(bestY) + fracY
	result.peak = peak
	prominence := math.Max(0, peak-second)
	scoreQuality := diagnosticClamp01((peak - 0.02) / 0.25)
	prominenceQuality := diagnosticClamp01(prominence / 0.08)
	curvatureQuality := diagnosticClamp01((curvatureX + curvatureY) / 0.40)
	result.confidence = 0.60*scoreQuality + 0.25*prominenceQuality + 0.15*curvatureQuality
	if bestX == centerX-radius || bestX == centerX+radius || bestY == centerY-radius || bestY == centerY+radius {
		result.boundary = true
		result.confidence *= 0.80
	}
	return result, true
}

// diagnosticEstimateBlindSpatialPhase self-registers the repeated v3 tile
// without a key and without any expected bit values. It compares the observed
// 1120-position DCT-margin pattern between spatial cells, estimates bounded
// pairwise relative shifts, then solves the relative-offset graph with a
// zero-mean gauge. The common phase is intentionally left to the existing
// global phase estimator; this routine measures only spatial residual drift.
func diagnosticEstimateBlindSpatialPhasePairwise(cells []diagnosticSpatialGridCell) (diagnosticBlindPhaseResult, bool) {
	result := diagnosticBlindPhaseResult{}
	if len(cells) < 3 {
		return result, false
	}
	normalized := make([][]float64, len(cells))
	for i := range cells {
		normalized[i] = diagnosticNormalizeBlindGrid(cells[i].grid)
		if len(normalized[i]) != eccBits {
			return result, false
		}
	}

	constraints := make([]diagnosticBlindPairConstraint, 0, len(cells)*(len(cells)-1)/2)
	peakSum := 0.0
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			pair, ok := diagnosticBlindPairSurface(normalized[i], normalized[j], diagnosticBlindPairRadius)
			if !ok || pair.peak < diagnosticBlindMinPairPeak || pair.confidence < diagnosticBlindMinPairConfidence {
				continue
			}
			pair.a = i
			pair.b = j
			pair.robust = 1
			constraints = append(constraints, pair)
			peakSum += pair.peak
		}
	}
	if len(constraints) < len(cells)-1 {
		return result, false
	}

	offsetsX, offsetsY, outliers, ok := diagnosticSolveBlindGraph(len(cells), constraints)
	if !ok {
		return result, false
	}
	result.controls = make([]diagnosticBlindCellControl, len(cells))
	result.pairs = len(constraints)
	result.robustOutliers = outliers
	if len(constraints) > 0 {
		result.meanPairPeak = peakSum / float64(len(constraints))
	}

	confSum := 0.0
	result.minConfidence = 1
	for i := range cells {
		weightedConf, weightedResidual, weightSum := 0.0, 0.0, 0.0
		pairs := 0
		for _, p := range constraints {
			if p.a != i && p.b != i {
				continue
			}
			residualX := (offsetsX[p.a] - offsetsX[p.b]) - p.dx
			residualY := (offsetsY[p.a] - offsetsY[p.b]) - p.dy
			residual := math.Hypot(residualX, residualY)
			w := math.Max(p.confidence*p.robust, 0.01)
			weightedConf += w * p.confidence
			weightedResidual += w * residual
			weightSum += w
			pairs++
		}
		if pairs == 0 || weightSum <= 0 {
			continue
		}
		meanPairConf := weightedConf / weightSum
		meanResidual := weightedResidual / weightSum
		consistency := math.Exp(-0.5 * meanResidual * meanResidual)
		confidence := diagnosticClamp01(meanPairConf * consistency)
		control := diagnosticBlindCellControl{
			available: true,
			offsetX:   offsetsX[i], offsetY: offsetsY[i],
			confidence: confidence, pairs: pairs, residual: meanResidual,
		}
		result.controls[i] = control
		confSum += confidence
		if confidence < result.minConfidence {
			result.minConfidence = confidence
		}
		offset := math.Hypot(offsetsX[i], offsetsY[i])
		result.meanOffset += offset
		if offset > result.maxOffset {
			result.maxOffset = offset
		}
	}
	available := 0
	for _, c := range result.controls {
		if c.available {
			available++
		}
	}
	if available < 3 {
		return diagnosticBlindPhaseResult{}, false
	}
	result.meanConfidence = confSum / float64(available)
	result.meanOffset /= float64(available)
	if result.minConfidence == 1 && available == 0 {
		result.minConfidence = 0
	}
	return result, true
}

func diagnosticNormalizeBlindGrid(grid []float64) []float64 {
	if len(grid) < eccBits {
		return nil
	}
	abs := make([]float64, eccBits)
	for i := 0; i < eccBits; i++ {
		abs[i] = math.Abs(grid[i])
	}
	sort.Float64s(abs)
	scale := diagnosticQuantileSorted(abs, 0.50)
	if scale < 1e-9 {
		scale = 1
	}
	out := make([]float64, eccBits)
	mean := 0.0
	for i := 0; i < eccBits; i++ {
		v := grid[i] / scale
		if v > 3 {
			v = 3
		} else if v < -3 {
			v = -3
		}
		out[i] = v
		mean += v
	}
	mean /= float64(eccBits)
	energy := 0.0
	for i := range out {
		out[i] -= mean
		energy += out[i] * out[i]
	}
	rms := math.Sqrt(energy / float64(eccBits))
	if rms < 1e-9 {
		return nil
	}
	for i := range out {
		out[i] /= rms
	}
	return out
}

func diagnosticBlindPairScore(a, b []float64, dx, dy int) float64 {
	if len(a) < eccBits || len(b) < eccBits {
		return 0
	}
	sum := 0.0
	for y := 0; y < tileHeight; y++ {
		by := positiveMod(y+dy, tileHeight)
		for x := 0; x < tileWidth; x++ {
			bx := positiveMod(x+dx, tileWidth)
			sum += a[y*tileWidth+x] * b[by*tileWidth+bx]
		}
	}
	return sum / float64(eccBits)
}

func diagnosticBlindPairSurface(a, b []float64, radius int) (diagnosticBlindPairConstraint, bool) {
	result := diagnosticBlindPairConstraint{}
	if len(a) < eccBits || len(b) < eccBits || radius < 0 {
		return result, false
	}
	scores := make(map[[2]int]float64, (2*radius+1)*(2*radius+1))
	bestX, bestY := 0, 0
	peak, second := math.Inf(-1), math.Inf(-1)
	bestDistance := math.Inf(1)
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			score := diagnosticBlindPairScore(a, b, dx, dy)
			scores[[2]int{dx, dy}] = score
			distance := math.Hypot(float64(dx), float64(dy))
			if score > peak || (score == peak && distance < bestDistance) {
				second = peak
				peak = score
				bestX, bestY = dx, dy
				bestDistance = distance
			} else if score > second {
				second = score
			}
		}
	}
	if math.IsInf(peak, -1) {
		return result, false
	}
	center := scores[[2]int{bestX, bestY}]
	fracX, curvatureX := 0.0, 0.0
	if bestX > -radius && bestX < radius {
		fracX, curvatureX = diagnosticParabolicPeakOffset(scores[[2]int{bestX - 1, bestY}], center, scores[[2]int{bestX + 1, bestY}])
	}
	fracY, curvatureY := 0.0, 0.0
	if bestY > -radius && bestY < radius {
		fracY, curvatureY = diagnosticParabolicPeakOffset(scores[[2]int{bestX, bestY - 1}], center, scores[[2]int{bestX, bestY + 1}])
	}
	result.dx = float64(bestX) + fracX
	result.dy = float64(bestY) + fracY
	result.peak = center
	prominence := math.Max(0, center-second)
	scoreQuality := diagnosticClamp01((center - 0.05) / 0.55)
	prominenceQuality := diagnosticClamp01(prominence / 0.20)
	curvatureQuality := diagnosticClamp01((curvatureX + curvatureY) / 0.60)
	result.confidence = 0.60*scoreQuality + 0.25*prominenceQuality + 0.15*curvatureQuality
	boundaryAxes := 0
	if bestX == -radius || bestX == radius {
		boundaryAxes++
	}
	if bestY == -radius || bestY == radius {
		boundaryAxes++
	}
	if boundaryAxes > 0 {
		result.boundary = true
		for k := 0; k < boundaryAxes; k++ {
			result.confidence *= 0.65
		}
	}
	return result, true
}

func diagnosticSolveBlindGraph(n int, constraints []diagnosticBlindPairConstraint) ([]float64, []float64, int, bool) {
	if n < 2 || len(constraints) < n-1 {
		return nil, nil, 0, false
	}
	robust := make([]float64, len(constraints))
	for i := range robust {
		robust[i] = 1
	}
	var x, y []float64
	outliers := 0
	for iteration := 0; iteration < 3; iteration++ {
		normal := make([][]float64, n)
		for i := range normal {
			normal[i] = make([]float64, n)
		}
		rhsX := make([]float64, n)
		rhsY := make([]float64, n)
		totalWeight := 0.0
		for k, p := range constraints {
			w := math.Max(p.confidence, 0.01) * robust[k]
			totalWeight += w
			normal[p.a][p.a] += w
			normal[p.b][p.b] += w
			normal[p.a][p.b] -= w
			normal[p.b][p.a] -= w
			rhsX[p.a] += w * p.dx
			rhsX[p.b] -= w * p.dx
			rhsY[p.a] += w * p.dy
			rhsY[p.b] -= w * p.dy
		}
		if totalWeight <= 0 {
			return nil, nil, 0, false
		}
		// Zero-mean gauge. Adding lambda*(sum offsets)^2 removes the null mode
		// without privileging any cell as a special anchor.
		gauge := totalWeight / float64(n*n)
		for r := 0; r < n; r++ {
			for c := 0; c < n; c++ {
				normal[r][c] += gauge
			}
		}
		var okX, okY bool
		x, okX = diagnosticSolveDense(normal, rhsX)
		y, okY = diagnosticSolveDense(normal, rhsY)
		if !okX || !okY {
			return nil, nil, 0, false
		}
		residuals := make([]float64, len(constraints))
		sorted := make([]float64, len(constraints))
		for k, p := range constraints {
			residuals[k] = math.Hypot((x[p.a]-x[p.b])-p.dx, (y[p.a]-y[p.b])-p.dy)
			sorted[k] = residuals[k]
		}
		sort.Float64s(sorted)
		scale := diagnosticQuantileSorted(sorted, 0.50)
		if scale < 0.10 {
			scale = 0.10
		}
		threshold := diagnosticBlindHuberK * scale
		outliers = 0
		for k, residual := range residuals {
			robust[k] = 1
			if residual > threshold {
				robust[k] = threshold / residual
				outliers++
			}
		}
	}
	return x, y, outliers, true
}

func diagnosticSolveDense(matrix [][]float64, rhs []float64) ([]float64, bool) {
	n := len(rhs)
	if n == 0 || len(matrix) != n {
		return nil, false
	}
	a := make([][]float64, n)
	for i := 0; i < n; i++ {
		if len(matrix[i]) != n {
			return nil, false
		}
		a[i] = make([]float64, n+1)
		copy(a[i], matrix[i])
		a[i][n] = rhs[i]
	}
	for col := 0; col < n; col++ {
		pivot := col
		for row := col + 1; row < n; row++ {
			if math.Abs(a[row][col]) > math.Abs(a[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(a[pivot][col]) < 1e-10 {
			return nil, false
		}
		a[col], a[pivot] = a[pivot], a[col]
		div := a[col][col]
		for k := col; k <= n; k++ {
			a[col][k] /= div
		}
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := a[row][col]
			if factor == 0 {
				continue
			}
			for k := col; k <= n; k++ {
				a[row][k] -= factor * a[col][k]
			}
		}
	}
	out := make([]float64, n)
	for i := range out {
		out[i] = a[i][n]
		if math.IsNaN(out[i]) || math.IsInf(out[i], 0) {
			return nil, false
		}
	}
	return out, true
}

func diagnosticPeriodicBilinearGrid(base []float64, offsetX, offsetY float64) []float64 {
	if len(base) < eccBits {
		return nil
	}
	out := make([]float64, eccBits)
	for y := 0; y < tileHeight; y++ {
		for x := 0; x < tileWidth; x++ {
			sx := float64(x) + offsetX
			sy := float64(y) + offsetY
			x0 := int(math.Floor(sx))
			y0 := int(math.Floor(sy))
			fx := sx - float64(x0)
			fy := sy - float64(y0)
			x1, y1 := x0+1, y0+1
			v00 := base[positiveMod(y0, tileHeight)*tileWidth+positiveMod(x0, tileWidth)]
			v10 := base[positiveMod(y0, tileHeight)*tileWidth+positiveMod(x1, tileWidth)]
			v01 := base[positiveMod(y1, tileHeight)*tileWidth+positiveMod(x0, tileWidth)]
			v11 := base[positiveMod(y1, tileHeight)*tileWidth+positiveMod(x1, tileWidth)]
			top := v00*(1-fx) + v10*fx
			bottom := v01*(1-fx) + v11*fx
			out[y*tileWidth+x] = top*(1-fy) + bottom*fy
		}
	}
	return out
}
