package viz

import (
	"enemeter-data-processing/internal/parser"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

// FastChartGenerator provides simplified chart generation for very large datasets
// This sacrifices some details for performance
type FastChartGenerator struct {
	records     []parser.EnemeterRecord
	outputDir   string
	maxPoints   int
	title       string
	yAxisName   string
	valueUnit   string
	timeFormat  string
	timeFactor  float64
	valueFactor float64
}

// NewFastChartGenerator creates a new generator for simplified charts
func NewFastChartGenerator(records []parser.EnemeterRecord, outputDir string, maxPoints int) *FastChartGenerator {
	return &FastChartGenerator{
		records:     records,
		outputDir:   outputDir,
		maxPoints:   maxPoints,
		timeFormat:  "15:04:05",
		timeFactor:  1.0, // seconds by default
		valueFactor: 1.0, // no scaling by default
	}
}

// WithTitle sets the chart title
func (f *FastChartGenerator) WithTitle(title string) *FastChartGenerator {
	f.title = title
	return f
}

// WithAxisNames sets the axis names
func (f *FastChartGenerator) WithAxisNames(yName string) *FastChartGenerator {
	f.yAxisName = yName
	return f
}

// WithScaleFactors sets scale factors for time and values
func (f *FastChartGenerator) WithScaleFactors(timeFactor float64, valueFactor float64, valueUnit string) *FastChartGenerator {
	f.timeFactor = timeFactor
	f.valueFactor = valueFactor
	f.valueUnit = valueUnit
	return f
}

// GenerateChart creates a simplified chart with minimal processing
func (f *FastChartGenerator) GenerateChart(valueExtractor func(parser.EnemeterRecord) float64, filename string) (string, error) {
	if len(f.records) == 0 {
		return "", fmt.Errorf("no data to generate chart")
	}

	// Ensure directory exists
	if err := os.MkdirAll(f.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %v", err)
	}

	// Create chart
	line := charts.NewLine()

	// Configure chart
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Theme:  "westeros",
			Width:  "900px",
			Height: "500px",
		}),
		charts.WithTitleOpts(opts.Title{
			Title: f.title,
			Left:  "center",
		}),
		charts.WithLegendOpts(opts.Legend{
			Show: opts.Bool(true),
			Top:  "5%",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show: opts.Bool(true),
		}),
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
		charts.WithXAxisOpts(opts.XAxis{
			Name: "Time (s)",
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: f.yAxisName,
		}),
		charts.WithDataZoomOpts(opts.DataZoom{
			Start:      0,
			End:        100,
			XAxisIndex: []int{0},
		}),
	)

	// Direct downsampling with even sampling
	step := 1.0
	if len(f.records) > f.maxPoints {
		step = float64(len(f.records)) / float64(f.maxPoints)
		fmt.Printf("Fast mode: Sampling at 1:%.1f ratio (every %.0f points)\n", step, step)
	}

	fmt.Println("Fast mode: Processing data points...")
	xAxisData := make([]string, 0, f.maxPoints)
	yData := make([]opts.LineData, 0, f.maxPoints)

	// Reference start time
	startTime := f.records[0].Timestamp

	// Progress reporting
	reportStep := len(f.records) / 10
	if reportStep < 1 {
		reportStep = 1
	}

	// Process data with simple even sampling
	for i := 0.0; i < float64(len(f.records)); i += step {
		idx := int(math.Floor(i))
		if idx >= len(f.records) {
			continue
		}

		record := f.records[idx]
		value := valueExtractor(record) * f.valueFactor

		// Calculate time in seconds from start
		timeFromStart := record.Timestamp.Sub(startTime).Seconds() * f.timeFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)

		xAxisData = append(xAxisData, timeRounded)
		yData = append(yData, opts.LineData{
			Value: value,
			Name:  record.Timestamp.Format(f.timeFormat),
		})

		// Report progress
		if idx > 0 && idx%reportStep == 0 {
			fmt.Printf("Fast mode: Processed %d/%d points (%.0f%%)\n",
				idx, len(f.records), float64(idx)*100/float64(len(f.records)))
		}
	}

	// Make sure we include the last point
	lastRecordIdx := len(f.records) - 1
	if lastRecordIdx > 0 {
		lastRecord := f.records[lastRecordIdx]
		timeFromStart := lastRecord.Timestamp.Sub(startTime).Seconds() * f.timeFactor
		timeRounded := fmt.Sprintf("%.2f", timeFromStart)
		value := valueExtractor(lastRecord) * f.valueFactor

		// Only add if it's not already in the data
		if len(xAxisData) > 0 && xAxisData[len(xAxisData)-1] != timeRounded {
			xAxisData = append(xAxisData, timeRounded)
			yData = append(yData, opts.LineData{
				Value: value,
				Name:  lastRecord.Timestamp.Format(f.timeFormat),
			})
		}
	}

	fmt.Printf("Fast mode: Final chart contains %d data points\n", len(xAxisData))

	// Add data to chart
	seriesName := f.yAxisName
	if f.valueUnit != "" {
		seriesName = fmt.Sprintf("%s (%s)", f.yAxisName, f.valueUnit)
	}

	line.SetXAxis(xAxisData).
		AddSeries(seriesName, yData).
		SetSeriesOptions(
			charts.WithLineChartOpts(opts.LineChart{
				Smooth: opts.Bool(false), // Disable smoothing for performance
			}),
		)

	// Create output file
	filePath := filepath.Join(f.outputDir, filename)
	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %v", err)
	}
	defer f.Close()

	// Render the chart
	if err := line.Render(f); err != nil {
		return "", fmt.Errorf("failed to render chart: %v", err)
	}

	return filePath, nil
}
