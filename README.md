# will-msg

Parse Microsoft Outlook `.msg` files into structured CSV rows.

The parser is tuned for the Medford tag emails in `data/`. It accepts either one `.msg` file or a directory containing `.msg` files, then emits one CSV row per reported service exception.

## Build

With docker running:

```sh
go install github.com/fyne-io/fyne-cross@latest

#  downloads a Docker image containing the Windows GCC cross-compiler and outputs a Windows .exe inside a newly created fyne-cross/dist/windows-amd64/ directory
fyne-cross windows -arch amd64

# generates binaries for both Apple Silicon (arm64) and Intel (amd64) Macs in the fyne-cross/dist/darwin/ directory.
fyne-cross darwin -arch arm64,amd64
```
