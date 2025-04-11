package commands

import (
	"encoding/json"
	"enemeter-data-processing/internal/metrics"
	"enemeter-data-processing/internal/parser"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OutputFormat determines how the CLI results should be displayed
type OutputFormat string

const (
	FormatText OutputFormat = "text"
	FormatJSON OutputFormat = "json"
	FormatCSV  OutputFormat = "csv"
)

// CommandLineOptions holds all CLI options
type CommandLineOptions struct {
	// Input/output options
	InputFile  string
	OutputFile string
	Format     OutputFormat

	// Processing options
	UseStreaming bool
	SampleRate   int
	MaxRecords   int

	// Time filtering options
	StartTime  string
	EndTime    string
	TimeWindow string // e.g. "1h", "30m", "24h"

	// Data filtering options
	MinTemp    int64
	VoltageMin int64
	VoltageMax int64
	CurrentMin int64
	CurrentMax int64

	// Specific metrics to extract
	Metric string
}

// SetupProcessCommand configures the process command with all its flags
func SetupProcessCommand() *flag.FlagSet {
	processCmd := flag.NewFlagSet("process", flag.ExitOnError)

	// Input/output options
	processCmd.String("input", "", "Path to the input CSV file")
	processCmd.String("output", "", "Path to save the output report (optional)")
	processCmd.String("format", "text", "Output format: text, json, or csv")

	// Processing options
	processCmd.Bool("stream", false, "Use streaming mode for processing large files")
	processCmd.Int("sample", 1, "Process every Nth record (1 = all records)")
	processCmd.Int("max", 0, "Maximum records to process (0 = no limit)")

	// Time filtering options
	processCmd.String("start", "", "Start time for filtering (format: YYYY-MM-DD[THH:MM:SS])")
	processCmd.String("end", "", "End time for filtering (format: YYYY-MM-DD[THH:MM:SS])")
	processCmd.String("window", "", "Time window to process (e.g., 1h, 30m, 24h)")

	// Data filtering options
	processCmd.Int64("min-temp", 0, "Minimum temperature threshold in millicelsius")
	processCmd.Int64("volt-min", 0, "Minimum voltage threshold in microvolts")
	processCmd.Int64("volt-max", 0, "Maximum voltage threshold in microvolts")
	processCmd.Int64("curr-min", 0, "Minimum current threshold in nanoamperes")
	processCmd.Int64("curr-max", 0, "Maximum current threshold in nanoamperes")

	// Specific metrics extraction
	processCmd.String("metric", "",
		"Extract specific metric: total_energy, average_power, peak_power, temperature, "+
			"energy_by_hour, voltage_stats, current_stats, battery_discharge, solar_contribution")

	// Help function for the process command
	processCmd.Usage = func() {
		fmt.Println(AppName + " - Process and analyze ENEMETER data")
		fmt.Println("\nUsage:")
		fmt.Println("  enemeter-cli process [options]")
		fmt.Println("\nExamples:")
		fmt.Println("  # Basic usage")
		fmt.Println("  enemeter-cli process --input=data/data.csv")
		fmt.Println("\n  # Extract specific metrics")
		fmt.Println("  enemeter-cli process --input=data.csv --metric=average_power")
		fmt.Println("\n  # Filter by time window")
		fmt.Println("  enemeter-cli process --input=data.csv --start=2025-04-01 --end=2025-04-02")
		fmt.Println("\n  # Process large file efficiently")
		fmt.Println("  enemeter-cli process --input=big-data.csv --stream --sample=10")
		fmt.Println("\n  # Get temperature statistics in JSON format")
		fmt.Println("  enemeter-cli process --input=data.csv --metric=temperature --format=json")
		fmt.Println("\n  # Save output to a file")
		fmt.Println("  enemeter-cli process --input=data.csv --output=report.txt")
		fmt.Println("\nOptions:")
		processCmd.PrintDefaults()
	}

	return processCmd
}

// ParseCommandLineOptions parses command line flags into a structured options object
func ParseCommandLineOptions(cmd *flag.FlagSet) CommandLineOptions {
	// Input/output options
	inputFile := cmd.Lookup("input").Value.String()
	outputFile := cmd.Lookup("output").Value.String()
	format := cmd.Lookup("format").Value.String()

	// Processing options
	useStreaming := cmd.Lookup("stream").Value.(flag.Getter).Get().(bool)
	sampleRate := int(cmd.Lookup("sample").Value.(flag.Getter).Get().(int64))
	maxRecords := int(cmd.Lookup("max").Value.(flag.Getter).Get().(int64))

	// Time filtering options
	startTime := cmd.Lookup("start").Value.String()
	endTime := cmd.Lookup("end").Value.String()
	timeWindow := cmd.Lookup("window").Value.String()

	// Data filtering options
	minTemp := cmd.Lookup("min-temp").Value.(flag.Getter).Get().(int64)
	voltageMin := cmd.Lookup("volt-min").Value.(flag.Getter).Get().(int64)
	voltageMax := cmd.Lookup("volt-max").Value.(flag.Getter).Get().(int64)
	currentMin := cmd.Lookup("curr-min").Value.(flag.Getter).Get().(int64)
	currentMax := cmd.Lookup("curr-max").Value.(flag.Getter).Get().(int64)

	// Specific metrics extraction
	metric := cmd.Lookup("metric").Value.String()

	// Convert format string to OutputFormat
	var outputFormat OutputFormat
	switch strings.ToLower(format) {
	case "json":
		outputFormat = FormatJSON
	case "csv":
		outputFormat = FormatCSV
	default:
		outputFormat = FormatText
	}

	return CommandLineOptions{
		InputFile:    inputFile,
		OutputFile:   outputFile,
		Format:       outputFormat,
		UseStreaming: useStreaming,
		SampleRate:   sampleRate,
		MaxRecords:   maxRecords,
		StartTime:    startTime,
		EndTime:      endTime,
		TimeWindow:   timeWindow,
		MinTemp:      minTemp,
		VoltageMin:   voltageMin,
		VoltageMax:   voltageMax,
		CurrentMin:   currentMin,
		CurrentMax:   currentMax,
		Metric:       metric,
	}
}

// ProcessCommand executes the main data processing functionality
func ProcessCommand(options CommandLineOptions) error {
	// Validate input file
	if options.InputFile == "" {
		return fmt.Errorf("input file is required")
	}

	// Ensure input file exists
	if _, err := os.Stat(options.InputFile); os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", options.InputFile)
	}

	// Create CSV parser with appropriate options
	csvParser := parser.NewCSVParser(options.InputFile)
	filterOptions, err := buildFilterOptions(options)
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
	fmt.Printf("Processing data from %s...\n", options.InputFile)

	var energyMetrics metrics.EnergyMetrics

	// Configure metrics options
	metricsOptions := buildMetricsOptions(options)

	// Process data either with streaming or regular mode
	if options.UseStreaming {
		fmt.Println("Using streaming mode for memory-efficient processing...")
		energyMetrics, err = metrics.StreamCalculateMetrics(csvParser, metricsOptions)
		if err != nil {
			return fmt.Errorf("failed to process CSV data in streaming mode: %v", err)
		}
	} else {
		// Parse all records at once
		records, err := csvParser.Parse()
		if err != nil {
			return fmt.Errorf("failed to parse CSV data: %v", err)
		}
		fmt.Printf("Successfully parsed %d records\n", len(records))

		// Calculate metrics
		calculator := metrics.NewEnergyCalculator(records).WithOptions(metricsOptions)
		energyMetrics = calculator.CalculateMetrics()
	}

	// Generate appropriate output based on requested format and metrics
	output, err := generateOutput(energyMetrics, options)
	if err != nil {
		return fmt.Errorf("failed to generate output: %v", err)
	}

	// Display or save the output
	if options.OutputFile == "" {
		fmt.Println(output)
	} else {
		err = os.WriteFile(options.OutputFile, []byte(output), 0644)
		if err != nil {
			return fmt.Errorf("failed to write output file: %v", err)
		}
		fmt.Printf("Results saved to %s\n", options.OutputFile)
	}

	return nil
}

