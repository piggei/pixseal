package watermark

import "math"

const (
	diagnosticBlindLatticeMinRegionConfidence          = 0.12
	diagnosticBlindLatticeConsensusMinConfidence       = 0.16
	diagnosticBlindLatticeConsensusMaxFractionDistance = 0.42
	diagnosticBlindLatticeCycleMaxPrimaryConfidence    = 0.24
	diagnosticBlindLatticeCycleMinConfidence           = 0.22
	diagnosticBlindLatticeCycleMinResidualGain         = 0.45
	diagnosticBlindLatticeCycleMaxResidual             = 0.55
	diagnosticBlindLatticeUnwrapIterations             = 2
)

// diagnosticEstimateLocalLatticeFractionalPhase is the build14 secondary blind
// observer. It reuses the already-selected 3x3 LocalLatticeEstimate records,
// projects one measured lattice intersection per region back into the current
// candidate's canonical plane, and reduces the residual modulo the 8-pixel v3
// block grid. No key, expected bit, payload byte, or HMAC result participates.
//
// The result contains only fractional-block phase. Integer cycle choice remains
// anchored by the build12 repetition observer and may move by at most one cycle
// under the separate smooth-unwrapping gate in diagnosticFuseBlindLatticePhase.
func diagnosticEstimateLocalLatticeFractionalPhase(candidate DiagnosticScaleCandidate, mapper diagnosticProjectiveMapper, cells []diagnosticSpatialGridCell, regions []LocalLatticeEstimate) (diagnosticBlindPhaseResult, bool) {
	result := diagnosticBlindPhaseResult{method: "local-lattice-fractional-phase"}
	if candidate.CanonicalWidthPixels <= 1 || candidate.CanonicalHeightPixels <= 1 || len(cells) < 3 || len(regions) == 0 {
		return result, false
	}
	result.controls = make([]diagnosticBlindCellControl, len(cells))
	regionByCell := make(map[[2]int]LocalLatticeEstimate, len(regions))
	for _, region := range regions {
		key := [2]int{region.RegionX, region.RegionY}
		if existing, ok := regionByCell[key]; !ok || region.Confidence > existing.Confidence {
			regionByCell[key] = region
		}
	}

	available := 0
	result.minConfidence = 1
	for i, cell := range cells {
		region, ok := regionByCell[[2]int{cell.RegionX, cell.RegionY}]
		if !ok || region.Confidence < diagnosticBlindLatticeMinRegionConfidence {
			continue
		}
		sx, sy, ok := diagnosticNearestLocalLatticePoint(region)
		if !ok {
			continue
		}
		cx, cy, ok := diagnosticApproxInverseProjectiveMapper(mapper, sx, sy)
		if !ok {
			continue
		}
		marginX := 0.08 * candidate.CanonicalWidthPixels
		marginY := 0.08 * candidate.CanonicalHeightPixels
		if cx < -marginX || cy < -marginY || cx > candidate.CanonicalWidthPixels+marginX || cy > candidate.CanonicalHeightPixels+marginY {
			continue
		}

		// Report the observed canonical lattice drift with the same sign used by
		// local phase observations. The smooth-phase fitter later applies the
		// inverse correction, so a +fractional block drift becomes a -fractional
		// block sampler correction.
		fracX := diagnosticWrapHalf(cx / float64(blockSize))
		fracY := diagnosticWrapHalf(cy / float64(blockSize))
		structure := 0.5*diagnosticClamp01(region.PeriodicCoherence) + 0.5*diagnosticClamp01(region.TileRepetitionCoherence)
		confidence := diagnosticClamp01(0.60*region.Confidence + 0.40*structure)
		result.controls[i] = diagnosticBlindCellControl{
			available:  true,
			offsetX:    fracX,
			offsetY:    fracY,
			confidence: confidence,
			pairs:      1,
		}
		available++
		result.meanConfidence += confidence
		if confidence < result.minConfidence {
			result.minConfidence = confidence
		}
	}
	if available < 3 {
		return diagnosticBlindPhaseResult{}, false
	}
	result.meanConfidence /= float64(available)
	diagnosticCircularRecenterBlindFractionalControls(&result)
	return result, true
}

