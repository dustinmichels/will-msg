# will-msg: Fyne → Wails migration checklist

Migrate `fyne-version/will-msg` (Go + Fyne desktop app) to `wails-version/will-msg`
(Go + Wails backend, **bun + Vue 3 + Tailwind** frontend).

Scope of Phases 0–12: **feature parity + modern frontend stack.** Deliberate behavior changes are
limited to separate file/folder pickers, visible skipped-file errors, a dirty-rules exit guard, and
a parsing-only animation that honors reduced motion. Everything else belongs in Phase 13.

The Go core is already UI-agnostic — only `internal/gui/` (1,677 lines of Fyne) is discarded and rebuilt.

---

## 0. Decisions (settle before writing code)

- [x] **Pin Wails v2.15.0** in both `go.mod` and the CLI install; do not use `@latest`. Wails v3 is
      still beta as of 2026-09, and this is a single-window app. Revisit v3 only after this
      migration; do not mix v2 and v3 APIs or generated bindings.
- [x] **Frontend stack**: bun (package manager + task runner) · Vite · Vue 3 (`<script setup>`, TS)
      · Tailwind CSS v4 via `@tailwindcss/vite` (no `tailwind.config.js`, no PostCSS config).
- [x] **State**: Pinia. Two stores (`parse`, `rules`); the rules editor has real cross-component
      state (working config + selection + dirty flag) that prop-drilling would smear.
- [x] **Copy the Go core before changing it.** `internal/{config,engine,parser,scanner,stats}`
      import zero UI packages (verified). Limit core changes to the JSON tags required by the
      Wails boundary; classification, scanning, config, and CSV behavior stay frozen.
- [x] **`internal/gui/` is deleted, not ported.** Its business logic (sandbox eval, config
      validate+persist, import/export) moves into a bound Go service layer.
- [x] **Config location and schema unchanged**: `os.UserConfigDir()/will-msg/rules.json`
      (macOS `~/Library/Application Support/will-msg/rules.json`), `version: 1`.
      Existing users' rules must carry over untouched.
- [x] **CSV contract frozen**: 11 columns, exact order/headers
      `source_file, subject, message_date, reported_at, dispatcher, row_in_message, raw_entry,
location, issue, label, issue_time` (`engine.CSVHeaders`, `Record.ToRow`).
- [x] Keep `fyne-version/` read-only through validation. At cutover, tag the last Fyne commit
      `legacy-fyne`, promote `wails-version/will-msg/` to the repository root, then delete both
      versioned working directories. Do not develop in both trees.

---

## 1. Prerequisites & scaffold

- [x] Install the pinned CLI:
      `go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`; `wails version` reports
      `v2.15.0`.
- [ ] `wails doctor` — confirm Xcode CLT on macOS.
- [x] bun 1.4.0, Go 1.27.1, and `mise` are present; `mise.toml` pins the executable toolchain.
- [x] Scaffold `wails-version/will-msg/` with the official `vue-ts` template. Preserve the existing
      `data/`, `data-new/`, `truck.png`, and `mise.toml`; keep scaffold platform files under
      `build/`.
- [ ] Replace npm with bun in `wails.json`:
      `"frontend:install": "bun install"`, `"frontend:build": "bun run build"`,
      `"frontend:dev:watcher": "bun run dev"`, `"frontend:dev:serverUrl": "auto"`.
      Delete `frontend/package-lock.json`; commit `frontend/bun.lock`.
- [ ] Set `wails.json.info`: `productName: "will-msg"`, `companyName: "Dustin Michels"`,
      `copyright: "Copyright © 2026 Dustin Michels"`, and `productVersion: "1.0.0"`. Wails has no
      bundle-ID/build-number fields there: set `CFBundleIdentifier =
      io.github.dustinmichels.willmsg` and `CFBundleVersion = 35` in both
      `build/darwin/Info.plist` and `Info.dev.plist`; keep `CFBundleShortVersionString` driven by
      `productVersion`. A version bump is a separate release decision.
- [x] Keep the scaffolded module directive as `module will-msg`; do **not** run `go mod init`.
      The name must stay stable so copied `will-msg/internal/...` imports compile.
- [ ] Verify the empty shell: `wails dev` opens a window and `wails build` produces
      `build/bin/will-msg.app`.

---

## 2. Port the Go core (behavior frozen except bridge serialization)

