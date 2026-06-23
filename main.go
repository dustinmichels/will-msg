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
	timestampLineRE   = regexp.MustCompile(`^(\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2})(?:\s+(\S+))?\s*$`)
	entryTimeRE       = regexp.MustCompile(`\b(\d{3,4}(?:AM|PM))\s*$`)
	subjectDateRE     = regexp.MustCompile(`(\d{2})[._/-](\d{2})[._/-](\d{2,4})`)
	listRE            = regexp.MustCompile(`^(?i)(\d+(?:(?:\s*(?:[.,]|&amp;|&|AND)\s*|\s+)\d+)+(?:\s*[.,])?)\s+(.*\S)\s*$`)
	leadingListTailRE = regexp.MustCompile(`^(?i)(?:[.,]|&amp;|&|AND)\s*(\d+)\s+(.*\S)\s*$`)
	numberRE          = regexp.MustCompile(`\d+`)
	suffixRE          = regexp.MustCompile(`(?i)\b(?:STREET|ST|AVENUE|AVE|ROAD|RD|WAY|DRIVE|DR|LANE|LN|PLACE|PL|CIRCLE|CIR|BOULEVARD|BLVD|HIGHWAY|HWY|TERRACE|TER|TERR|PARKWAY|PKWY|COURT|CT|COVE|SQUARE|SQ|APTS|APT|CONDOS|CONDO|UNITS|UNIT|SUITES|SUITE|STE|FLOOR|FL|FELLSWAY|BROADWAY|GREENWAY|EXPRESSWAY|SPEEDWAY)\b`)
	unitModifierRE    = regexp.MustCompile(`^(?i)(?:\s*#?\s*\d+[A-Z]?|\s+[A-Z\d]\b)`)
	locModifierRE     = regexp.MustCompile(`^(?i)(?:\s+(?:MANY|ALL)\s+(?:HOMES|HOUSES|APTS|CONDOS|UNITS)\b)`)
	statusStartRE     = regexp.MustCompile(`^(?i)\s*(?:IS|WAS|HAS|ARE|BE|TOO|WILL)\b`)
	precedingRejectRE = regexp.MustCompile(`(?i)\b(?:WHOLE|OF|ON|IN|THE|BOTH|EACH|EVERY|THIS|THAT|TO|FOR|BY)\s*$`)
	directionalRE     = regexp.MustCompile(`^(?i)\s+(?:WEST|W|EAST|E|NORTH|N|SOUTH|S)\b`)
	blockedStatusRE   = regexp.MustCompile(`\bBLOCK(?:ED|ING|S)?\b`)
	signatureLineRE   = regexp.MustCompile(`^(?:regards|best|sincerely|thank you|thanks)[,!.\s]*$`)
	wideGapRE          = regexp.MustCompile(`\s{3,}`)
)

type issuePattern struct {
	Label   string
	Pattern string
}

var issuePatterns = []issuePattern{
	{Label: "msw_and_recyc_not_out", Pattern: "MSW AND RECYC NOT OUT"},
	{Label: "msw_and_recyc_not_out", Pattern: "RECYC AND MSW NOT OUT"},
	{Label: "recyc_not_out", Pattern: "RECYC NOT OUT"},
	{Label: "recyc_not_out", Pattern: "RECYCLE NOT OUT"},
	{Label: "recyc_not_out", Pattern: "RECYCCLE NOT OUT"},
	{Label: "recyc_not_out", Pattern: "NOT OUT RECYC"},
	{Label: "msw_not_out", Pattern: "MSW NOT OUT"},
	{Label: "msw_not_out", Pattern: "TRASH NOT OUT"},
	{Label: "special_item_not_out", Pattern: "BULK ITEM NOT OUT"},
	{Label: "special_item_not_out", Pattern: "BEDFRAME AND SOFA NOT OUT"},
	{Label: "special_item_not_out", Pattern: "FRIDGE NOT OUT"},
	{Label: "special_item_not_out", Pattern: "SOFA NOT OUT"},
	{Label: "recy_contaminated", Pattern: "RECY CONTAM"},
	{Label: "recy_contaminated", Pattern: "RECYC CONTAM"},
	{Label: "recy_contaminated", Pattern: "RECYCLE CONTAM"},
	{Label: "recy_contaminated", Pattern: "RECYCLING CONTAM"},
	{Label: "recy_contaminated", Pattern: "CONTAMINATED RECYC"},
	{Label: "recy_contaminated", Pattern: "CONTAMINATED RECYCLE"},
	{Label: "recy_contaminated", Pattern: "CONTAMINATED RECYCLING"},
	{Label: "recy_contaminated", Pattern: "RECYCLING CONTAMINATED"},
}

