package watermark

import (
	"image"
	"math"
	"sort"
	"strconv"
)

const (
	diagnosticPhotometricRaw diagnosticPhotometricMode = iota
	diagnosticPhotometricLocalNormalize
	diagnosticPhotometricMildHighPass

	diagnosticPhotometricModeCount        = 3
	diagnosticMaxPhotometricGeometries    = 3
	diagnosticMaxPhotometricProbeAttempts = diagnosticPhotometricModeCount * diagnosticMaxPhotometricGeometries
	diagnosticPhotometricLocalRadius      = 2
	diagnosticPhotometricStdFloor         = 6.0
	diagnosticPhotometricHighPassMix      = 0.55
)

type diagnosticPhotometricMode int

func (mode diagnosticPhotometricMode) String() string {
	switch mode {
	case diagnosticPhotometricLocalNormalize:
		return "local-normalize"
	case diagnosticPhotometricMildHighPass:
		return "mild-highpass"
	default:
		return "raw"
	}
}

func diagnosticPhotometricModes() []diagnosticPhotometricMode {
	return []diagnosticPhotometricMode{
		diagnosticPhotometricRaw,
		diagnosticPhotometricLocalNormalize,
		diagnosticPhotometricMildHighPass,
	}
}

// DiagnosticPhotometricEvidence reports how one bounded photometric view scores
// on the known authenticated-v3 header. It is diagnostic evidence only. A mode
// can be selected for a full decode, but only the ordinary Format v3 HMAC can
// authenticate a payload.
type DiagnosticPhotometricEvidence struct {
	Mode                 string  `json:"mode"`
	GeometrySource       string  `json:"geometry_source"`
	CanonicalWidthPx     float64 `json:"canonical_width_px"`
	CanonicalHeightPx    float64 `json:"canonical_height_px"`
	SyncProfile          Profile `json:"sync_profile,omitempty"`
	SyncFraction         float64 `json:"sync_fraction"`
	SyncZScore           float64 `json:"sync_z_score"`
	MeanAbsoluteMargin   float64 `json:"mean_absolute_margin"`
	NormalizedSyncMargin float64 `json:"normalized_sync_margin"`
}

type diagnosticPhotometricProbe struct {
	probe                diagnosticProjectiveProbe
	meanAbsoluteMargin   float64
	normalizedSyncMargin float64
}

// diagnosticReadProjectiveBlockWithMapperPhotometric applies the small build5
// photometric bank after geometric mapping. The bank is intentionally fixed and
// deterministic. It never changes Format v3, its coefficient pair, or HMAC.
func diagnosticReadProjectiveBlockWithMapperPhotometric(src image.Image, mapper diagnosticProjectiveMapper, originX, originY float64, mode diagnosticPhotometricMode) (float64, bool) {
	if mode == diagnosticPhotometricRaw {
		return diagnosticReadProjectiveBlockWithMapper(src, mapper, originX, originY)
	}

	// Both non-raw modes use the same 12x12 mapped patch. Sharing the support
	// keeps their cost bounded while giving the 8x8 DCT block a two-pixel local
	// neighbourhood on every side.
	const patchSize = blockSize + 2*diagnosticPhotometricLocalRadius
	var patch [patchSize][patchSize]float64
	for py := 0; py < patchSize; py++ {
		for px := 0; px < patchSize; px++ {
			cx := originX + float64(px-diagnosticPhotometricLocalRadius)
			cy := originY + float64(py-diagnosticPhotometricLocalRadius)
			sx, sy, ok := mapper.mapPoint(cx, cy)
			if !ok {
				return 0, false
			}
			value, ok := diagnosticSampleImageLuminance(src, sx, sy)
			if !ok {
				return 0, false
			}
			patch[py][px] = value
		}
	}

	coefficient23, coefficient32 := 0.0, 0.0
	for y := 0; y < blockSize; y++ {
		for x := 0; x < blockSize; x++ {
			cx := x + diagnosticPhotometricLocalRadius
			cy := y + diagnosticPhotometricLocalRadius
			center := patch[cy][cx]
			value := center

			switch mode {
			case diagnosticPhotometricLocalNormalize:
				mean, variance := diagnosticPatchMeanVariance(&patch, cx, cy, diagnosticPhotometricLocalRadius)
				stddev := math.Sqrt(math.Max(variance, 0))
				if stddev < diagnosticPhotometricStdFloor {
					stddev = diagnosticPhotometricStdFloor
				}
				value = (center - mean) / stddev
			case diagnosticPhotometricMildHighPass:
				mean, _ := diagnosticPatchMeanVariance(&patch, cx, cy, 1)
				// Preserve part of the original signal rather than using a pure
				// high-pass. This is intentionally mild so printer/camera texture is
				// not amplified without bound.
				value = center - diagnosticPhotometricHighPassMix*mean
			}

			coefficient23 += value * cosTable[3][x] * cosTable[2][y]
			coefficient32 += value * cosTable[2][x] * cosTable[3][y]
		}
	}
	return math.Abs(coefficient23) - math.Abs(coefficient32), true
}

