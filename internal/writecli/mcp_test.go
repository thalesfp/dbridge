package writecli

import (
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/thalesfp/dbridge/internal/cli"
)

func TestRegisterWriteTools(t *testing.T) {
	s := server.NewMCPServer("test", "test")
	cli.RegisterReadTools(s)
	registerWriteTools(s)

	tools := s.ListTools()
	if len(tools) != 8 {
		t.Fatalf("tool count = %d, want 8", len(tools))
	}

	execute := tools["execute"]
	if execute == nil {
		t.Fatal("execute tool is not registered")
	}
	if execute.Tool.Annotations.ReadOnlyHint == nil || *execute.Tool.Annotations.ReadOnlyHint {
		t.Fatal("execute readOnlyHint must be false")
	}
	if execute.Tool.Annotations.DestructiveHint == nil || !*execute.Tool.Annotations.DestructiveHint {
		t.Fatal("execute destructiveHint must be true")
	}
	if execute.Tool.Annotations.IdempotentHint == nil || *execute.Tool.Annotations.IdempotentHint {
		t.Fatal("execute idempotentHint must be false")
	}

	query := tools["query"]
	if query == nil || query.Tool.Annotations.ReadOnlyHint == nil || !*query.Tool.Annotations.ReadOnlyHint {
		t.Fatal("shared query tool must remain read-only")
	}
}
