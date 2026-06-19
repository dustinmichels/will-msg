#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
INPUT_DIR=${1:-"$ROOT_DIR/data"}
OUTPUT_FILE=${2:-"$ROOT_DIR/output/all_data.csv"}

(
	cd "$ROOT_DIR" &&
	go run . -input "$INPUT_DIR" -output "$OUTPUT_FILE"
)
