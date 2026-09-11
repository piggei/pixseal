package watermark

import (
	"image"
	"math"
	"sort"
	"time"
)

const (
	diagnosticMaxProjectiveScaleCandidates = 8
	diagnosticMaxProjectiveFullDecodes     = 4
	diagnosticSyncRepeatsPerAxis           = 4
	diagnosticWeakBoundaryMinConfidence    = 0.20
	diagnosticMaxFullGridBlocks            = 50000
	diagnosticMaxSampledTilesPerAxis       = 5
	diagnosticMaxSampledTiles              = diagnosticMaxSampledTilesPerAxis * diagnosticMaxSampledTilesPerAxis
)

// DiagnosticScaleCandidate is one bounded estimate of the original digital
// carrier geometry inferred from the local lattice field after normalization by
// the observed print boundary. It is research evidence, not authentication.
type DiagnosticScaleCandidate struct {
	CanonicalWidthPixels  float64 `json:"canonical_width_px"`
	CanonicalHeightPixels float64 `json:"canonical_height_px"`
	Confidence            float64 `json:"confidence"`
	SupportingRegions     int     `json:"supporting_regions"`
	SupportingLevels      int     `json:"supporting_levels"`
	CrossLevelRegions     int     `json:"cross_level_regions"`
	Fundamental           bool    `json:"fundamental"`
}

// DiagnosticProjectiveEstimate is the v0.3 coarse projective initializer. The
// matrix maps canonical carrier pixels to native source-image coordinates. A
// model can be available even when no PixSeal watermark is present; only HMAC
// authentication can establish a recovered payload.
type DiagnosticProjectiveEstimate struct {
	Available             bool                       `json:"available"`
	Source                string                     `json:"source,omitempty"`
	Confidence            float64                    `json:"confidence"`
	CanonicalWidthPixels  float64                    `json:"canonical_width_px,omitempty"`
	CanonicalHeightPixels float64                    `json:"canonical_height_px,omitempty"`
	Matrix                [9]float64                 `json:"matrix,omitempty"`
	ScaleCandidates       []DiagnosticScaleCandidate `json:"scale_candidates,omitempty"`
	LocalProjectiveFit    float64                    `json:"local_projective_fit"`
	BoundaryConfidence    float64                    `json:"boundary_confidence"`
	CandidateBudget       int                        `json:"candidate_budget"`
	localRegions          []LocalLatticeEstimate
}

type diagnosticScaleProposal struct {
	width, height float64
	score         float64
	region        int
	divisor       int
}

type diagnosticScaleCluster struct {
	widthSum, heightSum, weight float64
	bestByRegion                map[int]float64
	levels                      map[int]struct{}
	levelsByRegion              map[int]map[int]struct{}
}

func estimateDiagnosticProjective(boundary PrintBoundaryEstimate, pools [][]LocalLatticeEstimate, selected []LocalLatticeEstimate, globalConsistency float64, regionsX, regionsY int) DiagnosticProjectiveEstimate {
	return estimateDiagnosticProjectiveWithBoundary(boundary, pools, selected, globalConsistency, regionsX, regionsY, "print-boundary+lattice", true)
}

func estimateDiagnosticProjectiveWeakBoundary(boundary PrintBoundaryEstimate, pools [][]LocalLatticeEstimate, selected []LocalLatticeEstimate, globalConsistency float64, regionsX, regionsY int) DiagnosticProjectiveEstimate {
	return estimateDiagnosticProjectiveWithBoundary(boundary, pools, selected, globalConsistency, regionsX, regionsY, "lattice+weak-boundary", false)
}

func estimateDiagnosticProjectiveWithBoundary(boundary PrintBoundaryEstimate, pools [][]LocalLatticeEstimate, selected []LocalLatticeEstimate, globalConsistency float64, regionsX, regionsY int, source string, requireStrongBoundary bool) DiagnosticProjectiveEstimate {
	result := DiagnosticProjectiveEstimate{
		Source:             source,
		BoundaryConfidence: boundary.Confidence,
		CandidateBudget:    diagnosticMaxProjectiveScaleCandidates,
		localRegions:       append([]LocalLatticeEstimate(nil), selected...),
	}
	if requireStrongBoundary {
		if !boundary.Detected || boundary.Confidence < 0.35 {
			return result
		}
	} else if boundary.Confidence < diagnosticWeakBoundaryMinConfidence {
		return result
	}

	proposals := make([]diagnosticScaleProposal, 0, len(pools)*diagnosticFinalPool)
	for poolIndex, pool := range pools {
		for _, candidate := range pool {
			_, comparable, fit, ok := normalizeDiagnosticEstimateByBoundary(boundary, candidate, regionsX, regionsY)
			if !ok || fit < 0.25 {
				continue
			}
			// In boundary-normalized coordinates, U.X and V.Y are fractions of
			// the full printed width/height occupied by one 8-pixel v3 block.
			// Off-axis components are already represented in fit and are not
			// silently forced to zero here.
			ux := math.Abs(comparable.U.X)
			vy := math.Abs(comparable.V.Y)
			if ux < 1e-6 || vy < 1e-6 {
				continue
			}
			width := float64(blockSize) / ux
			height := float64(blockSize) / vy
			if width < float64(tileWidth*blockSize) || height < float64(tileHeight*blockSize) || width > 12000 || height > 12000 {
				continue
			}
			score := candidate.Confidence * (0.20 + 0.80*fit)
			if score < 0.04 {
				continue
			}
			proposals = append(proposals, diagnosticScaleProposal{
				width: width, height: height, score: score,
				region: poolIndex, divisor: candidate.AnalysisDivisor,
			})
		}
	}

	clusters := clusterDiagnosticScales(proposals)
	if len(clusters) > diagnosticMaxProjectiveScaleCandidates {
		clusters = clusters[:diagnosticMaxProjectiveScaleCandidates]
	}
	result.ScaleCandidates = clusters
	if len(clusters) == 0 {
		return result
	}

	best := clusters[0]
	for _, candidate := range clusters {
		if candidate.Fundamental {
			best = candidate
			break
		}
	}
	result.CanonicalWidthPixels = best.CanonicalWidthPixels
	result.CanonicalHeightPixels = best.CanonicalHeightPixels
	if h, ok := homographyForPrintBoundary(best.CanonicalWidthPixels, best.CanonicalHeightPixels, boundary); ok {
		result.Matrix = h.h
		result.Available = true
	}

	result.LocalProjectiveFit = diagnosticSelectedProjectiveFit(boundary, selected, regionsX, regionsY)
	boundaryWeight := boundary.Confidence
	if !requireStrongBoundary {
		// Weak boundaries are admitted only after independent lattice evidence.
		// Keep their low confidence visible but avoid squaring the detector's
		// explicit 0.5 non-detection penalty into a near-zero model score.
		boundaryWeight = math.Max(boundaryWeight, 0.20)
	}
	result.Confidence = clampUnit(boundaryWeight * (0.30 + 0.70*globalConsistency) * (0.35 + 0.65*best.Confidence) * (0.40 + 0.60*result.LocalProjectiveFit))
	return result
}

func diagnosticBoundaryCandidateUsable(boundary PrintBoundaryEstimate, width, height int) bool {
	if width <= 0 || height <= 0 || boundary.Confidence < diagnosticWeakBoundaryMinConfidence {
		return false
	}
	points := [...]ImagePoint{boundary.TopLeft, boundary.TopRight, boundary.BottomRight, boundary.BottomLeft}
	for _, p := range points {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || math.IsInf(p.X, 0) || math.IsInf(p.Y, 0) {
			return false
		}
		if p.X < -0.05*float64(width) || p.X > 1.05*float64(width) || p.Y < -0.05*float64(height) || p.Y > 1.05*float64(height) {
			return false
		}
	}
	area := math.Abs(polygonArea4(boundary.TopLeft, boundary.TopRight, boundary.BottomRight, boundary.BottomLeft))
	areaFraction := area / float64(width*height)
	if areaFraction < 0.25 || areaFraction > 1.05 {
		return false
	}
	edges := [...]float64{
		math.Hypot(boundary.TopRight.X-boundary.TopLeft.X, boundary.TopRight.Y-boundary.TopLeft.Y),
		math.Hypot(boundary.BottomRight.X-boundary.TopRight.X, boundary.BottomRight.Y-boundary.TopRight.Y),
		math.Hypot(boundary.BottomLeft.X-boundary.BottomRight.X, boundary.BottomLeft.Y-boundary.BottomRight.Y),
		math.Hypot(boundary.TopLeft.X-boundary.BottomLeft.X, boundary.TopLeft.Y-boundary.BottomLeft.Y),
	}
	minimum := 0.20 * math.Min(float64(width), float64(height))
	for _, edge := range edges {
		if edge < minimum {
			return false
		}
	}
	return true
}

