package output

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/thalesgelinger/pgmcp/internal/database"
)

// FormatTable formats a query result as an ASCII table with borders
func FormatTable(result *database.QueryResult) (string, error) {
	if len(result.Rows) == 0 {
		var buf bytes.Buffer
		// Show header even for empty results
		buf.WriteString(renderTableHeader(result.Columns, []int{}))
		buf.WriteString("\n(0 rows)\n")
		return buf.String(), nil
	}

	// Convert all values to strings and calculate column widths
	widths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		widths[i] = len(col)
	}

	stringRows := make([][]string, len(result.Rows))
	for rowIdx, row := range result.Rows {
		stringRows[rowIdx] = make([]string, len(row))
		for colIdx, val := range row {
			str := formatValue(val)
			stringRows[rowIdx][colIdx] = str
			if len(str) > widths[colIdx] {
				widths[colIdx] = len(str)
			}
		}
	}

	var buf bytes.Buffer

	// Top border
	buf.WriteString(renderBorder(widths, "┌", "┬", "┐"))
	buf.WriteString("\n")

	// Header
	buf.WriteString("│ ")
	for i, col := range result.Columns {
		buf.WriteString(padRight(col, widths[i]))
		if i < len(result.Columns)-1 {
			buf.WriteString(" │ ")
		}
	}
	buf.WriteString(" │\n")

	// Header separator
	buf.WriteString(renderBorder(widths, "├", "┼", "┤"))
	buf.WriteString("\n")

	// Data rows
	for _, row := range stringRows {
		buf.WriteString("│ ")
		for i, val := range row {
			buf.WriteString(padRight(val, widths[i]))
			if i < len(row)-1 {
				buf.WriteString(" │ ")
			}
		}
		buf.WriteString(" │\n")
	}

	// Bottom border
	buf.WriteString(renderBorder(widths, "└", "┴", "┘"))
	buf.WriteString("\n")

	// Row count footer
	footer := fmt.Sprintf("\n(%d row", result.RowCount)
	if result.RowCount != 1 {
		footer += "s"
	}
	footer += ")\n"
	buf.WriteString(footer)

	return buf.String(), nil
}

// renderBorder creates a table border line
func renderBorder(widths []int, left, middle, right string) string {
	var parts []string
	for _, width := range widths {
		parts = append(parts, strings.Repeat("─", width+2))
	}
	return left + strings.Join(parts, middle) + right
}

// renderTableHeader creates just the header with borders (for empty tables)
func renderTableHeader(columns []string, widths []int) string {
	if len(widths) == 0 {
		widths = make([]int, len(columns))
		for i, col := range columns {
			widths[i] = len(col)
		}
	}

	var buf bytes.Buffer
	buf.WriteString(renderBorder(widths, "┌", "┬", "┐"))
	buf.WriteString("\n│ ")
	for i, col := range columns {
		buf.WriteString(padRight(col, widths[i]))
		if i < len(columns)-1 {
			buf.WriteString(" │ ")
		}
	}
	buf.WriteString(" │\n")
	buf.WriteString(renderBorder(widths, "└", "┴", "┘"))
	return buf.String()
}

// formatValue converts a value to a string for table display
func formatValue(val interface{}) string {
	if val == nil {
		return "NULL"
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
	case float32, float64:
		return fmt.Sprintf("%v", v)
	case bool:
		if v {
			return "t"
		}
		return "f"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// FormatTableCompact formats a query result as a compact table (no borders)
func FormatTableCompact(result *database.QueryResult) (string, error) {
	if len(result.Rows) == 0 {
		return "(0 rows)\n", nil
	}

	var buf bytes.Buffer

	// Calculate column widths
	widths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		widths[i] = len(col)
	}

	// Convert all values to strings and track max widths
	stringRows := make([][]string, len(result.Rows))
	for rowIdx, row := range result.Rows {
		stringRows[rowIdx] = make([]string, len(row))
		for colIdx, val := range row {
			str := formatValue(val)
			stringRows[rowIdx][colIdx] = str
			if len(str) > widths[colIdx] {
				widths[colIdx] = len(str)
			}
		}
	}

	// Print header
	for i, col := range result.Columns {
		buf.WriteString(padRight(col, widths[i]))
		if i < len(result.Columns)-1 {
			buf.WriteString(" | ")
		}
	}
	buf.WriteString("\n")

	// Print separator
	for i, width := range widths {
		buf.WriteString(strings.Repeat("-", width))
		if i < len(widths)-1 {
			buf.WriteString("-+-")
		}
	}
	buf.WriteString("\n")

	// Print rows
	for _, row := range stringRows {
		for i, val := range row {
			buf.WriteString(padRight(val, widths[i]))
			if i < len(row)-1 {
				buf.WriteString(" | ")
			}
		}
		buf.WriteString("\n")
	}

	// Add row count footer
	footer := fmt.Sprintf("(%d row", result.RowCount)
	if result.RowCount != 1 {
		footer += "s"
	}
	footer += ")\n"

	buf.WriteString(footer)

	return buf.String(), nil
}

// padRight pads a string to the right with spaces
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
