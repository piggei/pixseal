package watermark

import "testing"

func TestDiagnosticCrossfitRepetitionFoldsAreGroupDisjoint(t *testing.T) {
	for _, profile := range []Profile{ProfileRobust, ProfileBalanced} {
		spec, ok := profileSpecFor(profile)
		if !ok {
			t.Fatalf("profile %v unavailable", profile)
		}
		seen := [2]map[int]bool{make(map[int]bool), make(map[int]bool)}
		pairTotal := 0
		for fold := 0; fold < diagnosticUnwrapCrossfitFolds; fold++ {
			pairs := diagnosticCrossfitRepetitionPairs(profile, fold)
			if len(pairs) == 0 {
				t.Fatalf("profile %v fold %d empty", profile, fold)
			}
			pairTotal += len(pairs)
			for _, pair := range pairs {
				if diagnosticCrossfitGroupFold(v3CodeIndex(pair[0], spec.codedBits)) != fold ||
					diagnosticCrossfitGroupFold(v3CodeIndex(pair[1], spec.codedBits)) != fold {
					t.Fatalf("profile %v pair %v escaped fold %d", profile, pair, fold)
				}
				seen[fold][pair[0]] = true
				seen[fold][pair[1]] = true
			}
		}
		if pairTotal != diagnosticBlindRepetitionPairCount(profile) {
			t.Fatalf("profile %v partition has %d pairs, want %d", profile, pairTotal, diagnosticBlindRepetitionPairCount(profile))
		}
		for pos := range seen[0] {
			if seen[1][pos] {
				t.Fatalf("profile %v logical position %d appears in both folds", profile, pos)
			}
		}
	}
}

func TestDiagnosticCrossfitFoldEstimatorUsesBoundedSubset(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	grid := diagnosticCrossfitSyntheticGrid(ProfileRobust)
	for i := range cells {
		cells[i].grid = append([]float64(nil), grid...)
	}
	for fold := 0; fold < diagnosticUnwrapCrossfitFolds; fold++ {
		got, ok := diagnosticEstimateBlindRepetitionPhaseCrossfitFold(cells, grid, fold)
		if !ok {
			t.Fatalf("fold %d estimator unavailable", fold)
		}
		t.Logf("fold %d global=(%d,%d) score=%.4f conf=%.4f min=%.4f pairs=%d", fold, got.globalPhaseX, got.globalPhaseY, got.globalScore, got.meanConfidence, got.minConfidence, got.pairs)
		if got.profile != ProfileRobust {
			t.Fatalf("fold %d profile=%v want robust", fold, got.profile)
		}
		wantPairs := len(diagnosticCrossfitRepetitionPairs(ProfileRobust, fold))
		if got.pairs != wantPairs {
			t.Fatalf("fold %d pairs=%d want=%d", fold, got.pairs, wantPairs)
		}
		if len(got.controls) != len(cells) {
			t.Fatalf("fold %d controls=%d want=%d", fold, len(got.controls), len(cells))
		}
	}
}

func TestDiagnosticCrossfitTop2HeldoutSupportsKnownPhase(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	grid := diagnosticCrossfitSyntheticGrid(ProfileRobust)
	for i := range cells {
		cells[i].grid = append([]float64(nil), grid...)
	}
	primary, ok := diagnosticEstimateBlindRepetitionPhaseCrossfitFold(cells, grid, 0)
	if !ok {
		t.Fatal("proposal fold unavailable")
	}
	bestX, bestY := make([]int, len(cells)), make([]int, len(cells))
	secondX, secondY := make([]int, len(cells)), make([]int, len(cells))
	secondX[4] = 1
	heldout := diagnosticCrossfitRepetitionPairs(primary.profile, 1)
	used, delta, ok := diagnosticValidateCrossfitTop2(primary, cells, bestX, bestY, secondX, secondY, heldout)
	if !ok || used != 1 {
		t.Fatalf("held-out validation unavailable: ok=%v cells=%d", ok, used)
	}
	if delta <= 0 {
		t.Fatalf("held-out fold preferred wrong shifted phase: delta=%f", delta)
	}
}

