package main

import (
	"regexp"
	"strings"
	"time"
)

type compiledRule struct {
	rule       ClassificationRule
	upperPat   string
	compiledRE *regexp.Regexp
}

// RuleEngine evaluates classification rules, splits addresses, and normalizes issue labels.
type RuleEngine struct {
	config        RuleConfig
	compiledRules []compiledRule
	metrics       map[string]MetricContribution
}

// NewRuleEngine constructs a new RuleEngine from a RuleConfig.
func NewRuleEngine(cfg RuleConfig) *RuleEngine {
	metrics := make(map[string]MetricContribution)
	for _, l := range cfg.Labels {
		metrics[l.Key] = l.Metric
	}

	compiled := make([]compiledRule, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		cr := compiledRule{
			rule: r,
		}
		if r.Type == RuleTypeSubstring {
			cr.upperPat = strings.ToUpper(r.Pattern)
		} else if r.Type == RuleTypeRegex {
			re, err := compileRegexPattern(r.Pattern)
			if err == nil {
				cr.compiledRE = re
			}
		}
		compiled = append(compiled, cr)
	}

	return &RuleEngine{
		config:        cfg.Clone(),
		compiledRules: compiled,
		metrics:       metrics,
	}
}

// Config returns a copy of the active engine configuration.
func (e *RuleEngine) Config() RuleConfig {
	return e.config.Clone()
}

// MetricForLabel returns the metric category associated with a label.
func (e *RuleEngine) MetricForLabel(label string) MetricContribution {
	if m, ok := e.metrics[label]; ok {
		return m
	}
	return MetricNone
}

// SplitAddress separates the street address from the issue/status text using suffixes and active rules.
func (e *RuleEngine) SplitAddress(raw string) (address string, status string) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, "–", "-")
	cleaned = strings.ReplaceAll(cleaned, "—", "-")

	// 1. Split based on the LAST address suffix.
	endIdx := findLastAddressIndex(cleaned)
	if endIdx != -1 {
		address = strings.TrimSpace(cleaned[:endIdx])
		status = strings.TrimSpace(cleaned[endIdx:])
	} else {
		// 2. If no suffix matches, check if we contain any known issue pattern.
		// E.g., for suffix-less addresses like "23 AND 25 KILSYTH MSW AND RECYC NOT OUT"
		// Only substring rules feed address splitting, picking the earliest index across all patterns.
		upper := strings.ToUpper(cleaned)
		earliestIdx := -1
		for _, cr := range e.compiledRules {
			if !cr.rule.Enabled || cr.rule.Type != RuleTypeSubstring {
				continue
			}
			idx := strings.Index(upper, cr.upperPat)
			if idx != -1 {
				if earliestIdx == -1 || idx < earliestIdx {
					earliestIdx = idx
				}
			}
		}
		if earliestIdx != -1 {
			address = strings.TrimSpace(cleaned[:earliestIdx])
			status = strings.TrimSpace(cleaned[earliestIdx:])
		} else {
			// 3. Fallback: split by first comma, semicolon, or space-dash-space
			firstDelim := strings.Index(cleaned, " - ")
			delimLen := 3
			if firstDelim == -1 {
				delimLen = 0
			}
			if idx := strings.IndexAny(cleaned, ",;"); idx != -1 && (firstDelim == -1 || idx < firstDelim) {
				firstDelim = idx
				delimLen = 1
			}

			if firstDelim != -1 {
				address = strings.TrimSpace(cleaned[:firstDelim])
				status = strings.TrimSpace(cleaned[firstDelim+delimLen:])
			} else {
				return cleaned, ""
			}
		}
	}

	address = strings.TrimFunc(address, func(r rune) bool {
		return r == ',' || r == '-' || r == ';' || r == ' ' || r == '.'
	})
	status = strings.TrimFunc(status, func(r rune) bool {
		return r == ',' || r == '-' || r == ';' || r == ' ' || r == '.'
	})

	return address, status
}