type record struct {
	SourceFile   string
	Subject      string
	MessageDate  string
	ReportedAt   string
	Dispatcher   string
	RowInMessage int
	RawEntry     string
	LocationHint string
	ParsedIssue  string
	Label        string
	IssueTime    string
}

type messageMetadata struct {
	SourceFile  string
	Subject     string
	MessageDate time.Time
	Body        string
}

type parseSummary struct {
	ParsedFiles  int
	SkippedFiles int
}

func main() {
	log.SetFlags(0)

	input := flag.String("input", "", "Path to a .msg file or a directory of .msg files")
	output := flag.String("output", "", "Path to the CSV file to write")
	flag.Parse()

	if *input == "" && *output == "" {
		runGUI()
		return
	}

	if *input == "" || *output == "" {
		flag.Usage()
		os.Exit(2)
	}

	inputPaths, err := collectInputPaths(*input)
	if err != nil {
		log.Fatalf("collect input paths: %v", err)
	}

	records, summary, err := parseInputPaths(inputPaths)
	if err != nil {
		log.Fatalf("parse inputs: %v", err)
	}
	if len(records) == 0 {
		log.Fatalf("no structured rows found in %s", *input)
	}

	if err := writeCSV(*output, records); err != nil {
		log.Fatalf("write csv: %v", err)
	}

	log.Printf("parsed %d files into %d rows; skipped %d files", summary.ParsedFiles, len(records), summary.SkippedFiles)
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
		Body:        body,
	}, nil
}

func expandEntries(line string) []string {
	if listMatches := listRE.FindStringSubmatch(line); listMatches != nil {
		numbers := numberRE.FindAllString(listMatches[1], -1)
		remainder := listMatches[2]
		for {
			tailMatches := leadingListTailRE.FindStringSubmatch(remainder)
			if tailMatches == nil {
				break
			}
			numbers = append(numbers, tailMatches[1])
			remainder = tailMatches[2]
		}
		entries := make([]string, 0, len(numbers))
		for _, num := range numbers {
			entries = append(entries, num+" "+remainder)
		}
		return entries
	}

	return []string{line}
}

func parseRecords(meta messageMetadata) []record {
	lines := cleanLines(meta.Body)
	records := make([]record, 0)
	var currentTime time.Time
	var currentDispatcher string
	rowInMessage := 0

	for _, line := range lines {
		if matches := timestampLineRE.FindStringSubmatch(line); matches != nil {
			parsedTime, err := time.ParseInLocation("01/02/2006 15:04:05", matches[1], time.Local)
			if err == nil {
				currentTime = parsedTime
			}
			currentDispatcher = matches[2]
			continue
		}
		if isFooterLine(line) {
			if currentTime.IsZero() {
				continue
			}
			break
		}
		if strings.EqualFold(line, "Please see tags called in today:") {
			continue
		}

		if currentTime.IsZero() {
			continue
		}

		rowInMessage++
		for _, entry := range expandEntries(line) {
			locationHint, parsedIssue, label, issueTime := classifyEntry(entry)
			records = append(records, record{
				SourceFile:   meta.SourceFile,
				Subject:      meta.Subject,
				MessageDate:  formatTime(meta.MessageDate),
				ReportedAt:   currentTime.Format(time.RFC3339),
				Dispatcher:   currentDispatcher,
				RowInMessage: rowInMessage,
				RawEntry:     entry,
				LocationHint: locationHint,
				ParsedIssue:  parsedIssue,
				Label:        label,
				IssueTime:    issueTime,
			})
		}
	}

	return records
}

