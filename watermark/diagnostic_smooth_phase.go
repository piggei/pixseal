package watermark

import (
	"image"
	"math"
	"sort"
)

const (
	diagnosticSmoothPhaseMinControls              = 6
	diagnosticSmoothPhaseQuadraticMinControls     = 8
	diagnosticSmoothPhaseMaxFitRMSBlocks          = 1.75
	diagnosticSmoothPhaseMaxLOORMSBlocks          = 2.50
	diagnosticSmoothPhaseDecodeMaxFitRMSBlocks    = 1.25
	diagnosticSmoothPhaseDecodeMaxLOORMSBlocks    = 2.00
	diagnosticSmoothPhaseMinMeanConfidence        = 0.12
	diagnosticSmoothPhaseDecodeMinMeanConfidence  = 0.20
	diagnosticSmoothPhaseQuadraticRidge           = 0.60
	diagnosticSmoothPhaseQuadraticMinAbsoluteGain = 0.15
	diagnosticSmoothPhaseQuadraticMinRelativeGain = 0.12
	diagnosticSmoothPhaseHuberK                   = 1.50
	diagnosticSmoothPhaseMaxComponentBlocks       = float64(diagnosticSpatialPhaseRadius) + 0.5
	diagnosticMaxSmoothPhaseResamples             = 2
)

// DiagnosticSmoothPhaseEvidence reports a bounded phase field fitted to the
// 3x3 local phase observations. Build11 introduced fractional-block controls,
// robust confidence weighting and affine-vs-quadratic leave-one-out selection.
// Build12 keeps that fitter but allows the projective path to source controls
// from key-independent blind self-registration. No hidden payload bit is used
// to fit or select the field.
type DiagnosticSmoothPhaseEvidence struct {
	Available                  bool       `json:"available"`
	ControlSource              string     `json:"control_source,omitempty"`
	Eligible                   bool       `json:"eligible"`
	DecodeEligible             bool       `json:"decode_eligible"`
	Model                      string     `json:"model,omitempty"`
	Controls                   int        `json:"controls"`
	MeanControlConfidence      float64    `json:"mean_control_confidence"`
	MinControlConfidence       float64    `json:"min_control_confidence"`
	RobustOutliers             int        `json:"robust_outliers"`
	FitRMSBlocks               float64    `json:"fit_rms_blocks"`
	LeaveOneOutRMSBlocks       float64    `json:"leave_one_out_rms_blocks"`
	AffineFitRMSBlocks         float64    `json:"affine_fit_rms_blocks"`
	AffineLeaveOneOutRMSBlocks float64    `json:"affine_leave_one_out_rms_blocks"`
	QuadraticAvailable         bool       `json:"quadratic_available"`
	QuadraticFitRMSBlocks      float64    `json:"quadratic_fit_rms_blocks"`
	QuadraticLeaveOneOutRMS    float64    `json:"quadratic_leave_one_out_rms_blocks"`
	QuadraticSelected          bool       `json:"quadratic_selected"`
	MaxCorrectionBlocks        float64    `json:"max_correction_blocks"`
	CorrectionXCoefficients    [3]float64 `json:"correction_x_coefficients_blocks"`
	CorrectionYCoefficients    [3]float64 `json:"correction_y_coefficients_blocks"`
	CorrectionXQuadratic       [6]float64 `json:"correction_x_quadratic_coefficients_blocks"`
	CorrectionYQuadratic       [6]float64 `json:"correction_y_quadratic_coefficients_blocks"`
	CorrectedGridAnalyzed      bool       `json:"corrected_grid_analyzed"`
	CorrectedProfile           Profile    `json:"corrected_profile,omitempty"`
	CorrectedKnownCodedErrors  int        `json:"corrected_known_header_coded_bit_errors"`
	CorrectedPostECCErrors     int        `json:"corrected_post_ecc_known_header_bit_errors"`
	CorrectedSoftPostECCErrors int        `json:"corrected_soft_post_ecc_known_header_bit_errors"`
	CorrectedSyncFraction      float64    `json:"corrected_sync_fraction"`
	CorrectedSyncZScore        float64    `json:"corrected_sync_z_score"`
	PostECCImprovement         int        `json:"post_ecc_known_header_bit_improvement"`
	SelectedForFullDecode      bool       `json:"selected_for_full_decode"`
}

