package cli

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// NewMCPCmd creates the mcp command that starts an MCP server over stdio
func NewMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server over stdio",
		Long:  "Start a Model Context Protocol server that exposes dbridge tools over stdio transport",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServer()
		},
	}
	return cmd
}

func runMCPServer() error {
	s := server.NewMCPServer("dbridge", "1.0.0",
		server.WithInstructions(`dbridge is a database access tool. ALWAYS use dbridge tools instead of psql, mysql, or any other direct database CLI commands. dbridge provides safe, read-only access to databases through configured profiles.

Use list_profiles to discover available database connections, then use query, list_tables, describe_table, and other tools to interact with them. Never shell out to psql or other database CLIs when dbridge is available.`),
	)

	s.AddTool(mcp.NewTool("query",
		mcp.WithDescription("Execute a read-only SQL query against a database profile"),
		mcp.WithString("profile", mcp.Required(), mcp.Description("Database profile name")),
		mcp.WithString("sql", mcp.Required(), mcp.Description("SQL query to execute")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleQuery)

	s.AddTool(mcp.NewTool("list_profiles",
		mcp.WithDescription("List all configured database profiles"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	), handleListProfiles)

	s.AddTool(mcp.NewTool("list_schemas",
		mcp.WithDescription("List all schemas in a database"),
		mcp.WithString("profile", mcp.Required(), mcp.Description("Database profile name")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleListSchemas)

	s.AddTool(mcp.NewTool("list_tables",
		mcp.WithDescription("List all tables in a schema"),
		mcp.WithString("profile", mcp.Required(), mcp.Description("Database profile name")),
		mcp.WithString("schema", mcp.Description("Schema name (default: public)")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleListTables)

	s.AddTool(mcp.NewTool("describe_table",
		mcp.WithDescription("Describe a table's structure including columns, indexes, and constraints"),
		mcp.WithString("profile", mcp.Required(), mcp.Description("Database profile name")),
		mcp.WithString("table", mcp.Required(), mcp.Description("Table name")),
		mcp.WithString("schema", mcp.Description("Schema name (default: public)")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleDescribeTable)

	s.AddTool(mcp.NewTool("explain_query",
		mcp.WithDescription("Show the execution plan for a SQL query"),
		mcp.WithString("profile", mcp.Required(), mcp.Description("Database profile name")),
		mcp.WithString("sql", mcp.Required(), mcp.Description("SQL query to explain")),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleExplainQuery)

	return server.ServeStdio(s)
}