func TestDiagnosticGlobalUnwrapCrossfitSyntheticTwoDirections(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	grid := diagnosticCrossfitSyntheticGrid(ProfileRobust)
	for i := range cells {
		dx := 2 * (cells[i].RegionX - 1)
		dy := 2 * (cells[i].RegionY - 1)
		cells[i].grid = diagnosticCrossfitShiftGrid(grid, dx, dy)
	}
	lattice := diagnosticBlindPhaseResult{method: "local-lattice-fractional-phase", controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.8, minConfidence: 0.8}
	for i := range lattice.controls {
		lattice.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.8}
	}
	got := diagnosticGlobalUnwrapCrossfit(cells, grid, lattice)
	if !got.available {
		t.Fatalf("crossfit unavailable: A=%+v B=%+v", got.aToB, got.bToA)
	}
	if !got.aToB.available || !got.bToA.available {
		t.Fatalf("crossfit directions incomplete: A=%v B=%v", got.aToB.available, got.bToA.available)
	}
	if got.aToB.proposalPairs == 0 || got.bToA.proposalPairs == 0 || got.aToB.validationPairs == 0 || got.bToA.validationPairs == 0 {
		t.Fatalf("crossfit pair budgets missing: A=%+v B=%+v", got.aToB, got.bToA)
	}
	if got.aToB.evaluatedStates == 0 || got.bToA.evaluatedStates == 0 {
		t.Fatalf("crossfit exact solver did not run: A=%d B=%d", got.aToB.evaluatedStates, got.bToA.evaluatedStates)
	}
}

func diagnosticCrossfitShiftGrid(grid []float64, dx, dy int) []float64 {
	shifted := make([]float64, len(grid))
	for y := 0; y < tileHeight; y++ {
		for x := 0; x < tileWidth; x++ {
			sx := positiveMod(x+dx, tileWidth)
			sy := positiveMod(y+dy, tileHeight)
			shifted[sy*tileWidth+sx] = grid[y*tileWidth+x]
		}
	}
	return shifted
}

func diagnosticCrossfitSyntheticGrid(profile Profile) []float64 {
	spec, _ := profileSpecFor(profile)
	grid := make([]float64, eccBits)
	for pos := 0; pos < eccBits; pos++ {
		code := v3CodeIndex(pos, spec.codedBits)
		value := 1.0
		if code%2 != 0 {
			value = -1
		}
		// Deterministic sign noise lowers confidence enough for the integer
		// solver to remain eligible while preserving a unique repetition phase.
		if (pos*37+code*11+5)%13 < 2 {
			value = -value
		}
		grid[pos] = value
	}
	return grid
}

func TestDiagnosticApplyCrossfitSpatialEvidenceReportsPerCellCycles(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	grid := diagnosticCrossfitSyntheticGrid(ProfileRobust)
	for i := range cells {
		dx := 2 * (cells[i].RegionX - 1)
		dy := 2 * (cells[i].RegionY - 1)
		cells[i].grid = diagnosticCrossfitShiftGrid(grid, dx, dy)
	}
	lattice := diagnosticBlindPhaseResult{method: "local-lattice-fractional-phase", controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.8, minConfidence: 0.8}
	for i := range lattice.controls {
		lattice.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.8}
	}
	crossfit := diagnosticGlobalUnwrapCrossfit(cells, grid, lattice)
	if !crossfit.available {
		t.Fatal("synthetic crossfit unavailable")
	}
	spatial := &DiagnosticSpatialBitEvidence{}
	for _, cell := range cells {
		spatial.CellsEvidence = append(spatial.CellsEvidence, DiagnosticSpatialCellEvidence{RegionX: cell.RegionX, RegionY: cell.RegionY})
	}
	diagnosticApplyCrossfitSpatialEvidence(spatial, crossfit, cells)
	if !spatial.BlindGlobalUnwrapCrossfitAvailable || spatial.BlindGlobalUnwrapCrossfitComparedCells == 0 {
		t.Fatalf("aggregate crossfit telemetry missing: %+v", spatial)
	}
	availableA, availableB, agreements := 0, 0, 0
	for _, cell := range spatial.CellsEvidence {
		if cell.BlindCrossfitAToBAvailable {
			availableA++
			if cell.BlindCrossfitAToBConfidence <= 0 {
				t.Fatalf("A->B confidence missing for cell (%d,%d)", cell.RegionX, cell.RegionY)
			}
		}
		if cell.BlindCrossfitBToAAvailable {
			availableB++
			if cell.BlindCrossfitBToAConfidence <= 0 {
				t.Fatalf("B->A confidence missing for cell (%d,%d)", cell.RegionX, cell.RegionY)
			}
		}
		if cell.BlindCrossfitCycleAgreement {
			agreements++
		}
	}
	if availableA == 0 || availableB == 0 {
		t.Fatalf("per-cell crossfit telemetry incomplete: A=%d B=%d", availableA, availableB)
	}
	if agreements != spatial.BlindGlobalUnwrapCrossfitAgreementCells {
		t.Fatalf("agreement mismatch: cells=%d aggregate=%d", agreements, spatial.BlindGlobalUnwrapCrossfitAgreementCells)
	}
}

