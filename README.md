# will-msg

Parse Microsoft Outlook `.msg` files into structured CSV rows.

The parser is tuned for the Medford tag emails in `data/`. It accepts either one `.msg` file or a directory containing `.msg` files, then emits one CSV row per reported service exception.

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
