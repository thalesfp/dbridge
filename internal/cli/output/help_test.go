package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

func TestFormatHelpJSON_RootWithSubcommands(t *testing.T) {
	root := &cobra.Command{
		Use:   "dbridge",
		Short: "Multi-database CLI for AI agents",
	}
	sub1 := &cobra.Command{
		Use:   "config",
		Short: "Manage database connections",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
	sub2 := &cobra.Command{
		Use:   "query",
		Short: "Execute a SELECT query",
		RunE:  func(cmd *cobra.Command, args []string) error { return nil },
	}
	root.AddCommand(sub1, sub2)
	root.PersistentFlags().Bool("human", false, "Force human-readable output")
	root.PersistentFlags().Bool("json", false, "Force JSON output")

	var buf bytes.Buffer
	FormatHelpJSON(root, &buf)

	var h HelpOutput
	if err := json.Unmarshal(buf.Bytes(), &h); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if h.Command != "dbridge" {
		t.Errorf("expected cmd 'dbridge', got %q", h.Command)
	}
	if h.Description != "Multi-database CLI for AI agents" {
		t.Errorf("unexpected desc: %q", h.Description)
	}
	if len(h.Subcommands) != 2 {
		t.Fatalf("expected 2 subcommands, got %d", len(h.Subcommands))
	}
	if h.Subcommands[0].Name != "config" {
		t.Errorf("expected first subcommand 'config', got %q", h.Subcommands[0].Name)
	}
	if h.Subcommands[1].Name != "query" {
		t.Errorf("expected second subcommand 'query', got %q", h.Subcommands[1].Name)
	}
	// --human and --json should appear as local flags on root
	foundHuman := false
	foundJSON := false
	for _, f := range h.Flags {
		if f.Name == "--human" {
			foundHuman = true
			if f.Type != "bool" {
				t.Errorf("expected human flag type 'bool', got %q", f.Type)
			}
		}
		if f.Name == "--json" {
			foundJSON = true
			if f.Type != "bool" {
				t.Errorf("expected json flag type 'bool', got %q", f.Type)
			}
		}
	}
	if !foundHuman {
		t.Error("expected --human in flags")
	}
	if !foundJSON {
		t.Error("expected --json in flags")
	}
}

func TestFormatHelpJSON_LeafWithFlags(t *testing.T) {
	root := &cobra.Command{
		Use:   "dbridge",
		Short: "Multi-database CLI for AI agents",
	}
	root.PersistentFlags().Bool("human", false, "Force human-readable output")
	root.PersistentFlags().Bool("json", false, "Force JSON output")

	config := &cobra.Command{
		Use:   "config",
		Short: "Manage database connections",
	}
	root.AddCommand(config)

	add := &cobra.Command{
		Use:   "add [connection-name]",
		Short: "Add a new database connection",
	}
	add.Flags().String("host", "localhost", "Database host")
	add.Flags().Int("port", 5432, "Database port")
	add.Flags().String("database", "", "Database name")
	config.AddCommand(add)

	var buf bytes.Buffer
	FormatHelpJSON(add, &buf)

	var h HelpOutput
	if err := json.Unmarshal(buf.Bytes(), &h); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if h.Command != "dbridge config add" {
		t.Errorf("expected cmd 'dbridge config add', got %q", h.Command)
	}
	if len(h.Flags) != 3 {
		t.Fatalf("expected 3 flags, got %d", len(h.Flags))
	}

	// Check inherited --human flag
	foundInherited := false
	for _, f := range h.InheritedFlags {
		if f.Name == "--human" {
			foundInherited = true
		}
	}
	if !foundInherited {
		t.Error("expected --human in inherited_flags")
	}
}

func TestFormatHelpJSON_CompactSingleLine(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "A test command",
	}

	var buf bytes.Buffer
	FormatHelpJSON(cmd, &buf)

	output := buf.String()
	// Should be a single line (one newline at end)
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) != 1 {
		t.Errorf("expected single-line JSON, got %d lines: %s", len(lines), output)
	}

	// Should be valid JSON
	var h HelpOutput
	if err := json.Unmarshal([]byte(output), &h); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
