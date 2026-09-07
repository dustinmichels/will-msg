package main

import (
	"reflect"
	"testing"
)

func TestComputeStats(t *testing.T) {
	records := []record{
		// Day 1
		{
			MessageDate: "2026-01-01T12:00:00Z",
			Label:       "msw_not_out",
		},
		{
			MessageDate: "2026-01-01T13:00:00Z",
			Label:       "msw_and_recyc_not_out",
		},
		{
			MessageDate: "2026-01-01T14:00:00Z",
			Label:       "recyc_not_out",
		},
		{
			MessageDate: "2026-01-01T15:00:00Z",
			Label:       "other", // should be ignored in stats
		},
		// Day 2
		{
			MessageDate: "2026-01-02T12:00:00Z",
			Label:       "msw_not_out",
		},
		{
			MessageDate: "2026-01-02T13:00:00Z",
			Label:       "msw_not_out",
		},
		{
			MessageDate: "2026-01-02T14:00:00Z",
			Label:       "msw_and_recyc_not_out",
		},
	}

	// Expected values:
	// Day 1 (2026-01-01):
	// - TrashNotOut: msw_not_out (1) + msw_and_recyc_not_out (1) = 2
	// - RecyclingNotOut: recyc_not_out (1) + msw_and_recyc_not_out (1) = 2
	// Day 2 (2026-01-02):
	// - TrashNotOut: msw_not_out (2) + msw_and_recyc_not_out (1) = 3
	// - RecyclingNotOut: recyc_not_out (0) + msw_and_recyc_not_out (1) = 1
	//
	// Averages over 2 days:
	// - AverageTrashNotOut: (2 + 3) / 2 = 2.5
	// - AverageRecyclingNotOut: (2 + 1) / 2 = 1.5

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
	cfg := RuleConfig{
		Version:          1,
		EnableHeuristics: true,
		DefaultLabel:     "other",
		Labels: []LabelDefinition{
			{Key: "custom_yard_waste", DisplayName: "Yard Waste", Metric: MetricTrash},
			{Key: "custom_dual_stream", DisplayName: "Dual Stream", Metric: MetricBoth},
		},
		Rules: []ClassificationRule{},
	}
	engine := NewRuleEngine(cfg)

	records := []record{
		{
			MessageDate: "2026-01-01T12:00:00Z",
			Label:       "custom_yard_waste",
		},
		{
			MessageDate: "2026-01-01T13:00:00Z",
			Label:       "custom_dual_stream",
		},
	}

	result := ComputeStatsWithEngine(records, engine)

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
