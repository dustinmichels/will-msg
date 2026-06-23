# will-msg

Parse Microsoft Outlook `.msg` files into structured CSV rows.

The parser is tuned for the Medford tag emails in `data/`. It accepts either one `.msg` file or a directory containing `.msg` files, then emits one CSV row per reported service exception.

## `msgcat` — inspect a single `.msg` file

`msgcat` is a standalone CLI that dumps the plain-text body (and optional header block) of any Outlook `.msg` file to stdout. It has no dependencies beyond this module's existing parser.

### Install

```sh
go install will-msg/cmd/msgcat
```

Or run without installing:

```sh
go run ./cmd/msgcat <file.msg>
```

### Usage

```
msgcat [-headers=false] <file.msg>
```

| Flag       | Default | Description                                      |
| ---------- | ------- | ------------------------------------------------ |
| `-headers` | `true`  | Print Subject/From/To/Date block before the body |

### Examples

```sh
# Full output (headers + body)
msgcat "data-new/Medford Tags 09_10_25.msg"

# Body only — pipe into grep, wc, etc.
msgcat -headers=false "data-new/Medford Tags 09_10_25.msg" | grep "BULK"

# Process every .msg in a directory
for f in data-new/*.msg; do echo "=== $f ==="; msgcat -headers=false "$f"; done
```

## Build & Cross-Compilation

Fyne uses CGo to interface with native graphics drivers (OpenGL), which requires target-specific C compilers and system headers.

The recommended best practice to cross-compile Fyne applications is using **`fyne-cross`** (which uses Docker containers pre-configured with the required SDKs and compiler toolchains) alongside the modern **`fyne` CLI** on the host.

### Prerequisites

Ensure Docker is running, and install the required tools:

```sh
# Install the modern host packaging tool (required by fyne-cross)
go install fyne.io/tools/cmd/fyne@latest

# Install the cross-compilation runner
go install github.com/fyne-io/fyne-cross@latest
```

### Building with Mise

Tasks are pre-configured in the `mise.toml` file. You can run:

```sh
# Build both macOS and Windows packages
mise run build

# Build only macOS (separate arm64 + amd64 bundles)
mise run build-darwin

# Build only Windows (amd64)
mise run build-windows
```

The output binaries and packaged distributions will be placed in the `bin/` directory with platform-specific names:

- `bin/will-msg-macos-arm64` (macOS Apple Silicon binary)
- `bin/will-msg-macos-arm64.app` (macOS Apple Silicon app bundle)
- `bin/will-msg-macos-arm64.zip` (macOS Apple Silicon zipped app bundle)
- `bin/will-msg-macos-amd64` (macOS Intel binary)
- `bin/will-msg-macos-amd64.app` (macOS Intel app bundle)
- `bin/will-msg-macos-amd64.zip` (macOS Intel zipped app bundle)
- `bin/will-msg-windows-amd64.exe` (Windows binary)
- `bin/will-msg-windows-amd64.zip` (Windows packaged zip)

### Building Manually

Alternatively, you can run the commands directly:

```sh
# Generate Windows .exe inside fyne-cross/dist/windows-amd64/
fyne-cross windows -arch amd64

# Generate macOS bundles inside fyne-cross/dist/darwin/
fyne-cross darwin -arch arm64,amd64
```
