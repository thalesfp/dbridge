package writedb

import (
	"net"
	"net/url"
	"os"
	"strconv"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestPostgresBatch_Integration(t *testing.T) {
	config := postgresTestConfig(t)
	assertBatchReturnsRows(t, config, `
		CREATE TEMP TABLE _dbridge_write_test (id int);
		INSERT INTO _dbridge_write_test VALUES (1), (2);
		SELECT id FROM _dbridge_write_test ORDER BY id;
	`)
}

func TestMySQLBatch_Integration(t *testing.T) {
	config := mysqlTestConfig(t)
	assertBatchReturnsRows(t, config, `
		DROP TEMPORARY TABLE IF EXISTS _dbridge_write_test;
		CREATE TEMPORARY TABLE _dbridge_write_test (id int);
		INSERT INTO _dbridge_write_test VALUES (1), (2);
		SELECT id FROM _dbridge_write_test ORDER BY id;
	`)
}

func TestMSSQLBatch_Integration(t *testing.T) {
	config := mssqlTestConfig(t)
	assertBatchReturnsRows(t, config, `
		CREATE TABLE #dbridge_write_test (id int);
		INSERT INTO #dbridge_write_test VALUES (1), (2);
		SELECT id FROM #dbridge_write_test ORDER BY id;
	`)
}

func TestMySQLAffectedRowsLimitation_Integration(t *testing.T) {
	config := mysqlTestConfig(t)
	assertAffectedRowsWarning(t, config, `
		CREATE TEMPORARY TABLE _dbridge_write_count_test (id int);
		INSERT INTO _dbridge_write_count_test VALUES (1), (2);
	`)
}

func TestMSSQLAffectedRowsLimitation_Integration(t *testing.T) {
	config := mssqlTestConfig(t)
	assertAffectedRowsWarning(t, config, `
		CREATE TABLE #dbridge_write_count_test (id int);
		INSERT INTO #dbridge_write_count_test VALUES (1), (2);
	`)
}

func assertBatchReturnsRows(t *testing.T, config *Config, batch string) {
	t.Helper()

	conn, err := Connect(t.Context(), config)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	result, err := conn.Execute(t.Context(), batch)
	if err != nil {
		t.Fatalf("Execute() error = %v, result = %+v", err, result)
	}

	for _, statement := range result.Results {
		if statement.RowCount == 2 {
			return
		}
	}

	t.Fatalf("batch returned no two-row result: %+v", result)
}

func assertAffectedRowsWarning(t *testing.T, config *Config, batch string) {
	t.Helper()

	conn, err := Connect(t.Context(), config)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	result, err := conn.Execute(t.Context(), batch)
	if err != nil {
		t.Fatalf("Execute() error = %v, result = %+v", err, result)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("Execute() returned no affected-row warning")
	}
	for _, statement := range result.Results {
		if statement.RowsAffected != nil {
			t.Fatalf("RowsAffected = %d, want unavailable", *statement.RowsAffected)
		}
	}
}

func postgresTestConfig(t *testing.T) *Config {
	t.Helper()

	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid TEST_DATABASE_URL: %v", err)
	}

	return configFromURL(t, "postgres", u)
}

func mysqlTestConfig(t *testing.T) *Config {
	t.Helper()

	raw := os.Getenv("TEST_MYSQL_URL")
	if raw == "" {
		t.Skip("TEST_MYSQL_URL not set")
	}
	cfg, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Fatalf("invalid TEST_MYSQL_URL: %v", err)
	}
	host, portString, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		t.Fatalf("invalid MySQL address: %v", err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatalf("invalid MySQL port: %v", err)
	}

	return &Config{
		Driver:   "mysql",
		Host:     host,
		Port:     port,
		Database: cfg.DBName,
		Username: cfg.User,
		Password: cfg.Passwd,
		SSLMode:  "disable",
	}
}

func mssqlTestConfig(t *testing.T) *Config {
	t.Helper()

	raw := os.Getenv("TEST_MSSQL_URL")
	if raw == "" {
		t.Skip("TEST_MSSQL_URL not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("invalid TEST_MSSQL_URL: %v", err)
	}

	return configFromURL(t, "mssql", u)
}

func configFromURL(t *testing.T, driver string, u *url.URL) *Config {
	t.Helper()

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("invalid port: %v", err)
	}
	password, _ := u.User.Password()
	database := u.Query().Get("database")
	if database == "" {
		database = u.Path[1:]
	}

	return &Config{
		Driver:   driver,
		Host:     u.Hostname(),
		Port:     port,
		Database: database,
		Username: u.User.Username(),
		Password: password,
		SSLMode:  "disable",
	}
}
