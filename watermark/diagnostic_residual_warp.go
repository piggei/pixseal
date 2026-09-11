package watermark

import (
	"image"
	"math"
)

const (
	diagnosticResidualPhaseTilesPerAxis = 3
	diagnosticMaxResidualWarpFits       = 1
	diagnosticResidualMaxCorrectionPx   = 64.0
	diagnosticResidualMinControls       = 7
)

type diagnosticProjectiveMapper struct {
	h                homography
	warp             *diagnosticResidualWarp
	smoothPhaseWarp  *diagnosticResidualWarp
	canonicalOffsetX float64
	canonicalOffsetY float64
}

func (mapper diagnosticProjectiveMapper) mapPoint(x, y float64) (float64, float64, bool) {
	x += mapper.canonicalOffsetX
	y += mapper.canonicalOffsetY
	if mapper.warp != nil {
		dx, dy := mapper.warp.correction(x, y)
		x += dx
		y += dy
	}
	if mapper.smoothPhaseWarp != nil {
		dx, dy := mapper.smoothPhaseWarp.correction(x, y)
		x += dx
		y += dy
	}
	return mapper.h.mapPoint(x, y)
}

type diagnosticResidualControl struct {
	x, y   float64
	dx, dy float64
	weight float64
}

// diagnosticResidualWarp is a bounded smooth correction in canonical carrier
// coordinates. The global projective component stays in the homography; this
// model only absorbs low-order residual drift left after that fit.
type diagnosticResidualWarp struct {
	width, height float64
	dx            [6]float64
	dy            [6]float64
	controls      int
	rmsPixels     float64
	maxPixels     float64
	profile       Profile
	phaseScore    float64
}

func (warp *diagnosticResidualWarp) correction(x, y float64) (float64, float64) {
	if warp == nil || warp.width <= 0 || warp.height <= 0 {
		return 0, 0
	}
	nx := 2*x/warp.width - 1
	ny := 2*y/warp.height - 1
	basis := [6]float64{1, nx, ny, nx * ny, nx * nx, ny * ny}
	dx, dy := 0.0, 0.0
	for i := range basis {
		dx += warp.dx[i] * basis[i]
		dy += warp.dy[i] * basis[i]
	}
	limit := warp.maxPixels
	if limit <= 0 || limit > diagnosticResidualMaxCorrectionPx {
		limit = diagnosticResidualMaxCorrectionPx
	}
	if dx > limit {
		dx = limit
	} else if dx < -limit {
		dx = -limit
	}
	if dy > limit {
		dy = limit
	} else if dy < -limit {
		dy = -limit
	}
	return dx, dy
}

func buildDiagnosticResidualWarp(src image.Image, candidate DiagnosticScaleCandidate, base diagnosticProjectiveMapper, decoder *decoder) (*diagnosticResidualWarp, diagnosticPhaseConsensus, bool) {
	width := int(math.Round(candidate.CanonicalWidthPixels))
	height := int(math.Round(candidate.CanonicalHeightPixels))
	if width < tileWidth*blockSize || height < tileHeight*blockSize {
		return nil, diagnosticPhaseConsensus{}, false
	}
	phase := diagnosticProjectivePhaseConsensusWithMapper(src, width, height, base, decoder, diagnosticResidualPhaseTilesPerAxis)
	if phase.profile == "" || len(phase.observations) < diagnosticResidualMinControls {
		return nil, phase, false
	}
	targetX := diagnosticCircularPhaseTarget(phase.observations, true)
	targetY := diagnosticCircularPhaseTarget(phase.observations, false)
	controls := make([]diagnosticResidualControl, 0, len(phase.observations))
	for _, observation := range phase.observations {
		dx := diagnosticPhaseDifference(observation.phaseX, targetX, tileWidth) * blockSize
		dy := diagnosticPhaseDifference(observation.phaseY, targetY, tileHeight) * blockSize
		if math.Abs(dx) > diagnosticResidualMaxCorrectionPx || math.Abs(dy) > diagnosticResidualMaxCorrectionPx {
			continue
		}
		controls = append(controls, diagnosticResidualControl{
			x:      float64(observation.tileX * tileWidth * blockSize),
			y:      float64(observation.tileY * tileHeight * blockSize),
			dx:     dx,
			dy:     dy,
			weight: math.Max(observation.z, 0.5),
		})
	}
	if len(controls) < diagnosticResidualMinControls {
		return nil, phase, false
	}
	warp, ok := fitDiagnosticResidualWarp(float64(width), float64(height), controls)
	if !ok {
		return nil, phase, false
	}
	warp.profile = phase.profile
	warp.phaseScore = phase.score
	return warp, phase, true
}

