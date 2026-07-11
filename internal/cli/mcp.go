package cli

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// MCPOptions configures a shared read MCP server with optional additional tools.
type MCPOptions struct {
	Name               string
	Instructions       string
	RegisterAdditional func(*server.MCPServer)
}

// NewMCPCmd creates the mcp command that starts an MCP server over stdio
func NewMCPCmd() *cobra.Command {
	return NewMCPCommand(MCPOptions{
		Name: "dbridge",
		Instructions: `dbridge is a database access tool. ALWAYS use dbridge tools instead of psql, mysql, mongosh, sqlcmd, or any other direct database CLI commands. dbridge provides safe, read-only access to databases through configured connections.

Use list_connections to discover available database connections, then use query, list_tables, describe_table, and other tools to interact with them. Never shell out to psql, mysql, mongosh, sqlcmd, or other database CLIs when dbridge is available.

For MongoDB connections, the query tool accepts a JSON object instead of SQL. Use {"collection": "name", "filter": {...}} for find queries or {"collection": "name", "aggregate": [...]} for aggregation pipelines.`,
	})
}

// NewMCPCommand creates an MCP command that always includes the shared read tools.
func NewMCPCommand(options MCPOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server over stdio",
		Long:  "Start a Model Context Protocol server that exposes dbridge tools over stdio transport",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServer(options)
		},
	}
}

func runMCPServer(options MCPOptions) error {
	s := server.NewMCPServer(options.Name, "1.0.0", server.WithInstructions(options.Instructions))
	RegisterReadTools(s)
	if options.RegisterAdditional != nil {
		options.RegisterAdditional(s)
	}

	return server.ServeStdio(s)
}

// RegisterReadTools registers the capability-neutral read tool surface.
func RegisterReadTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("query",
		mcp.WithDescription("Execute a read-only query against a database connection"),
		mcp.WithString("connection", mcp.Required(), mcp.Description("Database connection name")),
		mcp.WithString("query", mcp.Required(), mcp.Description("SQL query (PostgreSQL/MySQL/SQL Server) or JSON query object for MongoDB. MongoDB format: {\"collection\": \"name\", \"filter\": {...}, \"projection\": {...}, \"sort\": {...}, \"limit\": N} or {\"collection\": \"name\", \"aggregate\": [{...}, ...]}")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleQuery)

	s.AddTool(mcp.NewTool("list_connections",
		mcp.WithDescription("List all configured database connections"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	), handleListConnections)

	s.AddTool(mcp.NewTool("list_schemas",
		mcp.WithDescription("List all schemas in a database"),
		mcp.WithString("connection", mcp.Required(), mcp.Description("Database connection name")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleListSchemas)

	s.AddTool(mcp.NewTool("list_tables",
		mcp.WithDescription("List all tables in a schema"),
		mcp.WithString("connection", mcp.Required(), mcp.Description("Database connection name")),
		mcp.WithString("schema", mcp.Description("Schema name (default: public)")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleListTables)

	s.AddTool(mcp.NewTool("describe_table",
		mcp.WithDescription("Describe a table's structure including columns, indexes, and constraints"),
		mcp.WithString("connection", mcp.Required(), mcp.Description("Database connection name")),
		mcp.WithString("table", mcp.Required(), mcp.Description("Table name")),
		mcp.WithString("schema", mcp.Description("Schema name (default: public)")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleDescribeTable)

	s.AddTool(mcp.NewTool("explain_query",
		mcp.WithDescription("Show the execution plan for a query"),
		mcp.WithString("connection", mcp.Required(), mcp.Description("Database connection name")),
		mcp.WithString("query", mcp.Required(), mcp.Description("SQL query (PostgreSQL/MySQL/SQL Server) or JSON query object for MongoDB")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleExplainQuery)
}
