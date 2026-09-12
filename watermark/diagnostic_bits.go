package watermark

import (
	"math"
	"sort"
)

const diagnosticSpatialPhaseRadius = 2

// DiagnosticBitChannelEvidence measures the protected v3 bit channel for one
// geometry/photometric candidate using only information that is known before
// payload recovery. The first three frame bytes (magic + version/profile) are
// fixed by Format v3, so their six Hamming(7,4) words can be compared exactly
// without knowing the hidden message. None of these fields authenticate data.
type DiagnosticBitChannelEvidence struct {
	GeometrySource string  `json:"geometry_source"`
	Mode           string  `json:"mode"`
	Profile        Profile `json:"profile,omitempty"`
	PhaseX         int     `json:"phase_x"`
	PhaseY         int     `json:"phase_y"`

	KnownSyncBits             int  `json:"known_sync_bits"`
	KnownSyncBitErrors        int  `json:"known_sync_bit_errors"`
	KnownHeaderCodedBits      int  `json:"known_header_coded_bits"`
	KnownHeaderCodedErrors    int  `json:"known_header_coded_bit_errors"`
	KnownHeaderCleanWords     int  `json:"known_header_hamming_clean_words"`
	KnownHeaderOneBitWords    int  `json:"known_header_hamming_one_bit_words"`
	KnownHeaderMultiWords     int  `json:"known_header_hamming_multi_error_words"`
	PostECCHdrBits            int  `json:"post_ecc_known_header_bits"`
	PostECCHdrErrors          int  `json:"post_ecc_known_header_bit_errors"`
	SoftPostECCHdrErrors      int  `json:"soft_post_ecc_known_header_bit_errors"`
	SoftChangedWords          int  `json:"soft_hamming_changed_words"`
	SoftHeaderImprovement     int  `json:"soft_known_header_bit_improvement"`
	ReliabilityDecodeEligible bool `json:"reliability_decode_eligible"`

	HammingWords          int                            `json:"hamming_words"`
	SyndromeWords         int                            `json:"hamming_nonzero_syndrome_words"`
	SyndromeWordFraction  float64                        `json:"hamming_nonzero_syndrome_fraction"`
	MeanAbsCodedMargin    float64                        `json:"mean_absolute_coded_margin"`
	MedianAbsCodedMargin  float64                        `json:"median_absolute_coded_margin"`
	P10AbsCodedMargin     float64                        `json:"p10_absolute_coded_margin"`
	KnownCorrectMeanAbs   float64                        `json:"known_correct_mean_absolute_margin"`
	KnownWrongMeanAbs     float64                        `json:"known_wrong_mean_absolute_margin"`
	KnownWrongMarginRatio float64                        `json:"known_wrong_to_correct_margin_ratio"`
	Spatial               *DiagnosticSpatialBitEvidence  `json:"spatial,omitempty"`
	SmoothPhase           *DiagnosticSmoothPhaseEvidence `json:"smooth_phase,omitempty"`
}

// DiagnosticSpatialCellEvidence reports the known-prefix error count observed
// in one coarse 3x3 spatial cell. It never authenticates payload data.
type DiagnosticSpatialCellEvidence struct {
	RegionX                         int     `json:"region_x"`
	RegionY                         int     `json:"region_y"`
	KnownHeaderCodedErrors          int     `json:"known_header_coded_bit_errors"`
	PostECCHeaderErrors             int     `json:"post_ecc_known_header_bit_errors"`
	LocalPhaseX                     int     `json:"local_phase_x"`
	LocalPhaseY                     int     `json:"local_phase_y"`
	LocalPhaseOffsetX               int     `json:"local_phase_offset_x"`
	LocalPhaseOffsetY               int     `json:"local_phase_offset_y"`
	LocalPhaseSyncFraction          float64 `json:"local_phase_sync_fraction"`
	LocalPhaseSubblockAvailable     bool    `json:"local_phase_subblock_available"`
	LocalPhaseSubblockOffsetX       float64 `json:"local_phase_subblock_offset_x"`
	LocalPhaseSubblockOffsetY       float64 `json:"local_phase_subblock_offset_y"`
	LocalPhasePeakScore             float64 `json:"local_phase_peak_score"`
	LocalPhaseSecondScore           float64 `json:"local_phase_second_score"`
	LocalPhaseConfidence            float64 `json:"local_phase_confidence"`
	LocalPhaseAtSearchBoundary      bool    `json:"local_phase_at_search_boundary"`
	LocalPhaseKnownCodedErrors      int     `json:"local_phase_known_header_coded_bit_errors"`
	LocalPhasePostECCHeaderErrors   int     `json:"local_phase_post_ecc_known_header_bit_errors"`
	BlindPhaseAvailable             bool    `json:"blind_phase_available"`
	BlindPhaseOffsetX               float64 `json:"blind_phase_offset_x"`
	BlindPhaseOffsetY               float64 `json:"blind_phase_offset_y"`
	BlindPhaseConfidence            float64 `json:"blind_phase_confidence"`
	BlindPhasePairSupport           int     `json:"blind_phase_pair_support"`
	BlindPhaseResidualBlocks        float64 `json:"blind_phase_pair_residual_blocks"`
	BlindSecondaryAvailable         bool    `json:"blind_secondary_available"`
	BlindSecondaryOffsetX           float64 `json:"blind_secondary_offset_x"`
	BlindSecondaryOffsetY           float64 `json:"blind_secondary_offset_y"`
	BlindSecondaryConfidence        float64 `json:"blind_secondary_confidence"`
	BlindObserverDistanceBlocks     float64 `json:"blind_observer_distance_blocks"`
	BlindCycleSlipAdjusted          bool    `json:"blind_cycle_slip_adjusted"`
	BlindLatticePhaseAvailable      bool    `json:"blind_lattice_phase_available"`
	BlindLatticePhaseOffsetX        float64 `json:"blind_lattice_phase_offset_x"`
	BlindLatticePhaseOffsetY        float64 `json:"blind_lattice_phase_offset_y"`
	BlindLatticePhaseConfidence     float64 `json:"blind_lattice_phase_confidence"`
	BlindLatticeFractionDistance    float64 `json:"blind_lattice_fraction_distance_blocks"`
	BlindLatticeCycleSlipAdjusted   bool    `json:"blind_lattice_cycle_slip_adjusted"`
	BlindGlobalUnwrapAdjusted       bool    `json:"blind_global_unwrap_adjusted"`
	BlindGlobalUnwrapShiftX         int     `json:"blind_global_unwrap_shift_x"`
	BlindGlobalUnwrapShiftY         int     `json:"blind_global_unwrap_shift_y"`
	BlindGlobalUnwrapProposedShiftX int     `json:"blind_global_unwrap_proposed_shift_x"`
	BlindGlobalUnwrapProposedShiftY int     `json:"blind_global_unwrap_proposed_shift_y"`
	BlindCrossfitAToBAvailable      bool    `json:"blind_crossfit_a_to_b_available"`
	BlindCrossfitAToBCycleX         int     `json:"blind_crossfit_a_to_b_cycle_x"`
	BlindCrossfitAToBCycleY         int     `json:"blind_crossfit_a_to_b_cycle_y"`
	BlindCrossfitAToBConfidence     float64 `json:"blind_crossfit_a_to_b_confidence"`
	BlindCrossfitBToAAvailable      bool    `json:"blind_crossfit_b_to_a_available"`
	BlindCrossfitBToACycleX         int     `json:"blind_crossfit_b_to_a_cycle_x"`
	BlindCrossfitBToACycleY         int     `json:"blind_crossfit_b_to_a_cycle_y"`
	BlindCrossfitBToAConfidence     float64 `json:"blind_crossfit_b_to_a_confidence"`
	BlindCrossfitCycleAgreement     bool    `json:"blind_crossfit_cycle_agreement"`
	BlindStabilityAvailable         bool    `json:"blind_stability_available"`
	BlindStabilityObservations      int     `json:"blind_stability_observations"`
	BlindStabilityUniqueCycles      int     `json:"blind_stability_unique_cycles"`
	BlindStabilityModalCycleX       int     `json:"blind_stability_modal_cycle_x"`
	BlindStabilityModalCycleY       int     `json:"blind_stability_modal_cycle_y"`
	BlindStabilityModalCount        int     `json:"blind_stability_modal_count"`
	BlindStabilityModalFraction     float64 `json:"blind_stability_modal_fraction"`
	BlindStabilitySupportedObs      int     `json:"blind_stability_supported_observations"`
	BlindStabilitySupportedUnique   int     `json:"blind_stability_supported_unique_cycles"`
	BlindStabilitySupportedModalX   int     `json:"blind_stability_supported_modal_cycle_x"`
	BlindStabilitySupportedModalY   int     `json:"blind_stability_supported_modal_cycle_y"`
	BlindStabilitySupportedCount    int     `json:"blind_stability_supported_modal_count"`
	BlindStabilitySupportedFraction float64 `json:"blind_stability_supported_modal_fraction"`
	BlindCycleAnchorAvailable       bool    `json:"blind_cycle_anchor_available"`
	BlindCycleAnchorOffsetX         float64 `json:"blind_cycle_anchor_offset_x"`
	BlindCycleAnchorOffsetY         float64 `json:"blind_cycle_anchor_offset_y"`
	BlindCycleAnchorConfidence      float64 `json:"blind_cycle_anchor_confidence"`
	BlindCycleAnchorTop1CycleX      int     `json:"blind_cycle_anchor_top1_cycle_x"`
	BlindCycleAnchorTop1CycleY      int     `json:"blind_cycle_anchor_top1_cycle_y"`
	BlindCycleAnchorSecondCycleX    int     `json:"blind_cycle_anchor_second_cycle_x"`
	BlindCycleAnchorSecondCycleY    int     `json:"blind_cycle_anchor_second_cycle_y"`
	BlindPhaseOracleDistanceBlocks  float64 `json:"blind_phase_oracle_distance_blocks"`
}

