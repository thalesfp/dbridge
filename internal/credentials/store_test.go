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

func (m *MockStore) Save(ctx context.Context, profile string, creds Credentials) error {
	m.data[profile] = creds
	return nil
}

func (m *MockStore) Load(ctx context.Context, profile string) (Credentials, error) {
	creds, ok := m.data[profile]
	if !ok {
		return Credentials{}, &KeyringError{Profile: profile}
	}
	return creds, nil
}

func (m *MockStore) Delete(ctx context.Context, profile string) error {
	delete(m.data, profile)
	return nil
}

func (m *MockStore) List(ctx context.Context) ([]string, error) {
	profiles := make([]string, 0, len(m.data))
	for profile := range m.data {
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (m *MockStore) Available() bool {
	return true
}

func (m *MockStore) Type() string {
	return "mock"
}

// KeyringError represents a keyring error
type KeyringError struct {
	Profile string
}

func (e *KeyringError) Error() string {
	return "profile not found: " + e.Profile
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

	err := store.Save(ctx, "test-profile", creds)
	if err != nil {
		t.Fatalf("Failed to save credentials: %v", err)
	}

	// Test loading credentials
	loadedCreds, err := store.Load(ctx, "test-profile")
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

// TestMockStore_LoadNonexistent tests loading a nonexistent profile
func TestMockStore_LoadNonexistent(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	_, err := store.Load(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error when loading nonexistent profile")
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
	store.Save(ctx, "test-profile", creds)

	// Delete credentials
	err := store.Delete(ctx, "test-profile")
	if err != nil {
		t.Fatalf("Failed to delete credentials: %v", err)
	}

	// Verify deletion
	_, err = store.Load(ctx, "test-profile")
	if err == nil {
		t.Error("Expected error after deleting credentials")
	}
}

// TestMockStore_List tests listing all profiles
func TestMockStore_List(t *testing.T) {
	store := NewMockStore()
	ctx := context.Background()

	// Add multiple profiles
	profiles := []string{"profile1", "profile2", "profile3"}
	for _, profile := range profiles {
		store.Save(ctx, profile, Credentials{
			Username: profile + "-user",
			Password: "password",
		})
	}

	// List profiles
	listedProfiles, err := store.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list profiles: %v", err)
	}

	if len(listedProfiles) != len(profiles) {
		t.Errorf("Expected %d profiles, got %d", len(profiles), len(listedProfiles))
	}

	// Verify all profiles are present
	profileMap := make(map[string]bool)
	for _, profile := range listedProfiles {
		profileMap[profile] = true
	}

	for _, profile := range profiles {
		if !profileMap[profile] {
			t.Errorf("Profile %s not found in list", profile)
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
	store.Save(ctx, "test-profile", initialCreds)

	// Update credentials
	updatedCreds := Credentials{
		Username: "user2",
		Password: "pass2",
	}
	store.Save(ctx, "test-profile", updatedCreds)

	// Load and verify updated credentials
	loadedCreds, err := store.Load(ctx, "test-profile")
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
