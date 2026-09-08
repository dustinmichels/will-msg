package engine

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"will-msg/internal/config"
	"will-msg/internal/parser"
)

var (
	entryTimeRE       = regexp.MustCompile(`\b(\d{3,4}(?:AM|PM))\s*$`)
	suffixRE          = regexp.MustCompile(`(?i)\b(?:STREET|ST|STR|AVENUE|AVE|ROAD|RD|WAY|DRIVE|DR|LANE|LN|PLACE|PL|CIRCLE|CIR|BOULEVARD|BLVD|HIGHWAY|HWY|TERRACE|TER|TERR|PARKWAY|PKWY|COURT|CT|COVE|SQUARE|SQ|PARK|TRAIL|TRL|APTS|APT|CONDOS|CONDO|UNITS|UNIT|SUITES|SUITE|STE|FLOOR|FL|FELLSWAY|BROADWAY|GREENWAY|EXPRESSWAY|SPEEDWAY)\b`)
	unitModifierRE    = regexp.MustCompile(`^(?i)(?:\s*#?\s*\d+[A-Z]?|\s+[A-Z\d]\b)`)
	locModifierRE     = regexp.MustCompile(`^(?i)(?:\s+(?:MANY|ALL)\s+(?:HOMES|HOUSES|APTS|CONDOS|UNITS)\b)`)
	statusStartRE     = regexp.MustCompile(`^(?i)\s*(?:IS|WAS|HAS|ARE|BE|TOO|WILL)\b`)
	precedingRejectRE = regexp.MustCompile(`(?i)\b(?:WHOLE|OF|ON|IN|THE|BOTH|EACH|EVERY|THIS|THAT|TO|FOR|BY)\s*$`)
	directionalRE     = regexp.MustCompile(`^(?i)\s+(?:WEST|W|EAST|E|NORTH|N|SOUTH|S)\b`)
)

// Record represents a single structured parsed row from a message.
type Record struct {
	SourceFile   string `json:"source_file"`
	Subject      string `json:"subject"`
	MessageDate  string `json:"message_date"`
	ReportedAt   string `json:"reported_at"`
	Dispatcher   string `json:"dispatcher"`
	RowInMessage int    `json:"row_in_message"`
	RawEntry     string `json:"raw_entry"`
	LocationHint string `json:"location"`
	ParsedIssue  string `json:"issue"`
	Label        string `json:"label"`
	IssueTime    string `json:"issue_time"`
}

// CSVHeaders defines the standard CSV column headers.
var CSVHeaders = []string{
	"source_file",
	"subject",
	"message_date",
	"reported_at",
	"dispatcher",
	"row_in_message",
	"raw_entry",
	"location",
	"issue",
	"label",
	"issue_time",
}

// ToRow converts a Record into a CSV row string slice.
func (rec Record) ToRow() []string {
	return []string{
		rec.SourceFile,
		rec.Subject,
		rec.MessageDate,
		rec.ReportedAt,
		rec.Dispatcher,
		strconv.Itoa(rec.RowInMessage),
		rec.RawEntry,
		rec.LocationHint,
		rec.ParsedIssue,
		rec.Label,
		rec.IssueTime,
	}
}

type compiledRule struct {
	rule       config.ClassificationRule
	upperPat   string
	compiledRE *regexp.Regexp
}

// RuleEngine evaluates classification rules, splits addresses, and normalizes issue labels.
type RuleEngine struct {
	config        config.RuleConfig
	compiledRules []compiledRule
	metrics       map[string]config.MetricContribution
}

// NewRuleEngineValidated constructs a new RuleEngine from a RuleConfig after validating it.
// If the configuration fails validation or any regex rule fails to compile, it returns a descriptive error.
func NewRuleEngineValidated(cfg config.RuleConfig) (*RuleEngine, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate rule config: %w", err)
	}

	metrics := make(map[string]config.MetricContribution)
	for _, l := range cfg.Labels {
		metrics[l.Key] = l.Metric
	}

	compiled := make([]compiledRule, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		cr := compiledRule{
			rule: r,
		}
		if r.Type == config.RuleTypeSubstring {
			cr.upperPat = strings.ToUpper(r.Pattern)
		} else if r.Type == config.RuleTypeRegex {
			re, err := config.CompileRegexPattern(r.Pattern)
			if err != nil {
				return nil, fmt.Errorf("rule %q: invalid regex %q: %w", r.ID, r.Pattern, err)
			}
			cr.compiledRE = re
		} else {
			return nil, fmt.Errorf("rule %q: unknown rule type %q", r.ID, r.Type)
		}
		compiled = append(compiled, cr)
	}

	return &RuleEngine{
		config:        cfg.Clone(),
		compiledRules: compiled,
		metrics:       metrics,
	}, nil
}