type diagnosticSmoothPhaseControl struct {
	nx, ny     float64
	dx, dy     float64 // canonical correction in blocks, not observed phase delta
	weight     float64
	confidence float64
}

type diagnosticSmoothPhaseFit struct {
	model         string
	coefX         [6]float64
	coefY         [6]float64
	fitRMS        float64
	looRMS        float64
	maxCorrection float64
	outliers      int
}

func diagnosticFitSmoothPhaseField(spatial *DiagnosticSpatialBitEvidence, width, height float64) (*diagnosticResidualWarp, DiagnosticSmoothPhaseEvidence, bool) {
	return diagnosticFitSmoothPhaseFieldSource(spatial, width, height, false)
}

// diagnosticFitBlindSmoothPhaseField is the build12 path used by real diagnose
// candidates. Its controls come exclusively from blind self-registration of
// the repeated carrier. No key-assisted local phase, known header bit, or
// payload-dependent score participates in the fit or its confidence weights.
func diagnosticFitBlindSmoothPhaseField(spatial *DiagnosticSpatialBitEvidence, width, height float64) (*diagnosticResidualWarp, DiagnosticSmoothPhaseEvidence, bool) {
	return diagnosticFitSmoothPhaseFieldSource(spatial, width, height, true)
}

func diagnosticFitSmoothPhaseFieldSource(spatial *DiagnosticSpatialBitEvidence, width, height float64, blind bool) (*diagnosticResidualWarp, DiagnosticSmoothPhaseEvidence, bool) {
	evidence := DiagnosticSmoothPhaseEvidence{}
	if spatial == nil || width <= 1 || height <= 1 {
		return nil, evidence, false
	}
	controls := make([]diagnosticSmoothPhaseControl, 0, len(spatial.CellsEvidence))
	confidenceSum := 0.0
	minConfidence := 1.0
	for _, cell := range spatial.CellsEvidence {
		nx := 2*(float64(cell.RegionX)+0.5)/float64(diagnosticSpatialGridAxis) - 1
		ny := 2*(float64(cell.RegionY)+0.5)/float64(diagnosticSpatialGridAxis) - 1
		if blind {
			if !cell.BlindPhaseAvailable {
				continue
			}
			confidence := diagnosticClamp01(cell.BlindPhaseConfidence)
			controls = append(controls, diagnosticSmoothPhaseControl{
				nx: nx, ny: ny,
				dx: -cell.BlindPhaseOffsetX, dy: -cell.BlindPhaseOffsetY,
				weight: math.Max(confidence, 0.02), confidence: confidence,
			})
			confidenceSum += confidence
			if confidence < minConfidence {
				minConfidence = confidence
			}
			continue
		}

		if cell.LocalPhaseSyncFraction <= 0 {
			continue
		}
		offsetX := float64(cell.LocalPhaseOffsetX)
		offsetY := float64(cell.LocalPhaseOffsetY)
		confidence := diagnosticClamp01(cell.LocalPhaseConfidence)
		if cell.LocalPhaseSubblockAvailable {
			offsetX = cell.LocalPhaseSubblockOffsetX
			offsetY = cell.LocalPhaseSubblockOffsetY
		} else if confidence <= 0 {
			// Preserve deterministic synthetic/control tests that provide ideal
			// integer observations but predate the build11 surface estimator.
			confidence = diagnosticClamp01(cell.LocalPhaseSyncFraction)
		}
		baseWeight := math.Max(confidence, 0.05) * math.Max(cell.LocalPhaseSyncFraction, 0.35)
		controls = append(controls, diagnosticSmoothPhaseControl{
			nx: nx, ny: ny,
			dx: -offsetX, dy: -offsetY,
			weight: baseWeight, confidence: confidence,
		})
		confidenceSum += confidence
		if confidence < minConfidence {
			minConfidence = confidence
		}
	}
	if blind {
		evidence.ControlSource = "blind-self-registration"
	} else {
		evidence.ControlSource = "key-assisted-local-sync"
	}
	evidence.Controls = len(controls)
	if len(controls) < 4 {
		return nil, evidence, false
	}
	evidence.MeanControlConfidence = confidenceSum / float64(len(controls))
	evidence.MinControlConfidence = minConfidence

	affine, ok := diagnosticFitSmoothPhaseModel(controls, 3, 0, -1)
	if !ok {
		return nil, evidence, false
	}
	affine.model = "affine-robust"
	affine.looRMS = diagnosticSmoothPhaseLOO(controls, 3, 0)
	affine.maxCorrection = diagnosticSmoothPhaseMaxCorrection(affine.coefX, affine.coefY, 3)
	evidence.Available = true
	evidence.AffineFitRMSBlocks = affine.fitRMS
	evidence.AffineLeaveOneOutRMSBlocks = affine.looRMS

	selected := affine
	if len(controls) >= diagnosticSmoothPhaseQuadraticMinControls {
		quadratic, qok := diagnosticFitSmoothPhaseModel(controls, 6, diagnosticSmoothPhaseQuadraticRidge, -1)
		if qok {
			quadratic.model = "quadratic-ridge-robust"
			quadratic.looRMS = diagnosticSmoothPhaseLOO(controls, 6, diagnosticSmoothPhaseQuadraticRidge)
			quadratic.maxCorrection = diagnosticSmoothPhaseMaxCorrection(quadratic.coefX, quadratic.coefY, 6)
			evidence.QuadraticAvailable = true
			evidence.QuadraticFitRMSBlocks = quadratic.fitRMS
			evidence.QuadraticLeaveOneOutRMS = quadratic.looRMS
			absoluteGain := affine.looRMS - quadratic.looRMS
			relativeGain := 0.0
			if affine.looRMS > 1e-9 {
				relativeGain = absoluteGain / affine.looRMS
			}
			if quadratic.looRMS > 0 && absoluteGain >= diagnosticSmoothPhaseQuadraticMinAbsoluteGain &&
				relativeGain >= diagnosticSmoothPhaseQuadraticMinRelativeGain &&
				quadratic.fitRMS <= affine.fitRMS {
				selected = quadratic
				evidence.QuadraticSelected = true
			}
		}
	}

	evidence.Model = selected.model
	evidence.FitRMSBlocks = selected.fitRMS
	evidence.LeaveOneOutRMSBlocks = selected.looRMS
	evidence.MaxCorrectionBlocks = selected.maxCorrection
	evidence.RobustOutliers = selected.outliers
	copy(evidence.CorrectionXCoefficients[:], selected.coefX[:3])
	copy(evidence.CorrectionYCoefficients[:], selected.coefY[:3])
	evidence.CorrectionXQuadratic = selected.coefX
	evidence.CorrectionYQuadratic = selected.coefY

	evidence.Eligible = len(controls) >= diagnosticSmoothPhaseMinControls &&
		evidence.MeanControlConfidence >= diagnosticSmoothPhaseMinMeanConfidence &&
		evidence.FitRMSBlocks <= diagnosticSmoothPhaseMaxFitRMSBlocks &&
		evidence.LeaveOneOutRMSBlocks > 0 && evidence.LeaveOneOutRMSBlocks <= diagnosticSmoothPhaseMaxLOORMSBlocks &&
		evidence.MaxCorrectionBlocks <= diagnosticSmoothPhaseMaxComponentBlocks
	evidence.DecodeEligible = len(controls) >= diagnosticSmoothPhaseMinControls &&
		evidence.MeanControlConfidence >= diagnosticSmoothPhaseDecodeMinMeanConfidence &&
		evidence.FitRMSBlocks <= diagnosticSmoothPhaseDecodeMaxFitRMSBlocks &&
		evidence.LeaveOneOutRMSBlocks > 0 && evidence.LeaveOneOutRMSBlocks <= diagnosticSmoothPhaseDecodeMaxLOORMSBlocks &&
		evidence.MaxCorrectionBlocks <= diagnosticSmoothPhaseMaxComponentBlocks

	warp := &diagnosticResidualWarp{
		width: width, height: height,
		maxPixels: diagnosticSmoothPhaseMaxComponentBlocks * float64(blockSize),
		controls:  len(controls),
	}
	for i := range warp.dx {
		warp.dx[i] = selected.coefX[i] * float64(blockSize)
		warp.dy[i] = selected.coefY[i] * float64(blockSize)
	}
	return warp, evidence, true
}

