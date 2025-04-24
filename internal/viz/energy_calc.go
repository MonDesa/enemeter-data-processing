package viz

import (
	"enemeter-data-processing/internal/parser"
	"fmt"
	"time"
)

// TimeUnit represents different time resolutions for aggregating energy data
type TimeUnit string

const (
	TimeUnitSecond TimeUnit = "second"
	TimeUnitMinute TimeUnit = "minute"
	TimeUnitHour   TimeUnit = "hour"
	TimeUnitDay    TimeUnit = "day"
)

// EnergyCalculation handles energy and power calculations from sensor data
type EnergyCalculation struct {
	records []parser.EnemeterRecord
}

// NewEnergyCalculation creates a new energy calculation instance
func NewEnergyCalculation(records []parser.EnemeterRecord) *EnergyCalculation {
	return &EnergyCalculation{
		records: records,
	}
}

// CalculateCumulativeEnergy calculates energy consumption over time
// Returns timestamps and cumulative energy in joules
func (ec *EnergyCalculation) CalculateCumulativeEnergy() ([]time.Time, []float64) {
	if len(ec.records) < 2 {
		return []time.Time{}, []float64{}
	}

	// Estimate a reasonable capacity for the output arrays
	// For very large datasets, limit initial capacity to avoid memory issues
	estimatedCapacity := len(ec.records)
	if estimatedCapacity > 100000 {
		estimatedCapacity = 100000
	}

	timestamps := make([]time.Time, 0, estimatedCapacity)
	energyValues := make([]float64, 0, estimatedCapacity)
	cumulativeEnergy := 0.0

	// Add starting point
	timestamps = append(timestamps, ec.records[0].Timestamp)
	energyValues = append(energyValues, 0)

	// For large datasets, report progress
	reportProgress := len(ec.records) > 100000
	reportInterval := len(ec.records) / 10
	if reportProgress {
		fmt.Printf("Processing %d records for energy calculation...\n", len(ec.records))
	}

	// Keep track of last recorded point to prevent too-frequent updates
	lastRecordedTime := ec.records[0].Timestamp
	var minTimeBetweenPoints time.Duration

	// For very large datasets, we need larger time intervals between points
	if len(ec.records) > 500000 {
		minTimeBetweenPoints = 5 * time.Second
	} else if len(ec.records) > 100000 {
		minTimeBetweenPoints = 2 * time.Second
	} else {
		minTimeBetweenPoints = 500 * time.Millisecond
	}

	for i := 1; i < len(ec.records); i++ {
		current := ec.records[i]
		previous := ec.records[i-1]

		currentAdapter := NewAdapter(current)
		previousAdapter := NewAdapter(previous)

		// Time delta in seconds
		deltaT := current.Timestamp.Sub(previous.Timestamp).Seconds()
		if deltaT <= 0 || deltaT > 60 { // Skip gaps or invalid time deltas
			continue
		}

		// Calculate average power between consecutive points
		avgPower := (previousAdapter.AbsPowerWatts() + currentAdapter.AbsPowerWatts()) / 2

		// Energy in joules = power in watts * time in seconds
		energyIncrease := avgPower * deltaT
		cumulativeEnergy += energyIncrease

		// Only add points if enough time has passed since the last recorded point
		// This greatly reduces the number of points for large datasets
		if current.Timestamp.Sub(lastRecordedTime) >= minTimeBetweenPoints || i == len(ec.records)-1 {
			timestamps = append(timestamps, current.Timestamp)
			energyValues = append(energyValues, cumulativeEnergy)
			lastRecordedTime = current.Timestamp
		}

		// Report progress for large datasets
		if reportProgress && i%reportInterval == 0 {
			fmt.Printf("Energy calculation: %d/%d records (%.1f%%)\n",
				i, len(ec.records), float64(i)*100/float64(len(ec.records)))
		}
	}

	// For safety, if we have very few points (due to filtering), add the last one
	if len(timestamps) < 2 && len(ec.records) > 1 {
		lastRecord := ec.records[len(ec.records)-1]
		timestamps = append(timestamps, lastRecord.Timestamp)
		energyValues = append(energyValues, cumulativeEnergy)
	}

	if reportProgress {
		fmt.Printf("Energy calculation complete: generated %d data points\n", len(timestamps))
	}

	return timestamps, energyValues
}

// AggregateEnergyByTimeUnit aggregates energy consumption by specified time unit
// Returns a map with keys as time unit periods (hour number, day number, etc.)
// and values as total energy in joules for that period
func (ec *EnergyCalculation) AggregateEnergyByTimeUnit(unit TimeUnit) map[int]float64 {
	result := make(map[int]float64)

	if len(ec.records) < 2 {
		return result
	}

	for i := 1; i < len(ec.records); i++ {
		current := ec.records[i]
		previous := ec.records[i-1]

		currentAdapter := NewAdapter(current)
		previousAdapter := NewAdapter(previous)

		// Time delta in seconds
		deltaT := current.Timestamp.Sub(previous.Timestamp).Seconds()
		if deltaT <= 0 || deltaT > 60 { // Skip gaps or invalid time deltas
			continue
		}

		// Calculate average power between consecutive points
		avgPower := (previousAdapter.AbsPowerWatts() + currentAdapter.AbsPowerWatts()) / 2

		// Energy in joules = power in watts * time in seconds
		energyIncrease := avgPower * deltaT

		// Determine the time unit key
		var unitKey int
		switch unit {
		case TimeUnitSecond:
			unitKey = current.Timestamp.Second()
		case TimeUnitMinute:
			unitKey = current.Timestamp.Minute()
		case TimeUnitHour:
			unitKey = current.Timestamp.Hour()
		case TimeUnitDay:
			unitKey = current.Timestamp.Day()
		default:
			unitKey = current.Timestamp.Hour() // Default to hour
		}

		// Add to the aggregate
		result[unitKey] += energyIncrease
	}

	return result
}
