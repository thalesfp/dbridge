package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli/output"
	"github.com/thalesfp/dbridge/internal/config"
	"github.com/thalesfp/dbridge/internal/credentials"
	"github.com/thalesfp/dbridge/internal/database"
)

// NewQueryCmd creates the query command
func NewQueryCmd() *cobra.Command {
	var (
		profile string
		format  string
	)

	cmd := &cobra.Command{
		Use:   "query [sql]",
		Short: "Execute a SELECT query",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sql := args[0]

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Get profile
			if profile == "" {
				profile = os.Getenv("PGMCP_PROFILE")
			}
			if profile == "" {
				profile = cfg.Settings.DefaultProfile
			}

			profileConfig, err := cfg.GetProfile(profile)
			if err != nil {
				return err
			}

			// Load credentials
			credStore, err := credentials.NewStore("dbridge")
			if err != nil {
				return fmt.Errorf("failed to open credential store: %w", err)
			}

			ctx := context.Background()
			creds, err := credStore.Load(ctx, profile)
			if err != nil {
				return fmt.Errorf("failed to load credentials: %w", err)
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
				ReadOnly: profileConfig.ReadOnly,
			}

			conn, err := database.NewConnection(ctx, connConfig)
			if err != nil {
				return fmt.Errorf("failed to connect to database: %w", err)
			}
			defer conn.Close(ctx)

			// Execute query
			result, err := conn.Query(ctx, sql)
			if err != nil {
				// Format error in compact JSON
				errOutput, _ := output.FormatError(err, nil, nil, nil)
				fmt.Println(errOutput)
				return nil // Don't return error to avoid duplicate error message
			}

			// Determine output format
			outputFormat := format
			if outputFormat == "auto" || outputFormat == "" {
				outputFormat = "compact" // Default to compact for now
			}

			// Format output
			switch outputFormat {
			case "compact":
				opts := output.FormatOptions{
					IncludeTypes:    cfg.Settings.Output.IncludeTypes,
					IncludeTiming:   cfg.Settings.Output.IncludeTiming,
					IncludeWarnings: cfg.Settings.Output.IncludeWarnings,
					SmartSimplify:   cfg.Settings.Output.SmartSimplify,
				}
				jsonOutput, err := output.FormatCompact(result, opts)
				if err != nil {
					return err
				}
				fmt.Println(jsonOutput)

			case "table":
				tableOutput, err := output.FormatTable(result)
				if err != nil {
					return err
				}
				fmt.Print(tableOutput)

			case "table-compact":
				compactTable, err := output.FormatTableCompact(result)
				if err != nil {
					return err
				}
				fmt.Print(compactTable)

			case "csv":
				csvOutput, err := output.FormatCSV(result, true)
				if err != nil {
					return err
				}
				fmt.Print(csvOutput)

			default:
				return fmt.Errorf("unknown output format: %s (available: compact, table, table-compact, csv)", outputFormat)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&profile, "profile", "p", "", "Connection profile to use")
	cmd.Flags().StringVarP(&format, "format", "f", "auto", "Output format: auto, compact, table, csv")

	return cmd
}
