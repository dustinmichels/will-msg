package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
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
	msgPath := filepath.Join("testdata", "Medford Tags 01_02_26.msg")
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
	sources, err := findMsgFiles("testdata")
	if err != nil {
		t.Fatalf("findMsgFiles error: %v", err)
	}

	if len(sources) != 6 {
		t.Fatalf("expected 6 sources, got %d", len(sources))
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

func TestFindMsgFiles_Folder_WithIgnored(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-folder-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	validFile1 := filepath.Join(tmpDir, "email1.msg")
	if err := os.WriteFile(validFile1, []byte("valid"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	validFile2 := filepath.Join(tmpDir, "sub", "email2.msg")
	if err := os.Mkdir(filepath.Dir(validFile2), 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	if err := os.WriteFile(validFile2, []byte("valid"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ignoredFile1 := filepath.Join(tmpDir, "._email1.msg")
	if err := os.WriteFile(ignoredFile1, []byte("ignored"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	ignoredDir := filepath.Join(tmpDir, "__MACOSX")
	if err := os.Mkdir(ignoredDir, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	ignoredFile2 := filepath.Join(ignoredDir, "email3.msg")
	if err := os.WriteFile(ignoredFile2, []byte("ignored"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	ignoredDotDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(ignoredDotDir, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	ignoredFile3 := filepath.Join(ignoredDotDir, "email4.msg")
	if err := os.WriteFile(ignoredFile3, []byte("ignored"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	sources, err := findMsgFiles(tmpDir)
	if err != nil {
		t.Fatalf("findMsgFiles error: %v", err)
	}

	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}

	found1, found2 := false, false
	for _, src := range sources {
		base := filepath.Base(src.Path)
		if base == "email1.msg" {
			found1 = true
		} else if base == "email2.msg" {
			found2 = true
		} else {
			t.Errorf("found unexpected file: %q", src.Path)
		}
	}

	if !found1 || !found2 {
		t.Errorf("did not find expected files (email1.msg: %v, email2.msg: %v)", found1, found2)
	}
}

func TestFindMsgFiles_HiddenAncestor(t *testing.T) {
	parentTmpDir, err := os.MkdirTemp("", "test-parent-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(parentTmpDir)

	hiddenDir := filepath.Join(parentTmpDir, ".hidden", "mail")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	validFile := filepath.Join(hiddenDir, "email.msg")
	if err := os.WriteFile(validFile, []byte("valid"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	sources, err := findMsgFiles(hiddenDir)
	if err != nil {
		t.Fatalf("findMsgFiles error: %v", err)
	}

	if len(sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(sources))
	}

	if filepath.Base(sources[0].Path) != "email.msg" {
		t.Errorf("expected email.msg, got %q", sources[0].Path)
	}
}

func TestFindMsgFiles_Zip(t *testing.T) {
	realMsgContent, err := os.ReadFile(filepath.Join("testdata", "Medford Tags 01_02_26.msg"))
	if err != nil {
		t.Fatalf("read real msg file: %v", err)
	}

	zipPath := createTestZip(t, map[string][]byte{
		"nested/email1.msg":            realMsgContent,
		"readme.txt":                   []byte("hello world"),
		"__MACOSX/nested/._email1.msg": realMsgContent,
		"nested/._email1.msg":          realMsgContent,
		".DS_Store":                    []byte("some ds store"),
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

	if len(records) != 5 {
		t.Errorf("expected 5 records, got %d", len(records))
	}

	for _, rec := range records {
		if rec.SourceFile != "email1.msg" {
			t.Errorf("expected SourceFile 'email1.msg', got %q", rec.SourceFile)
		}
	}
}


func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"data/Medford Tags 01_02_26.msg", false},
		{"Medford.msg", false},
		{"./Medford.msg", false},
		{"../Medford.msg", false},
		{"__MACOSX/data/._Medford.msg", true},
		{"data/._Medford.msg", true},
		{".DS_Store", true},
		{"data/.git/config", true},
		{"data/__macosx/email.msg", true},
		{"C:\\Users\\User\\__MACOSX\\file.msg", true},
		{"C:\\Users\\User\\.config\\file.msg", true},
	}

	for _, test := range tests {
		got := shouldIgnore(test.path)
		if got != test.expected {
			t.Errorf("shouldIgnore(%q) = %v; want %v", test.path, got, test.expected)
		}
	}
}

func TestGetDownloadsDir(t *testing.T) {
	dir := getDownloadsDir()
	if dir == "" {
		t.Error("expected non-empty downloads directory")
	}
}