// buildFilterOptions converts CLI options into parser filter options
func buildFilterOptions(cliOptions CommandLineOptions) (parser.FilterOptions, error) {
	filterOptions := parser.FilterOptions{
		SampleRate: cliOptions.SampleRate,
		MaxRecords: cliOptions.MaxRecords,
	}

	// Process time filters
	if cliOptions.StartTime != "" {
		startTime, err := parseTimeString(cliOptions.StartTime)
		if err != nil {
			return filterOptions, fmt.Errorf("invalid start time: %w", err)
		}
		filterOptions.StartTime = &startTime
	}

	if cliOptions.EndTime != "" {
		endTime, err := parseTimeString(cliOptions.EndTime)
		if err != nil {
			return filterOptions, fmt.Errorf("invalid end time: %w", err)
		}
		filterOptions.EndTime = &endTime
	}

	// Process time window if specified
	if cliOptions.TimeWindow != "" && filterOptions.EndTime == nil {
		// Parse the time window (e.g., 1h, 30m, 24h)
		duration, err := time.ParseDuration(cliOptions.TimeWindow)
		if err != nil {
			return filterOptions, fmt.Errorf("invalid time window: %w", err)
		}

		// If start time is specified, set end time relative to it
		// Otherwise, set end time to now and start time relative to that
		if filterOptions.StartTime != nil {
			endTime := filterOptions.StartTime.Add(duration)
			filterOptions.EndTime = &endTime
		} else {
			endTime := time.Now()
			startTime := endTime.Add(-duration)
			filterOptions.EndTime = &endTime
			filterOptions.StartTime = &startTime
		}
	}

	// Set temperature threshold if specified
	if cliOptions.MinTemp > 0 {
		filterOptions.TempThreshold = &cliOptions.MinTemp
	}

	// Set voltage range if specified
	if cliOptions.VoltageMin != 0 || cliOptions.VoltageMax != 0 {
		voltMin := cliOptions.VoltageMin
		voltMax := cliOptions.VoltageMax

		// If only min is specified, use very high max
		if voltMin != 0 && voltMax == 0 {
			voltMax = 1_000_000_000 // 1000V in µV
		}

		// If only max is specified, use very low min
		if voltMin == 0 && voltMax != 0 {
			voltMin = -1_000_000_000 // -1000V in µV
		}

		filterOptions.VoltageRange = &[2]int64{voltMin, voltMax}
	}

	// Set current range if specified
	if cliOptions.CurrentMin != 0 || cliOptions.CurrentMax != 0 {
		currMin := cliOptions.CurrentMin
		currMax := cliOptions.CurrentMax

		// If only min is specified, use very high max
		if currMin != 0 && currMax == 0 {
			currMax = 1_000_000_000_000 // 1000A in nA
		}

		// If only max is specified, use very low min
		if currMin == 0 && currMax != 0 {
			currMin = -1_000_000_000_000 // -1000A in nA
		}

		filterOptions.CurrentRange = &[2]int64{currMin, currMax}
	}

	return filterOptions, nil
}

