package watermark

import (
	"math"
	"testing"
)

func TestDiagnosticBitChannelExactKnownHeaderAndECC(t *testing.T) {
	key := []byte("diagnostic-bit-key")
	spec, ok := profileSpecFor(ProfileRobust)
	if !ok {
		t.Fatal("robust profile missing")
	}
	frame := makeV3Frame([]byte("bit channel"), key, spec)
	protected := hammingEncode(whiten(bytesToBits(frame), key, v3WhitenLabel))
	grid := diagnosticGridFromProtected(protected, spec.codedBits)

	evidence, ok := diagnosticAnalyzeBitChannel(grid, key, newDecoder(key), "synthetic", diagnosticPhotometricRaw)
	if !ok {
		t.Fatal("bit diagnostic rejected exact synthetic grid")
	}
	if evidence.Profile != ProfileRobust {
		t.Fatalf("profile=%s want robust", evidence.Profile)
	}
	if evidence.KnownHeaderCodedErrors != 0 || evidence.KnownHeaderMultiWords != 0 || evidence.PostECCHdrErrors != 0 {
		t.Fatalf("unexpected known-header errors: coded=%d multi=%d post-ecc=%d", evidence.KnownHeaderCodedErrors, evidence.KnownHeaderMultiWords, evidence.PostECCHdrErrors)
	}
	if evidence.KnownHeaderCleanWords != 6 || evidence.KnownHeaderOneBitWords != 0 {
		t.Fatalf("known header words clean=%d one-bit=%d want 6/0", evidence.KnownHeaderCleanWords, evidence.KnownHeaderOneBitWords)
	}
	if evidence.SyndromeWords != 0 || evidence.SyndromeWordFraction != 0 {
		t.Fatalf("exact protected frame has syndrome words=%d fraction=%f", evidence.SyndromeWords, evidence.SyndromeWordFraction)
	}
}

func TestDiagnosticBitChannelIdentifiesKnownHeaderBeyondHammingCapacity(t *testing.T) {
	key := []byte("diagnostic-bit-key")
	spec, ok := profileSpecFor(ProfileRobust)
	if !ok {
		t.Fatal("robust profile missing")
	}
	frame := makeV3Frame([]byte("bit channel"), key, spec)
	protected := hammingEncode(whiten(bytesToBits(frame), key, v3WhitenLabel))
	grid := diagnosticGridFromProtected(protected, spec.codedBits)

	// Flip two protected bits in the first known Hamming word. Hamming(7,4)
	// can correct one bit per word, so this word is provably beyond its design
	// correction radius even though the hidden payload remains unknown.
	for position := 0; position < eccBits; position++ {
		index := v3CodeIndex(position, spec.codedBits)
		if index == 0 || index == 1 {
			grid[position] = -grid[position]
		}
	}

	evidence, ok := diagnosticAnalyzeBitChannel(grid, key, newDecoder(key), "synthetic", diagnosticPhotometricRaw)
	if !ok {
		t.Fatal("bit diagnostic rejected perturbed synthetic grid")
	}
	if evidence.Profile != ProfileRobust {
		t.Fatalf("profile=%s want robust", evidence.Profile)
	}
	if evidence.KnownHeaderCodedErrors != 2 {
		t.Fatalf("known coded errors=%d want 2", evidence.KnownHeaderCodedErrors)
	}
	if evidence.KnownHeaderMultiWords != 1 {
		t.Fatalf("multi-error known words=%d want 1", evidence.KnownHeaderMultiWords)
	}
	if evidence.PostECCHdrErrors == 0 {
		t.Fatal("two errors in one known Hamming word unexpectedly produced an exact post-ECC header")
	}
}

func diagnosticGridFromProtected(protected []byte, codedBits int) []float64 {
	grid := make([]float64, eccBits)
	for position := 0; position < eccBits; position++ {
		index := v3CodeIndex(position, codedBits)
		if protected[index] == 1 {
			grid[position] = 10
		} else {
			grid[position] = -10
		}
	}
	return grid
}

