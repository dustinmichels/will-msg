package main

import (
	"testing"
)

func TestRuleEngineCustomSubstringPrecedence(t *testing.T) {
	cfg := RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels: []LabelDefinition{
			{Key: "custom_yard_waste", DisplayName: "Yard Waste", Metric: MetricTrash},
			{Key: "recyc_not_out", DisplayName: "Recycling Not Out", Metric: MetricRecycling},
		},
		Rules: []ClassificationRule{
			{
				ID:      "yard_waste_1",
				Pattern: "YARD WASTE NOT OUT",
				Type:    RuleTypeSubstring,
				Label:   "custom_yard_waste",
				Enabled: true,
			},
			{
				ID:      "recyc_not_out_1",
				Pattern: "NOT OUT",
				Type:    RuleTypeSubstring,
				Label:   "recyc_not_out",
				Enabled: true,
			},
		},
	}

	engine := NewRuleEngine(cfg)

	loc, issue, label, time := engine.Classify("123 MAIN ST YARD WASTE NOT OUT 0900AM")
	if loc != "123 MAIN ST" {
		t.Errorf("expected loc '123 MAIN ST', got %q", loc)
	}
	if issue != "YARD WASTE NOT OUT" {
		t.Errorf("expected issue 'YARD WASTE NOT OUT', got %q", issue)
	}
	if label != "custom_yard_waste" {
		t.Errorf("expected label 'custom_yard_waste', got %q", label)
	}
	if time != "0900AM" {
		t.Errorf("expected time '0900AM', got %q", time)
	}

	rule, idx, ok := engine.MatchRule("YARD WASTE NOT OUT")
	if !ok || idx != 0 || rule.ID != "yard_waste_1" {
		t.Errorf("MatchRule failed: got rule=%+v, idx=%d, ok=%v", rule, idx, ok)
	}
}

func TestRuleEngineRuleReorderingPrecedence(t *testing.T) {
	// General rule before specific rule
	cfg := RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels:           DefaultLabels(),
		Rules: []ClassificationRule{
			{
				ID:      "general_rule",
				Pattern: "NOT OUT",
				Type:    RuleTypeSubstring,
				Label:   "recyc_not_out",
				Enabled: true,
			},
			{
				ID:      "specific_rule",
				Pattern: "MSW AND RECYC NOT OUT",
				Type:    RuleTypeSubstring,
				Label:   "msw_and_recyc_not_out",
				Enabled: true,
			},
		},
	}

	engine := NewRuleEngine(cfg)
	label := engine.NormalizeIssueLabel("MSW AND RECYC NOT OUT")
	if label != "recyc_not_out" {
		t.Errorf("expected general rule to match first when placed ahead, got %q", label)
	}

	// Specific rule before general rule
	cfg.Rules = []ClassificationRule{cfg.Rules[1], cfg.Rules[0]}
	engine2 := NewRuleEngine(cfg)
	label2 := engine2.NormalizeIssueLabel("MSW AND RECYC NOT OUT")
	if label2 != "msw_and_recyc_not_out" {
		t.Errorf("expected specific rule to match when placed ahead, got %q", label2)
	}
}

func TestRuleEngineRegexRules(t *testing.T) {
	cfg := RuleConfig{
		Version:          1,
		EnableHeuristics: false,
		DefaultLabel:     "other",
		Labels: []LabelDefinition{
			{Key: "snow_obstruction", DisplayName: "Snow Obstruction", Metric: MetricNone},
		},
		Rules: []ClassificationRule{
			{
				ID:      "snow_regex",
				Pattern: `\bSNOW(?:BANK|DRIFT)?\b`,
				Type:    RuleTypeRegex,
				Label:   "snow_obstruction",
				Enabled: true,
			},
		},
	}

	engine := NewRuleEngine(cfg)

	tests := []struct {
		input     string
		wantLabel string
	}{
		{"CAR IN SNOWBANK", "snow_obstruction"},
		{"BIG SNOWDRIFT IN WAY", "snow_obstruction"},
		{"SNOW ON ROAD", "snow_obstruction"},
		{"ICE ON ROAD", "other"},
	}

	for _, tt := range tests {
		got := engine.NormalizeIssueLabel(tt.input)
		if got != tt.wantLabel {
			t.Errorf("NormalizeIssueLabel(%q) = %q, want %q", tt.input, got, tt.wantLabel)
		}
	}
}

