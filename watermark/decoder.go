package watermark

import (
	"errors"
	"image"
	"math"
	"sort"
)

// ExtractInfo identifies the authenticated v3 profile recovered by the decoder.
type ExtractInfo struct {
	Version                   int
	Profile                   Profile
	Confidence                float64
	RotationCorrectionDegrees float64
	ScaleXCorrection          float64
	ScaleYCorrection          float64
	ShearXCorrection          float64
	ShearYCorrection          float64
}

type decoder struct {
	key        []byte
	v3Patterns []v3SyncPattern
}

func newDecoder(key []byte) *decoder {
	result := &decoder{
		key:        key,
		v3Patterns: make([]v3SyncPattern, 0, len(v3Profiles)),
	}
	for _, spec := range v3Profiles {
		result.v3Patterns = append(result.v3Patterns, newV3SyncPattern(key, spec))
	}
	return result
}

// ExtractWithInfo searches the fixed, bounded v3 geometric candidate space and
// automatically identifies the adaptive profile from the authenticated header.
func ExtractWithInfo(src image.Image, key []byte) ([]byte, ExtractInfo, error) {
	if len(key) < 8 {
		return nil, ExtractInfo{}, errors.New("key must contain at least 8 bytes")
	}
	return extractV3(src, key)
}

func extractV3(src image.Image, key []byte) ([]byte, ExtractInfo, error) {
	decoder := newDecoder(key)
	sourcePlane := newPixelPlane(src)
	payload, info, directScore, ok := searchV3(sourcePlane, decoder, candidateBlockSizes[:])
	if ok {
		return payload, info, nil
	}
	nativeCoherence := nativeRepetitionCoherence(sourcePlane)

	// Exact 90/180/270-degree rotations preserve the 8x8 lattice and can be
	// corrected losslessly. Build 9 adds a key-independent repetition gate: 90/270
	// swap the 35x32 tile periods to 32x35, while 180 keeps the native period. This
	// prevents a high content-derived sync score from launching three expensive
	// full quarter-turn probes on unrelated composed geometry.
	if directScore >= 760 && !exceedsPixelLimit(sourcePlane.bounds.Dx(), sourcePlane.bounds.Dy(), maxSearchPixels) {
		quarterTurnsToTry := make([]int, 0, 3)
		// Repetition coherence is a fast gate only when the carrier is large
		// enough to contain two complete logical periods. A small carrier can
		// still contain one fully decodable tile, so lack of measurable
		// repetition must not suppress exact quarter-turn recovery.
		if !canMeasureAlignedRepetition(sourcePlane, tileHeight, tileWidth) || quarterTurnRepetitionCoherence(sourcePlane) >= 0.82 {
			quarterTurnsToTry = append(quarterTurnsToTry, 1, 3)
		}
		if !canMeasureAlignedRepetition(sourcePlane, tileWidth, tileHeight) || nativeCoherence >= 0.82 {
			quarterTurnsToTry = append(quarterTurnsToTry, 2)
		}
		for _, quarterTurns := range quarterTurnsToTry {
			corrected := rotatePixelPlaneQuarter(sourcePlane, quarterTurns)
			if payload, info, _, ok := searchV3(corrected, decoder, candidateBlockSizes[:]); ok {
				info.RotationCorrectionDegrees = normalizeDegrees(float64(quarterTurns * 90))
				return payload, info, nil
			}
		}
	}

	// Build 8 introduced a small real-corpus basis bank. Build 9 expands that bank
	// to symmetric +/-5% and +/-10% anisotropies while keeping the same direct
	// lattice search model. The bank scores the repeated v3 DCT lattice directly
	// instead of trusting a standalone rotation estimate that anisotropic scaling
	// may distort. Native coherent carriers skip it, preserving the cheap
	// wrong-key path.
	directLatticeEvidence := latticeCandidate{}
	if nativeCoherence < 0.82 {
		payload, info, candidate, ok := searchV3DirectLatticeBasis(sourcePlane, decoder)
		directLatticeEvidence = candidate
		if ok {
			info.RotationCorrectionDegrees = normalizeDegrees(-candidate.angle)
			info.ScaleXCorrection = 1 / candidate.scaleX
			info.ScaleYCorrection = 1 / candidate.scaleY
			return payload, info, nil
		}
	}

	// Preserve the build-5 axis-aligned affine fast path before arbitrary-angle
	// probing when the DCT lattice still has a strong zero-degree signature. This
	// distinguishes pure anisotropic scale/shear from composed geometry and avoids
	// letting an affine-distorted carrier masquerade as a rotation candidate.
	if hasStrongZeroDegreeLattice(sourcePlane) && nativeCoherence < 0.82 {
		if payload, info, candidate, ok := searchV3AxisAlignedAffine(sourcePlane, decoder); ok {
			applyAffineCorrectionInfo(&info, candidate)
			return payload, info, nil
		}
	}

	// Arbitrary-angle recovery is a bounded two-stage search. First estimate
	// lattice orientation with sparse DCT probes. Only high-contrast candidates
	// are rectified and passed to the normal authenticated decoder. The angle
	// probe is modulo 90 degrees; quarter-turn decoding resolves the quadrant.
	rotationCandidates := detectRotationCandidates(sourcePlane)
	for _, candidate := range rotationCandidates {
		rotatedWidth, rotatedHeight := rotatedPixelDimensions(sourcePlane.bounds.Dx(), sourcePlane.bounds.Dy(), -candidate.angle)
		if exceedsPixelLimit(rotatedWidth, rotatedHeight, maxSearchPixels) {
			continue
		}
		rectified := rotatePixelPlane(sourcePlane, -candidate.angle)
		for quarterTurns := 0; quarterTurns <= 3; quarterTurns++ {
			corrected := rectified
			if quarterTurns != 0 {
				corrected = rotatePixelPlaneQuarter(rectified, quarterTurns)
			}
			if payload, info, _, ok := searchV3(corrected, decoder, []int{candidate.blockSize}); ok {
				info.RotationCorrectionDegrees = normalizeDegrees(-candidate.angle + float64(quarterTurns*90))
				return payload, info, nil
			}
		}
	}

	// Axis-aligned affine recovery remains available independently when no strong
	// rotation is present.
	if len(rotationCandidates) == 0 || rotationCandidateQuality(rotationCandidates[0]) < 25 {
		if payload, info, candidate, ok := searchV3AxisAlignedAffine(sourcePlane, decoder); ok {
			applyAffineCorrectionInfo(&info, candidate)
			return payload, info, nil
		}
	}

	// If direct lattice estimation found a decisive composed-geometry peak but
	// authentication failed, we have now also given the established axis-aligned
	// affine and rotation paths a chance to recover legitimate alternate geometry.
	// Do not continue into the expensive pure-resize normalization cascade: this
	// is the bounded wrong-key/damaged-payload exit for the build-9 lattice-basis
	// bank.
	if directLatticeEvidence.decisive {
		return nil, ExtractInfo{}, errors.New("v3 hidden payload not found or key is incorrect")
	}

	// A very strong non-zero lattice peak means the geometry was identified but
	// authenticated decoding failed. Continuing into pure-resize normalization
	// cannot repair a rotated carrier and makes wrong-key failures needlessly
	// expensive. Keep weaker/ambiguous peaks eligible for the historical resize
	// path so ordinary fractional-resize recovery is not cut off by image texture.
	if len(rotationCandidates) > 0 && rotationCandidateQuality(rotationCandidates[0]) >= 25 {
		return nil, ExtractInfo{}, errors.New("v3 hidden payload not found or key is incorrect")
	}

	// Pure resize fast path: inverse-normalize candidate dimensions and inspect
	// the aligned 8x8 grid. This keeps resize recovery independent of profile.
	bounds := src.Bounds()
	neighborDeltas := [...]point{
		{-1, -1}, {0, -1}, {1, -1},
		{-1, 0}, {1, 0},
		{-1, 1}, {0, 1}, {1, 1},
	}
	scales := make([]scaleCandidate, 0, len(normalizedScales))
	for _, percent := range normalizedScales {
		width := int(math.Round(float64(bounds.Dx()) * 100 / float64(percent)))
		height := int(math.Round(float64(bounds.Dy()) * 100 / float64(percent)))
		if width < tileWidth*blockSize || height < tileHeight*blockSize || exceedsPixelLimit(width, height, maxSearchPixels) {
			continue
		}
		normalized := resizePixelPlaneBicubic(sourcePlane, width, height)
		payload, info, score, ok := searchV3Aligned(normalized, decoder)
		if ok {
			return payload, info, nil
		}
		scales = append(scales, scaleCandidate{percent: percent, score: score})
	}

	// Limit +/-1 rounding recovery to the three strongest nominal scales. The
	// maximum search remains 116 direct grids + 13 aligned normalizations + 24
	// neighboring reconstructions = 153 geometric candidates before size skips.
	sort.SliceStable(scales, func(i, j int) bool { return scales[i].score > scales[j].score })
	if len(scales) > 3 {
		scales = scales[:3]
	}
	for _, candidate := range scales {
		baseWidth := int(math.Round(float64(bounds.Dx()) * 100 / float64(candidate.percent)))
		baseHeight := int(math.Round(float64(bounds.Dy()) * 100 / float64(candidate.percent)))
		for _, delta := range neighborDeltas {
			width := baseWidth + delta.x
			height := baseHeight + delta.y
			if width < tileWidth*blockSize || height < tileHeight*blockSize || exceedsPixelLimit(width, height, maxSearchPixels) {
				continue
			}
			normalized := resizePixelPlaneBicubic(sourcePlane, width, height)
			if payload, info, _, ok := searchV3Aligned(normalized, decoder); ok {
				return payload, info, nil
			}
		}
	}
	return nil, ExtractInfo{}, errors.New("v3 hidden payload not found or key is incorrect")
}

