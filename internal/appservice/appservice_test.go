package appservice_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"will-msg/internal/appservice"
	"will-msg/internal/config"
	"will-msg/internal/engine"
	"will-msg/internal/parser"
	"will-msg/internal/scanner"
)

func TestMain(m *testing.M) {
	tmpHome, err := os.MkdirTemp("", "will-msg-test-home-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpHome)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	code := m.Run()
	os.Setenv("HOME", origHome)
	os.Exit(code)
}

func newIsolatedService(t *testing.T, opts ...appservice.ServiceOptions) (*appservice.Service, string) {
	t.Helper()
	tmpConfigFile := filepath.Join(t.TempDir(), "rules.json")
	var baseOpts appservice.ServiceOptions
	if len(opts) > 0 {
		baseOpts = opts[0]
	}
	if baseOpts.ConfigSaver == nil {
		baseOpts.ConfigSaver = func(cfg config.RuleConfig) error {
			return config.SaveConfigToPath(tmpConfigFile, cfg)
		}
	}
	if baseOpts.ConfigLoader == nil {
		baseOpts.ConfigLoader = func() config.RuleConfig {
			cfg, err := config.LoadConfigFromPath(tmpConfigFile)
			if err != nil {
				return config.DefaultRuleConfig()
			}
			return cfg
		}
	}
	svc := appservice.NewServiceWithOptions(baseOpts)
	return svc, tmpConfigFile
}

// mockDialogAdapter provides controllable responses for native dialogs in unit tests.
type mockDialogAdapter struct {
	mu sync.Mutex

	openMultipleFilesResult []string
	openMultipleFilesErr    error

	openDirectoryResult string
	openDirectoryErr    error

	openFileResult string
	openFileErr    error

	saveFileResult string
	saveFileErr    error

	messageDialogResult string
	messageDialogErr    error
}

func (m *mockDialogAdapter) OpenMultipleFilesDialog(ctx context.Context, options wailsruntime.OpenDialogOptions) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.openMultipleFilesResult, m.openMultipleFilesErr
}

func (m *mockDialogAdapter) OpenDirectoryDialog(ctx context.Context, options wailsruntime.OpenDialogOptions) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.openDirectoryResult, m.openDirectoryErr
}

func (m *mockDialogAdapter) OpenFileDialog(ctx context.Context, options wailsruntime.OpenDialogOptions) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.openFileResult, m.openFileErr
}

func (m *mockDialogAdapter) SaveFileDialog(ctx context.Context, options wailsruntime.SaveDialogOptions) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveFileResult, m.saveFileErr
}

func (m *mockDialogAdapter) MessageDialog(ctx context.Context, options wailsruntime.MessageDialogOptions) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messageDialogResult, m.messageDialogErr
}

func TestDTOSerialization(t *testing.T) {
	t.Run("ScanResult empty slice marshals as array", func(t *testing.T) {
		res := appservice.ScanResult{
			Sources:     make([]scanner.MessageSource, 0),
			Count:       0,
			SourcePaths: make([]string, 0),
		}
		data, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal ScanResult: %v", err)
		}
		expected := `{"sources":[],"count":0,"source_paths":[]}`
		if string(data) != expected {
			t.Errorf("got %s, want %s", string(data), expected)
		}
	})

	t.Run("ParseResult empty slice marshals as array", func(t *testing.T) {
		res := appservice.ParseResult{
			Records:    make([]engine.Record, 0),
			Skipped:    make([]appservice.SkippedSource, 0),
			Superseded: false,
		}
		data, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal ParseResult: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal ParseResult: %v", err)
		}
		if decoded["records"] == nil || decoded["skipped"] == nil {
			t.Errorf("expected non-null records and skipped arrays, got %s", string(data))
		}
	})

	t.Run("ImportRulesResult cancelled omits config", func(t *testing.T) {
		res := appservice.ImportRulesResult{
			Cancelled: true,
		}
		data, err := json.Marshal(res)
		if err != nil {
			t.Fatalf("marshal ImportRulesResult: %v", err)
		}
		expected := `{"cancelled":true}`
		if string(data) != expected {
			t.Errorf("got %s, want %s", string(data), expected)
		}
	})

	t.Run("ValidationError json tags", func(t *testing.T) {
		ve := appservice.ValidationError{
			Path:    "/rules/0/pattern",
			Message: "Pattern cannot be empty",
		}
		data, err := json.Marshal(ve)
		if err != nil {
			t.Fatalf("marshal ValidationError: %v", err)
		}
		expected := `{"path":"/rules/0/pattern","message":"Pattern cannot be empty"}`
		if string(data) != expected {
			t.Errorf("got %s, want %s", string(data), expected)
		}
	})

	t.Run("SandboxResult json tags", func(t *testing.T) {
		sr := appservice.SandboxResult{
			Address:          "45 FOREST ST",
			Status:           "TRASH AND RCY NOT OUT",
			IssueTime:        "08:30 AM",
			Label:            "msw_and_recyc_not_out",
			MatchedRuleIndex: 0,
			MatchedRuleID:    "msw_and_recyc_not_out_7",
			MatchKind:        "rule",
			Metric:           config.MetricBoth,
		}
		data, err := json.Marshal(sr)
		if err != nil {
			t.Fatalf("marshal SandboxResult: %v", err)
		}
		var decoded map[string]any
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if decoded["matched_rule_index"] != float64(0) || decoded["matched_rule_id"] != "msw_and_recyc_not_out_7" {
			t.Errorf("unexpected decoded payload: %v", decoded)
		}
	})
}

func TestSelectFilesAndFolder(t *testing.T) {
	mockDialog := &mockDialogAdapter{
		openMultipleFilesResult: []string{"/tmp/a.msg", "/tmp/b.zip"},
		openDirectoryResult:     "/tmp/msg_folder",
	}

	svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
		DialogAdapter: mockDialog,
	})

	files, err := svc.SelectFiles()
	if err != nil {
		t.Fatalf("SelectFiles: %v", err)
	}
	if len(files) != 2 || files[0] != "/tmp/a.msg" || files[1] != "/tmp/b.zip" {
		t.Errorf("unexpected files: %v", files)
	}

	folder, err := svc.SelectFolder()
	if err != nil {
		t.Fatalf("SelectFolder: %v", err)
	}
	if folder != "/tmp/msg_folder" {
		t.Errorf("unexpected folder: %q", folder)
	}

	// Test Cancel
	mockDialog.openMultipleFilesResult = nil
	mockDialog.openDirectoryResult = ""

	files, err = svc.SelectFiles()
	if err != nil {
		t.Fatalf("SelectFiles cancel: %v", err)
	}
	if files == nil || len(files) != 0 {
		t.Errorf("expected empty non-nil slice on cancel, got: %v", files)
	}

	folder, err = svc.SelectFolder()
	if err != nil {
		t.Fatalf("SelectFolder cancel: %v", err)
	}
	if folder != "" {
		t.Errorf("expected empty string on cancel, got: %q", folder)
	}
}

