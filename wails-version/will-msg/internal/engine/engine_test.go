package engine

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"will-msg/internal/config"
	"will-msg/internal/parser"
)

func defaultTestEngine() *RuleEngine {
	return NewRuleEngine(config.DefaultRuleConfig())
}

func TestRuleEngineCustomSubstringPrecedence(t *testing.T) {
	cfg := config.RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels: []config.LabelDefinition{
			{Key: "custom_yard_waste", DisplayName: "Yard Waste", Metric: config.MetricTrash},
			{Key: "recyc_not_out", DisplayName: "Recycling Not Out", Metric: config.MetricRecycling},
		},
		Rules: []config.ClassificationRule{
			{
				ID:      "yard_waste_1",
				Pattern: "YARD WASTE NOT OUT",
				Type:    config.RuleTypeSubstring,
				Label:   "custom_yard_waste",
				Enabled: true,
			},
			{
				ID:      "recyc_not_out_1",
				Pattern: "NOT OUT",
				Type:    config.RuleTypeSubstring,
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
	cfg := config.RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels:           config.DefaultLabels(),
		Rules: []config.ClassificationRule{
			{
				ID:      "general_rule",
				Pattern: "NOT OUT",
				Type:    config.RuleTypeSubstring,
				Label:   "recyc_not_out",
				Enabled: true,
			},
			{
				ID:      "specific_rule",
				Pattern: "MSW AND RECYC NOT OUT",
				Type:    config.RuleTypeSubstring,
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

	cfg.Rules = []config.ClassificationRule{cfg.Rules[1], cfg.Rules[0]}
	engine2 := NewRuleEngine(cfg)
	label2 := engine2.NormalizeIssueLabel("MSW AND RECYC NOT OUT")
	if label2 != "msw_and_recyc_not_out" {
		t.Errorf("expected specific rule to match when placed ahead, got %q", label2)
	}
}

func TestRuleEngineRegexRules(t *testing.T) {
	cfg := config.RuleConfig{
		Version:          1,
		EnableHeuristics: false,
		DefaultLabel:     "other",
		Labels: []config.LabelDefinition{
			{Key: "snow_obstruction", DisplayName: "Snow Obstruction", Metric: config.MetricNone},
		},
		Rules: []config.ClassificationRule{
			{
				ID:      "snow_regex",
				Pattern: `\bSNOW(?:BANK|DRIFT)?\b`,
				Type:    config.RuleTypeRegex,
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
	cfg := config.RuleConfig{
		Version:          1,
		EnableHeuristics: false,
		DefaultLabel:     "other",
		Labels: []config.LabelDefinition{
			{Key: "strict_case", DisplayName: "Strict Case", Metric: config.MetricNone},
			{Key: "default_case", DisplayName: "Default Case", Metric: config.MetricNone},
		},
		Rules: []config.ClassificationRule{
			{
				ID:      "case_sensitive_regex",
				Pattern: `(?-i)\bexactLower\b`,
				Type:    config.RuleTypeRegex,
				Label:   "strict_case",
				Enabled: true,
			},
			{
				ID:      "default_insensitive_regex",
				Pattern: `\bdefaultCase\b`,
				Type:    config.RuleTypeRegex,
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
		{"item with exactLower word", "strict_case"},
		{"item with EXACTLOWER word", "other"},
		{"item with ExactLower word", "other"},
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
	cfg := config.RuleConfig{
		Version:          1,
		EnableHeuristics: false,
		DefaultLabel:     "custom_other",
		Labels:           config.DefaultLabels(),
		Rules: []config.ClassificationRule{
			{
				ID:      "rule_disabled",
				Pattern: "MSW NOT OUT",
				Type:    config.RuleTypeSubstring,
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
	cfg := config.DefaultRuleConfig()
	engine := NewRuleEngine(cfg)

	addr, status := engine.SplitAddress("23 AND 25 KILSYTH MSW AND RECYC NOT OUT")
	if addr != "23 AND 25 KILSYTH" {
		t.Errorf("expected addr '23 AND 25 KILSYTH', got %q", addr)
	}
	if status != "MSW AND RECYC NOT OUT" {
		t.Errorf("expected status 'MSW AND RECYC NOT OUT', got %q", status)
	}

	addr2, status2 := engine.SplitAddress("45 FOREST ST TRASH NOT OUT")
	if addr2 != "45 FOREST ST" {
		t.Errorf("expected addr '45 FOREST ST', got %q", addr2)
	}
	if status2 != "TRASH NOT OUT" {
		t.Errorf("expected status 'TRASH NOT OUT', got %q", status2)
	}
}

func TestRuleEngineHeuristicsToggle(t *testing.T) {
	cfgWithHeuristics := config.RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels:           config.DefaultLabels(),
		Rules:            []config.ClassificationRule{},
	}

	engineWithHeuristics := NewRuleEngine(cfgWithHeuristics)
	if label := engineWithHeuristics.NormalizeIssueLabel("MATTRESS NOT OUT"); label != "special_item_not_out" {
		t.Errorf("expected special_item_not_out with heuristics enabled, got %q", label)
	}

	cfgWithoutHeuristics := config.RuleConfig{
		Version:          1,
		EnableHeuristics: false,
		DefaultLabel:     "other",
		Labels:           config.DefaultLabels(),
		Rules:            []config.ClassificationRule{},
	}

	engineWithoutHeuristics := NewRuleEngine(cfgWithoutHeuristics)
	if label := engineWithoutHeuristics.NormalizeIssueLabel("MATTRESS NOT OUT"); label != "other" {
		t.Errorf("expected 'other' with heuristics disabled, got %q", label)
	}
}

func TestRuleEngineMetricLookup(t *testing.T) {
	cfg := config.RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels: []config.LabelDefinition{
			{Key: "custom_trash", DisplayName: "Custom Trash", Metric: config.MetricTrash},
			{Key: "custom_recyc", DisplayName: "Custom Recycling", Metric: config.MetricRecycling},
			{Key: "custom_both", DisplayName: "Custom Both", Metric: config.MetricBoth},
		},
		Rules: []config.ClassificationRule{},
	}

	engine := NewRuleEngine(cfg)

	if m := engine.MetricForLabel("custom_trash"); m != config.MetricTrash {
		t.Errorf("expected MetricTrash, got %v", m)
	}
	if m := engine.MetricForLabel("custom_recyc"); m != config.MetricRecycling {
		t.Errorf("expected MetricRecycling, got %v", m)
	}
	if m := engine.MetricForLabel("custom_both"); m != config.MetricBoth {
		t.Errorf("expected MetricBoth, got %v", m)
	}
	if m := engine.MetricForLabel("unknown_label"); m != config.MetricNone {
		t.Errorf("expected MetricNone for unknown label, got %v", m)
	}
}

func TestConcurrentEngineAccessAndUpdates(t *testing.T) {
	var currentEngine atomic.Pointer[RuleEngine]
	currentEngine.Store(NewRuleEngine(config.DefaultRuleConfig()))

	done := make(chan struct{})
	const goroutines = 8

	for range 2 {
		go func() {
			cfg := config.DefaultRuleConfig()
			for {
				select {
				case <-done:
					return
				default:
					currentEngine.Store(NewRuleEngine(cfg))
				}
			}
		}()
	}

	meta := parser.MessageMetadata{
		Subject: "Tags 01/02/26",
		Body:    "09/22/2025 11:29:05 SSAWALLI\n123 MAIN ST MSW NOT OUT\n45 ELM ST RECYC NOT OUT\n",
	}

	readerDone := make(chan struct{}, goroutines)
	for range goroutines {
		go func() {
			defer func() { readerDone <- struct{}{} }()
			for range 100 {
				eng := currentEngine.Load()
				_ = eng.ParseRecords(meta)
				_, _, _, _ = eng.Classify("100 MAIN ST MSW AND RECYC NOT OUT 0900AM")
				_, _ = eng.SplitAddress("100 MAIN ST MSW NOT OUT")
				_ = eng.NormalizeIssueLabel("RECYC NOT OUT")
			}
		}()
	}

	for range goroutines {
		<-readerDone
	}
	close(done)
}

func TestNewRuleEngineValidated_Valid(t *testing.T) {
	cfg := config.DefaultRuleConfig()
	engine, err := NewRuleEngineValidated(cfg)
	if err != nil {
		t.Fatalf("expected valid engine, got error: %v", err)
	}
	if engine == nil {
		t.Fatal("expected non-nil engine")
	}

	label := engine.NormalizeIssueLabel("MSW NOT OUT")
	if label != "msw_not_out" {
		t.Errorf("expected 'msw_not_out', got %q", label)
	}
}

func TestNewRuleEngineValidated_InvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		modify func(cfg *config.RuleConfig)
	}{
		{
			name: "invalid regex pattern",
			modify: func(cfg *config.RuleConfig) {
				cfg.Rules = append([]config.ClassificationRule{
					{
						ID:      "broken_regex",
						Type:    config.RuleTypeRegex,
						Pattern: "[unclosed_bracket",
						Label:   "msw_not_out",
						Enabled: true,
					},
				}, cfg.Rules...)
			},
		},
		{
			name: "empty default label",
			modify: func(cfg *config.RuleConfig) {
				cfg.DefaultLabel = ""
			},
		},
		{
			name: "undefined label reference",
			modify: func(cfg *config.RuleConfig) {
				cfg.Rules[0].Label = "non_existent_label_key"
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultRuleConfig()
			tc.modify(&cfg)
			engine, err := NewRuleEngineValidated(cfg)
			if err == nil {
				t.Fatalf("expected validation error for case %q, got nil error and engine %+v", tc.name, engine)
			}
			if engine != nil {
				t.Fatalf("expected nil engine on validation error, got %+v", engine)
			}
		})
	}
}

func TestNewRuleEngine_InvalidRegexDegradation(t *testing.T) {
	cfg := config.DefaultRuleConfig()
	cfg.Rules = []config.ClassificationRule{
		{
			ID:      "bad_regex_rule",
			Type:    config.RuleTypeRegex,
			Pattern: "[unclosed",
			Label:   "msw_not_out",
			Enabled: true,
		},
		{
			ID:      "good_sub_rule",
			Type:    config.RuleTypeSubstring,
			Pattern: "RECYC NOT OUT",
			Label:   "recyc_not_out",
			Enabled: true,
		},
	}

	engine := NewRuleEngine(cfg)
	if engine == nil {
		t.Fatal("expected non-nil engine from NewRuleEngine even with broken regex")
	}

	labelGood := engine.NormalizeIssueLabel("RECYC NOT OUT")
	if labelGood != "recyc_not_out" {
		t.Errorf("expected 'recyc_not_out', got %q", labelGood)
	}

	labelBad := engine.NormalizeIssueLabel("[unclosed")
	if labelBad != "other" {
		t.Errorf("expected fallback 'other', got %q", labelBad)
	}
}

func TestParseRecordsFromSampleMessage(t *testing.T) {
	msgPath := filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg")
	meta, err := parser.LoadMessage(msgPath)
	if err != nil {
		t.Fatalf("loadMessage: %v", err)
	}

	records := defaultTestEngine().ParseRecords(meta)
	if len(records) != 5 {
		t.Fatalf("expected 5 records, got %d", len(records))
	}

	if records[0].Label != "recyc_not_out" {
		t.Fatalf("expected first label recyc_not_out, got %q", records[0].Label)
	}
	if records[2].IssueTime != "0831AM" {
		t.Fatalf("expected third issue time 0831AM, got %q", records[2].IssueTime)
	}
	if records[4].Label != "special_item_not_out" {
		t.Fatalf("expected fifth label special_item_not_out, got %q", records[4].Label)
	}
}

func TestParseRecordsRejoinsWrappedPlaintextRows(t *testing.T) {
	meta := parser.MessageMetadata{
		SourceFile: "test.msg",
		Subject:    "Test subject",
		Body: strings.Join([]string{
			"03/02/2026 07:53:10 SSAWALLI",
			"252,248, 236, 224, 196, 192, 190 , 172, 164, 156, 148, 136 AND 132 SPRI",
			"NG ST RECYC NOT OUT",
			"03/02/2026 15:19:39 SSAWALLI",
			"EVANS ST - TOO MANY PARKED CARS ON BOTH CORNERS AND END OF STREET, UNABL E",
			"TO SVC TRASH",
		}, "\n"),
	}

	records := defaultTestEngine().ParseRecords(meta)
	if len(records) != 14 {
		t.Fatalf("expected 14 records, got %d", len(records))
	}

	wantSpring := []string{
		"252 SPRING ST RECYC NOT OUT",
		"248 SPRING ST RECYC NOT OUT",
		"236 SPRING ST RECYC NOT OUT",
		"224 SPRING ST RECYC NOT OUT",
		"196 SPRING ST RECYC NOT OUT",
		"192 SPRING ST RECYC NOT OUT",
		"190 SPRING ST RECYC NOT OUT",
		"172 SPRING ST RECYC NOT OUT",
		"164 SPRING ST RECYC NOT OUT",
		"156 SPRING ST RECYC NOT OUT",
		"148 SPRING ST RECYC NOT OUT",
		"136 SPRING ST RECYC NOT OUT",
		"132 SPRING ST RECYC NOT OUT",
	}
	for i, want := range wantSpring {
		if records[i].RawEntry != want {
			t.Fatalf("records[%d]: expected %q, got %q", i, want, records[i].RawEntry)
		}
		if records[i].LocationHint != strings.TrimSuffix(want, " RECYC NOT OUT") {
			t.Fatalf("records[%d]: expected location %q, got %q", i, strings.TrimSuffix(want, " RECYC NOT OUT"), records[i].LocationHint)
		}
	}

	wantNarrative := "EVANS ST - TOO MANY PARKED CARS ON BOTH CORNERS AND END OF STREET, UNABLE TO SVC TRASH"
	if records[len(records)-1].RawEntry != wantNarrative {
		t.Fatalf("expected wrapped narrative to rejoin as %q, got %q", wantNarrative, records[len(records)-1].RawEntry)
	}
}

func TestParseRecordsRejoinsWrappedRealMessage(t *testing.T) {
	msgPath := filepath.Join("..", "..", "testdata", "MEDFORD TAGS 03_02_26.msg")
	meta, err := parser.LoadMessage(msgPath)
	if err != nil {
		t.Fatalf("loadMessage: %v", err)
	}

	records := defaultTestEngine().ParseRecords(meta)
	if len(records) != 23 {
		t.Fatalf("expected 23 records, got %d", len(records))
	}
	if records[0].RawEntry != "252 SPRING ST RECYC NOT OUT" {
		t.Fatalf("unexpected first wrapped entry %q", records[0].RawEntry)
	}
	if records[len(records)-1].RawEntry != "EVANS ST - TOO MANY PARKED CARS ON BOTH CORNERS AND END OF STREET, UNABLE TO SVC TRASH" {
		t.Fatalf("unexpected wrapped final entry %q", records[len(records)-1].RawEntry)
	}

	for _, rec := range records {
		if rec.RawEntry == "NG ST RECYC NOT OUT" || strings.HasSuffix(rec.RawEntry, " SPRI") {
			t.Fatalf("wrapped fragment leaked into parsed output: %q", rec.RawEntry)
		}
	}
}

func TestParseRecordsSplitsWideGapAddresses(t *testing.T) {
	meta := parser.MessageMetadata{
		SourceFile: "test.msg",
		Subject:    "Test subject",
		Body: "07/01/2025 14:34:19\n" +
			"4 MAYNARD ST                                    171B FOREST ST",
	}

	records := defaultTestEngine().ParseRecords(meta)
	if len(records) != 2 {
		entries := make([]string, len(records))
		for i, r := range records {
			entries[i] = r.RawEntry
		}
		t.Fatalf("expected 2 records for wide-gap line, got %d: %v", len(records), entries)
	}
	if records[0].RawEntry != "4 MAYNARD ST" {
		t.Errorf("records[0]: expected %q, got %q", "4 MAYNARD ST", records[0].RawEntry)
	}
	if records[1].RawEntry != "171B FOREST ST" {
		t.Errorf("records[1]: expected %q, got %q", "171B FOREST ST", records[1].RawEntry)
	}
	for _, r := range records {
		if r.ParsedIssue != "" || r.Label != "" {
			t.Errorf("expected empty issue/label for %q, got issue=%q label=%q",
				r.RawEntry, r.ParsedIssue, r.Label)
		}
	}
}

func TestParseRecordsTrailingWhitespaceBlocksMerge(t *testing.T) {
	meta := parser.MessageMetadata{
		SourceFile: "test.msg",
		Subject:    "Test subject",
		Body: "07/01/2025 09:28:01\n" +
			"23 MAPLE ST DRIVER REPORTED      \n" +
			"COULD NOT ACCESS PROPERTY",
	}

	records := defaultTestEngine().ParseRecords(meta)
	if len(records) != 2 {
		entries := make([]string, len(records))
		for i, r := range records {
			entries[i] = r.RawEntry
		}
		t.Fatalf("trailing whitespace must block merge; expected 2 records, got %d: %v", len(records), entries)
	}
	if records[0].RawEntry != "23 MAPLE ST DRIVER REPORTED" {
		t.Errorf("records[0]: expected %q, got %q", "23 MAPLE ST DRIVER REPORTED", records[0].RawEntry)
	}
	if records[1].RawEntry != "COULD NOT ACCESS PROPERTY" {
		t.Errorf("records[1]: expected %q, got %q", "COULD NOT ACCESS PROPERTY", records[1].RawEntry)
	}
}

func TestClassifyEntry(t *testing.T) {
	engine := defaultTestEngine()
	tests := []struct {
		name          string
		input         string
		expectedLoc   string
		expectedLabel string
		expectedTime  string
	}{
		{
			name:          "original case",
			input:         "42 WOBURN ST MSW NOT OUT 1102AM",
			expectedLoc:   "42 WOBURN ST",
			expectedLabel: "msw_not_out",
			expectedTime:  "1102AM",
		},
		{
			name:          "trash not out maps to msw",
			input:         "32 WALNUT ST TRASH NOT OUT 06261029 AM",
			expectedLoc:   "32 WALNUT ST",
			expectedLabel: "msw_not_out",
			expectedTime:  "",
		},
		{
			name:          "harvard ave apt",
			input:         "118 HARVARD AVE APT 2, AREA RUG NOT OUT",
			expectedLoc:   "118 HARVARD AVE APT 2",
			expectedLabel: "special_item_not_out",
			expectedTime:  "",
		},
		{
			name:          "sharon st microwave",
			input:         "161 SHARON ST, MICROWAVE NOT OUT",
			expectedLoc:   "161 SHARON ST",
			expectedLabel: "special_item_not_out",
			expectedTime:  "",
		},
		{
			name:          "colby st blocked",
			input:         "15-17 COLBY ST BLOCKED BY CAR AND SNOW - MSW NOT SERVICED",
			expectedLoc:   "15-17 COLBY ST",
			expectedLabel: "blocked",
			expectedTime:  "",
		},
		{
			name:          "ship ave green condos",
			input:         "SHIP AVE GREEN CONDOS - DUMPSTER BLOCKED BY CAR. RCN TKT PLACED FOR TOM",
			expectedLoc:   "SHIP AVE GREEN CONDOS",
			expectedLabel: "blocked",
			expectedTime:  "",
		},
		{
			name:          "strathmore rd blocking toters",
			input:         "8 STRATHMORE RD - CARS ARE BLOCKING TOTERS MSW NOT SVCD",
			expectedLoc:   "8 STRATHMORE RD",
			expectedLabel: "blocked",
			expectedTime:  "",
		},
		{
			name:          "washington st ac",
			input:         "198 WASHINGTON ST APT 1, AC NOT OUT",
			expectedLoc:   "198 WASHINGTON ST APT 1",
			expectedLabel: "special_item_not_out",
			expectedTime:  "",
		},
		{
			name:          "hamilton st ac bare date code",
			input:         "15 HAMILTON ST APT 1 AC- NOT OUT 0909",
			expectedLoc:   "15 HAMILTON ST APT 1",
			expectedLabel: "special_item_not_out",
			expectedTime:  "",
		},
		{
			name:          "washington st sofa yard",
			input:         "60 WASHINGTON ST SOFA IS ON YARD NOT ON CURB, NOT SVCD",
			expectedLoc:   "60 WASHINGTON ST",
			expectedLabel: "other",
			expectedTime:  "",
		},
		{
			name:          "winthrop st unable to svc msw",
			input:         "555 WINTHROP ST... UNABLE TO SVC MSW",
			expectedLoc:   "555 WINTHROP ST",
			expectedLabel: "other",
			expectedTime:  "",
		},
		{
			name:          "charnwood rd many homes",
			input:         "CHARNWOOD RD MANY HOMES RECYC NOT OUT",
			expectedLoc:   "CHARNWOOD RD MANY HOMES",
			expectedLabel: "recyc_not_out",
			expectedTime:  "",
		},
		{
			name:          "traincroft reversed recyc status",
			input:         "10 TRAINCROFT NOT OUT RECYC",
			expectedLoc:   "10 TRAINCROFT",
			expectedLabel: "recyc_not_out",
			expectedTime:  "",
		},
		{
			name:          "recycle spelled out maps to recyc",
			input:         "108 MAGOUN AVE RECYCLE NOT OUT",
			expectedLoc:   "108 MAGOUN AVE",
			expectedLabel: "recyc_not_out",
			expectedTime:  "",
		},
		{
			name:          "recyccle typo maps to recyc",
			input:         "135 HIGH ST RECYCCLE NOT OUT ON 0709 910 AM",
			expectedLoc:   "135 HIGH ST",
			expectedLabel: "recyc_not_out",
			expectedTime:  "",
		},
		{
			name:          "rcy not out maps to recyc_not_out",
			input:         "108 MAGOUN AVE RCY NOT OUT",
			expectedLoc:   "108 MAGOUN AVE",
			expectedLabel: "recyc_not_out",
			expectedTime:  "",
		},
		{
			name:          "not out rcy maps to recyc_not_out",
			input:         "10 TRAINCROFT NOT OUT RCY",
			expectedLoc:   "10 TRAINCROFT",
			expectedLabel: "recyc_not_out",
			expectedTime:  "",
		},
		{
			name:          "trash and rcy not out",
			input:         "44 MITCHELL TRASH AND RCY NOT OUT",
			expectedLoc:   "44 MITCHELL",
			expectedLabel: "msw_and_recyc_not_out",
			expectedTime:  "",
		},
		{
			name:          "rcy and msw not out",
			input:         "42 WOBURN ST RCY AND MSW NOT OUT",
			expectedLoc:   "42 WOBURN ST",
			expectedLabel: "msw_and_recyc_not_out",
			expectedTime:  "",
		},
		{
			name:          "combined status reversed order",
			input:         "42 WOBURN ST RECYC AND MSW NOT OUT",
			expectedLoc:   "42 WOBURN ST",
			expectedLabel: "msw_and_recyc_not_out",
			expectedTime:  "",
		},
		{
			name:          "winthrop st private way road icey",
			input:         "555 WINTHROP ST, PRIVATE WAY, ROAD IS TOO ICEY, UNABLE TO SVC MSW",
			expectedLoc:   "555 WINTHROP ST, PRIVATE WAY",
			expectedLabel: "other",
			expectedTime:  "",
		},
		{
			name:          "elm st coffee table",
			input:         "16 ELM ST , COFFE TABLE AND DESK NOT OUT",
			expectedLoc:   "16 ELM ST",
			expectedLabel: "special_item_not_out",
			expectedTime:  "",
		},
		{
			name:          "ship ave green condos en-dash",
			input:         "SHIP AVE – GREEN CONDOS – RECYCLE NOT SVCD",
			expectedLoc:   "SHIP AVE - GREEN CONDOS",
			expectedLabel: "other",
			expectedTime:  "",
		},
		{
			name:          "summit rd area rug",
			input:         "92 SUMMIT RD AREA RUG NOT OUT",
			expectedLoc:   "92 SUMMIT RD",
			expectedLabel: "special_item_not_out",
			expectedTime:  "",
		},
		{
			name:          "bowdoin st bed frame chair",
			input:         "131 BOWDOIN ST BED FRAME, UPHOLSTERED CHAIR- NOT OUT",
			expectedLoc:   "131 BOWDOIN ST",
			expectedLabel: "special_item_not_out",
			expectedTime:  "",
		},
		{
			name:          "park st all units",
			input:         "60 PARK ST - ALL UNITS - RECYC NOT OUT",
			expectedLoc:   "60 PARK ST - ALL UNITS",
			expectedLabel: "recyc_not_out",
			expectedTime:  "",
		},
		{
			name:          "recy contam w non acceptable items left behind",
			input:         "123 MAIN ST RECY CONTAM W NONN ACCEPTABLE ITEMS IN BIN- LEFT BHND",
			expectedLoc:   "123 MAIN ST",
			expectedLabel: "recyc_contaminated",
			expectedTime:  "",
		},
		{
			name:          "recy contam w unacceptable materials left behind",
			input:         "45 BROADWAY RECY CONTAM W UNACCEPTABLE MATERIALS IN BIN- LEFT BHND",
			expectedLoc:   "45 BROADWAY",
			expectedLabel: "recyc_contaminated",
			expectedTime:  "",
		},
		{
			name:          "recy contam w wood left behind",
			input:         "12 COLBY ST RECY CONTAM W WOOD IN BIN- LEFT BHND",
			expectedLoc:   "12 COLBY ST",
			expectedLabel: "recyc_contaminated",
			expectedTime:  "",
		},
		{
			name:          "recycle contam w trash left behind",
			input:         "78 OAK AVE RECYCLE CONTAM W TRASH IN BIN- LEFT BHND",
			expectedLoc:   "78 OAK AVE",
			expectedLabel: "recyc_contaminated",
			expectedTime:  "",
		},
		{
			name:          "contaminated recyc",
			input:         "99 ELM RD CONTAMINATED RECYC",
			expectedLoc:   "99 ELM RD",
			expectedLabel: "recyc_contaminated",
			expectedTime:  "",
		},
		{
			name:          "recycling contaminated not picked up",
			input:         "14 MAPLE LN, RECYCLING CONTAMINATED, NOT PICKED UP",
			expectedLoc:   "14 MAPLE LN",
			expectedLabel: "recyc_contaminated",
			expectedTime:  "",
		},
		{
			name:          "rcy contam",
			input:         "12 COLBY ST RCY CONTAM W WOOD IN BIN- LEFT BHND",
			expectedLoc:   "12 COLBY ST",
			expectedLabel: "recyc_contaminated",
			expectedTime:  "",
		},
		{
			name:          "contaminated rcy",
			input:         "99 ELM RD CONTAMINATED RCY",
			expectedLoc:   "99 ELM RD",
			expectedLabel: "recyc_contaminated",
			expectedTime:  "",
		},
		{
			name:          "rcy contaminated",
			input:         "14 MAPLE LN, RCY CONTAMINATED, NOT PICKED UP",
			expectedLoc:   "14 MAPLE LN",
			expectedLabel: "recyc_contaminated",
			expectedTime:  "",
		},
		{
			name:          "contaminated recycling",
			input:         "250 HIGH ST CONTAMINATED RECYCLING",
			expectedLoc:   "250 HIGH ST",
			expectedLabel: "recyc_contaminated",
			expectedTime:  "",
		},
		{
			name:          "msw overloaded",
			input:         "50 CUSHING ST - MSW OVERLOADED",
			expectedLoc:   "50 CUSHING ST",
			expectedLabel: "overflowing",
			expectedTime:  "",
		},
		{
			name:          "msw overflowing",
			input:         "46 SPRINGS ST - MSW OVERFLOWING",
			expectedLoc:   "46 SPRINGS ST",
			expectedLabel: "overflowing",
			expectedTime:  "",
		},
		{
			name:          "trash can overloaded",
			input:         "356 BOSTON AVE TRASH CAN OVERLOADED- LEFT BHND",
			expectedLoc:   "356 BOSTON AVE",
			expectedLabel: "overflowing",
			expectedTime:  "",
		},
		{
			name:          "recycle bin overloaded",
			input:         "81 MAGOUN ST RECYCLE BIN OVERLOADED/MULTIPLE BOXES ON GROUND-LEFT 7 AM",
			expectedLoc:   "81 MAGOUN ST",
			expectedLabel: "overflowing",
			expectedTime:  "",
		},
		{
			name:          "trash overflowing",
			input:         "10 MAIN ST TRASH OVERFLOWING",
			expectedLoc:   "10 MAIN ST",
			expectedLabel: "overflowing",
			expectedTime:  "",
		},
		{
			name:          "overflow bags should remain other",
			input:         "15 BOYNTON RD - INCORRECT OVERFLOW BAGS USED FOR MSW, NOT SVCD",
			expectedLoc:   "15 BOYNTON RD",
			expectedLabel: "other",
			expectedTime:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loc, _, label, issueTime := engine.Classify(tc.input)
			if loc != tc.expectedLoc {
				t.Errorf("expected location %q, got %q", tc.expectedLoc, loc)
			}
			if label != tc.expectedLabel {
				t.Errorf("expected label %q, got %q", tc.expectedLabel, label)
			}
			if issueTime != tc.expectedTime {
				t.Errorf("expected issueTime %q, got %q", tc.expectedTime, issueTime)
			}
		})
	}
}

func TestClassifyEntryReturnsParsedIssueAndLabel(t *testing.T) {
	loc, parsedIssue, label, issueTime := defaultTestEngine().Classify("473 BORN COURT APTS, BROADWAY ST, MSW AND RECYC NOT OUT")
	if loc != "473 BORN COURT APTS, BROADWAY ST" {
		t.Fatalf("expected location to preserve address, got %q", loc)
	}
	if parsedIssue != "MSW AND RECYC NOT OUT" {
		t.Fatalf("expected parsed issue %q, got %q", "MSW AND RECYC NOT OUT", parsedIssue)
	}
	if label != "msw_and_recyc_not_out" {
		t.Fatalf("expected label %q, got %q", "msw_and_recyc_not_out", label)
	}
	if issueTime != "" {
		t.Fatalf("expected empty issue time, got %q", issueTime)
	}
}

func TestParseRecordsSplitList(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected []struct {
			rawEntry     string
			locationHint string
			label        string
			issueTime    string
		}
	}{
		{
			name: "multiple houses comma and and",
			body: "01/02/2026 08:30:00 dispatcher1\n7, 21 AND 26 HILLSIDE AVE RECYC NOT OUT 0831AM",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "7 HILLSIDE AVE RECYC NOT OUT 0831AM",
					locationHint: "7 HILLSIDE AVE",
					label:        "recyc_not_out",
					issueTime:    "0831AM",
				},
				{
					rawEntry:     "21 HILLSIDE AVE RECYC NOT OUT 0831AM",
					locationHint: "21 HILLSIDE AVE",
					label:        "recyc_not_out",
					issueTime:    "0831AM",
				},
				{
					rawEntry:     "26 HILLSIDE AVE RECYC NOT OUT 0831AM",
					locationHint: "26 HILLSIDE AVE",
					label:        "recyc_not_out",
					issueTime:    "0831AM",
				},
			},
		},
		{
			name: "ampersand separator",
			body: "01/02/2026 08:30:00 dispatcher1\n8 & 14 CURTIS ST RECYC NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "8 CURTIS ST RECYC NOT OUT",
					locationHint: "8 CURTIS ST",
					label:        "recyc_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "14 CURTIS ST RECYC NOT OUT",
					locationHint: "14 CURTIS ST",
					label:        "recyc_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "ampersand html entity separator",
			body: "01/02/2026 08:30:00 dispatcher1\n8 &amp; 14 CURTIS ST RECYC NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "8 CURTIS ST RECYC NOT OUT",
					locationHint: "8 CURTIS ST",
					label:        "recyc_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "14 CURTIS ST RECYC NOT OUT",
					locationHint: "14 CURTIS ST",
					label:        "recyc_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "tricky comma street name",
			body: "01/02/2026 08:30:00 dispatcher1\n473 AND 476 BORN COURT APTS, BROADWAY ST, MSW AND RECYC NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "473 BORN COURT APTS, BROADWAY ST, MSW AND RECYC NOT OUT",
					locationHint: "473 BORN COURT APTS, BROADWAY ST",
					label:        "msw_and_recyc_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "476 BORN COURT APTS, BROADWAY ST, MSW AND RECYC NOT OUT",
					locationHint: "476 BORN COURT APTS, BROADWAY ST",
					label:        "msw_and_recyc_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "blocking toters list stays blocked",
			body: "01/02/2026 08:30:00 dispatcher1\n8, 10 AND 12 STRATHMORE RD - CARS ARE BLOCKING TOTERS MSW NOT SVCD",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "8 STRATHMORE RD - CARS ARE BLOCKING TOTERS MSW NOT SVCD",
					locationHint: "8 STRATHMORE RD",
					label:        "blocked",
					issueTime:    "",
				},
				{
					rawEntry:     "10 STRATHMORE RD - CARS ARE BLOCKING TOTERS MSW NOT SVCD",
					locationHint: "10 STRATHMORE RD",
					label:        "blocked",
					issueTime:    "",
				},
				{
					rawEntry:     "12 STRATHMORE RD - CARS ARE BLOCKING TOTERS MSW NOT SVCD",
					locationHint: "12 STRATHMORE RD",
					label:        "blocked",
					issueTime:    "",
				},
			},
		},
		{
			name: "no leading numbers to split",
			body: "01/02/2026 08:30:00 dispatcher1\nCHARNWOOD RD MANY HOMES RECYC NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "CHARNWOOD RD MANY HOMES RECYC NOT OUT",
					locationHint: "CHARNWOOD RD MANY HOMES",
					label:        "recyc_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "trailing and house number stays in same list",
			body: "01/02/2026 08:30:00 dispatcher1\n8, 12,16, 19 ,20,22 32, 40, AND 44 POWDER HOUSE RD EXT MSW NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "8 POWDER HOUSE RD EXT MSW NOT OUT",
					locationHint: "8 POWDER HOUSE RD",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "12 POWDER HOUSE RD EXT MSW NOT OUT",
					locationHint: "12 POWDER HOUSE RD",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "16 POWDER HOUSE RD EXT MSW NOT OUT",
					locationHint: "16 POWDER HOUSE RD",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "19 POWDER HOUSE RD EXT MSW NOT OUT",
					locationHint: "19 POWDER HOUSE RD",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "20 POWDER HOUSE RD EXT MSW NOT OUT",
					locationHint: "20 POWDER HOUSE RD",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "22 POWDER HOUSE RD EXT MSW NOT OUT",
					locationHint: "22 POWDER HOUSE RD",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "32 POWDER HOUSE RD EXT MSW NOT OUT",
					locationHint: "32 POWDER HOUSE RD",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "40 POWDER HOUSE RD EXT MSW NOT OUT",
					locationHint: "40 POWDER HOUSE RD",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "44 POWDER HOUSE RD EXT MSW NOT OUT",
					locationHint: "44 POWDER HOUSE RD",
					label:        "msw_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "suffixless street reversed recyc status",
			body: "01/02/2026 08:30:00 dispatcher1\n10, 15 AND 30 TRAINCROFT NOT OUT RECYC",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "10 TRAINCROFT NOT OUT RECYC",
					locationHint: "10 TRAINCROFT",
					label:        "recyc_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "15 TRAINCROFT NOT OUT RECYC",
					locationHint: "15 TRAINCROFT",
					label:        "recyc_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "30 TRAINCROFT NOT OUT RECYC",
					locationHint: "30 TRAINCROFT",
					label:        "recyc_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "dot-separated house list",
			body: "01/02/2026 08:30:00 dispatcher1\n152. 156. 158. 212. 214 HIGH ST. MSW NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "152 HIGH ST. MSW NOT OUT",
					locationHint: "152 HIGH ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "156 HIGH ST. MSW NOT OUT",
					locationHint: "156 HIGH ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "158 HIGH ST. MSW NOT OUT",
					locationHint: "158 HIGH ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "212 HIGH ST. MSW NOT OUT",
					locationHint: "212 HIGH ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "214 HIGH ST. MSW NOT OUT",
					locationHint: "214 HIGH ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "space and dot separated house list",
			body: "01/02/2026 08:30:00 dispatcher1\n44 46. 47. 51. 55. ALLSTON ST. MSW NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "44 ALLSTON ST. MSW NOT OUT",
					locationHint: "44 ALLSTON ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "46 ALLSTON ST. MSW NOT OUT",
					locationHint: "46 ALLSTON ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "47 ALLSTON ST. MSW NOT OUT",
					locationHint: "47 ALLSTON ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "51 ALLSTON ST. MSW NOT OUT",
					locationHint: "51 ALLSTON ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
				{
					rawEntry:     "55 ALLSTON ST. MSW NOT OUT",
					locationHint: "55 ALLSTON ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "ordinal street name stays single row",
			body: "01/02/2026 08:30:00 dispatcher1\n97 3RD ST MSW NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "97 3RD ST MSW NOT OUT",
					locationHint: "97 3RD ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "first street name stays single row",
			body: "01/02/2026 08:30:00 dispatcher1\n53 1ST ST MSW NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "53 1ST ST MSW NOT OUT",
					locationHint: "53 1ST ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "fourth street name stays single row",
			body: "01/02/2026 08:30:00 dispatcher1\n95 4TH ST MSW NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "95 4TH ST MSW NOT OUT",
					locationHint: "95 4TH ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "seventh street first entry stays single row",
			body: "01/02/2026 08:30:00 dispatcher1\n46 7TH ST MSW NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "46 7TH ST MSW NOT OUT",
					locationHint: "46 7TH ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
			},
		},
		{
			name: "seventh street second entry stays single row",
			body: "01/02/2026 08:30:00 dispatcher1\n48 7TH ST MSW NOT OUT",
			expected: []struct {
				rawEntry     string
				locationHint string
				label        string
				issueTime    string
			}{
				{
					rawEntry:     "48 7TH ST MSW NOT OUT",
					locationHint: "48 7TH ST",
					label:        "msw_not_out",
					issueTime:    "",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := parser.MessageMetadata{
				SourceFile: "test.msg",
				Subject:    "Test subject",
				Body:       tc.body,
			}
			records := defaultTestEngine().ParseRecords(meta)
			if len(records) != len(tc.expected) {
				t.Fatalf("expected %d records, got %d", len(tc.expected), len(records))
			}
			for i, r := range records {
				exp := tc.expected[i]
				if r.RawEntry != exp.rawEntry {
					t.Errorf("records[%d]: expected RawEntry %q, got %q", i, exp.rawEntry, r.RawEntry)
				}
				if r.LocationHint != exp.locationHint {
					t.Errorf("records[%d]: expected LocationHint %q, got %q", i, exp.locationHint, r.LocationHint)
				}
				if r.Label != exp.label {
					t.Errorf("records[%d]: expected Label %q, got %q", i, exp.label, r.Label)
				}
				if r.IssueTime != exp.issueTime {
					t.Errorf("records[%d]: expected IssueTime %q, got %q", i, exp.issueTime, r.IssueTime)
				}
				if r.RowInMessage != 1 {
					t.Errorf("records[%d]: expected RowInMessage %d, got %d", i, 1, r.RowInMessage)
				}
			}
		})
	}
}

func TestParseRecordsStopsAtSignatureBlock(t *testing.T) {
	tests := []struct {
		name      string
		signature string
	}{
		{name: "regards", signature: "Regards,"},
		{name: "thank you", signature: "Thank You"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta := parser.MessageMetadata{
				SourceFile: "test.msg",
				Subject:    "Test subject",
				Body: strings.Join([]string{
					"01/02/2026 08:30:00 dispatcher1",
					"23 BELL ST TRASH NOT OUT 950AM",
					tc.signature,
					"Lisa Rios",
					"Dispatcher for WOBURN 209",
					"New England Area Operations Center",
				}, "\n"),
			}

			records := defaultTestEngine().ParseRecords(meta)
			if len(records) != 1 {
				t.Fatalf("expected 1 record before signature, got %d", len(records))
			}
			if records[0].RawEntry != "23 BELL ST TRASH NOT OUT 950AM" {
				t.Fatalf("expected structured entry before signature, got %q", records[0].RawEntry)
			}
		})
	}
}

func TestParseRecordsAcceptsTimestampWithoutDispatcher(t *testing.T) {
	meta := parser.MessageMetadata{
		SourceFile: "test.msg",
		Subject:    "Test subject",
		Body: strings.Join([]string{
			"Please see TAGS called in today.",
			"06/27/2025 12:00:41",
			"139 SHARON ST BULK ITEM NOT OUT ON 0626 928 AM",
		}, "\n"),
	}

	records := defaultTestEngine().ParseRecords(meta)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].RawEntry != "139 SHARON ST BULK ITEM NOT OUT ON 0626 928 AM" {
		t.Fatalf("unexpected RawEntry %q", records[0].RawEntry)
	}
	if records[0].Dispatcher != "" {
		t.Fatalf("expected empty dispatcher, got %q", records[0].Dispatcher)
	}
	if records[0].ReportedAt == "" {
		t.Fatal("expected reported time to be set")
	}
}

func TestParseRecordsSkipsPreambleFooterUntilFirstTimestamp(t *testing.T) {
	meta := parser.MessageMetadata{
		SourceFile: "test.msg",
		Subject:    "Test subject",
		Body: strings.Join([]string{
			"Hello,",
			"I have included tags for 11/17 and 11/18.",
			"Thank you",
			"11/17/2025 06:36:20 SSAWALLI",
			"574 FULTON ST HOT WATER HEATER NOT OUT",
			"40 FOSTER CT FRIDGE NOT OUT",
			"Sheri Sawallich",
			"Dispatcher/Router",
		}, "\n"),
	}

	records := defaultTestEngine().ParseRecords(meta)
	if len(records) != 2 {
		t.Fatalf("expected 2 records after preamble, got %d", len(records))
	}
	if records[0].RawEntry != "574 FULTON ST HOT WATER HEATER NOT OUT" {
		t.Fatalf("unexpected first RawEntry %q", records[0].RawEntry)
	}
	if records[1].RawEntry != "40 FOSTER CT FRIDGE NOT OUT" {
		t.Fatalf("unexpected second RawEntry %q", records[1].RawEntry)
	}
	if records[0].Dispatcher != "SSAWALLI" {
		t.Fatalf("expected dispatcher SSAWALLI, got %q", records[0].Dispatcher)
	}
}

func TestValidateAllCSVSamples(t *testing.T) {
	file, err := os.Open(filepath.Join("..", "..", "testdata", "msg_parsed.csv"))
	if err != nil {
		t.Skip("sample CSV not found")
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to read csv: %v", err)
	}

	statusTokens := []string{"NOT OUT", "NOT SVCD", "BLOCKED", "STILL IN", "ICEY"}
	failCount := 0
	eng := defaultTestEngine()
	addressPrefixRE := regexp.MustCompile(`^\d+(?:-\d+)?\b`)

	for i, row := range records {
		if i == 0 {
			continue
		}
		rawEntry := row[6]
		isRealAddress := addressPrefixRE.MatchString(rawEntry) || FindLastAddressIndex(rawEntry) != -1
		if !isRealAddress || parser.IsFooterLine(rawEntry) {
			continue
		}

		loc, _, label, _ := eng.Classify(rawEntry)

		hasStatusToken := false
		var matchedToken string
		upperRaw := strings.ToUpper(rawEntry)
		for _, tok := range statusTokens {
			if strings.Contains(upperRaw, tok) {
				hasStatusToken = true
				matchedToken = tok
				break
			}
		}

		if hasStatusToken {
			upperLoc := strings.ToUpper(loc)
			if strings.Contains(upperLoc, matchedToken) {
				t.Errorf("Row %d: LocationHint still contains status token %q: Loc=%q, Raw=%q", i+1, matchedToken, loc, rawEntry)
				failCount++
			}

			if label == "" {
				t.Errorf("Row %d: Label is empty for entry with status token: Raw=%q", i+1, rawEntry)
				failCount++
			}
		}
	}

	t.Logf("Validated %d CSV rows. Total failures: %d", len(records)-1, failCount)
}

func TestParseRecordsRejoinsWrappedParagraph(t *testing.T) {
	meta := parser.MessageMetadata{
		SourceFile: "test.msg",
		Subject:    "Test subject",
		Body: "09/10/2025 13:02:32\n" +
			"84 BICKNELL RD SCHEDULED WGC TKT#296774 ONLINE FOR A FRIDGE PICK UP AND  \n" +
			"WHEN THEY SCHEDULE ONLINE THE INSTRUCTIONS ARE GIVEN TO ENSURE THE INSID \n" +
			"ES ARE BUNDLED TOGETHER AND SAFEY STORED INIDE TO PREVENT ACCIDENTS BUT  \n" +
			"WHEN DRVR ARRIVED HERE THE FRIDGE WAS LAYING ON THE LAWN & WHEN HE WENT  \n" +
			"TO STAND IT UP AND LIFT IT THE INSERTS ALONG W GLASS FELL OUT/SPILLED ON \n" +
			"TO THE LAWN/DRIVEWAY AREA-SAME SPOT THE FRIDGE WAS IN- DRIVER DID NOT CL \n" +
			"EAN UP AND IS NOT RESPONSIBLE AS CUST DISREGARDED THE INSTRUCTIONS AND T \n" +
			"HESE ITEMS ARE NOT ACCEPTABLE FOR PICK UP-0 WHITE GOOD PICK UP TURNED IN \n" +
			"TO A SAFETY CONCERN WHEN GLASS FELL OUT AND BROKE AS THEY WERE GETTING READY TO LIFT     \n" +
			"DRIVER SOUNDED UPSET & PANICKED      ",
	}

	records := defaultTestEngine().ParseRecords(meta)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	want := "84 BICKNELL RD SCHEDULED WGC TKT#296774 ONLINE FOR A FRIDGE PICK UP AND WHEN THEY SCHEDULE ONLINE THE INSTRUCTIONS ARE GIVEN TO ENSURE THE INSIDES ARE BUNDLED TOGETHER AND SAFEY STORED INIDE TO PREVENT ACCIDENTS BUT WHEN DRVR ARRIVED HERE THE FRIDGE WAS LAYING ON THE LAWN & WHEN HE WENT TO STAND IT UP AND LIFT IT THE INSERTS ALONG W GLASS FELL OUT/SPILLED ON TO THE LAWN/DRIVEWAY AREA-SAME SPOT THE FRIDGE WAS IN- DRIVER DID NOT CLEAN UP AND IS NOT RESPONSIBLE AS CUST DISREGARDED THE INSTRUCTIONS AND THESE ITEMS ARE NOT ACCEPTABLE FOR PICK UP-0 WHITE GOOD PICK UP TURNED IN TO A SAFETY CONCERN WHEN GLASS FELL OUT AND BROKE AS THEY WERE GETTING READY TO LIFT DRIVER SOUNDED UPSET & PANICKED"
	if records[0].RawEntry != want {
		t.Errorf("expected joined entry:\n%q\ngot:\n%q", want, records[0].RawEntry)
	}
	if records[0].LocationHint != "84 BICKNELL RD" {
		t.Errorf("expected LocationHint %q, got %q", "84 BICKNELL RD", records[0].LocationHint)
	}
}

func TestParseRecordsMedfordTags(t *testing.T) {
	meta := parser.MessageMetadata{
		SourceFile: "test.msg",
		Subject:    "Test subject",
		Body: "09/22/2025 11:29:05\n" +
			"73 MEDFORD ST BULK ITEM NOT OUT ON 0919 AND DRVR CHECKED EVERYWHERE CURB \n\n" +
			"SIDE                                                                     \n" +
			"73 MEDFORD ST BULK MPU NOT OUT AT CURB - ALL ITEMS MUST BE OUT AT CURB A \n\n" +
			"ND NOT ON PROPERTY                                                       ",
	}

	records := defaultTestEngine().ParseRecords(meta)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestRecordJSONSerializationContract(t *testing.T) {
	rec := Record{
		SourceFile:   "test.msg",
		Subject:      "Subject",
		MessageDate:  "01/02/2026",
		ReportedAt:   "01/02/2026 10:00",
		Dispatcher:   "Disp",
		RowInMessage: 1,
		RawEntry:     "123 MAIN ST TRASH NOT OUT",
		LocationHint: "123 MAIN ST",
		ParsedIssue:  "TRASH NOT OUT",
		Label:        "msw_not_out",
		IssueTime:    "1000AM",
	}

	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var rawMap map[string]any
	if err := json.Unmarshal(data, &rawMap); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expectedKeys := make(map[string]bool)
	for _, h := range CSVHeaders {
		expectedKeys[h] = true
	}

	if len(rawMap) != len(expectedKeys) {
		t.Errorf("expected %d JSON keys matching CSVHeaders, got %d (%v)", len(expectedKeys), len(rawMap), rawMap)
	}

	for k := range rawMap {
		if !expectedKeys[k] {
			t.Errorf("unexpected JSON key %q found in marshalled Record", k)
		}
	}
	for _, h := range CSVHeaders {
		if _, ok := rawMap[h]; !ok {
			t.Errorf("missing expected JSON key %q matching CSV header in marshalled Record", h)
		}
	}
}
