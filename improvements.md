# Repository Improvements & Engineering Action Plan

This document outlines prioritized, actionable engineering improvements for the `will-msg` repository across correctness, concurrency, packaging, UI/UX performance, modular architecture, and CI/CD pipelines.

---

## 1. Actionable Phased Todo Checklist

### Phase 1: Critical Correctness, Error Handling & Concurrency Fixes

- [ ] **Fix CSV writer flush and error check ordering in `main.go` (`writeCSV`)**
  - Explicitly call `writer.Flush()` before checking `writer.Error()`.
  - Propagate errors from both buffer flushing and `file.Close()`.
- [ ] **Guard global logging mutation (`log.SetOutput`) in MSG parser**
  - Mutex-protect or isolate `log.SetOutput(io.Discard)` in `main.go:133-136` and `cmd/msgcat/main.go:48-51` to prevent data races during concurrent message parsing.
- [ ] **Defensively copy `currentSources` in GUI background parser**
  - In `gui.go:528`, slice-copy `currentSources` prior to launching the worker goroutine to prevent concurrent read/write mutations when the user reloads files during active parsing.
- [ ] **Terminate the Pacman animation ticker loop on window close**
  - Bind `gui.go:341` background ticker loop to a cancellation context or stop mechanism triggered on window close (`w.SetOnClosed`) to prevent indefinite ~33 FPS background CPU consumption.

### Phase 2: Embedded Assets & Packaging Resilience

- [ ] **Embed `truck.png` using Go standard library `//go:embed`**
  - In `gui.go:654`, replace `canvas.NewImageFromFile("truck.png")` with `canvas.NewImageFromResource(truckResource)`.
  - Prevents the header image from failing to load when running packaged `.app` or `.exe` distributions outside the repository working directory.

### Phase 3: Engine Construction & Regex Validation Hardening

- [ ] **Prevent silent no-op rule degradation in `NewRuleEngine`**
  - In `engine.go:23-50`, validate `RuleConfig` or return `(*RuleEngine, error)` rather than discarding regex compilation errors.
  - In `engine.go:140` and `engine.go:187`, log warnings or fail loudly if an enabled regex rule is evaluated with an invalid expression.

### Phase 4: UI/UX Feedback & Resource Optimization

- [ ] **Add user confirmation feedback for "Save As..." in GUI**
  - In `gui.go:608-648`, replace empty `fyne.Do(func(){})` with a success dialog/notification matching the `Download CSV` flow.
  - Handle `file.Close()` and write errors cleanly.
- [ ] **Optimize CPU raster dotted border on high-DPI displays**
  - Replace the per-pixel trigonometric/Euclidean raster in `gui.go:730-787` with standard Fyne vector drawing primitives or a structured container layout to avoid heavy scalar math on every redraw frame.

### Phase 5: Architecture Decoupling & Code De-duplication

- [ ] **Extract core parser into an `internal/parser` package**
  - De-duplicate `.msg` body extraction logic shared between `main.go` and `cmd/msgcat/main.go`.
- [ ] **Separate GUI from root CLI package to isolate CGo / OpenGL dependencies**
  - Move Fyne UI code to `internal/gui` or `cmd/will-msg-gui`.
  - Allow headless CLI usage (`cmd/will-msg`) and core test execution without CGo/graphics headers.
- [ ] **Unify file and archive discovery logic**
  - Consolidate directory walking and `.zip` archive scanning from `main.go:collectInputPaths` and `gui.go:findMsgFiles` into a shared scanner.

### Phase 6: Build Determinism, Linting & CI/CD

