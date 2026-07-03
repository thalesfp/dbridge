package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli/form"
	"github.com/thalesfp/dbridge/internal/cli/output"
	"github.com/thalesfp/dbridge/internal/config"
	"github.com/thalesfp/dbridge/internal/credentials"
	dbpkg "github.com/thalesfp/dbridge/internal/database"
	"golang.org/x/term"
)

// ContextKey is a custom type for context keys to avoid collisions.
type ContextKey string

// HumanOutputKey is the context key for the human output flag.
const HumanOutputKey ContextKey = "human_output"

// NewConfigCmd creates the config command
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage database connections",
		Long:  "Add, edit, remove, and list database connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)
			if formatter.HumanMode {
				return runManageTUI()
			}
			return cmd.Help()
		},
	}

	cmd.AddCommand(newConfigAddCmd())
	cmd.AddCommand(newConfigListCmd())
	cmd.AddCommand(newConfigRemoveCmd())
	cmd.AddCommand(newConfigCloneCmd())

	manageCmd := newConfigManageCmd()
	manageCmd.Hidden = true
	cmd.AddCommand(manageCmd)

	return cmd
}

// newConfigAddCmd creates the 'config add' command
func newConfigAddCmd() *cobra.Command {
	var (
		driver         string
		host           string
		port           int
		database       string
		username       string
		password       string
		sslMode        string
		testConnection bool
		environment    string
		description    string
	)

	cmd := &cobra.Command{
		Use:   "add [connection-name]",
		Short: "Add a new connection",
		Long: `Add a new database connection.

By default, launches an interactive form with visual validation and progress tracking.
You can also use flags for non-interactive/scripted usage.

Interactive mode (default):
  - Visual validation with inline error messages
  - Progress tracking (Step X/3)
  - Arrow key navigation (↑/↓ to move between fields)
  - Secure password input with masking
  - Password confirmation

Flag mode (for automation):
  - Provide all required flags to skip the interactive form
  - Omit --password to be prompted securely; avoid passing passwords on the command line
    (shell history, process listings, and CI logs may expose them)

Examples:
  dbridge config add                    # Interactive TUI form
  dbridge config add production         # Interactive with pre-filled name

  # Flag-based (non-interactive) — password will be prompted
  dbridge config add mydb --host=localhost --database=myapp --username=admin
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get formatter
			formatter := getFormatter(cmd)

			// Check if any flags were explicitly set (flag-based mode)
			flagMode := cmd.Flags().Changed("database") || cmd.Flags().Changed("username") || cmd.Flags().Changed("driver")

			// Default (JSON) mode requires flag mode (no interactive TUI)
			if !formatter.HumanMode && !flagMode {
				return formatError(cmd, "invalid_mode",
					"Interactive mode requires a terminal (or --human flag)",
					nil)
			}

			var connData *form.ConnectionData
			var err error

			if flagMode {
				// Flag-based mode: Use provided flags
				if len(args) == 0 {
					return fmt.Errorf("connection name is required when using flags")
				}
				connName := args[0]

				// Validate required fields
				validDrivers := dbpkg.DriverNames()
				if driver == "" {
					return fmt.Errorf("--driver is required (%s)", strings.Join(validDrivers, ", "))
				}
				driverValid := false
				for _, d := range validDrivers {
					if driver == d {
						driverValid = true
						break
					}
				}
				if !driverValid {
					return fmt.Errorf("unsupported driver %q, must be one of: %s", driver, strings.Join(validDrivers, ", "))
				}
				if database == "" {
					return fmt.Errorf("--database is required")
				}
				if username == "" {
					return fmt.Errorf("--username is required")
				}

				// Get password if not provided via flag
				if !cmd.Flags().Changed("password") {
					if !formatter.HumanMode {
						return formatError(cmd, "missing_password",
							"--password flag is required in non-interactive mode (use --password=\"\" for passwordless auth)",
							nil)
					}
					fmt.Printf("Enter password for %s@%s (or press Enter for passwordless auth): ", username, host)
					passwordBytes, err := readPassword()
					if err != nil {
						return fmt.Errorf("failed to read password: %w", err)
					}
					fmt.Println()

					password = string(passwordBytes)

					// Only confirm if password was provided
					if password != "" {
						fmt.Print("Confirm password: ")
						confirmBytes, err := readPassword()
						if err != nil {
							return fmt.Errorf("failed to read password confirmation: %w", err)
						}
						fmt.Println()

						if string(confirmBytes) != password {
							return fmt.Errorf("passwords do not match")
						}
					}
				}

				// Apply driver-aware defaults
				if defaults, ok := config.DriverDefaultsMap()[driver]; ok {
					if !cmd.Flags().Changed("port") {
						port = defaults.Port
					}
					if !cmd.Flags().Changed("ssl-mode") {
						sslMode = defaults.SSLMode
					}
				}

				connData = &form.ConnectionData{
					Driver:      driver,
					Name:        connName,
					Host:        host,
					Port:        port,
					Database:    database,
					Username:    username,
					SSLMode:     sslMode,
					Password:    password,
					Environment: environment,
					Description: description,
				}
			} else {
				// Interactive TUI mode — form handles saving via WithSave
				initialName := ""
				if len(args) > 0 {
					initialName = args[0]
				}

				initial := &form.ConnectionData{Name: initialName}
				connData, err = form.RunConnectionForm(initial, testConnectionOption(), form.WithSave(saveConnectionCallback))
				if err != nil {
					return fmt.Errorf("form cancelled or error: %w", err)
				}
			}

			if flagMode {
				// Flag-based mode: save manually (interactive mode already saved via callback)
				cfg, err := config.Load()
				if err != nil {
					return fmt.Errorf("failed to load config: %w", err)
				}

				connCfg := &config.Connection{
					Driver:      connData.Driver,
					Name:        connData.Name,
					Host:        connData.Host,
					Port:        connData.Port,
					Database:    connData.Database,
					Username:    connData.Username,
					SSLMode:     connData.SSLMode,
					SRV:         connData.SRV,
					Environment: connData.Environment,
					Description: connData.Description,
				}

				credStore, err := credentials.NewStore("dbridge")
				if err != nil {
					return fmt.Errorf("failed to open credential store: %w", err)
				}

				ctx := context.Background()

				if connData.Password != "" {
					if err := credStore.Save(ctx, connData.Name, credentials.Credentials{
						Username: connData.Username,
						Password: connData.Password,
					}); err != nil {
						return fmt.Errorf("failed to save credentials: %w", err)
					}
				}

				cfg.AddConnection(connCfg)
				if err := cfg.Save(); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}
			}

			// Test connection if requested via flag
			var connTestResult *connectionTestResult
			if testConnection {
				ctx := context.Background()
				connTestResult = runConnectionTest(ctx, connData)
			}

			// Output success message
			if !formatter.HumanMode {
				// JSON output
				credStore := "none"
				if connData.Password != "" {
					store, _ := credentials.NewStore("dbridge")
					if store != nil {
						credStore = store.Type()
					}
				}

				data := map[string]interface{}{
					"connection": map[string]interface{}{
						"driver":   connData.Driver,
						"name":     connData.Name,
						"host":     connData.Host,
						"port":     connData.Port,
						"database": connData.Database,
						"username": connData.Username,
						"ssl_mode": connData.SSLMode,
					},
					"credentials_stored": connData.Password != "",
					"credential_store":   credStore,
				}

				if connTestResult != nil {
					data["connection_test"] = connTestResult.toMap()
				}

				msg := fmt.Sprintf("Connection '%s' added successfully", connData.Name)
				return formatter.Success("config_add", data, msg)
			} else if flagMode {
				// Simple output for flag mode
				fmt.Printf("✓ Connection '%s' added successfully\n", connData.Name)
				if connData.Password != "" {
					store, _ := credentials.NewStore("dbridge")
					if store != nil {
						fmt.Printf("✓ Credentials stored in %s\n", store.Type())
					}
				} else {
					fmt.Printf("✓ Using passwordless authentication\n")
				}
				if connTestResult != nil {
					printConnectionTestResult(connTestResult)
				}
			}
			// Interactive mode: save dialog is shown by the form itself

			return nil
		},
	}

	// Add flags for non-interactive mode
	cmd.Flags().StringVar(&driver, "driver", "", fmt.Sprintf("Database driver (%s) — required", strings.Join(dbpkg.DriverNames(), ", ")))
	cmd.Flags().StringVar(&host, "host", "localhost", "Database host")
	cmd.Flags().IntVar(&port, "port", 0, "Database port (default: driver-specific)")
	cmd.Flags().StringVar(&database, "database", "", "Database name (required for flag mode)")
	cmd.Flags().StringVar(&username, "username", "", "Database username (required for flag mode)")
	cmd.Flags().StringVar(&password, "password", "", "Database password (optional, will prompt if not provided)")
	cmd.Flags().StringVar(&sslMode, "ssl-mode", "", "SSL mode (default: driver-specific)")
	cmd.Flags().BoolVar(&testConnection, "test-connection", false, "Test the database connection after adding")
	cmd.Flags().StringVar(&environment, "environment", "production", "Environment (production, staging, development, local)")
	cmd.Flags().StringVar(&description, "description", "", "Connection description")

	return cmd
}

// readPassword reads a password from stdin without echoing
func readPassword() ([]byte, error) {
	return term.ReadPassword(int(syscall.Stdin))
}

// getFormatter creates a formatter instance based on the human output flag
func getFormatter(cmd *cobra.Command) *output.Formatter {
	humanMode := false
	if val := cmd.Context().Value(HumanOutputKey); val != nil {
		humanMode = val.(bool)
	}
	return output.NewFormatter(humanMode)
}

// HandledError is an error that has already been output (e.g. as JSON)
type HandledError struct {
	Message string
}

func (e *HandledError) Error() string { return e.Message }

// formatError outputs a structured error and returns it
func formatError(cmd *cobra.Command, code, message string, details interface{}) error {
	formatter := getFormatter(cmd)
	if !formatter.HumanMode {
		_ = formatter.Error(code, message, details)
		return &HandledError{Message: message}
	}
	return fmt.Errorf("%s", message)
}

// newConfigListCmd creates the 'config list' command
func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if len(cfg.Connections) == 0 {
				if !formatter.HumanMode {
					return formatter.Success("config_list", map[string]interface{}{
						"connections": []interface{}{},
					}, "No connections configured")
				}
				fmt.Println("No connections configured")
				return nil
			}

			names := cfg.ListConnections()
			sort.Strings(names)

			if !formatter.HumanMode {
				connections := make([]map[string]interface{}, 0, len(names))
				for _, name := range names {
					conn := cfg.Connections[name]
					driver := conn.Driver
					if driver == "" {
						driver = "postgres"
					}
					entry := map[string]interface{}{
						"name":        name,
						"driver":      driver,
						"disabled":    conn.Disabled,
						"environment": conn.Environment,
					}
					if conn.Description != "" {
						entry["description"] = conn.Description
					}
					connections = append(connections, entry)
				}
				return formatter.Success("config_list", map[string]interface{}{
					"connections": connections,
				}, fmt.Sprintf("Found %d connection(s)", len(names)))
			}

			fmt.Printf("  %-30s %-12s %-14s %-10s %s\n", "NAME", "DRIVER", "ENV", "STATUS", "DESCRIPTION")
			for _, name := range names {
				conn := cfg.Connections[name]
				driver := conn.Driver
				if driver == "" {
					driver = "postgres"
				}

				env := conn.Environment
				if env == "" {
					env = "-"
				}

				status := "enabled"
				if conn.Disabled {
					status = "disabled"
				}

				desc := conn.Description
				if desc == "" {
					desc = "-"
				}

				fmt.Printf("  %-30s %-12s %-14s %-10s %s\n", name, driver, env, status, desc)
			}

			return nil
		},
	}
}

// newConfigRemoveCmd creates the 'config remove' command
func newConfigRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [connection-name]",
		Short: "Remove a connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)
			connName := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Remove credentials from keychain
			credStore, err := credentials.NewStore("dbridge")
			if err != nil {
				return fmt.Errorf("failed to open credential store: %w", err)
			}

			ctx := context.Background()
			credDeleteErr := credStore.Delete(ctx, connName)
			if credDeleteErr != nil && formatter.HumanMode {
				fmt.Printf("Warning: failed to delete credentials: %v\n", credDeleteErr)
			}

			// Remove connection from config
			if err := cfg.RemoveConnection(connName); err != nil {
				return formatError(cmd, "connection_not_found",
					fmt.Sprintf("connection not found: %s", connName),
					map[string]interface{}{
						"connection_name":       connName,
						"available_connections": cfg.ListConnections(),
					})
			}

			// Save config
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			if !formatter.HumanMode {
				data := map[string]interface{}{
					"connection_name":     connName,
					"credentials_deleted": credDeleteErr == nil,
				}
				return formatter.Success("config_remove", data,
					fmt.Sprintf("Connection '%s' removed successfully", connName))
			}

			fmt.Printf("✓ Connection '%s' removed\n", connName)

			return nil
		},
	}
}

// newConfigCloneCmd creates the 'config clone' command
func newConfigCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <source-connection> [new-connection-name]",
		Short: "Clone an existing connection with a new name",
		Long: `Clone an existing database connection to create a new one.

All settings from the source connection will be copied to the new connection.
You can interactively edit any fields during the cloning process.

The new connection will have its own separate credentials in the keychain.

Examples:
  dbridge config clone production staging
  dbridge config clone local     # Interactive name prompt
`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)
			sourceConnName := args[0]

			// Default (JSON) mode not supported for clone (requires interactive TUI)
			if !formatter.HumanMode {
				return formatError(cmd, "invalid_mode",
					"Interactive mode requires a terminal (or --human flag)",
					nil)
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Get source connection
			sourceConn, err := cfg.GetConnection(sourceConnName)
			if err != nil {
				return fmt.Errorf("source connection not found: %w", err)
			}

			// Load source credentials (may not exist for passwordless connections)
			credStore, err := credentials.NewStore("dbridge")
			if err != nil {
				return fmt.Errorf("failed to open credential store: %w", err)
			}

			ctx := context.Background()
			sourceCreds, err := credStore.Load(ctx, sourceConnName)
			if err != nil && !errors.Is(err, credentials.ErrNotFound) {
				return fmt.Errorf("failed to load credentials for '%s': %w", sourceConnName, err)
			}
			sourcePassword := sourceCreds.Password

			// Determine new connection name
			newConnName := ""
			if len(args) > 1 {
				newConnName = args[1]
			}

			// Launch form pre-filled with source connection data
			_, err = form.RunConnectionForm(&form.ConnectionData{
				Driver:      sourceConn.Driver,
				Name:        newConnName,
				Database:    sourceConn.Database,
				Host:        sourceConn.Host,
				Port:        sourceConn.Port,
				Username:    sourceConn.Username,
				SSLMode:     sourceConn.SSLMode,
				Password:    sourcePassword,
				SRV:         sourceConn.SRV,
				Environment: sourceConn.Environment,
				Description: sourceConn.Description,
			}, testConnectionOption(), form.WithSave(saveConnectionCallback))

			return err
		},
	}
}

// establishRenamedCredential stores the edited connection's credential under its
// new name without touching the old key, so a later config-save failure cannot
// lose the secret. Used only when the connection is being renamed.
func establishRenamedCredential(ctx context.Context, store credentials.Store, oldName string, d *form.ConnectionData) error {
	switch {
	case d.PasswordChanged && d.Password == "":
		if err := store.Delete(ctx, d.Name); err != nil && !errors.Is(err, credentials.ErrNotFound) {
			return fmt.Errorf("failed to clear credentials: %w", err)
		}
	case d.PasswordChanged:
		if err := store.Save(ctx, d.Name, credentials.Credentials{Username: d.Username, Password: d.Password}); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
	default:
		existing, err := store.Load(ctx, oldName)
		if err != nil && !errors.Is(err, credentials.ErrNotFound) {
			return fmt.Errorf("failed to read existing credentials: %w", err)
		}
		if errors.Is(err, credentials.ErrNotFound) {
			// The source is passwordless, so the new name must be too. Clear any
			// stale secret already sitting under the new name (an orphan from an
			// earlier failed edit) so the renamed connection cannot silently
			// authenticate with an unrelated credential.
			if delErr := store.Delete(ctx, d.Name); delErr != nil && !errors.Is(delErr, credentials.ErrNotFound) {
				return fmt.Errorf("failed to clear stale credentials: %w", delErr)
			}
			return nil
		}
		existing.Username = d.Username
		if err := store.Save(ctx, d.Name, existing); err != nil {
			return fmt.Errorf("failed to migrate credentials: %w", err)
		}
	}
	return nil
}

// applySameNamePassword applies an explicit password set/clear to a connection
// whose name did not change.
func applySameNamePassword(ctx context.Context, store credentials.Store, d *form.ConnectionData) error {
	if d.Password == "" {
		if err := store.Delete(ctx, d.Name); err != nil && !errors.Is(err, credentials.ErrNotFound) {
			return fmt.Errorf("failed to clear credentials: %w", err)
		}
		return nil
	}
	if err := store.Save(ctx, d.Name, credentials.Credentials{Username: d.Username, Password: d.Password}); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}
	return nil
}

// applyConnectionEdit reconciles the keychain and config for an edited connection
// so that no single step's failure can lose the stored secret:
//  1. On rename, establish the credential under the new name (old key untouched).
//  2. Persist config via saveConfig.
//  3. Only after the config is durable, remove the old key (rename) or apply a
//     same-name password change; a rejected config edit leaves the secret intact.
//
// A rename onto a name already used by another connection is rejected up front
// (nameTaken), so it cannot clobber that connection's config and credential.
//
// If saveConfig fails during a rename, nothing was persisted, so the credential
// staged under the new name is removed again to leave both stores as they were.
func applyConnectionEdit(ctx context.Context, store credentials.Store, oldName string, d *form.ConnectionData, nameTaken func(string) bool, saveConfig func() error) error {
	nameChanged := d.Name != oldName

	if nameChanged && nameTaken(d.Name) {
		return fmt.Errorf("a connection named %q already exists", d.Name)
	}

	if nameChanged {
		if err := establishRenamedCredential(ctx, store, oldName, d); err != nil {
			return err
		}
	}

	if err := saveConfig(); err != nil {
		saveErr := fmt.Errorf("failed to save config: %w", err)
		if nameChanged {
			if delErr := store.Delete(ctx, d.Name); delErr != nil && !errors.Is(delErr, credentials.ErrNotFound) {
				return errors.Join(saveErr, fmt.Errorf("failed to clean up staged credentials: %w", delErr))
			}
		}
		return saveErr
	}

	if nameChanged {
		if err := store.Delete(ctx, oldName); err != nil && !errors.Is(err, credentials.ErrNotFound) {
			return fmt.Errorf("connection saved, but failed to remove old credentials: %w", err)
		}
	} else if d.PasswordChanged {
		if err := applySameNamePassword(ctx, store, d); err != nil {
			return err
		}
	}

	return nil
}

// persistConnectionEdit applies the edited connection to cfg and persists it via
// save. On a rename the old entry is removed first. If save fails, nothing was
// written to disk, so the in-memory config is rolled back to its prior state:
// otherwise the half-applied map would block a retry (the collision guard would
// see the failed destination name) or write a phantom connection on the next save.
func persistConnectionEdit(cfg *config.Config, oldName string, updatedConn, existingConn *config.Connection, save func() error) error {
	nameChanged := updatedConn.Name != oldName

	if nameChanged {
		// oldName was loaded via GetConnection at flow entry, so it is present.
		_ = cfg.RemoveConnection(oldName)
	}
	cfg.AddConnection(updatedConn)

	if err := save(); err != nil {
		// updatedConn.Name was just added, so it is present.
		_ = cfg.RemoveConnection(updatedConn.Name)
		cfg.AddConnection(existingConn)
		return err
	}

	return nil
}

// runEditFlow runs the interactive edit flow for a connection
func runEditFlow(cfg *config.Config, connName string) error {
	// Get existing connection
	existingConn, err := cfg.GetConnection(connName)
	if err != nil {
		return fmt.Errorf("connection not found: %w", err)
	}

	// Load existing credentials (may not exist for passwordless connections)
	credStore, err := credentials.NewStore("dbridge")
	if err != nil {
		return fmt.Errorf("failed to open credential store: %w", err)
	}

	ctx := context.Background()

	// Prefill the existing password so an in-form connection test (ctrl+t)
	// authenticates for real instead of with an empty password.
	existingCreds, err := credStore.Load(ctx, connName)
	if err != nil && !errors.Is(err, credentials.ErrNotFound) {
		return fmt.Errorf("failed to load credentials for '%s': %w", connName, err)
	}

	// Launch form pre-filled with existing data
	editSaveFn := func(d *form.ConnectionData) string {
		nameTaken := func(name string) bool {
			_, exists := cfg.Connections[name]
			return exists
		}

		saveConfig := func() error {
			updatedConn := &config.Connection{
				Driver:      d.Driver,
				Name:        d.Name,
				Host:        d.Host,
				Port:        d.Port,
				Database:    d.Database,
				Username:    d.Username,
				SSLMode:     d.SSLMode,
				Disabled:    existingConn.Disabled,
				SRV:         d.SRV,
				Environment: d.Environment,
				Description: d.Description,
			}
			return persistConnectionEdit(cfg, connName, updatedConn, existingConn, cfg.Save)
		}

		if err := applyConnectionEdit(ctx, credStore, connName, d, nameTaken, saveConfig); err != nil {
			return err.Error()
		}
		return ""
	}

	_, err = form.RunConnectionForm(&form.ConnectionData{
		Driver:      existingConn.Driver,
		Name:        existingConn.Name,
		Database:    existingConn.Database,
		Host:        existingConn.Host,
		Port:        existingConn.Port,
		Username:    existingConn.Username,
		SSLMode:     existingConn.SSLMode,
		Password:    existingCreds.Password,
		SRV:         existingConn.SRV,
		Environment: existingConn.Environment,
		Description: existingConn.Description,
	}, testConnectionOption(), form.WithSave(editSaveFn))

	return err
}

// newConfigManageCmd creates the 'config manage' command
func newConfigManageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "manage",
		Short: "Interactive connection management menu",
		Long: `Interactively manage database connections.

Provides a menu to enable/disable and delete connections.
Requires a terminal for interactive mode.

Examples:
  dbridge config manage
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)

			if !formatter.HumanMode {
				return formatError(cmd, "invalid_mode",
					"Interactive mode requires a terminal (or --human flag)",
					nil)
			}

			return runManageTUI()
		},
	}
}

