package watermark

import (
	"image"
	"math"
	"sort"
)

const (
	diagnosticMaxProjectiveScaleCandidates = 8
	diagnosticMaxProjectiveFullDecodes     = 4
	diagnosticSyncRepeatsPerAxis           = 4
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
}

func estimateDiagnosticProjective(boundary PrintBoundaryEstimate, pools [][]LocalLatticeEstimate, selected []LocalLatticeEstimate, globalConsistency float64, regionsX, regionsY int) DiagnosticProjectiveEstimate {
	result := DiagnosticProjectiveEstimate{
		Source:             "print-boundary+lattice",
		BoundaryConfidence: boundary.Confidence,
		CandidateBudget:    diagnosticMaxProjectiveScaleCandidates,
	}
	if !boundary.Detected || boundary.Confidence < 0.35 {
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
	result.CanonicalWidthPixels = best.CanonicalWidthPixels
	result.CanonicalHeightPixels = best.CanonicalHeightPixels
	if h, ok := homographyForPrintBoundary(best.CanonicalWidthPixels, best.CanonicalHeightPixels, boundary); ok {
		result.Matrix = h.h
		result.Available = true
	}

	result.LocalProjectiveFit = diagnosticSelectedProjectiveFit(boundary, selected, regionsX, regionsY)
	result.Confidence = clampUnit(boundary.Confidence * (0.30 + 0.70*globalConsistency) * (0.35 + 0.65*best.Confidence) * (0.40 + 0.60*result.LocalProjectiveFit))
	return result
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
			cluster = &diagnosticScaleCluster{bestByRegion: make(map[int]float64), levels: make(map[int]struct{})}
			clusters[k] = cluster
		}
		cluster.widthSum += proposal.width * proposal.score
		cluster.heightSum += proposal.height * proposal.score
		cluster.weight += proposal.score
		if proposal.score > cluster.bestByRegion[proposal.region] {
			cluster.bestByRegion[proposal.region] = proposal.score
		}
		cluster.levels[proposal.divisor] = struct{}{}
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
		confidence := clampUnit((support / 2.0) * (0.45 + 0.55*regionFactor) * levelFactor)
		result = append(result, DiagnosticScaleCandidate{
			CanonicalWidthPixels:  cluster.widthSum / cluster.weight,
			CanonicalHeightPixels: cluster.heightSum / cluster.weight,
			Confidence:            confidence,
			SupportingRegions:     len(cluster.bestByRegion),
			SupportingLevels:      len(cluster.levels),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SupportingRegions != result[j].SupportingRegions {
			return result[i].SupportingRegions > result[j].SupportingRegions
		}
		if result[i].SupportingLevels != result[j].SupportingLevels {
			return result[i].SupportingLevels > result[j].SupportingLevels
		}
		if result[i].Confidence != result[j].Confidence {
			return result[i].Confidence > result[j].Confidence
		}
		if result[i].CanonicalWidthPixels != result[j].CanonicalWidthPixels {
			return result[i].CanonicalWidthPixels < result[j].CanonicalWidthPixels
		}
		return result[i].CanonicalHeightPixels < result[j].CanonicalHeightPixels
	})
	return result
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
	if width <= 1 || height <= 1 || !boundary.Detected {
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
	CandidatesProbed         int                                      `json:"candidates_probed"`
	FullDecodeAttempts       int                                      `json:"full_decode_attempts"`
	ProbeResults             []DiagnosticProjectiveProbeEvidence      `json:"probe_results,omitempty"`
	RefinedResults           []DiagnosticProjectiveRefinementEvidence `json:"refined_results,omitempty"`
	PhaseRefinementAttempts  int                                      `json:"phase_refinement_attempts"`
	PhaseHomographyFits      int                                      `json:"phase_homography_fits"`
	BestSyncProfile          Profile                                  `json:"best_sync_profile,omitempty"`
	BestSyncFraction         float64                                  `json:"best_sync_fraction"`
	BestSyncZScore           float64                                  `json:"best_sync_z_score"`
	BestCanonicalWidthPx     float64                                  `json:"best_canonical_width_px,omitempty"`
	BestCanonicalHeightPx    float64                                  `json:"best_canonical_height_px,omitempty"`
	BestPhaseProfile         Profile                                  `json:"best_phase_profile,omitempty"`
	BestPhaseConsistency     float64                                  `json:"best_phase_consistency"`
	BestPhaseXCoherence      float64                                  `json:"best_phase_x_coherence"`
	BestPhaseYCoherence      float64                                  `json:"best_phase_y_coherence"`
	BestPhaseCanonicalWidth  float64                                  `json:"best_phase_canonical_width_px,omitempty"`
	BestPhaseCanonicalHeight float64                                  `json:"best_phase_canonical_height_px,omitempty"`
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
			auth := diagnosticAuthCandidate{source: "coarse", candidate: candidate, phase: phase, h: h, probe: probe}
			authCandidates = append(authCandidates, auth)
			phaseSeeds = append(phaseSeeds, auth)
		}
		updateBest(candidate, phase, probe)
	}

	sortDiagnosticAuthCandidates(phaseSeeds)
	seedLimit := diagnosticMaxPhaseRefineSeeds
	if len(phaseSeeds) < seedLimit {
		seedLimit = len(phaseSeeds)
	}
	for _, seed := range phaseSeeds[:seedLimit] {
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
		authCandidates = append(authCandidates, diagnosticAuthCandidate{source: "phase-scale", candidate: refined, phase: phase, h: h, probe: probe})
		updateBest(refined, phase, probe)

		phaseH, ok := diagnosticPhaseRefinedHomography(src, boundary, refined, phase, decoder)
		if !ok {
			continue
		}
		evidence.PhaseHomographyFits++
		phaseAfter := diagnosticProjectivePhaseConsensusWithHomography(src, width, height, phaseH, decoder)
		phaseProbe := diagnosticProbeProjectiveSyncWithHomography(src, refined, phaseH, decoder)
		evidence.RefinedResults = append(evidence.RefinedResults, diagnosticRefinementEvidence("phase-dlt", refined, phaseAfter, phaseProbe))
		authCandidates = append(authCandidates, diagnosticAuthCandidate{source: "phase-dlt", candidate: refined, phase: phaseAfter, h: phaseH, probe: phaseProbe})
		updateBest(refined, phaseAfter, phaseProbe)
	}

	sortDiagnosticAuthCandidates(authCandidates)
	limit := diagnosticMaxProjectiveFullDecodes
	if len(authCandidates) < limit {
		limit = len(authCandidates)
	}
	for _, candidate := range authCandidates[:limit] {
		width := int(math.Round(candidate.candidate.CanonicalWidthPixels))
		height := int(math.Round(candidate.candidate.CanonicalHeightPixels))
		grid, ok := diagnosticProjectiveGridWithHomography(src, width, height, candidate.h)
		if !ok {
			continue
		}
		evidence.FullDecodeAttempts++
		payload, info, _, found := decoder.decodeGrid(grid)
		if found {
			info.PerspectiveCorrection = "diagnostic-projective-" + candidate.source
			return payload, info, evidence, true
		}
	}
	return nil, ExtractInfo{}, evidence, false
}

