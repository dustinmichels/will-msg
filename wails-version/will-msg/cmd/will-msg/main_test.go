package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"

	"will-msg/internal/config"
	"will-msg/internal/engine"
	"will-msg/internal/scanner"
)

func TestParseSourcesForDirectory(t *testing.T) {
	testDir := filepath.Join("..", "..", "testdata")
	sources, err := scanner.FindSources(testDir)
	if err != nil {
		t.Fatalf("FindSources failed: %v", err)
	}

	eng := engine.NewRuleEngine(config.DefaultRuleConfig())
	records, summary, err := parseSources(sources, eng)
	if err != nil {
		t.Fatalf("parseSources: %v", err)
	}

	if summary.ParsedFiles != len(sources) {
		t.Fatalf("expected %d parsed files, got %d", len(sources), summary.ParsedFiles)
	}
	if len(records) == 0 {
		t.Fatal("expected non-empty parsed records")
	}
}

func TestWriteCSV(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "sub", "out.csv")

	records := []engine.Record{
		{
			SourceFile:   "test.msg",
			Subject:      "Test Subject",
			MessageDate:  "2026-01-02T00:00:00Z",
			ReportedAt:   "2026-01-02T08:00:00Z",
			Dispatcher:   "DISPATCHER",
			RowInMessage: 1,
			RawEntry:     "100 MAIN ST RECYC NOT OUT",
			LocationHint: "100 MAIN ST",
			ParsedIssue:  "RECYC NOT OUT",
			Label:        "recyc_not_out",
			IssueTime:    "0800AM",
		},
	}

	if err := writeCSV(outputPath, records); err != nil {
		t.Fatalf("writeCSV failed: %v", err)
	}

	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("open written CSV: %v", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read CSV rows: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (header + record), got %d", len(rows))
	}
	if rows[0][0] != "source_file" || rows[0][7] != "location" || rows[0][8] != "issue" {
		t.Errorf("unexpected headers: %v", rows[0])
	}
	if rows[1][0] != "test.msg" || rows[1][7] != "100 MAIN ST" || rows[1][8] != "RECYC NOT OUT" {
		t.Errorf("unexpected record row: %v", rows[1])
	}
}
func TestParseSourcesSummarizesSkippedFiles(t *testing.T) {
	tempDir := t.TempDir()

	validInput, err := os.ReadFile(filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg"))
	if err != nil {
		t.Fatalf("read valid sample: %v", err)
	}
	validPath := filepath.Join(tempDir, "valid.msg")
	if err := os.WriteFile(validPath, validInput, 0o644); err != nil {
		t.Fatalf("write valid sample: %v", err)
	}

	invalidPath := filepath.Join(tempDir, "invalid.msg")
	if err := os.WriteFile(invalidPath, []byte("not an outlook msg file"), 0o644); err != nil {
		t.Fatalf("write invalid sample: %v", err)
	}

	sources := []scanner.MessageSource{
		{Path: validPath, DisplayName: "valid.msg"},
		{Path: invalidPath, DisplayName: "invalid.msg"},
	}

	eng := engine.NewRuleEngine(config.DefaultRuleConfig())
	records, summary, err := parseSources(sources, eng)
	if err != nil {
		t.Fatalf("parseSources error: %v", err)
	}

	if len(records) != 5 {
		t.Fatalf("expected 5 records from valid message, got %d", len(records))
	}
	if summary.ParsedFiles != 1 || summary.SkippedFiles != 1 {
		t.Fatalf("expected summary {ParsedFiles:1 SkippedFiles:1}, got %+v", summary)
	}
}
