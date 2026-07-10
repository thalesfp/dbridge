package writedb

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type postgresConnection struct {
	pool *pgxpool.Pool
}

func connectPostgres(ctx context.Context, config *Config) (Connection, error) {
	poolConfig, err := pgxpool.ParseConfig(buildPostgresConnString(config))
	if err != nil {
		return nil, fmt.Errorf("invalid write connection config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create write connection pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &postgresConnection{pool: pool}, nil
}

func buildPostgresConnString(config *Config) string {
	query := url.Values{}
	if config.SSLMode != "" {
		query.Set("sslmode", config.SSLMode)
	}

	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(config.Username, config.Password),
		Host:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Path:     "/" + config.Database,
		RawQuery: query.Encode(),
	}

	return u.String()
}

func (c *postgresConnection) Execute(ctx context.Context, sql string) (*BatchResult, error) {
	start := time.Now()
	result := &BatchResult{Results: make([]StatementResult, 0)}

	conn, err := c.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire write connection: %w", err)
	}
	defer conn.Release()

	multi := conn.Conn().PgConn().Exec(ctx, sql)
	for multi.NextResult() {
		rows := pgx.RowsFromResultReader(conn.Conn().TypeMap(), multi.ResultReader())
		statement, readErr := collectPostgresResult(rows)
		result.Results = append(result.Results, statement)
		if readErr != nil {
			result.Duration = time.Since(start)

			return result, fmt.Errorf("failed to read batch result: %w", readErr)
		}
	}

	result.Duration = time.Since(start)
	if err := multi.Close(); err != nil {
		return result, fmt.Errorf("batch execution failed: %w", err)
	}

	return result, nil
}

func collectPostgresResult(rows pgx.Rows) (StatementResult, error) {
	fields := rows.FieldDescriptions()
	result := StatementResult{
		Columns:     make([]string, len(fields)),
		ColumnTypes: make([]string, len(fields)),
		Rows:        make([][]interface{}, 0),
	}
	for i, field := range fields {
		result.Columns[i] = field.Name
		result.ColumnTypes[i] = fmt.Sprintf("oid_%d", field.DataTypeOID)
	}

	for rows.Next() {
		if len(result.Rows) >= maxResultRows {
			result.Truncated = true
			continue
		}

		values, err := rows.Values()
		if err != nil {
			return result, err
		}
		result.Rows = append(result.Rows, values)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}

	tag := rows.CommandTag()
	result.RowCount = len(result.Rows)
	result.CommandTag = tag.String()
	rowsAffected := tag.RowsAffected()
	result.RowsAffected = &rowsAffected

	return result, nil
}

func (c *postgresConnection) Close() error {
	c.pool.Close()

	return nil
}
