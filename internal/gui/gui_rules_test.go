package gui

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"will-msg/internal/config"
	"will-msg/internal/engine"
	"will-msg/internal/parser"
)

func writeTestCSV(path string, records []engine.Record) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write(engine.CSVHeaders); err != nil {
		return err
	}
	for _, rec := range records {
		if err := w.Write(rec.ToRow()); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func TestRuleManagerView_Initialization(t *testing.T) {
	a := test.NewApp()
	w := test.NewWindow(widget.NewLabel("Test"))
	defer w.Close()

	view := NewRuleManagerView(a, w, nil)
	if view == nil {
		t.Fatal("expected NewRuleManagerView to return non-nil view")
	}

	ui := view.BuildUI()
	if ui == nil {
		t.Fatal("expected BuildUI to return non-nil CanvasObject")
	}

	if len(view.workingConfig.Rules) == 0 {
		t.Errorf("expected workingConfig to have rules initialized, got 0")
	}

	if view.selectedIndex != -1 {
		t.Errorf("expected initial selectedIndex -1, got %d", view.selectedIndex)
	}

	if view.editBtn.Disabled() != true {
		t.Errorf("expected editBtn to be disabled initially")
	}
	if view.deleteBtn.Disabled() != true {
		t.Errorf("expected deleteBtn to be disabled initially")
	}
	if view.moveUpBtn.Disabled() != true {
		t.Errorf("expected moveUpBtn to be disabled initially")
	}
	if view.moveDownBtn.Disabled() != true {
		t.Errorf("expected moveDownBtn to be disabled initially")
	}
}

func TestRuleManagerView_SelectionAndReordering(t *testing.T) {
	a := test.NewApp()
	w := test.NewWindow(widget.NewLabel("Test"))
	defer w.Close()

	view := NewRuleManagerView(a, w, nil)
	view.BuildUI()

	if len(view.workingConfig.Rules) < 2 {
		t.Fatalf("need at least 2 rules for reordering test")
	}

	rule0 := view.workingConfig.Rules[0]
	rule1 := view.workingConfig.Rules[1]

	// Select rule 0 (row 1 in table)
	view.table.OnSelected(widget.TableCellID{Row: 1, Col: 0})
	if view.selectedIndex != 0 {
		t.Fatalf("expected selectedIndex 0, got %d", view.selectedIndex)
	}

	if view.moveUpBtn.Disabled() != true {
		t.Errorf("expected moveUpBtn to be disabled for first item")
	}
	if view.moveDownBtn.Disabled() != false {
		t.Errorf("expected moveDownBtn to be enabled for first item")
	}

	// Move down
	view.moveSelectedRule(1)
	if view.selectedIndex != 1 {
		t.Errorf("expected selectedIndex 1 after moveDown, got %d", view.selectedIndex)
	}
	if view.workingConfig.Rules[0].ID != rule1.ID || view.workingConfig.Rules[1].ID != rule0.ID {
		t.Errorf("rules not swapped correctly after moveDown")
	}

	// Move up
	view.moveSelectedRule(-1)
	if view.selectedIndex != 0 {
		t.Errorf("expected selectedIndex 0 after moveUp, got %d", view.selectedIndex)
	}
	if view.workingConfig.Rules[0].ID != rule0.ID || view.workingConfig.Rules[1].ID != rule1.ID {
		t.Errorf("rules not swapped back after moveUp")
	}
}

func TestRuleManagerView_ToggleAndEditRule(t *testing.T) {
	a := test.NewApp()
	w := test.NewWindow(widget.NewLabel("Test"))
	defer w.Close()

	view := NewRuleManagerView(a, w, nil)
	view.BuildUI()

	view.selectedIndex = 0
	originalState := view.workingConfig.Rules[0].Enabled

	view.toggleSelectedRuleEnabled()
	if view.workingConfig.Rules[0].Enabled == originalState {
		t.Errorf("expected enabled state to toggle from %v to %v", originalState, !originalState)
	}

	view.toggleSelectedRuleEnabled()
	if view.workingConfig.Rules[0].Enabled != originalState {
		t.Errorf("expected enabled state to toggle back to %v", originalState)
	}
}

func TestRuleManagerView_LiveSandboxExecution(t *testing.T) {
	a := test.NewApp()
	w := test.NewWindow(widget.NewLabel("Test"))
	defer w.Close()

	view := NewRuleManagerView(a, w, nil)
	view.BuildUI()

	// 1. Standard matching
	view.sandboxInput.SetText("45 FOREST ST TRASH AND RCY NOT OUT 0830AM")
	view.runSandbox()

	if view.sandboxAddressLabel.Text != "45 FOREST ST" {
		t.Errorf("sandbox expected address '45 FOREST ST', got %q", view.sandboxAddressLabel.Text)
	}
	if view.sandboxStatusLabel.Text != "TRASH AND RCY NOT OUT" {
		t.Errorf("sandbox expected status 'TRASH AND RCY NOT OUT', got %q", view.sandboxStatusLabel.Text)
	}
	if view.sandboxLabelLabel.Text != "msw_and_recyc_not_out" {
		t.Errorf("sandbox expected label 'msw_and_recyc_not_out', got %q", view.sandboxLabelLabel.Text)
	}
	if !strings.Contains(view.sandboxMatchedRule.Text, "Rule #") {
		t.Errorf("sandbox expected matched rule description, got %q", view.sandboxMatchedRule.Text)
	}
	if view.sandboxMetricLabel.Text != "Both (Trash + Recycling)" {
		t.Errorf("sandbox expected metric 'Both (Trash + Recycling)', got %q", view.sandboxMetricLabel.Text)
	}

	// 2. Dynamic heuristic
	view.sandboxInput.SetText("10 HIGH ST MATTRESS NOT OUT")
	view.runSandbox()

	if view.sandboxLabelLabel.Text != "special_item_not_out" {
		t.Errorf("sandbox expected heuristic label 'special_item_not_out', got %q", view.sandboxLabelLabel.Text)
	}
	if !strings.Contains(view.sandboxMatchedRule.Text, "Heuristic") {
		t.Errorf("sandbox expected heuristic note, got %q", view.sandboxMatchedRule.Text)
	}

	// 3. Fallback when heuristics disabled
	view.workingConfig.EnableHeuristics = false
	view.runSandbox()

	if view.sandboxLabelLabel.Text != "other" {
		t.Errorf("sandbox expected fallback 'other' when heuristics disabled, got %q", view.sandboxLabelLabel.Text)
	}

	// 4. Empty input
	view.sandboxInput.SetText("")
	view.runSandbox()
	if view.sandboxAddressLabel.Text != "-" {
		t.Errorf("sandbox expected '-' on empty input, got %q", view.sandboxAddressLabel.Text)
	}
}

func TestRuleManager_CustomRuleAndPrecedenceIntegration(t *testing.T) {
	// Isolate user config directory and engine for this test
	origEngine := DefaultEngine()
	defer SetDefaultEngine(origEngine)

	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)
	t.Setenv("APPDATA", tempDir)
	configPath := filepath.Join(tempDir, "isolated_rules.json")

	a := test.NewApp()
	w := test.NewWindow(widget.NewLabel("Test"))
	defer w.Close()

	// Initial parse with default engine
	meta := parser.MessageMetadata{
		Subject: "Tags 01/02/26",
		Body:    "09/22/2025 11:29:05 SSAWALLI\n123 MAIN ST CARPET NOT OUT\n45 ELM ST MSW AND RECYC NOT OUT\n",
	}

	initialRecords := DefaultEngine().ParseRecords(meta)
	if len(initialRecords) != 2 {
		t.Fatalf("expected 2 initial records, got %d", len(initialRecords))
	}
	// "CARPET NOT OUT" hits dynamic heuristic special_item_not_out initially
	if initialRecords[0].Label != "special_item_not_out" {
		t.Errorf("expected initial label 'special_item_not_out', got %q", initialRecords[0].Label)
	}

	// Step 1: Add a new custom rule with custom label using isolated config path
	view := NewRuleManagerViewWithPath(a, w, configPath, nil)
	view.BuildUI()

	newLabel := config.LabelDefinition{
		Key:         "carpet_special",
		DisplayName: "Carpet Waste",
		Metric:      config.MetricTrash,
	}
	view.workingConfig.Labels = append(view.workingConfig.Labels, newLabel)

	customRule := config.ClassificationRule{
		ID:          "custom_carpet_1",
		Pattern:     "CARPET NOT OUT",
		Type:        config.RuleTypeSubstring,
		Label:       "carpet_special",
		Description: "Carpet pickup rule",
		Enabled:     true,
	}
	// Insert at top of rules (precedence 1)
	view.workingConfig.Rules = append([]config.ClassificationRule{customRule}, view.workingConfig.Rules...)
	view.refreshUI()

	// Step 2: Save & Apply
	if err := view.ApplyChanges(); err != nil {
		t.Fatalf("failed to apply changes: %v", err)
	}

	// Verify parsed records now reflect the custom rule and label
	updatedRecords := DefaultEngine().ParseRecords(meta)
	if len(updatedRecords) != 2 {
		t.Fatalf("expected 2 records after save, got %d", len(updatedRecords))
	}
	if updatedRecords[0].Label != "carpet_special" {
		t.Errorf("expected custom label 'carpet_special', got %q", updatedRecords[0].Label)
	}

	// Verify writing to CSV contains the updated label
	csvPath := filepath.Join(tempDir, "test_output.csv")
	if err := writeTestCSV(csvPath, updatedRecords); err != nil {
		t.Fatalf("failed to write CSV: %v", err)
	}
	csvData, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("failed to read written CSV: %v", err)
	}
	if !strings.Contains(string(csvData), "carpet_special") {
		t.Errorf("expected CSV content to contain 'carpet_special', got:\n%s", string(csvData))
	}

	// Step 3: Test Reordering Precedence
	// Create another rule that matches "CARPET" generally
	generalCarpetRule := config.ClassificationRule{
		ID:          "general_carpet_1",
		Pattern:     "CARPET",
		Type:        config.RuleTypeSubstring,
		Label:       "other",
		Description: "General carpet",
		Enabled:     true,
	}

	// Place general rule ahead of specific rule
	view.workingConfig.Rules = append([]config.ClassificationRule{generalCarpetRule}, view.workingConfig.Rules...)
	if err := view.ApplyChanges(); err != nil {
		t.Fatalf("failed to apply changes: %v", err)
	}

	reorderedRecords := DefaultEngine().ParseRecords(meta)
	if reorderedRecords[0].Label != "other" {
		t.Errorf("expected general rule to take precedence when placed first, got %q", reorderedRecords[0].Label)
	}

	// Step 4: Reset to Defaults
	view.workingConfig = config.DefaultRuleConfig()
	if err := view.ApplyChanges(); err != nil {
		t.Fatalf("failed to apply changes: %v", err)
	}

	resetRecords := DefaultEngine().ParseRecords(meta)
	if resetRecords[0].Label != "special_item_not_out" {
		t.Errorf("expected reset to restore 'special_item_not_out', got %q", resetRecords[0].Label)
	}
}

