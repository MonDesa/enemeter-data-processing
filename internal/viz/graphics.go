package viz

import (
	"enemeter-data-processing/internal/metrics"
	"enemeter-data-processing/internal/parser"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"
)

// GraphType represents the type of graph to generate
type GraphType string

// Available graph types
const (
	GraphTypePower       GraphType = "power"
	GraphTypeEnergy      GraphType = "energy"
	GraphTypeVoltage     GraphType = "voltage"
	GraphTypeCurrent     GraphType = "current"
	GraphTypeTemperature GraphType = "temperature"
)

// Unit scale options for each metric type
type UnitScale string

const (
	// Time scales
	TimeScaleMilliseconds UnitScale = "milliseconds"
	TimeScaleSeconds      UnitScale = "seconds"
	TimeScaleHours        UnitScale = "hours"
	TimeScaleDays         UnitScale = "days"

	// Voltage scales
	VoltageScaleMicroVolts UnitScale = "microvolts"
	VoltageScaleMilliVolts UnitScale = "millivolts"
	VoltageScaleVolts      UnitScale = "volts"

	// Current scales
	CurrentScaleNanoAmperes  UnitScale = "nanoamperes"
	CurrentScaleMicroAmperes UnitScale = "microamperes"
	CurrentScaleMilliAmperes UnitScale = "milliamperes"
	CurrentScaleAmperes      UnitScale = "amperes"

	// Temperature scales
	TempScaleMilliCelsius UnitScale = "millicelsius"
	TempScaleCelsius      UnitScale = "celsius"
)

// ScaleOptions holds configuration for chart scales
type ScaleOptions struct {
	TimeScale    string // milliseconds, seconds, minutes, hours, days
	VoltageScale string // microvolts, millivolts, volts
	CurrentScale string // nanoamperes, microamperes, milliamperes, amperes
	TempScale    string // millicelsius, celsius
	PowerScale   string // microwatts, milliwatts, watts, kilowatts
	EnergyScale  string // microjoules, millijoules, joules, kilojoules, watt-hours, kilowatt-hours
}

// GraphicsGenerator handles the creation of charts from energy metrics
type GraphicsGenerator struct {
	records       []parser.EnemeterRecord
	outputDir     string
	dateFormat    string
	timeFormat    string
	chartTheme    string
	chartWidth    string
	chartHeight   string
	scaleOptions  ScaleOptions
	maxDataPoints int    // Maximum number of data points to render for performance
	samplingMode  string // Method used for sampling: "even", "adaptive", "lttb"
}

// NewGraphicsGenerator creates a new GraphicsGenerator with default settings
func NewGraphicsGenerator(records []parser.EnemeterRecord, outputDir string) *GraphicsGenerator {
	return &GraphicsGenerator{
		records:       records,
		outputDir:     outputDir,
		dateFormat:    "2006-01-02",
		timeFormat:    "15:04:05",
		chartTheme:    types.ThemeWesteros,
		chartWidth:    "900px",
		chartHeight:   "500px",
		maxDataPoints: 2000,       // Default to 2000 points max for better performance
		samplingMode:  "adaptive", // Use adaptive sampling by default
		scaleOptions: ScaleOptions{
			TimeScale:    "seconds",
			VoltageScale: "volts",
			CurrentScale: "milliamperes",
			TempScale:    "celsius",
			PowerScale:   "watts",
			EnergyScale:  "joules",
		},
	}
}

// WithScaleOptions sets the scale options for charts
func (g *GraphicsGenerator) WithScaleOptions(options ScaleOptions) *GraphicsGenerator {
	g.scaleOptions = options
	return g
}

// WithMaxDataPoints sets the maximum number of data points to plot
func (g *GraphicsGenerator) WithMaxDataPoints(max int) *GraphicsGenerator {
	g.maxDataPoints = max
	return g
}

// WithSamplingMode sets the data sampling algorithm
// Supported modes: "even" (evenly spaced sampling), "adaptive" (more detail in areas of high variance)
// "lttb" (Largest Triangle Three Buckets - preserves visual characteristics)
func (g *GraphicsGenerator) WithSamplingMode(mode string) *GraphicsGenerator {
	g.samplingMode = mode
	return g
}

// getTimeScaleFactor returns the conversion factor for the selected time scale
func (g *GraphicsGenerator) getTimeScaleFactor() (float64, string) {
	switch g.scaleOptions.TimeScale {
	case "milliseconds":
		return 1000, "ms"
	case "minutes":
		return 1.0 / 60.0, "min"
	case "hours":
		return 1.0 / 3600.0, "h"
	case "days":
		return 1.0 / 86400.0, "d"
	default: // seconds
		return 1.0, "s"
	}
}

// getVoltageScaleFactor returns the conversion factor for the selected voltage scale
func (g *GraphicsGenerator) getVoltageScaleFactor() (float64, string) {
	switch g.scaleOptions.VoltageScale {
	case "microvolts":
		return 1000000.0, "µV"
	case "millivolts":
		return 1000.0, "mV"
	default: // volts
		return 1.0, "V"
	}
}

// getCurrentScaleFactor returns the conversion factor for the selected current scale
func (g *GraphicsGenerator) getCurrentScaleFactor() (float64, string) {
	switch g.scaleOptions.CurrentScale {
	case "nanoamperes":
		return 1000000000.0, "nA"
	case "microamperes":
		return 1000000.0, "µA"
	case "amperes":
		return 1.0, "A"
	default: // milliamperes
		return 1000.0, "mA"
	}
}

// getTempScaleFactor returns the conversion factor for the selected temperature scale
func (g *GraphicsGenerator) getTempScaleFactor() (float64, string) {
	switch g.scaleOptions.TempScale {
	case "millicelsius":
		return 1000.0, "m°C"
	default: // celsius
		return 1.0, "°C"
	}
}

// getPowerScaleFactor returns the conversion factor for the selected power scale
func (g *GraphicsGenerator) getPowerScaleFactor() (float64, string) {
	switch g.scaleOptions.PowerScale {
	case "microwatts":
		return 1000000.0, "µW"
	case "milliwatts":
		return 1000.0, "mW"
	case "kilowatts":
		return 0.001, "kW"
	default: // watts
		return 1.0, "W"
	}
}

// getEnergyScaleFactor returns the conversion factor for the selected energy scale
func (g *GraphicsGenerator) getEnergyScaleFactor() (float64, string) {
	switch g.scaleOptions.EnergyScale {
	case "microjoules":
		return 1000000.0, "µJ"
	case "millijoules":
		return 1000.0, "mJ"
	case "kilojoules":
		return 0.001, "kJ"
	case "watt-hours":
		return 1.0 / 3600.0, "Wh"
	case "kilowatt-hours":
		return 1.0 / 3600000.0, "kWh"
	default: // joules
		return 1.0, "J"
	}
}

