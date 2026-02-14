package output

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/thalesgelinger/dbridge/internal/database"
)

// TestFormatCSV tests basic CSV formatting with header
func TestFormatCSV(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"id", "name", "email"},
		ColumnTypes: []string{"int4", "varchar", "varchar"},
		Rows: [][]interface{}{
			{1, "Alice", "alice@example.com"},
			{2, "Bob", "bob@example.com"},
			{3, "Charlie", "charlie@example.com"},
		},
		RowCount: 3,
		Duration: 100 * time.Millisecond,
	}

	output, err := FormatCSV(result, true)
	if err != nil {
		t.Fatalf("Failed to format CSV: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have 4 lines (1 header + 3 data rows)
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}

	// Check header
	if lines[0] != "id,name,email" {
		t.Errorf("Expected header 'id,name,email', got '%s'", lines[0])
	}

	// Check first data row
	if lines[1] != "1,Alice,alice@example.com" {
		t.Errorf("Unexpected first row: %s", lines[1])
	}
}

// TestFormatCSV_NoHeader tests CSV formatting without header
func TestFormatCSV_NoHeader(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"id", "name"},
		Rows: [][]interface{}{
			{1, "Alice"},
			{2, "Bob"},
		},
		RowCount: 2,
	}

	output, err := FormatCSV(result, false)
	if err != nil {
		t.Fatalf("Failed to format CSV without header: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have only 2 data lines (no header)
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines (no header), got %d", len(lines))
	}

	// First line should be data, not header
	if lines[0] == "id,name" {
		t.Error("Should not include header when includeHeader is false")
	}

	if lines[0] != "1,Alice" {
		t.Errorf("Expected first line '1,Alice', got '%s'", lines[0])
	}
}

// TestFormatCSV_EmptyResult tests CSV with no rows
func TestFormatCSV_EmptyResult(t *testing.T) {
	result := &database.QueryResult{
		Columns:  []string{"id", "name"},
		Rows:     [][]interface{}{},
		RowCount: 0,
	}

	output, err := FormatCSV(result, true)
	if err != nil {
		t.Fatalf("Failed to format empty CSV: %v", err)
	}

	// Should only have header
	if output != "id,name\n" {
		t.Errorf("Expected only header, got: %s", output)
	}
}

// TestFormatCSV_NullValues tests CSV handling of NULL values
func TestFormatCSV_NullValues(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"id", "name", "email"},
		Rows: [][]interface{}{
			{1, "Alice", "alice@example.com"},
			{2, nil, "bob@example.com"},
			{3, "Charlie", nil},
		},
		RowCount: 3,
	}

	output, err := FormatCSV(result, true)
	if err != nil {
		t.Fatalf("Failed to format CSV with NULLs: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Second row should have empty value for NULL
	if lines[2] != "2,,bob@example.com" {
		t.Errorf("Expected '2,,bob@example.com', got '%s'", lines[2])
	}

	// Third row should have empty value for NULL
	if lines[3] != "3,Charlie," {
		t.Errorf("Expected '3,Charlie,', got '%s'", lines[3])
	}
}

// TestFormatCSV_QuotedValues tests CSV quoting of special characters
func TestFormatCSV_QuotedValues(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"id", "description"},
		Rows: [][]interface{}{
			{1, "Simple text"},
			{2, "Text with, comma"},
			{3, "Text with \"quotes\""},
			{4, "Text with\nNewline"},
		},
		RowCount: 4,
	}

	output, err := FormatCSV(result, true)
	if err != nil {
		t.Fatalf("Failed to format CSV with special chars: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Value with comma should be quoted
	if !strings.Contains(lines[2], "\"Text with, comma\"") {
		t.Errorf("Comma should trigger quoting: %s", lines[2])
	}

	// Value with quotes should be quoted and escaped
	if !strings.Contains(lines[3], "\"Text with \"\"quotes\"\"\"") {
		t.Errorf("Quotes should be escaped: %s", lines[3])
	}

	// Newlines should be preserved in quotes
	if !strings.Contains(output, "\"Text with\nNewline\"") {
		t.Error("Newlines should be preserved in quoted values")
	}
}

// TestFormatCSV_BooleanValues tests boolean formatting
func TestFormatCSV_BooleanValues(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"id", "active", "verified"},
		Rows: [][]interface{}{
			{1, true, false},
			{2, false, true},
		},
		RowCount: 2,
	}

	output, err := FormatCSV(result, true)
	if err != nil {
		t.Fatalf("Failed to format CSV with booleans: %v", err)
	}

	if !strings.Contains(output, "true") {
		t.Error("CSV should contain 'true' for boolean true")
	}

	if !strings.Contains(output, "false") {
		t.Error("CSV should contain 'false' for boolean false")
	}
}

