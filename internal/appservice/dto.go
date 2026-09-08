package appservice

import (
	"will-msg/internal/config"
	"will-msg/internal/engine"
	"will-msg/internal/scanner"
)

// ScanResult represents the sources discovered from one or more root paths.
type ScanResult struct {
	Sources     []scanner.MessageSource `json:"sources"`
	Count       int                     `json:"count"`
	SourcePaths []string                `json:"source_paths"`
}

// SkippedSource records an individual source that failed to load during parsing.
type SkippedSource struct {
	DisplayName string `json:"display_name"`
	Error       string `json:"error"`
}

// ParseResult represents the output of a batch parse run.
type ParseResult struct {
	Records    []engine.Record `json:"records"`
	Skipped    []SkippedSource `json:"skipped"`
	Superseded bool            `json:"superseded"`
}

// SavedFile represents the result of a file save operation.
type SavedFile struct {
	Filename  string `json:"filename"`
	Dir       string `json:"dir"`
	Path      string `json:"path"`
	Cancelled bool   `json:"cancelled"`
}

// ImportRulesResult represents the result of importing a rules configuration file.
type ImportRulesResult struct {
	Config    *config.RuleConfig `json:"config,omitempty"`
	Cancelled bool               `json:"cancelled"`
}

// ValidationError represents a single validation issue addressed by a JSON Pointer path.
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// SandboxResult represents live testing evaluation results on sample input.
type SandboxResult struct {
	Address          string                    `json:"address"`
	Status           string                    `json:"status"`
	IssueTime        string                    `json:"issue_time"`
	Label            string                    `json:"label"`
	MatchedRuleIndex int                       `json:"matched_rule_index"`
	MatchedRuleID    string                    `json:"matched_rule_id"`
	MatchKind        string                    `json:"match_kind"`
	Metric           config.MetricContribution `json:"metric"`
}
