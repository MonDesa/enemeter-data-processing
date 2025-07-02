package parser

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Add variables for progress reporting
var (
	progressReportingEnabled = false
	progressMutex            sync.Mutex
)

// EnableProgressReporting enables detailed progress reporting during parsing
func EnableProgressReporting() {
	progressMutex.Lock()
	defer progressMutex.Unlock()
	progressReportingEnabled = true
}

// DisableProgressReporting disables detailed progress reporting
func DisableProgressReporting() {
	progressMutex.Lock()
	defer progressMutex.Unlock()
	progressReportingEnabled = false
}

// isProgressReportingEnabled safely checks if progress reporting is enabled
func isProgressReportingEnabled() bool {
	progressMutex.Lock()
	defer progressMutex.Unlock()
	return progressReportingEnabled
}

type EnemeterRecord struct {
	TimeDeltaMs     int64
	TempMiliCelsius int64
	VoltageMicroV   int64
	CurrentNanoA    int64
	Timestamp       time.Time
}

type FilterOptions struct {
	StartTime                   *time.Time
	EndTime                     *time.Time
	SampleRate                  int
	MaxRecords                  int
	TempThreshold               *int64
	VoltageRange                *[2]int64
	CurrentRange                *[2]int64
	SelectedFields              []string
	PreserveTimeRangeBoundaries []EnemeterRecord // Add this field
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

// GetTimeRangeBoundaries returns the first and last records in the file
func (p *CSVParser) GetTimeRangeBoundaries() ([]EnemeterRecord, error) {
	file, err := os.Open(p.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create a buffered reader for better performance
	bufferedReader := bufio.NewReaderSize(file, 1024*1024) // 1MB buffer
	reader := csv.NewReader(bufferedReader)

	if p.options.StartTime == nil {
		return nil, fmt.Errorf("start time must be provided")
	}

	startTime := *p.options.StartTime

	var firstRecord, lastRecord EnemeterRecord
	var foundFirst, foundLast bool
	lineNum := 0
	accumulatedTimeMs := int64(0)

	// First, quickly scan to find first valid record
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue // Skip errors silently to find first valid record
		}
		lineNum++

		// Try to parse row
		if len(row) != 4 {
			continue
		}

		timeDelta, err := strconv.ParseInt(row[0], 10, 64)
		if err != nil {
			continue
		}

		voltageMicroV, err := strconv.ParseInt(row[1], 10, 64)
		if err != nil {
			continue
		}

		currentNanoA, err := strconv.ParseInt(row[2], 10, 64)
		if err != nil {
			continue
		}

		tempMiliCelsius, err := strconv.ParseInt(row[3], 10, 64)
		if err != nil {
			continue
		}

		// Calculate timestamp
		accumulatedTimeMs += timeDelta
		timestamp := startTime.Add(time.Duration(accumulatedTimeMs) * time.Millisecond)

		// If this is a valid record, save it as the first record and break
		firstRecord = EnemeterRecord{
			TimeDeltaMs:     timeDelta,
			TempMiliCelsius: tempMiliCelsius,
			VoltageMicroV:   voltageMicroV,
			CurrentNanoA:    currentNanoA,
			Timestamp:       timestamp,
		}
		foundFirst = true
		break
	}

	// Now let's find the last record - quickest way is to scan most of the file
	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// For large files, we'll skip to near the end
	seekPos := fileSize - 1024*1024 // 1MB from end
	if seekPos < 0 {
		seekPos = 0
	}

	// Start from the beginning again to reset accumulatedTimeMs
	file.Seek(0, io.SeekStart)
	accumulatedTimeMs = int64(0)

	// Process the entire file to calculate the correct timestamp for the last record
	fmt.Println("Scanning file to determine full time range...")
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if len(row) != 4 {
			continue
		}

		timeDelta, err := strconv.ParseInt(row[0], 10, 64)
		if err != nil {
			continue
		}

		voltageMicroV, err := strconv.ParseInt(row[1], 10, 64)
		if err != nil {
			continue
		}

		currentNanoA, err := strconv.ParseInt(row[2], 10, 64)
		if err != nil {
			continue
		}

		tempMiliCelsius, err := strconv.ParseInt(row[3], 10, 64)
		if err != nil {
			continue
		}

		// Calculate timestamp
		accumulatedTimeMs += timeDelta
		timestamp := startTime.Add(time.Duration(accumulatedTimeMs) * time.Millisecond)

		// Always update last record so we end up with the final one
		lastRecord = EnemeterRecord{
			TimeDeltaMs:     timeDelta,
			TempMiliCelsius: tempMiliCelsius,
			VoltageMicroV:   voltageMicroV,
			CurrentNanoA:    currentNanoA,
			Timestamp:       timestamp,
		}
		foundLast = true
	}

	if !foundFirst || !foundLast {
		return nil, fmt.Errorf("couldn't find valid records in file")
	}

	fmt.Printf("Found time boundaries: %s to %s (%.2f hours)\n",
		firstRecord.Timestamp.Format("2006-01-02 15:04:05"),
		lastRecord.Timestamp.Format("2006-01-02 15:04:05"),
		lastRecord.Timestamp.Sub(firstRecord.Timestamp).Hours())

	return []EnemeterRecord{firstRecord, lastRecord}, nil
}

