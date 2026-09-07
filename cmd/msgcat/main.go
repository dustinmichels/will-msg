// msgcat prints the plain-text content of an Outlook .msg file to stdout.
//
// Usage:
//
//	msgcat [-headers=false] <file.msg>
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"will-msg/internal/parser"
)

func main() {
	showHeaders := flag.Bool("headers", true, "print From/To/Subject/Date header block before body")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [-headers=false] <file.msg>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	body, headers, err := parser.ExtractBodyAndHeaders(flag.Arg(0))
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
