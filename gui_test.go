package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func createTestZip(t *testing.T, files map[string][]byte) string {
	tmpFile, err := os.CreateTemp("", "test-*.zip")
	if err != nil {
		t.Fatalf("create temp zip: %v", err)
	}
	defer tmpFile.Close()

	zipWriter := zip.NewWriter(tmpFile)
	for name, content := range files {
		f, err := zipWriter.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		_, err = f.Write(content)
		if err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	err = zipWriter.Close()
	if err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	return tmpFile.Name()
}

func TestFindMsgFiles_SingleMsg(t *testing.T) {
	msgPath := filepath.Join("data", "Medford Tags 01_02_26.msg")
	sources, err := findMsgFiles(msgPath)
	if err != nil {
		t.Fatalf("findMsgFiles error: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	if sources[0].Path != msgPath {
		t.Errorf("expected path %q, got %q", msgPath, sources[0].Path)
	}
	if sources[0].InZip {
		t.Errorf("expected InZip to be false")
	}
}

func TestFindMsgFiles_Folder(t *testing.T) {
	sources, err := findMsgFiles("data")
	if err != nil {
		t.Fatalf("findMsgFiles error: %v", err)
	}

	if len(sources) != 5 {
		t.Fatalf("expected 5 sources, got %d", len(sources))
	}

	for _, src := range sources {
		if filepath.Ext(src.Path) != ".msg" {
			t.Errorf("expected .msg extension for %q", src.Path)
		}
		if src.InZip {
			t.Errorf("expected InZip to be false for %q", src.Path)
		}
	}
}

func TestFindMsgFiles_Zip(t *testing.T) {
	realMsgContent, err := os.ReadFile(filepath.Join("data", "Medford Tags 01_02_26.msg"))
	if err != nil {
		t.Fatalf("read real msg file: %v", err)
	}

	zipPath := createTestZip(t, map[string][]byte{
		"nested/email1.msg": realMsgContent,
		"readme.txt":        []byte("hello world"),
	})
	defer os.Remove(zipPath)

	sources, err := findMsgFiles(zipPath)
	if err != nil {
		t.Fatalf("findMsgFiles error: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	src := sources[0]
	if src.Path != "nested/email1.msg" {
		t.Errorf("expected path 'nested/email1.msg', got %q", src.Path)
	}
	if !src.InZip {
		t.Errorf("expected InZip to be true")
	}
	if src.ZipPath != zipPath {
		t.Errorf("expected ZipPath %q, got %q", zipPath, src.ZipPath)
	}

	records, err := parseMsgSources(sources)
	if err != nil {
		t.Fatalf("parseMsgSources error: %v", err)
	}

	if len(records) != 4 {
		t.Errorf("expected 4 records, got %d", len(records))
	}

	for _, rec := range records {
		if rec.SourceFile != "email1.msg" {
			t.Errorf("expected SourceFile 'email1.msg', got %q", rec.SourceFile)
		}
	}
}

func TestNewBorderLayoutObjectsOrder(t *testing.T) {
	icon := widget.NewIcon(theme.DocumentIcon())
	label := widget.NewLabel("test")
	border := container.NewBorder(nil, nil, icon, nil, label)
	for i, obj := range border.Objects {
		t.Logf("Objects[%d] type: %T", i, obj)
	}

	if len(border.Objects) < 2 {
		t.Fatalf("expected at least 2 objects, got %d", len(border.Objects))
	}

	if _, ok := border.Objects[0].(*widget.Label); !ok {
		t.Errorf("expected border.Objects[0] to be *widget.Label, got %T", border.Objects[0])
	}
	if _, ok := border.Objects[1].(*widget.Icon); !ok {
		t.Errorf("expected border.Objects[1] to be *widget.Icon, got %T", border.Objects[1])
	}
}