func fitDiagnosticResidualWarp(width, height float64, controls []diagnosticResidualControl) (*diagnosticResidualWarp, bool) {
	if width <= 1 || height <= 1 || len(controls) < diagnosticResidualMinControls {
		return nil, false
	}
	var normal [6][6]float64
	var rhsX, rhsY [6]float64
	for _, control := range controls {
		nx := 2*control.x/width - 1
		ny := 2*control.y/height - 1
		basis := [6]float64{1, nx, ny, nx * ny, nx * nx, ny * ny}
		weight := math.Max(control.weight, 0.25)
		for r := 0; r < 6; r++ {
			rhsX[r] += weight * basis[r] * control.dx
			rhsY[r] += weight * basis[r] * control.dy
			for c := 0; c < 6; c++ {
				normal[r][c] += weight * basis[r] * basis[c]
			}
		}
	}
	dx, okX := solveDiagnostic6(normal, rhsX)
	dy, okY := solveDiagnostic6(normal, rhsY)
	if !okX || !okY {
		return nil, false
	}
	warp := &diagnosticResidualWarp{width: width, height: height, dx: dx, dy: dy, controls: len(controls), maxPixels: diagnosticResidualMaxCorrectionPx}
	weightedError, weightSum := 0.0, 0.0
	maxCorrection := 0.0
	for _, control := range controls {
		px, py := warp.correction(control.x, control.y)
		err := math.Hypot(px-control.dx, py-control.dy)
		weight := math.Max(control.weight, 0.25)
		weightedError += weight * err * err
		weightSum += weight
		if magnitude := math.Hypot(px, py); magnitude > maxCorrection {
			maxCorrection = magnitude
		}
	}
	if weightSum <= 0 {
		return nil, false
	}
	warp.rmsPixels = math.Sqrt(weightedError / weightSum)
	warp.maxPixels = math.Min(diagnosticResidualMaxCorrectionPx, math.Max(8, maxCorrection+8))
	return warp, true
}

