package database

import "context"

// Connection represents a database connection
type Connection interface {
	// Query executes a SELECT query
	Query(ctx context.Context, sql string, args ...interface{}) (*QueryResult, error)

	// Exec executes a write operation (INSERT, UPDATE, DELETE)
	Exec(ctx context.Context, sql string, args ...interface{}) (*ExecResult, error)

	// Schema returns schema inspection interface
	Schema() SchemaInspector

	// Close closes the connection
	Close(ctx context.Context) error

	// Config returns connection configuration
	Config() *ConnectionConfig
}

// SchemaInspector provides schema introspection capabilities
type SchemaInspector interface {
	ListSchemas(ctx context.Context) ([]Schema, error)
	ListTables(ctx context.Context, schema string) ([]Table, error)
	DescribeTable(ctx context.Context, schema, table string) (*TableDefinition, error)
	ExplainQuery(ctx context.Context, sql string) (*ExplainResult, error)
}

// ConnectionConfig holds connection configuration
type ConnectionConfig struct {
	Driver   string
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
	ReadOnly bool
}
