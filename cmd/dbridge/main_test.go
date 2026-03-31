package main

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveOutputMode(t *testing.T) {
	origIsTerminal := isTerminal
	defer func() { isTerminal = origIsTerminal }()

	setup := func(args []string) *cobra.Command {
		// Reset package-level vars
		humanOutput = false
		jsonOutput = false

		cmd := &cobra.Command{Use: "test"}
		cmd.PersistentFlags().BoolVar(&humanOutput, "human", false, "")
		cmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "")
		cmd.SetArgs(args)
		// Parse flags so Changed() works
		_ = cmd.ParseFlags(args)
		return cmd
	}

	t.Run("TTY fallback", func(t *testing.T) {
		isTerminal = func() bool { return true }
		cmd := setup([]string{})
		if got := resolveOutputMode(cmd); got != true {
			t.Errorf("expected human (TTY), got JSON")
		}

		isTerminal = func() bool { return false }
		cmd = setup([]string{})
		if got := resolveOutputMode(cmd); got != false {
			t.Errorf("expected JSON (no TTY), got human")
		}
	})

	t.Run("explicit flags", func(t *testing.T) {
		isTerminal = func() bool { return true }

		tests := []struct {
			name string
			args []string
			want bool
		}{
			{"--human forces human", []string{"--human"}, true},
			{"--json forces JSON", []string{"--json"}, false},
			{"--human=false forces JSON on TTY", []string{"--human=false"}, false},
			{"--json=false forces human on TTY", []string{"--json=false"}, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cmd := setup(tt.args)
				if got := resolveOutputMode(cmd); got != tt.want {
					t.Errorf("resolveOutputMode(%v) = %v, want %v", tt.args, got, tt.want)
				}
			})
		}
	})

	t.Run("--human takes priority over --json", func(t *testing.T) {
		isTerminal = func() bool { return false }
		cmd := setup([]string{"--human", "--json"})
		if got := resolveOutputMode(cmd); got != true {
			t.Errorf("expected --human to win, got JSON")
		}
	})
}

func TestResolveOutputFromArgs(t *testing.T) {
	// Override TTY detection for predictable tests
	origIsTerminal := isTerminal
	defer func() { isTerminal = origIsTerminal }()

	t.Run("with TTY", func(t *testing.T) {
		isTerminal = func() bool { return true }

		tests := []struct {
			name string
			args []string
			want bool
		}{
			{"no flags defaults to human (TTY)", []string{"config", "list"}, true},
			{"--human flag", []string{"--human"}, true},
			{"--human=true", []string{"--human=true"}, true},
			{"--json forces JSON", []string{"--json"}, false},
			{"--json=true forces JSON", []string{"--json=true"}, false},
			{"--human=false forces JSON", []string{"--human=false"}, false},
			{"--json=false forces human", []string{"--json=false"}, true},
			{"--human overrides TTY explicitly", []string{"--human", "config"}, true},
			{"empty args defaults to human (TTY)", []string{}, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := resolveOutputFromArgs(tt.args)
				if got != tt.want {
					t.Errorf("resolveOutputFromArgs(%v) = %v, want %v", tt.args, got, tt.want)
				}
			})
		}
	})

	t.Run("without TTY", func(t *testing.T) {
		isTerminal = func() bool { return false }

		tests := []struct {
			name string
			args []string
			want bool
		}{
			{"no flags defaults to JSON (no TTY)", []string{"config", "list"}, false},
			{"--human forces human even without TTY", []string{"--human"}, true},
			{"--json keeps JSON", []string{"--json"}, false},
			{"--human=false forces JSON even without TTY", []string{"--human=false"}, false},
			{"--json=false forces human even without TTY", []string{"--json=false"}, true},
			{"empty args defaults to JSON (no TTY)", []string{}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := resolveOutputFromArgs(tt.args)
				if got != tt.want {
					t.Errorf("resolveOutputFromArgs(%v) = %v, want %v", tt.args, got, tt.want)
				}
			})
		}
	})

	t.Run("edge cases", func(t *testing.T) {
		isTerminal = func() bool { return true }

		tests := []struct {
			name string
			args []string
			want bool
		}{
			{"--json after double dash ignored", []string{"--", "--json"}, true},
			{"--human after double dash ignored", []string{"--", "--human"}, true},
			{"--human before double dash", []string{"--human", "--", "arg"}, true},
			{"--json before double dash", []string{"--json", "--", "arg"}, false},
			{"first explicit flag wins (json first)", []string{"--json", "--human"}, false},
			{"first explicit flag wins (human first)", []string{"--human", "--json"}, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got := resolveOutputFromArgs(tt.args)
				if got != tt.want {
					t.Errorf("resolveOutputFromArgs(%v) = %v, want %v", tt.args, got, tt.want)
				}
			})
		}
	})
}
