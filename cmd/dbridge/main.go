package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli"
	"github.com/thalesfp/dbridge/internal/cli/output"
)

var (
	version     = "dev"
	commit      = "none"
	date        = "unknown"
	humanOutput bool
	showVersion bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:           "dbridge",
		Short:         "Multi-database CLI for AI agents with MCP server",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.WithValue(cmd.Context(), "human_output", humanOutput)
			cmd.SetContext(ctx)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				if humanOutput {
					fmt.Printf("dbridge %s (commit: %s, built: %s)\n", version, commit, date)
				} else {
					v := map[string]string{
						"version": version,
						"commit":  commit,
						"built":   date,
					}
					bytes, _ := json.Marshal(v)
					fmt.Println(string(bytes))
				}
				return nil
			}
			return cmd.Help()
		},
	}

	// Add global --human flag
	rootCmd.PersistentFlags().BoolVar(&humanOutput, "human", false,
		"Human-readable output")

	// Add --version / -v flag (manual, not Cobra's built-in)
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version information")

	// Override help to output JSON by default
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if humanOutput {
			defaultHelp(cmd, args)
			return
		}
		output.FormatHelpJSON(cmd, os.Stdout)
	})

	// Add commands
	rootCmd.AddCommand(cli.NewConfigCmd())
	rootCmd.AddCommand(cli.NewQueryCmd())
	rootCmd.AddCommand(cli.NewSchemaCmd())

	if err := rootCmd.Execute(); err != nil {
		// Skip re-outputting errors that were already formatted as JSON
		if _, ok := err.(*cli.HandledError); !ok {
			// humanOutput may not be set if Cobra failed during flag parsing
			// (PersistentPreRunE never ran). Fall back to scanning os.Args.
			isHuman := humanOutput || hasHumanFlag(os.Args[1:])
			if !isHuman {
				errObj := map[string]string{"error": err.Error()}
				bytes, _ := json.Marshal(errObj)
				fmt.Fprintln(os.Stderr, string(bytes))
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		}
		os.Exit(1)
	}
}

// hasHumanFlag scans raw args for --human before Cobra's flag parsing.
// This is needed because parse-time errors (unknown command, bad flag)
// happen before PersistentPreRunE sets humanOutput.
func hasHumanFlag(args []string) bool {
	for _, a := range args {
		if a == "--human" || a == "--human=true" {
			return true
		}
		if a == "--human=false" {
			return false
		}
		if a == "--" {
			return false
		}
	}
	return false
}
