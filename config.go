package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	// ConfigDirName is the subfolder name under the user config directory.
	ConfigDirName = "will-msg"
	// ConfigFileName is the default filename for stored rules.
	ConfigFileName = "rules.json"
	// CurrentConfigVersion is the current schema version for RuleConfig.
	CurrentConfigVersion = 1
)

var renameFile = os.Rename

// RuleType specifies whether a classification rule uses exact substring matching or a regular expression.
type RuleType string

const (
	RuleTypeSubstring RuleType = "substring"
	RuleTypeRegex     RuleType = "regex"
)

// MetricContribution defines how a label affects trash and recycling statistics.
type MetricContribution string

const (
	MetricNone      MetricContribution = "none"
	MetricTrash     MetricContribution = "trash"
	MetricRecycling MetricContribution = "recycling"
	MetricBoth      MetricContribution = "both"
)

// ClassificationRule represents a single rule to match an issue pattern and assign a label.
type ClassificationRule struct {
	ID          string   `json:"id"`
	Pattern     string   `json:"pattern"`
	Type        RuleType `json:"type"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Enabled     bool     `json:"enabled"`
}

// LabelDefinition defines a recognized label key, friendly display name, and its metric mapping.
type LabelDefinition struct {
	Key         string             `json:"key"`
	DisplayName string             `json:"display_name"`
	Metric      MetricContribution `json:"metric"`
}

// RuleConfig encapsulates all configurable rules, labels, and heuristic options.
type RuleConfig struct {
	Version          int                  `json:"version"`
	EnableHeuristics bool                 `json:"enable_heuristics"`
	DefaultLabel     string               `json:"default_label"`
	Labels           []LabelDefinition    `json:"labels"`
	Rules            []ClassificationRule `json:"rules"`
}

// DefaultLabels returns the standard default label definitions.
func DefaultLabels() []LabelDefinition {
	return []LabelDefinition{
		{Key: "msw_and_recyc_not_out", DisplayName: "Trash & Recycling Not Out", Metric: MetricBoth},
		{Key: "msw_not_out", DisplayName: "Trash Not Out", Metric: MetricTrash},
		{Key: "recyc_not_out", DisplayName: "Recycling Not Out", Metric: MetricRecycling},
		{Key: "special_item_not_out", DisplayName: "Special Item Not Out", Metric: MetricNone},
		{Key: "recyc_contaminated", DisplayName: "Recycling Contaminated", Metric: MetricNone},
		{Key: "blocked", DisplayName: "Blocked / Inaccessible", Metric: MetricNone},
		{Key: "overflowing", DisplayName: "Overflowing / Overloaded", Metric: MetricNone},
		{Key: "other", DisplayName: "Other", Metric: MetricNone},
	}
}

// DefaultClassificationRules returns the default ordered list of classification rules.
func DefaultClassificationRules() []ClassificationRule {
	return []ClassificationRule{
		{ID: "msw_and_recyc_not_out_1", Pattern: "MSW AND RECYC NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "msw_and_recyc_not_out_2", Pattern: "MSW AND RCY NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "msw_and_recyc_not_out_3", Pattern: "RECYC AND MSW NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "msw_and_recyc_not_out_4", Pattern: "RCY AND MSW NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "msw_and_recyc_not_out_5", Pattern: "TRASH AND RECYCLING NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "msw_and_recyc_not_out_6", Pattern: "TRASH AND RECYC NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "msw_and_recyc_not_out_7", Pattern: "TRASH AND RCY NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "msw_and_recyc_not_out_8", Pattern: "TRASH/RECYC NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "msw_and_recyc_not_out_9", Pattern: "TRASH/RCY NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "msw_and_recyc_not_out_10", Pattern: "TRASH / RECYC NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "msw_and_recyc_not_out_11", Pattern: "TRASH / RCY NOT OUT", Type: RuleTypeSubstring, Label: "msw_and_recyc_not_out", Enabled: true},
		{ID: "recyc_not_out_1", Pattern: "RECYC NOT OUT", Type: RuleTypeSubstring, Label: "recyc_not_out", Enabled: true},
		{ID: "recyc_not_out_2", Pattern: "RCY NOT OUT", Type: RuleTypeSubstring, Label: "recyc_not_out", Enabled: true},
		{ID: "recyc_not_out_3", Pattern: "RECYCLE NOT OUT", Type: RuleTypeSubstring, Label: "recyc_not_out", Enabled: true},
		{ID: "recyc_not_out_4", Pattern: "RECYCLING NOT OUT", Type: RuleTypeSubstring, Label: "recyc_not_out", Enabled: true},
		{ID: "recyc_not_out_5", Pattern: "RECYCCLE NOT OUT", Type: RuleTypeSubstring, Label: "recyc_not_out", Enabled: true},
		{ID: "recyc_not_out_6", Pattern: "NOT OUT RECYC", Type: RuleTypeSubstring, Label: "recyc_not_out", Enabled: true},
		{ID: "recyc_not_out_7", Pattern: "NOT OUT RCY", Type: RuleTypeSubstring, Label: "recyc_not_out", Enabled: true},
		{ID: "msw_not_out_1", Pattern: "MSW NOT OUT", Type: RuleTypeSubstring, Label: "msw_not_out", Enabled: true},
		{ID: "msw_not_out_2", Pattern: "TRASH NOT OUT", Type: RuleTypeSubstring, Label: "msw_not_out", Enabled: true},
		{ID: "special_item_not_out_1", Pattern: "BULK ITEM NOT OUT", Type: RuleTypeSubstring, Label: "special_item_not_out", Enabled: true},
		{ID: "special_item_not_out_2", Pattern: "BEDFRAME AND SOFA NOT OUT", Type: RuleTypeSubstring, Label: "special_item_not_out", Enabled: true},
		{ID: "special_item_not_out_3", Pattern: "FRIDGE NOT OUT", Type: RuleTypeSubstring, Label: "special_item_not_out", Enabled: true},
		{ID: "special_item_not_out_4", Pattern: "SOFA NOT OUT", Type: RuleTypeSubstring, Label: "special_item_not_out", Enabled: true},
		{ID: "recyc_contaminated_1", Pattern: "RECY CONTAM", Type: RuleTypeSubstring, Label: "recyc_contaminated", Enabled: true},
		{ID: "recyc_contaminated_2", Pattern: "RECYC CONTAM", Type: RuleTypeSubstring, Label: "recyc_contaminated", Enabled: true},
		{ID: "recyc_contaminated_3", Pattern: "RCY CONTAM", Type: RuleTypeSubstring, Label: "recyc_contaminated", Enabled: true},
		{ID: "recyc_contaminated_4", Pattern: "RECYCLE CONTAM", Type: RuleTypeSubstring, Label: "recyc_contaminated", Enabled: true},
		{ID: "recyc_contaminated_5", Pattern: "RECYCLING CONTAM", Type: RuleTypeSubstring, Label: "recyc_contaminated", Enabled: true},
		{ID: "recyc_contaminated_6", Pattern: "CONTAMINATED RECYC", Type: RuleTypeSubstring, Label: "recyc_contaminated", Enabled: true},
		{ID: "recyc_contaminated_7", Pattern: "CONTAMINATED RCY", Type: RuleTypeSubstring, Label: "recyc_contaminated", Enabled: true},
		{ID: "recyc_contaminated_8", Pattern: "CONTAMINATED RECYCLE", Type: RuleTypeSubstring, Label: "recyc_contaminated", Enabled: true},
		{ID: "recyc_contaminated_9", Pattern: "CONTAMINATED RECYCLING", Type: RuleTypeSubstring, Label: "recyc_contaminated", Enabled: true},
		{ID: "recyc_contaminated_10", Pattern: "RECYCLING CONTAMINATED", Type: RuleTypeSubstring, Label: "recyc_contaminated", Enabled: true},
		{ID: "blocked_1", Pattern: `\bBLOCK(?:ED|ING|S)?\b`, Type: RuleTypeRegex, Label: "blocked", Description: "Blocked access", Enabled: true},
		{ID: "overflowing_1", Pattern: `\b(?:OVERFLOW(?:ING|ED|S)|OVERLOAD(?:ED|ING|S)?)\b`, Type: RuleTypeRegex, Label: "overflowing", Description: "Overflowing or overloaded container", Enabled: true},
	}
}

// DefaultRuleConfig constructs a fresh RuleConfig initialized with default rules and settings.
func DefaultRuleConfig() RuleConfig {
	return RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels:           DefaultLabels(),
		Rules:            DefaultClassificationRules(),
	}
}

// Clone creates a deep copy of the RuleConfig.
func (cfg RuleConfig) Clone() RuleConfig {
	cloned := cfg
	cloned.Labels = make([]LabelDefinition, len(cfg.Labels))
	copy(cloned.Labels, cfg.Labels)
	cloned.Rules = make([]ClassificationRule, len(cfg.Rules))
	copy(cloned.Rules, cfg.Rules)
	return cloned
}

// FindLabel returns the LabelDefinition for a given key, and whether it was found.
func (cfg RuleConfig) FindLabel(key string) (LabelDefinition, bool) {
	for _, l := range cfg.Labels {
		if l.Key == key {
			return l, true
		}
	}
	return LabelDefinition{}, false
}

// MetricForLabel returns the MetricContribution associated with a label key.
func (cfg RuleConfig) MetricForLabel(key string) MetricContribution {
	if lbl, ok := cfg.FindLabel(key); ok {
		return lbl.Metric
	}
	return MetricNone
}

// ToJSON serializes the configuration to indented JSON.
func (cfg RuleConfig) ToJSON() ([]byte, error) {
	return json.MarshalIndent(cfg, "", "  ")
}

// ParseRuleConfig parses, migrates, and validates JSON data into a RuleConfig.
func ParseRuleConfig(data []byte) (RuleConfig, error) {
	var cfg RuleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return RuleConfig{}, fmt.Errorf("unmarshal rule config: %w", err)
	}
	cfg = MigrateConfig(cfg)
	if err := cfg.Validate(); err != nil {
		return RuleConfig{}, fmt.Errorf("validate rule config: %w", err)
	}
	return cfg, nil
}

// MigrateConfig normalizes and upgrades older or partial configuration structures.
func MigrateConfig(cfg RuleConfig) RuleConfig {
	if cfg.Version < CurrentConfigVersion {
		cfg.Version = CurrentConfigVersion
	}
	if len(cfg.Labels) == 0 {
		cfg.Labels = DefaultLabels()
	} else {
		for i := range cfg.Labels {
			if cfg.Labels[i].Metric == "" {
				cfg.Labels[i].Metric = MetricNone
			}
		}
	}
	if cfg.DefaultLabel == "" {
		cfg.DefaultLabel = "other"
	}
	return cfg
}

// Validate verifies that the configuration is structurally sound.
func (cfg RuleConfig) Validate() error {
	if strings.TrimSpace(cfg.DefaultLabel) == "" {
		return fmt.Errorf("default_label cannot be empty")
	}

	seenLabels := make(map[string]bool)
	for i, lbl := range cfg.Labels {
		if strings.TrimSpace(lbl.Key) == "" {
			return fmt.Errorf("label #%d: key cannot be empty", i+1)
		}
		if seenLabels[lbl.Key] {
			return fmt.Errorf("label #%d: duplicate label key %q", i+1, lbl.Key)
		}
		seenLabels[lbl.Key] = true

		switch lbl.Metric {
		case MetricNone, MetricTrash, MetricRecycling, MetricBoth:
			// valid
		default:
			return fmt.Errorf("label %q: invalid metric contribution %q", lbl.Key, lbl.Metric)
		}
	}

	if !seenLabels[cfg.DefaultLabel] {
		return fmt.Errorf("default_label %q is not defined in labels list", cfg.DefaultLabel)
	}

	seenIDs := make(map[string]bool)
	for i, rule := range cfg.Rules {
		if strings.TrimSpace(rule.ID) == "" {
			return fmt.Errorf("rule #%d: ID cannot be empty", i+1)
		}
		if seenIDs[rule.ID] {
			return fmt.Errorf("rule #%d: duplicate rule ID %q", i+1, rule.ID)
		}
		seenIDs[rule.ID] = true

		if strings.TrimSpace(rule.Pattern) == "" {
			return fmt.Errorf("rule %q: pattern cannot be empty", rule.ID)
		}

		if strings.TrimSpace(rule.Label) == "" {
			return fmt.Errorf("rule %q: label cannot be empty", rule.ID)
		}
		if !seenLabels[rule.Label] {
			return fmt.Errorf("rule %q: label %q is not defined in labels list", rule.ID, rule.Label)
		}

		switch rule.Type {
		case RuleTypeSubstring:
			// valid
		case RuleTypeRegex:
			if _, err := compileRegexPattern(rule.Pattern); err != nil {
				return fmt.Errorf("rule %q: invalid regex %q: %w", rule.ID, rule.Pattern, err)
			}
		default:
			return fmt.Errorf("rule %q: unknown rule type %q", rule.ID, rule.Type)
		}
	}
	return nil
}

// DefaultConfigDir returns the base directory for storing application configuration.
func DefaultConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		home, hErr := os.UserHomeDir()
		if hErr != nil || home == "" {
			return filepath.Join(".", "."+ConfigDirName), nil
		}
		return filepath.Join(home, ".config", ConfigDirName), nil
	}
	return filepath.Join(base, ConfigDirName), nil
}

// DefaultConfigPath returns the full path to rules.json in the user config directory.
func DefaultConfigPath() (string, error) {
	dir, err := DefaultConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigFileName), nil
}

// LoadConfig loads the active classification rules from the user config directory.
// If the configuration file does not exist, it returns the default configuration.
// If the file exists but contains invalid or corrupted JSON, it creates a backup copy,
// logs a warning, and returns the default configuration.
func LoadConfig() RuleConfig {
	path, err := DefaultConfigPath()
	if err != nil {
		log.Printf("warning: unable to determine config path (%v); using default rules", err)
		return DefaultRuleConfig()
	}

	cfg, err := LoadConfigFromPath(path)
	if err != nil {
		log.Printf("warning: failed to load config from %s: %v; using default rules", path, err)
		return DefaultRuleConfig()
	}
	return cfg
}

// LoadConfigFromPath reads, parses, validates, and migrates a RuleConfig from the given path.
// If the file does not exist, it returns DefaultRuleConfig() and nil error.
// If the file is corrupted, it backs up the file to <path>.corrupted.<timestamp> before returning an error.
func LoadConfigFromPath(path string) (RuleConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultRuleConfig(), nil
		}
		return DefaultRuleConfig(), fmt.Errorf("read config file: %w", err)
	}

	cfg, err := ParseRuleConfig(data)
	if err != nil {
		// Back up corrupt file so user changes aren't lost
		backupPath := fmt.Sprintf("%s.corrupted.%d", path, time.Now().UnixNano())
		if writeErr := os.WriteFile(backupPath, data, 0644); writeErr == nil {
			log.Printf("warning: corrupt config file backed up to %s", backupPath)
		}
		return DefaultRuleConfig(), fmt.Errorf("corrupt config file (backed up to %s): %w", backupPath, err)
	}

	return cfg, nil
}

// SaveConfig saves the configuration to the default user config path.
func SaveConfig(cfg RuleConfig) error {
	path, err := DefaultConfigPath()
	if err != nil {
		return fmt.Errorf("resolve default config path: %w", err)
	}
	return SaveConfigToPath(path, cfg)
}

// SaveConfigToPath validates and writes the configuration to the specified path atomically.
func SaveConfigToPath(path string, cfg RuleConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	data, err := cfg.ToJSON()
	if err != nil {
		return fmt.Errorf("serialize config: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Write to temporary file in the same directory for atomic replacement
	tmpFile, err := os.CreateTemp(dir, "rules-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write temp config: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temp config: %w", err)
	}

	// Rename temp file to target path atomically
	if err := renameFile(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("replace config file: %w", err)
	}
	return nil
}

// ExportConfigFile writes the RuleConfig as formatted JSON to the specified destination path.
func ExportConfigFile(path string, cfg RuleConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	data, err := cfg.ToJSON()
	if err != nil {
		return fmt.Errorf("serialize config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ImportConfigFile reads and parses a RuleConfig from an external JSON file with full validation.
func ImportConfigFile(path string) (RuleConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RuleConfig{}, fmt.Errorf("read import file: %w", err)
	}
	cfg, err := ParseRuleConfig(data)
	if err != nil {
		return RuleConfig{}, fmt.Errorf("invalid import config: %w", err)
	}
	return cfg, nil
}

// ExportConfig writes formatted JSON to the writer with validation.
func ExportConfig(w io.Writer, cfg RuleConfig) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	data, err := cfg.ToJSON()
	if err != nil {
		return fmt.Errorf("serialize config: %w", err)
	}
	_, err = w.Write(data)
	return err
}

// ImportConfig reads and parses JSON from the reader with validation and migration.
func ImportConfig(r io.Reader) (RuleConfig, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return RuleConfig{}, fmt.Errorf("read import data: %w", err)
	}
	cfg, err := ParseRuleConfig(data)
	if err != nil {
		return RuleConfig{}, fmt.Errorf("invalid import config: %w", err)
	}
	return cfg, nil
}

func compileRegexPattern(pat string) (*regexp.Regexp, error) {
	if strings.HasPrefix(pat, "(?i)") || strings.HasPrefix(pat, "(?-i)") {
		return regexp.Compile(pat)
	}
	return regexp.Compile("(?i)" + pat)
}
