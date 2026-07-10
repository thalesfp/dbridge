package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/thalesfp/dbridge/internal/dbconfig"
)

// mysqlReadOnlyErr is returned by Exec: dbridge connections are read-only, so
// writes are refused in application code (the session is also read-only at the
// DB level via SET SESSION TRANSACTION READ ONLY).
var mysqlReadOnlyErr = errors.New("read-only connection: only read queries are permitted")

func init() { RegisterDriver("mysql", &MysqlDriver{}) }

// MysqlDriver implements Driver for MySQL.
type MysqlDriver struct{}

func (d *MysqlDriver) Connect(ctx context.Context, config *ConnectionConfig) (Connection, error) {
	dsn := buildMysqlDSN(config)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql connection: %w", err)
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "SET SESSION TRANSACTION READ ONLY"); err != nil {
		conn.Close()
		db.Close()
		return nil, fmt.Errorf("failed to set read-only mode: %w", err)
	}

	return &MysqlConnection{
		db:     db,
		conn:   conn,
		config: config,
	}, nil
}

// MysqlConnection implements Connection using database/sql.
// Uses a pinned *sql.Conn so session settings persist across queries.
type MysqlConnection struct {
	db     *sql.DB
	conn   *sql.Conn
	config *ConnectionConfig
}

// buildMysqlDSN builds a MySQL DSN string using mysql.Config for proper escaping.
func buildMysqlDSN(config *ConnectionConfig) string {
	cfg := mysql.NewConfig()
	cfg.User = config.Username
	cfg.Passwd = config.Password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", config.Host, config.Port)
	cfg.DBName = config.Database
	cfg.ParseTime = true
	cfg.TLSConfig = mapSSLToMysqlTLS(config.SSLMode)
	return cfg.FormatDSN()
}

// mapSSLToMysqlTLS maps dbridge ssl_mode values to MySQL tls parameter values.
func mapSSLToMysqlTLS(sslMode string) string {
	return dbconfig.MySQLTLSMode(sslMode)
}

// mysqlTypeName normalizes a MySQL column type string to a short lowercase name.
func mysqlTypeName(typ string) string {
	// MySQL returns types like "INT", "BIGINT", "VARCHAR(255)", "DECIMAL(10,2)"
	t := strings.ToLower(strings.TrimSpace(typ))

	// Strip unsigned suffix for mapping
	base := strings.TrimSuffix(t, " unsigned")

	// Strip length/precision for base type lookup
	if idx := strings.Index(base, "("); idx != -1 {
		base = base[:idx]
	}

	switch base {
	case "tinyint":
		return "int1"
	case "smallint":
		return "int2"
	case "mediumint":
		return "int3"
	case "int", "integer":
		return "int4"
	case "bigint":
		return "int8"
	case "float":
		return "float4"
	case "double":
		return "float8"
	case "decimal", "numeric":
		return "numeric"
	case "char":
		return "char"
	case "varchar":
		return "varchar"
	case "tinytext":
		return "tinytext"
	case "text":
		return "text"
	case "mediumtext":
		return "mediumtext"
	case "longtext":
		return "longtext"
	case "binary", "varbinary":
		return base
	case "tinyblob":
		return "tinyblob"
	case "blob":
		return "blob"
	case "mediumblob":
		return "mediumblob"
	case "longblob":
		return "longblob"
	case "date":
		return "date"
	case "time":
		return "time"
	case "datetime":
		return "datetime"
	case "timestamp":
		return "timestamp"
	case "year":
		return "year"
	case "json":
		return "json"
	case "enum":
		return "enum"
	case "set":
		return "set"
	case "bit":
		return "bit"
	case "bool", "boolean":
		return "bool"
	default:
		return t
	}
}

