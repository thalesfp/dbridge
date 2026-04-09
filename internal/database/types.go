package database

import "time"

const maxSQLRows = 10000
const truncatedWarning = "Results limited to %d rows. Add LIMIT to your query to retrieve more."

// QueryResult represents the result of a SELECT query
type QueryResult struct {
	Columns     []string
	ColumnTypes []string // Short type names (e.g., "int4", "varchar", "timestamptz")
	Rows        [][]interface{}
	RowCount    int
	Duration    time.Duration
	Truncated   bool
	TotalRows   int
	Warnings    []string
}

// ExecResult represents the result of a write operation
type ExecResult struct {
	RowsAffected int64
	Duration     time.Duration
}

// Schema represents a database schema
type Schema struct {
	Name string
}

// Table represents a database table
type Table struct {
	Schema   string
	Name     string
	RowCount int64
}

// ColumnInfo represents a table column
type ColumnInfo struct {
	Name     string
	Type     string
	Nullable bool
	Default  *string
}

// IndexInfo represents a table index
type IndexInfo struct {
	Name    string
	Columns []string
	Unique  bool
	Primary bool
}

// ConstraintInfo represents a table constraint
type ConstraintInfo struct {
	Name string
	Type string // PRIMARY KEY, FOREIGN KEY, CHECK, UNIQUE
	Def  string
}

// TableDefinition represents complete table structure
type TableDefinition struct {
	Schema      string
	Name        string
	Columns     []ColumnInfo
	Indexes     []IndexInfo
	Constraints []ConstraintInfo
}

// ExplainResult represents query execution plan
type ExplainResult struct {
	Plan          string
	ActualRows    *int64
	ActualTime    *float64
	EstimatedCost float64
}
