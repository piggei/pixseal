package watermark

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

const diagnosticUnwrapStabilityPartitions = 8

type diagnosticUnwrapStabilityTrial struct {
	partition      int
	proposalFold   int
	validationFold int
	direction      diagnosticUnwrapCrossfitDirection
	cycles         [][2]int
	cycleAvailable []bool
	complete       bool
	signature      string
}

type diagnosticUnwrapStabilityCell struct {
	observations           int
	uniqueCycles           int
	modalX                 int
	modalY                 int
	modalCount             int
	modalFraction          float64
	supportedObservations  int
	supportedUniqueCycles  int
	supportedModalX        int
	supportedModalY        int
	supportedModalCount    int
	supportedModalFraction float64
}

type diagnosticUnwrapStabilityResult struct {
	method                         string
	available                      bool
	partitions                     int
	trialsRequested                int
	trialsAvailable                int
	trialsSupported                int
	supportedTrialFraction         float64
	evaluatedStates                int
	meanMargin                     float64
	meanValidationDelta            float64
	completeFieldTrials            int
	uniqueFields                   int
	modalFieldCount                int
	modalFieldFraction             float64
	supportedCompleteFieldTrials   int
	supportedUniqueFields          int
	supportedModalFieldCount       int
	supportedModalFieldFraction    float64
	comparedCells                  int
	unanimousCells                 int
	meanCellModalFraction          float64
	minCellModalFraction           float64
	supportedComparedCells         int
	supportedUnanimousCells        int
	supportedMeanCellModalFraction float64
	supportedMinCellModalFraction  float64
	meanPairwiseAgreementFraction  float64
	cells                          []diagnosticUnwrapStabilityCell
	trials                         []diagnosticUnwrapStabilityTrial
}