- [ ] Copy verbatim: `internal/config/`, `internal/engine/`, `internal/parser/`, `internal/scanner/`,
      `internal/stats/` (+ their `_test.go` files).
- [ ] Copy `testdata/` (6 `.msg` fixtures + `msg_parsed.csv`).
- [ ] Copy CLI commands `cmd/will-msg/` (flags `-input`, `-output`; exit 2 on missing flag) and
      `cmd/msgcat/` (flag `-headers`, 1 positional arg, exit 2 on misuse). **Do not** copy `cmd/will-msg-gui/`.
- [ ] `go.mod`: require `github.com/wailsapp/wails/v2` + `github.com/willthrom/outlook-msg-parser`.
      Must **not** contain `fyne.io/fyne/v2`, `fyne.io/systray`, `github.com/ncruces/zenity`,
      `github.com/go-gl/*`, `github.com/fyne-io/*`.
- [ ] Keep `go 1.25.0` in `go.mod` (the Wails v2.15 minimum) and Go `1.27.1` in `mise.toml`
      (the reproducible build toolchain). These fields have different purposes and need not match.
- [ ] `go mod tidy && go build ./... && go test ./internal/...` — all pre-existing engine/parser/
      scanner/stats/config tests green before service work begins.
- [ ] **Add JSON tags** so bindings emit stable snake_case instead of Go field names.
      `engine.Record` has **no** struct tags today — TS bindings would be `SourceFile`, `Subject`, …
      Tag them to match `CSVHeaders` (`json:"source_file"` …, `RowInMessage → row_in_message`).
      Same for `scanner.MessageSource` (`path`, `in_zip`, `zip_path`, `display_name`).
      `config.*` already has tags — leave them exactly as-is (`rules.json` compatibility).
- [ ] Add serialization-contract tests: marshalled `Record` keys equal `engine.CSVHeaders` as a
      set, and `MessageSource` keys are exactly `path,in_zip,zip_path,display_name`.

---

## 3. Backend service layer (bound Go API)

Create one `internal/appservice.Service` and bind that pointer directly. It owns the Wails
`context.Context` received by `OnStartup`, an initialized `atomic.Pointer[engine.RuleEngine]`, and,
under one `sync.Mutex`, `lastRecords []engine.Record` plus `parseGen uint64`. Keep parsing,
validation, and CSV helpers free of Wails `runtime.*`; only dialog/reveal adapter methods need the
runtime context.

Every boundary DTO (`ScanResult`, `ParseResult`, `SkippedSource`, `SavedFile`,
`ImportRulesResult`, `ValidationError`, `SandboxResult`) gets explicit JSON tags. Return empty
arrays as `[]`, not `null`, and cover representative encoded payloads plus generated TS signatures.

**Why service-owned records, not a frontend argument.** Fyne kept `csvData` in the window closure
and both export buttons read exactly the last parse (gui.go:188, 473, 538). Service ownership
preserves that invariant and avoids sending the records back to Go for export. `Parse` is the only
writer and never mutates a committed backing array, so exports take only the immutable slice header
under the lock, release the lock, then perform I/O. An empty snapshot is an error. Never hold the
state lock during parsing, dialogs, or disk I/O.

**One completion channel, two stale-result guards.** Wails already dispatches each bound method on a
Go goroutine and generates a Promise-returning frontend wrapper. `Parse` therefore does its work
synchronously inside the bound method—do not launch another goroutine and do not duplicate Promise
completion with `parse:done`/`parse:error` events.

1. `Parse` locks, increments `parseGen`, clears `lastRecords`, copies the generation and engine
   pointer, then unlocks.
2. `ClearParse()` and every newer `Parse` increment `parseGen` and clear `lastRecords`.
3. A completed parse commits only when its generation is still current; its `ParseResult` includes
   `superseded: true` otherwise.
4. The Pinia action also captures a frontend request epoch and ignores resolution/rejection after a
   reset or newer request. This prevents stale UI state even when an old Promise settles last.

Inject the source loader in service tests so the two concurrency cases are deterministic rather
than timing-dependent.

### Parsing

- [ ] `SelectFiles() ([]string, error)` → `runtime.OpenMultipleFilesDialog` with separate `*.msg`
      and `*.zip` patterns. Cancel returns an empty slice and no error.
- [ ] `SelectFolder() (string, error)` → `runtime.OpenDirectoryDialog`. Cancel returns `""` and no
      error. This deliberately fixes Fyne's macOS picker, which advertised folders but could not
      select one.