- [ ] **Pin Go version in `mise.toml`**
  - Change `go = "latest"` in `mise.toml` to match `go 1.23.0` (or the project's baseline toolchain version) for reproducible builds.
- [ ] **Add GitHub Actions workflow (`.github/workflows/ci.yml`)**
  - Automated CI running `go test -race ./...`, `go vet ./...`, and cross-compilation smoke checks.
- [ ] **Optimize test regex compilation in `main_test.go`**
  - Hoist `regexp.MustCompile` out of the dataset iteration loop in `main_test.go:1203`.

---

## 2. Detailed Technical Specifications & Remedies

### 2.1 CSV Writer Flush & Error Ordering

- **File:** [`main.go:609-634`](main.go#L609-L634)
- **Problem:**
  In `writeCSV`, `defer writer.Flush()` executes _after_ `return writer.Error()`. While deferred calls execute in LIFO order (so `Flush()` runs before `file.Close()`), any I/O error generated during the final buffer flush or `file.Close()` is completely lost.
- **Proposed Code:**

  ```go
  func writeCSV(path string, records []record) (err error) {
      if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
          return err
      }

      file, err := os.Create(path)
      if err != nil {
          return err
      }
      defer func() {
          if closeErr := file.Close(); err == nil {
              err = closeErr
          }
      }()

      writer := csv.NewWriter(file)
      if err := writer.Write(csvHeaders); err != nil {
          return err
      }
      for _, rec := range records {
          if err := writer.Write(rec.toRow()); err != nil {
              return err
          }
      }

      writer.Flush()
      return writer.Error()
  }
  ```

---

### 2.2 Global `log.SetOutput` Race Condition

- **Files:** [`main.go:133-136`](main.go#L133-L136), [`cmd/msgcat/main.go:48-51`](cmd/msgcat/main.go#L48-L51)
- **Problem:**
  The Outlook MSG parser library (`github.com/willthrom/outlook-msg-parser`) emits noisy logs to the default standard library logger. Both `loadMessage` and `msgcat` mutate global `log.SetOutput(io.Discard)` without synchronization. When the GUI parses files in a background goroutine while other routines log, this produces a data race.
- **Proposed Code:**

  ```go
  var parseLogMu sync.Mutex

  func safeParseMsgFile(path string) (*msgparser.MsgFile, error) {
      parseLogMu.Lock()
      defer parseLogMu.Unlock()

      prevWriter := log.Writer()
      prevFlags := log.Flags()
      log.SetOutput(io.Discard)
      defer func() {
          log.SetOutput(prevWriter)
          log.SetFlags(prevFlags)
      }()

      return msgparser.ParseMsgFile(path)
  }
  ```

---

### 2.3 Asset Embedding (`truck.png`)

- **File:** [`gui.go:654`](gui.go#L654)
- **Problem:**
  `canvas.NewImageFromFile("truck.png")` expects `truck.png` in the current working directory (`os.Getwd()`). In standard application bundles (`.app` on macOS, installed `.exe` on Windows), the working directory is the user's home or system directory, causing the header logo to silently fail to render.
- **Proposed Code:**

  ```go
  // In gui.go:
  import (
      _ "embed"
      "fyne.io/fyne/v2"
      "fyne.io/fyne/v2/canvas"
  )

  //go:embed truck.png
  var truckPNGBytes []byte

  var truckResource = fyne.NewStaticResource("truck.png", truckPNGBytes)

  // In header construction:
  truckImg := canvas.NewImageFromResource(truckResource)
  truckImg.FillMode = canvas.ImageFillContain
  truckImg.SetMinSize(fyne.NewSize(40, 40))
  ```

---

### 2.4 Hardening `NewRuleEngine` Error Handling

- **File:** [`engine.go:23-50`](engine.go#L23-L50), [`engine.go:177-194`](engine.go#L177-L194)
- **Problem:**
  `NewRuleEngine` silently ignores errors from `compileRegexPattern(r.Pattern)`. When a regex rule fails to compile, `cr.compiledRE` remains `nil`. In `MatchRule` and `NormalizeIssueLabel`, `cr.compiledRE != nil` checks cause the broken rule to be silently skipped rather than alerting the developer/user to the malformed pattern.
- **Proposed Solution:**
  1. Add a constructor `NewRuleEngineValidated(cfg RuleConfig) (*RuleEngine, error)` that calls `cfg.Validate()`.
  2. For the infallible constructor `NewRuleEngine`, log a warning or panic in test/debug mode when an enabled rule contains an invalid regex.

---

### 2.5 GUI Animation Lifecycle & Background Ticker

- **File:** [`gui.go:341-395`](gui.go#L341-L395)
- **Problem:**
  The Pacman animation loop runs on a 30ms ticker in an unbounded goroutine. It continues running and queueing `fyne.Do()` UI dispatches even if the window is closed or backgrounded.
- **Proposed Code:**

  ```go
  ctx, cancelAnimation := context.WithCancel(context.Background())
  w.SetOnClosed(func() {
      cancelAnimation()
  })

  go func() {
      ticker := time.NewTicker(30 * time.Millisecond)
      defer ticker.Stop()

      for {
          select {
          case <-ctx.Done():
              return
          case <-ticker.C:
              // compute and dispatch frame via fyne.Do
          }
      }
  }()
  ```

---

### 2.6 Modular Architecture & Package Restructuring

- **Target Package Layout:**
  ```
  will-msg/
  ├── cmd/
  │   ├── will-msg/         # CLI runner (-input, -output)
  │   ├── will-msg-gui/     # Standalone Fyne desktop application
  │   └── msgcat/           # Outlook message dumper utility
  ├── internal/
  │   ├── config/           # RuleConfig, LabelDefinition, JSON serializer
  │   ├── engine/           # RuleEngine, classification, address splitting
  │   ├── parser/           # MSG reading, line normalization, wrapped continuation
  │   ├── scanner/          # Recursive folder & ZIP scanner
  │   ├── stats/            # Daily & Summary statistics computation
  │   └── gui/              # Fyne UI views, theme, and animations
  ├── testdata/             # Sample .msg files & CSV fixtures
  ├── scripts/              # Build & packaging scripts
  ├── mise.toml
  ├── go.mod
  └── go.sum
  ```

---

## 3. Verification & Testing Plan

| Component             | Verification Command / Step                                                               | Expected Outcome                                                        |
| :-------------------- | :---------------------------------------------------------------------------------------- | :---------------------------------------------------------------------- |
| **Race Detector**     | `go test -race ./...`                                                                     | Pass cleanly with zero data race warnings under concurrent parsing.     |
| **CSV Writing**       | `go test -run TestWriteCSV` with read-only/simulated disk failure                         | Returns and asserts non-nil error if disk/buffer fails.                 |
| **Packaged Asset**    | Launch binary from temporary directory outside repository: `cd /tmp && /path/to/will-msg` | Truck icon displays correctly in GUI header.                            |
| **Regex Rule Errors** | Construct `RuleConfig` with pattern `[`                                                   | `cfg.Validate()` and `NewRuleEngineValidated` return descriptive error. |
| **CI Pipeline**       | Trigger GitHub Actions workflow on PR                                                     | All unit tests, `go vet`, and cross-compilation jobs pass.              |
