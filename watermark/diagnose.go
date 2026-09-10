package watermark

import (
	"errors"
	"image"
	"math"
	"sort"
	"time"
)

const (
	defaultDiagnosticRegionsX      = 3
	defaultDiagnosticRegionsY      = 3
	defaultDiagnosticMaxDimension  = 2048
	defaultDiagnosticMaxLevels     = 2
	diagnosticMinPeriod            = 3.0
	diagnosticMaxPeriod            = 12.0
	diagnosticPeriodStep           = 1.0
	diagnosticMinAngleDegrees      = -45.0
	diagnosticMaxAngleDegrees      = 45.0
	diagnosticAngleStepDegrees     = 7.5
	diagnosticCoarsePhaseDivisions = 3
	diagnosticFinePhaseDivisions   = 4
	diagnosticSampleBlocks         = 5
	diagnosticCoarseKeep           = 3
	diagnosticFinalPool            = 8
	diagnosticRepetitionPool       = 12
)

// DiagnosticOptions bounds the experimental local-lattice estimator. It is
// intentionally separate from ExtractWithInfo: diagnostic evidence must not
// become an authenticated watermark result unless the Format v3 HMAC passes.
type DiagnosticOptions struct {
	RegionsX              int
	RegionsY              int
	MaxAnalysisDimension  int
	MaxLevels             int
	AttemptAuthentication bool
}

// DefaultDiagnosticOptions returns the bounded v0.3.0-build3 research budget.
func DefaultDiagnosticOptions() DiagnosticOptions {
	return DiagnosticOptions{
		RegionsX:             defaultDiagnosticRegionsX,
		RegionsY:             defaultDiagnosticRegionsY,
		MaxAnalysisDimension: defaultDiagnosticMaxDimension,
		MaxLevels:            defaultDiagnosticMaxLevels,
	}
}

// LatticeVector is one observed local block-period vector in source-image pixels.
type LatticeVector struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// LocalLatticeEstimate is the strongest bounded lattice hypothesis for one
// spatial region. U and V are reported in native source-image pixels even when
// the estimator worked on a downsampled diagnostic level.
type LocalLatticeEstimate struct {
	RegionX                 int           `json:"region_x"`
	RegionY                 int           `json:"region_y"`
	NativeX                 int           `json:"native_x"`
	NativeY                 int           `json:"native_y"`
	NativeWidth             int           `json:"native_width"`
	NativeHeight            int           `json:"native_height"`
	AnalysisDivisor         int           `json:"analysis_divisor"`
	U                       LatticeVector `json:"u"`
	V                       LatticeVector `json:"v"`
	PeriodU                 float64       `json:"period_u"`
	PeriodV                 float64       `json:"period_v"`
	OrientationDegrees      float64       `json:"orientation_degrees"`
	InterAxisDegrees        float64       `json:"inter_axis_degrees"`
	PhaseU                  float64       `json:"phase_u"`
	PhaseV                  float64       `json:"phase_v"`
	DCTDifferentialMargin   float64       `json:"dct_differential_margin"`
	PeriodicCoherence       float64       `json:"periodic_coherence"`
	TileRepetitionCoherence float64       `json:"tile_repetition_coherence"`
	Confidence              float64       `json:"confidence"`
	CandidatesEvaluated     int           `json:"candidates_evaluated"`
}

// DiagnosticLevel describes one sampled pyramid level. Divisor is relative to
// native source coordinates: 8 means approximately one diagnostic pixel per
// 8x8 source-pixel cell.
type DiagnosticLevel struct {
	Divisor int `json:"divisor"`
	Width   int `json:"width"`
	Height  int `json:"height"`
}

// DiagnosticBudgets makes the bounded research search explicit in machine output.
type DiagnosticBudgets struct {
	MaxRegions                   int `json:"max_regions"`
	MaxLevels                    int `json:"max_levels"`
	CoarseBasisCandidates        int `json:"coarse_basis_candidates_per_region"`
	RefinedBasisCandidates       int `json:"refined_basis_candidates_per_region"`
	QuickRepetitionCandidates    int `json:"quick_repetition_candidates_per_region"`
	FullRepetitionShortlist      int `json:"full_repetition_shortlist_per_region"`
	MaxFullRepetitionCandidates  int `json:"max_full_repetition_candidates_per_region"`
	MaxPhaseHypotheses           int `json:"max_phase_hypotheses_per_basis"`
	SampleBlocksPerPhase         int `json:"sample_blocks_per_phase"`
	MaxProjectiveScaleCandidates int `json:"max_projective_scale_candidates"`
	MaxProjectiveFullDecodes     int `json:"max_projective_full_decodes"`
	MaxPhaseRefineSeeds          int `json:"max_phase_refine_seeds"`
	MaxPhaseScaleProbes          int `json:"max_phase_scale_probes"`
	MaxPhaseHomographyFits       int `json:"max_phase_homography_fits"`
}

// DiagnosticTimings reports wall-clock stage timings without affecting decoder behavior.
type DiagnosticTimings struct {
	PyramidMilliseconds        int64 `json:"pyramid_ms"`
	PrintBoundaryMilliseconds  int64 `json:"print_boundary_ms"`
	LocalLatticeMilliseconds   int64 `json:"local_lattice_ms"`
	ProjectiveFitMilliseconds  int64 `json:"projective_fit_ms"`
	AuthenticationMilliseconds int64 `json:"authentication_ms"`
	TotalMilliseconds          int64 `json:"total_ms"`
}

// DiagnosticReport is research evidence, not a watermark-detection result.
// AuthenticatedPayload can become true only through the existing v3 HMAC path.
type DiagnosticReport struct {
	Width                    int                                `json:"width"`
	Height                   int                                `json:"height"`
	Levels                   []DiagnosticLevel                  `json:"levels"`
	PrintBoundary            PrintBoundaryEstimate              `json:"print_boundary"`
	BoundaryPriorUsed        bool                               `json:"boundary_prior_used"`
	ProjectiveEstimate       DiagnosticProjectiveEstimate       `json:"projective_estimate"`
	Regions                  []LocalLatticeEstimate             `json:"regions"`
	GlobalU                  LatticeVector                      `json:"global_u"`
	GlobalV                  LatticeVector                      `json:"global_v"`
	GlobalConsistency        float64                            `json:"global_consistency"`
	ConsensusRegions         int                                `json:"consensus_regions"`
	ConsensusFraction        float64                            `json:"consensus_fraction"`
	LatticeEvidence          bool                               `json:"lattice_evidence"`
	AuthenticationStatus     string                             `json:"authentication_status"`
	AuthenticatedPayload     bool                               `json:"authenticated_payload"`
	AuthenticatedMessage     string                             `json:"authenticated_message,omitempty"`
	AuthenticatedProfile     Profile                            `json:"authenticated_profile,omitempty"`
	AuthenticationConfidence float64                            `json:"authentication_confidence,omitempty"`
	ProjectiveAuthentication DiagnosticProjectiveAuthentication `json:"projective_authentication"`
	Budgets                  DiagnosticBudgets                  `json:"budgets"`
	Timings                  DiagnosticTimings                  `json:"timings"`
	Note                     string                             `json:"note"`
}

