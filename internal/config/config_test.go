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

	// Empty name should return an error
	_, err = cfg.GetProfile("")
	if err == nil {
		t.Error("Expected error when getting profile with empty name")
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

	if len(cfg.Profiles) != 0 {
		t.Errorf("Expected 0 profiles, got %d", len(cfg.Profiles))
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

	// Default disabled should be false (zero value)
	if profile.Disabled {
		t.Error("Expected default Disabled to be false")
	}
}

// TestProfileDisabledField tests the Disabled field behavior
func TestProfileDisabledField(t *testing.T) {
	cfg := DefaultConfig()

	// Add an enabled profile (default)
	cfg.AddProfile(&Profile{
		Name:     "enabled-db",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})

	// Add a disabled profile
	cfg.AddProfile(&Profile{
		Name:     "disabled-db",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
		Disabled: true,
	})

	// Verify enabled profile
	enabledProfile, err := cfg.GetProfile("enabled-db")
	if err != nil {
		t.Fatalf("Failed to get enabled profile: %v", err)
	}
	if enabledProfile.Disabled {
		t.Error("Expected enabled profile to have Disabled=false")
	}

	// Verify disabled profile
	disabledProfile, err := cfg.GetProfile("disabled-db")
	if err != nil {
		t.Fatalf("Failed to get disabled profile: %v", err)
	}
	if !disabledProfile.Disabled {
		t.Error("Expected disabled profile to have Disabled=true")
	}
}

// TestToggleProfileDisabled tests toggling the Disabled state
func TestToggleProfileDisabled(t *testing.T) {
	cfg := DefaultConfig()

	cfg.AddProfile(&Profile{
		Name:     "toggle-test",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
	})

	profile := cfg.Profiles["toggle-test"]

	// Initially not disabled
	if profile.Disabled {
		t.Error("Expected profile to start as not disabled")
	}

	// Disable it
	profile.Disabled = true
	if !profile.Disabled {
		t.Error("Expected profile to be disabled after toggle")
	}

	// Re-enable it
	profile.Disabled = false
	if profile.Disabled {
		t.Error("Expected profile to be enabled after second toggle")
	}
}

// TestGetProfileStillWorksWhenDisabled tests that GetProfile returns disabled profiles
func TestGetProfileStillWorksWhenDisabled(t *testing.T) {
	cfg := DefaultConfig()

	cfg.AddProfile(&Profile{
		Name:     "disabled-profile",
		Host:     "localhost",
		Database: "testdb",
		Username: "user",
		Disabled: true,
	})

	// GetProfile should still return the profile (disabled check is at connection points)
	profile, err := cfg.GetProfile("disabled-profile")
	if err != nil {
		t.Fatalf("GetProfile should return disabled profiles, got error: %v", err)
	}
	if !profile.Disabled {
		t.Error("Expected profile to be disabled")
	}
}
