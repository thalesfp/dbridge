package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesgelinger/pgmcp/internal/config"
	"github.com/thalesgelinger/pgmcp/internal/credentials"
	"github.com/thalesgelinger/pgmcp/internal/database"
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
	var profile string

	cmd := &cobra.Command{
		Use:   "list-schemas",
		Short: "List all schemas in the database",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := getConnection(profile)
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

	cmd.Flags().StringVarP(&profile, "profile", "p", "", "Connection profile to use")
	return cmd
}

// newListTablesCmd creates the 'schema list-tables' command
func newListTablesCmd() *cobra.Command {
	var (
		profile string
		schema  string
	)

	cmd := &cobra.Command{
		Use:   "list-tables",
		Short: "List all tables in a schema",
		RunE: func(cmd *cobra.Command, args []string) error {
			conn, err := getConnection(profile)
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

	cmd.Flags().StringVarP(&profile, "profile", "p", "", "Connection profile to use")
	cmd.Flags().StringVarP(&schema, "schema", "s", "public", "Schema name")
	return cmd
}

// newDescribeTableCmd creates the 'schema describe' command
func newDescribeTableCmd() *cobra.Command {
	var (
		profile string
		schema  string
	)

	cmd := &cobra.Command{
		Use:   "describe [table]",
		Short: "Describe table structure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tableName := args[0]

			conn, err := getConnection(profile)
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

	cmd.Flags().StringVarP(&profile, "profile", "p", "", "Connection profile to use")
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

	// Get profile
	if profileName == "" {
		profileName = os.Getenv("PGMCP_PROFILE")
	}
	if profileName == "" {
		profileName = cfg.Settings.DefaultProfile
	}

	profileConfig, err := cfg.GetProfile(profileName)
	if err != nil {
		return nil, err
	}

	// Load credentials
	credStore, err := credentials.NewStore("pgmcp")
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
		PoolSize: profileConfig.PoolSize,
	}

	conn, err := database.NewConnection(ctx, connConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return conn, nil
}