func diagnosticRefinementEvidence(source string, candidate DiagnosticScaleCandidate, phase diagnosticPhaseConsensus, probe diagnosticProjectiveProbe) DiagnosticProjectiveRefinementEvidence {
	return DiagnosticProjectiveRefinementEvidence{
		Source: source, CanonicalWidthPx: candidate.CanonicalWidthPixels, CanonicalHeightPx: candidate.CanonicalHeightPixels,
		PhaseProfile: phase.profile, PhaseConsistency: phase.score, PhaseXCoherence: phase.xCoherence,
		PhaseYCoherence: phase.yCoherence, PhaseMeanZScore: phase.meanZ,
		SyncProfile: probe.profile, SyncFraction: probe.fraction, SyncZScore: probe.z,
	}
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
	width := int(math.Round(candidate.CanonicalWidthPixels))
	height := int(math.Round(candidate.CanonicalHeightPixels))
	if width < tileWidth*blockSize || height < tileHeight*blockSize {
		return diagnosticProjectiveProbe{candidate: candidate}
	}
	bw, bh := width/blockSize, height/blockSize
	tileXs := diagnosticSpacedIndices((bw+tileWidth-1)/tileWidth, diagnosticSyncRepeatsPerAxis)
	tileYs := diagnosticSpacedIndices((bh+tileHeight-1)/tileHeight, diagnosticSyncRepeatsPerAxis)
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
					margin, ok := diagnosticReadProjectiveBlock(src, h, float64(bx*blockSize), float64(by*blockSize))
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
	if width < tileWidth*blockSize || height < tileHeight*blockSize {
		return nil, false
	}
	bw, bh := width/blockSize, height/blockSize
	grid := make([]float64, eccBits)
	counts := make([]int, eccBits)
	for by := 0; by < bh; by++ {
		for bx := 0; bx < bw; bx++ {
			margin, ok := diagnosticReadProjectiveBlock(src, h, float64(bx*blockSize), float64(by*blockSize))
			if !ok {
				continue
			}
			position := (by%tileHeight)*tileWidth + bx%tileWidth
			grid[position] += margin
			counts[position]++
		}
	}
	for _, count := range counts {
		if count == 0 {
			return nil, false
		}
	}
	return grid, true
}

func diagnosticReadProjectiveBlock(src image.Image, h homography, originX, originY float64) (float64, bool) {
	coefficient23, coefficient32 := 0.0, 0.0
	for y := 0; y < blockSize; y++ {
		for x := 0; x < blockSize; x++ {
			sx, sy, ok := h.mapPoint(originX+float64(x), originY+float64(y))
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
