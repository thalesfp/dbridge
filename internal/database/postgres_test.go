package database

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
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

// TestBuildPgConnString_AlwaysReadOnly tests that the connection string always
// includes default_transaction_read_only=on.
func TestBuildPgConnString_AlwaysReadOnly(t *testing.T) {
	cfg := ConnectionConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "user",
		Password: "pass",
		SSLMode:  "disable",
	}
	cs := buildPgConnString(&cfg)
	if !strings.Contains(cs, "default_transaction_read_only=on") {
		t.Errorf("connection string must always contain default_transaction_read_only=on, got: %s", cs)
	}
}

// TestBuildPgConnString_EscapesMetacharacters verifies that credentials and
// fields containing URL metacharacters are escaped, so they round-trip through
// the parser instead of breaking the DSN or injecting parameters (e.g. a
// database name that smuggles in sslmode=disable / read_only=off).
func TestBuildPgConnString_EscapesMetacharacters(t *testing.T) {
	cfg := ConnectionConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "mydb?sslmode=disable&default_transaction_read_only=off",
		Username: "ad min",
		Password: "p@ss:w/ord?&x=1",
		SSLMode:  "verify-full",
	}

	pc, err := pgxpool.ParseConfig(buildPgConnString(&cfg))
	if err != nil {
		t.Fatalf("connection string must parse, got error: %v", err)
	}
	if pc.ConnConfig.User != cfg.Username {
		t.Errorf("username = %q, want %q", pc.ConnConfig.User, cfg.Username)
	}
	if pc.ConnConfig.Password != cfg.Password {
		t.Errorf("password = %q, want %q", pc.ConnConfig.Password, cfg.Password)
	}
	if pc.ConnConfig.Database != cfg.Database {
		t.Errorf("database = %q, want %q", pc.ConnConfig.Database, cfg.Database)
	}
	if got := pc.ConnConfig.RuntimeParams["default_transaction_read_only"]; got != "on" {
		t.Errorf("default_transaction_read_only = %q, want \"on\" (injection must not override it)", got)
	}
	if pc.ConnConfig.TLSConfig == nil {
		t.Error("TLS must remain enabled for verify-full; injection must not disable it")
	}
}

// TestBuildPgConnString_PasswordlessDoesNotInheritPGPASSWORD verifies that a
// connection configured without a password sends an explicit empty password
// rather than falling back to the ambient PGPASSWORD. pgx only treats the URL
// password as supplied when the userinfo carries a password component, so the
// builder must always emit one (user:@host), never bare userinfo (user@host).
func TestBuildPgConnString_PasswordlessDoesNotInheritPGPASSWORD(t *testing.T) {
	t.Setenv("PGPASSWORD", "ambient-secret")

	cfg := ConnectionConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "mydb",
		Username: "app",
		Password: "",
		SSLMode:  "disable",
	}

	pc, err := pgxpool.ParseConfig(buildPgConnString(&cfg))
	if err != nil {
		t.Fatalf("connection string must parse, got error: %v", err)
	}
	if pc.ConnConfig.Password != "" {
		t.Errorf("passwordless connection resolved password = %q, want \"\" (must not inherit PGPASSWORD)", pc.ConnConfig.Password)
	}
}

// pgTestConfig parses TEST_DATABASE_URL into a ConnectionConfig.
func pgTestConfig(t *testing.T) *ConnectionConfig {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("Failed to parse TEST_DATABASE_URL: %v", err)
	}
	host := u.Hostname()
	port, _ := strconv.Atoi(u.Port())
	if port == 0 {
		port = 5432
	}
	database := strings.TrimPrefix(u.Path, "/")
	username := u.User.Username()
	password, _ := u.User.Password()
	sslMode := u.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}
	return &ConnectionConfig{
		Driver:   "postgres",
		Host:     host,
		Port:     port,
		Database: database,
		Username: username,
		Password: password,
		SSLMode:  sslMode,
	}
}

// TestReadOnlyConnection_Integration verifies that a read-only connection
// rejects write operations at the database level.
func TestReadOnlyConnection_Integration(t *testing.T) {
	config := pgTestConfig(t)
	ctx := context.Background()

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

	// Writes attempted via Query must be rejected by the read-only transaction
	// (default_transaction_read_only=on), proving DB-level enforcement.
	writeStatements := []string{
		"CREATE TABLE _dbridge_ro_test (id int)",
		"DROP TABLE IF EXISTS _dbridge_ro_test",
	}

	for _, stmt := range writeStatements {
		_, err := conn.Query(ctx, stmt)
		if err == nil {
			t.Errorf("Expected error for %q on read-only connection, but it succeeded", stmt)
		} else if !strings.Contains(err.Error(), "read-only") && !strings.Contains(err.Error(), "read only") {
			t.Errorf("Expected read-only error for %q, got: %v", stmt, err)
		}
	}

	// Exec is always refused at the application layer.
	if _, err := conn.Exec(ctx, "CREATE TABLE _dbridge_ro_test (id int)"); err == nil {
		t.Error("Expected Exec to be rejected on read-only connection")
	}
}

// TestPgQueryWithData_Integration tests querying real fixture data.
func TestPgQueryWithData_Integration(t *testing.T) {
	config := pgTestConfig(t)
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

	// Verify type mapping produces short names
	expectedTypes := []string{"int4", "varchar", "varchar", "bool", "int4"}
	for i, typ := range expectedTypes {
		if result.ColumnTypes[i] != typ {
			t.Errorf("Column %q type: expected %q, got %q", expectedCols[i], typ, result.ColumnTypes[i])
		}
	}
}

// TestPgSchemaInspection_Integration tests schema inspector against fixture data.
func TestPgSchemaInspection_Integration(t *testing.T) {
	config := pgTestConfig(t)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	inspector := conn.Schema()

	// ListSchemas should include public and analytics
	schemas, err := inspector.ListSchemas(ctx)
	if err != nil {
		t.Fatalf("ListSchemas failed: %v", err)
	}
	found := map[string]bool{}
	for _, s := range schemas {
		found[s.Name] = true
	}
	for _, want := range []string{"public", "analytics"} {
		if !found[want] {
			t.Errorf("Expected schema %q in list, got %v", want, schemas)
		}
	}

	// ListTables in public should include users and orders
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
			t.Errorf("Expected table %q in public schema, got %v", want, tables)
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

// TestPgExplainQuery_Integration tests EXPLAIN returns a plan.
func TestPgExplainQuery_Integration(t *testing.T) {
	config := pgTestConfig(t)
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

// TestPgQueryTruncation_Integration verifies that queries returning more than
// maxSQLRows rows set Truncated=true, cap RowCount, and include a warning.
func TestPgQueryTruncation_Integration(t *testing.T) {
	config := pgTestConfig(t)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	result, err := conn.Query(ctx, "SELECT * FROM large_table")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if !result.Truncated {
		t.Fatal("expected Truncated=true for query returning >10000 rows")
	}
	if result.RowCount != maxSQLRows {
		t.Errorf("expected RowCount=%d, got %d", maxSQLRows, result.RowCount)
	}
	if result.TotalRows != 0 {
		t.Errorf("TotalRows should be 0 (unknown), got %d", result.TotalRows)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected truncation warning in result")
	}
	if !strings.Contains(result.Warnings[0], "10000") {
		t.Errorf("warning should mention row cap, got: %q", result.Warnings[0])
	}
}