func TestRuleEngineRegexCaseSensitivity(t *testing.T) {
	cfg := RuleConfig{
		Version:          1,
		EnableHeuristics: false,
		DefaultLabel:     "other",
		Labels: []LabelDefinition{
			{Key: "strict_case", DisplayName: "Strict Case", Metric: MetricNone},
			{Key: "default_case", DisplayName: "Default Case", Metric: MetricNone},
		},
		Rules: []ClassificationRule{
			{
				ID:      "case_sensitive_regex",
				Pattern: `(?-i)\bexactLower\b`,
				Type:    RuleTypeRegex,
				Label:   "strict_case",
				Enabled: true,
			},
			{
				ID:      "default_insensitive_regex",
				Pattern: `\bdefaultCase\b`,
				Type:    RuleTypeRegex,
				Label:   "default_case",
				Enabled: true,
			},
		},
	}

	engine := NewRuleEngine(cfg)

	tests := []struct {
		input     string
		wantLabel string
	}{
		// Case-sensitive rule with (?-i)
		{"item with exactLower word", "strict_case"},
		{"item with EXACTLOWER word", "other"},
		{"item with ExactLower word", "other"},
		// Case-insensitive rule without prefix
		{"item with defaultCase word", "default_case"},
		{"item with DEFAULTCASE word", "default_case"},
		{"item with defaultcase word", "default_case"},
	}

	for _, tt := range tests {
		got := engine.NormalizeIssueLabel(tt.input)
		if got != tt.wantLabel {
			t.Errorf("NormalizeIssueLabel(%q) = %q, want %q", tt.input, got, tt.wantLabel)
		}

		rule, _, matched := engine.MatchRule(tt.input)
		if tt.wantLabel != "other" {
			if !matched || rule.Label != tt.wantLabel {
				t.Errorf("MatchRule(%q) = %+v, matched=%v; want label %q", tt.input, rule, matched, tt.wantLabel)
			}
		} else {
			if matched && rule.Label == "strict_case" {
				t.Errorf("MatchRule(%q) matched strict_case unexpectedly", tt.input)
			}
		}
	}
}

func TestRuleEngineDisabledRules(t *testing.T) {
	cfg := RuleConfig{
		Version:          1,
		EnableHeuristics: false,
		DefaultLabel:     "custom_other",
		Labels:           DefaultLabels(),
		Rules: []ClassificationRule{
			{
				ID:      "rule_disabled",
				Pattern: "MSW NOT OUT",
				Type:    RuleTypeSubstring,
				Label:   "msw_not_out",
				Enabled: false,
			},
		},
	}

	engine := NewRuleEngine(cfg)
	label := engine.NormalizeIssueLabel("MSW NOT OUT")
	if label != "custom_other" {
		t.Errorf("expected disabled rule to be skipped and return %q, got %q", "custom_other", label)
	}

	_, _, ok := engine.MatchRule("MSW NOT OUT")
	if ok {
		t.Errorf("expected MatchRule to return false for disabled rule")
	}
}

func TestRuleEngineSplitAddressWithRules(t *testing.T) {
	cfg := DefaultRuleConfig()
	engine := NewRuleEngine(cfg)

	// Suffix-less address split by issue pattern
	addr, status := engine.SplitAddress("23 AND 25 KILSYTH MSW AND RECYC NOT OUT")
	if addr != "23 AND 25 KILSYTH" {
		t.Errorf("expected addr '23 AND 25 KILSYTH', got %q", addr)
	}
	if status != "MSW AND RECYC NOT OUT" {
		t.Errorf("expected status 'MSW AND RECYC NOT OUT', got %q", status)
	}

	// Address with suffix
	addr2, status2 := engine.SplitAddress("45 FOREST ST TRASH NOT OUT")
	if addr2 != "45 FOREST ST" {
		t.Errorf("expected addr '45 FOREST ST', got %q", addr2)
	}
	if status2 != "TRASH NOT OUT" {
		t.Errorf("expected status 'TRASH NOT OUT', got %q", status2)
	}
}

