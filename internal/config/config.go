package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	Settings Settings            `mapstructure:"settings" yaml:"settings"`
	Connections map[string]*Connection `mapstructure:"connections" yaml:"connections"`
}

// Settings holds global application settings
type Settings struct {
	Output       OutputConfig `mapstructure:"output" yaml:"output"`
	Safety       SafetyConfig `mapstructure:"safety" yaml:"safety"`
	AuditLog     bool         `mapstructure:"audit_log" yaml:"audit_log"`
	AuditLogPath string       `mapstructure:"audit_log_path" yaml:"audit_log_path"`
}

// OutputConfig holds output format settings
type OutputConfig struct {
	Default         string `mapstructure:"default" yaml:"default"`                   // auto, compact, full, table, csv
	AutoDetectTTY   bool   `mapstructure:"auto_detect_tty" yaml:"auto_detect_tty"`   // Auto-switch based on TTY
	IncludeTypes    bool   `mapstructure:"include_types" yaml:"include_types"`        // Include column types
	IncludeTiming   bool   `mapstructure:"include_timing" yaml:"include_timing"`      // Include execution time
	IncludeWarnings bool   `mapstructure:"include_warnings" yaml:"include_warnings"`  // Include warnings
	SmartSimplify   bool   `mapstructure:"smart_simplify" yaml:"smart_simplify"`      // Smart format for single col/row
}

// SafetyConfig holds safety settings
type SafetyConfig struct {
	RequireConfirmation        []string `mapstructure:"require_confirmation" yaml:"require_confirmation"`                   // Operations requiring confirmation
	MaxRowsWithoutConfirmation int      `mapstructure:"max_rows_without_confirmation" yaml:"max_rows_without_confirmation"` // Threshold for confirmations
}

// Connection represents a named database connection
type Connection struct {
	Driver      string `mapstructure:"driver" yaml:"driver,omitempty"`
	Name        string `mapstructure:"name" yaml:"name"`
	Host        string `mapstructure:"host" yaml:"host"`
	Port        int    `mapstructure:"port" yaml:"port"`
	Database    string `mapstructure:"database" yaml:"database"`
	Username    string `mapstructure:"username" yaml:"username"`
	SSLMode     string `mapstructure:"ssl_mode" yaml:"ssl_mode"`
	Disabled    bool   `mapstructure:"disabled" yaml:"disabled"`
	URI         string `mapstructure:"uri" yaml:"uri,omitempty"`
	SRV         bool   `mapstructure:"srv" yaml:"srv,omitempty"`
	Environment string `mapstructure:"environment" yaml:"environment,omitempty"`
	Description string `mapstructure:"description" yaml:"description,omitempty"`
}

// DriverDefaults holds default port and SSL mode for a database driver.
type DriverDefaults struct {
	Port    int
	SSLMode string
}

var driverDefaults = map[string]DriverDefaults{
	"postgres": {5432, "verify-full"},
	"mysql":    {3306, "verify-full"},
	"mongodb":  {27017, "verify-full"},
}

// DriverDefaultsMap returns the driver defaults map for external use.
func DriverDefaultsMap() map[string]DriverDefaults {
	return driverDefaults
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Settings: Settings{
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
			AuditLog:     false,
			AuditLogPath: "",
		},
		Connections: make(map[string]*Connection),
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

	// Apply defaults if missing
	if config.Settings.Output.Default == "" {
		config.Settings.Output = defaults.Settings.Output
	}

	// Backward compat: migrate old "profiles" key to "connections"
	if len(config.Connections) == 0 && viper.IsSet("profiles") {
		var compat struct {
			Profiles map[string]*Connection `mapstructure:"profiles"`
		}
		if err := viper.Unmarshal(&compat); err == nil && len(compat.Profiles) > 0 {
			config.Connections = compat.Profiles
		}
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
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := filepath.Join(configDir, "config.yaml")

	viper.Set("settings", c.Settings)
	viper.Set("connections", c.Connections)
	// Clean up old "profiles" key after migration
	if viper.IsSet("profiles") {
		viper.Set("profiles", nil)
	}

	// Write to a temp file, set permissions, then rename so the final file
	// is never readable by others even briefly.
	tmpPath := filepath.Join(filepath.Dir(configPath), "config.tmp.yaml")
	if err := viper.WriteConfigAs(tmpPath); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	if err := os.Chmod(tmpPath, 0600); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to set config file permissions: %w", err)
	}
	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize config file: %w", err)
	}

	return nil
}

// AddConnection adds or updates a connection
func (c *Config) AddConnection(conn *Connection) {
	if c.Connections == nil {
		c.Connections = make(map[string]*Connection)
	}
	c.Connections[conn.Name] = conn
}

// GetConnection retrieves a connection by name
func (c *Config) GetConnection(name string) (*Connection, error) {
	if name == "" {
		return nil, fmt.Errorf("connection name is required")
	}

	conn, ok := c.Connections[name]
	if !ok {
		return nil, fmt.Errorf("connection '%s' not found", name)
	}

	// Default empty driver to postgres (backward compat for pre-multi-driver connections)
	if conn.Driver == "" {
		conn.Driver = "postgres"
	}

	// Apply driver-specific defaults
	defaults, ok := driverDefaults[conn.Driver]
	if !ok {
		// Unknown or empty driver: use postgres defaults for port/ssl
		defaults = driverDefaults["postgres"]
	}
	if conn.Port == 0 {
		conn.Port = defaults.Port
	}
	if conn.SSLMode == "" {
		conn.SSLMode = defaults.SSLMode
	}

	return conn, nil
}

// RemoveConnection removes a connection
func (c *Config) RemoveConnection(name string) error {
	if _, ok := c.Connections[name]; !ok {
		return fmt.Errorf("connection '%s' not found", name)
	}

	delete(c.Connections, name)

	return nil
}

// ListConnections returns all connection names
func (c *Config) ListConnections() []string {
	connections := make([]string, 0, len(c.Connections))
	for name := range c.Connections {
		connections = append(connections, name)
	}
	return connections
}

// getConfigDir returns the configuration directory path
func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".config", "dbridge"), nil
}
