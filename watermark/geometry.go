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
	return alignedRepetitionCoherence(src, tileWidth, tileHeight)
}

// canMeasureAlignedRepetition reports whether the carrier contains at least two
// complete logical periods along either axis for the requested block-period
// geometry. Small carriers can contain a decodable tile without containing two
// full repeats; in that case repetition coherence is unavailable rather than
// evidence against the candidate orientation.
func canMeasureAlignedRepetition(src *pixelPlane, repeatWidth, repeatHeight int) bool {
	blocksWide := src.bounds.Dx() / blockSize
	blocksHigh := src.bounds.Dy() / blockSize
	return (blocksWide >= 2*repeatWidth && blocksHigh >= repeatHeight) ||
		(blocksWide >= repeatWidth && blocksHigh >= 2*repeatHeight)
}

// alignedRepetitionCoherence measures tile repetition directly on an aligned
// integer DCT lattice without the affine virtual sampler. repeatWidth and
// repeatHeight are expressed in DCT blocks. Using 32x35 instead of the native
// 35x32 detects exact 90/270-degree quarter turns cheaply and prevents the
// expensive quarter-turn decoder from firing on unrelated high sync scores.
func alignedRepetitionCoherence(src *pixelPlane, repeatWidth, repeatHeight int) float64 {
	size := blockSize
	width, height := src.bounds.Dx(), src.bounds.Dy()
	best := 0.0
	for phaseY := 0; phaseY < size; phaseY++ {
		for phaseX := 0; phaseX < size; phaseX++ {
			blocksWide := (width - phaseX) / size
			blocksHigh := (height - phaseY) / size
			if blocksWide < repeatWidth || blocksHigh < repeatHeight {
				continue
			}
			same, total := 0, 0
			compare := func(originAX, originAY, originBX, originBY int) {
				for logical := 0; logical < repeatWidth*repeatHeight; logical += 11 {
					x := logical % repeatWidth
					y := logical / repeatWidth
					ax := phaseX + (originAX+x)*size
					ay := phaseY + (originAY+y)*size
					bx := phaseX + (originBX+x)*size
					by := phaseY + (originBY+y)*size
					if ax < 0 || ay < 0 || bx < 0 || by < 0 ||
						ax+size > width || ay+size > height || bx+size > width || by+size > height {
						continue
					}
					valueA := readBlockSized(src, point{x: ax, y: ay}, size)
					valueB := readBlockSized(src, point{x: bx, y: by}, size)
					total++
					if (valueA >= 0) == (valueB >= 0) {
						same++
					}
				}
			}
			if blocksWide >= 2*repeatWidth {
				startX := (blocksWide - 2*repeatWidth) / 2
				startY := (blocksHigh - repeatHeight) / 2
				compare(startX, startY, startX+repeatWidth, startY)
			}
			if blocksHigh >= 2*repeatHeight {
				startX := (blocksWide - repeatWidth) / 2
				startY := (blocksHigh - 2*repeatHeight) / 2
				compare(startX, startY, startX, startY+repeatHeight)
			}
			if total == 0 {
				continue
			}
			score := float64(same) / float64(total)
			if score > best {
				best = score
			}
		}
	}
	return best
}

func quarterTurnRepetitionCoherence(src *pixelPlane) float64 {
	return alignedRepetitionCoherence(src, tileHeight, tileWidth)
}

// searchV3AxisAlignedAffine performs authenticated decoding directly through a
// virtual affine sampler. No corrected full-resolution image is materialized.
// Repetition coherence gates the expensive path and ranks matrices without using
// the key. Authentication remains the sole acceptance criterion.

// isotropicScaleCandidate describes a pure, axis-aligned resize hypothesis. It
// uses the same virtual affine sampler as the affine decoder, so no normalized
// full-resolution bitmap needs to be materialized merely to test a scale.
type isotropicScaleCandidate struct {
	percent   int
	scale     float64
	matrix    linearTransform
	coherence float64
	contrast  float64
	phases    []point
	score     int
	decisive  bool
}

