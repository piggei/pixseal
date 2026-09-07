package watermark

import (
	"image"
	"math"
	"sort"
)

const (
	rotationProbeMinDegrees  = -45
	rotationProbeMaxDegrees  = 45
	rotationProbeStep        = 0.25
	rotationProbeMinContrast = 12.0
	rotationAlignedContrast  = 20.0
	rotationDecodeCandidates = 2
)

type rotationCandidate struct {
	angle    float64
	contrast float64
}

// detectRotationCandidates estimates arbitrary rotation modulo 90 degrees.
// It never performs payload decoding. Instead it samples a bounded set of
// oriented 8x8 blocks and measures how strongly one pixel phase exhibits the
// DCT coefficient separation imposed by PixSeal embedding compared with the
// median phase. The coarse probe space is fixed: 361 quarter-degree angles,
// 64 phases and at most 49 sampled blocks per phase. Only the strongest coarse
// peaks are refined locally at 0.05-degree resolution before any full decode.
func detectRotationCandidates(src *pixelPlane) []rotationCandidate {
	// A strong phase contrast at zero degrees means the 8x8 embedding lattice is
	// already aligned. This cheap pre-check avoids the full angle scan for the
	// common unrotated case, including most wrong-key and unmarked negatives.
	if aligned := probeRotationAngle(src, 0); aligned.contrast >= rotationAlignedContrast {
		return nil
	}

	candidates := make([]rotationCandidate, 0, 361)
	for angle := float64(rotationProbeMinDegrees); angle <= float64(rotationProbeMaxDegrees); angle += rotationProbeStep {
		if math.Abs(angle) < 1e-9 {
			continue // zero degrees was already measured by the aligned fast rejection.
		}
		candidates = append(candidates, probeRotationAngle(src, angle))
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].contrast == candidates[j].contrast {
			return math.Abs(candidates[i].angle) < math.Abs(candidates[j].angle)
		}
		return candidates[i].contrast > candidates[j].contrast
	})

	// A native/quarter-turn image produces its strongest lattice at 0 degrees.
	// Returning no arbitrary candidates prevents needless resampling in that case.
	if len(candidates) == 0 || math.Abs(candidates[0].angle) < 0.5 {
		return nil
	}
	if candidates[0].contrast < rotationProbeMinContrast {
		return nil
	}

	// Refine only the strongest distinct coarse peaks. Fractional-angle recovery
	// is sensitive to residual errors well below one degree after a second
	// resampling pass, so a small local 0.05-degree refinement is cheaper and
	// more reliable than decoding neighboring coarse angles.
	coarse := make([]rotationCandidate, 0, rotationDecodeCandidates)
	for _, candidate := range candidates {
		if math.Abs(candidate.angle) < 0.5 || candidate.contrast < rotationProbeMinContrast {
			continue
		}
		distinct := true
		for _, existing := range coarse {
			if math.Abs(existing.angle-candidate.angle) < 0.75 {
				distinct = false
				break
			}
		}
		if distinct {
			coarse = append(coarse, candidate)
		}
		if len(coarse) == rotationDecodeCandidates {
			break
		}
	}
	if len(coarse) == 0 {
		return nil
	}

	refined := make([]rotationCandidate, 0, len(coarse)*11)
	for _, candidate := range coarse {
		for delta := -0.25; delta <= 0.250001; delta += 0.05 {
			angle := candidate.angle + delta
			if angle < rotationProbeMinDegrees || angle > rotationProbeMaxDegrees || math.Abs(angle) < 0.5 {
				continue
			}
			refined = append(refined, probeRotationAngle(src, angle))
		}
	}
	sort.Slice(refined, func(i, j int) bool { return refined[i].contrast > refined[j].contrast })
	selected := make([]rotationCandidate, 0, rotationDecodeCandidates)
	for _, candidate := range refined {
		if candidate.contrast < rotationProbeMinContrast {
			continue
		}
		distinct := true
		for _, existing := range selected {
			if math.Abs(existing.angle-candidate.angle) < 0.2 {
				distinct = false
				break
			}
		}
		if distinct {
			selected = append(selected, candidate)
		}
		if len(selected) == rotationDecodeCandidates {
			break
		}
	}
	return selected
}