func TestDiagnosticStabilityPartitionsAreGroupDisjointAndDiverse(t *testing.T) {
	for _, profile := range []Profile{ProfileRobust, ProfileBalanced} {
		spec, ok := profileSpecFor(profile)
		if !ok {
			t.Fatalf("profile %v unavailable", profile)
		}
		patterns := make(map[string]bool)
		canonicalPatterns := make(map[string]bool)
		for partition := 0; partition < diagnosticUnwrapStabilityPartitions; partition++ {
			seen := [2]map[int]bool{make(map[int]bool), make(map[int]bool)}
			pairTotal := 0
			pattern := make([]byte, spec.codedBits)
			for code := 0; code < spec.codedBits; code++ {
				pattern[code] = byte('0' + diagnosticStabilityGroupFold(code, partition))
			}
			patterns[string(pattern)] = true
			complement := make([]byte, len(pattern))
			for i, bit := range pattern {
				if bit == '0' {
					complement[i] = '1'
				} else {
					complement[i] = '0'
				}
			}
			canonical := string(pattern)
			if string(complement) < canonical {
				canonical = string(complement)
			}
			canonicalPatterns[canonical] = true
			for fold := 0; fold < 2; fold++ {
				pairs := diagnosticStabilityRepetitionPairs(profile, partition, fold)
				if len(pairs) == 0 {
					t.Fatalf("profile %v partition %d fold %d empty", profile, partition, fold)
				}
				pairTotal += len(pairs)
				for _, pair := range pairs {
					if diagnosticStabilityGroupFold(v3CodeIndex(pair[0], spec.codedBits), partition) != fold ||
						diagnosticStabilityGroupFold(v3CodeIndex(pair[1], spec.codedBits), partition) != fold {
						t.Fatalf("profile %v partition %d pair %v escaped fold %d", profile, partition, pair, fold)
					}
					seen[fold][pair[0]] = true
					seen[fold][pair[1]] = true
				}
			}
			if pairTotal != diagnosticBlindRepetitionPairCount(profile) {
				t.Fatalf("profile %v partition %d has %d pairs, want %d", profile, partition, pairTotal, diagnosticBlindRepetitionPairCount(profile))
			}
			for pos := range seen[0] {
				if seen[1][pos] {
					t.Fatalf("profile %v partition %d logical position %d appears in both folds", profile, partition, pos)
				}
			}
		}
		if len(patterns) < diagnosticUnwrapStabilityPartitions/2 {
			t.Fatalf("profile %v produced only %d distinct partition patterns", profile, len(patterns))
		}
		if len(canonicalPatterns) != diagnosticUnwrapStabilityPartitions {
			t.Fatalf("profile %v produced complement-equivalent partitions: %d canonical patterns for %d partitions", profile, len(canonicalPatterns), diagnosticUnwrapStabilityPartitions)
		}
	}
}

