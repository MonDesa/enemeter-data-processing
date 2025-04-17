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
./enemeter-data-processing process --input=data/esp32.csv --start="2023-04-01 08:00:00"
```

The `--start` parameter specifies the starting time of the measurements and is required. 
**Important:** You must include both the date AND time of day in the format "YYYY-MM-DD HH:MM:SS".

Save results to a text file:

```bash
./enemeter-data-processing process --input=data/esp32.csv --start="2023-04-01 08:00:00" --output=esp32_report.txt
```

## Command-line Options

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

## Examples

### Basic Processing

Process a CSV file and display all metrics:

```bash
./enemeter-data-processing process --input=data/esp32.csv --start="2023-04-01 08:00:00"
```

### Memory-Efficient Processing for Large Files

Use streaming mode with sampling for very large files:

```bash
./enemeter-data-processing process --input=big-data.csv --start="2023-04-01 10:15:30" --stream --sample=10
```

This processes only every 10th record, reducing memory requirements.

### Time-Based Analysis

To analyze data from a specific time period:

```bash
# Analyze data between specific dates and times
./enemeter-data-processing process --input=esp32.csv --start="2023-04-01 08:00:00" --end="2023-04-02 17:30:00"

# Analyze data for a specific duration
./enemeter-data-processing process --input=esp32.csv --start="2023-04-01 12:45:00" --window=24h
```

### Extracting Specific Metrics

Get only temperature statistics in JSON format:

```bash
./enemeter-data-processing process --input=esp32.csv --start="2023-04-01 08:00:00" --metric=temperature --format=json
```

Extract hourly energy consumption in CSV format:

```bash
./enemeter-data-processing process --input=esp32.csv --start="2023-04-01 08:00:00" --metric=energy_by_hour --format=csv
```

### Filtering by Data Values

Process only records with voltage between specified values:

```bash
./enemeter-data-processing process --input=esp32.csv --start="2023-04-01 08:00:00" --volt-min=3500000 --volt-max=4200000
```

Process only records where temperature is above a certain threshold:

```bash
./enemeter-data-processing process --input=esp32.csv --start="2023-04-01 08:00:00" --min-temp=25000
```

## Output Examples

### Text Output (Default)