// NormalizeIssueLabel assigns a canonical label to a status text using active rules and heuristics.
func (e *RuleEngine) NormalizeIssueLabel(status string) string {
	s := strings.TrimSpace(status)
	sUpper := strings.ToUpper(s)

	for _, cr := range e.compiledRules {
		if !cr.rule.Enabled {
			continue
		}
		if cr.rule.Type == RuleTypeSubstring {
			if strings.Contains(sUpper, cr.upperPat) {
				return cr.rule.Label
			}
		} else if cr.rule.Type == RuleTypeRegex && cr.compiledRE != nil {
			if cr.compiledRE.MatchString(s) {
				return cr.rule.Label
			}
		}
	}

	if e.config.EnableHeuristics {
		s = strings.ReplaceAll(sUpper, "NOT SVCD", "NOT SERVICED")
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
		if strings.HasSuffix(labelWithoutTrailingDigits, "_not_out") {
			return "special_item_not_out"
		}
	}

	if e.config.DefaultLabel != "" {
		return e.config.DefaultLabel
	}
	return "other"
}

// MatchRule returns the first matching enabled rule, its index, and true if matched.
func (e *RuleEngine) MatchRule(status string) (ClassificationRule, int, bool) {
	s := strings.TrimSpace(status)
	sUpper := strings.ToUpper(s)
	for i, cr := range e.compiledRules {
		if !cr.rule.Enabled {
			continue
		}
		if cr.rule.Type == RuleTypeSubstring {
			if strings.Contains(sUpper, cr.upperPat) {
				return cr.rule, i, true
			}
		} else if cr.rule.Type == RuleTypeRegex && cr.compiledRE != nil {
			if cr.compiledRE.MatchString(s) {
				return cr.rule, i, true
			}
		}
	}
	return ClassificationRule{}, -1, false
}

// Classify extracts location, issue description, canonical label, and issue time from a raw entry line.
func (e *RuleEngine) Classify(raw string) (locationHint string, parsedIssue string, label string, issueTime string) {
	cleaned := strings.TrimSpace(raw)
	if matches := entryTimeRE.FindStringSubmatch(cleaned); matches != nil {
		issueTime = matches[1]
		cleaned = strings.TrimSpace(cleaned[:len(cleaned)-len(matches[0])])
	}

	address, status := e.SplitAddress(cleaned)
	if status == "" {
		return address, "", "", issueTime
	}

	return address, status, e.NormalizeIssueLabel(status), issueTime
}

// ParseRecords parses structured records from message metadata using the engine rules.
func (e *RuleEngine) ParseRecords(meta messageMetadata) []record {
	lines := cleanLines(meta.Body)
	records := make([]record, 0)
	var currentTime time.Time
	var currentDispatcher string
	rowInMessage := 0

	for _, line := range lines {
		if matches := timestampLineRE.FindStringSubmatch(line); matches != nil {
			parsedTime, err := time.ParseInLocation("01/02/2006 15:04:05", matches[1], time.Local)
			if err == nil {
				currentTime = parsedTime
			}
			currentDispatcher = matches[2]
			continue
		}
		if isFooterLine(line) {
			if currentTime.IsZero() {
				continue
			}
			break
		}
		if isIntroLine(line) {
			continue
		}

		if currentTime.IsZero() {
			continue
		}

		rowInMessage++
		for _, entry := range expandEntries(line) {
			locationHint, parsedIssue, label, issueTime := e.Classify(entry)
			records = append(records, record{
				SourceFile:   meta.SourceFile,
				Subject:      meta.Subject,
				MessageDate:  formatTime(meta.MessageDate),
				ReportedAt:   currentTime.Format(time.RFC3339),
				Dispatcher:   currentDispatcher,
				RowInMessage: rowInMessage,
				RawEntry:     entry,
				LocationHint: locationHint,
				ParsedIssue:  parsedIssue,
				Label:        label,
				IssueTime:    issueTime,
			})
		}
	}

	return records
}
