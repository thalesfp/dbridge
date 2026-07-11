package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli/output"
)

// AppOptions defines the capability-specific commands composed into a dbridge binary.
type AppOptions struct {
	Name     string
	Short    string
	Version  string
	Commit   string
	Date     string
	Commands []*cobra.Command
}

// Run executes a dbridge binary using the shared CLI runtime.
func Run(options AppOptions) int {
	isTerminal := func() bool {
		outFD := os.Stdout.Fd()
		inFD := os.Stdin.Fd()

		return (isatty.IsTerminal(outFD) || isatty.IsCygwinTerminal(outFD)) &&
			(isatty.IsTerminal(inFD) || isatty.IsCygwinTerminal(inFD))
	}

	rootCmd, resolveOutput := newRootCommand(options, isTerminal)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		<-sigCh
		cancel()
		<-sigCh
		os.Exit(130)
	}()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if _, ok := err.(*HandledError); !ok {
			if resolveOutput(os.Args[1:]) {
				fmt.Fprintln(os.Stderr, err)
			} else {
				data, marshalErr := json.Marshal(map[string]string{"error": err.Error()})
				if marshalErr != nil {
					fmt.Fprintln(os.Stderr, err)
				} else {
					fmt.Fprintln(os.Stderr, string(data))
				}
			}
		}

		return 1
	}

	return 0
}

func newRootCommand(options AppOptions, isTerminal func() bool) (*cobra.Command, func([]string) bool) {
	var humanOutput bool
	var jsonOutput bool
	var showVersion bool

	resolveMode := func(cmd *cobra.Command) bool {
		flags := cmd.Root().PersistentFlags()
		if flags.Changed("human") {
			return humanOutput
		}
		if flags.Changed("json") {
			return !jsonOutput
		}

		return isTerminal()
	}
	resolveArgs := func(args []string) bool {
		return resolveOutputFromArgs(args, isTerminal)
	}

	rootCmd := &cobra.Command{
		Use:           options.Name,
		Short:         options.Short,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.WithValue(cmd.Context(), HumanOutputKey, resolveMode(cmd))
			cmd.SetContext(ctx)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !showVersion {
				return cmd.Help()
			}
			if resolveMode(cmd) {
				fmt.Printf("%s %s (commit: %s, built: %s)\n", options.Name, options.Version, options.Commit, options.Date)

				return nil
			}

			data, err := json.Marshal(map[string]string{
				"version": options.Version,
				"commit":  options.Commit,
				"built":   options.Date,
			})
			if err != nil {
				return err
			}
			fmt.Println(string(data))

			return nil
		},
	}

	rootCmd.PersistentFlags().BoolVar(&humanOutput, "human", false, "Force human-readable output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Force JSON output")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Show version information")

	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if resolveMode(cmd) {
			defaultHelp(cmd, args)

			return
		}
		output.FormatHelpJSON(cmd, os.Stdout)
	})

	rootCmd.AddCommand(options.Commands...)

	return rootCmd, resolveArgs
}

func resolveOutputFromArgs(args []string, isTerminal func() bool) bool {
	for _, arg := range args {
		switch arg {
		case "--json", "--json=true", "--human=false":
			return false
		case "--human", "--human=true", "--json=false":
			return true
		case "--":
			return isTerminal()
		}
	}

	return isTerminal()
}
