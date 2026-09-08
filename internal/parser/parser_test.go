package parser

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanLinesSplitsWideGaps(t *testing.T) {
	body := "07/01/2025 14:34:19\n" +
		"4 MAYNARD ST                                    171B FOREST ST"

	lines := CleanLines(body)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "07/01/2025 14:34:19" {
		t.Errorf("line 0 mismatch: %q", lines[0])
	}
	if lines[1] != "4 MAYNARD ST" {
		t.Errorf("line 1 mismatch: %q", lines[1])
	}
	if lines[2] != "171B FOREST ST" {
		t.Errorf("line 2 mismatch: %q", lines[2])
	}
}

func TestCleanLinesRejoinsWrappedContinuations(t *testing.T) {
	body := "03/02/2026 07:53:10 SSAWALLI\n" +
		"252,248, 236, 224, 196, 192, 190 , 172, 164, 156, 148, 136 AND 132 SPRI\n" +
		"NG ST RECYC NOT OUT\n" +
		"03/02/2026 15:19:39 SSAWALLI\n" +
		"EVANS ST - TOO MANY PARKED CARS ON BOTH CORNERS AND END OF STREET, UNABL E\n" +
		"TO SVC TRASH"

	lines := CleanLines(body)
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
	}
	if !strings.HasSuffix(lines[1], "SPRING ST RECYC NOT OUT") {
		t.Errorf("expected wrapped word repair, got: %q", lines[1])
	}
	if !strings.HasSuffix(lines[3], "UNABLE TO SVC TRASH") {
		t.Errorf("expected wrapped word repair in narrative, got: %q", lines[3])
	}
}

func TestExpandEntries(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{
			input: "252, 248 AND 236 SPRING ST RECYC NOT OUT",
			want: []string{
				"252 SPRING ST RECYC NOT OUT",
				"248 SPRING ST RECYC NOT OUT",
				"236 SPRING ST RECYC NOT OUT",
			},
		},
		{
			input: "10 MAIN ST TRASH NOT OUT",
			want: []string{
				"10 MAIN ST TRASH NOT OUT",
			},
		},
	}

	for _, tt := range tests {
		got := ExpandEntries(tt.input)
		if len(got) != len(tt.want) {
			t.Fatalf("ExpandEntries(%q) returned %d entries, want %d", tt.input, len(got), len(tt.want))
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
			}
		}
	}
}

func TestParseDateFromSubject(t *testing.T) {
	tests := []struct {
		subject string
		want    string
	}{
		{"Medford Tags 01.07.26", "2026-01-07"},
		{"MEDFORD TAGS 03_02_26", "2026-03-02"},
		{"Tags 12-25-2025", "2025-12-25"},
		{"No Date in Subject", ""},
	}

	for _, tt := range tests {
		d := ParseDateFromSubject(tt.subject)
		if tt.want == "" {
			if !d.IsZero() {
				t.Errorf("expected zero time for %q, got %v", tt.subject, d)
			}
		} else {
			if d.Format("2006-01-02") != tt.want {
				t.Errorf("ParseDateFromSubject(%q) = %q, want %q", tt.subject, d.Format("2006-01-02"), tt.want)
			}
		}
	}
}

func TestExtractBodyAndHeaders(t *testing.T) {
	msgPath := filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg")
	body, headers, err := ExtractBodyAndHeaders(msgPath)
	if err != nil {
		t.Fatalf("ExtractBodyAndHeaders failed: %v", err)
	}

	if !strings.Contains(headers, "Subject: Medford Tags 01.02.26") {
		t.Errorf("expected subject in headers, got: %q", headers)
	}

	if !strings.Contains(body, "CHARNWOOD RD MANY HOMES RECYC NOT OUT") {
		t.Errorf("expected body to contain sample entry, got: %q", body)
	}
}

func TestLoadMessage(t *testing.T) {
	msgPath := filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg")
	meta, err := LoadMessage(msgPath)
	if err != nil {
		t.Fatalf("LoadMessage failed: %v", err)
	}

	if meta.SourceFile != "Medford Tags 01_02_26.msg" {
		t.Errorf("expected SourceFile 'Medford Tags 01_02_26.msg', got %q", meta.SourceFile)
	}
	if meta.Subject != "Medford Tags 01.02.26" {
		t.Errorf("expected Subject 'Medford Tags 01.02.26', got %q", meta.Subject)
	}
	if meta.MessageDate.IsZero() {
		t.Errorf("expected non-zero MessageDate")
	}
	if len(meta.Body) == 0 {
		t.Errorf("expected non-empty Body")
	}
}

func TestExtractBody_FallbackOrderAndErrors(t *testing.T) {
	tests := []struct {
		name      string
		plainText string
		convHTML  string
		bodyHTML  string
		wantBody  string
		wantErr   bool
	}{
		{
			name:      "plain text wins when present",
			plainText: "  plain text content  ",
			convHTML:  "converted html content",
			bodyHTML:  "raw html content",
			wantBody:  "plain text content",
			wantErr:   false,
		},
		{
			name:      "converted html fallback when plain text empty",
			plainText: "   ",
			convHTML:  "  converted html content  ",
			bodyHTML:  "raw html content",
			wantBody:  "converted html content",
			wantErr:   false,
		},
		{
			name:      "raw html fallback when plain and converted empty",
			plainText: "",
			convHTML:  "",
			bodyHTML:  "  raw html content  ",
			wantBody:  "raw html content",
			wantErr:   false,
		},
		{
			name:      "error when all fields empty or whitespace",
			plainText: "   ",
			convHTML:  "",
			bodyHTML:  " \n\t ",
			wantBody:  "",
			wantErr:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := SafeParseMsgFile(filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg"))
			if err != nil {
				t.Fatalf("setup message: %v", err)
			}
			msg.BodyPlainText = tc.plainText
			msg.ConvertedBodyHTML = tc.convHTML
			msg.BodyHTML = tc.bodyHTML

			body, err := ExtractBody(msg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ExtractBody() error = %v, wantErr %v", err, tc.wantErr)
			}
			if body != tc.wantBody {
				t.Errorf("ExtractBody() = %q, want %q", body, tc.wantBody)
			}
		})
	}

	t.Run("nil message returns error", func(t *testing.T) {
		_, err := ExtractBody(nil)
		if err == nil {
			t.Error("expected error for nil message, got nil")
		}
	})
}
