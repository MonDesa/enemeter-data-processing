# ENEMETER Data Processing Tool

A powerful command-line tool for efficiently processing and analyzing ENEMETER energy measurement data.

## Overview

This tool processes data from ENEMETER devices, which measure various electrical parameters such as voltage, current, and temperature. The tool is designed to handle large CSV files efficiently and provide meaningful energy metrics for analysis.

## Features

- Process large CSV files efficiently with streaming mode
- Filter data by time range, temperature, voltage, or current
- Sample data to reduce processing requirements
- Extract specific metrics for targeted analysis
- Export results in multiple formats (text, JSON, CSV)
- Calculate energy consumption metrics and statistics
- Generate interactive charts and visualizations
- Analyze battery charging/discharging patterns
- Evaluate solar panel contribution

## Installation

### Pre-compiled Binaries

The easiest way to install is to download pre-compiled binaries from the [GitHub Releases page](https://github.com/MonDesa/enemeter-data-processing/releases).

Choose the appropriate binary for your platform:
- `enemeter-data-processing-linux` for Linux
- `enemeter-data-processing.exe` for Windows 
- `enemeter-data-processing-mac` for macOS

### Building from source

If you prefer to build from source:

#### Prerequisites

- Go 1.23 or later

```bash
git clone https://github.com/MonDesa/enemeter-data-processing.git
cd enemeter-data-processing
make build
```

## Basic Usage

Process a CSV file with required parameters:

```bash
./enemeter-data-processing process --input=data/esp32.csv --start="2025-04-01 08:00:00"
```

The `--start` parameter specifies the starting time of the measurements and is required. 
**Important:** You must include both the date AND time of day in the format "YYYY-MM-DD HH:MM:SS".

Save results to a text file:

```bash
./enemeter-data-processing process --input=data/esp32.csv --start="2025-04-01 08:00:00" --output=esp32_report.txt
```

## Command Reference

The tool offers several commands:

- `process`: Process data and generate metrics reports
- `graphics`: Generate interactive charts from the data
- `help`: Display help information

## Command: process

### Required Parameters
- `--input=<path>`: Path to the input CSV file
- `--start=<time>`: Start time for measurements (format: YYYY-MM-DD HH:MM:SS) - must include time of day

### Optional Parameters
- `--output=<path>`: Path to save the output report
- `--format=<text|json|csv>`: Output format (default: text)

### Processing Options

- `--stream`: Use memory-efficient streaming mode for large files
- `--sample=<N>`: Process every Nth record (default: 1, process all records)
- `--max=<N>`: Maximum number of records to process (default: 0, no limit)

### Time Filtering Options

- `--start=<time>`: (Required) Start time for measurements (format: YYYY-MM-DD HH:MM:SS) - must include time of day
- `--end=<time>`: End time for filtering (format: YYYY-MM-DD HH:MM:SS)
- `--window=<duration>`: Time window to process (e.g., 1h, 30m, 24h)

### Data Filtering Options

- `--min-temp=<value>`: Minimum temperature threshold in millicelsius
- `--volt-min=<value>`: Minimum voltage threshold in microvolts
- `--volt-max=<value>`: Maximum voltage threshold in microvolts
- `--curr-min=<value>`: Minimum current threshold in nanoamperes
- `--curr-max=<value>`: Maximum current threshold in nanoamperes

### Metric Extraction

- `--metric=<name>`: Extract a specific metric (see below)

## Available Metrics

- `total_energy`: Total energy consumption in joules
- `average_power`: Average power in watts
- `peak_power`: Peak power in watts
- `temperature`: Temperature statistics
- `energy_by_hour`: Energy consumption by hour
- `voltage_stats`: Voltage statistics
- `current_stats`: Current statistics
- `battery_discharge`: Battery discharge statistics
- `solar_contribution`: Solar panel contribution

## Command: graphics

The `graphics` command generates interactive HTML charts from ENEMETER data. These charts offer visual insights into power consumption, energy usage, voltage, current, and temperature over time.

### Required Parameters
- `--input=<path>`: Path to the input CSV file
- `--start=<time>`: Start time for measurements (format: YYYY-MM-DD HH:MM:SS) - must include time of day
- `--type=<type>`: Type of chart to generate (power, energy, voltage, current, temperature)

### Optional Parameters
- `--output-dir=<path>`: Directory to save generated charts (default: charts)

### Scale Options
These options control the units displayed on chart axes:

- `--time-scale=<scale>`: Time (X-axis) scale (milliseconds, seconds, minutes, hours, days)
- `--power-scale=<scale>`: Power scale (microwatts, milliwatts, watts, kilowatts)
- `--energy-scale=<scale>`: Energy scale (microjoules, millijoules, joules, kilojoules, watt-hours, kilowatt-hours)
- `--voltage-scale=<scale>`: Voltage scale (microvolts, millivolts, volts)
- `--current-scale=<scale>`: Current scale (nanoamperes, microamperes, milliamperes, amperes)
- `--temp-scale=<scale>`: Temperature scale (millicelsius, celsius)

### Performance Options
These options help optimize chart generation for large datasets:

- `--max-points=<N>`: Maximum number of data points to plot (default: 2000)
- `--sampling=<method>`: Downsampling algorithm (even, adaptive, lttb)
- `--force`: Force chart generation even for large datasets with small scales

### Interactive Mode
If you don't specify certain required options (like chart type or scales), the tool will ask interactively.

### Chart Types

1. **Power Chart** (`power`)
   - Shows power consumption in watts (or selected unit) over time
   - Highlights average and peak power
   - Most useful for analyzing energy usage patterns

2. **Energy Chart** (`energy`)
   - Shows cumulative energy consumption
   - Useful for tracking total energy usage over time
   - Can help identify periods of high consumption

3. **Voltage Chart** (`voltage`)
   - Shows voltage measurements over time
   - Useful for monitoring battery levels or power supply quality
   - Can identify voltage drops or unstable power sources

4. **Current Chart** (`current`)
   - Shows current measurements over time
   - Helps identify power-hungry operations or components
   - Can be used to analyze device activity patterns

5. **Temperature Chart** (`temperature`)
   - Shows temperature readings over time
   - Useful for monitoring device operating conditions
   - Can help identify thermal issues or environmental effects

### Downsampling Algorithms

For large datasets, the tool offers three downsampling methods to maintain chart responsiveness:

1. **Even Sampling** (`even`)
   - Selects evenly spaced points from the dataset
   - Fastest but may miss important peaks or fluctuations
   - Best for datasets with consistent patterns

2. **Adaptive Sampling** (`adaptive`) - Default
   - Keeps more points in areas with high data variance
   - Preserves important features like peaks and valleys
   - Good balance between speed and visual quality

3. **LTTB Sampling** (`lttb` - Largest Triangle Three Buckets)
   - Optimized algorithm that preserves visual characteristics
   - Best visual representation of the original data
   - Slightly more computation-intensive but excellent results

### Examples

#### Basic Chart Generation

Generate a power consumption chart:
```bash
./enemeter-data-processing graphics --input=data/esp32.csv --start="2025-04-01 08:00:00" --type=power
```

Generate a voltage chart with custom scales:
```bash
./enemeter-data-processing graphics --input=data/esp32.csv --start="2025-04-01 08:00:00" --type=voltage --voltage-scale=millivolts --time-scale=minutes
```

Generate an energy chart with custom output directory:
```bash
./enemeter-data-processing graphics --input=data/esp32.csv --start="2025-04-01 08:00:00" --type=energy --output-dir=./my-charts
```

#### Performance Optimization Examples

Process a very large file with optimized settings:
```bash
./enemeter-data-processing graphics --input=big-data.csv --start="2025-04-01 08:00:00" --type=power --max-points=1000 --sampling=lttb
```

Use streaming mode for efficient memory usage:
```bash
./enemeter-data-processing graphics --input=big-data.csv --start="2025-04-01 08:00:00" --type=power --stream --sample=10
```

#### Advanced Chart Configuration

Generate a power chart with kilowatt units and hour-based time scale:
```bash
./enemeter-data-processing graphics --input=data/esp32.csv --start="2025-04-01 08:00:00" --type=power --power-scale=kilowatts --time-scale=hours
```

Generate an energy chart with kilowatt-hour units:
```bash
./enemeter-data-processing graphics --input=data/esp32.csv --start="2025-04-01 08:00:00" --type=energy --energy-scale=kilowatt-hours
```

### Best Practices for Chart Generation

1. **Choose appropriate scales** - Use scales that match your data range:
   - For short-duration measurements: milliseconds or seconds time scale
   - For long-duration measurements: minutes, hours, or days time scale
   - For low-power devices: milliwatts or microwatts
   - For high-power devices: watts or kilowatts

2. **Optimize for large datasets**:
   - Use `--max-points` to limit the number of points (1000-2000 is usually sufficient)
   - Try different sampling algorithms (`--sampling=lttb` usually gives best results)
   - Use `--stream` mode for very large files
   - Consider using larger time scales (minutes/hours instead of seconds)

3. **Avoid small scales with large datasets** unless necessary:
   - Using milliseconds time scale with days of data may create extremely large charts
   - Using microunits (microvolts, microwatts) with large datasets may impact performance
   - If you need small units with large datasets, use `--force` to override warnings

### Interactive Chart Features

The generated HTML charts include several interactive features:

- **Zoom**: Use the zoom tool or mouse wheel to zoom in on specific periods
- **Pan**: Click and drag to pan across the chart
- **Tooltips**: Hover over data points to see exact values
- **Data view**: View the underlying data in tabular format
- **Save as image**: Export the chart as a PNG image

### Chart Output

Charts are saved as HTML files in the specified output directory (default: `./charts/`):
- `power_chart.html`: Power consumption chart
- `energy_chart.html`: Cumulative energy usage chart  
- `voltage_chart.html`: Voltage measurements chart
- `current_chart.html`: Current measurements chart
- `temperature_chart.html`: Temperature readings chart

These HTML files can be opened in any web browser and don't require an internet connection or special software.

## Performance Optimization Guidelines

### Processing Large Datasets

The tool now includes several optimizations for handling large datasets efficiently:

1. **Parallel Processing**
   - For files larger than 100MB, the tool automatically uses parallel processing
   - Utilizes multiple CPU cores to speed up data processing
   - Adapts to available system resources for optimal performance
   - Processes data in chunks to balance memory usage and speed

2. **Memory Efficiency**
   - Streaming mode (`--stream`) processes data without loading the entire file into memory
   - Progress reporting shows real-time status for large file processing
   - Memory usage is continuously monitored to prevent out-of-memory errors
   - Automatic garbage collection at strategic points to optimize memory usage

3. **Adaptive Sampling**
   - For extremely large files, automatic pre-sampling reduces memory requirements
   - Intelligent downsampling preserves important data characteristics
   - Several downsampling algorithms available (even, adaptive, lttb)
   - Automatic detection of appropriate sampling rates based on file size

### Command-line Options for Performance

When working with large datasets (>100MB or >1M records), consider these options:

```bash
# Enable streaming mode for very large files
./enemeter-data-processing process --input=large-data.csv --start="2025-04-01 08:00:00" --stream

# Use sample rate for extremely large files
./enemeter-data-processing process --input=very-large-data.csv --start="2025-04-01 08:00:00" --stream --sample=10

# For chart generation with large files
./enemeter-data-processing graphics --input=large-data.csv --start="2025-04-01 08:00:00" --type=power --max-points=1000
```

### Automatic Optimizations

The tool now implements several automatic optimizations:

- **Auto-detection of large files**: Automatically suggests or enables streaming mode
- **Progress reporting**: Real-time feedback on processing large datasets
- **Parallel chunking**: Large files are processed in parallel chunks
- **Memory monitoring**: Continuous monitoring prevents out-of-memory errors
- **Timeout adjustment**: Chart generation timeouts are dynamically calculated based on data size
- **Error prevention**: Built-in safeguards against common issues like divide-by-zero errors

### Recommended Settings by File Size

| File Size | Records    | Recommended Settings                                   |
|-----------|------------|-------------------------------------------------------|
| <10MB     | <100K      | Default settings                                       |
| 10-100MB  | 100K-1M    | `--stream`, `--max-points=2000`                        |
| 100-500MB | 1M-5M      | `--stream`, `--sample=5`, `--max-points=1000`          |
| >500MB    | >5M        | `--stream`, `--sample=10`, `--max-points=1000`, `--sampling=lttb` |

### Advanced Performance Tuning

For the most demanding datasets or resource-constrained environments:

```bash
# Maximum performance settings for extremely large files
./enemeter-data-processing process --input=massive-data.csv --start="2025-04-01 08:00:00" \
  --stream --sample=20 --max-records=1000000

# Memory-optimized chart generation
./enemeter-data-processing graphics --input=massive-data.csv --start="2025-04-01 08:00:00" \
  --type=power --max-points=500 --sampling=even --time-scale=hours
```

### Monitoring Performance

The tool now provides detailed performance metrics during processing:

- Memory usage statistics
- Processing speed (records/second)
- Completion percentage
- Estimated time remaining (for very large files)
- Data point density metrics

These metrics help you determine if your chosen settings are appropriate for your dataset size and system capabilities.

### Interactive Mode for Chart Generation

When using the `graphics` command, the tool offers an interactive mode if you don't specify certain required options:

- If no chart type is specified, you'll be prompted to select one
- If no scale options are provided, you'll be prompted to choose appropriate scales
- Recommendations are made based on your dataset size and characteristics

Example interactive session:
```
./enemeter-data-processing graphics --input=data/data.csv --start="2025-04-01 08:00:00"

Select a chart type to generate:
1. Power
2. Energy
3. Voltage
4. Current
5. Temperature

Enter your choice (1-5): 1
Selected chart type: power

Select X-axis time scale:
1. Milliseconds
2. Seconds (default)
3. Minutes
4. Hours
5. Days

Enter your choice (1-5): 4
Selected X-axis time scale: hours

Select Y-axis power scale:
1. Microwatts
2. Milliwatts
3. Watts (default)
4. Kilowatts

Enter your choice (1-4): 
Selected Y-axis power scale: watts
```

This interactive mode makes the tool more user-friendly, especially for new users who may not be familiar with all available options.

## Examples

### Process Command Examples

Process a CSV file and display all metrics:

```bash
./enemeter-data-processing process --input=data/esp32.csv --start="2025-04-01 08:00:00"
```

### Memory-Efficient Processing for Large Files

Use streaming mode with sampling for very large files:

```bash
./enemeter-data-processing process --input=big-data.csv --start="2025-04-01 10:15:30" --stream --sample=10
```

This processes only every 10th record, reducing memory requirements.

### Time-Based Analysis

To analyze data from a specific time period:

```bash
# Analyze data between specific dates and times
./enemeter-data-processing process --input=esp32.csv --start="2025-04-01 08:00:00" --end="2025-04-02 17:30:00"

# Analyze data for a specific duration
./enemeter-data-processing process --input=esp32.csv --start="2025-04-01 12:45:00" --window=24h
```

### Extracting Specific Metrics

Get only temperature statistics in JSON format:

```bash
./enemeter-data-processing process --input=esp32.csv --start="2025-04-01 08:00:00" --metric=temperature --format=json
```

Extract hourly energy consumption in CSV format:

```bash
./enemeter-data-processing process --input=esp32.csv --start="2025-04-01 08:00:00" --metric=energy_by_hour --format=csv
```

### Filtering by Data Values

Process only records with voltage between specified values:

```bash
./enemeter-data-processing process --input=esp32.csv --start="2025-04-01 08:00:00" --volt-min=3500000 --volt-max=4200000
```

Process only records where temperature is above a certain threshold:

```bash
./enemeter-data-processing process --input=esp32.csv --start="2025-04-01 08:00:00" --min-temp=25000
```

## Output Examples

### Text Output (Default)
```
Total Energy: 12345.67 J
Average Power: 12.34 W
Peak Power: 56.78 W
Temperature: Min=25.0°C, Max=30.0°C, Avg=27.5°C
```

### JSON Output
```json
{
  "total_energy": 12345.67,
  "average_power": 12.34,
  "peak_power": 56.78,
  "temperature": {
    "min": 25.0,
    "max": 30.0,
    "avg": 27.5
  }
}
```

### CSV Output
```
Metric,Value
Total Energy,12345.67
Average Power,12.34
Peak Power,56.78
Temperature Min,25.0
Temperature Max,30.0
Temperature Avg,27.5
```

## Analyze CSV Tool

The package includes a complementary tool called `analyze-csv` that helps analyze CSV data structure and units before processing.

### Basic Usage

```bash
./analyze-csv --input=data/raw_data.csv --samples=20
```

### Available Options

- `--input=<path>`: Path to the CSV file to analyze (required)
- `--samples=<N>`: Number of sample rows to display (default: 10)
- `--full`: Display full row content (by default, long values are truncated)
- `--stats`: Generate statistical information about each column
- `--detect-units`: Try to automatically detect appropriate units for the data

### Examples

#### Basic Analysis

```bash
./analyze-csv --input=data/esp32.csv
```

This will show the first 10 rows and basic structure information.

#### Detailed Analysis

```bash
./analyze-csv --input=data/esp32.csv --samples=50 --stats
```

This will display 50 sample rows and include statistical analysis of each column.

#### Unit Detection

```bash
./analyze-csv --input=data/esp32.csv --detect-units
```

This will analyze the data and suggest appropriate units for temperature, voltage, and current values.

### Output Example

```
CSV ANALYZER RESULTS
===================
File: esp32.csv
Total rows: 15623
Sample size: 10

COLUMN STRUCTURE
--------------
Column 1: Appears to be time delta (milliseconds)
Column 2: Appears to be voltage measurements (microvolts)
Column 3: Appears to be current measurements (nanoamperes)
Column 4: Appears to be temperature (millicelsius)

SAMPLE DATA
----------
Row 1: 0052051, 3750000, -125000, 24500
Row 2: 0000047, 3750000, -125000, 24500
Row 3: 0000047, 3749500, -126000, 24700
...

UNIT SUGGESTIONS
--------------
Temperature: Values appear to be in millicelsius
             Consider using raw_value / 1000 as °C
Voltage: Values appear to be in microvolts
         Consider using raw_value / 1000000 as V
Current: Values appear to be in nanoamperes
         Consider using raw_value / 1000000000 as A
```

### When to Use Analyze CSV

Use `analyze-csv` when:
- You're working with a new data source and need to understand its structure
- You want to verify units before processing
- You need to determine appropriate filter values for voltage, current, or temperature
- You suspect data format issues that might affect processing results

After analyzing the CSV structure, you can then use the main `enemeter-data-processing` tool with the appropriate parameters to process the data.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the GNU General Public License v3.0 - see the LICENSE file for details.