// Fix the formatWithUnit function to correctly define the formatter
func formatWithUnit(unit string) types.FuncStr {
	// The function should return just the format string, not a JS function declaration
	return types.FuncStr("{value} " + unit)
}

// downsampleData reduces the number of data points using the selected sampling mode
func (g *GraphicsGenerator) downsampleData(timestamps []time.Time, values []float64, maxPoints int) ([]time.Time, []float64) {
	n := len(timestamps)
	if n <= maxPoints {
		return timestamps, values // No need to downsample
	}

	switch g.samplingMode {
	case "even":
		// Simple evenly-spaced sampling (fastest but can miss important details)
		step := float64(n) / float64(maxPoints)

		newTimestamps := make([]time.Time, 0, maxPoints)
		newValues := make([]float64, 0, maxPoints)

		for i := 0; i < maxPoints; i++ {
			idx := int(math.Floor(float64(i) * step))
			if idx >= n {
				idx = n - 1
			}
			newTimestamps = append(newTimestamps, timestamps[idx])
			newValues = append(newValues, values[idx])
		}

		return newTimestamps, newValues

	case "lttb":
		// Largest Triangle Three Buckets algorithm (best visual representation)
		result := lttbDownsample(timestamps, values, maxPoints)
		return result.timestamps, result.values

	default:
		// "adaptive" sampling - more points where data changes rapidly
		return adaptiveDownsample(timestamps, values, maxPoints)
	}
}

// Adaptive downsampling keeps more points in areas of high variance
func adaptiveDownsample(timestamps []time.Time, values []float64, maxPoints int) ([]time.Time, []float64) {
	n := len(timestamps)
	if n <= maxPoints {
		return timestamps, values
	}

	// Always include first and last points
	resultTimestamps := make([]time.Time, 0, maxPoints)
	resultValues := make([]float64, 0, maxPoints)
	resultTimestamps = append(resultTimestamps, timestamps[0])
	resultValues = append(resultValues, values[0])

	// Calculate point importance based on local variance
	importance := make([]float64, n)
	for i := 1; i < n-1; i++ {
		diff1 := math.Abs(values[i] - values[i-1])
		diff2 := math.Abs(values[i] - values[i+1])
		importance[i] = math.Max(diff1, diff2)
	}

	// Sort points by importance and select top points
	type pointWithImportance struct {
		index      int
		importance float64
	}

	pts := make([]pointWithImportance, n-2)
	for i := 1; i < n-1; i++ {
		pts[i-1] = pointWithImportance{i, importance[i]}
	}

	sort.Slice(pts, func(i, j int) bool {
		return pts[i].importance > pts[j].importance
	})

	// Take most important points
	toSelect := maxPoints - 2 // Already selected first and last
	if toSelect > len(pts) {
		toSelect = len(pts)
	}

	selectedIndices := make([]int, toSelect)
	for i := 0; i < toSelect; i++ {
		selectedIndices[i] = pts[i].index
	}

	sort.Ints(selectedIndices)

	for _, idx := range selectedIndices {
		resultTimestamps = append(resultTimestamps, timestamps[idx])
		resultValues = append(resultValues, values[idx])
	}

	// Add the last point
	resultTimestamps = append(resultTimestamps, timestamps[n-1])
	resultValues = append(resultValues, values[n-1])

	return resultTimestamps, resultValues
}

// LTTB downsampling result
type lttbResult struct {
	timestamps []time.Time
	values     []float64
}

// Largest Triangle Three Buckets algorithm
// Preserves visual characteristics of the original data
func lttbDownsample(timestamps []time.Time, values []float64, maxPoints int) lttbResult {
	dataLength := len(timestamps)
	if dataLength <= maxPoints {
		return lttbResult{timestamps, values}
	}

	// Always include first and last points
	result := lttbResult{
		timestamps: make([]time.Time, 0, maxPoints),
		values:     make([]float64, 0, maxPoints),
	}

	// Add first point
	result.timestamps = append(result.timestamps, timestamps[0])
	result.values = append(result.values, values[0])

	// Bucket size
	bucketSize := float64(dataLength-2) / float64(maxPoints-2)

	// Process buckets
	var a int = 0 // Initially a is the first point
	for i := 0; i < maxPoints-2; i++ {
		// Calculate bucket boundaries
		fromBucket := int(math.Floor(float64(i)*bucketSize)) + 1
		toBucket := int(math.Floor(float64(i+1)*bucketSize)) + 1

		if toBucket >= dataLength {
			toBucket = dataLength - 1
		}

		// Safety check to prevent infinite loops or invalid ranges
		if fromBucket >= toBucket || fromBucket >= dataLength {
			continue
		}

		// Calculate average for next bucket
		avgX := 0.0
		avgY := 0.0

		pointsInBucket := toBucket - fromBucket
		if pointsInBucket > 0 {
			for j := fromBucket; j < toBucket; j++ {
				avgX += float64(timestamps[j].Unix())
				avgY += values[j]
			}
			avgX /= float64(pointsInBucket)
			avgY /= float64(pointsInBucket)
		} else {
			avgX = float64(timestamps[fromBucket].Unix())
			avgY = values[fromBucket]
		}

		// Find point in this bucket with largest triangle area
		var maxArea float64 = -1
		var maxAreaIndex int = fromBucket

		var x1 float64 = float64(timestamps[a].Unix())
		var y1 float64 = values[a]
		var x3 float64 = avgX
		var y3 float64 = avgY

		for j := fromBucket; j < toBucket; j++ {
			x2 := float64(timestamps[j].Unix())
			y2 := values[j]

			// Calculate triangle area
			area := math.Abs((x1*(y2-y3) + x2*(y3-y1) + x3*(y1-y2)) / 2.0)

			if area > maxArea {
				maxArea = area
				maxAreaIndex = j
			}
		}

		// Add point with largest triangle area in this bucket
		result.timestamps = append(result.timestamps, timestamps[maxAreaIndex])
		result.values = append(result.values, values[maxAreaIndex])

		// Next point becomes reference point
		a = maxAreaIndex
	}

	// Add last point
	result.timestamps = append(result.timestamps, timestamps[dataLength-1])
	result.values = append(result.values, values[dataLength-1])

	return result
}

