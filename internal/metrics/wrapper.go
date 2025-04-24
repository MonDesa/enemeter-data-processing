package metrics

import (
	"enemeter-data-processing/internal/parser"
	"fmt"
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

	// Parse all records
	records, err := actualParser.Parse()
	if err != nil {
		return EnergyMetrics{}, fmt.Errorf("failed to parse CSV data: %v", err)
	}

	// Use the existing calculator with the records
	calculator := NewEnergyCalculator(records).WithOptions(options)
	return calculator.CalculateMetrics(), nil
}
