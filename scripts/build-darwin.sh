#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
BIN_DIR="$ROOT_DIR/bin"
FYNE_CROSS_DIR="$ROOT_DIR/fyne-cross"

(
	cd "$ROOT_DIR"

	fyne-cross darwin -arch arm64,amd64
	mkdir -p "$BIN_DIR"
	cp "$FYNE_CROSS_DIR/bin/darwin-amd64/will-msg" "$BIN_DIR/will-msg-macos-amd64"
	cp "$FYNE_CROSS_DIR/bin/darwin-arm64/will-msg" "$BIN_DIR/will-msg-macos-arm64"
	rm -rf "$BIN_DIR/will-msg-macos-amd64.app" "$BIN_DIR/will-msg-macos-arm64.app"
	cp -r "$FYNE_CROSS_DIR/dist/darwin-amd64/will-msg.app" "$BIN_DIR/will-msg-macos-amd64.app"
	cp -r "$FYNE_CROSS_DIR/dist/darwin-arm64/will-msg.app" "$BIN_DIR/will-msg-macos-arm64.app"

	# Clear external attributes and ad-hoc sign to prevent Gatekeeper "damaged" warnings
	xattr -cr "$BIN_DIR/will-msg-macos-amd64" "$BIN_DIR/will-msg-macos-arm64" "$BIN_DIR/will-msg-macos-amd64.app" "$BIN_DIR/will-msg-macos-arm64.app"
	codesign --force --sign - "$BIN_DIR/will-msg-macos-amd64" "$BIN_DIR/will-msg-macos-arm64"
	codesign --force --deep --sign - "$BIN_DIR/will-msg-macos-amd64.app" "$BIN_DIR/will-msg-macos-arm64.app"

	# Package the clean, signed bundles into zips
	(
		cd "$BIN_DIR"
		rm -f will-msg-macos-amd64.zip
		zip -q -r will-msg-macos-amd64.zip will-msg-macos-amd64.app
	)
	(
		cd "$BIN_DIR"
		rm -f will-msg-macos-arm64.zip
		zip -q -r will-msg-macos-arm64.zip will-msg-macos-arm64.app
	)
)