// DiagnosticSpatialBitEvidence measures whether protected coded-bit signs are
// stable across spatially separated repetitions of the v3 tile. The expected
// values are used only for the fixed three-byte Format-v3 prefix; whole-stream
// agreement metrics remain message-independent. These numbers are research
// evidence only and cannot authenticate a watermark.
type DiagnosticSpatialBitEvidence struct {
	Cells                                               int                             `json:"cells"`
	MeanTilePositionSignAgreement                       float64                         `json:"mean_tile_position_sign_agreement"`
	UnstableTilePositions                               int                             `json:"unstable_tile_positions"`
	UnstableTilePositionFraction                        float64                         `json:"unstable_tile_position_fraction"`
	KnownHeaderCodedBits                                int                             `json:"known_header_coded_bits"`
	KnownHeaderStableCorrectBits                        int                             `json:"known_header_stable_correct_bits"`
	KnownHeaderStableWrongBits                          int                             `json:"known_header_stable_wrong_bits"`
	KnownHeaderMixedBits                                int                             `json:"known_header_mixed_bits"`
	KnownHeaderMajorityCodedErrors                      int                             `json:"known_header_majority_coded_bit_errors"`
	KnownHeaderMajorityPostECCErrors                    int                             `json:"known_header_majority_post_ecc_bit_errors"`
	KnownHeaderAggregatePostECCErrors                   int                             `json:"known_header_aggregate_post_ecc_bit_errors"`
	KnownHeaderMajorityECCImprovement                   int                             `json:"known_header_majority_ecc_improvement"`
	MeanKnownHeaderCorrectCellFraction                  float64                         `json:"mean_known_header_correct_cell_fraction"`
	MeanAllCodedBitAgreement                            float64                         `json:"mean_all_coded_bit_agreement"`
	UnstableAllCodedBits                                int                             `json:"unstable_all_coded_bits"`
	UnstableAllCodedBitFraction                         float64                         `json:"unstable_all_coded_bit_fraction"`
	LocalPhaseCells                                     int                             `json:"local_phase_cells"`
	LocalPhaseSameAsGlobalCells                         int                             `json:"local_phase_same_as_global_cells"`
	MeanLocalPhaseOffsetBlocks                          float64                         `json:"mean_local_phase_offset_blocks"`
	MaxLocalPhaseOffsetBlocks                           float64                         `json:"max_local_phase_offset_blocks"`
	MeanLocalPhaseSubblockOffsetBlocks                  float64                         `json:"mean_local_phase_subblock_offset_blocks"`
	MaxLocalPhaseSubblockOffsetBlocks                   float64                         `json:"max_local_phase_subblock_offset_blocks"`
	MeanLocalPhaseConfidence                            float64                         `json:"mean_local_phase_confidence"`
	MinLocalPhaseConfidence                             float64                         `json:"min_local_phase_confidence"`
	LocalPhaseStableWrongBits                           int                             `json:"local_phase_known_header_stable_wrong_bits"`
	LocalPhaseMixedBits                                 int                             `json:"local_phase_known_header_mixed_bits"`
	LocalPhaseMajorityCodedErrors                       int                             `json:"local_phase_majority_coded_bit_errors"`
	LocalPhaseMajorityPostECCErrors                     int                             `json:"local_phase_majority_post_ecc_bit_errors"`
	LocalPhaseMeanAllCodedAgreement                     float64                         `json:"local_phase_mean_all_coded_bit_agreement"`
	LocalPhaseUnstableAllCodedFraction                  float64                         `json:"local_phase_unstable_all_coded_bit_fraction"`
	BlindPhaseMethod                                    string                          `json:"blind_phase_method,omitempty"`
	BlindPhaseProfile                                   Profile                         `json:"blind_phase_profile,omitempty"`
	BlindPhaseGlobalX                                   int                             `json:"blind_phase_global_x"`
	BlindPhaseGlobalY                                   int                             `json:"blind_phase_global_y"`
	BlindPhaseGlobalScore                               float64                         `json:"blind_phase_global_score"`
	BlindPhaseCells                                     int                             `json:"blind_phase_cells"`
	BlindPhasePairs                                     int                             `json:"blind_phase_pairs"`
	BlindPhaseMeanPairScore                             float64                         `json:"blind_phase_mean_pair_score"`
	BlindPhaseMeanOffsetBlocks                          float64                         `json:"blind_phase_mean_offset_blocks"`
	BlindPhaseMaxOffsetBlocks                           float64                         `json:"blind_phase_max_offset_blocks"`
	BlindPhaseMeanConfidence                            float64                         `json:"blind_phase_mean_confidence"`
	BlindPhaseMinConfidence                             float64                         `json:"blind_phase_min_confidence"`
	BlindPhaseRobustOutliers                            int                             `json:"blind_phase_robust_pair_outliers"`
	BlindSecondaryMethod                                string                          `json:"blind_secondary_method,omitempty"`
	BlindSecondaryCells                                 int                             `json:"blind_secondary_cells"`
	BlindSecondaryMeanPairScore                         float64                         `json:"blind_secondary_mean_pair_score"`
	BlindConsensusCells                                 int                             `json:"blind_consensus_cells"`
	BlindCycleSlipCorrections                           int                             `json:"blind_cycle_slip_corrections"`
	BlindMeanObserverDistanceBlocks                     float64                         `json:"blind_mean_observer_distance_blocks"`
	BlindMaxObserverDistanceBlocks                      float64                         `json:"blind_max_observer_distance_blocks"`
	BlindLatticePhaseMethod                             string                          `json:"blind_lattice_phase_method,omitempty"`
	BlindLatticePhaseCells                              int                             `json:"blind_lattice_phase_cells"`
	BlindLatticeMeanConfidence                          float64                         `json:"blind_lattice_phase_mean_confidence"`
	BlindLatticeMinConfidence                           float64                         `json:"blind_lattice_phase_min_confidence"`
	BlindLatticeMeanFractionDistance                    float64                         `json:"blind_lattice_mean_fraction_distance_blocks"`
	BlindLatticeMaxFractionDistance                     float64                         `json:"blind_lattice_max_fraction_distance_blocks"`
	BlindLatticeConsensusCells                          int                             `json:"blind_lattice_consensus_cells"`
	BlindLatticeCycleSlipCorrections                    int                             `json:"blind_lattice_cycle_slip_corrections"`
	BlindGlobalUnwrapMethod                             string                          `json:"blind_global_unwrap_method,omitempty"`
	BlindGlobalUnwrapStatus                             string                          `json:"blind_global_unwrap_status,omitempty"`
	BlindGlobalUnwrapStatusX                            string                          `json:"blind_global_unwrap_status_x,omitempty"`
	BlindGlobalUnwrapStatusY                            string                          `json:"blind_global_unwrap_status_y,omitempty"`
	BlindGlobalUnwrapEvaluatedStates                    int                             `json:"blind_global_unwrap_evaluated_states"`
	BlindGlobalUnwrapEligibleCells                      int                             `json:"blind_global_unwrap_eligible_cells"`
	BlindGlobalUnwrapEligibleCellsX                     int                             `json:"blind_global_unwrap_eligible_cells_x"`
	BlindGlobalUnwrapEligibleCellsY                     int                             `json:"blind_global_unwrap_eligible_cells_y"`
	BlindGlobalUnwrapChangedCells                       int                             `json:"blind_global_unwrap_changed_cells"`
	BlindGlobalUnwrapProposedChanged                    int                             `json:"blind_global_unwrap_proposed_changed_cells"`
	BlindGlobalUnwrapAcceptedAxes                       int                             `json:"blind_global_unwrap_accepted_axes"`
	BlindGlobalUnwrapBaselineObjective                  float64                         `json:"blind_global_unwrap_baseline_objective"`
	BlindGlobalUnwrapObjective                          float64                         `json:"blind_global_unwrap_proposed_objective"`
	BlindGlobalUnwrapAppliedObjective                   float64                         `json:"blind_global_unwrap_applied_objective"`
	BlindGlobalUnwrapSecondObjective                    float64                         `json:"blind_global_unwrap_second_objective"`
	BlindGlobalUnwrapSecondAvailable                    bool                            `json:"blind_global_unwrap_second_objective_available"`
	BlindGlobalUnwrapImprovement                        float64                         `json:"blind_global_unwrap_improvement"`
	BlindGlobalUnwrapMargin                             float64                         `json:"blind_global_unwrap_margin"`
	BlindGlobalUnwrapAmbiguous                          bool                            `json:"blind_global_unwrap_ambiguous"`
	BlindGlobalUnwrapValidationMethod                   string                          `json:"blind_global_unwrap_validation_method,omitempty"`
	BlindGlobalUnwrapValidationAvailable                bool                            `json:"blind_global_unwrap_validation_available"`
	BlindGlobalUnwrapValidationCells                    int                             `json:"blind_global_unwrap_validation_cells"`
	BlindGlobalUnwrapValidationPairsFold0               int                             `json:"blind_global_unwrap_validation_pairs_fold0"`
	BlindGlobalUnwrapValidationPairsFold1               int                             `json:"blind_global_unwrap_validation_pairs_fold1"`
	BlindGlobalUnwrapValidationFold0Delta               float64                         `json:"blind_global_unwrap_validation_fold0_delta"`
	BlindGlobalUnwrapValidationFold1Delta               float64                         `json:"blind_global_unwrap_validation_fold1_delta"`
	BlindGlobalUnwrapValidationMeanDelta                float64                         `json:"blind_global_unwrap_validation_mean_delta"`
	BlindGlobalUnwrapValidationSupportsBest             bool                            `json:"blind_global_unwrap_validation_supports_best"`
	BlindGlobalUnwrapCrossfitMethod                     string                          `json:"blind_global_unwrap_crossfit_method,omitempty"`
	BlindGlobalUnwrapCrossfitAvailable                  bool                            `json:"blind_global_unwrap_crossfit_available"`
	BlindGlobalUnwrapCrossfitAToBAvailable              bool                            `json:"blind_global_unwrap_crossfit_a_to_b_available"`
	BlindGlobalUnwrapCrossfitBToAAvailable              bool                            `json:"blind_global_unwrap_crossfit_b_to_a_available"`
	BlindGlobalUnwrapCrossfitAToBProfile                Profile                         `json:"blind_global_unwrap_crossfit_a_to_b_profile,omitempty"`
	BlindGlobalUnwrapCrossfitBToAProfile                Profile                         `json:"blind_global_unwrap_crossfit_b_to_a_profile,omitempty"`
	BlindGlobalUnwrapCrossfitProposalPairsA             int                             `json:"blind_global_unwrap_crossfit_proposal_pairs_a"`
	BlindGlobalUnwrapCrossfitProposalPairsB             int                             `json:"blind_global_unwrap_crossfit_proposal_pairs_b"`
	BlindGlobalUnwrapCrossfitValidationPairsA           int                             `json:"blind_global_unwrap_crossfit_validation_pairs_a"`
	BlindGlobalUnwrapCrossfitValidationPairsB           int                             `json:"blind_global_unwrap_crossfit_validation_pairs_b"`
	BlindGlobalUnwrapCrossfitAToBCells                  int                             `json:"blind_global_unwrap_crossfit_a_to_b_cells"`
	BlindGlobalUnwrapCrossfitBToACells                  int                             `json:"blind_global_unwrap_crossfit_b_to_a_cells"`
	BlindGlobalUnwrapCrossfitAToBStates                 int                             `json:"blind_global_unwrap_crossfit_a_to_b_evaluated_states"`
	BlindGlobalUnwrapCrossfitBToAStates                 int                             `json:"blind_global_unwrap_crossfit_b_to_a_evaluated_states"`
	BlindGlobalUnwrapCrossfitAToBMargin                 float64                         `json:"blind_global_unwrap_crossfit_a_to_b_margin"`
	BlindGlobalUnwrapCrossfitBToAMargin                 float64                         `json:"blind_global_unwrap_crossfit_b_to_a_margin"`
	BlindGlobalUnwrapCrossfitAToBDelta                  float64                         `json:"blind_global_unwrap_crossfit_a_to_b_validation_delta"`
	BlindGlobalUnwrapCrossfitBToADelta                  float64                         `json:"blind_global_unwrap_crossfit_b_to_a_validation_delta"`
	BlindGlobalUnwrapCrossfitAToBSupportsBest           bool                            `json:"blind_global_unwrap_crossfit_a_to_b_supports_best"`
	BlindGlobalUnwrapCrossfitBToASupportsBest           bool                            `json:"blind_global_unwrap_crossfit_b_to_a_supports_best"`
	BlindGlobalUnwrapCrossfitComparedCells              int                             `json:"blind_global_unwrap_crossfit_compared_cells"`
	BlindGlobalUnwrapCrossfitAgreementCells             int                             `json:"blind_global_unwrap_crossfit_agreement_cells"`
	BlindGlobalUnwrapCrossfitAgreementFraction          float64                         `json:"blind_global_unwrap_crossfit_agreement_fraction"`
	BlindGlobalUnwrapCrossfitAgreementMeanConfidence    float64                         `json:"blind_global_unwrap_crossfit_agreement_mean_confidence"`
	BlindGlobalUnwrapCrossfitDisagreementMeanConfidence float64                         `json:"blind_global_unwrap_crossfit_disagreement_mean_confidence"`
	BlindGlobalUnwrapCrossfitAgreementMeanLattice       float64                         `json:"blind_global_unwrap_crossfit_agreement_mean_lattice_confidence"`
	BlindGlobalUnwrapCrossfitDisagreementMeanLattice    float64                         `json:"blind_global_unwrap_crossfit_disagreement_mean_lattice_confidence"`
	BlindGlobalUnwrapCrossfitSupportsBest               bool                            `json:"blind_global_unwrap_crossfit_supports_best"`
	BlindGlobalUnwrapStabilityMethod                    string                          `json:"blind_global_unwrap_stability_method,omitempty"`
	BlindGlobalUnwrapStabilityAvailable                 bool                            `json:"blind_global_unwrap_stability_available"`
	BlindGlobalUnwrapStabilityPartitions                int                             `json:"blind_global_unwrap_stability_partitions"`
	BlindGlobalUnwrapStabilityTrialsRequested           int                             `json:"blind_global_unwrap_stability_trials_requested"`
	BlindGlobalUnwrapStabilityTrialsAvailable           int                             `json:"blind_global_unwrap_stability_trials_available"`
	BlindGlobalUnwrapStabilityTrialsSupported           int                             `json:"blind_global_unwrap_stability_trials_supported"`
	BlindGlobalUnwrapStabilitySupportedFraction         float64                         `json:"blind_global_unwrap_stability_supported_trial_fraction"`
	BlindGlobalUnwrapStabilityEvaluatedStates           int                             `json:"blind_global_unwrap_stability_evaluated_states"`
	BlindGlobalUnwrapStabilityMeanMargin                float64                         `json:"blind_global_unwrap_stability_mean_margin"`
	BlindGlobalUnwrapStabilityMeanValidationDelta       float64                         `json:"blind_global_unwrap_stability_mean_validation_delta"`
	BlindGlobalUnwrapStabilityCompleteFields            int                             `json:"blind_global_unwrap_stability_complete_field_trials"`
	BlindGlobalUnwrapStabilityUniqueFields              int                             `json:"blind_global_unwrap_stability_unique_fields"`
	BlindGlobalUnwrapStabilityModalFieldCount           int                             `json:"blind_global_unwrap_stability_modal_field_count"`
	BlindGlobalUnwrapStabilityModalFieldFraction        float64                         `json:"blind_global_unwrap_stability_modal_field_fraction"`
	BlindGlobalUnwrapStabilitySupportedCompleteFields   int                             `json:"blind_global_unwrap_stability_supported_complete_field_trials"`
	BlindGlobalUnwrapStabilitySupportedUniqueFields     int                             `json:"blind_global_unwrap_stability_supported_unique_fields"`
	BlindGlobalUnwrapStabilitySupportedModalFieldCount  int                             `json:"blind_global_unwrap_stability_supported_modal_field_count"`
	BlindGlobalUnwrapStabilitySupportedModalFieldFrac   float64                         `json:"blind_global_unwrap_stability_supported_modal_field_fraction"`
	BlindGlobalUnwrapStabilityComparedCells             int                             `json:"blind_global_unwrap_stability_compared_cells"`
	BlindGlobalUnwrapStabilityUnanimousCells            int                             `json:"blind_global_unwrap_stability_unanimous_cells"`
	BlindGlobalUnwrapStabilityMeanCellModalFraction     float64                         `json:"blind_global_unwrap_stability_mean_cell_modal_fraction"`
	BlindGlobalUnwrapStabilityMinCellModalFraction      float64                         `json:"blind_global_unwrap_stability_min_cell_modal_fraction"`
	BlindGlobalUnwrapStabilitySupportedComparedCells    int                             `json:"blind_global_unwrap_stability_supported_compared_cells"`
	BlindGlobalUnwrapStabilitySupportedUnanimousCells   int                             `json:"blind_global_unwrap_stability_supported_unanimous_cells"`
	BlindGlobalUnwrapStabilitySupportedMeanCellModal    float64                         `json:"blind_global_unwrap_stability_supported_mean_cell_modal_fraction"`
	BlindGlobalUnwrapStabilitySupportedMinCellModal     float64                         `json:"blind_global_unwrap_stability_supported_min_cell_modal_fraction"`
	BlindGlobalUnwrapStabilityMeanPairwiseAgreement     float64                         `json:"blind_global_unwrap_stability_mean_pairwise_agreement_fraction"`
	BlindGlobalUnwrapCycleAnchorMethod                  string                          `json:"blind_global_unwrap_cycle_anchor_method,omitempty"`
	BlindGlobalUnwrapCycleAnchorAvailable               bool                            `json:"blind_global_unwrap_cycle_anchor_available"`
	BlindGlobalUnwrapCycleAnchorPairs                   int                             `json:"blind_global_unwrap_cycle_anchor_pairs"`
	BlindGlobalUnwrapCycleAnchorCells                   int                             `json:"blind_global_unwrap_cycle_anchor_cells"`
	BlindGlobalUnwrapCycleAnchorMeanConfidence          float64                         `json:"blind_global_unwrap_cycle_anchor_mean_confidence"`
	BlindGlobalUnwrapCycleAnchorTop1Objective           float64                         `json:"blind_global_unwrap_cycle_anchor_top1_objective"`
	BlindGlobalUnwrapCycleAnchorSecondObjective         float64                         `json:"blind_global_unwrap_cycle_anchor_second_objective"`
	BlindGlobalUnwrapCycleAnchorDelta                   float64                         `json:"blind_global_unwrap_cycle_anchor_delta_second_minus_top1"`
	BlindGlobalUnwrapCycleAnchorPrefersTop1             bool                            `json:"blind_global_unwrap_cycle_anchor_prefers_top1"`
	BlindGlobalUnwrapCycleAnchorTop1AgreementCells      int                             `json:"blind_global_unwrap_cycle_anchor_top1_agreement_cells"`
	BlindGlobalUnwrapCycleAnchorTop1AgreementFraction   float64                         `json:"blind_global_unwrap_cycle_anchor_top1_agreement_fraction"`
	BlindGlobalUnwrapCycleAnchorSecondAgreementCells    int                             `json:"blind_global_unwrap_cycle_anchor_second_agreement_cells"`
	BlindGlobalUnwrapCycleAnchorSecondAgreementFraction float64                         `json:"blind_global_unwrap_cycle_anchor_second_agreement_fraction"`
	BlindPhaseOracleComparedCells                       int                             `json:"blind_phase_oracle_compared_cells"`
	BlindPhaseMeanOracleDistanceBlocks                  float64                         `json:"blind_phase_mean_oracle_distance_blocks"`
	BlindPhaseMaxOracleDistanceBlocks                   float64                         `json:"blind_phase_max_oracle_distance_blocks"`
	CellsEvidence                                       []DiagnosticSpatialCellEvidence `json:"cell_evidence,omitempty"`
}