func collectInputPaths(input string) ([]string, error) {
	info, err := os.Stat(input)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return []string{input}, nil
	}

	entries, err := os.ReadDir(input)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if shouldIgnore(entry.Name()) {
			continue
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".msg") {
			paths = append(paths, filepath.Join(input, entry.Name()))
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no .msg files found in %s", input)
	}

	return paths, nil
}

func parseInputPaths(paths []string) ([]record, parseSummary, error) {
	allRecords := make([]record, 0)
	summary := parseSummary{}
	isBatch := len(paths) > 1
	for _, path := range paths {
		metadata, err := loadMessage(path)
		if err != nil {
			if isBatch {
				summary.SkippedFiles++
				log.Printf("warning: skipping %s: %v", path, err)
				continue
			}
			return nil, parseSummary{}, fmt.Errorf("%s: %w", path, err)
		}

		records := parseRecords(metadata)
		if len(records) == 0 {
			if isBatch {
				summary.SkippedFiles++
				log.Printf("warning: skipping %s: no structured rows found", path)
				continue
			}
			return nil, parseSummary{}, fmt.Errorf("%s: no structured rows found", path)
		}

		summary.ParsedFiles++
		allRecords = append(allRecords, records...)
	}

	return allRecords, summary, nil
}

type normalizedBodyLine struct {
	text                 string
	hadTrailingWhitespace bool
}

func cleanLines(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.ReplaceAll(body, "\r", "\n")

	lines := strings.Split(body, "\n")
	cleaned := make([]normalizedBodyLine, 0, len(lines))
	for _, raw := range lines {
		for _, seg := range splitOnWideGaps(raw) {
			line := normalizeBodyLine(seg)
			if line.text == "" {
				continue
			}
			if len(cleaned) > 0 && isWrappedContinuation(cleaned[len(cleaned)-1], line) {
				cleaned[len(cleaned)-1] = mergeWrappedLine(cleaned[len(cleaned)-1], line)
				continue
			}
			cleaned = append(cleaned, line)
		}
	}

	result := make([]string, 0, len(cleaned))
	for _, line := range cleaned {
		result = append(result, line.text)
	}
	return result
}

func normalizeBodyLine(raw string) normalizedBodyLine {
	trimmedRight := strings.TrimRight(raw, " \t")
	line := strings.TrimSpace(trimmedRight)
	if line == "" {
		return normalizedBodyLine{}
	}
	return normalizedBodyLine{
		text:                 strings.Join(strings.Fields(line), " "),
		hadTrailingWhitespace: len(raw)-len(trimmedRight) >= 6,
	}
}

// splitOnWideGaps splits raw on runs of 2+ whitespace characters, turning
// column-layout lines like "4 MAYNARD ST          171B FOREST ST" into
// separate segments before whitespace is collapsed by normalizeBodyLine.
// Timestamp, intro, and footer lines are returned as-is so they are never
// fragmented.
func splitOnWideGaps(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if timestampLineRE.MatchString(trimmed) || isIntroLine(trimmed) || isFooterLine(trimmed) {
		return []string{raw}
	}
	parts := wideGapRE.Split(trimmed, -1)
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	// Single segment: no real gap found. Return the original raw string so
	// normalizeBodyLine can still detect hadTrailingWhitespace correctly.
	if len(out) <= 1 {
		return []string{raw}
	}
	return out
}