func clusterDiagnosticScales(proposals []diagnosticScaleProposal) []DiagnosticScaleCandidate {
	if len(proposals) == 0 {
		return nil
	}
	// Five-percent logarithmic bins are deterministic and scale-relative: they
	// do not encode any knowledge of the two private print-camera photographs.
	const base = 1.05
	type key struct{ w, h int }
	clusters := make(map[key]*diagnosticScaleCluster)
	for _, proposal := range proposals {
		k := key{
			w: int(math.Round(math.Log(proposal.width) / math.Log(base))),
			h: int(math.Round(math.Log(proposal.height) / math.Log(base))),
		}
		cluster := clusters[k]
		if cluster == nil {
			cluster = &diagnosticScaleCluster{
				bestByRegion:   make(map[int]float64),
				levels:         make(map[int]struct{}),
				levelsByRegion: make(map[int]map[int]struct{}),
			}
			clusters[k] = cluster
		}
		cluster.widthSum += proposal.width * proposal.score
		cluster.heightSum += proposal.height * proposal.score
		cluster.weight += proposal.score
		if proposal.score > cluster.bestByRegion[proposal.region] {
			cluster.bestByRegion[proposal.region] = proposal.score
		}
		cluster.levels[proposal.divisor] = struct{}{}
		regionLevels := cluster.levelsByRegion[proposal.region]
		if regionLevels == nil {
			regionLevels = make(map[int]struct{})
			cluster.levelsByRegion[proposal.region] = regionLevels
		}
		regionLevels[proposal.divisor] = struct{}{}
	}

	result := make([]DiagnosticScaleCandidate, 0, len(clusters))
	for _, cluster := range clusters {
		if cluster.weight <= 0 {
			continue
		}
		support := 0.0
		for _, score := range cluster.bestByRegion {
			support += score
		}
		regionFactor := clampUnit(float64(len(cluster.bestByRegion)) / 5.0)
		levelFactor := 0.55
		if len(cluster.levels) >= 2 {
			levelFactor = 1.0
		}
		crossLevelRegions := 0
		for _, levels := range cluster.levelsByRegion {
			if len(levels) >= 2 {
				crossLevelRegions++
			}
		}
		confidence := clampUnit((support / 2.0) * (0.45 + 0.55*regionFactor) * levelFactor)
		result = append(result, DiagnosticScaleCandidate{
			CanonicalWidthPixels:  cluster.widthSum / cluster.weight,
			CanonicalHeightPixels: cluster.heightSum / cluster.weight,
			Confidence:            confidence,
			SupportingRegions:     len(cluster.bestByRegion),
			SupportingLevels:      len(cluster.levels),
			CrossLevelRegions:     crossLevelRegions,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SupportingRegions != result[j].SupportingRegions {
			return result[i].SupportingRegions > result[j].SupportingRegions
		}
		if result[i].SupportingLevels != result[j].SupportingLevels {
			return result[i].SupportingLevels > result[j].SupportingLevels
		}
		if result[i].CrossLevelRegions != result[j].CrossLevelRegions {
			return result[i].CrossLevelRegions > result[j].CrossLevelRegions
		}
		if result[i].Confidence != result[j].Confidence {
			return result[i].Confidence > result[j].Confidence
		}
		if result[i].CanonicalWidthPixels != result[j].CanonicalWidthPixels {
			return result[i].CanonicalWidthPixels < result[j].CanonicalWidthPixels
		}
		return result[i].CanonicalHeightPixels < result[j].CanonicalHeightPixels
	})
	markDiagnosticFundamental(result)
	return result
}

// markDiagnosticFundamental separates the lowest-frequency lattice family from
// higher-frequency aliases. A fundamental candidate must be seen at multiple
// pyramid levels and in multiple independent spatial regions. Among candidates
// with non-trivial support relative to the strongest multi-level cluster, the
// lowest spatial frequency (largest observed block pitch, equivalently smallest
// canonical carrier area) is retained. This rule is image-independent and does
// not encode dimensions from the private print-camera corpus.
func markDiagnosticFundamental(candidates []DiagnosticScaleCandidate) {
	bestConfidence := 0.0
	for _, candidate := range candidates {
		if candidate.SupportingLevels >= 2 && candidate.SupportingRegions >= 2 && candidate.Confidence > bestConfidence {
			bestConfidence = candidate.Confidence
		}
	}
	if bestConfidence <= 0 {
		return
	}
	threshold := 0.30 * bestConfidence
	best := -1
	bestArea := math.Inf(1)
	for i, candidate := range candidates {
		if candidate.SupportingLevels < 2 || candidate.SupportingRegions < 2 || candidate.Confidence < threshold {
			continue
		}
		area := candidate.CanonicalWidthPixels * candidate.CanonicalHeightPixels
		if area < bestArea || (area == bestArea && candidate.CrossLevelRegions > candidates[best].CrossLevelRegions) {
			best, bestArea = i, area
		}
	}
	if best >= 0 {
		candidates[best].Fundamental = true
	}
}

func diagnosticSelectedProjectiveFit(boundary PrintBoundaryEstimate, selected []LocalLatticeEstimate, regionsX, regionsY int) float64 {
	if len(selected) == 0 {
		return 0
	}
	total, weight := 0.0, 0.0
	for _, candidate := range selected {
		_, _, fit, ok := normalizeDiagnosticEstimateByBoundary(boundary, candidate, regionsX, regionsY)
		if !ok {
			continue
		}
		w := math.Max(candidate.Confidence, 0.05)
		total += fit * w
		weight += w
	}
	if weight == 0 {
		return 0
	}
	return clampUnit(total / weight)
}

func homographyForPrintBoundary(width, height float64, boundary PrintBoundaryEstimate) (homography, bool) {
	if width <= 1 || height <= 1 {
		return homography{}, false
	}
	src := [4][2]float64{{0, 0}, {width - 1, 0}, {0, height - 1}, {width - 1, height - 1}}
	dst := [4][2]float64{
		{boundary.TopLeft.X, boundary.TopLeft.Y},
		{boundary.TopRight.X, boundary.TopRight.Y},
		{boundary.BottomLeft.X, boundary.BottomLeft.Y},
		{boundary.BottomRight.X, boundary.BottomRight.Y},
	}
	var a [8][9]float64
	for i := 0; i < 4; i++ {
		x, y := src[i][0], src[i][1]
		u, v := dst[i][0], dst[i][1]
		a[2*i] = [9]float64{x, y, 1, 0, 0, 0, -u * x, -u * y, u}
		a[2*i+1] = [9]float64{0, 0, 0, x, y, 1, -v * x, -v * y, v}
	}
	s, ok := solveLinear8(a)
	if !ok {
		return homography{}, false
	}
	return homography{h: [9]float64{s[0], s[1], s[2], s[3], s[4], s[5], s[6], s[7], 1}}, true
}

// DiagnosticProjectiveAuthentication reports the bounded key-assisted oracle
// probe used only by diagnose. Sync evidence can rank geometry candidates, but
// it is never treated as a recovered watermark; AuthenticatedPayload still
// requires the ordinary Format v3 HMAC to pass.
type DiagnosticProjectiveAuthentication struct {
	CandidatesProbed                                   int                                      `json:"candidates_probed"`
	FullDecodeAttempts                                 int                                      `json:"full_decode_attempts"`
	ReliabilityDecodeAttempts                          int                                      `json:"reliability_decode_attempts"`
	FullGridBoundedUses                                int                                      `json:"full_grid_bounded_uses"`
	MaxFullGridSampledBlocks                           int                                      `json:"max_full_grid_sampled_blocks"`
	MaxFullGridSampledTiles                            int                                      `json:"max_full_grid_sampled_tiles"`
	CoarseProbeMilliseconds                            int64                                    `json:"coarse_probe_ms"`
	RefinementMilliseconds                             int64                                    `json:"refinement_ms"`
	PhotometricMilliseconds                            int64                                    `json:"photometric_ms"`
	FullDecodeMilliseconds                             int64                                    `json:"full_decode_ms"`
	ProbeResults                                       []DiagnosticProjectiveProbeEvidence      `json:"probe_results,omitempty"`
	RefinedResults                                     []DiagnosticProjectiveRefinementEvidence `json:"refined_results,omitempty"`
	PhaseRefinementAttempts                            int                                      `json:"phase_refinement_attempts"`
	PhaseHomographyFits                                int                                      `json:"phase_homography_fits"`
	ResidualWarpFits                                   int                                      `json:"residual_warp_fits"`
	BestResidualControls                               int                                      `json:"best_residual_controls"`
	BestResidualRMSPixels                              float64                                  `json:"best_residual_rms_px"`
	SubpixelRefinements                                int                                      `json:"subpixel_refinements"`
	SubpixelProbeAttempts                              int                                      `json:"subpixel_probe_attempts"`
	FundamentalScaleProbes                             int                                      `json:"fundamental_scale_probe_attempts"`
	BestFundamentalWidthPx                             float64                                  `json:"best_fundamental_width_px,omitempty"`
	BestFundamentalHeightPx                            float64                                  `json:"best_fundamental_height_px,omitempty"`
	BestFundamentalSyncZ                               float64                                  `json:"best_fundamental_sync_z_score"`
	BestSubpixelOffsetX                                float64                                  `json:"best_subpixel_offset_x"`
	BestSubpixelOffsetY                                float64                                  `json:"best_subpixel_offset_y"`
	BestSubpixelZScore                                 float64                                  `json:"best_subpixel_z_score"`
	BestSyncProfile                                    Profile                                  `json:"best_sync_profile,omitempty"`
	BestSyncFraction                                   float64                                  `json:"best_sync_fraction"`
	BestSyncZScore                                     float64                                  `json:"best_sync_z_score"`
	BestCanonicalWidthPx                               float64                                  `json:"best_canonical_width_px,omitempty"`
	BestCanonicalHeightPx                              float64                                  `json:"best_canonical_height_px,omitempty"`
	BestPhaseProfile                                   Profile                                  `json:"best_phase_profile,omitempty"`
	BestPhaseConsistency                               float64                                  `json:"best_phase_consistency"`
	BestPhaseXCoherence                                float64                                  `json:"best_phase_x_coherence"`
	BestPhaseYCoherence                                float64                                  `json:"best_phase_y_coherence"`
	BestPhaseCanonicalWidth                            float64                                  `json:"best_phase_canonical_width_px,omitempty"`
	BestPhaseCanonicalHeight                           float64                                  `json:"best_phase_canonical_height_px,omitempty"`
	PhotometricProbeAttempts                           int                                      `json:"photometric_probe_attempts"`
	PhotometricResults                                 []DiagnosticPhotometricEvidence          `json:"photometric_results,omitempty"`
	BestPhotometricMode                                string                                   `json:"best_photometric_mode,omitempty"`
	BestPhotometricProfile                             Profile                                  `json:"best_photometric_profile,omitempty"`
	BestPhotometricFraction                            float64                                  `json:"best_photometric_sync_fraction"`
	BestPhotometricZScore                              float64                                  `json:"best_photometric_sync_z_score"`
	BestPhotometricMeanAbs                             float64                                  `json:"best_photometric_mean_absolute_margin"`
	BestPhotometricMargin                              float64                                  `json:"best_photometric_normalized_sync_margin"`
	BitDiagnosticAttempts                              int                                      `json:"bit_diagnostic_attempts"`
	BitDiagnostics                                     []DiagnosticBitChannelEvidence           `json:"bit_diagnostics,omitempty"`
	BestBitProfile                                     Profile                                  `json:"best_bit_profile,omitempty"`
	BestBitMode                                        string                                   `json:"best_bit_mode,omitempty"`
	BestBitGeometrySource                              string                                   `json:"best_bit_geometry_source,omitempty"`
	BestKnownHeaderClean                               int                                      `json:"best_known_header_hamming_clean_words"`
	BestKnownHeaderOneBit                              int                                      `json:"best_known_header_hamming_one_bit_words"`
	BestKnownHeaderErrors                              int                                      `json:"best_known_header_coded_bit_errors"`
	BestKnownHeaderMulti                               int                                      `json:"best_known_header_hamming_multi_error_words"`
	BestPostECCHeaderErrors                            int                                      `json:"best_post_ecc_known_header_bit_errors"`
	BestSoftPostECCHeaderErrors                        int                                      `json:"best_soft_post_ecc_known_header_bit_errors"`
	BestSoftHeaderImprovement                          int                                      `json:"best_soft_known_header_bit_improvement"`
	BestSoftProfile                                    Profile                                  `json:"best_soft_profile,omitempty"`
	BestSoftMode                                       string                                   `json:"best_soft_mode,omitempty"`
	BestSoftGeometrySource                             string                                   `json:"best_soft_geometry_source,omitempty"`
	BestHardCandidateSoftErrors                        int                                      `json:"best_hard_candidate_soft_post_ecc_known_header_bit_errors"`
	BestHardCandidateSoftGain                          int                                      `json:"best_hard_candidate_soft_known_header_bit_improvement"`
	BestSoftCandidateHardErrors                        int                                      `json:"best_soft_candidate_hard_post_ecc_known_header_bit_errors"`
	SpatialDiagnosticAttempts                          int                                      `json:"spatial_diagnostic_attempts"`
	BestSpatialProfile                                 Profile                                  `json:"best_spatial_profile,omitempty"`
	BestSpatialMode                                    string                                   `json:"best_spatial_mode,omitempty"`
	BestSpatialGeometrySource                          string                                   `json:"best_spatial_geometry_source,omitempty"`
	BestSpatialCells                                   int                                      `json:"best_spatial_cells"`
	BestSpatialTileAgreement                           float64                                  `json:"best_spatial_mean_tile_position_sign_agreement"`
	BestSpatialTileUnstable                            float64                                  `json:"best_spatial_unstable_tile_position_fraction"`
	BestSpatialStableWrongBits                         int                                      `json:"best_spatial_stable_wrong_known_header_bits"`
	BestSpatialMixedBits                               int                                      `json:"best_spatial_mixed_known_header_bits"`
	BestSpatialMajorityErrors                          int                                      `json:"best_spatial_majority_coded_bit_errors"`
	BestSpatialMajorityPostECC                         int                                      `json:"best_spatial_majority_post_ecc_known_header_bit_errors"`
	BestSpatialMajorityGain                            int                                      `json:"best_spatial_majority_post_ecc_improvement"`
	BestSpatialAllAgreement                            float64                                  `json:"best_spatial_mean_all_coded_bit_agreement"`
	BestSpatialUnstableFraction                        float64                                  `json:"best_spatial_unstable_all_coded_bit_fraction"`
	BestSpatialLocalPhaseSame                          int                                      `json:"best_spatial_local_phase_same_as_global_cells"`
	BestSpatialMeanPhaseOffset                         float64                                  `json:"best_spatial_mean_local_phase_offset_blocks"`
	BestSpatialMaxPhaseOffset                          float64                                  `json:"best_spatial_max_local_phase_offset_blocks"`
	BestSpatialMeanSubblockPhaseOffset                 float64                                  `json:"best_spatial_mean_local_phase_subblock_offset_blocks"`
	BestSpatialMaxSubblockPhaseOffset                  float64                                  `json:"best_spatial_max_local_phase_subblock_offset_blocks"`
	BestSpatialMeanPhaseConfidence                     float64                                  `json:"best_spatial_mean_local_phase_confidence"`
	BestSpatialMinPhaseConfidence                      float64                                  `json:"best_spatial_min_local_phase_confidence"`
	BestSpatialBlindMethod                             string                                   `json:"best_spatial_blind_phase_method,omitempty"`
	BestSpatialBlindProfile                            Profile                                  `json:"best_spatial_blind_phase_profile,omitempty"`
	BestSpatialBlindGlobalX                            int                                      `json:"best_spatial_blind_phase_global_x"`
	BestSpatialBlindGlobalY                            int                                      `json:"best_spatial_blind_phase_global_y"`
	BestSpatialBlindGlobalScore                        float64                                  `json:"best_spatial_blind_phase_global_score"`
	BestSpatialBlindCells                              int                                      `json:"best_spatial_blind_phase_cells"`
	BestSpatialBlindPairs                              int                                      `json:"best_spatial_blind_phase_pairs"`
	BestSpatialBlindMeanPairScore                      float64                                  `json:"best_spatial_blind_phase_mean_pair_score"`
	BestSpatialBlindMeanOffset                         float64                                  `json:"best_spatial_blind_phase_mean_offset_blocks"`
	BestSpatialBlindMaxOffset                          float64                                  `json:"best_spatial_blind_phase_max_offset_blocks"`
	BestSpatialBlindMeanConfidence                     float64                                  `json:"best_spatial_blind_phase_mean_confidence"`
	BestSpatialBlindMinConfidence                      float64                                  `json:"best_spatial_blind_phase_min_confidence"`
	BestSpatialBlindRobustOutliers                     int                                      `json:"best_spatial_blind_phase_robust_pair_outliers"`
	BestSpatialBlindSecondaryMethod                    string                                   `json:"best_spatial_blind_secondary_method,omitempty"`
	BestSpatialBlindSecondaryCells                     int                                      `json:"best_spatial_blind_secondary_cells"`
	BestSpatialBlindSecondaryMeanScore                 float64                                  `json:"best_spatial_blind_secondary_mean_pair_score"`
	BestSpatialBlindConsensusCells                     int                                      `json:"best_spatial_blind_consensus_cells"`
	BestSpatialBlindCycleSlipFixes                     int                                      `json:"best_spatial_blind_cycle_slip_corrections"`
	BestSpatialBlindMeanObserverDist                   float64                                  `json:"best_spatial_blind_mean_observer_distance_blocks"`
	BestSpatialBlindMaxObserverDist                    float64                                  `json:"best_spatial_blind_max_observer_distance_blocks"`
	BestSpatialBlindLatticeMethod                      string                                   `json:"best_spatial_blind_lattice_phase_method,omitempty"`
	BestSpatialBlindLatticeCells                       int                                      `json:"best_spatial_blind_lattice_phase_cells"`
	BestSpatialBlindLatticeMeanConf                    float64                                  `json:"best_spatial_blind_lattice_phase_mean_confidence"`
	BestSpatialBlindLatticeMinConf                     float64                                  `json:"best_spatial_blind_lattice_phase_min_confidence"`
	BestSpatialBlindLatticeMeanDist                    float64                                  `json:"best_spatial_blind_lattice_mean_fraction_distance_blocks"`
	BestSpatialBlindLatticeMaxDist                     float64                                  `json:"best_spatial_blind_lattice_max_fraction_distance_blocks"`
	BestSpatialBlindLatticeConsensus                   int                                      `json:"best_spatial_blind_lattice_consensus_cells"`
	BestSpatialBlindLatticeCycleFixes                  int                                      `json:"best_spatial_blind_lattice_cycle_slip_corrections"`
	BestSpatialBlindGlobalUnwrapMethod                 string                                   `json:"best_spatial_blind_global_unwrap_method,omitempty"`
	BestSpatialBlindGlobalUnwrapStatus                 string                                   `json:"best_spatial_blind_global_unwrap_status,omitempty"`
	BestSpatialBlindGlobalUnwrapStatusX                string                                   `json:"best_spatial_blind_global_unwrap_status_x,omitempty"`
	BestSpatialBlindGlobalUnwrapStatusY                string                                   `json:"best_spatial_blind_global_unwrap_status_y,omitempty"`
	BestSpatialBlindGlobalUnwrapStates                 int                                      `json:"best_spatial_blind_global_unwrap_evaluated_states"`
	BestSpatialBlindGlobalUnwrapEligible               int                                      `json:"best_spatial_blind_global_unwrap_eligible_cells"`
	BestSpatialBlindGlobalUnwrapEligibleX              int                                      `json:"best_spatial_blind_global_unwrap_eligible_cells_x"`
	BestSpatialBlindGlobalUnwrapEligibleY              int                                      `json:"best_spatial_blind_global_unwrap_eligible_cells_y"`
	BestSpatialBlindGlobalUnwrapChanged                int                                      `json:"best_spatial_blind_global_unwrap_changed_cells"`
	BestSpatialBlindGlobalUnwrapProposedChanged        int                                      `json:"best_spatial_blind_global_unwrap_proposed_changed_cells"`
	BestSpatialBlindGlobalUnwrapAcceptedAxes           int                                      `json:"best_spatial_blind_global_unwrap_accepted_axes"`
	BestSpatialBlindGlobalUnwrapBaseline               float64                                  `json:"best_spatial_blind_global_unwrap_baseline_objective"`
	BestSpatialBlindGlobalUnwrapObjective              float64                                  `json:"best_spatial_blind_global_unwrap_proposed_objective"`
	BestSpatialBlindGlobalUnwrapAppliedObjective       float64                                  `json:"best_spatial_blind_global_unwrap_applied_objective"`
	BestSpatialBlindGlobalUnwrapSecond                 float64                                  `json:"best_spatial_blind_global_unwrap_second_objective"`
	BestSpatialBlindGlobalUnwrapSecondAvailable        bool                                     `json:"best_spatial_blind_global_unwrap_second_objective_available"`
	BestSpatialBlindGlobalUnwrapImprovement            float64                                  `json:"best_spatial_blind_global_unwrap_improvement"`
	BestSpatialBlindGlobalUnwrapMargin                 float64                                  `json:"best_spatial_blind_global_unwrap_margin"`
	BestSpatialBlindGlobalUnwrapAmbiguous              bool                                     `json:"best_spatial_blind_global_unwrap_ambiguous"`
	BestSpatialBlindGlobalUnwrapValidationMethod       string                                   `json:"best_spatial_blind_global_unwrap_validation_method,omitempty"`
	BestSpatialBlindGlobalUnwrapValidationAvailable    bool                                     `json:"best_spatial_blind_global_unwrap_validation_available"`
	BestSpatialBlindGlobalUnwrapValidationCells        int                                      `json:"best_spatial_blind_global_unwrap_validation_cells"`
	BestSpatialBlindGlobalUnwrapValidationPairsFold0   int                                      `json:"best_spatial_blind_global_unwrap_validation_pairs_fold0"`
	BestSpatialBlindGlobalUnwrapValidationPairsFold1   int                                      `json:"best_spatial_blind_global_unwrap_validation_pairs_fold1"`
	BestSpatialBlindGlobalUnwrapValidationFold0Delta   float64                                  `json:"best_spatial_blind_global_unwrap_validation_fold0_delta"`
	BestSpatialBlindGlobalUnwrapValidationFold1Delta   float64                                  `json:"best_spatial_blind_global_unwrap_validation_fold1_delta"`
	BestSpatialBlindGlobalUnwrapValidationMeanDelta    float64                                  `json:"best_spatial_blind_global_unwrap_validation_mean_delta"`
	BestSpatialBlindGlobalUnwrapValidationSupportsBest bool                                     `json:"best_spatial_blind_global_unwrap_validation_supports_best"`
	BestSpatialBlindOracleCells                        int                                      `json:"best_spatial_blind_phase_oracle_compared_cells"`
	BestSpatialBlindMeanOracleDistance                 float64                                  `json:"best_spatial_blind_phase_mean_oracle_distance_blocks"`
	BestSpatialBlindMaxOracleDistance                  float64                                  `json:"best_spatial_blind_phase_max_oracle_distance_blocks"`
	BestSpatialLocalStableWrong                        int                                      `json:"best_spatial_local_phase_stable_wrong_known_header_bits"`
	BestSpatialLocalMixed                              int                                      `json:"best_spatial_local_phase_mixed_known_header_bits"`
	BestSpatialLocalMajorityECC                        int                                      `json:"best_spatial_local_phase_majority_post_ecc_known_header_bit_errors"`
	BestSpatialLocalAgreement                          float64                                  `json:"best_spatial_local_phase_mean_all_coded_bit_agreement"`
	BestSpatialLocalUnstable                           float64                                  `json:"best_spatial_local_phase_unstable_all_coded_bit_fraction"`
	SmoothPhaseFitAttempts                             int                                      `json:"smooth_phase_fit_attempts"`
	SmoothPhaseEligibleFits                            int                                      `json:"smooth_phase_eligible_fits"`
	SmoothPhaseResampleAttempts                        int                                      `json:"smooth_phase_resample_attempts"`
	SmoothPhaseDecodeAttempts                          int                                      `json:"smooth_phase_decode_attempts"`
	BestSmoothPhaseProfile                             Profile                                  `json:"best_smooth_phase_profile,omitempty"`
	BestSmoothPhaseMode                                string                                   `json:"best_smooth_phase_mode,omitempty"`
	BestSmoothPhaseGeometry                            string                                   `json:"best_smooth_phase_geometry_source,omitempty"`
	BestSmoothPhaseControls                            int                                      `json:"best_smooth_phase_controls"`
	BestSmoothPhaseModel                               string                                   `json:"best_smooth_phase_model,omitempty"`
	BestSmoothPhaseControlSource                       string                                   `json:"best_smooth_phase_control_source,omitempty"`
	BestSmoothPhaseMeanConfidence                      float64                                  `json:"best_smooth_phase_mean_control_confidence"`
	BestSmoothPhaseMinConfidence                       float64                                  `json:"best_smooth_phase_min_control_confidence"`
	BestSmoothPhaseRobustOutliers                      int                                      `json:"best_smooth_phase_robust_outliers"`
	BestSmoothPhaseAffineFitRMS                        float64                                  `json:"best_smooth_phase_affine_fit_rms_blocks"`
	BestSmoothPhaseAffineLOORMS                        float64                                  `json:"best_smooth_phase_affine_leave_one_out_rms_blocks"`
	BestSmoothPhaseQuadraticAvailable                  bool                                     `json:"best_smooth_phase_quadratic_available"`
	BestSmoothPhaseQuadraticFitRMS                     float64                                  `json:"best_smooth_phase_quadratic_fit_rms_blocks"`
	BestSmoothPhaseQuadraticLOORMS                     float64                                  `json:"best_smooth_phase_quadratic_leave_one_out_rms_blocks"`
	BestSmoothPhaseQuadraticSelected                   bool                                     `json:"best_smooth_phase_quadratic_selected"`
	BestSmoothPhaseFitRMS                              float64                                  `json:"best_smooth_phase_fit_rms_blocks"`
	BestSmoothPhaseLOORMS                              float64                                  `json:"best_smooth_phase_leave_one_out_rms_blocks"`
	BestSmoothPhaseMaxCorrection                       float64                                  `json:"best_smooth_phase_max_correction_blocks"`
	BestSmoothPhaseBasePostECC                         int                                      `json:"best_smooth_phase_base_post_ecc_known_header_bit_errors"`
	BestSmoothPhasePostECC                             int                                      `json:"best_smooth_phase_post_ecc_known_header_bit_errors"`
	BestSmoothPhaseImprovement                         int                                      `json:"best_smooth_phase_post_ecc_improvement"`
	BestSmoothPhaseCodedErrors                         int                                      `json:"best_smooth_phase_known_header_coded_bit_errors"`
	BestSmoothPhaseSyncZ                               float64                                  `json:"best_smooth_phase_sync_z_score"`
	BestSyndromeFraction                               float64                                  `json:"best_hamming_nonzero_syndrome_fraction"`
	BestWrongMarginRatio                               float64                                  `json:"best_known_wrong_to_correct_margin_ratio"`
}

// DiagnosticProjectiveProbeEvidence exposes the bounded known-header score for
// one geometry candidate. It is key-assisted research evidence, not a watermark
// result.
type DiagnosticProjectiveProbeEvidence struct {
	CanonicalWidthPx   float64 `json:"canonical_width_px"`
	CanonicalHeightPx  float64 `json:"canonical_height_px"`
	GeometryConfidence float64 `json:"geometry_confidence"`
	SyncProfile        Profile `json:"sync_profile,omitempty"`
	SyncFraction       float64 `json:"sync_fraction"`
	SyncZScore         float64 `json:"sync_z_score"`
	PhaseProfile       Profile `json:"phase_profile,omitempty"`
	PhaseConsistency   float64 `json:"phase_consistency"`
	PhaseXCoherence    float64 `json:"phase_x_coherence"`
	PhaseYCoherence    float64 `json:"phase_y_coherence"`
	PhaseMeanZScore    float64 `json:"phase_mean_z_score"`
	PhaseTiles         int     `json:"phase_tiles"`
}

// DiagnosticProjectiveRefinementEvidence records one bounded phase-aware
// refinement candidate. It remains research evidence until the ordinary v3
// decoder authenticates a payload.
type DiagnosticProjectiveRefinementEvidence struct {
	Source            string  `json:"source"`
	CanonicalWidthPx  float64 `json:"canonical_width_px"`
	CanonicalHeightPx float64 `json:"canonical_height_px"`
	PhaseProfile      Profile `json:"phase_profile,omitempty"`
	PhaseConsistency  float64 `json:"phase_consistency"`
	PhaseXCoherence   float64 `json:"phase_x_coherence"`
	PhaseYCoherence   float64 `json:"phase_y_coherence"`
	PhaseMeanZScore   float64 `json:"phase_mean_z_score"`
	SyncProfile       Profile `json:"sync_profile,omitempty"`
	SyncFraction      float64 `json:"sync_fraction"`
	SyncZScore        float64 `json:"sync_z_score"`
	ResidualControls  int     `json:"residual_controls,omitempty"`
	ResidualRMSPixels float64 `json:"residual_rms_px,omitempty"`
	CanonicalOffsetX  float64 `json:"canonical_offset_x,omitempty"`
	CanonicalOffsetY  float64 `json:"canonical_offset_y,omitempty"`
}

type diagnosticProjectiveProbe struct {
	candidate DiagnosticScaleCandidate
	profile   Profile
	fraction  float64
	z         float64
}

func attemptDiagnosticProjectiveAuthentication(src image.Image, key []byte, estimate DiagnosticProjectiveEstimate) ([]byte, ExtractInfo, DiagnosticProjectiveAuthentication, bool) {
	evidence := DiagnosticProjectiveAuthentication{}
	if src == nil || len(key) < 8 || !estimate.Available || len(estimate.ScaleCandidates) == 0 {
		return nil, ExtractInfo{}, evidence, false
	}
	decoder := newDecoder(key)
	boundary := boundaryFromProjectiveEstimate(estimate)
	authCandidates := make([]diagnosticAuthCandidate, 0, len(estimate.ScaleCandidates)+2*diagnosticMaxPhaseRefineSeeds)
	phaseSeeds := make([]diagnosticAuthCandidate, 0, len(estimate.ScaleCandidates))

	updateBest := func(candidate DiagnosticScaleCandidate, phase diagnosticPhaseConsensus, probe diagnosticProjectiveProbe) {
		if probe.z > evidence.BestSyncZScore || evidence.BestSyncProfile == "" {
			evidence.BestSyncProfile = probe.profile
			evidence.BestSyncFraction = probe.fraction
			evidence.BestSyncZScore = probe.z
			evidence.BestCanonicalWidthPx = candidate.CanonicalWidthPixels
			evidence.BestCanonicalHeightPx = candidate.CanonicalHeightPixels
		}
		if phase.score > evidence.BestPhaseConsistency || evidence.BestPhaseProfile == "" {
			evidence.BestPhaseProfile = phase.profile
			evidence.BestPhaseConsistency = phase.score
			evidence.BestPhaseXCoherence = phase.xCoherence
			evidence.BestPhaseYCoherence = phase.yCoherence
			evidence.BestPhaseCanonicalWidth = candidate.CanonicalWidthPixels
			evidence.BestPhaseCanonicalHeight = candidate.CanonicalHeightPixels
		}
	}

	coarseStarted := time.Now()
	for _, candidate := range estimate.ScaleCandidates {
		probe := diagnosticProbeProjectiveSync(src, boundary, candidate, decoder)
		phase := diagnosticProjectivePhaseConsensus(src, boundary, candidate, decoder)
		evidence.ProbeResults = append(evidence.ProbeResults, DiagnosticProjectiveProbeEvidence{
			CanonicalWidthPx: candidate.CanonicalWidthPixels, CanonicalHeightPx: candidate.CanonicalHeightPixels,
			GeometryConfidence: candidate.Confidence, SyncProfile: probe.profile, SyncFraction: probe.fraction, SyncZScore: probe.z,
			PhaseProfile: phase.profile, PhaseConsistency: phase.score, PhaseXCoherence: phase.xCoherence,
			PhaseYCoherence: phase.yCoherence, PhaseMeanZScore: phase.meanZ, PhaseTiles: phase.tiles,
		})
		evidence.CandidatesProbed++
		h, ok := homographyForPrintBoundary(math.Round(candidate.CanonicalWidthPixels), math.Round(candidate.CanonicalHeightPixels), boundary)
		if ok {
			auth := diagnosticAuthCandidate{source: "coarse", candidate: candidate, phase: phase, mapper: diagnosticProjectiveMapper{h: h}, probe: probe}
			authCandidates = append(authCandidates, auth)
			phaseSeeds = append(phaseSeeds, auth)
		}
		updateBest(candidate, phase, probe)
	}

	evidence.CoarseProbeMilliseconds = time.Since(coarseStarted).Milliseconds()
	refinementStarted := time.Now()
	phaseSeeds = selectDiagnosticRefineSeeds(phaseSeeds, diagnosticMaxPhaseRefineSeeds)
	for _, seed := range phaseSeeds {
		if seed.candidate.Fundamental {
			fundamental, fundamentalMapper, fundamentalProbe, scaleAttempts := refineDiagnosticFundamentalScaleBySync(src, boundary, seed.candidate, decoder)
			evidence.FundamentalScaleProbes += scaleAttempts
			fundamentalMapper, fundamentalProbe, subpixelAttempts := refineDiagnosticSubpixelOffset(src, fundamental, fundamentalMapper, decoder)
			evidence.SubpixelRefinements++
			evidence.SubpixelProbeAttempts += subpixelAttempts
			fundamentalPhase := diagnosticProjectivePhaseConsensusWithMapper(src, int(math.Round(fundamental.CanonicalWidthPixels)), int(math.Round(fundamental.CanonicalHeightPixels)), fundamentalMapper, decoder, diagnosticPhaseTilesPerAxis)
			fundamentalEvidence := diagnosticRefinementEvidence("fundamental-sync-subpixel", fundamental, fundamentalPhase, fundamentalProbe)
			fundamentalEvidence.CanonicalOffsetX = fundamentalMapper.canonicalOffsetX
			fundamentalEvidence.CanonicalOffsetY = fundamentalMapper.canonicalOffsetY
			evidence.RefinedResults = append(evidence.RefinedResults, fundamentalEvidence)
			authCandidates = append(authCandidates, diagnosticAuthCandidate{source: "fundamental-sync-subpixel", candidate: fundamental, phase: fundamentalPhase, mapper: fundamentalMapper, probe: fundamentalProbe})
			evidence.BestFundamentalWidthPx = fundamental.CanonicalWidthPixels
			evidence.BestFundamentalHeightPx = fundamental.CanonicalHeightPixels
			evidence.BestFundamentalSyncZ = fundamentalProbe.z
			if fundamentalProbe.z > evidence.BestSubpixelZScore || evidence.BestSubpixelZScore == 0 {
				evidence.BestSubpixelZScore = fundamentalProbe.z
				evidence.BestSubpixelOffsetX = fundamentalMapper.canonicalOffsetX
				evidence.BestSubpixelOffsetY = fundamentalMapper.canonicalOffsetY
			}
			updateBest(fundamental, fundamentalPhase, fundamentalProbe)
		}

		refined, phase, attempts := refineDiagnosticScaleByPhase(src, boundary, seed.candidate, decoder)
		evidence.PhaseRefinementAttempts += attempts
		width := int(math.Round(refined.CanonicalWidthPixels))
		height := int(math.Round(refined.CanonicalHeightPixels))
		h, ok := homographyForPrintBoundary(float64(width), float64(height), boundary)
		if !ok {
			continue
		}
		probe := diagnosticProbeProjectiveSyncWithHomography(src, refined, h, decoder)
		evidence.RefinedResults = append(evidence.RefinedResults, diagnosticRefinementEvidence("phase-scale", refined, phase, probe))
		authCandidates = append(authCandidates, diagnosticAuthCandidate{source: "phase-scale", candidate: refined, phase: phase, mapper: diagnosticProjectiveMapper{h: h}, probe: probe})
		updateBest(refined, phase, probe)

		if refined.Fundamental {
			subpixelMapper, subpixelProbe, subpixelAttempts := refineDiagnosticSubpixelOffset(src, refined, diagnosticProjectiveMapper{h: h}, decoder)
			evidence.SubpixelRefinements++
			evidence.SubpixelProbeAttempts += subpixelAttempts
			subpixelPhase := diagnosticProjectivePhaseConsensusWithMapper(src, width, height, subpixelMapper, decoder, diagnosticPhaseTilesPerAxis)
			subpixelEvidence := diagnosticRefinementEvidence("subpixel-phase", refined, subpixelPhase, subpixelProbe)
			subpixelEvidence.CanonicalOffsetX = subpixelMapper.canonicalOffsetX
			subpixelEvidence.CanonicalOffsetY = subpixelMapper.canonicalOffsetY
			evidence.RefinedResults = append(evidence.RefinedResults, subpixelEvidence)
			authCandidates = append(authCandidates, diagnosticAuthCandidate{source: "subpixel-phase", candidate: refined, phase: subpixelPhase, mapper: subpixelMapper, probe: subpixelProbe})
			if subpixelProbe.z > evidence.BestSubpixelZScore || evidence.SubpixelRefinements == 1 {
				evidence.BestSubpixelZScore = subpixelProbe.z
				evidence.BestSubpixelOffsetX = subpixelMapper.canonicalOffsetX
				evidence.BestSubpixelOffsetY = subpixelMapper.canonicalOffsetY
			}
			updateBest(refined, subpixelPhase, subpixelProbe)
		}

		if refined.Fundamental && evidence.ResidualWarpFits == 0 {
			if warp, ok := buildDiagnosticLatticeResidualWarp(refined, h, estimate.localRegions); ok {
				evidence.ResidualWarpFits++
				mapper := diagnosticProjectiveMapper{h: h, warp: warp}
				residualPhase := diagnosticProjectivePhaseConsensusWithMapper(src, width, height, mapper, decoder, diagnosticPhaseTilesPerAxis)
				residualProbe := diagnosticProbeProjectiveSyncWithMapper(src, refined, mapper, decoder)
				evidence.RefinedResults = append(evidence.RefinedResults, diagnosticResidualRefinementEvidence("lattice-residual-warp", refined, residualPhase, residualProbe, warp))
				authCandidates = append(authCandidates, diagnosticAuthCandidate{
					source: "lattice-residual-warp", candidate: refined, phase: residualPhase, mapper: mapper, probe: residualProbe,
					residualControls: warp.controls, residualRMS: warp.rmsPixels,
				})
				if evidence.BestResidualControls == 0 || warp.rmsPixels < evidence.BestResidualRMSPixels {
					evidence.BestResidualControls = warp.controls
					evidence.BestResidualRMSPixels = warp.rmsPixels
				}
				updateBest(refined, residualPhase, residualProbe)
			}
		}

		phaseH, ok := diagnosticPhaseRefinedHomography(src, boundary, refined, phase, decoder)
		if ok {
			evidence.PhaseHomographyFits++
			phaseAfter := diagnosticProjectivePhaseConsensusWithHomography(src, width, height, phaseH, decoder)
			phaseProbe := diagnosticProbeProjectiveSyncWithHomography(src, refined, phaseH, decoder)
			evidence.RefinedResults = append(evidence.RefinedResults, diagnosticRefinementEvidence("phase-dlt", refined, phaseAfter, phaseProbe))
			authCandidates = append(authCandidates, diagnosticAuthCandidate{source: "phase-dlt", candidate: refined, phase: phaseAfter, mapper: diagnosticProjectiveMapper{h: phaseH}, probe: phaseProbe})
			updateBest(refined, phaseAfter, phaseProbe)

			if refined.Fundamental {
				phaseSubMapper, phaseSubProbe, subpixelAttempts := refineDiagnosticSubpixelOffset(src, refined, diagnosticProjectiveMapper{h: phaseH}, decoder)
				evidence.SubpixelRefinements++
				evidence.SubpixelProbeAttempts += subpixelAttempts
				phaseSubPhase := diagnosticProjectivePhaseConsensusWithMapper(src, width, height, phaseSubMapper, decoder, diagnosticPhaseTilesPerAxis)
				phaseSubEvidence := diagnosticRefinementEvidence("phase-dlt-subpixel", refined, phaseSubPhase, phaseSubProbe)
				phaseSubEvidence.CanonicalOffsetX = phaseSubMapper.canonicalOffsetX
				phaseSubEvidence.CanonicalOffsetY = phaseSubMapper.canonicalOffsetY
				evidence.RefinedResults = append(evidence.RefinedResults, phaseSubEvidence)
				authCandidates = append(authCandidates, diagnosticAuthCandidate{source: "phase-dlt-subpixel", candidate: refined, phase: phaseSubPhase, mapper: phaseSubMapper, probe: phaseSubProbe})
				if phaseSubProbe.z > evidence.BestSubpixelZScore {
					evidence.BestSubpixelZScore = phaseSubProbe.z
					evidence.BestSubpixelOffsetX = phaseSubMapper.canonicalOffsetX
					evidence.BestSubpixelOffsetY = phaseSubMapper.canonicalOffsetY
				}
				updateBest(refined, phaseSubPhase, phaseSubProbe)
			}
		}

	}

	evidence.RefinementMilliseconds = time.Since(refinementStarted).Milliseconds()
	photometricStarted := time.Now()
	geometryCandidates := selectDiagnosticPhotometricGeometries(authCandidates, diagnosticMaxPhotometricGeometries)
	photometricCandidates := buildDiagnosticPhotometricCandidates(src, geometryCandidates, decoder)
	evidence.PhotometricProbeAttempts = len(photometricCandidates)
	for _, candidate := range photometricCandidates {
		probe := candidate.probe
		evidence.PhotometricResults = append(evidence.PhotometricResults, DiagnosticPhotometricEvidence{
			Mode: candidate.mode.String(), GeometrySource: candidate.geometry.source,
			CanonicalWidthPx: candidate.geometry.candidate.CanonicalWidthPixels, CanonicalHeightPx: candidate.geometry.candidate.CanonicalHeightPixels,
			SyncProfile: probe.probe.profile, SyncFraction: probe.probe.fraction, SyncZScore: probe.probe.z,
			MeanAbsoluteMargin: probe.meanAbsoluteMargin, NormalizedSyncMargin: probe.normalizedSyncMargin,
		})
		if evidence.BestPhotometricMode == "" || probe.probe.z > evidence.BestPhotometricZScore ||
			(probe.probe.z == evidence.BestPhotometricZScore && probe.normalizedSyncMargin > evidence.BestPhotometricMargin) {
			evidence.BestPhotometricMode = candidate.mode.String()
			evidence.BestPhotometricProfile = probe.probe.profile
			evidence.BestPhotometricFraction = probe.probe.fraction
			evidence.BestPhotometricZScore = probe.probe.z
			evidence.BestPhotometricMeanAbs = probe.meanAbsoluteMargin
			evidence.BestPhotometricMargin = probe.normalizedSyncMargin
		}
	}

	evidence.PhotometricMilliseconds = time.Since(photometricStarted).Milliseconds()
	decodeStarted := time.Now()
	decodeCandidates := selectDiagnosticPhotometricDecodeCandidates(photometricCandidates, diagnosticMaxProjectiveFullDecodes)
	for _, candidate := range decodeCandidates {
		width := int(math.Round(candidate.geometry.candidate.CanonicalWidthPixels))
		height := int(math.Round(candidate.geometry.candidate.CanonicalHeightPixels))
		grid, stats, ok := diagnosticProjectiveGridWithMapperPhotometricStats(src, width, height, candidate.geometry.mapper, candidate.mode)
		if !ok {
			continue
		}
		if stats.Bounded {
			evidence.FullGridBoundedUses++
		}
		if stats.SampledBlocks > evidence.MaxFullGridSampledBlocks {
			evidence.MaxFullGridSampledBlocks = stats.SampledBlocks
		}
		if stats.SampledTiles > evidence.MaxFullGridSampledTiles {
			evidence.MaxFullGridSampledTiles = stats.SampledTiles
		}

		var bitEvidence DiagnosticBitChannelEvidence
		bitOK := false
		decodeGrid := grid
		smoothSelected := false
		if bitEvidence, bitOK = diagnosticAnalyzeBitChannel(grid, key, decoder, candidate.geometry.source, candidate.mode); bitOK {
			latticePhase, latticePhaseOK := diagnosticEstimateLocalLatticeFractionalPhase(candidate.geometry.candidate, candidate.geometry.mapper, stats.spatialCells, estimate.localRegions)
			diagnosticAttachSpatialBitEvidenceWithGridAndLattice(&bitEvidence, stats.spatialCells, grid, key, latticePhase, latticePhaseOK)
			evidence.BitDiagnosticAttempts++
			if bitEvidence.Spatial != nil {
				evidence.SpatialDiagnosticAttempts++
				evidence.SmoothPhaseFitAttempts++
				_, smooth, fitOK := diagnosticFitBlindSmoothPhaseField(bitEvidence.Spatial, float64(width), float64(height))
				if fitOK && smooth.Available {
					if smooth.Eligible {
						evidence.SmoothPhaseEligibleFits++
					}
					// Preserve the existing early-exit semantics: smooth fields are
					// considered only on the already-ranked full-decode candidates,
					// with at most two extra bounded resamples and no extra HMAC slot.
					if smooth.Eligible && evidence.SmoothPhaseResampleAttempts < diagnosticMaxSmoothPhaseResamples {
						evidence.SmoothPhaseResampleAttempts++
						corrected, correctedGrid, correctedOK := diagnosticAnalyzeSmoothPhaseCorrection(src, width, height, candidate.geometry.mapper, candidate.mode, bitEvidence, key, decoder)
						smooth = corrected
						if correctedOK {
							updateDiagnosticSmoothPhaseSummary(&evidence, bitEvidence, corrected)
							if corrected.DecodeEligible && corrected.PostECCImprovement > 0 && corrected.CorrectedKnownCodedErrors <= bitEvidence.KnownHeaderCodedErrors {
								decodeGrid = correctedGrid
								smoothSelected = true
								smooth.SelectedForFullDecode = true
							}
						}
					}
					bitEvidence.SmoothPhase = &smooth
				}
			}
			evidence.BitDiagnostics = append(evidence.BitDiagnostics, bitEvidence)
			updateDiagnosticBitSummary(&evidence, bitEvidence)
		}

		evidence.FullDecodeAttempts++
		useReliability := bitOK && !smoothSelected && diagnosticShouldUseReliabilityDecode(bitEvidence)
		var payload []byte
		var info ExtractInfo
		var found bool
		if smoothSelected {
			evidence.SmoothPhaseDecodeAttempts++
			payload, info, _, found = decoder.decodeGrid(decodeGrid)
		} else if useReliability {
			evidence.ReliabilityDecodeAttempts++
			payload, info, _, found = diagnosticDecodeGridReliabilityAware(decodeGrid, key, decoder)
		} else {
			payload, info, _, found = decoder.decodeGrid(decodeGrid)
		}
		if found {
			method := "hard"
			if smoothSelected {
				method = "smooth-phase-hard"
			} else if useReliability {
				method = "soft-hamming"
			}
			info.PerspectiveCorrection = "diagnostic-projective-" + candidate.geometry.source + "-" + candidate.mode.String() + "-" + method
			evidence.FullDecodeMilliseconds = time.Since(decodeStarted).Milliseconds()
			return payload, info, evidence, true
		}
	}
	evidence.FullDecodeMilliseconds = time.Since(decodeStarted).Milliseconds()
	return nil, ExtractInfo{}, evidence, false
}

func selectDiagnosticDecodeCandidates(candidates []diagnosticAuthCandidate, limit int) []diagnosticAuthCandidate {
	if limit <= 0 || len(candidates) == 0 {
		return nil
	}
	result := make([]diagnosticAuthCandidate, 0, limit)
	usedSourceScale := make(map[string]struct{})
	appendCandidate := func(candidate diagnosticAuthCandidate) {
		if len(result) >= limit {
			return
		}
		key := candidate.source
		if candidate.candidate.Fundamental {
			key += "-fundamental"
		}
		if _, exists := usedSourceScale[key]; exists {
			return
		}
		result = append(result, candidate)
		usedSourceScale[key] = struct{}{}
	}

	// The build4 fundamental-scale + modulo-8 candidate gets the first decode
	// slot when available. It is constrained to the multi-level family selected
	// before any key-assisted refinement.
	for _, candidate := range candidates {
		if candidate.source == "fundamental-sync-subpixel" && candidate.candidate.Fundamental {
			appendCandidate(candidate)
			break
		}
	}

	// Reserve the next slot for the modulo-8 subpixel alignment of the
	// phase-refined fundamental family. A physical print boundary need not
	// coincide exactly with the original DCT block origin.
	var fundamentalSubpixel *diagnosticAuthCandidate
	for i := range candidates {
		candidate := &candidates[i]
		if (candidate.source != "subpixel-phase" && candidate.source != "phase-dlt-subpixel") || !candidate.candidate.Fundamental {
			continue
		}
		if fundamentalSubpixel == nil || candidate.probe.z > fundamentalSubpixel.probe.z {
			fundamentalSubpixel = candidate
		}
	}
	if fundamentalSubpixel != nil {
		appendCandidate(*fundamentalSubpixel)
	}

	// Reserve a second slot for the key-independent lattice residual model on
	// the same fundamental family when available.
	var fundamentalResidual *diagnosticAuthCandidate
	for i := range candidates {
		candidate := &candidates[i]
		if candidate.source != "lattice-residual-warp" || !candidate.candidate.Fundamental {
			continue
		}
		if fundamentalResidual == nil || candidate.probe.z > fundamentalResidual.probe.z {
			fundamentalResidual = candidate
		}
	}
	if fundamentalResidual != nil {
		appendCandidate(*fundamentalResidual)
	}

	// Preserve one additional view of the fundamental family in case either
	// refinement model is overfitting noisy local measurements.
	var fundamentalBase *diagnosticAuthCandidate
	for i := range candidates {
		candidate := &candidates[i]
		if !candidate.candidate.Fundamental || candidate.source == "lattice-residual-warp" {
			continue
		}
		if fundamentalBase == nil || candidate.probe.z > fundamentalBase.probe.z {
			fundamentalBase = candidate
		}
	}
	if fundamentalBase != nil {
		appendCandidate(*fundamentalBase)
	}

	ranked := append([]diagnosticAuthCandidate(nil), candidates...)
	sortDiagnosticAuthCandidates(ranked)
	for _, candidate := range ranked {
		appendCandidate(candidate)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func selectDiagnosticRefineSeeds(candidates []diagnosticAuthCandidate, limit int) []diagnosticAuthCandidate {
	if limit <= 0 || len(candidates) == 0 {
		return nil
	}
	result := make([]diagnosticAuthCandidate, 0, limit)
	appendDistinct := func(candidate diagnosticAuthCandidate) {
		for _, existing := range result {
			wr := math.Max(existing.candidate.CanonicalWidthPixels, candidate.candidate.CanonicalWidthPixels)
			hr := math.Max(existing.candidate.CanonicalHeightPixels, candidate.candidate.CanonicalHeightPixels)
			if wr > 0 && hr > 0 && math.Abs(existing.candidate.CanonicalWidthPixels-candidate.candidate.CanonicalWidthPixels)/wr < 0.025 && math.Abs(existing.candidate.CanonicalHeightPixels-candidate.candidate.CanonicalHeightPixels)/hr < 0.025 {
				return
			}
		}
		if len(result) < limit {
			result = append(result, candidate)
		}
	}

	// The explicitly selected fundamental family always receives one bounded
	// refinement slot. This prevents a high-frequency phase alias from consuming
	// the entire refinement budget.
	for _, candidate := range candidates {
		if candidate.candidate.Fundamental {
			appendDistinct(candidate)
			break
		}
	}

	byPhase := append([]diagnosticAuthCandidate(nil), candidates...)
	sortDiagnosticAuthCandidates(byPhase)
	for _, candidate := range byPhase {
		appendDistinct(candidate)
		if len(result) >= limit {
			return result
		}
	}

	byGeometry := append([]diagnosticAuthCandidate(nil), candidates...)
	sort.Slice(byGeometry, func(i, j int) bool {
		a, b := byGeometry[i].candidate, byGeometry[j].candidate
		if a.SupportingLevels != b.SupportingLevels {
			return a.SupportingLevels > b.SupportingLevels
		}
		if a.SupportingRegions != b.SupportingRegions {
			return a.SupportingRegions > b.SupportingRegions
		}
		return a.Confidence > b.Confidence
	})
	for _, candidate := range byGeometry {
		appendDistinct(candidate)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func diagnosticRefinementEvidence(source string, candidate DiagnosticScaleCandidate, phase diagnosticPhaseConsensus, probe diagnosticProjectiveProbe) DiagnosticProjectiveRefinementEvidence {
	return DiagnosticProjectiveRefinementEvidence{
		Source: source, CanonicalWidthPx: candidate.CanonicalWidthPixels, CanonicalHeightPx: candidate.CanonicalHeightPixels,
		PhaseProfile: phase.profile, PhaseConsistency: phase.score, PhaseXCoherence: phase.xCoherence,
		PhaseYCoherence: phase.yCoherence, PhaseMeanZScore: phase.meanZ,
		SyncProfile: probe.profile, SyncFraction: probe.fraction, SyncZScore: probe.z,
	}
}

func diagnosticResidualRefinementEvidence(source string, candidate DiagnosticScaleCandidate, phase diagnosticPhaseConsensus, probe diagnosticProjectiveProbe, warp *diagnosticResidualWarp) DiagnosticProjectiveRefinementEvidence {
	evidence := diagnosticRefinementEvidence(source, candidate, phase, probe)
	if warp != nil {
		evidence.ResidualControls = warp.controls
		evidence.ResidualRMSPixels = warp.rmsPixels
	}
	return evidence
}

// boundaryFromProjectiveEstimate reconstructs the observed quadrilateral from
// the selected matrix. This keeps the authentication helper dependent only on
// the immutable diagnostic estimate, rather than reaching back into the report.
func boundaryFromProjectiveEstimate(estimate DiagnosticProjectiveEstimate) PrintBoundaryEstimate {
	if !estimate.Available || estimate.CanonicalWidthPixels <= 1 || estimate.CanonicalHeightPixels <= 1 {
		return PrintBoundaryEstimate{}
	}
	h := homography{h: estimate.Matrix}
	w, hgt := estimate.CanonicalWidthPixels-1, estimate.CanonicalHeightPixels-1
	tlx, tly, ok1 := h.mapPoint(0, 0)
	trx, try, ok2 := h.mapPoint(w, 0)
	brx, bry, ok3 := h.mapPoint(w, hgt)
	blx, bly, ok4 := h.mapPoint(0, hgt)
	if !(ok1 && ok2 && ok3 && ok4) {
		return PrintBoundaryEstimate{}
	}
	return PrintBoundaryEstimate{
		Detected: true, Confidence: estimate.BoundaryConfidence,
		TopLeft: ImagePoint{X: tlx, Y: tly}, TopRight: ImagePoint{X: trx, Y: try},
		BottomRight: ImagePoint{X: brx, Y: bry}, BottomLeft: ImagePoint{X: blx, Y: bly},
	}
}

func diagnosticProbeProjectiveSync(src image.Image, boundary PrintBoundaryEstimate, candidate DiagnosticScaleCandidate, decoder *decoder) diagnosticProjectiveProbe {
	width := int(math.Round(candidate.CanonicalWidthPixels))
	height := int(math.Round(candidate.CanonicalHeightPixels))
	if width < tileWidth*blockSize || height < tileHeight*blockSize {
		return diagnosticProjectiveProbe{candidate: candidate}
	}
	h, ok := homographyForPrintBoundary(float64(width), float64(height), boundary)
	if !ok {
		return diagnosticProjectiveProbe{candidate: candidate}
	}
	return diagnosticProbeProjectiveSyncWithHomography(src, candidate, h, decoder)
}

func diagnosticProbeProjectiveSyncWithHomography(src image.Image, candidate DiagnosticScaleCandidate, h homography, decoder *decoder) diagnosticProjectiveProbe {
	return diagnosticProbeProjectiveSyncWithMapper(src, candidate, diagnosticProjectiveMapper{h: h}, decoder)
}

func diagnosticProbeProjectiveSyncWithMapper(src image.Image, candidate DiagnosticScaleCandidate, mapper diagnosticProjectiveMapper, decoder *decoder) diagnosticProjectiveProbe {
	return diagnosticProbeProjectiveSyncWithMapperRepeats(src, candidate, mapper, decoder, diagnosticSyncRepeatsPerAxis)
}

func diagnosticProbeProjectiveSyncWithMapperRepeats(src image.Image, candidate DiagnosticScaleCandidate, mapper diagnosticProjectiveMapper, decoder *decoder, repeatsPerAxis int) diagnosticProjectiveProbe {
	width := int(math.Round(candidate.CanonicalWidthPixels))
	height := int(math.Round(candidate.CanonicalHeightPixels))
	if width < tileWidth*blockSize || height < tileHeight*blockSize {
		return diagnosticProjectiveProbe{candidate: candidate}
	}
	if repeatsPerAxis <= 0 {
		repeatsPerAxis = 1
	}
	bw, bh := width/blockSize, height/blockSize
	tileXs := diagnosticSpacedIndices((bw+tileWidth-1)/tileWidth, repeatsPerAxis)
	tileYs := diagnosticSpacedIndices((bh+tileHeight-1)/tileHeight, repeatsPerAxis)
	grid := make([]float64, eccBits)
	counts := make([]int, eccBits)
	for logicalY := 0; logicalY < tileHeight; logicalY++ {
		for logicalX := 0; logicalX < tileWidth; logicalX++ {
			position := logicalY*tileWidth + logicalX
			for _, tileY := range tileYs {
				by := logicalY + tileY*tileHeight
				if by >= bh {
					continue
				}
				for _, tileX := range tileXs {
					bx := logicalX + tileX*tileWidth
					if bx >= bw {
						continue
					}
					margin, ok := diagnosticReadProjectiveBlockWithMapper(src, mapper, float64(bx*blockSize), float64(by*blockSize))
					if !ok {
						continue
					}
					grid[position] += margin
					counts[position]++
				}
			}
		}
	}
	for position, count := range counts {
		if count == 0 {
			grid[position] = 0
		}
	}
	best := diagnosticProjectiveProbe{candidate: candidate, z: math.Inf(-1)}
	for _, pattern := range decoder.v3Patterns {
		phases := strongestV3Phases(grid, pattern, 1)
		if len(phases) == 0 || phases[0].total == 0 {
			continue
		}
		phase := phases[0]
		fraction := float64(phase.score) / float64(phase.total)
		z := (float64(phase.score) - float64(phase.total)/2) / math.Sqrt(float64(phase.total)/4)
		if z > best.z {
			best.profile, best.fraction, best.z = pattern.spec.profile, fraction, z
		}
	}
	if math.IsInf(best.z, -1) {
		best.z = 0
	}
	return best
}

func diagnosticSpacedIndices(count, maximum int) []int {
	if count <= 0 || maximum <= 0 {
		return nil
	}
	if count <= maximum {
		result := make([]int, count)
		for i := range result {
			result[i] = i
		}
		return result
	}
	result := make([]int, 0, maximum)
	last := -1
	for i := 0; i < maximum; i++ {
		value := int(math.Round(float64(i) * float64(count-1) / float64(maximum-1)))
		if value != last {
			result = append(result, value)
			last = value
		}
	}
	return result
}

func diagnosticProjectiveGrid(src image.Image, width, height int, boundary PrintBoundaryEstimate) ([]float64, bool) {
	if width < tileWidth*blockSize || height < tileHeight*blockSize {
		return nil, false
	}
	h, ok := homographyForPrintBoundary(float64(width), float64(height), boundary)
	if !ok {
		return nil, false
	}
	return diagnosticProjectiveGridWithHomography(src, width, height, h)
}

func diagnosticProjectiveGridWithHomography(src image.Image, width, height int, h homography) ([]float64, bool) {
	return diagnosticProjectiveGridWithMapper(src, width, height, diagnosticProjectiveMapper{h: h})
}

type diagnosticGridSamplingStats struct {
	Bounded       bool
	SampledBlocks int
	SampledTiles  int
	spatialCells  []diagnosticSpatialGridCell
}

const diagnosticSpatialGridAxis = 3

type diagnosticSpatialGridCell struct {
	RegionX int
	RegionY int
	grid    []float64
	counts  []int
}

func newDiagnosticSpatialGridCells() []diagnosticSpatialGridCell {
	cells := make([]diagnosticSpatialGridCell, 0, diagnosticSpatialGridAxis*diagnosticSpatialGridAxis)
	for y := 0; y < diagnosticSpatialGridAxis; y++ {
		for x := 0; x < diagnosticSpatialGridAxis; x++ {
			cells = append(cells, diagnosticSpatialGridCell{
				RegionX: x,
				RegionY: y,
				grid:    make([]float64, eccBits),
				counts:  make([]int, eccBits),
			})
		}
	}
	return cells
}

func diagnosticSpatialBucket(index, total int) int {
	if total <= 1 {
		return 1
	}
	bucket := index * diagnosticSpatialGridAxis / total
	if bucket < 0 {
		return 0
	}
	if bucket >= diagnosticSpatialGridAxis {
		return diagnosticSpatialGridAxis - 1
	}
	return bucket
}

func diagnosticAddSpatialSample(cells []diagnosticSpatialGridCell, regionX, regionY, position int, margin float64) {
	if regionX < 0 || regionX >= diagnosticSpatialGridAxis || regionY < 0 || regionY >= diagnosticSpatialGridAxis || position < 0 || position >= eccBits {
		return
	}
	cell := &cells[regionY*diagnosticSpatialGridAxis+regionX]
	cell.grid[position] += margin
	cell.counts[position]++
}

func diagnosticFinalizeSpatialCells(cells []diagnosticSpatialGridCell) []diagnosticSpatialGridCell {
	result := make([]diagnosticSpatialGridCell, 0, len(cells))
	for _, cell := range cells {
		complete := true
		for position, count := range cell.counts {
			if count == 0 {
				complete = false
				break
			}
			cell.grid[position] /= float64(count)
		}
		if complete {
			result = append(result, cell)
		}
	}
	return result
}

type diagnosticProjectiveBlockReader func(originX, originY float64) (float64, bool)

func diagnosticProjectiveGridWithMapper(src image.Image, width, height int, mapper diagnosticProjectiveMapper) ([]float64, bool) {
	grid, _, ok := diagnosticProjectiveGridSampled(width, height, func(originX, originY float64) (float64, bool) {
		return diagnosticReadProjectiveBlockWithMapper(src, mapper, originX, originY)
	})
	return grid, ok
}

func diagnosticProjectiveGridSampled(width, height int, read diagnosticProjectiveBlockReader) ([]float64, diagnosticGridSamplingStats, bool) {
	stats := diagnosticGridSamplingStats{}
	if width < tileWidth*blockSize || height < tileHeight*blockSize || read == nil {
		return nil, stats, false
	}
	bw, bh := width/blockSize, height/blockSize
	grid := make([]float64, eccBits)
	counts := make([]int, eccBits)
	spatialCells := newDiagnosticSpatialGridCells()
	if bw*bh <= diagnosticMaxFullGridBlocks {
		for by := 0; by < bh; by++ {
			for bx := 0; bx < bw; bx++ {
				margin, ok := read(float64(bx*blockSize), float64(by*blockSize))
				if !ok {
					continue
				}
				position := (by%tileHeight)*tileWidth + bx%tileWidth
				grid[position] += margin
				counts[position]++
				// Spatial diagnostics use complete v3 tiles only. Partial edge tiles
				// still contribute to the aggregate decode grid above, but they must
				// not make a 3x3 diagnostic cell incomplete. This preserves the
				// decoder while giving the smooth-field fitter well-defined cells.
				fullTilesX, fullTilesY := bw/tileWidth, bh/tileHeight
				tileX, tileY := bx/tileWidth, by/tileHeight
				if tileX < fullTilesX && tileY < fullTilesY {
					regionX := diagnosticSpatialBucket(tileX, fullTilesX)
					regionY := diagnosticSpatialBucket(tileY, fullTilesY)
					diagnosticAddSpatialSample(spatialCells, regionX, regionY, position, margin)
				}
				stats.SampledBlocks++
			}
		}
	} else {
		stats.Bounded = true
		fullTilesX, fullTilesY := bw/tileWidth, bh/tileHeight
		tileXs := diagnosticSpacedIndices(fullTilesX, diagnosticMaxSampledTilesPerAxis)
		tileYs := diagnosticSpacedIndices(fullTilesY, diagnosticMaxSampledTilesPerAxis)
		for _, tileY := range tileYs {
			for _, tileX := range tileXs {
				stats.SampledTiles++
				regionX := diagnosticSpatialBucket(tileX, fullTilesX)
				regionY := diagnosticSpatialBucket(tileY, fullTilesY)
				baseX := tileX * tileWidth
				baseY := tileY * tileHeight
				for logicalY := 0; logicalY < tileHeight; logicalY++ {
					for logicalX := 0; logicalX < tileWidth; logicalX++ {
						bx := baseX + logicalX
						by := baseY + logicalY
						margin, ok := read(float64(bx*blockSize), float64(by*blockSize))
						if !ok {
							continue
						}
						position := logicalY*tileWidth + logicalX
						grid[position] += margin
						counts[position]++
						diagnosticAddSpatialSample(spatialCells, regionX, regionY, position, margin)
						stats.SampledBlocks++
					}
				}
			}
		}
	}
	for _, count := range counts {
		if count == 0 {
			return nil, stats, false
		}
	}
	stats.spatialCells = diagnosticFinalizeSpatialCells(spatialCells)
	return grid, stats, true
}

func diagnosticReadProjectiveBlock(src image.Image, h homography, originX, originY float64) (float64, bool) {
	return diagnosticReadProjectiveBlockWithMapper(src, diagnosticProjectiveMapper{h: h}, originX, originY)
}

func diagnosticReadProjectiveBlockWithMapper(src image.Image, mapper diagnosticProjectiveMapper, originX, originY float64) (float64, bool) {
	coefficient23, coefficient32 := 0.0, 0.0
	for y := 0; y < blockSize; y++ {
		for x := 0; x < blockSize; x++ {
			sx, sy, ok := mapper.mapPoint(originX+float64(x), originY+float64(y))
			if !ok {
				return 0, false
			}
			luminance, ok := diagnosticSampleImageLuminance(src, sx, sy)
			if !ok {
				return 0, false
			}
			coefficient23 += luminance * cosTable[3][x] * cosTable[2][y]
			coefficient32 += luminance * cosTable[2][x] * cosTable[3][y]
		}
	}
	return math.Abs(coefficient23) - math.Abs(coefficient32), true
}

func diagnosticSampleImageLuminance(src image.Image, x, y float64) (float64, bool) {
	bounds := src.Bounds()
	localX, localY := x-float64(bounds.Min.X), y-float64(bounds.Min.Y)
	if localX < 0 || localY < 0 || localX > float64(bounds.Dx()-1) || localY > float64(bounds.Dy()-1) {
		return 0, false
	}
	x0, y0 := int(math.Floor(localX)), int(math.Floor(localY))
	x1, y1 := x0+1, y0+1
	if x1 >= bounds.Dx() {
		x1 = bounds.Dx() - 1
	}
	if y1 >= bounds.Dy() {
		y1 = bounds.Dy() - 1
	}
	fx, fy := localX-float64(x0), localY-float64(y0)
	luminanceAt := func(px, py int) float64 {
		if ycbcr, ok := src.(*image.YCbCr); ok {
			return float64(ycbcr.Y[ycbcr.YOffset(px+bounds.Min.X, py+bounds.Min.Y)]) - 128
		}
		r, g, b, _ := src.At(bounds.Min.X+px, bounds.Min.Y+py).RGBA()
		return .299*float64(r>>8) + .587*float64(g>>8) + .114*float64(b>>8) - 128
	}
	v00 := luminanceAt(x0, y0)
	v10 := luminanceAt(x1, y0)
	v01 := luminanceAt(x0, y1)
	v11 := luminanceAt(x1, y1)
	top := v00*(1-fx) + v10*fx
	bottom := v01*(1-fx) + v11*fx
	return top*(1-fy) + bottom*fy, true
}

func updateDiagnosticSmoothPhaseSummary(evidence *DiagnosticProjectiveAuthentication, bit DiagnosticBitChannelEvidence, smooth DiagnosticSmoothPhaseEvidence) {
	if evidence == nil || !smooth.CorrectedGridAnalyzed {
		return
	}
	if evidence.BestSmoothPhaseProfile == "" || smooth.CorrectedPostECCErrors < evidence.BestSmoothPhasePostECC ||
		(smooth.CorrectedPostECCErrors == evidence.BestSmoothPhasePostECC && smooth.CorrectedKnownCodedErrors < evidence.BestSmoothPhaseCodedErrors) ||
		(smooth.CorrectedPostECCErrors == evidence.BestSmoothPhasePostECC && smooth.CorrectedKnownCodedErrors == evidence.BestSmoothPhaseCodedErrors && smooth.LeaveOneOutRMSBlocks < evidence.BestSmoothPhaseLOORMS) {
		evidence.BestSmoothPhaseProfile = smooth.CorrectedProfile
		evidence.BestSmoothPhaseMode = bit.Mode
		evidence.BestSmoothPhaseGeometry = bit.GeometrySource
		evidence.BestSmoothPhaseControls = smooth.Controls
		evidence.BestSmoothPhaseModel = smooth.Model
		evidence.BestSmoothPhaseControlSource = smooth.ControlSource
		evidence.BestSmoothPhaseMeanConfidence = smooth.MeanControlConfidence
		evidence.BestSmoothPhaseMinConfidence = smooth.MinControlConfidence
		evidence.BestSmoothPhaseRobustOutliers = smooth.RobustOutliers
		evidence.BestSmoothPhaseAffineFitRMS = smooth.AffineFitRMSBlocks
		evidence.BestSmoothPhaseAffineLOORMS = smooth.AffineLeaveOneOutRMSBlocks
		evidence.BestSmoothPhaseQuadraticAvailable = smooth.QuadraticAvailable
		evidence.BestSmoothPhaseQuadraticFitRMS = smooth.QuadraticFitRMSBlocks
		evidence.BestSmoothPhaseQuadraticLOORMS = smooth.QuadraticLeaveOneOutRMS
		evidence.BestSmoothPhaseQuadraticSelected = smooth.QuadraticSelected
		evidence.BestSmoothPhaseFitRMS = smooth.FitRMSBlocks
		evidence.BestSmoothPhaseLOORMS = smooth.LeaveOneOutRMSBlocks
		evidence.BestSmoothPhaseMaxCorrection = smooth.MaxCorrectionBlocks
		evidence.BestSmoothPhaseBasePostECC = bit.PostECCHdrErrors
		evidence.BestSmoothPhasePostECC = smooth.CorrectedPostECCErrors
		evidence.BestSmoothPhaseImprovement = smooth.PostECCImprovement
		evidence.BestSmoothPhaseCodedErrors = smooth.CorrectedKnownCodedErrors
		evidence.BestSmoothPhaseSyncZ = smooth.CorrectedSyncZScore
	}
}

func updateDiagnosticBitSummary(evidence *DiagnosticProjectiveAuthentication, bit DiagnosticBitChannelEvidence) {
	if evidence == nil {
		return
	}
	if evidence.BestSoftProfile == "" || bit.SoftPostECCHdrErrors < evidence.BestSoftPostECCHeaderErrors ||
		(bit.SoftPostECCHdrErrors == evidence.BestSoftPostECCHeaderErrors && bit.SoftHeaderImprovement > evidence.BestSoftHeaderImprovement) {
		evidence.BestSoftPostECCHeaderErrors = bit.SoftPostECCHdrErrors
		evidence.BestSoftHeaderImprovement = bit.SoftHeaderImprovement
		evidence.BestSoftCandidateHardErrors = bit.PostECCHdrErrors
		evidence.BestSoftProfile = bit.Profile
		evidence.BestSoftMode = bit.Mode
		evidence.BestSoftGeometrySource = bit.GeometrySource
	}
	if evidence.BestBitProfile == "" || diagnosticBitEvidenceBetter(bit, DiagnosticBitChannelEvidence{
		Profile:                evidence.BestBitProfile,
		KnownHeaderCodedErrors: evidence.BestKnownHeaderErrors,
		KnownHeaderMultiWords:  evidence.BestKnownHeaderMulti,
		PostECCHdrErrors:       evidence.BestPostECCHeaderErrors,
		SyndromeWordFraction:   evidence.BestSyndromeFraction,
		KnownWrongMarginRatio:  evidence.BestWrongMarginRatio,
	}) {
		evidence.BestBitProfile = bit.Profile
		evidence.BestBitMode = bit.Mode
		evidence.BestBitGeometrySource = bit.GeometrySource
		evidence.BestKnownHeaderClean = bit.KnownHeaderCleanWords
		evidence.BestKnownHeaderOneBit = bit.KnownHeaderOneBitWords
		evidence.BestKnownHeaderErrors = bit.KnownHeaderCodedErrors
		evidence.BestKnownHeaderMulti = bit.KnownHeaderMultiWords
		evidence.BestPostECCHeaderErrors = bit.PostECCHdrErrors
		evidence.BestHardCandidateSoftErrors = bit.SoftPostECCHdrErrors
		evidence.BestHardCandidateSoftGain = bit.SoftHeaderImprovement
		evidence.BestSyndromeFraction = bit.SyndromeWordFraction
		evidence.BestWrongMarginRatio = bit.KnownWrongMarginRatio
		if bit.Spatial != nil {
			evidence.BestSpatialProfile = bit.Profile
			evidence.BestSpatialMode = bit.Mode
			evidence.BestSpatialGeometrySource = bit.GeometrySource
			evidence.BestSpatialCells = bit.Spatial.Cells
			evidence.BestSpatialTileAgreement = bit.Spatial.MeanTilePositionSignAgreement
			evidence.BestSpatialTileUnstable = bit.Spatial.UnstableTilePositionFraction
			evidence.BestSpatialStableWrongBits = bit.Spatial.KnownHeaderStableWrongBits
			evidence.BestSpatialMixedBits = bit.Spatial.KnownHeaderMixedBits
			evidence.BestSpatialMajorityErrors = bit.Spatial.KnownHeaderMajorityCodedErrors
			evidence.BestSpatialMajorityPostECC = bit.Spatial.KnownHeaderMajorityPostECCErrors
			evidence.BestSpatialMajorityGain = bit.Spatial.KnownHeaderMajorityECCImprovement
			evidence.BestSpatialAllAgreement = bit.Spatial.MeanAllCodedBitAgreement
			evidence.BestSpatialUnstableFraction = bit.Spatial.UnstableAllCodedBitFraction
			evidence.BestSpatialLocalPhaseSame = bit.Spatial.LocalPhaseSameAsGlobalCells
			evidence.BestSpatialMeanPhaseOffset = bit.Spatial.MeanLocalPhaseOffsetBlocks
			evidence.BestSpatialMaxPhaseOffset = bit.Spatial.MaxLocalPhaseOffsetBlocks
			evidence.BestSpatialMeanSubblockPhaseOffset = bit.Spatial.MeanLocalPhaseSubblockOffsetBlocks
			evidence.BestSpatialMaxSubblockPhaseOffset = bit.Spatial.MaxLocalPhaseSubblockOffsetBlocks
			evidence.BestSpatialMeanPhaseConfidence = bit.Spatial.MeanLocalPhaseConfidence
			evidence.BestSpatialMinPhaseConfidence = bit.Spatial.MinLocalPhaseConfidence
			evidence.BestSpatialBlindMethod = bit.Spatial.BlindPhaseMethod
			evidence.BestSpatialBlindProfile = bit.Spatial.BlindPhaseProfile
			evidence.BestSpatialBlindGlobalX = bit.Spatial.BlindPhaseGlobalX
			evidence.BestSpatialBlindGlobalY = bit.Spatial.BlindPhaseGlobalY
			evidence.BestSpatialBlindGlobalScore = bit.Spatial.BlindPhaseGlobalScore
			evidence.BestSpatialBlindCells = bit.Spatial.BlindPhaseCells
			evidence.BestSpatialBlindPairs = bit.Spatial.BlindPhasePairs
			evidence.BestSpatialBlindMeanPairScore = bit.Spatial.BlindPhaseMeanPairScore
			evidence.BestSpatialBlindMeanOffset = bit.Spatial.BlindPhaseMeanOffsetBlocks
			evidence.BestSpatialBlindMaxOffset = bit.Spatial.BlindPhaseMaxOffsetBlocks
			evidence.BestSpatialBlindMeanConfidence = bit.Spatial.BlindPhaseMeanConfidence
			evidence.BestSpatialBlindMinConfidence = bit.Spatial.BlindPhaseMinConfidence
			evidence.BestSpatialBlindRobustOutliers = bit.Spatial.BlindPhaseRobustOutliers
			evidence.BestSpatialBlindSecondaryMethod = bit.Spatial.BlindSecondaryMethod
			evidence.BestSpatialBlindSecondaryCells = bit.Spatial.BlindSecondaryCells
			evidence.BestSpatialBlindSecondaryMeanScore = bit.Spatial.BlindSecondaryMeanPairScore
			evidence.BestSpatialBlindConsensusCells = bit.Spatial.BlindConsensusCells
			evidence.BestSpatialBlindCycleSlipFixes = bit.Spatial.BlindCycleSlipCorrections
			evidence.BestSpatialBlindMeanObserverDist = bit.Spatial.BlindMeanObserverDistanceBlocks
			evidence.BestSpatialBlindMaxObserverDist = bit.Spatial.BlindMaxObserverDistanceBlocks
			evidence.BestSpatialBlindLatticeMethod = bit.Spatial.BlindLatticePhaseMethod
			evidence.BestSpatialBlindLatticeCells = bit.Spatial.BlindLatticePhaseCells
			evidence.BestSpatialBlindLatticeMeanConf = bit.Spatial.BlindLatticeMeanConfidence
			evidence.BestSpatialBlindLatticeMinConf = bit.Spatial.BlindLatticeMinConfidence
			evidence.BestSpatialBlindLatticeMeanDist = bit.Spatial.BlindLatticeMeanFractionDistance
			evidence.BestSpatialBlindLatticeMaxDist = bit.Spatial.BlindLatticeMaxFractionDistance
			evidence.BestSpatialBlindLatticeConsensus = bit.Spatial.BlindLatticeConsensusCells
			evidence.BestSpatialBlindLatticeCycleFixes = bit.Spatial.BlindLatticeCycleSlipCorrections
			evidence.BestSpatialBlindGlobalUnwrapMethod = bit.Spatial.BlindGlobalUnwrapMethod
			evidence.BestSpatialBlindGlobalUnwrapStatus = bit.Spatial.BlindGlobalUnwrapStatus
			evidence.BestSpatialBlindGlobalUnwrapStatusX = bit.Spatial.BlindGlobalUnwrapStatusX
			evidence.BestSpatialBlindGlobalUnwrapStatusY = bit.Spatial.BlindGlobalUnwrapStatusY
			evidence.BestSpatialBlindGlobalUnwrapStates = bit.Spatial.BlindGlobalUnwrapEvaluatedStates
			evidence.BestSpatialBlindGlobalUnwrapEligible = bit.Spatial.BlindGlobalUnwrapEligibleCells
			evidence.BestSpatialBlindGlobalUnwrapEligibleX = bit.Spatial.BlindGlobalUnwrapEligibleCellsX
			evidence.BestSpatialBlindGlobalUnwrapEligibleY = bit.Spatial.BlindGlobalUnwrapEligibleCellsY
			evidence.BestSpatialBlindGlobalUnwrapChanged = bit.Spatial.BlindGlobalUnwrapChangedCells
			evidence.BestSpatialBlindGlobalUnwrapProposedChanged = bit.Spatial.BlindGlobalUnwrapProposedChanged
			evidence.BestSpatialBlindGlobalUnwrapAcceptedAxes = bit.Spatial.BlindGlobalUnwrapAcceptedAxes
			evidence.BestSpatialBlindGlobalUnwrapBaseline = bit.Spatial.BlindGlobalUnwrapBaselineObjective
			evidence.BestSpatialBlindGlobalUnwrapObjective = bit.Spatial.BlindGlobalUnwrapObjective
			evidence.BestSpatialBlindGlobalUnwrapAppliedObjective = bit.Spatial.BlindGlobalUnwrapAppliedObjective
			evidence.BestSpatialBlindGlobalUnwrapSecond = bit.Spatial.BlindGlobalUnwrapSecondObjective
			evidence.BestSpatialBlindGlobalUnwrapSecondAvailable = bit.Spatial.BlindGlobalUnwrapSecondAvailable
			evidence.BestSpatialBlindGlobalUnwrapImprovement = bit.Spatial.BlindGlobalUnwrapImprovement
			evidence.BestSpatialBlindGlobalUnwrapMargin = bit.Spatial.BlindGlobalUnwrapMargin
			evidence.BestSpatialBlindGlobalUnwrapAmbiguous = bit.Spatial.BlindGlobalUnwrapAmbiguous
			evidence.BestSpatialBlindGlobalUnwrapValidationMethod = bit.Spatial.BlindGlobalUnwrapValidationMethod
			evidence.BestSpatialBlindGlobalUnwrapValidationAvailable = bit.Spatial.BlindGlobalUnwrapValidationAvailable
			evidence.BestSpatialBlindGlobalUnwrapValidationCells = bit.Spatial.BlindGlobalUnwrapValidationCells
			evidence.BestSpatialBlindGlobalUnwrapValidationPairsFold0 = bit.Spatial.BlindGlobalUnwrapValidationPairsFold0
			evidence.BestSpatialBlindGlobalUnwrapValidationPairsFold1 = bit.Spatial.BlindGlobalUnwrapValidationPairsFold1
			evidence.BestSpatialBlindGlobalUnwrapValidationFold0Delta = bit.Spatial.BlindGlobalUnwrapValidationFold0Delta
			evidence.BestSpatialBlindGlobalUnwrapValidationFold1Delta = bit.Spatial.BlindGlobalUnwrapValidationFold1Delta
			evidence.BestSpatialBlindGlobalUnwrapValidationMeanDelta = bit.Spatial.BlindGlobalUnwrapValidationMeanDelta
			evidence.BestSpatialBlindGlobalUnwrapValidationSupportsBest = bit.Spatial.BlindGlobalUnwrapValidationSupportsBest
			evidence.BestSpatialBlindOracleCells = bit.Spatial.BlindPhaseOracleComparedCells
			evidence.BestSpatialBlindMeanOracleDistance = bit.Spatial.BlindPhaseMeanOracleDistanceBlocks
			evidence.BestSpatialBlindMaxOracleDistance = bit.Spatial.BlindPhaseMaxOracleDistanceBlocks
			evidence.BestSpatialLocalStableWrong = bit.Spatial.LocalPhaseStableWrongBits
			evidence.BestSpatialLocalMixed = bit.Spatial.LocalPhaseMixedBits
			evidence.BestSpatialLocalMajorityECC = bit.Spatial.LocalPhaseMajorityPostECCErrors
			evidence.BestSpatialLocalAgreement = bit.Spatial.LocalPhaseMeanAllCodedAgreement
			evidence.BestSpatialLocalUnstable = bit.Spatial.LocalPhaseUnstableAllCodedFraction
		}
	}
}
