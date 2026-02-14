package output

import (
	"strings"
	"testing"
	"time"

	"github.com/thalesgelinger/dbbridge/internal/database"
)

// TestFormatTable tests basic table formatting
func TestFormatTable(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"id", "name", "active"},
		ColumnTypes: []string{"int4", "varchar", "bool"},
		Rows: [][]interface{}{
			{1, "Alice", true},
			{2, "Bob", false},
			{3, "Charlie", true},
		},
		RowCount: 3,
		Duration: 100 * time.Millisecond,
	}

	output, err := FormatTable(result)
	if err != nil {
		t.Fatalf("Failed to format table: %v", err)
	}

	// Check that output contains expected elements
	if !strings.Contains(output, "id") {
		t.Error("Table should contain 'id' column header")
	}

	if !strings.Contains(output, "name") {
		t.Error("Table should contain 'name' column header")
	}

	if !strings.Contains(output, "Alice") {
		t.Error("Table should contain 'Alice'")
	}

	if !strings.Contains(output, "Bob") {
		t.Error("Table should contain 'Bob'")
	}

	if !strings.Contains(output, "(3 rows)") {
		t.Error("Table should show row count")
	}
}

// TestFormatTable_EmptyResult tests table formatting with no rows
func TestFormatTable_EmptyResult(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"id", "name"},
		ColumnTypes: []string{"int4", "varchar"},
		Rows:        [][]interface{}{},
		RowCount:    0,
		Duration:    100 * time.Millisecond,
	}

	output, err := FormatTable(result)
	if err != nil {
		t.Fatalf("Failed to format empty table: %v", err)
	}

	if !strings.Contains(output, "(0 rows)") {
		t.Error("Empty table should show '(0 rows)'")
	}

	if !strings.Contains(output, "id") || !strings.Contains(output, "name") {
		t.Error("Empty table should still show column headers")
	}
}

// TestFormatTable_SingleRow tests table with one row
func TestFormatTable_SingleRow(t *testing.T) {
	result := &database.QueryResult{
		Columns:  []string{"count"},
		Rows:     [][]interface{}{{42}},
		RowCount: 1,
	}

	output, err := FormatTable(result)
	if err != nil {
		t.Fatalf("Failed to format single-row table: %v", err)
	}

	if !strings.Contains(output, "(1 row)") {
		t.Error("Single row table should show '(1 row)' not '(1 rows)'")
	}

	if !strings.Contains(output, "42") {
		t.Error("Table should contain the value '42'")
	}
}

// TestFormatTable_NullValues tests handling of NULL values
func TestFormatTable_NullValues(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"id", "name", "email"},
		Rows: [][]interface{}{
			{1, "Alice", "alice@example.com"},
			{2, "Bob", nil},
			{3, nil, "charlie@example.com"},
		},
		RowCount: 3,
	}

	output, err := FormatTable(result)
	if err != nil {
		t.Fatalf("Failed to format table with NULLs: %v", err)
	}

	if !strings.Contains(output, "NULL") {
		t.Error("Table should display NULL for nil values")
	}

	// Count NULL occurrences (should be 2)
	nullCount := strings.Count(output, "NULL")
	if nullCount < 2 {
		t.Errorf("Expected at least 2 NULL values, found %d", nullCount)
	}
}

// TestFormatTable_BooleanValues tests boolean formatting
func TestFormatTable_BooleanValues(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"active", "verified"},
		Rows: [][]interface{}{
			{true, false},
			{false, true},
		},
		RowCount: 2,
	}

	output, err := FormatTable(result)
	if err != nil {
		t.Fatalf("Failed to format table with booleans: %v", err)
	}

	// Booleans should be formatted as 't' or 'f'
	if !strings.Contains(output, "t") {
		t.Error("Table should contain 't' for true")
	}

	if !strings.Contains(output, "f") {
		t.Error("Table should contain 'f' for false")
	}
}

