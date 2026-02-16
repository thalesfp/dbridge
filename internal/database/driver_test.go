package database

import (
	"context"
	"testing"
)

func TestDriverNames(t *testing.T) {
	names := DriverNames()
	if len(names) == 0 {
		t.Fatal("Expected at least one registered driver")
	}

	// postgres should always be registered via init()
	found := false
	for _, n := range names {
		if n == "postgres" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'postgres' in DriverNames(), got %v", names)
	}
}

func TestNewConnection_UnknownDriver(t *testing.T) {
	ctx := context.Background()
	cfg := &ConnectionConfig{Driver: "oracle"}

	_, err := NewConnection(ctx, cfg)
	if err == nil {
		t.Fatal("Expected error for unknown driver")
	}
	if err.Error() != "unsupported driver: oracle" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestNewConnection_EmptyDriverReturnsError(t *testing.T) {
	ctx := context.Background()
	cfg := &ConnectionConfig{
		Driver: "", // empty driver should error
	}

	_, err := NewConnection(ctx, cfg)
	if err == nil {
		t.Fatal("Expected error for empty driver")
	}
	if err.Error() != "driver is required" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}
