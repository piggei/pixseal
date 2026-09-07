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
	rotationProbeMinContrast = 17.0 // reference threshold at an 8-pixel lattice
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
func hasStrongZeroDegreeLattice(src *pixelPlane) bool {
	for _, size := range rotationProbeBlockSizes {
		aligned := probeRotationAngle(src, 0, size)
		if aligned.contrast >= rotationAlignedContrast*rotationContrastScale(size) {
			return true
		}
	}
	return false
}

func detectRotationCandidates(src *pixelPlane) []rotationCandidate {
	// A strong zero-degree contrast alone is not enough once affine distortion is
	// allowed: anisotropic scale or shear can create a misleading local peak at
	// zero even when the periodic v3 lattice is not geometrically aligned. Build 6
	// therefore uses the key-independent repetition signal to confirm the fast
	// path. Native/wrong-key carriers retain the cheap exit, while low-coherence
	// images continue to the bounded angle estimator.
	if hasStrongZeroDegreeLattice(src) && nativeRepetitionCoherence(src) >= 0.82 {
		return nil
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

// linearTransform maps coordinates in a rectified carrier into coordinates in
// the observed image, relative to the image centre. Build 5 uses this small
// matrix model for axis-aligned affine distortion (anisotropic scale or shear).
type linearTransform struct {
	a, b float64
	c, d float64
}

func (m linearTransform) determinant() float64 { return m.a*m.d - m.b*m.c }

func (m linearTransform) inverse() (linearTransform, bool) {
	det := m.determinant()
	if math.Abs(det) < 1e-9 {
		return linearTransform{}, false
	}
	return linearTransform{a: m.d / det, b: -m.b / det, c: -m.c / det, d: m.a / det}, true
}

type affineKind string

const (
	affineScaleXY affineKind = "scale-xy"
	affineShearX  affineKind = "shear-x"
	affineShearY  affineKind = "shear-y"
)

type affineCandidate struct {
	kind       affineKind
	parameter  float64
	parameter2 float64
	matrix     linearTransform
	contrast   float64
	blockSize  int
}

func shearXMatrix(amount float64) linearTransform {
	return linearTransform{a: 1, b: amount, d: 1}
}

func shearYMatrix(amount float64) linearTransform {
	return linearTransform{a: 1, c: amount, d: 1}
}

type affineVirtualBounds struct {
	minX, minY float64
	width      int
	height     int
}

func affineOutputBounds(src *pixelPlane, matrix linearTransform) (affineVirtualBounds, bool) {
	inverse, ok := matrix.inverse()
	if !ok {
		return affineVirtualBounds{}, false
	}
	halfW := float64(src.bounds.Dx()-1) / 2
	halfH := float64(src.bounds.Dy()-1) / 2
	corners := [][2]float64{{-halfW, -halfH}, {halfW, -halfH}, {-halfW, halfH}, {halfW, halfH}}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, corner := range corners {
		x := inverse.a*corner[0] + inverse.b*corner[1]
		y := inverse.c*corner[0] + inverse.d*corner[1]
		minX = math.Min(minX, x)
		maxX = math.Max(maxX, x)
		minY = math.Min(minY, y)
		maxY = math.Max(maxY, y)
	}
	width := int(math.Floor(maxX-minX)) + 1
	height := int(math.Floor(maxY-minY)) + 1
	if width < 1 || height < 1 {
		return affineVirtualBounds{}, false
	}
	return affineVirtualBounds{minX: minX, minY: minY, width: width, height: height}, true
}

type affineProbe struct {
	contrast float64
	phases   []point
}

func readAffineBlockValue(src *pixelPlane, bounds affineVirtualBounds, matrix linearTransform, originX, originY, size int) (float64, bool) {
	table := readCosTables[size]
	coefficient23, coefficient32 := 0.0, 0.0
	sourceCenterX := float64(src.bounds.Dx()-1) / 2
	sourceCenterY := float64(src.bounds.Dy()-1) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			rectifiedX := bounds.minX + float64(originX+x)
			rectifiedY := bounds.minY + float64(originY+y)
			sourceX := matrix.a*rectifiedX + matrix.b*rectifiedY + sourceCenterX
			sourceY := matrix.c*rectifiedX + matrix.d*rectifiedY + sourceCenterY
			luminance, ok := samplePlaneLuminance(src, sourceX, sourceY)
			if !ok {
				return 0, false
			}
			coefficient23 += luminance * table[3][x] * table[2][y]
			coefficient32 += luminance * table[2][x] * table[3][y]
		}
	}
	return math.Abs(coefficient23) - math.Abs(coefficient32), true
}

