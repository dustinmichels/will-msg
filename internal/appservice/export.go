package appservice

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"will-msg/internal/engine"
)

// SaveToDownloads exports the last parsed records to a timestamped CSV file in the Downloads folder.
func (s *Service) SaveToDownloads() (SavedFile, error) {
	s.recordsMu.Lock()
	records := s.lastRecords
	s.recordsMu.Unlock()

	if len(records) == 0 {
		return SavedFile{}, errors.New("no records available to export")
	}

	dir, err := s.downloadsDir()
	if err != nil {
		return SavedFile{}, fmt.Errorf("locate downloads directory: %w", err)
	}

	filename := fmt.Sprintf("msg_parsed_%s.csv", time.Now().Format("2006-01-02_150405"))
	path := filepath.Join(dir, filename)

	if err := writeRecordsToCSV(path, records); err != nil {
		return SavedFile{}, err
	}

	return SavedFile{
		Filename:  filename,
		Dir:       dir,
		Path:      path,
		Cancelled: false,
	}, nil
}

// SaveAs prompts the user for a destination file path and exports the last parsed records.
func (s *Service) SaveAs() (SavedFile, error) {
	s.recordsMu.Lock()
	records := s.lastRecords
	s.recordsMu.Unlock()

	if len(records) == 0 {
		return SavedFile{}, errors.New("no records available to export")
	}

	defaultFilename := fmt.Sprintf("msg_parsed_%s.csv", time.Now().Format("2006-01-02_150405"))
	selectedPath, err := s.dialogAdapter.SaveFileDialog(s.context(), wailsruntime.SaveDialogOptions{
		Title:           "Save Parsed CSV",
		DefaultFilename: defaultFilename,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "CSV Files (*.csv)", Pattern: "*.csv"},
		},
	})
	if err != nil {
		return SavedFile{}, err
	}
	if selectedPath == "" {
		return SavedFile{Cancelled: true}, nil
	}

	if err := writeRecordsToCSV(selectedPath, records); err != nil {
		return SavedFile{}, err
	}

	return SavedFile{
		Filename:  filepath.Base(selectedPath),
		Dir:       filepath.Dir(selectedPath),
		Path:      selectedPath,
		Cancelled: false,
	}, nil
}

// RevealFile shows the specified file in the system file explorer or file manager.
func (s *Service) RevealFile(path string) error {
	return s.revealer(s.context(), path)
}

// writeRecordsToCSV serializes records to CSV format with headers, checking flush and close errors.
func writeRecordsToCSV(path string, records []engine.Record) (err error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}()

	writer := csv.NewWriter(file)
	if err := writer.Write(engine.CSVHeaders); err != nil {
		return err
	}

	for _, rec := range records {
		if err := writer.Write(rec.ToRow()); err != nil {
			return err
		}
	}

	writer.Flush()
	if flushErr := writer.Error(); flushErr != nil {
		return flushErr
	}

	return nil
}

// WriteRecordsToCSVForTest exposes writeRecordsToCSV for unit testing.
func WriteRecordsToCSVForTest(path string, records []engine.Record) error {
	return writeRecordsToCSV(path, records)
}
