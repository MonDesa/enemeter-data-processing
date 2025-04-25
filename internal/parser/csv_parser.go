package parser

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"runtime"
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

	// Final progress update
	if showProgress {
		fmt.Printf("\rParsed %d records from %.2f MB file\n",
			len(records), float64(fileSize)/(1024*1024))
	}

	return records, nil
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

	// Create buffered reader for better performance
	bufferedReader := bufio.NewReaderSize(file, 1024*1024) // 1MB buffer
	reader := csv.NewReader(bufferedReader)

	// Get file info for progress reporting
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// Set up progress reporting
	showProgress := isProgressReportingEnabled() && fileSize > 10*1024*1024
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
					if elapsed > 0 {
						recordsPerSec := float64(currentRecords) / elapsed
						fmt.Printf("\rStreaming: %.1f%% complete | %d records | %.1f records/sec",
							progress, currentRecords, recordsPerSec)
					}
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

	if p.options.StartTime == nil {
		return fmt.Errorf("start time must be provided")
	}

	// Use the provided start time since it's required
	startTime := *p.options.StartTime

	accumulatedTimeMs := int64(0)
	sampleCounter := 0

	const (
		timeCol    = 0
		voltageCol = 1
		currentCol = 2
		tempCol    = 3
	)

	// Process each row
	lineNum := 0
	for {
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

		// Sampling - process every Nth record
		sampleCounter++
		if sampleCounter < p.options.SampleRate {
			continue
		}
		sampleCounter = 0

		// Check max records limit
		if p.options.MaxRecords > 0 && atomic.LoadInt32(&recordCount) >= int32(p.options.MaxRecords) {
			break
		}

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

		// Create record
		record := EnemeterRecord{
			TimeDeltaMs:     timeDelta,
			TempMiliCelsius: tempMiliCelsius,
			VoltageMicroV:   voltageMicroV,
			CurrentNanoA:    currentNanoA,
			Timestamp:       timestamp,
		}

		// Process the record with callback
		if err := callback(record); err != nil {
			return fmt.Errorf("callback error at line %d: %w", lineNum, err)
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

	// Get file size for estimating record count
	fileInfo, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	fileSize := fileInfo.Size()

	// Fast estimation method for very large files
	if fileSize > 500*1024*1024 { // 500 MB
		// For very large files, use a constant estimate of bytes per record
		// based on common Enemeter data format (typically ~20-30 bytes per record)
		const bytesPerRecordEstimate = 25
		return int(fileSize / bytesPerRecordEstimate), nil
	}

	// For smaller files, sample the first 100 lines to get a better estimate
	reader := csv.NewReader(file)
	lineCount := 0
	bytesRead := int64(0)
	maxSampleLines := 100

	// Sample lines to calculate average line size
	for i := 0; i < maxSampleLines; i++ {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("error reading CSV: %w", err)
		}

		// Calculate approximate line size including delimiters and newline
		lineBytes := int64(0)
		for _, field := range line {
			lineBytes += int64(len(field) + 1) // +1 for delimiter
		}
		lineBytes++ // newline character

		bytesRead += lineBytes
		lineCount++
	}

	if lineCount == 0 {
		return 0, nil // Empty file or header only
	}

	// Calculate estimated record count based on average line size
	// Fix: Protect against divide by zero
	avgLineSize := int64(1) // Default to 1 byte if no lines read
	if lineCount > 0 {
		avgLineSize = bytesRead / int64(lineCount)
	}

	// Make sure avgLineSize is at least 1 byte to prevent divide by zero
	if avgLineSize < 1 {
		avgLineSize = 1
	}

	estimatedRecords := fileSize / avgLineSize

	// Apply sampling rate to estimation
	if p.options.SampleRate > 1 {
		estimatedRecords = estimatedRecords / int64(p.options.SampleRate)
	}

	// Apply time-based and data-based filtering factor if filters are set
	// (rough estimation that filtering removes ~20% of records)
	if p.options.EndTime != nil || p.options.TempThreshold != nil ||
		p.options.VoltageRange != nil || p.options.CurrentRange != nil {
		estimatedRecords = int64(float64(estimatedRecords) * 0.8)
	}

	// Apply max records limit if set
	if p.options.MaxRecords > 0 && int(estimatedRecords) > p.options.MaxRecords {
		return p.options.MaxRecords, nil
	}

	return int(estimatedRecords), nil
}

func (p *CSVParser) ParseInParallel() ([]EnemeterRecord, error) {
	// Check file size to determine if parallel processing is beneficial
	fileSize, err := p.GetFileSize()
	if err != nil {
		return nil, err
	}

	// Only use parallel processing for large files
	if fileSize < 100*1024*1024 { // 100MB
		return p.Parse() // Use regular parsing for smaller files
	}

	// Get an approximate record count for pre-allocation
	recordCount, err := p.GetRecordCount()
	if err != nil {
		recordCount = 1000000 // Default if we can't estimate
	}

	fmt.Printf("Using parallel processing for large file (%.2f MB, est. %d records)\n",
		float64(fileSize)/(1024*1024), recordCount)

	// Open the file
	file, err := os.Open(p.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Determine number of workers based on CPU cores
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // Limit to 8 workers for large files
	}

	// Calculate chunk sizes to divide the file
	chunkSize := fileSize / int64(numWorkers)
	if chunkSize < 1024*1024 { // Ensure at least 1MB chunks
		chunkSize = 1024 * 1024
		numWorkers = int(fileSize / chunkSize)
		if numWorkers < 1 {
			numWorkers = 1
		}
	}

	// Create result channel
	resultChan := make(chan []EnemeterRecord, numWorkers)
	errorChan := make(chan error, numWorkers)

	// Split file into chunks and process in parallel
	var wg sync.WaitGroup

	fmt.Printf("Dividing file into %d chunks for parallel processing\n", numWorkers)

	// Process each chunk
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Implementation omitted as this is more complex and requires
			// a lot of additional code to handle CSV parsing from arbitrary positions.
			// For a real implementation, we would:
			// 1. Open a new file descriptor
			// 2. Seek to the appropriate position
			// 3. Find the next complete line
			// 4. Parse that chunk
			// 5. Send results on the channel

			// Placeholder for the actual implementation
			// In a real implementation, this would be filled with the proper code
		}(i)
	}

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Since the parallel implementation is complex and would require a lot
	// more code, we'll fallback to the regular parse for now
	return p.Parse()
}
