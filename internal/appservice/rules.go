package appservice

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"will-msg/internal/config"
	"will-msg/internal/engine"
)

// GetRules returns the active rule configuration from disk or default rules.
func (s *Service) GetRules() config.RuleConfig {
	return s.loadConfig()
}

// GetDefaultRules returns a fresh default rule configuration.
func (s *Service) GetDefaultRules() config.RuleConfig {
	return config.DefaultRuleConfig()
}

// ValidateRules performs authoritative validation of the configuration and returns JSON Pointer error paths.
func (s *Service) ValidateRules(cfg config.RuleConfig) []ValidationError {
	errors := make([]ValidationError, 0)

	if strings.TrimSpace(cfg.DefaultLabel) == "" {
		errors = append(errors, ValidationError{
			Path:    "/default_label",
			Message: "Default label cannot be empty",
		})
	}

	seenLabels := make(map[string]bool)
	for i, lbl := range cfg.Labels {
		trimmedKey := strings.TrimSpace(lbl.Key)
		if trimmedKey == "" {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/labels/%d/key", i),
				Message: fmt.Sprintf("Label #%d: key cannot be empty", i+1),
			})
		} else if seenLabels[lbl.Key] {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/labels/%d/key", i),
				Message: fmt.Sprintf("Duplicate label key %q", lbl.Key),
			})
		}
		if trimmedKey != "" {
			seenLabels[lbl.Key] = true
		}

		switch lbl.Metric {
		case config.MetricNone, config.MetricTrash, config.MetricRecycling, config.MetricBoth:
			// valid metric
		default:
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/labels/%d/metric", i),
				Message: fmt.Sprintf("Invalid metric contribution %q", lbl.Metric),
			})
		}
	}

	if cfg.DefaultLabel != "" && !seenLabels[cfg.DefaultLabel] {
		errors = append(errors, ValidationError{
			Path:    "/default_label",
			Message: fmt.Sprintf("Default label %q is not defined in labels list", cfg.DefaultLabel),
		})
	}

	seenIDs := make(map[string]bool)
	for i, rule := range cfg.Rules {
		trimmedID := strings.TrimSpace(rule.ID)
		if trimmedID == "" {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/rules/%d/id", i),
				Message: fmt.Sprintf("Rule #%d: ID cannot be empty", i+1),
			})
		} else if seenIDs[rule.ID] {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/rules/%d/id", i),
				Message: fmt.Sprintf("Duplicate rule ID %q", rule.ID),
			})
		}
		if trimmedID != "" {
			seenIDs[rule.ID] = true
		}

		if strings.TrimSpace(rule.Pattern) == "" {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/rules/%d/pattern", i),
				Message: "Pattern cannot be empty",
			})
		}

		trimmedLabel := strings.TrimSpace(rule.Label)
		if trimmedLabel == "" {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/rules/%d/label", i),
				Message: "Target label cannot be empty",
			})
		} else if !seenLabels[rule.Label] {
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/rules/%d/label", i),
				Message: fmt.Sprintf("Target label %q is not defined in labels list", rule.Label),
			})
		}

		switch rule.Type {
		case config.RuleTypeSubstring:
			// valid
		case config.RuleTypeRegex:
			if strings.TrimSpace(rule.Pattern) != "" {
				if _, err := config.CompileRegexPattern(rule.Pattern); err != nil {
					errors = append(errors, ValidationError{
						Path:    fmt.Sprintf("/rules/%d/pattern", i),
						Message: fmt.Sprintf("Invalid regex pattern: %v", err),
					})
				}
			}
		default:
			errors = append(errors, ValidationError{
				Path:    fmt.Sprintf("/rules/%d/type", i),
				Message: fmt.Sprintf("Unknown rule type %q", rule.Type),
			})
		}
	}

	return errors
}

// SaveRules validates, persists, swaps the active rule engine, and clears the rules-dirty flag under lock.
func (s *Service) SaveRules(cfg config.RuleConfig) error {
	s.rulesMu.Lock()
	defer s.rulesMu.Unlock()

	eng, err := engine.NewRuleEngineValidated(cfg)
	if err != nil {
		return err
	}

	if err := s.saveConfig(cfg); err != nil {
		return err
	}

	s.engine.Store(eng)
	s.rulesDirty.Store(false)
	return nil
}

