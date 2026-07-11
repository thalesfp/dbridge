package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestRootOutputMode(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		isTerminal bool
		want       bool
	}{
		{name: "TTY default", isTerminal: true, want: true},
		{name: "non-TTY default", want: false},
		{name: "human", args: []string{"--human"}, want: true},
		{name: "json", args: []string{"--json"}, isTerminal: true, want: false},
		{name: "human false", args: []string{"--human=false"}, isTerminal: true, want: false},
		{name: "json false", args: []string{"--json=false"}, want: true},
		{name: "human takes priority", args: []string{"--human", "--json"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			probe := &cobra.Command{
				Use: "probe",
				RunE: func(cmd *cobra.Command, args []string) error {
					value, ok := cmd.Context().Value(HumanOutputKey).(bool)
					if !ok {
						t.Fatal("human output mode missing from command context")
					}
					got = value

					return nil
				},
			}
			root, _ := newRootCommand(AppOptions{Name: "test", Commands: []*cobra.Command{probe}}, func() bool {
				return tt.isTerminal
			})
			root.SetArgs(append(tt.args, "probe"))
			if err := root.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("output mode = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveOutputFromArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		isTerminal bool
		want       bool
	}{
		{name: "TTY default", isTerminal: true, want: true},
		{name: "non-TTY default", isTerminal: false, want: false},
		{name: "human", args: []string{"--human"}, want: true},
		{name: "json", args: []string{"--json"}, isTerminal: true, want: false},
		{name: "human false", args: []string{"--human=false"}, isTerminal: true, want: false},
		{name: "json false", args: []string{"--json=false"}, want: true},
		{name: "after separator ignored", args: []string{"--", "--json"}, isTerminal: true, want: true},
		{name: "first flag wins", args: []string{"--json", "--human"}, isTerminal: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveOutputFromArgs(tt.args, func() bool { return tt.isTerminal })
			if got != tt.want {
				t.Fatalf("resolveOutputFromArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}
