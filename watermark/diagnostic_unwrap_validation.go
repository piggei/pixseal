package watermark

import "math"

const diagnosticUnwrapValidationFolds = 2

type diagnosticUnwrapValidationResult struct {
	method       string
	available    bool
	cells        int
	pairsFold0   int
	pairsFold1   int
	fold0Delta   float64
	fold1Delta   float64
	meanDelta    float64
	supportsBest bool
}

// diagnosticValidateGlobalUnwrapSplitRepetition compares the exact geometric
// top-1 and top-2 cycle assignments using two deterministic, disjoint subsets
// of the Format-v3 repetition pairs. Pair agreement never uses the key or the
// value of any coded bit. Build16 reports this as an additional diagnostic only:
// the primary repetition controls were estimated from the complete pair set,
// so this split is useful consistency evidence but is not statistically held
// out strongly enough to override an ambiguous exact geometric top-2 result.
func diagnosticValidateGlobalUnwrapSplitRepetition(primary diagnosticBlindPhaseResult, cells []diagnosticSpatialGridCell, bestX, bestY, secondX, secondY []int) diagnosticUnwrapValidationResult {
	result := diagnosticUnwrapValidationResult{method: "split-repetition-top2"}
	pairs := diagnosticBlindRepetitionPairs(primary.profile)
	if len(pairs) < diagnosticUnwrapValidationFolds || len(primary.controls) != len(cells) {
		return result
	}
	folds := [diagnosticUnwrapValidationFolds][][2]int{}
	for _, pair := range pairs {
		fold := diagnosticUnwrapValidationFold(pair)
		folds[fold] = append(folds[fold], pair)
	}
	result.pairsFold0 = len(folds[0])
	result.pairsFold1 = len(folds[1])
	if result.pairsFold0 == 0 || result.pairsFold1 == 0 {
		return result
	}

	delta := [diagnosticUnwrapValidationFolds]float64{}
	weights := [diagnosticUnwrapValidationFolds]float64{}
	for i, cell := range cells {
		if i >= len(primary.controls) || len(cell.grid) < eccBits || !primary.controls[i].available {
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
		weight := math.Max(control.confidence, 0.03)
		for fold := 0; fold < diagnosticUnwrapValidationFolds; fold++ {
			bestScore := diagnosticBlindRepetitionScorePairs(cell.grid, bestPhaseX, bestPhaseY, folds[fold])
			secondScore := diagnosticBlindRepetitionScorePairs(cell.grid, secondPhaseX, secondPhaseY, folds[fold])
			delta[fold] += weight * (bestScore - secondScore)
			weights[fold] += weight
		}
		result.cells++
	}
	if result.cells == 0 || weights[0] <= 0 || weights[1] <= 0 {
		return result
	}
	result.fold0Delta = delta[0] / weights[0]
	result.fold1Delta = delta[1] / weights[1]
	result.meanDelta = 0.5 * (result.fold0Delta + result.fold1Delta)
	result.supportsBest = result.fold0Delta > 0 && result.fold1Delta > 0
	result.available = true
	return result
}

func diagnosticUnwrapValidationFold(pair [2]int) int {
	// Mix both positions so neighboring/list-adjacent repetition pairs do not
	// systematically land in the same fold. All arithmetic is deterministic.
	x := uint64(pair[0]+1)*0x9e3779b185ebca87 ^ uint64(pair[1]+1)*0xc2b2ae3d27d4eb4f
	x ^= x >> 33
	return int(x & 1)
}

func shiftAt(shifts []int, i int) int {
	if i < 0 || i >= len(shifts) {
		return 0
	}
	return shifts[i]
}
