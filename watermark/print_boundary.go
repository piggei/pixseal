package watermark

import (
	"image"
	"math"
)

const (
	printBoundaryMaxDimension = 1200
	printBoundarySamples      = 72
)

// ImagePoint is one source-image coordinate reported by the v0.3 research
// diagnostics. Coordinates are expressed in native decoded pixels.
type ImagePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// PrintBoundaryEstimate is an optional geometric initializer. A detected
// quadrilateral is never evidence of a PixSeal watermark; it only describes a
// likely printed-image boundary that can constrain later lattice hypotheses.
type PrintBoundaryEstimate struct {
	Detected        bool       `json:"detected"`
	Confidence      float64    `json:"confidence"`
	TopLeft         ImagePoint `json:"top_left"`
	TopRight        ImagePoint `json:"top_right"`
	BottomRight     ImagePoint `json:"bottom_right"`
	BottomLeft      ImagePoint `json:"bottom_left"`
	TopScore        float64    `json:"top_score"`
	RightScore      float64    `json:"right_score"`
	BottomScore     float64    `json:"bottom_score"`
	LeftScore       float64    `json:"left_score"`
	AnalysisDivisor int        `json:"analysis_divisor"`
}

type boundaryColorPlane struct {
	width, height int
	divisor       int
	rgb           []uint8
}

type boundaryLine struct {
	base, slope float64
	score       float64
	inside      float64
	outside     float64
}

func estimatePrintBoundary(src image.Image) PrintBoundaryEstimate {
	plane := buildBoundaryColorPlane(src, printBoundaryMaxDimension)
	if plane == nil || plane.width < 160 || plane.height < 120 {
		return PrintBoundaryEstimate{}
	}
	paper, ok := estimateBoundaryPaperColor(plane)
	if !ok {
		return PrintBoundaryEstimate{AnalysisDivisor: plane.divisor}
	}

	top := searchBoundaryHorizontal(plane, paper, true)
	bottom := searchBoundaryHorizontal(plane, paper, false)
	left := searchBoundaryVertical(plane, paper, true)
	right := searchBoundaryVertical(plane, paper, false)

	tl, okTL := intersectBoundaryLines(plane, top, left)
	tr, okTR := intersectBoundaryLines(plane, top, right)
	br, okBR := intersectBoundaryLines(plane, bottom, right)
	bl, okBL := intersectBoundaryLines(plane, bottom, left)

	result := PrintBoundaryEstimate{
		TopScore: top.score, RightScore: right.score, BottomScore: bottom.score, LeftScore: left.score,
		AnalysisDivisor: plane.divisor,
	}
	if !(okTL && okTR && okBR && okBL) {
		return result
	}

	divisor := float64(plane.divisor)
	result.TopLeft = ImagePoint{X: tl.X * divisor, Y: tl.Y * divisor}
	result.TopRight = ImagePoint{X: tr.X * divisor, Y: tr.Y * divisor}
	result.BottomRight = ImagePoint{X: br.X * divisor, Y: br.Y * divisor}
	result.BottomLeft = ImagePoint{X: bl.X * divisor, Y: bl.Y * divisor}

	area := math.Abs(polygonArea4(tl, tr, br, bl))
	areaFraction := area / float64(plane.width*plane.height)
	strongSides := 0
	strength := 0.0
	for _, score := range [...]float64{top.score, right.score, bottom.score, left.score} {
		if score > 5 {
			strongSides++
		}
		strength += clampUnit((score + 5) / 55)
	}
	strength /= 4
	geometry := clampUnit((areaFraction - 0.30) / 0.45)
	result.Confidence = clampUnit(0.70*strength + 0.30*geometry)
	result.Detected = strongSides >= 3 && top.score > -8 && right.score > -8 && bottom.score > -8 && left.score > -8 &&
		areaFraction >= 0.35 && boundaryQuadSane(plane, tl, tr, br, bl)
	if !result.Detected {
		result.Confidence *= 0.5
	}
	return result
}

func buildBoundaryColorPlane(src image.Image, maxDimension int) *boundaryColorPlane {
	if src == nil {
		return nil
	}
	bounds := src.Bounds()
	largest := bounds.Dx()
	if bounds.Dy() > largest {
		largest = bounds.Dy()
	}
	divisor := 1
	for (largest+divisor-1)/divisor > maxDimension {
		divisor *= 2
	}
	width := (bounds.Dx() + divisor - 1) / divisor
	height := (bounds.Dy() + divisor - 1) / divisor
	plane := &boundaryColorPlane{width: width, height: height, divisor: divisor, rgb: make([]uint8, width*height*3)}
	for y := 0; y < height; y++ {
		sy := y*divisor + divisor/2
		if sy >= bounds.Dy() {
			sy = bounds.Dy() - 1
		}
		for x := 0; x < width; x++ {
			sx := x*divisor + divisor/2
			if sx >= bounds.Dx() {
				sx = bounds.Dx() - 1
			}
			pixel := flattenedNRGBA(src.At(bounds.Min.X+sx, bounds.Min.Y+sy))
			index := (y*width + x) * 3
			plane.rgb[index], plane.rgb[index+1], plane.rgb[index+2] = pixel.R, pixel.G, pixel.B
		}
	}
	return plane
}

