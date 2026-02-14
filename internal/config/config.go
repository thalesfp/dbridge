package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Settings Settings            `mapstructure:"settings"`
	Profiles map[string]*Profile `mapstructure:"profiles"`
}

// Settings holds global application settings
type Settings struct {
	DefaultProfile string       `mapstructure:"default_profile"`
	Output         OutputConfig `mapstructure:"output"`
	Safety         SafetyConfig `mapstructure:"safety"`
	AuditLog       bool         `mapstructure:"audit_log"`
	AuditLogPath   string       `mapstructure:"audit_log_path"`
}

// OutputConfig holds output format settings
type OutputConfig struct {
	Default        string `mapstructure:"default"`         // auto, compact, full, table, csv
	AutoDetectTTY  bool   `mapstructure:"auto_detect_tty"` // Auto-switch based on TTY
	IncludeTypes   bool   `mapstructure:"include_types"`   // Include column types
	IncludeTiming  bool   `mapstructure:"include_timing"`  // Include execution time
	IncludeWarnings bool  `mapstructure:"include_warnings"` // Include warnings
	SmartSimplify  bool   `mapstructure:"smart_simplify"`  // Smart format for single col/row
}

// SafetyConfig holds safety settings
type SafetyConfig struct {
	RequireConfirmation        []string `mapstructure:"require_confirmation"`         // Operations requiring confirmation
	MaxRowsWithoutConfirmation int      `mapstructure:"max_rows_without_confirmation"` // Threshold for confirmations
}

// Profile represents a database connection profile
type Profile struct {
	Name     string `mapstructure:"name"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	Username string `mapstructure:"username"` // Optional: can be stored in keychain only
	SSLMode  string `mapstructure:"ssl_mode"`
	PoolSize int    `mapstructure:"pool_size"`
	ReadOnly bool   `mapstructure:"readonly"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Settings: Settings{
			DefaultProfile: "local",
			Output: OutputConfig{
				Default:         "auto",
				AutoDetectTTY:   true,
				IncludeTypes:    true,
				IncludeTiming:   false,
				IncludeWarnings: true,
				SmartSimplify:   true,
			},
			Safety: SafetyConfig{
				RequireConfirmation:        []string{"DELETE", "DROP", "TRUNCATE"},
				MaxRowsWithoutConfirmation: 1000,
			},
			AuditLog:     true,
			AuditLogPath: "~/.pgmcp/audit.log",
		},
		Profiles: make(map[string]*Profile),
	}
}

// Load loads configuration from file
func Load() (*Config, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return nil, err
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(configDir)

	// Set defaults
	defaults := DefaultConfig()
	viper.SetDefault("settings", defaults.Settings)

	// Try to read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, use defaults
			return defaults, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// Save saves configuration to file
func (c *Config) Save() error {
	configDir, err := getConfigDir()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")

	viper.Set("settings", c.Settings)
	viper.Set("profiles", c.Profiles)

	if err := viper.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// AddProfile adds or updates a profile
func (c *Config) AddProfile(profile *Profile) {
	if c.Profiles == nil {
		c.Profiles = make(map[string]*Profile)
	}
	c.Profiles[profile.Name] = profile

	// Set as default if it's the first profile
	if len(c.Profiles) == 1 {
		c.Settings.DefaultProfile = profile.Name
	}
}

// GetProfile retrieves a profile by name
func (c *Config) GetProfile(name string) (*Profile, error) {
	if name == "" {
		name = c.Settings.DefaultProfile
	}

	profile, ok := c.Profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile '%s' not found", name)
	}

	return profile, nil
}

// RemoveProfile removes a profile
func (c *Config) RemoveProfile(name string) error {
	if _, ok := c.Profiles[name]; !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}

	delete(c.Profiles, name)

	// Update default if removed
	if c.Settings.DefaultProfile == name {
		if len(c.Profiles) > 0 {
			// Set first available profile as default
			for profileName := range c.Profiles {
				c.Settings.DefaultProfile = profileName
				break
			}
		} else {
			c.Settings.DefaultProfile = ""
		}
	}

	return nil
}

// ListProfiles returns all profile names
func (c *Config) ListProfiles() []string {
	profiles := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		profiles = append(profiles, name)
	}
	return profiles
}

// getConfigDir returns the configuration directory path
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".config", "pgmcp"), nil
}
