package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
)

func main() {
	inputFile := flag.String("input", "", "Path to the CSV file to analyze")
	sampleSize := flag.Int("samples", 10, "Number of sample rows to display")
	flag.Parse()

	if *inputFile == "" {
		fmt.Println("Error: Please specify an input file with --input")
		os.Exit(1)
	}

	file, err := os.Open(*inputFile)
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: Error closing file: %v\n", closeErr)
		}
	}()

	reader := csv.NewReader(file)

	fmt.Printf("Analyzing file: %s\n", *inputFile)
	fmt.Printf("Showing %d sample rows:\n\n", *sampleSize)

	var timeDeltas []int64
	var voltages []int64
	var currents []int64
	var temps []int64

	fmt.Println("RAW DATA SAMPLES:")
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("%-15s %-15s %-15s %-15s\n", "TIME_DELTA", "VOLTAGE", "CURRENT", "TEMP")
	fmt.Println("---------------------------------------------------------------")

	for i := 0; i < *sampleSize; i++ {
		row, err := reader.Read()
		if err != nil {
			break
		}

		if len(row) != 4 {
			fmt.Printf("Row %d has %d columns, expected 4\n", i+1, len(row))
			continue
		}

		timeDelta, _ := strconv.ParseInt(row[0], 10, 64)
		voltage, _ := strconv.ParseInt(row[1], 10, 64)
		current, _ := strconv.ParseInt(row[2], 10, 64)
		temp, _ := strconv.ParseInt(row[3], 10, 64)

		timeDeltas = append(timeDeltas, timeDelta)
		voltages = append(voltages, voltage)
		currents = append(currents, current)
		temps = append(temps, temp)

		fmt.Printf("%-15d %-15d %-15d %-15d\n", timeDelta, voltage, current, temp)
	}

	fmt.Println("\nVALUE RANGES:")
	fmt.Println("---------------------------------------------------------------")

	minTimeDelta, maxTimeDelta := findMinMax(timeDeltas)
	minTemp, maxTemp := findMinMax(temps)
	minVoltage, maxVoltage := findMinMax(voltages)
	minCurrent, maxCurrent := findMinMax(currents)

	fmt.Printf("Time Delta:   Min=%d ms, Max=%d ms\n", minTimeDelta, maxTimeDelta)
	fmt.Printf("Temperature:  Min=%d, Max=%d (Raw value)\n", minTemp, maxTemp)
	fmt.Printf("              Min=%.2f °C, Max=%.2f °C (Millicelsius)\n", float64(minTemp)/1000.0, float64(maxTemp)/1000.0)
	fmt.Printf("              Min=%.2f °C, Max=%.2f °C (Raw/100)\n", float64(minTemp)/100.0, float64(maxTemp)/100.0)

	fmt.Printf("Voltage:      Min=%d, Max=%d (Raw value)\n", minVoltage, maxVoltage)
	fmt.Printf("              Min=%.6f V, Max=%.6f V (Microvolts)\n", float64(minVoltage)/1000000.0, float64(maxVoltage)/1000000.0)
	fmt.Printf("              Min=%.6f V, Max=%.6f V (Millivolts)\n", float64(minVoltage)/1000.0, float64(maxVoltage)/1000.0)

	fmt.Printf("Current:      Min=%d, Max=%d (Raw value)\n", minCurrent, maxCurrent)
	fmt.Printf("              Min=%.6f A, Max=%.6f A (Nanoamperes)\n", float64(minCurrent)/1000000000.0, float64(maxCurrent)/1000000000.0)
	fmt.Printf("              Min=%.6f A, Max=%.6f A (Microamperes)\n", float64(minCurrent)/1000000.0, float64(maxCurrent)/1000000.0)

	fmt.Printf("\nSUGGESTED UNITS FOR YOUR ESP32 DATA:\n")
	fmt.Printf("---------------------------------------------------------------\n")

	suggestUnits(0, maxTemp, minVoltage, maxVoltage, minCurrent, maxCurrent)

	fmt.Printf("ENEMETER DATA FORMAT INFORMATION:\n")
	fmt.Printf("---------------------------------------------------------------\n")
	fmt.Printf("Column order: TIME_DELTA, VOLTAGE, CURRENT, TEMP\n")
	fmt.Printf("Temperature: Values are in millicelsius (°C = value / 1000)\n")
	fmt.Printf("Voltage:     Values are in microvolts (V = value / 1000000)\n")
	fmt.Printf("Current:     Values are in nanoamperes (A = value / 1000000000)\n")
}

func findMinMax(values []int64) (int64, int64) {
	if len(values) == 0 {
		return 0, 0
	}

	min := values[0]
	max := values[0]

	for _, val := range values {
		if val < min {
			min = val
		}
		if val > max {
			max = val
		}
	}

	return min, max
}

func suggestUnits(_, maxTemp, minVoltage, maxVoltage, minCurrent, maxCurrent int64) {
	// Suggest temperature units
	if maxTemp > 100000 {
		fmt.Println("Temperature: Values appear to be in raw ADC format")
		fmt.Println("             Consider using raw_value / 10 as °C")
	} else if maxTemp > 10000 {
		fmt.Println("Temperature: Values appear to be in millicelsius")
		fmt.Println("             Consider using raw_value / 1000 as °C")
	} else if maxTemp > 1000 {
		fmt.Println("Temperature: Values appear to be in centicelsius")
		fmt.Println("             Consider using raw_value / 100 as °C")
	} else {
		fmt.Println("Temperature: Values appear to be direct celsius readings")
	}

	// Suggest voltage units
	if math.Abs(float64(minVoltage)) > 1000000 || math.Abs(float64(maxVoltage)) > 1000000 {
		fmt.Println("Voltage:     Values appear to be in microvolts")
		fmt.Println("             Consider using raw_value / 1000000 as V")
	} else if math.Abs(float64(minVoltage)) > 1000 || math.Abs(float64(maxVoltage)) > 1000 {
		fmt.Println("Voltage:     Values appear to be in millivolts")
		fmt.Println("             Consider using raw_value / 1000 as V")
	} else {
		fmt.Println("Voltage:     Values appear to be direct voltage readings (V)")
	}

	// Suggest current units
	if maxCurrent > 1000000 || minCurrent < -1000000 {
		fmt.Println("Current:     Values appear to be in nanoamperes")
		fmt.Println("             Consider using raw_value / 1000000000 as A")
	} else if maxCurrent > 1000 || minCurrent < -1000 {
		fmt.Println("Current:     Values appear to be in microamperes")
		fmt.Println("             Consider using raw_value / 1000000 as A")
	} else {
		fmt.Println("Current:     Values appear to be in milliamperes")
		fmt.Println("             Consider using raw_value / 1000 as A")
	}
}