func probeRotationAngle(src *pixelPlane, angle float64) rotationCandidate {
	phaseScores := make([]float64, 0, blockSize*blockSize)
	width, height := src.bounds.Dx(), src.bounds.Dy()
	if width < blockSize*3 || height < blockSize*3 {
		return rotationCandidate{angle: angle}
	}

	for phaseY := 0; phaseY < blockSize; phaseY++ {
		for phaseX := 0; phaseX < blockSize; phaseX++ {
			sum := 0.0
			count := 0
			// 7x7 spatially spread samples are enough to detect lattice orientation
			// while keeping the bounded angle probe much cheaper than a full decode.
			for sampleY := 1; sampleY <= 7; sampleY++ {
				for sampleX := 1; sampleX <= 7; sampleX++ {
					targetX := sampleX * (width - blockSize) / 8
					targetY := sampleY * (height - blockSize) / 8
					originX := nearestBlockOrigin(targetX, phaseX, width-blockSize)
					originY := nearestBlockOrigin(targetY, phaseY, height-blockSize)
					value, ok := readOrientedBlockMargin(src, originX, originY, angle)
					if !ok {
						continue
					}
					sum += value
					count++
				}
			}
			if count > 0 {
				phaseScores = append(phaseScores, sum/float64(count))
			}
		}
	}
	if len(phaseScores) == 0 {
		return rotationCandidate{angle: angle}
	}
	sort.Float64s(phaseScores)
	maximum := phaseScores[len(phaseScores)-1]
	median := phaseScores[len(phaseScores)/2]
	return rotationCandidate{
		angle:    angle,
		contrast: maximum - median,
	}
}

func nearestBlockOrigin(target, phase, maximum int) int {
	origin := target - positiveMod(target-phase, blockSize)
	for origin < 0 {
		origin += blockSize
	}
	if origin > maximum {
		origin -= blockSize
	}
	return origin
}

func readOrientedBlockMargin(src *pixelPlane, originX, originY int, angle float64) (float64, bool) {
	radians := angle * math.Pi / 180
	cosine, sine := math.Cos(radians), math.Sin(radians)
	centerX := float64(src.bounds.Dx()-1) / 2
	centerY := float64(src.bounds.Dy()-1) / 2
	coefficient23 := 0.0
	coefficient32 := 0.0

	for y := 0; y < blockSize; y++ {
		for x := 0; x < blockSize; x++ {
			dx := float64(originX+x) - centerX
			dy := float64(originY+y) - centerY
			sourceX := cosine*dx - sine*dy + centerX
			sourceY := sine*dx + cosine*dy + centerY
			luminance, ok := samplePlaneLuminance(src, sourceX, sourceY)
			if !ok {
				return 0, false
			}
			coefficient23 += luminance * cosTable[3][x] * cosTable[2][y]
			coefficient32 += luminance * cosTable[2][x] * cosTable[3][y]
		}
	}
	return math.Abs(math.Abs(coefficient23) - math.Abs(coefficient32)), true
}

func samplePlaneLuminance(src *pixelPlane, x, y float64) (float64, bool) {
	width, height := src.bounds.Dx(), src.bounds.Dy()
	if x < 0 || y < 0 || x > float64(width-1) || y > float64(height-1) {
		return 0, false
	}
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := x0+1, y0+1
	if x1 >= width {
		x1 = width - 1
	}
	if y1 >= height {
		y1 = height - 1
	}
	fx, fy := x-float64(x0), y-float64(y0)
	luma := func(px, py int) float64 {
		index := (py*width + px) * 3
		return .299*float64(src.rgb[index]) + .587*float64(src.rgb[index+1]) + .114*float64(src.rgb[index+2]) - 128
	}
	top := luma(x0, y0)*(1-fx) + luma(x1, y0)*fx
	bottom := luma(x0, y1)*(1-fx) + luma(x1, y1)*fx
	return top*(1-fy) + bottom*fy, true
}

