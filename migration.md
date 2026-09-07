# will-msg: Fyne → Wails migration checklist

Migrate `fyne-version/will-msg` (Go + Fyne desktop app) to `wails-version/will-msg`
(Go + Wails backend, **bun + Vue 3 + Tailwind** frontend).

Scope of Phases 0–12: **feature parity + modern frontend stack, nothing else.** New
capabilities, extra dependencies, and packaging changes live in Phase 13 (optional) and are
not required for the migration to be done.

The Go core is already UI-agnostic — only `internal/gui/` (1,677 lines of Fyne) is discarded and rebuilt.

---

## 0. Decisions (settle before writing code)

- [ ] **Wails v2.15.0 (stable)**, not v3 (beta as of 2026-09). Single-window app; v3's headline
      feature (native multi-window) is not needed — the Fyne "Rules & Labels" second window
      becomes an in-app route/modal. Revisit v3 only if multi-window is wanted later.
- [ ] **Frontend stack**: bun (package manager + task runner) · Vite · Vue 3 (`<script setup>`, TS)
      · Tailwind CSS v4 via `@tailwindcss/vite` (no `tailwind.config.js`, no PostCSS config).
- [ ] **State**: Pinia. Two stores (`parse`, `rules`); the rules editor has real cross-component
      state (working config + selection + dirty flag) that prop-drilling would smear.
- [ ] **Go core is copied verbatim, not rewritten.** `internal/{config,engine,parser,scanner,stats}`
      import zero UI packages (verified: no `fyne.io/*`, no `ncruces/zenity`).
- [ ] **`internal/gui/` is deleted, not ported.** Its business logic (sandbox eval, config
      validate+persist, import/export) moves into a bound Go service layer.
- [ ] **Config location and schema unchanged**: `os.UserConfigDir()/will-msg/rules.json`
      (macOS `~/Library/Application Support/will-msg/rules.json`), `version: 1`.
      Existing users' rules must carry over untouched.
- [ ] **CSV contract frozen**: 11 columns, exact order/headers
      `source_file, subject, message_date, reported_at, dispatcher, row_in_message, raw_entry,
    location, issue, label, issue_time` (`engine.CSVHeaders`, `Record.ToRow`).
- [ ] Decide fate of `fyne-version/`: keep as read-only reference during the migration, delete at
      cutover (Phase 12). Do not develop in both.

---

## 1. Prerequisites & scaffold

