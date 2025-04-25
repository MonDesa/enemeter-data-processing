package viz

import (
	"enemeter-data-processing/internal/parser"
)

// EnemeterRecordAdapter provides accessor methods for EnemeterRecord fields
// with proper unit conversions
type EnemeterRecordAdapter struct {
	record parser.EnemeterRecord
}

// NewAdapter creates a new adapter for an EnemeterRecord
func NewAdapter(record parser.EnemeterRecord) EnemeterRecordAdapter {
	return EnemeterRecordAdapter{record: record}
}

// GetTimeDeltaSeconds returns the time delta in seconds
func (a EnemeterRecordAdapter) GetTimeDeltaSeconds() float64 {
	return float64(a.record.TimeDeltaMs) / 1000.0
}

// GetVoltage returns the voltage value in microvolts
func (a EnemeterRecordAdapter) GetVoltage() int64 {
	return a.record.VoltageMicroV
}

// GetVoltageVolts returns the voltage in volts
func (a EnemeterRecordAdapter) GetVoltageVolts() float64 {
	return float64(a.record.VoltageMicroV) / 1e6
}

// GetCurrent returns the current value in nanoamperes
func (a EnemeterRecordAdapter) GetCurrent() int64 {
	return a.record.CurrentNanoA
}

// GetCurrentAmps returns the current in amperes
func (a EnemeterRecordAdapter) GetCurrentAmps() float64 {
	return float64(a.record.CurrentNanoA) / 1e9
}

// GetTemperature returns the temperature in millicelsius
func (a EnemeterRecordAdapter) GetTemperature() int64 {
	return a.record.TempMiliCelsius
}

// GetTemperatureCelsius returns the temperature in celsius
func (a EnemeterRecordAdapter) GetTemperatureCelsius() float64 {
	return float64(a.record.TempMiliCelsius) / 1e3
}

// GetPowerWatts returns the instantaneous power in watts (V * I)
func (a EnemeterRecordAdapter) GetPowerWatts() float64 {
	volts := a.GetVoltageVolts()
	amps := a.GetCurrentAmps()
	return volts * amps
}

// AbsPowerWatts returns the absolute value of power in watts
func (a EnemeterRecordAdapter) AbsPowerWatts() float64 {
	power := a.GetPowerWatts()
	if power < 0 {
		return -power
	}
	return power
}

// GetEnergyJoules returns the energy in joules
// This is a placeholder since we can't directly measure energy from a single record
// The actual energy calculation is done in EnergyCalculation.CalculateCumulativeEnergy
func (a EnemeterRecordAdapter) GetEnergyJoules() float64 {
	// This is just a placeholder as energy is calculated from power over time
	// and not directly available in a single record
	return 0.0
}