func diagnosticFitSmoothPhaseModel(controls []diagnosticSmoothPhaseControl, basisCount int, ridge float64, omit int) (diagnosticSmoothPhaseFit, bool) {
	result := diagnosticSmoothPhaseFit{}
	if basisCount != 3 && basisCount != 6 {
		return result, false
	}
	robust := make([]float64, len(controls))
	for i := range robust {
		robust[i] = 1
	}
	var coefX, coefY [6]float64
	for iteration := 0; iteration < 3; iteration++ {
		var normal [6][6]float64
		var rhsX, rhsY [6]float64
		weightSum := 0.0
		used := 0
		for i, c := range controls {
			if i == omit {
				continue
			}
			basis := diagnosticSmoothPhaseBasis(c.nx, c.ny)
			weight := math.Max(c.weight, 0.01) * robust[i]
			weightSum += weight
			for r := 0; r < basisCount; r++ {
				rhsX[r] += weight * basis[r] * c.dx
				rhsY[r] += weight * basis[r] * c.dy
				for col := 0; col < basisCount; col++ {
					normal[r][col] += weight * basis[r] * basis[col]
				}
			}
			used++
		}
		if used < basisCount || weightSum <= 0 {
			return result, false
		}
		if basisCount == 6 && ridge > 0 {
			scaledRidge := ridge * weightSum / float64(used)
			for i := 3; i < 6; i++ {
				normal[i][i] += scaledRidge
			}
		}
		var okX, okY bool
		coefX, okX = diagnosticSolveSmoothPhase(normal, rhsX, basisCount)
		coefY, okY = diagnosticSolveSmoothPhase(normal, rhsY, basisCount)
		if !okX || !okY {
			return result, false
		}
		residuals := make([]float64, 0, used)
		for i, c := range controls {
			if i == omit {
				continue
			}
			px, py := diagnosticSmoothPhasePredict(coefX, coefY, basisCount, c.nx, c.ny)
			residuals = append(residuals, math.Hypot(px-c.dx, py-c.dy))
		}
		sort.Float64s(residuals)
		median := diagnosticQuantileSorted(residuals, 0.50)
		scale := math.Max(0.10, 1.4826*median)
		threshold := diagnosticSmoothPhaseHuberK * scale
		idx := 0
		for i := range controls {
			if i == omit {
				continue
			}
			err := residuals[idx]
			// residuals is sorted, so recompute the point-specific error for the
			// robust factor rather than using the sorted ordering.
			c := controls[i]
			px, py := diagnosticSmoothPhasePredict(coefX, coefY, basisCount, c.nx, c.ny)
			err = math.Hypot(px-c.dx, py-c.dy)
			factor := 1.0
			if err > threshold && err > 1e-9 {
				factor = threshold / err
			}
			robust[i] = factor
			idx++
		}
	}

	weightedSquared, weightSum := 0.0, 0.0
	outliers := 0
	for i, c := range controls {
		if i == omit {
			continue
		}
		px, py := diagnosticSmoothPhasePredict(coefX, coefY, basisCount, c.nx, c.ny)
		err := math.Hypot(px-c.dx, py-c.dy)
		weight := math.Max(c.weight, 0.01) * robust[i]
		weightedSquared += weight * err * err
		weightSum += weight
		if robust[i] < 0.5 {
			outliers++
		}
	}
	if weightSum <= 0 {
		return result, false
	}
	result.coefX = coefX
	result.coefY = coefY
	result.fitRMS = math.Sqrt(weightedSquared / weightSum)
	result.outliers = outliers
	return result, true
}