- [ ] `ScanSources(paths []string) (ScanResult, error)` → call `scanner.FindSources` for every
      selected/dropped path, preserve input and scanner order, and de-duplicate identical
      `MessageSource` identities. Return
      `{sources, count, source_paths}`; one bad root is an error rather than a partial scan.
- [ ] `Parse(sources []scanner.MessageSource) (ParseResult, error)` → use the engine snapshot taken
      at start; per source call `scanner.LoadSource` + `eng.ParseRecords`. Return
      `{records, skipped: []SkippedSource{display_name,error}, superseded}` and commit records only
      through the generation check above. A current result with zero records leaves
      `lastRecords` empty; the frontend shows "No structured records found", keeps export disabled,
      and does not treat an all-skipped run as success.
- [ ] `ClearParse()` — bump `parseGen` and nil `lastRecords` for "Load New Source". Later export
      requests fail; an export already initiated may finish from its immutable snapshot.
- [ ] Do not defensively copy the bridge input or create a worker goroutine: JSON decoding already
      gives each Wails call an owned slice, and Wails dispatches bound calls concurrently.
      Progress/cancellation events remain Phase 13.
- [ ] Surface `skipped` in the result. This deliberately improves on Fyne, which only logged
      per-file failures (gui.go:77).

### Export

- [ ] `SaveToDownloads() (SavedFile, error)` — take an immutable `lastRecords` snapshot under the
      lock, then write headers + `ToRow` to `getDownloadsDir()` as
      `msg_parsed_<2006-01-02_150405>.csv`; return `{filename, dir, path, cancelled:false}`.
- [ ] `SaveAs() (SavedFile, error)` → snapshot records before opening `runtime.SaveFileDialog`
      (`DefaultFilename = msg_parsed_<timestamp>.csv`, filter `*.csv`). Return
      `{cancelled:true}` for an empty dialog path; cancellation is not an error and writes nothing.
- [ ] `RevealFile(path string) error` — isolate platform commands behind small per-OS helpers:
      `open -R` (darwin), `explorer.exe /select,` (windows), FileManager1 → `xdg-open` (linux).
      Use `exec.CommandContext` argument vectors, never a shell. Build fallback file URLs with
      `url.URL{Scheme:"file", Path:dir}.String()` so spaces and `#` are escaped correctly.
- [ ] CSV writing stays server-side (`encoding/csv`), rows come only from `engine.CSVHeaders` +
      `Record.ToRow`, and writer flush and file-close errors are both returned. Keep the copied CLI
      writer unchanged and prove service output equivalent in Phase 9.

### Rules (replaces `internal/gui/gui_rules.go`, 915 lines)

- [ ] `GetRules() config.RuleConfig` — return `config.LoadConfig()`; it already returns a fresh
      decoded/default value, so another Go-side clone is unnecessary.
- [ ] `GetDefaultRules() config.RuleConfig` — return a fresh `config.DefaultRuleConfig()`.
- [ ] `ValidateRules(cfg config.RuleConfig) []ValidationError` — authoritative validation for all
      config invariants and every regex; return stable `{path, message}` entries. `SaveRules` and
      `TestRule` call the same helper so frontend feedback cannot drift from save behavior.
- [ ] `SaveRules(cfg config.RuleConfig) error` — validate/build with
      `engine.NewRuleEngineValidated`, persist via `config.SaveConfig`, then atomically swap the
      engine. The Promise is the completion signal; no `rules:applied` event.
- [ ] Keep `LoadConfigFromPath`/`SaveConfigToPath` for dependency injection and existing tests, but
      do not bind arbitrary-path methods: `NewRuleManagerViewWithPath` is test plumbing, not a
      user-visible Fyne feature.
- [ ] `ImportRules() (ImportRulesResult, error)` → `OpenFileDialog` (`*.json`) +
      `config.ParseRuleConfig`; return `{cancelled:true}` on cancel or `{config,cancelled:false}`.
      Import changes only the working copy and never saves implicitly.
- [ ] `ExportRules(cfg config.RuleConfig) (SavedFile, error)` → validate, `SaveFileDialog`, and
      `cfg.ToJSON()`; use the same explicit cancellation contract as `SaveAs`.
- [ ] `TestRule(input string, cfg config.RuleConfig) (SandboxResult, error)` — validate/build a
      temporary engine, then return
      `{address,status,issue_time,label,matched_rule_index,matched_rule_id,match_kind,metric}` from
      `Classify`, `MatchRule(issue)`, and `MetricForLabel`. Invalid in-progress regex input returns a
      validation error instead of silently skipping the rule.