func rotatedPixelDimensions(width, height int, degrees float64) (int, int) {
	radians := degrees * math.Pi / 180
	cosine, sine := math.Cos(radians), math.Sin(radians)
	outWidth := int(math.Ceil(math.Abs(float64(width)*cosine) + math.Abs(float64(height)*sine)))
	outHeight := int(math.Ceil(math.Abs(float64(width)*sine) + math.Abs(float64(height)*cosine)))
	return outWidth, outHeight
}

// rotatePixelPlane returns a white-canvas, expanded rotation. Positive angles
// rotate the visible image counter-clockwise. Bilinear interpolation mirrors a
// typical digital image editor closely enough for geometry recovery tests.
func rotatePixelPlane(src *pixelPlane, degrees float64) *pixelPlane {
	if math.Abs(degrees) < 1e-9 {
		return src
	}
	width, height := src.bounds.Dx(), src.bounds.Dy()
	radians := degrees * math.Pi / 180
	cosine, sine := math.Cos(radians), math.Sin(radians)
	outWidth, outHeight := rotatedPixelDimensions(width, height, degrees)
	out := &pixelPlane{
		bounds: image.Rect(0, 0, outWidth, outHeight),
		rgb:    make([]uint8, outWidth*outHeight*3),
	}
	for index := range out.rgb {
		out.rgb[index] = 255
	}

	sourceCenterX := float64(width-1) / 2
	sourceCenterY := float64(height-1) / 2
	outCenterX := float64(outWidth-1) / 2
	outCenterY := float64(outHeight-1) / 2

	// Inverse map each destination pixel into the source.
	for y := 0; y < outHeight; y++ {
		for x := 0; x < outWidth; x++ {
			dx := float64(x) - outCenterX
			dy := float64(y) - outCenterY
			sourceX := cosine*dx + sine*dy + sourceCenterX
			sourceY := -sine*dx + cosine*dy + sourceCenterY
			if sourceX < 0 || sourceY < 0 || sourceX > float64(width-1) || sourceY > float64(height-1) {
				continue
			}
			x0, y0 := int(math.Floor(sourceX)), int(math.Floor(sourceY))
			x1, y1 := x0+1, y0+1
			if x1 >= width {
				x1 = width - 1
			}
			if y1 >= height {
				y1 = height - 1
			}
			fx, fy := sourceX-float64(x0), sourceY-float64(y0)
			destination := (y*outWidth + x) * 3
			for channel := 0; channel < 3; channel++ {
				at := func(px, py int) float64 {
					return float64(src.rgb[(py*width+px)*3+channel])
				}
				top := at(x0, y0)*(1-fx) + at(x1, y0)*fx
				bottom := at(x0, y1)*(1-fx) + at(x1, y1)*fx
				out.rgb[destination+channel] = clamp(top*(1-fy) + bottom*fy)
			}
		}
	}
	return out
}

// rotatePixelPlaneQuarter performs lossless right-angle rotations. turns is the
// number of counter-clockwise quarter turns and is normalized modulo four.
func rotatePixelPlaneQuarter(src *pixelPlane, turns int) *pixelPlane {
	turns = positiveMod(turns, 4)
	if turns == 0 {
		return src
	}
	width, height := src.bounds.Dx(), src.bounds.Dy()
	outWidth, outHeight := width, height
	if turns%2 == 1 {
		outWidth, outHeight = height, width
	}
	out := &pixelPlane{
		bounds: image.Rect(0, 0, outWidth, outHeight),
		rgb:    make([]uint8, outWidth*outHeight*3),
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			destinationX, destinationY := x, y
			switch turns {
			case 1:
				destinationX, destinationY = y, width-1-x
			case 2:
				destinationX, destinationY = width-1-x, height-1-y
			case 3:
				destinationX, destinationY = height-1-y, x
			}
			sourceIndex := (y*width + x) * 3
			destinationIndex := (destinationY*outWidth + destinationX) * 3
			copy(out.rgb[destinationIndex:destinationIndex+3], src.rgb[sourceIndex:sourceIndex+3])
		}
	}
	return out
}

func normalizeDegrees(degrees float64) float64 {
	for degrees >= 180 {
		degrees -= 360
	}
	for degrees < -180 {
		degrees += 360
	}
	if math.Abs(degrees) < 1e-9 {
		return 0
	}
	return degrees
}
