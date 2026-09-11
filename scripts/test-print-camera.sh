#!/usr/bin/env bash

set -uo pipefail

PIXSEAL="${PIXSEAL:?PIXSEAL is required}"
PRINT_CAMERA_DIR="${PRINT_CAMERA_DIR:-print-camera private}"
PRINT_CAMERA_KEY="${PRINT_CAMERA_KEY:-}"
PRINT_CAMERA_TIMEOUT="${PRINT_CAMERA_TIMEOUT:-180}"
PRINT_CAMERA_LABEL="${PRINT_CAMERA_LABEL:-Print-camera}"

if [[ ! -d "$PRINT_CAMERA_DIR" ]]; then
    echo "SKIP  private acquisition corpus not present: $PRINT_CAMERA_DIR"
    exit 0
fi

if [[ -z "$PRINT_CAMERA_KEY" ]]; then
    echo "error: PRINT_CAMERA_KEY is required when a private physical corpus is present" >&2
    echo "       provide it via the environment or make PRINT_CAMERA_KEY=...; it is intentionally not stored in source" >&2
    exit 2
fi

if ! [[ "$PRINT_CAMERA_TIMEOUT" =~ ^[0-9]+$ ]] || (( PRINT_CAMERA_TIMEOUT <= 0 )); then
    echo "error: PRINT_CAMERA_TIMEOUT must be a positive integer number of seconds" >&2
    exit 2
fi

mapfile -d '' files < <(find "$PRINT_CAMERA_DIR" -maxdepth 1 -type f \
    \( -iname '*.png' -o -iname '*.jpg' -o -iname '*.jpeg' \) -print0 | sort -z)
