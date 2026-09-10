package watermark

import (
	"image"
	"math"
	"sort"
)

const (
	diagnosticMaxPhaseRefineSeeds      = 3
	diagnosticPhaseTilesPerAxis        = 2
	diagnosticPhaseScaleProbesPerSeed  = 10
	diagnosticMaxPhaseCorrectionPixels = 64.0
)

var diagnosticPhaseScaleDeltas = [...]float64{-0.02, -0.01, 0.01, 0.02}

type diagnosticPhaseObservation struct {
	tileX, tileY   int
	phaseX, phaseY int
	z              float64
}

type diagnosticPhaseConsensus struct {
	profile      Profile
	score        float64
	xCoherence   float64
	yCoherence   float64
	meanZ        float64
	tiles        int
	observations []diagnosticPhaseObservation
}

func diagnosticProjectivePhaseConsensus(src image.Image, boundary PrintBoundaryEstimate, candidate DiagnosticScaleCandidate, decoder *decoder) diagnosticPhaseConsensus {
	width := int(math.Round(candidate.CanonicalWidthPixels))
	height := int(math.Round(candidate.CanonicalHeightPixels))
	if width < tileWidth*blockSize || height < tileHeight*blockSize {
		return diagnosticPhaseConsensus{}
	}
	h, ok := homographyForPrintBoundary(float64(width), float64(height), boundary)
	if !ok {
		return diagnosticPhaseConsensus{}
	}
	return diagnosticProjectivePhaseConsensusWithHomography(src, width, height, h, decoder)
}

func diagnosticProjectivePhaseConsensusWithHomography(src image.Image, width, height int, h homography, decoder *decoder) diagnosticPhaseConsensus {
	bw, bh := width/blockSize, height/blockSize
	fullTilesX, fullTilesY := bw/tileWidth, bh/tileHeight
	if fullTilesX <= 0 || fullTilesY <= 0 {
		return diagnosticPhaseConsensus{}
	}
	tileXs := diagnosticSpacedIndices(fullTilesX, diagnosticPhaseTilesPerAxis)
	tileYs := diagnosticSpacedIndices(fullTilesY, diagnosticPhaseTilesPerAxis)
	byProfile := make(map[Profile][]diagnosticPhaseObservation, len(decoder.v3Patterns))
	for _, tileY := range tileYs {
		for _, tileX := range tileXs {
			grid := make([]float64, eccBits)
			valid := true
			for logicalY := 0; logicalY < tileHeight && valid; logicalY++ {
				for logicalX := 0; logicalX < tileWidth; logicalX++ {
					bx := logicalX + tileX*tileWidth
					by := logicalY + tileY*tileHeight
					margin, ok := diagnosticReadProjectiveBlock(src, h, float64(bx*blockSize), float64(by*blockSize))
					if !ok {
						valid = false
						break
					}
					grid[logicalY*tileWidth+logicalX] = margin
				}
			}
			if !valid {
				continue
			}
			for _, pattern := range decoder.v3Patterns {
				phases := strongestV3Phases(grid, pattern, 1)
				if len(phases) == 0 || phases[0].total == 0 {
					continue
				}
				phase := phases[0]
				z := (float64(phase.score) - float64(phase.total)/2) / math.Sqrt(float64(phase.total)/4)
				byProfile[pattern.spec.profile] = append(byProfile[pattern.spec.profile], diagnosticPhaseObservation{
					tileX: tileX, tileY: tileY, phaseX: phase.x, phaseY: phase.y, z: z,
				})
			}
		}
	}

	best := diagnosticPhaseConsensus{}
	for profile, observations := range byProfile {
		if len(observations) < 2 {
			continue
		}
		sinX, cosX, sinY, cosY, weight, zSum := 0.0, 0.0, 0.0, 0.0, 0.0, 0.0
		for _, observation := range observations {
			w := math.Max(observation.z, 0.5)
			ax := 2 * math.Pi * float64(observation.phaseX) / float64(tileWidth)
			ay := 2 * math.Pi * float64(observation.phaseY) / float64(tileHeight)
			sinX += w * math.Sin(ax)
			cosX += w * math.Cos(ax)
			sinY += w * math.Sin(ay)
			cosY += w * math.Cos(ay)
			weight += w
			zSum += observation.z
		}
		if weight <= 0 {
			continue
		}
		xCoherence := math.Hypot(sinX, cosX) / weight
		yCoherence := math.Hypot(sinY, cosY) / weight
		meanZ := zSum / float64(len(observations))
		score := math.Sqrt(xCoherence*yCoherence) * (0.5 + 0.5*clampUnit(meanZ/5))
		if score > best.score {
			best = diagnosticPhaseConsensus{
				profile: profile, score: score, xCoherence: xCoherence, yCoherence: yCoherence,
				meanZ: meanZ, tiles: len(observations), observations: append([]diagnosticPhaseObservation(nil), observations...),
			}
		}
	}
	return best
}

func refineDiagnosticScaleByPhase(src image.Image, boundary PrintBoundaryEstimate, seed DiagnosticScaleCandidate, decoder *decoder) (DiagnosticScaleCandidate, diagnosticPhaseConsensus, int) {
	best := seed
	bestPhase := diagnosticProjectivePhaseConsensus(src, boundary, best, decoder)
	attempts := 1

	baseWidth := best.CanonicalWidthPixels
	for _, delta := range diagnosticPhaseScaleDeltas {
		candidate := best
		candidate.CanonicalWidthPixels = baseWidth * (1 + delta)
		phase := diagnosticProjectivePhaseConsensus(src, boundary, candidate, decoder)
		attempts++
		if diagnosticPhaseBetter(phase, candidate, bestPhase, best) {
			best, bestPhase = candidate, phase
		}
	}

	baseHeight := best.CanonicalHeightPixels
	for _, delta := range diagnosticPhaseScaleDeltas {
		candidate := best
		candidate.CanonicalHeightPixels = baseHeight * (1 + delta)
		phase := diagnosticProjectivePhaseConsensus(src, boundary, candidate, decoder)
		attempts++
		if diagnosticPhaseBetter(phase, candidate, bestPhase, best) {
			best, bestPhase = candidate, phase
		}
	}
	return best, bestPhase, attempts
}

