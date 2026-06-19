package main

import (
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	msgparser "github.com/willthrom/outlook-msg-parser"
)

var (
	timestampLineRE = regexp.MustCompile(`^(\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2})\s+(\S+)\s*$`)
	entryTimeRE     = regexp.MustCompile(`\b(\d{3,4}(?:AM|PM))\s*$`)
	subjectDateRE   = regexp.MustCompile(`(\d{2})[._/-](\d{2})[._/-](\d{2,4})`)
)

type issuePattern struct {
	Type    string
	Pattern string
}

var issuePatterns = []issuePattern{
	{Type: "msw_and_recyc_not_out", Pattern: "MSW AND RECYC NOT OUT"},
	{Type: "recyc_and_msw_not_out", Pattern: "RECYC AND MSW NOT OUT"},
	{Type: "bulk_item_not_out", Pattern: "BULK ITEM NOT OUT"},
	{Type: "bedframe_and_sofa_not_out", Pattern: "BEDFRAME AND SOFA NOT OUT"},
	{Type: "recyc_not_out", Pattern: "RECYC NOT OUT"},
	{Type: "msw_not_out", Pattern: "MSW NOT OUT"},
	{Type: "fridge_not_out", Pattern: "FRIDGE NOT OUT"},
	{Type: "sofa_not_out", Pattern: "SOFA NOT OUT"},
	{Type: "unable_to_service_msw", Pattern: "UNABLE TO SVC MSW"},
	{Type: "not_serviced", Pattern: "NOT SVCD"},
}

type record struct {
	SourceFile   string
	Subject      string
	MessageDate  string
	From         string
	To           string
	CC           string
	ReportedAt   string
	Dispatcher   string
	EntryIndex   int
	RawEntry     string
	LocationHint string
	IssueType    string
	IssueTime    string
}

type messageMetadata struct {
	SourceFile  string
	Subject     string
	MessageDate time.Time
	From        string
	To          string
	CC          string
	Body        string
}

func main() {
	log.SetFlags(0)

	input := flag.String("input", "", "Path to the Outlook .msg file to parse")
	output := flag.String("output", "", "Path to the CSV file to write")
	flag.Parse()

	if *input == "" || *output == "" {
		flag.Usage()
		os.Exit(2)
	}

	metadata, err := loadMessage(*input)
	if err != nil {
		log.Fatalf("load message: %v", err)
	}

	records := parseRecords(metadata)
	if len(records) == 0 {
		log.Fatalf("no structured rows found in %s", *input)
	}

	if err := writeCSV(*output, records); err != nil {
		log.Fatalf("write csv: %v", err)
	}
}

func loadMessage(path string) (messageMetadata, error) {
	logWriter := log.Writer()
	log.SetOutput(io.Discard)
	msg, err := msgparser.ParseMsgFile(path)
	log.SetOutput(logWriter)
	if err != nil {
		return messageMetadata{}, err
	}

	body := strings.TrimSpace(msg.BodyPlainText)
	if body == "" {
		body = strings.TrimSpace(msg.ConvertedBodyHTML)
	}
	if body == "" {
		body = strings.TrimSpace(msg.BodyHTML)
	}
	if body == "" {
		return messageMetadata{}, errors.New("message body is empty")
	}

	messageDate := firstNonZeroTime(msg.Date, msg.ClientSubmitTime, msg.CreationDate, msg.LastModificationDate)
	if messageDate.IsZero() {
		messageDate = parseDateFromHeaders(msg.TransportMessageHeaders)
	}
	if messageDate.IsZero() {
		messageDate = parseDateFromSubject(msg.Subject)
	}

	return messageMetadata{
		SourceFile:  filepath.Base(path),
		Subject:     strings.TrimSpace(msg.Subject),
		MessageDate: messageDate,
		From:        strings.TrimSpace(msg.FromEmail),
		To:          strings.TrimSpace(msg.To),
		CC:          strings.TrimSpace(msg.CC),
		Body:        body,
	}, nil
}

