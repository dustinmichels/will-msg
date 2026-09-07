package stats

import (
	"reflect"
	"testing"

	"will-msg/internal/config"
	"will-msg/internal/engine"
)

func TestComputeStats(t *testing.T) {
	records := []engine.Record{
		// Day 1
		{
			MessageDate: "2026-01-01T08:00:00Z",
			Label:       "msw_not_out",
		},
		{
			MessageDate: "2026-01-01T08:05:00Z",
			Label:       "recyc_not_out",
		},
		{
			MessageDate: "2026-01-01T08:10:00Z",
			Label:       "msw_and_recyc_not_out",
		},
		{
			MessageDate: "2026-01-01T08:15:00Z",
			Label:       "special_item_not_out",
		},
		// Day 2
		{
			MessageDate: "2026-01-02T09:00:00Z",
			Label:       "msw_not_out",
		},
		{
			MessageDate: "2026-01-02T09:05:00Z",
			Label:       "msw_not_out",
		},
		{
			MessageDate: "2026-01-02T09:10:00Z",
			Label:       "msw_and_recyc_not_out",
		},
		{
			MessageDate: "2026-01-02T09:15:00Z",
			Label:       "other",
		},
	}

	expected := SummaryStats{
		AverageTrashNotOut:     2.5,
		AverageRecyclingNotOut: 1.5,
		Daily: []DailyStats{
			{
				Date:            "2026-01-01",
				TrashNotOut:     2,
				RecyclingNotOut: 2,
			},
			{
				Date:            "2026-01-02",
				TrashNotOut:     3,
				RecyclingNotOut: 1,
			},
		},
	}

	result := ComputeStats(records)

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("ComputeStats() =\n%+v\nexpected:\n%+v", result, expected)
	}
}

func TestComputeStatsWithEngineCustomMetrics(t *testing.T) {
	cfg := config.RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels: []config.LabelDefinition{
			{Key: "custom_trash", DisplayName: "Custom Trash", Metric: config.MetricTrash},
			{Key: "custom_recyc", DisplayName: "Custom Recycling", Metric: config.MetricRecycling},
		},
		Rules: []config.ClassificationRule{},
	}
	eng := engine.NewRuleEngine(cfg)

	records := []engine.Record{
		{
			MessageDate: "2026-01-01T08:00:00Z",
			Label:       "custom_trash",
		},
		{
			MessageDate: "2026-01-01T08:05:00Z",
			Label:       "custom_trash",
		},
		{
			MessageDate: "2026-01-01T08:10:00Z",
			Label:       "custom_recyc",
		},
	}

	result := ComputeStatsWithEngine(records, eng)

	if len(result.Daily) != 1 {
		t.Fatalf("expected 1 daily stat, got %d", len(result.Daily))
	}
	if result.Daily[0].TrashNotOut != 2 {
		t.Errorf("expected TrashNotOut 2, got %d", result.Daily[0].TrashNotOut)
	}
	if result.Daily[0].RecyclingNotOut != 1 {
		t.Errorf("expected RecyclingNotOut 1, got %d", result.Daily[0].RecyclingNotOut)
	}
}
