package cli

import (
	"context"
	"encoding/json"
	"errors"
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
		Use:   "list-schemas <connection>",
		Short: "List all schemas in the database",
		Long: `List all schemas in the specified database connection.

Examples:
  dbridge schema list-schemas production
  dbridge schema list-schemas local`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connName := args[0]

			conn, err := getConnection(connName)
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
		Use:   "list-tables <connection>",
		Short: "List all tables in a schema",
		Long: `List all tables in the specified database connection and schema.

Examples:
  dbridge schema list-tables production
  dbridge schema list-tables local --schema myschema`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connName := args[0]

			conn, err := getConnection(connName)
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
		Use:   "describe <connection> <table>",
		Short: "Describe table structure",
		Long: `Describe the structure of a table in the specified database connection.

Examples:
  dbridge schema describe production users
  dbridge schema describe local orders`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			connName := args[0]
			tableName := args[1]

			conn, err := getConnection(connName)
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

func getConnection(connName string) (database.Connection, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	connCfg, err := cfg.GetConnection(connName)
	if err != nil {
		return nil, err
	}

	if connCfg.Disabled {
		return nil, fmt.Errorf("connection '%s' is disabled (enable it with: dbridge config)", connName)
	}

	credStore, err := credentials.NewStore("dbridge")
	if err != nil {
		return nil, fmt.Errorf("failed to open credential store: %w", err)
	}

	ctx := context.Background()
	creds, err := credStore.Load(ctx, connName)
	if err != nil {
		if !errors.Is(err, credentials.ErrNotFound) {
			return nil, fmt.Errorf("failed to load credentials for '%s': %w", connName, err)
		}
		creds = credentials.Credentials{
			Username: connCfg.Username,
		}
	}

	connConfig := &database.ConnectionConfig{
		Driver:   connCfg.Driver,
		Host:     connCfg.Host,
		Port:     connCfg.Port,
		Database: connCfg.Database,
		Username: creds.Username,
		Password: creds.Password,
		SSLMode:  connCfg.SSLMode,
		URI:      connCfg.URI,
		SRV:      connCfg.SRV,
	}

	conn, err := database.NewConnection(ctx, connConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return conn, nil
}