type diagnosticPlane struct {
	width, height int
	divisor       int
	luma          []float32
}

type diagnosticRect struct {
	x, y, width, height int
}

type diagnosticBasisCandidate struct {
	u, v                LatticeVector
	phaseU, phaseV      float64
	bestMargin          float64
	contrast            float64
	coherence           float64
	repetitionCoherence float64
	quality             float64
	candidatesEvaluated int
}

// DiagnoseGeometry performs the v0.3.0-build3 bounded, key-independent local
// lattice analysis. Optional baseline authentication is deliberately separate:
// it does not consume lattice estimates and therefore cannot turn a false lattice
// candidate into an authenticated result.
func DiagnoseGeometry(src image.Image, key []byte, options DiagnosticOptions) (DiagnosticReport, error) {
	started := time.Now()
	if src == nil {
		return DiagnosticReport{}, errors.New("nil image")
	}
	if err := validateWorkingImageSize(src); err != nil {
		return DiagnosticReport{}, err
	}
	options = normalizeDiagnosticOptions(options)
	bounds := src.Bounds()
	report := DiagnosticReport{
		Width:                bounds.Dx(),
		Height:               bounds.Dy(),
		AuthenticationStatus: "not-requested",
		Note:                 "lattice evidence is diagnostic only; only a valid Format v3 HMAC authenticates a payload",
	}
	coarseCount := diagnosticCoarseCandidateCount()
	refinedCount := diagnosticCoarseKeep * 3 * 3 * 3 * 3
	report.Budgets = DiagnosticBudgets{
		MaxRegions:                   options.RegionsX * options.RegionsY,
		MaxLevels:                    options.MaxLevels,
		CoarseBasisCandidates:        coarseCount,
		RefinedBasisCandidates:       refinedCount,
		QuickRepetitionCandidates:    coarseCount,
		FullRepetitionShortlist:      diagnosticRepetitionPool,
		MaxFullRepetitionCandidates:  2*diagnosticRepetitionPool + 1,
		MaxPhaseHypotheses:           diagnosticFinePhaseDivisions * diagnosticFinePhaseDivisions,
		SampleBlocksPerPhase:         diagnosticSampleBlocks * diagnosticSampleBlocks,
		MaxProjectiveScaleCandidates: diagnosticMaxProjectiveScaleCandidates,
		MaxProjectiveFullDecodes:     diagnosticMaxProjectiveFullDecodes,
		MaxPhaseRefineSeeds:          diagnosticMaxPhaseRefineSeeds,
		MaxPhaseScaleProbes:          diagnosticMaxPhaseRefineSeeds * diagnosticPhaseScaleProbesPerSeed,
		MaxPhaseHomographyFits:       diagnosticMaxPhaseRefineSeeds,
	}

	pyramidStarted := time.Now()
	divisors := diagnosticDivisors(bounds.Dx(), bounds.Dy(), options.MaxAnalysisDimension, options.MaxLevels)
	planes := make([]*diagnosticPlane, 0, len(divisors))
	for _, divisor := range divisors {
		plane := buildDiagnosticPlane(src, divisor)
		planes = append(planes, plane)
		report.Levels = append(report.Levels, DiagnosticLevel{Divisor: divisor, Width: plane.width, Height: plane.height})
	}
	report.Timings.PyramidMilliseconds = time.Since(pyramidStarted).Milliseconds()

	boundaryStarted := time.Now()
	report.PrintBoundary = estimatePrintBoundary(src)
	report.Timings.PrintBoundaryMilliseconds = time.Since(boundaryStarted).Milliseconds()
	report.BoundaryPriorUsed = report.PrintBoundary.Detected && report.PrintBoundary.Confidence >= 0.35 && bounds.Dx() >= 1000 && bounds.Dy() >= 1000

	latticeStarted := time.Now()
	pools := make([][]LocalLatticeEstimate, options.RegionsX*options.RegionsY)
	for _, plane := range planes {
		for ry := 0; ry < options.RegionsY; ry++ {
			for rx := 0; rx < options.RegionsX; rx++ {
				region := diagnosticRegion(plane, rx, ry, options.RegionsX, options.RegionsY)
				var pageU, pageV LatticeVector
				useLocalBoundaryPrior := false
				if report.BoundaryPriorUsed {
					pageU, pageV, useLocalBoundaryPrior = boundaryExpectedTangents(report.PrintBoundary, rx, ry, options.RegionsX, options.RegionsY)
				}
				candidates := estimateDiagnosticBasisPoolWithPrior(plane, region, pageU, pageV, useLocalBoundaryPrior)
				index := ry*options.RegionsX + rx
				for _, candidate := range candidates {
					pools[index] = append(pools[index], diagnosticCandidateToEstimate(candidate, plane, region, rx, ry))
				}
			}
		}
	}
	report.Regions, report.GlobalU, report.GlobalV, report.GlobalConsistency, report.ConsensusRegions, report.LatticeEvidence = selectDiagnosticConsensus(pools, report.PrintBoundary, report.BoundaryPriorUsed, options.RegionsX, options.RegionsY)
	if len(pools) > 0 {
		report.ConsensusFraction = float64(report.ConsensusRegions) / float64(len(pools))
	}
	report.Timings.LocalLatticeMilliseconds = time.Since(latticeStarted).Milliseconds()

	projectiveStarted := time.Now()
	if report.BoundaryPriorUsed {
		report.ProjectiveEstimate = estimateDiagnosticProjective(report.PrintBoundary, pools, report.Regions, report.GlobalConsistency, options.RegionsX, options.RegionsY)
	}
	report.Timings.ProjectiveFitMilliseconds = time.Since(projectiveStarted).Milliseconds()

	if options.AttemptAuthentication {
		authStarted := time.Now()
		if len(key) < 8 {
			report.AuthenticationStatus = "invalid-key"
		} else {
			baselineAttempted := !exceedsPixelLimit(bounds.Dx(), bounds.Dy(), maxSearchPixels)
			if baselineAttempted {
				payload, info, err := ExtractWithInfo(src, key)
				if err == nil {
					report.AuthenticationStatus = "baseline-authenticated"
					report.AuthenticatedPayload = true
					report.AuthenticatedMessage = string(payload)
					report.AuthenticatedProfile = info.Profile
					report.AuthenticationConfidence = info.Confidence
				}
			}
			if !report.AuthenticatedPayload && report.ProjectiveEstimate.Available {
				payload, info, evidence, found := attemptDiagnosticProjectiveAuthentication(src, key, report.ProjectiveEstimate)
				report.ProjectiveAuthentication = evidence
				if found {
					report.AuthenticationStatus = "projective-authenticated"
					report.AuthenticatedPayload = true
					report.AuthenticatedMessage = string(payload)
					report.AuthenticatedProfile = info.Profile
					report.AuthenticationConfidence = info.Confidence
				} else if baselineAttempted {
					report.AuthenticationStatus = "baseline+projective-failed"
				} else {
					report.AuthenticationStatus = "projective-failed"
				}
			} else if !report.AuthenticatedPayload {
				if baselineAttempted {
					report.AuthenticationStatus = "baseline-failed"
				} else {
					report.AuthenticationStatus = "skipped-large-image"
				}
			}
		}
		report.Timings.AuthenticationMilliseconds = time.Since(authStarted).Milliseconds()
	}
	report.Timings.TotalMilliseconds = time.Since(started).Milliseconds()
	return report, nil
}