// GenerateChart creates a chart based on the specified type and writes it to a file
func (g *GraphicsGenerator) GenerateChart(graphType GraphType, metrics metrics.EnergyMetrics) (string, error) {
	if len(g.records) == 0 {
		return "", fmt.Errorf("no data records available")
	}

	// Sort records by timestamp
	sortedRecords := make([]parser.EnemeterRecord, len(g.records))
	copy(sortedRecords, g.records)

	// Sort by timestamp
	for i := 0; i < len(sortedRecords)-1; i++ {
		for j := i + 1; j < len(sortedRecords); j++ {
			if sortedRecords[j].Timestamp.Before(sortedRecords[i].Timestamp) {
				sortedRecords[i], sortedRecords[j] = sortedRecords[j], sortedRecords[i]
			}
		}
	}

	// Create appropriate chart based on type
	var filename string
	var err error

	switch graphType {
	case GraphTypePower:
		filename, err = g.generatePowerChart(sortedRecords, metrics)
	case GraphTypeEnergy:
		filename, err = g.generateEnergyChart(sortedRecords, metrics)
	case GraphTypeVoltage:
		filename, err = g.generateVoltageChart(sortedRecords)
	case GraphTypeCurrent:
		filename, err = g.generateCurrentChart(sortedRecords)
	case GraphTypeTemperature:
		filename, err = g.generateTemperatureChart(sortedRecords)
	default:
		return "", fmt.Errorf("unsupported graph type: %s", graphType)
	}

	if err != nil {
		return "", err
	}

	return filename, nil
}

// GenerateAllCharts generates all charts and enables time scaling
func (g *GraphicsGenerator) GenerateAllCharts(metrics metrics.EnergyMetrics) (map[GraphType]string, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(g.outputDir, os.ModePerm); err != nil {
		return nil, err
	}

	// Generate all charts
	chartTypes := []GraphType{
		GraphTypePower,
		GraphTypeEnergy,
		GraphTypeVoltage,
		GraphTypeCurrent,
		GraphTypeTemperature,
	}

	results := make(map[GraphType]string)
	for _, chartType := range chartTypes {
		filename, err := g.GenerateChart(chartType, metrics)
		if err != nil {
			return results, err
		}
		results[chartType] = filename
	}

	return results, nil
}

// Update the generateEnergyChart function to use title and subtitle
func (g *GraphicsGenerator) generateEnergyChart(records []parser.EnemeterRecord, metrics metrics.EnergyMetrics) (string, error) {
	// Create a line chart
	line := charts.NewLine()

	// Get scale factors for time and energy
	timeFactor, timeUnit := g.getTimeScaleFactor()
	energyFactor, energyUnit := g.getEnergyScaleFactor()

	// Get start time for reference
	startTime := records[0].Timestamp
	endTime := records[len(records)-1].Timestamp
	duration := endTime.Sub(startTime)

	// Add progress reporting
	fmt.Printf("Preparing data for energy chart (%d records)...\n", len(records))

	// For extremely large datasets, do aggressive pre-sampling before any other processing
	const maxRecordsToProcess = 100000 // Cap at 100K records for initial processing

	var recordsToProcess []parser.EnemeterRecord
	if len(records) > maxRecordsToProcess {
		fmt.Printf("Dataset very large (%d records). Pre-sampling to %d records...\n",
			len(records), maxRecordsToProcess)

		// Use simple even sampling for the pre-sampling phase
		step := float64(len(records)) / float64(maxRecordsToProcess)
		recordsToProcess = make([]parser.EnemeterRecord, 0, maxRecordsToProcess)

		for i := 0.0; i < float64(len(records)); i += step {
			idx := int(i)
			if idx >= len(records) {
				continue
			}
			recordsToProcess = append(recordsToProcess, records[idx])
		}

		// Make sure we include the last record for proper time range
		if len(recordsToProcess) > 0 && recordsToProcess[len(recordsToProcess)-1].Timestamp != records[len(records)-1].Timestamp {
			recordsToProcess = append(recordsToProcess, records[len(records)-1])
		}

		fmt.Printf("Pre-sampled to %d records\n", len(recordsToProcess))
	} else {
		recordsToProcess = records
	}

	// Configure chart
	title := "Cumulative Energy Consumption"
	subtitle := fmt.Sprintf("From: %s | Duration: %s | Total: %.4f %s",
		startTime.Format("2006-01-02 15:04:05"),
		duration.Round(time.Second).String(),
		metrics.TotalJoules*energyFactor,
		energyUnit)

	// Set chart options with title and subtitle
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  g.chartTheme,
			Width:  g.chartWidth,
			Height: g.chartHeight,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    title,
			Subtitle: subtitle,
			Left:     "center",
			Top:      "5%",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(true),
			Top:  "15%",
			Left: "center",
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show:  opts.Bool(true),
			Right: "20px",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show: opts.Bool(true),
				},
				DataZoom: &opts.ToolBoxFeatureDataZoom{
					Show: opts.Bool(true),
				},
				DataView: &opts.ToolBoxFeatureDataView{
					Show: opts.Bool(true),
				},
			},
		}),
		charts.WithGridOpts(opts.Grid{
			Top:          "20%",
			Right:        "5%",
			Bottom:       "5%",
			Left:         "5%",
			ContainLabel: opts.Bool(true),
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: fmt.Sprintf("Time (%s from start)", g.scaleOptions.TimeScale),
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: formatWithUnit(timeUnit),
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: fmt.Sprintf("Energy (%s)", energyUnit),
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: formatWithUnit(energyUnit),
			},
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
	)

	// Use our energy calculation utility for accurate energy calculation
	fmt.Println("Calculating cumulative energy...")
	energyCalc := NewEnergyCalculation(recordsToProcess)
	timestamps, energyValues := energyCalc.CalculateCumulativeEnergy()

	// Check if we have enough data points to continue
	if len(timestamps) < 2 {
		return "", fmt.Errorf("not enough valid data points for energy chart")
	}

	fmt.Printf("Processing %d energy data points...\n", len(timestamps))

	// For very large datasets, we need to pre-downsample before regular processing
	if len(timestamps) > 10000 {
		fmt.Printf("Pre-downsampling energy data from %d points to ~5000 points for processing efficiency...\n",
			len(timestamps))

		// Pre-downsampling to ~5000 points for efficiency using even sampling
		step := float64(len(timestamps)) / 5000.0
		newTs := make([]time.Time, 0, 5000+2) // +2 for first and last points
		newVals := make([]float64, 0, 5000+2)

		// Always include first point
		newTs = append(newTs, timestamps[0])
		newVals = append(newVals, energyValues[0])

		// Sample middle points
		for i := 1.0; i < float64(len(timestamps)-1); i += step {
			idx := int(i)
			if idx >= len(timestamps) {
				break
			}
			newTs = append(newTs, timestamps[idx])
			newVals = append(newVals, energyValues[idx])
		}

		// Always include last point
		newTs = append(newTs, timestamps[len(timestamps)-1])
		newVals = append(newVals, energyValues[len(energyValues)-1])

		timestamps = newTs
		energyValues = newVals

		fmt.Printf("Pre-downsampled to %d points\n", len(timestamps))
	}

	// Normalize time measurements - take a point every interval to reduce chart size
	minTimeDelta := 1.0 // seconds
	if len(records) > 100000 {
		minTimeDelta = 5.0 // increase for very large datasets
	}

	lastTimestampAdded := startTime.Add(-time.Second) // Ensure first point is always added

	xAxisData := make([]string, 0, len(timestamps))
	energyData := make([]opts.LineData, 0, len(timestamps))

	// Set a maximum number of points to process to avoid infinite loops
	maxPointsToProcess := len(timestamps)
	if len(timestamps) > 100000 {
		fmt.Println("Warning: Very large dataset. Limiting to 100,000 points for chart rendering.")
		maxPointsToProcess = 100000
	}

	// Process with progress reporting
	progressInterval := maxPointsToProcess / 20 // Report progress 20 times
	if progressInterval < 1 {
		progressInterval = 1
	}

	pointsProcessed := 0

	for i := 0; i < maxPointsToProcess; i++ {
		// Stop processing if we reach the end of the array
		if i >= len(timestamps) {
			break
		}

		// Skip if less than minTimeDelta since last added point
		if timestamps[i].Sub(lastTimestampAdded).Seconds() < minTimeDelta && i > 0 {
			continue
		}
		lastTimestampAdded = timestamps[i]

		// Calculate time from start with scaling factor
		timeFromStart := timestamps[i].Sub(startTime).Seconds() * timeFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		// Apply energy scaling factor
		scaledEnergy := energyValues[i] * energyFactor

		xAxisData = append(xAxisData, timeRounded)
		energyData = append(energyData, opts.LineData{
			Value: scaledEnergy,
			Name:  timestamps[i].Format(g.timeFormat), // Keep absolute time in tooltip
		})

		pointsProcessed++

		// Report progress periodically
		if i > 0 && i%progressInterval == 0 {
			fmt.Printf("Processed %d/%d energy points (%.1f%%)\n",
				i, maxPointsToProcess, float64(i)*100/float64(maxPointsToProcess))
		}
	}

	fmt.Printf("Total points processed: %d\n", pointsProcessed)

	// Apply downsampling if needed
	if len(xAxisData) > g.maxDataPoints {
		fmt.Printf("Downsampling from %d to %d data points using '%s' algorithm...\n",
			len(xAxisData), g.maxDataPoints, g.samplingMode)

		// Convert string times back to Time objects for downsampling
		times := make([]time.Time, 0, len(xAxisData))
		values := make([]float64, 0, len(xAxisData))

		for i := range xAxisData {
			if i >= len(energyData) {
				continue
			}

			seconds, err := strconv.ParseFloat(xAxisData[i], 64)
			if err != nil {
				continue
			}

			// Safe type assertion
			energyValue, ok := energyData[i].Value.(float64)
			if !ok {
				continue
			}

			times = append(times, startTime.Add(time.Duration(seconds/timeFactor*float64(time.Second))))
			values = append(values, energyValue)
		}

		// Safety check: if we couldn't convert any points, use simple sampling
		if len(times) == 0 || len(values) == 0 {
			fmt.Println("Warning: Could not convert data for downsampling. Using simple sampling instead.")

			// Simple sampling
			if len(xAxisData) > 0 && len(energyData) > 0 {
				step := float64(len(xAxisData)) / float64(g.maxDataPoints)
				newXData := make([]string, 0, g.maxDataPoints)
				newEnergyData := make([]opts.LineData, 0, g.maxDataPoints)

				for i := 0.0; i < float64(len(xAxisData)); i += step {
					idx := int(i)
					if idx >= len(xAxisData) || idx >= len(energyData) {
						continue
					}
					newXData = append(newXData, xAxisData[idx])
					newEnergyData = append(newEnergyData, energyData[idx])
				}

				xAxisData = newXData
				energyData = newEnergyData
			}
		} else {
			// Only downsample if we have data
			sampledTimes, sampledValues := g.downsampleData(times, values, g.maxDataPoints)

			// Convert back to chart format
			xAxisData = make([]string, len(sampledTimes))
			energyData = make([]opts.LineData, len(sampledValues))

			for i := range sampledTimes {
				timeFromStart := sampledTimes[i].Sub(startTime).Seconds() * timeFactor
				xAxisData[i] = fmt.Sprintf("%.2f", timeFromStart)
				energyData[i] = opts.LineData{
					Value: sampledValues[i],
					Name:  sampledTimes[i].Format(g.timeFormat),
				}
			}
		}
	}

	// Safety check to prevent chart rendering with no data
	if len(xAxisData) == 0 || len(energyData) == 0 {
		return "", fmt.Errorf("no valid data points to generate chart")
	}

	fmt.Printf("Final energy chart will have %d data points\n", len(xAxisData))

	// Add data to chart
	line.SetXAxis(xAxisData).
		AddSeries(fmt.Sprintf("Energy (%s)", energyUnit), energyData).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				Smooth: opts.Bool(false),
			}),
			charts.WithAreaStyleOpts(opts.AreaStyle{
				Opacity: 0.2,
			}),
		)

	// Create output file
	fmt.Println("Creating energy chart file...")
	filename := filepath.Join(g.outputDir, "energy_chart.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Render the chart
	fmt.Println("Rendering energy chart to file...")
	if err := line.Render(f); err != nil {
		return "", err
	}

	fmt.Println("Energy chart generation completed successfully!")
	return filename, nil
}