func TestScanSources(t *testing.T) {
	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "file1.msg")
	f2 := filepath.Join(tmpDir, "file2.msg")
	if err := os.WriteFile(f1, []byte("fake msg 1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("fake msg 2"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := appservice.NewService()

	// Scan with duplicate root and individual file
	res, err := svc.ScanSources([]string{tmpDir, f1})
	if err != nil {
		t.Fatalf("ScanSources: %v", err)
	}
	if res.Count != 2 {
		t.Errorf("expected 2 deduplicated sources, got %d", res.Count)
	}
	if len(res.SourcePaths) != 2 {
		t.Errorf("expected 2 source paths, got %d", len(res.SourcePaths))
	}

	// Invalid root returns error
	_, err = svc.ScanSources([]string{"/nonexistent/path/that/does/not/exist"})
	if err == nil {
		t.Error("expected error for non-existent path, got nil")
	}
}

func TestParseLifecycleAndConcurrency(t *testing.T) {
	msgDate := time.Date(2026, 1, 5, 8, 30, 0, 0, time.UTC)

	t.Run("Normal parse commits records", func(t *testing.T) {
		mockLoader := func(src scanner.MessageSource) (parser.MessageMetadata, error) {
			return parser.MessageMetadata{
				SourceFile:  src.Path,
				Subject:     "Medford Tags",
				MessageDate: msgDate,
				Body:        "01/05/2026 08:30:00 Dave\n45 FOREST ST TRASH AND RCY NOT OUT 0830AM",
			}, nil
		}

		svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
			SourceLoader: mockLoader,
		})

		sources := []scanner.MessageSource{
			{Path: "test1.msg", DisplayName: "test1.msg"},
		}

		res, err := svc.Parse(sources)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if res.Superseded {
			t.Error("expected superseded to be false")
		}
		if len(res.Records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(res.Records))
		}
		if res.Records[0].LocationHint != "45 FOREST ST" {
			t.Errorf("got location %q, want '45 FOREST ST'", res.Records[0].LocationHint)
		}

		// Save to downloads works after parse
		tmpDir := t.TempDir()
		svc = appservice.NewServiceWithOptions(appservice.ServiceOptions{
			SourceLoader: mockLoader,
			DownloadsDir: func() (string, error) { return tmpDir, nil },
		})
		_, _ = svc.Parse(sources)
		saved, err := svc.SaveToDownloads()
		if err != nil {
			t.Fatalf("SaveToDownloads: %v", err)
		}
		if saved.Cancelled {
			t.Error("expected not cancelled")
		}
		if _, err := os.Stat(saved.Path); err != nil {
			t.Fatalf("expected saved file on disk: %v", err)
		}
	})

	t.Run("Skipped sources surfaced in result", func(t *testing.T) {
		mockLoader := func(src scanner.MessageSource) (parser.MessageMetadata, error) {
			if src.Path == "corrupt.msg" {
				return parser.MessageMetadata{}, errors.New("read error: corrupted file")
			}
			return parser.MessageMetadata{
				SourceFile:  src.Path,
				Subject:     "Medford Tags",
				MessageDate: msgDate,
				Body:        "01/05/2026 08:30:00 Dave\n10 MAIN ST TRASH NOT OUT",
			}, nil
		}

		svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
			SourceLoader: mockLoader,
		})

		sources := []scanner.MessageSource{
			{Path: "corrupt.msg", DisplayName: "corrupt.msg"},
			{Path: "good.msg", DisplayName: "good.msg"},
		}

		res, err := svc.Parse(sources)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(res.Skipped) != 1 {
			t.Fatalf("expected 1 skipped source, got %d", len(res.Skipped))
		}
		if res.Skipped[0].DisplayName != "corrupt.msg" || res.Skipped[0].Error != "read error: corrupted file" {
			t.Errorf("unexpected skipped entry: %+v", res.Skipped[0])
		}
		if len(res.Records) != 1 {
			t.Fatalf("expected 1 record from good source, got %d", len(res.Records))
		}
	})

	t.Run("ClearParse clears active state and supersedes ongoing parse", func(t *testing.T) {
		parseStarted := make(chan struct{})
		allowParseFinish := make(chan struct{})

		mockLoader := func(src scanner.MessageSource) (parser.MessageMetadata, error) {
			close(parseStarted)
			<-allowParseFinish
			return parser.MessageMetadata{
				SourceFile:  src.Path,
				Subject:     "Medford Tags",
				MessageDate: msgDate,
				Body:        "01/05/2026 08:30:00 Dave\n10 MAIN ST TRASH NOT OUT",
			}, nil
		}

		svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
			SourceLoader: mockLoader,
		})

		var res appservice.ParseResult
		var parseErr error
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			res, parseErr = svc.Parse([]scanner.MessageSource{{Path: "slow.msg"}})
		}()

		<-parseStarted
		// Clear parse while parsing is in progress
		svc.ClearParse()
		close(allowParseFinish)
		wg.Wait()

		if parseErr != nil {
			t.Fatalf("Parse error: %v", parseErr)
		}
		if !res.Superseded {
			t.Errorf("expected superseded=true, got %v", res.Superseded)
		}
		if len(res.Records) != 0 {
			t.Errorf("expected 0 records on superseded parse, got %d", len(res.Records))
		}

		// Export after superseded parse must fail because lastRecords is empty
		_, err := svc.SaveToDownloads()
		if err == nil {
			t.Error("expected export error after clear/superseded parse, got nil")
		}
	})

	t.Run("Newer parse supersedes older parse", func(t *testing.T) {
		firstStarted := make(chan struct{})
		allowFirstFinish := make(chan struct{})

		mockLoader := func(src scanner.MessageSource) (parser.MessageMetadata, error) {
			if src.Path == "first.msg" {
				close(firstStarted)
				<-allowFirstFinish
				return parser.MessageMetadata{
					SourceFile:  src.Path,
					Subject:     "Medford Tags",
					MessageDate: msgDate,
					Body:        "01/05/2026 08:30:00 Dave\n1 FIRST ST TRASH NOT OUT",
				}, nil
			}
			return parser.MessageMetadata{
				SourceFile:  src.Path,
				Subject:     "Medford Tags",
				MessageDate: msgDate,
				Body:        "01/05/2026 08:30:00 Dave\n2 SECOND ST RECYCLING NOT OUT",
			}, nil
		}

		svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
			SourceLoader: mockLoader,
		})

		var res1, res2 appservice.ParseResult
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			res1, _ = svc.Parse([]scanner.MessageSource{{Path: "first.msg"}})
		}()

		<-firstStarted
		go func() {
			defer wg.Done()
			res2, _ = svc.Parse([]scanner.MessageSource{{Path: "second.msg"}})
		}()

		// Give second parse a moment to execute and increment parseGen
		time.Sleep(50 * time.Millisecond)
		close(allowFirstFinish)
		wg.Wait()

		if !res1.Superseded {
			t.Error("expected first parse to be superseded")
		}
		if res2.Superseded {
			t.Error("expected second parse to NOT be superseded")
		}
		if len(res2.Records) != 1 {
			t.Fatalf("expected 1 record from second parse, got %d", len(res2.Records))
		}
	})
}

