package credentials

import (
	"context"
	"testing"
)

// MockStore implements Store for testing
type MockStore struct {
	data map[string]Credentials
}

// NewMockStore creates a new mock credential store
func NewMockStore() *MockStore {
	return &MockStore{
		data: make(map[string]Credentials),
	}
}

func (m *MockStore) Save(ctx context.Context, connection string, creds Credentials) error {
	m.data[connection] = creds
	return nil
}

func (m *MockStore) Load(ctx context.Context, connection string) (Credentials, error) {
	creds, ok := m.data[connection]
	if !ok {
		return Credentials{}, &KeyringError{Connection: connection}
	}
	return creds, nil
}

func (m *MockStore) Delete(ctx context.Context, connection string) error {
	delete(m.data, connection)
	return nil
}

func (m *MockStore) List(ctx context.Context) ([]string, error) {
	connections := make([]string, 0, len(m.data))
	for name := range m.data {
		connections = append(connections, name)
	}
	return connections, nil
}

func (m *MockStore) Available() bool {
	return true
}

func (m *MockStore) Type() string {
	return "mock"
}

// KeyringError represents a keyring error
type KeyringError struct {
	Connection string
}

func (e *KeyringError) Error() string {
	return "connection not found: " + e.Connection
}

// TestMockStore_SaveAndLoad tests saving and loading credentials
func TestMockStore_SaveAndLoad(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	// Test saving credentials
	creds := Credentials{
		Username: "testuser",
		Password: "testpass",
	}

	err := store.Save(ctx, "test-conn", creds)
	if err != nil {
		t.Fatalf("Failed to save credentials: %v", err)
	}

	// Test loading credentials
	loadedCreds, err := store.Load(ctx, "test-conn")
	if err != nil {
		t.Fatalf("Failed to load credentials: %v", err)
	}

	if loadedCreds.Username != creds.Username {
		t.Errorf("Expected username %s, got %s", creds.Username, loadedCreds.Username)
	}

	if loadedCreds.Password != creds.Password {
		t.Errorf("Expected password %s, got %s", creds.Password, loadedCreds.Password)
	}
}

// TestMockStore_LoadNonexistent tests loading a nonexistent connection
func TestMockStore_LoadNonexistent(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	_, err := store.Load(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error when loading nonexistent connection")
	}
}

// TestMockStore_Delete tests deleting credentials
func TestMockStore_Delete(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	// Save credentials
	creds := Credentials{
		Username: "testuser",
		Password: "testpass",
	}
	if err := store.Save(ctx, "test-conn", creds); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete credentials
	err := store.Delete(ctx, "test-conn")
	if err != nil {
		t.Fatalf("Failed to delete credentials: %v", err)
	}

	// Verify deletion
	_, err = store.Load(ctx, "test-conn")
	if err == nil {
		t.Error("Expected error after deleting credentials")
	}
}

// TestMockStore_List tests listing all connections
func TestMockStore_List(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	names := []string{"conn1", "conn2", "conn3"}
	for _, name := range names {
		if err := store.Save(ctx, name, Credentials{
			Username: name + "-user",
			Password: "password",
		}); err != nil {
			t.Fatalf("Save(%s) failed: %v", name, err)
		}
	}

	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list connections: %v", err)
	}

	if len(listed) != len(names) {
		t.Errorf("Expected %d connections, got %d", len(names), len(listed))
	}

	nameMap := make(map[string]bool)
	for _, n := range listed {
		nameMap[n] = true
	}

	for _, name := range names {
		if !nameMap[name] {
			t.Errorf("Connection %s not found in list", name)
		}
	}
}

// TestMockStore_Available tests availability check
func TestMockStore_Available(t *testing.T) {
	store := NewMockStore()
	if !store.Available() {
		t.Error("Mock store should always be available")
	}
}

// TestMockStore_Type tests store type
func TestMockStore_Type(t *testing.T) {
	store := NewMockStore()
	if store.Type() != "mock" {
		t.Errorf("Expected type 'mock', got '%s'", store.Type())
	}
}

// TestMockStore_UpdateCredentials tests updating existing credentials
func TestMockStore_UpdateCredentials(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	// Save initial credentials
	initialCreds := Credentials{
		Username: "user1",
		Password: "pass1",
	}
	if err := store.Save(ctx, "test-conn", initialCreds); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Update credentials
	updatedCreds := Credentials{
		Username: "user2",
		Password: "pass2",
	}
	if err := store.Save(ctx, "test-conn", updatedCreds); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load and verify updated credentials
	loadedCreds, err := store.Load(ctx, "test-conn")
	if err != nil {
		t.Fatalf("Failed to load credentials: %v", err)
	}

	if loadedCreds.Username != updatedCreds.Username {
		t.Errorf("Expected username %s, got %s", updatedCreds.Username, loadedCreds.Username)
	}

	if loadedCreds.Password != updatedCreds.Password {
		t.Errorf("Expected password %s, got %s", updatedCreds.Password, loadedCreds.Password)
	}
}
