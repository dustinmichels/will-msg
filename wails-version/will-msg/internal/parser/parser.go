package parser

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/mail"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	msgparser "github.com/willthrom/outlook-msg-parser"
	"github.com/willthrom/outlook-msg-parser/models"
)

var (
	TimestampLineRE   = regexp.MustCompile(`^(\d{2}/\d{2}/\d{4} \d{2}:\d{2}:\d{2})(?:\s+(\S+))?\s*$`)
	EntryTimeRE       = regexp.MustCompile(`\b(\d{3,4}(?:AM|PM))\s*$`)
	SubjectDateRE     = regexp.MustCompile(`(\d{2})[._/-](\d{2})[._/-](\d{2,4})`)
	ListRE            = regexp.MustCompile(`^(?i)(\d+(?:(?:\s*(?:[.,]|&amp;|&|AND)\s*|\s+)\d+)+(?:\s*[.,])?)\s+(.*\S)\s*$`)
	LeadingListTailRE = regexp.MustCompile(`^(?i)(?:[.,]|&amp;|&|AND)\s*(\d+)\s+(.*\S)\s*$`)
	NumberRE          = regexp.MustCompile(`\d+`)
	SuffixRE          = regexp.MustCompile(`(?i)\b(?:STREET|ST|STR|AVENUE|AVE|ROAD|RD|WAY|DRIVE|DR|LANE|LN|PLACE|PL|CIRCLE|CIR|BOULEVARD|BLVD|HIGHWAY|HWY|TERRACE|TER|TERR|PARKWAY|PKWY|COURT|CT|COVE|SQUARE|SQ|PARK|TRAIL|TRL|APTS|APT|CONDOS|CONDO|UNITS|UNIT|SUITES|SUITE|STE|FLOOR|FL|FELLSWAY|BROADWAY|GREENWAY|EXPRESSWAY|SPEEDWAY)\b`)
	SignatureLineRE   = regexp.MustCompile(`^(?:regards|best|sincerely|thank you|thanks)[,!.\s]*$`)
	WideGapRE         = regexp.MustCompile(`\s{3,}`)
)

var parseLogMu sync.Mutex

// MessageMetadata contains the extracted metadata and plain-text body of an Outlook message.
type MessageMetadata struct {
	SourceFile  string
	Subject     string
	MessageDate time.Time
	Body        string
}