func TestShowRuleEditorWindow_Singleton(t *testing.T) {
	activeRuleEditorWindow = nil
	a := test.NewApp()
	parent := test.NewWindow(widget.NewLabel("Parent"))
	defer parent.Close()

	w1 := ShowRuleEditorWindow(a, parent, nil)
	if w1 == nil {
		t.Fatal("expected ShowRuleEditorWindow to return window")
	}
	defer func() {
		activeRuleEditorWindow = nil
		w1.Close()
	}()

	w2 := ShowRuleEditorWindow(a, parent, nil)
	if w1 != w2 {
		t.Errorf("expected singleton window instance, got different windows")
	}
}

func TestRuleManager_ExportAndImportRoundTrip(t *testing.T) {
	tempDir := t.TempDir()
	exportPath := filepath.Join(tempDir, "exported_rules.json")

	cfg := config.DefaultRuleConfig()
	cfg.Rules = append([]config.ClassificationRule{
		{
			ID:      "test_rule_999",
			Pattern: "CUSTOM PATTERN FOR IMPORT",
			Type:    config.RuleTypeSubstring,
			Label:   "other",
			Enabled: true,
		},
	}, cfg.Rules...)

	err := config.ExportConfigFile(exportPath, cfg)
	if err != nil {
		t.Fatalf("failed to export config: %v", err)
	}

	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read exported file: %v", err)
	}

	imported, err := config.ParseRuleConfig(data)
	if err != nil {
		t.Fatalf("failed to parse exported file: %v", err)
	}

	if len(imported.Rules) != len(cfg.Rules) {
		t.Errorf("expected %d rules, got %d", len(cfg.Rules), len(imported.Rules))
	}
	if imported.Rules[0].ID != "test_rule_999" {
		t.Errorf("expected first rule ID 'test_rule_999', got %q", imported.Rules[0].ID)
	}
}
