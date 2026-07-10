package writecli

import (
	"errors"
	"strings"
	"testing"

	"github.com/thalesfp/dbridge/internal/writedb"
)

func TestRenderBatchResult(t *testing.T) {
	executeErr := errors.New("statement 2 failed")
	data, failed, err := renderBatchResult(&writedb.BatchResult{}, executeErr)
	if err != nil {
		t.Fatalf("renderBatchResult() error = %v", err)
	}
	if !failed {
		t.Fatal("renderBatchResult() failed = false")
	}
	if !strings.Contains(string(data), executeErr.Error()) {
		t.Fatalf("rendered result does not contain execution error: %s", data)
	}
}

func TestRenderBatchResultRejectsMissingResult(t *testing.T) {
	if _, _, err := renderBatchResult(nil, nil); err == nil {
		t.Fatal("renderBatchResult(nil, nil) error = nil")
	}
}
