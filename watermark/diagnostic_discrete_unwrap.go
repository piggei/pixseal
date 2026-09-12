package watermark

import (
	"math"
	"sort"
)

const (
	diagnosticDiscreteUnwrapMaxShift               = 1
	diagnosticDiscreteUnwrapChoicesPerCell         = 2*diagnosticDiscreteUnwrapMaxShift + 1
	diagnosticDiscreteUnwrapMaxAssignmentsPerAxis  = 19683 // 3^9 for the fixed 3x3 control grid.
	diagnosticDiscreteUnwrapMinLatticeConfidence   = 0.22
	diagnosticDiscreteUnwrapLockPrimaryConfidence  = 0.55
	diagnosticDiscreteUnwrapMaxFractionDistance    = 0.42
	diagnosticDiscreteUnwrapCyclePenalty           = 0.055
	diagnosticDiscreteUnwrapCurvaturePenalty       = 0.12
	diagnosticDiscreteUnwrapMinAbsoluteImprovement = 0.008
	diagnosticDiscreteUnwrapMinRelativeImprovement = 0.10
	diagnosticDiscreteUnwrapMinAbsoluteMargin      = 0.010
	diagnosticDiscreteUnwrapMinRelativeMargin      = 0.035
)

type diagnosticDiscreteUnwrapAxisResult struct {
	shifts            []int
	secondShifts      []int
	baselineObjective float64
	bestObjective     float64
	secondObjective   float64
	secondAvailable   bool
	improvement       float64
	margin            float64
	evaluatedStates   int
	eligibleCells     int
	changedCells      int
	accepted          bool
	ambiguous         bool
	status            string
}

type diagnosticDiscreteUnwrapState struct {
	shifts []int
	score  float64
}

