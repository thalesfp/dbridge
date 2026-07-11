package writedb

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBuildPostgresConnStringIsWritable(t *testing.T) {
	connString := buildPostgresConnString(&Config{
		Host:     "localhost",
		Port:     5432,
		Database: "app?default_transaction_read_only=on",
		Username: "writer",
		Password: "secret",
		SSLMode:  "verify-full",
	})

	parsed, err := pgxpool.ParseConfig(connString)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if parsed.ConnConfig.Database != "app?default_transaction_read_only=on" {
		t.Fatalf("database = %q, want escaped literal name", parsed.ConnConfig.Database)
	}
	if value, ok := parsed.ConnConfig.RuntimeParams["default_transaction_read_only"]; ok {
		t.Fatalf("default_transaction_read_only = %q, want unset", value)
	}
}

func TestConnectRejectsUnsupportedDriver(t *testing.T) {
	if _, err := Connect(t.Context(), &Config{Driver: "mongodb"}); err == nil {
		t.Fatal("Connect() error = nil")
	}
}

func TestSupportsDriver(t *testing.T) {
	for _, driver := range []string{"", "postgres", "mysql", "mssql"} {
		if !SupportsDriver(driver) {
			t.Fatalf("SupportsDriver(%q) = false", driver)
		}
	}
	for _, driver := range []string{"mongodb", "oracle", "unknown"} {
		if SupportsDriver(driver) {
			t.Fatalf("SupportsDriver(%q) = true", driver)
		}
	}
}