// buildMetricsOptions converts CLI options into metrics calculation options
func buildMetricsOptions(cliOptions CommandLineOptions) metrics.MetricsOptions {
	options := metrics.MetricsOptions{
		TimeResolution: time.Minute * 5, // Default 5-minute resolution
	}

	// Add specific metrics if requested
	if cliOptions.Metric != "" {
		metricType := metrics.MetricType(cliOptions.Metric)
		options.RequestedMetrics = []metrics.MetricType{metricType}
	}

	return options
}

// generateOutput creates the appropriate output format based on user options
func generateOutput(energyMetrics metrics.EnergyMetrics, options CommandLineOptions) (string, error) {
	// If a specific metric was requested, extract just that
	if options.Metric != "" {
		metricType := metrics.MetricType(options.Metric)
		specificMetric, err := metrics.GetSpecificMetric(energyMetrics, metricType)
		if err != nil {
			return "", err
		}

		// Format the specific metric according to output format
		switch options.Format {
		case FormatJSON:
			jsonData, err := json.MarshalIndent(specificMetric, "", "  ")
			if err != nil {
				return "", fmt.Errorf("failed to marshal JSON: %w", err)
			}
			return string(jsonData), nil

		case FormatCSV:
			return formatMetricAsCSV(specificMetric, metricType)

		default: // Text format
			return formatMetricAsText(specificMetric, metricType), nil
		}
	}

	// If no specific metric was requested, format the full report
	switch options.Format {
	case FormatJSON:
		jsonData, err := json.MarshalIndent(energyMetrics, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal JSON: %w", err)
		}
		return string(jsonData), nil

	case FormatCSV:
		return generateCSVReport(energyMetrics)

	default: // Text format
		return generateReport(energyMetrics, options.InputFile), nil
	}
}

