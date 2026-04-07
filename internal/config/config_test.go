package config

import (
	"testing"
)

// TestDefaultConfig tests the default configuration
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Settings.Output.Default != "auto" {
		t.Errorf("Expected output format 'auto', got '%s'", cfg.Settings.Output.Default)
	}

	if !cfg.Settings.Output.AutoDetectTTY {
		t.Error("Expected AutoDetectTTY to be true")
	}

	if !cfg.Settings.Output.SmartSimplify {
		t.Error("Expected SmartSimplify to be true")
	}

	if cfg.Settings.Output.IncludeTiming {
		t.Error("Expected IncludeTiming to be false")
	}

	if cfg.Settings.Safety.MaxRowsWithoutConfirmation != 1000 {
		t.Errorf("Expected max rows 1000, got %d", cfg.Settings.Safety.MaxRowsWithoutConfirmation)
	}

	expectedConfirmations := []string{"DELETE", "DROP", "TRUNCATE"}
	if len(cfg.Settings.Safety.RequireConfirmation) != len(expectedConfirmations) {
		t.Errorf("Expected %d confirmation types, got %d", len(expectedConfirmations), len(cfg.Settings.Safety.RequireConfirmation))
	}
}

// TestAddConnection tests adding a connection
func TestAddConnection(t *testing.T) {
	cfg := DefaultConfig()

	connection := &Connection{
		Name:     "test-db",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
		SSLMode:  "require",
		ReadOnly: false,
	}

	cfg.AddConnection(connection)

	// Verify connection was added
	if len(cfg.Connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(cfg.Connections))
	}

	loadedConnection, ok := cfg.Connections["test-db"]
	if !ok {
		t.Fatal("Connection 'test-db' not found")
	}

	if loadedConnection.Host != connection.Host {
		t.Errorf("Expected host %s, got %s", connection.Host, loadedConnection.Host)
	}

	if loadedConnection.Port != connection.Port {
		t.Errorf("Expected port %d, got %d", connection.Port, loadedConnection.Port)
	}
}

// TestGetConnection tests retrieving a connection
func TestGetConnection(t *testing.T) {
	cfg := DefaultConfig()

	connection := &Connection{
		Name:     "test-db",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
	}

	cfg.AddConnection(connection)

	// Get connection by name
	loadedConnection, err := cfg.GetConnection("test-db")
	if err != nil {
		t.Fatalf("Failed to get connection: %v", err)
	}

	if loadedConnection.Name != connection.Name {
		t.Errorf("Expected name %s, got %s", connection.Name, loadedConnection.Name)
	}

	// Empty name should return an error
	_, err = cfg.GetConnection("")
	if err == nil {
		t.Error("Expected error when getting connection with empty name")
	}
}

// TestGetConnection_Nonexistent tests getting a nonexistent connection
func TestGetConnection_Nonexistent(t *testing.T) {
	cfg := DefaultConfig()

	_, err := cfg.GetConnection("nonexistent")
	if err == nil {
		t.Error("Expected error when getting nonexistent connection")
	}
}

// TestRemoveConnection tests removing a connection
func TestRemoveConnection(t *testing.T) {
	cfg := DefaultConfig()

	// Add two connections
	cfg.AddConnection(&Connection{
		Name:     "connection1",
		Host:     "host1",
		Database: "db1",
		Username: "user1",
	})

	cfg.AddConnection(&Connection{
		Name:     "connection2",
		Host:     "host2",
		Database: "db2",
		Username: "user2",
	})

	// Remove connection1
	err := cfg.RemoveConnection("connection1")
	if err != nil {
		t.Fatalf("Failed to remove connection: %v", err)
	}

	// Verify removal
	if len(cfg.Connections) != 1 {
		t.Errorf("Expected 1 connection remaining, got %d", len(cfg.Connections))
	}

	_, ok := cfg.Connections["connection1"]
	if ok {
		t.Error("Connection 'connection1' should be removed")
	}
}

// TestRemoveConnection_LastConnection tests removing the last connection
func TestRemoveConnection_LastConnection(t *testing.T) {
	cfg := DefaultConfig()

	// Add one connection
	cfg.AddConnection(&Connection{
		Name:     "only-connection",
		Host:     "host",
		Database: "db",
		Username: "user",
	})

	// Remove it
	err := cfg.RemoveConnection("only-connection")
	if err != nil {
		t.Fatalf("Failed to remove connection: %v", err)
	}

	if len(cfg.Connections) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(cfg.Connections))
	}
}

// TestRemoveConnection_Nonexistent tests removing a nonexistent connection
func TestRemoveConnection_Nonexistent(t *testing.T) {
	cfg := DefaultConfig()

	err := cfg.RemoveConnection("nonexistent")
	if err == nil {
		t.Error("Expected error when removing nonexistent connection")
	}
}

// TestListConnections tests listing all connections
func TestListConnections(t *testing.T) {
	cfg := DefaultConfig()

	// Add multiple connections
	connectionNames := []string{"connection1", "connection2", "connection3"}
	for _, name := range connectionNames {
		cfg.AddConnection(&Connection{
			Name:     name,
			Host:     "host",
			Database: "db",
			Username: "user",
		})
	}

	// List connections
	listedConnections := cfg.ListConnections()

	if len(listedConnections) != len(connectionNames) {
		t.Errorf("Expected %d connections, got %d", len(connectionNames), len(listedConnections))
	}

	// Verify all connections are present
	connectionMap := make(map[string]bool)
	for _, name := range listedConnections {
		connectionMap[name] = true
	}

	for _, name := range connectionNames {
		if !connectionMap[name] {
			t.Errorf("Connection %s not found in list", name)
		}
	}
}

