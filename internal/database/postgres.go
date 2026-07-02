package database

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func init() { RegisterDriver("postgres", &PostgresDriver{}) }

// PostgresDriver implements Driver for PostgreSQL via pgx.
type PostgresDriver struct{}

func (d *PostgresDriver) Connect(ctx context.Context, config *ConnectionConfig) (Connection, error) {
	connString := buildPgConnString(config)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("invalid connection config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PgxConnection{
		pool:   pool,
		config: config,
	}, nil
}

// PgxConnection implements Connection using pgx
type PgxConnection struct {
	pool   *pgxpool.Pool
	config *ConnectionConfig
}

// buildPgConnString builds a PostgreSQL connection string from the config.
// Uses net/url so that credentials, host and database containing URL
// metacharacters are escaped rather than breaking or injecting DSN parameters
// (e.g. an unescaped "?sslmode=disable" could silently downgrade TLS).
func buildPgConnString(config *ConnectionConfig) string {
	query := url.Values{}
	if config.SSLMode != "" {
		query.Set("sslmode", config.SSLMode)
	}
	query.Set("default_transaction_read_only", "on")

	var user *url.Userinfo
	if config.Password != "" {
		user = url.UserPassword(config.Username, config.Password)
	} else {
		user = url.User(config.Username)
	}

	u := &url.URL{
		Scheme:   "postgres",
		User:     user,
		Host:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Path:     "/" + config.Database,
		RawQuery: query.Encode(),
	}
	return u.String()
}

// Query executes a SELECT query
func (c *PgxConnection) Query(ctx context.Context, sql string, args ...interface{}) (*QueryResult, error) {
	start := time.Now()

	rows, err := c.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// Get column information
	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	columnTypes := make([]string, len(fieldDescriptions))

	for i, fd := range fieldDescriptions {
		columns[i] = string(fd.Name)
		columnTypes[i] = getTypeName(fd.DataTypeOID)
	}

	// Collect rows up to the hard cap
	var result [][]interface{}
	truncated := false
	for rows.Next() {
		if len(result) >= maxSQLRows {
			truncated = true
			break
		}
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		result = append(result, values)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	duration := time.Since(start)

	qr := &QueryResult{
		Columns:     columns,
		ColumnTypes: columnTypes,
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

// Exec executes a write operation
func (c *PgxConnection) Exec(ctx context.Context, sql string, args ...interface{}) (*ExecResult, error) {
	start := time.Now()

	commandTag, err := c.pool.Exec(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("exec failed: %w", err)
	}

	duration := time.Since(start)

	return &ExecResult{
		RowsAffected: commandTag.RowsAffected(),
		Duration:     duration,
	}, nil
}

// Schema returns schema inspector
func (c *PgxConnection) Schema() SchemaInspector {
	return &PgxSchemaInspector{conn: c}
}

// Close closes the connection pool
func (c *PgxConnection) Close(ctx context.Context) error {
	c.pool.Close()
	return nil
}

// Config returns connection configuration (without password)
func (c *PgxConnection) Config() *ConnectionConfig {
	config := *c.config
	config.Password = "***" // Redact password
	return &config
}

// getTypeName returns a short type name for common PostgreSQL types
func getTypeName(oid uint32) string {
	switch oid {
	case 16:
		return "bool"
	case 20:
		return "int8"
	case 21:
		return "int2"
	case 23:
		return "int4"
	case 25:
		return "text"
	case 700:
		return "float4"
	case 701:
		return "float8"
	case 1043:
		return "varchar"
	case 1082:
		return "date"
	case 1083:
		return "time"
	case 1114:
		return "timestamp"
	case 1184:
		return "timestamptz"
	case 1700:
		return "numeric"
	case 2950:
		return "uuid"
	case 114:
		return "json"
	case 3802:
		return "jsonb"
	default:
		return fmt.Sprintf("oid_%d", oid)
	}
}

// PgxSchemaInspector implements SchemaInspector
type PgxSchemaInspector struct {
	conn *PgxConnection
}

// ListSchemas lists all schemas
func (s *PgxSchemaInspector) ListSchemas(ctx context.Context) ([]Schema, error) {
	sql := `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY schema_name
	`

	rows, err := s.conn.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []Schema
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		schemas = append(schemas, Schema{Name: name})
	}

	return schemas, rows.Err()
}

// ListTables lists tables in a schema
func (s *PgxSchemaInspector) ListTables(ctx context.Context, schema string) ([]Table, error) {
	sql := `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema = $1 AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`

	rows, err := s.conn.pool.Query(ctx, sql, schema)
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

// DescribeTable gets table structure
func (s *PgxSchemaInspector) DescribeTable(ctx context.Context, schema, table string) (*TableDefinition, error) {
	colSQL := `
		SELECT column_name, data_type, is_nullable, column_default
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position
	`

	rows, err := s.conn.pool.Query(ctx, colSQL, schema, table)
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
		Schema:  schema,
		Name:    table,
		Columns: columns,
	}, nil
}

// ExplainQuery gets query execution plan
func (s *PgxSchemaInspector) ExplainQuery(ctx context.Context, sql string) (*ExplainResult, error) {
	explainSQL := "EXPLAIN " + sql

	var plan string
	err := s.conn.pool.QueryRow(ctx, explainSQL).Scan(&plan)
	if err != nil {
		return nil, err
	}

	return &ExplainResult{
		Plan: plan,
	}, nil
}