func normalizeDiagnosticOptions(options DiagnosticOptions) DiagnosticOptions {
	defaults := DefaultDiagnosticOptions()
	if options.RegionsX <= 0 {
		options.RegionsX = defaults.RegionsX
	}
	if options.RegionsY <= 0 {
		options.RegionsY = defaults.RegionsY
	}
	if options.RegionsX > 4 {
		options.RegionsX = 4
	}
	if options.RegionsY > 4 {
		options.RegionsY = 4
	}
	if options.MaxAnalysisDimension <= 0 {
		options.MaxAnalysisDimension = defaults.MaxAnalysisDimension
	}
	if options.MaxAnalysisDimension < 512 {
		options.MaxAnalysisDimension = 512
	}
	if options.MaxAnalysisDimension > 4096 {
		options.MaxAnalysisDimension = 4096
	}
	if options.MaxLevels <= 0 {
		options.MaxLevels = defaults.MaxLevels
	}
	if options.MaxLevels > 3 {
		options.MaxLevels = 3
	}
	return options
}

func diagnosticDivisors(width, height, maxDimension, maxLevels int) []int {
	largest := width
	if height > largest {
		largest = height
	}
	divisor := 1
	for (largest+divisor-1)/divisor > maxDimension {
		divisor *= 2
	}
	if divisor == 1 {
		return []int{1}
	}
	result := make([]int, 0, maxLevels)
	for level := 0; level < maxLevels; level++ {
		current := divisor << level
		levelWidth := (width + current - 1) / current
		levelHeight := (height + current - 1) / current
		if levelWidth < 160 || levelHeight < 160 {
			break
		}
		result = append(result, current)
	}
	if len(result) == 0 {
		result = append(result, divisor)
	}
	return result
}

func buildDiagnosticPlane(src image.Image, divisor int) *diagnosticPlane {
	bounds := src.Bounds()
	width := (bounds.Dx() + divisor - 1) / divisor
	height := (bounds.Dy() + divisor - 1) / divisor
	plane := &diagnosticPlane{width: width, height: height, divisor: divisor, luma: make([]float32, width*height)}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if divisor == 1 {
				pixel := flattenedNRGBA(src.At(bounds.Min.X+x, bounds.Min.Y+y))
				plane.luma[y*width+x] = float32(.299*float64(pixel.R) + .587*float64(pixel.G) + .114*float64(pixel.B) - 128)
				continue
			}
			// Four deterministic sub-cell samples reduce aliasing without creating a
			// full-resolution RGB copy. This keeps peak memory bounded for 200 MP input.
			sum := 0.0
			for _, oy := range [...]float64{0.25, 0.75} {
				for _, ox := range [...]float64{0.25, 0.75} {
					sx := x*divisor + int(ox*float64(divisor))
					sy := y*divisor + int(oy*float64(divisor))
					if sx >= bounds.Dx() {
						sx = bounds.Dx() - 1
					}
					if sy >= bounds.Dy() {
						sy = bounds.Dy() - 1
					}
					pixel := flattenedNRGBA(src.At(bounds.Min.X+sx, bounds.Min.Y+sy))
					sum += .299*float64(pixel.R) + .587*float64(pixel.G) + .114*float64(pixel.B) - 128
				}
			}
			plane.luma[y*width+x] = float32(sum / 4)
		}
	}
	return plane
}

func diagnosticRegion(plane *diagnosticPlane, rx, ry, regionsX, regionsY int) diagnosticRect {
	x0 := rx * plane.width / regionsX
	x1 := (rx + 1) * plane.width / regionsX
	y0 := ry * plane.height / regionsY
	y1 := (ry + 1) * plane.height / regionsY
	margin := 16
	if x1-x0 > margin*2+64 {
		x0 += margin
		x1 -= margin
	}
	if y1-y0 > margin*2+64 {
		y0 += margin
		y1 -= margin
	}
	return diagnosticRect{x: x0, y: y0, width: x1 - x0, height: y1 - y0}
}

func diagnosticCoarseCandidateCount() int {
	periods := int(math.Floor((diagnosticMaxPeriod-diagnosticMinPeriod)/diagnosticPeriodStep+0.5)) + 1
	angles := int(math.Floor((diagnosticMaxAngleDegrees-diagnosticMinAngleDegrees)/diagnosticAngleStepDegrees+0.5)) + 1
	return periods * angles
}

func estimateDiagnosticBasisPool(plane *diagnosticPlane, region diagnosticRect) []diagnosticBasisCandidate {
	return estimateDiagnosticBasisPoolWithPrior(plane, region, LatticeVector{}, LatticeVector{}, false)
}

