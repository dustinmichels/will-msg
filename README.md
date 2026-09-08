# will-msg

Parse Microsoft Outlook `.msg` files into structured CSV rows.

`will-msg` extracts service exception reports from municipal trash and recycling tag emails. It accepts individual `.msg` files, directories of `.msg` files, or `.zip` archives, then outputs structured CSV rows classified with customizable rules.

Available as both a **modern desktop GUI application** (Wails v2 + Vue 3 + Tailwind CSS v4) and **headless CLI utilities** for batch processing and terminal workflows.

---

## Classification Labels

- `msw_and_recyc_not_out`
- `msw_not_out`
- `recyc_not_out`
- `special_item_not_out`
- `recyc_contaminated`
- `blocked`
- `overflowing`
- `other`

---

## Desktop Application

The desktop app provides an interactive interface with real-time feedback:

- **Drag & drop** support for `.msg` files, folders, and `.zip` archives.
- **CSV table preview** with column sorting and summary metrics.
- **One-click export** to `Downloads` or custom location with "Show in Folder" reveal.
- **Rules & Labels Manager**: in-app rule builder, reordering, enabling/disabling heuristics, and custom label definition.
- **Live Rule Sandbox**: test regexes and substring rules against live sample messages before saving.
- **Native OS integration**: dark mode support, dirty-rules close guards, and native dialogs.

### Prerequisites

- **Go**: `1.25.0+` (Go `1.27.1` configured in `mise.toml`)
- **Bun**: `1.4.0+` (package manager & task runner for frontend)
- **Wails CLI**: `v2.15.0`
  ```sh
  go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
  ```
- **macOS**: Xcode Command Line Tools (`xcode-select --install`)
- **Windows**: Microsoft Edge WebView2 (installed by default on Windows 10/11; embedded loader included in release binaries)

### Live Development

Start the live development environment (Vite hot module reloading + connected Go backend):

```sh
# Using mise task
mise run dev

# Or directly with Wails CLI
wails dev
```

### Frontend Development & Tests

```sh
cd frontend

# Install dependencies
bun install

# Run frontend unit & store tests
bun test

# Type-check and build production assets
bun run build
```

### Generated Bindings Workflow

Go methods and DTO structs in `internal/appservice` are bound to TypeScript via Wails.
Whenever backend service APIs or models change, regenerate the bindings:

```sh
wails generate module
```

The generated files in `frontend/wailsjs/` are tracked in version control as the Go/TypeScript API contract. Never hand-edit files under `frontend/wailsjs/`.

### Building & Native Packaging

Binaries are packaged natively for macOS and Windows. No Docker, CGo cross-compilers, or `fyne-cross` containers are required.

```sh
# Build packages for the current platform
mise run build

# macOS: builds separate arm64 and amd64 .app bundles + zipped distributions
mise run build-darwin

# Windows: builds 64-bit .exe with embedded WebView2 loader + zip
mise run build-windows
```

#### Output Artifacts

Packaged distribution artifacts are placed in `bin/`:

- `bin/will-msg-macos-arm64.app` & `bin/will-msg-macos-arm64.zip` (macOS Apple Silicon)
- `bin/will-msg-macos-amd64.app` & `bin/will-msg-macos-amd64.zip` (macOS Intel)
- `bin/will-msg-windows-amd64.exe` & `bin/will-msg-windows-amd64.zip` (Windows 64-bit)

#### Manual Wails Build Commands

```sh
# macOS (dual-architecture build)
wails build -platform darwin/arm64,darwin/amd64 -clean

# Windows (embedded WebView2 loader automatically checks Evergreen runtime or prompts installer)
wails build -platform windows/amd64 -clean -webview2 embed
```

---

## CLI Tools

### `will-msg` — Headless Batch Parser

Run the parser directly from the command line without opening the GUI.

#### Install

```sh
go install will-msg/cmd/will-msg
```

#### Usage

```sh
go run ./cmd/will-msg -input <path> -output <out.csv>
```

| Flag      | Description                                                    |
| --------- | -------------------------------------------------------------- |
| `-input`  | Path to a `.msg` file, `.zip` archive, or directory of messages |
| `-output` | Path to destination `.csv` file                                |

#### Examples

```sh
# Parse a directory of messages
go run ./cmd/will-msg -input data/ -output output/parsed.csv

# Parse a zipped archive
go run ./cmd/will-msg -input data/archive.zip -output output/parsed.csv

# Parse a single .msg file
go run ./cmd/will-msg -input "data/Medford Tags 01_02_26.msg" -output output/single.csv
```

---

### `msgcat` — Inspect a Single `.msg` File

`msgcat` is a standalone CLI utility that dumps the plain-text body (and optional header block) of any Outlook `.msg` file to stdout. It has zero external runtime dependencies beyond the standard library and parser.

#### Install

```sh
go install will-msg/cmd/msgcat
```

#### Usage

```sh
go run ./cmd/msgcat [-headers=false] <file.msg>
```

| Flag       | Default | Description                                      |
| ---------- | ------- | ------------------------------------------------ |
| `-headers` | `true`  | Print Subject/From/To/Date block before the body |

#### Examples

```sh
# Full output (headers + body)
go run ./cmd/msgcat "data/Medford Tags 01_02_26.msg"

# Body only — pipe into grep, wc, etc.
go run ./cmd/msgcat -headers=false "data/Medford Tags 01_02_26.msg" | grep "NOT OUT"

# Process every .msg in a directory
for f in data/*.msg; do echo "=== $f ==="; go run ./cmd/msgcat -headers=false "$f"; done

# Using the pre-configured mise parse helper
mise run parse "Medford Tags 01_02_26.msg"
```

---

## Testing

```sh
# Run backend tests with Go race detector
go test -race ./...

# Run frontend tests
(cd frontend && bun test)

# Run full project test suite via mise
mise run test
```