func applyAffineCorrectionInfo(info *ExtractInfo, candidate affineCandidate) {
	switch candidate.kind {
	case affineScaleXY:
		info.ScaleXCorrection = 1 / candidate.parameter
		info.ScaleYCorrection = 1 / candidate.parameter2
	case affineShearX:
		info.ShearXCorrection = -candidate.parameter
	case affineShearY:
		info.ShearYCorrection = -candidate.parameter
	}
}

func searchV3(src *pixelPlane, decoder *decoder, sizes []int) ([]byte, ExtractInfo, int, bool) {
	bestScore := 0
	for _, size := range sizes {
		bounds := src.bounds
		if bounds.Dx() < tileWidth*size || bounds.Dy() < tileHeight*size {
			continue
		}
		for offsetY := 0; offsetY < size; offsetY++ {
			for offsetX := 0; offsetX < size; offsetX++ {
				grid, ok := aggregateGrid(src, size, offsetX, offsetY)
				if !ok {
					continue
				}
				payload, info, score, found := decoder.decodeGrid(grid)
				if score > bestScore {
					bestScore = score
				}
				if found {
					return payload, info, bestScore, true
				}
			}
		}
	}
	return nil, ExtractInfo{}, bestScore, false
}

func searchV3Aligned(src *pixelPlane, decoder *decoder) ([]byte, ExtractInfo, int, bool) {
	grid, ok := aggregateGrid(src, blockSize, 0, 0)
	if !ok {
		return nil, ExtractInfo{}, 0, false
	}
	return decoder.decodeGrid(grid)
}

func (decoder *decoder) decodeGrid(grid []float64) ([]byte, ExtractInfo, int, bool) {
	bestScore := 0

	// The three profile probes are fixed and bounded. The profile identifier is
	// authenticated in the v3 header and is never supplied by the extractor.
	for _, pattern := range decoder.v3Patterns {
		payload, confidence, score, ok := decodeV3WithPattern(grid, decoder.key, pattern)
		if score > bestScore {
			bestScore = score
		}
		if ok {
			return payload, ExtractInfo{
				Version:    v3Version,
				Profile:    pattern.spec.profile,
				Confidence: confidence,
			}, bestScore, true
		}
	}
	return nil, ExtractInfo{}, bestScore, false
}
