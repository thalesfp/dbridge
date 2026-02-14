package output

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/thalesgelinger/dbridge/internal/database"
)

// TestFormatCompact_SingleValue tests formatting a single value result
func TestFormatCompact_SingleValue(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"count"},
		ColumnTypes: []string{"int8"},
		Rows: [][]interface{}{
			{1247},
		},
		RowCount: 1,
		Duration: 100 * time.Millisecond,
	}

	opts := FormatOptions{
		SmartSimplify: true,
	}

	output, err := FormatCompact(result, opts)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	// Should be just the value
	if output != "1247" {
		t.Errorf("Expected '1247', got '%s'", output)
	}
}

// TestFormatCompact_SingleColumn tests formatting a single column result
func TestFormatCompact_SingleColumn(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"email"},
		ColumnTypes: []string{"varchar"},
		Rows: [][]interface{}{
			{"alice@example.com"},
			{"bob@example.com"},
			{"charlie@example.com"},
		},
		RowCount: 3,
		Duration: 100 * time.Millisecond,
	}

	opts := FormatOptions{
		SmartSimplify: true,
	}

	output, err := FormatCompact(result, opts)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	// Should be an array of values
	var values []string
	if err := json.Unmarshal([]byte(output), &values); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if len(values) != 3 {
		t.Errorf("Expected 3 values, got %d", len(values))
	}

	if values[0] != "alice@example.com" {
		t.Errorf("Expected 'alice@example.com', got '%s'", values[0])
	}
}

// TestFormatCompact_SingleRow tests formatting a single row with multiple columns
func TestFormatCompact_SingleRow(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"total", "avg_age"},
		ColumnTypes: []string{"int8", "numeric"},
		Rows: [][]interface{}{
			{1247, 32.5},
		},
		RowCount: 1,
		Duration: 100 * time.Millisecond,
	}

	opts := FormatOptions{
		SmartSimplify: true,
	}

	output, err := FormatCompact(result, opts)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	// Should be an object
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(output), &obj); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if len(obj) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(obj))
	}

	if obj["total"].(float64) != 1247 {
		t.Errorf("Expected total 1247, got %v", obj["total"])
	}

	if obj["avg_age"].(float64) != 32.5 {
		t.Errorf("Expected avg_age 32.5, got %v", obj["avg_age"])
	}
}

// TestFormatCompact_Standard tests formatting a standard multi-row, multi-column result
func TestFormatCompact_Standard(t *testing.T) {
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

	opts := FormatOptions{
		IncludeTypes:  true,
		SmartSimplify: true,
	}

	output, err := FormatCompact(result, opts)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	// Should be compact format
	var compact CompactResult
	if err := json.Unmarshal([]byte(output), &compact); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if len(compact.Cols) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(compact.Cols))
	}

	if compact.Cols[0] != "id" || compact.Cols[1] != "name" || compact.Cols[2] != "active" {
		t.Errorf("Unexpected columns: %v", compact.Cols)
	}

	if len(compact.Types) != 3 {
		t.Errorf("Expected 3 types, got %d", len(compact.Types))
	}

	if len(compact.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(compact.Rows))
	}

	// Check first row
	if compact.Rows[0][0].(float64) != 1 {
		t.Errorf("Expected first row id 1, got %v", compact.Rows[0][0])
	}

	if compact.Rows[0][1].(string) != "Alice" {
		t.Errorf("Expected first row name 'Alice', got %v", compact.Rows[0][1])
	}
}

// TestFormatCompact_EmptyResult tests formatting an empty result
func TestFormatCompact_EmptyResult(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"id", "name"},
		ColumnTypes: []string{"int4", "varchar"},
		Rows:        [][]interface{}{},
		RowCount:    0,
		Duration:    100 * time.Millisecond,
	}

	opts := FormatOptions{
		SmartSimplify: true,
	}

	output, err := FormatCompact(result, opts)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	// Should be empty array
	if output != "[]" {
		t.Errorf("Expected '[]', got '%s'", output)
	}
}

// TestFormatCompact_WithTiming tests including timing information
func TestFormatCompact_WithTiming(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"id", "name"},
		ColumnTypes: []string{"int4", "varchar"},
		Rows: [][]interface{}{
			{1, "Alice"},
		},
		RowCount: 1,
		Duration: 245 * time.Millisecond,
	}

	opts := FormatOptions{
		IncludeTiming: true,
		SmartSimplify: false, // Force compact format
	}

	output, err := FormatCompact(result, opts)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	var compact CompactResult
	if err := json.Unmarshal([]byte(output), &compact); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if compact.T == nil {
		t.Error("Expected timing to be included")
	}

	if *compact.T != 245 {
		t.Errorf("Expected timing 245ms, got %dms", *compact.T)
	}
}