func parseRecords(meta messageMetadata) []record {
	lines := cleanLines(meta.Body)
	records := make([]record, 0)
	var currentTime time.Time
	var currentDispatcher string
	entryIndex := 0

	for _, line := range lines {
		if isFooterLine(line) {
			break
		}
		if strings.EqualFold(line, "Please see tags called in today:") {
			continue
		}

		if matches := timestampLineRE.FindStringSubmatch(line); matches != nil {
			parsedTime, err := time.ParseInLocation("01/02/2006 15:04:05", matches[1], time.Local)
			if err == nil {
				currentTime = parsedTime
			}
			currentDispatcher = matches[2]
			continue
		}

		if currentTime.IsZero() {
			continue
		}

		entryIndex++
		locationHint, issueType, issueTime := classifyEntry(line)
		records = append(records, record{
			SourceFile:   meta.SourceFile,
			Subject:      meta.Subject,
			MessageDate:  formatTime(meta.MessageDate),
			From:         meta.From,
			To:           meta.To,
			CC:           meta.CC,
			ReportedAt:   currentTime.Format(time.RFC3339),
			Dispatcher:   currentDispatcher,
			EntryIndex:   entryIndex,
			RawEntry:     line,
			LocationHint: locationHint,
			IssueType:    issueType,
			IssueTime:    issueTime,
		})
	}

	return records
}

func cleanLines(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")

	lines := strings.Split(body, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cleaned = append(cleaned, strings.Join(strings.Fields(line), " "))
	}
	return cleaned
}

func isFooterLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case lower == "sheri sawallich":
		return true
	case strings.Contains(lower, "dispatcher/router"):
		return true
	case strings.Contains(lower, "new england division"):
		return true
	case strings.Contains(lower, "@"):
		return true
	case strings.HasPrefix(lower, "welcome "):
		return true
	default:
		return false
	}
}

func classifyEntry(raw string) (locationHint string, issueType string, issueTime string) {
	cleaned := strings.TrimSpace(raw)
	if matches := entryTimeRE.FindStringSubmatch(cleaned); matches != nil {
		issueTime = matches[1]
		cleaned = strings.TrimSpace(cleaned[:len(cleaned)-len(matches[0])])
	}

	upper := strings.ToUpper(cleaned)
	for _, candidate := range issuePatterns {
		idx := strings.Index(upper, candidate.Pattern)
		if idx == -1 {
			continue
		}

		prefix := strings.TrimSpace(strings.Trim(cleaned[:idx], ","))
		if prefix == "" {
			prefix = cleaned
		}
		return prefix, candidate.Type, issueTime
	}

	return cleaned, "", issueTime
}

func writeCSV(path string, records []record) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	headers := []string{
		"source_file",
		"subject",
		"message_date",
		"from",
		"to",
		"cc",
		"reported_at",
		"dispatcher",
		"entry_index",
		"raw_entry",
		"location_hint",
		"issue_type",
		"issue_time",
	}
	if err := writer.Write(headers); err != nil {
		return err
	}

	for _, rec := range records {
		row := []string{
			rec.SourceFile,
			rec.Subject,
			rec.MessageDate,
			rec.From,
			rec.To,
			rec.CC,
			rec.ReportedAt,
			rec.Dispatcher,
			fmt.Sprintf("%d", rec.EntryIndex),
			rec.RawEntry,
			rec.LocationHint,
			rec.IssueType,
			rec.IssueTime,
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}

	return writer.Error()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Time{}
}

func parseDateFromHeaders(headers string) time.Time {
	headers = strings.TrimSpace(headers)
	if headers == "" {
		return time.Time{}
	}

	msg, err := mail.ReadMessage(strings.NewReader(headers + "\r\n\r\n"))
	if err != nil {
		return time.Time{}
	}
	date, err := mail.ParseDate(msg.Header.Get("Date"))
	if err != nil {
		return time.Time{}
	}
	return date
}

func parseDateFromSubject(subject string) time.Time {
	matches := subjectDateRE.FindStringSubmatch(subject)
	if matches == nil {
		return time.Time{}
	}

	month := matches[1]
	day := matches[2]
	year := matches[3]
	if len(year) == 2 {
		year = "20" + year
	}

	value, err := time.ParseInLocation("01/02/2006", month+"/"+day+"/"+year, time.Local)
	if err != nil {
		return time.Time{}
	}
	return value
}
