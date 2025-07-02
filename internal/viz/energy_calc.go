package viz

import (
	"enemeter-data-processing/internal/parser"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
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

// EnergyCalculationOptions provides options for energy calculation
type EnergyCalculationOptions struct {
	PreserveTimeRange bool
}

// EnergyCalculation handles energy and power calculations from sensor data
type EnergyCalculation struct {
	records []parser.EnemeterRecord
	options EnergyCalculationOptions
}

// NewEnergyCalculation creates a new energy calculation instance
func NewEnergyCalculation(records []parser.EnemeterRecord) *EnergyCalculation {
	return &EnergyCalculation{
		records: records,
	}
}

// WithOptions sets options for the energy calculation
func (ec *EnergyCalculation) WithOptions(options EnergyCalculationOptions) *EnergyCalculation {
	ec.options = options
	return ec
}

// CalculateCumulativeEnergy calculates energy consumption over time
// Returns timestamps and cumulative energy in joules
func (ec *EnergyCalculation) CalculateCumulativeEnergy() ([]time.Time, []float64) {
	if len(ec.records) < 2 {
		return []time.Time{}, []float64{}
	}

	// Log the actual time span of our data
	fmt.Printf("Full dataset spans from %s to %s (%.2f hours)\n",
		ec.records[0].Timestamp.Format("2006-01-02 15:04:05"),
		ec.records[len(ec.records)-1].Timestamp.Format("2006-01-02 15:04:05"),
		ec.records[len(ec.records)-1].Timestamp.Sub(ec.records[0].Timestamp).Hours())

	// Estimate a reasonable capacity for the output arrays
	// For very large datasets, limit initial capacity to avoid memory issues
	estimatedCapacity := len(ec.records)
	if estimatedCapacity > 100000 {
		estimatedCapacity = 100000
	}

	// Calculate energy in parallel chunks and then integrate
	fmt.Printf("Calculating energy for %d records using parallel processing...\n", len(ec.records))

	// Calculate the number of chunks based on CPU cores
	numWorkers := runtime.NumCPU()
	if numWorkers > 8 {
		numWorkers = 8 // Limit to 8 workers to avoid excessive memory usage
	}

	// For very large datasets, use more chunks to reduce memory pressure
	if len(ec.records) > 500000 {
		numWorkers *= 2
	}

	chunkSize := len(ec.records) / numWorkers
	if chunkSize < 100 {
		chunkSize = 100
	}

	// Create channel for results
	type EnergyPoint struct {
		timestamp time.Time
		energy    float64
		index     int
	}
	resultChan := make(chan EnergyPoint, estimatedCapacity)

	// Process records in chunks, calculating energy within each chunk
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(chunkIndex int) {
			defer wg.Done()

			start := chunkIndex * chunkSize
			end := (chunkIndex + 1) * chunkSize
			if end > len(ec.records) {
				end = len(ec.records)
			}
			if start >= end {
				return
			}

			// Skip the first record in each chunk except the first chunk
			localStart := start
			if chunkIndex > 0 && localStart < len(ec.records) {
				localStart++
			}

			// Each chunk calculates its own cumulative energy starting from zero
			localEnergy := 0.0

			// Add starting point for first chunk only
			if chunkIndex == 0 {
				resultChan <- EnergyPoint{ec.records[0].Timestamp, 0, 0}
			}

			// Keep track of last recorded point to prevent too-frequent updates
			lastRecordedTime := time.Time{}
			if chunkIndex == 0 && len(ec.records) > 0 {
				lastRecordedTime = ec.records[0].Timestamp
			}

			// Modify min time delta logic to ensure we don't filter too aggressively
			var minTimeDelta time.Duration
			// For very large datasets, use dynamic time delta based on total duration
			totalDuration := ec.records[len(ec.records)-1].Timestamp.Sub(ec.records[0].Timestamp)
			if len(ec.records) > 500000 {
				// Use 0.05% of total duration as minimum
				minTimeDelta = time.Duration(float64(totalDuration) * 0.0005)
				if minTimeDelta > 5*time.Second {
					minTimeDelta = 5 * time.Second
				}
			} else if len(ec.records) > 100000 {
				// Use 0.02% of total duration as minimum
				minTimeDelta = time.Duration(float64(totalDuration) * 0.0002)
				if minTimeDelta > 2*time.Second {
					minTimeDelta = 2 * time.Second
				}
			} else {
				minTimeDelta = 500 * time.Millisecond
			}

			// Process records in this chunk
			for i := localStart; i < end; i++ {
				if i == 0 {
					continue // First record has no previous to calculate energy from
				}

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
				localEnergy += energyIncrease

				// Only add points if enough time has passed since the last recorded point
				// or if we're at the end of our chunk
				shouldAdd := lastRecordedTime.IsZero() ||
					current.Timestamp.Sub(lastRecordedTime) >= minTimeDelta ||
					i == end-1

				if shouldAdd {
					resultChan <- EnergyPoint{
						timestamp: current.Timestamp,
						energy:    localEnergy,
						index:     i,
					}
					lastRecordedTime = current.Timestamp
				}

				// Report progress for large datasets periodically
				if len(ec.records) > 100000 && i%10000 == 0 {
					fmt.Printf("  Chunk %d: Processed %d/%d records (%.1f%%)\n",
						chunkIndex, i-localStart, end-localStart,
						float64(i-localStart)*100/float64(end-localStart))
				}
			}
		}(i)
	}

	// Close the results channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and sort by timestamp
	points := make([]EnergyPoint, 0, estimatedCapacity)
	for point := range resultChan {
		points = append(points, point)
	}

	// Sort points by timestamp
	fmt.Println("Sorting energy data points by time...")

	// Efficient sort using Go's built-in sort
	sort.Slice(points, func(i, j int) bool {
		return points[i].timestamp.Before(points[j].timestamp)
	})

	// Now integrate across chunks for continuous cumulative energy
	timestamps := make([]time.Time, 0, len(points))
	energyValues := make([]float64, 0, len(points))

	fmt.Println("Integrating energy across chunks...")

	// Use the first point as starting point
	if len(points) > 0 {
		timestamps = append(timestamps, points[0].timestamp)
		energyValues = append(energyValues, points[0].energy)

		// Adjust all subsequent points
		cumulativeEnergy := points[0].energy
		prevChunkIndex := points[0].index / chunkSize

		for i := 1; i < len(points); i++ {
			chunkIndex := points[i].index / chunkSize

			// If we're in a new chunk, add the cumulative energy from previous chunks
			if chunkIndex > prevChunkIndex {
				cumulativeEnergy += points[i-1].energy
				prevChunkIndex = chunkIndex
			}

			// Add the point with adjusted energy
			timestamps = append(timestamps, points[i].timestamp)
			energyValues = append(energyValues, cumulativeEnergy+points[i].energy)
		}
	}

	// Always make sure to include first and last timestamps from the original records
	// even if they might be filtered by sampling
	if ec.options.PreserveTimeRange && len(ec.records) > 0 {
		firstRecordTime := ec.records[0].Timestamp
		lastRecordTime := ec.records[len(ec.records)-1].Timestamp

		// Check if the first and last points are already included
		hasFirst := false
		hasLast := false

		for _, t := range timestamps {
			if t.Equal(firstRecordTime) || t.Sub(firstRecordTime).Abs() < 2*time.Second {
				hasFirst = true
			}
			if t.Equal(lastRecordTime) || t.Sub(lastRecordTime).Abs() < 2*time.Second {
				hasLast = true
			}
		}

		// Add first point if missing
		if !hasFirst && len(timestamps) > 0 {
			// Insert at beginning
			timestamps = append([]time.Time{firstRecordTime}, timestamps...)
			energyValues = append([]float64{0}, energyValues...)
			fmt.Println("Added first timestamp to ensure full time range")
		}

		// Add last point if missing
		if !hasLast && len(timestamps) > 1 {
			// Add at end
			timestamps = append(timestamps, lastRecordTime)
			// Use last available energy value
			energyValues = append(energyValues, energyValues[len(energyValues)-1])
			fmt.Println("Added last timestamp to ensure full time range")
		}

		// Sort to ensure chronological order
		if !hasFirst || !hasLast {
			// Create pairs
			type timeEnergyPair struct {
				time   time.Time
				energy float64
			}
			pairs := make([]timeEnergyPair, len(timestamps))
			for i := range timestamps {
				pairs[i] = timeEnergyPair{timestamps[i], energyValues[i]}
			}

			// Sort by timestamp
			sort.Slice(pairs, func(i, j int) bool {
				return pairs[i].time.Before(pairs[j].time)
			})

			// Rebuild slices
			for i := range pairs {
				timestamps[i] = pairs[i].time
				energyValues[i] = pairs[i].energy
			}
		}
	}

	// Make sure to include the first and last record in the visualization
	// to capture the full time range
	if len(timestamps) > 0 && len(ec.records) > 0 {
		// Always ensure start and end points are included
		hasFirst := false
		hasLast := false

		// Check if first and last points are included
		firstTime := ec.records[0].Timestamp
		lastTime := ec.records[len(ec.records)-1].Timestamp

		for _, t := range timestamps {
			if t.Equal(firstTime) {
				hasFirst = true
			}
			if t.Equal(lastTime) {
				hasLast = true
			}
		}

		// Add first point if missing
		if !hasFirst {
			timestamps = append([]time.Time{firstTime}, timestamps...)
			energyValues = append([]float64{0}, energyValues...)
			fmt.Println("Added first record to ensure full time range visibility")
		}

		// Add last point if missing
		if !hasLast {
			timestamps = append(timestamps, lastTime)
			// Use the last energy value we have or estimate it
			lastEnergy := energyValues[len(energyValues)-1]
			energyValues = append(energyValues, lastEnergy)
			fmt.Println("Added last record to ensure full time range visibility")
		}
	}

	// Sort points to ensure they're in chronological order after adding first/last
	if len(timestamps) > 1 {
		// Create pairs of timestamps and energy values
		type timeEnergyPair struct {
			time   time.Time
			energy float64
		}

		pairs := make([]timeEnergyPair, len(timestamps))
		for i := range timestamps {
			pairs[i] = timeEnergyPair{timestamps[i], energyValues[i]}
		}

		// Sort by timestamp
		sort.Slice(pairs, func(i, j int) bool {
			return pairs[i].time.Before(pairs[j].time)
		})

		// Reconstruct the separate slices
		for i := range pairs {
			timestamps[i] = pairs[i].time
			energyValues[i] = pairs[i].energy
		}
	}

	// At the end of processing, verify we have the full time range
	if len(timestamps) > 0 {
		dataTimeRange := timestamps[len(timestamps)-1].Sub(timestamps[0])
		fmt.Printf("Final energy data spans %.2f hours with %d data points\n",
			dataTimeRange.Hours(), len(timestamps))

		// Warn if we're missing significant parts of the time range
		expectedRange := ec.records[len(ec.records)-1].Timestamp.Sub(ec.records[0].Timestamp)
		if math.Abs(dataTimeRange.Hours()-expectedRange.Hours()) > 0.5 {
			fmt.Printf("WARNING: Visualization time range (%.2f hours) doesn't match data time range (%.2f hours)\n",
				dataTimeRange.Hours(), expectedRange.Hours())
		}
	}

	fmt.Printf("Energy calculation complete: generated %d data points\n", len(timestamps))

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