// TestConnectionDefaults tests connection default values
func TestConnectionDefaults(t *testing.T) {
	connection := &Connection{
		Name:     "test",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	}

	// Default port should be 0 (will be set to 5432 by defaults or during connection)
	if connection.Port != 0 {
		t.Errorf("Expected default port 0, got %d", connection.Port)
	}

	// Default readonly should be false
	if connection.ReadOnly {
		t.Error("Expected default ReadOnly to be false")
	}

	// Default disabled should be false (zero value)
	if connection.Disabled {
		t.Error("Expected default Disabled to be false")
	}
}

// TestConnectionDisabledField tests the Disabled field behavior
func TestConnectionDisabledField(t *testing.T) {
	cfg := DefaultConfig()

	// Add an enabled connection (default)
	cfg.AddConnection(&Connection{
		Name:     "enabled-db",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})

	// Add a disabled connection
	cfg.AddConnection(&Connection{
		Name:     "disabled-db",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
		Disabled: true,
	})

	// Verify enabled connection
	enabledConnection, err := cfg.GetConnection("enabled-db")
	if err != nil {
		t.Fatalf("Failed to get enabled connection: %v", err)
	}
	if enabledConnection.Disabled {
		t.Error("Expected enabled connection to have Disabled=false")
	}

	// Verify disabled connection
	disabledConnection, err := cfg.GetConnection("disabled-db")
	if err != nil {
		t.Fatalf("Failed to get disabled connection: %v", err)
	}
	if !disabledConnection.Disabled {
		t.Error("Expected disabled connection to have Disabled=true")
	}
}

// TestToggleConnectionDisabled tests toggling the Disabled state
func TestToggleConnectionDisabled(t *testing.T) {
	cfg := DefaultConfig()

	cfg.AddConnection(&Connection{
		Name:     "toggle-test",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})

	connection := cfg.Connections["toggle-test"]

	// Initially not disabled
	if connection.Disabled {
		t.Error("Expected connection to start as not disabled")
	}

	// Disable it
	connection.Disabled = true
	if !connection.Disabled {
		t.Error("Expected connection to be disabled after toggle")
	}

	// Re-enable it
	connection.Disabled = false
	if connection.Disabled {
		t.Error("Expected connection to be enabled after second toggle")
	}
}

// TestGetConnection_DriverDefaults tests driver-specific defaults
func TestGetConnection_DriverDefaults(t *testing.T) {
	t.Run("postgres driver defaults to port 5432", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AddConnection(&Connection{
			Name:     "pg-db",
			Driver:   "postgres",
			Host:     "localhost",
			Database: "testdb",
			Username: "user",
		})

		p, err := cfg.GetConnection("pg-db")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Port != 5432 {
			t.Errorf("Expected port 5432, got %d", p.Port)
		}
		if p.SSLMode != "require" {
			t.Errorf("Expected ssl 'require', got '%s'", p.SSLMode)
		}
	})

	t.Run("empty driver falls back to postgres port/ssl defaults", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AddConnection(&Connection{
			Name:     "legacy-db",
			Host:     "localhost",
			Database: "testdb",
			Username: "user",
		})

		p, err := cfg.GetConnection("legacy-db")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Driver != "postgres" {
			t.Errorf("Expected driver 'postgres', got '%s'", p.Driver)
		}
		if p.Port != 5432 {
			t.Errorf("Expected port 5432, got %d", p.Port)
		}
		if p.SSLMode != "require" {
			t.Errorf("Expected ssl 'require', got '%s'", p.SSLMode)
		}
	})

	t.Run("mysql driver defaults to port 3306", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AddConnection(&Connection{
			Name:     "my-db",
			Driver:   "mysql",
			Host:     "localhost",
			Database: "testdb",
			Username: "user",
		})

		p, err := cfg.GetConnection("my-db")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Driver != "mysql" {
			t.Errorf("Expected driver 'mysql', got '%s'", p.Driver)
		}
		if p.Port != 3306 {
			t.Errorf("Expected port 3306, got %d", p.Port)
		}
		if p.SSLMode != "preferred" {
			t.Errorf("Expected ssl 'preferred', got '%s'", p.SSLMode)
		}
	})

	t.Run("explicit port and ssl override driver defaults", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.AddConnection(&Connection{
			Name:     "custom-db",
			Driver:   "mysql",
			Host:     "localhost",
			Port:     3307,
			Database: "testdb",
			Username: "user",
			SSLMode:  "disable",
		})

		p, err := cfg.GetConnection("custom-db")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Port != 3307 {
			t.Errorf("Expected port 3307, got %d", p.Port)
		}
		if p.SSLMode != "disable" {
			t.Errorf("Expected ssl 'disable', got '%s'", p.SSLMode)
		}
	})
}

// TestGetConnectionStillWorksWhenDisabled tests that GetConnection returns disabled connections
func TestGetConnectionStillWorksWhenDisabled(t *testing.T) {
	cfg := DefaultConfig()

	cfg.AddConnection(&Connection{
		Name:     "disabled-connection",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
		Disabled: true,
	})

	// GetConnection should still return the connection (disabled check is at connection points)
	connection, err := cfg.GetConnection("disabled-connection")
	if err != nil {
		t.Fatalf("GetConnection should return disabled connections, got error: %v", err)
	}
	if !connection.Disabled {
		t.Error("Expected connection to be disabled")
	}
}