func TestDiagnosticGlobalUnwrapPartitionStabilitySynthetic(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	grid := diagnosticCrossfitSyntheticGrid(ProfileRobust)
	for i := range cells {
		dx := 2 * (cells[i].RegionX - 1)
		dy := 2 * (cells[i].RegionY - 1)
		cells[i].grid = diagnosticCrossfitShiftGrid(grid, dx, dy)
	}
	lattice := diagnosticBlindPhaseResult{method: "local-lattice-fractional-phase", controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.8, minConfidence: 0.8}
	for i := range lattice.controls {
		lattice.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.8}
	}
	got := diagnosticGlobalUnwrapPartitionStability(cells, grid, lattice)
	if !got.available {
		t.Fatal("partition stability unavailable")
	}
	t.Logf("trials=%d/%d supported=%d uniqueFields=%d modalField=%.3f meanCell=%.3f minCell=%.3f supportedMean=%.3f pairwise=%.3f", got.trialsAvailable, got.trialsRequested, got.trialsSupported, got.uniqueFields, got.modalFieldFraction, got.meanCellModalFraction, got.minCellModalFraction, got.supportedMeanCellModalFraction, got.meanPairwiseAgreementFraction)
	if got.trialsAvailable < diagnosticUnwrapStabilityPartitions {
		t.Fatalf("too few available stability trials: %d", got.trialsAvailable)
	}
	if got.evaluatedStates == 0 || got.comparedCells != len(cells) {
		t.Fatalf("stability telemetry incomplete: states=%d cells=%d", got.evaluatedStates, got.comparedCells)
	}
	if got.meanCellModalFraction <= 0 || got.minCellModalFraction <= 0 {
		t.Fatalf("invalid modal fractions: mean=%f min=%f", got.meanCellModalFraction, got.minCellModalFraction)
	}
}

func TestDiagnosticApplyStabilitySpatialEvidenceReportsPerCellModes(t *testing.T) {
	cells := diagnosticTestSpatialCells9()
	grid := diagnosticCrossfitSyntheticGrid(ProfileRobust)
	for i := range cells {
		dx := 2 * (cells[i].RegionX - 1)
		dy := 2 * (cells[i].RegionY - 1)
		cells[i].grid = diagnosticCrossfitShiftGrid(grid, dx, dy)
	}
	lattice := diagnosticBlindPhaseResult{method: "local-lattice-fractional-phase", controls: make([]diagnosticBlindCellControl, len(cells)), meanConfidence: 0.8, minConfidence: 0.8}
	for i := range lattice.controls {
		lattice.controls[i] = diagnosticBlindCellControl{available: true, confidence: 0.8}
	}
	stability := diagnosticGlobalUnwrapPartitionStability(cells, grid, lattice)
	if !stability.available {
		t.Fatal("synthetic stability unavailable")
	}
	spatial := &DiagnosticSpatialBitEvidence{}
	for _, cell := range cells {
		spatial.CellsEvidence = append(spatial.CellsEvidence, DiagnosticSpatialCellEvidence{RegionX: cell.RegionX, RegionY: cell.RegionY})
	}
	diagnosticApplyStabilitySpatialEvidence(spatial, stability, cells)
	if !spatial.BlindGlobalUnwrapStabilityAvailable || spatial.BlindGlobalUnwrapStabilityTrialsAvailable == 0 {
		t.Fatalf("aggregate stability telemetry missing: %+v", spatial)
	}
	available := 0
	for _, cell := range spatial.CellsEvidence {
		if !cell.BlindStabilityAvailable {
			continue
		}
		available++
		if cell.BlindStabilityObservations == 0 || cell.BlindStabilityModalCount == 0 || cell.BlindStabilityModalFraction <= 0 {
			t.Fatalf("cell stability telemetry incomplete at (%d,%d): %+v", cell.RegionX, cell.RegionY, cell)
		}
	}
	if available != len(cells) {
		t.Fatalf("per-cell stability telemetry available=%d want=%d", available, len(cells))
	}
}