func TestDiagnosticSoftHammingUsesReliabilityToRecoverDoubleError(t *testing.T) {
	input := []byte{1, 0, 1, 0}
	encoded := hammingEncode(input)
	margins := make([]float64, 7)
	for i, bit := range encoded {
		if bit != 0 {
			margins[i] = 10
		} else {
			margins[i] = -10
		}
	}
	// Two weak sign errors put the hard word outside Hamming(7,4)'s guaranteed
	// one-bit radius. Weighted maximum-likelihood should still prefer the codeword
	// supported by the five strong observations.
	margins[0] = -margins[0] / 100
	margins[1] = -margins[1] / 100
	hard := make([]byte, 7)
	for i := range hard {
		if margins[i] >= 0 {
			hard[i] = 1
		}
	}
	hardDecoded := hammingDecode(hard)
	if string(hardDecoded) == string(input) {
		t.Fatal("crafted double-error word unexpectedly succeeded with hard Hamming")
	}
	soft, changed := diagnosticSoftHammingDecodeMargins(margins)
	if string(soft) != string(input) {
		t.Fatalf("soft decoded=%v want %v", soft, input)
	}
	if changed != 1 {
		t.Fatalf("soft changed words=%d want 1", changed)
	}
}

func TestDiagnosticReliabilityGateIsMessageIndependentAndConservative(t *testing.T) {
	good := DiagnosticBitChannelEvidence{PostECCHdrErrors: 2, KnownHeaderMultiWords: 1, KnownWrongMarginRatio: 0.67}
	if !diagnosticShouldUseReliabilityDecode(good) {
		t.Fatal("recoverable weak-error prefix did not enable reliability decode")
	}
	for _, bad := range []DiagnosticBitChannelEvidence{
		{PostECCHdrErrors: 0, KnownHeaderMultiWords: 0, KnownWrongMarginRatio: 0.5},
		{PostECCHdrErrors: 2, KnownHeaderMultiWords: 2, KnownWrongMarginRatio: 0.5},
		{PostECCHdrErrors: 2, KnownHeaderMultiWords: 1, KnownWrongMarginRatio: 1.1},
	} {
		if diagnosticShouldUseReliabilityDecode(bad) {
			t.Fatalf("unsafe reliability gate accepted %+v", bad)
		}
	}
}

func TestDiagnosticFullGridSamplingIsBounded(t *testing.T) {
	calls := 0
	grid, stats, ok := diagnosticProjectiveGridSampled(4000, 4000, func(originX, originY float64) (float64, bool) {
		calls++
		bx := int(originX) / blockSize
		by := int(originY) / blockSize
		position := (by%tileHeight)*tileWidth + bx%tileWidth
		if position%2 == 0 {
			return 10, true
		}
		return -10, true
	})
	if !ok || len(grid) != eccBits {
		t.Fatalf("bounded grid unavailable ok=%t len=%d", ok, len(grid))
	}
	if !stats.Bounded {
		t.Fatal("large canonical grid did not enter bounded sampling")
	}
	if stats.SampledTiles > diagnosticMaxSampledTiles || stats.SampledBlocks > diagnosticMaxSampledTiles*eccBits {
		t.Fatalf("sampling escaped budget: %+v", stats)
	}
	if calls != stats.SampledBlocks {
		t.Fatalf("reader calls=%d sampled=%d", calls, stats.SampledBlocks)
	}
	if stats.SampledBlocks >= (4000/blockSize)*(4000/blockSize) {
		t.Fatalf("bounded sampler did not reduce work: %+v", stats)
	}
}