func diagnosticPatchMeanVariance(patch *[blockSize + 2*diagnosticPhotometricLocalRadius][blockSize + 2*diagnosticPhotometricLocalRadius]float64, cx, cy, radius int) (float64, float64) {
	if radius < 0 {
		radius = 0
	}
	sum, sumSquares := 0.0, 0.0
	count := 0.0
	for y := cy - radius; y <= cy+radius; y++ {
		for x := cx - radius; x <= cx+radius; x++ {
			value := patch[y][x]
			sum += value
			sumSquares += value * value
			count++
		}
	}
	if count == 0 {
		return 0, 0
	}
	mean := sum / count
	variance := sumSquares/count - mean*mean
	if variance < 0 && variance > -1e-9 {
		variance = 0
	}
	return mean, variance
}

func diagnosticProbeProjectivePhotometric(src image.Image, candidate DiagnosticScaleCandidate, mapper diagnosticProjectiveMapper, decoder *decoder, mode diagnosticPhotometricMode) diagnosticPhotometricProbe {
	width := int(math.Round(candidate.CanonicalWidthPixels))
	height := int(math.Round(candidate.CanonicalHeightPixels))
	result := diagnosticPhotometricProbe{probe: diagnosticProjectiveProbe{candidate: candidate, z: math.Inf(-1)}}
	if width < tileWidth*blockSize || height < tileHeight*blockSize {
		result.probe.z = 0
		return result
	}

	bw, bh := width/blockSize, height/blockSize
	tileXs := diagnosticSpacedIndices((bw+tileWidth-1)/tileWidth, diagnosticSyncRepeatsPerAxis)
	tileYs := diagnosticSpacedIndices((bh+tileHeight-1)/tileHeight, diagnosticSyncRepeatsPerAxis)
	grid := make([]float64, eccBits)
	counts := make([]int, eccBits)
	absSum, sampleCount := 0.0, 0
	for logicalY := 0; logicalY < tileHeight; logicalY++ {
		for logicalX := 0; logicalX < tileWidth; logicalX++ {
			position := logicalY*tileWidth + logicalX
			for _, tileY := range tileYs {
				by := logicalY + tileY*tileHeight
				if by >= bh {
					continue
				}
				for _, tileX := range tileXs {
					bx := logicalX + tileX*tileWidth
					if bx >= bw {
						continue
					}
					margin, ok := diagnosticReadProjectiveBlockWithMapperPhotometric(src, mapper, float64(bx*blockSize), float64(by*blockSize), mode)
					if !ok {
						continue
					}
					grid[position] += margin
					counts[position]++
					absSum += math.Abs(margin)
					sampleCount++
				}
			}
		}
	}
	if sampleCount > 0 {
		result.meanAbsoluteMargin = absSum / float64(sampleCount)
	}

	best := diagnosticProjectiveProbe{candidate: candidate, z: math.Inf(-1)}
	bestNormalizedMargin := 0.0
	for _, pattern := range decoder.v3Patterns {
		phases := strongestV3Phases(grid, pattern, 1)
		if len(phases) == 0 || phases[0].total == 0 {
			continue
		}
		phase := phases[0]
		fraction := float64(phase.score) / float64(phase.total)
		z := (float64(phase.score) - float64(phase.total)/2) / math.Sqrt(float64(phase.total)/4)
		normalizedMargin := diagnosticKnownSyncMargin(grid, pattern, phase.x, phase.y)
		if z > best.z {
			best.profile, best.fraction, best.z = pattern.spec.profile, fraction, z
			bestNormalizedMargin = normalizedMargin
		}
	}
	if math.IsInf(best.z, -1) {
		best.z = 0
	}
	result.probe = best
	result.normalizedSyncMargin = bestNormalizedMargin
	return result
}