func TestExportOperations(t *testing.T) {
	tmpDir := t.TempDir()
	msgDate := time.Date(2026, 1, 5, 8, 30, 0, 0, time.UTC)

	mockLoader := func(src scanner.MessageSource) (parser.MessageMetadata, error) {
		return parser.MessageMetadata{
			SourceFile:  src.Path,
			Subject:     "Medford Tags",
			MessageDate: msgDate,
			Body:        "01/05/2026 08:30:00 Dave\n45 FOREST ST TRASH AND RCY NOT OUT 0830AM",
		}, nil
	}

	savePath := filepath.Join(tmpDir, "custom_export.csv")
	mockDialog := &mockDialogAdapter{
		saveFileResult: savePath,
	}

	svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
		SourceLoader:  mockLoader,
		DialogAdapter: mockDialog,
		DownloadsDir:  func() (string, error) { return tmpDir, nil },
	})

	// Export before parse should fail with error
	_, err := svc.SaveToDownloads()
	if err == nil {
		t.Error("expected error exporting before any parse, got nil")
	}

	_, err = svc.SaveAs()
	if err == nil {
		t.Error("expected error SaveAs before any parse, got nil")
	}

	// Parse records
	_, err = svc.Parse([]scanner.MessageSource{{Path: "test.msg"}})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// SaveAs success
	saved, err := svc.SaveAs()
	if err != nil {
		t.Fatalf("SaveAs: %v", err)
	}
	if saved.Cancelled {
		t.Error("expected cancelled to be false")
	}
	if saved.Path != savePath {
		t.Errorf("got path %q, want %q", saved.Path, savePath)
	}

	content, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatalf("read saved CSV: %v", err)
	}
	if len(content) == 0 {
		t.Error("exported CSV is empty")
	}

	// SaveAs cancelled
	mockDialog.saveFileResult = ""
	savedCancel, err := svc.SaveAs()
	if err != nil {
		t.Fatalf("SaveAs cancel: %v", err)
	}
	if !savedCancel.Cancelled {
		t.Error("expected cancelled=true for empty dialog path")
	}
}

func TestRulesOperations(t *testing.T) {
	svc, _ := newIsolatedService(t)
	t.Run("GetRules and GetDefaultRules", func(t *testing.T) {
		def := svc.GetDefaultRules()
		if def.DefaultLabel != "other" {
			t.Errorf("expected default label 'other', got %q", def.DefaultLabel)
		}
		if len(def.Rules) == 0 {
			t.Error("expected non-empty default rules")
		}

		current := svc.GetRules()
		if current.DefaultLabel == "" {
			t.Error("expected non-empty current rules default label")
		}
	})

	t.Run("ValidateRules JSON pointer paths", func(t *testing.T) {
		cfg := config.DefaultRuleConfig()
		cfg.DefaultLabel = "missing_label"
		cfg.Rules = append(cfg.Rules, config.ClassificationRule{
			ID:      "test_bad_regex",
			Pattern: "[invalid(regex",
			Type:    config.RuleTypeRegex,
			Label:   "msw_not_out",
			Enabled: true,
		})
		cfg.Rules = append(cfg.Rules, config.ClassificationRule{
			ID:      "",
			Pattern: "",
			Type:    config.RuleTypeSubstring,
			Label:   "undefined_label",
			Enabled: true,
		})

		errs := svc.ValidateRules(cfg)
		if len(errs) == 0 {
			t.Fatal("expected validation errors, got none")
		}

		paths := make(map[string]bool)
		for _, e := range errs {
			paths[e.Path] = true
		}

		if !paths["/default_label"] {
			t.Errorf("expected error at /default_label, got paths: %+v", paths)
		}
		ruleIdx1 := len(cfg.Rules) - 2
		ruleIdx2 := len(cfg.Rules) - 1
		if !paths[fmt.Sprintf("/rules/%d/pattern", ruleIdx1)] {
			t.Errorf("expected error at /rules/%d/pattern", ruleIdx1)
		}
		if !paths[fmt.Sprintf("/rules/%d/id", ruleIdx2)] {
			t.Errorf("expected error at /rules/%d/id", ruleIdx2)
		}
		if !paths[fmt.Sprintf("/rules/%d/pattern", ruleIdx2)] {
			t.Errorf("expected error at /rules/%d/pattern", ruleIdx2)
		}
		if !paths[fmt.Sprintf("/rules/%d/label", ruleIdx2)] {
			t.Errorf("expected error at /rules/%d/label", ruleIdx2)
		}
	})

	t.Run("TestRule Sandbox evaluation", func(t *testing.T) {
		cfg := config.DefaultRuleConfig()

		// Sample 1: Both Not Out
		res1, err := svc.TestRule("45 FOREST ST TRASH AND RCY NOT OUT 0830AM", cfg)
		if err != nil {
			t.Fatalf("TestRule sample 1: %v", err)
		}
		if res1.Address != "45 FOREST ST" {
			t.Errorf("got address %q, want '45 FOREST ST'", res1.Address)
		}
		if res1.MatchKind != "rule" {
			t.Errorf("got match_kind %q, want 'rule'", res1.MatchKind)
		}
		if res1.Label != "msw_and_recyc_not_out" {
			t.Errorf("got label %q, want 'msw_and_recyc_not_out'", res1.Label)
		}
		if res1.Metric != config.MetricBoth {
			t.Errorf("got metric %q, want %q", res1.Metric, config.MetricBoth)
		}

		// Sample 2: Contaminated
		res2, err := svc.TestRule("14 DARTMOUTH ST RECYC CONTAM", cfg)
		if err != nil {
			t.Fatalf("TestRule sample 2: %v", err)
		}
		if res2.MatchKind != "rule" || res2.Label != "recyc_contaminated" {
			t.Errorf("got match_kind %q, label %q", res2.MatchKind, res2.Label)
		}

		// Sample 3: Blocked
		res3, err := svc.TestRule("88 BOSTON AVE BLOCKED ACCESS", cfg)
		if err != nil {
			t.Fatalf("TestRule sample 3: %v", err)
		}
		if res3.MatchKind != "rule" || res3.Label != "blocked" {
			t.Errorf("got match_kind %q, label %q", res3.MatchKind, res3.Label)
		}

		// Sample 4: Heuristic Suffix
		res4, err := svc.TestRule("10 HIGH ST MATTRESS NOT OUT", cfg)
		if err != nil {
			t.Fatalf("TestRule sample 4: %v", err)
		}
		if res4.MatchKind != "heuristic" || res4.Label != "special_item_not_out" {
			t.Errorf("got match_kind %q, label %q", res4.MatchKind, res4.Label)
		}

		// Invalid regex returns validation error
		badCfg := cfg.Clone()
		badCfg.Rules[0].Type = config.RuleTypeRegex
		badCfg.Rules[0].Pattern = "[invalid"
		_, err = svc.TestRule("sample", badCfg)
		if err == nil {
			t.Error("expected error on TestRule with invalid regex, got nil")
		}
	})

	t.Run("Rules dirty flag management", func(t *testing.T) {
		cleanSvc, _ := newIsolatedService(t)
		if cleanSvc.IsRulesDirty() {
			t.Error("expected initial dirty to be false")
		}
		cleanSvc.SetRulesDirty(true)
		if !cleanSvc.IsRulesDirty() {
			t.Error("expected dirty to be true")
		}
		cleanSvc.SetRulesDirty(false)
		if cleanSvc.IsRulesDirty() {
			t.Error("expected dirty to be false")
		}
	})

	t.Run("SaveRules validates, persists, and resets dirty flag", func(t *testing.T) {
		cfg := config.DefaultRuleConfig()
		cfg.DefaultLabel = "msw_not_out"

		svc.SetRulesDirty(true)
		if err := svc.SaveRules(cfg); err != nil {
			t.Fatalf("SaveRules: %v", err)
		}

		if svc.IsRulesDirty() {
			t.Error("expected dirty flag to be cleared after SaveRules")
		}

		// Invalid configuration rejected
		badCfg := cfg.Clone()
		badCfg.DefaultLabel = "non_existent_label"
		if err := svc.SaveRules(badCfg); err == nil {
			t.Error("expected SaveRules to fail with invalid config, got nil")
		}
	})

	t.Run("ImportRules and ExportRules", func(t *testing.T) {
		tmpDir := t.TempDir()
		jsonPath := filepath.Join(tmpDir, "exported_rules.json")

		mockDialog := &mockDialogAdapter{
			saveFileResult: jsonPath,
			openFileResult: jsonPath,
		}

		svcWithOptions, _ := newIsolatedService(t, appservice.ServiceOptions{
			DialogAdapter: mockDialog,
		})
		cfg := config.DefaultRuleConfig()
		saved, err := svcWithOptions.ExportRules(cfg)
		if err != nil {
			t.Fatalf("ExportRules: %v", err)
		}
		if saved.Cancelled || saved.Path != jsonPath {
			t.Errorf("unexpected saved file result: %+v", saved)
		}

		imported, err := svcWithOptions.ImportRules()
		if err != nil {
			t.Fatalf("ImportRules: %v", err)
		}
		if imported.Cancelled || imported.Config == nil {
			t.Fatalf("unexpected imported rules result: %+v", imported)
		}
		if imported.Config.DefaultLabel != cfg.DefaultLabel {
			t.Errorf("got default label %q, want %q", imported.Config.DefaultLabel, cfg.DefaultLabel)
		}

		// Import cancelled
		mockDialog.openFileResult = ""
		importCancel, err := svcWithOptions.ImportRules()
		if err != nil {
			t.Fatalf("ImportRules cancel: %v", err)
		}
		if !importCancel.Cancelled {
			t.Error("expected cancelled=true for empty import path")
		}

		// Export cancelled
		mockDialog.saveFileResult = ""
		exportCancel, err := svcWithOptions.ExportRules(cfg)
		if err != nil {
			t.Fatalf("ExportRules cancel: %v", err)
		}
		if !exportCancel.Cancelled {
			t.Error("expected cancelled=true for empty export path")
		}
	})
}

