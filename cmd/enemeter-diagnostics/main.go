package main

import (
	"enemeter-data-processing/internal/parser"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func main() {
	inputFile := flag.String("input", "", "Input CSV file to analyze")
	startTime := flag.String("start", "", "Start time for measurements (format: YYYY-MM-DD HH:MM:SS)")
	flag.Parse()

	if *inputFile == "" || *startTime == "" {
		fmt.Println("Usage: enemeter-diagnostics --input=data.csv --start=\"2023-04-01 08:00:00\"")
		os.Exit(1)
	}

	fmt.Printf("Analyzing file: %s\n\n", *inputFile)

	// Parse the file
	startTimeObj, err := time.Parse("2006-01-02 15:04:05", *startTime)
	if err != nil {
		fmt.Printf("Error parsing start time: %v\n", err)
		os.Exit(1)
	}

	// Create parser
	csvParser := parser.NewCSVParser(*inputFile)
	csvParser.WithFilterOptions(parser.FilterOptions{
		StartTime: &startTimeObj,
	})

	// Parse records
	records, err := csvParser.Parse()
	if err != nil {
		fmt.Printf("Error parsing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully parsed %d records\n", len(records))
	if len(records) == 0 {
		fmt.Println("No records found!")
		os.Exit(0)
	}

	// Sort by timestamp to ensure proper order
	sort.Slice(records, func(i, j int) bool {
		return records[i].Timestamp.Before(records[j].Timestamp)
	})

	// Basic time range analysis
	firstTimestamp := records[0].Timestamp
	lastTimestamp := records[len(records)-1].Timestamp
	duration := lastTimestamp.Sub(firstTimestamp)

	fmt.Printf("First record: %s\n", firstTimestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Last record: %s\n", lastTimestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Total duration: %s (%.2f hours)\n",
		duration.Round(time.Second), duration.Hours())

	// Time gaps analysis
	fmt.Println("\nAnalyzing for time gaps...")
	var largestGap time.Duration
	var largestGapStart time.Time
	var gapsOver1Min int
	var gapsOver10Min int
	var gapsOver1Hour int

	for i := 1; i < len(records); i++ {
		gap := records[i].Timestamp.Sub(records[i-1].Timestamp)

		if gap > largestGap {
			largestGap = gap
			largestGapStart = records[i-1].Timestamp
		}

		if gap > time.Minute {
			gapsOver1Min++
		}
		if gap > 10*time.Minute {
			gapsOver10Min++
		}
		if gap > time.Hour {
			gapsOver1Hour++
			fmt.Printf("  Large gap (%.1f minutes) at %s\n",
				gap.Minutes(), records[i-1].Timestamp.Format("2006-01-02 15:04:05"))
		}
	}

	fmt.Printf("\nGap Statistics:\n")
	fmt.Printf("  Gaps > 1 minute: %d\n", gapsOver1Min)
	fmt.Printf("  Gaps > 10 minutes: %d\n", gapsOver10Min)
	fmt.Printf("  Gaps > 1 hour: %d\n", gapsOver1Hour)
	fmt.Printf("  Largest gap: %s at %s\n",
		largestGap.Round(time.Second), largestGapStart.Format("2006-01-02 15:04:05"))

	// Time distribution analysis
	fmt.Println("\nHourly Record Distribution:")
	hourCounts := make(map[int]int)
	for _, record := range records {
		hour := record.Timestamp.Hour()
		hourCounts[hour]++
	}

	for hour := 0; hour < 24; hour++ {
		count := hourCounts[hour]
		fmt.Printf("  %02d:00 - %02d:59: %6d records | %s\n",
			hour, hour, count, getBarChart(count, len(records)))
	}
}

func getBarChart(count, total int) string {
	const maxWidth = 40
	percentage := float64(count) / float64(total)
	width := int(percentage * maxWidth)
	return fmt.Sprintf("%s %.1f%%", strings.Repeat("█", width), percentage*100)
}