// Add the generatePowerChart function
func (g *GraphicsGenerator) generatePowerChart(records []parser.EnemeterRecord, metrics metrics.EnergyMetrics) (string, error) {
	// Create a line chart
	line := charts.NewLine()

	// Get scale factors for time and power
	timeFactor, timeUnit := g.getTimeScaleFactor()
	powerFactor, powerUnit := g.getPowerScaleFactor()

	// Get start time for reference
	startTime := records[0].Timestamp
	endTime := records[len(records)-1].Timestamp
	duration := endTime.Sub(startTime)

	// Configure chart title and subtitle
	title := "Power Consumption Over Time"
	subtitle := fmt.Sprintf("From: %s | Duration: %s | Avg: %.4f %s | Peak: %.4f %s",
		startTime.Format("2006-01-02 15:04:05"),
		duration.Round(time.Second).String(),
		metrics.AveragePowerWatts*powerFactor,
		powerUnit,
		metrics.PeakPowerWatts*powerFactor,
		powerUnit)

	// Add progress reporting for large datasets
	fmt.Printf("Preparing data for power chart (%d records)...\n", len(records))

	// For extremely large datasets, do aggressive pre-sampling before any other processing
	const maxRecordsToProcess = 100000 // Cap at 100K records for initial processing

	var recordsToProcess []parser.EnemeterRecord
	if len(records) > maxRecordsToProcess {
		fmt.Printf("Dataset very large (%d records). Pre-sampling to %d records...\n",
			len(records), maxRecordsToProcess)

		// Use simple even sampling for the pre-sampling phase
		step := float64(len(records)) / float64(maxRecordsToProcess)
		recordsToProcess = make([]parser.EnemeterRecord, 0, maxRecordsToProcess)

		for i := 0.0; i < float64(len(records)); i += step {
			idx := int(i)
			if idx >= len(records) {
				continue
			}
			recordsToProcess = append(recordsToProcess, records[idx])
		}

		// Make sure we include the last record for proper time range
		if len(recordsToProcess) > 0 && recordsToProcess[len(recordsToProcess)-1].Timestamp != records[len(records)-1].Timestamp {
			recordsToProcess = append(recordsToProcess, records[len(records)-1])
		}

		fmt.Printf("Pre-sampled to %d records\n", len(recordsToProcess))
	} else {
		recordsToProcess = records
	}

	// Set up chart options
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  g.chartTheme,
			Width:  g.chartWidth,
			Height: g.chartHeight,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    title,
			Subtitle: subtitle,
			Left:     "center",
			Top:      "5%",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(true),
			Top:  "15%",
			Left: "center",
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true), Trigger: "axis"}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show:  opts.Bool(true),
			Right: "20px",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show: opts.Bool(true),
				},
				DataZoom: &opts.ToolBoxFeatureDataZoom{
					Show: opts.Bool(true),
				},
				DataView: &opts.ToolBoxFeatureDataView{
					Show: opts.Bool(true),
				},
			},
		}),
		charts.WithGridOpts(opts.Grid{
			Top:          "20%",
			Right:        "5%",
			Bottom:       "5%",
			Left:         "5%",
			ContainLabel: opts.Bool(true),
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: fmt.Sprintf("Time (%s from start)", g.scaleOptions.TimeScale),
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: formatWithUnit(timeUnit),
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: fmt.Sprintf("Power (%s)", powerUnit),
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: formatWithUnit(powerUnit),
			},
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
	)

	// Prepare data - use simpler sequential processing to avoid parallelism issues
	fmt.Println("Processing data points...")
	xAxisData := make([]string, 0, len(recordsToProcess))
	powerData := make([]opts.LineData, 0, len(recordsToProcess))

	// Process records sequentially with a higher minimum time delta for extremely large datasets
	minTimeDelta := 1.0 // seconds
	if len(records) > 500000 {
		minTimeDelta = 5.0 // For very large datasets, increase minimum time between points
	}

	lastTimestampAdded := startTime.Add(-time.Second)

	// Use a more aggressive filtering approach for large datasets
	for i, record := range recordsToProcess {
		// Skip points with excessive intervals to avoid visual gaps
		if i > 0 && record.Timestamp.Sub(recordsToProcess[i-1].Timestamp) > time.Minute*5 {
			continue
		}

		// Skip points that are too close in time
		if record.Timestamp.Sub(lastTimestampAdded).Seconds() < minTimeDelta {
			continue
		}

		lastTimestampAdded = record.Timestamp

		adapter := NewAdapter(record)
		powerWatts := adapter.GetPowerWatts() * powerFactor

		// Calculate time from start with scaling factor
		timeFromStart := record.Timestamp.Sub(startTime).Seconds() * timeFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		xAxisData = append(xAxisData, timeRounded)
		powerData = append(powerData, opts.LineData{
			Value: powerWatts,
			Name:  record.Timestamp.Format(g.timeFormat),
		})

		// Print progress for very large datasets
		if len(records) > 100000 && i > 0 && i%10000 == 0 {
			fmt.Printf("Processed %d/%d points (%.1f%%)\n",
				i, len(recordsToProcess), float64(i)*100/float64(len(recordsToProcess)))
		}
	}

	fmt.Printf("Collected %d data points for chart\n", len(xAxisData))

	// Apply downsampling if needed and we have enough points
	if len(xAxisData) > g.maxDataPoints {
		fmt.Printf("Downsampling from %d to %d data points using '%s' algorithm...\n",
			len(xAxisData), g.maxDataPoints, g.samplingMode)

		// Convert string times back to Time objects for downsampling
		times := make([]time.Time, 0, len(xAxisData))
		values := make([]float64, 0, len(xAxisData))

		// Convert with careful error checking
		for i := range xAxisData {
			if i >= len(powerData) {
				continue // Avoid index out of bounds
			}

			seconds, err := strconv.ParseFloat(xAxisData[i], 64)
			if err != nil {
				continue
			}

			// Safe type assertion
			powerValue, ok := powerData[i].Value.(float64)
			if !ok {
				continue
			}

			times = append(times, startTime.Add(time.Duration(seconds/timeFactor*float64(time.Second))))
			values = append(values, powerValue)
		}

		// Only downsample if we have data
		if len(times) > 0 && len(values) > 0 {
			sampledTimes, sampledValues := g.downsampleData(times, values, g.maxDataPoints)

			// Convert back to chart format
			xAxisData = make([]string, len(sampledTimes))
			powerData = make([]opts.LineData, len(sampledValues))

			for i := range sampledTimes {
				timeFromStart := sampledTimes[i].Sub(startTime).Seconds() * timeFactor
				xAxisData[i] = fmt.Sprintf("%.2f", timeFromStart)
				powerData[i] = opts.LineData{
					Value: sampledValues[i],
					Name:  sampledTimes[i].Format(g.timeFormat),
				}
			}
		}
	}

	// Check if we have any data points
	if len(xAxisData) == 0 || len(powerData) == 0 {
		return "", fmt.Errorf("no valid data points to generate chart")
	}

	fmt.Printf("Final chart will have %d data points\n", len(xAxisData))

	// Add data to chart
	line.SetXAxis(xAxisData).
		AddSeries("Power", powerData).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				Smooth: opts.Bool(true),
			}),
			charts.WithAreaStyleOpts(opts.AreaStyle{
				Opacity: 0.2,
			}),
			charts.WithMarkLineNameTypeItemOpts(opts.MarkLineNameTypeItem{
				Name: "Average",
				Type: "average",
			}),
			charts.WithMarkPointNameTypeItemOpts(opts.MarkPointNameTypeItem{
				Name: "Max",
				Type: "max",
			}),
		)

	// Create output file
	fmt.Println("Creating output file...")
	filename := filepath.Join(g.outputDir, "power_chart.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Render the chart
	fmt.Println("Rendering chart to file...")
	if err := line.Render(f); err != nil {
		return "", err
	}

	fmt.Println("Chart generation completed successfully!")
	return filename, nil
}

