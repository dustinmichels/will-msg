package scanner

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

func TestFindSources_SingleMsg(t *testing.T) {
	msgPath := filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg")
	sources, err := FindSources(msgPath)
	if err != nil {
		t.Fatalf("FindSources error: %v", err)
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

func TestFindSources_Folder(t *testing.T) {
	testDir := filepath.Join("..", "..", "testdata")
	sources, err := FindSources(testDir)
	if err != nil {
		t.Fatalf("FindSources error: %v", err)
	}

	if len(sources) == 0 {
		t.Fatalf("expected at least 1 .msg file in testdata folder")
	}

	for _, src := range sources {
		if src.InZip {
			t.Errorf("expected InZip to be false for folder scan, got true for %s", src.Path)
		}
		if filepath.Ext(src.Path) != ".msg" {
			t.Errorf("expected .msg extension, got %s", src.Path)
		}
	}
}

func TestFindSources_Folder_WithIgnored(t *testing.T) {
	tempDir := t.TempDir()

	// Valid msg files
	_ = os.WriteFile(filepath.Join(tempDir, "file1.msg"), []byte("dummy"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "file2.MSG"), []byte("dummy"), 0644)

	// Non-msg file
	_ = os.WriteFile(filepath.Join(tempDir, "notes.txt"), []byte("dummy"), 0644)

	// Hidden file
	_ = os.WriteFile(filepath.Join(tempDir, "._hidden.msg"), []byte("dummy"), 0644)

	// Subfolder with hidden files & valid msg
	subDir := filepath.Join(tempDir, "subfolder")
	_ = os.MkdirAll(subDir, 0755)
	_ = os.WriteFile(filepath.Join(subDir, "file3.msg"), []byte("dummy"), 0644)
	_ = os.WriteFile(filepath.Join(subDir, ".DS_Store"), []byte("dummy"), 0644)

	// Ignored folder (__MACOSX)
	macDir := filepath.Join(tempDir, "__MACOSX")
	_ = os.MkdirAll(macDir, 0755)
	_ = os.WriteFile(filepath.Join(macDir, "file4.msg"), []byte("dummy"), 0644)

	// Hidden folder (.hidden_dir)
	hiddenDir := filepath.Join(tempDir, ".hidden_dir")
	_ = os.MkdirAll(hiddenDir, 0755)
	_ = os.WriteFile(filepath.Join(hiddenDir, "file5.msg"), []byte("dummy"), 0644)

	sources, err := FindSources(tempDir)
	if err != nil {
		t.Fatalf("FindSources error: %v", err)
	}

	if len(sources) != 3 {
		t.Fatalf("expected 3 valid sources, got %d", len(sources))
	}

	expectedDisplayNames := map[string]bool{
		"file1.msg":                     false,
		"file2.MSG":                     false,
		filepath.Join("subfolder", "file3.msg"): false,
	}

	for _, src := range sources {
		if _, exists := expectedDisplayNames[src.DisplayName]; !exists {
			t.Errorf("unexpected source found: %s (%s)", src.DisplayName, src.Path)
		} else {
			expectedDisplayNames[src.DisplayName] = true
		}
	}

	for name, found := range expectedDisplayNames {
		if !found {
			t.Errorf("expected source not found: %s", name)
		}
	}
}

func TestFindSources_Zip(t *testing.T) {
	zipFiles := map[string][]byte{
		"test1.msg":              []byte("dummy msg 1"),
		"nested/test2.msg":       []byte("dummy msg 2"),
		"nested/.hidden.msg":     []byte("dummy hidden msg"),
		"__MACOSX/._test1.msg":   []byte("dummy macosx"),
		"notes.txt":              []byte("dummy notes"),
	}

	zipPath := createTestZip(t, zipFiles)
	defer os.Remove(zipPath)

	sources, err := FindSources(zipPath)
	if err != nil {
		t.Fatalf("FindSources on zip error: %v", err)
	}

	if len(sources) != 2 {
		t.Fatalf("expected 2 valid sources in zip, got %d", len(sources))
	}

	for _, src := range sources {
		if !src.InZip {
			t.Errorf("expected InZip to be true for %s", src.Path)
		}
		if src.ZipPath != zipPath {
			t.Errorf("expected ZipPath %q, got %q", zipPath, src.ZipPath)
		}
	}
}

func TestShouldIgnore(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"normal/path/file.msg", false},
		{".hidden/file.msg", true},
		{"normal/.hidden/file.msg", true},
		{"normal/._file.msg", true},
		{"__MACOSX/file.msg", true},
		{"normal/__MACOSX/file.msg", true},
		{"normal/__macosx/file.msg", true},
		{"normal/file.msg", false},
		{"file.msg", false},
		{".file.msg", true},
		{"normal\\__MACOSX\\file.msg", true},
		{"normal\\.hidden\\file.msg", true},
	}

	for _, test := range tests {
		result := ShouldIgnore(test.path)
		if result != test.expected {
			t.Errorf("ShouldIgnore(%q) = %v, expected %v", test.path, result, test.expected)
		}
	}
}

func TestLoadSource_Zip(t *testing.T) {
	realMsgBytes, err := os.ReadFile(filepath.Join("..", "..", "testdata", "Medford Tags 01_02_26.msg"))
	if err != nil {
		t.Fatalf("read real msg file: %v", err)
	}

	zipPath := createTestZip(t, map[string][]byte{
		"archive/Medford Tags 01_02_26.msg": realMsgBytes,
	})
	defer os.Remove(zipPath)

	src := MessageSource{
		Path:        "archive/Medford Tags 01_02_26.msg",
		InZip:       true,
		ZipPath:     zipPath,
		DisplayName: "Medford Tags 01_02_26.msg (in archive.zip)",
	}

	meta, err := LoadSource(src)
	if err != nil {
		t.Fatalf("LoadSource failed: %v", err)
	}

	if meta.Subject != "Medford Tags 01.02.26" {
		t.Errorf("expected Subject 'Medford Tags 01.02.26', got %q", meta.Subject)
	}
	if len(meta.Body) == 0 {
		t.Errorf("expected non-empty body")
	}
}
