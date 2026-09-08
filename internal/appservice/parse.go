package appservice

import (
	"fmt"
	"path/filepath"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"will-msg/internal/engine"
	"will-msg/internal/scanner"
)

// SelectFiles prompts the user to select one or more .msg or .zip files.
func (s *Service) SelectFiles() ([]string, error) {
	files, err := s.dialogAdapter.OpenMultipleFilesDialog(s.context(), wailsruntime.OpenDialogOptions{
		Title: "Select Outlook Messages or Zip Archives",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Outlook Messages (*.msg)", Pattern: "*.msg"},
			{DisplayName: "Zip Archives (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return []string{}, err
	}
	if files == nil {
		return []string{}, nil
	}
	return files, nil
}

// SelectFolder prompts the user to select a directory containing .msg files.
func (s *Service) SelectFolder() (string, error) {
	folder, err := s.dialogAdapter.OpenDirectoryDialog(s.context(), wailsruntime.OpenDialogOptions{
		Title: "Select Folder Containing .msg Files",
	})
	if err != nil {
		return "", err
	}
	return folder, nil
}

// ScanSources scans all provided filesystem paths for valid .msg sources, deduplicating identical sources.
func (s *Service) ScanSources(paths []string) (ScanResult, error) {
	if paths == nil {
		paths = []string{}
	}

	allSources := make([]scanner.MessageSource, 0)
	seen := make(map[string]bool)

	for _, p := range paths {
		sources, err := scanner.FindSources(p)
		if err != nil {
			return ScanResult{
				Sources:     make([]scanner.MessageSource, 0),
				Count:       0,
				SourcePaths: paths,
			}, err
		}
		for _, src := range sources {
			cleanPath := src.Path
			cleanZip := src.ZipPath
			if !src.InZip {
				cleanPath = filepath.Clean(src.Path)
			} else {
				cleanZip = filepath.Clean(src.ZipPath)
			}
			key := fmt.Sprintf("%s|%t|%s", cleanPath, src.InZip, cleanZip)
			if !seen[key] {
				seen[key] = true
				allSources = append(allSources, src)
			}
		}
	}

	return ScanResult{
		Sources:     allSources,
		Count:       len(allSources),
		SourcePaths: paths,
	}, nil
}

// Parse processes the provided message sources using the active rule engine snapshot.
func (s *Service) Parse(sources []scanner.MessageSource) (ParseResult, error) {
	s.recordsMu.Lock()
	s.parseGen++
	gen := s.parseGen
	s.lastRecords = nil
	eng := s.engine.Load()
	s.recordsMu.Unlock()

	allRecords := make([]engine.Record, 0)
	skipped := make([]SkippedSource, 0)

	for _, src := range sources {
		meta, err := s.loadSource(src)
		if err != nil {
			skipped = append(skipped, SkippedSource{
				DisplayName: src.DisplayName,
				Error:       err.Error(),
			})
			continue
		}

		recs := eng.ParseRecords(meta)
		allRecords = append(allRecords, recs...)
	}

	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()

	if s.parseGen != gen {
		return ParseResult{
			Records:    make([]engine.Record, 0),
			Skipped:    skipped,
			Superseded: true,
		}, nil
	}

	if len(allRecords) > 0 {
		s.lastRecords = allRecords
	}

	return ParseResult{
		Records:    allRecords,
		Skipped:    skipped,
		Superseded: false,
	}, nil
}

// ClearParse invalidates active parse state and releases last recorded data.
func (s *Service) ClearParse() {
	s.recordsMu.Lock()
	defer s.recordsMu.Unlock()
	s.parseGen++
	s.lastRecords = nil
}