// Add the generateVoltageChart function
func (g *GraphicsGenerator) generateVoltageChart(records []parser.EnemeterRecord) (string, error) {
	// Create a line chart
	line := charts.NewLine()

	// Get scale factors for time and voltage
	timeFactor, timeUnit := g.getTimeScaleFactor()
	voltageFactor, voltageUnit := g.getVoltageScaleFactor()

	// Get start time for reference
	startTime := records[0].Timestamp
	endTime := records[len(records)-1].Timestamp
	duration := endTime.Sub(startTime)

	// Configure chart
	title := "Voltage Measurements"
	subtitle := fmt.Sprintf("From: %s | Duration: %s",
		startTime.Format("2006-01-02 15:04:05"),
		duration.Round(time.Second).String())

	// Add progress reporting for large datasets
	fmt.Printf("Preparing data for voltage chart (%d records)...\n", len(records))

	// For extremely large datasets, do pre-sampling
	const maxRecordsToProcess = 100000
	var recordsToProcess []parser.EnemeterRecord

	if len(records) > maxRecordsToProcess {
		fmt.Printf("Dataset very large (%d records). Pre-sampling to %d records...\n",
			len(records), maxRecordsToProcess)

		step := float64(len(records)) / float64(maxRecordsToProcess)
		recordsToProcess = make([]parser.EnemeterRecord, 0, maxRecordsToProcess)

		for i := 0.0; i < float64(len(records)); i += step {
			idx := int(i)
			if idx >= len(records) {
				continue
			}
			recordsToProcess = append(recordsToProcess, records[idx])
		}

		if len(recordsToProcess) > 0 && recordsToProcess[len(recordsToProcess)-1].Timestamp != records[len(records)-1].Timestamp {
			recordsToProcess = append(recordsToProcess, records[len(records)-1])
		}

		fmt.Printf("Pre-sampled to %d records\n", len(recordsToProcess))
	} else {
		recordsToProcess = records
	}

	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  g.chartTheme,
			Width:  g.chartWidth,
			Height: g.chartHeight,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    title,
			Subtitle: subtitle,
			Left:     "center",
			Top:      "5%",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(true),
			Top:  "15%",
			Left: "center",
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show:  opts.Bool(true),
			Right: "20px",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show: opts.Bool(true),
				},
				DataZoom: &opts.ToolBoxFeatureDataZoom{
					Show: opts.Bool(true),
				},
			},
		}),
		charts.WithGridOpts(opts.Grid{
			Top:          "20%",
			Right:        "5%",
			Bottom:       "5%",
			Left:         "5%",
			ContainLabel: opts.Bool(true),
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: fmt.Sprintf("Time (%s from start)", g.scaleOptions.TimeScale),
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: formatWithUnit(timeUnit),
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: fmt.Sprintf("Voltage (%s)", voltageUnit),
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: formatWithUnit(voltageUnit),
			},
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
	)

	// Prepare data for chart
	fmt.Println("Processing voltage data points...")
	xAxisData := make([]string, 0, len(recordsToProcess))
	voltageData := make([]opts.LineData, 0, len(recordsToProcess))

	// Process with higher time delta for large datasets
	minTimeDelta := 1.0 // seconds
	if len(records) > 500000 {
		minTimeDelta = 5.0 // For very large datasets, increase minimum time between points
	}

	lastTimestampAdded := startTime.Add(-time.Second) // Ensure first point is always added
	reportInterval := len(recordsToProcess) / 10      // Report progress 10 times
	if reportInterval < 1 {
		reportInterval = 1
	}

	// Use raw microvolts for data and apply scaling factor
	for i, record := range recordsToProcess {
		// Skip points with excessive intervals to avoid visual gaps
		if i > 0 && record.Timestamp.Sub(recordsToProcess[i-1].Timestamp) > time.Minute*5 {
			continue
		}

		// Skip if less than minTimeDelta since last added point
		if record.Timestamp.Sub(lastTimestampAdded).Seconds() < minTimeDelta && i > 0 {
			continue
		}
		lastTimestampAdded = record.Timestamp

		adapter := NewAdapter(record)
		voltage := float64(adapter.GetVoltage()) * voltageFactor // Apply voltage scaling

		// Calculate time from start with scaling factor
		timeFromStart := record.Timestamp.Sub(startTime).Seconds() * timeFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		xAxisData = append(xAxisData, timeRounded)
		voltageData = append(voltageData, opts.LineData{
			Value: voltage,
			Name:  record.Timestamp.Format(g.timeFormat), // Keep absolute time in tooltip
		})

		// Report progress for large datasets
		if len(recordsToProcess) > 10000 && i > 0 && i%reportInterval == 0 {
			fmt.Printf("Processed %d/%d voltage points (%.1f%%)\n",
				i, len(recordsToProcess), float64(i)*100/float64(len(recordsToProcess)))
		}
	}

	// Apply downsampling if needed
	if len(xAxisData) > g.maxDataPoints {
		fmt.Printf("Downsampling from %d to %d data points using '%s' algorithm...\n",
			len(xAxisData), g.maxDataPoints, g.samplingMode)

		// Same downsampling logic as in power chart
		times := make([]time.Time, 0, len(xAxisData))
		values := make([]float64, 0, len(voltageData))

		for i := range xAxisData {
			if i >= len(voltageData) {
				continue
			}

			seconds, err := strconv.ParseFloat(xAxisData[i], 64)
			if err != nil {
				continue
			}

			voltageValue, ok := voltageData[i].Value.(float64)
			if !ok {
				continue
			}

			times = append(times, startTime.Add(time.Duration(seconds/timeFactor*float64(time.Second))))
			values = append(values, voltageValue)
		}

		// Only downsample if we have data
		if len(times) > 0 && len(values) > 0 {
			sampledTimes, sampledValues := g.downsampleData(times, values, g.maxDataPoints)

			// Convert back to chart format
			xAxisData = make([]string, len(sampledTimes))
			voltageData = make([]opts.LineData, len(sampledValues))

			for i := range sampledTimes {
				timeFromStart := sampledTimes[i].Sub(startTime).Seconds() * timeFactor
				xAxisData[i] = fmt.Sprintf("%.2f", timeFromStart)
				voltageData[i] = opts.LineData{
					Value: sampledValues[i],
					Name:  sampledTimes[i].Format(g.timeFormat),
				}
			}
		}
	}

	fmt.Printf("Final voltage chart will have %d data points\n", len(xAxisData))

	// Add data to chart
	line.SetXAxis(xAxisData).
		AddSeries(fmt.Sprintf("Voltage (%s)", voltageUnit), voltageData).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				Smooth: opts.Bool(true),
			}),
			charts.WithMarkLineNameTypeItemOpts(opts.MarkLineNameTypeItem{
				Name: "Average",
				Type: "average",
			}),
		)

	// Create output file
	fmt.Println("Creating voltage chart file...")
	filename := filepath.Join(g.outputDir, "voltage_chart.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Render the chart
	fmt.Println("Rendering voltage chart to file...")
	if err := line.Render(f); err != nil {
		return "", err
	}

	fmt.Println("Voltage chart generation completed successfully!")
	return filename, nil
}