// diagnosticGlobalDiscreteUnwrap resolves integer +/-1 block cycle choices as
// a small, axis-separable, key-independent global optimization problem. The
// local lattice observer supplies modulo-one-block phase; the repetition
// observer supplies the initial cycle. Build16 enumerates the complete bounded
// state space for each axis so the reported first/second objectives are exact,
// not survivors of a heuristic beam. The objective remains affine-residual +
// cycle-change + second-difference curvature. No expected Format-v3 bit, key
// material, payload byte, CRC or HMAC result is used.
func diagnosticGlobalDiscreteUnwrap(primary, lattice diagnosticBlindPhaseResult, cells []diagnosticSpatialGridCell) (diagnosticBlindPhaseResult, bool) {
	result := primary
	result.unwrapMethod = "axis-separable-exact-affine"
	result.unwrapStatus = "not-applicable"
	result.unwrapShiftX = make([]int, len(result.controls))
	result.unwrapShiftY = make([]int, len(result.controls))
	result.unwrapProposedShiftX = make([]int, len(result.controls))
	result.unwrapProposedShiftY = make([]int, len(result.controls))

	if len(cells) != len(result.controls) {
		result.unwrapStatus = "invalid-input"
		result.unwrapStatusX = "invalid-input"
		result.unwrapStatusY = "invalid-input"
		result.unwrapAmbiguous = true
		return result, false
	}
	if len(result.controls) < diagnosticSmoothPhaseMinControls || len(result.controls) != len(lattice.controls) {
		// Insufficient or partial lattice coverage is an ordinary research
		// outcome, not malformed geometry. Leave the primary controls untouched
		// and report that a global integer-cycle problem cannot be posed.
		result.unwrapStatus = "not-applicable"
		result.unwrapStatusX = "not-applicable"
		result.unwrapStatusY = "not-applicable"
		result.unwrapAmbiguous = false
		return result, false
	}

	x := diagnosticSolveDiscreteUnwrapAxis(result.controls, lattice.controls, cells, true)
	y := diagnosticSolveDiscreteUnwrapAxis(result.controls, lattice.controls, cells, false)
	result.unwrapStatusX = x.status
	result.unwrapStatusY = y.status
	result.unwrapEvaluatedStates = x.evaluatedStates + y.evaluatedStates
	result.unwrapEligibleCellsX = x.eligibleCells
	result.unwrapEligibleCellsY = y.eligibleCells
	result.unwrapEligibleCells = maxInt(x.eligibleCells, y.eligibleCells) // compatibility aggregate
	result.unwrapBaselineObjective = finiteOrZero(x.baselineObjective) + finiteOrZero(y.baselineObjective)
	result.unwrapObjective = finiteOrZero(x.bestObjective) + finiteOrZero(y.bestObjective)
	result.unwrapAppliedObjective = result.unwrapBaselineObjective

	second := math.Inf(1)
	secondX := append([]int(nil), x.shifts...)
	secondY := append([]int(nil), y.shifts...)
	if x.secondAvailable && isFiniteObjective(y.bestObjective) {
		second = x.secondObjective + y.bestObjective
		secondX = append([]int(nil), x.secondShifts...)
	}
	if y.secondAvailable && isFiniteObjective(x.bestObjective) {
		candidate := x.bestObjective + y.secondObjective
		if candidate < second || (math.Abs(candidate-second) <= 1e-12 && diagnosticDiscreteShiftLexLess(y.secondShifts, secondY)) {
			second = candidate
			secondX = append([]int(nil), x.shifts...)
			secondY = append([]int(nil), y.secondShifts...)
		}
	}
	if isFiniteObjective(second) {
		result.unwrapSecondAvailable = true
		result.unwrapSecondObjective = second
		result.unwrapMargin = second - result.unwrapObjective
		validation := diagnosticValidateGlobalUnwrapSplitRepetition(result, cells, x.shifts, y.shifts, secondX, secondY)
		result.unwrapValidationMethod = validation.method
		result.unwrapValidationAvailable = validation.available
		result.unwrapValidationCells = validation.cells
		result.unwrapValidationPairsFold0 = validation.pairsFold0
		result.unwrapValidationPairsFold1 = validation.pairsFold1
		result.unwrapValidationFold0Delta = validation.fold0Delta
		result.unwrapValidationFold1Delta = validation.fold1Delta
		result.unwrapValidationMeanDelta = validation.meanDelta
		result.unwrapValidationSupportsBest = validation.supportsBest
	}
	result.unwrapImprovement = result.unwrapBaselineObjective - result.unwrapObjective
	result.unwrapAmbiguous = x.ambiguous || y.ambiguous

	// Ambiguity on either axis invalidates the integer-cycle transaction as a
	// whole. Keep both axis diagnostics, but never partially commit the other
	// axis: a mixed accepted/ambiguous result is still globally ambiguous.
	if !result.unwrapAmbiguous {
		if x.accepted {
			result.unwrapAppliedObjective += x.bestObjective - x.baselineObjective
			result.unwrapAcceptedAxes++
		}
		if y.accepted {
			result.unwrapAppliedObjective += y.bestObjective - y.baselineObjective
			result.unwrapAcceptedAxes++
		}
	}

	for i := range result.controls {
		px, py := 0, 0
		if i < len(x.shifts) {
			px = x.shifts[i]
		}
		if i < len(y.shifts) {
			py = y.shifts[i]
		}
		result.unwrapProposedShiftX[i] = px
		result.unwrapProposedShiftY[i] = py
		if px != 0 || py != 0 {
			result.unwrapProposedChanged++
		}
	}

	for i := range result.controls {
		sx, sy := 0, 0
		if !result.unwrapAmbiguous && x.accepted && i < len(x.shifts) {
			sx = x.shifts[i]
		}
		if !result.unwrapAmbiguous && y.accepted && i < len(y.shifts) {
			sy = y.shifts[i]
		}
		result.unwrapShiftX[i] = sx
		result.unwrapShiftY[i] = sy
		if sx == 0 && sy == 0 {
			continue
		}
		result.controls[i].offsetX += float64(sx)
		result.controls[i].offsetY += float64(sy)
		result.unwrapChangedCells++
	}

	switch {
	case result.unwrapAmbiguous:
		result.unwrapStatus = "ambiguous"
	case result.unwrapChangedCells > 0:
		result.unwrapStatus = "accepted"
	case x.status == "rejected-improvement" || y.status == "rejected-improvement":
		result.unwrapStatus = "rejected-improvement"
	case x.status == "no-change" || y.status == "no-change":
		result.unwrapStatus = "no-change"
	default:
		result.unwrapStatus = "not-applicable"
	}

	if result.unwrapChangedCells == 0 {
		return result, false
	}
	diagnosticRecenterBlindControls(&result)
	return result, true
}