func TestDiagnosticReliabilityDecoderAuthenticatesWeakDoubleErrorFrame(t *testing.T) {
	key := []byte("diagnostic-soft-auth-key")
	spec, ok := profileSpecFor(ProfileRobust)
	if !ok {
		t.Fatal("robust profile missing")
	}
	message := []byte("soft-auth")
	frame := makeV3Frame(message, key, spec)
	protected := hammingEncode(whiten(bytesToBits(frame), key, v3WhitenLabel))
	grid := diagnosticGridFromProtected(protected, spec.codedBits)
	// Two low-confidence errors in the first protected Hamming word are beyond
	// hard Hamming's one-bit correction radius while leaving five strong bits in
	// the word supporting the original codeword.
	for position := 0; position < eccBits; position++ {
		index := v3CodeIndex(position, spec.codedBits)
		if index == 0 || index == 1 {
			if grid[position] >= 0 {
				grid[position] = -0.1
			} else {
				grid[position] = 0.1
			}
		}
	}
	decoder := newDecoder(key)
	if _, _, _, found := decoder.decodeGrid(grid); found {
		t.Fatal("hard diagnostic decode unexpectedly authenticated weak double-error frame")
	}
	evidence, ok := diagnosticAnalyzeBitChannel(grid, key, decoder, "synthetic", diagnosticPhotometricRaw)
	if !ok || !evidence.ReliabilityDecodeEligible {
		t.Fatalf("reliability gate not enabled: ok=%t evidence=%+v", ok, evidence)
	}
	payload, info, _, found := diagnosticDecodeGridReliabilityAware(grid, key, decoder)
	if !found {
		t.Fatalf("soft reliability decoder did not authenticate; evidence=%+v", evidence)
	}
	if string(payload) != string(message) || info.Profile != ProfileRobust {
		t.Fatalf("payload=%q profile=%s want %q/robust", payload, info.Profile, message)
	}
}

func TestDiagnosticSpatialBitEvidenceSeparatesLocalAndSystematicErrors(t *testing.T) {
	key := []byte("diagnostic-spatial-key")
	spec, ok := profileSpecFor(ProfileRobust)
	if !ok {
		t.Fatal("robust profile missing")
	}
	frame := makeV3Frame([]byte("spatial"), key, spec)
	protected := hammingEncode(whiten(bytesToBits(frame), key, v3WhitenLabel))
	baseGrid := diagnosticGridFromProtected(protected, spec.codedBits)
	decoder := newDecoder(key)
	evidence, ok := diagnosticAnalyzeBitChannel(baseGrid, key, decoder, "synthetic", diagnosticPhotometricRaw)
	if !ok {
		t.Fatal("exact spatial seed rejected")
	}

	makeCells := func(systematic bool) []diagnosticSpatialGridCell {
		cells := make([]diagnosticSpatialGridCell, 9)
		for i := range cells {
			cells[i] = diagnosticSpatialGridCell{RegionX: i % 3, RegionY: i / 3, grid: append([]float64(nil), baseGrid...)}
		}
		for i := range cells {
			if !systematic && i != 0 {
				continue
			}
			flipDiagnosticCodedBitInGrid(cells[i].grid, evidence.PhaseX, evidence.PhaseY, spec.codedBits, 0)
		}
		return cells
	}

	local := evidence
	diagnosticAttachSpatialBitEvidence(&local, makeCells(false), key)
	if local.Spatial == nil {
		t.Fatal("local spatial evidence missing")
	}
	if local.Spatial.KnownHeaderStableWrongBits != 0 || local.Spatial.KnownHeaderMixedBits != 1 {
		t.Fatalf("local classification stable-wrong=%d mixed=%d want 0/1", local.Spatial.KnownHeaderStableWrongBits, local.Spatial.KnownHeaderMixedBits)
	}
	if local.Spatial.KnownHeaderMajorityCodedErrors != 0 {
		t.Fatalf("local majority errors=%d want 0", local.Spatial.KnownHeaderMajorityCodedErrors)
	}

	systematic := evidence
	diagnosticAttachSpatialBitEvidence(&systematic, makeCells(true), key)
	if systematic.Spatial == nil {
		t.Fatal("systematic spatial evidence missing")
	}
	if systematic.Spatial.KnownHeaderStableWrongBits != 1 || systematic.Spatial.KnownHeaderMixedBits != 0 {
		t.Fatalf("systematic classification stable-wrong=%d mixed=%d want 1/0", systematic.Spatial.KnownHeaderStableWrongBits, systematic.Spatial.KnownHeaderMixedBits)
	}
	if systematic.Spatial.KnownHeaderMajorityCodedErrors != 1 {
		t.Fatalf("systematic majority errors=%d want 1", systematic.Spatial.KnownHeaderMajorityCodedErrors)
	}
}

