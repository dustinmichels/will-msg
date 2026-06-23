package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestParseRecordsFromSampleMessage(t *testing.T) {
	meta, err := loadMessage(filepath.Join("data", "Medford Tags 01_02_26.msg"))
	if err != nil {
		t.Fatalf("loadMessage: %v", err)
	}

	records := parseRecords(meta)
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

	records, summary, err := parseInputPaths(paths)
	if err != nil {
		t.Fatalf("parseInputPaths: %v", err)
	}
	if len(records) != 22 {
		t.Fatalf("expected 22 records, got %d", len(records))
	}
	if summary.ParsedFiles != 5 || summary.SkippedFiles != 0 {
		t.Fatalf("expected summary {ParsedFiles:5 SkippedFiles:0}, got %+v", summary)
	}
}

func TestParseInputPathsSummarizesSkippedFiles(t *testing.T) {
	tempDir := t.TempDir()

	validInput, err := os.ReadFile(filepath.Join("data", "Medford Tags 01_02_26.msg"))
	if err != nil {
		t.Fatalf("ReadFile valid sample: %v", err)
	}
	validPath := filepath.Join(tempDir, "valid.msg")
	if err := os.WriteFile(validPath, validInput, 0o644); err != nil {
		t.Fatalf("WriteFile valid sample: %v", err)
	}

	invalidPath := filepath.Join(tempDir, "invalid.msg")
	if err := os.WriteFile(invalidPath, []byte("not an outlook message"), 0o644); err != nil {
		t.Fatalf("WriteFile invalid sample: %v", err)
	}

	records, summary, err := parseInputPaths([]string{validPath, invalidPath})
	if err != nil {
		t.Fatalf("parseInputPaths: %v", err)
	}
	if len(records) != 5 {
		t.Fatalf("expected 5 records, got %d", len(records))
	}
	if summary.ParsedFiles != 1 || summary.SkippedFiles != 1 {
		t.Fatalf("expected summary {ParsedFiles:1 SkippedFiles:1}, got %+v", summary)
	}
}

func TestWriteCSVUsesRenamedHeaders(t *testing.T) {
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "out.csv")

	records := []record{{
		SourceFile:   "sample.msg",
		Subject:      "Sample",
		MessageDate:  "2026-01-02T00:00:00Z",
		ReportedAt:   "2026-01-02T12:34:56Z",
		Dispatcher:   "Dispatch",
		RowInMessage: 1,
		RawEntry:     "42 WOBURN ST MSW NOT OUT 1102AM",
		LocationHint: "42 WOBURN ST",
		ParsedIssue:  "MSW NOT OUT",
		Label:        "msw_not_out",
		IssueTime:    "1102AM",
	}}

	if err := writeCSV(outputPath, records); err != nil {
		t.Fatalf("writeCSV: %v", err)
	}

	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected header and one data row, got %d rows", len(rows))
	}

	expectedHeaders := []string{
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
	for i, want := range expectedHeaders {
		if rows[0][i] != want {
			t.Fatalf("header %d: expected %q, got %q", i, want, rows[0][i])
		}
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
			name:          "washington st ac",
			input:         "198 WASHINGTON ST APT 1, AC NOT OUT",
			expectedLoc:   "198 WASHINGTON ST APT 1",
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
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loc, _, label, issueTime := classifyEntry(tc.input)
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
	loc, parsedIssue, label, issueTime := classifyEntry("473 BORN COURT APTS, BROADWAY ST, MSW AND RECYC NOT OUT")
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
			meta := messageMetadata{
				SourceFile: "test.msg",
				Subject:    "Test subject",
				Body:       tc.body,
			}
			records := parseRecords(meta)
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
			meta := messageMetadata{
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

			records := parseRecords(meta)
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
	meta := messageMetadata{
		SourceFile: "test.msg",
		Subject:    "Test subject",
		Body: strings.Join([]string{
			"Please see TAGS called in today.",
			"06/27/2025 12:00:41",
			"139 SHARON ST BULK ITEM NOT OUT ON 0626 928 AM",
		}, "\n"),
	}

	records := parseRecords(meta)
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
	meta := messageMetadata{
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

	records := parseRecords(meta)
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
	file, err := os.Open(filepath.Join("sample", "msg_parsed_2026-06-23_125025.csv"))
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

	for i, row := range records {
		if i == 0 {
			continue // skip header
		}
		rawEntry := row[6]
		// Filter out noise, fragments, and footers using general structural criteria
		isRealAddress := regexp.MustCompile(`^\d+(?:-\d+)?\b`).MatchString(rawEntry) || findLastAddressIndex(rawEntry) != -1
		if !isRealAddress || isFooterLine(rawEntry) {
			continue
		}

		loc, _, label, _ := classifyEntry(rawEntry)

		// Check if rawEntry contains any status tokens
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
			// If rawEntry had a status token, we expect the parsed location_hint to NOT contain that token anymore (meaning it was split out)
			upperLoc := strings.ToUpper(loc)
			if strings.Contains(upperLoc, matchedToken) {
				t.Errorf("Row %d: LocationHint still contains status token %q: Loc=%q, Raw=%q", i+1, matchedToken, loc, rawEntry)
				failCount++
			}

			// Also, label should not be empty
			if label == "" {
				t.Errorf("Row %d: Label is empty for entry with status token: Raw=%q", i+1, rawEntry)
				failCount++
			}
		}
	}

	t.Logf("Validated %d CSV rows. Total failures: %d", len(records)-1, failCount)
}
