package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli"
	"github.com/thalesfp/dbridge/internal/writecli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	commands := []*cobra.Command{
		writecli.NewConfigCmd(),
		writecli.NewExecuteCmd(),
		cli.NewQueryCmd(),
		cli.NewSchemaCmd(),
		writecli.NewMCPCmd(),
	}

	os.Exit(cli.Run(cli.AppOptions{
		Name:     "dbridge-write",
		Short:    "Write-capable database CLI and MCP server",
		Version:  version,
		Commit:   commit,
		Date:     date,
		Commands: commands,
	}))
}