func estimateBoundaryPaperColor(plane *boundaryColorPlane) ([3]float64, bool) {
	var sum [3]float64
	count := 0
	fallback := [3]float64{}
	fallbackCount := 0
	step := 2
	for y := 0; y < plane.height; y += step {
		for x := 0; x < plane.width; x += step {
			if x > plane.width*18/100 && x < plane.width*82/100 && y > plane.height*18/100 && y < plane.height*82/100 {
				continue
			}
			r, g, b := plane.colorAt(x, y)
			fallback[0] += r
			fallback[1] += g
			fallback[2] += b
			fallbackCount++
			maximum := math.Max(r, math.Max(g, b))
			minimum := math.Min(r, math.Min(g, b))
			mean := (r + g + b) / 3
			if mean < 170 || maximum-minimum > 55 {
				continue
			}
			sum[0] += r
			sum[1] += g
			sum[2] += b
			count++
		}
	}
	if count >= 100 {
		return [3]float64{sum[0] / float64(count), sum[1] / float64(count), sum[2] / float64(count)}, true
	}
	if fallbackCount == 0 {
		return [3]float64{}, false
	}
	return [3]float64{fallback[0] / float64(fallbackCount), fallback[1] / float64(fallbackCount), fallback[2] / float64(fallbackCount)}, true
}

func (plane *boundaryColorPlane) colorAt(x, y int) (float64, float64, float64) {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= plane.width {
		x = plane.width - 1
	}
	if y >= plane.height {
		y = plane.height - 1
	}
	index := (y*plane.width + x) * 3
	return float64(plane.rgb[index]), float64(plane.rgb[index+1]), float64(plane.rgb[index+2])
}

func boundaryColorDistance(plane *boundaryColorPlane, paper [3]float64, x, y float64) float64 {
	r, g, b := plane.colorAt(int(math.Round(x)), int(math.Round(y)))
	dr, dg, db := r-paper[0], g-paper[1], b-paper[2]
	distance := math.Sqrt(dr*dr + dg*dg + db*db)
	if distance > 100 {
		return 100
	}
	return distance
}

func searchBoundaryHorizontal(plane *boundaryColorPlane, paper [3]float64, top bool) boundaryLine {
	centerX := float64(plane.width-1) / 2
	offset := math.Max(3, float64(plane.height)/150)
	start, end := 0.015*float64(plane.height), 0.26*float64(plane.height)
	if !top {
		start, end = 0.74*float64(plane.height), 0.985*float64(plane.height)
	}
	best := boundaryLine{score: math.Inf(-1)}
	for base := start; base <= end; base += 2 {
		for slope := -0.15; slope <= 0.150001; slope += 0.01 {
			inside, outside := 0.0, 0.0
			valid := 0
			for sample := 0; sample < printBoundarySamples; sample++ {
				x := 0.05*float64(plane.width-1) + 0.90*float64(plane.width-1)*float64(sample)/float64(printBoundarySamples-1)
				y := base + slope*(x-centerX)
				if y < 2*offset || y > float64(plane.height-1)-2*offset {
					continue
				}
				yInside, yOutside := y+offset, y-offset
				if !top {
					yInside, yOutside = y-offset, y+offset
				}
				inside += boundaryColorDistance(plane, paper, x, yInside)
				outside += boundaryColorDistance(plane, paper, x, yOutside)
				valid++
			}
			if valid < printBoundarySamples*3/4 {
				continue
			}
			inside /= float64(valid)
			outside /= float64(valid)
			score := 1.25*inside - 1.75*outside
			if score > best.score {
				best = boundaryLine{base: base, slope: slope, score: score, inside: inside, outside: outside}
			}
		}
	}
	return best
}

func searchBoundaryVertical(plane *boundaryColorPlane, paper [3]float64, left bool) boundaryLine {
	centerY := float64(plane.height-1) / 2
	offset := math.Max(3, float64(plane.width)/200)
	start, end := 0.015*float64(plane.width), 0.26*float64(plane.width)
	if !left {
		start, end = 0.74*float64(plane.width), 0.985*float64(plane.width)
	}
	best := boundaryLine{score: math.Inf(-1)}
	for base := start; base <= end; base += 2 {
		for slope := -0.20; slope <= 0.200001; slope += 0.01 {
			inside, outside := 0.0, 0.0
			valid := 0
			for sample := 0; sample < printBoundarySamples; sample++ {
				y := 0.05*float64(plane.height-1) + 0.90*float64(plane.height-1)*float64(sample)/float64(printBoundarySamples-1)
				x := base + slope*(y-centerY)
				if x < 2*offset || x > float64(plane.width-1)-2*offset {
					continue
				}
				xInside, xOutside := x+offset, x-offset
				if !left {
					xInside, xOutside = x-offset, x+offset
				}
				inside += boundaryColorDistance(plane, paper, xInside, y)
				outside += boundaryColorDistance(plane, paper, xOutside, y)
				valid++
			}
			if valid < printBoundarySamples*3/4 {
				continue
			}
			inside /= float64(valid)
			outside /= float64(valid)
			score := 1.25*inside - 1.75*outside
			if score > best.score {
				best = boundaryLine{base: base, slope: slope, score: score, inside: inside, outside: outside}
			}
		}
	}
	return best
}