func diagnosticApproxInverseProjectiveMapper(mapper diagnosticProjectiveMapper, sx, sy float64) (float64, float64, bool) {
	inverse, ok := invertDiagnosticHomography(mapper.h)
	if !ok {
		return 0, 0, false
	}
	zx, zy, ok := inverse.mapPoint(sx, sy)
	if !ok {
		return 0, 0, false
	}
	x := zx - mapper.canonicalOffsetX
	y := zy - mapper.canonicalOffsetY
	for iteration := 0; iteration < 4; iteration++ {
		dx, dy := 0.0, 0.0
		if mapper.warp != nil {
			wx, wy := mapper.warp.correction(x, y)
			dx += wx
			dy += wy
		}
		if mapper.smoothPhaseWarp != nil {
			wx, wy := mapper.smoothPhaseWarp.correction(x, y)
			dx += wx
			dy += wy
		}
		x = zx - mapper.canonicalOffsetX - dx
		y = zy - mapper.canonicalOffsetY - dy
	}
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return 0, 0, false
	}
	return x, y, true
}

func diagnosticWrapHalf(value float64) float64 {
	value -= math.Round(value)
	if value >= 0.5 {
		value -= 1
	}
	if value < -0.5 {
		value += 1
	}
	return value
}

func diagnosticCircularRecenterBlindFractionalControls(result *diagnosticBlindPhaseResult) {
	if result == nil {
		return
	}
	meanAxis := func(axisX bool) float64 {
		sinSum, cosSum := 0.0, 0.0
		for _, control := range result.controls {
			if !control.available {
				continue
			}
			value := control.offsetY
			if axisX {
				value = control.offsetX
			}
			w := math.Max(control.confidence, 0.02)
			angle := 2 * math.Pi * value
			sinSum += w * math.Sin(angle)
			cosSum += w * math.Cos(angle)
		}
		if sinSum == 0 && cosSum == 0 {
			return 0
		}
		return math.Atan2(sinSum, cosSum) / (2 * math.Pi)
	}
	meanX, meanY := meanAxis(true), meanAxis(false)
	result.meanOffset = 0
	result.maxOffset = 0
	available := 0
	for i := range result.controls {
		control := &result.controls[i]
		if !control.available {
			continue
		}
		control.offsetX = diagnosticWrapHalf(control.offsetX - meanX)
		control.offsetY = diagnosticWrapHalf(control.offsetY - meanY)
		offset := math.Hypot(control.offsetX, control.offsetY)
		result.meanOffset += offset
		if offset > result.maxOffset {
			result.maxOffset = offset
		}
		available++
	}
	if available > 0 {
		result.meanOffset /= float64(available)
	}
}

