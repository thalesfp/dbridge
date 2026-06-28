package database

import (
	"context"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestBuildMssqlDSN(t *testing.T) {
	config := &ConnectionConfig{
		Host:     "localhost",
		Port:     1433,
		Database: "mydb",
		Username: "sa",
		Password: "secret",
		SSLMode:  "disable",
	}

	dsn := buildMssqlDSN(config)

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("buildMssqlDSN produced unparseable URL: %v", err)
	}
	if u.Scheme != "sqlserver" {
		t.Errorf("scheme: got %q, want %q", u.Scheme, "sqlserver")
	}
	if u.Host != "localhost:1433" {
		t.Errorf("host: got %q, want %q", u.Host, "localhost:1433")
	}
	user := u.User.Username()
	pass, _ := u.User.Password()
	if user != "sa" || pass != "secret" {
		t.Errorf("credentials: got %q/%q, want sa/secret", user, pass)
	}

	q := u.Query()
	if q.Get("database") != "mydb" {
		t.Errorf("database param: got %q, want %q", q.Get("database"), "mydb")
	}
	if q.Get("applicationintent") != "ReadOnly" {
		t.Errorf("applicationintent: got %q, want %q", q.Get("applicationintent"), "ReadOnly")
	}
	if q.Get("encrypt") != "disable" {
		t.Errorf("encrypt: got %q, want %q", q.Get("encrypt"), "disable")
	}
}

func TestBuildMssqlDSN_SpecialChars(t *testing.T) {
	config := &ConnectionConfig{
		Host:     "db.example.com",
		Port:     1433,
		Database: "app",
		Username: "user",
		Password: "p@ss:w/rd?&=",
		SSLMode:  "require",
	}

	dsn := buildMssqlDSN(config)

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("buildMssqlDSN produced unparseable URL: %v", err)
	}
	pass, _ := u.User.Password()
	if pass != config.Password {
		t.Errorf("password round-trip: got %q, want %q", pass, config.Password)
	}
}

func TestMapSSLToMssql(t *testing.T) {
	tests := []struct {
		ssl         string
		wantEncrypt string
		wantTrust   string
	}{
		{"disable", "disable", ""},
		{"prefer", "false", ""},
		{"preferred", "false", ""},
		{"require", "true", "true"},
		{"verify-ca", "true", "false"},
		{"verify-full", "true", "false"},
		{"", "true", "false"},      // unknown defaults to secure
		{"bogus", "true", "false"}, // unknown defaults to secure
	}

	for _, tt := range tests {
		t.Run(tt.ssl, func(t *testing.T) {
			gotEnc, gotTrust := mapSSLToMssql(tt.ssl)
			if gotEnc != tt.wantEncrypt || gotTrust != tt.wantTrust {
				t.Errorf("mapSSLToMssql(%q) = (%q,%q), want (%q,%q)",
					tt.ssl, gotEnc, gotTrust, tt.wantEncrypt, tt.wantTrust)
			}
		})
	}
}

func TestMssqlTypeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"INT", "int4"},
		{"BIGINT", "int8"},
		{"SMALLINT", "int2"},
		{"TINYINT", "int1"},
		{"BIT", "bool"},
		{"DECIMAL(10,2)", "numeric"},
		{"NUMERIC", "numeric"},
		{"MONEY", "numeric"},
		{"SMALLMONEY", "numeric"},
		{"FLOAT", "float8"},
		{"REAL", "float4"},
		{"CHAR(10)", "char"},
		{"NCHAR(10)", "nchar"}, // Unicode identity preserved
		{"VARCHAR(255)", "varchar"},
		{"NVARCHAR(MAX)", "nvarchar"}, // Unicode identity preserved + length stripped
		{"TEXT", "text"},
		{"NTEXT", "ntext"},
		{"DATE", "date"},
		{"DATETIME2", "datetime2"},
		{"DATETIMEOFFSET", "datetimeoffset"},
		{"UNIQUEIDENTIFIER", "uuid"},
		{"ROWVERSION", "rowversion"},
		{"TIMESTAMP", "rowversion"}, // SQL Server timestamp is a rowversion alias
		{"VARBINARY(MAX)", "varbinary"},
		{"XML", "xml"},
		{"HIERARCHYID", "hierarchyid"},
		{"GEOGRAPHY", "geography"},
		{"GEOMETRY", "geometry"},
		{"SQL_VARIANT", "sql_variant"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mssqlTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("mssqlTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMssqlCheckReadOnly(t *testing.T) {
	allowed := []string{
		"SELECT 1",
		"select id, name from users where id = 1",
		"  SELECT * FROM t  ",
		"WITH cte AS (SELECT id FROM users) SELECT * FROM cte",
		"VALUES (1), (2)",
		"SELECT 'into' AS word",                           // keyword inside string literal
		"SELECT [insert] FROM t",                          // keyword as bracketed identifier
		"SELECT \"update\" FROM t",                        // keyword as quoted identifier
		"SELECT 1 -- update this later",                   // keyword inside line comment
		"SELECT 1 /* drop everything */",                  // keyword inside block comment
		"SELECT 1;",                                       // trailing semicolon
		"SELECT 1 SELECT 2",                               // semicolon-less all-read batch, harmless
		"(SELECT 1)",                                      // parenthesized leading select
		";WITH cte AS (SELECT 1 AS id) SELECT * FROM cte", // leading ;WITH idiom
		"SELECT id FROM t ORDER BY id OFFSET 10 ROWS FETCH NEXT 5 ROWS ONLY", // pagination
		"SELECT ';' AS x",                // semicolon inside a string literal is not a separator
		"SELECT next, value FROM t",      // columns named next/value (no FOR) stay allowed
		"SELECT updlock FROM t",          // lock-hint words are not reserved; valid as identifiers
		"SELECT * FROM dbo.xlock",        // a table named xlock is a valid read
		"SELECT * FROM t WITH (UPDLOCK)", // lock hints are intentionally not policed (see mssqlWriteTokens)
	}
	for _, q := range allowed {
		t.Run("allow/"+q, func(t *testing.T) {
			if err := mssqlCheckReadOnly(q); err != nil {
				t.Errorf("expected %q to be allowed, got: %v", q, err)
			}
		})
	}

	rejected := []string{
		"",
		"INSERT INTO users (id) VALUES (1)",
		"UPDATE users SET name = 'x'",
		"DELETE FROM users",
		"MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN UPDATE SET t.x = s.x",
		"DROP TABLE users",
		"TRUNCATE TABLE users",
		"CREATE TABLE t (id INT)",
		"ALTER TABLE t ADD c INT",
		"SELECT * INTO new_table FROM users", // SELECT...INTO creates a table
		"WITH cte AS (SELECT 1 AS id) INSERT INTO t SELECT id FROM cte",             // CTE write
		"SELECT 1; DROP TABLE users",                                                // stacked with semicolon
		"SELECT 1 DROP TABLE users",                                                 // stacked without semicolon
		"DECLARE @s nvarchar(max) = N'UPDATE users SET x=1'; EXEC sp_executesql @s", // bypass
		"EXEC sp_who",
		"EXECUTE some_proc",
		"GRANT SELECT ON users TO bob",
		"DBCC CHECKDB",
		"BACKUP DATABASE mydb TO DISK = 'x'",
		"SELECT 1; SELECT 2",                          // multi-statement batch (separator)
		"SELECT 1; USE master",                        // session command stacked after read
		"SELECT 1; WAITFOR DELAY '00:00:05'",          // DoS stacked after read
		"SELECT 1; KILL 52",                           // kills another session
		"SELECT 1; SET LOCK_TIMEOUT 0",                // session setting stacked after read
		"SELECT 1; BEGIN TRANSACTION",                 // transaction control stacked after read
		"SELECT 1 WAITFOR DELAY '00:00:05'",           // dangerous verb without a separator
		"SELECT 1 KILL 52",                            // dangerous verb without a separator
		"SELECT * FROM OPENQUERY(linked, 'SELECT 1')", // distributed query can hide writes
		"SELECT NEXT VALUE FOR dbo.seq",               // advances a sequence (state change)
		"SELECT NEXT VALUE FOR seq AS n",              // sequence advance in any position
	}
	for _, q := range rejected {
		t.Run("reject/"+q, func(t *testing.T) {
			if err := mssqlCheckReadOnly(q); err == nil {
				t.Errorf("expected %q to be rejected, but it was allowed", q)
			}
		})
	}
}

func TestMssqlSchemaOrDefault(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", "dbo"},
		{"public", "dbo"},
		{"dbo", "dbo"},
		{"sales", "sales"},
	}
	for _, tt := range tests {
		if got := mssqlSchemaOrDefault(tt.in); got != tt.want {
			t.Errorf("mssqlSchemaOrDefault(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMssqlDriverRegistered(t *testing.T) {
	names := DriverNames()
	found := false
	for _, n := range names {
		if n == "mssql" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'mssql' in DriverNames(), got %v", names)
	}
}

// mssqlTestConfig builds a ConnectionConfig from TEST_MSSQL_URL.
func mssqlTestConfig(t *testing.T) *ConnectionConfig {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	dsn := os.Getenv("TEST_MSSQL_URL")
	if dsn == "" {
		t.Skip("TEST_MSSQL_URL not set, skipping integration test")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("Failed to parse TEST_MSSQL_URL: %v", err)
	}
	host := u.Hostname()
	port := 1433
	if p := u.Port(); p != "" {
		parsed, scanErr := strconv.Atoi(p)
		if scanErr != nil {
			t.Fatalf("invalid port in TEST_MSSQL_URL: %v", scanErr)
		}
		port = parsed
	}
	pass, _ := u.User.Password()
	return &ConnectionConfig{
		Driver:   "mssql",
		Host:     host,
		Port:     port,
		Database: u.Query().Get("database"),
		Username: u.User.Username(),
		Password: pass,
		SSLMode:  "disable",
	}
}

// TestMssqlConnection_Integration tests basic SQL Server connectivity.
func TestMssqlConnection_Integration(t *testing.T) {
	config := mssqlTestConfig(t)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	result, err := conn.Query(ctx, "SELECT 1 AS ok")
	if err != nil {
		t.Fatalf("SELECT should succeed: %v", err)
	}
	if result.RowCount != 1 {
		t.Fatalf("Expected 1 row, got %d", result.RowCount)
	}
}

// TestMssqlReadOnly_Integration verifies writes are rejected.
func TestMssqlReadOnly_Integration(t *testing.T) {
	config := mssqlTestConfig(t)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	// Write attempted via Query must be rejected by the app-level guard.
	_, err = conn.Query(ctx, "CREATE TABLE _dbridge_ro_test (id INT)")
	if err == nil {
		_, _ = conn.Exec(ctx, "DROP TABLE _dbridge_ro_test")
		t.Fatal("Expected error for write on read-only connection")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("Expected read-only error, got: %v", err)
	}

	// Exec is always rejected.
	if _, err := conn.Exec(ctx, "CREATE TABLE _dbridge_ro_test2 (id INT)"); err == nil {
		t.Fatal("Expected Exec to be rejected on read-only connection")
	}
}

// TestMssqlQueryWithData_Integration tests querying fixture data.
func TestMssqlQueryWithData_Integration(t *testing.T) {
	config := mssqlTestConfig(t)
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

	expectedTypes := []string{"int4", "nvarchar", "nvarchar", "bool", "int4"}
	for i, typ := range expectedTypes {
		if result.ColumnTypes[i] != typ {
			t.Errorf("Column %q type: expected %q, got %q", expectedCols[i], typ, result.ColumnTypes[i])
		}
	}
}

// TestMssqlSchemaInspection_Integration tests the schema inspector against fixtures.
func TestMssqlSchemaInspection_Integration(t *testing.T) {
	config := mssqlTestConfig(t)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	inspector := conn.Schema()

	schemas, err := inspector.ListSchemas(ctx)
	if err != nil {
		t.Fatalf("ListSchemas failed: %v", err)
	}
	hasDbo := false
	for _, s := range schemas {
		if s.Name == "dbo" {
			hasDbo = true
		}
		if strings.HasPrefix(s.Name, "db_") || s.Name == "sys" || s.Name == "INFORMATION_SCHEMA" {
			t.Errorf("system schema %q should be filtered out", s.Name)
		}
	}
	if !hasDbo {
		t.Errorf("expected 'dbo' schema, got %v", schemas)
	}

	// "public" should map to dbo.
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

// TestMssqlExplainQuery_Integration verifies SHOWPLAN_XML returns a plan and leaves
// the pinned connection usable afterwards.
func TestMssqlExplainQuery_Integration(t *testing.T) {
	config := mssqlTestConfig(t)
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
		t.Error("Expected non-empty execution plan")
	}
	if !strings.Contains(result.Plan, "ShowPlanXML") && !strings.Contains(result.Plan, "<") {
		t.Errorf("Expected XML plan, got: %q", result.Plan)
	}

	// SHOWPLAN must be OFF again: a normal query returns rows, not a plan.
	after, err := conn.Query(ctx, "SELECT 1 AS ok")
	if err != nil {
		t.Fatalf("query after EXPLAIN failed (connection left in SHOWPLAN mode?): %v", err)
	}
	if after.RowCount != 1 {
		t.Errorf("Expected 1 row after EXPLAIN, got %d", after.RowCount)
	}
}

// TestMssqlQueryTruncation_Integration verifies the row cap and warning.
func TestMssqlQueryTruncation_Integration(t *testing.T) {
	config := mssqlTestConfig(t)
	ctx := context.Background()

	conn, err := NewConnection(ctx, config)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close(ctx)

	result, err := conn.Query(ctx, "SELECT id FROM large_table")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if !result.Truncated {
		t.Fatal("expected Truncated=true for query returning >10000 rows")
	}
	if result.RowCount != maxSQLRows {
		t.Errorf("expected RowCount=%d, got %d", maxSQLRows, result.RowCount)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected truncation warning in result")
	}
	if !strings.Contains(result.Warnings[0], "10000") {
		t.Errorf("warning should mention row cap, got: %q", result.Warnings[0])
	}
}
