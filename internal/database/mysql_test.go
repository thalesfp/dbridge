package database

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestBuildMysqlDSN(t *testing.T) {
	tests := []struct {
		name     string
		config   ConnectionConfig
		expected string
	}{
		{
			name: "basic config",
			config: ConnectionConfig{
				Host:     "localhost",
				Port:     3306,
				Database: "mydb",
				Username: "root",
				Password: "secret",
				SSLMode:  "disable",
			},
			expected: "root:secret@tcp(localhost:3306)/mydb?parseTime=true&tls=false",
		},
		{
			name: "prefer ssl",
			config: ConnectionConfig{
				Host:     "db.example.com",
				Port:     3307,
				Database: "app",
				Username: "admin",
				Password: "pass",
				SSLMode:  "prefer",
			},
			expected: "admin:pass@tcp(db.example.com:3307)/app?parseTime=true&tls=preferred",
		},
		{
			name: "require ssl",
			config: ConnectionConfig{
				Host:     "db.example.com",
				Port:     3306,
				Database: "app",
				Username: "admin",
				Password: "pass",
				SSLMode:  "require",
			},
			expected: "admin:pass@tcp(db.example.com:3306)/app?parseTime=true&tls=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := buildMysqlDSN(&tt.config)
			if dsn != tt.expected {
				t.Errorf("buildMysqlDSN() = %q, want %q", dsn, tt.expected)
			}
		})
	}
}

func TestBuildMysqlDSN_SpecialChars(t *testing.T) {
	config := &ConnectionConfig{
		Host:     "localhost",
		Port:     3306,
		Database: "mydb",
		Username: "user",
		Password: "p@ss:w/rd?",
		SSLMode:  "disable",
	}

	dsn := buildMysqlDSN(config)

	// Verify the DSN round-trips through the driver's parser
	parsed, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("FormatDSN produced unparseable DSN: %v", err)
	}
	if parsed.User != config.Username {
		t.Errorf("Username: got %q, want %q", parsed.User, config.Username)
	}
	if parsed.Passwd != config.Password {
		t.Errorf("Password: got %q, want %q", parsed.Passwd, config.Password)
	}
	if parsed.DBName != config.Database {
		t.Errorf("DBName: got %q, want %q", parsed.DBName, config.Database)
	}
}

func TestMapSSLToMysqlTLS(t *testing.T) {
	tests := []struct {
		ssl      string
		expected string
	}{
		{"disable", "false"},
		{"prefer", "preferred"},
		{"preferred", "preferred"},
		{"require", "true"},
		{"verify-ca", "true"},
		{"verify-full", "true"},
		{"", "preferred"},          // unknown defaults to preferred
		{"something", "preferred"}, // unknown defaults to preferred
	}

	for _, tt := range tests {
		t.Run(tt.ssl, func(t *testing.T) {
			result := mapSSLToMysqlTLS(tt.ssl)
			if result != tt.expected {
				t.Errorf("mapSSLToMysqlTLS(%q) = %q, want %q", tt.ssl, result, tt.expected)
			}
		})
	}
}

func TestMysqlTypeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"INT", "int4"},
		{"INTEGER", "int4"},
		{"BIGINT", "int8"},
		{"SMALLINT", "int2"},
		{"TINYINT", "int1"},
		{"MEDIUMINT", "int3"},
		{"FLOAT", "float4"},
		{"DOUBLE", "float8"},
		{"DECIMAL(10,2)", "numeric"},
		{"VARCHAR(255)", "varchar"},
		{"CHAR(10)", "char"},
		{"TEXT", "text"},
		{"LONGTEXT", "longtext"},
		{"BLOB", "blob"},
		{"DATE", "date"},
		{"DATETIME", "datetime"},
		{"TIMESTAMP", "timestamp"},
		{"JSON", "json"},
		{"ENUM", "enum"},
		{"SET", "set"},
		{"BIT", "bit"},
		{"BOOLEAN", "bool"},
		{"INT UNSIGNED", "int4"},
		{"BIGINT UNSIGNED", "int8"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mysqlTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("mysqlTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMysqlDriverRegistered(t *testing.T) {
	names := DriverNames()
	found := false
	for _, n := range names {
		if n == "mysql" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'mysql' in DriverNames(), got %v", names)
	}
}

// mysqlTestConfig parses TEST_MYSQL_URL into a ConnectionConfig.
func mysqlTestConfig(t *testing.T, readOnly bool) *ConnectionConfig {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dsn := os.Getenv("TEST_MYSQL_URL")
	if dsn == "" {
		t.Skip("TEST_MYSQL_URL not set, skipping integration test")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("Failed to parse TEST_MYSQL_URL: %v", err)
	}
	host := cfg.Addr
	port := 3306
	if parts := strings.SplitN(cfg.Addr, ":", 2); len(parts) == 2 {
		host = parts[0]
		port, _ = strconv.Atoi(parts[1])
	}
	return &ConnectionConfig{
		Driver:   "mysql",
		Host:     host,
		Port:     port,
		Database: cfg.DBName,
		Username: cfg.User,
		Password: cfg.Passwd,
		SSLMode:  "disable",
		ReadOnly: readOnly,
	}
}

// TestMysqlConnection_Integration tests basic MySQL connectivity.
func TestMysqlConnection_Integration(t *testing.T) {
	config := mysqlTestConfig(t, true)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	// SELECT should succeed
	result, err := conn.Query(ctx, "SELECT 1 AS ok")
	if err != nil {
		t.Fatalf("SELECT should succeed: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("Expected 1 row, got %d", result.RowCount)
	}
}

// TestMysqlReadOnly_Integration tests that read-only mode blocks writes.
func TestMysqlReadOnly_Integration(t *testing.T) {
	config := mysqlTestConfig(t, true)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	// Write should fail
	_, err = conn.Exec(ctx, "CREATE TABLE _dbridge_ro_test (id INT)")
	if err == nil {
		_, _ = conn.Exec(ctx, "DROP TABLE _dbridge_ro_test")
		t.Fatal("Expected error for write on read-only connection")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "read only") &&
		!strings.Contains(strings.ToLower(err.Error()), "read-only") {
		t.Errorf("Expected read-only error, got: %v", err)
	}
}

// TestMysqlQueryWithData_Integration tests querying real fixture data.
func TestMysqlQueryWithData_Integration(t *testing.T) {
	config := mysqlTestConfig(t, true)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	result, err := conn.Query(ctx, "SELECT id, name, email, active, age FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result.RowCount != 10 {
		t.Errorf("Expected 10 users, got %d", result.RowCount)
	}

	expectedCols := []string{"id", "name", "email", "active", "age"}
	if len(result.Columns) != len(expectedCols) {
		t.Fatalf("Expected %d columns, got %d", len(expectedCols), len(result.Columns))
	}
	for i, col := range expectedCols {
		if result.Columns[i] != col {
			t.Errorf("Column %d: expected %q, got %q", i, col, result.Columns[i])
		}
	}

	// MySQL BOOLEAN is TINYINT(1), so active maps to int1
	expectedTypes := []string{"int4", "varchar", "varchar", "int1", "int4"}
	for i, typ := range expectedTypes {
		if result.ColumnTypes[i] != typ {
			t.Errorf("Column %q type: expected %q, got %q", expectedCols[i], typ, result.ColumnTypes[i])
		}
	}
}

// TestMysqlSchemaInspection_Integration tests schema inspector against fixture data.
func TestMysqlSchemaInspection_Integration(t *testing.T) {
	config := mysqlTestConfig(t, true)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	inspector := conn.Schema()

	// ListSchemas returns the connected database name
	schemas, err := inspector.ListSchemas(ctx)
	if err != nil {
		t.Fatalf("ListSchemas failed: %v", err)
	}
	if len(schemas) != 1 || schemas[0].Name != config.Database {
		t.Errorf("Expected single schema %q, got %v", config.Database, schemas)
	}

	// ListTables should include users and orders
	tables, err := inspector.ListTables(ctx, "public")
	if err != nil {
		t.Fatalf("ListTables failed: %v", err)
	}
	tableNames := map[string]bool{}
	for _, tbl := range tables {
		tableNames[tbl.Name] = true
	}
	for _, want := range []string{"users", "orders"} {
		if !tableNames[want] {
			t.Errorf("Expected table %q, got %v", want, tables)
		}
	}

	// DescribeTable for users
	def, err := inspector.DescribeTable(ctx, "public", "users")
	if err != nil {
		t.Fatalf("DescribeTable failed: %v", err)
	}
	if def.Name != "users" {
		t.Errorf("Expected table name 'users', got %q", def.Name)
	}
	if len(def.Columns) < 5 {
		t.Errorf("Expected at least 5 columns in users, got %d", len(def.Columns))
	}
}

// TestMysqlExplainQuery_Integration tests EXPLAIN returns a plan.
func TestMysqlExplainQuery_Integration(t *testing.T) {
	config := mysqlTestConfig(t, true)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	result, err := conn.Schema().ExplainQuery(ctx, "SELECT * FROM users WHERE id = 1")
	if err != nil {
		t.Fatalf("ExplainQuery failed: %v", err)
	}
	if result.Plan == "" {
		t.Error("Expected non-empty explain plan")
	}
}

// TestMysqlConnectionPinning_Integration verifies multiple queries use the same pinned connection.
func TestMysqlConnectionPinning_Integration(t *testing.T) {
	config := mysqlTestConfig(t, true)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	// Run multiple queries; all should succeed on the pinned connection
	for i := 0; i < 5; i++ {
		result, err := conn.Query(ctx, "SELECT 1 AS ok")
		if err != nil {
			t.Fatalf("Query %d failed: %v", i, err)
		}
		if result.RowCount != 1 {
			t.Errorf("Query %d: expected 1 row, got %d", i, result.RowCount)
		}
	}

	// Verify read-only is still enforced after multiple queries
	_, err = conn.Exec(ctx, "CREATE TABLE _dbridge_pin_test (id INT)")
	if err == nil {
		_, _ = conn.Exec(ctx, "DROP TABLE _dbridge_pin_test")
		t.Fatal("Expected read-only error after multiple queries")
	}
}