// formatMetricAsCSV formats a specific metric in CSV format
func formatMetricAsCSV(metric interface{}, metricType metrics.MetricType) (string, error) {
	var sb strings.Builder

	switch metricType {
	case metrics.MetricTotalEnergy, metrics.MetricAveragePower, metrics.MetricPeakPower:
		value, ok := metric.(float64)
		if !ok {
			return "", fmt.Errorf("unexpected type for %s", metricType)
		}
		sb.WriteString(fmt.Sprintf("%s,Value\n", metricType))
		sb.WriteString(fmt.Sprintf("%s,%.6f\n", metricType, value))

	case metrics.MetricTemperature:
		tempStats, ok := metric.(metrics.TemperatureStats)
		if !ok {
			return "", fmt.Errorf("unexpected type for temperature stats")
		}
		sb.WriteString("Measurement,Value\n")
		sb.WriteString(fmt.Sprintf("MinTemperature,%.2f\n", tempStats.MinTempCelsius))
		sb.WriteString(fmt.Sprintf("MaxTemperature,%.2f\n", tempStats.MaxTempCelsius))
		sb.WriteString(fmt.Sprintf("AvgTemperature,%.2f\n", tempStats.AvgTempCelsius))

	case metrics.MetricEnergyByHour:
		hourlyEnergy, ok := metric.(map[int]float64)
		if !ok {
			return "", fmt.Errorf("unexpected type for hourly energy")
		}
		sb.WriteString("Hour,EnergyJoules\n")
		for h := 0; h < 24; h++ {
			if energy, exists := hourlyEnergy[h]; exists {
				sb.WriteString(fmt.Sprintf("%d,%.6f\n", h, energy))
			}
		}

	case metrics.MetricVoltageStats:
		voltStats, ok := metric.(metrics.VoltageStats)
		if !ok {
			return "", fmt.Errorf("unexpected type for voltage stats")
		}
		sb.WriteString("Measurement,Value\n")
		sb.WriteString(fmt.Sprintf("MinVoltage,%.6f\n", voltStats.MinVoltage))
		sb.WriteString(fmt.Sprintf("MaxVoltage,%.6f\n", voltStats.MaxVoltage))
		sb.WriteString(fmt.Sprintf("AvgVoltage,%.6f\n", voltStats.AvgVoltage))

	case metrics.MetricCurrentStats:
		currentStats, ok := metric.(metrics.CurrentStats)
		if !ok {
			return "", fmt.Errorf("unexpected type for current stats")
		}
		sb.WriteString("Measurement,Value\n")
		sb.WriteString(fmt.Sprintf("MinCurrent,%.9f\n", currentStats.MinCurrent))
		sb.WriteString(fmt.Sprintf("MaxCurrent,%.9f\n", currentStats.MaxCurrent))
		sb.WriteString(fmt.Sprintf("AvgCurrent,%.9f\n", currentStats.AvgCurrent))
		sb.WriteString(fmt.Sprintf("MaxDischarge,%.9f\n", currentStats.MaxDischarge))
		sb.WriteString(fmt.Sprintf("MaxCharging,%.9f\n", currentStats.MaxCharging))

	case metrics.MetricBatteryDischarge:
		batteryStats, ok := metric.(metrics.BatteryStats)
		if !ok {
			return "", fmt.Errorf("unexpected type for battery stats")
		}
		sb.WriteString("Measurement,Value\n")
		sb.WriteString(fmt.Sprintf("TotalDischargeTime,%.2f\n", batteryStats.TotalDischargeTime))
		sb.WriteString(fmt.Sprintf("TotalChargeTime,%.2f\n", batteryStats.TotalChargeTime))
		sb.WriteString(fmt.Sprintf("DischargeToChargeRatio,%.6f\n", batteryStats.DischargeToChargeRatio))
		sb.WriteString(fmt.Sprintf("AverageDischargeRate,%.6f\n", batteryStats.AverageDischargeRate))

	case metrics.MetricSolarContribution:
		solarStats, ok := metric.(metrics.SolarStats)
		if !ok {
			return "", fmt.Errorf("unexpected type for solar stats")
		}
		sb.WriteString("Measurement,Value\n")
		sb.WriteString(fmt.Sprintf("TotalEnergyProduced,%.6f\n", solarStats.TotalEnergyProduced))
		sb.WriteString(fmt.Sprintf("AverageOutput,%.6f\n", solarStats.AverageOutput))
		sb.WriteString(fmt.Sprintf("PeakOutput,%.6f\n", solarStats.PeakOutput))
		sb.WriteString(fmt.Sprintf("ContributionPercentage,%.2f\n", solarStats.ContributionPercentage))

	default:
		return "", fmt.Errorf("CSV formatting not supported for metric type: %s", metricType)
	}

	return sb.String(), nil
}

