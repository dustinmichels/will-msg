package scanner

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"will-msg/internal/parser"
)

// MessageSource represents a located .msg file on the local filesystem or inside a .zip archive.
type MessageSource struct {
	Path        string // path on disk OR path inside zip
	InZip       bool   // whether it's inside a zip
	ZipPath     string // path to the zip file itself
	DisplayName string // name shown in the UI list or logs
}

// ShouldIgnore determines whether a path or path component should be skipped (hidden files, macOS metadata).
func ShouldIgnore(path string) bool {
	normalized := strings.ReplaceAll(path, "\\", "/")
	for _, part := range strings.Split(normalized, "/") {
		if part == "" || part == "." || part == ".." {
			continue
		}
		if strings.HasPrefix(part, ".") || strings.EqualFold(part, "__MACOSX") {
			return true
		}
	}
	return false
}

// FindSources scans a file, directory, or .zip archive for all valid .msg files.
func FindSources(path string) ([]MessageSource, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	var sources []MessageSource

	if !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".msg" {
			sources = append(sources, MessageSource{
				Path:        path,
				DisplayName: filepath.Base(path),
			})
		} else if ext == ".zip" {
			r, err := zip.OpenReader(path)
			if err != nil {
				return nil, fmt.Errorf("open zip: %w", err)
			}
			defer r.Close()

			for _, f := range r.File {
				if f.FileInfo().IsDir() {
					continue
				}
				if ShouldIgnore(f.Name) {
					continue
				}
				if strings.EqualFold(filepath.Ext(f.Name), ".msg") {
					sources = append(sources, MessageSource{
						Path:        f.Name,
						InZip:       true,
						ZipPath:     path,
						DisplayName: f.Name + " (in " + filepath.Base(path) + ")",
					})
				}
			}
		} else {
			return nil, fmt.Errorf("unsupported file extension: %s", ext)
		}
	} else {
		err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(path, p)
			if err != nil {
				rel = filepath.Base(p)
			}
			if ShouldIgnore(rel) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.IsDir() && strings.EqualFold(filepath.Ext(p), ".msg") {
				sources = append(sources, MessageSource{
					Path:        p,
					DisplayName: rel,
				})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return sources, nil
}

// LoadSource reads and parses message metadata from disk or an archive.
func LoadSource(src MessageSource) (parser.MessageMetadata, error) {
	if !src.InZip {
		return parser.LoadMessage(src.Path)
	}

	r, err := zip.OpenReader(src.ZipPath)
	if err != nil {
		return parser.MessageMetadata{}, err
	}
	defer r.Close()

	var zipFile *zip.File
	for _, f := range r.File {
		if f.Name == src.Path {
			zipFile = f
			break
		}
	}
	if zipFile == nil {
		return parser.MessageMetadata{}, fmt.Errorf("file not found in zip: %s", src.Path)
	}

	rc, err := zipFile.Open()
	if err != nil {
		return parser.MessageMetadata{}, err
	}
	defer rc.Close()

	tempFile, err := os.CreateTemp("", "msg-*.msg")
	if err != nil {
		return parser.MessageMetadata{}, err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := io.Copy(tempFile, rc); err != nil {
		tempFile.Close()
		return parser.MessageMetadata{}, err
	}
	if err := tempFile.Close(); err != nil {
		return parser.MessageMetadata{}, err
	}

	meta, err := parser.LoadMessage(tempPath)
	if err != nil {
		return parser.MessageMetadata{}, err
	}
	meta.SourceFile = filepath.Base(src.Path)
	return meta, nil
}