func helperCreateZip(t *testing.T, files map[string][]byte) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test-*.zip")
	if err != nil {
		t.Fatalf("create temp zip: %v", err)
	}
	defer tmpFile.Close()

	zw := zip.NewWriter(tmpFile)
	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		_, err = f.Write(content)
		if err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return tmpFile.Name()
}

func TestScanSources_DropFolderMsgZipAndOverlappingRoots(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Folder with nested .msg files
	subDir := filepath.Join(tmpDir, "subfolder")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file1 := filepath.Join(tmpDir, "file1.msg")
	file2 := filepath.Join(subDir, "file2.msg")
	if err := os.WriteFile(file1, []byte("fake msg 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("fake msg 2"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Standalone .msg file
	file3 := filepath.Join(tmpDir, "standalone.msg")
	if err := os.WriteFile(file3, []byte("fake msg 3"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Zip archive containing .msg files
	zipPath := helperCreateZip(t, map[string][]byte{
		"archive/file4.msg": []byte("fake msg 4"),
		"file5.msg":         []byte("fake msg 5"),
	})
	defer os.Remove(zipPath)

	svc := appservice.NewService()

	// Scan with:
	// - folder (contains file1, file2)
	// - file1 specifically (overlapping with folder)
	// - file3 standalone
	// - zipPath (contains file4, file5)
	// - duplicate zipPath
	// - duplicate folder
	res, err := svc.ScanSources([]string{tmpDir, file1, file3, zipPath, zipPath, tmpDir})
	if err != nil {
		t.Fatalf("ScanSources: %v", err)
	}

	// Exactly 5 unique message sources expected: file1, file2, file3, file4 (in zip), file5 (in zip)
	if res.Count != 5 {
		t.Fatalf("expected 5 deduplicated sources, got %d", res.Count)
	}
	if len(res.Sources) != 5 {
		t.Fatalf("expected 5 sources in slice, got %d", len(res.Sources))
	}

	// Verify no duplicates in the sources slice
	seenKeys := make(map[string]bool)
	for _, src := range res.Sources {
		key := fmt.Sprintf("%s|%t|%s", src.Path, src.InZip, src.ZipPath)
		if seenKeys[key] {
			t.Errorf("duplicate source found: %s", key)
		}
		seenKeys[key] = true
	}
}

func TestDialogCancellation_ExplicitResults(t *testing.T) {
	mockDialog := &mockDialogAdapter{}
	svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
		DialogAdapter: mockDialog,
	})

	t.Run("SelectFiles cancelled returns empty slice and no error", func(t *testing.T) {
		mockDialog.openMultipleFilesResult = nil
		mockDialog.openMultipleFilesErr = nil
		files, err := svc.SelectFiles()
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if len(files) != 0 {
			t.Errorf("expected empty slice, got %+v", files)
		}
	})

	t.Run("SelectFolder cancelled returns empty string and no error", func(t *testing.T) {
		mockDialog.openDirectoryResult = ""
		mockDialog.openDirectoryErr = nil
		folder, err := svc.SelectFolder()
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if folder != "" {
			t.Errorf("expected empty string, got %q", folder)
		}
	})

	t.Run("SaveAs cancelled returns Cancelled=true and no error", func(t *testing.T) {
		// First prime lastRecords with a parse
		mockLoader := func(src scanner.MessageSource) (parser.MessageMetadata, error) {
			return parser.MessageMetadata{
				SourceFile:  src.Path,
				Subject:     "Tags",
				MessageDate: time.Date(2026, 1, 5, 8, 30, 0, 0, time.UTC),
				Body:        "01/05/2026 08:30:00 Dave\n45 FOREST ST TRASH AND RCY NOT OUT 0830AM",
			}, nil
		}
		svcWithRecords := appservice.NewServiceWithOptions(appservice.ServiceOptions{
			DialogAdapter: mockDialog,
			SourceLoader:  mockLoader,
		})
		_, _ = svcWithRecords.Parse([]scanner.MessageSource{{Path: "test.msg", DisplayName: "test.msg"}})

		mockDialog.saveFileResult = ""
		mockDialog.saveFileErr = nil
		saved, err := svcWithRecords.SaveAs()
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !saved.Cancelled {
			t.Error("expected Cancelled=true for SaveAs cancellation")
		}
	})

	t.Run("ImportRules cancelled returns Cancelled=true and no error", func(t *testing.T) {
		mockDialog.openFileResult = ""
		mockDialog.openFileErr = nil
		result, err := svc.ImportRules()
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !result.Cancelled {
			t.Error("expected Cancelled=true for ImportRules cancellation")
		}
		if result.Config != nil {
			t.Errorf("expected Config=nil, got %+v", result.Config)
		}
	})

	t.Run("ExportRules cancelled returns Cancelled=true and no error", func(t *testing.T) {
		mockDialog.saveFileResult = ""
		mockDialog.saveFileErr = nil
		cfg := config.DefaultRuleConfig()
		saved, err := svc.ExportRules(cfg)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if !saved.Cancelled {
			t.Error("expected Cancelled=true for ExportRules cancellation")
		}
	})
}

func TestRevealFile_Integration(t *testing.T) {
	t.Run("Injected revealer receives exact file path", func(t *testing.T) {
		var revealedPath string
		var revealerCalled bool
		mockRevealer := func(ctx context.Context, path string) error {
			revealerCalled = true
			revealedPath = path
			return nil
		}

		svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
			Revealer: mockRevealer,
		})

		targetPath := "/path/to/exported file #1 (copy).csv"
		err := svc.RevealFile(targetPath)
		if err != nil {
			t.Fatalf("RevealFile: %v", err)
		}
		if !revealerCalled {
			t.Error("expected revealer to be called")
		}
		if revealedPath != targetPath {
			t.Errorf("got %q, want %q", revealedPath, targetPath)
		}
	})

	t.Run("Default revealer does not error on existing temp file on macOS", func(t *testing.T) {
		tmpDir := t.TempDir()
		tmpFile := filepath.Join(tmpDir, "test_reveal.csv")
		if err := os.WriteFile(tmpFile, []byte("col1,col2\nval1,val2\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		svc := appservice.NewService()
		err := svc.RevealFile(tmpFile)
		if err != nil {
			t.Fatalf("RevealFile on existing file returned error: %v", err)
		}
	})
}

func TestPathsWithSpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()

	// Read a real .msg fixture
	fixturePath := filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg")
	fixtureData, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// Create subdirectories with spaces, parentheses, #, and non-ASCII unicode
	specialDirs := []string{
		filepath.Join(tmpDir, "folder with spaces"),
		filepath.Join(tmpDir, "folder (with parens) [copy 1]"),
		filepath.Join(tmpDir, "folder#with#hash"),
		filepath.Join(tmpDir, "dossier_français_日本語_üñîçødé"),
	}

	var createdMsgFiles []string
	for i, dir := range specialDirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		msgName := fmt.Sprintf("sample #%d (date 2026-01-02) 測試 %s.msg", i+1, filepath.Base(dir))
		msgPath := filepath.Join(dir, msgName)
		if err := os.WriteFile(msgPath, fixtureData, 0o644); err != nil {
			t.Fatalf("write msg %s: %v", msgPath, err)
		}
		createdMsgFiles = append(createdMsgFiles, msgPath)
	}

	// Create a zip file containing special-character entry names inside a special-character directory
	zipDir := filepath.Join(tmpDir, "zip folder (archive #99)")
	if err := os.MkdirAll(zipDir, 0o755); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(zipDir, "special #1 (test) 郵便.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zipFile)
	entryName := "archive subfolder (1)/message #99 日本語.msg"
	w, err := zw.Create(entryName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(fixtureData); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zipFile.Close()

	svc := appservice.NewService()

	// 1. Scan all special-character paths together
	scanPaths := append(specialDirs, zipPath)
	scanResult, err := svc.ScanSources(scanPaths)
	if err != nil {
		t.Fatalf("ScanSources with special characters: %v", err)
	}

	expectedCount := len(createdMsgFiles) + 1 // 4 created files + 1 in zip
	if scanResult.Count != expectedCount {
		t.Fatalf("expected %d sources from special character scan, got %d", expectedCount, scanResult.Count)
	}

	// 2. Parse all special-character sources
	parseResult, err := svc.Parse(scanResult.Sources)
	if err != nil {
		t.Fatalf("Parse with special characters: %v", err)
	}
	if len(parseResult.Skipped) != 0 {
		t.Fatalf("expected 0 skipped sources, got %+v", parseResult.Skipped)
	}
	if len(parseResult.Records) == 0 {
		t.Fatal("expected parsed records from special character sources, got 0")
	}

	// 3. Export to a destination path with spaces, parens, #, and unicode
	exportDir := filepath.Join(tmpDir, "export destination (final) #1 日本語")
	exportFile := filepath.Join(exportDir, "msg_parsed #1 (test) üñîçødé.csv")

	mockDialog := &mockDialogAdapter{
		saveFileResult: exportFile,
	}
	svcWithMock := appservice.NewServiceWithOptions(appservice.ServiceOptions{
		DialogAdapter: mockDialog,
	})
	// Parse into this service to populate records
	_, _ = svcWithMock.Parse(scanResult.Sources)

	savedFile, err := svcWithMock.SaveAs()
	if err != nil {
		t.Fatalf("SaveAs to special path: %v", err)
	}
	if savedFile.Cancelled {
		t.Fatal("SaveAs cancelled unexpectedly")
	}
	if savedFile.Path != exportFile {
		t.Errorf("got path %q, want %q", savedFile.Path, exportFile)
	}

	// Verify exported CSV file exists and has content
	stat, err := os.Stat(exportFile)
	if err != nil {
		t.Fatalf("stat exported file: %v", err)
	}
	if stat.Size() == 0 {
		t.Fatal("exported CSV file is empty")
	}

	// 4. Reveal file at special path
	if err := svc.RevealFile(exportFile); err != nil {
		t.Fatalf("RevealFile with special path: %v", err)
	}
}

func TestParseMsgSources_Rehomed(t *testing.T) {
	// Re-homes TestParseMsgSources from legacy gui_test.go
	msgPath := filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg")
	svc := appservice.NewService()

	scanRes, err := svc.ScanSources([]string{msgPath})
	if err != nil {
		t.Fatalf("ScanSources failed: %v", err)
	}
	if scanRes.Count != 1 {
		t.Fatalf("expected 1 source, got %d", scanRes.Count)
	}

	parseRes, err := svc.Parse(scanRes.Sources)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(parseRes.Records) != 5 {
		t.Errorf("expected 5 records, got %d", len(parseRes.Records))
	}
	if len(parseRes.Skipped) != 0 {
		t.Errorf("expected 0 skipped, got %d", len(parseRes.Skipped))
	}
}

func TestGoldenCSVEquivalence_CutoverGate(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata")
	tmpDir := t.TempDir()

	// 1. Service path: Scan -> Parse -> SaveAs over testdata/
	serviceExportPath := filepath.Join(tmpDir, "service_export.csv")
	mockDialog := &mockDialogAdapter{
		saveFileResult: serviceExportPath,
	}
	svc, _ := newIsolatedService(t, appservice.ServiceOptions{
		DialogAdapter: mockDialog,
	})

	scanRes, err := svc.ScanSources([]string{testdataDir})
	if err != nil {
		t.Fatalf("svc.ScanSources: %v", err)
	}
	if scanRes.Count != 6 {
		t.Fatalf("expected 6 testdata fixtures, got %d", scanRes.Count)
	}

	parseRes, err := svc.Parse(scanRes.Sources)
	if err != nil {
		t.Fatalf("svc.Parse: %v", err)
	}
	if len(parseRes.Records) != 45 {
		t.Fatalf("expected 45 parsed records from testdata, got %d", len(parseRes.Records))
	}

	savedFile, err := svc.SaveAs()
	if err != nil {
		t.Fatalf("svc.SaveAs: %v", err)
	}
	if savedFile.Cancelled {
		t.Fatal("expected SaveAs to succeed, got cancelled")
	}

	serviceCSVBytes, err := os.ReadFile(serviceExportPath)
	if err != nil {
		t.Fatalf("read service exported CSV: %v", err)
	}

	// 2. Direct engine + scanner reference path (CLI equivalent)
	sources, err := scanner.FindSources(testdataDir)
	if err != nil {
		t.Fatalf("scanner.FindSources: %v", err)
	}
	eng := engine.NewRuleEngine(config.DefaultRuleConfig())
	var expectedRecords []engine.Record
	for _, src := range sources {
		msg, err := scanner.LoadSource(src)
		if err != nil {
			t.Fatalf("LoadSource %s: %v", src.DisplayName, err)
		}
		recs := eng.ParseRecords(msg)
		expectedRecords = append(expectedRecords, recs...)
	}

	var cliBuf bytes.Buffer
	csvWriter := csv.NewWriter(&cliBuf)
	if err := csvWriter.Write(engine.CSVHeaders); err != nil {
		t.Fatalf("write headers: %v", err)
	}
	for _, rec := range expectedRecords {
		if err := csvWriter.Write(rec.ToRow()); err != nil {
			t.Fatalf("write row: %v", err)
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		t.Fatalf("flush cliBuf: %v", err)
	}

	if !bytes.Equal(serviceCSVBytes, cliBuf.Bytes()) {
		t.Errorf("Service exported CSV bytes do not match CLI/engine CSV bytes on testdata/")
	}

	// 3. Test on data/ directory if present
	dataDir := filepath.Join("..", "..", "data")
	if dataSources, err := scanner.FindSources(dataDir); err == nil && len(dataSources) > 0 {
		dataScanRes, err := svc.ScanSources([]string{dataDir})
		if err != nil {
			t.Fatalf("scan data/: %v", err)
		}
		_, err = svc.Parse(dataScanRes.Sources)
		if err != nil {
			t.Fatalf("parse data/: %v", err)
		}
		dataExportPath := filepath.Join(tmpDir, "service_data_export.csv")
		mockDialog.saveFileResult = dataExportPath
		_, err = svc.SaveAs()
		if err != nil {
			t.Fatalf("save data/: %v", err)
		}
		serviceDataBytes, err := os.ReadFile(dataExportPath)
		if err != nil {
			t.Fatalf("read service data export: %v", err)
		}

		var dataExpectedRecords []engine.Record
		for _, src := range dataSources {
			msg, err := scanner.LoadSource(src)
			if err != nil {
				t.Fatalf("LoadSource %s: %v", src.DisplayName, err)
			}
			recs := eng.ParseRecords(msg)
			dataExpectedRecords = append(dataExpectedRecords, recs...)
		}
		var cliDataBuf bytes.Buffer
		dataCSVWriter := csv.NewWriter(&cliDataBuf)
		if err := dataCSVWriter.Write(engine.CSVHeaders); err != nil {
			t.Fatalf("write headers: %v", err)
		}
		for _, rec := range dataExpectedRecords {
			if err := dataCSVWriter.Write(rec.ToRow()); err != nil {
				t.Fatalf("write row: %v", err)
			}
		}
		dataCSVWriter.Flush()
		if err := dataCSVWriter.Error(); err != nil {
			t.Fatalf("flush cliDataBuf: %v", err)
		}
		if !bytes.Equal(serviceDataBytes, cliDataBuf.Bytes()) {
			t.Errorf("Service exported CSV bytes do not match CLI/engine CSV bytes on data/")
		}
	}

	// 4. Exhaustive validation of testdata/msg_parsed.csv
	msgParsedCSVPath := filepath.Join(testdataDir, "msg_parsed.csv")
	csvFile, err := os.Open(msgParsedCSVPath)
	if err != nil {
		t.Skipf("sample CSV not found at %s: %v", msgParsedCSVPath, err)
	}
	defer csvFile.Close()

	reader := csv.NewReader(csvFile)
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read msg_parsed.csv: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least header + 1 data row, got %d", len(rows))
	}

	for colIdx, h := range engine.CSVHeaders {
		if rows[0][colIdx] != h {
			t.Errorf("column %d: got header %q, want %q", colIdx, rows[0][colIdx], h)
		}
	}

	goldenRecords := make([]engine.Record, 0, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 11 {
			t.Fatalf("row %d has %d columns, want 11", i, len(row))
		}
		rowNum, _ := strconv.Atoi(row[5])
		goldenRecords = append(goldenRecords, engine.Record{
			SourceFile:   row[0],
			Subject:      row[1],
			MessageDate:  row[2],
			ReportedAt:   row[3],
			Dispatcher:   row[4],
			RowInMessage: rowNum,
			RawEntry:     row[6],
			LocationHint: row[7],
			ParsedIssue:  row[8],
			Label:        row[9],
			IssueTime:    row[10],
		})
	}

	goldenExportPath := filepath.Join(tmpDir, "golden_reexport.csv")
	err = appservice.WriteRecordsToCSVForTest(goldenExportPath, goldenRecords)
	if err != nil {
		t.Fatalf("writeRecordsToCSV: %v", err)
	}

	reexportedFile, err := os.Open(goldenExportPath)
	if err != nil {
		t.Fatalf("open re-exported file: %v", err)
	}
	defer reexportedFile.Close()

	reexportedReader := csv.NewReader(reexportedFile)
	reexportedRows, err := reexportedReader.ReadAll()
	if err != nil {
		t.Fatalf("read reexported rows: %v", err)
	}

	if len(reexportedRows) != len(rows) {
		t.Fatalf("re-exported CSV row count mismatch: got %d, want %d", len(reexportedRows), len(rows))
	}

	for i := range rows {
		for j := range rows[i] {
			if reexportedRows[i][j] != rows[i][j] {
				t.Fatalf("row %d col %d mismatch: got %q, want %q", i, j, reexportedRows[i][j], rows[i][j])
			}
		}
	}

	// 5. Exhaustive classification check on all 1675 rows in msg_parsed.csv
	defaultCfg := config.DefaultRuleConfig()
	classifiedCount := 0
	statusTokens := []string{"NOT OUT", "NOT SVCD", "BLOCKED", "STILL IN", "ICEY"}
	for i := 1; i < len(rows); i++ {
		rawEntry := rows[i][6]
		if rawEntry == "" || parser.IsFooterLine(rawEntry) {
			continue
		}
		sandboxRes, err := svc.TestRule(rawEntry, defaultCfg)
		if err != nil {
			t.Fatalf("row %d TestRule(%q): %v", i+1, rawEntry, err)
		}
		hasStatusToken := false
		upperRaw := strings.ToUpper(rawEntry)
		for _, tok := range statusTokens {
			if strings.Contains(upperRaw, tok) {
				hasStatusToken = true
				break
			}
		}
		if hasStatusToken && sandboxRes.Label == "" {
			t.Errorf("row %d: empty label returned for entry with status token: %q", i+1, rawEntry)
		}
		classifiedCount++
	}
	if classifiedCount < 1000 {
		t.Errorf("expected to classify >= 1000 rows from msg_parsed.csv, got %d", classifiedCount)
	}
}

func TestSaveRules_OrderingAndConcurrency(t *testing.T) {
	svc, tmpConfigFile := newIsolatedService(t)

	// Verify initial state
	initialRules := svc.GetRules()
	if len(initialRules.Rules) == 0 {
		t.Fatal("expected non-empty initial rules")
	}

	// Test save -> persist -> engine swap ordering
	customCfg := config.DefaultRuleConfig()
	customLabel := config.LabelDefinition{
		Key:         "custom_rule_label",
		DisplayName: "Custom Rule Label",
		Metric:      config.MetricTrash,
	}
	customCfg.Labels = append(customCfg.Labels, customLabel)
	customRule := config.ClassificationRule{
		ID:          "custom_rule_ordering",
		Pattern:     "CUSTOM_SAVE_ORDERING_TEST",
		Type:        config.RuleTypeSubstring,
		Label:       "custom_rule_label",
		Description: "Rule to verify active engine swap",
		Enabled:     true,
	}
	customCfg.Rules = append([]config.ClassificationRule{customRule}, customCfg.Rules...)

	msgDate := time.Date(2026, 1, 5, 8, 30, 0, 0, time.UTC)
	mockLoader := func(src scanner.MessageSource) (parser.MessageMetadata, error) {
		return parser.MessageMetadata{
			SourceFile:  src.Path,
			Subject:     "Medford Tags",
			MessageDate: msgDate,
			Body:        "01/05/2026 08:30:00 Dave\n10 MAIN ST CUSTOM_SAVE_ORDERING_TEST",
		}, nil
	}

	svcWithLoader, _ := newIsolatedService(t, appservice.ServiceOptions{
		SourceLoader: mockLoader,
		ConfigSaver: func(cfg config.RuleConfig) error {
			return config.SaveConfigToPath(tmpConfigFile, cfg)
		},
		ConfigLoader: func() config.RuleConfig {
			cfg, err := config.LoadConfigFromPath(tmpConfigFile)
			if err != nil {
				return config.DefaultRuleConfig()
			}
			return cfg
		},
	})

	// Before saving custom rule: parses with default engine (not custom_rule_label)
	beforeRes, err := svcWithLoader.Parse([]scanner.MessageSource{{Path: "test.msg", DisplayName: "test.msg"}})
	if err != nil {
		t.Fatalf("Parse before save: %v", err)
	}
	if len(beforeRes.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(beforeRes.Records))
	}
	if beforeRes.Records[0].Label == "custom_rule_label" {
		t.Fatal("expected label to NOT be custom_rule_label before SaveRules")
	}

	// Save custom rule
	svcWithLoader.SetRulesDirty(true)
	if err := svcWithLoader.SaveRules(customCfg); err != nil {
		t.Fatalf("SaveRules failed: %v", err)
	}

	// 1. Verify dirty flag cleared
	if svcWithLoader.IsRulesDirty() {
		t.Error("expected dirty flag to be cleared after successful SaveRules")
	}

	// 2. Verify persisted config matches active engine
	persistedCfg, err := config.LoadConfigFromPath(tmpConfigFile)
	if err != nil {
		t.Fatalf("failed to read persisted config: %v", err)
	}
	if persistedCfg.DefaultLabel != customCfg.DefaultLabel {
		t.Errorf("persisted default label: got %q, want %q", persistedCfg.DefaultLabel, customCfg.DefaultLabel)
	}
	if len(persistedCfg.Rules) != len(customCfg.Rules) || persistedCfg.Rules[0].ID != "custom_rule_ordering" {
		t.Errorf("persisted rule mismatch: %+v", persistedCfg.Rules[0])
	}

	// 3. Verify in-memory engine swap: Parse now produces the custom label
	afterRes, err := svcWithLoader.Parse([]scanner.MessageSource{{Path: "test.msg", DisplayName: "test.msg"}})
	if err != nil {
		t.Fatalf("Parse after save: %v", err)
	}
	if len(afterRes.Records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(afterRes.Records))
	}
	if afterRes.Records[0].Label != "custom_rule_label" {
		t.Fatalf("expected record label %q, got %q (engine swap verification failed)", "custom_rule_label", afterRes.Records[0].Label)
	}

	// Concurrent SaveRules consistency test
	var mu sync.Mutex
	var concurrentStore config.RuleConfig
	concurrentSaver := func(cfg config.RuleConfig) error {
		mu.Lock()
		defer mu.Unlock()
		concurrentStore = cfg.Clone()
		return config.SaveConfigToPath(tmpConfigFile, cfg)
	}
	concurrentLoader := func() config.RuleConfig {
		mu.Lock()
		defer mu.Unlock()
		if concurrentStore.DefaultLabel != "" {
			return concurrentStore.Clone()
		}
		cfg, err := config.LoadConfigFromPath(tmpConfigFile)
		if err != nil {
			return config.DefaultRuleConfig()
		}
		return cfg
	}

	concurrentSvc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
		ConfigSaver:  concurrentSaver,
		ConfigLoader: concurrentLoader,
	})

	var wg sync.WaitGroup
	errCh := make(chan error, 20)

	for i := range 20 {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			workerCfg := config.DefaultRuleConfig()
			labelKey := fmt.Sprintf("label_%d", iteration)
			workerCfg.DefaultLabel = labelKey
			workerCfg.Labels = append(workerCfg.Labels, config.LabelDefinition{
				Key:         labelKey,
				DisplayName: labelKey,
				Metric:      config.MetricNone,
			})
			if err := concurrentSvc.SaveRules(workerCfg); err != nil {
				errCh <- fmt.Errorf("worker %d: %w", iteration, err)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent SaveRules error: %v", err)
	}

	// Ensure the engine is valid and accessible after concurrent writes
	currentRules := concurrentSvc.GetRules()
	if currentRules.DefaultLabel == "" {
		t.Error("expected valid default label after concurrent saves")
	}
	persistedFinal, err := config.LoadConfigFromPath(tmpConfigFile)
	if err != nil {
		t.Fatalf("failed to read persisted final config: %v", err)
	}
	if persistedFinal.DefaultLabel != currentRules.DefaultLabel {
		t.Errorf("persisted final default label (%q) != currentRules default label (%q)", persistedFinal.DefaultLabel, currentRules.DefaultLabel)
	}
}