// formatMetricAsText formats a specific metric in human-readable text format
func formatMetricAsText(metric interface{}, metricType metrics.MetricType) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("===== %s =====\n", strings.ToUpper(string(metricType))))

	switch metricType {
	case metrics.MetricTotalEnergy:
		value := metric.(float64)
		sb.WriteString(fmt.Sprintf("Total Energy: %.4f joules\n", value))

	case metrics.MetricAveragePower:
		value := metric.(float64)
		sb.WriteString(fmt.Sprintf("Average Power: %.4f watts\n", value))

	case metrics.MetricPeakPower:
		value := metric.(float64)
		sb.WriteString(fmt.Sprintf("Peak Power: %.4f watts\n", value))

	case metrics.MetricTemperature:
		tempStats := metric.(metrics.TemperatureStats)
		sb.WriteString(fmt.Sprintf("Minimum Temperature: %.2f °C\n", tempStats.MinTempCelsius))
		sb.WriteString(fmt.Sprintf("Maximum Temperature: %.2f °C\n", tempStats.MaxTempCelsius))
		sb.WriteString(fmt.Sprintf("Average Temperature: %.2f °C\n", tempStats.AvgTempCelsius))

	case metrics.MetricEnergyByHour:
		hourlyEnergy := metric.(map[int]float64)
		sb.WriteString("Energy Consumption by Hour:\n")
		for h := 0; h < 24; h++ {
			if energy, exists := hourlyEnergy[h]; exists {
				sb.WriteString(fmt.Sprintf("Hour %02d: %.4f joules\n", h, energy))
			}
		}

	case metrics.MetricVoltageStats:
		voltStats := metric.(metrics.VoltageStats)
		sb.WriteString(fmt.Sprintf("Minimum Voltage: %.6f V\n", voltStats.MinVoltage))
		sb.WriteString(fmt.Sprintf("Maximum Voltage: %.6f V\n", voltStats.MaxVoltage))
		sb.WriteString(fmt.Sprintf("Average Voltage: %.6f V\n", voltStats.AvgVoltage))

	case metrics.MetricCurrentStats:
		currentStats := metric.(metrics.CurrentStats)
		sb.WriteString(fmt.Sprintf("Minimum Current: %.9f A\n", currentStats.MinCurrent))
		sb.WriteString(fmt.Sprintf("Maximum Current: %.9f A\n", currentStats.MaxCurrent))
		sb.WriteString(fmt.Sprintf("Average Current: %.9f A\n", currentStats.AvgCurrent))
		sb.WriteString(fmt.Sprintf("Maximum Discharge Current: %.9f A\n", currentStats.MaxDischarge))
		sb.WriteString(fmt.Sprintf("Maximum Charging Current: %.9f A\n", currentStats.MaxCharging))

	case metrics.MetricBatteryDischarge:
		batteryStats := metric.(metrics.BatteryStats)
		sb.WriteString(fmt.Sprintf("Total Discharge Time: %.2f seconds\n", batteryStats.TotalDischargeTime))
		sb.WriteString(fmt.Sprintf("Total Charge Time: %.2f seconds\n", batteryStats.TotalChargeTime))
		sb.WriteString(fmt.Sprintf("Discharge to Charge Ratio: %.2f%%\n", batteryStats.DischargeToChargeRatio*100))
		sb.WriteString(fmt.Sprintf("Average Discharge Rate: %.4f watts\n", batteryStats.AverageDischargeRate))

	case metrics.MetricSolarContribution:
		solarStats := metric.(metrics.SolarStats)
		sb.WriteString(fmt.Sprintf("Total Energy Produced: %.4f joules\n", solarStats.TotalEnergyProduced))
		sb.WriteString(fmt.Sprintf("Average Output: %.4f watts\n", solarStats.AverageOutput))
		sb.WriteString(fmt.Sprintf("Peak Output: %.4f watts\n", solarStats.PeakOutput))
		sb.WriteString(fmt.Sprintf("Contribution to Energy: %.2f%%\n", solarStats.ContributionPercentage))

	default:
		sb.WriteString(fmt.Sprintf("No text formatter available for metric type: %s\n", metricType))
	}

	return sb.String()
}

