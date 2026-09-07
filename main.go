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
	"sync"
	"sync/atomic"
	"time"

	msgparser "github.com/willthrom/outlook-msg-parser"
	"github.com/willthrom/outlook-msg-parser/models"
)

var (
	timestampLineRE   = regexp.MustCompile(`^(\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2})(?:\s+(\S+))?\s*$`)
	entryTimeRE       = regexp.MustCompile(`\b(\d{3,4}(?:AM|PM))\s*$`)
	subjectDateRE     = regexp.MustCompile(`(\d{2})[._/-](\d{2})[._/-](\d{2,4})`)
	listRE            = regexp.MustCompile(`^(?i)(\d+(?:(?:\s*(?:[.,]|&amp;|&|AND)\s*|\s+)\d+)+(?:\s*[.,])?)\s+(.*\S)\s*$`)
	leadingListTailRE = regexp.MustCompile(`^(?i)(?:[.,]|&amp;|&|AND)\s*(\d+)\s+(.*\S)\s*$`)
	numberRE          = regexp.MustCompile(`\d+`)
	suffixRE          = regexp.MustCompile(`(?i)\b(?:STREET|ST|STR|AVENUE|AVE|ROAD|RD|WAY|DRIVE|DR|LANE|LN|PLACE|PL|CIRCLE|CIR|BOULEVARD|BLVD|HIGHWAY|HWY|TERRACE|TER|TERR|PARKWAY|PKWY|COURT|CT|COVE|SQUARE|SQ|PARK|TRAIL|TRL|APTS|APT|CONDOS|CONDO|UNITS|UNIT|SUITES|SUITE|STE|FLOOR|FL|FELLSWAY|BROADWAY|GREENWAY|EXPRESSWAY|SPEEDWAY)\b`)
	unitModifierRE    = regexp.MustCompile(`^(?i)(?:\s*#?\s*\d+[A-Z]?|\s+[A-Z\d]\b)`)
	locModifierRE     = regexp.MustCompile(`^(?i)(?:\s+(?:MANY|ALL)\s+(?:HOMES|HOUSES|APTS|CONDOS|UNITS)\b)`)
	statusStartRE     = regexp.MustCompile(`^(?i)\s*(?:IS|WAS|HAS|ARE|BE|TOO|WILL)\b`)
	precedingRejectRE = regexp.MustCompile(`(?i)\b(?:WHOLE|OF|ON|IN|THE|BOTH|EACH|EVERY|THIS|THAT|TO|FOR|BY)\s*$`)
	directionalRE     = regexp.MustCompile(`^(?i)\s+(?:WEST|W|EAST|E|NORTH|N|SOUTH|S)\b`)
	signatureLineRE   = regexp.MustCompile(`^(?:regards|best|sincerely|thank you|thanks)[,!.\s]*$`)
	wideGapRE         = regexp.MustCompile(`\s{3,}`)
)

var defaultEngine atomic.Pointer[RuleEngine]

func init() {
	defaultEngine.Store(NewRuleEngine(DefaultRuleConfig()))
}

// DefaultEngine returns the active snapshot of the default rule engine.
func DefaultEngine() *RuleEngine {
	eng := defaultEngine.Load()
	if eng == nil {
		return NewRuleEngine(DefaultRuleConfig())
	}
	return eng
}

// SetDefaultEngine updates the active default rule engine atomically.
func SetDefaultEngine(engine *RuleEngine) {
	if engine == nil {
		engine = NewRuleEngine(DefaultRuleConfig())
	}
	defaultEngine.Store(engine)
}

// ReloadDefaultEngine reloads configuration from disk and updates the default rule engine atomically.
func ReloadDefaultEngine() {
	SetDefaultEngine(NewRuleEngine(LoadConfig()))
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

var csvHeaders = []string{
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

func (rec record) toRow() []string {
	return []string{
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
	ReloadDefaultEngine()
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

var parseLogMu sync.Mutex

func safeParseMsgFile(path string) (*models.Message, error) {
	parseLogMu.Lock()
	defer parseLogMu.Unlock()

	prevWriter := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(io.Discard)
	defer func() {
		log.SetOutput(prevWriter)
		log.SetFlags(prevFlags)
	}()

	return msgparser.ParseMsgFile(path)
}

func loadMessage(path string) (messageMetadata, error) {
	msg, err := safeParseMsgFile(path)
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

	var messageDate time.Time
	for _, t := range []time.Time{msg.Date, msg.ClientSubmitTime, msg.CreationDate, msg.LastModificationDate} {
		if !t.IsZero() {
			messageDate = t
			break
		}
	}
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
	return parseRecordsWithEngine(meta, DefaultEngine())
}

func parseRecordsWithEngine(meta messageMetadata, engine *RuleEngine) []record {
	if engine == nil {
		engine = DefaultEngine()
	}
	return engine.ParseRecords(meta)
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
	return parseInputPathsWithEngine(paths, DefaultEngine())
}

func parseInputPathsWithEngine(paths []string, engine *RuleEngine) ([]record, parseSummary, error) {
	if engine == nil {
		engine = DefaultEngine()
	}
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

		records := engine.ParseRecords(metadata)
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
	text                  string
	hadTrailingWhitespace bool
}

func cleanLines(body string) []string {
	body = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(body)

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
	fields := strings.Fields(trimmedRight)
	if len(fields) == 0 {
		return normalizedBodyLine{}
	}
	return normalizedBodyLine{
		text:                  strings.Join(fields, " "),
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
		text:                  text,
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
		"RCY", "RD", "READY", "RECYC", "RECYCLE", "RECYCLING", "ROAD",
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
			// Only reject if it's a suffix that can double as a standalone noun/subject in shorthand
			// (like "ROAD", "WAY", "PARK") rather than a definitive street abbreviation (like "ST", "AVE").
			suffixUpper := strings.ToUpper(cleaned[start:end])
			if suffixUpper == "ROAD" || suffixUpper == "WAY" || suffixUpper == "PARK" {
				continue
			}
		}

		return end
	}

	return -1
}

func splitAddressAndStatus(raw string) (address string, status string) {
	return DefaultEngine().SplitAddress(raw)
}

func normalizeIssueLabel(status string) string {
	return DefaultEngine().NormalizeIssueLabel(status)
}

func classifyEntry(raw string) (locationHint string, parsedIssue string, label string, issueTime string) {
	return DefaultEngine().Classify(raw)
}

func writeCSV(path string, records []record) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()

	writer := csv.NewWriter(file)
	if err := writer.Write(csvHeaders); err != nil {
		return err
	}

	for _, rec := range records {
		if err := writer.Write(rec.toRow()); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
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