// searchV3IsotropicScale restores the historical fractional-resize baseline
// before more speculative rotation/lattice heuristics. Candidate scales are the
// same fixed percentages used by the original inverse-normalization path.
//
// The search is bounded:
//   - 13 fixed isotropic scale hypotheses;
//   - at most three phase probes per hypothesis;
//   - at most six shortlisted scale hypotheses, each with up to three phases (18 full-carrier virtual aggregations).
//
// Repetition coherence ranks candidates without the key. Authentication remains
// the sole success criterion. This path intentionally excludes 100/75/50%,
// which are already covered by direct integer block sizes 8/6/4.
func searchV3IsotropicScale(src *pixelPlane, decoder *decoder) ([]byte, ExtractInfo, isotropicScaleCandidate, bool) {
	const fullShortlistMax = 6

	if nativeRepetitionCoherence(src) >= 0.82 {
		return nil, ExtractInfo{}, isotropicScaleCandidate{}, false
	}

	candidates := make([]isotropicScaleCandidate, 0, len(normalizedScales))
	for _, percent := range normalizedScales {
		scale := float64(percent) / 100.0
		candidate := affineCandidate{
			kind:       affineScaleXY,
			parameter:  scale,
			parameter2: scale,
			matrix:     linearTransform{a: scale, d: scale},
			blockSize:  blockSize,
		}
		probe := probeAffineMatrixDetailed(src, candidate.matrix, blockSize)
		coherence, phases := bestAffineCoherence(src, candidate, probe)
		if len(phases) == 0 {
			continue
		}
		entry := isotropicScaleCandidate{
			percent:   percent,
			scale:     scale,
			matrix:    candidate.matrix,
			coherence: coherence,
			contrast:  probe.contrast,
			phases:    phases,
		}

		// A single-tile authenticated hit is a cheap positive fast path. Even when
		// it does not authenticate, preserve the best sync score as a secondary
		// ranking signal for phase-sensitive carriers.
		for _, phase := range phases {
			grid, ok := aggregateAffineTile(src, candidate.matrix, blockSize, phase)
			if !ok {
				continue
			}
			payload, info, score, found := decoder.decodeGrid(grid)
			if score > entry.score {
				entry.score = score
			}
			if found {
				return payload, info, entry, true
			}
		}
		candidates = append(candidates, entry)
	}

	if len(candidates) == 0 {
		return nil, ExtractInfo{}, isotropicScaleCandidate{}, false
	}

	// Repetition is the primary geometry signal; authenticated-header sync score
	// breaks ties and helps when resampling weakens periodicity.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].coherence == candidates[j].coherence {
			if candidates[i].score == candidates[j].score {
				return candidates[i].contrast > candidates[j].contrast
			}
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].coherence > candidates[j].coherence
	})

	bestEvidence := candidates[0]
	second := 0.0
	if len(candidates) > 1 {
		second = candidates[1].coherence
	}
	bestEvidence.decisive = bestEvidence.coherence >= 0.88 ||
		(bestEvidence.coherence >= 0.78 && bestEvidence.score >= 800) ||
		(bestEvidence.coherence >= 0.74 && bestEvidence.coherence-second >= 0.08)

	if len(candidates) > fullShortlistMax {
		candidates = candidates[:fullShortlistMax]
	}
	for i := range candidates {
		entry := &candidates[i]
		for _, phase := range entry.phases {
			grid, ok := aggregateAffineGrid(src, entry.matrix, blockSize, phase.x, phase.y)
			if !ok {
				continue
			}
			payload, info, score, found := decoder.decodeGrid(grid)
			if score > entry.score {
				entry.score = score
			}
			if found {
				return payload, info, *entry, true
			}
		}
	}

	// If virtual sampling is just short of authentication (notably capacity at
	// aggressive fractional scales), the authenticated-header sync score is a
	// better pointer to the historical physical-normalization fallback than raw
	// repetition alone. Random/unmarked candidates remain far below this range.
	for _, entry := range candidates {
		if entry.score > bestEvidence.score {
			bestEvidence = entry
		}
	}
	bestEvidence.decisive = bestEvidence.score >= 900 ||
		bestEvidence.coherence >= 0.88 ||
		(bestEvidence.coherence >= 0.78 && bestEvidence.score >= 800)
	return nil, ExtractInfo{}, bestEvidence, false
}

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

// latticeBasisShape describes an orientation-neutral pair of transformed lattice
// basis vectors. Build 9 keeps the direct geometric representation introduced in
// build 8 and expands the validated bank to two anisotropy magnitudes in both
// orientations: +/-5% and +/-10%. The representation is geometric rather than
// edit-order based: each hypothesis is a pair of basis vectors which is then
// rotated as a unit.
type latticeBasisShape struct {
	name   string
	basisU [2]float64
	basisV [2]float64
	scaleX float64
	scaleY float64
}