// toggleConnection flips the Disabled state and saves
func toggleConnection(cfg *config.Config, name string) error {
	conn := cfg.Connections[name]
	conn.Disabled = !conn.Disabled
	return cfg.Save()
}

// connectionTestResult holds the outcome of a connection test
type connectionTestResult struct {
	Success bool
	Error   string
}

func (r *connectionTestResult) toMap() map[string]interface{} {
	m := map[string]interface{}{
		"status": "ok",
	}
	if !r.Success {
		m["status"] = "failed"
		m["error"] = r.Error
	}
	return m
}

// testConnectionOption returns a FormOption that adds inline connection testing via ctrl+t.
func testConnectionOption() form.FormOption {
	return form.WithTestConnection(func(ctx context.Context, d *form.ConnectionData) string {
		result := runConnectionTest(ctx, d)
		if result.Success {
			return ""
		}
		return result.Error
	})
}

// runConnectionTest tests a database connection using the given connection data
func runConnectionTest(ctx context.Context, data *form.ConnectionData) *connectionTestResult {
	connConfig := &dbpkg.ConnectionConfig{
		Driver:   data.Driver,
		Host:     data.Host,
		Port:     data.Port,
		Database: data.Database,
		Username: data.Username,
		Password: data.Password,
		SSLMode:  data.SSLMode,
		SRV:      data.SRV,
	}
	conn, err := dbpkg.NewConnection(ctx, connConfig)
	if err != nil {
		return &connectionTestResult{Success: false, Error: simplifyConnError(err)}
	}
	conn.Close(ctx)
	return &connectionTestResult{Success: true}
}