func flipDiagnosticCodedBitInGrid(grid []float64, phaseX, phaseY, codedBits, codedIndex int) {
	for logicalPosition := 0; logicalPosition < eccBits; logicalPosition++ {
		if v3CodeIndex(logicalPosition, codedBits) != codedIndex {
			continue
		}
		logicalX := logicalPosition % tileWidth
		logicalY := logicalPosition / tileWidth
		observedX := positiveMod(logicalX-phaseX, tileWidth)
		observedY := positiveMod(logicalY-phaseY, tileHeight)
		position := observedY*tileWidth + observedX
		grid[position] = -grid[position]
	}
}

func TestDiagnosticFullGridSamplingCollectsSpatialCellsWithoutExtraReads(t *testing.T) {
	calls := 0
	grid, stats, ok := diagnosticProjectiveGridSampled(1800, 1800, func(originX, originY float64) (float64, bool) {
		calls++
		position := ((int(originY)/blockSize)%tileHeight)*tileWidth + (int(originX)/blockSize)%tileWidth
		if position%2 == 0 {
			return 10, true
		}
		return -10, true
	})
	if !ok || len(grid) != eccBits {
		t.Fatalf("grid unavailable ok=%t len=%d", ok, len(grid))
	}
	if len(stats.spatialCells) < 4 {
		t.Fatalf("spatial cells=%d want at least 4", len(stats.spatialCells))
	}
	if calls != stats.SampledBlocks {
		t.Fatalf("spatial accounting added reader calls: calls=%d sampled=%d", calls, stats.SampledBlocks)
	}
}

func TestDiagnosticSpatialPhaseRefinementStaysWithinBoundedNeighborhood(t *testing.T) {
	key := []byte("diagnostic-spatial-phase-key")
	spec, ok := profileSpecFor(ProfileRobust)
	if !ok {
		t.Fatal("robust profile missing")
	}
	frame := makeV3Frame([]byte("phase"), key, spec)
	protected := hammingEncode(whiten(bytesToBits(frame), key, v3WhitenLabel))
	grid := diagnosticGridFromProtected(protected, spec.codedBits)
	pattern := newV3SyncPattern(key, spec)
	exact := strongestV3Phases(grid, pattern, 1)
	if len(exact) != 1 {
		t.Fatal("exact phase missing")
	}
	referenceX := positiveMod(exact[0].x+1, tileWidth)
	referenceY := positiveMod(exact[0].y-1, tileHeight)
	local, ok := diagnosticBestV3PhaseNear(grid, pattern, referenceX, referenceY, diagnosticSpatialPhaseRadius)
	if !ok {
		t.Fatal("bounded local phase missing")
	}
	if local.x != exact[0].x || local.y != exact[0].y {
		t.Fatalf("local phase=(%d,%d) want exact=(%d,%d)", local.x, local.y, exact[0].x, exact[0].y)
	}
	if absInt(diagnosticSignedPhaseDelta(local.x, referenceX, tileWidth)) > diagnosticSpatialPhaseRadius ||
		absInt(diagnosticSignedPhaseDelta(local.y, referenceY, tileHeight)) > diagnosticSpatialPhaseRadius {
		t.Fatalf("local phase escaped radius: local=(%d,%d) reference=(%d,%d)", local.x, local.y, referenceX, referenceY)
	}
}