func diagnosticKnownSyncMargin(grid []float64, pattern v3SyncPattern, phaseX, phaseY int) float64 {
	signed, magnitude := 0.0, 0.0
	for _, syncPoint := range pattern.points {
		logicalX := syncPoint.tilePosition % tileWidth
		logicalY := syncPoint.tilePosition / tileWidth
		observedX := positiveMod(logicalX-phaseX, tileWidth)
		observedY := positiveMod(logicalY-phaseY, tileHeight)
		value := grid[observedY*tileWidth+observedX]
		expectedSign := -1.0
		if syncPoint.expected != 0 {
			expectedSign = 1
		}
		signed += expectedSign * value
		magnitude += math.Abs(value)
	}
	if magnitude == 0 {
		return 0
	}
	return signed / magnitude
}

func diagnosticProjectiveGridWithMapperPhotometric(src image.Image, width, height int, mapper diagnosticProjectiveMapper, mode diagnosticPhotometricMode) ([]float64, bool) {
	grid, _, ok := diagnosticProjectiveGridWithMapperPhotometricStats(src, width, height, mapper, mode)
	return grid, ok
}

func diagnosticProjectiveGridWithMapperPhotometricStats(src image.Image, width, height int, mapper diagnosticProjectiveMapper, mode diagnosticPhotometricMode) ([]float64, diagnosticGridSamplingStats, bool) {
	return diagnosticProjectiveGridSampled(width, height, func(originX, originY float64) (float64, bool) {
		if mode == diagnosticPhotometricRaw {
			return diagnosticReadProjectiveBlockWithMapper(src, mapper, originX, originY)
		}
		return diagnosticReadProjectiveBlockWithMapperPhotometric(src, mapper, originX, originY, mode)
	})
}

type diagnosticPhotometricAuthCandidate struct {
	geometry diagnosticAuthCandidate
	mode     diagnosticPhotometricMode
	probe    diagnosticPhotometricProbe
}

func buildDiagnosticPhotometricCandidates(src image.Image, geometry []diagnosticAuthCandidate, decoder *decoder) []diagnosticPhotometricAuthCandidate {
	result := make([]diagnosticPhotometricAuthCandidate, 0, len(geometry)*diagnosticPhotometricModeCount)
	for _, candidate := range geometry {
		for _, mode := range diagnosticPhotometricModes() {
			probe := diagnosticProbeProjectivePhotometric(src, candidate.candidate, candidate.mapper, decoder, mode)
			result = append(result, diagnosticPhotometricAuthCandidate{geometry: candidate, mode: mode, probe: probe})
		}
	}
	return result
}