func probeAffineMatrixDetailed(src *pixelPlane, matrix linearTransform, size int) affineProbe {
	bounds, ok := affineOutputBounds(src, matrix)
	if !ok || bounds.width < size*3 || bounds.height < size*3 {
		return affineProbe{}
	}
	sampleGrid := 5
	denominator := sampleGrid + 1
	phaseScores := make([]float64, 0, size*size)
	type scoredPhase struct {
		score float64
		x, y  int
	}
	all := make([]scoredPhase, 0, size*size)
	for phaseY := 0; phaseY < size; phaseY++ {
		for phaseX := 0; phaseX < size; phaseX++ {
			sum, count := 0.0, 0
			for sampleY := 1; sampleY <= sampleGrid; sampleY++ {
				for sampleX := 1; sampleX <= sampleGrid; sampleX++ {
					targetX := sampleX * (bounds.width - size) / denominator
					targetY := sampleY * (bounds.height - size) / denominator
					originX := nearestBlockOrigin(targetX, phaseX, bounds.width-size, size)
					originY := nearestBlockOrigin(targetY, phaseY, bounds.height-size, size)
					value, valid := readAffineBlockValue(src, bounds, matrix, originX, originY, size)
					if !valid {
						continue
					}
					sum += math.Abs(value)
					count++
				}
			}
			if count > 0 {
				all = append(all, scoredPhase{score: sum / float64(count), x: phaseX, y: phaseY})
				phaseScores = append(phaseScores, sum/float64(count))
			}
		}
	}
	if len(all) == 0 {
		return affineProbe{}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].score > all[j].score })
	sort.Float64s(phaseScores)
	phaseCount := 12
	if len(all) < phaseCount {
		phaseCount = len(all)
	}
	phases := make([]point, 0, phaseCount+1)
	phases = append(phases, point{x: 0, y: 0})
	for _, candidate := range all[:phaseCount] {
		phase := point{x: candidate.x, y: candidate.y}
		duplicate := false
		for _, existing := range phases {
			if existing == phase {
				duplicate = true
				break
			}
		}
		if !duplicate {
			phases = append(phases, phase)
		}
	}
	return affineProbe{
		contrast: all[0].score - phaseScores[len(phaseScores)/2],
		phases:   phases,
	}
}

func aggregateAffineGrid(src *pixelPlane, matrix linearTransform, size, offsetX, offsetY int) ([]float64, bool) {
	bounds, ok := affineOutputBounds(src, matrix)
	if !ok {
		return nil, false
	}
	blocksWide := (bounds.width - offsetX) / size
	blocksHigh := (bounds.height - offsetY) / size
	if blocksWide < tileWidth || blocksHigh < tileHeight {
		return nil, false
	}
	grid := make([]float64, eccBits)
	for blockY := 0; blockY < blocksHigh; blockY++ {
		for blockX := 0; blockX < blocksWide; blockX++ {
			originX := offsetX + blockX*size
			originY := offsetY + blockY*size
			value, valid := readAffineBlockValue(src, bounds, matrix, originX, originY, size)
			if !valid {
				continue
			}
			position := (blockY%tileHeight)*tileWidth + blockX%tileWidth
			grid[position] += value
		}
	}
	return grid, true
}

func axisAlignedAffineHypotheses() []affineCandidate {
	// Build 5 deliberately limits native-lattice anisotropic scale to +/-10% per
	// axis and shear to 3/5/8/10 degrees. Uniform scale is already handled by the
	// established resize path; equal X/Y pairs are therefore omitted here.
	scales := [...]float64{0.90, 0.95, 1.00, 1.05, 1.10}
	result := make([]affineCandidate, 0, 40)
	for _, scaleX := range scales {
		for _, scaleY := range scales {
			if math.Abs(scaleX-scaleY) < 1e-9 {
				continue
			}
			result = append(result, affineCandidate{
				kind:       affineScaleXY,
				parameter:  scaleX,
				parameter2: scaleY,
				matrix:     linearTransform{a: scaleX, d: scaleY},
				blockSize:  blockSize,
			})
		}
	}
	for _, degrees := range [...]float64{3, 5, 8, 10} {
		amount := math.Tan(degrees * math.Pi / 180)
		for _, signed := range []float64{-amount, amount} {
			result = append(result,
				affineCandidate{kind: affineShearX, parameter: signed, matrix: shearXMatrix(signed), blockSize: blockSize},
				affineCandidate{kind: affineShearY, parameter: signed, matrix: shearYMatrix(signed), blockSize: blockSize},
			)
		}
	}
	return result
}