var latticeBasisShapes = [...]latticeBasisShape{
	{
		name:   "anisotropic-110x90",
		basisU: [2]float64{1.10, 0},
		basisV: [2]float64{0, 0.90},
		scaleX: 1.10,
		scaleY: 0.90,
	},
	{
		name:   "anisotropic-90x110",
		basisU: [2]float64{0.90, 0},
		basisV: [2]float64{0, 1.10},
		scaleX: 0.90,
		scaleY: 1.10,
	},
	{
		name:   "anisotropic-105x95",
		basisU: [2]float64{1.05, 0},
		basisV: [2]float64{0, 0.95},
		scaleX: 1.05,
		scaleY: 0.95,
	},
	{
		name:   "anisotropic-95x105",
		basisU: [2]float64{0.95, 0},
		basisV: [2]float64{0, 1.05},
		scaleX: 0.95,
		scaleY: 1.05,
	},
}

// latticeCandidate is a concrete transformed-basis hypothesis. matrix columns
// are the observed horizontal/vertical basis vectors per rectified source pixel.
type latticeCandidate struct {
	shapeName string
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

func rotateBasisVector(vector [2]float64, degrees float64) [2]float64 {
	radians := degrees * math.Pi / 180
	cosine, sine := math.Cos(radians), math.Sin(radians)
	return [2]float64{
		cosine*vector[0] - sine*vector[1],
		sine*vector[0] + cosine*vector[1],
	}
}

func latticeBasisMatrix(shape latticeBasisShape, angle float64) linearTransform {
	u := rotateBasisVector(shape.basisU, angle)
	v := rotateBasisVector(shape.basisV, angle)
	return linearTransform{a: u[0], b: v[0], c: u[1], d: v[1]}
}

// quickLatticeCoherence is the inexpensive first stage of direct lattice
// estimation. It samples a deterministic sparse subset of v3 tile positions
// and measures sign agreement between adjacent repeated tiles through the
// candidate transform. The score is key-independent and is used only for
// ranking; HMAC remains the sole acceptance criterion.
//
// Pixel phase is searched coarsely on the even 4x4 grid and then refined in a
// 3x3 neighbourhood. With 20 logical samples this rejects ordinary image
// texture far more reliably than orientation contrast alone while keeping the
// cost independent of image dimensions.
func quickLatticeCoherenceSamples(src *pixelPlane, matrix linearTransform, size, sampleCount int) (float64, point) {
	bounds, ok := affineOutputBounds(src, matrix)
	if !ok {
		return 0, point{}
	}
	blocksWide := bounds.width / size
	blocksHigh := bounds.height / size
	if blocksWide < tileWidth || blocksHigh < tileHeight {
		return 0, point{}
	}

	logicalPositions := make([]int, sampleCount)
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

func quickLatticeCoherence(src *pixelPlane, matrix linearTransform, size int) (float64, point) {
	return quickLatticeCoherenceSamples(src, matrix, size, 20)
}

// searchV3DirectLatticeBasis searches the small bank of transformed lattice-basis
// shapes directly. Unlike the build-6 composition path, angle selection is not
// inherited from the standalone rotation detector. Unlike an unrestricted
// affine brute force, the shape bank is fixed and intentionally small.
//
// Build 9's validated bank contains four anisotropic basis shapes:
//   - 110%x90%
//   - 90%x110%
//   - 105%x95%
//   - 95%x105%
//
// each followed by arbitrary rotation.
//
// The search is explicitly bounded:
//   - 4 basis shapes
//   - 361 angles per shape (-45..+45 at 0.25 degree)
//   - 1444 sparse lattice probes maximum
//   - at most 48 candidates per shape (192 total) receive stronger periodicity measurement
//   - at most 4 candidates x 3 phases reach full-carrier authenticated decoding per shape group
//   - the +/-10% and +/-5% groups are sequential, so the overall worst case is 24 full grids
func searchV3DirectLatticeBasis(src *pixelPlane, decoder *decoder) ([]byte, ExtractInfo, latticeCandidate, bool) {
	const (
		quickFloor           = 0.70
		moderateQuickFloor   = 0.58
		perShapeQuickMax     = 48
		moderateQuickSamples = 60
		fullMinimum          = 0.72
		fullShortlistMax     = 4
	)

	// Keep a bounded shortlist for every basis shape instead of relying on one
	// global high quick-score threshold. Real carriers showed that pixel-phase
	// interactions can depress the 20-sample quick score even when the stronger
	// repetition score at the correct matrix is excellent (notably for negative
	// rotations). All four shapes keep at most 48 candidates. The moderate shapes use a richer
	// 60-position sparse probe so real phase-sensitive cases stay inside that
	// shortlist without expanding the expensive stronger stage beyond 192 matrices.
	shortlistForShape := func(shape latticeBasisShape) []latticeCandidate {
		maxCandidates := perShapeQuickMax
		floor := quickFloor
		quickSamples := 20
		if math.Abs(shape.scaleX-shape.scaleY) < 0.11 {
			// The 105x95/95x105 real-corpus cases are more sensitive to pixel
			// phase. A denser 60-position sparse probe ranks the true matrix
			// reliably enough to keep the same 48-candidate stronger-stage cap.
			quickSamples = moderateQuickSamples
			floor = moderateQuickFloor
		}
		candidates := make([]latticeCandidate, 0, maxCandidates)
		consider := func(candidate latticeCandidate) {
			insertAt := len(candidates)
			for i, existing := range candidates {
				if candidate.quick > existing.quick {
					insertAt = i
					break
				}
			}
			if insertAt >= maxCandidates {
				return
			}
			candidates = append(candidates, latticeCandidate{})
			copy(candidates[insertAt+1:], candidates[insertAt:])
			candidates[insertAt] = candidate
			if len(candidates) > maxCandidates {
				candidates = candidates[:maxCandidates]
			}
		}

		for angle := -45.0; angle <= 45.000001; angle += 0.25 {
			matrix := latticeBasisMatrix(shape, angle)
			quick, _ := quickLatticeCoherenceSamples(src, matrix, blockSize, quickSamples)
			if quick < floor {
				continue
			}
			consider(latticeCandidate{
				shapeName: shape.name,
				angle:     angle,
				scaleX:    shape.scaleX,
				scaleY:    shape.scaleY,
				matrix:    matrix,
				quick:     quick,
			})
		}
		return candidates
	}

	// Evaluate one bounded shape group completely. Build 9 deliberately probes the
	// original +/-10% anchor pair first; successful anchor carriers therefore keep
	// approximately the build-8 positive-path cost. Only if authentication fails
	// do the newer +/-5% shapes enter the stronger periodicity stage.
	evaluateGroup := func(shapes []latticeBasisShape) ([]byte, ExtractInfo, latticeCandidate, bool) {
		quickCandidates := make([]latticeCandidate, 0)
		for _, shape := range shapes {
			quickCandidates = append(quickCandidates, shortlistForShape(shape)...)
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
				if quickCandidates[i].quick == quickCandidates[j].quick {
					return quickCandidates[i].contrast > quickCandidates[j].contrast
				}
				return quickCandidates[i].quick > quickCandidates[j].quick
			}
			return quickCandidates[i].coherence > quickCandidates[j].coherence
		})

		bestEvidence := quickCandidates[0]
		best := bestEvidence.coherence
		second := 0.0
		if len(quickCandidates) > 1 {
			second = quickCandidates[1].coherence
		}
		// Require both a strong repetition score and either a strong sparse score
		// or isolation from the next matrix. This keeps wrong-key fast failure on
		// identified composed geometry without letting ordinary texture or a pure
		// rotation trigger a premature exit.
		bestEvidence.decisive = best >= 0.88 ||
			(best >= 0.78 && bestEvidence.quick >= 0.80) ||
			(best >= 0.76 && best-second >= 0.10)

		fullCandidates := quickCandidates
		if len(fullCandidates) > fullShortlistMax {
			fullCandidates = fullCandidates[:fullShortlistMax]
		}
		for _, candidate := range fullCandidates {
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

	betterEvidence := func(a, b latticeCandidate) latticeCandidate {
		if b.coherence > a.coherence ||
			(b.coherence == a.coherence && b.quick > a.quick) ||
			(b.coherence == a.coherence && b.quick == a.quick && b.contrast > a.contrast) {
			return b
		}
		return a
	}

	anchorShapes := latticeBasisShapes[:2]
	payload, info, anchorEvidence, ok := evaluateGroup(anchorShapes)
	if ok {
		return payload, info, anchorEvidence, true
	}

	moderateShapes := latticeBasisShapes[2:]
	payload, info, moderateEvidence, ok := evaluateGroup(moderateShapes)
	if ok {
		return payload, info, moderateEvidence, true
	}
	return nil, ExtractInfo{}, betterEvidence(anchorEvidence, moderateEvidence), false
}

// projectiveCandidate describes a small, bounded projective warp hypothesis.
// Build 11 deliberately starts with two mild vertical-keystone shapes as a research
// bridge toward general homography estimation; Format v3 is unchanged.
type projectiveCandidate struct {
	name string
	quad [4][2]float64 // TL, TR, BL, BR in normalized observed coordinates
}

var projectiveHypotheses = [...]projectiveCandidate{
	{name: "top-narrow-4", quad: [4][2]float64{{.04, 0}, {.96, 0}, {0, 1}, {1, 1}}},
	{name: "bottom-narrow-4", quad: [4][2]float64{{0, 0}, {1, 0}, {.04, 1}, {.96, 1}}},
}

type homography struct{ h [9]float64 }

func solveLinear8(a [8][9]float64) ([8]float64, bool) {
	for col := 0; col < 8; col++ {
		pivot := col
		for row := col + 1; row < 8; row++ {
			if math.Abs(a[row][col]) > math.Abs(a[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(a[pivot][col]) < 1e-12 {
			return [8]float64{}, false
		}
		a[col], a[pivot] = a[pivot], a[col]
		v := a[col][col]
		for j := col; j < 9; j++ {
			a[col][j] /= v
		}
		for row := 0; row < 8; row++ {
			if row == col {
				continue
			}
			f := a[row][col]
			for j := col; j < 9; j++ {
				a[row][j] -= f * a[col][j]
			}
		}
	}
	var out [8]float64
	for i := range out {
		out[i] = a[i][8]
	}
	return out, true
}

func homographyForQuad(width, height int, quad [4][2]float64) (homography, bool) {
	if width < 2 || height < 2 {
		return homography{}, false
	}
	src := [4][2]float64{{0, 0}, {float64(width - 1), 0}, {0, float64(height - 1)}, {float64(width - 1), float64(height - 1)}}
	var a [8][9]float64
	for i := 0; i < 4; i++ {
		x, y := src[i][0], src[i][1]
		u, v := quad[i][0]*float64(width-1), quad[i][1]*float64(height-1)
		a[2*i] = [9]float64{x, y, 1, 0, 0, 0, -u * x, -u * y, u}
		a[2*i+1] = [9]float64{0, 0, 0, x, y, 1, -v * x, -v * y, v}
	}
	s, ok := solveLinear8(a)
	if !ok {
		return homography{}, false
	}
	return homography{h: [9]float64{s[0], s[1], s[2], s[3], s[4], s[5], s[6], s[7], 1}}, true
}

func (h homography) mapPoint(x, y float64) (float64, float64, bool) {
	d := h.h[6]*x + h.h[7]*y + h.h[8]
	if math.Abs(d) < 1e-12 {
		return 0, 0, false
	}
	return (h.h[0]*x + h.h[1]*y + h.h[2]) / d, (h.h[3]*x + h.h[4]*y + h.h[5]) / d, true
}

func readProjectiveBlockValue(src *pixelPlane, h homography, originX, originY, size int) (float64, bool) {
	table := readCosTables[size]
	c23, c32 := 0.0, 0.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx, sy, ok := h.mapPoint(float64(originX+x), float64(originY+y))
			if !ok {
				return 0, false
			}
			l, ok := samplePlaneLuminance(src, sx, sy)
			if !ok {
				return 0, false
			}
			c23 += l * table[3][x] * table[2][y]
			c32 += l * table[2][x] * table[3][y]
		}
	}
	return math.Abs(c23) - math.Abs(c32), true
}

func aggregateProjectiveGrid(src *pixelPlane, h homography, size, offsetX, offsetY int) ([]float64, bool) {
	w, hgt := src.bounds.Dx(), src.bounds.Dy()
	bw := (w - offsetX) / size
	bh := (hgt - offsetY) / size
	if bw < tileWidth || bh < tileHeight {
		return nil, false
	}
	grid := make([]float64, eccBits)
	valid := 0
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			v, ok := readProjectiveBlockValue(src, h, offsetX+bx*size, offsetY+by*size, size)
			if !ok {
				continue
			}
			grid[(by%tileHeight)*tileWidth+bx%tileWidth] += v
			valid++
		}
	}
	return grid, valid >= tileWidth*tileHeight
}

func searchV3MildPerspective(src *pixelPlane, decoder *decoder) ([]byte, ExtractInfo, string, bool) {
	// Fixed two-shape bank, each with one aligned phase. This is a
	// deliberately bounded first print-camera experiment, not general perspective.
	phases := [...]point{{0, 0}}
	for _, candidate := range projectiveHypotheses {
		h, ok := homographyForQuad(src.bounds.Dx(), src.bounds.Dy(), candidate.quad)
		if !ok {
			continue
		}
		for _, phase := range phases {
			grid, ok := aggregateProjectiveGrid(src, h, blockSize, phase.x, phase.y)
			if !ok {
				continue
			}
			if payload, info, _, found := decoder.decodeGrid(grid); found {
				return payload, info, candidate.name, true
			}
		}
	}
	return nil, ExtractInfo{}, "", false
}