func isWrappedContinuation(prev normalizedBodyLine, cur normalizedBodyLine) bool {
	switch {
	case prev.text == "", cur.text == "":
		return false
	case timestampLineRE.MatchString(prev.text), timestampLineRE.MatchString(cur.text):
		return false
	case isFooterLine(prev.text), isFooterLine(cur.text):
		return false
	case isIntroLine(prev.text), isIntroLine(cur.text):
		return false
	case prev.hadTrailingWhitespace:
		return false
	}

	if endsWithJoinableFragment(prev.text, cur.text) {
		return true
	}

	return !looksLikeStandaloneEntry(cur.text)
}

func mergeWrappedLine(prev normalizedBodyLine, cur normalizedBodyLine) normalizedBodyLine {
	text := repairTrailingSplitWord(prev.text)
	if endsWithJoinableFragment(text, cur.text) {
		text += cur.text
	} else {
		text += " " + cur.text
	}
	return normalizedBodyLine{
		text:                 text,
		hadTrailingWhitespace: cur.hadTrailingWhitespace,
	}
}

func repairTrailingSplitWord(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return line
	}

	last := strings.Trim(fields[len(fields)-1], ",.;:-")
	prev := strings.Trim(fields[len(fields)-2], ",.;:-")
	if len(last) != 1 || len(prev) < 4 || !isUpperWord(last) || !isUpperWord(prev) {
		return line
	}
	if isCommonWord(last) {
		return line
	}

	fields[len(fields)-2] += fields[len(fields)-1]
	fields = fields[:len(fields)-1]
	return strings.Join(fields, " ")
}

func endsWithJoinableFragment(prev string, cur string) bool {
	tail := trailingAlphaToken(prev)
	head := leadingAlphaToken(cur)
	if tail == "" || head == "" {
		return false
	}
	if isCommonWord(tail + head) {
		return true
	}
	if isCommonWord(tail) || isCommonWord(head) {
		return false
	}
	return len(tail) >= 1 && len(tail) <= 8 && len(head) >= 1 && len(head) <= 8
}

func isCommonWord(w string) bool {
	switch w {
	case "A", "ABOUT", "AC", "ACS", "ALL", "ALSO", "AN", "AND", "ANY", "APT", "APTS", "ARE", "AS", "AT", "AVE",
		"BAG", "BAGS", "BE", "BED", "BEDFRAME", "BEHIND", "BHND", "BLOCKED", "BLVD", "BOTH", "BOX", "BOXES", "BOXSPRING", "BROADWAY", "BULK", "BURIED", "BUT", "BY",
		"CAN", "CANNOT", "CAR", "CARDBOARD", "CARS", "CHECKED", "CIR", "CIRCLE", "CLEAN", "COULD", "COUCH", "COURT", "COVE", "COMPLETED", "CONCERN", "CONDO", "CONDOS", "CONTAM", "CONTAMINATED", "CT", "CURB", "CURBSIDE", "CUST", "CUSTOMER",
		"DELAY", "DID", "DO", "DONE", "DOWN", "DR", "DRIVER", "DRIVE", "DRVR",
		"EACH", "EVEN", "EVERY", "EVERYWHERE",
		"FELLSWAY", "FILLED", "FL", "FLOOR", "FOR", "FRIDGE", "FROM",
		"GET", "GETTING", "GLASS", "GO", "GOOD", "GOODS", "GREENWAY",
		"HAD", "HAS", "HAVE", "HE", "HER", "HIS", "HOME", "HOMES", "HOUSE", "HOW", "HWY",
		"I", "ICE", "IN", "INACCESSIBLE", "INACESSIBLE", "INSIDES", "IS", "IT", "ITEM", "ITEMS", "ITS",
		"JUST",
		"LANE", "LAWN", "LEFT", "LIFT", "LN",
		"MATTRESS", "METAL", "MPU", "MSW",
		"NO", "NOT",
		"OF", "ON", "ONE", "ONLY", "ONLINE", "OR", "OTHER", "OUR", "OUT",
		"PANICKED", "PARKWAY", "PICK", "PKWY", "PL", "PLACE", "PLEASE", "PROPERTY",
		"RD", "READY", "RECYC", "RECYCLE", "RECYCLING", "ROAD",
		"SAFETY", "SAME", "SERVICE", "SERVICED", "SHE", "SHOULD", "SIDE", "SNOW", "SNOWBANK", "SOFA", "SOME", "SQ", "SQUARE", "ST", "STE", "STREET", "SUITE", "SUITES", "SVCD",
		"TER", "TERR", "TERRACE", "THAT", "THE", "THEIR", "THEM", "THEN", "THERE", "THESE", "THEY", "THIS", "THREE", "TICKET", "TKT", "TO", "TOY", "TOYS", "TRASH", "TRUCK", "TWO",
		"UNABLE", "UNIT", "UNITS", "UP", "UPSET",
		"VERY",
		"WAS", "WAY", "WE", "WENT", "WHEN", "WHO", "WILL", "WITH", "WOOD", "WOULD",
		"YES", "YOU", "YOUR":
		return true
	}
	return false
}