// Query executes a SELECT query
func (c *MysqlConnection) Query(ctx context.Context, sqlStr string, args ...interface{}) (*QueryResult, error) {
	start := time.Now()

	rows, err := c.conn.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// Get column information
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("failed to get column types: %w", err)
	}

	columns := make([]string, len(colTypes))
	columnTypeNames := make([]string, len(colTypes))
	for i, ct := range colTypes {
		columns[i] = ct.Name()
		columnTypeNames[i] = mysqlTypeName(ct.DatabaseTypeName())
	}

	// Pre-allocate scan destinations once; the *interface{} pointers are reused each row.
	dest := make([]interface{}, len(colTypes))
	for i := range dest {
		dest[i] = new(interface{})
	}

	// Collect rows up to the hard cap
	var result [][]interface{}
	truncated := false
	for rows.Next() {
		if len(result) >= maxSQLRows {
			truncated = true
			break
		}

		if err := rows.Scan(dest...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make([]interface{}, len(colTypes))
		for i, d := range dest {
			val := *(d.(*interface{}))
			// MySQL driver returns []byte for text columns; convert to string
			if b, ok := val.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = val
			}
		}
		result = append(result, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	duration := time.Since(start)

	qr := &QueryResult{
		Columns:     columns,
		ColumnTypes: columnTypeNames,
		Rows:        result,
		RowCount:    len(result),
		Duration:    duration,
		Truncated:   truncated,
	}
	if truncated {
		qr.Warnings = append(qr.Warnings, fmt.Sprintf(truncatedWarning, maxSQLRows))
	}
	return qr, nil
}

// Exec is rejected: dbridge connections are read-only, so write operations are
// refused in application code.
func (c *MysqlConnection) Exec(ctx context.Context, sqlStr string, args ...interface{}) (*ExecResult, error) {
	return nil, mysqlReadOnlyErr
}

// Schema returns schema inspector
func (c *MysqlConnection) Schema() SchemaInspector {
	return &MysqlSchemaInspector{conn: c}
}

// Close closes the pinned connection and the underlying pool
func (c *MysqlConnection) Close(ctx context.Context) error {
	c.conn.Close()
	return c.db.Close()
}

// Config returns connection configuration (without password)
func (c *MysqlConnection) Config() *ConnectionConfig {
	config := *c.config
	config.Password = "***"
	return &config
}

// MysqlSchemaInspector implements SchemaInspector for MySQL.
type MysqlSchemaInspector struct {
	conn *MysqlConnection
}

// ListSchemas returns only the connected database name (MySQL has no schema concept like PG).
func (s *MysqlSchemaInspector) ListSchemas(ctx context.Context) ([]Schema, error) {
	return []Schema{{Name: s.conn.config.Database}}, nil
}

// ListTables lists tables in the connected database.
// The schema parameter is accepted for interface compatibility; "public" is mapped to the connected database.
func (s *MysqlSchemaInspector) ListTables(ctx context.Context, schema string) ([]Table, error) {
	dbName := schema
	if dbName == "public" || dbName == "" {
		dbName = s.conn.config.Database
	}

	query := `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema = ? AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	rows, err := s.conn.conn.QueryContext(ctx, query, dbName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.Schema, &t.Name); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}

	return tables, rows.Err()
}

// DescribeTable gets table structure using information_schema.columns.
func (s *MysqlSchemaInspector) DescribeTable(ctx context.Context, schema, table string) (*TableDefinition, error) {
	dbName := schema
	if dbName == "public" || dbName == "" {
		dbName = s.conn.config.Database
	}

	query := `
		SELECT column_name, column_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position
	`

	rows, err := s.conn.conn.QueryContext(ctx, query, dbName, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo
	for rows.Next() {
		var col ColumnInfo
		var nullable string
		if err := rows.Scan(&col.Name, &col.Type, &nullable, &col.Default); err != nil {
			return nil, err
		}
		col.Nullable = nullable == "YES"
		columns = append(columns, col)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &TableDefinition{
		Schema:  dbName,
		Name:    table,
		Columns: columns,
	}, nil
}

// ExplainQuery gets query execution plan using EXPLAIN FORMAT=JSON.
func (s *MysqlSchemaInspector) ExplainQuery(ctx context.Context, sqlStr string) (*ExplainResult, error) {
	explainSQL := "EXPLAIN FORMAT=JSON " + sqlStr

	var plan string
	err := s.conn.conn.QueryRowContext(ctx, explainSQL).Scan(&plan)
	if err != nil {
		return nil, err
	}

	return &ExplainResult{
		Plan: plan,
	}, nil
}
