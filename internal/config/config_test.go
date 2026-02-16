package config

import (
	"testing"
)

// TestDefaultConfig tests the default configuration
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Settings.DefaultProfile != "local" {
		t.Errorf("Expected default profile 'local', got '%s'", cfg.Settings.DefaultProfile)
	}

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

// TestAddProfile tests adding a profile
func TestAddProfile(t *testing.T) {
	cfg := DefaultConfig()

	profile := &Profile{
		Name:     "test-db",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
		SSLMode:  "require",
		ReadOnly: false,
	}

	cfg.AddProfile(profile)

	// Verify profile was added
	if len(cfg.Profiles) != 1 {
		t.Errorf("Expected 1 profile, got %d", len(cfg.Profiles))
	}

	loadedProfile, ok := cfg.Profiles["test-db"]
	if !ok {
		t.Fatal("Profile 'test-db' not found")
	}

	if loadedProfile.Host != profile.Host {
		t.Errorf("Expected host %s, got %s", profile.Host, loadedProfile.Host)
	}

	if loadedProfile.Port != profile.Port {
		t.Errorf("Expected port %d, got %d", profile.Port, loadedProfile.Port)
	}

	// First profile should become default
	if cfg.Settings.DefaultProfile != "test-db" {
		t.Errorf("Expected default profile 'test-db', got '%s'", cfg.Settings.DefaultProfile)
	}
}

// TestGetProfile tests retrieving a profile
func TestGetProfile(t *testing.T) {
	cfg := DefaultConfig()

	profile := &Profile{
		Name:     "test-db",
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "testuser",
	}

	cfg.AddProfile(profile)

	// Get profile by name
	loadedProfile, err := cfg.GetProfile("test-db")
	if err != nil {
		t.Fatalf("Failed to get profile: %v", err)
	}

	if loadedProfile.Name != profile.Name {
		t.Errorf("Expected name %s, got %s", profile.Name, loadedProfile.Name)
	}

	// Get default profile (empty name)
	defaultProfile, err := cfg.GetProfile("")
	if err != nil {
		t.Fatalf("Failed to get default profile: %v", err)
	}

	if defaultProfile.Name != profile.Name {
		t.Errorf("Expected default profile %s, got %s", profile.Name, defaultProfile.Name)
	}
}

// TestGetProfile_Nonexistent tests getting a nonexistent profile
func TestGetProfile_Nonexistent(t *testing.T) {
	cfg := DefaultConfig()

	_, err := cfg.GetProfile("nonexistent")
	if err == nil {
		t.Error("Expected error when getting nonexistent profile")
	}
}

// TestRemoveProfile tests removing a profile
func TestRemoveProfile(t *testing.T) {
	cfg := DefaultConfig()

	// Add two profiles
	cfg.AddProfile(&Profile{
		Name:     "profile1",
		Host:     "host1",
		Database: "db1",
		Username: "user1",
	})

	cfg.AddProfile(&Profile{
		Name:     "profile2",
		Host:     "host2",
		Database: "db2",
		Username: "user2",
	})

	// Remove profile1
	err := cfg.RemoveProfile("profile1")
	if err != nil {
		t.Fatalf("Failed to remove profile: %v", err)
	}

	// Verify removal
	if len(cfg.Profiles) != 1 {
		t.Errorf("Expected 1 profile remaining, got %d", len(cfg.Profiles))
	}

	_, ok := cfg.Profiles["profile1"]
	if ok {
		t.Error("Profile 'profile1' should be removed")
	}

	// Default should have changed
	if cfg.Settings.DefaultProfile == "profile1" {
		t.Error("Default profile should not be the removed profile")
	}
}

// TestRemoveProfile_LastProfile tests removing the last profile
func TestRemoveProfile_LastProfile(t *testing.T) {
	cfg := DefaultConfig()

	// Add one profile
	cfg.AddProfile(&Profile{
		Name:     "only-profile",
		Host:     "host",
		Database: "db",
		Username: "user",
	})

	// Remove it
	err := cfg.RemoveProfile("only-profile")
	if err != nil {
		t.Fatalf("Failed to remove profile: %v", err)
	}

	// Default profile should be empty
	if cfg.Settings.DefaultProfile != "" {
		t.Errorf("Expected empty default profile, got '%s'", cfg.Settings.DefaultProfile)
	}
}

// TestRemoveProfile_Nonexistent tests removing a nonexistent profile
func TestRemoveProfile_Nonexistent(t *testing.T) {
	cfg := DefaultConfig()

	err := cfg.RemoveProfile("nonexistent")
	if err == nil {
		t.Error("Expected error when removing nonexistent profile")
	}
}

// TestListProfiles tests listing all profiles
func TestListProfiles(t *testing.T) {
	cfg := DefaultConfig()

	// Add multiple profiles
	profileNames := []string{"profile1", "profile2", "profile3"}
	for _, name := range profileNames {
		cfg.AddProfile(&Profile{
			Name:     name,
			Host:     "host",
			Database: "db",
			Username: "user",
		})
	}

	// List profiles
	listedProfiles := cfg.ListProfiles()

	if len(listedProfiles) != len(profileNames) {
		t.Errorf("Expected %d profiles, got %d", len(profileNames), len(listedProfiles))
	}

	// Verify all profiles are present
	profileMap := make(map[string]bool)
	for _, name := range listedProfiles {
		profileMap[name] = true
	}

	for _, name := range profileNames {
		if !profileMap[name] {
			t.Errorf("Profile %s not found in list", name)
		}
	}
}

// TestProfileDefaults tests profile default values
func TestProfileDefaults(t *testing.T) {
	profile := &Profile{
		Name:     "test",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	}

	// Default port should be 0 (will be set to 5432 by defaults or during connection)
	if profile.Port != 0 {
		t.Errorf("Expected default port 0, got %d", profile.Port)
	}

	// Default readonly should be false
	if profile.ReadOnly {
		t.Error("Expected default ReadOnly to be false")
	}
}
