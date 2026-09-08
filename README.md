# will-msg

Extract, classify, and export municipal trash and recycling service exception tag emails into structured CSV data from Microsoft Outlook `.msg` files, directories, or `.zip` archives.

Available as both a **modern cross-platform desktop GUI application** (Wails v2 + Vue 3 + Tailwind CSS v4 + Pinia) and **headless CLI utilities** for batch processing and command-line automation.

---

## Features

- **Multi-Source Ingestion**: Parse individual `.msg` files, recursive directories of `.msg` files, or `.zip` archives containing messages. Ignores hidden files and macOS metadata (`__MACOSX`) automatically.
- **Intelligent Message Extraction**: Extracts email headers (From, Subject, Date), body text, dispatchers, reported timestamps, street addresses, issue descriptions, and specific issue times.
- **Address & List Normalization**: Handles multi-address lists (e.g., `8, 12, 16 Powder House Rd` expanded into distinct records), street suffix parsing, and wrapped line continuations.
- **Customizable Classification Engine**: Assigns canonical labels using ordered substring and regex rules (case-insensitive) with heuristic fallbacks.
- **Desktop GUI**:
  - **Drag & Drop**: Drop files, directories, or `.zip` archives directly into the app.
  - **Interactive CSV Preview**: Live table view with column sorting and record counts.
  - **Summary Metrics**: Real-time aggregation of Trash Not Out, Recycling Not Out, and Daily Statistics.
  - **One-Click Export**: Save to `Downloads` or choose a custom location with native "Show in Folder" file reveal.
  - **Rules & Labels Manager**: Add, edit, reorder, delete, and toggle rules; customize display names and metric mappings.
  - **Live Rule Sandbox**: Test regex and substring patterns against sample message lines in real time before saving.
  - **Persistent Configuration**: Saved to OS user config directory (`rules.json`) with automated schema migration and corruption recovery.
  - **Dirty State Guard**: Native dialog prompts to prevent accidental window closure with unsaved rule modifications.
- **Headless CLI Tools**: Fast, zero-GUI utilities for batch processing (`will-msg`) and single-message inspection (`msgcat`).

---

## CSV Output Schema

`will-msg` produces CSV files with the following 11 standard columns:

| Column | Description | Example |
| ------ | ----------- | ------- |
| `source_file` | File name or path of the source `.msg` file | `MEDFORD TAGS 02_04_26.msg` |
| `subject` | Email subject line | `MEDFORD TAGS 02.04.26` |
| `message_date` | Timestamp when the email was sent (RFC 3339 / ISO 8601) | `2026-02-04T20:45:14Z` |
| `reported_at` | Timestamp extracted from the email body (RFC 3339 / ISO 8601) | `2026-02-04T07:44:56-05:00` |
| `dispatcher` | Dispatcher name or ID extracted from body or headers | `SSAWALLI` |
| `row_in_message` | 1-based index of the entry block within the message | `4` |
| `raw_entry` | Full raw line extracted from the message | `8 POWDER HOUSE RD EXT MSW NOT OUT` |
| `location` | Extracted and normalized street address / location | `8 POWDER HOUSE RD` |
| `issue` | Extracted status or issue description text | `EXT MSW NOT OUT` |
| `label` | Canonical classification label key | `msw_not_out` |
| `issue_time` | Specific time noted in the entry line (if present) | `7:45AM` |

---

## Classification Labels & Metrics

The rule engine ships with 8 default classification labels mapped to statistical metric categories:

| Label Key | Display Name | Metric Category | Description |
| --------- | ------------ | --------------- | ----------- |
| `msw_and_recyc_not_out` | Trash & Recycling Not Out | Both (Trash + Recycling) | Both trash and recycling containers were not put out |
| `msw_not_out` | Trash Not Out | Trash | Municipal solid waste (trash) was not put out |
| `recyc_not_out` | Recycling Not Out | Recycling | Recycling container was not put out |
| `special_item_not_out` | Special Item Not Out | None | Scheduled bulk / special item was not put out (e.g. sofa, fridge) |
| `recyc_contaminated` | Recycling Contaminated | None | Recycling bin contained non-recyclable contamination |
| `blocked` | Blocked / Inaccessible | None | Container access blocked by vehicle, snow, or obstruction |
| `overflowing` | Overflowing / Overloaded | None | Container overloaded or lid unable to close |
| `other` | Other | None | Unclassified or custom issue |

---

## Desktop Application

### Prerequisites

- **Go**: `1.25.0+` (Go `1.27.1` configured in `mise.toml`)
- **Bun**: `1.4.0+` (JavaScript runtime & package manager)
- **Wails CLI**: `v2.15.0`
  ```sh
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
  ```
- **macOS**: Xcode Command Line Tools (`xcode-select --install`)
- **Windows**: Microsoft Edge WebView2 (installed by default on Windows 10/11; embedded loader included in packaged release binaries)

### Live Development

Start the development server with hot-reloading frontend and live Go backend bridge:

```sh
# Using mise task runner
mise run dev

# Or directly with Wails CLI
wails dev
```

