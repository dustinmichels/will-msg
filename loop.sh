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
# Main Loop
# ---------------------------------------------------------------------------
ITERATION=0

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

    ITERATION=$((ITERATION + 1))

    if [[ "$MAX_ITERATIONS" -gt 0 && "$ITERATION" -gt "$MAX_ITERATIONS" ]]; then
        echo -e "\033[1;33m[loop.sh] Reached maximum iterations ($MAX_ITERATIONS). Stopping.\033[0m"
        break
    fi

    # Count remaining unchecked tasks
    REMAINING_TASKS=$(grep -c '^[[:space:]]*- \[ \]' "$DOC_FILE" 2>/dev/null || true)

    echo -e "\033[1;34m--------------------------------------------------------\033[0m"
    echo -e "\033[1;34mIteration $ITERATION\033[0m | $(date '+%Y-%m-%d %H:%M:%S') | Remaining tasks: \033[1;33m$REMAINING_TASKS\033[0m"
    echo -e "\033[1;34m--------------------------------------------------------\033[0m"

    if [[ "$REMAINING_TASKS" -eq 0 ]]; then
        echo -e "\033[1;32m[loop.sh] No unchecked tasks (- [ ]) remaining in $DOC_FILE. All done!\033[0m"
        break
    fi

    # Define prompt for this turn
    PROMPT="Look at the migration doc ($DOC_FILE). Identify the next section that needs to be implemented. Work thru the tasks one at a time, checking them off in the document as you go. If the task cannot be completed, or requires user input, write the issue directly in the document then exit."

    # Execute omp in non-interactive print mode with a fresh session
    echo -e "\033[0;32m[loop.sh] Running: omp -p @$DOC_FILE \"$PROMPT\" ${EXTRA_ARGS[*]}\033[0m\n"

    # Run omp
    omp -p "@$DOC_FILE" "$PROMPT" "${EXTRA_ARGS[@]}"
    EXIT_CODE=$?

    # Check exit status
    if [[ $EXIT_CODE -eq 130 || $EXIT_CODE -eq 137 || $INTERRUPTED -eq 1 ]]; then
        echo -e "\n\033[1;33m[loop.sh] Execution interrupted. Exiting loop.\033[0m"
        break
    elif [[ $EXIT_CODE -ne 0 ]]; then
        echo -e "\n\033[1;31m[loop.sh] omp exited with error code $EXIT_CODE.\033[0m"
    fi

    # Re-check remaining tasks after session
    POST_REMAINING=$(grep -c '^[[:space:]]*- \[ \]' "$DOC_FILE" 2>/dev/null || true)
    COMPLETED_IN_TURN=$((REMAINING_TASKS - POST_REMAINING))

    if [[ $COMPLETED_IN_TURN -gt 0 ]]; then
        echo -e "\033[1;32m[loop.sh] Completed $COMPLETED_IN_TURN task(s) this turn. $POST_REMAINING remaining.\033[0m"
    else
        echo -e "\033[0;33m[loop.sh] Task count unchanged ($POST_REMAINING remaining).\033[0m"
    fi

    if [[ "$POST_REMAINING" -eq 0 ]]; then
        echo -e "\033[1;32m[loop.sh] All tasks in $DOC_FILE have been checked off!\033[0m"
        break
    fi

    echo -e "\033[0;37m[loop.sh] Preparing next fresh session in ${SLEEP_SECONDS}s... (Ctrl+C to abort)\033[0m\n"
    sleep "$SLEEP_SECONDS"
done

echo -e "\n\033[1;36m[loop.sh] Finished after $ITERATION iteration(s).\033[0m"