func solveDiagnostic6(matrix [6][6]float64, rhs [6]float64) ([6]float64, bool) {
	var augmented [6][7]float64
	for r := 0; r < 6; r++ {
		copy(augmented[r][:6], matrix[r][:])
		augmented[r][6] = rhs[r]
	}
	for col := 0; col < 6; col++ {
		pivot := col
		for row := col + 1; row < 6; row++ {
			if math.Abs(augmented[row][col]) > math.Abs(augmented[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(augmented[pivot][col]) < 1e-9 {
			return [6]float64{}, false
		}
		augmented[col], augmented[pivot] = augmented[pivot], augmented[col]
		divisor := augmented[col][col]
		for c := col; c < 7; c++ {
			augmented[col][c] /= divisor
		}
		for row := 0; row < 6; row++ {
			if row == col {
				continue
			}
			factor := augmented[row][col]
			for c := col; c < 7; c++ {
				augmented[row][c] -= factor * augmented[col][c]
			}
		}
	}
	var solution [6]float64
	for i := range solution {
		solution[i] = augmented[i][6]
	}
	return solution, true
}

// buildDiagnosticLatticeResidualWarp derives sub-block phase corrections from
// the key-independent local lattice estimator. Each local lattice point is
// projected back into canonical coordinates and reduced modulo the 8-pixel v3
// block grid. A correct global geometry should leave only a smooth, bounded
// residual field; random texture produces incoherent modulo-block offsets.
func buildDiagnosticLatticeResidualWarp(candidate DiagnosticScaleCandidate, h homography, regions []LocalLatticeEstimate) (*diagnosticResidualWarp, bool) {
	width := candidate.CanonicalWidthPixels
	height := candidate.CanonicalHeightPixels
	if width <= 1 || height <= 1 || len(regions) < diagnosticResidualMinControls {
		return nil, false
	}
	inverse, ok := invertDiagnosticHomography(h)
	if !ok {
		return nil, false
	}
	controls := make([]diagnosticResidualControl, 0, len(regions))
	for _, region := range regions {
		if region.Confidence < 0.12 {
			continue
		}
		px, py, ok := diagnosticNearestLocalLatticePoint(region)
		if !ok {
			continue
		}
		cx, cy, ok := inverse.mapPoint(px, py)
		if !ok {
			continue
		}
		marginX, marginY := 0.05*width, 0.05*height
		if cx < -marginX || cy < -marginY || cx > width+marginX || cy > height+marginY {
			continue
		}
		gx := math.Round(cx/float64(blockSize)) * float64(blockSize)
		gy := math.Round(cy/float64(blockSize)) * float64(blockSize)
		dx, dy := cx-gx, cy-gy
		if math.Abs(dx) > float64(blockSize)/2+1e-6 || math.Abs(dy) > float64(blockSize)/2+1e-6 {
			continue
		}
		controls = append(controls, diagnosticResidualControl{
			x: gx, y: gy, dx: dx, dy: dy,
			weight: math.Max(region.Confidence, 0.12),
		})
	}
	if len(controls) < diagnosticResidualMinControls {
		return nil, false
	}
	warp, ok := fitDiagnosticResidualWarp(width, height, controls)
	if !ok {
		return nil, false
	}
	warp.maxPixels = float64(blockSize) / 2
	return warp, true
}

func diagnosticNearestLocalLatticePoint(region LocalLatticeEstimate) (float64, float64, bool) {
	determinant := region.U.X*region.V.Y - region.U.Y*region.V.X
	if math.Abs(determinant) < 1e-9 {
		return 0, 0, false
	}
	anchorX := region.PhaseU*region.U.X + region.PhaseV*region.V.X
	anchorY := region.PhaseU*region.U.Y + region.PhaseV*region.V.Y
	centerX := float64(region.NativeX) + float64(region.NativeWidth)/2
	centerY := float64(region.NativeY) + float64(region.NativeHeight)/2
	dx, dy := centerX-anchorX, centerY-anchorY
	i := math.Round((dx*region.V.Y - dy*region.V.X) / determinant)
	j := math.Round((region.U.X*dy - region.U.Y*dx) / determinant)
	return anchorX + i*region.U.X + j*region.V.X, anchorY + i*region.U.Y + j*region.V.Y, true
}

func invertDiagnosticHomography(input homography) (homography, bool) {
	a := input.h
	det := a[0]*(a[4]*a[8]-a[5]*a[7]) - a[1]*(a[3]*a[8]-a[5]*a[6]) + a[2]*(a[3]*a[7]-a[4]*a[6])
	if math.Abs(det) < 1e-12 {
		return homography{}, false
	}
	inv := 1 / det
	return homography{h: [9]float64{
		(a[4]*a[8] - a[5]*a[7]) * inv,
		(a[2]*a[7] - a[1]*a[8]) * inv,
		(a[1]*a[5] - a[2]*a[4]) * inv,
		(a[5]*a[6] - a[3]*a[8]) * inv,
		(a[0]*a[8] - a[2]*a[6]) * inv,
		(a[2]*a[3] - a[0]*a[5]) * inv,
		(a[3]*a[7] - a[4]*a[6]) * inv,
		(a[1]*a[6] - a[0]*a[7]) * inv,
		(a[0]*a[4] - a[1]*a[3]) * inv,
	}}, true
}

const (
	diagnosticSubpixelProbeRepeats         = 2
	diagnosticSubpixelProbeBudgetPerRefine = 35
	diagnosticMaxSubpixelRefinements       = 3
	diagnosticFundamentalScaleProbeBudget  = 16
)

var diagnosticSubpixelCoarseOffsets = [...]float64{-4, -2, 0, 2, 4}
var diagnosticSubpixelFineOffsets = [...]float64{-0.5, 0, 0.5}

// refineDiagnosticSubpixelOffset searches only one modulo-8 block cell. Any
// larger integer-block displacement belongs to the existing tile-phase logic,
// so this cannot grow into an unbounded translation bank.
func refineDiagnosticSubpixelOffset(src image.Image, candidate DiagnosticScaleCandidate, base diagnosticProjectiveMapper, decoder *decoder) (diagnosticProjectiveMapper, diagnosticProjectiveProbe, int) {
	base.canonicalOffsetX = wrapDiagnosticBlockOffset(base.canonicalOffsetX)
	base.canonicalOffsetY = wrapDiagnosticBlockOffset(base.canonicalOffsetY)
	bestMapper := base
	bestProbe := diagnosticProbeProjectiveSyncWithMapperRepeats(src, candidate, base, decoder, 1)
	attempts := 1
	baseX, baseY := base.canonicalOffsetX, base.canonicalOffsetY
	for _, dy := range diagnosticSubpixelCoarseOffsets {
		for _, dx := range diagnosticSubpixelCoarseOffsets {
			if dx == 0 && dy == 0 {
				continue
			}
			mapper := base
			mapper.canonicalOffsetX = wrapDiagnosticBlockOffset(baseX + dx)
			mapper.canonicalOffsetY = wrapDiagnosticBlockOffset(baseY + dy)
			probe := diagnosticProbeProjectiveSyncWithMapperRepeats(src, candidate, mapper, decoder, 1)
			attempts++
			if diagnosticProbeBetter(probe, mapper, bestProbe, bestMapper) {
				bestMapper, bestProbe = mapper, probe
			}
		}
	}

	coarseX, coarseY := bestMapper.canonicalOffsetX, bestMapper.canonicalOffsetY
	for _, dy := range diagnosticSubpixelFineOffsets {
		for _, dx := range diagnosticSubpixelFineOffsets {
			if dx == 0 && dy == 0 {
				continue
			}
			mapper := base
			mapper.canonicalOffsetX = coarseX + dx
			mapper.canonicalOffsetY = coarseY + dy
			// Keep the final offset in one periodic 8-pixel cell.
			mapper.canonicalOffsetX = wrapDiagnosticBlockOffset(mapper.canonicalOffsetX)
			mapper.canonicalOffsetY = wrapDiagnosticBlockOffset(mapper.canonicalOffsetY)
			probe := diagnosticProbeProjectiveSyncWithMapperRepeats(src, candidate, mapper, decoder, diagnosticSubpixelProbeRepeats)
			attempts++
			if diagnosticProbeBetter(probe, mapper, bestProbe, bestMapper) {
				bestMapper, bestProbe = mapper, probe
			}
		}
	}
	// Re-score both the sparse-search winner and the unshifted base with the
	// ordinary probe. Sparse ranking is allowed to propose an offset but never to
	// degrade the candidate that reaches a full HMAC decode slot.
	bestProbe = diagnosticProbeProjectiveSyncWithMapper(src, candidate, bestMapper, decoder)
	baseProbe := diagnosticProbeProjectiveSyncWithMapper(src, candidate, base, decoder)
	attempts += 2
	if diagnosticProbeBetter(baseProbe, base, bestProbe, bestMapper) {
		return base, baseProbe, attempts
	}
	return bestMapper, bestProbe, attempts
}

func diagnosticProbeBetter(a diagnosticProjectiveProbe, am diagnosticProjectiveMapper, b diagnosticProjectiveProbe, bm diagnosticProjectiveMapper) bool {
	if a.z != b.z {
		return a.z > b.z
	}
	if a.fraction != b.fraction {
		return a.fraction > b.fraction
	}
	ad := math.Hypot(am.canonicalOffsetX, am.canonicalOffsetY)
	bd := math.Hypot(bm.canonicalOffsetX, bm.canonicalOffsetY)
	return ad < bd
}

func wrapDiagnosticBlockOffset(value float64) float64 {
	period := float64(blockSize)
	for value >= period/2 {
		value -= period
	}
	for value < -period/2 {
		value += period
	}
	return value
}

var diagnosticFundamentalScaleDeltas = [...]float64{-0.02, -0.01, -0.005, 0, 0.005, 0.01, 0.02}

// refineDiagnosticFundamentalScaleBySync performs a small, deterministic search
// around the key-independent fundamental scale. It is key-assisted and therefore
// diagnostic only; the search radius is fixed and cannot jump to another scale
// family or grow into a global brute-force bank.
func refineDiagnosticFundamentalScaleBySync(src image.Image, boundary PrintBoundaryEstimate, seed DiagnosticScaleCandidate, decoder *decoder) (DiagnosticScaleCandidate, diagnosticProjectiveMapper, diagnosticProjectiveProbe, int) {
	bestCandidate := seed
	bestMapper := diagnosticProjectiveMapper{}
	bestProbe := diagnosticProjectiveProbe{candidate: seed, z: math.Inf(-1)}
	attempts := 0
	evaluate := func(candidate DiagnosticScaleCandidate) {
		h, ok := homographyForPrintBoundary(candidate.CanonicalWidthPixels, candidate.CanonicalHeightPixels, boundary)
		if !ok {
			return
		}
		mapper := diagnosticProjectiveMapper{h: h}
		probe := diagnosticProbeProjectiveSyncWithMapperRepeats(src, candidate, mapper, decoder, diagnosticSubpixelProbeRepeats)
		attempts++
		if bestProbe.profile == "" || probe.z > bestProbe.z || (probe.z == bestProbe.z && diagnosticScaleDistance(candidate, seed) < diagnosticScaleDistance(bestCandidate, seed)) {
			bestCandidate, bestMapper, bestProbe = candidate, mapper, probe
		}
	}

	baseWidth := seed.CanonicalWidthPixels
	for _, delta := range diagnosticFundamentalScaleDeltas {
		candidate := seed
		candidate.CanonicalWidthPixels = baseWidth * (1 + delta)
		evaluate(candidate)
	}
	if bestProbe.profile == "" {
		return seed, diagnosticProjectiveMapper{}, diagnosticProjectiveProbe{candidate: seed}, attempts
	}
	baseHeight := seed.CanonicalHeightPixels
	width := bestCandidate.CanonicalWidthPixels
	for _, delta := range diagnosticFundamentalScaleDeltas {
		candidate := seed
		candidate.CanonicalWidthPixels = width
		candidate.CanonicalHeightPixels = baseHeight * (1 + delta)
		evaluate(candidate)
	}
	bestProbe = diagnosticProbeProjectiveSyncWithMapper(src, bestCandidate, bestMapper, decoder)
	attempts++
	seedH, ok := homographyForPrintBoundary(seed.CanonicalWidthPixels, seed.CanonicalHeightPixels, boundary)
	if ok {
		seedMapper := diagnosticProjectiveMapper{h: seedH}
		seedProbe := diagnosticProbeProjectiveSyncWithMapper(src, seed, seedMapper, decoder)
		attempts++
		if seedProbe.z > bestProbe.z || (seedProbe.z == bestProbe.z && seedProbe.fraction > bestProbe.fraction) {
			return seed, seedMapper, seedProbe, attempts
		}
	}
	return bestCandidate, bestMapper, bestProbe, attempts
}

func diagnosticScaleDistance(a, b DiagnosticScaleCandidate) float64 {
	if b.CanonicalWidthPixels <= 0 || b.CanonicalHeightPixels <= 0 {
		return math.Inf(1)
	}
	dx := (a.CanonicalWidthPixels - b.CanonicalWidthPixels) / b.CanonicalWidthPixels
	dy := (a.CanonicalHeightPixels - b.CanonicalHeightPixels) / b.CanonicalHeightPixels
	return math.Hypot(dx, dy)
}