// NewRuleEngine constructs a new RuleEngine from a RuleConfig.
// If any rule fails validation or regex compilation, a warning is logged
// and the engine is constructed with valid rules to prevent crashes.
func NewRuleEngine(cfg config.RuleConfig) *RuleEngine {
	if err := cfg.Validate(); err != nil {
		log.Printf("warning: NewRuleEngine called with invalid config: %v", err)
	}

	metrics := make(map[string]config.MetricContribution)
	for _, l := range cfg.Labels {
		metrics[l.Key] = l.Metric
	}

	compiled := make([]compiledRule, 0, len(cfg.Rules))
	for _, r := range cfg.Rules {
		cr := compiledRule{
			rule: r,
		}
		if r.Type == config.RuleTypeSubstring {
			cr.upperPat = strings.ToUpper(r.Pattern)
		} else if r.Type == config.RuleTypeRegex {
			re, err := config.CompileRegexPattern(r.Pattern)
			if err == nil {
				cr.compiledRE = re
			} else {
				log.Printf("warning: rule %q: invalid regex %q: %v", r.ID, r.Pattern, err)
			}
		} else {
			log.Printf("warning: rule %q: unknown rule type %q", r.ID, r.Type)
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
func (e *RuleEngine) Config() config.RuleConfig {
	return e.config.Clone()
}

// MetricForLabel returns the metric category associated with a label.
func (e *RuleEngine) MetricForLabel(label string) config.MetricContribution {
	if m, ok := e.metrics[label]; ok {
		return m
	}
	return config.MetricNone
}

// SplitAddress separates the street address from the issue/status text using suffixes and active rules.
func (e *RuleEngine) SplitAddress(raw string) (address string, status string) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, "–", "-")
	cleaned = strings.ReplaceAll(cleaned, "—", "-")

	// 1. Split based on the LAST address suffix.
	endIdx := FindLastAddressIndex(cleaned)
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
			if !cr.rule.Enabled || cr.rule.Type != config.RuleTypeSubstring {
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

// FindLastAddressIndex identifies the index separating the street address suffix from trailing status modifiers.
func FindLastAddressIndex(cleaned string) int {
	matches := suffixRE.FindAllStringSubmatchIndex(cleaned, -1)
	if len(matches) == 0 {
		return -1
	}

	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		start := match[0]
		end := match[1]

		suffixStr := strings.ToUpper(cleaned[start:end])
		isUnitSuffix := false
		unitSuffixes := []string{"APT", "APTS", "UNIT", "UNITS", "CONDOS", "CONDO", "SUITE", "SUITES", "STE", "FL", "FLOOR"}
		for _, us := range unitSuffixes {
			if suffixStr == us {
				isUnitSuffix = true
				break
			}
		}

		if isUnitSuffix {
			rem := cleaned[end:]
			if modifierMatches := unitModifierRE.FindStringIndex(rem); modifierMatches != nil && modifierMatches[0] == 0 {
				end += modifierMatches[1]
			}
		}

		// Check if a location modifier follows (e.g. "MANY HOMES")
		rem := cleaned[end:]
		if locModifierMatches := locModifierRE.FindStringIndex(rem); locModifierMatches != nil && locModifierMatches[0] == 0 {
			end += locModifierMatches[1]
			rem = cleaned[end:]
		}

		// Check if a directional follows (e.g. "WEST", "W", "EAST", "E")
		if directionalMatches := directionalRE.FindStringIndex(rem); directionalMatches != nil && directionalMatches[0] == 0 {
			end += directionalMatches[1]
			rem = cleaned[end:]
		}

		// Verify this is a valid address suffix by checking preceding text and remainder
		preceding := cleaned[:start]
		if precedingRejectRE.MatchString(preceding) {
			continue
		}
		if statusStartRE.MatchString(rem) {
			// Only reject if it's a suffix that can double as a standalone noun/subject in shorthand
			// (like "ROAD", "WAY", "PARK") rather than a definitive street abbreviation (like "ST", "AVE").
			suffixUpper := strings.ToUpper(cleaned[start:end])
			if suffixUpper == "ROAD" || suffixUpper == "WAY" || suffixUpper == "PARK" {
				continue
			}
		}

		return end
	}

	return -1
}

// NormalizeIssueLabel assigns a canonical label to a status text using active rules and heuristics.
func (e *RuleEngine) NormalizeIssueLabel(status string) string {
	s := strings.TrimSpace(status)
	sUpper := strings.ToUpper(s)

	for _, cr := range e.compiledRules {
		if !cr.rule.Enabled {
			continue
		}
		if cr.rule.Type == config.RuleTypeSubstring {
			if strings.Contains(sUpper, cr.upperPat) {
				return cr.rule.Label
			}
		} else if cr.rule.Type == config.RuleTypeRegex && cr.compiledRE != nil {
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
func (e *RuleEngine) MatchRule(status string) (config.ClassificationRule, int, bool) {
	s := strings.TrimSpace(status)
	sUpper := strings.ToUpper(s)
	for i, cr := range e.compiledRules {
		if !cr.rule.Enabled {
			continue
		}
		if cr.rule.Type == config.RuleTypeSubstring {
			if strings.Contains(sUpper, cr.upperPat) {
				return cr.rule, i, true
			}
		} else if cr.rule.Type == config.RuleTypeRegex && cr.compiledRE != nil {
			if cr.compiledRE.MatchString(s) {
				return cr.rule, i, true
			}
		}
	}
	return config.ClassificationRule{}, -1, false
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
func (e *RuleEngine) ParseRecords(meta parser.MessageMetadata) []Record {
	lines := parser.CleanLines(meta.Body)
	records := make([]Record, 0)
	var currentTime time.Time
	var currentDispatcher string
	rowInMessage := 0

	for _, line := range lines {
		if t, disp, ok := parser.ParseTimestampLine(line); ok {
			currentTime = t
			currentDispatcher = disp
			continue
		}
		if parser.IsFooterLine(line) {
			if currentTime.IsZero() {
				continue
			}
			break
		}
		if parser.IsIntroLine(line) {
			continue
		}

		if currentTime.IsZero() {
			continue
		}

		rowInMessage++
		for _, entry := range parser.ExpandEntries(line) {
			locationHint, parsedIssue, label, issueTime := e.Classify(entry)
			records = append(records, Record{
				SourceFile:   meta.SourceFile,
				Subject:      meta.Subject,
				MessageDate:  parser.FormatTime(meta.MessageDate),
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
