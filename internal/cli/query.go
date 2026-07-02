package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli/output"
	"github.com/thalesfp/dbridge/internal/config"
)

// NewQueryCmd creates the query command
func NewQueryCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "query <connection> <sql>",
		Short: "Execute a SELECT query",
		Long: `Execute a SELECT query against the specified database connection.

Examples:
  dbridge query production "SELECT * FROM users LIMIT 10"
  dbridge query staging-local "SELECT COUNT(*) FROM orders"
  dbridge query local "SELECT version()"`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			connName := args[0]
			sql := args[1]
			ctx := cmd.Context()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			conn, err := getConnection(ctx, connName)
			if err != nil {
				return err
			}
			defer conn.Close(ctx)

			// Execute query
			result, err := conn.Query(ctx, sql)
			if err != nil {
				// Format error in compact JSON on stderr so it isn't mixed into
				// the data stream on stdout.
				errOutput, _ := output.FormatError(err, nil, nil, nil)
				fmt.Fprintln(os.Stderr, errOutput)
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

	cmd.Flags().StringVarP(&format, "format", "f", "auto", "Output format: auto, compact, table, csv")

	return cmd
}