func diagnosticAnalyzeBitChannel(grid []float64, key []byte, decoder *decoder, geometrySource string, mode diagnosticPhotometricMode) (DiagnosticBitChannelEvidence, bool) {
	if len(grid) < eccBits || decoder == nil || len(key) < 8 {
		return DiagnosticBitChannelEvidence{}, false
	}

	bestPattern := -1
	bestPhase := v3PhaseCandidate{}
	bestZ := math.Inf(-1)
	for i, pattern := range decoder.v3Patterns {
		phases := strongestV3Phases(grid, pattern, 1)
		if len(phases) == 0 || phases[0].total == 0 {
			continue
		}
		phase := phases[0]
		z := (float64(phase.score) - float64(phase.total)/2) / math.Sqrt(float64(phase.total)/4)
		if z > bestZ {
			bestPattern = i
			bestPhase = phase
			bestZ = z
		}
	}
	if bestPattern < 0 {
		return DiagnosticBitChannelEvidence{}, false
	}

	pattern := decoder.v3Patterns[bestPattern]
	coded, margins := diagnosticReadV3ProtectedMargins(grid, bestPhase.x, bestPhase.y, pattern.spec.codedBits)
	if len(coded) != pattern.spec.codedBits || len(margins) != pattern.spec.codedBits {
		return DiagnosticBitChannelEvidence{}, false
	}

	evidence := DiagnosticBitChannelEvidence{
		GeometrySource:     geometrySource,
		Mode:               mode.String(),
		Profile:            pattern.spec.profile,
		PhaseX:             bestPhase.x,
		PhaseY:             bestPhase.y,
		KnownSyncBits:      bestPhase.total,
		KnownSyncBitErrors: bestPhase.total - bestPhase.score,
	}

	knownBytes := []byte{magic[0], magic[1], v3HeaderByte(pattern.spec)}
	knownRaw := bytesToBits(knownBytes)
	knownCoded := hammingEncode(whiten(knownRaw, key, v3WhitenLabel))
	evidence.KnownHeaderCodedBits = len(knownCoded)
	correctMarginSum, wrongMarginSum := 0.0, 0.0
	correctMargins, wrongMargins := 0, 0
	for i, expected := range knownCoded {
		if i >= len(coded) {
			break
		}
		if coded[i] != expected {
			evidence.KnownHeaderCodedErrors++
			wrongMarginSum += math.Abs(margins[i])
			wrongMargins++
		} else {
			correctMarginSum += math.Abs(margins[i])
			correctMargins++
		}
	}
	for start := 0; start+7 <= len(knownCoded); start += 7 {
		errors := 0
		for i := 0; i < 7; i++ {
			if coded[start+i] != knownCoded[start+i] {
				errors++
			}
		}
		switch errors {
		case 0:
			evidence.KnownHeaderCleanWords++
		case 1:
			evidence.KnownHeaderOneBitWords++
		default:
			evidence.KnownHeaderMultiWords++
		}
	}
	if correctMargins > 0 {
		evidence.KnownCorrectMeanAbs = correctMarginSum / float64(correctMargins)
	}
	if wrongMargins > 0 {
		evidence.KnownWrongMeanAbs = wrongMarginSum / float64(wrongMargins)
	}
	if evidence.KnownCorrectMeanAbs > 0 {
		evidence.KnownWrongMarginRatio = evidence.KnownWrongMeanAbs / evidence.KnownCorrectMeanAbs
	}

	decodedWhitened := hammingDecode(coded)
	decodedRaw := whiten(decodedWhitened, key, v3WhitenLabel)
	evidence.PostECCHdrBits = len(knownRaw)
	for i, expected := range knownRaw {
		if i >= len(decodedRaw) || decodedRaw[i] != expected {
			evidence.PostECCHdrErrors++
		}
	}

	softDecodedWhitened, changedWords := diagnosticSoftHammingDecodeMargins(margins)
	softDecodedRaw := whiten(softDecodedWhitened, key, v3WhitenLabel)
	evidence.SoftChangedWords = changedWords
	for i, expected := range knownRaw {
		if i >= len(softDecodedRaw) || softDecodedRaw[i] != expected {
			evidence.SoftPostECCHdrErrors++
		}
	}
	evidence.SoftHeaderImprovement = evidence.PostECCHdrErrors - evidence.SoftPostECCHdrErrors
	evidence.ReliabilityDecodeEligible = diagnosticShouldUseReliabilityDecode(evidence)

	evidence.HammingWords = len(coded) / 7
	for start := 0; start+7 <= len(coded); start += 7 {
		if diagnosticHammingSyndrome(coded[start:start+7]) != 0 {
			evidence.SyndromeWords++
		}
	}
	if evidence.HammingWords > 0 {
		evidence.SyndromeWordFraction = float64(evidence.SyndromeWords) / float64(evidence.HammingWords)
	}

	absMargins := make([]float64, len(margins))
	sum := 0.0
	for i, value := range margins {
		absMargins[i] = math.Abs(value)
		sum += absMargins[i]
	}
	if len(absMargins) > 0 {
		evidence.MeanAbsCodedMargin = sum / float64(len(absMargins))
		sort.Float64s(absMargins)
		evidence.MedianAbsCodedMargin = diagnosticQuantileSorted(absMargins, 0.50)
		evidence.P10AbsCodedMargin = diagnosticQuantileSorted(absMargins, 0.10)
	}
	return evidence, true
}