- [ ] Install Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest` (currently missing on this machine).
- [ ] `wails doctor` — confirm Xcode CLT on macOS.
- [ ] bun 1.4.0 ✔ present; Go ✔ present; `mise` ✔ present (`wails-version/will-msg/mise.toml` pins `go = "1.27.1"`).
- [ ] Scaffold into a temp dir (target dir already contains `data/`, `data-new/`, `mise.toml`, `.gitignore`):
      `wails init -n will-msg -t vue-ts -d /tmp/wails-scaffold`
- [ ] Copy scaffold into `wails-version/will-msg/`: `main.go`, `app.go`, `wails.json`, `go.mod`,
      `go.sum`, `frontend/`, `build/`.
      Keep the existing `mise.toml` and `.gitignore`; merge scaffold ignores into them.
- [ ] Replace npm with bun in `wails.json`:
      `"frontend:install": "bun install"`, `"frontend:build": "bun run build"`,
      `"frontend:dev:watcher": "bun run dev"`, `"frontend:dev:serverUrl": "auto"`.
      Delete `frontend/package-lock.json`; commit `frontend/bun.lock`.
- [ ] Set `wails.json` `info`: `productName: "will-msg"`, `companyName`, `copyright`, and
      `productVersion` carried over from `FyneApp.toml` (`Version = 1.0.0`, `Build = 35`,
      `ID = io.github.dustinmichels.willmsg`). A version bump is a separate release decision,
      not part of this migration.
- [ ] `wails init` already wrote `go.mod` — edit its `module` directive to `will-msg` (do **not**
      run `go mod init`; it fails on an existing file). The module name must stay `will-msg` so
      `will-msg/internal/...` imports keep working.
- [ ] Verify the empty shell runs: `wails dev` opens a window; `wails build` produces `build/bin/will-msg.app`.

---

## 2. Port the Go core (no behavior change)

- [ ] Copy verbatim: `internal/config/`, `internal/engine/`, `internal/parser/`, `internal/scanner/`,
      `internal/stats/` (+ their `_test.go` files).
- [ ] Copy `testdata/` (6 `.msg` fixtures + `msg_parsed.csv`).
- [ ] Copy CLI commands `cmd/will-msg/` (flags `-input`, `-output`; exit 2 on missing flag) and
      `cmd/msgcat/` (flag `-headers`, 1 positional arg, exit 2 on misuse). **Do not** copy `cmd/will-msg-gui/`.
- [ ] `go.mod`: require `github.com/wailsapp/wails/v2` + `github.com/willthrom/outlook-msg-parser`.
      Must **not** contain `fyne.io/fyne/v2`, `fyne.io/systray`, `github.com/ncruces/zenity`,
      `github.com/go-gl/*`, `github.com/fyne-io/*`.
- [ ] Reconcile Go version: `mise.toml` pins `1.27.1`, old `go.mod` says `go 1.23.0`. Pick one.
- [ ] `go mod tidy && go build ./... && go test ./internal/...` — all pre-existing engine/parser/
      scanner/stats/config tests green, unchanged.
- [ ] **Add JSON tags** so bindings emit stable snake_case instead of Go field names.
      `engine.Record` has **no** struct tags today — TS bindings would be `SourceFile`, `Subject`, …
      Tag them to match `CSVHeaders` (`json:"source_file"` …, `RowInMessage → row_in_message`).
      Same for `scanner.MessageSource` (`path`, `in_zip`, `zip_path`, `display_name`).
      `config.*` already has tags — leave them exactly as-is (`rules.json` compatibility).
- [ ] Add a tag-coverage test: marshal a `Record`, assert its keys equal `engine.CSVHeaders`.

---

## 3. Backend service layer (bound Go API)

Create `internal/appservice/` (or keep on the scaffold's `App` struct) holding `ctx context.Context`
from `OnStartup`, an `atomic.Pointer[engine.RuleEngine]` — the port of `gui.DefaultEngine` /
`SetDefaultEngine` / `ReloadDefaultEngine` (gui.go:42-68) — and, under one `sync.Mutex`,
`lastRecords []engine.Record` plus `parseGen uint64`.

**Why service-owned records, not an argument.** Fyne kept `csvData` in the window closure and both
export buttons read it directly (gui.go:188, 473, 538); the exported file was always exactly the
last parse, never a filtered or re-sorted view. Keeping that state in Go preserves the invariant and
avoids shipping thousands of rows back across the bridge. Invariant: `Parse` is the only writer;
export with empty `lastRecords` returns an error, and the frontend keeps the export buttons disabled
until `status === 'ready'` (Fyne disabled them identically, gui.go:375, 379, 453-454).

**Generation token (a mutex is not enough).** `Parse` runs off the UI path, so a `ClearParse()` — or
a second `Parse` — can land while the first is still walking files; the mutex serializes the writes
but does not stop the late one from resurrecting discarded records:

1. `Parse` takes the lock, increments `parseGen`, copies the value into a local `gen`, releases.
2. `ClearParse()` (and each new `Parse`) increments `parseGen` and nils `lastRecords`.
3. On completion `Parse` re-takes the lock and commits `lastRecords` **only if** `parseGen == gen`;
   otherwise it drops its result and emits nothing.

Same check gates `parse:done` / `parse:error` emission, so a superseded run cannot flip the
frontend to `ready`. Cover it with a test: start a parse, `ClearParse()`, let the parse finish,
assert `lastRecords` is empty and export errors.

### Parsing

- [ ] `SelectFiles() ([]string, error)` → `runtime.OpenMultipleFilesDialog` (filters `*.msg;*.zip`).
- [ ] `SelectFolder() (string, error)` → `runtime.OpenDirectoryDialog`.
      **Parity note / bug fix**: Fyne used one zenity `SelectFile` labeled "Select file or folder"
      with `*.msg;*.zip` filters, which cannot actually pick a folder on macOS. Two explicit
      affordances are the minimum honest equivalent of the advertised behavior.
- [ ] `ScanSource(path string) (ScanResult, error)` → wraps `scanner.FindSources`;
      returns `{sources: []scanner.MessageSource, count: int, path: string}`.
- [ ] `Parse(sources []scanner.MessageSource) (ParseResult, error)` → per-source
      `scanner.LoadSource` + `eng.ParseRecords`; commits to `lastRecords` (sole writer, generation-
      checked per above) and returns
      `{records: []engine.Record, skipped: []SkippedSource{display_name, error}}`.
- [ ] `ClearParse()` — bumps `parseGen` and nils `lastRecords` for the "Load New Source" reset
      (gui.go:693 set `csvData = nil`), so a stale export cannot outlive the cleared preview.
- [ ] Defensive copy of the source slice before handing it to a goroutine
      (gui.go:426 pattern; also an open `improvements.md` item).
- [ ] Run `Parse` off the UI path; emit `runtime.EventsEmit(ctx, "parse:done"|"parse:error")`.
      Replaces the Fyne `go func` + `fyne.Do` dance (gui.go:421-458).
      Progress events (`parse:progress`) are optional → Phase 13.
- [ ] Surface `skipped` in the result. The Fyne version dropped per-file failures into
      `log.Printf` (gui.go:77) where no user ever saw them; returning the list is the parity-safe fix.

### Export

- [ ] `SaveToDownloads() (SavedFile, error)` — writes `lastRecords` (headers + `ToRow`) to
      `getDownloadsDir()` (port of gui.go:123-133) as `msg_parsed_<2006-01-02_150405>.csv`;
      returns `{filename, dir, path}` so the frontend renders the "CSV Saved Automatically"
      confirmation (gui.go:484-506).
- [ ] `SaveAs() (SavedFile, error)` → `runtime.SaveFileDialog` (`DefaultFilename` =
      `msg_parsed_<timestamp>.csv`, filter `*.csv`), then writes `lastRecords` to the chosen path.
      Empty returned path = user cancelled, no error (replaces `errors.Is(err, zenity.ErrCanceled)`).
- [ ] `RevealFile(path string) error` — port `revealFile` (gui.go:135-170): `open -R` (darwin),
      `explorer.exe /select,` (windows), dbus `FileManager1.ShowItems` → `xdg-open` (linux).
      Drop the Fyne `storage.NewFileURI` fallback; use `runtime.BrowserOpenURL(ctx, "file://"+dir)`.
- [ ] CSV writing stays server-side (`encoding/csv`), rows from `engine.CSVHeaders` + `Record.ToRow`;
      flush before checking errors (verify the copied `writeCSV` in `cmd/will-msg/main.go` has this fix).

### Rules (replaces `internal/gui/gui_rules.go`, 915 lines)

- [ ] `GetRules() config.RuleConfig` — `config.LoadConfig()`.
- [ ] `GetDefaultRules() config.RuleConfig` — `config.DefaultRuleConfig()` (for "Reset Defaults").
- [ ] `ValidateRules(cfg config.RuleConfig) []ValidationError` — wraps `cfg.Validate()` +
      `config.CompileRegexPattern` per regex rule; return per-rule errors, not one opaque string.
- [ ] `SaveRules(cfg config.RuleConfig) error` — `engine.NewRuleEngineValidated` → `config.SaveConfig`
      → swap the atomic engine pointer → emit `"rules:applied"`.
- [ ] `SaveRulesToPath(path string, cfg) error` / `LoadRulesFromPath(path string)` — keeps the
      custom-config-path feature from commit `52182f1`.
- [ ] `ImportRules() (config.RuleConfig, error)` → `OpenFileDialog` (`*.json`) + `config.ParseRuleConfig`
      (which also runs `MigrateConfig`).
- [ ] `ExportRules(cfg config.RuleConfig) (string, error)` → `SaveFileDialog` + `cfg.ToJSON()`.
- [ ] `TestRule(input string, cfg config.RuleConfig) SandboxResult` — port `runSandbox`
      (gui_rules.go:403): build a temp engine, return
      `{address, status, issue_time, label, matched_rule_index, matched_rule_id, match_kind:
    "rule"|"heuristic"|"fallback", metric}` from `Classify` + `MatchRule` + `MetricForLabel`.
- [ ] Rule list mutations (add/edit/delete/move up/move down/toggle) stay **client-side** on the
      working config; only `ValidateRules`/`SaveRules`/`TestRule` cross the bridge. Precedence =
      array order; first enabled match wins — preserve order exactly.
- [ ] Label CRUD: inline label creation (`[+ Add New Label...]`, gui_rules.go:560) needs key
      validation (snake_case, unique) + `metric ∈ {none, trash, recycling, both}`.

### Wiring

- [ ] `main.go`: `//go:embed all:frontend/dist`, `wails.Run(&options.App{...})` with
      `Title: "Outlook MSG to CSV Parser"`, `Width: 950`, `Height: 700` (Fyne's size),
      `MinWidth`/`MinHeight`, `BackgroundColour` = slate-50 `#F8FAFC`,
      `Bind: []any{app, rulesSvc}`, `OnStartup`, `OnShutdown`.
- [ ] `DragAndDrop: &options.DragAndDrop{EnableFileDrop: true, CSSDropProperty: "--wails-drop-target", CSSDropValue: "drop"}`
      and `runtime.OnFileDrop(ctx, func(x, y int, paths []string))` → replaces `w.SetOnDropped`
      (gui.go:412-419). Fyne consumed only `uris[0]`; matching that is fine, accepting the slice is free.

---

## 4. Frontend scaffold

- [ ] `cd frontend && bun install`
- [ ] `bun add -d tailwindcss @tailwindcss/vite` ; `bun add pinia`
- [ ] `vite.config.ts`: `plugins: [vue(), tailwindcss()]`, `@` alias → `src`.
- [ ] `src/style.css`: `@import "tailwindcss";` + `@theme` tokens (Phase 5).
- [ ] `tsconfig`: `strict: true`; path alias for the generated `wailsjs` bindings.
- [ ] Scripts in `frontend/package.json`: `dev`, `build` (`vue-tsc --noEmit && vite build`).
- [ ] Delete scaffold demo files (`Greet` component, logo assets, boilerplate CSS).
- [ ] Confirm bindings generate into `frontend/wailsjs/go/...`; decide committed vs gitignored and be consistent.

---

## 5. Design system (port the Fyne theme)

Exact colors from `customTheme` (gui.go:92-121) → Tailwind v4 `@theme` tokens:

| Token                                | Hex             | Fyne source                                    |
| ------------------------------------ | --------------- | ---------------------------------------------- |
| `--color-truck-500` (primary)        | `#6CB944`       | `ColorNamePrimary` (108,185,68)                |
| `--color-truck-600` (focus)          | `#559B32`       | `ColorNameFocus` (85,155,50)                   |
| `--color-truck-100` (drop-zone tint) | `#E8F7DC`       | drop zone bg (232,247,220)                     |
| `--color-truck-200` (table header)   | `#DCF5C3`       | light table header (220,245,195)               |
| `--color-truck-btn`                  | `#DCEDD2`       | `ColorNameButton` light (220,237,210)          |
| `--color-truck-input`                | `#F1F8ED`       | `ColorNameInputBackground` light (241,248,237) |
| `--color-slate-900`                  | `#0F172A`       | dark background (15,23,42)                     |
| `--color-slate-800`                  | `#1E293B`       | dark surface / status bar (30,41,59)           |
| `--color-slate-50`                   | `#F8FAFC`       | light background (248,250,252)                 |
| selection overlay                    | `#6CB944` @ 31% | `ColorNameSelection` (alpha 80/255)            |

- [ ] Light/dark via `prefers-color-scheme` + `dark:` variants (Fyne branched on
      `theme.VariantDark` in 4 places).
- [ ] Copy `internal/gui/truck.png` → `frontend/src/assets/truck.png`; it becomes a frontend asset,
      so delete the Go `//go:embed truck.png` + `truckResource`.
- [ ] Generate `build/appicon.png` (1024×1024) from `truck.png`.
- [ ] Type scale: Fyne header sizes were 18 px (title), 13 px (subtitle), 28 px (welcome headline);
      use tabular numerals in the CSV table.

---

## 6. Frontend architecture

- [ ] `src/stores/parse.ts` — `sourcePath`, `sources[]`, `records[]`, `skipped[]`,
      `status: idle|scanning|parsing|ready|error`.
      Replaces the Fyne closure state `currentSources`, `displayNames`, `csvData` (gui.go:186-189).
- [ ] `src/stores/rules.ts` — `savedConfig`, `workingConfig`, `selectedIndex`, `isDirty`,
      `validation[]`; mutations for add/edit/delete/move/toggle/reset.
- [ ] `src/composables/useWailsEvents.ts` — typed `EventsOn` wrappers for `parse:done`,
      `parse:error`, `rules:applied`, file drop.
- [ ] `src/lib/toast.ts` — replaces `dialog.ShowError` / `dialog.ShowInformation`
      (≈12 call sites across gui.go / gui_rules.go).
- [ ] Screen switching: `bodyContainer` + `showWelcome()`/`showWorkspace()` (gui.go:740-748)
      becomes a `view` computed; the rules manager is a third view (Fyne used a second window).

---

## 7. Vue components (feature parity, screen by screen)

### Shell

- [ ] `AppHeader.vue` — 50 px truck-green bar: truck logo, "Outlook MSG Parser",
      subtitle "Feed me your msg files, Will", "Rules & Labels" button (gui.go:584-616).
- [ ] `StatusBar.vue` — 30 px slate-800 strip with the Pac-Man animation (gui.go:195-298:
      raster Pac-Man, 40 dots at 50 px spacing, 30 ms ticker, mouth `|sin(t)|*0.8`, 1.4 px/frame,
      wraps at the right edge) as CSS keyframes or a small `<canvas>`.
      Fyne ran it continuously (stopping only on window close, and only after the fix in
      `improvements.md`); gate it on `status === 'parsing'` or keep it always-on — your call, it is
      pure decoration either way.
- [ ] `ToastHost.vue`, `ConfirmDialog.vue`, `Modal.vue` primitives.

### Welcome screen (gui.go:618-688)

- [ ] `WelcomeScreen.vue` — rounded (16 px) drop zone, truck-green 2 px stroke, light-green tint,
      upload icon, headline "Feed me your msg files, Will", copy
      "Drop or select. Accepts .msg files, folders containing .msg files, or .zip archives."
- [ ] Drop-target highlight via the `wails-drop-target-active` class from `OnFileDrop(cb, true)`.
- [ ] Buttons: **Choose files…** (`SelectFiles`) and **Choose folder…** (`SelectFolder`).
- [ ] Empty-scan feedback — Fyne showed a modal "No .msg files were found in the selected source."
      (gui.go:408); a toast or inline warning is equivalent.

### Workspace (gui.go:690-738)

- [ ] `SourcePanel.vue` — "Load New Source" reset (clears sources/records/preview, gui.go:690-702),
      selected path (word-wrapped), "Found N .msg files" (italic), scrollable detected-file list
      with document icon + ellipsis truncation (gui.go:306-327).
- [ ] `RunParserButton.vue` — disabled until sources exist; disables Run/Download/Save-As while
      parsing, re-enables on completion or error (gui.go:421-456).
- [ ] `CsvPreview.vue` — table, 11 columns, sticky bold header row
      (light `#DCF5C3` / dark `#1E293B`), Fyne column widths as defaults:
      `150,150,120,120,80,50,250,150,120,80,80`.
      Fyne's `widget.Table` recycled cells; a plain DOM table does not. Measure with `data-new/`
      (~200 files) before adding any virtualization dependency → Phase 13.
- [ ] `ExportBar.vue` — "Download CSV" (`SaveToDownloads`) + "Save As…" (`SaveAs`), then the
      saved-file confirmation (File Name / Saved To / **Show in Folder** → `RevealFile`;
      gui.go:484-506, 556-580).
- [ ] `SkippedSourcesNotice.vue` — renders `ParseResult.skipped`.

### Rules & Labels manager (gui_rules.go)

- [ ] `RulesView.vue` — in-app view, not a second window; header "Rules & Labels Manager"
      (the `activeRuleEditorWindow` singleton at gui_rules.go:51 disappears).
- [ ] `RulesTable.vue` — columns `#`, Type, Pattern, Target Label, Metric, Enabled, Description
      (widths `45,95,260,190,95,75,200`); selected row translucent green; disabled rows italic.
- [ ] Toolbar (gui_rules.go:147-213): Add Rule, Edit, Delete (confirm), Move Up, Move Down,
      Toggle On/Off, Reset Defaults (confirm), Import JSON, Export JSON — with the same
      enable/disable rules (Edit/Delete/Toggle need a selection; Move Up/Down disabled at the ends).
- [ ] Settings bar (gui_rules.go:214): "Enable heuristics" checkbox (`EnableHeuristics`),
      "Default label" select (`DefaultLabel`), rule-count label.
- [ ] `RuleFormModal.vue` (gui_rules.go:560) — Pattern, Match Type (`substring`/`regex`),
      Target Label select including `[+ Add New Label…]`, Description, Enabled toggle.
      Validation: non-empty pattern, regex compiles, new-label key snake_case + unique + metric chosen.
- [ ] `RuleSandbox.vue` (gui_rules.go:347-464) — debounced live input (default sample
      `45 FOREST ST TRASH AND RCY NOT OUT 0830AM`), 4 sample buttons
      (Both Not Out / Contaminated / Blocked / Heuristic Suffix) + Clear, and 6 result cards:
      Address, Issue/Status, Issue Time, Assigned Label, Matched Rule (rule # / heuristic / fallback),
      Metric Impact.
- [ ] Action bar (gui_rules.go:329): **Save & Apply Rules** (validate → save → engine swap → toast)
      and **Cancel** (discard); warn when leaving with `isDirty`.

---

## 8. Native integration checks

- [ ] File drop of a folder, a `.msg`, a `.zip`, and multiple files each reach `ScanSource`.
- [ ] Cancelled dialogs return empty/nil and produce no error toast.
- [ ] `RevealFile` verified on macOS; Windows/Linux branches compile and are smoke-tested where available.
- [ ] Paths with spaces and parentheses work (real fixtures: `MEDFORD TAGS 04_16_26 (1).msg`).
- [ ] Windows: WebView2 runtime — either document the prerequisite or build with `-webview2 embed`.
- [ ] The built `.app` works **outside** the repo directory (Fyne had an asset-path bug fixed by
      `//go:embed`; confirm `frontend/dist` is embedded, not read from disk).

---

## 9. Verification

- [ ] `go test ./...` green (core tests unchanged).
- [ ] **Golden CSV equivalence — the gate that blocks cutover.** Run the old Fyne binary and the new
      app over `testdata/` and `data/`; `cmp` the CSVs byte-for-byte (ignoring the timestamped
      filename). Any diff is a migration bug.
- [ ] Go service tests replacing `internal/gui/gui_rules_test.go` (369 lines) coverage:
      reorder precedence, toggle enabled, sandbox result for each built-in sample, invalid-regex
      rejection, reset-to-defaults, import/export round-trip, save→engine-swap.
- [ ] Parse-lifecycle tests: `Parse` populates `lastRecords`; `ClearParse()` mid-parse leaves it
      empty after the parse returns (generation check) and export then errors; a second `Parse`
      supersedes the first, and the superseded run emits no `parse:done`.
- [ ] Delete `internal/gui/gui_test.go` (47 lines — asserts the embedded `truck.png` bytes and Fyne
      resource wiring; the asset moved to the frontend). Re-home its `parseMsgSources` coverage as a
      `Parse` service test.
- [ ] `rules.json` written by the new app is byte-compatible with the Fyne app's file
      (same path, same schema, `version: 1`); load an existing user file unchanged.
- [ ] Manual smoke run (`wails dev`): drop folder → scan → run → preview → Download CSV →
      Show in Folder; then edit a rule → sandbox → Save & Apply → re-run → labels change.

---

## 10. Build & packaging (replaces fyne-cross)

Parity target = the same artifact set the Fyne build produced.

- [ ] `wails build -platform darwin/arm64 -clean` → `bin/will-msg-macos-arm64.app` (+ zip).
- [ ] `wails build -platform darwin/amd64 -clean` → `bin/will-msg-macos-amd64.app` (+ zip).
- [ ] `wails build -platform windows/amd64 -clean` → `bin/will-msg-windows-amd64.exe` (+ zip).
- [ ] Rewrite `wails-version/will-msg/mise.toml` tasks: `dev` (`wails dev`), `build`,
      `build-darwin`, `build-windows`, `test`, and keep the existing `parse`
      (`go run ./cmd/msgcat`) helper. Delete `scripts/build-darwin.sh` (fyne-cross specific).
- [ ] Drop the Docker prerequisite (fyne-cross needed it; Wails does not for these targets).
- [ ] macOS `Info.plist`: product name/version, bundle ID `io.github.dustinmichels.willmsg`.
- [ ] `.gitignore`: add `frontend/node_modules/`, `frontend/dist/`, `build/bin/`;
      keep the existing `data*/`, `output/`, `sample/`, `bin/` entries.

---

## 11. Docs

- [ ] Rewrite `README.md`: new stack, `wails dev` / `wails build`, bun commands, prerequisites
      (Go, bun, Wails CLI, WebView2 on Windows). Delete the Fyne/CGo/OpenGL/fyne-cross/Docker section.
- [ ] Keep the `msgcat` and CLI (`-input`/`-output`) sections — those binaries survive unchanged.
- [ ] Rewrite `claude.md` for the new layout (Go services + Vue frontend, bun, where bindings live).
- [ ] Prune `improvements.md`: items already implemented (embed assets, log mutex, defensive copy,
      engine validation, parser/GUI/scanner decoupling) and items this migration obsoletes
      (Pac-Man raster CPU, Fyne raster border on high-DPI, Save-As feedback). Carry forward the rest.

---

## 12. Cutover

- [ ] Feature-parity walkthrough against the matrix below; every row checked.
- [ ] Verify nothing Fyne-shaped was copied in: no `internal/gui/`, `cmd/will-msg-gui/`,
      `FyneApp.toml`, `fyne-cross/`, `Icon.png`, root `truck.png`.
- [ ] Delete `fyne-version/` (or park it on a `legacy-fyne` tag/branch) so there is one source of truth.
- [ ] Decide whether `wails-version/will-msg` gets promoted to the repo root layout.
- [ ] Final `go mod tidy`, `bun install --frozen-lockfile`, full build on macOS (+ Windows if available).

---

## Feature parity matrix (Fyne → Wails)

| Fyne feature (source)                                   | Wails/Vue replacement                       | Done |
| ------------------------------------------------------- | ------------------------------------------- | ---- |
| Window "Outlook MSG to CSV Parser" 950×700 (gui.go:178) | `options.App{Title, Width, Height}`         | ☐    |
| Custom truck-green theme, light/dark (gui.go:92-121)    | Tailwind `@theme` tokens                    | ☐    |
| Drop files onto window (gui.go:412)                     | `EnableFileDrop` + `runtime.OnFileDrop`     | ☐    |
| zenity "Select file or folder" (gui.go:631)             | `SelectFiles` + `SelectFolder`              | ☐    |
| `scanner.FindSources` + count labels (gui.go:386-410)   | `ScanSource` + `SourcePanel.vue`            | ☐    |
| Detected-files list (gui.go:306)                        | list in `SourcePanel.vue`                   | ☐    |
| Run Parser + background goroutine (gui.go:421)          | `Parse` + `parse:done`/`parse:error` events | ☐    |
| Per-file failures logged only (gui.go:77)               | `ParseResult.skipped` + notice              | ☐    |
| CSV preview table, 11 cols (gui.go:329-367)             | `CsvPreview.vue`                            | ☐    |
| Download CSV → ~/Downloads (gui.go:460)                 | `SaveToDownloads`                           | ☐    |
| Save As… via zenity (gui.go:509)                        | `SaveAs` → `runtime.SaveFileDialog`         | ☐    |
| Saved-file confirmation dialog (gui.go:484-506)         | confirmation card                           | ☐    |
| "Show in Folder" (gui.go:135-170)                       | `RevealFile`                                | ☐    |
| Pac-Man status bar animation (gui.go:195-298)           | CSS/canvas `StatusBar.vue`                  | ☐    |
| "Load New Source" reset (gui.go:690)                    | store reset action                          | ☐    |
| Rules & Labels window (gui_rules.go:51)                 | `RulesView.vue`                             | ☐    |
| Rules table + toolbar (gui_rules.go:147-328)            | `RulesTable.vue` + toolbar                  | ☐    |
| Heuristics toggle + default label (gui_rules.go:214)    | settings bar                                | ☐    |
| Add/Edit rule dialog (gui_rules.go:560)                 | `RuleFormModal.vue`                         | ☐    |
| Inline new-label creation (gui_rules.go:560)            | label sub-form + validation                 | ☐    |
| Live sandbox (gui_rules.go:347-464)                     | `RuleSandbox.vue` + `TestRule`              | ☐    |
| Import/Export rules JSON (gui_rules.go:777-881)         | `ImportRules` / `ExportRules`               | ☐    |
| Reset defaults (gui_rules.go:487)                       | `GetDefaultRules` + confirm                 | ☐    |
| Save & Apply rules (gui_rules.go:737-775)               | `SaveRules` + engine swap                   | ☐    |
| Custom config path (commit 52182f1)                     | `Load/SaveRulesToPath`                      | ☐    |
| `dialog.ShowError` / `ShowInformation`                  | toast host                                  | ☐    |
| `cmd/will-msg`, `cmd/msgcat`                            | copied unchanged                            | ☐    |

---

## Risks / watch items

- **Binding field names.** `engine.Record` and `scanner.MessageSource` have no JSON tags; without
  the Phase 2 tagging step the TS bindings use Go field names and the frontend silently diverges
  from the CSV contract. Tag before writing components.
- **Rule precedence is array order.** Any frontend transform (sort, filter, `map` over a copy) must
  not reorder `cfg.Rules` before `SaveRules`.
- **Config compatibility.** Do not touch `config.*` struct tags, `ConfigDirName`, `ConfigFileName`,
  or `CurrentConfigVersion`.
- **Preview scale.** `data-new/` holds ~200 `.msg` files; Fyne's table recycled cells, a DOM table
  does not. Measure before reaching for a virtualization library.
- **`RevealFile` platform branches** and zip-internal source paths (`MessageSource.InZip`) are the
  least-tested code paths in the port.
- **Wails v3 beta** exists and changes bindings/build significantly. Keep the Phase 3 service layer
  free of `runtime.*` calls where practical so a later v3 move is mechanical.

---

## 13. Optional / post-migration (explicitly NOT parity)

Do none of these before Phase 12 is green. Each adds scope or a dependency the Fyne app did not have.

- [ ] **Stats dashboard.** `internal/stats` (`ComputeStats`, `DailyStats`, `SummaryStats`) exists and
      is **unused by the Fyne GUI**. Surfacing averages + daily series is a new feature: needs a
      `ComputeStats` binding, JSON tags on the stats structs, a `StatsView.vue`, and a chart approach.
- [ ] **Table virtualization** (`@tanstack/vue-virtual` or similar) — only if Phase 7 measurement
      shows the plain table stalls.
- [ ] **CSV preview affordances**: column sort, label filter chips, text search, resizable columns,
      row detail drawer (`raw_entry` vs parsed fields).
- [ ] **File list affordances**: search/filter, per-file parsed-row count, zip badge for `InZip`.
- [ ] **Parse progress events + cancellation** (`parse:progress`, `Cancel()`); the Fyne app had neither.
- [ ] **Drag-to-reorder rules** replacing Move Up/Down (keep a keyboard-accessible fallback).
- [ ] **Sandbox extras**: highlight the matching rule row, show the regex capture span.
- [ ] **macOS universal binary** (`-platform darwin/universal`) instead of two per-arch bundles.
- [ ] **Windows NSIS installer** (`-nsis`) in addition to the zip.
- [ ] **Code signing / notarization** for distribution outside the org.
- [ ] **`vue-router`** if the three views outgrow a `view` computed.
- [ ] **Frontend test tooling** (Vitest for store/table logic, Playwright E2E) — Phase 9's Go tests +
      golden CSV + manual smoke are the parity bar.
- [ ] **`SingleInstanceLock`** app option.