func trailingAlphaToken(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	token := strings.Trim(fields[len(fields)-1], ",.;:-")
	if !isUpperWord(token) {
		return ""
	}
	return token
}

func leadingAlphaToken(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	token := strings.Trim(fields[0], ",.;:-")
	if !isUpperWord(token) {
		return ""
	}
	return token
}

func isUpperWord(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

func looksLikeStandaloneEntry(line string) bool {
	if line == "" {
		return false
	}
	if line[0] >= '0' && line[0] <= '9' {
		return true
	}
	if match := suffixRE.FindStringIndex(line); match != nil && match[0] <= 18 {
		return true
	}
	return false
}

func isIntroLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.HasPrefix(lower, "please see tags called in today") ||
		strings.HasPrefix(lower, "please find today")
}

func isFooterLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case signatureLineRE.MatchString(lower):
		return true
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

func findLastAddressIndex(cleaned string) int {
	matches := suffixRE.FindAllStringSubmatchIndex(cleaned, -1)
	if len(matches) == 0 {
		return -1
	}

	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		start := match[0]
		end := match[1]

		suffixStr := strings.ToUpper(cleaned[start:end])
		isUnitSuffix := false
		unitSuffixes := []string{"APT", "APTS", "UNIT", "UNITS", "CONDOS", "CONDO", "SUITE", "SUITES", "STE", "FL", "FLOOR"}
		for _, us := range unitSuffixes {
			if suffixStr == us {
				isUnitSuffix = true
				break
			}
		}

		if isUnitSuffix {
			rem := cleaned[end:]
			if modifierMatches := unitModifierRE.FindStringIndex(rem); modifierMatches != nil && modifierMatches[0] == 0 {
				end += modifierMatches[1]
			}
		}

		// Check if a location modifier follows (e.g. "MANY HOMES")
		rem := cleaned[end:]
		if locModifierMatches := locModifierRE.FindStringIndex(rem); locModifierMatches != nil && locModifierMatches[0] == 0 {
			end += locModifierMatches[1]
			rem = cleaned[end:]
		}

		// Check if a directional follows (e.g. "WEST", "W", "EAST", "E")
		if directionalMatches := directionalRE.FindStringIndex(rem); directionalMatches != nil && directionalMatches[0] == 0 {
			end += directionalMatches[1]
			rem = cleaned[end:]
		}

		// Verify this is a valid address suffix by checking preceding text and remainder
		preceding := cleaned[:start]
		if precedingRejectRE.MatchString(preceding) {
			continue
		}
		if statusStartRE.MatchString(rem) {
			continue
		}

		return end
	}

	return -1
}