- [ ] Rule mutations (add/edit/delete/move/toggle/reset) stay client-side. Preserve array order and
      stable IDs exactly; first enabled match wins. Never sort the saved rules for display.
- [ ] Preserve the existing label-key contract: non-empty and unique, with
      `metric ∈ {none,trash,recycling,both}`. Do **not** impose snake_case during migration because
      existing valid configs may contain other keys. Inline creation may update an existing key,
      matching the Fyne form.

### Wiring

- [ ] `main.go`: keep `//go:embed all:frontend/dist` and `assetserver.Options{Assets: assets}`; use
      `Title: "Outlook MSG to CSV Parser"`, `Width: 950`, `Height: 700`, explicit minimums,
      `BackgroundColour: options.NewRGB(248,250,252)`, `Bind: []any{svc}`, and `OnStartup`.
- [ ] Enable `options.DragAndDrop{EnableFileDrop:true,...}`. Register
      `OnFileDrop(callback, true)` in Vue from `frontend/wailsjs/runtime`, call `ScanSources(paths)`,
      and unregister with `OnFileDropOff` on unmount. Do not also register Go
      `runtime.OnFileDrop`; two listeners create duplicate scans.

---

## 4. Frontend scaffold

- [ ] `cd frontend && bun install`
- [ ] `bun add pinia`; `bun add -d tailwindcss @tailwindcss/vite`
- [ ] `vite.config.ts`: `plugins: [vue(), tailwindcss()]`, `@` alias → `src`.
- [ ] `src/style.css`: `@import "tailwindcss";` + `@theme` tokens (Phase 5).
- [ ] `tsconfig`: `strict: true`; path alias for generated `wailsjs` bindings.
- [ ] Keep scripts `dev` and `build` (`vue-tsc --noEmit && vite build`); add `test: "bun test"`.
- [ ] Delete scaffold demo files (Greet component, logo assets, boilerplate CSS).
- [ ] Commit generated `frontend/wailsjs/` bindings as the checked-in Go/TS API contract. After
      service changes, run `wails generate module` and require a clean regeneration before cutover;
      never hand-edit generated files.

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
      `theme.VariantDark` in 4 places). Honor `prefers-reduced-motion` for all nonessential motion.
- [ ] Copy `internal/gui/truck.png` → `frontend/src/assets/truck.png`; it becomes a frontend asset,
      so delete the Go `//go:embed truck.png` + `truckResource`.
- [ ] The source truck icon is 512×512. Generate the required 1024×1024 `build/appicon.png`
      deliberately, inspect the packaged icon at native sizes, and keep a higher-resolution redraw
      as Phase 13 polish rather than blocking migration.
- [ ] Type scale: Fyne header sizes were 18 px (title), 13 px (subtitle), 28 px (welcome headline);
      use tabular numerals in the CSV table.

---

## 6. Frontend architecture

- [ ] `src/stores/parse.ts` — `sourcePaths`, `sources[]`, `records[]`, `skipped[]`,
      `requestEpoch`, and `status: idle|scanning|parsing|ready|error`. Every scan/parse/reset action
      owns its state transition; stale Promise results must fail the epoch check before mutation.
- [ ] `src/stores/rules.ts` — `savedConfig`, deep-cloned `workingConfig`, `selectedIndex`,
      `isDirty`, `validation[]`; mutations for add/edit/delete/move/toggle/reset/import. Never alias
      the Wails-generated model arrays between saved and working state.
- [ ] `src/composables/useFileDrop.ts` — register the Wails runtime `OnFileDrop` listener once,
      route all paths through the parse store's `ScanSources` action, and clean it up on unmount.
- [ ] `src/lib/toast.ts` — replaces `dialog.ShowError` / `dialog.ShowInformation`
      (≈12 call sites across gui.go / gui_rules.go).
- [ ] Screen switching: `bodyContainer` + `showWelcome()`/`showWorkspace()` (gui.go:740-748)
      becomes a `view` computed; the rules manager is the third view. Route all exits from a dirty
      rules view through one discard-confirmation guard.

---

## 7. Vue components (feature parity, screen by screen)

### Shell