func diagnosticAttachSpatialBitEvidence(evidence *DiagnosticBitChannelEvidence, cells []diagnosticSpatialGridCell, key []byte) {
	diagnosticAttachSpatialBitEvidenceWithGrid(evidence, cells, nil, key)
}

func diagnosticAttachSpatialBitEvidenceWithGrid(evidence *DiagnosticBitChannelEvidence, cells []diagnosticSpatialGridCell, aggregate []float64, key []byte) {
	diagnosticAttachSpatialBitEvidenceWithGridAndLattice(evidence, cells, aggregate, key, diagnosticBlindPhaseResult{}, false)
}

func diagnosticAttachSpatialBitEvidenceWithGridAndLattice(evidence *DiagnosticBitChannelEvidence, cells []diagnosticSpatialGridCell, aggregate []float64, key []byte, lattice diagnosticBlindPhaseResult, latticeOK bool) {
	diagnosticAttachSpatialBitEvidenceWithGridAndLatticeOptions(evidence, cells, aggregate, key, lattice, latticeOK, true)
}

func diagnosticAttachSpatialBitEvidenceWithGridAndLatticeOptions(evidence *DiagnosticBitChannelEvidence, cells []diagnosticSpatialGridCell, aggregate []float64, key []byte, lattice diagnosticBlindPhaseResult, latticeOK bool, runCrossfit bool) {
	if evidence == nil || len(cells) < 2 || len(key) < 8 || evidence.Profile == "" {
		return
	}
	spec, ok := profileSpecFor(evidence.Profile)
	if !ok || spec.codedBits <= 0 {
		return
	}
	pattern := newV3SyncPattern(key, spec)
	knownRaw := bytesToBits([]byte{magic[0], magic[1], v3HeaderByte(spec)})
	knownCoded := hammingEncode(whiten(knownRaw, key, v3WhitenLabel))
	globalPhaseCoded := make([][]byte, 0, len(cells))
	globalPhaseMargins := make([][]float64, 0, len(cells))
	localPhaseCoded := make([][]byte, 0, len(cells))
	localPhaseMargins := make([][]float64, 0, len(cells))
	spatial := &DiagnosticSpatialBitEvidence{
		Cells:                             len(cells),
		KnownHeaderCodedBits:              len(knownCoded),
		KnownHeaderAggregatePostECCErrors: evidence.PostECCHdrErrors,
		CellsEvidence:                     make([]DiagnosticSpatialCellEvidence, 0, len(cells)),
	}

	phaseOffsetSum := 0.0
	phaseSubblockOffsetSum := 0.0
	phaseConfidenceSum := 0.0
	spatial.MinLocalPhaseConfidence = 1
	for _, cell := range cells {
		coded, margins := diagnosticReadV3ProtectedMargins(cell.grid, evidence.PhaseX, evidence.PhaseY, spec.codedBits)
		if len(coded) != spec.codedBits || len(margins) != spec.codedBits {
			continue
		}
		globalPhaseCoded = append(globalPhaseCoded, coded)
		globalPhaseMargins = append(globalPhaseMargins, margins)
		cellResult := DiagnosticSpatialCellEvidence{RegionX: cell.RegionX, RegionY: cell.RegionY}
		cellResult.KnownHeaderCodedErrors = diagnosticKnownCodedErrors(coded, knownCoded)
		cellResult.PostECCHeaderErrors = diagnosticKnownRawErrors(coded, knownRaw, key)

		if surface, ok := diagnosticBestV3PhaseSurfaceNear(cell.grid, pattern, evidence.PhaseX, evidence.PhaseY, diagnosticSpatialPhaseRadius); ok && surface.best.total > 0 {
			local := surface.best
			localCoded, localMargins := diagnosticReadV3ProtectedMargins(cell.grid, local.x, local.y, spec.codedBits)
			if len(localCoded) == spec.codedBits && len(localMargins) == spec.codedBits {
				localPhaseCoded = append(localPhaseCoded, localCoded)
				localPhaseMargins = append(localPhaseMargins, localMargins)
				spatial.LocalPhaseCells++
				cellResult.LocalPhaseX = local.x
				cellResult.LocalPhaseY = local.y
				cellResult.LocalPhaseOffsetX = diagnosticSignedPhaseDelta(local.x, evidence.PhaseX, tileWidth)
				cellResult.LocalPhaseOffsetY = diagnosticSignedPhaseDelta(local.y, evidence.PhaseY, tileHeight)
				if cellResult.LocalPhaseOffsetX == 0 && cellResult.LocalPhaseOffsetY == 0 {
					spatial.LocalPhaseSameAsGlobalCells++
				}
				offset := math.Hypot(float64(cellResult.LocalPhaseOffsetX), float64(cellResult.LocalPhaseOffsetY))
				phaseOffsetSum += offset
				if offset > spatial.MaxLocalPhaseOffsetBlocks {
					spatial.MaxLocalPhaseOffsetBlocks = offset
				}
				cellResult.LocalPhaseSyncFraction = float64(local.score) / float64(local.total)
				cellResult.LocalPhaseSubblockAvailable = true
				cellResult.LocalPhaseSubblockOffsetX = surface.offsetX
				cellResult.LocalPhaseSubblockOffsetY = surface.offsetY
				cellResult.LocalPhasePeakScore = surface.peakScore
				cellResult.LocalPhaseSecondScore = surface.secondScore
				cellResult.LocalPhaseConfidence = surface.confidence
				cellResult.LocalPhaseAtSearchBoundary = surface.atSearchBoundary
				subblockOffset := math.Hypot(surface.offsetX, surface.offsetY)
				phaseSubblockOffsetSum += subblockOffset
				if subblockOffset > spatial.MaxLocalPhaseSubblockOffsetBlocks {
					spatial.MaxLocalPhaseSubblockOffsetBlocks = subblockOffset
				}
				phaseConfidenceSum += surface.confidence
				if surface.confidence < spatial.MinLocalPhaseConfidence {
					spatial.MinLocalPhaseConfidence = surface.confidence
				}
				cellResult.LocalPhaseKnownCodedErrors = diagnosticKnownCodedErrors(localCoded, knownCoded)
				cellResult.LocalPhasePostECCHeaderErrors = diagnosticKnownRawErrors(localCoded, knownRaw, key)
			}
		}
		spatial.CellsEvidence = append(spatial.CellsEvidence, cellResult)
	}
	if blind, blindOK := diagnosticEstimateBlindSpatialPhaseWithLatticeOptions(cells, aggregate, lattice, latticeOK, runCrossfit); blindOK {
		spatial.BlindPhaseMethod = blind.method
		spatial.BlindPhaseProfile = blind.profile
		spatial.BlindPhaseGlobalX = blind.globalPhaseX
		spatial.BlindPhaseGlobalY = blind.globalPhaseY
		spatial.BlindPhaseGlobalScore = blind.globalScore
		spatial.BlindPhasePairs = blind.pairs
		spatial.BlindPhaseMeanPairScore = blind.meanPairPeak
		spatial.BlindPhaseMeanOffsetBlocks = blind.meanOffset
		spatial.BlindPhaseMaxOffsetBlocks = blind.maxOffset
		spatial.BlindPhaseMeanConfidence = blind.meanConfidence
		spatial.BlindPhaseMinConfidence = blind.minConfidence
		spatial.BlindPhaseRobustOutliers = blind.robustOutliers
		spatial.BlindSecondaryMethod = blind.secondaryMethod
		spatial.BlindSecondaryCells = blind.secondaryCells
		spatial.BlindSecondaryMeanPairScore = blind.secondaryMeanPairPeak
		spatial.BlindConsensusCells = blind.consensusCells
		spatial.BlindCycleSlipCorrections = blind.cycleSlipCorrections
		spatial.BlindMeanObserverDistanceBlocks = blind.meanObserverDistance
		spatial.BlindMaxObserverDistanceBlocks = blind.maxObserverDistance
		spatial.BlindLatticePhaseMethod = blind.latticeMethod
		spatial.BlindLatticePhaseCells = blind.latticeCells
		spatial.BlindLatticeMeanConfidence = blind.latticeMeanConfidence
		spatial.BlindLatticeMinConfidence = blind.latticeMinConfidence
		spatial.BlindLatticeMeanFractionDistance = blind.latticeMeanFractionDistance
		spatial.BlindLatticeMaxFractionDistance = blind.latticeMaxFractionDistance
		spatial.BlindLatticeConsensusCells = blind.latticeConsensusCells
		spatial.BlindLatticeCycleSlipCorrections = blind.latticeCycleSlipCorrections
		spatial.BlindGlobalUnwrapMethod = blind.unwrapMethod
		spatial.BlindGlobalUnwrapStatus = blind.unwrapStatus
		spatial.BlindGlobalUnwrapStatusX = blind.unwrapStatusX
		spatial.BlindGlobalUnwrapStatusY = blind.unwrapStatusY
		spatial.BlindGlobalUnwrapEvaluatedStates = blind.unwrapEvaluatedStates
		spatial.BlindGlobalUnwrapEligibleCells = blind.unwrapEligibleCells
		spatial.BlindGlobalUnwrapEligibleCellsX = blind.unwrapEligibleCellsX
		spatial.BlindGlobalUnwrapEligibleCellsY = blind.unwrapEligibleCellsY
		spatial.BlindGlobalUnwrapChangedCells = blind.unwrapChangedCells
		spatial.BlindGlobalUnwrapProposedChanged = blind.unwrapProposedChanged
		spatial.BlindGlobalUnwrapAcceptedAxes = blind.unwrapAcceptedAxes
		spatial.BlindGlobalUnwrapBaselineObjective = blind.unwrapBaselineObjective
		spatial.BlindGlobalUnwrapObjective = blind.unwrapObjective
		spatial.BlindGlobalUnwrapAppliedObjective = blind.unwrapAppliedObjective
		spatial.BlindGlobalUnwrapSecondObjective = blind.unwrapSecondObjective
		spatial.BlindGlobalUnwrapSecondAvailable = blind.unwrapSecondAvailable
		spatial.BlindGlobalUnwrapImprovement = blind.unwrapImprovement
		spatial.BlindGlobalUnwrapMargin = blind.unwrapMargin
		spatial.BlindGlobalUnwrapAmbiguous = blind.unwrapAmbiguous
		spatial.BlindGlobalUnwrapValidationMethod = blind.unwrapValidationMethod
		spatial.BlindGlobalUnwrapValidationAvailable = blind.unwrapValidationAvailable
		spatial.BlindGlobalUnwrapValidationCells = blind.unwrapValidationCells
		spatial.BlindGlobalUnwrapValidationPairsFold0 = blind.unwrapValidationPairsFold0
		spatial.BlindGlobalUnwrapValidationPairsFold1 = blind.unwrapValidationPairsFold1
		spatial.BlindGlobalUnwrapValidationFold0Delta = blind.unwrapValidationFold0Delta
		spatial.BlindGlobalUnwrapValidationFold1Delta = blind.unwrapValidationFold1Delta
		spatial.BlindGlobalUnwrapValidationMeanDelta = blind.unwrapValidationMeanDelta
		spatial.BlindGlobalUnwrapValidationSupportsBest = blind.unwrapValidationSupportsBest
		spatial.BlindGlobalUnwrapCrossfitMethod = blind.unwrapCrossfitMethod
		spatial.BlindGlobalUnwrapCrossfitAvailable = blind.unwrapCrossfitAvailable
		spatial.BlindGlobalUnwrapCrossfitAToBAvailable = blind.unwrapCrossfitAToBAvailable
		spatial.BlindGlobalUnwrapCrossfitBToAAvailable = blind.unwrapCrossfitBToAAvailable
		spatial.BlindGlobalUnwrapCrossfitAToBProfile = blind.unwrapCrossfitAToBProfile
		spatial.BlindGlobalUnwrapCrossfitBToAProfile = blind.unwrapCrossfitBToAProfile
		spatial.BlindGlobalUnwrapCrossfitProposalPairsA = blind.unwrapCrossfitProposalPairsA
		spatial.BlindGlobalUnwrapCrossfitProposalPairsB = blind.unwrapCrossfitProposalPairsB
		spatial.BlindGlobalUnwrapCrossfitValidationPairsA = blind.unwrapCrossfitValidationPairsA
		spatial.BlindGlobalUnwrapCrossfitValidationPairsB = blind.unwrapCrossfitValidationPairsB
		spatial.BlindGlobalUnwrapCrossfitAToBCells = blind.unwrapCrossfitAToBCells
		spatial.BlindGlobalUnwrapCrossfitBToACells = blind.unwrapCrossfitBToACells
		spatial.BlindGlobalUnwrapCrossfitAToBStates = blind.unwrapCrossfitAToBStates
		spatial.BlindGlobalUnwrapCrossfitBToAStates = blind.unwrapCrossfitBToAStates
		spatial.BlindGlobalUnwrapCrossfitAToBMargin = blind.unwrapCrossfitAToBMargin
		spatial.BlindGlobalUnwrapCrossfitBToAMargin = blind.unwrapCrossfitBToAMargin
		spatial.BlindGlobalUnwrapCrossfitAToBDelta = blind.unwrapCrossfitAToBDelta
		spatial.BlindGlobalUnwrapCrossfitBToADelta = blind.unwrapCrossfitBToADelta
		spatial.BlindGlobalUnwrapCrossfitAToBSupportsBest = blind.unwrapCrossfitAToBSupportsBest
		spatial.BlindGlobalUnwrapCrossfitBToASupportsBest = blind.unwrapCrossfitBToASupportsBest
		spatial.BlindGlobalUnwrapCrossfitComparedCells = blind.unwrapCrossfitComparedCells
		spatial.BlindGlobalUnwrapCrossfitAgreementCells = blind.unwrapCrossfitAgreementCells
		spatial.BlindGlobalUnwrapCrossfitAgreementFraction = blind.unwrapCrossfitAgreementFraction
		spatial.BlindGlobalUnwrapCrossfitSupportsBest = blind.unwrapCrossfitSupportsBest
		for cellIndex, control := range blind.controls {
			if !control.available || cellIndex >= len(cells) {
				continue
			}
			for evidenceIndex := range spatial.CellsEvidence {
				cellEvidence := &spatial.CellsEvidence[evidenceIndex]
				if cellEvidence.RegionX != cells[cellIndex].RegionX || cellEvidence.RegionY != cells[cellIndex].RegionY {
					continue
				}
				cellEvidence.BlindPhaseAvailable = true
				cellEvidence.BlindPhaseOffsetX = control.offsetX
				cellEvidence.BlindPhaseOffsetY = control.offsetY
				cellEvidence.BlindPhaseConfidence = control.confidence
				cellEvidence.BlindPhasePairSupport = control.pairs
				cellEvidence.BlindPhaseResidualBlocks = control.residual
				if cellIndex < len(blind.secondaryControls) && blind.secondaryControls[cellIndex].available {
					secondary := blind.secondaryControls[cellIndex]
					cellEvidence.BlindSecondaryAvailable = true
					cellEvidence.BlindSecondaryOffsetX = secondary.offsetX
					cellEvidence.BlindSecondaryOffsetY = secondary.offsetY
					cellEvidence.BlindSecondaryConfidence = secondary.confidence
					cellEvidence.BlindObserverDistanceBlocks = math.Hypot(secondary.offsetX-control.offsetX, secondary.offsetY-control.offsetY)
				}
				if cellIndex < len(blind.cycleSlipAdjusted) {
					cellEvidence.BlindCycleSlipAdjusted = blind.cycleSlipAdjusted[cellIndex]
				}
				if cellIndex < len(blind.latticeControls) && blind.latticeControls[cellIndex].available {
					latticeControl := blind.latticeControls[cellIndex]
					cellEvidence.BlindLatticePhaseAvailable = true
					cellEvidence.BlindLatticePhaseOffsetX = latticeControl.offsetX
					cellEvidence.BlindLatticePhaseOffsetY = latticeControl.offsetY
					cellEvidence.BlindLatticePhaseConfidence = latticeControl.confidence
					cellEvidence.BlindLatticeFractionDistance = math.Hypot(diagnosticWrapHalf(control.offsetX-latticeControl.offsetX), diagnosticWrapHalf(control.offsetY-latticeControl.offsetY))
				}
				if cellIndex < len(blind.latticeCycleAdjusted) {
					cellEvidence.BlindLatticeCycleSlipAdjusted = blind.latticeCycleAdjusted[cellIndex]
				}
				if cellIndex < len(blind.unwrapShiftX) {
					cellEvidence.BlindGlobalUnwrapShiftX = blind.unwrapShiftX[cellIndex]
				}
				if cellIndex < len(blind.unwrapShiftY) {
					cellEvidence.BlindGlobalUnwrapShiftY = blind.unwrapShiftY[cellIndex]
				}
				if cellIndex < len(blind.unwrapProposedShiftX) {
					cellEvidence.BlindGlobalUnwrapProposedShiftX = blind.unwrapProposedShiftX[cellIndex]
				}
				if cellIndex < len(blind.unwrapProposedShiftY) {
					cellEvidence.BlindGlobalUnwrapProposedShiftY = blind.unwrapProposedShiftY[cellIndex]
				}
				cellEvidence.BlindGlobalUnwrapAdjusted = cellEvidence.BlindGlobalUnwrapShiftX != 0 || cellEvidence.BlindGlobalUnwrapShiftY != 0
				spatial.BlindPhaseCells++
				break
			}
		}

		// Compare the blind offsets with the key-assisted build11 oracle only
		// after both have been estimated. Center each set independently so the
		// blind zero-mean gauge cannot be aligned using oracle information.
		matched := make([]int, 0, len(spatial.CellsEvidence))
		blindMeanX, blindMeanY, oracleMeanX, oracleMeanY := 0.0, 0.0, 0.0, 0.0
		for i := range spatial.CellsEvidence {
			cell := &spatial.CellsEvidence[i]
			if !cell.BlindPhaseAvailable || !cell.LocalPhaseSubblockAvailable {
				continue
			}
			matched = append(matched, i)
			blindMeanX += cell.BlindPhaseOffsetX
			blindMeanY += cell.BlindPhaseOffsetY
			oracleMeanX += cell.LocalPhaseSubblockOffsetX
			oracleMeanY += cell.LocalPhaseSubblockOffsetY
		}
		if len(matched) > 0 {
			denom := float64(len(matched))
			blindMeanX /= denom
			blindMeanY /= denom
			oracleMeanX /= denom
			oracleMeanY /= denom
			distanceSum := 0.0
			for _, i := range matched {
				cell := &spatial.CellsEvidence[i]
				dx := (cell.BlindPhaseOffsetX - blindMeanX) - (cell.LocalPhaseSubblockOffsetX - oracleMeanX)
				dy := (cell.BlindPhaseOffsetY - blindMeanY) - (cell.LocalPhaseSubblockOffsetY - oracleMeanY)
				distance := math.Hypot(dx, dy)
				cell.BlindPhaseOracleDistanceBlocks = distance
				distanceSum += distance
				if distance > spatial.BlindPhaseMaxOracleDistanceBlocks {
					spatial.BlindPhaseMaxOracleDistanceBlocks = distance
				}
			}
			spatial.BlindPhaseOracleComparedCells = len(matched)
			spatial.BlindPhaseMeanOracleDistanceBlocks = distanceSum / denom
		}
	}

	if len(globalPhaseCoded) < 2 {
		return
	}
	spatial.Cells = len(globalPhaseCoded)
	spatial.MeanTilePositionSignAgreement, spatial.UnstableTilePositions, spatial.UnstableTilePositionFraction = diagnosticTilePositionAgreement(cells)
	if spatial.LocalPhaseCells > 0 {
		spatial.MeanLocalPhaseOffsetBlocks = phaseOffsetSum / float64(spatial.LocalPhaseCells)
		spatial.MeanLocalPhaseSubblockOffsetBlocks = phaseSubblockOffsetSum / float64(spatial.LocalPhaseCells)
		spatial.MeanLocalPhaseConfidence = phaseConfidenceSum / float64(spatial.LocalPhaseCells)
	} else {
		spatial.MinLocalPhaseConfidence = 0
	}

	globalSummary := diagnosticSummarizeSpatialCoded(globalPhaseCoded, globalPhaseMargins, knownCoded, knownRaw, key)
	spatial.KnownHeaderStableCorrectBits = globalSummary.stableCorrect
	spatial.KnownHeaderStableWrongBits = globalSummary.stableWrong
	spatial.KnownHeaderMixedBits = globalSummary.mixed
	spatial.KnownHeaderMajorityCodedErrors = globalSummary.majorityCodedErrors
	spatial.KnownHeaderMajorityPostECCErrors = globalSummary.majorityPostECCErrors
	spatial.KnownHeaderMajorityECCImprovement = evidence.PostECCHdrErrors - globalSummary.majorityPostECCErrors
	spatial.MeanKnownHeaderCorrectCellFraction = globalSummary.meanKnownCorrectFraction
	spatial.MeanAllCodedBitAgreement = globalSummary.meanAllAgreement
	spatial.UnstableAllCodedBits = globalSummary.unstableBits
	spatial.UnstableAllCodedBitFraction = globalSummary.unstableFraction

	if len(localPhaseCoded) >= 2 {
		localSummary := diagnosticSummarizeSpatialCoded(localPhaseCoded, localPhaseMargins, knownCoded, knownRaw, key)
		spatial.LocalPhaseStableWrongBits = localSummary.stableWrong
		spatial.LocalPhaseMixedBits = localSummary.mixed
		spatial.LocalPhaseMajorityCodedErrors = localSummary.majorityCodedErrors
		spatial.LocalPhaseMajorityPostECCErrors = localSummary.majorityPostECCErrors
		spatial.LocalPhaseMeanAllCodedAgreement = localSummary.meanAllAgreement
		spatial.LocalPhaseUnstableAllCodedFraction = localSummary.unstableFraction
	}
	evidence.Spatial = spatial
}