func estimateDiagnosticBasisPoolWithPrior(plane *diagnosticPlane, region diagnosticRect, pageU, pageV LatticeVector, useBoundaryPrior bool) []diagnosticBasisCandidate {
	coarse := make([]diagnosticBasisCandidate, 0, diagnosticCoarseCandidateCount())
	for angle := diagnosticMinAngleDegrees; angle <= diagnosticMaxAngleDegrees+1e-9; angle += diagnosticAngleStepDegrees {
		radians := angle * math.Pi / 180
		cosine, sine := math.Cos(radians), math.Sin(radians)
		for period := diagnosticMinPeriod; period <= diagnosticMaxPeriod+1e-9; period += diagnosticPeriodStep {
			candidate := scoreDiagnosticBasisPhase(plane, region,
				LatticeVector{X: period * cosine, Y: period * sine},
				LatticeVector{X: -period * sine, Y: period * cosine},
				diagnosticCoarsePhaseDivisions)
			coarse = append(coarse, candidate)
		}
	}
	for index := range coarse {
		applyDiagnosticRepetitionQuick(plane, region, &coarse[index])
	}
	sort.Slice(coarse, func(i, j int) bool {
		return diagnosticCandidateRank(coarse[i], plane.divisor, pageU, pageV, useBoundaryPrior) > diagnosticCandidateRank(coarse[j], plane.divisor, pageU, pageV, useBoundaryPrior)
	})
	coarsePool := selectDistinctDiagnosticCandidates(coarse, diagnosticRepetitionPool)
	coarsePool = ensureDiagnosticCanonicalAnchor(plane, region, coarsePool, coarse)
	for index := range coarsePool {
		applyDiagnosticRepetition(plane, region, &coarsePool[index])
	}
	sort.Slice(coarsePool, func(i, j int) bool {
		return diagnosticCandidateRank(coarsePool[i], plane.divisor, pageU, pageV, useBoundaryPrior) > diagnosticCandidateRank(coarsePool[j], plane.divisor, pageU, pageV, useBoundaryPrior)
	})
	seeds := selectDistinctDiagnosticCandidates(coarsePool, diagnosticCoarseKeep)

	refined := make([]diagnosticBasisCandidate, 0, diagnosticCoarseKeep*81)
	for _, seed := range seeds {
		baseAngle := math.Atan2(seed.u.Y, seed.u.X)
		basePeriodU := math.Hypot(seed.u.X, seed.u.Y)
		basePeriodV := math.Hypot(seed.v.X, seed.v.Y)
		for _, rotationDelta := range [...]float64{-2, 0, 2} {
			for _, shearDelta := range [...]float64{-5, 0, 5} {
				for _, scaleU := range [...]float64{0.90, 1.00, 1.10} {
					for _, scaleV := range [...]float64{0.90, 1.00, 1.10} {
						uAngle := baseAngle + rotationDelta*math.Pi/180
						vAngle := baseAngle + (90+rotationDelta+shearDelta)*math.Pi/180
						uPeriod := basePeriodU * scaleU
						vPeriod := basePeriodV * scaleV
						candidate := scoreDiagnosticBasisPhase(plane, region,
							LatticeVector{X: uPeriod * math.Cos(uAngle), Y: uPeriod * math.Sin(uAngle)},
							LatticeVector{X: vPeriod * math.Cos(vAngle), Y: vPeriod * math.Sin(vAngle)},
							diagnosticFinePhaseDivisions)
						refined = append(refined, candidate)
					}
				}
			}
		}
	}
	if len(refined) == 0 {
		if len(coarsePool) == 0 {
			return nil
		}
		limit := diagnosticFinalPool
		if len(coarsePool) < limit {
			limit = len(coarsePool)
		}
		result := append([]diagnosticBasisCandidate(nil), coarsePool[:limit]...)
		for index := range result {
			result[index].candidatesEvaluated = len(coarse)
		}
		return result
	}
	sort.Slice(refined, func(i, j int) bool {
		return diagnosticCandidateRank(refined[i], plane.divisor, pageU, pageV, useBoundaryPrior) > diagnosticCandidateRank(refined[j], plane.divisor, pageU, pageV, useBoundaryPrior)
	})
	refinedPool := selectDistinctDiagnosticCandidates(refined, diagnosticRepetitionPool)
	for index := range refinedPool {
		applyDiagnosticRepetition(plane, region, &refinedPool[index])
	}
	sort.Slice(refinedPool, func(i, j int) bool {
		return diagnosticCandidateRank(refinedPool[i], plane.divisor, pageU, pageV, useBoundaryPrior) > diagnosticCandidateRank(refinedPool[j], plane.divisor, pageU, pageV, useBoundaryPrior)
	})
	if len(refinedPool) == 0 {
		return nil
	}
	limit := diagnosticFinalPool
	if len(refinedPool) < limit {
		limit = len(refinedPool)
	}
	result := append([]diagnosticBasisCandidate(nil), refinedPool[:limit]...)
	for index := range result {
		result[index].candidatesEvaluated = len(coarse) + len(refined)
	}
	return result
}

func diagnosticCandidateRank(candidate diagnosticBasisCandidate, divisor int, pageU, pageV LatticeVector, useBoundaryPrior bool) float64 {
	if !useBoundaryPrior {
		return candidate.quality
	}
	fit := diagnosticCandidateBoundaryFit(candidate, divisor, pageU, pageV)
	// The print boundary is an initializer, never watermark evidence. Keep a
	// non-zero floor so strong lattice measurements can still disagree with it.
	return candidate.quality * (0.12 + 0.88*fit)
}

func diagnosticCandidateBoundaryFit(candidate diagnosticBasisCandidate, divisor int, pageU, pageV LatticeVector) float64 {
	determinant := pageU.X*pageV.Y - pageU.Y*pageV.X
	if math.Abs(determinant) < 1e-9 {
		return 0
	}
	native := LocalLatticeEstimate{
		U: LatticeVector{X: candidate.u.X * float64(divisor), Y: candidate.u.Y * float64(divisor)},
		V: LatticeVector{X: candidate.v.X * float64(divisor), Y: candidate.v.Y * float64(divisor)},
	}
	best := 0.0
	for quarter := 0; quarter < 4; quarter++ {
		variant := diagnosticQuarterEquivalent(native, quarter)
		transform := func(vector LatticeVector) LatticeVector {
			return LatticeVector{
				X: (vector.X*pageV.Y - vector.Y*pageV.X) / determinant,
				Y: (pageU.X*vector.Y - pageU.Y*vector.X) / determinant,
			}
		}
		u, v := transform(variant.U), transform(variant.V)
		ul, vl := math.Hypot(u.X, u.Y), math.Hypot(v.X, v.Y)
		if ul < 1e-12 || vl < 1e-12 {
			continue
		}
		offAxis := math.Abs(u.Y)/ul + math.Abs(v.X)/vl
		signPenalty := 0.0
		if u.X < 0 {
			signPenalty += 0.8
		}
		if v.Y < 0 {
			signPenalty += 0.8
		}
		fit := math.Exp(-2.8 * (offAxis + signPenalty))
		if fit > best {
			best = fit
		}
	}
	return clampUnit(best)
}

func ensureDiagnosticCanonicalAnchor(plane *diagnosticPlane, region diagnosticRect, pool, all []diagnosticBasisCandidate) []diagnosticBasisCandidate {
	var anchor diagnosticBasisCandidate
	found := false
	for _, candidate := range all {
		period := math.Hypot(candidate.u.X, candidate.u.Y)
		angle := normalizeDiagnosticAngle(math.Atan2(candidate.u.Y, candidate.u.X) * 180 / math.Pi)
		if math.Abs(period-8) < 0.1 && math.Abs(angle) < 0.1 {
			anchor = candidate
			found = true
			break
		}
	}
	if !found {
		return pool
	}
	bestRepetition := 0.0
	for py := 0; py < diagnosticFinePhaseDivisions; py++ {
		for px := 0; px < diagnosticFinePhaseDivisions; px++ {
			repetition, ok := diagnosticRepetitionScore(plane, region, anchor.u, anchor.v,
				float64(px)/float64(diagnosticFinePhaseDivisions),
				float64(py)/float64(diagnosticFinePhaseDivisions))
			if ok && repetition > bestRepetition {
				bestRepetition = repetition
			}
		}
	}
	if bestRepetition < 0.80 {
		return pool
	}
	for _, candidate := range pool {
		period := math.Hypot(candidate.u.X, candidate.u.Y)
		angle := normalizeDiagnosticAngle(math.Atan2(candidate.u.Y, candidate.u.X) * 180 / math.Pi)
		if math.Abs(period-8) < 0.1 && math.Abs(angle) < 0.1 {
			return pool
		}
	}
	return append(pool, anchor)
}