func selectDiagnosticPhotometricDecodeCandidates(candidates []diagnosticPhotometricAuthCandidate, limit int) []diagnosticPhotometricAuthCandidate {
	if limit <= 0 || len(candidates) == 0 {
		return nil
	}
	result := make([]diagnosticPhotometricAuthCandidate, 0, limit)
	used := make(map[string]struct{})
	keyFor := func(candidate diagnosticPhotometricAuthCandidate) string {
		return candidate.geometry.source + ":" + candidate.mode.String() + ":" +
			formatDiagnosticScaleKey(candidate.geometry.candidate.CanonicalWidthPixels, candidate.geometry.candidate.CanonicalHeightPixels)
	}
	appendDistinct := func(candidate diagnosticPhotometricAuthCandidate) {
		if len(result) >= limit {
			return
		}
		key := keyFor(candidate)
		if _, exists := used[key]; exists {
			return
		}
		result = append(result, candidate)
		used[key] = struct{}{}
	}

	// Preserve build4 behavior first: the first geometry chosen by the bounded
	// geometry selector is decoded in raw luminance before any photometric mode.
	for _, candidate := range candidates {
		if candidate.mode == diagnosticPhotometricRaw {
			appendDistinct(candidate)
			break
		}
	}

	// Give the same first geometry one non-raw opportunity, selecting only by
	// known-header z score. This tests the physical channel without consuming the
	// full budget on alternate geometries.
	if len(candidates) > 0 {
		firstSource := candidates[0].geometry.source
		firstWidth := candidates[0].geometry.candidate.CanonicalWidthPixels
		firstHeight := candidates[0].geometry.candidate.CanonicalHeightPixels
		best := -1
		for i, candidate := range candidates {
			if candidate.mode == diagnosticPhotometricRaw || candidate.geometry.source != firstSource {
				continue
			}
			if math.Abs(candidate.geometry.candidate.CanonicalWidthPixels-firstWidth) > 1e-6 || math.Abs(candidate.geometry.candidate.CanonicalHeightPixels-firstHeight) > 1e-6 {
				continue
			}
			if best < 0 || diagnosticPhotometricCandidateBetter(candidate, candidates[best]) {
				best = i
			}
		}
		if best >= 0 {
			appendDistinct(candidates[best])
		}
	}

	ranked := append([]diagnosticPhotometricAuthCandidate(nil), candidates...)
	sort.Slice(ranked, func(i, j int) bool { return diagnosticPhotometricCandidateBetter(ranked[i], ranked[j]) })
	for _, candidate := range ranked {
		appendDistinct(candidate)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func diagnosticPhotometricCandidateBetter(a, b diagnosticPhotometricAuthCandidate) bool {
	if a.probe.probe.z != b.probe.probe.z {
		return a.probe.probe.z > b.probe.probe.z
	}
	if a.probe.normalizedSyncMargin != b.probe.normalizedSyncMargin {
		return a.probe.normalizedSyncMargin > b.probe.normalizedSyncMargin
	}
	if a.geometry.candidate.Fundamental != b.geometry.candidate.Fundamental {
		return a.geometry.candidate.Fundamental
	}
	if a.geometry.phase.score != b.geometry.phase.score {
		return a.geometry.phase.score > b.geometry.phase.score
	}
	return a.mode < b.mode
}

func formatDiagnosticScaleKey(width, height float64) string {
	// Integer centipixels are sufficient to distinguish the bounded candidate set.
	return strconv.FormatInt(int64(math.Round(width*100)), 10) + "/" + strconv.FormatInt(int64(math.Round(height*100)), 10)
}

func selectDiagnosticPhotometricGeometries(candidates []diagnosticAuthCandidate, limit int) []diagnosticAuthCandidate {
	if limit <= 0 || len(candidates) == 0 {
		return nil
	}
	result := make([]diagnosticAuthCandidate, 0, limit)
	appendDistinct := func(candidate diagnosticAuthCandidate) {
		if len(result) >= limit {
			return
		}
		for _, existing := range result {
			wr := math.Max(existing.candidate.CanonicalWidthPixels, candidate.candidate.CanonicalWidthPixels)
			hr := math.Max(existing.candidate.CanonicalHeightPixels, candidate.candidate.CanonicalHeightPixels)
			if existing.source == candidate.source && wr > 0 && hr > 0 &&
				math.Abs(existing.candidate.CanonicalWidthPixels-candidate.candidate.CanonicalWidthPixels)/wr < 0.005 &&
				math.Abs(existing.candidate.CanonicalHeightPixels-candidate.candidate.CanonicalHeightPixels)/hr < 0.005 {
				return
			}
		}
		result = append(result, candidate)
	}

	// First keep the best view of the key-independent fundamental family.
	bestFundamental := -1
	for i, candidate := range candidates {
		if !candidate.candidate.Fundamental {
			continue
		}
		if bestFundamental < 0 || candidate.probe.z > candidates[bestFundamental].probe.z {
			bestFundamental = i
		}
	}
	if bestFundamental >= 0 {
		appendDistinct(candidates[bestFundamental])
	}

	// Then keep the strongest raw sync geometry, even when it is an alias. The
	// photometric experiment must measure the best build4 geometry rather than
	// accidentally excluding it through the decode-slot diversity policy.
	bestSync := 0
	for i := 1; i < len(candidates); i++ {
		if candidates[i].probe.z > candidates[bestSync].probe.z {
			bestSync = i
		}
	}
	appendDistinct(candidates[bestSync])

	// Finally preserve the strongest phase-consistent geometry that is still
	// distinct. This gives the bank one alternative physical interpretation
	// without expanding the geometry search.
	ranked := append([]diagnosticAuthCandidate(nil), candidates...)
	sortDiagnosticAuthCandidates(ranked)
	for _, candidate := range ranked {
		appendDistinct(candidate)
		if len(result) >= limit {
			break
		}
	}
	return result
}
