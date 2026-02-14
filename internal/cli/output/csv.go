package output

import (
	"bytes"
	"encoding/csv"
	"fmt"

	"github.com/thalesgelinger/dbridge/internal/database"
)

// FormatCSV formats a query result as CSV
func FormatCSV(result *database.QueryResult, includeHeader bool) (string, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header if requested
	if includeHeader {
		if err := writer.Write(result.Columns); err != nil {
			return "", fmt.Errorf("failed to write CSV header: %w", err)
		}
	}

	// Write rows
	for _, row := range result.Rows {
		stringRow := make([]string, len(row))
		for i, val := range row {
			stringRow[i] = formatCSVValue(val)
		}
		if err := writer.Write(stringRow); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("CSV writer error: %w", err)
	}

	return buf.String(), nil
}

// formatCSVValue converts a value to a string for CSV output
func formatCSVValue(val interface{}) string {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32:
		return fmt.Sprintf("%f", v)
	case float64:
		return fmt.Sprintf("%f", v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}