func aggregateAffineTile(src *pixelPlane, matrix linearTransform, size int, phase point) ([]float64, bool) {
	bounds, ok := affineOutputBounds(src, matrix)
	if !ok {
		return nil, false
	}
	blocksWide := (bounds.width - phase.x) / size
	blocksHigh := (bounds.height - phase.y) / size
	if blocksWide < tileWidth || blocksHigh < tileHeight {
		return nil, false
	}

	// Use a single complete tile nearest the centre. This is enough for an
	// authenticated v3 decision and keeps each affine hypothesis inexpensive.
	startBlockX := (blocksWide - tileWidth) / 2
	startBlockY := (blocksHigh - tileHeight) / 2
	grid := make([]float64, eccBits)
	for tileY := 0; tileY < tileHeight; tileY++ {
		for tileX := 0; tileX < tileWidth; tileX++ {
			originX := phase.x + (startBlockX+tileX)*size
			originY := phase.y + (startBlockY+tileY)*size
			value, valid := readAffineBlockValue(src, bounds, matrix, originX, originY, size)
			if !valid {
				return nil, false
			}
			grid[tileY*tileWidth+tileX] = value
		}
	}
	return grid, true
}

func affineRepetitionCoherence(src *pixelPlane, matrix linearTransform, size int, phase point) (float64, bool) {
	bounds, ok := affineOutputBounds(src, matrix)
	if !ok {
		return 0, false
	}
	blocksWide := (bounds.width - phase.x) / size
	blocksHigh := (bounds.height - phase.y) / size
	if blocksWide < tileWidth || blocksHigh < tileHeight {
		return 0, false
	}

	same, total := 0, 0
	compare := func(originAX, originAY, originBX, originBY int) {
		for logical := 0; logical < eccBits; logical += 11 {
			x := logical % tileWidth
			y := logical / tileWidth
			valueA, validA := readAffineBlockValue(src, bounds, matrix,
				phase.x+(originAX+x)*size, phase.y+(originAY+y)*size, size)
			valueB, validB := readAffineBlockValue(src, bounds, matrix,
				phase.x+(originBX+x)*size, phase.y+(originBY+y)*size, size)
			if !validA || !validB {
				continue
			}
			total++
			if (valueA >= 0) == (valueB >= 0) {
				same++
			}
		}
	}

	if blocksWide >= 2*tileWidth {
		startX := (blocksWide - 2*tileWidth) / 2
		startY := (blocksHigh - tileHeight) / 2
		compare(startX, startY, startX+tileWidth, startY)
	}
	if blocksHigh >= 2*tileHeight {
		startX := (blocksWide - tileWidth) / 2
		startY := (blocksHigh - 2*tileHeight) / 2
		compare(startX, startY, startX, startY+tileHeight)
	}
	if total == 0 {
		return 0, false
	}
	return float64(same) / float64(total), true
}

func affineCandidatePhases(candidate affineCandidate, probe affineProbe) []point {
	phases := append([]point(nil), probe.phases...)
	appendUnique := func(phase point) {
		for _, existing := range phases {
			if existing == phase {
				return
			}
		}
		phases = append(phases, phase)
	}
	appendUnique(point{x: 0, y: 0})
	switch candidate.kind {
	case affineShearX:
		for x := 0; x < blockSize; x++ {
			appendUnique(point{x: x, y: 0})
		}
	case affineShearY:
		for y := 0; y < blockSize; y++ {
			appendUnique(point{x: 0, y: y})
		}
	}
	return phases
}

func bestAffineCoherence(src *pixelPlane, candidate affineCandidate, probe affineProbe) (float64, []point) {
	type scored struct {
		phase point
		score float64
	}
	values := make([]scored, 0, 3)
	for _, phase := range affineCandidatePhases(candidate, probe) {
		score, ok := affineRepetitionCoherence(src, candidate.matrix, blockSize, phase)
		if !ok {
			continue
		}
		insertAt := len(values)
		for i, existing := range values {
			if score > existing.score {
				insertAt = i
				break
			}
		}
		if insertAt < 3 {
			values = append(values, scored{})
			copy(values[insertAt+1:], values[insertAt:])
			values[insertAt] = scored{phase: phase, score: score}
			if len(values) > 3 {
				values = values[:3]
			}
		}
	}
	if len(values) == 0 {
		return 0, nil
	}
	phases := make([]point, len(values))
	for i, value := range values {
		phases[i] = value.phase
	}
	return values[0].score, phases
}