func diagnosticPhaseBetter(a diagnosticPhaseConsensus, ac DiagnosticScaleCandidate, b diagnosticPhaseConsensus, bc DiagnosticScaleCandidate) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	if a.tiles != b.tiles {
		return a.tiles > b.tiles
	}
	if ac.Confidence != bc.Confidence {
		return ac.Confidence > bc.Confidence
	}
	if ac.CanonicalWidthPixels != bc.CanonicalWidthPixels {
		return ac.CanonicalWidthPixels < bc.CanonicalWidthPixels
	}
	return ac.CanonicalHeightPixels < bc.CanonicalHeightPixels
}

func diagnosticCircularPhaseTarget(observations []diagnosticPhaseObservation, axisX bool) int {
	modulus := tileHeight
	if axisX {
		modulus = tileWidth
	}
	sinSum, cosSum := 0.0, 0.0
	for _, observation := range observations {
		value := observation.phaseY
		if axisX {
			value = observation.phaseX
		}
		angle := 2 * math.Pi * float64(value) / float64(modulus)
		weight := math.Max(observation.z, 0.5)
		sinSum += weight * math.Sin(angle)
		cosSum += weight * math.Cos(angle)
	}
	if sinSum == 0 && cosSum == 0 {
		return 0
	}
	target := int(math.Round(math.Atan2(sinSum, cosSum) * float64(modulus) / (2 * math.Pi)))
	return positiveMod(target, modulus)
}

func diagnosticPhaseDifference(value, target, modulus int) float64 {
	diff := float64(value - target)
	span := float64(modulus)
	for diff > span/2 {
		diff -= span
	}
	for diff < -span/2 {
		diff += span
	}
	return diff
}

func diagnosticPhaseRefinedHomography(src image.Image, boundary PrintBoundaryEstimate, candidate DiagnosticScaleCandidate, phase diagnosticPhaseConsensus, decoder *decoder) (homography, bool) {
	if len(phase.observations) != 4 || phase.profile == "" {
		return homography{}, false
	}
	width := int(math.Round(candidate.CanonicalWidthPixels))
	height := int(math.Round(candidate.CanonicalHeightPixels))
	coarse, ok := homographyForPrintBoundary(float64(width), float64(height), boundary)
	if !ok {
		return homography{}, false
	}
	targetX := diagnosticCircularPhaseTarget(phase.observations, true)
	targetY := diagnosticCircularPhaseTarget(phase.observations, false)
	var canonical, source [4][2]float64
	for index, observation := range phase.observations {
		cx := float64(observation.tileX * tileWidth * blockSize)
		cy := float64(observation.tileY * tileHeight * blockSize)
		sx, sy, ok := coarse.mapPoint(cx, cy)
		if !ok {
			return homography{}, false
		}
		dx := diagnosticPhaseDifference(observation.phaseX, targetX, tileWidth) * blockSize
		dy := diagnosticPhaseDifference(observation.phaseY, targetY, tileHeight) * blockSize
		if math.Abs(dx) > diagnosticMaxPhaseCorrectionPixels || math.Abs(dy) > diagnosticMaxPhaseCorrectionPixels {
			return homography{}, false
		}
		// If one tile reports a positive logical phase drift, the canonical
		// coordinate that currently lands at the same source point lies behind
		// the nominal anchor by that many blocks. Fitting these four corrected
		// correspondences removes first-order projective phase drift.
		canonical[index] = [2]float64{cx - dx, cy - dy}
		source[index] = [2]float64{sx, sy}
	}
	return diagnosticHomographyFromFourPoints(canonical, source)
}

func diagnosticHomographyFromFourPoints(src, dst [4][2]float64) (homography, bool) {
	var a [8][9]float64
	for i := 0; i < 4; i++ {
		x, y := src[i][0], src[i][1]
		u, v := dst[i][0], dst[i][1]
		a[2*i] = [9]float64{x, y, 1, 0, 0, 0, -u * x, -u * y, u}
		a[2*i+1] = [9]float64{0, 0, 0, x, y, 1, -v * x, -v * y, v}
	}
	solution, ok := solveLinear8(a)
	if !ok {
		return homography{}, false
	}
	return homography{h: [9]float64{solution[0], solution[1], solution[2], solution[3], solution[4], solution[5], solution[6], solution[7], 1}}, true
}

type diagnosticAuthCandidate struct {
	source    string
	candidate DiagnosticScaleCandidate
	phase     diagnosticPhaseConsensus
	h         homography
	probe     diagnosticProjectiveProbe
}

func sortDiagnosticAuthCandidates(candidates []diagnosticAuthCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].phase.score != candidates[j].phase.score {
			return candidates[i].phase.score > candidates[j].phase.score
		}
		if candidates[i].probe.z != candidates[j].probe.z {
			return candidates[i].probe.z > candidates[j].probe.z
		}
		if candidates[i].candidate.Confidence != candidates[j].candidate.Confidence {
			return candidates[i].candidate.Confidence > candidates[j].candidate.Confidence
		}
		return candidates[i].candidate.CanonicalWidthPixels < candidates[j].candidate.CanonicalWidthPixels
	})
}
