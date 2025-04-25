package viz

import (
	"enemeter-data-processing/internal/metrics"
	"enemeter-data-processing/internal/parser"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
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

	// Report initial memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Initial memory usage: %.2f MB\n", float64(m.Alloc)/1024/1024)

	// Sort records by timestamp - using a more efficient sort algorithm
	fmt.Printf("Sorting %d records by timestamp...\n", len(g.records))
	sortedRecords := make([]parser.EnemeterRecord, len(g.records))
	copy(sortedRecords, g.records)

	// Use Go's built-in sort instead of bubble sort for better performance
	sort.Slice(sortedRecords, func(i, j int) bool {
		return sortedRecords[i].Timestamp.Before(sortedRecords[j].Timestamp)
	})

	fmt.Println("Sorting complete.")

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

	// Report final memory usage
	runtime.ReadMemStats(&m)
	fmt.Printf("Final memory usage: %.2f MB\n", float64(m.Alloc)/1024/1024)

	if err != nil {
		return "", err
	}

	return filename, nil
}

// Add a chunkedPreprocess helper function that processes records in parallel chunks
func chunkedPreprocess(
	records []parser.EnemeterRecord,
	startTime time.Time,
	processFunc func(record parser.EnemeterRecord, timeFromStart float64) (string, interface{}, bool),
	timeFactor float64,
	progressName string,
) ([]string, []opts.LineData) {
	// Calculate number of workers based on CPU count
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // Limit to 8 workers to avoid excessive memory usage
	}

	// Don't use more workers than records
	if numWorkers > len(records) {
		numWorkers = len(records)
	}

	chunkSize := len(records) / numWorkers
	if chunkSize < 1 {
		chunkSize = 1
	}

	fmt.Printf("Processing %s data using %d parallel workers with chunk size %d...\n",
		progressName, numWorkers, chunkSize)

	// Set up channels for result collection
	type resultItem struct {
		xValue string
		yValue interface{}
		index  int
	}

	resultChan := make(chan resultItem, len(records))
	var wg sync.WaitGroup

	// Set up progress tracking
	var processedCount int32
	progressInterval := len(records) / 20 // Report progress ~20 times
	if progressInterval < 1 {
		progressInterval = 1
	}

	// Launch progress reporter
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				processed := atomic.LoadInt32(&processedCount)
				fmt.Printf("  %s: Processed %d/%d records (%.1f%%)\n",
					progressName,
					processed,
					len(records),
					float64(processed)*100.0/float64(len(records)))
			case <-done:
				return
			}
		}
	}()

	// Process data in parallel chunks
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			start := workerID * chunkSize
			end := (workerID + 1) * chunkSize
			if workerID == numWorkers-1 {
				end = len(records)
			}

			// Track last timestamp to avoid adding too many points
			lastTimestampAdded := time.Time{}
			minTimeDelta := 0.0
			if len(records) > 100000 {
				minTimeDelta = 2.0
			} else if len(records) > 500000 {
				minTimeDelta = 5.0
			}

			for i := start; i < end; i++ {
				record := records[i]

				// Skip points that are too close in time if we have many records
				if !lastTimestampAdded.IsZero() &&
					minTimeDelta > 0 &&
					record.Timestamp.Sub(lastTimestampAdded).Seconds() < minTimeDelta {
					continue
				}

				// Calculate time from start with scaling factor
				timeFromStart := record.Timestamp.Sub(startTime).Seconds() * timeFactor

				// Call the provided processing function
				xValue, yValue, shouldAdd := processFunc(record, timeFromStart)

				if shouldAdd {
					lastTimestampAdded = record.Timestamp
					resultChan <- resultItem{xValue, yValue, i}
				}

				// Update progress every N records
				if i%progressInterval == 0 {
					atomic.AddInt32(&processedCount, int32(progressInterval))
				}
			}
		}(w)
	}

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
		close(done) // Stop the progress reporter
		fmt.Printf("  %s: Processing complete. Collected %d data points.\n", progressName, atomic.LoadInt32(&processedCount))
	}()

	// Collect and sort results
	resultMap := make(map[int]struct {
		xVal string
		yVal interface{}
	})
	for r := range resultChan {
		resultMap[r.index] = struct {
			xVal string
			yVal interface{}
		}{r.xValue, r.yValue}
	}

	// Convert to ordered slices
	xAxisData := make([]string, 0, len(resultMap))
	yData := make([]opts.LineData, 0, len(resultMap))

	// Create ordered list of indices
	indices := make([]int, 0, len(resultMap))
	for idx := range resultMap {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	// Create final data arrays in proper order
	for _, idx := range indices {
		res := resultMap[idx]
		xAxisData = append(xAxisData, res.xVal)
		yData = append(yData, opts.LineData{
			Value: res.yVal,
			Name:  records[idx].Timestamp.Format(time.RFC3339),
		})
	}

	return xAxisData, yData
}

