package cli

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/thalesfp/dbridge/internal/cli/output"
	"github.com/thalesfp/dbridge/internal/config"
)

func handleQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profile, err := request.RequireString("profile")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	sql, err := request.RequireString("sql")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	conn, err := getReadOnlyConnection(profile)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer conn.Close(ctx)

	result, err := conn.Query(ctx, sql)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	cfg, _ := config.Load()
	opts := output.FormatOptions{
		IncludeTypes:    true,
		IncludeTiming:   false,
		IncludeWarnings: true,
		SmartSimplify:   true,
	}
	if cfg != nil {
		opts.IncludeTypes = cfg.Settings.Output.IncludeTypes
		opts.IncludeTiming = cfg.Settings.Output.IncludeTiming
		opts.IncludeWarnings = cfg.Settings.Output.IncludeWarnings
		opts.SmartSimplify = cfg.Settings.Output.SmartSimplify
	}

	jsonOutput, err := output.FormatCompact(result, opts)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(jsonOutput), nil
}

func handleListProfiles(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cfg, err := config.Load()
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	type profileInfo struct {
		Name     string `json:"name"`
		Driver   string `json:"driver"`
		Host     string `json:"host"`
		Database string `json:"db"`
		Disabled bool   `json:"disabled,omitempty"`
	}

	profiles := make([]profileInfo, 0, len(cfg.Profiles))
	for name, p := range cfg.Profiles {
		driver := p.Driver
		if driver == "" {
			driver = "postgres"
		}
		profiles = append(profiles, profileInfo{
			Name:     name,
			Driver:   driver,
			Host:     p.Host,
			Database: p.Database,
			Disabled: p.Disabled,
		})
	}

	bytes, err := json.Marshal(profiles)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(bytes)), nil
}

func handleListSchemas(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profile, err := request.RequireString("profile")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	conn, err := getConnection(profile)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer conn.Close(ctx)

	schemas, err := conn.Schema().ListSchemas(ctx)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	schemaNames := make([]string, len(schemas))
	for i, s := range schemas {
		schemaNames[i] = s.Name
	}

	bytes, err := json.Marshal(schemaNames)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(bytes)), nil
}

func handleListTables(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profile, err := request.RequireString("profile")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	schema := request.GetString("schema", "public")

	conn, err := getConnection(profile)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer conn.Close(ctx)

	tables, err := conn.Schema().ListTables(ctx, schema)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	tableNames := make([]string, len(tables))
	for i, t := range tables {
		tableNames[i] = t.Name
	}

	bytes, err := json.Marshal(tableNames)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(bytes)), nil
}

func handleDescribeTable(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profile, err := request.RequireString("profile")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	table, err := request.RequireString("table")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	schema := request.GetString("schema", "public")

	conn, err := getConnection(profile)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer conn.Close(ctx)

	def, err := conn.Schema().DescribeTable(ctx, schema, table)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	bytes, err := json.Marshal(def)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(bytes)), nil
}

func handleExplainQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	profile, err := request.RequireString("profile")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	sql, err := request.RequireString("sql")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	conn, err := getReadOnlyConnection(profile)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	defer conn.Close(ctx)

	plan, err := conn.Schema().ExplainQuery(ctx, sql)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	bytes, err := json.Marshal(plan)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	return mcp.NewToolResultText(string(bytes)), nil
}