// diagnosticGlobalUnwrapPartitionStability is the build19 diagnostic-only
// multi-partition experiment. It repeats the held-out integer-cycle proposal
// over eight fixed coded-bit-group partitions and both directions. No result is
// applied to sampling, HMAC, or production decode; the output measures whether
// the inferred recentered local cycle field persists when the repetition
// evidence is repartitioned.
func diagnosticGlobalUnwrapPartitionStability(cells []diagnosticSpatialGridCell, aggregate []float64, lattice diagnosticBlindPhaseResult) diagnosticUnwrapStabilityResult {
	result := diagnosticUnwrapStabilityResult{
		method:          "multi-partition-repetition-cycle-stability",
		partitions:      diagnosticUnwrapStabilityPartitions,
		trialsRequested: diagnosticUnwrapStabilityPartitions * 2,
		cells:           make([]diagnosticUnwrapStabilityCell, len(cells)),
	}
	if len(cells) < 3 || len(lattice.controls) != len(cells) {
		return result
	}

	for partition := 0; partition < diagnosticUnwrapStabilityPartitions; partition++ {
		for proposalFold := 0; proposalFold < 2; proposalFold++ {
			validationFold := 1 - proposalFold
			direction := diagnosticGlobalUnwrapStabilityDirection(cells, aggregate, lattice, partition, proposalFold, validationFold)
			trial := diagnosticUnwrapStabilityTrial{partition: partition, proposalFold: proposalFold, validationFold: validationFold, direction: direction}
			if direction.available {
				trial.cycles = make([][2]int, len(cells))
				trial.cycleAvailable = make([]bool, len(cells))
				complete := true
				signatureParts := make([]string, 0, len(cells))
				for i := range cells {
					x, y, ok := diagnosticCrossfitBestLocalCycle(direction, i)
					if !ok {
						complete = false
						continue
					}
					trial.cycles[i] = [2]int{x, y}
					trial.cycleAvailable[i] = true
					signatureParts = append(signatureParts, fmt.Sprintf("%d,%d", x, y))
				}
				trial.complete = complete
				if complete {
					trial.signature = strings.Join(signatureParts, ";")
				}
				result.trialsAvailable++
				result.evaluatedStates += direction.evaluatedStates
				result.meanMargin += direction.margin
				result.meanValidationDelta += direction.delta
				if direction.supportsBest {
					result.trialsSupported++
				}
			}
			result.trials = append(result.trials, trial)
		}
	}
	if result.trialsAvailable == 0 {
		return result
	}
	result.available = true
	result.meanMargin /= float64(result.trialsAvailable)
	result.meanValidationDelta /= float64(result.trialsAvailable)
	result.supportedTrialFraction = float64(result.trialsSupported) / float64(result.trialsAvailable)

	fieldCounts := map[string]int{}
	supportedFieldCounts := map[string]int{}
	for _, trial := range result.trials {
		if !trial.direction.available || !trial.complete {
			continue
		}
		result.completeFieldTrials++
		fieldCounts[trial.signature]++
		if trial.direction.supportsBest {
			result.supportedCompleteFieldTrials++
			supportedFieldCounts[trial.signature]++
		}
	}
	result.uniqueFields, result.modalFieldCount = diagnosticModalStringCount(fieldCounts)
	if result.completeFieldTrials > 0 {
		result.modalFieldFraction = float64(result.modalFieldCount) / float64(result.completeFieldTrials)
	}
	result.supportedUniqueFields, result.supportedModalFieldCount = diagnosticModalStringCount(supportedFieldCounts)
	if result.supportedCompleteFieldTrials > 0 {
		result.supportedModalFieldFraction = float64(result.supportedModalFieldCount) / float64(result.supportedCompleteFieldTrials)
	}

	result.minCellModalFraction = 1
	result.supportedMinCellModalFraction = 1
	for i := range cells {
		allCounts := map[[2]int]int{}
		supportedCounts := map[[2]int]int{}
		for _, trial := range result.trials {
			if !trial.direction.available || i >= len(trial.cycleAvailable) || !trial.cycleAvailable[i] {
				continue
			}
			cycle := trial.cycles[i]
			allCounts[cycle]++
			if trial.direction.supportsBest {
				supportedCounts[cycle]++
			}
		}
		cell := diagnosticSummarizeStabilityCell(allCounts, supportedCounts)
		result.cells[i] = cell
		if cell.observations > 0 {
			result.comparedCells++
			result.meanCellModalFraction += cell.modalFraction
			if cell.modalFraction < result.minCellModalFraction {
				result.minCellModalFraction = cell.modalFraction
			}
			if cell.modalCount == cell.observations {
				result.unanimousCells++
			}
		}
		if cell.supportedObservations > 0 {
			result.supportedComparedCells++
			result.supportedMeanCellModalFraction += cell.supportedModalFraction
			if cell.supportedModalFraction < result.supportedMinCellModalFraction {
				result.supportedMinCellModalFraction = cell.supportedModalFraction
			}
			if cell.supportedModalCount == cell.supportedObservations {
				result.supportedUnanimousCells++
			}
		}
	}
	if result.comparedCells > 0 {
		result.meanCellModalFraction /= float64(result.comparedCells)
	} else {
		result.minCellModalFraction = 0
	}
	if result.supportedComparedCells > 0 {
		result.supportedMeanCellModalFraction /= float64(result.supportedComparedCells)
	} else {
		result.supportedMinCellModalFraction = 0
	}
	result.meanPairwiseAgreementFraction = diagnosticStabilityMeanPairwiseAgreement(result.trials, len(cells))
	return result
}