func nativeRepetitionCoherence(src *pixelPlane) float64 {
	identity := affineCandidate{kind: affineScaleXY, parameter: 1, parameter2: 1, matrix: linearTransform{a: 1, d: 1}, blockSize: blockSize}
	phases := make([]point, 0, blockSize*blockSize)
	for y := 0; y < blockSize; y++ {
		for x := 0; x < blockSize; x++ {
			phases = append(phases, point{x: x, y: y})
		}
	}
	best := 0.0
	for _, phase := range phases {
		score, ok := affineRepetitionCoherence(src, identity.matrix, blockSize, phase)
		if ok && score > best {
			best = score
		}
	}
	return best
}

// searchV3AxisAlignedAffine performs authenticated decoding directly through a
// virtual affine sampler. No corrected full-resolution image is materialized.
// Repetition coherence gates the expensive path and ranks matrices without using
// the key. Authentication remains the sole acceptance criterion.
func searchV3AxisAlignedAffine(src *pixelPlane, decoder *decoder) ([]byte, ExtractInfo, affineCandidate, bool) {
	const minCoherence = 0.72
	type probed struct {
		candidate affineCandidate
		probe     affineProbe
		coherence float64
		phases    []point
	}
	type fullCandidate struct {
		candidate affineCandidate
		phase     point
		score     int
	}

	// A highly coherent native lattice means geometry is already correct. If the
	// authenticated direct decoder failed, the most likely cause is a wrong key or
	// damaged payload; do not spend the affine budget on it.
	if nativeRepetitionCoherence(src) >= 0.82 {
		return nil, ExtractInfo{}, affineCandidate{}, false
	}

	hypotheses := axisAlignedAffineHypotheses()
	probedCandidates := make([]probed, 0, len(hypotheses))
	for _, candidate := range hypotheses {
		probe := probeAffineMatrixDetailed(src, candidate.matrix, blockSize)
		candidate.contrast = probe.contrast
		coherence, phases := bestAffineCoherence(src, candidate, probe)
		if coherence < minCoherence || len(phases) == 0 {
			continue
		}
		probedCandidates = append(probedCandidates, probed{candidate: candidate, probe: probe, coherence: coherence, phases: phases})
	}
	sort.Slice(probedCandidates, func(i, j int) bool {
		if probedCandidates[i].coherence == probedCandidates[j].coherence {
			return probedCandidates[i].probe.contrast > probedCandidates[j].probe.contrast
		}
		return probedCandidates[i].coherence > probedCandidates[j].coherence
	})
	if len(probedCandidates) > 6 {
		probedCandidates = probedCandidates[:6]
	}

	bestFull := make([]fullCandidate, 0, 4)
	considerFull := func(candidate fullCandidate) {
		insertAt := len(bestFull)
		for i, existing := range bestFull {
			if candidate.score > existing.score {
				insertAt = i
				break
			}
		}
		if insertAt >= 4 {
			return
		}
		bestFull = append(bestFull, fullCandidate{})
		copy(bestFull[insertAt+1:], bestFull[insertAt:])
		bestFull[insertAt] = candidate
		if len(bestFull) > 4 {
			bestFull = bestFull[:4]
		}
	}

	for _, entry := range probedCandidates {
		for _, phase := range entry.phases {
			grid, ok := aggregateAffineTile(src, entry.candidate.matrix, blockSize, phase)
			if !ok {
				continue
			}
			payload, info, score, found := decoder.decodeGrid(grid)
			if found {
				return payload, info, entry.candidate, true
			}
			considerFull(fullCandidate{candidate: entry.candidate, phase: phase, score: score})
		}
	}

	for _, entry := range bestFull {
		grid, ok := aggregateAffineGrid(src, entry.candidate.matrix, blockSize, entry.phase.x, entry.phase.y)
		if !ok {
			continue
		}
		payload, info, _, found := decoder.decodeGrid(grid)
		if found {
			return payload, info, entry.candidate, true
		}
	}
	return nil, ExtractInfo{}, affineCandidate{}, false
}