// TestFormatTable_NumericTypes tests various numeric types
func TestFormatTable_NumericTypes(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"int_val", "float_val", "big_int"},
		Rows: [][]interface{}{
			{42, 3.14, int64(9223372036854775807)},
			{-10, -2.5, int64(-9223372036854775808)},
		},
		RowCount: 2,
	}

	output, err := FormatTable(result)
	if err != nil {
		t.Fatalf("Failed to format table with numerics: %v", err)
	}

	if !strings.Contains(output, "42") {
		t.Error("Table should contain integer value")
	}

	if !strings.Contains(output, "3.14") {
		t.Error("Table should contain float value")
	}
}

// TestFormatTableCompact tests compact table formatting
func TestFormatTableCompact(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"id", "name"},
		Rows: [][]interface{}{
			{1, "Alice"},
			{2, "Bob"},
		},
		RowCount: 2,
	}

	output, err := FormatTableCompact(result)
	if err != nil {
		t.Fatalf("Failed to format compact table: %v", err)
	}

	// Should contain column separator
	if !strings.Contains(output, "|") {
		t.Error("Compact table should use | as column separator")
	}

	// Should contain row separator
	if !strings.Contains(output, "-+-") {
		t.Error("Compact table should have separator line")
	}

	// Should contain data
	if !strings.Contains(output, "Alice") || !strings.Contains(output, "Bob") {
		t.Error("Compact table should contain data")
	}

	// Should show row count
	if !strings.Contains(output, "(2 rows)") {
		t.Error("Compact table should show row count")
	}
}

// TestFormatTableCompact_EmptyResult tests compact table with no rows
func TestFormatTableCompact_EmptyResult(t *testing.T) {
	result := &database.QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]interface{}{},
		RowCount: 0,
	}

	output, err := FormatTableCompact(result)
	if err != nil {
		t.Fatalf("Failed to format empty compact table: %v", err)
	}

	if output != "(0 rows)\n" {
		t.Errorf("Expected '(0 rows)\\n', got '%s'", output)
	}
}

// TestFormatTableCompact_WideColumns tests alignment with different column widths
func TestFormatTableCompact_WideColumns(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"id", "very_long_column_name", "x"},
		Rows: [][]interface{}{
			{1, "short", "y"},
			{2, "this is a much longer value", "z"},
		},
		RowCount: 2,
	}

	output, err := FormatTableCompact(result)
	if err != nil {
		t.Fatalf("Failed to format compact table with wide columns: %v", err)
	}

	// Check that columns are aligned
	lines := strings.Split(output, "\n")
	if len(lines) < 4 {
		t.Error("Compact table should have at least 4 lines (header, separator, 2 data rows)")
	}

	// All data lines should have the same structure
	headerParts := strings.Split(lines[0], "|")
	if len(headerParts) != 3 {
		t.Error("Header should have 3 columns")
	}
}

// TestFormatValue tests the value formatting function
func TestFormatValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{nil, "NULL"},
		{true, "t"},
		{false, "f"},
		{42, "42"},
		{int64(9223372036854775807), "9223372036854775807"},
		{3.14, "3.14"},
		{"hello", "hello"},
		{[]byte("bytes"), "bytes"},
	}

	for _, test := range tests {
		result := formatValue(test.input)
		if result != test.expected {
			t.Errorf("formatValue(%v) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

// TestPadRight tests string padding
func TestPadRight(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hello", 10, "hello     "},
		{"test", 4, "test"},
		{"x", 5, "x    "},
		{"", 3, "   "},
		{"toolong", 3, "toolong"}, // Doesn't truncate
	}

	for _, test := range tests {
		result := padRight(test.input, test.width)
		if result != test.expected {
			t.Errorf("padRight(%q, %d) = %q, expected %q", test.input, test.width, result, test.expected)
		}

		// Check length for cases where padding should occur
		if len(test.input) < test.width && len(result) != test.width {
			t.Errorf("padRight result should have length %d, got %d", test.width, len(result))
		}
	}
}