// TestFormatCompact_WithWarnings tests including warnings
func TestFormatCompact_WithWarnings(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"id"},
		ColumnTypes: []string{"int4"},
		Rows: [][]interface{}{
			{1},
		},
		RowCount: 1,
		Duration: 100 * time.Millisecond,
		Warnings: []string{"Sequential scan on large table"},
	}

	opts := FormatOptions{
		IncludeWarnings: true,
		SmartSimplify:   false,
	}

	output, err := FormatCompact(result, opts)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	var compact CompactResult
	if err := json.Unmarshal([]byte(output), &compact); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if len(compact.W) != 1 {
		t.Errorf("Expected 1 warning, got %d", len(compact.W))
	}

	if compact.W[0] != "Sequential scan on large table" {
		t.Errorf("Unexpected warning: %s", compact.W[0])
	}
}

// TestFormatCompact_WithoutTypes tests excluding types
func TestFormatCompact_WithoutTypes(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"id", "name"},
		ColumnTypes: []string{"int4", "varchar"},
		Rows: [][]interface{}{
			{1, "Alice"},
		},
		RowCount: 1,
		Duration: 100 * time.Millisecond,
	}

	opts := FormatOptions{
		IncludeTypes:  false,
		SmartSimplify: false,
	}

	output, err := FormatCompact(result, opts)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	var compact CompactResult
	if err := json.Unmarshal([]byte(output), &compact); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if len(compact.Types) != 0 {
		t.Error("Expected types to be excluded")
	}
}

// TestFormatError tests error formatting
func TestFormatError(t *testing.T) {
	err := &testError{msg: "column \"emai\" does not exist"}
	hint := "Perhaps you meant \"email\""
	code := "42703"
	position := 8

	output, formatErr := FormatError(err, &hint, &code, &position)
	if formatErr != nil {
		t.Fatalf("Failed to format error: %v", formatErr)
	}

	var errResult ErrorResult
	if err := json.Unmarshal([]byte(output), &errResult); err != nil {
		t.Fatalf("Failed to parse error output: %v", err)
	}

	if errResult.Error != "column \"emai\" does not exist" {
		t.Errorf("Unexpected error message: %s", errResult.Error)
	}

	if errResult.Hint == nil || *errResult.Hint != hint {
		t.Errorf("Unexpected hint: %v", errResult.Hint)
	}

	if errResult.Code == nil || *errResult.Code != code {
		t.Errorf("Unexpected code: %v", errResult.Code)
	}

	if errResult.Pos == nil || *errResult.Pos != position {
		t.Errorf("Unexpected position: %v", errResult.Pos)
	}
}

// TestFormatExecResult tests execution result formatting
func TestFormatExecResult(t *testing.T) {
	result := &database.ExecResult{
		RowsAffected: 42,
		Duration:     150 * time.Millisecond,
	}

	opts := FormatOptions{
		IncludeTiming: true,
	}

	output, err := FormatExecResult(result, opts)
	if err != nil {
		t.Fatalf("Failed to format exec result: %v", err)
	}

	var jsonResult map[string]interface{}
	if err := json.Unmarshal([]byte(output), &jsonResult); err != nil {
		t.Fatalf("Failed to parse output: %v", err)
	}

	if jsonResult["rows_affected"].(float64) != 42 {
		t.Errorf("Expected rows_affected 42, got %v", jsonResult["rows_affected"])
	}

	if jsonResult["t"].(float64) != 150 {
		t.Errorf("Expected timing 150ms, got %v", jsonResult["t"])
	}
}

// TestTokenSavings tests that compact format actually saves tokens
func TestTokenSavings(t *testing.T) {
	result := &database.QueryResult{
		Columns:     []string{"id", "email", "created_at"},
		ColumnTypes: []string{"int4", "varchar", "timestamptz"},
		Rows: [][]interface{}{
			{1, "alice@example.com", "2025-01-15T10:23:11Z"},
			{2, "bob@example.com", "2025-01-16T14:45:33Z"},
			{3, "charlie@example.com", "2025-01-17T09:12:00Z"},
		},
		RowCount: 3,
		Duration: 100 * time.Millisecond,
	}

	opts := FormatOptions{
		IncludeTypes:    true,
		IncludeTiming:   false,
		IncludeWarnings: false,
		SmartSimplify:   true,
	}

	output, err := FormatCompact(result, opts)
	if err != nil {
		t.Fatalf("Failed to format: %v", err)
	}

	// Compact output should be significantly shorter than verbose JSON
	// Rough estimate: compact should be < 300 chars for this dataset
	if len(output) > 300 {
		t.Errorf("Compact output too long: %d chars (expected < 300)", len(output))
	}

	// Verify it's valid JSON
	var parsed interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Should contain key structural elements
	if !strings.Contains(output, "cols") {
		t.Error("Expected 'cols' in output")
	}

	if !strings.Contains(output, "rows") {
		t.Error("Expected 'rows' in output")
	}
}

// testError implements error interface for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
