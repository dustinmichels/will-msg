# will-msg Developer & Architecture Guide

Outlook `.msg` parser for municipal trash and recycling service exception tag emails, producing structured CSV output.

---

## 1. Project Overview

`will-msg` extracts service exception records (address, issue, classification label, issue time, dispatcher, reported date) from Microsoft Outlook `.msg` emails, folders, or `.zip` archives.

The application is structured as:
1. **Desktop Application**: Go backend bound to a Vue 3 + Tailwind CSS v4 + Pinia frontend via **Wails v2**.
2. **Headless CLI Tools**:
   - `cmd/will-msg`: Batch processor taking `-input` and `-output` flags.
   - `cmd/msgcat`: Standalone utility to dump the plain-text body and headers of any `.msg` file.

---

## 2. Architecture & Codebase Layout

```
will-msg/
├── main.go                     # Wails application entry point & lifecycle hooks
├── wails.json                  # Wails project configuration
├── mise.toml                   # Toolchain pinning (Go 1.27.1, Bun 1.4.0) & task definitions
├── cmd/
│   ├── will-msg/               # Headless batch CSV parser CLI (-input, -output)
│   └── msgcat/                 # Single .msg plain-text dumper CLI (-headers)
├── internal/
│   ├── appservice/             # Bound Wails service layer (Service, DTOs, dialogs, exports)
│   ├── config/                 # Rule configuration schema, persistence (rules.json), validation
│   ├── engine/                 # Classification engine, regex matching, heuristics, CSV rows
│   ├── parser/                 # OLE .msg parsing, line normalization, wrapped continuation
│   ├── scanner/                # Recursive file, directory, and .zip archive scanner
│   └── stats/                  # Daily and summary statistics computation
├── frontend/
│   ├── src/
│   │   ├── stores/             # Pinia stores (parse.ts, rules.ts, navigation.ts)
│   │   ├── components/         # Vue 3 SFCs (WelcomeScreen, Workspace, RulesView, RuleSandbox)
│   │   ├── composables/        # useFileDrop.ts (Wails runtime drag-and-drop)
│   │   ├── lib/                # Toast and confirmation dialog primitives
│   │   ├── style.css           # Tailwind v4 theme & custom color tokens
│   │   └── App.vue             # Main shell & view router
│   ├── wailsjs/                # Generated Go/TS bindings & models (DO NOT EDIT MANUALLY)
│   ├── package.json            # Frontend dependencies (Vue 3, Pinia, Tailwind CSS v4, Bun)
│   └── vite.config.ts          # Vite build configuration
├── build/                      # App icon assets and macOS/Windows packaging resources
├── bin/                        # Built release distributions (.app, .exe, .zip)
└── testdata/                   # Sample .msg files and golden CSV fixtures
```

---

## 3. Key Components & Contracts

### Backend (`internal/`)

- **`internal/appservice`**: Single bound Go struct (`*appservice.Service`).
  - Manages Wails runtime `context.Context` from `OnStartup`.
  - Concurrency guards: generation counter `parseGen` prevents stale asynchronous parse results from overwriting newer runs.
  - Mirrored rules-dirty state (`atomic.Bool`) coordinates with `OnBeforeClose` native window close guards.
  - Owns parse records snapshot for direct export (`SaveToDownloads`, `SaveAs`) without round-tripping data across the bridge.
  - Provides dialog adapters, rule validation (`ValidateRules`), sandbox testing (`TestRule`), and platform file reveal (`RevealFile`).
- **`internal/engine`**: Core classification logic (`RuleEngine`). Evaluates regex/substring rules, heuristic rules, address normalization, and emits `Record` structs (`CSVHeaders` contract).
- **`internal/config`**: Manages `rules.json` schema (stored in `os.UserConfigDir()/will-msg/rules.json`, version 1).
- **`internal/scanner`**: Finds `.msg` files across directories and `.zip` archives into `MessageSource` models.
- **`internal/parser`**: Extracts raw text and headers from `.msg` files using `github.com/willthrom/outlook-msg-parser`.

### Frontend (`frontend/src/`)

- **`stores/parse.ts`**: Manages source discovery, parse lifecycle, skipped source reporting, and superseded request epochs.
- **`stores/rules.ts`**: Deep-clones working configuration for client-side mutations (add, edit, delete, reorder, toggle, import/export), performs live validation, and tracks dirty state.
- **`wailsjs/`**: Generated TypeScript client for `appservice.Service`.

---

## 4. Bindings Workflow

Wails generates TypeScript definitions for bound Go methods and DTO structs.

Whenever Go method signatures or DTO structs change in `internal/appservice`:

```sh
wails generate module
```

Generated files under `frontend/wailsjs/` must be checked into git. **Never hand-edit generated files in `frontend/wailsjs/`.**

---

## 5. Development & Testing Commands

All tasks are defined in `mise.toml`:

```sh
# Live development (Vite hot reload + Wails Go bridge)
mise run dev
# or: wails dev

# Run all tests (Go unit/race tests + Bun frontend tests)
mise run test

# Run Go tests only
go test -race ./...

# Run frontend tests only
(cd frontend && bun test)

# Typecheck and build frontend
(cd frontend && bun run build)

# Inspect a message file in terminal
mise run parse <path_to_msg_file>
# or: go run ./cmd/msgcat <path_to_msg_file>
```

---

## 6. Native Packaging

Builds run natively on target platforms without Docker or `fyne-cross`:

- **macOS**:
  ```sh
  mise run build-darwin
  # Generates bin/will-msg-macos-arm64.app and bin/will-msg-macos-amd64.app (+ zips)
  ```
- **Windows**:
  ```sh
  mise run build-windows
  # Generates bin/will-msg-windows-amd64.exe with embedded WebView2 loader (+ zip)
  ```
- **Both**:
  ```sh
  mise run build
  ```
