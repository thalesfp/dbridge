package writedb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "github.com/microsoft/go-mssqldb"
)

type sqlConnection struct {
	db *sql.DB
}

func connectMySQL(ctx context.Context, config *Config) (Connection, error) {
	cfg := mysql.NewConfig()
	cfg.User = config.Username
	cfg.Passwd = config.Password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", config.Host, config.Port)
	cfg.DBName = config.Database
	cfg.ParseTime = true
	cfg.MultiStatements = true
	cfg.TLSConfig = mysqlTLSMode(config.SSLMode)

	return openSQLConnection(ctx, "mysql", cfg.FormatDSN())
}

func connectMSSQL(ctx context.Context, config *Config) (Connection, error) {
	query := url.Values{}
	if config.Database != "" {
		query.Set("database", config.Database)
	}

	encrypt, trust := mssqlTLSMode(config.SSLMode)
	query.Set("encrypt", encrypt)
	if trust != "" {
		query.Set("trustservercertificate", trust)
	}

	u := &url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(config.Username, config.Password),
		Host:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		RawQuery: query.Encode(),
	}

	return openSQLConnection(ctx, "sqlserver", u.String())
}

func openSQLConnection(ctx context.Context, driver, dsn string) (Connection, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open write connection: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()

		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &sqlConnection{db: db}, nil
}

func (c *sqlConnection) Execute(ctx context.Context, batch string) (*BatchResult, error) {
	start := time.Now()
	result := &BatchResult{Results: make([]StatementResult, 0)}

	rows, err := c.db.QueryContext(ctx, batch)
	if err != nil {
		result.Duration = time.Since(start)

		return result, fmt.Errorf("batch execution failed: %w", err)
	}
	defer rows.Close()

	for {
		statement, readErr := collectSQLResult(rows)
		result.Results = append(result.Results, statement)
		if readErr != nil {
			result.Duration = time.Since(start)

			return result, fmt.Errorf("failed to read batch result: %w", readErr)
		}
		if !rows.NextResultSet() {
			break
		}
	}

	result.Duration = time.Since(start)
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("batch execution failed: %w", err)
	}

	return result, nil
}

func collectSQLResult(rows *sql.Rows) (StatementResult, error) {
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return StatementResult{}, err
	}

	result := StatementResult{
		Columns:     make([]string, len(columnTypes)),
		ColumnTypes: make([]string, len(columnTypes)),
		Rows:        make([][]interface{}, 0),
	}
	dest := make([]interface{}, len(columnTypes))
	for i, columnType := range columnTypes {
		result.Columns[i] = columnType.Name()
		result.ColumnTypes[i] = strings.ToLower(columnType.DatabaseTypeName())
		dest[i] = new(interface{})
	}

	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return result, err
		}
		if len(result.Rows) >= maxResultRows {
			result.Truncated = true
			continue
		}

		row := make([]interface{}, len(dest))
		for i, value := range dest {
			scanned := *(value.(*interface{}))
			if bytes, ok := scanned.([]byte); ok {
				row[i] = string(bytes)
			} else {
				row[i] = scanned
			}
		}
		result.Rows = append(result.Rows, row)
	}

	result.RowCount = len(result.Rows)

	return result, rows.Err()
}

func mysqlTLSMode(sslMode string) string {
	switch strings.ToLower(sslMode) {
	case "disable":
		return "false"
	case "prefer", "preferred":
		return "preferred"
	case "require", "verify-ca", "verify-full", "":
		return "true"
	default:
		return "true"
	}
}

func mssqlTLSMode(sslMode string) (string, string) {
	switch strings.ToLower(sslMode) {
	case "disable":
		return "disable", ""
	case "prefer", "preferred":
		return "true", "true"
	case "require":
		return "true", "true"
	default:
		return "true", "false"
	}
}

func (c *sqlConnection) Close() error {
	return c.db.Close()
}