if (( ${#files[@]} == 0 )); then
    echo "SKIP  no PNG/JPEG print-camera photographs found: $PRINT_CAMERA_DIR"
    exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf -- "$work"' EXIT

passed=0
research=0
errors=0
timeouts=0
for input in "${files[@]}"; do
    name="$(basename "$input")"
    safe_name="${name// /_}"
    output="$work/${safe_name}.json"
    echo "Image: $name"
    timeout "${PRINT_CAMERA_TIMEOUT}s" "$PIXSEAL" diagnose \
        -in "$input" -key "$PRINT_CAMERA_KEY" -json >"$output"
    status=$?
    if (( status != 0 )); then
        if (( status == 124 )); then
            echo "  TIMEOUT diagnostic exceeded ${PRINT_CAMERA_TIMEOUT}s"
            ((timeouts += 1))
        else
            echo "  ERROR diagnostic command failed (status $status)"
            ((errors += 1))
        fi
        continue
    fi

    grep -E '"(width|height|adaptive_escalated|adaptive_divisor|adaptive_reason|lattice_first_fallback_used|global_consistency|consensus_fraction|lattice_evidence|authentication_status|authenticated_payload|full_decode_attempts|best_sync_profile|best_sync_fraction|best_sync_z_score|best_phase_profile|best_phase_consistency|best_phase_x_coherence|best_phase_y_coherence|best_phase_canonical_width_px|best_phase_canonical_height_px|phase_refinement_attempts|phase_homography_fits|fundamental_scale_probe_attempts|best_fundamental_width_px|best_fundamental_height_px|best_fundamental_sync_z_score|subpixel_refinements|subpixel_probe_attempts|best_subpixel_offset_x|best_subpixel_offset_y|best_subpixel_z_score|residual_warp_fits|best_residual_controls|best_residual_rms_px|photometric_probe_attempts|best_photometric_mode|best_photometric_profile|best_photometric_sync_fraction|best_photometric_sync_z_score|best_photometric_mean_absolute_margin|best_photometric_normalized_sync_margin|bit_diagnostic_attempts|best_bit_profile|best_bit_mode|best_bit_geometry_source|best_known_header_hamming_clean_words|best_known_header_hamming_one_bit_words|best_known_header_coded_bit_errors|best_known_header_hamming_multi_error_words|best_post_ecc_known_header_bit_errors|best_hamming_nonzero_syndrome_fraction|best_known_wrong_to_correct_margin_ratio|best_soft_post_ecc_known_header_bit_errors|best_soft_known_header_bit_improvement|best_soft_profile|best_soft_mode|best_soft_geometry_source|best_hard_candidate_soft_post_ecc_known_header_bit_errors|best_hard_candidate_soft_known_header_bit_improvement|best_soft_candidate_hard_post_ecc_known_header_bit_errors|spatial_diagnostic_attempts|best_spatial_profile|best_spatial_mode|best_spatial_geometry_source|best_spatial_cells|best_spatial_mean_tile_position_sign_agreement|best_spatial_unstable_tile_position_fraction|best_spatial_stable_wrong_known_header_bits|best_spatial_mixed_known_header_bits|best_spatial_majority_coded_bit_errors|best_spatial_majority_post_ecc_known_header_bit_errors|best_spatial_majority_post_ecc_improvement|best_spatial_mean_all_coded_bit_agreement|best_spatial_unstable_all_coded_bit_fraction|best_spatial_local_phase_same_as_global_cells|best_spatial_mean_local_phase_offset_blocks|best_spatial_max_local_phase_offset_blocks|best_spatial_mean_local_phase_subblock_offset_blocks|best_spatial_max_local_phase_subblock_offset_blocks|best_spatial_mean_local_phase_confidence|best_spatial_min_local_phase_confidence|best_spatial_local_phase_stable_wrong_known_header_bits|best_spatial_local_phase_mixed_known_header_bits|best_spatial_local_phase_majority_post_ecc_known_header_bit_errors|best_spatial_local_phase_mean_all_coded_bit_agreement|best_spatial_local_phase_unstable_all_coded_bit_fraction|best_spatial_blind_phase_method|best_spatial_blind_phase_profile|best_spatial_blind_phase_global_x|best_spatial_blind_phase_global_y|best_spatial_blind_phase_global_score|best_spatial_blind_phase_cells|best_spatial_blind_phase_pairs|best_spatial_blind_phase_mean_pair_score|best_spatial_blind_phase_mean_offset_blocks|best_spatial_blind_phase_max_offset_blocks|best_spatial_blind_phase_mean_confidence|best_spatial_blind_phase_min_confidence|best_spatial_blind_phase_robust_pair_outliers|best_spatial_blind_secondary_method|best_spatial_blind_secondary_cells|best_spatial_blind_secondary_mean_pair_score|best_spatial_blind_consensus_cells|best_spatial_blind_cycle_slip_corrections|best_spatial_blind_mean_observer_distance_blocks|best_spatial_blind_max_observer_distance_blocks|best_spatial_blind_lattice_phase_method|best_spatial_blind_lattice_phase_cells|best_spatial_blind_lattice_phase_mean_confidence|best_spatial_blind_lattice_phase_min_confidence|best_spatial_blind_lattice_mean_fraction_distance_blocks|best_spatial_blind_lattice_max_fraction_distance_blocks|best_spatial_blind_lattice_consensus_cells|best_spatial_blind_lattice_cycle_slip_corrections|best_spatial_blind_phase_oracle_compared_cells|best_spatial_blind_phase_mean_oracle_distance_blocks|best_spatial_blind_phase_max_oracle_distance_blocks|smooth_phase_fit_attempts|smooth_phase_eligible_fits|smooth_phase_resample_attempts|smooth_phase_decode_attempts|best_smooth_phase_profile|best_smooth_phase_mode|best_smooth_phase_geometry_source|best_smooth_phase_controls|best_smooth_phase_model|best_smooth_phase_control_source|best_smooth_phase_mean_control_confidence|best_smooth_phase_min_control_confidence|best_smooth_phase_robust_outliers|best_smooth_phase_affine_fit_rms_blocks|best_smooth_phase_affine_leave_one_out_rms_blocks|best_smooth_phase_quadratic_available|best_smooth_phase_quadratic_fit_rms_blocks|best_smooth_phase_quadratic_leave_one_out_rms_blocks|best_smooth_phase_quadratic_selected|best_smooth_phase_fit_rms_blocks|best_smooth_phase_leave_one_out_rms_blocks|best_smooth_phase_max_correction_blocks|best_smooth_phase_base_post_ecc_known_header_bit_errors|best_smooth_phase_post_ecc_known_header_bit_errors|best_smooth_phase_post_ecc_improvement|best_smooth_phase_known_header_coded_bit_errors|best_smooth_phase_sync_z_score|reliability_decode_attempts|full_grid_bounded_uses|max_full_grid_sampled_blocks|max_full_grid_sampled_tiles|coarse_probe_ms|refinement_ms|photometric_ms|full_decode_ms|baseline_authentication_attempted|baseline_authentication_skipped_reason|total_ms)"' "$output" \
        | sed 's/^/  /'

    grep -E '"best_spatial_blind_global_unwrap_(method|status|status_x|status_y|evaluated_states|eligible_cells|eligible_cells_x|eligible_cells_y|changed_cells|proposed_changed_cells|accepted_axes|baseline_objective|proposed_objective|applied_objective|second_objective|second_objective_available|improvement|margin|ambiguous|validation_method|validation_available|validation_cells|validation_pairs_fold0|validation_pairs_fold1|validation_fold0_delta|validation_fold1_delta|validation_mean_delta|validation_supports_best)"' "$output" \
        | sed 's/^/  /'

    if grep -Eq '"authenticated_payload"[[:space:]]*:[[:space:]]*true' "$output"; then
        echo "  PASS  authenticated Format v3 payload recovered"
        ((passed += 1))
    else
        echo "  RESEARCH  authenticated payload not recovered; this is not a PASS"
        ((research += 1))
    fi
done

total=${#files[@]}
echo "${PRINT_CAMERA_LABEL} summary: ${total} images, ${passed} authenticated, ${research} research failures, ${timeouts} timeouts, ${errors} errors"

if (( research > 0 || timeouts > 0 || errors > 0 )); then
    exit 1
fi
