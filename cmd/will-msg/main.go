package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"will-msg/internal/config"
	"will-msg/internal/engine"
	"will-msg/internal/scanner"
)

type parseSummary struct {
	ParsedFiles  int
	SkippedFiles int
}

func parseSources(sources []scanner.MessageSource, eng *engine.RuleEngine) ([]engine.Record, parseSummary, error) {
	if eng == nil {
		eng = engine.NewRuleEngine(config.DefaultRuleConfig())
	}

	allRecords := make([]engine.Record, 0)
	var summary parseSummary

	for _, src := range sources {
		meta, err := scanner.LoadSource(src)
		if err != nil {
			summary.SkippedFiles++
			log.Printf("warning: skipping %s: %v", src.DisplayName, err)
			continue
		}

		records := eng.ParseRecords(meta)
		allRecords = append(allRecords, records...)
		summary.ParsedFiles++
	}

	return allRecords, summary, nil
}

func writeCSV(path string, records []engine.Record) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
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
	return writer.Error()
}

func main() {
	log.SetFlags(0)
	input := flag.String("input", "", "Path to a .msg file, .zip archive, or directory of .msg files")
	output := flag.String("output", "", "Path to the CSV file to write")
	flag.Parse()

	if *input == "" || *output == "" {
		flag.Usage()
		os.Exit(2)
	}

	sources, err := scanner.FindSources(*input)
	if err != nil {
		log.Fatalf("scan inputs: %v", err)
	}
	if len(sources) == 0 {
		log.Fatalf("no .msg files found in %s", *input)
	}

	eng := engine.NewRuleEngine(config.LoadConfig())
	records, summary, err := parseSources(sources, eng)
	if err != nil {
		log.Fatalf("parse inputs: %v", err)
	}
	if len(records) == 0 {
		log.Fatalf("no structured rows found in %s", *input)
	}

	if err := writeCSV(*output, records); err != nil {
		log.Fatalf("write csv: %v", err)
	}

	fmt.Printf("parsed %d files into %d rows; skipped %d files\n", summary.ParsedFiles, len(records), summary.SkippedFiles)
}