// SafeParseMsgFile parses an Outlook .msg file while muting noisy third-party logging with mutex protection.
func SafeParseMsgFile(path string) (*models.Message, error) {
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

// ExtractBody extracts plain text or converted HTML body from an Outlook message model.
func ExtractBody(msg *models.Message) (string, error) {
	if msg == nil {
		return "", errors.New("message is nil")
	}
	body := strings.TrimSpace(msg.BodyPlainText)
	if body == "" {
		body = strings.TrimSpace(msg.ConvertedBodyHTML)
	}
	if body == "" {
		body = strings.TrimSpace(msg.BodyHTML)
	}
	if body == "" {
		return "", errors.New("message body is empty")
	}
	return body, nil
}

// ExtractDate extracts the message timestamp prioritizing native fields, transport headers, and subject line.
func ExtractDate(msg *models.Message) time.Time {
	if msg == nil {
		return time.Time{}
	}
	for _, t := range []time.Time{msg.Date, msg.ClientSubmitTime, msg.CreationDate, msg.LastModificationDate} {
		if !t.IsZero() {
			return t
		}
	}
	if d := ParseDateFromHeaders(msg.TransportMessageHeaders); !d.IsZero() {
		return d
	}
	return ParseDateFromSubject(msg.Subject)
}

// LoadMessage extracts message metadata and body from an Outlook .msg file.
func LoadMessage(path string) (MessageMetadata, error) {
	msg, err := SafeParseMsgFile(path)
	if err != nil {
		return MessageMetadata{}, err
	}
	body, err := ExtractBody(msg)
	if err != nil {
		return MessageMetadata{}, err
	}

	return MessageMetadata{
		SourceFile:  filepath.Base(path),
		Subject:     strings.TrimSpace(msg.Subject),
		MessageDate: ExtractDate(msg),
		Body:        body,
	}, nil
}

// ExtractBodyAndHeaders returns the message body and formatted header block (for CLI inspection utilities).
func ExtractBodyAndHeaders(path string) (body, headers string, err error) {
	msg, err := SafeParseMsgFile(path)
	if err != nil {
		return "", "", err
	}
	body, err = ExtractBody(msg)
	if err != nil {
		return "", "", err
	}

	var hb strings.Builder
	if msg.Subject != "" {
		fmt.Fprintf(&hb, "Subject: %s\n", strings.TrimSpace(msg.Subject))
	}
	from := strings.TrimSpace(msg.FromName)
	if msg.FromEmail != "" {
		from += " <" + strings.TrimSpace(msg.FromEmail) + ">"
	}
	if from != "" {
		fmt.Fprintf(&hb, "From:    %s\n", from)
	}
	if msg.ToDisplay != "" {
		fmt.Fprintf(&hb, "To:      %s\n", strings.TrimSpace(msg.ToDisplay))
	}
	if !msg.Date.IsZero() {
		fmt.Fprintf(&hb, "Date:    %s\n", msg.Date.Format("Mon 02 Jan 2006 15:04:05 MST"))
	}

	return body, strings.TrimRight(hb.String(), "\n"), nil
}

// ParseTimestampLine extracts timestamp and optional dispatcher from a line matching TimestampLineRE.
func ParseTimestampLine(line string) (time.Time, string, bool) {
	matches := TimestampLineRE.FindStringSubmatch(line)
	if matches == nil {
		return time.Time{}, "", false
	}
	t, err := time.ParseInLocation("01/02/2006 15:04:05", matches[1], time.Local)
	if err != nil {
		return time.Time{}, "", false
	}
	dispatcher := ""
	if len(matches) > 2 {
		dispatcher = matches[2]
	}
	return t, dispatcher, true
}

// FormatTime formats a time.Time into RFC3339 string or empty if zero.
func FormatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

// ParseDateFromHeaders extracts Date from transport message headers.
func ParseDateFromHeaders(headers string) time.Time {
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

// ParseDateFromSubject extracts a date matching MM.DD.YY(YY) from subject lines.
func ParseDateFromSubject(subject string) time.Time {
	matches := SubjectDateRE.FindStringSubmatch(subject)
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

// ExpandEntries splits compound entry numbers like "252, 248 AND 236 MAIN ST" into individual entries.
func ExpandEntries(line string) []string {
	if listMatches := ListRE.FindStringSubmatch(line); listMatches != nil {
		numbers := NumberRE.FindAllString(listMatches[1], -1)
		remainder := listMatches[2]
		for {
			tailMatches := LeadingListTailRE.FindStringSubmatch(remainder)
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

type normalizedBodyLine struct {
	text                  string
	hadTrailingWhitespace bool
}

// CleanLines normalizes whitespace, splits wide-gap columns, and repairs wrapped lines from a message body.
func CleanLines(body string) []string {
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

func splitOnWideGaps(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if TimestampLineRE.MatchString(trimmed) || IsIntroLine(trimmed) || IsFooterLine(trimmed) {
		return []string{raw}
	}
	parts := WideGapRE.Split(trimmed, -1)
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) <= 1 {
		return []string{raw}
	}
	return out
}

func isWrappedContinuation(prev normalizedBodyLine, cur normalizedBodyLine) bool {
	switch {
	case prev.text == "", cur.text == "":
		return false
	case TimestampLineRE.MatchString(prev.text), TimestampLineRE.MatchString(cur.text):
		return false
	case IsFooterLine(prev.text), IsFooterLine(cur.text):
		return false
	case IsIntroLine(prev.text), IsIntroLine(cur.text):
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
	if match := SuffixRE.FindStringIndex(line); match != nil && match[0] <= 18 {
		return true
	}
	return false
}

// IsIntroLine checks if a line is a header/introductory greeting.
func IsIntroLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.HasPrefix(lower, "please see tags called in today") ||
		strings.HasPrefix(lower, "please find today")
}

// IsFooterLine checks if a line is an email footer or dispatcher signature block.
func IsFooterLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case SignatureLineRE.MatchString(lower):
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
