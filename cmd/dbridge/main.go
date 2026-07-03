package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli"
	"github.com/thalesfp/dbridge/internal/cli/output"
)

var (
	version     = "dev"
	commit      = "none"
	date        = "unknown"
	humanOutput bool
	jsonOutput  bool
	showVersion bool
)

// isTerminal reports whether both stdout and stdin are terminals.
// Checks stdin too so that TUI commands don't launch when input is redirected.
// Overridable for testing.
var isTerminal = func() bool {
	outFd := os.Stdout.Fd()
	inFd := os.Stdin.Fd()
	return (isatty.IsTerminal(outFd) || isatty.IsCygwinTerminal(outFd)) &&
		(isatty.IsTerminal(inFd) || isatty.IsCygwinTerminal(inFd))
}

// resolveOutputMode returns true for human mode, false for JSON mode.
// Uses Cobra's Changed() to detect explicit =false flags; falls back to TTY detection.
func resolveOutputMode(cmd *cobra.Command) bool {
	pflags := cmd.Root().PersistentFlags()
	if pflags.Changed("human") {
		return humanOutput // true for --human/--human=true, false for --human=false
	}
	if pflags.Changed("json") {
		return !jsonOutput // false for --json/--json=true, true for --json=false
	}
	return isTerminal()
}

func main() {
	rootCmd := &cobra.Command{
		Use:           "dbridge",
		Short:         "Multi-database CLI for AI agents with MCP server",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.WithValue(cmd.Context(), cli.HumanOutputKey, resolveOutputMode(cmd))
			cmd.SetContext(ctx)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				if resolveOutputMode(cmd) {
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

	// Add global --human and --json flags
	rootCmd.PersistentFlags().BoolVar(&humanOutput, "human", false,
		"Force human-readable output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false,
		"Force JSON output")

	// Add --version / -v flag (manual, not Cobra's built-in)
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version information")

	// Override help to output JSON when not in human mode
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if resolveOutputMode(cmd) {
			defaultHelp(cmd, args)
			return
		}
		output.FormatHelpJSON(cmd, os.Stdout)
	})

	// Add commands
	rootCmd.AddCommand(cli.NewConfigCmd())
	rootCmd.AddCommand(cli.NewQueryCmd())
	rootCmd.AddCommand(cli.NewSchemaCmd())
	rootCmd.AddCommand(cli.NewMCPCmd())

	// Interrupt handling. The first SIGINT/SIGTERM cancels in-flight DB operations
	// (connect/query) for a graceful stop; a second one force-exits, so an
	// operation that ignores cancellation can always be killed with a second
	// Ctrl+C. A buffered channel captures both signals with no scheduling race,
	// and os.Exit fires regardless of other subscribers (such as the handler
	// mcp-go's ServeStdio installs).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
		<-sigCh
		os.Exit(130)
	}()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		// Skip re-outputting errors that were already formatted as JSON
		if _, ok := err.(*cli.HandledError); !ok {
			// humanOutput/jsonOutput may not be set if Cobra failed during
			// flag parsing (PersistentPreRunE never ran). Fall back to scanning os.Args.
			isHuman := resolveOutputFromArgs(os.Args[1:])
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

// resolveOutputFromArgs scans raw args before Cobra's flag parsing.
// This is needed because parse-time errors (unknown command, bad flag)
// happen before PersistentPreRunE runs. Falls back to TTY detection.
func resolveOutputFromArgs(args []string) bool {
	for _, a := range args {
		if a == "--json" || a == "--json=true" || a == "--human=false" {
			return false
		}
		if a == "--human" || a == "--human=true" || a == "--json=false" {
			return true
		}
		if a == "--" {
			break
		}
	}
	return isTerminal()
}