// simplifyConnError extracts the root cause from verbose driver errors.
func simplifyConnError(err error) string {
	msg := err.Error()

	// Match common error patterns across the database drivers
	lower := strings.ToLower(msg)

	if strings.Contains(lower, "connection refused") {
		return "connection refused"
	}
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out") {
		return "connection timed out"
	}
	if strings.Contains(lower, "password authentication failed") || strings.Contains(lower, "access denied") || strings.Contains(lower, "login failed for user") {
		return "authentication failed (check username/password)"
	}
	if strings.Contains(lower, "does not exist") || strings.Contains(lower, "unknown database") {
		if strings.Contains(lower, "role") || strings.Contains(lower, "user") {
			return "user does not exist"
		}
		return "database does not exist"
	}
	if strings.Contains(lower, "no pg_hba.conf entry") {
		return "connection rejected by server (check pg_hba.conf)"
	}
	if strings.Contains(lower, "too many connections") {
		return "too many connections"
	}
	if strings.Contains(lower, "ssl") || strings.Contains(lower, "tls") {
		return "SSL/TLS error: " + lastLine(msg)
	}

	// Fallback: last meaningful line (usually the root cause)
	return lastLine(msg)
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return s
}

// printConnectionTestResult prints the connection test result for human-readable output
func printConnectionTestResult(result *connectionTestResult) {
	if result.Success {
		fmt.Println("✓ Connection test successful")
	} else {
		fmt.Printf("⚠ Connection test failed: %s\n", result.Error)
	}
}