// generateCSVReport creates a CSV report for all metrics
func generateCSVReport(metrics metrics.EnergyMetrics) (string, error) {
	var sb strings.Builder

	sb.WriteString("Metric,Value\n")
	sb.WriteString(fmt.Sprintf("TotalJoules,%.6f\n", metrics.TotalJoules))
	sb.WriteString(fmt.Sprintf("AveragePowerWatts,%.6f\n", metrics.AveragePowerWatts))
	sb.WriteString(fmt.Sprintf("PeakPowerWatts,%.6f\n", metrics.PeakPowerWatts))
	sb.WriteString(fmt.Sprintf("JoulesPerDay,%.6f\n", metrics.JoulesPerDay))
	sb.WriteString(fmt.Sprintf("DurationSeconds,%.2f\n", metrics.DurationSeconds))
	sb.WriteString(fmt.Sprintf("DataPoints,%d\n", metrics.DataPoints))
	sb.WriteString(fmt.Sprintf("StartTime,%s\n", metrics.TimeRange.StartTime.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("EndTime,%s\n", metrics.TimeRange.EndTime.Format(time.RFC3339)))

	sb.WriteString("\nTemperatureStats,Value\n")
	sb.WriteString(fmt.Sprintf("MinTempCelsius,%.2f\n", metrics.TemperatureStats.MinTempCelsius))
	sb.WriteString(fmt.Sprintf("MaxTempCelsius,%.2f\n", metrics.TemperatureStats.MaxTempCelsius))
	sb.WriteString(fmt.Sprintf("AvgTempCelsius,%.2f\n", metrics.TemperatureStats.AvgTempCelsius))

	sb.WriteString("\nVoltageStats,Value\n")
	sb.WriteString(fmt.Sprintf("MinVoltage,%.6f\n", metrics.VoltageStats.MinVoltage))
	sb.WriteString(fmt.Sprintf("MaxVoltage,%.6f\n", metrics.VoltageStats.MaxVoltage))
	sb.WriteString(fmt.Sprintf("AvgVoltage,%.6f\n", metrics.VoltageStats.AvgVoltage))

	sb.WriteString("\nCurrentStats,Value\n")
	sb.WriteString(fmt.Sprintf("MinCurrent,%.9f\n", metrics.CurrentStats.MinCurrent))
	sb.WriteString(fmt.Sprintf("MaxCurrent,%.9f\n", metrics.CurrentStats.MaxCurrent))
	sb.WriteString(fmt.Sprintf("AvgCurrent,%.9f\n", metrics.CurrentStats.AvgCurrent))
	sb.WriteString(fmt.Sprintf("MaxDischarge,%.9f\n", metrics.CurrentStats.MaxDischarge))
	sb.WriteString(fmt.Sprintf("MaxCharging,%.9f\n", metrics.CurrentStats.MaxCharging))

	sb.WriteString("\nBatteryStats,Value\n")
	sb.WriteString(fmt.Sprintf("TotalDischargeTime,%.2f\n", metrics.BatteryStats.TotalDischargeTime))
	sb.WriteString(fmt.Sprintf("TotalChargeTime,%.2f\n", metrics.BatteryStats.TotalChargeTime))
	sb.WriteString(fmt.Sprintf("DischargeToChargeRatio,%.6f\n", metrics.BatteryStats.DischargeToChargeRatio))
	sb.WriteString(fmt.Sprintf("AverageDischargeRate,%.6f\n", metrics.BatteryStats.AverageDischargeRate))

	sb.WriteString("\nSolarStats,Value\n")
	sb.WriteString(fmt.Sprintf("TotalEnergyProduced,%.6f\n", metrics.SolarStats.TotalEnergyProduced))
	sb.WriteString(fmt.Sprintf("AverageOutput,%.6f\n", metrics.SolarStats.AverageOutput))
	sb.WriteString(fmt.Sprintf("PeakOutput,%.6f\n", metrics.SolarStats.PeakOutput))
	sb.WriteString(fmt.Sprintf("ContributionPercentage,%.2f\n", metrics.SolarStats.ContributionPercentage))

	sb.WriteString("\nHour,EnergyJoules\n")
	for h := 0; h < 24; h++ {
		if energy, exists := metrics.EnergyConsumptionByHour[h]; exists {
			sb.WriteString(fmt.Sprintf("%d,%.6f\n", h, energy))
		}
	}

	return sb.String(), nil
}

// generateReport creates a human-readable report of the energy metrics
func generateReport(metrics metrics.EnergyMetrics, inputFile string) string {
	var sb strings.Builder

	sb.WriteString("========== ENEMETER DATA PROCESSING REPORT ==========\n")
	sb.WriteString(fmt.Sprintf("Input File: %s\n", filepath.Base(inputFile)))
	sb.WriteString(fmt.Sprintf("Date: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("Data Points: %d\n", metrics.DataPoints))
	sb.WriteString(fmt.Sprintf("Time Range: %s to %s\n\n",
		metrics.TimeRange.StartTime.Format("2006-01-02 15:04:05"),
		metrics.TimeRange.EndTime.Format("2006-01-02 15:04:05")))

	sb.WriteString("ENERGY METRICS\n")
	sb.WriteString("-------------\n")
	sb.WriteString(fmt.Sprintf("Total Energy Consumed: %.4f joules\n", metrics.TotalJoules))
	sb.WriteString(fmt.Sprintf("Average Power: %.4f watts\n", metrics.AveragePowerWatts))
	sb.WriteString(fmt.Sprintf("Peak Power: %.4f watts\n", metrics.PeakPowerWatts))
	sb.WriteString(fmt.Sprintf("Estimated Energy per Day: %.4f joules\n", metrics.JoulesPerDay))
	sb.WriteString(fmt.Sprintf("Measurement Duration: %.2f seconds\n\n", metrics.DurationSeconds))

	sb.WriteString("TEMPERATURE STATISTICS\n")
	sb.WriteString("---------------------\n")
	sb.WriteString(fmt.Sprintf("Minimum Temperature: %.2f °C\n", metrics.TemperatureStats.MinTempCelsius))
	sb.WriteString(fmt.Sprintf("Maximum Temperature: %.2f °C\n", metrics.TemperatureStats.MaxTempCelsius))
	sb.WriteString(fmt.Sprintf("Average Temperature: %.2f °C\n\n", metrics.TemperatureStats.AvgTempCelsius))

	sb.WriteString("VOLTAGE STATISTICS\n")
	sb.WriteString("----------------\n")
	sb.WriteString(fmt.Sprintf("Minimum Voltage: %.6f V\n", metrics.VoltageStats.MinVoltage))
	sb.WriteString(fmt.Sprintf("Maximum Voltage: %.6f V\n", metrics.VoltageStats.MaxVoltage))
	sb.WriteString(fmt.Sprintf("Average Voltage: %.6f V\n\n", metrics.VoltageStats.AvgVoltage))

	sb.WriteString("CURRENT STATISTICS\n")
	sb.WriteString("----------------\n")
	sb.WriteString(fmt.Sprintf("Minimum Current: %.9f A\n", metrics.CurrentStats.MinCurrent))
	sb.WriteString(fmt.Sprintf("Maximum Current: %.9f A\n", metrics.CurrentStats.MaxCurrent))
	sb.WriteString(fmt.Sprintf("Average Current: %.9f A\n", metrics.CurrentStats.AvgCurrent))
	sb.WriteString(fmt.Sprintf("Maximum Discharge Current: %.9f A\n", metrics.CurrentStats.MaxDischarge))
	sb.WriteString(fmt.Sprintf("Maximum Charging Current: %.9f A\n\n", metrics.CurrentStats.MaxCharging))

	sb.WriteString("BATTERY STATISTICS\n")
	sb.WriteString("------------------\n")
	sb.WriteString(fmt.Sprintf("Total Discharge Time: %.2f seconds\n", metrics.BatteryStats.TotalDischargeTime))
	sb.WriteString(fmt.Sprintf("Total Charge Time: %.2f seconds\n", metrics.BatteryStats.TotalChargeTime))
	sb.WriteString(fmt.Sprintf("Discharge to Charge Ratio: %.2f%%\n", metrics.BatteryStats.DischargeToChargeRatio*100))
	sb.WriteString(fmt.Sprintf("Average Discharge Rate: %.4f watts\n\n", metrics.BatteryStats.AverageDischargeRate))

	sb.WriteString("SOLAR CONTRIBUTION\n")
	sb.WriteString("-----------------\n")
	sb.WriteString(fmt.Sprintf("Total Energy Produced: %.4f joules\n", metrics.SolarStats.TotalEnergyProduced))
	sb.WriteString(fmt.Sprintf("Average Output: %.4f watts\n", metrics.SolarStats.AverageOutput))
	sb.WriteString(fmt.Sprintf("Peak Output: %.4f watts\n", metrics.SolarStats.PeakOutput))
	sb.WriteString(fmt.Sprintf("Contribution to Energy: %.2f%%\n\n", metrics.SolarStats.ContributionPercentage))

	sb.WriteString("HOURLY ENERGY CONSUMPTION\n")
	sb.WriteString("------------------------\n")

	// Sort and print hourly consumption
	if len(metrics.EnergyConsumptionByHour) > 0 {
		for hour := 0; hour < 24; hour++ {
			if joules, exists := metrics.EnergyConsumptionByHour[hour]; exists {
				sb.WriteString(fmt.Sprintf("Hour %02d: %.4f joules\n", hour, joules))
			}
		}
	} else {
		sb.WriteString("No hourly data available\n")
	}

	return sb.String()
}

// parseTimeString parses a time string in the format YYYY-MM-DD[THH:MM:SS]
func parseTimeString(timeStr string) (time.Time, error) {
	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, timeStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("could not parse time using supported formats")
}
