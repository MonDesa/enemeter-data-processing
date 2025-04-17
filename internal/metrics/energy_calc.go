package metrics

import (
	"fmt"
	"math"
	"time"

	"enemeter-data-processing/internal/parser"
)

type MetricType string

const (
	MetricTotalEnergy       MetricType = "total_energy"
	MetricAveragePower      MetricType = "average_power"
	MetricPeakPower         MetricType = "peak_power"
	MetricTemperature       MetricType = "temperature"
	MetricEnergyByHour      MetricType = "energy_by_hour"
	MetricVoltageStats      MetricType = "voltage_stats"
	MetricCurrentStats      MetricType = "current_stats"
	MetricBatteryDischarge  MetricType = "battery_discharge"
	MetricSolarContribution MetricType = "solar_contribution"
)

type EnergyMetrics struct {
	TotalJoules             float64
	AveragePowerWatts       float64
	PeakPowerWatts          float64
	JoulesPerDay            float64
	DurationSeconds         float64
	TemperatureStats        TemperatureStats
	EnergyConsumptionByHour map[int]float64
	VoltageStats            VoltageStats
	CurrentStats            CurrentStats
	BatteryStats            BatteryStats
	SolarStats              SolarStats
	TimeRange               TimeRange
	DataPoints              int
}

type TemperatureStats struct {
	MinTempCelsius float64
	MaxTempCelsius float64
	AvgTempCelsius float64
}

type VoltageStats struct {
	MinVoltage float64
	MaxVoltage float64
	AvgVoltage float64
}

type CurrentStats struct {
	MinCurrent   float64
	MaxCurrent   float64
	AvgCurrent   float64
	MaxDischarge float64
	MaxCharging  float64
}

type BatteryStats struct {
	EstimatedCapacity      float64
	AverageDischargeRate   float64
	TotalDischargeTime     float64
	TotalChargeTime        float64
	DischargeToChargeRatio float64
}

type SolarStats struct {
	TotalEnergyProduced    float64
	AverageOutput          float64
	PeakOutput             float64
	ContributionPercentage float64
}

type TimeRange struct {
	StartTime time.Time
	EndTime   time.Time
}

type MetricsOptions struct {
	RequestedMetrics      []MetricType
	TimeResolution        time.Duration
	IncludeTimeSeriesData bool
}

type EnergyCalculator struct {
	records   []parser.EnemeterRecord
	options   MetricsOptions
	streaming bool
}

func NewEnergyCalculator(records []parser.EnemeterRecord) *EnergyCalculator {
	return &EnergyCalculator{
		records:   records,
		streaming: false,
		options: MetricsOptions{
			TimeResolution: time.Minute * 5,
		},
	}
}

func (e *EnergyCalculator) WithOptions(options MetricsOptions) *EnergyCalculator {
	e.options = options
	return e
}

func (e *EnergyCalculator) CalculateMetrics() EnergyMetrics {
	if len(e.records) == 0 {
		return EnergyMetrics{}
	}

	tracker := newMetricsTracker(e.options)

	for i, record := range e.records {
		tracker.processRecord(record, i)
	}

	return tracker.finalizeMetrics()
}

func StreamCalculateMetrics(p *parser.CSVParser, options MetricsOptions) (EnergyMetrics, error) {
	tracker := newMetricsTracker(options)

	recordIndex := 0

	var processFunc = func(record parser.EnemeterRecord) error {
		tracker.processRecord(record, recordIndex)
		recordIndex++
		return nil
	}

	err := p.StreamRecords(processFunc)

	if err != nil {
		return EnergyMetrics{}, fmt.Errorf("streaming calculation error: %w", err)
	}

	return tracker.finalizeMetrics(), nil
}

type metricsTracker struct {
	options MetricsOptions

	totalJoules     float64
	totalPower      float64
	peakPower       float64
	totalDurationMs int64
	dataPoints      int

	startTime      time.Time
	endTime        time.Time
	firstTimestamp bool

	tempSum   float64
	minTemp   float64
	maxTemp   float64
	tempCount int

	voltSum   float64
	minVolt   float64
	maxVolt   float64
	voltCount int

	currentSum   float64
	minCurrent   float64
	maxCurrent   float64
	maxDischarge float64
	maxCharging  float64
	currentCount int

	totalDischargeTime   float64
	totalChargeTime      float64
	totalDischargeEnergy float64
	totalChargeEnergy    float64

	energyByHour map[int]float64

	prevRecord *parser.EnemeterRecord
}

func newMetricsTracker(options MetricsOptions) *metricsTracker {
	return &metricsTracker{
		options:        options,
		energyByHour:   make(map[int]float64),
		firstTimestamp: true,
		minTemp:        math.MaxFloat64,
		maxTemp:        -math.MaxFloat64,
		minVolt:        math.MaxFloat64,
		maxVolt:        -math.MaxFloat64,
		minCurrent:     math.MaxFloat64,
		maxCurrent:     -math.MaxFloat64,
	}
}

