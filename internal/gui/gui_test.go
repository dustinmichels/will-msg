package gui

import (
	"path/filepath"
	"testing"

	"will-msg/internal/scanner"
)

func TestGetDownloadsDir(t *testing.T) {
	dir := getDownloadsDir()
	if dir == "" {
		t.Error("expected non-empty downloads directory")
	}
}

func TestTruckResource(t *testing.T) {
	if len(truckPNGBytes) == 0 {
		t.Fatal("expected truckPNGBytes to be embedded and non-empty")
	}
	if truckResource == nil {
		t.Fatal("expected truckResource to not be nil")
	}
	if truckResource.Name() != "truck.png" {
		t.Errorf("expected truckResource name %q, got %q", "truck.png", truckResource.Name())
	}
	if len(truckResource.Content()) == 0 {
		t.Fatal("expected truckResource content to be non-empty")
	}
}

func TestParseMsgSources(t *testing.T) {
	msgPath := filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg")
	sources, err := scanner.FindSources(msgPath)
	if err != nil {
		t.Fatalf("FindSources failed: %v", err)
	}

	records, err := parseMsgSources(sources)
	if err != nil {
		t.Fatalf("parseMsgSources failed: %v", err)
	}

	if len(records) != 5 {
		t.Errorf("expected 5 records, got %d", len(records))
	}
}