func diagnosticGlobalUnwrapStabilityDirection(cells []diagnosticSpatialGridCell, aggregate []float64, lattice diagnosticBlindPhaseResult, partition, proposalFold, validationFold int) diagnosticUnwrapCrossfitDirection {
	result := diagnosticUnwrapCrossfitDirection{proposalFold: proposalFold, validationFold: validationFold}
	primary, ok := diagnosticEstimateBlindRepetitionPhaseStabilityFold(cells, aggregate, partition, proposalFold)
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
	validationPairs := diagnosticStabilityRepetitionPairs(primary.profile, partition, validationFold)
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

func diagnosticEstimateBlindRepetitionPhaseStabilityFold(cells []diagnosticSpatialGridCell, aggregate []float64, partition, fold int) (diagnosticBlindPhaseResult, bool) {
	result := diagnosticBlindPhaseResult{method: "intra-tile-repetition-multipartition"}
	if partition < 0 || partition >= diagnosticUnwrapStabilityPartitions || fold < 0 || fold > 1 || len(cells) < 3 || len(aggregate) < eccBits {
		return result, false
	}
	global := diagnosticBlindRepetitionPeak{peak: math.Inf(-1)}
	var proposalPairs [][2]int
	for _, profile := range []Profile{ProfileRobust, ProfileBalanced} {
		pairs := diagnosticStabilityRepetitionPairs(profile, partition, fold)
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

func diagnosticStabilityRepetitionPairs(profile Profile, partition, fold int) [][2]int {
	if partition < 0 || partition >= diagnosticUnwrapStabilityPartitions || fold < 0 || fold > 1 {
		return nil
	}
	spec, ok := profileSpecFor(profile)
	if !ok {
		return nil
	}
	all := diagnosticBlindRepetitionPairs(profile)
	result := make([][2]int, 0, len(all)/2+1)
	for _, pair := range all {
		codeIndex := v3CodeIndex(pair[0], spec.codedBits)
		if diagnosticStabilityGroupFold(codeIndex, partition) == fold {
			result = append(result, pair)
		}
	}
	return result
}

func diagnosticStabilityGroupFold(codeIndex, partition int) int {
	if partition == 0 {
		return diagnosticCrossfitGroupFold(codeIndex)
	}
	x := uint64(codeIndex+1) * 0x9e3779b185ebca87
	x ^= uint64(partition) * 0x94d049bb133111eb
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return int(x & 1)
}

func diagnosticModalStringCount(counts map[string]int) (int, int) {
	best := 0
	for _, count := range counts {
		if count > best {
			best = count
		}
	}
	return len(counts), best
}

func diagnosticSummarizeStabilityCell(allCounts, supportedCounts map[[2]int]int) diagnosticUnwrapStabilityCell {
	result := diagnosticUnwrapStabilityCell{}
	result.uniqueCycles = len(allCounts)
	result.modalX, result.modalY, result.modalCount, result.observations = diagnosticModalCycle(allCounts)
	if result.observations > 0 {
		result.modalFraction = float64(result.modalCount) / float64(result.observations)
	}
	result.supportedUniqueCycles = len(supportedCounts)
	result.supportedModalX, result.supportedModalY, result.supportedModalCount, result.supportedObservations = diagnosticModalCycle(supportedCounts)
	if result.supportedObservations > 0 {
		result.supportedModalFraction = float64(result.supportedModalCount) / float64(result.supportedObservations)
	}
	return result
}

func diagnosticModalCycle(counts map[[2]int]int) (int, int, int, int) {
	if len(counts) == 0 {
		return 0, 0, 0, 0
	}
	keys := make([][2]int, 0, len(counts))
	total := 0
	for cycle, count := range counts {
		keys = append(keys, cycle)
		total += count
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	best := keys[0]
	bestCount := counts[best]
	for _, cycle := range keys[1:] {
		count := counts[cycle]
		if count > bestCount {
			best, bestCount = cycle, count
		}
	}
	return best[0], best[1], bestCount, total
}

func diagnosticStabilityMeanPairwiseAgreement(trials []diagnosticUnwrapStabilityTrial, cells int) float64 {
	sum := 0.0
	pairs := 0
	for i := 0; i < len(trials); i++ {
		if !trials[i].direction.available {
			continue
		}
		for j := i + 1; j < len(trials); j++ {
			if !trials[j].direction.available {
				continue
			}
			compared, agreed := 0, 0
			for cell := 0; cell < cells; cell++ {
				if cell >= len(trials[i].cycleAvailable) || cell >= len(trials[j].cycleAvailable) || !trials[i].cycleAvailable[cell] || !trials[j].cycleAvailable[cell] {
					continue
				}
				compared++
				if trials[i].cycles[cell] == trials[j].cycles[cell] {
					agreed++
				}
			}
			if compared > 0 {
				sum += float64(agreed) / float64(compared)
				pairs++
			}
		}
	}
	if pairs == 0 {
		return 0
	}
	return sum / float64(pairs)
}
