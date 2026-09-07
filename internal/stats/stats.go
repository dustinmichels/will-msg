package stats

import (
	"sort"
	"time"

	"will-msg/internal/config"
	"will-msg/internal/engine"
)

// DailyStats represents the calculated metrics for a single day.
type DailyStats struct {
	Date            string // YYYY-MM-DD
	TrashNotOut     int    // Msw_not_out + msw_and_recyc_not_out
	RecyclingNotOut int    // Recyc_not_out + msw_and_recyc_not_out
}

// SummaryStats represents the aggregate stats across all days.
type SummaryStats struct {
	AverageTrashNotOut     float64
	AverageRecyclingNotOut float64
	Daily                  []DailyStats // Sorted by date
}

// GetRecordDay extracts the date string in YYYY-MM-DD format from a record.
func GetRecordDay(rec engine.Record) string {
	if rec.MessageDate != "" {
		if t, err := time.Parse(time.RFC3339, rec.MessageDate); err == nil {
			return t.Format("2006-01-02")
		}
	}
	if rec.ReportedAt != "" {
		if t, err := time.Parse(time.RFC3339, rec.ReportedAt); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return "Unknown"
}

// ComputeStats aggregates record data into DailyStats and computes SummaryStats using default engine metrics.
func ComputeStats(records []engine.Record) SummaryStats {
	eng := engine.NewRuleEngine(config.DefaultRuleConfig())
	return ComputeStatsWithEngine(records, eng)
}

// ComputeStatsWithEngine aggregates record data into DailyStats and computes SummaryStats using specified engine metrics.
func ComputeStatsWithEngine(records []engine.Record, eng *engine.RuleEngine) SummaryStats {
	if eng == nil {
		eng = engine.NewRuleEngine(config.DefaultRuleConfig())
	}
	dailyCounts := make(map[string]*DailyStats)

	for _, rec := range records {
		day := GetRecordDay(rec)
		if _, exists := dailyCounts[day]; !exists {
			dailyCounts[day] = &DailyStats{Date: day}
		}

		stat := dailyCounts[day]
		metric := eng.MetricForLabel(rec.Label)

		switch metric {
		case config.MetricTrash:
			stat.TrashNotOut++
		case config.MetricRecycling:
			stat.RecyclingNotOut++
		case config.MetricBoth:
			stat.TrashNotOut++
			stat.RecyclingNotOut++
		}
	}

	var dailyList []DailyStats
	for _, ds := range dailyCounts {
		dailyList = append(dailyList, *ds)
	}

	// Sort dailyList by Date ascending
	sort.Slice(dailyList, func(i, j int) bool {
		return dailyList[i].Date < dailyList[j].Date
	})

	var totalTrash, totalRecycling int
	numDays := len(dailyList)
	if numDays > 0 {
		for _, ds := range dailyList {
			totalTrash += ds.TrashNotOut
			totalRecycling += ds.RecyclingNotOut
		}
	}

	var avgTrash, avgRecycling float64
	if numDays > 0 {
		avgTrash = float64(totalTrash) / float64(numDays)
		avgRecycling = float64(totalRecycling) / float64(numDays)
	}

	return SummaryStats{
		AverageTrashNotOut:     avgTrash,
		AverageRecyclingNotOut: avgRecycling,
		Daily:                  dailyList,
	}
}