func diagnosticApplyCrossfitSpatialEvidence(spatial *DiagnosticSpatialBitEvidence, crossfit diagnosticUnwrapCrossfitResult, cells []diagnosticSpatialGridCell) {
	if spatial == nil {
		return
	}
	spatial.BlindGlobalUnwrapCrossfitMethod = crossfit.method
	spatial.BlindGlobalUnwrapCrossfitAvailable = crossfit.available
	spatial.BlindGlobalUnwrapCrossfitAToBAvailable = crossfit.aToB.available
	spatial.BlindGlobalUnwrapCrossfitBToAAvailable = crossfit.bToA.available
	spatial.BlindGlobalUnwrapCrossfitAToBProfile = crossfit.aToB.profile
	spatial.BlindGlobalUnwrapCrossfitBToAProfile = crossfit.bToA.profile
	spatial.BlindGlobalUnwrapCrossfitProposalPairsA = crossfit.aToB.proposalPairs
	spatial.BlindGlobalUnwrapCrossfitProposalPairsB = crossfit.bToA.proposalPairs
	spatial.BlindGlobalUnwrapCrossfitValidationPairsA = crossfit.bToA.validationPairs
	spatial.BlindGlobalUnwrapCrossfitValidationPairsB = crossfit.aToB.validationPairs
	spatial.BlindGlobalUnwrapCrossfitAToBCells = crossfit.aToB.cells
	spatial.BlindGlobalUnwrapCrossfitBToACells = crossfit.bToA.cells
	spatial.BlindGlobalUnwrapCrossfitAToBStates = crossfit.aToB.evaluatedStates
	spatial.BlindGlobalUnwrapCrossfitBToAStates = crossfit.bToA.evaluatedStates
	spatial.BlindGlobalUnwrapCrossfitAToBMargin = crossfit.aToB.margin
	spatial.BlindGlobalUnwrapCrossfitBToAMargin = crossfit.bToA.margin
	spatial.BlindGlobalUnwrapCrossfitAToBDelta = crossfit.aToB.delta
	spatial.BlindGlobalUnwrapCrossfitBToADelta = crossfit.bToA.delta
	spatial.BlindGlobalUnwrapCrossfitAToBSupportsBest = crossfit.aToB.supportsBest
	spatial.BlindGlobalUnwrapCrossfitBToASupportsBest = crossfit.bToA.supportsBest
	spatial.BlindGlobalUnwrapCrossfitComparedCells = crossfit.comparedCells
	spatial.BlindGlobalUnwrapCrossfitAgreementCells = crossfit.agreementCells
	spatial.BlindGlobalUnwrapCrossfitAgreementFraction = crossfit.agreementFraction
	spatial.BlindGlobalUnwrapCrossfitSupportsBest = crossfit.supportsBest

	agreeConfidenceSum, disagreeConfidenceSum := 0.0, 0.0
	agreeLatticeSum, disagreeLatticeSum := 0.0, 0.0
	agreeCount, disagreeCount := 0, 0
	for i := range cells {
		var target *DiagnosticSpatialCellEvidence
		for j := range spatial.CellsEvidence {
			if spatial.CellsEvidence[j].RegionX == cells[i].RegionX && spatial.CellsEvidence[j].RegionY == cells[i].RegionY {
				target = &spatial.CellsEvidence[j]
				break
			}
		}
		if target == nil {
			continue
		}
		ax, ay, aok := diagnosticCrossfitBestLocalCycle(crossfit.aToB, i)
		bx, by, bok := diagnosticCrossfitBestLocalCycle(crossfit.bToA, i)
		if aok {
			target.BlindCrossfitAToBAvailable = true
			target.BlindCrossfitAToBCycleX = ax
			target.BlindCrossfitAToBCycleY = ay
			if i < len(crossfit.aToB.primary.controls) {
				target.BlindCrossfitAToBConfidence = crossfit.aToB.primary.controls[i].confidence
			}
		}
		if bok {
			target.BlindCrossfitBToAAvailable = true
			target.BlindCrossfitBToACycleX = bx
			target.BlindCrossfitBToACycleY = by
			if i < len(crossfit.bToA.primary.controls) {
				target.BlindCrossfitBToAConfidence = crossfit.bToA.primary.controls[i].confidence
			}
		}
		if aok && bok {
			target.BlindCrossfitCycleAgreement = ax == bx && ay == by
			jointConfidence := math.Min(target.BlindCrossfitAToBConfidence, target.BlindCrossfitBToAConfidence)
			if target.BlindCrossfitCycleAgreement {
				agreeConfidenceSum += jointConfidence
				agreeLatticeSum += target.BlindLatticePhaseConfidence
				agreeCount++
			} else {
				disagreeConfidenceSum += jointConfidence
				disagreeLatticeSum += target.BlindLatticePhaseConfidence
				disagreeCount++
			}
		}
	}
	if agreeCount > 0 {
		spatial.BlindGlobalUnwrapCrossfitAgreementMeanConfidence = agreeConfidenceSum / float64(agreeCount)
		spatial.BlindGlobalUnwrapCrossfitAgreementMeanLattice = agreeLatticeSum / float64(agreeCount)
	}
	if disagreeCount > 0 {
		spatial.BlindGlobalUnwrapCrossfitDisagreementMeanConfidence = disagreeConfidenceSum / float64(disagreeCount)
		spatial.BlindGlobalUnwrapCrossfitDisagreementMeanLattice = disagreeLatticeSum / float64(disagreeCount)
	}
}

