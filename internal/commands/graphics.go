package commands

import (
	"bufio"
	"enemeter-data-processing/internal/metrics"
	"enemeter-data-processing/internal/parser"
	"enemeter-data-processing/internal/viz"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// GraphicsOptions holds the command line options for the graphics command
type GraphicsOptions struct {
	// Input/output options
	InputFile   string
	OutputDir   string
	GraphicType viz.GraphType // Changed from slice to single type

	// Processing options
	UseStreaming bool
	SampleRate   int
	MaxRecords   int

	// Time filtering options
	StartTime  string
	EndTime    string
	TimeWindow string

	// Data filtering options
	MinTemp    int64
	VoltageMin int64
	VoltageMax int64
	CurrentMin int64
	CurrentMax int64

	// Scale options
	TimeScale     string // X axis scale (milliseconds, seconds, minutes, hours, days)
	VoltageScale  string // Y axis scale for voltage
	CurrentScale  string // Y axis scale for current
	TempScale     string // Y axis scale for temperature
	PowerScale    string // Y axis scale for power
	EnergyScale   string // Y axis scale for energy
	ForceGenerate bool   // Force generation even for large datasets with small scales

	// Performance options
	MaxDataPoints int    // Maximum number of data points to plot
	SamplingMode  string // Downsampling algorithm to use
}

// SetupGraphicsCommand configures the graphics command with all its flags
func SetupGraphicsCommand() *flag.FlagSet {
	graphicsCmd := flag.NewFlagSet("graphics", flag.ExitOnError)

	// Input/output options
	graphicsCmd.String("input", "", "Path to the input CSV file")
	graphicsCmd.String("output-dir", "charts", "Directory to save generated charts")
	graphicsCmd.String("type", "", "Type of graph to generate (power, energy, voltage, current, temperature) - REQUIRED")

	// Processing options
	graphicsCmd.Bool("stream", false, "Use streaming mode for processing large files")
	graphicsCmd.Int("sample", 1, "Process every Nth record (1 = all records)")
	graphicsCmd.Int("max", 0, "Maximum records to process (0 = no limit)")

	// Make start time required and clarify that it must include time of day
	graphicsCmd.String("start", "", "Start time for measurements (format: YYYY-MM-DD HH:MM:SS) - REQUIRED")
	graphicsCmd.String("end", "", "End time for filtering (format: YYYY-MM-DD[THH:MM:SS])")
	graphicsCmd.String("window", "", "Time window to process (e.g., 1h, 30m, 24h)")

	// Data filtering options
	graphicsCmd.Int64("min-temp", 0, "Minimum temperature threshold in millicelsius")
	graphicsCmd.Int64("volt-min", 0, "Minimum voltage threshold in microvolts")
	graphicsCmd.Int64("volt-max", 0, "Maximum voltage threshold in microvolts")
	graphicsCmd.Int64("curr-min", 0, "Minimum current threshold in nanoamperes")
	graphicsCmd.Int64("curr-max", 0, "Maximum current threshold in nanoamperes")

	// Scale options
	graphicsCmd.String("time-scale", "seconds", "Time (X-axis) scale: milliseconds, seconds, minutes, hours, days")
	graphicsCmd.String("voltage-scale", "volts", "Voltage scale: microvolts, millivolts, volts")
	graphicsCmd.String("current-scale", "milliamperes", "Current scale: nanoamperes, microamperes, milliamperes, amperes")
	graphicsCmd.String("temp-scale", "celsius", "Temperature scale: millicelsius, celsius")
	graphicsCmd.String("power-scale", "watts", "Power scale: microwatts, milliwatts, watts, kilowatts")
	graphicsCmd.String("energy-scale", "joules", "Energy scale: microjoules, millijoules, joules, kilojoules, watt-hours, kilowatt-hours")
	graphicsCmd.Bool("force", false, "Force chart generation even for large datasets with small scales")

	// Performance options
	graphicsCmd.Int("max-points", 2000, "Maximum number of data points to plot (0 for unlimited)")
	graphicsCmd.String("sampling", "adaptive", "Downsampling algorithm: even, adaptive, lttb")

	// Help function for the graphics command
	graphicsCmd.Usage = func() {
		fmt.Println(AppName + " - Generate charts from ENEMETER data")
		fmt.Println("\nUsage:")
		fmt.Println("  enemeter-data-processing graphics [options]")
		fmt.Println("\nRequired Parameters:")
		fmt.Println("  --input=<file>     Input CSV file")
		fmt.Println("  --start=<time>     Start time in format YYYY-MM-DD HH:MM:SS (must include time of day)")
		fmt.Println("  --type=<type>      Type of graph to generate (power, energy, voltage, current, temperature)")
		fmt.Println("\nExamples:")
		fmt.Println("  Generate power chart:")
		fmt.Println("  enemeter-data-processing graphics --input=data/data.csv --start=\"2023-04-01 12:00:00\" --type=power")
		fmt.Println("\n  Generate voltage chart with specific scales:")
		fmt.Println("  enemeter-data-processing graphics --input=data.csv --start=\"2023-04-01 08:30:00\" --type=voltage --voltage-scale=millivolts --time-scale=minutes")
		fmt.Println("\n  Specify output directory:")
		fmt.Println("  enemeter-data-processing graphics --input=data.csv --start=\"2023-04-01\" --type=current --output-dir=./my-charts")
		fmt.Println("\nOptions:")
		graphicsCmd.PrintDefaults()
	}

	return graphicsCmd
}

// ParseGraphicsOptions parses command line flags into a structured graphics options object
func ParseGraphicsOptions(cmd *flag.FlagSet) GraphicsOptions {
	inputFile := cmd.Lookup("input").Value.String()
	outputDir := cmd.Lookup("output-dir").Value.String()
	typeStr := cmd.Lookup("type").Value.String()

	// Parse single graph type
	var graphicType viz.GraphType
	typeStr = strings.TrimSpace(typeStr)
	if typeStr != "" {
		graphicType = viz.GraphType(typeStr)
	}

	// Processing options - properly parse flags
	streamValue := cmd.Lookup("stream").Value.String()
	useStreaming := streamValue == "true"

	sampleRateStr := cmd.Lookup("sample").Value.String()
	sampleRate, _ := strconv.Atoi(sampleRateStr)
	if sampleRate < 1 {
		sampleRate = 1
	}

	maxRecordsStr := cmd.Lookup("max").Value.String()
	maxRecords, _ := strconv.Atoi(maxRecordsStr)

	// Time filtering options
	startTime := cmd.Lookup("start").Value.String()
	endTime := cmd.Lookup("end").Value.String()
	timeWindow := cmd.Lookup("window").Value.String()

	// Data filtering options
	minTempStr := cmd.Lookup("min-temp").Value.String()
	minTemp, _ := strconv.ParseInt(minTempStr, 10, 64)

	voltageMinStr := cmd.Lookup("volt-min").Value.String()
	voltageMin, _ := strconv.ParseInt(voltageMinStr, 10, 64)

	voltageMaxStr := cmd.Lookup("volt-max").Value.String()
	voltageMax, _ := strconv.ParseInt(voltageMaxStr, 10, 64)

	currentMinStr := cmd.Lookup("curr-min").Value.String()
	currentMin, _ := strconv.ParseInt(currentMinStr, 10, 64)

	currentMaxStr := cmd.Lookup("curr-max").Value.String()
	currentMax, _ := strconv.ParseInt(currentMaxStr, 10, 64)

	// Parse scale options
	timeScale := cmd.Lookup("time-scale").Value.String()
	voltageScale := cmd.Lookup("voltage-scale").Value.String()
	currentScale := cmd.Lookup("current-scale").Value.String()
	tempScale := cmd.Lookup("temp-scale").Value.String()
	powerScale := cmd.Lookup("power-scale").Value.String()
	energyScale := cmd.Lookup("energy-scale").Value.String()

	forceValue := cmd.Lookup("force").Value.String()
	forceGenerate := forceValue == "true"

	// Parse performance options
	maxPointsStr := cmd.Lookup("max-points").Value.String()
	maxPoints, _ := strconv.Atoi(maxPointsStr)
	samplingMode := cmd.Lookup("sampling").Value.String()

	return GraphicsOptions{
		InputFile:     inputFile,
		OutputDir:     outputDir,
		GraphicType:   graphicType, // Changed from slice to single type
		UseStreaming:  useStreaming,
		SampleRate:    sampleRate,
		MaxRecords:    maxRecords,
		StartTime:     startTime,
		EndTime:       endTime,
		TimeWindow:    timeWindow,
		MinTemp:       minTemp,
		VoltageMin:    voltageMin,
		VoltageMax:    voltageMax,
		CurrentMin:    currentMin,
		CurrentMax:    currentMax,
		TimeScale:     timeScale,
		VoltageScale:  voltageScale,
		CurrentScale:  currentScale,
		TempScale:     tempScale,
		PowerScale:    powerScale,
		EnergyScale:   energyScale,
		ForceGenerate: forceGenerate,
		MaxDataPoints: maxPoints,
		SamplingMode:  samplingMode,
	}
}

// GraphicsCommand executes the graphics generation functionality
func GraphicsCommand(options GraphicsOptions) error {
	// Validate input file
	if options.InputFile == "" {
		return fmt.Errorf("input file is required (--input)")
	}

	// Validate start time (now required)
	if options.StartTime == "" {
		return fmt.Errorf("start time is required (--start)")
	}

	// Interactive mode: ask the user to select a graph type if not provided
	if options.GraphicType == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("\nSelect a chart type to generate:")
		fmt.Println("1. Power")
		fmt.Println("2. Energy")
		fmt.Println("3. Voltage")
		fmt.Println("4. Current")
		fmt.Println("5. Temperature")
		fmt.Print("\nEnter your choice (1-5): ")

		choice, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error reading input: %v", err)
		}

		choice = strings.TrimSpace(choice)
		switch choice {
		case "1":
			options.GraphicType = viz.GraphTypePower
		case "2":
			options.GraphicType = viz.GraphTypeEnergy
		case "3":
			options.GraphicType = viz.GraphTypeVoltage
		case "4":
			options.GraphicType = viz.GraphTypeCurrent
		case "5":
			options.GraphicType = viz.GraphTypeTemperature
		default:
			return fmt.Errorf("invalid choice: %s", choice)
		}

		fmt.Printf("\nSelected chart type: %s\n", options.GraphicType)
	}

	// Interactive mode: ask the user to select X-axis scale
	if options.TimeScale == "seconds" { // Default value
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("\nSelect X-axis time scale:")
		fmt.Println("1. Milliseconds")
		fmt.Println("2. Seconds (default)")
		fmt.Println("3. Minutes")
		fmt.Println("4. Hours")
		fmt.Println("5. Days")
		fmt.Print("\nEnter your choice (1-5): ")

		choice, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error reading input: %v", err)
		}

		choice = strings.TrimSpace(choice)
		switch choice {
		case "1":
			options.TimeScale = "milliseconds"
		case "2", "": // Default or empty selection
			options.TimeScale = "seconds"
		case "3":
			options.TimeScale = "minutes"
		case "4":
			options.TimeScale = "hours"
		case "5":
			options.TimeScale = "days"
		default:
			return fmt.Errorf("invalid choice: %s", choice)
		}

		fmt.Printf("Selected X-axis time scale: %s\n", options.TimeScale)
	}

	// Interactive mode: ask the user to select Y-axis scale based on chart type
	reader := bufio.NewReader(os.Stdin)

	switch options.GraphicType {
	case viz.GraphTypePower:
		if options.PowerScale == "watts" { // Default value
			fmt.Println("\nSelect Y-axis power scale:")
			fmt.Println("1. Microwatts")
			fmt.Println("2. Milliwatts")
			fmt.Println("3. Watts (default)")
			fmt.Println("4. Kilowatts")
			fmt.Print("\nEnter your choice (1-4): ")

			choice, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("error reading input: %v", err)
			}

			choice = strings.TrimSpace(choice)
			switch choice {
			case "1":
				options.PowerScale = "microwatts"
			case "2":
				options.PowerScale = "milliwatts"
			case "3", "": // Default or empty selection
				options.PowerScale = "watts"
			case "4":
				options.PowerScale = "kilowatts"
			default:
				return fmt.Errorf("invalid choice: %s", choice)
			}

			fmt.Printf("Selected Y-axis power scale: %s\n", options.PowerScale)
		}

	case viz.GraphTypeEnergy:
		if options.EnergyScale == "joules" { // Default value
			fmt.Println("\nSelect Y-axis energy scale:")
			fmt.Println("1. Microjoules")
			fmt.Println("2. Millijoules")
			fmt.Println("3. Joules (default)")
			fmt.Println("4. Kilojoules")
			fmt.Println("5. Watt-hours")
			fmt.Println("6. Kilowatt-hours")
			fmt.Print("\nEnter your choice (1-6): ")

			choice, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("error reading input: %v", err)
			}

			choice = strings.TrimSpace(choice)
			switch choice {
			case "1":
				options.EnergyScale = "microjoules"
			case "2":
				options.EnergyScale = "millijoules"
			case "3", "": // Default or empty selection
				options.EnergyScale = "joules"
			case "4":
				options.EnergyScale = "kilojoules"
			case "5":
				options.EnergyScale = "watt-hours"
			case "6":
				options.EnergyScale = "kilowatt-hours"
			default:
				return fmt.Errorf("invalid choice: %s", choice)
			}

			fmt.Printf("Selected Y-axis energy scale: %s\n", options.EnergyScale)
		}

	case viz.GraphTypeVoltage:
		if options.VoltageScale == "volts" { // Default value
			fmt.Println("\nSelect Y-axis voltage scale:")
			fmt.Println("1. Microvolts")
			fmt.Println("2. Millivolts")
			fmt.Println("3. Volts (default)")
			fmt.Print("\nEnter your choice (1-3): ")

			choice, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("error reading input: %v", err)
			}

			choice = strings.TrimSpace(choice)
			switch choice {
			case "1":
				options.VoltageScale = "microvolts"
			case "2":
				options.VoltageScale = "millivolts"
			case "3", "": // Default or empty selection
				options.VoltageScale = "volts"
			default:
				return fmt.Errorf("invalid choice: %s", choice)
			}

			fmt.Printf("Selected Y-axis voltage scale: %s\n", options.VoltageScale)
		}

	case viz.GraphTypeCurrent:
		if options.CurrentScale == "milliamperes" { // Default value
			fmt.Println("\nSelect Y-axis current scale:")
			fmt.Println("1. Nanoamperes")
			fmt.Println("2. Microamperes")
			fmt.Println("3. Milliamperes (default)")
			fmt.Println("4. Amperes")
			fmt.Print("\nEnter your choice (1-4): ")

			choice, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("error reading input: %v", err)
			}

			choice = strings.TrimSpace(choice)
			switch choice {
			case "1":
				options.CurrentScale = "nanoamperes"
			case "2":
				options.CurrentScale = "microamperes"
			case "3", "": // Default or empty selection
				options.CurrentScale = "milliamperes"
			case "4":
				options.CurrentScale = "amperes"
			default:
				return fmt.Errorf("invalid choice: %s", choice)
			}

			fmt.Printf("Selected Y-axis current scale: %s\n", options.CurrentScale)
		}

	case viz.GraphTypeTemperature:
		if options.TempScale == "celsius" { // Default value
			fmt.Println("\nSelect Y-axis temperature scale:")
			fmt.Println("1. Millicelsius")
			fmt.Println("2. Celsius (default)")
			fmt.Print("\nEnter your choice (1-2): ")

			choice, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("error reading input: %v", err)
			}

			choice = strings.TrimSpace(choice)
			switch choice {
			case "1":
				options.TempScale = "millicelsius"
			case "2", "": // Default or empty selection
				options.TempScale = "celsius"
			default:
				return fmt.Errorf("invalid choice: %s", choice)
			}

			fmt.Printf("Selected Y-axis temperature scale: %s\n", options.TempScale)
		}
	}

	// Ensure input file exists
	if _, err := os.Stat(options.InputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", options.InputFile)
	}

	// Create output directory if it doesn't exist
	if _, err := os.Stat(options.OutputDir); os.IsNotExist(err) {
		if err := os.MkdirAll(options.OutputDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	// Create CSV parser with appropriate options
	csvParser := parser.NewCSVParser(options.InputFile)
	filterOptions, err := buildFilterOptions(CommandLineOptions{
		StartTime:  options.StartTime,
		EndTime:    options.EndTime,
		TimeWindow: options.TimeWindow,
		MinTemp:    options.MinTemp,
		VoltageMin: options.VoltageMin,
		VoltageMax: options.VoltageMax,
		CurrentMin: options.CurrentMin,
		CurrentMax: options.CurrentMax,
	})
	if err != nil {
		return fmt.Errorf("error configuring filters: %v", err)
	}
	csvParser.WithFilterOptions(filterOptions)

	// Check file size to determine if we should use streaming
	fileSize, err := csvParser.GetFileSize()
	if err != nil {
		return fmt.Errorf("error getting file size: %v", err)
	}

	// Suggest streaming mode for large files (>100MB) if not explicitly set
	if fileSize > 100*1024*1024 && !options.UseStreaming {
		fmt.Printf("Note: Processing a large file (%.2f MB). Consider using --stream for better performance.\n",
			float64(fileSize)/(1024*1024))
	}

	// Get an estimate of the number of records
	recordCount, err := csvParser.GetRecordCount()
	if err != nil {
		log.Printf("Warning: Couldn't estimate record count: %v", err)
	} else {
		fmt.Printf("Estimated records in file: %d\n", recordCount)
	}

	// Process the data
	fmt.Printf("Processing data from %s for chart generation...\n", options.InputFile)

	var energyMetrics metrics.EnergyMetrics
	var records []parser.EnemeterRecord

	// Configure metrics options
	metricsOptions := metrics.MetricsOptions{
		TimeResolution: time.Minute * 5, // Default 5-minute resolution
	}

	// Process data either with streaming or regular mode
	if options.UseStreaming {
		fmt.Println("Using streaming mode for memory-efficient processing...")
		// Use our wrapper function that handles different parameter types
		energyMetrics, err = metrics.StreamCalculateMetricsWrapper(csvParser, metricsOptions)
		if err != nil {
			return fmt.Errorf("failed to process CSV data in streaming mode: %v", err)
		}

		// We need the records for visualization, so parse them separately
		records, err = csvParser.Parse()
		if err != nil {
			return fmt.Errorf("failed to parse CSV data: %v", err)
		}
	} else {
		// Parse all records at once
		records, err = csvParser.Parse()
		if err != nil {
			return fmt.Errorf("failed to parse CSV data: %v", err)
		}
		fmt.Printf("Successfully parsed %d records\n", len(records))

		// Calculate metrics
		calculator := metrics.NewEnergyCalculator(records).WithOptions(metricsOptions)
		energyMetrics = calculator.CalculateMetrics()
	}

	// Check if we're generating charts with small scales on large datasets
	const largeDatasetThreshold = 10000 // Consider 10K+ records as large
	numRecords := len(records)

	smallScaleWarning := false
	if numRecords > largeDatasetThreshold {
		// Check if using small scales based on the selected graph type
		switch options.GraphicType {
		case viz.GraphTypePower:
			if options.TimeScale == "milliseconds" || options.PowerScale == "microwatts" {
				smallScaleWarning = true
			}
		case viz.GraphTypeEnergy:
			if options.TimeScale == "milliseconds" || options.EnergyScale == "microjoules" {
				smallScaleWarning = true
			}
		case viz.GraphTypeVoltage:
			if options.TimeScale == "milliseconds" || options.VoltageScale == "microvolts" {
				smallScaleWarning = true
			}
		case viz.GraphTypeCurrent:
			if options.TimeScale == "milliseconds" || options.CurrentScale == "nanoamperes" {
				smallScaleWarning = true
			}
		case viz.GraphTypeTemperature:
			if options.TimeScale == "milliseconds" || options.TempScale == "millicelsius" {
				smallScaleWarning = true
			}
		}
	}

	if smallScaleWarning && !options.ForceGenerate {
		fmt.Printf("\nWARNING: Generating a %s chart with %d records using small scales (%s, %s) may produce very large files.\n",
			options.GraphicType, numRecords, options.TimeScale, getYAxisScale(options))
		fmt.Println("Do you want to continue? (y/n): ")

		choice, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error reading input: %v", err)
		}

		choice = strings.TrimSpace(strings.ToLower(choice))
		if choice != "y" && choice != "yes" {
			return fmt.Errorf("chart generation aborted by user")
		}

		fmt.Println("Proceeding with chart generation...")
	} else if smallScaleWarning {
		fmt.Printf("Warning: Generating large %s chart (%d records) with small scales (%s, %s). This may be slow.\n",
			options.GraphicType, numRecords, options.TimeScale, getYAxisScale(options))
	}

	// Create graphics generator with scale options
	graphicsGen := viz.NewGraphicsGenerator(records, options.OutputDir)
	graphicsGen.WithScaleOptions(viz.ScaleOptions{
		TimeScale:    options.TimeScale,
		VoltageScale: options.VoltageScale,
		CurrentScale: options.CurrentScale,
		TempScale:    options.TempScale,
		PowerScale:   options.PowerScale,
		EnergyScale:  options.EnergyScale,
	})

	// Apply performance options
	if options.MaxDataPoints > 0 {
		graphicsGen.WithMaxDataPoints(options.MaxDataPoints)
	}
	if options.SamplingMode != "" {
		graphicsGen.WithSamplingMode(options.SamplingMode)
	}

	// Add a timeout mechanism for chart generation
	fmt.Printf("Generating %s chart (this may take a while for large datasets)...\n", options.GraphicType)

	// Create a channel to communicate chart generation results
	resultChan := make(chan struct {
		path string
		err  error
	})

	// Create a channel for progress updates
	progressChan := make(chan struct{})

	// Run progress indicator in background goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		dots := 0
		for {
			select {
			case <-ticker.C:
				fmt.Printf("\rProcessing%s", strings.Repeat(".", dots+1))
				dots = (dots + 1) % 3
			case <-progressChan:
				fmt.Println()
				return
			}
		}
	}()

	// Run chart generation in a goroutine
	go func() {
		chartPath, err := graphicsGen.GenerateChart(options.GraphicType, energyMetrics)
		close(progressChan) // Stop progress indicator
		resultChan <- struct {
			path string
			err  error
		}{chartPath, err}
	}()

	// Set a reasonable timeout for chart generation (3 minutes for normal files, 15 minutes for large files)
	timeout := 3 * time.Minute
	if numRecords > 100000 {
		timeout = 15 * time.Minute // extend timeout for very large files
	}
	if numRecords > 1000000 {
		timeout = 30 * time.Minute // even longer timeout for massive files
	}

	// Wait for either completion or timeout
	select {
	case result := <-resultChan:
		if result.err != nil {
			return fmt.Errorf("failed to generate %s chart: %v", options.GraphicType, result.err)
		}
		fmt.Printf("\nGenerated %s chart: %s\n", options.GraphicType, result.path)
		fmt.Printf("Chart has been saved to directory: %s\n", options.OutputDir)
		return nil

	case <-time.After(timeout):
		close(progressChan) // Stop progress indicator
		return fmt.Errorf("\nChart generation timed out after %v - the dataset may be too large for detailed visualization.\n"+
			"Try reducing the dataset with --sample=N or increasing --max-points", timeout)
	}
}

// Helper function to get the Y-axis scale based on the graph type
func getYAxisScale(options GraphicsOptions) string {
	switch options.GraphicType {
	case viz.GraphTypePower:
		return options.PowerScale
	case viz.GraphTypeEnergy:
		return options.EnergyScale
	case viz.GraphTypeVoltage:
		return options.VoltageScale
	case viz.GraphTypeCurrent:
		return options.CurrentScale
	case viz.GraphTypeTemperature:
		return options.TempScale
	default:
		return "unknown"
	}
}