// ImportRules opens a file dialog to select a JSON rule configuration file and parses it without saving.
func (s *Service) ImportRules() (ImportRulesResult, error) {
	path, err := s.dialogAdapter.OpenFileDialog(s.context(), wailsruntime.OpenDialogOptions{
		Title: "Import Rules Configuration",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return ImportRulesResult{}, err
	}
	if path == "" {
		return ImportRulesResult{Cancelled: true}, nil
	}

	cfg, err := config.ImportConfigFile(path)
	if err != nil {
		return ImportRulesResult{}, err
	}

	return ImportRulesResult{
		Config:    &cfg,
		Cancelled: false,
	}, nil
}

// ExportRules validates and writes the provided configuration to a selected JSON file.
func (s *Service) ExportRules(cfg config.RuleConfig) (SavedFile, error) {
	valErrors := s.ValidateRules(cfg)
	if len(valErrors) > 0 {
		return SavedFile{}, fmt.Errorf("cannot export invalid configuration: %s", valErrors[0].Message)
	}

	path, err := s.dialogAdapter.SaveFileDialog(s.context(), wailsruntime.SaveDialogOptions{
		Title:           "Export Rules Configuration",
		DefaultFilename: "rules.json",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "JSON Files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return SavedFile{}, err
	}
	if path == "" {
		return SavedFile{Cancelled: true}, nil
	}

	data, err := cfg.ToJSON()
	if err != nil {
		return SavedFile{}, err
	}

	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return SavedFile{}, err
		}
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return SavedFile{}, err
	}

	return SavedFile{
		Filename:  filepath.Base(path),
		Dir:       filepath.Dir(path),
		Path:      path,
		Cancelled: false,
	}, nil
}

// TestRule tests rule classification on sample input using the supplied configuration.
func (s *Service) TestRule(input string, cfg config.RuleConfig) (SandboxResult, error) {
	eng, err := engine.NewRuleEngineValidated(cfg)
	if err != nil {
		return SandboxResult{}, err
	}

	addr, status := eng.SplitAddress(input)
	_, issue, label, issueTime := eng.Classify(input)
	rule, idx, matched := eng.MatchRule(issue)
	metric := eng.MetricForLabel(label)

	matchKind := "none"
	matchedRuleIndex := -1
	matchedRuleID := ""

	if matched {
		matchKind = "rule"
		matchedRuleIndex = idx
		matchedRuleID = rule.ID
	} else if cfg.EnableHeuristics && isHeuristicMatch(issue) {
		matchKind = "heuristic"
	} else if cfg.DefaultLabel != "" {
		matchKind = "fallback"
	}

	return SandboxResult{
		Address:          addr,
		Status:           status,
		IssueTime:        issueTime,
		Label:            label,
		MatchedRuleIndex: matchedRuleIndex,
		MatchedRuleID:    matchedRuleID,
		MatchKind:        matchKind,
		Metric:           metric,
	}, nil
}

// isHeuristicMatch determines whether an issue description matches the dynamic heuristic (*_not_out).
func isHeuristicMatch(status string) bool {
	s := strings.ToUpper(strings.TrimSpace(status))
	s = strings.ReplaceAll(s, "NOT SVCD", "NOT SERVICED")
	s = strings.ReplaceAll(s, "UNABLE TO SVC", "UNABLE TO SERVICE")

	var sb strings.Builder
	lastWasUnderscore := false
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			lastWasUnderscore = false
		} else if !lastWasUnderscore && sb.Len() > 0 {
			sb.WriteRune('_')
			lastWasUnderscore = true
		}
	}

	label := strings.Trim(strings.ToLower(sb.String()), "_")
	labelWithoutTrailingDigits := strings.Trim(strings.TrimRight(label, "0123456789"), "_")
	return strings.HasSuffix(labelWithoutTrailingDigits, "_not_out")
}

// SetRulesDirty sets the rules dirty state for native close handling.
func (s *Service) SetRulesDirty(dirty bool) {
	s.rulesDirty.Store(dirty)
}

// IsRulesDirty returns whether there are unsaved rules changes.
func (s *Service) IsRulesDirty() bool {
	return s.rulesDirty.Load()
}
