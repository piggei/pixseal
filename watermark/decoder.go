package watermark

import (
	"errors"
	"image"
	"math"
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
	PerspectiveCorrection     string
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
	strongZeroDegree := hasStrongZeroDegreeLattice(sourcePlane)

	// Exact 90/180/270-degree rotations preserve the integer lattice and are
	// corrected losslessly. Repetition gates keep unrelated images from paying for
	// three full quarter-turn decodes while small one-tile carriers remain eligible.
	if directScore >= 760 && !exceedsPixelLimit(sourcePlane.bounds.Dx(), sourcePlane.bounds.Dy(), maxSearchPixels) {
		quarterTurnsToTry := make([]int, 0, 3)
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

	// Build 10 restores the pure-resize baseline before speculative geometric
	// recovery. Instead of materializing up to 13 inverse-resized bitmaps, the
	// decoder samples each fixed isotropic scale directly through the affine view.
	// This prevents false rotation/lattice peaks from suppressing 95/85/65/55%
	// recovery while keeping the search bounded and memory-light.
	if nativeCoherence < 0.82 {
		payload, info, candidate, ok := searchV3IsotropicScale(sourcePlane, decoder)
		if ok {
			return payload, info, nil
		}
		if candidate.decisive && candidate.percent > 0 {
			if payload, info, ok := searchV3PureResizePercent(sourcePlane, decoder, candidate.percent); ok {
				return payload, info, nil
			}
		}
	}

	// Preserve the mature axis-aligned affine path before arbitrary-angle probing
	// when a zero-degree DCT signature remains visible.
	if strongZeroDegree && nativeCoherence < 0.82 {
		if payload, info, candidate, ok := searchV3AxisAlignedAffine(sourcePlane, decoder); ok {
			applyAffineCorrectionInfo(&info, candidate)
			return payload, info, nil
		}
	}

	// Build 11 experimental mild-perspective bank. Mature direct/resize and
	// axis-aligned affine paths keep priority; projective probing then gets a small
	// bounded opportunity before the more expensive lattice/rotation heuristics.
	if nativeCoherence < 0.82 {
		if payload, info, correction, ok := searchV3MildPerspective(sourcePlane, decoder); ok {
			info.PerspectiveCorrection = correction
			return payload, info, nil
		}
	}

	// Direct lattice-basis recovery handles the validated composed anisotropies.
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

	// Arbitrary-angle recovery remains a bounded two-stage orientation search.
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

	// Axis-aligned affine recovery is also available when the orientation detector
	// had no convincing candidate.
	if len(rotationCandidates) == 0 || rotationCandidateQuality(rotationCandidates[0]) < 25 {
		if payload, info, candidate, ok := searchV3AxisAlignedAffine(sourcePlane, decoder); ok {
			applyAffineCorrectionInfo(&info, candidate)
			return payload, info, nil
		}
	}

	// Decisive advanced geometry is still useful for bounded wrong-key failure,
	// but only after the established pure-resize path has had its opportunity.
	if directLatticeEvidence.decisive ||
		(len(rotationCandidates) > 0 && rotationCandidateQuality(rotationCandidates[0]) >= 25) {
		return nil, ExtractInfo{}, errors.New("v3 hidden payload not found or key is incorrect")
	}

	return nil, ExtractInfo{}, errors.New("v3 hidden payload not found or key is incorrect")
}

func searchV3PureResizePercent(sourcePlane *pixelPlane, decoder *decoder, percent int) ([]byte, ExtractInfo, bool) {
	bounds := sourcePlane.bounds
	baseWidth := int(math.Round(float64(bounds.Dx()) * 100 / float64(percent)))
	baseHeight := int(math.Round(float64(bounds.Dy()) * 100 / float64(percent)))
	neighborDeltas := [...]point{
		{0, 0},
		{-1, -1}, {0, -1}, {1, -1},
		{-1, 0}, {1, 0},
		{-1, 1}, {0, 1}, {1, 1},
	}
	for _, delta := range neighborDeltas {
		width := baseWidth + delta.x
		height := baseHeight + delta.y
		if width < tileWidth*blockSize || height < tileHeight*blockSize || exceedsPixelLimit(width, height, maxSearchPixels) {
			continue
		}
		normalized := resizePixelPlaneBilinear(sourcePlane, width, height)
		if payload, info, _, ok := searchV3Aligned(normalized, decoder); ok {
			return payload, info, true
		}
	}
	return nil, ExtractInfo{}, false
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
