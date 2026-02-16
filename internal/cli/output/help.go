package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// HelpOutput is the structured JSON help for a command
type HelpOutput struct {
	Command        string           `json:"cmd"`
	Description    string           `json:"desc"`
	Usage          string           `json:"usage,omitempty"`
	Subcommands    []SubcommandInfo `json:"cmds,omitempty"`
	Flags          []FlagInfo       `json:"flags,omitempty"`
	InheritedFlags []FlagInfo       `json:"inherited_flags,omitempty"`
}

// SubcommandInfo describes a subcommand
type SubcommandInfo struct {
	Name        string `json:"name"`
	Description string `json:"desc"`
}

// FlagInfo describes a flag
type FlagInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Default string `json:"default"`
	Desc    string `json:"desc"`
}

// FormatHelpJSON outputs structured JSON help for a cobra command
func FormatHelpJSON(cmd *cobra.Command, w io.Writer) {
	h := HelpOutput{
		Command:     cmd.CommandPath(),
		Description: cmd.Short,
		Usage:       strings.TrimSpace(cmd.UseLine()),
	}

	// Subcommands
	for _, sub := range cmd.Commands() {
		if sub.IsAvailableCommand() {
			h.Subcommands = append(h.Subcommands, SubcommandInfo{
				Name:        sub.Name(),
				Description: sub.Short,
			})
		}
	}

	// Local flags
	cmd.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		name := "--" + f.Name
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", --" + f.Name
		}
		h.Flags = append(h.Flags, FlagInfo{
			Name:    name,
			Type:    f.Value.Type(),
			Default: f.DefValue,
			Desc:    f.Usage,
		})
	})

	// Inherited flags
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		name := "--" + f.Name
		if f.Shorthand != "" {
			name = "-" + f.Shorthand + ", --" + f.Name
		}
		h.InheritedFlags = append(h.InheritedFlags, FlagInfo{
			Name:    name,
			Type:    f.Value.Type(),
			Default: f.DefValue,
			Desc:    f.Usage,
		})
	})

	bytes, _ := json.Marshal(h)
	fmt.Fprintln(w, string(bytes))
}