// diagnosticFuseBlindLatticePhase combines the repetition observer's integer
// cycle with local-lattice modulo-block phase. Fractional replacement requires
// independent lattice confidence and phase agreement. Build16 evaluates every
// bounded +/-1 integer assignment exactly, per axis, and commits a proposal only
// when the exact top-2 margin satisfies the fixed key-independent gates. Any
// rejected or ambiguous proposal rolls the fractional lattice transaction back.
func diagnosticFuseBlindLatticePhase(primary, lattice diagnosticBlindPhaseResult, cells []diagnosticSpatialGridCell) diagnosticBlindPhaseResult {
	result := primary
	result.latticeMethod = lattice.method
	result.latticeControls = append([]diagnosticBlindCellControl(nil), lattice.controls...)
	result.latticeMeanConfidence = lattice.meanConfidence
	result.latticeMinConfidence = lattice.minConfidence
	result.latticeCycleAdjusted = make([]bool, len(result.controls))
	safePrimaryControls := append([]diagnosticBlindCellControl(nil), primary.controls...)

	if len(result.controls) != len(lattice.controls) || len(cells) != len(result.controls) {
		return result
	}

	fractionDistanceSum := 0.0
	for i := range result.controls {
		p, l := result.controls[i], lattice.controls[i]
		if !p.available || !l.available {
			continue
		}
		result.latticeCells++
		dx := diagnosticWrapHalf(p.offsetX - l.offsetX)
		dy := diagnosticWrapHalf(p.offsetY - l.offsetY)
		distance := math.Hypot(dx, dy)
		fractionDistanceSum += distance
		if distance > result.latticeMaxFractionDistance {
			result.latticeMaxFractionDistance = distance
		}
	}
	if result.latticeCells > 0 {
		result.latticeMeanFractionDistance = fractionDistanceSum / float64(result.latticeCells)
	}

	// First use the lattice measurement only as a fractional refinement when
	// both observers independently agree within the modulo-block cell.
	for i := range result.controls {
		p := &result.controls[i]
		l := lattice.controls[i]
		if !p.available || !l.available || l.confidence < diagnosticBlindLatticeConsensusMinConfidence {
			continue
		}
		pFracX := diagnosticWrapHalf(p.offsetX)
		pFracY := diagnosticWrapHalf(p.offsetY)
		distance := math.Hypot(diagnosticWrapHalf(pFracX-l.offsetX), diagnosticWrapHalf(pFracY-l.offsetY))
		if distance > diagnosticBlindLatticeConsensusMaxFractionDistance {
			continue
		}
		baseX := p.offsetX - pFracX
		baseY := p.offsetY - pFracY
		wp := math.Max(p.confidence, 0.03)
		wl := 0.55 * math.Max(l.confidence, 0.03)
		fracX := diagnosticWrapHalf((wp*pFracX + wl*l.offsetX) / (wp + wl))
		fracY := diagnosticWrapHalf((wp*pFracY + wl*l.offsetY) / (wp + wl))
		p.offsetX = baseX + fracX
		p.offsetY = baseY + fracY
		agreement := math.Exp(-2 * distance * distance)
		p.confidence = diagnosticClamp01(p.confidence*(0.90+0.10*agreement) + 0.12*l.confidence*agreement)
		result.latticeConsensusCells++
	}

	// Build16 retains the build15 global formulation but replaces heuristic
	// beam ranking with exact bounded enumeration. The modulo-one-block lattice
	// phase remains the fractional observation; integer +/-1 choices are solved
	// jointly (axis-separable) under affine smoothness, cycle-change cost and
	// exact ambiguity-margin gates. The optimizer is entirely key-independent.
	unwrapped, unwrapOK := diagnosticGlobalDiscreteUnwrap(result, lattice, cells)
	result = unwrapped
	result.latticeCycleAdjusted = make([]bool, len(result.controls))
	if unwrapOK && !result.unwrapAmbiguous {
		for i := range result.controls {
			sx, sy := 0, 0
			if i < len(result.unwrapShiftX) {
				sx = result.unwrapShiftX[i]
			}
			if i < len(result.unwrapShiftY) {
				sy = result.unwrapShiftY[i]
			}
			if sx != 0 || sy != 0 {
				result.latticeCycleAdjusted[i] = true
			}
		}
		result.latticeCycleSlipCorrections = result.unwrapChangedCells
	} else {
		// A rejected/ambiguous integer assignment invalidates the fractional
		// proposal as a correction too. Preserve all lattice/solver diagnostics,
		// but roll controls back to the pre-lattice blind observer so the
		// fractional-only failure mode documented in build14 cannot leak into
		// decode candidates.
		result.controls = safePrimaryControls
		result.latticeCycleSlipCorrections = 0
		result.unwrapChangedCells = 0
		result.unwrapAcceptedAxes = 0
		result.unwrapAppliedObjective = result.unwrapBaselineObjective
	}

	if result.latticeCycleSlipCorrections > 0 {
		result.method = "intra-tile-repetition+lattice-fractional-global-unwrap"
		diagnosticRecenterBlindControls(&result)
	} else {
		switch result.unwrapStatus {
		case "ambiguous":
			result.method = "intra-tile-repetition+lattice-global-unwrap-ambiguous"
		case "rejected-improvement":
			result.method = "intra-tile-repetition+lattice-global-unwrap-rejected"
		case "not-applicable":
			result.method = "intra-tile-repetition+lattice-global-unwrap-not-applicable"
		default:
			result.method = "intra-tile-repetition+lattice-global-unwrap-nochange"
		}
	}

	return result
}
