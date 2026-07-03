package credentials

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/99designs/keyring"
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
		return Credentials{}, ErrNotFound
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

// TestEncodeDecodeCredentials_RoundTrip verifies JSON serialization round-trips,
// including values that broke the legacy colon format (colons in the username).
func TestEncodeDecodeCredentials_RoundTrip(t *testing.T) {
	cases := []Credentials{
		{Username: "user", Password: "pass"},
		{Username: "user:with:colons", Password: "p@ss:w/ord?"},
		{Username: "", Password: "only-password"},
		{Username: "user", Password: ""},
	}
	for _, want := range cases {
		got := decodeCredentials(encodeCredentials(want))
		if got != want {
			t.Errorf("round-trip = %+v, want %+v", got, want)
		}
	}
}

// TestDecodeCredentials_LegacyFormats verifies old keychain entries still load.
func TestDecodeCredentials_LegacyFormats(t *testing.T) {
	if got := decodeCredentials([]byte("admin:secret")); got.Username != "admin" || got.Password != "secret" {
		t.Errorf("legacy colon form = %+v, want {admin secret}", got)
	}
	if got := decodeCredentials([]byte("barepassword")); got.Username != "" || got.Password != "barepassword" {
		t.Errorf("legacy bare password = %+v, want {\"\" barepassword}", got)
	}
}

// TestDecodeCredentials_LegacyPasswordLooksLikeJSON guards a migration hazard:
// a legacy value that happens to look like JSON must not be parsed as the new
// format. Expected outputs match the base version's decoder exactly (verified
// against main), so this branch neither introduces nor worsens the behavior.
func TestDecodeCredentials_LegacyPasswordLooksLikeJSON(t *testing.T) {
	// Colon-free JSON-looking bare password: preserved intact. The earlier
	// first-byte heuristic corrupted this to an empty secret; the scheme prefix
	// fixes it and restores the base decoder's result.
	if got := decodeCredentials([]byte("{}")); got.Username != "" || got.Password != "{}" {
		t.Errorf(`bare password "{}" = %+v, want {"" "{}"}`, got)
	}
	// JSON-looking value with a colon: decoded by the unchanged legacy colon
	// split, byte-for-byte as the base version did. The guarantee this branch
	// adds is only that it is never parsed as the new JSON format (which dropped
	// the secret entirely); the colon split itself is pre-existing behavior.
	if got := decodeCredentials([]byte(`{"a":1}`)); got.Username != `{"a"` || got.Password != `1}` {
		t.Errorf(`bare password '{"a":1}' = %+v, want {'{"a"' '1}'} (legacy colon split)`, got)
	}
}

// TestDecodeCredentials_LegacySchemeLabelCollision proves the scheme marker
// cannot collide with a legacy "username:password" value. main wrote text with
// fmt.Sprintf("%s:%s", ...), so even a username spelling out the scheme label
// starts with a printable byte, never the marker's leading NUL, and is decoded
// by the legacy colon split rather than parsed as the new JSON format.
func TestDecodeCredentials_LegacySchemeLabelCollision(t *testing.T) {
	legacy := "dbridge-credential/v1:" + `{"username":"x","password":"secret"}`

	got := decodeCredentials([]byte(legacy))
	if got.Username != "dbridge-credential/v1" {
		t.Errorf("username = %q, want the literal scheme label", got.Username)
	}
	if got.Password != `{"username":"x","password":"secret"}` {
		t.Errorf("password = %q, want the full legacy JSON string preserved", got.Password)
	}
}

// TestAllowedBackends_OsNativeOnly verifies that allowedBackends returns exactly
// one backend and that it is not a fallback backend (file or pass).
func TestAllowedBackends_OsNativeOnly(t *testing.T) {
	backends := allowedBackends()

	if len(backends) != 1 {
		t.Fatalf("expected exactly 1 allowed backend, got %d: %v", len(backends), backends)
	}

	for _, b := range backends {
		if b == keyring.FileBackend {
			t.Error("FileBackend must not be in allowed backends — credentials would be stored in plaintext")
		}
		if b == keyring.PassBackend {
			t.Error("PassBackend must not be in allowed backends — only OS-native keychains are permitted")
		}
	}
}

// TestAllowedBackends_MatchesPlatform verifies the backend matches the current OS.
func TestAllowedBackends_MatchesPlatform(t *testing.T) {
	backends := allowedBackends()
	got := backends[0]

	var want keyring.BackendType
	switch runtime.GOOS {
	case "darwin":
		want = keyring.KeychainBackend
	case "windows":
		want = keyring.WinCredBackend
	default:
		want = keyring.SecretServiceBackend
	}

	if got != want {
		t.Errorf("allowedBackends() on %s = %q, want %q", runtime.GOOS, got, want)
	}
}

// TestKeyringStore_Type_MatchesPlatform verifies Type() is consistent with allowedBackends().
func TestKeyringStore_Type_MatchesPlatform(t *testing.T) {
	s := &KeyringStore{}
	got := s.Type()
	want := backendTypeToName[allowedBackends()[0]]

	if got != want {
		t.Errorf("KeyringStore.Type() on %s = %q, want %q", runtime.GOOS, got, want)
	}
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
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
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
