// msgcat prints the plain-text content of an Outlook .msg file to stdout.
//
// Usage:
//
//	msgcat [-headers=false] <file.msg>
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	msgparser "github.com/willthrom/outlook-msg-parser"
)

func main() {
	showHeaders := flag.Bool("headers", true, "print From/To/Subject/Date header block before body")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: msgcat [-headers=false] <file.msg>\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	body, headers, err := parse(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "msgcat: %v\n", err)
		os.Exit(1)
	}

	if *showHeaders {
		fmt.Println(headers)
		fmt.Println(strings.Repeat("-", 72))
	}
	fmt.Println(body)
}

func parse(path string) (body, headers string, err error) {
	// suppress noisy internal logging from the parser
	log.SetOutput(io.Discard)
	msg, err := msgparser.ParseMsgFile(path)
	log.SetOutput(os.Stderr)
	if err != nil {
		return "", "", err
	}

	body = strings.TrimSpace(msg.BodyPlainText)
	if body == "" {
		body = strings.TrimSpace(msg.ConvertedBodyHTML)
	}
	if body == "" {
		body = strings.TrimSpace(msg.BodyHTML)
	}
	if body == "" {
		return "", "", errors.New("message body is empty")
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