// TestFormatCSV_NumericTypes tests various numeric types
func TestFormatCSV_NumericTypes(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"int_val", "float_val", "big_int"},
		Rows: [][]interface{}{
			{42, 3.14159, int64(9223372036854775807)},
			{-10, -2.5, int64(-9223372036854775808)},
		},
		RowCount: 2,
	}

	output, err := FormatCSV(result, true)
	if err != nil {
		t.Fatalf("Failed to format CSV with numerics: %v", err)
	}

	if !strings.Contains(output, "42") {
		t.Error("CSV should contain integer value")
	}

	if !strings.Contains(output, "3.14159") {
		t.Error("CSV should contain float value")
	}

	if !strings.Contains(output, "9223372036854775807") {
		t.Error("CSV should contain big integer value")
	}
}

// TestFormatCSV_SingleColumn tests CSV with one column
func TestFormatCSV_SingleColumn(t *testing.T) {
	result := &database.QueryResult{
		Columns: []string{"email"},
		Rows: [][]interface{}{
			{"alice@example.com"},
			{"bob@example.com"},
			{"charlie@example.com"},
		},
		RowCount: 3,
	}

	output, err := FormatCSV(result, true)
	if err != nil {
		t.Fatalf("Failed to format single-column CSV: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have 4 lines (header + 3 data)
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}

	// Header should be just the column name
	if lines[0] != "email" {
		t.Errorf("Expected header 'email', got '%s'", lines[0])
	}
}

// TestFormatCSVValue tests the CSV value formatting function
func TestFormatCSVValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected string
	}{
		{nil, ""},
		{true, "true"},
		{false, "false"},
		{42, "42"},
		{int64(9223372036854775807), "9223372036854775807"},
		{3.14, "3.140000"},
		{float32(2.5), "2.500000"},
		{"hello", "hello"},
		{[]byte("bytes"), "bytes"},
	}

	for _, test := range tests {
		result := formatCSVValue(test.input)
		if result != test.expected {
			t.Errorf("formatCSVValue(%v) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

// TestFormatCSV_LargeDataset tests CSV with many rows
func TestFormatCSV_LargeDataset(t *testing.T) {
	// Create 1000 rows
	rows := make([][]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		rows[i] = []interface{}{i, fmt.Sprintf("name%d", i), i * 100}
	}

	result := &database.QueryResult{
		Columns:  []string{"id", "name", "value"},
		Rows:     rows,
		RowCount: 1000,
	}

	output, err := FormatCSV(result, true)
	if err != nil {
		t.Fatalf("Failed to format large CSV: %v", err)
	}

	// Check that output contains header
	if !strings.HasPrefix(output, "id,name,value\n") {
		t.Error("CSV should start with header")
	}

	// Count data rows (split by newline, filter empty, subtract header)
	lines := strings.Split(output, "\n")
	dataRows := 0
	for i, line := range lines {
		if i > 0 && line != "" { // Skip header and empty lines
			dataRows++
		}
	}

	// Should have 1000 data rows
	if dataRows != 1000 {
		t.Errorf("Expected 1000 data rows, got %d", dataRows)
	}
}