func TestDiagnosticTilePositionAgreementIsKeyIndependent(t *testing.T) {
	cells := make([]diagnosticSpatialGridCell, 3)
	for i := range cells {
		cells[i] = diagnosticSpatialGridCell{RegionX: i, RegionY: 0, grid: make([]float64, eccBits)}
		for p := range cells[i].grid {
			if p%2 == 0 {
				cells[i].grid[p] = 10
			} else {
				cells[i].grid[p] = -10
			}
		}
	}
	mean, unstable, fraction := diagnosticTilePositionAgreement(cells)
	if mean != 1 || unstable != 0 || fraction != 0 {
		t.Fatalf("exact spatial agreement mean=%f unstable=%d fraction=%f", mean, unstable, fraction)
	}
	cells[0].grid[17] = -cells[0].grid[17]
	mean, unstable, fraction = diagnosticTilePositionAgreement(cells)
	if mean >= 1 || unstable != 1 || fraction <= 0 {
		t.Fatalf("one local disagreement not detected: mean=%f unstable=%d fraction=%f", mean, unstable, fraction)
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func TestDiagnosticBlindPhaseBudgetsStayBounded(t *testing.T) {
	wantGlobal := 2 * eccBits
	if diagnosticMaxBlindGlobalPhaseProbes != wantGlobal {
		t.Fatalf("blind global probes=%d, want %d", diagnosticMaxBlindGlobalPhaseProbes, wantGlobal)
	}
	wantLocal := diagnosticSpatialGridAxis * diagnosticSpatialGridAxis * 25
	if diagnosticMaxBlindLocalPhaseProbes != wantLocal {
		t.Fatalf("blind local probes=%d, want %d", diagnosticMaxBlindLocalPhaseProbes, wantLocal)
	}
	wantGuided := 36 * 9
	if diagnosticMaxBlindGuidedPairwiseProbes != wantGuided {
		t.Fatalf("blind guided pairwise probes=%d, want %d", diagnosticMaxBlindGuidedPairwiseProbes, wantGuided)
	}
	wantPairwise := 36 * 81
	if diagnosticMaxBlindPairwiseFallbackProbes != wantPairwise {
		t.Fatalf("blind pairwise fallback probes=%d, want %d", diagnosticMaxBlindPairwiseFallbackProbes, wantPairwise)
	}
}

func TestDiagnosticBlindSpatialPhaseRecoversAffineRelativeDriftWithoutKey(t *testing.T) {
	base := make([]float64, eccBits)
	state := uint32(0x12345678)
	for i := range base {
		state = state*1664525 + 1013904223
		magnitude := 0.8 + float64((state>>8)&255)/255.0
		if state&1 != 0 {
			base[i] = magnitude
		} else {
			base[i] = -magnitude
		}
	}
	cells := make([]diagnosticSpatialGridCell, 0, 9)
	expectedX := make([]float64, 0, 9)
	expectedY := make([]float64, 0, 9)
	for ry := 0; ry < 3; ry++ {
		for rx := 0; rx < 3; rx++ {
			nx := 2*(float64(rx)+0.5)/3 - 1
			ny := 2*(float64(ry)+0.5)/3 - 1
			ox := 1.20*nx - 0.60*ny
			oy := -0.50*nx + 1.00*ny
			cells = append(cells, diagnosticSpatialGridCell{RegionX: rx, RegionY: ry, grid: diagnosticPeriodicBilinearGrid(base, ox, oy)})
			expectedX = append(expectedX, ox)
			expectedY = append(expectedY, oy)
		}
	}
	blind, ok := diagnosticEstimateBlindSpatialPhase(cells, nil)
	if !ok || len(blind.controls) != 9 {
		t.Fatalf("blind phase unavailable: ok=%t controls=%d pairs=%d", ok, len(blind.controls), blind.pairs)
	}
	if blind.pairs < 20 {
		t.Fatalf("blind pair support=%d, want broad graph support", blind.pairs)
	}
	maxError := 0.0
	for i, control := range blind.controls {
		if !control.available {
			t.Fatalf("blind control %d unavailable", i)
		}
		err := math.Hypot(control.offsetX-expectedX[i], control.offsetY-expectedY[i])
		if err > maxError {
			maxError = err
		}
	}
	if maxError > 0.45 {
		t.Fatalf("blind phase maximum control error=%.3f blocks", maxError)
	}
}

func TestDiagnosticBlindSmoothFieldUsesNoKeyAssistedControls(t *testing.T) {
	spatial := &DiagnosticSpatialBitEvidence{}
	for ry := 0; ry < 3; ry++ {
		for rx := 0; rx < 3; rx++ {
			nx := 2*(float64(rx)+0.5)/3 - 1
			ny := 2*(float64(ry)+0.5)/3 - 1
			observedX := -(0.10 + 0.45*nx - 0.20*ny)
			observedY := -(-0.05 + 0.15*nx + 0.35*ny)
			spatial.CellsEvidence = append(spatial.CellsEvidence, DiagnosticSpatialCellEvidence{
				RegionX: rx, RegionY: ry,
				// Deliberately contradictory key-assisted oracle fields. The blind
				// fitter must ignore them completely.
				LocalPhaseSyncFraction:      1,
				LocalPhaseSubblockAvailable: true,
				LocalPhaseSubblockOffsetX:   2,
				LocalPhaseSubblockOffsetY:   -2,
				LocalPhaseConfidence:        1,
				BlindPhaseAvailable:         true,
				BlindPhaseOffsetX:           observedX,
				BlindPhaseOffsetY:           observedY,
				BlindPhaseConfidence:        0.9,
			})
		}
	}
	_, smooth, ok := diagnosticFitBlindSmoothPhaseField(spatial, 1600, 1200)
	if !ok || !smooth.Available || !smooth.DecodeEligible {
		t.Fatalf("blind smooth field unavailable: %+v", smooth)
	}
	if smooth.ControlSource != "blind-self-registration" {
		t.Fatalf("control source=%q, want blind-self-registration", smooth.ControlSource)
	}
	if math.Abs(smooth.CorrectionXCoefficients[0]-0.10) > 1e-6 ||
		math.Abs(smooth.CorrectionXCoefficients[1]-0.45) > 1e-6 ||
		math.Abs(smooth.CorrectionXCoefficients[2]+0.20) > 1e-6 {
		t.Fatalf("blind x coefficients=%v", smooth.CorrectionXCoefficients)
	}
}

func TestDiagnosticBlindRepetitionFindsPhaseWithoutKey(t *testing.T) {
	spec, ok := profileSpecFor(ProfileRobust)
	if !ok {
		t.Fatal("robust profile missing")
	}
	coded := make([]byte, spec.codedBits)
	state := uint32(0x9e3779b9)
	for i := range coded {
		state = state*1664525 + 1013904223
		coded[i] = byte((state >> 31) & 1)
	}
	phaseX, phaseY := 7, 11
	grid := make([]float64, eccBits)
	for logicalPosition := 0; logicalPosition < eccBits; logicalPosition++ {
		logicalX := logicalPosition % tileWidth
		logicalY := logicalPosition / tileWidth
		observedX := positiveMod(logicalX-phaseX, tileWidth)
		observedY := positiveMod(logicalY-phaseY, tileHeight)
		value := -10.0
		if coded[v3CodeIndex(logicalPosition, spec.codedBits)] != 0 {
			value = 10
		}
		grid[observedY*tileWidth+observedX] = value
	}
	peak, ok := diagnosticBlindBestRepetitionGlobal(grid)
	if !ok {
		t.Fatal("blind repetition phase unavailable")
	}
	if peak.profile != ProfileRobust || peak.phaseX != phaseX || peak.phaseY != phaseY {
		t.Fatalf("blind phase=%s (%d,%d), want robust (%d,%d), score=%.3f", peak.profile, peak.phaseX, peak.phaseY, phaseX, phaseY, peak.peak)
	}
	if peak.peak < 0.99 {
		t.Fatalf("exact repetition score=%.6f, want near 1", peak.peak)
	}
}

func TestDiagnosticBlindRepetitionCapacityHasNoStructuralEvidence(t *testing.T) {
	grid := make([]float64, eccBits)
	for i := range grid {
		if i%3 == 0 {
			grid[i] = 1
		} else {
			grid[i] = -1
		}
	}
	if score := diagnosticBlindRepetitionScore(grid, ProfileCapacity, 0, 0); score != 0 {
		t.Fatalf("capacity blind repetition score=%f, want 0 without repeated coded positions", score)
	}
}

func TestDiagnosticBlindConsensusUsesIndependentSecondaryObserver(t *testing.T) {
	spec, ok := profileSpecFor(ProfileRobust)
	if !ok {
		t.Fatal("robust profile missing")
	}
	coded := make([]byte, spec.codedBits)
	state := uint32(0x31415926)
	for i := range coded {
		state = state*1664525 + 1013904223
		coded[i] = byte((state >> 31) & 1)
	}
	base := make([]float64, eccBits)
	phaseX, phaseY := 6, 9
	for logicalPosition := 0; logicalPosition < eccBits; logicalPosition++ {
		logicalX := logicalPosition % tileWidth
		logicalY := logicalPosition / tileWidth
		observedX := positiveMod(logicalX-phaseX, tileWidth)
		observedY := positiveMod(logicalY-phaseY, tileHeight)
		value := -4.0
		if coded[v3CodeIndex(logicalPosition, spec.codedBits)] != 0 {
			value = 4
		}
		base[observedY*tileWidth+observedX] = value
	}

	cells := make([]diagnosticSpatialGridCell, 0, 9)
	aggregate := make([]float64, eccBits)
	for ry := 0; ry < 3; ry++ {
		for rx := 0; rx < 3; rx++ {
			nx := float64(rx - 1)
			ny := float64(ry - 1)
			ox := 0.55*nx - 0.20*ny
			oy := -0.15*nx + 0.45*ny
			grid := diagnosticPeriodicBilinearGrid(base, ox, oy)
			cells = append(cells, diagnosticSpatialGridCell{RegionX: rx, RegionY: ry, grid: grid})
			for i, v := range grid {
				aggregate[i] += v
			}
		}
	}
	for i := range aggregate {
		aggregate[i] /= float64(len(cells))
	}

	blind, ok := diagnosticEstimateBlindSpatialPhase(cells, aggregate)
	if !ok {
		t.Fatal("blind consensus unavailable")
	}
	if blind.secondaryMethod != "cross-cell-guided-correlation" {
		t.Fatalf("secondary method=%q", blind.secondaryMethod)
	}
	if blind.consensusCells < 7 {
		t.Fatalf("consensus cells=%d, want broad support", blind.consensusCells)
	}
	if blind.method != "intra-tile-repetition+guided-cross-cell-consensus" {
		t.Fatalf("method=%q", blind.method)
	}
}

func TestDiagnosticBlindConsensusCorrectsOnlyBoundedCycleSlip(t *testing.T) {
	primary := diagnosticBlindPhaseResult{
		controls: []diagnosticBlindCellControl{
			{available: true, offsetX: 1.05, offsetY: 0.10, confidence: 0.20},
			{available: true, offsetX: -0.40, offsetY: -0.20, confidence: 0.70},
			{available: true, offsetX: 0.10, offsetY: 0.10, confidence: 0.20},
		},
	}
	secondary := diagnosticBlindPhaseResult{
		method:       "cross-cell-guided-correlation",
		meanPairPeak: 0.20,
		controls: []diagnosticBlindCellControl{
			{available: true, offsetX: 0.08, offsetY: 0.12, confidence: 0.20},
			{available: true, offsetX: 0.55, offsetY: -0.18, confidence: 0.20},
			{available: true, offsetX: 0.12, offsetY: 0.08, confidence: 0.20},
		},
	}
	fused := diagnosticFuseBlindObservers(primary, secondary)
	if fused.cycleSlipCorrections != 1 {
		t.Fatalf("cycle-slip corrections=%d, want 1", fused.cycleSlipCorrections)
	}
	if !fused.cycleSlipAdjusted[0] {
		t.Fatal("low-confidence +/-1 cycle slip was not corrected")
	}
	if fused.cycleSlipAdjusted[1] {
		t.Fatal("high-confidence primary control must not be cycle-slip corrected")
	}
}