// Add the generateCurrentChart function
func (g *GraphicsGenerator) generateCurrentChart(records []parser.EnemeterRecord) (string, error) {
	// Create a line chart
	line := charts.NewLine()

	// Get scale factors for time and current
	timeFactor, timeUnit := g.getTimeScaleFactor()
	currentFactor, currentUnit := g.getCurrentScaleFactor()

	// Get start time for reference
	startTime := records[0].Timestamp
	endTime := records[len(records)-1].Timestamp
	duration := endTime.Sub(startTime)

	// Configure chart
	title := "Current Measurements"
	subtitle := fmt.Sprintf("From: %s | Duration: %s",
		startTime.Format("2006-01-02 15:04:05"),
		duration.Round(time.Second).String())

	// Add progress reporting for large datasets
	fmt.Printf("Preparing data for current chart (%d records)...\n", len(records))

	// For extremely large datasets, do pre-sampling
	const maxRecordsToProcess = 100000
	var recordsToProcess []parser.EnemeterRecord

	if len(records) > maxRecordsToProcess {
		fmt.Printf("Dataset very large (%d records). Pre-sampling to %d records...\n",
			len(records), maxRecordsToProcess)

		step := float64(len(records)) / float64(maxRecordsToProcess)
		recordsToProcess = make([]parser.EnemeterRecord, 0, maxRecordsToProcess)

		for i := 0.0; i < float64(len(records)); i += step {
			idx := int(i)
			if idx >= len(records) {
				continue
			}
			recordsToProcess = append(recordsToProcess, records[idx])
		}

		if len(recordsToProcess) > 0 && recordsToProcess[len(recordsToProcess)-1].Timestamp != records[len(records)-1].Timestamp {
			recordsToProcess = append(recordsToProcess, records[len(records)-1])
		}

		fmt.Printf("Pre-sampled to %d records\n", len(recordsToProcess))
	} else {
		recordsToProcess = records
	}

	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  g.chartTheme,
			Width:  g.chartWidth,
			Height: g.chartHeight,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    title,
			Subtitle: subtitle,
			Left:     "center",
			Top:      "5%",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(true),
			Top:  "15%",
			Left: "center",
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show:  opts.Bool(true),
			Right: "20px",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show: opts.Bool(true),
				},
				DataZoom: &opts.ToolBoxFeatureDataZoom{
					Show: opts.Bool(true),
				},
			},
		}),
		charts.WithGridOpts(opts.Grid{
			Top:          "20%",
			Right:        "5%",
			Bottom:       "5%",
			Left:         "5%",
			ContainLabel: opts.Bool(true),
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: fmt.Sprintf("Time (%s from start)", g.scaleOptions.TimeScale),
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: formatWithUnit(timeUnit),
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: fmt.Sprintf("Current (%s)", currentUnit),
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: formatWithUnit(currentUnit),
			},
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
	)

	// Prepare data for chart
	fmt.Println("Processing current data points...")
	xAxisData := make([]string, 0, len(recordsToProcess))
	currentData := make([]opts.LineData, 0, len(recordsToProcess))

	minTimeDelta := 1.0 // seconds
	if len(records) > 500000 {
		minTimeDelta = 5.0 // For very large datasets, increase minimum time between points
	}

	lastTimestampAdded := startTime.Add(-time.Second) // Ensure first point is always added
	reportInterval := len(recordsToProcess) / 10      // Report progress 10 times
	if reportInterval < 1 {
		reportInterval = 1
	}

	for i, record := range recordsToProcess {
		// Skip points with excessive intervals to avoid visual gaps
		if i > 0 && record.Timestamp.Sub(recordsToProcess[i-1].Timestamp) > time.Minute*5 {
			continue
		}

		// Skip if less than minTimeDelta since last added point
		if record.Timestamp.Sub(lastTimestampAdded).Seconds() < minTimeDelta && i > 0 {
			continue
		}
		lastTimestampAdded = record.Timestamp

		adapter := NewAdapter(record)
		current := float64(adapter.GetCurrent()) * currentFactor // Apply current scaling

		// Calculate time from start with scaling factor
		timeFromStart := record.Timestamp.Sub(startTime).Seconds() * timeFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		xAxisData = append(xAxisData, timeRounded)
		currentData = append(currentData, opts.LineData{
			Value: current,
			Name:  record.Timestamp.Format(g.timeFormat), // Keep absolute time in tooltip
		})

		// Report progress for large datasets
		if len(recordsToProcess) > 10000 && i > 0 && i%reportInterval == 0 {
			fmt.Printf("Processed %d/%d current points (%.1f%%)\n",
				i, len(recordsToProcess), float64(i)*100/float64(len(recordsToProcess)))
		}
	}

	// Apply downsampling if needed
	if len(xAxisData) > g.maxDataPoints {
		fmt.Printf("Downsampling from %d to %d data points using '%s' algorithm...\n",
			len(xAxisData), g.maxDataPoints, g.samplingMode)

		// Same downsampling logic as in other charts
		times := make([]time.Time, 0, len(xAxisData))
		values := make([]float64, 0, len(currentData))

		for i := range xAxisData {
			if i >= len(currentData) {
				continue
			}

			seconds, err := strconv.ParseFloat(xAxisData[i], 64)
			if err != nil {
				continue
			}

			currentValue, ok := currentData[i].Value.(float64)
			if !ok {
				continue
			}

			times = append(times, startTime.Add(time.Duration(seconds/timeFactor*float64(time.Second))))
			values = append(values, currentValue)
		}

		// Only downsample if we have data
		if len(times) > 0 && len(values) > 0 {
			sampledTimes, sampledValues := g.downsampleData(times, values, g.maxDataPoints)

			// Convert back to chart format
			xAxisData = make([]string, len(sampledTimes))
			currentData = make([]opts.LineData, len(sampledValues))

			for i := range sampledTimes {
				timeFromStart := sampledTimes[i].Sub(startTime).Seconds() * timeFactor
				xAxisData[i] = fmt.Sprintf("%.2f", timeFromStart)
				currentData[i] = opts.LineData{
					Value: sampledValues[i],
					Name:  sampledTimes[i].Format(g.timeFormat),
				}
			}
		}
	}

	fmt.Printf("Final current chart will have %d data points\n", len(xAxisData))

	// Add data to chart
	line.SetXAxis(xAxisData).
		AddSeries(fmt.Sprintf("Current (%s)", currentUnit), currentData).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				Smooth: opts.Bool(true),
			}),
			charts.WithMarkLineNameTypeItemOpts(opts.MarkLineNameTypeItem{
				Name: "Average",
				Type: "average",
			}),
		)

	// Create output file
	fmt.Println("Creating current chart file...")
	filename := filepath.Join(g.outputDir, "current_chart.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Render the chart
	fmt.Println("Rendering current chart to file...")
	if err := line.Render(f); err != nil {
		return "", err
	}

	fmt.Println("Current chart generation completed successfully!")
	return filename, nil
}