func diagnosticApplyStabilitySpatialEvidence(spatial *DiagnosticSpatialBitEvidence, stability diagnosticUnwrapStabilityResult, cells []diagnosticSpatialGridCell) {
	if spatial == nil {
		return
	}
	spatial.BlindGlobalUnwrapStabilityMethod = stability.method
	spatial.BlindGlobalUnwrapStabilityAvailable = stability.available
	spatial.BlindGlobalUnwrapStabilityPartitions = stability.partitions
	spatial.BlindGlobalUnwrapStabilityTrialsRequested = stability.trialsRequested
	spatial.BlindGlobalUnwrapStabilityTrialsAvailable = stability.trialsAvailable
	spatial.BlindGlobalUnwrapStabilityTrialsSupported = stability.trialsSupported
	spatial.BlindGlobalUnwrapStabilitySupportedFraction = stability.supportedTrialFraction
	spatial.BlindGlobalUnwrapStabilityEvaluatedStates = stability.evaluatedStates
	spatial.BlindGlobalUnwrapStabilityMeanMargin = stability.meanMargin
	spatial.BlindGlobalUnwrapStabilityMeanValidationDelta = stability.meanValidationDelta
	spatial.BlindGlobalUnwrapStabilityCompleteFields = stability.completeFieldTrials
	spatial.BlindGlobalUnwrapStabilityUniqueFields = stability.uniqueFields
	spatial.BlindGlobalUnwrapStabilityModalFieldCount = stability.modalFieldCount
	spatial.BlindGlobalUnwrapStabilityModalFieldFraction = stability.modalFieldFraction
	spatial.BlindGlobalUnwrapStabilitySupportedCompleteFields = stability.supportedCompleteFieldTrials
	spatial.BlindGlobalUnwrapStabilitySupportedUniqueFields = stability.supportedUniqueFields
	spatial.BlindGlobalUnwrapStabilitySupportedModalFieldCount = stability.supportedModalFieldCount
	spatial.BlindGlobalUnwrapStabilitySupportedModalFieldFrac = stability.supportedModalFieldFraction
	spatial.BlindGlobalUnwrapStabilityComparedCells = stability.comparedCells
	spatial.BlindGlobalUnwrapStabilityUnanimousCells = stability.unanimousCells
	spatial.BlindGlobalUnwrapStabilityMeanCellModalFraction = stability.meanCellModalFraction
	spatial.BlindGlobalUnwrapStabilityMinCellModalFraction = stability.minCellModalFraction
	spatial.BlindGlobalUnwrapStabilitySupportedComparedCells = stability.supportedComparedCells
	spatial.BlindGlobalUnwrapStabilitySupportedUnanimousCells = stability.supportedUnanimousCells
	spatial.BlindGlobalUnwrapStabilitySupportedMeanCellModal = stability.supportedMeanCellModalFraction
	spatial.BlindGlobalUnwrapStabilitySupportedMinCellModal = stability.supportedMinCellModalFraction
	spatial.BlindGlobalUnwrapStabilityMeanPairwiseAgreement = stability.meanPairwiseAgreementFraction

	for i := range cells {
		if i >= len(stability.cells) {
			break
		}
		var target *DiagnosticSpatialCellEvidence
		for j := range spatial.CellsEvidence {
			if spatial.CellsEvidence[j].RegionX == cells[i].RegionX && spatial.CellsEvidence[j].RegionY == cells[i].RegionY {
				target = &spatial.CellsEvidence[j]
				break
			}
		}
		if target == nil {
			continue
		}
		cell := stability.cells[i]
		target.BlindStabilityAvailable = cell.observations > 0
		target.BlindStabilityObservations = cell.observations
		target.BlindStabilityUniqueCycles = cell.uniqueCycles
		target.BlindStabilityModalCycleX = cell.modalX
		target.BlindStabilityModalCycleY = cell.modalY
		target.BlindStabilityModalCount = cell.modalCount
		target.BlindStabilityModalFraction = cell.modalFraction
		target.BlindStabilitySupportedObs = cell.supportedObservations
		target.BlindStabilitySupportedUnique = cell.supportedUniqueCycles
		target.BlindStabilitySupportedModalX = cell.supportedModalX
		target.BlindStabilitySupportedModalY = cell.supportedModalY
		target.BlindStabilitySupportedCount = cell.supportedModalCount
		target.BlindStabilitySupportedFraction = cell.supportedModalFraction
	}
}