func intersectBoundaryLines(plane *boundaryColorPlane, horizontal, vertical boundaryLine) (ImagePoint, bool) {
	if math.IsInf(horizontal.score, 0) || math.IsInf(vertical.score, 0) {
		return ImagePoint{}, false
	}
	centerX := float64(plane.width-1) / 2
	centerY := float64(plane.height-1) / 2
	denominator := 1 - vertical.slope*horizontal.slope
	if math.Abs(denominator) < 1e-6 {
		return ImagePoint{}, false
	}
	x := (vertical.base + vertical.slope*(horizontal.base-horizontal.slope*centerX-centerY)) / denominator
	y := horizontal.base + horizontal.slope*(x-centerX)
	if x < -0.05*float64(plane.width) || x > 1.05*float64(plane.width) || y < -0.05*float64(plane.height) || y > 1.05*float64(plane.height) {
		return ImagePoint{}, false
	}
	return ImagePoint{X: x, Y: y}, true
}

func polygonArea4(a, b, c, d ImagePoint) float64 {
	points := [...]ImagePoint{a, b, c, d}
	area := 0.0
	for i := range points {
		j := (i + 1) % len(points)
		area += points[i].X*points[j].Y - points[j].X*points[i].Y
	}
	return area / 2
}

func boundaryQuadSane(plane *boundaryColorPlane, tl, tr, br, bl ImagePoint) bool {
	minWidth := 0.45 * float64(plane.width)
	minHeight := 0.45 * float64(plane.height)
	top := math.Hypot(tr.X-tl.X, tr.Y-tl.Y)
	bottom := math.Hypot(br.X-bl.X, br.Y-bl.Y)
	left := math.Hypot(bl.X-tl.X, bl.Y-tl.Y)
	right := math.Hypot(br.X-tr.X, br.Y-tr.Y)
	return top >= minWidth && bottom >= minWidth && left >= minHeight && right >= minHeight && polygonArea4(tl, tr, br, bl) > 0
}

func boundaryExpectedTangents(boundary PrintBoundaryEstimate, regionX, regionY, regionsX, regionsY int) (LatticeVector, LatticeVector, bool) {
	if !boundary.Detected || regionsX <= 0 || regionsY <= 0 {
		return LatticeVector{}, LatticeVector{}, false
	}
	fx := (float64(regionX) + 0.5) / float64(regionsX)
	fy := (float64(regionY) + 0.5) / float64(regionsY)
	top := LatticeVector{X: boundary.TopRight.X - boundary.TopLeft.X, Y: boundary.TopRight.Y - boundary.TopLeft.Y}
	bottom := LatticeVector{X: boundary.BottomRight.X - boundary.BottomLeft.X, Y: boundary.BottomRight.Y - boundary.BottomLeft.Y}
	left := LatticeVector{X: boundary.BottomLeft.X - boundary.TopLeft.X, Y: boundary.BottomLeft.Y - boundary.TopLeft.Y}
	right := LatticeVector{X: boundary.BottomRight.X - boundary.TopRight.X, Y: boundary.BottomRight.Y - boundary.TopRight.Y}
	u := LatticeVector{X: top.X*(1-fy) + bottom.X*fy, Y: top.Y*(1-fy) + bottom.Y*fy}
	v := LatticeVector{X: left.X*(1-fx) + right.X*fx, Y: left.Y*(1-fx) + right.Y*fx}
	if math.Hypot(u.X, u.Y) < 1e-9 || math.Hypot(v.X, v.Y) < 1e-9 {
		return LatticeVector{}, LatticeVector{}, false
	}
	return u, v, true
}

func boundaryExpectedBasis(boundary PrintBoundaryEstimate, regionX, regionY, regionsX, regionsY int) (LatticeVector, LatticeVector, bool) {
	u, v, ok := boundaryExpectedTangents(boundary, regionX, regionY, regionsX, regionsY)
	if !ok {
		return LatticeVector{}, LatticeVector{}, false
	}
	ul, vl := math.Hypot(u.X, u.Y), math.Hypot(v.X, v.Y)
	u.X, u.Y = u.X/ul, u.Y/ul
	v.X, v.Y = v.X/vl, v.Y/vl
	return u, v, true
}
