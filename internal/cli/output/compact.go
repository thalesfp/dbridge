package output

import (
	"encoding/json"
	"fmt"

	"github.com/thalesfp/dbridge/internal/database"
)

// CompactResult is the ultra-efficient output format
type CompactResult struct {
	Cols  []string        `json:"cols,omitempty"`  // Column names
	Types []string        `json:"types,omitempty"` // PostgreSQL types (short form)
	Rows  [][]interface{} `json:"rows,omitempty"`  // Row data as arrays
	N     *int            `json:"n,omitempty"`     // Row count if truncated
	T     *int64          `json:"t,omitempty"`     // Execution time (ms) - optional
	W     []string        `json:"w,omitempty"`     // Warnings - optional
}

// ErrorResult represents an error in compact format
type ErrorResult struct {
	Error string  `json:"error"`
	Hint  *string `json:"hint,omitempty"`
	Code  *string `json:"code,omitempty"`
	Pos   *int    `json:"pos,omitempty"`
}

// FormatOptions controls output formatting
type FormatOptions struct {
	IncludeTypes    bool
	IncludeTiming   bool
	IncludeWarnings bool
	SmartSimplify   bool
}

// FormatCompact formats a query result in ultra-compact JSON
func FormatCompact(result *database.QueryResult, opts FormatOptions) (string, error) {
	output := formatCompactResult(result, opts)
	bytes, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
}

// formatCompactResult converts QueryResult to the appropriate compact format
func formatCompactResult(result *database.QueryResult, opts FormatOptions) interface{} {
	// Empty result
	if len(result.Rows) == 0 {
		return []interface{}{}
	}

	// Smart simplification enabled
	if opts.SmartSimplify {
		// Single column, single row → Just the value
		if len(result.Columns) == 1 && len(result.Rows) == 1 {
			return result.Rows[0][0]
		}

		// Single column, multiple rows → Array of values
		if len(result.Columns) == 1 {
			values := make([]interface{}, len(result.Rows))
			for i, row := range result.Rows {
				values[i] = row[0]
			}
			return values
		}

		// Single row, multiple columns → Object
		if len(result.Rows) == 1 {
			obj := make(map[string]interface{})
			for i, col := range result.Columns {
				obj[col] = result.Rows[0][i]
			}
			return obj
		}
	}

	// Standard case → Compact format
	compact := &CompactResult{
		Cols: result.Columns,
		Rows: result.Rows,
	}

	// Optional fields
	if opts.IncludeTypes {
		compact.Types = result.ColumnTypes
	}

	if opts.IncludeTiming {
		durationMs := int64(result.Duration.Milliseconds())
		compact.T = &durationMs
	}

	if opts.IncludeWarnings && len(result.Warnings) > 0 {
		compact.W = result.Warnings
	}

	if result.Truncated {
		compact.N = &result.TotalRows
	}

	return compact
}

// FormatError formats an error in compact JSON
func FormatError(err error, hint, code *string, position *int) (string, error) {
	errResult := &ErrorResult{
		Error: err.Error(),
		Hint:  hint,
		Code:  code,
		Pos:   position,
	}

	bytes, err := json.Marshal(errResult)
	if err != nil {
		return "", fmt.Errorf("failed to marshal error: %w", err)
	}

	return string(bytes), nil
}

// FormatExecResult formats an execution result (INSERT/UPDATE/DELETE)
func FormatExecResult(result *database.ExecResult, opts FormatOptions) (string, error) {
	output := map[string]interface{}{
		"rows_affected": result.RowsAffected,
	}

	if opts.IncludeTiming {
		output["t"] = result.Duration.Milliseconds()
	}

	bytes, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal exec result: %w", err)
	}

	return string(bytes), nil
}