func diagnosticApplyCycleAnchorSpatialEvidence(spatial *DiagnosticSpatialBitEvidence, result diagnosticCycleAnchorResult, cells []diagnosticSpatialGridCell) {
	if spatial == nil {
		return
	}
	spatial.BlindGlobalUnwrapCycleAnchorMethod = result.method
	spatial.BlindGlobalUnwrapCycleAnchorAvailable = result.available
	spatial.BlindGlobalUnwrapCycleAnchorPairs = result.pairs
	spatial.BlindGlobalUnwrapCycleAnchorCells = result.cells
	spatial.BlindGlobalUnwrapCycleAnchorMeanConfidence = result.meanConfidence
	spatial.BlindGlobalUnwrapCycleAnchorTop1Objective = result.top1Objective
	spatial.BlindGlobalUnwrapCycleAnchorSecondObjective = result.secondObjective
	spatial.BlindGlobalUnwrapCycleAnchorDelta = result.deltaSecondMinusTop1
	spatial.BlindGlobalUnwrapCycleAnchorPrefersTop1 = result.prefersTop1
	spatial.BlindGlobalUnwrapCycleAnchorTop1AgreementCells = result.top1AgreementCells
	spatial.BlindGlobalUnwrapCycleAnchorTop1AgreementFraction = result.top1AgreementFraction
	spatial.BlindGlobalUnwrapCycleAnchorSecondAgreementCells = result.secondAgreementCells
	spatial.BlindGlobalUnwrapCycleAnchorSecondAgreementFraction = result.secondAgreementFraction
	if !result.available {
		return
	}
	for i := range cells {
		if i >= len(result.anchor.controls) {
			break
		}
		var target *DiagnosticSpatialCellEvidence
		for j := range spatial.CellsEvidence {
			if spatial.CellsEvidence[j].RegionX == cells[i].RegionX && spatial.CellsEvidence[j].RegionY == cells[i].RegionY {
				target = &spatial.CellsEvidence[j]
				break
			}
		}
		if target == nil || !result.anchor.controls[i].available {
			continue
		}
		a := result.anchor.controls[i]
		target.BlindCycleAnchorAvailable = true
		target.BlindCycleAnchorOffsetX = a.offsetX
		target.BlindCycleAnchorOffsetY = a.offsetY
		target.BlindCycleAnchorConfidence = a.confidence
		primaryCycleX := int(math.Round(target.BlindPhaseOffsetX))
		primaryCycleY := int(math.Round(target.BlindPhaseOffsetY))
		if i < len(result.top1X) {
			primaryCycleX += result.top1X[i]
		}
		if i < len(result.top1Y) {
			primaryCycleY += result.top1Y[i]
		}
		secondCycleX, secondCycleY := int(math.Round(target.BlindPhaseOffsetX)), int(math.Round(target.BlindPhaseOffsetY))
		if i < len(result.secondX) {
			secondCycleX += result.secondX[i]
		}
		if i < len(result.secondY) {
			secondCycleY += result.secondY[i]
		}
		target.BlindCycleAnchorTop1CycleX = primaryCycleX
		target.BlindCycleAnchorTop1CycleY = primaryCycleY
		target.BlindCycleAnchorSecondCycleX = secondCycleX
		target.BlindCycleAnchorSecondCycleY = secondCycleY
	}
}

func diagnosticTilePositionAgreement(cells []diagnosticSpatialGridCell) (float64, int, float64) {
	if len(cells) < 2 {
		return 0, 0, 0
	}
	agreementSum := 0.0
	positions := 0
	unstable := 0
	for position := 0; position < eccBits; position++ {
		ones := 0
		valid := 0
		for _, cell := range cells {
			if position >= len(cell.grid) {
				continue
			}
			valid++
			if cell.grid[position] >= 0 {
				ones++
			}
		}
		if valid < 2 {
			continue
		}
		zeros := valid - ones
		agreement := float64(maxInt(ones, zeros)) / float64(valid)
		agreementSum += agreement
		positions++
		if agreement < 0.75 {
			unstable++
		}
	}
	if positions == 0 {
		return 0, 0, 0
	}
	return agreementSum / float64(positions), unstable, float64(unstable) / float64(positions)
}

type diagnosticPhaseSurfaceEstimate struct {
	best             v3PhaseCandidate
	offsetX          float64
	offsetY          float64
	peakScore        float64
	secondScore      float64
	confidence       float64
	atSearchBoundary bool
}

// diagnosticBestV3PhaseSurfaceNear keeps the bounded integer phase search used
// by build9, then estimates a fractional-block peak from the local correlation
// surface. The surface uses only the fixed Format-v3 sync prefix and the key;
// it never observes or scores hidden payload bits.
func diagnosticBestV3PhaseSurfaceNear(grid []float64, pattern v3SyncPattern, referenceX, referenceY, radius int) (diagnosticPhaseSurfaceEstimate, bool) {
	result := diagnosticPhaseSurfaceEstimate{}
	if len(grid) < eccBits || len(pattern.points) == 0 || radius < 0 {
		return result, false
	}
	best, ok := diagnosticBestV3PhaseNear(grid, pattern, referenceX, referenceY, radius)
	if !ok || best.total <= 0 {
		return result, false
	}
	result.best = best
	bestDX := diagnosticSignedPhaseDelta(best.x, referenceX, tileWidth)
	bestDY := diagnosticSignedPhaseDelta(best.y, referenceY, tileHeight)
	scores := make(map[[2]int]float64, (2*radius+1)*(2*radius+1))
	peak := math.Inf(-1)
	second := math.Inf(-1)
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			score := diagnosticV3PhaseCorrelationScore(grid, pattern, positiveMod(referenceX+dx, tileWidth), positiveMod(referenceY+dy, tileHeight))
			scores[[2]int{dx, dy}] = score
			if score > peak {
				second = peak
				peak = score
			} else if score > second {
				second = score
			}
		}
	}
	center, exists := scores[[2]int{bestDX, bestDY}]
	if !exists {
		center = diagnosticV3PhaseCorrelationScore(grid, pattern, best.x, best.y)
	}
	// The integer binary-sync maximum is authoritative. Correlation is used only
	// to locate the vertex between neighboring integer samples.
	fracX, curvatureX := 0.0, 0.0
	if bestDX > -radius && bestDX < radius {
		left := scores[[2]int{bestDX - 1, bestDY}]
		right := scores[[2]int{bestDX + 1, bestDY}]
		fracX, curvatureX = diagnosticParabolicPeakOffset(left, center, right)
	}
	fracY, curvatureY := 0.0, 0.0
	if bestDY > -radius && bestDY < radius {
		down := scores[[2]int{bestDX, bestDY - 1}]
		up := scores[[2]int{bestDX, bestDY + 1}]
		fracY, curvatureY = diagnosticParabolicPeakOffset(down, center, up)
	}
	result.offsetX = float64(bestDX) + fracX
	result.offsetY = float64(bestDY) + fracY
	result.peakScore = center
	result.secondScore = second
	if math.IsInf(second, -1) {
		result.secondScore = center
	}
	prominence := math.Max(0, center-result.secondScore)
	signalQuality := diagnosticClamp01((center - 0.05) / 0.45)
	prominenceQuality := diagnosticClamp01(prominence / 0.10)
	curvatureQuality := diagnosticClamp01((curvatureX + curvatureY) / 0.15)
	result.confidence = 0.50*signalQuality + 0.30*prominenceQuality + 0.20*curvatureQuality
	boundaryAxes := 0
	if bestDX == -radius || bestDX == radius {
		boundaryAxes++
	}
	if bestDY == -radius || bestDY == radius {
		boundaryAxes++
	}
	if boundaryAxes > 0 {
		result.atSearchBoundary = true
		for i := 0; i < boundaryAxes; i++ {
			result.confidence *= 0.65
		}
	}
	return result, true
}

func diagnosticV3PhaseCorrelationScore(grid []float64, pattern v3SyncPattern, phaseX, phaseY int) float64 {
	if len(grid) < eccBits || len(pattern.points) == 0 {
		return 0
	}
	absValues := make([]float64, 0, len(pattern.points))
	for _, syncPoint := range pattern.points {
		logicalX := syncPoint.tilePosition % tileWidth
		logicalY := syncPoint.tilePosition / tileWidth
		observedX := positiveMod(logicalX-phaseX, tileWidth)
		observedY := positiveMod(logicalY-phaseY, tileHeight)
		absValues = append(absValues, math.Abs(grid[observedY*tileWidth+observedX]))
	}
	if len(absValues) == 0 {
		return 0
	}
	sort.Float64s(absValues)
	scale := diagnosticQuantileSorted(absValues, 0.50)
	if scale < 1e-9 {
		scale = 1
	}
	clip := 3 * scale
	numerator, denominator := 0.0, 0.0
	for _, syncPoint := range pattern.points {
		logicalX := syncPoint.tilePosition % tileWidth
		logicalY := syncPoint.tilePosition / tileWidth
		observedX := positiveMod(logicalX-phaseX, tileWidth)
		observedY := positiveMod(logicalY-phaseY, tileHeight)
		margin := grid[observedY*tileWidth+observedX]
		if margin > clip {
			margin = clip
		} else if margin < -clip {
			margin = -clip
		}
		expectedSign := -1.0
		if syncPoint.expected != 0 {
			expectedSign = 1
		}
		numerator += expectedSign * margin
		denominator += math.Abs(margin)
	}
	if denominator <= 1e-9 {
		return 0
	}
	return numerator / denominator
}

func diagnosticParabolicPeakOffset(left, center, right float64) (float64, float64) {
	denom := left - 2*center + right
	curvature := math.Max(0, center-(left+right)/2)
	if denom >= -1e-9 {
		return 0, curvature
	}
	offset := 0.5 * (left - right) / denom
	if offset < -0.5 {
		offset = -0.5
	} else if offset > 0.5 {
		offset = 0.5
	}
	return offset, curvature
}

func diagnosticClamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func diagnosticBestV3PhaseNear(grid []float64, pattern v3SyncPattern, referenceX, referenceY, radius int) (v3PhaseCandidate, bool) {
	if len(grid) < eccBits || len(pattern.points) == 0 || radius < 0 {
		return v3PhaseCandidate{}, false
	}
	best := v3PhaseCandidate{score: -1, total: len(pattern.points)}
	bestDistance := math.Inf(1)
	for dy := -radius; dy <= radius; dy++ {
		for dx := -radius; dx <= radius; dx++ {
			phaseX := positiveMod(referenceX+dx, tileWidth)
			phaseY := positiveMod(referenceY+dy, tileHeight)
			score := 0
			for _, syncPoint := range pattern.points {
				logicalX := syncPoint.tilePosition % tileWidth
				logicalY := syncPoint.tilePosition / tileWidth
				observedX := positiveMod(logicalX-phaseX, tileWidth)
				observedY := positiveMod(logicalY-phaseY, tileHeight)
				observed := byte(0)
				if grid[observedY*tileWidth+observedX] >= 0 {
					observed = 1
				}
				if observed == syncPoint.expected {
					score++
				}
			}
			distance := math.Hypot(float64(dx), float64(dy))
			if score > best.score || (score == best.score && distance < bestDistance) {
				best = v3PhaseCandidate{score: score, x: phaseX, y: phaseY, total: len(pattern.points)}
				bestDistance = distance
			}
		}
	}
	return best, best.score >= 0
}