// Fix generateEnergyChart by using the CalculateCumulativeEnergy function instead of GetEnergyJoules
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
	energyCalc := NewEnergyCalculation(records)
	timestamps, energyValues := energyCalc.CalculateCumulativeEnergy()

	// Debug the time range of our energy data
	if len(timestamps) > 1 {
		fmt.Printf("Energy data time range: %s to %s (%.2f hours)\n",
			timestamps[0].Format("2006-01-02 15:04:05"),
			timestamps[len(timestamps)-1].Format("2006-01-02 15:04:05"),
			timestamps[len(timestamps)-1].Sub(timestamps[0]).Hours())
	}

	// Process data points to create chart data
	fmt.Printf("Converting energy data to chart format (%d points)...\n", len(timestamps))

	// Process with a reasonable time delta to avoid too many points
	// Adjust based on total time range to ensure we don't sample too aggressively
	minTimeDelta := 0.0 // seconds
	totalHours := endTime.Sub(startTime).Hours()

	// Adjust sampling based on dataset size and time range
	if len(timestamps) > 100000 {
		// For large datasets, use ~0.05% of total time range as minimum delta
		minTimeDelta = totalHours * 3600 * 0.0005 // in seconds
	} else if len(timestamps) > 50000 {
		// For medium datasets, use ~0.02% of total time range
		minTimeDelta = totalHours * 3600 * 0.0002 // in seconds
	} else if totalHours > 12 {
		// For datasets spanning multiple hours, ensure at least 1000-2000 points
		minTimeDelta = totalHours * 3600 / 2000 // target ~2000 points max
	}

	fmt.Printf("Using minimum time delta of %.1f seconds between points (to limit chart density)\n", minTimeDelta)

	xAxisData := make([]string, 0, len(timestamps))
	energyData := make([]opts.LineData, 0, len(timestamps))

	lastTimestampAdded := time.Time{} // Zero value to ensure first point is added

	// Always add first and last points regardless of time delta
	if len(timestamps) > 0 {
		// Add first point
		firstTimeFromStart := timestamps[0].Sub(startTime).Seconds() * timeFactor
		xAxisData = append(xAxisData, fmt.Sprintf("%.2f", firstTimeFromStart))
		energyData = append(energyData, opts.LineData{
			Value: energyValues[0] * energyFactor,
			Name:  timestamps[0].Format(g.timeFormat),
		})
		lastTimestampAdded = timestamps[0]
	}

	// Process intermediate points with delta filtering
	for i := 1; i < len(timestamps)-1; i++ {
		// Skip points that are too close in time
		if minTimeDelta > 0 && timestamps[i].Sub(lastTimestampAdded).Seconds() < minTimeDelta {
			continue
		}

		// Calculate time from start with scaling factor
		timeFromStart := timestamps[i].Sub(startTime).Seconds() * timeFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		// Apply energy scaling factor
		scaledEnergy := energyValues[i] * energyFactor

		xAxisData = append(xAxisData, timeRounded)
		energyData = append(energyData, opts.LineData{
			Value: scaledEnergy,
			Name:  timestamps[i].Format(g.timeFormat),
		})

		lastTimestampAdded = timestamps[i]
	}

	// Always add the last point if we have points
	if len(timestamps) > 1 {
		lastTimeFromStart := timestamps[len(timestamps)-1].Sub(startTime).Seconds() * timeFactor
		xAxisData = append(xAxisData, fmt.Sprintf("%.2f", lastTimeFromStart))
		energyData = append(energyData, opts.LineData{
			Value: energyValues[len(energyValues)-1] * energyFactor,
			Name:  timestamps[len(timestamps)-1].Format(g.timeFormat),
		})
	}

	// Debug final chart data range
	if len(xAxisData) > 0 {
		firstX, _ := strconv.ParseFloat(xAxisData[0], 64)
		lastX, _ := strconv.ParseFloat(xAxisData[len(xAxisData)-1], 64)
		fmt.Printf("Chart X-axis range: %.2f to %.2f %s\n", firstX, lastX, timeUnit)
		durationInHours := (lastX - firstX) / timeFactor / 3600
		fmt.Printf("Chart time span: %.2f hours\n", durationInHours)
	}

	// Check if we have any data points
	if len(xAxisData) == 0 || len(energyData) == 0 {
		return "", fmt.Errorf("no valid data points to generate chart")
	}

	fmt.Printf("Final energy chart will have %d data points\n", len(xAxisData))

	// Add data to chart
	line.SetXAxis(xAxisData).
		AddSeries("Energy", energyData).
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
	filename := filepath.Join(g.outputDir, "energy_chart.html")
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

	fmt.Println("Energy chart generation completed successfully!")
	return filename, nil
}