func TestParse_NoRecordsAndAllSkipped(t *testing.T) {
	t.Run("All skipped sources", func(t *testing.T) {
		mockLoader := func(src scanner.MessageSource) (parser.MessageMetadata, error) {
			return parser.MessageMetadata{}, errors.New("corrupt msg format")
		}

		svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
			SourceLoader: mockLoader,
		})

		sources := []scanner.MessageSource{
			{Path: "bad1.msg", DisplayName: "bad1.msg"},
			{Path: "bad2.msg", DisplayName: "bad2.msg"},
		}

		res, err := svc.Parse(sources)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if res.Superseded {
			t.Error("expected superseded=false")
		}
		if len(res.Records) != 0 {
			t.Errorf("expected 0 records on all skipped, got %d", len(res.Records))
		}
		if len(res.Skipped) != 2 {
			t.Fatalf("expected 2 skipped sources, got %d", len(res.Skipped))
		}

		// Export must fail because lastRecords is empty
		_, err = svc.SaveToDownloads()
		if err == nil {
			t.Error("expected SaveToDownloads to fail when no records exist, got nil")
		}
		_, err = svc.SaveAs()
		if err == nil {
			t.Error("expected SaveAs to fail when no records exist, got nil")
		}
	})

	t.Run("Sources with zero structured records found", func(t *testing.T) {
		mockLoader := func(src scanner.MessageSource) (parser.MessageMetadata, error) {
			return parser.MessageMetadata{
				SourceFile:  src.Path,
				Subject:     "Empty Subject",
				MessageDate: time.Date(2026, 1, 5, 8, 30, 0, 0, time.UTC),
				Body:        "Only greetings here, no dispatch rows",
			}, nil
		}

		svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
			SourceLoader: mockLoader,
		})

		sources := []scanner.MessageSource{
			{Path: "empty1.msg", DisplayName: "empty1.msg"},
		}

		res, err := svc.Parse(sources)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(res.Records) != 0 {
			t.Errorf("expected 0 records, got %d", len(res.Records))
		}
		if len(res.Skipped) != 0 {
			t.Errorf("expected 0 skipped, got %d", len(res.Skipped))
		}

		// Export must fail
		_, err = svc.SaveToDownloads()
		if err == nil {
			t.Error("expected SaveToDownloads to fail on zero records")
		}
	})
}