func diagnosticSmoothPhaseLOO(controls []diagnosticSmoothPhaseControl, basisCount int, ridge float64) float64 {
	if len(controls) <= basisCount {
		return 0
	}
	weightedSquared, weightSum := 0.0, 0.0
	for omitted, c := range controls {
		fit, ok := diagnosticFitSmoothPhaseModel(controls, basisCount, ridge, omitted)
		if !ok {
			continue
		}
		px, py := diagnosticSmoothPhasePredict(fit.coefX, fit.coefY, basisCount, c.nx, c.ny)
		err := math.Hypot(px-c.dx, py-c.dy)
		weight := math.Max(c.weight, 0.01)
		weightedSquared += weight * err * err
		weightSum += weight
	}
	if weightSum <= 0 {
		return 0
	}
	return math.Sqrt(weightedSquared / weightSum)
}

func diagnosticSmoothPhaseBasis(nx, ny float64) [6]float64 {
	return [6]float64{1, nx, ny, nx * ny, nx * nx, ny * ny}
}

func diagnosticSmoothPhasePredict(coefX, coefY [6]float64, basisCount int, nx, ny float64) (float64, float64) {
	basis := diagnosticSmoothPhaseBasis(nx, ny)
	px, py := 0.0, 0.0
	for i := 0; i < basisCount; i++ {
		px += coefX[i] * basis[i]
		py += coefY[i] * basis[i]
	}
	return px, py
}

