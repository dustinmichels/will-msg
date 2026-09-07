#!/usr/bin/env bash
#
# loop.sh - Autonomous loop runner for Oh My Pi (omp) migration tasks.
#
# Usage:
#   ./loop.sh [document.md] [extra omp flags...]
#
# Examples:
#   ./loop.sh
#   ./loop.sh migration.md
#   ./loop.sh requirements.md --model opus
#

set -uo pipefail

# ---------------------------------------------------------------------------
# Configuration & Defaults
# ---------------------------------------------------------------------------
# Determine target document:
# 1. First argument (if ends with .md or file exists)
# 2. $DOC environment variable
# 3. requirements.md (if it exists)
# 4. migration.md (default)
EXTRA_ARGS=()

if [[ $# -gt 0 && ("$1" == *.md || -f "$1") ]]; then
    DOC_FILE="$1"
    shift
    EXTRA_ARGS=("$@")
elif [[ -n "${DOC:-}" ]]; then
    DOC_FILE="$DOC"
    EXTRA_ARGS=("$@")
elif [[ -f "requirements.md" ]]; then
    DOC_FILE="requirements.md"
    EXTRA_ARGS=("$@")
else
    DOC_FILE="migration.md"
    EXTRA_ARGS=("$@")
fi

MAX_ITERATIONS="${MAX_ITERATIONS:-0}" # 0 = infinite
SLEEP_SECONDS="${SLEEP_SECONDS:-2}"
STALL_LIMIT="${STALL_LIMIT:-3}"       # consecutive no-progress runs before giving up

# omp needs unattended tool execution in print mode; AUTO_APPROVE=0 opts out.
APPROVAL_ARGS=()
if [[ "${AUTO_APPROVE:-1}" != "0" ]]; then
    APPROVAL_ARGS=(--auto-approve)
fi

# ---------------------------------------------------------------------------
# Trap Signals for Clean Interruption
# ---------------------------------------------------------------------------
INTERRUPTED=0
trap 'echo -e "\n\033[1;33m[loop.sh] Interrupted by user. Exiting loop.\033[0m"; INTERRUPTED=1; exit 130' INT TERM

# ---------------------------------------------------------------------------
# Pre-flight Checks
# ---------------------------------------------------------------------------
if ! command -v omp >/dev/null 2>&1; then
    echo -e "\033[1;31mError: 'omp' command not found in PATH.\033[0m" >&2
    exit 1
fi

if [[ ! -f "$DOC_FILE" ]]; then
    echo -e "\033[1;31mError: Document '$DOC_FILE' not found.\033[0m" >&2
    exit 1
fi

# ---------------------------------------------------------------------------
# NDJSON Renderer (live tool-call output instead of "Working…")
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOOPTUI_DIR="$SCRIPT_DIR/looptui"
LOOPTUI_BIN="$LOOPTUI_DIR/looptui"
RENDERER=""

if [[ "${LOOP_TUI:-1}" != "0" ]] && [[ -t 1 ]] && command -v go >/dev/null 2>&1 && [[ -f "$LOOPTUI_DIR/main.go" ]]; then
    # Build if binary is missing or stale
    if [[ ! -x "$LOOPTUI_BIN" ]] || [[ "$LOOPTUI_DIR/main.go" -nt "$LOOPTUI_BIN" ]] || [[ "$LOOPTUI_DIR/go.mod" -nt "$LOOPTUI_BIN" ]]; then
        echo -e "\033[0;36m[loop.sh] Building NDJSON renderer…\033[0m"
        if (cd "$LOOPTUI_DIR" && go build -o looptui . 2>&1); then
            echo -e "\033[0;36m[loop.sh] Renderer ready.\033[0m"
        else
            echo -e "\033[0;33m[loop.sh] Renderer build failed; falling back to plain mode.\033[0m"
        fi
    fi
    [[ -x "$LOOPTUI_BIN" ]] && RENDERER="$LOOPTUI_BIN"
fi

# ---------------------------------------------------------------------------
# Main Loop
# ---------------------------------------------------------------------------
ITERATION=0
STALL_COUNT=0
STATUS=0 # 0 all done · omp's code on omp failure · 3 stall exhausted · 130 interrupted

echo -e "\033[1;36m========================================================\033[0m"
echo -e "\033[1;36m  Starting Oh My Pi Loop\033[0m"
echo -e "  Document : \033[1;32m$DOC_FILE\033[0m"
if [[ "$MAX_ITERATIONS" -gt 0 ]]; then
    echo -e "  Max Runs : $MAX_ITERATIONS"
else
    echo -e "  Max Runs : Continuous (until all tasks complete)"
fi
echo -e "\033[1;36m========================================================\033[0m\n"

while true; do
    if [[ "$INTERRUPTED" -eq 1 ]]; then
        break
    fi

    if [[ "$MAX_ITERATIONS" -gt 0 && "$ITERATION" -ge "$MAX_ITERATIONS" ]]; then
        echo -e "\033[1;33m[loop.sh] Reached maximum iterations ($MAX_ITERATIONS) with tasks remaining. Stopping.\033[0m"
        STATUS=2
        break
    fi

    # Blocked tasks (- [!]) are terminal: only a human can clear them.
    BLOCKED_TASKS=$(grep -c '^[[:space:]]*- \[!\]' "$DOC_FILE" 2>/dev/null || true)
    BLOCKED_TASKS="${BLOCKED_TASKS:-0}"
    if [[ "$BLOCKED_TASKS" -gt 0 ]]; then
        echo -e "\033[1;31m[loop.sh] $BLOCKED_TASKS blocked task(s) (- [!]) in $DOC_FILE need human input. Stopping.\033[0m"
        grep -n '^[[:space:]]*- \[!\]' "$DOC_FILE" || true
        STATUS=4
        break
    fi

    # Count remaining unchecked tasks
    REMAINING_TASKS=$(grep -c '^[[:space:]]*- \[ \]' "$DOC_FILE" 2>/dev/null || true)
    REMAINING_TASKS="${REMAINING_TASKS:-0}"

    if [[ "$REMAINING_TASKS" -eq 0 ]]; then
        echo -e "\033[1;32m[loop.sh] No unchecked tasks (- [ ]) remaining in $DOC_FILE. All done!\033[0m"
        break
    fi

    # ITERATION counts omp runs only: every guard above exits without running one.
    ITERATION=$((ITERATION + 1))

    echo -e "\033[1;34m--------------------------------------------------------\033[0m"
    echo -e "\033[1;34mIteration $ITERATION\033[0m | $(date '+%Y-%m-%d %H:%M:%S') | Remaining tasks: \033[1;33m$REMAINING_TASKS\033[0m"
    echo -e "\033[1;34m--------------------------------------------------------\033[0m"

    # Define prompt for this turn
    PROMPT="Look at the migration doc ($DOC_FILE). Identify the next section that needs to be implemented. Work thru the tasks one at a time, checking them off (- [x]) in the document as you go. If a task cannot be completed or requires user input, rewrite that checkbox as '- [!]' with the blocking issue inline on the same bullet, then exit. Never leave a '- [ ]' box that you have already decided you cannot do, and never skip past a '- [!]' box."

    # Execute omp in non-interactive print mode with a fresh session
    echo -e "\033[0;32m[loop.sh] Running: omp -p ${APPROVAL_ARGS[*]:-} @$DOC_FILE \"$PROMPT\" ${EXTRA_ARGS[*]:-}\033[0m\n"

    # Run omp
    echo -e "\033[0;32m[loop.sh] Running: omp -p ${APPROVAL_ARGS[*]:-} @$DOC_FILE ${EXTRA_ARGS[*]:-}\033[0m\n"

    # Run omp — pipe through NDJSON renderer when available, otherwise plain print mode
    if [[ -n "$RENDERER" ]]; then
        omp --mode json -p ${APPROVAL_ARGS[@]+"${APPROVAL_ARGS[@]}"} "@$DOC_FILE" "$PROMPT" ${EXTRA_ARGS[@]+"${EXTRA_ARGS[@]}"} | "$RENDERER"
        EXIT_CODE=${PIPESTATUS[0]}
    else
        omp -p ${APPROVAL_ARGS[@]+"${APPROVAL_ARGS[@]}"} "@$DOC_FILE" "$PROMPT" ${EXTRA_ARGS[@]+"${EXTRA_ARGS[@]}"}
        EXIT_CODE=$?
    fi

    # Check exit status
    if [[ $EXIT_CODE -eq 130 || $EXIT_CODE -eq 137 || $INTERRUPTED -eq 1 ]]; then
        echo -e "\n\033[1;33m[loop.sh] Execution interrupted. Exiting loop.\033[0m"
        STATUS=130
        break
    elif [[ $EXIT_CODE -ne 0 ]]; then
        echo -e "\n\033[1;31m[loop.sh] omp exited with error code $EXIT_CODE. Aborting loop.\033[0m"
        STATUS=$EXIT_CODE
        break
    fi

    # Re-check remaining tasks after session
    POST_REMAINING=$(grep -c '^[[:space:]]*- \[ \]' "$DOC_FILE" 2>/dev/null || true)
    POST_REMAINING="${POST_REMAINING:-0}"
    COMPLETED_IN_TURN=$((REMAINING_TASKS - POST_REMAINING))

    if [[ $COMPLETED_IN_TURN -gt 0 ]]; then
        STALL_COUNT=0
        echo -e "\033[1;32m[loop.sh] Completed $COMPLETED_IN_TURN task(s) this turn. $POST_REMAINING remaining.\033[0m"
    else
        STALL_COUNT=$((STALL_COUNT + 1))
        echo -e "\033[0;33m[loop.sh] Task count unchanged ($POST_REMAINING remaining). No-progress run $STALL_COUNT/$STALL_LIMIT.\033[0m"
        if [[ "$STALL_COUNT" -ge "$STALL_LIMIT" ]]; then
            echo -e "\033[1;31m[loop.sh] No checkbox progress in $STALL_LIMIT consecutive runs; a task is likely blocked. Inspect $DOC_FILE.\033[0m"
            STATUS=3
            break
        fi
    fi

    if [[ "$POST_REMAINING" -eq 0 ]]; then
        POST_BLOCKED=$(grep -c '^[[:space:]]*- \[!\]' "$DOC_FILE" 2>/dev/null || true)
        if [[ "${POST_BLOCKED:-0}" -gt 0 ]]; then
            echo -e "\033[1;31m[loop.sh] No unchecked tasks left, but ${POST_BLOCKED} blocked (- [!]) need human input.\033[0m"
            grep -n '^[[:space:]]*- \[!\]' "$DOC_FILE" || true
            STATUS=4
        else
            echo -e "\033[1;32m[loop.sh] All tasks in $DOC_FILE have been checked off!\033[0m"
        fi
        break
    fi

    echo -e "\033[0;37m[loop.sh] Preparing next fresh session in ${SLEEP_SECONDS}s... (Ctrl+C to abort)\033[0m\n"
    sleep "$SLEEP_SECONDS"
done

echo -e "\n\033[1;36m[loop.sh] Finished after $ITERATION iteration(s) with status $STATUS.\033[0m"
exit "$STATUS"