// GetFileSize returns the size of the CSV file in bytes
func (p *CSVParser) GetFileSize() (int64, error) {
	fileInfo, err := os.Stat(p.filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	return fileInfo.Size(), nil
}

// GetRecordCount attempts to estimate the number of records in the CSV file
// This is an approximation based on sampling the first few lines
func (p *CSVParser) GetRecordCount() (int, error) {
	file, err := os.Open(p.filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file size
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// Create a buffered reader for better performance
	bufferedReader := bufio.NewReaderSize(file, 64*1024) // 64KB buffer
	reader := csv.NewReader(bufferedReader)

	// Sample the first 1000 lines to estimate average line size
	var totalSize int64
	var lineCount int

	// Read up to 1000 lines and count their bytes
	for i := 0; i < 1000; i++ {
		startPos, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			break
		}

		_, err = reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		endPos, err := file.Seek(0, io.SeekCurrent)
		if err != nil {
			break
		}

		lineSize := endPos - startPos
		totalSize += lineSize
		lineCount++
	}

	if lineCount == 0 {
		return 0, fmt.Errorf("couldn't read any lines to estimate record count")
	}

	// Calculate average line size
	avgLineSize := float64(totalSize) / float64(lineCount)

	// Estimate total number of records
	estimatedRecords := int(float64(fileSize) / avgLineSize)

	return estimatedRecords, nil
}

// StreamRecords processes records one at a time without loading all records into memory
// The provided function is called for each record
func (p *CSVParser) StreamRecords(processFunc func(EnemeterRecord) error) error {
	file, err := os.Open(p.filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file size for progress reporting
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// Create a buffered reader for better performance
	bufferedReader := bufio.NewReaderSize(file, 1024*1024) // 1MB buffer
	reader := csv.NewReader(bufferedReader)

	// For large files, consider memory efficient processing
	showProgress := isProgressReportingEnabled() && fileSize > 10*1024*1024 // Show progress for files > 10MB

	if p.options.StartTime == nil {
		return fmt.Errorf("start time must be provided")
	}

	// Use the provided start time since it's required
	startTime := *p.options.StartTime

	// Set up progress reporting
	var processedBytes int64
	var recordCount int32
	var lastReportTime time.Time

	if showProgress {
		lastReportTime = time.Now()
		fmt.Printf("Streaming file: %.2f MB\n", float64(fileSize)/(1024*1024))

		// Start progress monitoring in a goroutine
		progressDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					currentBytes := atomic.LoadInt64(&processedBytes)
					currentRecords := atomic.LoadInt32(&recordCount)
					progress := float64(currentBytes) / float64(fileSize) * 100

					// Calculate processing speed
					elapsed := time.Since(lastReportTime).Seconds()
					recordsPerSec := float64(currentRecords) / elapsed

					fmt.Printf("\rStreaming: %.1f%% complete | %d records | %.1f records/sec",
						progress, currentRecords, recordsPerSec)
				case <-progressDone:
					return
				}
			}
		}()

		// Make sure to close the progress channel when done
		defer func() {
			close(progressDone)
			fmt.Println() // Line break after progress reporting
		}()
	}

	// Prepare processing
	accumulatedTimeMs := int64(0)
	sampleCounter := 0

	const (
		timeCol    = 0
		voltageCol = 1
		currentCol = 2
		tempCol    = 3
	)

	// Process CSV file line by line
	lineNum := 0
	for {
		// Read the next line
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading CSV row %d: %w", lineNum+1, err)
		}
		lineNum++

		// Update processed bytes (approximate)
		if showProgress {
			// Estimate bytes processed (row length + newline)
			rowSize := 0
			for _, field := range row {
				rowSize += len(field) + 1 // +1 for delimiter
			}
			atomic.AddInt64(&processedBytes, int64(rowSize))
		}

		// Apply sampling based on sample rate
		sampleCounter++
		if sampleCounter < p.options.SampleRate {
			continue
		}
		sampleCounter = 0

		// Validate row format
		if len(row) != 4 {
			return fmt.Errorf("invalid row format at line %d, expected 4 fields but got %d", lineNum, len(row))
		}

		// Parse fields
		timeDelta, err := strconv.ParseInt(row[timeCol], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse time delta at line %d: %w", lineNum, err)
		}

		voltageMicroV, err := strconv.ParseInt(row[voltageCol], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse voltage at line %d: %w", lineNum, err)
		}

		currentNanoA, err := strconv.ParseInt(row[currentCol], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse current at line %d: %w", lineNum, err)
		}

		tempMiliCelsius, err := strconv.ParseInt(row[tempCol], 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse temperature at line %d: %w", lineNum, err)
		}

		// Calculate timestamp
		accumulatedTimeMs += timeDelta
		timestamp := startTime.Add(time.Duration(accumulatedTimeMs) * time.Millisecond)

		// Apply time filtering
		if p.options.StartTime != nil && timestamp.Before(*p.options.StartTime) {
			continue
		}
		if p.options.EndTime != nil && timestamp.After(*p.options.EndTime) {
			continue
		}

		// Apply data filtering
		if p.options.TempThreshold != nil && tempMiliCelsius < *p.options.TempThreshold {
			continue
		}

		if p.options.VoltageRange != nil && (voltageMicroV < p.options.VoltageRange[0] || voltageMicroV > p.options.VoltageRange[1]) {
			continue
		}

		if p.options.CurrentRange != nil && (currentNanoA < p.options.CurrentRange[0] || currentNanoA > p.options.CurrentRange[1]) {
			continue
		}

		// Create record
		record := EnemeterRecord{
			TimeDeltaMs:     timeDelta,
			TempMiliCelsius: tempMiliCelsius,
			VoltageMicroV:   voltageMicroV,
			CurrentNanoA:    currentNanoA,
			Timestamp:       timestamp,
		}

		// Process the record
		if err := processFunc(record); err != nil {
			return fmt.Errorf("error processing record at line %d: %w", lineNum, err)
		}

		// Update record count for progress tracking
		if showProgress {
			atomic.AddInt32(&recordCount, 1)
		}
	}

	// Final progress update
	if showProgress {
		fmt.Printf("\rProcessed %d records from %.2f MB file\n",
			atomic.LoadInt32(&recordCount), float64(fileSize)/(1024*1024))
	}

	return nil
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

	// Get file size for progress reporting
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// Create a buffered reader for better performance
	bufferedReader := bufio.NewReaderSize(file, 1024*1024) // 1MB buffer

	// Set up CSV reader
	reader := csv.NewReader(bufferedReader)

	// For large files, consider memory efficient processing
	showProgress := isProgressReportingEnabled() && fileSize > 10*1024*1024 // Show progress for files > 10MB

	if p.options.StartTime == nil {
		return nil, fmt.Errorf("start time must be provided")
	}

	// Use the provided start time since it's required
	startTime := *p.options.StartTime

	// Set up progress reporting
	var processedBytes int64
	var recordCount int32
	var lastReportTime time.Time
	var records []EnemeterRecord

	if showProgress {
		lastReportTime = time.Now()
		fmt.Printf("Parsing file: %.2f MB\n", float64(fileSize)/(1024*1024))

		// Start progress monitoring in a goroutine
		progressDone := make(chan struct{})
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					currentBytes := atomic.LoadInt64(&processedBytes)
					currentRecords := atomic.LoadInt32(&recordCount)
					progress := float64(currentBytes) / float64(fileSize) * 100

					// Calculate processing speed
					elapsed := time.Since(lastReportTime).Seconds()
					recordsPerSec := float64(currentRecords) / elapsed

					fmt.Printf("\rParsing: %.1f%% complete | %d records | %.1f records/sec",
						progress, currentRecords, recordsPerSec)
				case <-progressDone:
					return
				}
			}
		}()

		// Make sure to close the progress channel when done
		defer func() {
			close(progressDone)
			fmt.Println() // Line break after progress reporting
		}()
	}

	// Prepare processing
	accumulatedTimeMs := int64(0)
	sampleCounter := 0

	const (
		timeCol    = 0
		voltageCol = 1
		currentCol = 2
		tempCol    = 3
	)

	// Process CSV file line by line
	lineNum := 0
	for p.options.MaxRecords <= 0 || len(records) < p.options.MaxRecords {
		// Read the next line
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error reading CSV row %d: %w", lineNum+1, err)
		}
		lineNum++

		// Update processed bytes (approximate)
		if showProgress {
			// Estimate bytes processed (row length + newline)
			rowSize := 0
			for _, field := range row {
				rowSize += len(field) + 1 // +1 for delimiter
			}
			atomic.AddInt64(&processedBytes, int64(rowSize))
		}

		// Apply sampling based on sample rate
		sampleCounter++
		if sampleCounter < p.options.SampleRate {
			continue
		}
		sampleCounter = 0

		// Validate row format
		if len(row) != 4 {
			return nil, fmt.Errorf("invalid row format at line %d, expected 4 fields but got %d", lineNum, len(row))
		}

		// Parse fields
		timeDelta, err := strconv.ParseInt(row[timeCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse time delta at line %d: %w", lineNum, err)
		}

		voltageMicroV, err := strconv.ParseInt(row[voltageCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse voltage at line %d: %w", lineNum, err)
		}

		currentNanoA, err := strconv.ParseInt(row[currentCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse current at line %d: %w", lineNum, err)
		}

		tempMiliCelsius, err := strconv.ParseInt(row[tempCol], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse temperature at line %d: %w", lineNum, err)
		}

		// Calculate timestamp
		accumulatedTimeMs += timeDelta
		timestamp := startTime.Add(time.Duration(accumulatedTimeMs) * time.Millisecond)

		// Apply time filtering
		if p.options.StartTime != nil && timestamp.Before(*p.options.StartTime) {
			continue
		}
		if p.options.EndTime != nil && timestamp.After(*p.options.EndTime) {
			continue // Change from break to continue to process all records
		}

		// Apply data filtering
		if p.options.TempThreshold != nil && tempMiliCelsius < *p.options.TempThreshold {
			continue
		}

		if p.options.VoltageRange != nil && (voltageMicroV < p.options.VoltageRange[0] || voltageMicroV > p.options.VoltageRange[1]) {
			continue
		}

		if p.options.CurrentRange != nil && (currentNanoA < p.options.CurrentRange[0] || currentNanoA > p.options.CurrentRange[1]) {
			continue
		}

		// Create and add record
		record := EnemeterRecord{
			TimeDeltaMs:     timeDelta,
			TempMiliCelsius: tempMiliCelsius,
			VoltageMicroV:   voltageMicroV,
			CurrentNanoA:    currentNanoA,
			Timestamp:       timestamp,
		}

		records = append(records, record)

		// Update record count for progress tracking
		if showProgress {
			atomic.AddInt32(&recordCount, 1)
		}
	}

	// After processing all records, ensure the time range boundaries are preserved
	if p.options.PreserveTimeRangeBoundaries != nil && len(p.options.PreserveTimeRangeBoundaries) >= 2 {
		firstBoundary := p.options.PreserveTimeRangeBoundaries[0]
		lastBoundary := p.options.PreserveTimeRangeBoundaries[len(p.options.PreserveTimeRangeBoundaries)-1]

		// Check if boundaries are already included
		hasFirst := false
		hasLast := false

		// Use a reasonable time window for matching (1 second)
		const timeWindow = 1 * time.Second

		for _, r := range records {
			if r.Timestamp.Sub(firstBoundary.Timestamp).Abs() < timeWindow {
				hasFirst = true
			}
			if r.Timestamp.Sub(lastBoundary.Timestamp).Abs() < timeWindow {
				hasLast = true
			}
		}

		// Add missing boundaries
		if !hasFirst {
			fmt.Println("Adding first boundary record to preserve time range")
			records = append([]EnemeterRecord{firstBoundary}, records...)
		}

		if !hasLast {
			fmt.Println("Adding last boundary record to preserve time range")
			records = append(records, lastBoundary)
		}

		// Log the final time range
		if len(records) >= 2 {
			fmt.Printf("Time range after boundary preservation: %s to %s (%.2f hours)\n",
				records[0].Timestamp.Format("2006-01-02 15:04:05"),
				records[len(records)-1].Timestamp.Format("2006-01-02 15:04:05"),
				records[len(records)-1].Timestamp.Sub(records[0].Timestamp).Hours())
		}
	}

	// Final progress update
	if showProgress {
		fmt.Printf("\rParsed %d records from %.2f MB file\n",
			len(records), float64(fileSize)/(1024*1024))
	}

	return records, nil
}

// The rest of the file remains unchanged