func diagnosticSmoothPhaseMaxCorrection(coefX, coefY [6]float64, basisCount int) float64 {
	maxCorrection := 0.0
	for _, p := range [][2]float64{
		{-1, -1}, {0, -1}, {1, -1},
		{-1, 0}, {0, 0}, {1, 0},
		{-1, 1}, {0, 1}, {1, 1},
	} {
		px, py := diagnosticSmoothPhasePredict(coefX, coefY, basisCount, p[0], p[1])
		if magnitude := math.Hypot(px, py); magnitude > maxCorrection {
			maxCorrection = magnitude
		}
	}
	return maxCorrection
}

func diagnosticSolveSmoothPhase(matrix [6][6]float64, rhs [6]float64, n int) ([6]float64, bool) {
	var augmented [6][7]float64
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			augmented[r][c] = matrix[r][c]
		}
		augmented[r][n] = rhs[r]
	}
	for col := 0; col < n; col++ {
		pivot := col
		for row := col + 1; row < n; row++ {
			if math.Abs(augmented[row][col]) > math.Abs(augmented[pivot][col]) {
				pivot = row
			}
		}
		if math.Abs(augmented[pivot][col]) < 1e-9 {
			return [6]float64{}, false
		}
		augmented[col], augmented[pivot] = augmented[pivot], augmented[col]
		divisor := augmented[col][col]
		for c := col; c <= n; c++ {
			augmented[col][c] /= divisor
		}
		for row := 0; row < n; row++ {
			if row == col {
				continue
			}
			factor := augmented[row][col]
			for c := col; c <= n; c++ {
				augmented[row][c] -= factor * augmented[col][c]
			}
		}
	}
	var solution [6]float64
	for i := 0; i < n; i++ {
		solution[i] = augmented[i][n]
	}
	return solution, true
}

func diagnosticAnalyzeSmoothPhaseCorrection(src image.Image, width, height int, mapper diagnosticProjectiveMapper, mode diagnosticPhotometricMode, base DiagnosticBitChannelEvidence, key []byte, decoder *decoder) (DiagnosticSmoothPhaseEvidence, []float64, bool) {
	warp, smooth, ok := diagnosticFitBlindSmoothPhaseField(base.Spatial, float64(width), float64(height))
	if !ok || !smooth.Available || !smooth.Eligible {
		return smooth, nil, false
	}
	correctedMapper := mapper
	correctedMapper.smoothPhaseWarp = warp
	grid, _, ok := diagnosticProjectiveGridWithMapperPhotometricStats(src, width, height, correctedMapper, mode)
	if !ok {
		return smooth, nil, false
	}
	corrected, ok := diagnosticAnalyzeBitChannel(grid, key, decoder, base.GeometrySource+"+smooth-phase", mode)
	if !ok {
		return smooth, nil, false
	}
	smooth.CorrectedGridAnalyzed = true
	smooth.CorrectedProfile = corrected.Profile
	smooth.CorrectedKnownCodedErrors = corrected.KnownHeaderCodedErrors
	smooth.CorrectedPostECCErrors = corrected.PostECCHdrErrors
	smooth.CorrectedSoftPostECCErrors = corrected.SoftPostECCHdrErrors
	if corrected.KnownSyncBits > 0 {
		score := corrected.KnownSyncBits - corrected.KnownSyncBitErrors
		smooth.CorrectedSyncFraction = float64(score) / float64(corrected.KnownSyncBits)
		smooth.CorrectedSyncZScore = (float64(score) - float64(corrected.KnownSyncBits)/2) / math.Sqrt(float64(corrected.KnownSyncBits)/4)
	}
	smooth.PostECCImprovement = base.PostECCHdrErrors - corrected.PostECCHdrErrors
	return smooth, grid, true
}