func TestRuleEngineHeuristicsToggle(t *testing.T) {
	cfgWithHeuristics := RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels:           DefaultLabels(),
		Rules:            []ClassificationRule{},
	}

	engineWithHeuristics := NewRuleEngine(cfgWithHeuristics)
	if label := engineWithHeuristics.NormalizeIssueLabel("MATTRESS NOT OUT"); label != "special_item_not_out" {
		t.Errorf("expected special_item_not_out with heuristics enabled, got %q", label)
	}

	cfgWithoutHeuristics := RuleConfig{
		Version:          1,
		EnableHeuristics: false,
		DefaultLabel:     "other",
		Labels:           DefaultLabels(),
		Rules:            []ClassificationRule{},
	}

	engineWithoutHeuristics := NewRuleEngine(cfgWithoutHeuristics)
	if label := engineWithoutHeuristics.NormalizeIssueLabel("MATTRESS NOT OUT"); label != "other" {
		t.Errorf("expected 'other' with heuristics disabled, got %q", label)
	}
}

func TestRuleEngineMetricLookup(t *testing.T) {
	cfg := RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels: []LabelDefinition{
			{Key: "custom_trash", DisplayName: "Custom Trash", Metric: MetricTrash},
			{Key: "custom_recyc", DisplayName: "Custom Recycling", Metric: MetricRecycling},
			{Key: "custom_both", DisplayName: "Custom Both", Metric: MetricBoth},
		},
		Rules: []ClassificationRule{},
	}

	engine := NewRuleEngine(cfg)

	if m := engine.MetricForLabel("custom_trash"); m != MetricTrash {
		t.Errorf("expected MetricTrash, got %v", m)
	}
	if m := engine.MetricForLabel("custom_recyc"); m != MetricRecycling {
		t.Errorf("expected MetricRecycling, got %v", m)
	}
	if m := engine.MetricForLabel("custom_both"); m != MetricBoth {
		t.Errorf("expected MetricBoth, got %v", m)
	}
	if m := engine.MetricForLabel("unknown_label"); m != MetricNone {
		t.Errorf("expected MetricNone for unknown label, got %v", m)
	}
}

func TestConcurrentEngineAccessAndUpdates(t *testing.T) {
	orig := DefaultEngine()
	defer SetDefaultEngine(orig)

	done := make(chan struct{})
	const goroutines = 8

	// Writers continuously update default engine
	for range 2 {
		go func() {
			cfg := DefaultRuleConfig()
			for {
				select {
				case <-done:
					return
				default:
					SetDefaultEngine(NewRuleEngine(cfg))
				}
			}
		}()
	}

	// Readers continuously classify entries and parse records
	meta := messageMetadata{
		Subject: "Tags 01/02/26",
		Body:    "09/22/2025 11:29:05 SSAWALLI\n123 MAIN ST MSW NOT OUT\n45 ELM ST RECYC NOT OUT\n",
	}

	readerDone := make(chan struct{}, goroutines)
	for range goroutines {
		go func() {
			defer func() { readerDone <- struct{}{} }()
			for range 100 {
				_ = parseRecords(meta)
				_, _, _, _ = classifyEntry("100 MAIN ST MSW AND RECYC NOT OUT 0900AM")
				_, _ = splitAddressAndStatus("100 MAIN ST MSW NOT OUT")
				_ = normalizeIssueLabel("RECYC NOT OUT")
			}
		}()
	}

	for range goroutines {
		<-readerDone
	}
	close(done)
}