// generatePowerChart creates a chart showing power consumption over time
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

	// Process data in parallel using the chunkedPreprocess helper
	processFunc := func(record parser.EnemeterRecord, timeFromStart float64) (string, interface{}, bool) {
		adapter := NewAdapter(record)
		powerWatts := adapter.GetPowerWatts() * powerFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		return timeRounded, powerWatts, true
	}

	xAxisData, powerData := chunkedPreprocess(
		records,
		startTime,
		processFunc,
		timeFactor,
		"Power chart",
	)

	// Apply downsampling if needed
	if len(xAxisData) > g.maxDataPoints {
		fmt.Printf("Downsampling from %d to %d data points using '%s' algorithm...\n",
			len(xAxisData), g.maxDataPoints, g.samplingMode)

		times := make([]time.Time, 0, len(xAxisData))
		values := make([]float64, 0, len(powerData))

		for i := range xAxisData {
			if i >= len(powerData) {
				continue
			}

			seconds, err := strconv.ParseFloat(xAxisData[i], 64)
			if err != nil {
				continue
			}

			powerValue, ok := powerData[i].Value.(float64)
			if !ok {
				continue
			}

			times = append(times, startTime.Add(time.Duration(seconds/timeFactor*float64(time.Second))))
			values = append(values, powerValue)
		}

		if len(times) > 0 && len(values) > 0 {
			sampledTimes, sampledValues := g.downsampleData(times, values, g.maxDataPoints)

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
	filename := filepath.Join(g.outputDir, "power_chart.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Render the chart
	if err := line.Render(f); err != nil {
		return "", err
	}

	return filename, nil
}

// generateVoltageChart creates a chart showing voltage over time
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

	// Set up chart options with title, subtitle, and axis labels
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

	// Process data in parallel using the chunkedPreprocess helper
	processFunc := func(record parser.EnemeterRecord, timeFromStart float64) (string, interface{}, bool) {
		adapter := NewAdapter(record)
		voltage := adapter.GetVoltageVolts() * voltageFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		return timeRounded, voltage, true
	}

	xAxisData, voltageData := chunkedPreprocess(
		records,
		startTime,
		processFunc,
		timeFactor,
		"Voltage chart",
	)

	// Apply downsampling if needed
	if len(xAxisData) > g.maxDataPoints {
		fmt.Printf("Downsampling from %d to %d data points using '%s' algorithm...\n",
			len(xAxisData), g.maxDataPoints, g.samplingMode)

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

		if len(times) > 0 && len(values) > 0 {
			sampledTimes, sampledValues := g.downsampleData(times, values, g.maxDataPoints)

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

	// Check if we have any data points
	if len(xAxisData) == 0 || len(voltageData) == 0 {
		return "", fmt.Errorf("no valid data points to generate chart")
	}

	// Add data to chart
	line.SetXAxis(xAxisData).
		AddSeries("Voltage", voltageData).
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
	filename := filepath.Join(g.outputDir, "voltage_chart.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Render the chart
	if err := line.Render(f); err != nil {
		return "", err
	}

	return filename, nil
}

// generateCurrentChart creates a chart showing current over time
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

	// Set up chart options with title, subtitle, and axis labels
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

	// Process data in parallel using the chunkedPreprocess helper
	processFunc := func(record parser.EnemeterRecord, timeFromStart float64) (string, interface{}, bool) {
		adapter := NewAdapter(record)
		current := adapter.GetCurrentAmps() * currentFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		return timeRounded, current, true
	}

	xAxisData, currentData := chunkedPreprocess(
		records,
		startTime,
		processFunc,
		timeFactor,
		"Current chart",
	)

	// Apply downsampling if needed
	if len(xAxisData) > g.maxDataPoints {
		fmt.Printf("Downsampling from %d to %d data points using '%s' algorithm...\n",
			len(xAxisData), g.maxDataPoints, g.samplingMode)

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

		if len(times) > 0 && len(values) > 0 {
			sampledTimes, sampledValues := g.downsampleData(times, values, g.maxDataPoints)

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

	// Check if we have any data points
	if len(xAxisData) == 0 || len(currentData) == 0 {
		return "", fmt.Errorf("no valid data points to generate chart")
	}

	// Add data to chart
	line.SetXAxis(xAxisData).
		AddSeries("Current", currentData).
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
	filename := filepath.Join(g.outputDir, "current_chart.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Render the chart
	if err := line.Render(f); err != nil {
		return "", err
	}

	return filename, nil
}

// generateTemperatureChart creates a chart showing temperature over time
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

	// Set up chart options with title, subtitle, and axis labels
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

	// Process data in parallel using the chunkedPreprocess helper
	processFunc := func(record parser.EnemeterRecord, timeFromStart float64) (string, interface{}, bool) {
		adapter := NewAdapter(record)
		temp := adapter.GetTemperatureCelsius() * tempFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		return timeRounded, temp, true
	}

	xAxisData, tempData := chunkedPreprocess(
		records,
		startTime,
		processFunc,
		timeFactor,
		"Temperature chart",
	)

	// Apply downsampling if needed
	if len(xAxisData) > g.maxDataPoints {
		fmt.Printf("Downsampling from %d to %d data points using '%s' algorithm...\n",
			len(xAxisData), g.maxDataPoints, g.samplingMode)

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

		if len(times) > 0 && len(values) > 0 {
			sampledTimes, sampledValues := g.downsampleData(times, values, g.maxDataPoints)

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

	// Check if we have any data points
	if len(xAxisData) == 0 || len(tempData) == 0 {
		return "", fmt.Errorf("no valid data points to generate chart")
	}

	// Add data to chart
	line.SetXAxis(xAxisData).
		AddSeries("Temperature", tempData).
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
	filename := filepath.Join(g.outputDir, "temperature_chart.html")
	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Render the chart
	if err := line.Render(f); err != nil {
		return "", err
	}

	return filename, nil
}