func applyDiagnosticRepetitionQuick(plane *diagnosticPlane, region diagnosticRect, candidate *diagnosticBasisCandidate) {
	repetition, ok := diagnosticRepetitionScore(plane, region, candidate.u, candidate.v, candidate.phaseU, candidate.phaseV)
	if !ok {
		candidate.quality = clampUnit(0.7 * candidate.coherence)
		return
	}
	candidate.repetitionCoherence = clampUnit((repetition - 0.5) * 2)
	candidate.quality = clampUnit(0.40*candidate.coherence + 0.60*candidate.repetitionCoherence)
}

func applyDiagnosticRepetition(plane *diagnosticPlane, region diagnosticRect, candidate *diagnosticBasisCandidate) {
	bestRepetition := 0.0
	bestPhaseU, bestPhaseV := candidate.phaseU, candidate.phaseV
	found := false
	for py := 0; py < diagnosticFinePhaseDivisions; py++ {
		for px := 0; px < diagnosticFinePhaseDivisions; px++ {
			phaseU := float64(px) / float64(diagnosticFinePhaseDivisions)
			phaseV := float64(py) / float64(diagnosticFinePhaseDivisions)
			repetition, ok := diagnosticRepetitionScore(plane, region, candidate.u, candidate.v, phaseU, phaseV)
			if !ok {
				continue
			}
			if !found || repetition > bestRepetition {
				found = true
				bestRepetition = repetition
				bestPhaseU, bestPhaseV = phaseU, phaseV
			}
		}
	}
	if !found {
		candidate.repetitionCoherence = 0
		candidate.quality = clampUnit(0.55 * candidate.coherence)
		return
	}
	candidate.phaseU, candidate.phaseV = bestPhaseU, bestPhaseV
	candidate.repetitionCoherence = clampUnit((bestRepetition - 0.5) * 2)
	candidate.coherence = clampUnit(0.35*candidate.coherence + 0.65*candidate.repetitionCoherence)
	candidate.quality = clampUnit(0.25*candidate.coherence + 0.75*candidate.repetitionCoherence)
}

