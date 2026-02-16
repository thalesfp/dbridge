package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/config"
	"github.com/thalesfp/dbridge/internal/credentials"
	"github.com/thalesfp/dbridge/internal/database"
)

// NewSchemaCmd creates the schema command
func NewSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Inspect database schema",
		Long:  "Commands for inspecting database schemas, tables, and structures",
	}

	cmd.AddCommand(newListSchemasCmd())
	cmd.AddCommand(newListTablesCmd())
	cmd.AddCommand(newDescribeTableCmd())

	return cmd
}

// newListSchemasCmd creates the 'schema list-schemas' command
func newListSchemasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-schemas <profile>",
		Short: "List all schemas in the database",
		Long: `List all schemas in the specified database profile.

Examples:
  dbridge schema list-schemas production
  dbridge schema list-schemas local`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]

			conn, err := getConnection(profileName)
			if err != nil {
				return err
			}
			defer conn.Close(context.Background())

			schemas, err := conn.Schema().ListSchemas(context.Background())
			if err != nil {
				return fmt.Errorf("failed to list schemas: %w", err)
			}

			// Output as JSON array
			schemaNames := make([]string, len(schemas))
			for i, schema := range schemas {
				schemaNames[i] = schema.Name
			}

			output, _ := json.Marshal(schemaNames)
			fmt.Println(string(output))

			return nil
		},
	}

	return cmd
}

// newListTablesCmd creates the 'schema list-tables' command
func newListTablesCmd() *cobra.Command {
	var schema string

	cmd := &cobra.Command{
		Use:   "list-tables <profile>",
		Short: "List all tables in a schema",
		Long: `List all tables in the specified database profile and schema.

Examples:
  dbridge schema list-tables production
  dbridge schema list-tables local --schema myschema`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]

			conn, err := getConnection(profileName)
			if err != nil {
				return err
			}
			defer conn.Close(context.Background())

			tables, err := conn.Schema().ListTables(context.Background(), schema)
			if err != nil {
				return fmt.Errorf("failed to list tables: %w", err)
			}

			// Output as JSON array
			tableNames := make([]string, len(tables))
			for i, table := range tables {
				tableNames[i] = table.Name
			}

			output, _ := json.Marshal(tableNames)
			fmt.Println(string(output))

			return nil
		},
	}

	cmd.Flags().StringVarP(&schema, "schema", "s", "public", "Schema name")
	return cmd
}

// newDescribeTableCmd creates the 'schema describe' command
func newDescribeTableCmd() *cobra.Command {
	var schema string

	cmd := &cobra.Command{
		Use:   "describe <profile> <table>",
		Short: "Describe table structure",
		Long: `Describe the structure of a table in the specified database profile.

Examples:
  dbridge schema describe production users
  dbridge schema describe local orders`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			tableName := args[1]

			conn, err := getConnection(profileName)
			if err != nil {
				return err
			}
			defer conn.Close(context.Background())

			def, err := conn.Schema().DescribeTable(context.Background(), schema, tableName)
			if err != nil {
				return fmt.Errorf("failed to describe table: %w", err)
			}

			// Output as compact JSON
			output, _ := json.MarshalIndent(def, "", "  ")
			fmt.Println(string(output))

			return nil
		},
	}

	cmd.Flags().StringVarP(&schema, "schema", "s", "public", "Schema name")
	return cmd
}

// getConnection is a helper to create a database connection
func getConnection(profileName string) (database.Connection, error) {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	profileConfig, err := cfg.GetProfile(profileName)
	if err != nil {
		return nil, err
	}

	if profileConfig.Disabled {
		return nil, fmt.Errorf("profile '%s' is disabled (enable it with: dbridge --human config manage)", profileName)
	}

	// Load credentials
	credStore, err := credentials.NewStore("dbridge")
	if err != nil {
		return nil, fmt.Errorf("failed to open credential store: %w", err)
	}

	ctx := context.Background()
	creds, err := credStore.Load(ctx, profileName)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials: %w", err)
	}

	// Create database connection
	connConfig := &database.ConnectionConfig{
		Host:     profileConfig.Host,
		Port:     profileConfig.Port,
		Database: profileConfig.Database,
		Username: creds.Username,
		Password: creds.Password,
		SSLMode:  profileConfig.SSLMode,
		ReadOnly: profileConfig.ReadOnly,
	}

	conn, err := database.NewConnection(ctx, connConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return conn, nil
}