// Add the generateTemperatureChart function
func (g *GraphicsGenerator) generateTemperatureChart(records []parser.EnemeterRecord) (string, error) {
	// Create a line chart
	line := charts.NewLine()

	// Get scale factors for time and temperature
	timeFactor, timeUnit := g.getTimeScaleFactor()
	tempFactor, tempUnit := g.getTempScaleFactor()

	// Get start time for reference
	startTime := records[0].Timestamp
	endTime := records[len(records)-1].Timestamp
	duration := endTime.Sub(startTime)

	// Configure chart
	title := "Temperature Over Time"
	subtitle := fmt.Sprintf("From: %s | Duration: %s",
		startTime.Format("2006-01-02 15:04:05"),
		duration.Round(time.Second).String())

	// Add progress reporting for large datasets
	fmt.Printf("Preparing data for temperature chart (%d records)...\n", len(records))

	// For extremely large datasets, do pre-sampling
	const maxRecordsToProcess = 100000
	var recordsToProcess []parser.EnemeterRecord

	if len(records) > maxRecordsToProcess {
		fmt.Printf("Dataset very large (%d records). Pre-sampling to %d records...\n",
			len(records), maxRecordsToProcess)

		step := float64(len(records)) / float64(maxRecordsToProcess)
		recordsToProcess = make([]parser.EnemeterRecord, 0, maxRecordsToProcess)

		for i := 0.0; i < float64(len(records)); i += step {
			idx := int(i)
			if idx >= len(records) {
				continue
			}
			recordsToProcess = append(recordsToProcess, records[idx])
		}

		if len(recordsToProcess) > 0 && recordsToProcess[len(recordsToProcess)-1].Timestamp != records[len(records)-1].Timestamp {
			recordsToProcess = append(recordsToProcess, records[len(records)-1])
		}

		fmt.Printf("Pre-sampled to %d records\n", len(recordsToProcess))
	} else {
		recordsToProcess = records
	}

	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  g.chartTheme,
			Width:  g.chartWidth,
			Height: g.chartHeight,
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    title,
			Subtitle: subtitle,
			Left:     "center",
			Top:      "5%",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(true),
			Top:  "15%",
			Left: "center",
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
		charts.WithToolboxOpts(opts.Toolbox{
			Show:  opts.Bool(true),
			Right: "20px",
			Feature: &opts.ToolBoxFeature{
				SaveAsImage: &opts.ToolBoxFeatureSaveAsImage{
					Show: opts.Bool(true),
				},
				DataZoom: &opts.ToolBoxFeatureDataZoom{
					Show: opts.Bool(true),
				},
			},
		}),
		charts.WithGridOpts(opts.Grid{
			Top:          "20%",
			Right:        "5%",
			Bottom:       "5%",
			Left:         "5%",
			ContainLabel: opts.Bool(true),
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: fmt.Sprintf("Time (%s from start)", g.scaleOptions.TimeScale),
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: formatWithUnit(timeUnit),
			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: fmt.Sprintf("Temperature (%s)", tempUnit),
			AxisLabel: &opts.AxisLabel{
				Show:      opts.Bool(true),
				Formatter: formatWithUnit(tempUnit),
			},
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
	)

	// Prepare data for chart
	fmt.Println("Processing temperature data points...")
	xAxisData := make([]string, 0, len(recordsToProcess))
	tempData := make([]opts.LineData, 0, len(recordsToProcess))

	minTimeDelta := 1.0 // seconds
	if len(records) > 500000 {
		minTimeDelta = 5.0 // For very large datasets, increase minimum time between points
	}

	lastTimestampAdded := startTime.Add(-time.Second) // Ensure first point is always added
	reportInterval := len(recordsToProcess) / 10      // Report progress 10 times
	if reportInterval < 1 {
		reportInterval = 1
	}

	for i, record := range recordsToProcess {
		// Skip points with excessive intervals to avoid visual gaps
		if i > 0 && record.Timestamp.Sub(recordsToProcess[i-1].Timestamp) > time.Minute*5 {
			continue
		}

		// Skip if less than minTimeDelta since last added point
		if record.Timestamp.Sub(lastTimestampAdded).Seconds() < minTimeDelta && i > 0 {
			continue
		}
		lastTimestampAdded = record.Timestamp

		adapter := NewAdapter(record)
		temp := float64(adapter.GetTemperature()) * tempFactor // Apply temperature scaling

		// Calculate time from start with scaling factor
		timeFromStart := record.Timestamp.Sub(startTime).Seconds() * timeFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		xAxisData = append(xAxisData, timeRounded)
		tempData = append(tempData, opts.LineData{
			Value: temp,
			Name:  record.Timestamp.Format(g.timeFormat), // Keep absolute time in tooltip
		})

		// Report progress for large datasets
		if len(recordsToProcess) > 10000 && i > 0 && i%reportInterval == 0 {
			fmt.Printf("Processed %d/%d temperature points (%.1f%%)\n",
				i, len(recordsToProcess), float64(i)*100/float64(len(recordsToProcess)))
		}
	}

	// Apply downsampling if needed
	if len(xAxisData) > g.maxDataPoints {
		fmt.Printf("Downsampling from %d to %d data points using '%s' algorithm...\n",
			len(xAxisData), g.maxDataPoints, g.samplingMode)

		// Same downsampling logic as in other charts
		times := make([]time.Time, 0, len(xAxisData))
		values := make([]float64, 0, len(tempData))

		for i := range xAxisData {
			if i >= len(tempData) {
				continue
			}

			seconds, err := strconv.ParseFloat(xAxisData[i], 64)
			if err != nil {
				continue
			}

			tempValue, ok := tempData[i].Value.(float64)
			if !ok {
				continue
			}

			times = append(times, startTime.Add(time.Duration(seconds/timeFactor*float64(time.Second))))
			values = append(values, tempValue)
		}

		// Only downsample if we have data
		if len(times) > 0 && len(values) > 0 {
			sampledTimes, sampledValues := g.downsampleData(times, values, g.maxDataPoints)

			// Convert back to chart format
			xAxisData = make([]string, len(sampledTimes))
			tempData = make([]opts.LineData, len(sampledValues))

			for i := range sampledTimes {
				timeFromStart := sampledTimes[i].Sub(startTime).Seconds() * timeFactor
				xAxisData[i] = fmt.Sprintf("%.2f", timeFromStart)
				tempData[i] = opts.LineData{
					Value: sampledValues[i],
					Name:  sampledTimes[i].Format(g.timeFormat),
				}
			}
		}
	}

	fmt.Printf("Final temperature chart will have %d data points\n", len(xAxisData))

	// Add data to chart
	line.SetXAxis(xAxisData).
		AddSeries(fmt.Sprintf("Temperature (%s)", tempUnit), tempData).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				Smooth: opts.Bool(true),
			}),
			charts.WithAreaStyleOpts(opts.AreaStyle{
				Opacity: 0.1,
			}),
			charts.WithMarkLineNameTypeItemOpts(opts.MarkLineNameTypeItem{
				Name: "Average",
				Type: "average",
			}),
		)

	// Create output file
	fmt.Println("Creating temperature chart file...")
	filename := filepath.Join(g.outputDir, "temperature_chart.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Render the chart
	fmt.Println("Rendering temperature chart to file...")
	if err := line.Render(f); err != nil {
		return "", err
	}

	fmt.Println("Temperature chart generation completed successfully!")
	return filename, nil
}