func (mt *metricsTracker) processRecord(record parser.EnemeterRecord, _ int) {
	tempCelsius := float64(record.TempMiliCelsius) / 1000.0
	volts := float64(record.VoltageMicroV) / 1000000.0
	amps := float64(record.CurrentNanoA) / 1000000000.0

	powerVolts := math.Abs(volts)

	if mt.firstTimestamp {
		mt.startTime = record.Timestamp
		mt.firstTimestamp = false
	}

	mt.endTime = record.Timestamp

	mt.tempSum += tempCelsius
	mt.tempCount++
	if tempCelsius < mt.minTemp {
		mt.minTemp = tempCelsius
	}
	if tempCelsius > mt.maxTemp {
		mt.maxTemp = tempCelsius
	}

	mt.voltSum += volts
	mt.voltCount++
	if volts < mt.minVolt {
		mt.minVolt = volts
	}
	if volts > mt.maxVolt {
		mt.maxVolt = volts
	}

	mt.currentSum += amps
	mt.currentCount++
	if amps < mt.minCurrent {
		mt.minCurrent = amps
	}
	if amps > mt.maxCurrent {
		mt.maxCurrent = amps
	}

	if amps < 0 && math.Abs(amps) > mt.maxDischarge {
		mt.maxDischarge = math.Abs(amps)
	} else if amps > 0 && amps > mt.maxCharging {
		mt.maxCharging = amps
	}

	instantPower := powerVolts * amps

	if math.Abs(instantPower) > mt.peakPower {
		mt.peakPower = math.Abs(instantPower)
	}

	if mt.prevRecord != nil {
		durationSecs := float64(record.TimeDeltaMs) / 1000.0
		mt.totalDurationMs += record.TimeDeltaMs

		joules := instantPower * durationSecs
		mt.totalJoules += joules

		mt.totalPower += instantPower

		hourOfDay := record.Timestamp.Hour()
		mt.energyByHour[hourOfDay] += joules

		if amps < 0 {
			mt.totalDischargeTime += durationSecs
			mt.totalDischargeEnergy += math.Abs(joules)
		} else {
			mt.totalChargeTime += durationSecs
			mt.totalChargeEnergy += joules
		}
	}

	mt.prevRecord = &record
	mt.dataPoints++
}

func (mt *metricsTracker) finalizeMetrics() EnergyMetrics {
	metrics := EnergyMetrics{
		EnergyConsumptionByHour: mt.energyByHour,
		DataPoints:              mt.dataPoints,
		TimeRange: TimeRange{
			StartTime: mt.startTime,
			EndTime:   mt.endTime,
		},
	}

	durationSeconds := float64(mt.totalDurationMs) / 1000.0
	metrics.TotalJoules = mt.totalJoules
	metrics.PeakPowerWatts = mt.peakPower
	metrics.DurationSeconds = durationSeconds

	if durationSeconds > 0 {
		metrics.AveragePowerWatts = mt.totalJoules / durationSeconds
	}

	if durationSeconds > 0 {
		secondsPerDay := 24 * 60 * 60
		metrics.JoulesPerDay = mt.totalJoules * (float64(secondsPerDay) / durationSeconds)
	}

	if mt.tempCount > 0 {
		metrics.TemperatureStats = TemperatureStats{
			MinTempCelsius: mt.minTemp,
			MaxTempCelsius: mt.maxTemp,
			AvgTempCelsius: mt.tempSum / float64(mt.tempCount),
		}
	}

	if mt.voltCount > 0 {
		metrics.VoltageStats = VoltageStats{
			MinVoltage: mt.minVolt,
			MaxVoltage: mt.maxVolt,
			AvgVoltage: mt.voltSum / float64(mt.voltCount),
		}
	}

	if mt.currentCount > 0 {
		metrics.CurrentStats = CurrentStats{
			MinCurrent:   mt.minCurrent,
			MaxCurrent:   mt.maxCurrent,
			AvgCurrent:   mt.currentSum / float64(mt.currentCount),
			MaxDischarge: mt.maxDischarge,
			MaxCharging:  mt.maxCharging,
		}
	}

	metrics.BatteryStats = BatteryStats{
		TotalDischargeTime: mt.totalDischargeTime,
		TotalChargeTime:    mt.totalChargeTime,
	}

	totalTime := mt.totalDischargeTime + mt.totalChargeTime
	if totalTime > 0 {
		metrics.BatteryStats.DischargeToChargeRatio = mt.totalDischargeTime / totalTime
	}

	if mt.totalDischargeTime > 0 {
		metrics.BatteryStats.AverageDischargeRate = mt.totalDischargeEnergy / mt.totalDischargeTime
	}

	metrics.SolarStats = SolarStats{
		TotalEnergyProduced: mt.totalChargeEnergy,
	}

	if mt.totalChargeTime > 0 {
		metrics.SolarStats.AverageOutput = mt.totalChargeEnergy / mt.totalChargeTime
	}

	metrics.SolarStats.PeakOutput = mt.maxCharging

	totalEnergy := mt.totalDischargeEnergy + mt.totalChargeEnergy
	if totalEnergy > 0 {
		metrics.SolarStats.ContributionPercentage = (mt.totalChargeEnergy / totalEnergy) * 100
	}

	return metrics
}

func GetSpecificMetric(metrics EnergyMetrics, metricType MetricType) (interface{}, error) {
	switch metricType {
	case MetricTotalEnergy:
		return metrics.TotalJoules, nil
	case MetricAveragePower:
		return metrics.AveragePowerWatts, nil
	case MetricPeakPower:
		return metrics.PeakPowerWatts, nil
	case MetricTemperature:
		return metrics.TemperatureStats, nil
	case MetricEnergyByHour:
		return metrics.EnergyConsumptionByHour, nil
	case MetricVoltageStats:
		return metrics.VoltageStats, nil
	case MetricCurrentStats:
		return metrics.CurrentStats, nil
	case MetricBatteryDischarge:
		return metrics.BatteryStats, nil
	case MetricSolarContribution:
		return metrics.SolarStats, nil
	default:
		return nil, fmt.Errorf("unknown metric type: %s", metricType)
	}
}