type diagnosticSpatialCodedSummary struct {
	stableCorrect            int
	stableWrong              int
	mixed                    int
	majorityCodedErrors      int
	majorityPostECCErrors    int
	meanKnownCorrectFraction float64
	meanAllAgreement         float64
	unstableBits             int
	unstableFraction         float64
}

func diagnosticSummarizeSpatialCoded(cellCoded [][]byte, cellMargins [][]float64, knownCoded, knownRaw, key []byte) diagnosticSpatialCodedSummary {
	result := diagnosticSpatialCodedSummary{}
	if len(cellCoded) < 2 || len(cellCoded[0]) == 0 {
		return result
	}
	codedBits := len(cellCoded[0])
	majorityCoded := make([]byte, codedBits)
	allAgreementSum := 0.0
	for bit := 0; bit < codedBits; bit++ {
		ones := 0
		for _, coded := range cellCoded {
			if bit < len(coded) && coded[bit] != 0 {
				ones++
			}
		}
		zeros := len(cellCoded) - ones
		if ones > zeros {
			majorityCoded[bit] = 1
		} else if ones == zeros {
			sum := 0.0
			for _, margins := range cellMargins {
				if bit < len(margins) {
					sum += margins[bit]
				}
			}
			if sum >= 0 {
				majorityCoded[bit] = 1
			}
		}
		agreement := float64(maxInt(ones, zeros)) / float64(len(cellCoded))
		allAgreementSum += agreement
		if agreement < 0.75 {
			result.unstableBits++
		}
	}
	result.meanAllAgreement = allAgreementSum / float64(codedBits)
	result.unstableFraction = float64(result.unstableBits) / float64(codedBits)

	correctFractionSum := 0.0
	for bit, expected := range knownCoded {
		correct := 0
		for _, coded := range cellCoded {
			if bit < len(coded) && coded[bit] == expected {
				correct++
			}
		}
		correctFractionSum += float64(correct) / float64(len(cellCoded))
		switch correct {
		case 0:
			result.stableWrong++
		case len(cellCoded):
			result.stableCorrect++
		default:
			result.mixed++
		}
		if bit >= len(majorityCoded) || majorityCoded[bit] != expected {
			result.majorityCodedErrors++
		}
	}
	if len(knownCoded) > 0 {
		result.meanKnownCorrectFraction = correctFractionSum / float64(len(knownCoded))
	}
	majorityRaw := whiten(hammingDecode(majorityCoded), key, v3WhitenLabel)
	for i, expected := range knownRaw {
		if i >= len(majorityRaw) || majorityRaw[i] != expected {
			result.majorityPostECCErrors++
		}
	}
	return result
}

func diagnosticKnownCodedErrors(coded, known []byte) int {
	errors := 0
	for i, expected := range known {
		if i >= len(coded) || coded[i] != expected {
			errors++
		}
	}
	return errors
}

func diagnosticKnownRawErrors(coded, knownRaw, key []byte) int {
	decodedRaw := whiten(hammingDecode(coded), key, v3WhitenLabel)
	errors := 0
	for i, expected := range knownRaw {
		if i >= len(decodedRaw) || decodedRaw[i] != expected {
			errors++
		}
	}
	return errors
}

func diagnosticSignedPhaseDelta(value, reference, modulus int) int {
	if modulus <= 0 {
		return value - reference
	}
	delta := positiveMod(value-reference, modulus)
	if delta > modulus/2 {
		delta -= modulus
	}
	return delta
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func diagnosticReadV3ProtectedMargins(grid []float64, phaseX, phaseY, codedBits int) ([]byte, []float64) {
	if codedBits <= 0 {
		return nil, nil
	}
	sums := make([]float64, codedBits)
	counts := make([]int, codedBits)
	for logicalPosition := 0; logicalPosition < eccBits; logicalPosition++ {
		logicalX := logicalPosition % tileWidth
		logicalY := logicalPosition / tileWidth
		observedX := positiveMod(logicalX-phaseX, tileWidth)
		observedY := positiveMod(logicalY-phaseY, tileHeight)
		value := grid[observedY*tileWidth+observedX]
		codeIndex := v3CodeIndex(logicalPosition, codedBits)
		sums[codeIndex] += value
		counts[codeIndex]++
	}
	coded := make([]byte, codedBits)
	margins := make([]float64, codedBits)
	for i := range sums {
		if sums[i] >= 0 {
			coded[i] = 1
		}
		if counts[i] > 0 {
			margins[i] = sums[i] / float64(counts[i])
		}
	}
	return coded, margins
}

func diagnosticHammingSyndrome(word []byte) int {
	if len(word) < 7 {
		return 0
	}
	s1 := word[0] ^ word[2] ^ word[4] ^ word[6]
	s2 := word[1] ^ word[2] ^ word[5] ^ word[6]
	s4 := word[3] ^ word[4] ^ word[5] ^ word[6]
	return int(s1 + 2*s2 + 4*s4)
}

// diagnosticSoftHammingDecodeMargins performs deterministic maximum-likelihood
// decoding of each Hamming(7,4) word from signed DCT margins. It returns exactly
// one decoded stream: there is no list expansion or candidate branching.
func diagnosticSoftHammingDecodeMargins(margins []float64) ([]byte, int) {
	words := len(margins) / 7
	out := make([]byte, words*4)
	changedWords := 0
	hard := make([]byte, words*7)
	for i := range hard {
		if margins[i] >= 0 {
			hard[i] = 1
		}
	}
	hardDecoded := hammingDecode(hard)
	for word := 0; word < words; word++ {
		base := word * 7
		bestScore := math.Inf(-1)
		var bestNibble [4]byte
		for value := 0; value < 16; value++ {
			nibble := []byte{byte((value >> 3) & 1), byte((value >> 2) & 1), byte((value >> 1) & 1), byte(value & 1)}
			encoded := hammingEncode(nibble)
			score := 0.0
			for bit := 0; bit < 7; bit++ {
				sign := -1.0
				if encoded[bit] != 0 {
					sign = 1
				}
				score += sign * margins[base+bit]
			}
			if score > bestScore {
				bestScore = score
				copy(bestNibble[:], nibble)
			}
		}
		copy(out[word*4:word*4+4], bestNibble[:])
		different := false
		for bit := 0; bit < 4; bit++ {
			if out[word*4+bit] != hardDecoded[word*4+bit] {
				different = true
				break
			}
		}
		if different {
			changedWords++
		}
	}
	return out, changedWords
}

func diagnosticDecodeGridReliabilityAware(grid []float64, key []byte, decoder *decoder) ([]byte, ExtractInfo, int, bool) {
	bestScore := 0
	for _, pattern := range decoder.v3Patterns {
		phases := strongestV3Phases(grid, pattern, 4)
		if len(phases) > 0 && phases[0].total > 0 {
			score := phases[0].score * 1000 / phases[0].total
			if score > bestScore {
				bestScore = score
			}
		}
		for _, phase := range phases {
			if phase.total == 0 || phase.score*4 < phase.total*3 {
				continue
			}
			_, margins := diagnosticReadV3ProtectedMargins(grid, phase.x, phase.y, pattern.spec.codedBits)
			decodedWhitened, _ := diagnosticSoftHammingDecodeMargins(margins)
			raw := bitsToBytes(whiten(decodedWhitened, key, v3WhitenLabel))
			if payload, err := parseV3Frame(raw, key, pattern.spec); err == nil {
				meanMargin := 0.0
				for _, margin := range margins {
					meanMargin += math.Abs(margin)
				}
				if len(margins) > 0 {
					meanMargin /= float64(len(margins))
				}
				return payload, ExtractInfo{Version: v3Version, Profile: pattern.spec.profile, Confidence: meanMargin}, bestScore, true
			}
		}
	}
	return nil, ExtractInfo{}, bestScore, false
}

func diagnosticShouldUseReliabilityDecode(e DiagnosticBitChannelEvidence) bool {
	// A soft decision is justified only when the known Format-v3 prefix shows
	// at most one word beyond hard Hamming radius and its wrong bits are weaker
	// than the correct bits. This is a format-derived, message-independent gate.
	return e.PostECCHdrErrors > 0 && e.KnownHeaderMultiWords <= 1 && e.KnownWrongMarginRatio > 0 && e.KnownWrongMarginRatio < 0.90
}

func diagnosticQuantileSorted(values []float64, q float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if q <= 0 {
		return values[0]
	}
	if q >= 1 {
		return values[len(values)-1]
	}
	position := q * float64(len(values)-1)
	lo := int(math.Floor(position))
	hi := int(math.Ceil(position))
	if lo == hi {
		return values[lo]
	}
	fraction := position - float64(lo)
	return values[lo]*(1-fraction) + values[hi]*fraction
}

func diagnosticBitEvidenceBetter(a, b DiagnosticBitChannelEvidence) bool {
	if a.KnownHeaderMultiWords != b.KnownHeaderMultiWords {
		return a.KnownHeaderMultiWords < b.KnownHeaderMultiWords
	}
	if a.PostECCHdrErrors != b.PostECCHdrErrors {
		return a.PostECCHdrErrors < b.PostECCHdrErrors
	}
	if a.KnownHeaderCodedErrors != b.KnownHeaderCodedErrors {
		return a.KnownHeaderCodedErrors < b.KnownHeaderCodedErrors
	}
	if a.SyndromeWordFraction != b.SyndromeWordFraction {
		return a.SyndromeWordFraction < b.SyndromeWordFraction
	}
	if a.KnownWrongMarginRatio != b.KnownWrongMarginRatio {
		return a.KnownWrongMarginRatio < b.KnownWrongMarginRatio
	}
	return a.MedianAbsCodedMargin > b.MedianAbsCodedMargin
}

func diagnosticSpatialEvidenceBetter(a, b *DiagnosticSpatialBitEvidence) bool {
	if a == nil {
		return false
	}
	if b == nil || b.Cells == 0 {
		return true
	}
	if a.KnownHeaderStableWrongBits != b.KnownHeaderStableWrongBits {
		return a.KnownHeaderStableWrongBits < b.KnownHeaderStableWrongBits
	}
	if a.KnownHeaderMajorityPostECCErrors != b.KnownHeaderMajorityPostECCErrors {
		return a.KnownHeaderMajorityPostECCErrors < b.KnownHeaderMajorityPostECCErrors
	}
	if a.KnownHeaderMixedBits != b.KnownHeaderMixedBits {
		return a.KnownHeaderMixedBits < b.KnownHeaderMixedBits
	}
	if a.KnownHeaderMajorityCodedErrors != b.KnownHeaderMajorityCodedErrors {
		return a.KnownHeaderMajorityCodedErrors < b.KnownHeaderMajorityCodedErrors
	}
	if a.UnstableAllCodedBitFraction != b.UnstableAllCodedBitFraction {
		return a.UnstableAllCodedBitFraction < b.UnstableAllCodedBitFraction
	}
	return a.MeanAllCodedBitAgreement > b.MeanAllCodedBitAgreement
}