func splitAddressAndStatus(raw string) (address string, status string) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, "–", "-")
	cleaned = strings.ReplaceAll(cleaned, "—", "-")

	// 1. Split based on the LAST address suffix.
	endIdx := findLastAddressIndex(cleaned)
	if endIdx != -1 {
		address = strings.TrimSpace(cleaned[:endIdx])
		status = strings.TrimSpace(cleaned[endIdx:])
	} else {
		// 2. If no suffix matches, check if we contain any known issue pattern.
		// E.g., for suffix-less addresses like "23 AND 25 KILSYTH MSW AND RECYC NOT OUT"
		upper := strings.ToUpper(cleaned)
		earliestIdx := -1
		for _, candidate := range issuePatterns {
			idx := strings.Index(upper, candidate.Pattern)
			if idx != -1 {
				if earliestIdx == -1 || idx < earliestIdx {
					earliestIdx = idx
				}
			}
		}

		if earliestIdx != -1 {
			address = strings.TrimSpace(cleaned[:earliestIdx])
			status = strings.TrimSpace(cleaned[earliestIdx:])
		} else {
			// 3. Fallback: split by first comma, semicolon, or space-dash-space
			firstDelim := -1
			delimLen := 0

			if idx := strings.Index(cleaned, " - "); idx != -1 {
				firstDelim = idx
				delimLen = 3
			}

			for i, r := range cleaned {
				if r == ',' || r == ';' {
					if firstDelim == -1 || i < firstDelim {
						firstDelim = i
						delimLen = 1
					}
				}
			}

			if firstDelim != -1 {
				address = strings.TrimSpace(cleaned[:firstDelim])
				status = strings.TrimSpace(cleaned[firstDelim+delimLen:])
			} else {
				return cleaned, ""
			}
		}
	}

	address = strings.TrimFunc(address, func(r rune) bool {
		return r == ',' || r == '-' || r == ';' || r == ' ' || r == '.'
	})
	status = strings.TrimFunc(status, func(r rune) bool {
		return r == ',' || r == '-' || r == ';' || r == ' ' || r == '.'
	})

	return address, status
}

func normalizeIssueLabel(status string) string {
	s := strings.TrimSpace(status)
	sUpper := strings.ToUpper(s)

	for _, candidate := range issuePatterns {
		if strings.Contains(sUpper, candidate.Pattern) {
			return candidate.Label
		}
	}
	if blockedStatusRE.MatchString(sUpper) {
		return "blocked"
	}

	s = strings.ReplaceAll(sUpper, "NOT SVCD", "NOT SERVICED")
	s = strings.ReplaceAll(s, "UNABLE TO SVC", "UNABLE TO SERVICE")

	var sb strings.Builder
	lastWasUnderscore := false
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			lastWasUnderscore = false
		} else if !lastWasUnderscore && sb.Len() > 0 {
			sb.WriteRune('_')
			lastWasUnderscore = true
		}
	}

	label := strings.Trim(strings.ToLower(sb.String()), "_")
	labelWithoutTrailingDigits := strings.Trim(strings.TrimRight(label, "0123456789"), "_")
	if strings.HasSuffix(labelWithoutTrailingDigits, "_not_out") {
		return "special_item_not_out"
	}

	return "other"
}

func classifyEntry(raw string) (locationHint string, parsedIssue string, label string, issueTime string) {
	cleaned := strings.TrimSpace(raw)
	if matches := entryTimeRE.FindStringSubmatch(cleaned); matches != nil {
		issueTime = matches[1]
		cleaned = strings.TrimSpace(cleaned[:len(cleaned)-len(matches[0])])
	}

	address, status := splitAddressAndStatus(cleaned)
	if status == "" {
		return address, "", "", issueTime
	}

	return address, status, normalizeIssueLabel(status), issueTime
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
		"reported_at",
		"dispatcher",
		"row_in_message",
		"raw_entry",
		"location",
		"issue",
		"label",
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
			rec.ReportedAt,
			rec.Dispatcher,
			fmt.Sprintf("%d", rec.RowInMessage),
			rec.RawEntry,
			rec.LocationHint,
			rec.ParsedIssue,
			rec.Label,
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
