package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultRuleConfig(t *testing.T) {
	cfg := DefaultRuleConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("DefaultRuleConfig() failed validation: %v", err)
	}

	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if !cfg.EnableHeuristics {
		t.Errorf("expected EnableHeuristics to be true")
	}
	if cfg.DefaultLabel != "other" {
		t.Errorf("expected DefaultLabel 'other', got %q", cfg.DefaultLabel)
	}
	if len(cfg.Labels) == 0 {
		t.Errorf("expected non-empty default labels")
	}
	if len(cfg.Rules) == 0 {
		t.Errorf("expected non-empty default rules")
	}
}

func TestRuleConfigClone(t *testing.T) {
	orig := DefaultRuleConfig()
	cloned := orig.Clone()

	if !reflect.DeepEqual(orig, cloned) {
		t.Fatalf("cloned config does not match original")
	}

	cloned.Rules[0].Pattern = "MODIFIED PATTERN"
	if orig.Rules[0].Pattern == "MODIFIED PATTERN" {
		t.Errorf("mutating cloned rule affected original")
	}

	cloned.Labels[0].DisplayName = "MODIFIED DISPLAY"
	if orig.Labels[0].DisplayName == "MODIFIED DISPLAY" {
		t.Errorf("mutating cloned label affected original")
	}
}

func TestRuleConfigJSONRoundTrip(t *testing.T) {
	orig := DefaultRuleConfig()
	data, err := orig.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	parsed, err := ParseRuleConfig(data)
	if err != nil {
		t.Fatalf("ParseRuleConfig() error: %v", err)
	}

	if !reflect.DeepEqual(orig, parsed) {
		t.Errorf("round-trip config mismatch")
	}
}

