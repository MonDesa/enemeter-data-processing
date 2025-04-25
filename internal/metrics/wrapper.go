package metrics

import (
	"enemeter-data-processing/internal/parser"
	"fmt"
	"runtime"
	"sync"
)

// StreamCalculateMetricsWrapper is a function that can be used as a drop-in replacement
// when the original StreamCalculateMetrics has signature issues
func StreamCalculateMetricsWrapper(csvParser interface{}, options MetricsOptions) (EnergyMetrics, error) {
	// Check if we have a pointer to CSVParser or a CSVParser value
	var actualParser *parser.CSVParser

	switch p := csvParser.(type) {
	case *parser.CSVParser:
		actualParser = p
	case parser.CSVParser:
		actualParser = &p
	default:
		return EnergyMetrics{}, fmt.Errorf("unexpected parser type: %T", csvParser)
	}

	// For very large files, try first to get an estimate of size
	fileSize, err := actualParser.GetFileSize()
	if err == nil && fileSize > 500*1024*1024 {
		// Try parallel processing for very large files
		fmt.Printf("Large file detected (%.2f MB). Using parallel processing for metrics calculation.\n",
			float64(fileSize)/(1024*1024))
		return calculateMetricsInParallel(actualParser, options)
	}

	// Default: Parse all records
	records, err := actualParser.Parse()
	if err != nil {
		return EnergyMetrics{}, fmt.Errorf("failed to parse CSV data: %v", err)
	}

	// Use the existing calculator with the records
	calculator := NewEnergyCalculator(records).WithOptions(options)
	return calculator.CalculateMetrics(), nil
}

// calculateMetricsInParallel implements a chunk-based approach to calculating metrics for very large datasets
func calculateMetricsInParallel(csvParser *parser.CSVParser, options MetricsOptions) (EnergyMetrics, error) {
	// Use streaming mode for memory efficiency
	fmt.Println("Streaming records and processing in chunks...")

	// Create multiple trackers, one per worker
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // Limit to 8 workers
	}

	trackers := make([]*metricsTracker, numWorkers)
	for i := 0; i < numWorkers; i++ {
		trackers[i] = newMetricsTracker(options)
	}

	// Create a mutex for worker assignment
	var mu sync.Mutex
	var currentWorker int
	var recordCount int64

	// Process records in streaming mode
	processFunc := func(record parser.EnemeterRecord) error {
		// Assign records to workers in a round-robin fashion
		mu.Lock()
		worker := currentWorker
		currentWorker = (currentWorker + 1) % numWorkers
		recordCount++
		mu.Unlock()

		// Process the record with the assigned worker
		trackers[worker].processRecord(record, int(recordCount))

		return nil
	}

	// Stream all records
	err := csvParser.StreamRecords(processFunc)
	if err != nil {
		return EnergyMetrics{}, fmt.Errorf("error streaming records: %v", err)
	}

	fmt.Printf("Processed %d records with %d workers\n", recordCount, numWorkers)

	// Merge results from all workers
	fmt.Println("Merging results from all workers...")
	finalMetrics := mergeMetricsTrackers(trackers)

	return finalMetrics, nil
}

