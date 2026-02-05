#!/usr/bin/env bash
# Master Integration Test Suite: Full NTM System Flow
# Implements bd-ak7b: end-to-end system flow with phased logging + JSON report.

set -uo pipefail
# Intentionally not using -e so we can log and record failures per phase.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/e2e/lib/log.sh"
set +e

TEST_ID="integration-001"
TEST_NAME="integration_master_test"
TEST_PREFIX="e2e-master-$$"

REPORT_DIR="${E2E_LOG_DIR:-/tmp/ntm-e2e-logs}"
REPORT_FILE="${REPORT_DIR}/integration_master_report_$(date -u +%Y%m%d_%H%M%S).json"

PHASE_NAMES=()
PHASE_STATUS=()
PHASE_DURATIONS=()

_current_phase=""
_current_phase_start_ms=0

phase_start() {
    _current_phase="$1"
    _current_phase_start_ms=$(_millis)
    log_section "Phase: ${_current_phase}"
}

phase_end() {
    local status="$1"
    local end_ms
    end_ms=$(_millis)
    local duration_ms=$((end_ms - _current_phase_start_ms))

    PHASE_NAMES+=("${_current_phase}")
    PHASE_STATUS+=("${status}")
    PHASE_DURATIONS+=("${duration_ms}")

    if [[ "$status" == "pass" ]]; then
        log_info "Phase passed: ${_current_phase} (${duration_ms}ms)"
    elif [[ "$status" == "skip" ]]; then
        log_skip "Phase skipped: ${_current_phase}"
    else
        log_error "Phase failed: ${_current_phase} (${duration_ms}ms)"
    fi

    # Count each phase as an assertion to surface failures in summary.
    log_assert_eq "$status" "pass" "phase ${_current_phase}"
}

write_report_json() {
    local total=${#PHASE_NAMES[@]}
    local passed=0
    local failed=0

    for i in "${!PHASE_STATUS[@]}"; do
        case "${PHASE_STATUS[$i]}" in
            pass)
                ((passed++)) || true
                ;;
            *)
                ((failed++)) || true
                ;;
        esac
    done

    local duration_ms
    duration_ms=$(_elapsed_ms)
    local duration_seconds=$((duration_ms / 1000))

    mkdir -p "$REPORT_DIR"

    {
        echo "{"
        echo "  \"test_id\": \"${TEST_ID}\","
        echo "  \"timestamp\": \"$(_timestamp)\","
        echo "  \"duration_seconds\": ${duration_seconds},"
        echo "  \"phases\": ["
        local first=1
        for i in "${!PHASE_NAMES[@]}"; do
            if [[ $first -eq 0 ]]; then
                echo ","
            fi
            first=0
            printf "    {\"name\": \"%s\", \"status\": \"%s\", \"duration_ms\": %s}" \
                "${PHASE_NAMES[$i]}" "${PHASE_STATUS[$i]}" "${PHASE_DURATIONS[$i]}"
        done
        echo ""
        echo "  ],"
        echo "  \"summary\": {\"total\": ${total}, \"passed\": ${passed}, \"failed\": ${failed}}"
        echo "}"
    } > "$REPORT_FILE"

    log_info "Wrote report: $REPORT_FILE"
    log_assert_valid_json "$(cat "$REPORT_FILE")" "report JSON is valid"
}

require_br() {
    if ! command -v br &>/dev/null; then
        log_error "br not found in PATH"
        return 1
    fi
    return 0
}

cleanup() {
    log_section "Cleanup"
    if [[ -n "${SESSION_NAME:-}" ]]; then
        log_exec ntm kill "$SESSION_NAME" --force
    fi

    if [[ -n "${PROJECTS_BASE:-}" && -d "${PROJECTS_BASE}" ]]; then
        log_info "Leaving projects base for inspection: ${PROJECTS_BASE}"
        log_info "Manual cleanup required (deletion disabled by safety policy)"
    fi
}
trap cleanup EXIT

main() {
    log_init "$TEST_NAME"

    require_ntm
    require_tmux
    require_jq

    PROJECTS_BASE=$(mktemp -d -t ntm-master-e2e-XXXX)
    export NTM_PROJECTS_BASE="$PROJECTS_BASE"

    PROJECT_NAME="${TEST_PREFIX}-project"
    SESSION_NAME="$PROJECT_NAME"
    PROJECT_DIR="${PROJECTS_BASE}/${PROJECT_NAME}"
    mkdir -p "$PROJECT_DIR"
    printf "E2E master integration test\n" > "${PROJECT_DIR}/README.md"

    # Phase 1: Project Initialization
    phase_start "init"
    local status="pass"
    if log_exec ntm init "$PROJECT_DIR" --non-interactive --no-hooks --force; then
        if [[ ! -f "${PROJECT_DIR}/.ntm/config.toml" ]]; then
            status="fail"
            log_error "config.toml missing after init"
        fi
    else
        status="fail"
    fi
    phase_end "$status"

    # Phase 2: Template-based Spawn
    phase_start "spawn_template"
    status="pass"
    if log_exec ntm spawn "$SESSION_NAME" --template review-pipeline --no-user; then
        if ! tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
            status="fail"
            log_error "tmux session not found after spawn"
        fi
    else
        status="fail"
    fi
    phase_end "$status"

    # Phase 3: CM Context Loading
    phase_start "context_build"
    status="pass"
    if pushd "$PROJECT_DIR" >/dev/null; then
        if log_exec ntm context build --task "Master integration test" --agent cc --bead "${TEST_PREFIX}" --files README.md --verbose --json; then
            local output
            output="$(get_last_output)"
            log_assert_valid_json "$output" "context build JSON"
        else
            status="fail"
        fi
        popd >/dev/null || true
    else
        status="fail"
        log_error "failed to enter project dir: ${PROJECT_DIR}"
    fi
    phase_end "$status"

    # Phase 4: Task Assignment
    phase_start "task_assignment"
    status="pass"
    if require_br; then
        if pushd "$PROJECT_DIR" >/dev/null; then
            if log_exec br create "E2E master integration task" -t task -p 2 --json; then
                if ! log_exec ntm assign "$SESSION_NAME" --auto --limit 1 --strategy round-robin --json; then
                    status="fail"
                else
                    local output
                    output="$(get_last_output)"
                    log_assert_valid_json "$output" "assign output JSON"
                fi
            else
                status="fail"
            fi
            popd >/dev/null || true
        else
            status="fail"
            log_error "failed to enter project dir: ${PROJECT_DIR}"
        fi
    else
        status="skip"
    fi
    phase_end "$status"

    # Phases 5-15: Stubbed for now (tracked as failures until implemented).
    for phase in \
        "file_reservation" \
        "agent_communication" \
        "context_monitoring" \
        "cost_tracking" \
        "staggered_operations" \
        "handoff" \
        "prompt_history" \
        "session_summarization" \
        "output_archive" \
        "effectiveness_scoring" \
        "session_recovery"; do
        phase_start "$phase"
        phase_end "skip"
    done

    write_report_json

    local exit_code=0
    log_summary || exit_code=$?

    # Emit report JSON last so the runner can parse it.
    cat "$REPORT_FILE"

    return $exit_code
}

main "$@"
