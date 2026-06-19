package main

import (
	"path/filepath"
	"testing"
)

func TestParseRecordsFromSampleMessage(t *testing.T) {
	meta, err := loadMessage(filepath.Join("data", "Medford Tags 01_02_26.msg"))
	if err != nil {
		t.Fatalf("loadMessage: %v", err)
	}

	records := parseRecords(meta)
	if len(records) != 4 {
		t.Fatalf("expected 4 records, got %d", len(records))
	}

	if records[0].IssueType != "recyc_not_out" {
		t.Fatalf("expected first issue type recyc_not_out, got %q", records[0].IssueType)
	}
	if records[2].IssueTime != "0831AM" {
		t.Fatalf("expected third issue time 0831AM, got %q", records[2].IssueTime)
	}
	if records[3].IssueType != "bulk_item_not_out" {
		t.Fatalf("expected fourth issue type bulk_item_not_out, got %q", records[3].IssueType)
	}
}

func TestCollectInputPathsForDirectory(t *testing.T) {
	paths, err := collectInputPaths("data")
	if err != nil {
		t.Fatalf("collectInputPaths: %v", err)
	}
	if len(paths) != 5 {
		t.Fatalf("expected 5 .msg paths, got %d", len(paths))
	}
	if filepath.Ext(paths[0]) != ".msg" {
		t.Fatalf("expected .msg path, got %q", paths[0])
	}
}

func TestParseInputPathsForDirectory(t *testing.T) {
	paths, err := collectInputPaths("data")
	if err != nil {
		t.Fatalf("collectInputPaths: %v", err)
	}

	records, err := parseInputPaths(paths)
	if err != nil {
		t.Fatalf("parseInputPaths: %v", err)
	}
	if len(records) != 17 {
		t.Fatalf("expected 17 records, got %d", len(records))
	}
}

func TestParseDateFromSubject(t *testing.T) {
	date := parseDateFromSubject("Medford Tags 01.07.26")
	if date.IsZero() {
		t.Fatal("expected subject date to parse")
	}
	if got := date.Format("2006-01-02"); got != "2026-01-07" {
		t.Fatalf("unexpected parsed date %s", got)
	}
}

func TestClassifyEntry(t *testing.T) {
	location, issueType, issueTime := classifyEntry("42 WOBURN ST MSW NOT OUT 1102AM")
	if location != "42 WOBURN ST" {
		t.Fatalf("expected location hint 42 WOBURN ST, got %q", location)
	}
	if issueType != "msw_not_out" {
		t.Fatalf("expected issue type msw_not_out, got %q", issueType)
	}
	if issueTime != "1102AM" {
		t.Fatalf("expected issue time 1102AM, got %q", issueTime)
	}
}