// mergeMetricsTrackers combines results from multiple trackers into a single result
func mergeMetricsTrackers(trackers []*metricsTracker) EnergyMetrics {
	if len(trackers) == 0 {
		return EnergyMetrics{}
	}

	// Use the first tracker as a base
	result := trackers[0].finalizeMetrics()

	if len(trackers) == 1 {
		return result
	}

	// Merge additional trackers
	for i := 1; i < len(trackers); i++ {
		metrics := trackers[i].finalizeMetrics()

		// Merge key metrics
		result.TotalJoules += metrics.TotalJoules

		// Update min/max values
		if metrics.TemperatureStats.MinTempCelsius < result.TemperatureStats.MinTempCelsius {
			result.TemperatureStats.MinTempCelsius = metrics.TemperatureStats.MinTempCelsius
		}
		if metrics.TemperatureStats.MaxTempCelsius > result.TemperatureStats.MaxTempCelsius {
			result.TemperatureStats.MaxTempCelsius = metrics.TemperatureStats.MaxTempCelsius
		}

		if metrics.VoltageStats.MinVoltage < result.VoltageStats.MinVoltage {
			result.VoltageStats.MinVoltage = metrics.VoltageStats.MinVoltage
		}
		if metrics.VoltageStats.MaxVoltage > result.VoltageStats.MaxVoltage {
			result.VoltageStats.MaxVoltage = metrics.VoltageStats.MaxVoltage
		}

		if metrics.CurrentStats.MinCurrent < result.CurrentStats.MinCurrent {
			result.CurrentStats.MinCurrent = metrics.CurrentStats.MinCurrent
		}
		if metrics.CurrentStats.MaxCurrent > result.CurrentStats.MaxCurrent {
			result.CurrentStats.MaxCurrent = metrics.CurrentStats.MaxCurrent
		}

		if metrics.CurrentStats.MaxDischarge > result.CurrentStats.MaxDischarge {
			result.CurrentStats.MaxDischarge = metrics.CurrentStats.MaxDischarge
		}
		if metrics.CurrentStats.MaxCharging > result.CurrentStats.MaxCharging {
			result.CurrentStats.MaxCharging = metrics.CurrentStats.MaxCharging
		}

		// Update power stats
		if metrics.PeakPowerWatts > result.PeakPowerWatts {
			result.PeakPowerWatts = metrics.PeakPowerWatts
		}

		// Merge time ranges
		if metrics.TimeRange.StartTime.Before(result.TimeRange.StartTime) {
			result.TimeRange.StartTime = metrics.TimeRange.StartTime
		}
		if metrics.TimeRange.EndTime.After(result.TimeRange.EndTime) {
			result.TimeRange.EndTime = metrics.TimeRange.EndTime
		}

		// Merge data points count
		result.DataPoints += metrics.DataPoints

		// Merge accumulated values for computing averages later
		result.DurationSeconds += metrics.DurationSeconds

		// Merge hourly data
		for hour, energy := range metrics.EnergyConsumptionByHour {
			result.EnergyConsumptionByHour[hour] += energy
		}

		// Merge battery stats
		result.BatteryStats.TotalDischargeTime += metrics.BatteryStats.TotalDischargeTime
		result.BatteryStats.TotalChargeTime += metrics.BatteryStats.TotalChargeTime

		// Merge solar stats
		result.SolarStats.TotalEnergyProduced += metrics.SolarStats.TotalEnergyProduced
	}

	// Recalculate averages
	if result.DurationSeconds > 0 {
		result.AveragePowerWatts = result.TotalJoules / result.DurationSeconds
	}

	// Recalculate daily consumption
	if result.DurationSeconds > 0 {
		secondsPerDay := 24 * 60 * 60
		result.JoulesPerDay = result.TotalJoules * (float64(secondsPerDay) / result.DurationSeconds)
	}

	// Update battery discharge ratio
	totalTime := result.BatteryStats.TotalDischargeTime + result.BatteryStats.TotalChargeTime
	if totalTime > 0 {
		result.BatteryStats.DischargeToChargeRatio = result.BatteryStats.TotalDischargeTime / totalTime
	}

	// Recalculate average discharge rate
	if result.BatteryStats.TotalDischargeTime > 0 {
		// Since we don't have the individual discharge energy values, we'll need to
		// approximate this based on total metrics
		totalDischargeEnergyRaw := 0.0
		for _, tracker := range trackers {
			totalDischargeEnergyRaw += tracker.totalDischargeEnergy
		}

		result.BatteryStats.AverageDischargeRate = totalDischargeEnergyRaw / result.BatteryStats.TotalDischargeTime
	}

	// Update solar contribution percentage
	totalEnergyRaw := 0.0
	for _, tracker := range trackers {
		totalEnergyRaw += tracker.totalDischargeEnergy + tracker.totalChargeEnergy
	}

	if totalEnergyRaw > 0 {
		chargeEnergyRaw := 0.0
		for _, tracker := range trackers {
			chargeEnergyRaw += tracker.totalChargeEnergy
		}
		result.SolarStats.ContributionPercentage = (chargeEnergyRaw / totalEnergyRaw) * 100
	}

	// Calculate solar average output
	if result.BatteryStats.TotalChargeTime > 0 {
		result.SolarStats.AverageOutput = result.SolarStats.TotalEnergyProduced / result.BatteryStats.TotalChargeTime
	}

	return result
}