### Frontend Development & Tests

```sh
cd frontend

# Install dependencies
bun install

# Run frontend tests (stores, components, utilities)
bun test

# Type-check and build production bundle
bun run build
```

### Generated Bindings Workflow

Go methods and DTO structs in `internal/appservice` are bound to TypeScript via Wails. Whenever backend service signatures or models change, regenerate the bindings:

```sh
wails generate module
```

> **Note**: The files under `frontend/wailsjs/` are tracked in git as the Go/TypeScript interface contract. **Do not edit files in `frontend/wailsjs/` manually.**

### Rule Configuration Storage

Custom classification rules and label definitions are persisted in the OS user configuration directory:

- **macOS**: `~/Library/Application Support/will-msg/rules.json`
- **Linux**: `~/.config/will-msg/rules.json` (or `$XDG_CONFIG_HOME/will-msg/rules.json`)
- **Windows**: `%APPDATA%\will-msg\rules.json`

If `rules.json` becomes corrupted, the application creates a timestamped backup (`rules.json.corrupted.<timestamp>`) and recovers safely with default settings. Rules can also be imported and exported as JSON files from within the GUI.

---

## Building & Packaging

Native desktop binaries are packaged directly without Docker or third-party containers:

```sh
# Build and package for both macOS and Windows
mise run build

# macOS: builds separate arm64 and amd64 .app bundles + zipped distributions
mise run build-darwin

# Windows: builds 64-bit .exe with embedded WebView2 loader + zip
mise run build-windows
```

### Distribution Artifacts

Packaged distribution artifacts are output to `bin/`:

- `bin/will-msg-macos-arm64.app` & `bin/will-msg-macos-arm64.zip` (macOS Apple Silicon)
- `bin/will-msg-macos-amd64.app` & `bin/will-msg-macos-amd64.zip` (macOS Intel)
- `bin/will-msg-windows-amd64.exe` & `bin/will-msg-windows-amd64.zip` (Windows 64-bit)

### Manual Wails Build Commands

```sh
# macOS (dual-architecture build)
wails build -platform darwin/arm64,darwin/amd64 -clean

# Windows (embedded WebView2 loader automatically checks Evergreen runtime or prompts installer)
wails build -platform windows/amd64 -clean -webview2 embed
```

---

## CLI Tools

### `will-msg` — Headless Batch Parser

Parse messages directly to CSV from the terminal.

#### Installation

```sh
# Install into $GOPATH/bin from within the repository
go install ./cmd/will-msg

# Or compile a standalone binary locally
go build -o will-msg ./cmd/will-msg
```

#### Usage

```sh
go run ./cmd/will-msg -input <path> -output <destination.csv>
```

| Flag | Description |
| ---- | ----------- |
| `-input` | Path to a `.msg` file, directory of `.msg` files, or `.zip` archive |
| `-output` | Path to destination `.csv` file |

#### Examples

```sh
# Parse a directory of messages
go run ./cmd/will-msg -input testdata/ -output output/parsed.csv

# Parse a zipped archive
go run ./cmd/will-msg -input testdata/archive.zip -output output/parsed.csv

# Parse a single .msg file
go run ./cmd/will-msg -input "testdata/Medford Tags 01_02_26.msg" -output output/single.csv
```

---

### `msgcat` — Single `.msg` Inspector

`msgcat` is a lightweight CLI utility to dump the plain-text body and headers of any `.msg` file to stdout.

#### Installation

```sh
# Install into $GOPATH/bin from within the repository
go install ./cmd/msgcat

# Or compile a standalone binary locally
go build -o msgcat ./cmd/msgcat
```

#### Usage

```sh
go run ./cmd/msgcat [-headers=false] <file.msg>
```

| Flag | Default | Description |
| ---- | ------- | ----------- |
| `-headers` | `true` | Print Subject / From / To / Date header block before the body |

#### Examples

```sh
# Full output (headers + body)
go run ./cmd/msgcat "testdata/Medford Tags 01_02_26.msg"

# Body only — pipe into grep, wc, jq, etc.
go run ./cmd/msgcat -headers=false "testdata/Medford Tags 01_02_26.msg" | grep "NOT OUT"

# Process all .msg files in a folder
for f in testdata/*.msg; do echo "=== $f ==="; go run ./cmd/msgcat -headers=false "$f"; done

# Using the mise parse helper
mise run parse "testdata/Medford Tags 01_02_26.msg"
```

---

## Project Structure

```
will-msg/
├── main.go                     # Wails desktop application entry point & lifecycle hooks
├── wails.json                  # Wails application configuration
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
│   │   ├── components/         # Vue 3 SFCs (WelcomeScreen, CsvPreview, RulesView, RuleSandbox)
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

## Testing

Run the comprehensive test suite across backend Go packages and frontend Vue/Pinia stores:

```sh
# Run full project test suite via mise
mise run test

# Run Go backend tests with race detector
go test -race ./...

# Run frontend tests
(cd frontend && bun test)
```
