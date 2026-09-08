# Repository Improvements & Engineering Roadmap

This document outlines prioritized, actionable engineering improvements for `will-msg` following the Wails v2 + Vue 3 desktop migration.

---

## 1. UI/UX & Frontend Enhancements

- [ ] **Table Virtualization / Pagination for Massive Datasets**
  - For datasets exceeding 1,000 messages or 10,000 extracted records, replace plain DOM rendering in `CsvPreview.vue` with a virtualized scrolling table or paginated view to reduce DOM node memory footprint and rendering latency.

- [ ] **Dedicated Statistics & Metrics Dashboard**
  - Create a dedicated stats view/tab in the GUI utilizing the pre-existing `internal/stats` engine (`stats.DailyBreakdown`, `stats.SummaryStats`).
  - Visualize daily tag distributions, common exception categories, dispatcher frequency, and total service disruptions.

- [ ] **File List Search, Filter & Multi-Selection in Source Panel**
  - Add search/filter capabilities to the detected file list in `SourcePanel.vue`.
  - Allow selective exclusion or removal of individual detected `.msg` files prior to running the parser.

- [ ] **Drag-and-Drop Rule Reordering**
  - Add intuitive drag-and-drop handle reordering to `RulesTable.vue` in addition to the existing Move Up / Move Down buttons.

---

## 2. Engine & Parser Enhancements

- [ ] **Real-Time Parse Progress & Streaming Events**
  - Emit Wails runtime progress events (`parse:progress`) during batch parsing of large archives or folders, updating a progress bar in `RunParserButton.vue` with processed/skipped file counts.

- [ ] **Active Parse Cancellation**
  - Pass a cancellable context from the Go service layer to the parser loop so users can abort long-running parse operations without restarting the application.

- [ ] **Expanded Email & Archive Format Support**
  - Add support for standard `.eml` (RFC 822) email formats and `.tar.gz` / `.7z` archives alongside existing `.msg` and `.zip` inputs.

- [ ] **Rule Sandbox Batch Testing**
  - Allow running the rule sandbox against an imported CSV or multiple sample strings simultaneously to evaluate regex precision and recall across historical exception logs.

---

## 3. Packaging, Distribution & DevOps

- [ ] **macOS Code Signing & Apple Notarization**
  - Integrate Apple Developer ID code signing and `notarytool` automated notarization into the GitHub Actions release workflow (`.github/workflows/ci.yml`) to eliminate Gatekeeper warnings on macOS.

- [ ] **Windows Authenticode Code Signing & NSIS Installer**
  - Configure Wails NSIS installer generation (`wails build -nsis`) and Authenticode certificate signing in CI for seamless Windows installations.

- [ ] **Universal macOS Application Bundle**
  - Combine `arm64` and `amd64` macOS binaries into a single universal binary using `lipo` so users can download a single `.app` bundle that runs natively on all Apple Silicon and Intel Macs.

- [ ] **Single-Instance Application Locking**
  - Implement single-instance mutex/socket locking on startup to prevent multiple concurrent GUI instances from overwriting the shared user configuration file (`rules.json`).

- [ ] **Automated End-to-End (E2E) GUI Testing**
  - Add automated frontend/backend integration tests driving the Wails dev environment to smoke-test native file drop, parsing, and export flows in CI.
