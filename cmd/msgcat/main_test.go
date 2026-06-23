package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	msgPath := filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg")
	body, headers, err := parse(msgPath)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if !strings.Contains(headers, "Subject: Medford Tags 01.02.26") {
		t.Errorf("expected subject in headers, got: %q", headers)
	}

	if !strings.Contains(body, "CHARNWOOD RD MANY HOMES RECYC NOT OUT") {
		t.Errorf("expected body to contain sample entry, got: %q", body)
	}
}
