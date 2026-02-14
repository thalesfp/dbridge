package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli"
)

var (
	version    = "dev"
	commit     = "none"
	date       = "unknown"
	jsonOutput bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dbridge",
		Short: "Multi-database CLI for AI agents with MCP server",
		Long: `dbridge is a cross-platform database CLI tool with MCP server support.

It provides secure database access for AI agents through named connection profiles,
with credentials stored in OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service).`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Store JSON output flag in context for subcommands
			ctx := context.WithValue(cmd.Context(), "json_output", jsonOutput)
			cmd.SetContext(ctx)
			return nil
		},
	}

	// Add global --json flag
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false,
		"Output in JSON format for machine parsing")

	// Add commands
	rootCmd.AddCommand(cli.NewConfigCmd())
	rootCmd.AddCommand(cli.NewQueryCmd())
	rootCmd.AddCommand(cli.NewSchemaCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