func selectDistinctDiagnosticCandidates(candidates []diagnosticBasisCandidate, count int) []diagnosticBasisCandidate {
	selected := make([]diagnosticBasisCandidate, 0, count)
	for _, candidate := range candidates {
		period := math.Hypot(candidate.u.X, candidate.u.Y)
		angle := math.Atan2(candidate.u.Y, candidate.u.X) * 180 / math.Pi
		distinct := true
		for _, existing := range selected {
			existingPeriod := math.Hypot(existing.u.X, existing.u.Y)
			existingAngle := math.Atan2(existing.u.Y, existing.u.X) * 180 / math.Pi
			if math.Abs(period-existingPeriod) < 1.0 && math.Abs(angle-existingAngle) < 6.0 {
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

func scoreDiagnosticBasisPhase(plane *diagnosticPlane, region diagnosticRect, u, v LatticeVector, phaseDivisions int) diagnosticBasisCandidate {
	phaseScores := make([]float64, 0, phaseDivisions*phaseDivisions)
	bestScore := 0.0
	bestPhaseU, bestPhaseV := 0.0, 0.0
	for py := 0; py < phaseDivisions; py++ {
		for px := 0; px < phaseDivisions; px++ {
			phaseU := float64(px) / float64(phaseDivisions)
			phaseV := float64(py) / float64(phaseDivisions)
			score, ok := diagnosticPhaseScore(plane, region, u, v, phaseU, phaseV)
			if !ok {
				continue
			}
			phaseScores = append(phaseScores, score)
			if len(phaseScores) == 1 || score > bestScore {
				bestScore, bestPhaseU, bestPhaseV = score, phaseU, phaseV
			}
		}
	}
	if len(phaseScores) == 0 {
		return diagnosticBasisCandidate{u: u, v: v}
	}
	sort.Float64s(phaseScores)
	median := phaseScores[len(phaseScores)/2]
	contrast := math.Max(0, bestScore-median)
	coherence := clampUnit(contrast / (bestScore + 8.0))
	return diagnosticBasisCandidate{
		u: u, v: v,
		phaseU: bestPhaseU, phaseV: bestPhaseV,
		bestMargin: bestScore, contrast: contrast, coherence: coherence, quality: coherence,
	}
}

func diagnosticRepetitionScore(plane *diagnosticPlane, region diagnosticRect, u, v LatticeVector, phaseU, phaseV float64) (float64, bool) {
	determinant := u.X*v.Y - u.Y*v.X
	if math.Abs(determinant) < 1e-6 {
		return 0, false
	}
	anchorX := phaseU*u.X + phaseV*v.X
	anchorY := phaseU*u.Y + phaseV*v.Y
	centerX := float64(region.x) + float64(region.width)/2
	centerY := float64(region.y) + float64(region.height)/2
	dx := centerX - anchorX
	dy := centerY - anchorY
	centerI := (dx*v.Y - dy*v.X) / determinant
	centerJ := (u.X*dy - u.Y*dx) / determinant
	full := diagnosticRect{x: 0, y: 0, width: plane.width, height: plane.height}

	bestScore := 0.0
	bestTotal := 0
	compare := func(horizontal bool, shiftI, shiftJ float64) (float64, int) {
		i0 := math.Round(centerI - float64(tileWidth)/2 + shiftI)
		j0 := math.Round(centerJ - float64(tileHeight)/2 + shiftJ)
		deltaI, deltaJ := 0.0, 0.0
		if horizontal {
			i0 -= float64(tileWidth) / 2
			deltaI = float64(tileWidth)
		} else {
			j0 -= float64(tileHeight) / 2
			deltaJ = float64(tileHeight)
		}
		same, total := 0, 0
		for logical := 0; logical < tileWidth*tileHeight; logical += 23 {
			x := float64(logical % tileWidth)
			y := float64(logical / tileWidth)
			ax := anchorX + (i0+x)*u.X + (j0+y)*v.X
			ay := anchorY + (i0+x)*u.Y + (j0+y)*v.Y
			bx := anchorX + (i0+x+deltaI)*u.X + (j0+y+deltaJ)*v.X
			by := anchorY + (i0+x+deltaI)*u.Y + (j0+y+deltaJ)*v.Y
			a, okA := readDiagnosticBasisBlock(plane, full, ax, ay, u, v)
			b, okB := readDiagnosticBasisBlock(plane, full, bx, by, u, v)
			if !okA || !okB {
				continue
			}
			total++
			if (a >= 0) == (b >= 0) {
				same++
			}
		}
		if total == 0 {
			return 0, 0
		}
		return float64(same) / float64(total), total
	}

	// A few deterministic lattice-coordinate translations keep corner regions
	// measurable without expanding into an unbounded placement search.
	for _, shift := range [...]struct{ i, j float64 }{{0, 0}, {-8, 0}, {8, 0}, {0, -8}, {0, 8}} {
		shiftI, shiftJ := shift.i, shift.j
		for _, horizontal := range [...]bool{true, false} {
			score, total := compare(horizontal, shiftI, shiftJ)
			if total < 18 {
				continue
			}
			if total > bestTotal || (total == bestTotal && score > bestScore) {
				bestScore, bestTotal = score, total
			}
		}
	}
	if bestTotal < 18 {
		return 0, false
	}
	return bestScore, true
}

func diagnosticPhaseScore(plane *diagnosticPlane, region diagnosticRect, u, v LatticeVector, phaseU, phaseV float64) (float64, bool) {
	determinant := u.X*v.Y - u.Y*v.X
	if math.Abs(determinant) < 1e-6 {
		return 0, false
	}
	anchorX := phaseU*u.X + phaseV*v.X
	anchorY := phaseU*u.Y + phaseV*v.Y
	full := diagnosticRect{x: 0, y: 0, width: plane.width, height: plane.height}
	sum := 0.0
	count := 0
	denominator := diagnosticSampleBlocks + 1
	for sampleY := 1; sampleY <= diagnosticSampleBlocks; sampleY++ {
		for sampleX := 1; sampleX <= diagnosticSampleBlocks; sampleX++ {
			targetX := float64(region.x) + float64(sampleX*region.width)/float64(denominator)
			targetY := float64(region.y) + float64(sampleY*region.height)/float64(denominator)
			dx := targetX - anchorX
			dy := targetY - anchorY
			i := math.Round((dx*v.Y - dy*v.X) / determinant)
			j := math.Round((u.X*dy - u.Y*dx) / determinant)
			originX := anchorX + i*u.X + j*v.X
			originY := anchorY + i*u.Y + j*v.Y
			margin, ok := readDiagnosticBasisBlock(plane, full, originX, originY, u, v)
			if !ok {
				continue
			}
			sum += math.Abs(margin)
			count++
		}
	}
	if count < diagnosticSampleBlocks*diagnosticSampleBlocks/2 {
		return 0, false
	}
	return sum / float64(count), true
}

func readDiagnosticBasisBlock(plane *diagnosticPlane, region diagnosticRect, originX, originY float64, u, v LatticeVector) (float64, bool) {
	coefficient23, coefficient32 := 0.0, 0.0
	for y := 0; y < blockSize; y++ {
		for x := 0; x < blockSize; x++ {
			fx := float64(x) / float64(blockSize)
			fy := float64(y) / float64(blockSize)
			sourceX := originX + fx*u.X + fy*v.X
			sourceY := originY + fx*u.Y + fy*v.Y
			if sourceX < float64(region.x) || sourceY < float64(region.y) || sourceX >= float64(region.x+region.width-1) || sourceY >= float64(region.y+region.height-1) {
				return 0, false
			}
			luminance, ok := sampleDiagnosticLuminance(plane, sourceX, sourceY)
			if !ok {
				return 0, false
			}
			coefficient23 += luminance * cosTable[3][x] * cosTable[2][y]
			coefficient32 += luminance * cosTable[2][x] * cosTable[3][y]
		}
	}
	return math.Abs(coefficient23) - math.Abs(coefficient32), true
}

func sampleDiagnosticLuminance(plane *diagnosticPlane, x, y float64) (float64, bool) {
	if x < 0 || y < 0 || x > float64(plane.width-1) || y > float64(plane.height-1) {
		return 0, false
	}
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	x1, y1 := x0+1, y0+1
	if x1 >= plane.width {
		x1 = plane.width - 1
	}
	if y1 >= plane.height {
		y1 = plane.height - 1
	}
	fx, fy := x-float64(x0), y-float64(y0)
	v00 := float64(plane.luma[y0*plane.width+x0])
	v10 := float64(plane.luma[y0*plane.width+x1])
	v01 := float64(plane.luma[y1*plane.width+x0])
	v11 := float64(plane.luma[y1*plane.width+x1])
	top := v00*(1-fx) + v10*fx
	bottom := v01*(1-fx) + v11*fx
	return top*(1-fy) + bottom*fy, true
}

func diagnosticCandidateToEstimate(candidate diagnosticBasisCandidate, plane *diagnosticPlane, region diagnosticRect, rx, ry int) LocalLatticeEstimate {
	divisor := float64(plane.divisor)
	u := LatticeVector{X: candidate.u.X * divisor, Y: candidate.u.Y * divisor}
	v := LatticeVector{X: candidate.v.X * divisor, Y: candidate.v.Y * divisor}
	periodU := math.Hypot(u.X, u.Y)
	periodV := math.Hypot(v.X, v.Y)
	orientation := normalizeDiagnosticAngle(math.Atan2(u.Y, u.X) * 180 / math.Pi)
	interAxis := vectorAngleDegrees(candidate.u, candidate.v)
	return LocalLatticeEstimate{
		RegionX: rx, RegionY: ry,
		NativeX: region.x * plane.divisor, NativeY: region.y * plane.divisor,
		NativeWidth: region.width * plane.divisor, NativeHeight: region.height * plane.divisor,
		AnalysisDivisor: plane.divisor,
		U:               u, V: v,
		PeriodU: periodU, PeriodV: periodV,
		OrientationDegrees: orientation, InterAxisDegrees: interAxis,
		PhaseU: candidate.phaseU, PhaseV: candidate.phaseV,
		DCTDifferentialMargin:   candidate.bestMargin,
		PeriodicCoherence:       clampUnit(candidate.coherence),
		TileRepetitionCoherence: clampUnit(candidate.repetitionCoherence),
		Confidence:              clampUnit(candidate.quality),
		CandidatesEvaluated:     candidate.candidatesEvaluated,
	}
}

type diagnosticConsensusCandidate struct {
	original     LocalLatticeEstimate
	comparable   LocalLatticeEstimate
	boundaryFit  float64
	levelSupport float64
}

func selectDiagnosticConsensus(pools [][]LocalLatticeEstimate, boundary PrintBoundaryEstimate, useBoundary bool, regionsX, regionsY int) ([]LocalLatticeEstimate, LatticeVector, LatticeVector, float64, int, bool) {
	working := make([][]diagnosticConsensusCandidate, len(pools))
	for poolIndex, pool := range pools {
		working[poolIndex] = make([]diagnosticConsensusCandidate, 0, len(pool))
		multipleLevels := false
		if len(pool) > 1 {
			firstDivisor := pool[0].AnalysisDivisor
			for _, candidate := range pool[1:] {
				if candidate.AnalysisDivisor != firstDivisor {
					multipleLevels = true
					break
				}
			}
		}
		for _, candidate := range pool {
			original, comparable, fit := candidate, candidate, 1.0
			if useBoundary {
				var ok bool
				original, comparable, fit, ok = normalizeDiagnosticEstimateByBoundary(boundary, candidate, regionsX, regionsY)
				if !ok {
					fit = 0.15
					original, comparable = candidate, candidate
				}
			}
			levelSupport := 1.0
			if multipleLevels {
				levelSupport = 0.15
				for _, other := range pool {
					if other.AnalysisDivisor == candidate.AnalysisDivisor {
						continue
					}
					distance := diagnosticBasisDistance(candidate, other)
					support := math.Exp(-1.25 * distance)
					if support > levelSupport {
						levelSupport = support
					}
				}
			}
			working[poolIndex] = append(working[poolIndex], diagnosticConsensusCandidate{original: original, comparable: comparable, boundaryFit: fit, levelSupport: levelSupport})
		}
	}

	var bestReference diagnosticConsensusCandidate
	bestScore := -1.0
	bestMatches := -1
	for _, pool := range working {
		for _, reference := range pool {
			score := 0.0
			matches := 0
			for _, otherPool := range working {
				bestLocal := 0.0
				matched := false
				for _, candidate := range otherPool {
					distance := diagnosticBasisDistance(reference.comparable, candidate.comparable)
					compatibility := math.Exp(-1.15 * distance)
					prior := 1.0
					if useBoundary {
						prior = 0.20 + 0.80*candidate.boundaryFit
					}
					prior *= 0.25 + 0.75*candidate.levelSupport
					value := candidate.original.Confidence * compatibility * prior
					if value > bestLocal {
						bestLocal = value
					}
					if distance <= 1.0 && candidate.original.Confidence >= 0.12 && (!useBoundary || candidate.boundaryFit >= 0.30) {
						matched = true
					}
				}
				if matched {
					matches++
				}
				score += bestLocal
			}
			if useBoundary {
				score *= 0.35 + 0.65*reference.boundaryFit
			}
			score *= 0.35 + 0.65*reference.levelSupport
			// Prefer broad agreement first, then intrinsic weighted support.
			if matches > bestMatches || (matches == bestMatches && score > bestScore) {
				bestMatches, bestScore, bestReference = matches, score, reference
			}
		}
	}
	if bestMatches < 0 {
		return nil, LatticeVector{}, LatticeVector{}, 0, 0, false
	}

	selected := make([]LocalLatticeEstimate, 0, len(working))
	selectedComparable := make([]LocalLatticeEstimate, 0, len(working))
	for _, pool := range working {
		bestIndex := -1
		bestValue := -1.0
		var bestOriginal, bestComparable LocalLatticeEstimate
		for index, candidate := range pool {
			alignedComparable, distance := alignDiagnosticEstimate(bestReference.comparable, candidate.comparable)
			prior := 1.0
			if useBoundary {
				prior = 0.20 + 0.80*candidate.boundaryFit
			}
			prior *= 0.25 + 0.75*candidate.levelSupport
			value := (candidate.original.Confidence*math.Exp(-1.35*distance) + 0.08*math.Exp(-2.0*distance)) * prior
			if value > bestValue {
				bestValue, bestIndex = value, index
				bestOriginal = candidate.original
				bestComparable = alignedComparable
			}
		}
		if bestIndex >= 0 {
			selected = append(selected, bestOriginal)
			selectedComparable = append(selectedComparable, bestComparable)
		}
	}
	globalU, globalV, _, _ := summarizeDiagnosticRegions(selected)
	consistency := diagnosticNormalizedConsistency(selectedComparable)
	strong := 0
	for index, region := range selectedComparable {
		if diagnosticBasisDistance(bestReference.comparable, region) <= 1.0 && selected[index].Confidence >= 0.12 {
			strong++
		}
	}
	minimumStrong := (len(pools) + 1) / 2
	evidenceThreshold := 0.55
	if useBoundary {
		evidenceThreshold = 0.62
	}
	evidence := strong >= minimumStrong && consistency >= evidenceThreshold
	return selected, globalU, globalV, consistency, strong, evidence
}

func normalizeDiagnosticEstimateByBoundary(boundary PrintBoundaryEstimate, candidate LocalLatticeEstimate, regionsX, regionsY int) (LocalLatticeEstimate, LocalLatticeEstimate, float64, bool) {
	pageU, pageV, ok := boundaryExpectedTangents(boundary, candidate.RegionX, candidate.RegionY, regionsX, regionsY)
	if !ok {
		return candidate, candidate, 0, false
	}
	determinant := pageU.X*pageV.Y - pageU.Y*pageV.X
	if math.Abs(determinant) < 1e-9 {
		return candidate, candidate, 0, false
	}
	transform := func(vector LatticeVector) LatticeVector {
		return LatticeVector{
			X: (vector.X*pageV.Y - vector.Y*pageV.X) / determinant,
			Y: (pageU.X*vector.Y - pageU.Y*vector.X) / determinant,
		}
	}

	bestFit := -1.0
	bestOriginal, bestComparable := candidate, candidate
	for quarter := 0; quarter < 4; quarter++ {
		original := diagnosticQuarterEquivalent(candidate, quarter)
		u := transform(original.U)
		v := transform(original.V)
		ul, vl := math.Hypot(u.X, u.Y), math.Hypot(v.X, v.Y)
		if ul < 1e-12 || vl < 1e-12 {
			continue
		}
		offAxis := math.Abs(u.Y)/ul + math.Abs(v.X)/vl
		signPenalty := 0.0
		if u.X < 0 {
			signPenalty += 0.8
		}
		if v.Y < 0 {
			signPenalty += 0.8
		}
		fit := math.Exp(-2.8 * (offAxis + signPenalty))
		if fit <= bestFit {
			continue
		}
		comparable := original
		comparable.U, comparable.V = u, v
		comparable.PeriodU, comparable.PeriodV = ul, vl
		comparable.OrientationDegrees = math.Atan2(u.Y, u.X) * 180 / math.Pi
		comparable.InterAxisDegrees = vectorAngleDegrees(u, v)
		bestFit, bestOriginal, bestComparable = fit, original, comparable
	}
	if bestFit < 0 {
		return candidate, candidate, 0, false
	}
	return bestOriginal, bestComparable, clampUnit(bestFit), true
}

func diagnosticNormalizedConsistency(regions []LocalLatticeEstimate) float64 {
	if len(regions) == 0 {
		return 0
	}
	var sumU, sumV LatticeVector
	totalWeight := 0.0
	for _, region := range regions {
		weight := region.Confidence
		if weight < 0.12 {
			continue
		}
		sumU.X += region.U.X * weight
		sumU.Y += region.U.Y * weight
		sumV.X += region.V.X * weight
		sumV.Y += region.V.Y * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return 0
	}
	meanU := LatticeVector{X: sumU.X / totalWeight, Y: sumU.Y / totalWeight}
	meanV := LatticeVector{X: sumV.X / totalWeight, Y: sumV.Y / totalWeight}
	meanULength := math.Max(math.Hypot(meanU.X, meanU.Y), 1e-9)
	meanVLength := math.Max(math.Hypot(meanV.X, meanV.Y), 1e-9)
	deviation, deviationWeight := 0.0, 0.0
	for _, region := range regions {
		weight := region.Confidence
		if weight < 0.12 {
			continue
		}
		du := math.Hypot(region.U.X-meanU.X, region.U.Y-meanU.Y) / meanULength
		dv := math.Hypot(region.V.X-meanV.X, region.V.Y-meanV.Y) / meanVLength
		deviation += weight * (du + dv) / 2
		deviationWeight += weight
	}
	if deviationWeight > 0 {
		deviation /= deviationWeight
	}
	return clampUnit(math.Exp(-2.5 * deviation))
}

func diagnosticBasisDistance(a, b LocalLatticeEstimate) float64 {
	_, distance := alignDiagnosticEstimate(a, b)
	return distance
}

func alignDiagnosticEstimate(reference, candidate LocalLatticeEstimate) (LocalLatticeEstimate, float64) {
	best := candidate
	bestDistance := math.Inf(1)
	for quarter := 0; quarter < 4; quarter++ {
		variant := diagnosticQuarterEquivalent(candidate, quarter)
		distance := diagnosticBasisDistanceAligned(reference, variant)
		if distance < bestDistance {
			best, bestDistance = variant, distance
		}
	}
	return best, bestDistance
}

func diagnosticQuarterEquivalent(candidate LocalLatticeEstimate, quarter int) LocalLatticeEstimate {
	result := candidate
	u, v := candidate.U, candidate.V
	switch quarter & 3 {
	case 0:
	case 1:
		result.U = v
		result.V = LatticeVector{X: -u.X, Y: -u.Y}
		result.PhaseU, result.PhaseV = candidate.PhaseV, 1-candidate.PhaseU
	case 2:
		result.U = LatticeVector{X: -u.X, Y: -u.Y}
		result.V = LatticeVector{X: -v.X, Y: -v.Y}
		result.PhaseU, result.PhaseV = 1-candidate.PhaseU, 1-candidate.PhaseV
	case 3:
		result.U = LatticeVector{X: -v.X, Y: -v.Y}
		result.V = u
		result.PhaseU, result.PhaseV = 1-candidate.PhaseV, candidate.PhaseU
	}
	result.PeriodU = math.Hypot(result.U.X, result.U.Y)
	result.PeriodV = math.Hypot(result.V.X, result.V.Y)
	result.OrientationDegrees = math.Atan2(result.U.Y, result.U.X) * 180 / math.Pi
	result.InterAxisDegrees = vectorAngleDegrees(result.U, result.V)
	return result
}

func diagnosticBasisDistanceAligned(a, b LocalLatticeEstimate) float64 {
	aPeriodU := math.Max(a.PeriodU, 1e-6)
	aPeriodV := math.Max(a.PeriodV, 1e-6)
	bPeriodU := math.Max(b.PeriodU, 1e-6)
	bPeriodV := math.Max(b.PeriodV, 1e-6)
	periodDistance := (math.Abs(math.Log(bPeriodU/aPeriodU)) + math.Abs(math.Log(bPeriodV/aPeriodV))) / (2 * math.Log(1.45))
	angleDelta := b.OrientationDegrees - a.OrientationDegrees
	for angleDelta <= -180 {
		angleDelta += 360
	}
	for angleDelta > 180 {
		angleDelta -= 360
	}
	angleDistance := math.Abs(angleDelta) / 18.0
	axisDistance := math.Abs(b.InterAxisDegrees-a.InterAxisDegrees) / 12.0
	return 0.48*periodDistance + 0.38*angleDistance + 0.14*axisDistance
}

func summarizeDiagnosticRegions(regions []LocalLatticeEstimate) (LatticeVector, LatticeVector, float64, bool) {
	var sumU, sumV LatticeVector
	totalWeight := 0.0
	strong := 0
	for _, region := range regions {
		weight := region.Confidence
		if weight < 0.12 {
			continue
		}
		sumU.X += region.U.X * weight
		sumU.Y += region.U.Y * weight
		sumV.X += region.V.X * weight
		sumV.Y += region.V.Y * weight
		totalWeight += weight
		if region.Confidence >= 0.22 {
			strong++
		}
	}
	if totalWeight == 0 {
		return LatticeVector{}, LatticeVector{}, 0, false
	}
	meanU := LatticeVector{X: sumU.X / totalWeight, Y: sumU.Y / totalWeight}
	meanV := LatticeVector{X: sumV.X / totalWeight, Y: sumV.Y / totalWeight}
	meanULength := math.Max(math.Hypot(meanU.X, meanU.Y), 1)
	meanVLength := math.Max(math.Hypot(meanV.X, meanV.Y), 1)
	deviation, deviationWeight := 0.0, 0.0
	for _, region := range regions {
		weight := region.Confidence
		if weight < 0.12 {
			continue
		}
		du := math.Hypot(region.U.X-meanU.X, region.U.Y-meanU.Y) / meanULength
		dv := math.Hypot(region.V.X-meanV.X, region.V.Y-meanV.Y) / meanVLength
		deviation += weight * (du + dv) / 2
		deviationWeight += weight
	}
	if deviationWeight > 0 {
		deviation /= deviationWeight
	}
	consistency := math.Exp(-2.5 * deviation)
	minimumStrong := (len(regions) + 2) / 3
	return meanU, meanV, clampUnit(consistency), strong >= minimumStrong && consistency >= 0.45
}

func vectorAngleDegrees(a, b LatticeVector) float64 {
	denominator := math.Hypot(a.X, a.Y) * math.Hypot(b.X, b.Y)
	if denominator == 0 {
		return 0
	}
	cosine := (a.X*b.X + a.Y*b.Y) / denominator
	if cosine < -1 {
		cosine = -1
	}
	if cosine > 1 {
		cosine = 1
	}
	return math.Acos(cosine) * 180 / math.Pi
}

func normalizeDiagnosticAngle(angle float64) float64 {
	for angle <= -45 {
		angle += 90
	}
	for angle > 45 {
		angle -= 90
	}
	return angle
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