- [ ] `AppHeader.vue` — 50 px truck-green bar: truck logo, "Outlook MSG Parser",
      subtitle "Feed me your msg files, Will", "Rules & Labels" button (gui.go:584-616).
- [ ] `StatusBar.vue` — 30 px slate-800 strip with a CSS Pac-Man/dot animation derived from
      gui.go:195-298. Run it only while `status === 'parsing'`, and render a static strip under
      `prefers-reduced-motion: reduce`; no canvas ticker or component-owned timer.
- [ ] `ToastHost.vue`, `ConfirmDialog.vue`, `Modal.vue` primitives.

### Welcome screen (gui.go:618-688)

- [ ] `WelcomeScreen.vue` — rounded (16 px) drop zone, truck-green 2 px stroke, light-green tint,
      upload icon, headline "Feed me your msg files, Will", copy
      "Drop or select. Accepts .msg files, folders containing .msg files, or .zip archives."
- [ ] Mark the drop zone with `style="--wails-drop-target: drop"` and style the
      `wails-drop-target-active` class installed by `OnFileDrop(callback, true)`.
- [ ] Buttons: **Choose files…** (`SelectFiles`) and **Choose folder…** (`SelectFolder`).
- [ ] Empty-scan feedback — Fyne showed a modal "No .msg files were found in the selected source."
      (gui.go:408); a toast or inline warning is equivalent.

### Workspace (gui.go:690-738)

- [ ] `SourcePanel.vue` — "Load New Source" calls the store reset and `ClearParse`, then clears
      sources/records/preview; render all selected root paths, "Found N .msg files" (italic), and a
      scrollable detected-file list with document icon + ellipsis truncation.
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
      Validation: non-empty pattern, compilable regex, non-empty unique label key, and valid metric.