func diagnosticSolveDiscreteUnwrapAxis(primary, lattice []diagnosticBlindCellControl, cells []diagnosticSpatialGridCell, axisX bool) diagnosticDiscreteUnwrapAxisResult {
	n := len(primary)
	result := diagnosticDiscreteUnwrapAxisResult{shifts: make([]int, n), secondShifts: make([]int, n), status: "not-applicable"}
	eligible := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if !primary[i].available || i >= len(lattice) || !lattice[i].available {
			continue
		}
		if lattice[i].confidence < diagnosticDiscreteUnwrapMinLatticeConfidence || primary[i].confidence >= diagnosticDiscreteUnwrapLockPrimaryConfidence {
			continue
		}
		p := primary[i].offsetY
		l := lattice[i].offsetY
		if axisX {
			p = primary[i].offsetX
			l = lattice[i].offsetX
		}
		if math.Abs(diagnosticWrapHalf(p-l)) > diagnosticDiscreteUnwrapMaxFractionDistance {
			continue
		}
		eligible = append(eligible, i)
	}
	result.eligibleCells = len(eligible)
	zero := make([]int, n)
	result.baselineObjective = diagnosticDiscreteUnwrapAxisObjective(primary, cells, zero, axisX)
	result.bestObjective = result.baselineObjective
	if !isFiniteObjective(result.baselineObjective) {
		// Too few usable controls or a degenerate smooth fit means there is no
		// meaningful unwrap problem for this axis. Report a finite, explicit
		// not-applicable state rather than leaking +Inf into JSON.
		result.baselineObjective = 0
		result.bestObjective = 0
		result.status = "not-applicable"
		return result
	}
	if len(eligible) == 0 {
		return result
	}

	// Enumeration order is deterministic and independent of confidence. Exact
	// top-2 ranking makes ordering irrelevant to correctness; spatial order is
	// used only to keep diagnostics/reproducibility simple.
	sort.SliceStable(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if cells[a].RegionY == cells[b].RegionY {
			return cells[a].RegionX < cells[b].RegionX
		}
		return cells[a].RegionY < cells[b].RegionY
	})

	best := diagnosticDiscreteUnwrapState{score: math.Inf(1)}
	second := diagnosticDiscreteUnwrapState{score: math.Inf(1)}
	candidate := make([]int, n)
	var enumerate func(int)
	enumerate = func(depth int) {
		if depth == len(eligible) {
			score := diagnosticDiscreteUnwrapAxisObjective(primary, cells, candidate, axisX)
			result.evaluatedStates++
			if !isFiniteObjective(score) {
				return
			}
			state := diagnosticDiscreteUnwrapState{shifts: append([]int(nil), candidate...), score: score}
			if diagnosticDiscreteStateLess(state, best) {
				second = best
				best = state
			} else if diagnosticDiscreteStateLess(state, second) && !diagnosticDiscreteShiftsEqual(state.shifts, best.shifts) {
				second = state
			}
			return
		}
		idx := eligible[depth]
		for shift := -diagnosticDiscreteUnwrapMaxShift; shift <= diagnosticDiscreteUnwrapMaxShift; shift++ {
			candidate[idx] = shift
			enumerate(depth + 1)
		}
		candidate[idx] = 0
	}
	enumerate(0)

	if result.evaluatedStates > diagnosticDiscreteUnwrapMaxAssignmentsPerAxis {
		result.status = "budget-exceeded"
		result.ambiguous = true
		return result
	}
	if !isFiniteObjective(best.score) {
		result.status = "invalid-input"
		result.ambiguous = true
		return result
	}
	result.bestObjective = best.score
	result.shifts = append([]int(nil), best.shifts...)
	if isFiniteObjective(second.score) {
		result.secondAvailable = true
		result.secondObjective = second.score
		result.secondShifts = append([]int(nil), second.shifts...)
	}
	result.improvement = result.baselineObjective - result.bestObjective
	if result.secondAvailable {
		result.margin = result.secondObjective - result.bestObjective
	}
	for _, shift := range result.shifts {
		if shift != 0 {
			result.changedCells++
		}
	}
	if result.changedCells == 0 {
		result.status = "no-change"
		return result
	}

	relativeImprovement := result.improvement / math.Max(result.baselineObjective, 1e-9)
	if result.improvement < diagnosticDiscreteUnwrapMinAbsoluteImprovement || relativeImprovement < diagnosticDiscreteUnwrapMinRelativeImprovement {
		result.status = "rejected-improvement"
		return result
	}
	if !result.secondAvailable {
		result.status = "ambiguous"
		result.ambiguous = true
		return result
	}
	relativeMargin := result.margin / math.Max(result.baselineObjective, 1e-9)
	if result.margin < diagnosticDiscreteUnwrapMinAbsoluteMargin || relativeMargin < diagnosticDiscreteUnwrapMinRelativeMargin {
		result.status = "ambiguous"
		result.ambiguous = true
		return result
	}
	result.status = "accepted"
	result.accepted = true
	return result
}

func diagnosticDiscreteStateLess(a, b diagnosticDiscreteUnwrapState) bool {
	if !isFiniteObjective(a.score) {
		return false
	}
	if !isFiniteObjective(b.score) {
		return true
	}
	if math.Abs(a.score-b.score) > 1e-12 {
		return a.score < b.score
	}
	return diagnosticDiscreteShiftLexLess(a.shifts, b.shifts)
}

func diagnosticDiscreteShiftsEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func isFiniteObjective(v float64) bool {
	return !math.IsInf(v, 0) && !math.IsNaN(v)
}

func finiteOrZero(v float64) float64 {
	if !isFiniteObjective(v) {
		return 0
	}
	return v
}

func diagnosticDiscreteUnwrapAxisObjective(primary []diagnosticBlindCellControl, cells []diagnosticSpatialGridCell, shifts []int, axisX bool) float64 {
	fitControls := make([]diagnosticSmoothPhaseControl, 0, len(primary))
	cycleCost := 0.0
	available := 0
	for i, control := range primary {
		if !control.available || i >= len(cells) {
			continue
		}
		value := control.offsetY
		if axisX {
			value = control.offsetX
		}
		shift := 0
		if i < len(shifts) {
			shift = shifts[i]
		}
		value += float64(shift)
		nx := 2*(float64(cells[i].RegionX)+0.5)/float64(diagnosticSpatialGridAxis) - 1
		ny := 2*(float64(cells[i].RegionY)+0.5)/float64(diagnosticSpatialGridAxis) - 1
		confidence := diagnosticClamp01(control.confidence)
		if axisX {
			fitControls = append(fitControls, diagnosticSmoothPhaseControl{nx: nx, ny: ny, dx: value, dy: 0, weight: math.Max(confidence, 0.03), confidence: confidence})
		} else {
			fitControls = append(fitControls, diagnosticSmoothPhaseControl{nx: nx, ny: ny, dx: 0, dy: value, weight: math.Max(confidence, 0.03), confidence: confidence})
		}
		if shift != 0 {
			cycleCost += (0.30 + 0.70*confidence) * math.Abs(float64(shift))
		}
		available++
	}
	if len(fitControls) < diagnosticSmoothPhaseMinControls || available == 0 {
		return math.Inf(1)
	}
	fit, ok := diagnosticFitSmoothPhaseModel(fitControls, 3, 0, -1)
	if !ok {
		return math.Inf(1)
	}
	residual := fit.fitRMS * fit.fitRMS
	cycleCost = diagnosticDiscreteUnwrapCyclePenalty * cycleCost / float64(available)
	curvature := diagnosticDiscreteUnwrapAxisCurvature(primary, cells, shifts, axisX)
	return residual + cycleCost + diagnosticDiscreteUnwrapCurvaturePenalty*curvature
}

func diagnosticDiscreteUnwrapAxisCurvature(primary []diagnosticBlindCellControl, cells []diagnosticSpatialGridCell, shifts []int, axisX bool) float64 {
	values := make(map[[2]int]float64, len(primary))
	weights := make(map[[2]int]float64, len(primary))
	for i, control := range primary {
		if !control.available || i >= len(cells) {
			continue
		}
		value := control.offsetY
		if axisX {
			value = control.offsetX
		}
		if i < len(shifts) {
			value += float64(shifts[i])
		}
		key := [2]int{cells[i].RegionX, cells[i].RegionY}
		values[key] = value
		weights[key] = math.Max(control.confidence, 0.03)
	}
	sum, weightSum := 0.0, 0.0
	for y := 0; y < diagnosticSpatialGridAxis; y++ {
		a, oka := values[[2]int{0, y}]
		b, okb := values[[2]int{1, y}]
		c, okc := values[[2]int{2, y}]
		if oka && okb && okc {
			w := math.Min(weights[[2]int{0, y}], math.Min(weights[[2]int{1, y}], weights[[2]int{2, y}]))
			d := a - 2*b + c
			sum += w * d * d
			weightSum += w
		}
	}
	for x := 0; x < diagnosticSpatialGridAxis; x++ {
		a, oka := values[[2]int{x, 0}]
		b, okb := values[[2]int{x, 1}]
		c, okc := values[[2]int{x, 2}]
		if oka && okb && okc {
			w := math.Min(weights[[2]int{x, 0}], math.Min(weights[[2]int{x, 1}], weights[[2]int{x, 2}]))
			d := a - 2*b + c
			sum += w * d * d
			weightSum += w
		}
	}
	if weightSum == 0 {
		return 0
	}
	return sum / weightSum
}

func diagnosticDiscreteShiftLexLess(a, b []int) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] == b[i] {
			continue
		}
		// Prefer no change, then -1, then +1 for deterministic ties.
		rank := func(v int) int {
			if v == 0 {
				return 0
			}
			if v < 0 {
				return 1
			}
			return 2
		}
		return rank(a[i]) < rank(b[i])
	}
	return len(a) < len(b)
}