// latticeCandidate describes a composed linear geometry candidate scored
// directly from the repeated v3 DCT lattice. Unlike the build-6 composition
// stage, this estimator does not depend on a prior rotation estimate.
type latticeCandidate struct {
	angle     float64
	scaleX    float64
	scaleY    float64
	matrix    linearTransform
	quick     float64
	coherence float64
	contrast  float64
	phases    []point
	decisive  bool
}

func rotationMatrix(degrees float64) linearTransform {
	radians := degrees * math.Pi / 180
	cosine, sine := math.Cos(radians), math.Sin(radians)
	return linearTransform{a: cosine, b: -sine, c: sine, d: cosine}
}

func multiplyLinearTransforms(left, right linearTransform) linearTransform {
	return linearTransform{
		a: left.a*right.a + left.b*right.c,
		b: left.a*right.b + left.b*right.d,
		c: left.c*right.a + left.d*right.c,
		d: left.c*right.b + left.d*right.d,
	}
}

var directLatticeScale = [2]float64{1.10, 0.90}

// quickLatticeCoherence is the inexpensive first stage of direct lattice
// estimation. It samples a deterministic sparse subset of v3 tile positions
// and measures sign agreement between adjacent repeated tiles through the
// candidate transform. The score is key-independent and is used only for
// ranking; HMAC remains the sole acceptance criterion.
//
// Pixel phase is searched coarsely on the even 4x4 grid and then refined in a
// 3x3 neighbourhood. With 20 logical samples this rejects ordinary image
// texture far more reliably than the build-6 orientation-only detector while
// keeping the cost independent of image dimensions.
func quickLatticeCoherence(src *pixelPlane, matrix linearTransform, size int) (float64, point) {
	bounds, ok := affineOutputBounds(src, matrix)
	if !ok {
		return 0, point{}
	}
	blocksWide := bounds.width / size
	blocksHigh := bounds.height / size
	if blocksWide < tileWidth || blocksHigh < tileHeight {
		return 0, point{}
	}

	logicalPositions := make([]int, 20)
	for i := range logicalPositions {
		// 97 is coprime with 1120, so the samples are spread over the whole tile.
		logicalPositions[i] = (i*97 + 13) % eccBits
	}

	scorePhase := func(phase point) (float64, bool) {
		same, total := 0, 0
		compare := func(originAX, originAY, originBX, originBY int) {
			for _, logical := range logicalPositions {
				x := logical % tileWidth
				y := logical / tileWidth
				valueA, validA := readAffineBlockValue(src, bounds, matrix,
					phase.x+(originAX+x)*size, phase.y+(originAY+y)*size, size)
				valueB, validB := readAffineBlockValue(src, bounds, matrix,
					phase.x+(originBX+x)*size, phase.y+(originBY+y)*size, size)
				if !validA || !validB {
					continue
				}
				total++
				if (valueA >= 0) == (valueB >= 0) {
					same++
				}
			}
		}
		if blocksWide >= 2*tileWidth {
			startX := (blocksWide - 2*tileWidth) / 2
			startY := (blocksHigh - tileHeight) / 2
			compare(startX, startY, startX+tileWidth, startY)
		}
		if blocksHigh >= 2*tileHeight {
			startX := (blocksWide - tileWidth) / 2
			startY := (blocksHigh - 2*tileHeight) / 2
			compare(startX, startY, startX, startY+tileHeight)
		}
		if total == 0 {
			return 0, false
		}
		return float64(same) / float64(total), true
	}

	best, bestPhase := 0.0, point{}
	for y := 0; y < size; y += 2 {
		for x := 0; x < size; x += 2 {
			phase := point{x: x, y: y}
			if score, valid := scorePhase(phase); valid && score > best {
				best, bestPhase = score, phase
			}
		}
	}
	coarse := bestPhase
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			phase := point{x: positiveMod(coarse.x+dx, size), y: positiveMod(coarse.y+dy, size)}
			if score, valid := scorePhase(phase); valid && score > best {
				best, bestPhase = score, phase
			}
		}
	}
	return best, bestPhase
}

