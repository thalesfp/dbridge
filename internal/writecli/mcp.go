package writecli

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli"
	"github.com/thalesfp/dbridge/internal/config"
)

// NewMCPCmd creates the write server with the shared read tools and write-only tools.
func NewMCPCmd() *cobra.Command {
	return cli.NewMCPCommand(cli.MCPOptions{
		Name: "dbridge-write",
		Instructions: `dbridge-write provides explicit write-capable database access. Use list_write_connections to discover writable targets. The execute tool sends arbitrary SQL batches exactly as provided and may make destructive or irreversible changes. Database permissions are the authorization boundary.

The read tools remain read-only and use the normal dbridge read connections. Use them to inspect the database before executing a batch.`,
		RegisterAdditional: registerWriteTools,
	})
}

func registerWriteTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("list_write_connections",
		mcp.WithDescription("List explicitly configured writable database connections"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(false),
	), handleListWriteConnections)

	s.AddTool(mcp.NewTool("execute",
		mcp.WithDescription("Execute an arbitrary SQL batch using a writable database connection"),
		mcp.WithString("connection", mcp.Required(), mcp.Description("Write connection name")),
		mcp.WithString("sql", mcp.Required(), mcp.Description("SQL batch to send exactly as provided")),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
	), handleExecute)
}

func handleListWriteConnections(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	type connectionInfo struct {
		Name       string `json:"name"`
		Connection string `json:"connection"`
		Username   string `json:"username"`
		Disabled   bool   `json:"disabled,omitempty"`
	}

	names := make([]string, 0, len(cfg.WriteConnections))
	for name := range cfg.WriteConnections {
		names = append(names, name)
	}
	sort.Strings(names)

	connections := make([]connectionInfo, 0, len(names))
	for _, name := range names {
		writeConn := cfg.WriteConnections[name]
		connections = append(connections, connectionInfo{
			Name:       name,
			Connection: writeConn.Connection,
			Username:   writeConn.Username,
			Disabled:   writeConn.Disabled,
		})
	}

	data, err := json.Marshal(connections)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

func handleExecute(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name, err := request.RequireString("connection")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	batch, err := request.RequireString("sql")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cfg, configErr := config.Load()
	if configErr != nil {
		return mcp.NewToolResultError(configErr.Error()), nil
	}
	if err := writeAuditEvent(cfg, name, batch); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	conn, err := getConnection(ctx, name)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer conn.Close()

	result, executeErr := conn.Execute(ctx, batch)
	if result == nil {
		if executeErr == nil {
			return mcp.NewToolResultError("batch returned no result"), nil
		}

		return mcp.NewToolResultError(executeErr.Error()), nil
	}
	if executeErr != nil {
		result.Error = executeErr.Error()
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	toolResult := mcp.NewToolResultText(string(data))
	toolResult.IsError = executeErr != nil

	return toolResult, nil
}