func TestExport_WriterAndCloseFailures(t *testing.T) {
	mockLoader := func(src scanner.MessageSource) (parser.MessageMetadata, error) {
		return parser.MessageMetadata{
			SourceFile:  src.Path,
			Subject:     "Medford Tags",
			MessageDate: time.Date(2026, 1, 5, 8, 30, 0, 0, time.UTC),
			Body:        "01/05/2026 08:30:00 Dave\n10 MAIN ST TRASH NOT OUT",
		}, nil
	}

	t.Run("SaveToDownloads fails when downloadsDir locator returns error", func(t *testing.T) {
		svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
			SourceLoader: mockLoader,
			DownloadsDir: func() (string, error) {
				return "", errors.New("cannot locate downloads folder")
			},
		})

		_, err := svc.Parse([]scanner.MessageSource{{Path: "test.msg"}})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}

		_, err = svc.SaveToDownloads()
		if err == nil {
			t.Error("expected SaveToDownloads to fail when downloads directory fails, got nil")
		}
	})

	t.Run("SaveAs fails when destination path is unwritable", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create a directory where the file would be to cause open/create failure
		blockingDir := filepath.Join(tmpDir, "blocking_dir")
		if err := os.MkdirAll(blockingDir, 0o755); err != nil {
			t.Fatal(err)
		}

		mockDialog := &mockDialogAdapter{
			saveFileResult: blockingDir, // trying to create file with same name as existing dir
		}

		svc := appservice.NewServiceWithOptions(appservice.ServiceOptions{
			SourceLoader:  mockLoader,
			DialogAdapter: mockDialog,
		})

		_, err := svc.Parse([]scanner.MessageSource{{Path: "test.msg"}})
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}

		_, err = svc.SaveAs()
		if err == nil {
			t.Error("expected SaveAs to fail when writing to invalid target path, got nil")
		}
	})
}