// searchV3DirectLatticeComposition estimates the composed DCT lattice directly
// instead of assuming that rotation can first be recovered independently from
// anisotropic scale. This is the build-7 replacement for the experimental
// build-6 rotation-first composition path.
//
// Build 7 deliberately promotes only the already-tested 110%x90% composition
// baseline. Broader scale pairs remain research work instead of multiplying the
// negative-case cost before the estimator has been validated on real carriers.
//
// The search is explicitly bounded:
//   - 1 anisotropic scale pair (110%x90%)
//   - 361 angles (-45..+45 at 0.25 degree)
//   - 361 sparse lattice probes maximum
//   - at most 16 candidates receive the stronger periodicity measurement
//   - at most 4 candidates x 3 phases reach full-carrier authenticated decoding
//
// A quick-coherence gate of 0.825 prevents normal image texture from reaching
// the expensive stages in the common case. The v3 frame is accepted only after
// the existing CRC/HMAC checks succeed.
func searchV3DirectLatticeComposition(src *pixelPlane, decoder *decoder) ([]byte, ExtractInfo, latticeCandidate, bool) {
	const (
		quickMinimum      = 0.825
		fullMinimum       = 0.72
		quickShortlistMax = 16
		fullShortlistMax  = 4
	)

	quickCandidates := make([]latticeCandidate, 0, quickShortlistMax)
	considerQuick := func(candidate latticeCandidate) {
		insertAt := len(quickCandidates)
		for i, existing := range quickCandidates {
			if candidate.quick > existing.quick {
				insertAt = i
				break
			}
		}
		if insertAt >= quickShortlistMax {
			return
		}
		quickCandidates = append(quickCandidates, latticeCandidate{})
		copy(quickCandidates[insertAt+1:], quickCandidates[insertAt:])
		quickCandidates[insertAt] = candidate
		if len(quickCandidates) > quickShortlistMax {
			quickCandidates = quickCandidates[:quickShortlistMax]
		}
	}

	scale := directLatticeScale
	for angle := -45.0; angle <= 45.000001; angle += 0.25 {
		rotation := rotationMatrix(angle)
		scaleMatrix := linearTransform{a: scale[0], d: scale[1]}
		matrix := multiplyLinearTransforms(rotation, scaleMatrix)
		quick, _ := quickLatticeCoherence(src, matrix, blockSize)
		if quick < quickMinimum {
			continue
		}
		considerQuick(latticeCandidate{
			angle: angle, scaleX: scale[0], scaleY: scale[1], matrix: matrix, quick: quick,
		})
	}
	if len(quickCandidates) == 0 {
		return nil, ExtractInfo{}, latticeCandidate{}, false
	}

	for i := range quickCandidates {
		entry := &quickCandidates[i]
		probe := probeAffineMatrixDetailed(src, entry.matrix, blockSize)
		candidate := affineCandidate{
			kind: affineScaleXY, parameter: entry.scaleX, parameter2: entry.scaleY,
			matrix: entry.matrix, blockSize: blockSize,
		}
		entry.coherence, entry.phases = bestAffineCoherence(src, candidate, probe)
		entry.contrast = probe.contrast
	}
	sort.Slice(quickCandidates, func(i, j int) bool {
		if quickCandidates[i].coherence == quickCandidates[j].coherence {
			return quickCandidates[i].contrast > quickCandidates[j].contrast
		}
		return quickCandidates[i].coherence > quickCandidates[j].coherence
	})
	if len(quickCandidates) > 0 {
		best := quickCandidates[0].coherence
		second := 0.0
		if len(quickCandidates) > 1 {
			second = quickCandidates[1].coherence
		}
		// A very high coherence, or a clearly isolated high-coherence peak, is
		// strong evidence that this narrow composed geometry has actually been
		// identified. This distinction matters for wrong-key fast failure: pure
		// rotation and rotate+resize can produce misleading moderate peaks, but
		// they typically do not produce the same isolated lattice maximum.
		quickCandidates[0].decisive = best >= 0.85 || (best >= 0.75 && (len(quickCandidates) == 1 || best-second >= 0.12))
	}
	if len(quickCandidates) > fullShortlistMax {
		quickCandidates = quickCandidates[:fullShortlistMax]
	}
	bestEvidence := latticeCandidate{}
	if len(quickCandidates) > 0 {
		bestEvidence = quickCandidates[0]
	}

	for _, candidate := range quickCandidates {
		if candidate.coherence < fullMinimum {
			continue
		}
		for _, phase := range candidate.phases {
			grid, ok := aggregateAffineGrid(src, candidate.matrix, blockSize, phase.x, phase.y)
			if !ok {
				continue
			}
			payload, info, _, found := decoder.decodeGrid(grid)
			if found {
				return payload, info, candidate, true
			}
		}
	}
	return nil, ExtractInfo{}, bestEvidence, false
}