func TestRuleConfigValidateErrors(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(cfg *RuleConfig)
		wantErr bool
	}{
		{
			name: "empty default label",
			modify: func(cfg *RuleConfig) {
				cfg.DefaultLabel = ""
			},
			wantErr: true,
		},
		{
			name: "empty rule ID",
			modify: func(cfg *RuleConfig) {
				cfg.Rules[0].ID = ""
			},
			wantErr: true,
		},
		{
			name: "duplicate rule ID",
			modify: func(cfg *RuleConfig) {
				cfg.Rules[1].ID = cfg.Rules[0].ID
			},
			wantErr: true,
		},
		{
			name: "empty pattern",
			modify: func(cfg *RuleConfig) {
				cfg.Rules[0].Pattern = "   "
			},
			wantErr: true,
		},
		{
			name: "unknown rule type",
			modify: func(cfg *RuleConfig) {
				cfg.Rules[0].Type = "invalid_type"
			},
			wantErr: true,
		},
		{
			name: "invalid regex pattern",
			modify: func(cfg *RuleConfig) {
				cfg.Rules[0].Type = RuleTypeRegex
				cfg.Rules[0].Pattern = "[unclosed"
			},
			wantErr: true,
		},
		{
			name: "empty label key",
			modify: func(cfg *RuleConfig) {
				cfg.Labels[0].Key = ""
			},
			wantErr: true,
		},
		{
			name: "duplicate label key",
			modify: func(cfg *RuleConfig) {
				cfg.Labels[1].Key = cfg.Labels[0].Key
			},
			wantErr: true,
		},
		{
			name: "invalid label metric",
			modify: func(cfg *RuleConfig) {
				cfg.Labels[0].Metric = "invalid_metric"
			},
			wantErr: true,
		},
		{
			name: "empty rule target label",
			modify: func(cfg *RuleConfig) {
				cfg.Rules[0].Label = "  "
			},
			wantErr: true,
		},
		{
			name: "undefined default label",
			modify: func(cfg *RuleConfig) {
				cfg.DefaultLabel = "non_existent_label"
			},
			wantErr: true,
		},
		{
			name: "undefined rule label",
			modify: func(cfg *RuleConfig) {
				cfg.Rules[0].Label = "non_existent_label"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultRuleConfig()
			tt.modify(&cfg)
			err := cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDefaultConfigDirAndPath(t *testing.T) {
	dir, err := DefaultConfigDir()
	if err != nil {
		t.Fatalf("DefaultConfigDir() returned error: %v", err)
	}
	if dir == "" {
		t.Errorf("expected non-empty config directory")
	}
	if !strings.Contains(dir, ConfigDirName) {
		t.Errorf("expected config directory to contain %q, got %q", ConfigDirName, dir)
	}

	path, err := DefaultConfigPath()
	if err != nil {
		t.Fatalf("DefaultConfigPath() returned error: %v", err)
	}
	if filepath.Base(path) != ConfigFileName {
		t.Errorf("expected config filename %q, got %q", ConfigFileName, filepath.Base(path))
	}
}

func TestLoadConfigFromPathNonExistent(t *testing.T) {
	nonExistentPath := filepath.Join(t.TempDir(), "sub", "non_existent_rules.json")
	cfg, err := LoadConfigFromPath(nonExistentPath)
	if err != nil {
		t.Fatalf("expected nil error for non-existent file, got: %v", err)
	}

	defaultCfg := DefaultRuleConfig()
	if len(cfg.Rules) != len(defaultCfg.Rules) {
		t.Errorf("expected fallback default rules count %d, got %d", len(defaultCfg.Rules), len(cfg.Rules))
	}
}

func TestSaveAndLoadConfigRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "will-msg", "custom_rules.json")

	customCfg := RuleConfig{
		Version:          1,
		EnableHeuristics: false,
		DefaultLabel:     "unclassified",
		Labels: []LabelDefinition{
			{Key: "custom_label", DisplayName: "Custom Issue", Metric: MetricTrash},
			{Key: "unclassified", DisplayName: "Unclassified", Metric: MetricNone},
		},
		Rules: []ClassificationRule{
			{
				ID:          "custom_1",
				Pattern:     "SPECIAL RECYCLE",
				Type:        RuleTypeSubstring,
				Label:       "custom_label",
				Description: "Custom recycled issue",
				Enabled:     true,
			},
		},
	}

	if err := SaveConfigToPath(tmpPath, customCfg); err != nil {
		t.Fatalf("SaveConfigToPath() failed: %v", err)
	}

	loadedCfg, err := LoadConfigFromPath(tmpPath)
	if err != nil {
		t.Fatalf("LoadConfigFromPath() failed: %v", err)
	}

	if !reflect.DeepEqual(customCfg, loadedCfg) {
		t.Errorf("loaded config does not match saved config.\nGot: %+v\nWant: %+v", loadedCfg, customCfg)
	}
}

func TestSaveConfigInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "rules.json")

	invalidCfg := RuleConfig{
		Version:      1,
		DefaultLabel: "", // invalid: empty default label
	}

	if err := SaveConfigToPath(tmpPath, invalidCfg); err == nil {
		t.Fatalf("expected error when saving invalid config, got nil")
	}

	// Ensure file was not created
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("expected invalid config not to create file on disk")
	}
}

func TestSaveConfigPreservesExistingFileOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "rules.json")

	initialCfg := DefaultRuleConfig()
	if err := SaveConfigToPath(filePath, initialCfg); err != nil {
		t.Fatalf("initial SaveConfigToPath failed: %v", err)
	}

	initialContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read initial config: %v", err)
	}

	// Test 1: Validation failure preserves existing file
	invalidCfg := RuleConfig{
		Version:      1,
		DefaultLabel: "", // invalid
	}
	if err := SaveConfigToPath(filePath, invalidCfg); err == nil {
		t.Fatalf("expected error saving invalid config, got nil")
	}

	afterContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read config after failed validation save: %v", err)
	}
	if !bytes.Equal(initialContent, afterContent) {
		t.Errorf("expected existing config to be preserved on validation failure")
	}

	// Test 2: Forced Rename failure after temp file creation
	var renameCalled bool
	oldRename := renameFile
	renameFile = func(oldpath, newpath string) error {
		renameCalled = true
		// Verify the temp file exists at the time rename is called
		if _, statErr := os.Stat(oldpath); statErr != nil {
			t.Errorf("expected temp file %s to exist before rename: %v", oldpath, statErr)
		}
		return errors.New("simulated rename failure")
	}
	defer func() {
		renameFile = oldRename
	}()

	validModifiedCfg := initialCfg.Clone()
	validModifiedCfg.Rules = nil // valid empty rules

	err = SaveConfigToPath(filePath, validModifiedCfg)
	if err == nil {
		t.Fatalf("expected error on simulated rename failure, got nil")
	}
	if !strings.Contains(err.Error(), "replace config file") {
		t.Errorf("expected error to contain 'replace config file', got: %v", err)
	}
	if !renameCalled {
		t.Errorf("expected renameFile to be invoked")
	}

	// Verify original file contents are still intact and preserved
	afterRenameFailContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read config after rename failure: %v", err)
	}
	if !bytes.Equal(initialContent, afterRenameFailContent) {
		t.Errorf("expected existing config to remain completely intact after rename failure")
	}

	// Verify no temporary files were leaked in tmpDir
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read tmpDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "rules-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leaked temporary file found: %s", e.Name())
		}
	}
}

func TestLoadConfigCorruptedRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	corruptPath := filepath.Join(tmpDir, "rules.json")

	corruptJSON := []byte(`{ "version": 1, "rules": [ { "id": "bad_rule", "type": "regex", "pattern": "[incomplete" } ] }`)
	if err := os.WriteFile(corruptPath, corruptJSON, 0644); err != nil {
		t.Fatalf("failed to write test corrupt file: %v", err)
	}

	cfg, err := LoadConfigFromPath(corruptPath)
	if err == nil {
		t.Fatalf("expected error when loading corrupt config, got nil")
	}

	// Must return valid default config despite the corruption
	if len(cfg.Rules) != len(DefaultRuleConfig().Rules) {
		t.Errorf("expected fallback default config rules count %d, got %d", len(DefaultRuleConfig().Rules), len(cfg.Rules))
	}

	// Must create a backup file with the corrupted contents
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}

	foundBackup := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "rules.json.corrupted.") {
			foundBackup = true
			backupData, readErr := os.ReadFile(filepath.Join(tmpDir, e.Name()))
			if readErr != nil {
				t.Fatalf("failed to read backup file: %v", readErr)
			}
			if !bytes.Equal(backupData, corruptJSON) {
				t.Errorf("backup file content mismatch")
			}
			break
		}
	}

	if !foundBackup {
		t.Errorf("expected backup file matching rules.json.corrupted.* to be created")
	}
}

func TestMigrateConfig(t *testing.T) {
	legacyCfg := RuleConfig{
		Version:          0, // legacy version
		DefaultLabel:     "",
		EnableHeuristics: true,
		Labels: []LabelDefinition{
			{Key: "custom_key", DisplayName: "Custom Key", Metric: ""},
		},
		Rules: []ClassificationRule{
			{ID: "r1", Pattern: "TEST", Type: RuleTypeSubstring, Label: "custom_key", Enabled: true},
		},
	}

	migrated := MigrateConfig(legacyCfg)

	if migrated.Version != CurrentConfigVersion {
		t.Errorf("expected version %d, got %d", CurrentConfigVersion, migrated.Version)
	}
	if migrated.DefaultLabel != "other" {
		t.Errorf("expected default label 'other', got %q", migrated.DefaultLabel)
	}
	if migrated.Labels[0].Metric != MetricNone {
		t.Errorf("expected empty metric to default to %q, got %q", MetricNone, migrated.Labels[0].Metric)
	}

	emptyLabelsCfg := RuleConfig{
		Version:      1,
		DefaultLabel: "other",
		Labels:       nil,
	}
	migratedEmpty := MigrateConfig(emptyLabelsCfg)
	if len(migratedEmpty.Labels) != len(DefaultLabels()) {
		t.Errorf("expected empty labels to be populated with defaults, got %d labels", len(migratedEmpty.Labels))
	}
}

func TestLoadConfigLegacyAndPartialJSON(t *testing.T) {
	tmpDir := t.TempDir()
	legacyPath := filepath.Join(tmpDir, "legacy_rules.json")

	// JSON with no version, no default_label, and no labels array
	legacyJSON := []byte(`{
		"rules": [
			{
				"id": "legacy_1",
				"pattern": "CUSTOM TRASH",
				"type": "substring",
				"label": "msw_not_out",
				"enabled": true
			}
		]
	}`)

	if err := os.WriteFile(legacyPath, legacyJSON, 0644); err != nil {
		t.Fatalf("failed to write test legacy file: %v", err)
	}

	cfg, err := LoadConfigFromPath(legacyPath)
	if err != nil {
		t.Fatalf("LoadConfigFromPath failed on valid legacy JSON: %v", err)
	}

	if cfg.Version != CurrentConfigVersion {
		t.Errorf("expected version to migrate to %d, got %d", CurrentConfigVersion, cfg.Version)
	}
	if cfg.DefaultLabel != "other" {
		t.Errorf("expected default_label to default to 'other', got %q", cfg.DefaultLabel)
	}
	if len(cfg.Labels) == 0 {
		t.Errorf("expected labels to be populated with default labels")
	}
	if len(cfg.Rules) != 1 || cfg.Rules[0].ID != "legacy_1" {
		t.Errorf("expected legacy rule to be preserved, got: %+v", cfg.Rules)
	}

	// Verify no corrupt backup was made
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "corrupted") {
			t.Errorf("unexpected corrupt backup file created: %s", e.Name())
		}
	}
}

func TestExportAndImportConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	exportPath := filepath.Join(tmpDir, "exported_rules.json")

	cfg := DefaultRuleConfig()
	cfg.Rules = append(cfg.Rules, ClassificationRule{
		ID:      "extra_rule",
		Pattern: "EXTRA PATTERN",
		Type:    RuleTypeSubstring,
		Label:   "other",
		Enabled: true,
	})

	if err := ExportConfigFile(exportPath, cfg); err != nil {
		t.Fatalf("ExportConfigFile() failed: %v", err)
	}

	importedCfg, err := ImportConfigFile(exportPath)
	if err != nil {
		t.Fatalf("ImportConfigFile() failed: %v", err)
	}

	if len(importedCfg.Rules) != len(cfg.Rules) {
		t.Fatalf("imported rules count mismatch: got %d, want %d", len(importedCfg.Rules), len(cfg.Rules))
	}
	if importedCfg.Rules[len(importedCfg.Rules)-1].ID != "extra_rule" {
		t.Errorf("imported rule mismatch: got %q", importedCfg.Rules[len(importedCfg.Rules)-1].ID)
	}
}

func TestExportAndImportConfigStreams(t *testing.T) {
	cfg := DefaultRuleConfig()

	var buf bytes.Buffer
	if err := ExportConfig(&buf, cfg); err != nil {
		t.Fatalf("ExportConfig() failed: %v", err)
	}

	importedCfg, err := ImportConfig(&buf)
	if err != nil {
		t.Fatalf("ImportConfig() failed: %v", err)
	}

	if !reflect.DeepEqual(cfg, importedCfg) {
		t.Errorf("stream imported config does not match original")
	}
}

func TestImportConfigValidationFailure(t *testing.T) {
	testCases := []struct {
		name string
		json string
	}{
		{
			name: "invalid regex",
			json: `{ "version": 1, "default_label": "other", "labels": [{"key": "other", "display_name": "Other", "metric": "none"}], "rules": [ { "id": "r1", "pattern": "[invalid(", "type": "regex", "label": "other" } ] }`,
		},
		{
			name: "invalid metric contribution typo",
			json: `{ "version": 1, "default_label": "other", "labels": [{"key": "other", "display_name": "Other", "metric": "trashh"}], "rules": [ { "id": "r1", "pattern": "TRASH", "type": "substring", "label": "other" } ] }`,
		},
		{
			name: "rule references undefined label",
			json: `{ "version": 1, "default_label": "other", "labels": [{"key": "other", "display_name": "Other", "metric": "none"}], "rules": [ { "id": "r1", "pattern": "TRASH", "type": "substring", "label": "missing_label" } ] }`,
		},
		{
			name: "default_label references undefined label",
			json: `{ "version": 1, "default_label": "missing_default", "labels": [{"key": "other", "display_name": "Other", "metric": "none"}], "rules": [ { "id": "r1", "pattern": "TRASH", "type": "substring", "label": "other" } ] }`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ImportConfig(strings.NewReader(tc.json))
			if err == nil {
				t.Fatalf("expected error on invalid import config (%s), got nil", tc.name)
			}

			tmpDir := t.TempDir()
			invalidFilePath := filepath.Join(tmpDir, "invalid.json")
			_ = os.WriteFile(invalidFilePath, []byte(tc.json), 0644)

			_, err = ImportConfigFile(invalidFilePath)
			if err == nil {
				t.Fatalf("expected error on invalid import file (%s), got nil", tc.name)
			}
		})
	}
}
