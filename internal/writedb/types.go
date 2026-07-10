package writedb

import (
	"context"
	"time"
)

const maxResultRows = 10000

// Config contains the endpoint and writer identity for a writable connection.
type Config struct {
	Driver   string
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
}

// Connection executes native SQL batches without rewriting or wrapping them.
type Connection interface {
	Execute(ctx context.Context, sql string) (*BatchResult, error)
	Close() error
}

// BatchResult contains every result the driver exposed before completion or failure.
type BatchResult struct {
	Results  []StatementResult `json:"results"`
	Duration time.Duration     `json:"duration"`
	Error    string            `json:"error,omitempty"`
}

// StatementResult represents one result or command completion in a batch.
type StatementResult struct {
	Columns      []string        `json:"columns,omitempty"`
	ColumnTypes  []string        `json:"column_types,omitempty"`
	Rows         [][]interface{} `json:"rows,omitempty"`
	RowCount     int             `json:"row_count,omitempty"`
	CommandTag   string          `json:"command_tag,omitempty"`
	RowsAffected *int64          `json:"rows_affected,omitempty"`
	Truncated    bool            `json:"truncated,omitempty"`
}
