package main

import (
	"sort"
	"time"
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

// getRecordDay extracts the date string in YYYY-MM-DD format from a record.
func getRecordDay(rec record) string {
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

// ComputeStats aggregates record data into DailyStats and computes SummaryStats.
func ComputeStats(records []record) SummaryStats {
	dailyCounts := make(map[string]*DailyStats)

	for _, rec := range records {
		day := getRecordDay(rec)
		if _, exists := dailyCounts[day]; !exists {
			dailyCounts[day] = &DailyStats{Date: day}
		}

		stats := dailyCounts[day]
		switch rec.Label {
		case "msw_not_out":
			stats.TrashNotOut++
		case "recyc_not_out":
			stats.RecyclingNotOut++
		case "msw_and_recyc_not_out":
			stats.TrashNotOut++
			stats.RecyclingNotOut++
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