func TestRulesRoundTrip_UserConfigFileCompatibility(t *testing.T) {
	// Test that an existing user rules.json config file with version: 1 and non-snake-case custom label keys
	// round-trips correctly without schema mutation or key changes.
	userJSON := `{
  "version": 1,
  "enable_heuristics": true,
  "default_label": "Other / Uncategorized",
  "labels": [
    {
      "key": "Other / Uncategorized",
      "name": "Other / Uncategorized",
      "metric": "none"
    },
    {
      "key": "SPECIAL-ITEM (Fridge & AC)",
      "name": "Special Item Fridge & AC",
      "metric": "both"
    }
  ],
  "rules": [
    {
      "id": "rule_special_item_1",
      "pattern": "FRIDGE NOT OUT",
      "type": "substring",
      "label": "SPECIAL-ITEM (Fridge & AC)",
      "enabled": true,
      "description": "Special item pickup"
    }
  ]
}`

	cfg, err := config.ParseRuleConfig([]byte(userJSON))
	if err != nil {
		t.Fatalf("ParseRuleConfig: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if cfg.DefaultLabel != "Other / Uncategorized" {
		t.Errorf("got default label %q", cfg.DefaultLabel)
	}
	if len(cfg.Labels) != 2 || cfg.Labels[1].Key != "SPECIAL-ITEM (Fridge & AC)" {
		t.Errorf("unexpected label keys: %+v", cfg.Labels)
	}

	// Validate using Service.ValidateRules
	svc := appservice.NewService()
	validationErrors := svc.ValidateRules(cfg)
	if len(validationErrors) != 0 {
		t.Fatalf("expected 0 validation errors for valid user config, got: %+v", validationErrors)
	}

	// Test engine creation and sandbox execution
	sandboxRes, err := svc.TestRule("10 MAIN ST FRIDGE NOT OUT", cfg)
	if err != nil {
		t.Fatalf("TestRule: %v", err)
	}
	if sandboxRes.Label != "SPECIAL-ITEM (Fridge & AC)" {
		t.Errorf("got label %q, want 'SPECIAL-ITEM (Fridge & AC)'", sandboxRes.Label)
	}
	if sandboxRes.Metric != config.MetricBoth {
		t.Errorf("got metric %q, want %q", sandboxRes.Metric, config.MetricBoth)
	}

	// Round-trip back to JSON
	marshaledJSON, err := cfg.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON: %v", err)
	}

	cfgRoundTripped, err := config.ParseRuleConfig(marshaledJSON)
	if err != nil {
		t.Fatalf("ParseRuleConfig round-tripped: %v", err)
	}
	if cfgRoundTripped.DefaultLabel != "Other / Uncategorized" {
		t.Errorf("got %q, want 'Other / Uncategorized'", cfgRoundTripped.DefaultLabel)
	}
	if cfgRoundTripped.Labels[1].Key != "SPECIAL-ITEM (Fridge & AC)" {
		t.Errorf("got %q, want 'SPECIAL-ITEM (Fridge & AC)'", cfgRoundTripped.Labels[1].Key)
	}
}
