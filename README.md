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
- Analyze battery charging/discharging patterns
- Evaluate solar panel contribution

## Installation

### Prerequisites

- Go 1.23 or later

### Building from source

```bash
git clone https://github.com/yourusername/enemeter-data-processing.git
cd enemeter-data-processing
go build
```

## Data Format

ENEMETER data is provided in CSV format with four columns:

1. **Time Delta (milliseconds)** - Time elapsed since the last measurement
2. **Temperature (millicelsius)** - System temperature
3. **Voltage (microvolts)** - Battery voltage
4. **Current (nanoamperes)** - System current (positive when charging, negative when discharging)

Example:
```
0000052051,0004815430,-1275169536,0000023192
0000000047,0004815430,-1275169536,0000023192
0000000047,0004815790,-1276443264,0000023192
```

## Basic Usage

Process a CSV file with default settings:

```bash
./enemeter-data-processing --input=data/data.csv
```

Save results to a text file:

```bash
./enemeter-data-processing --input=data/data.csv --output=report.txt
```

## Command-line Options

### Input/Output Options

- `--input=<path>`: Path to the input CSV file (required)
- `--output=<path>`: Path to save the output report (optional)
- `--format=<text|json|csv>`: Output format (default: text)

### Processing Options

- `--stream`: Use memory-efficient streaming mode for large files
- `--sample=<N>`: Process every Nth record (default: 1, process all records)
- `--max=<N>`: Maximum number of records to process (default: 0, no limit)

### Time Filtering Options

- `--start=<time>`: Start time for filtering (format: YYYY-MM-DD[THH:MM:SS])
- `--end=<time>`: End time for filtering (format: YYYY-MM-DD[THH:MM:SS])
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

## Examples

### Basic Processing

Process a CSV file and display all metrics:

```bash
./enemeter-data-processing --input=data/data.csv
```

### Memory-Efficient Processing for Large Files

Use streaming mode with sampling for very large files:

```bash
./enemeter-data-processing --input=big-data.csv --stream --sample=10
```

This processes only every 10th record, reducing memory requirements.

### Filtering by Time

Process data from a specific time range:

```bash
./enemeter-data-processing --input=data.csv --start="2025-04-01" --end="2025-04-02"
```

Process the last 24 hours of data:

```bash
./enemeter-data-processing --input=data.csv --window=24h
```

### Extracting Specific Metrics

Get only temperature statistics in JSON format:

```bash
./enemeter-data-processing --input=data.csv --metric=temperature --format=json
```

Extract hourly energy consumption in CSV format:

```bash
./enemeter-data-processing --input=data.csv --metric=energy_by_hour --format=csv
```

### Filtering by Data Values

Process only records with voltage between specified values:

```bash
./enemeter-data-processing --input=data.csv --volt-min=3500000 --volt-max=4200000
```

Process only records where temperature is above a certain threshold:

```bash
./enemeter-data-processing --input=data.csv --min-temp=25000
```

## Output Examples

### Text Output (Default)

```
========== ENEMETER DATA PROCESSING REPORT ==========
Input File: data.csv
Date: 2025-04-09 10:15:32
Data Points: 1253
Time Range: 2025-04-08 08:30:15 to 2025-04-09 08:30:10

ENERGY METRICS
-------------
Total Energy Consumed: 156.4578 joules
Average Power: 0.0548 watts
Peak Power: 0.1245 watts
Estimated Energy per Day: 157.9825 joules
Measurement Duration: 86395.00 seconds

TEMPERATURE STATISTICS
---------------------
Minimum Temperature: 22.45 °C
Maximum Temperature: 28.95 °C
Average Temperature: 24.75 °C

... additional sections ...
```

### JSON Output

```json
{
  "TotalJoules": 156.4578,
  "AveragePowerWatts": 0.0548,
  "PeakPowerWatts": 0.1245,
  "JoulesPerDay": 157.9825,
  "DurationSeconds": 86395.0,
  "TemperatureStats": {
    "MinTempCelsius": 22.45,
    "MaxTempCelsius": 28.95,
    "AvgTempCelsius": 24.75
  },
  ...
}
```

### CSV Output

```
Metric,Value
TotalJoules,156.457800
AveragePowerWatts,0.054800
PeakPowerWatts,0.124500
JoulesPerDay,157.982500
DurationSeconds,86395.00
...
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.