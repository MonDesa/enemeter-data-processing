package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

type EnemeterRecord struct {
	TimeDeltaMs     int64
	TempMiliCelsius int64
	VoltageMicroV   int64
	CurrentNanoA    int64
	Timestamp       time.Time
}

type FilterOptions struct {
	StartTime      *time.Time
	EndTime        *time.Time
	SampleRate     int
	MaxRecords     int
	TempThreshold  *int64
	VoltageRange   *[2]int64
	CurrentRange   *[2]int64
	SelectedFields []string
}

type CSVParser struct {
	filePath string
	options  FilterOptions
}

func NewCSVParser(filePath string) *CSVParser {
	return &CSVParser{
		filePath: filePath,
		options: FilterOptions{
			SampleRate: 1,
		},
	}
}

func (p *CSVParser) WithFilterOptions(options FilterOptions) *CSVParser {
	p.options = options
	return p
}

func (p *CSVParser) Parse() ([]EnemeterRecord, error) {
	file, err := os.Open(p.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			if err == nil {
				err = fmt.Errorf("error closing file: %w", closeErr)
			}
		}
	}()

	reader := csv.NewReader(file)
	var records []EnemeterRecord

	if p.options.StartTime == nil {
		return nil, fmt.Errorf("start time must be provided")
	}

	// Use the provided start time since it's required
	startTime := *p.options.StartTime

	accumulatedTimeMs := int64(0)
	recordCount := 0
	sampleCounter := 0

	const (
		timeCol    = 0
		voltageCol = 1
		currentCol = 2
		tempCol    = 3
	)

	for p.options.MaxRecords <= 0 || len(records) < p.options.MaxRecords {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV row: %w", err)
		}

		sampleCounter++
		if sampleCounter < p.options.SampleRate {
			continue
		}
		sampleCounter = 0

		if len(row) != 4 {
			return nil, fmt.Errorf("invalid row format, expected 4 fields but got %d", len(row))
		}

		timeDelta, err := strconv.ParseInt(row[timeCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse time delta: %w", err)
		}

		voltageMicroV, err := strconv.ParseInt(row[voltageCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse voltage: %w", err)
		}

		currentNanoA, err := strconv.ParseInt(row[currentCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse current: %w", err)
		}

		tempMiliCelsius, err := strconv.ParseInt(row[tempCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse temperature: %w", err)
		}

		accumulatedTimeMs += timeDelta
		timestamp := startTime.Add(time.Duration(accumulatedTimeMs) * time.Millisecond)

		if p.options.StartTime != nil && timestamp.Before(*p.options.StartTime) {
			continue
		}
		if p.options.EndTime != nil && timestamp.After(*p.options.EndTime) {
			break
		}

		if p.options.TempThreshold != nil && tempMiliCelsius < *p.options.TempThreshold {
			continue
		}

		if p.options.VoltageRange != nil && (voltageMicroV < p.options.VoltageRange[0] || voltageMicroV > p.options.VoltageRange[1]) {
			continue
		}

		if p.options.CurrentRange != nil && (currentNanoA < p.options.CurrentRange[0] || currentNanoA > p.options.CurrentRange[1]) {
			continue
		}

		record := EnemeterRecord{
			TimeDeltaMs:     timeDelta,
			TempMiliCelsius: tempMiliCelsius,
			VoltageMicroV:   voltageMicroV,
			CurrentNanoA:    currentNanoA,
			Timestamp:       timestamp,
		}

		records = append(records, record)
		recordCount++
	}

	return records, nil
}

func (p *CSVParser) GetFileSize() (int64, error) {
	fileInfo, err := os.Stat(p.filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	return fileInfo.Size(), nil
}

func (p *CSVParser) GetRecordCount() (int, error) {
	file, err := os.Open(p.filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			if err == nil {
				err = closeErr
			}
		}
	}()

	fileInfo, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	reader := csv.NewReader(file)
	lineCount := 0
	bytesRead := int64(0)

	for i := 0; i < 100; i++ {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("error reading CSV: %w", err)
		}

		lineBytes := int64(0)
		for _, field := range line {
			lineBytes += int64(len(field) + 1)
		}
		lineBytes += 1

		bytesRead += lineBytes
		lineCount++
	}

	if lineCount == 0 {
		return 0, nil
	}

	avgLineSize := bytesRead / int64(lineCount)
	estimatedRecords := fileSize / avgLineSize

	return int(estimatedRecords), nil
}

func (p *CSVParser) StreamRecords(callback func(record EnemeterRecord) error) error {
	file, err := os.Open(p.filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			if err == nil {
				err = fmt.Errorf("error closing file: %w", closeErr)
			}
		}
	}()

	reader := csv.NewReader(file)

	if p.options.StartTime == nil {
		return fmt.Errorf("start time must be provided")
	}

	// Use the provided start time since it's required
	startTime := *p.options.StartTime

	accumulatedTimeMs := int64(0)
	recordCount := 0
	sampleCounter := 0

	const (
		timeCol    = 0
		voltageCol = 1
		currentCol = 2
		tempCol    = 3
	)

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading CSV row: %w", err)
		}

		sampleCounter++
		if sampleCounter < p.options.SampleRate {
			continue
		}
		sampleCounter = 0

		if p.options.MaxRecords > 0 && recordCount >= p.options.MaxRecords {
			break
		}

		if len(row) != 4 {
			return fmt.Errorf("invalid row format, expected 4 fields but got %d", len(row))
		}

		timeDelta, err := strconv.ParseInt(row[timeCol], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse time delta: %w", err)
		}

		voltageMicroV, err := strconv.ParseInt(row[voltageCol], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse voltage: %w", err)
		}

		currentNanoA, err := strconv.ParseInt(row[currentCol], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse current: %w", err)
		}

		tempMiliCelsius, err := strconv.ParseInt(row[tempCol], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse temperature: %w", err)
		}

		accumulatedTimeMs += timeDelta
		timestamp := startTime.Add(time.Duration(accumulatedTimeMs) * time.Millisecond)

		if p.options.StartTime != nil && timestamp.Before(*p.options.StartTime) {
			continue
		}
		if p.options.EndTime != nil && timestamp.After(*p.options.EndTime) {
			break
		}

		if p.options.TempThreshold != nil && tempMiliCelsius < *p.options.TempThreshold {
			continue
		}

		if p.options.VoltageRange != nil && (voltageMicroV < p.options.VoltageRange[0] || voltageMicroV > p.options.VoltageRange[1]) {
			continue
		}

		if p.options.CurrentRange != nil && (currentNanoA < p.options.CurrentRange[0] || currentNanoA > p.options.CurrentRange[1]) {
			continue
		}

		record := EnemeterRecord{
			TimeDeltaMs:     timeDelta,
			TempMiliCelsius: tempMiliCelsius,
			VoltageMicroV:   voltageMicroV,
			CurrentNanoA:    currentNanoA,
			Timestamp:       timestamp,
		}

		if err := callback(record); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}

		recordCount++
	}

	return nil
}
