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
	rotationProbeFineStep    = 0.05
	rotationProbeMinContrast = 12.0 // reference threshold at an 8-pixel lattice
	rotationAlignedContrast  = 20.0 // reference threshold at an 8-pixel lattice
	rotationDecodeCandidates = 2
	rotationCoarseCandidates = 3
)

// rotationProbeBlockSizes are the useful integer lattice sizes already
// supported by the baseline decoder that remain useful after arbitrary-angle
// resampling: native 100% and 75%. A 50% carrier usually loses too much signal
// after the second interpolation, so arbitrary-angle probing does not spend time
// on the 4-pixel lattice. Quarter-turn plus 50% remains handled losslessly.
var rotationProbeBlockSizes = [...]int{8, 6}

type rotationCandidate struct {
	angle     float64
	contrast  float64
	blockSize int
}

func rotationContrastScale(size int) float64 {
	return float64(size*size) / float64(blockSize*blockSize)
}

func rotationCandidateQuality(candidate rotationCandidate) float64 {
	if candidate.blockSize <= 0 {
		return 0
	}
	return candidate.contrast / rotationContrastScale(candidate.blockSize)
}

func rotationCandidatePasses(candidate rotationCandidate) bool {
	return candidate.contrast >= rotationProbeMinContrast*rotationContrastScale(candidate.blockSize)
}

// detectRotationCandidates estimates arbitrary rotation modulo 90 degrees and
// the nearest native DCT lattice size. It never performs payload decoding.
//
// Search is explicitly bounded and hierarchical:
//   - 2 zero-degree alignment probes (8/6 px)
//   - at most 720 non-zero coarse probes (2 sizes x 360 quarter-degree angles)
//   - at most 33 fine probes (3 peaks x 11 probes at 0.05 degree)
//   - at most 2 candidates reach the full authenticated decoder
//
// Contrast is normalized for block area before candidates from different
// lattice sizes are ranked. Smaller lattices use fewer spatial samples, keeping
// the multi-scale probe close to the cost of the build-3 single-scale search.
func detectRotationCandidates(src *pixelPlane) []rotationCandidate {
	for _, size := range rotationProbeBlockSizes {
		aligned := probeRotationAngle(src, 0, size)
		if aligned.contrast >= rotationAlignedContrast*rotationContrastScale(size) {
			return nil
		}
	}

	coarseAll := make([]rotationCandidate, 0, 720)
	for _, size := range rotationProbeBlockSizes {
		for angle := float64(rotationProbeMinDegrees); angle <= float64(rotationProbeMaxDegrees)+1e-9; angle += rotationProbeStep {
			if math.Abs(angle) < 1e-9 {
				continue
			}
			coarseAll = append(coarseAll, probeRotationAngle(src, angle, size))
		}
	}
	sortRotationCandidates(coarseAll)
	if len(coarseAll) == 0 || !rotationCandidatePasses(coarseAll[0]) {
		return nil
	}

	coarse := selectDistinctRotationCandidates(coarseAll, rotationCoarseCandidates, 0.75)
	if len(coarse) == 0 {
		return nil
	}

	fine := make([]rotationCandidate, 0, len(coarse)*11)
	for _, candidate := range coarse {
		for delta := -0.25; delta <= 0.250001; delta += rotationProbeFineStep {
			angle := candidate.angle + delta
			if angle < rotationProbeMinDegrees || angle > rotationProbeMaxDegrees || math.Abs(angle) < 0.5 {
				continue
			}
			fine = append(fine, probeRotationAngle(src, angle, candidate.blockSize))
		}
	}
	sortRotationCandidates(fine)
	return selectDistinctRotationCandidates(fine, rotationDecodeCandidates, 0.2)
}

func sortRotationCandidates(candidates []rotationCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		qi := rotationCandidateQuality(candidates[i])
		qj := rotationCandidateQuality(candidates[j])
		if qi == qj {
			if candidates[i].blockSize == candidates[j].blockSize {
				return math.Abs(candidates[i].angle) < math.Abs(candidates[j].angle)
			}
			return candidates[i].blockSize > candidates[j].blockSize
		}
		return qi > qj
	})
}

func selectDistinctRotationCandidates(candidates []rotationCandidate, count int, angleDistance float64) []rotationCandidate {
	selected := make([]rotationCandidate, 0, count)
	for _, candidate := range candidates {
		if !rotationCandidatePasses(candidate) || math.Abs(candidate.angle) < 0.5 {
			continue
		}
		distinct := true
		for _, existing := range selected {
			if existing.blockSize == candidate.blockSize && math.Abs(existing.angle-candidate.angle) < angleDistance {
				distinct = false
				break
			}
		}
		if distinct {
			selected = append(selected, candidate)
		}
		if len(selected) == count {
			break
		}
	}
	return selected
}

func probeRotationAngle(src *pixelPlane, angle float64, size int) rotationCandidate {
	phaseScores := make([]float64, 0, size*size)
	width, height := src.bounds.Dx(), src.bounds.Dy()
	if width < size*3 || height < size*3 {
		return rotationCandidate{angle: angle, blockSize: size}
	}

	sampleGrid := 7
	if size < blockSize {
		sampleGrid = 5
	}
	denominator := sampleGrid + 1
	for phaseY := 0; phaseY < size; phaseY++ {
		for phaseX := 0; phaseX < size; phaseX++ {
			sum := 0.0
			count := 0
			for sampleY := 1; sampleY <= sampleGrid; sampleY++ {
				for sampleX := 1; sampleX <= sampleGrid; sampleX++ {
					targetX := sampleX * (width - size) / denominator
					targetY := sampleY * (height - size) / denominator
					originX := nearestBlockOrigin(targetX, phaseX, width-size, size)
					originY := nearestBlockOrigin(targetY, phaseY, height-size, size)
					value, ok := readOrientedBlockMargin(src, originX, originY, angle, size)
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
		return rotationCandidate{angle: angle, blockSize: size}
	}
	sort.Float64s(phaseScores)
	maximum := phaseScores[len(phaseScores)-1]
	median := phaseScores[len(phaseScores)/2]
	return rotationCandidate{
		angle:     angle,
		contrast:  maximum - median,
		blockSize: size,
	}
}

func nearestBlockOrigin(target, phase, maximum, size int) int {
	origin := target - positiveMod(target-phase, size)
	for origin < 0 {
		origin += size
	}
	if origin > maximum {
		origin -= size
	}
	return origin
}

func readOrientedBlockMargin(src *pixelPlane, originX, originY int, angle float64, size int) (float64, bool) {
	radians := angle * math.Pi / 180
	cosine, sine := math.Cos(radians), math.Sin(radians)
	centerX := float64(src.bounds.Dx()-1) / 2
	centerY := float64(src.bounds.Dy()-1) / 2
	coefficient23 := 0.0
	coefficient32 := 0.0
	table := readCosTables[size]

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(originX+x) - centerX
			dy := float64(originY+y) - centerY
			sourceX := cosine*dx - sine*dy + centerX
			sourceY := sine*dx + cosine*dy + centerY
			luminance, ok := samplePlaneLuminance(src, sourceX, sourceY)
			if !ok {
				return 0, false
			}
			coefficient23 += luminance * table[3][x] * table[2][y]
			coefficient32 += luminance * table[2][x] * table[3][y]
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
