package database

import "testing"

// TestQueryResult tests query result structure
func TestQueryResult(t *testing.T) {
	result := &QueryResult{
		Columns:     []string{"id", "name", "email"},
		ColumnTypes: []string{"int4", "varchar", "varchar"},
		Rows: [][]interface{}{
			{1, "Alice", "alice@example.com"},
			{2, "Bob", "bob@example.com"},
		},
		RowCount: 2,
	}

	if len(result.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(result.Columns))
	}

	if len(result.ColumnTypes) != 3 {
		t.Errorf("Expected 3 column types, got %d", len(result.ColumnTypes))
	}

	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(result.Rows))
	}

	if result.RowCount != 2 {
		t.Errorf("Expected row count 2, got %d", result.RowCount)
	}

	// Test first row
	if result.Rows[0][0].(int) != 1 {
		t.Errorf("Expected first row id 1, got %v", result.Rows[0][0])
	}

	if result.Rows[0][1].(string) != "Alice" {
		t.Errorf("Expected first row name 'Alice', got %v", result.Rows[0][1])
	}

	// Test second row
	if result.Rows[1][0].(int) != 2 {
		t.Errorf("Expected second row id 2, got %v", result.Rows[1][0])
	}

	if result.Rows[1][2].(string) != "bob@example.com" {
		t.Errorf("Expected second row email 'bob@example.com', got %v", result.Rows[1][2])
	}
}

// TestExecResult tests execution result structure
func TestExecResult(t *testing.T) {
	result := &ExecResult{
		RowsAffected: 42,
	}

	if result.RowsAffected != 42 {
		t.Errorf("Expected rows affected 42, got %d", result.RowsAffected)
	}
}

// TestTableDefinition tests table definition structure
func TestTableDefinition(t *testing.T) {
	def := &TableDefinition{
		Schema: "public",
		Name:   "users",
		Columns: []ColumnInfo{
			{
				Name:     "id",
				Type:     "integer",
				Nullable: false,
				Default:  nil,
			},
			{
				Name:     "email",
				Type:     "varchar(255)",
				Nullable: false,
				Default:  stringPtr("NULL"),
			},
			{
				Name:     "active",
				Type:     "boolean",
				Nullable: true,
				Default:  stringPtr("true"),
			},
		},
		Indexes: []IndexInfo{
			{
				Name:    "users_pkey",
				Columns: []string{"id"},
				Unique:  true,
				Primary: true,
			},
			{
				Name:    "users_email_key",
				Columns: []string{"email"},
				Unique:  true,
				Primary: false,
			},
		},
		Constraints: []ConstraintInfo{
			{
				Name: "users_pkey",
				Type: "PRIMARY KEY",
				Def:  "PRIMARY KEY (id)",
			},
		},
	}

	if def.Schema != "public" {
		t.Errorf("Expected schema 'public', got '%s'", def.Schema)
	}

	if def.Name != "users" {
		t.Errorf("Expected table name 'users', got '%s'", def.Name)
	}

	if len(def.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(def.Columns))
	}

	if def.Columns[0].Name != "id" {
		t.Errorf("Expected column name 'id', got '%s'", def.Columns[0].Name)
	}

	if def.Columns[0].Nullable {
		t.Error("Expected id column to be non-nullable")
	}

	if def.Columns[1].Default == nil {
		t.Error("Expected email column to have a default value")
	}

	if *def.Columns[1].Default != "NULL" {
		t.Errorf("Expected default 'NULL', got '%s'", *def.Columns[1].Default)
	}

	if len(def.Indexes) != 2 {
		t.Errorf("Expected 2 indexes, got %d", len(def.Indexes))
	}

	if !def.Indexes[0].Primary {
		t.Error("Expected first index to be primary")
	}

	if !def.Indexes[0].Unique {
		t.Error("Expected primary key to be unique")
	}

	if len(def.Constraints) != 1 {
		t.Errorf("Expected 1 constraint, got %d", len(def.Constraints))
	}

	if def.Constraints[0].Type != "PRIMARY KEY" {
		t.Errorf("Expected constraint type 'PRIMARY KEY', got '%s'", def.Constraints[0].Type)
	}
}

// TestSchema tests schema structure
func TestSchema(t *testing.T) {
	schemas := []Schema{
		{Name: "public"},
		{Name: "analytics"},
		{Name: "staging"},
	}

	if len(schemas) != 3 {
		t.Errorf("Expected 3 schemas, got %d", len(schemas))
	}

	if schemas[0].Name != "public" {
		t.Errorf("Expected first schema 'public', got '%s'", schemas[0].Name)
	}

	if schemas[1].Name != "analytics" {
		t.Errorf("Expected second schema 'analytics', got '%s'", schemas[1].Name)
	}
}

// TestTable tests table structure
func TestTable(t *testing.T) {
	table := &Table{
		Schema:   "public",
		Name:     "users",
		RowCount: 1247,
	}

	if table.Schema != "public" {
		t.Errorf("Expected schema 'public', got '%s'", table.Schema)
	}

	if table.Name != "users" {
		t.Errorf("Expected table name 'users', got '%s'", table.Name)
	}

	if table.RowCount != 1247 {
		t.Errorf("Expected row count 1247, got %d", table.RowCount)
	}
}

// TestColumnInfo tests column info structure
func TestColumnInfo(t *testing.T) {
	col := ColumnInfo{
		Name:     "email",
		Type:     "varchar(255)",
		Nullable: false,
		Default:  stringPtr("''::character varying"),
	}

	if col.Name != "email" {
		t.Errorf("Expected column name 'email', got '%s'", col.Name)
	}

	if col.Type != "varchar(255)" {
		t.Errorf("Expected type 'varchar(255)', got '%s'", col.Type)
	}

	if col.Nullable {
		t.Error("Expected column to be non-nullable")
	}

	if col.Default == nil {
		t.Fatal("Expected default value to be set")
	}

	if *col.Default != "''::character varying" {
		t.Errorf("Expected default ''::character varying, got '%s'", *col.Default)
	}
}

// TestExplainResult tests explain result structure
func TestExplainResult(t *testing.T) {
	actualRows := int64(1000)
	actualTime := 15.234

	result := &ExplainResult{
		Plan:          "Seq Scan on users  (cost=0.00..18.50 rows=850 width=40)",
		ActualRows:    &actualRows,
		ActualTime:    &actualTime,
		EstimatedCost: 18.50,
	}

	if result.Plan == "" {
		t.Error("Expected plan to be set")
	}

	if result.ActualRows == nil {
		t.Fatal("Expected actual rows to be set")
	}

	if *result.ActualRows != 1000 {
		t.Errorf("Expected actual rows 1000, got %d", *result.ActualRows)
	}

	if result.ActualTime == nil {
		t.Fatal("Expected actual time to be set")
	}

	if *result.ActualTime != 15.234 {
		t.Errorf("Expected actual time 15.234, got %f", *result.ActualTime)
	}

	if result.EstimatedCost != 18.50 {
		t.Errorf("Expected estimated cost 18.50, got %f", result.EstimatedCost)
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
