package database

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestGetTypeName tests PostgreSQL type OID to name conversion
func TestGetTypeName(t *testing.T) {
	tests := []struct {
		oid      uint32
		expected string
	}{
		{16, "bool"},
		{20, "int8"},
		{21, "int2"},
		{23, "int4"},
		{25, "text"},
		{700, "float4"},
		{701, "float8"},
		{1043, "varchar"},
		{1082, "date"},
		{1083, "time"},
		{1114, "timestamp"},
		{1184, "timestamptz"},
		{1700, "numeric"},
		{2950, "uuid"},
		{114, "json"},
		{3802, "jsonb"},
		{9999, "oid_9999"}, // Unknown type
	}

	for _, test := range tests {
		result := getTypeName(test.oid)
		if result != test.expected {
			t.Errorf("getTypeName(%d) = %s, expected %s", test.oid, result, test.expected)
		}
	}
}

// TestConnectionConfig tests connection configuration
func TestConnectionConfig(t *testing.T) {
	config := &ConnectionConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
		Password: "secret",
		SSLMode:  "require",
	}

	if config.Host != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", config.Host)
	}

	if config.Port != 5432 {
		t.Errorf("Expected port 5432, got %d", config.Port)
	}

	if config.Database != "testdb" {
		t.Errorf("Expected database 'testdb', got '%s'", config.Database)
	}

	if config.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", config.Username)
	}

	if config.Password != "secret" {
		t.Errorf("Expected password 'secret', got '%s'", config.Password)
	}

	if config.SSLMode != "require" {
		t.Errorf("Expected SSLMode 'require', got '%s'", config.SSLMode)
	}
}

// TestBuildConnString_ReadOnly tests that the connection string includes
// default_transaction_read_only=on when ReadOnly is true.
func TestBuildConnString_ReadOnly(t *testing.T) {
	base := ConnectionConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "user",
		Password: "pass",
		SSLMode:  "disable",
	}

	t.Run("read-only appends param", func(t *testing.T) {
		cfg := base
		cfg.ReadOnly = true
		cs := buildConnString(&cfg)

		if !strings.Contains(cs, "default_transaction_read_only=on") {
			t.Errorf("Expected connection string to contain default_transaction_read_only=on, got: %s", cs)
		}
	})

	t.Run("read-write omits param", func(t *testing.T) {
		cfg := base
		cfg.ReadOnly = false
		cs := buildConnString(&cfg)

		if strings.Contains(cs, "default_transaction_read_only") {
			t.Errorf("Expected connection string to NOT contain default_transaction_read_only, got: %s", cs)
		}
	})
}

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

	// Test first column
	if def.Columns[0].Name != "id" {
		t.Errorf("Expected column name 'id', got '%s'", def.Columns[0].Name)
	}

	if def.Columns[0].Nullable {
		t.Error("Expected id column to be non-nullable")
	}

	// Test second column with default
	if def.Columns[1].Default == nil {
		t.Error("Expected email column to have a default value")
	}

	if *def.Columns[1].Default != "NULL" {
		t.Errorf("Expected default 'NULL', got '%s'", *def.Columns[1].Default)
	}

	// Test indexes
	if len(def.Indexes) != 2 {
		t.Errorf("Expected 2 indexes, got %d", len(def.Indexes))
	}

	if !def.Indexes[0].Primary {
		t.Error("Expected first index to be primary")
	}

	if !def.Indexes[0].Unique {
		t.Error("Expected primary key to be unique")
	}

	// Test constraints
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

// TestReadOnlyConnection_Integration verifies that a read-only connection
// rejects write operations at the database level.
// Set TEST_DATABASE_URL to run (e.g. postgres://user:pass@localhost:5432/testdb?sslmode=disable).
func TestReadOnlyConnection_Integration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()

	config := &ConnectionConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "postgres",
		Username: "postgres",
		Password: "postgres",
		SSLMode:  "disable",
		ReadOnly: true,
	}

	// Parse DSN to override defaults if provided in a structured format.
	// For simplicity, we use pgxpool to parse the DSN and extract fields.
	// But since our NewConnection builds its own string, we just use defaults
	// and let TEST_DATABASE_URL signal "a PG is available".
	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	// SELECT should succeed
	result, err := conn.Query(ctx, "SELECT 1 AS ok")
	if err != nil {
		t.Fatalf("SELECT should succeed on read-only connection: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("Expected 1 row, got %d", result.RowCount)
	}

	// Write operations should fail
	writeStatements := []string{
		"CREATE TABLE _dbridge_ro_test (id int)",
		"INSERT INTO _dbridge_ro_test VALUES (1)",
		"DROP TABLE IF EXISTS _dbridge_ro_test",
	}

	for _, stmt := range writeStatements {
		_, err := conn.Exec(ctx, stmt)
		if err == nil {
			t.Errorf("Expected error for %q on read-only connection, but it succeeded", stmt)
		} else if !strings.Contains(err.Error(), "read-only") {
			t.Errorf("Expected read-only error for %q, got: %v", stmt, err)
		}
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