- [ ] `RuleSandbox.vue` (gui_rules.go:347-464) — debounced live input (default sample
      `45 FOREST ST TRASH AND RCY NOT OUT 0830AM`), 4 sample buttons
      (Both Not Out / Contaminated / Blocked / Heuristic Suffix) + Clear, and 6 result cards:
      Address, Issue/Status, Issue Time, Assigned Label, Matched Rule (rule # / heuristic / fallback),
      Metric Impact.
- [ ] Action bar (gui_rules.go:329): **Save & Apply Rules** (validate → save → engine swap → toast)
      and **Cancel** (discard); warn when leaving with `isDirty`.

---

## 8. Native integration checks

- [ ] File drop of a folder, a `.msg`, a `.zip`, and multiple files each call `ScanSources` once;
      overlapping roots do not duplicate detected messages.
- [ ] Cancelled dialogs return their explicit `cancelled` result and produce no error toast or
      state mutation.
- [ ] `RevealFile` verified on macOS. Exercise the Windows branch on a Windows runner and the Linux
      branch when a Linux artifact is added; a host-only Go test does not compile other build tags.
- [ ] Paths with spaces, parentheses, `#`, and non-ASCII characters work.
- [ ] Windows build uses `-webview2 embed`; smoke-run it on Windows with WebView2 absent or document
      the exact fallback/install behavior observed.
- [ ] The built `.app` works **outside** the repo directory; confirm `frontend/dist` is embedded and
      no runtime asset lookup depends on the working directory.

---

## 9. Verification

- [ ] `go test ./...` and `go test -race ./internal/appservice` green.
- [ ] **Golden CSV equivalence — cutover gate.** Run both old and new `cmd/will-msg` over
      `testdata/` and `data/` with the same rules config and compare CSV bytes. Separately exercise
      the service parse/export helper against `testdata/msg_parsed.csv`; this proves the GUI path,
      not only the copied CLI.
- [ ] Go service tests cover: boundary DTO encoding, sandbox result for every built-in sample,
      invalid-regex rejection, import/export round-trip and cancellation,
      save→persist→engine-swap ordering, no-record/all-skipped parsing, empty export, and CSV
      writer/close failures. Inject dialog and filesystem edges; do not require a live Wails runtime.
- [ ] Deterministic parse-lifecycle tests cover: successful commit; `ClearParse()` during parse;
      second parse superseding the first; `superseded:true`; and export remaining empty after a
      superseded completion.
- [ ] `bun test` covers observable store behavior: deep-clone isolation, reorder precedence,
      toggle, boundary moves, reset/import dirty state, and stale parse Promise suppression. Use
      Bun's test runner; do not add Vitest solely for these store tests.
- [ ] Do not copy `internal/gui/*_test.go`. Re-home only behavior that survives the boundary:
      `parseMsgSources` into service tests and rule mutations into store tests; drop asset/resource
      wiring assertions.
- [ ] `rules.json` from the new app round-trips an existing user file without schema or key changes
      (same path and `version:1`), including a valid non-snake-case custom label key.
- [ ] Run `wails generate module`, inspect the generated TS field names/signatures, then
      `cd frontend && bun run build`.
- [ ] Manual smoke (`wails dev`): drop multiple roots → scan once → run → preview → Download CSV →
      Show in Folder; reset during an active parse and verify no stale preview/export; then import,
      edit, sandbox, Save & Apply, re-run, and observe changed labels.

---

## 10. Build & packaging (replaces fyne-cross)

Parity target = the same artifact set the Fyne build produced. Build on the target OS; do not make a
macOS cross-compile the only evidence for a Windows release.

- [ ] On macOS, build both architectures in one invocation so Wails assigns distinct bundle names:
      `wails build -platform darwin/arm64,darwin/amd64 -clean`. Package/rename the resulting apps as
      `bin/will-msg-macos-{arm64,amd64}.app` and zip each with `ditto -c -k --sequesterRsrc
--keepParent`.
- [ ] On a native Windows CI runner:
      `wails build -platform windows/amd64 -clean -webview2 embed`; package
      `bin/will-msg-windows-amd64.exe` and its zip there, then launch the exe for a smoke check.
- [ ] Rewrite `mise.toml` tasks: `dev`, `build`, `build-darwin`, `build-windows`, `test`, and the
      existing `parse` helper. Make each packaging task produce the filenames above and fail if an
      expected artifact is absent. Delete the Fyne-specific `scripts/build-darwin.sh`.
- [ ] Add a native-OS CI matrix (macOS for Darwin, Windows for Windows); upload the three parity
      artifacts. Docker/fyne-cross is no longer required once the native jobs are green.
- [ ] Verify generated metadata, not just source templates: both macOS bundles have
      `CFBundleIdentifier=io.github.dustinmichels.willmsg`, short version `1.0.0`, build `35`, and
      the expected executable architecture; inspect Windows version resources too.
- [x] `.gitignore` covers `node_modules/`, `frontend/dist/`, `build/bin/`, local data/output/sample,
      and packaged `bin/`.

---

## 11. Docs

- [ ] Rewrite `README.md`: new stack, `wails dev` / `wails build`, bun commands, prerequisites,
      generated-binding workflow, native-OS packaging, and embedded WebView2 behavior. Delete the
      Fyne/CGo/OpenGL/fyne-cross/Docker section.
- [ ] Keep the `msgcat` and CLI (`-input`/`-output`) sections — those binaries survive unchanged.
- [ ] Rewrite `claude.md` for the new layout (Go service + Vue frontend, bun, bindings ownership).
- [ ] Prune `improvements.md`: remove completed or obsolete Fyne items, but retain still-open work;
      do not describe this migration itself as future work after cutover.

---

## 12. Cutover

- [ ] Feature-parity walkthrough against the matrix below; every row checked.
- [ ] Verify nothing Fyne-shaped was copied in: no `internal/gui/`, `cmd/will-msg-gui/`,
      `FyneApp.toml`, `fyne-cross/`, `Icon.png`, or root `truck.png`.
- [ ] Tag the final Fyne state `legacy-fyne`, promote `wails-version/will-msg/` to the repository
      root, and remove the two versioned directories so one source of truth remains.
- [ ] From the promoted clean checkout: `go mod tidy` (no diff) →
      `bun install --frozen-lockfile` → regenerate bindings (no diff) → Go/Bun tests → native
      macOS and Windows builds/smokes.

---

## Feature parity matrix (Fyne → Wails)

| Fyne feature (source)                                   | Wails/Vue replacement                   | Done |
| ------------------------------------------------------- | --------------------------------------- | ---- |
| Window "Outlook MSG to CSV Parser" 950×700 (gui.go:178) | `options.App{Title, Width, Height}`     | ☐    |
| Custom truck-green theme, light/dark (gui.go:92-121)    | Tailwind `@theme` tokens                | ☐    |
| Drop files onto window (gui.go:412)                     | `EnableFileDrop` + JS `OnFileDrop`      | ☐    |
| zenity "Select file or folder" (gui.go:631)             | `SelectFiles` + `SelectFolder`          | ☐    |
| `scanner.FindSources` + count labels (gui.go:386-410)   | `ScanSources` + `SourcePanel.vue`       | ☐    |
| Detected-files list (gui.go:306)                        | list in `SourcePanel.vue`               | ☐    |
| Run Parser + background goroutine (gui.go:421)          | concurrent bound `Parse` Promise        | ☐    |
| Per-file failures logged only (gui.go:77)               | `ParseResult.skipped` + notice          | ☐    |
| CSV preview table, 11 cols (gui.go:329-367)             | `CsvPreview.vue`                        | ☐    |
| Download CSV → ~/Downloads (gui.go:460)                 | `SaveToDownloads`                       | ☐    |
| Save As… via zenity (gui.go:509)                        | `SaveAs` → `runtime.SaveFileDialog`     | ☐    |
| Saved-file confirmation dialog (gui.go:484-506)         | confirmation card                       | ☐    |
| "Show in Folder" (gui.go:135-170)                       | `RevealFile`                            | ☐    |
| Pac-Man status bar animation (gui.go:195-298)           | reduced-motion-safe CSS `StatusBar.vue` | ☐    |
| "Load New Source" reset (gui.go:690)                    | store reset + `ClearParse`              | ☐    |
| Rules & Labels window (gui_rules.go:51)                 | `RulesView.vue`                         | ☐    |
| Rules table + toolbar (gui_rules.go:147-328)            | `RulesTable.vue` + toolbar              | ☐    |
| Heuristics toggle + default label (gui_rules.go:214)    | settings bar                            | ☐    |
| Add/Edit rule dialog (gui_rules.go:560)                 | `RuleFormModal.vue`                     | ☐    |
| Inline new-label creation (gui_rules.go:560)            | label sub-form + compatible validation  | ☐    |
| Live sandbox (gui_rules.go:347-464)                     | `RuleSandbox.vue` + `TestRule`          | ☐    |
| Import/Export rules JSON (gui_rules.go:777-881)         | `ImportRules` / `ExportRules`           | ☐    |
| Reset defaults (gui_rules.go:487)                       | `GetDefaultRules` + confirm             | ☐    |
| Save & Apply rules (gui_rules.go:737-775)               | `SaveRules` + engine swap               | ☐    |
| `dialog.ShowError` / `ShowInformation`                  | toast host                              | ☐    |
| `cmd/will-msg`, `cmd/msgcat`                            | copied unchanged                        | ☐    |

---

## Risks / watch items

- **Binding field names.** `engine.Record` and `scanner.MessageSource` need explicit JSON tags;
  regenerate bindings and inspect the generated TS instead of trusting Go compile success.
- **Bound calls are concurrent.** `Parse`, `ClearParse`, export, and `SaveRules` can overlap. Keep
  locks narrow, snapshot the engine/records, and enforce both backend generation and frontend epoch
  checks. Never hold a mutex across a Wails dialog or filesystem operation.
- **Rule precedence is array order.** Display filtering/sorting must not mutate `workingConfig.Rules`
  or reorder the payload passed to `SaveRules`.
- **Config compatibility.** Do not change `config.*` JSON tags, config path constants, version, or
  accepted label-key syntax during migration.
- **Preview scale.** `data-new/` holds ~200 `.msg` files; measure a production build before adding
  table virtualization.
- **Platform boundaries.** Reveal-file helpers, WebView2 bootstrap behavior, generated bundle
  metadata, and zip-internal sources require target-OS or fixture-backed checks.
- **Wails v3 beta** changes bindings/build significantly. Keep core service logic free of
  `runtime.*` so a later v3 migration remains mechanical.

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
- [ ] **Stricter label-key normalization** (for example snake_case), with an explicit config
      migration and UI explanation; never reject existing keys silently.
- [ ] **Drag-to-reorder rules** replacing Move Up/Down (keep a keyboard-accessible fallback).
- [ ] **Sandbox extras**: highlight the matching rule row, show the regex capture span.
- [ ] **macOS universal binary** (`-platform darwin/universal`) instead of two per-arch bundles.
- [ ] **Windows NSIS installer** (`-nsis`) in addition to the zip.
- [ ] **Code signing / notarization** for distribution outside the org.
- [ ] **`vue-router`** if the three views outgrow a `view` computed.
- [ ] **Frontend E2E tooling** (Playwright) or Vitest if component behavior outgrows Bun tests.
- [ ] **`SingleInstanceLock`** app option.
