package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"syscall"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
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
		Short: "Manage database connection profiles",
		Long:  "Add, edit, remove, and list database connection profiles",
	}

	cmd.AddCommand(newConfigAddCmd())
	cmd.AddCommand(newConfigListCmd())
	cmd.AddCommand(newConfigShowCmd())
	cmd.AddCommand(newConfigRemoveCmd())
	cmd.AddCommand(newConfigCloneCmd())
	cmd.AddCommand(newConfigManageCmd())

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
		readOnly       bool
		testConnection bool
	)

	cmd := &cobra.Command{
		Use:   "add [profile-name]",
		Short: "Add a new connection profile",
		Long: `Add a new database connection profile.

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
  - Use --password flag or omit it for interactive password prompt

Examples:
  dbridge config add                    # Interactive TUI form
  dbridge config add production         # Interactive with pre-filled name

  # Flag-based (non-interactive)
  dbridge config add mydb --host=localhost --database=myapp --username=admin --password=secret
  dbridge config add mydb --host=localhost --database=myapp --username=admin  # Password prompt only
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
					"Interactive mode requires --human flag",
					nil)
			}

			var profileData *form.ProfileData
			var err error

			if flagMode {
				// Flag-based mode: Use provided flags
				if len(args) == 0 {
					return fmt.Errorf("profile name is required when using flags")
				}
				profileName := args[0]

				// Validate required fields
				if driver == "" {
					return fmt.Errorf("--driver is required (postgres, mysql)")
				}
				validDrivers := dbpkg.DriverNames()
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

				profileData = &form.ProfileData{
					Driver:   driver,
					Name:     profileName,
					Host:     host,
					Port:     port,
					Database: database,
					Username: username,
					SSLMode:  sslMode,
					Password: password,
				}
			} else {
				// Interactive TUI mode
				initialName := ""
				if len(args) > 0 {
					initialName = args[0]
				}

				profileData, err = form.NewProfileForm(initialName)
				if err != nil {
					return fmt.Errorf("form cancelled or error: %w", err)
				}
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create profile
			profile := &config.Profile{
				Driver:   profileData.Driver,
				Name:     profileData.Name,
				Host:     profileData.Host,
				Port:     profileData.Port,
				Database: profileData.Database,
				Username: profileData.Username,
				SSLMode:  profileData.SSLMode,
				ReadOnly: true, // v1.0 is read-only only
			}

			// Save credentials to keychain
			credStore, err := credentials.NewStore("dbridge")
			if err != nil {
				return fmt.Errorf("failed to open credential store: %w", err)
			}

			ctx := context.Background()

			// Only save credentials if password is provided
			// For passwordless auth (trust, peer, cert), we skip credential storage
			if profileData.Password != "" {
				if err := credStore.Save(ctx, profileData.Name, credentials.Credentials{
					Username: profileData.Username,
					Password: profileData.Password,
				}); err != nil {
					return fmt.Errorf("failed to save credentials: %w", err)
				}
			}

			// Add profile to config
			cfg.AddProfile(profile)

			// Save config
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			// Test connection if requested (flag-based or TUI prompt)
			var connTestResult *connectionTestResult
			if !formatter.HumanMode {
				// JSON mode: use the flag
				if testConnection {
					connTestResult = runConnectionTest(ctx, profileData)
				}
			} else if flagMode {
				// Human flag mode: use the flag
				if testConnection {
					connTestResult = runConnectionTest(ctx, profileData)
				}
			} else {
				// Interactive TUI mode: prompt the user
				var runTest bool
				if err := huh.NewConfirm().
					Title("Test connection?").
					Value(&runTest).
					WithTheme(form.CustomTheme()).
					Run(); err != nil {
					return fmt.Errorf("prompt failed: %w", err)
				}

				if runTest {
					connTestResult = runConnectionTest(ctx, profileData)
				}
			}

			// Output success message
			if !formatter.HumanMode {
				// JSON output
				credStore := "none"
				if profileData.Password != "" {
					store, _ := credentials.NewStore("dbridge")
					if store != nil {
						credStore = store.Type()
					}
				}

				data := map[string]interface{}{
					"profile": map[string]interface{}{
						"driver":    profileData.Driver,
						"name":      profileData.Name,
						"host":      profileData.Host,
						"port":      profileData.Port,
						"database":  profileData.Database,
						"username":  profileData.Username,
						"ssl_mode":  profileData.SSLMode,
						"read_only": true,
					},
					"credentials_stored": profileData.Password != "",
					"credential_store":   credStore,
				}

				if connTestResult != nil {
					data["connection_test"] = connTestResult.toMap()
				}

				msg := fmt.Sprintf("Profile '%s' added successfully", profileData.Name)
				return formatter.Success("config_add", data, msg)
			} else if flagMode {
				// Simple output for flag mode
				fmt.Printf("✓ Profile '%s' added successfully\n", profileData.Name)
				if profileData.Password != "" {
					fmt.Printf("✓ Credentials stored in %s\n", credStore.Type())
				} else {
					fmt.Printf("✓ Using passwordless authentication\n")
				}
				if connTestResult != nil {
					printConnectionTestResult(connTestResult)
				}
			} else {
				// Enhanced success message for interactive mode
				successStyle := lipgloss.NewStyle().
					Foreground(lipgloss.Color("46")).
					Bold(true)

				boxStyle := lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("46")).
					Padding(1, 2)

				authInfo := credStore.Type()
				if profileData.Password == "" {
					authInfo = "Passwordless (trust/peer/cert)"
				}

				message := fmt.Sprintf(
					"%s Profile '%s' added successfully!\n\n"+
						"Host: %s:%d\n"+
						"Database: %s\n"+
						"Authentication: %s",
					successStyle.Render("✓"),
					profileData.Name,
					profileData.Host,
					profileData.Port,
					profileData.Database,
					authInfo,
				)

				fmt.Println("\n" + boxStyle.Render(message))
				if connTestResult != nil {
					printConnectionTestResult(connTestResult)
				}
			}

			return nil
		},
	}

	// Add flags for non-interactive mode
	cmd.Flags().StringVar(&driver, "driver", "", "Database driver (postgres, mysql) — required")
	cmd.Flags().StringVar(&host, "host", "localhost", "Database host")
	cmd.Flags().IntVar(&port, "port", 0, "Database port (default: driver-specific)")
	cmd.Flags().StringVar(&database, "database", "", "Database name (required for flag mode)")
	cmd.Flags().StringVar(&username, "username", "", "Database username (required for flag mode)")
	cmd.Flags().StringVar(&password, "password", "", "Database password (optional, will prompt if not provided)")
	cmd.Flags().StringVar(&sslMode, "ssl-mode", "", "SSL mode (default: driver-specific)")
	cmd.Flags().BoolVar(&readOnly, "readonly", true, "Read-only mode")
	cmd.Flags().BoolVar(&testConnection, "test-connection", false, "Test the database connection after adding the profile")

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

// getProfileNames returns a list of all profile names from config
func getProfileNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	return names
}

// newConfigListCmd creates the 'config list' command
func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all connection profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if len(cfg.Profiles) == 0 {
				if !formatter.HumanMode {
					return formatter.Success("config_list", map[string]interface{}{
						"profiles":    []interface{}{},
						"total_count": 0,
					}, "No profiles configured")
				}

				fmt.Println("No profiles configured")
				fmt.Println("\nAdd a profile with: dbridge config add <name>")
				return nil
			}

			if !formatter.HumanMode {
				profiles := make([]map[string]interface{}, 0, len(cfg.Profiles))
				for name, profile := range cfg.Profiles {
					driver := profile.Driver
					if driver == "" {
						driver = "postgres"
					}
					profiles = append(profiles, map[string]interface{}{
						"driver":     driver,
						"name":       name,
						"host":       profile.Host,
						"port":       profile.Port,
						"database":   profile.Database,
						"username":   profile.Username,
						"ssl_mode":   profile.SSLMode,
						"read_only":  profile.ReadOnly,
						"disabled":   profile.Disabled,
					})
				}

				return formatter.Success("config_list", map[string]interface{}{
					"profiles":        profiles,
					"total_count":     len(cfg.Profiles),
				}, fmt.Sprintf("Found %d profile(s)", len(cfg.Profiles)))
			}

			fmt.Println("Profiles:")
			for name, profile := range cfg.Profiles {
				markers := ""
				if profile.ReadOnly {
					markers += " [read-only]"
				} else {
					markers += " [read-write]"
				}
				if profile.Disabled {
					markers += " [DISABLED]"
				}

				fmt.Printf("  %s - %s:%d/%s%s\n",
					name,
					profile.Host,
					profile.Port,
					profile.Database,
					markers,
				)
			}

			return nil
		},
	}
}

// newConfigShowCmd creates the 'config show' command
func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show [profile-name]",
		Short: "Show profile details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)
			profileName := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			profile, err := cfg.GetProfile(profileName)
			if err != nil {
				return formatError(cmd, "profile_not_found",
					fmt.Sprintf("profile not found: %s", profileName),
					map[string]interface{}{
						"profile_name":       profileName,
						"available_profiles": getProfileNames(cfg),
					})
			}

			if !formatter.HumanMode {
				data := map[string]interface{}{
					"profile": map[string]interface{}{
						"driver":     profile.Driver,
						"name":       profile.Name,
						"host":       profile.Host,
						"port":       profile.Port,
						"database":   profile.Database,
						"username":   profile.Username,
						"ssl_mode":   profile.SSLMode,
						"read_only":  profile.ReadOnly,
						"disabled":   profile.Disabled,
					},
					"has_credentials": true,
				}

				return formatter.Success("config_show", data,
					fmt.Sprintf("Profile '%s' details", profileName))
			}

			fmt.Printf("Profile: %s\n", profile.Name)
			fmt.Printf("Driver: %s\n", profile.Driver)
			fmt.Printf("Host: %s\n", profile.Host)
			fmt.Printf("Port: %d\n", profile.Port)
			fmt.Printf("Database: %s\n", profile.Database)
			fmt.Printf("Username: %s\n", profile.Username)
			fmt.Printf("SSL Mode: %s\n", profile.SSLMode)
			fmt.Printf("Read-only: %t\n", profile.ReadOnly)
			fmt.Printf("Credentials: stored in keychain\n")

			return nil
		},
	}
}

// newConfigRemoveCmd creates the 'config remove' command
func newConfigRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [profile-name]",
		Short: "Remove a connection profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)
			profileName := args[0]

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
			credDeleteErr := credStore.Delete(ctx, profileName)
			if credDeleteErr != nil && formatter.HumanMode {
				fmt.Printf("Warning: failed to delete credentials: %v\n", credDeleteErr)
			}

			// Remove profile from config
			if err := cfg.RemoveProfile(profileName); err != nil {
				return formatError(cmd, "profile_not_found",
					fmt.Sprintf("profile not found: %s", profileName),
					map[string]interface{}{
						"profile_name":       profileName,
						"available_profiles": getProfileNames(cfg),
					})
			}

			// Save config
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			if !formatter.HumanMode {
				data := map[string]interface{}{
					"profile_name":        profileName,
					"credentials_deleted": credDeleteErr == nil,
				}
				return formatter.Success("config_remove", data,
					fmt.Sprintf("Profile '%s' removed successfully", profileName))
			}

			fmt.Printf("✓ Profile '%s' removed\n", profileName)

			return nil
		},
	}
}

// newConfigCloneCmd creates the 'config clone' command
func newConfigCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <source-profile> [new-profile-name]",
		Short: "Clone an existing profile with a new name",
		Long: `Clone an existing database profile to create a new one.

All settings from the source profile will be copied to the new profile.
You can interactively edit any fields during the cloning process.

The new profile will have its own separate credentials in the keychain.

Examples:
  dbridge config clone production staging
  dbridge config clone local     # Interactive name prompt
`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)
			sourceProfileName := args[0]

			// Default (JSON) mode not supported for clone (requires interactive TUI)
			if !formatter.HumanMode {
				return formatError(cmd, "invalid_mode",
					"Interactive mode requires --human flag",
					nil)
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Get source profile
			sourceProfile, err := cfg.GetProfile(sourceProfileName)
			if err != nil {
				return fmt.Errorf("source profile not found: %w", err)
			}

			// Load source credentials (may not exist for passwordless profiles)
			credStore, err := credentials.NewStore("dbridge")
			if err != nil {
				return fmt.Errorf("failed to open credential store: %w", err)
			}

			ctx := context.Background()
			sourceCreds, err := credStore.Load(ctx, sourceProfileName)

			// Default to empty password if credentials don't exist (passwordless profile)
			sourcePassword := ""
			if err == nil {
				sourcePassword = sourceCreds.Password
			}

			// Determine new profile name
			newProfileName := ""
			if len(args) > 1 {
				newProfileName = args[1]
			}

			// Launch form pre-filled with source profile data
			profileData, err := form.NewProfileFormWithDefaults(
				sourceProfile.Driver,
				newProfileName,
				sourceProfile.Database,
				sourceProfile.Host,
				sourceProfile.Port,
				sourceProfile.Username,
				sourceProfile.SSLMode,
				sourcePassword, // Pre-fill password from source (may be empty)
			)
			if err != nil {
				return fmt.Errorf("clone cancelled or error: %w", err)
			}

			// Create new profile
			newProfile := &config.Profile{
				Driver:   profileData.Driver,
				Name:     profileData.Name,
				Host:     profileData.Host,
				Port:     profileData.Port,
				Database: profileData.Database,
				Username: profileData.Username,
				SSLMode:  profileData.SSLMode,
				ReadOnly: true,
			}

			// Save new credentials (only if password provided)
			if profileData.Password != "" {
				if err := credStore.Save(ctx, profileData.Name, credentials.Credentials{
					Username: profileData.Username,
					Password: profileData.Password,
				}); err != nil {
					return fmt.Errorf("failed to save credentials: %w", err)
				}
			}

			// Add new profile to config
			cfg.AddProfile(newProfile)

			// Save config
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("\n✓ Profile '%s' cloned from '%s'\n", profileData.Name, sourceProfileName)
			if profileData.Password != "" {
				fmt.Printf("✓ Credentials stored in %s\n", credStore.Type())
			} else {
				fmt.Printf("✓ Using passwordless authentication\n")
			}

			return nil
		},
	}
}

// runEditFlow runs the interactive edit flow for a profile
func runEditFlow(cfg *config.Config, profileName string) error {
	// Get existing profile
	existingProfile, err := cfg.GetProfile(profileName)
	if err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}

	// Load existing credentials (may not exist for passwordless profiles)
	credStore, err := credentials.NewStore("dbridge")
	if err != nil {
		return fmt.Errorf("failed to open credential store: %w", err)
	}

	ctx := context.Background()
	existingCreds, err := credStore.Load(ctx, profileName)

	// Default to empty password if credentials don't exist (passwordless profile)
	existingPassword := ""
	if err == nil {
		existingPassword = existingCreds.Password
	}

	// Launch form pre-filled with existing data
	profileData, err := form.NewProfileFormWithDefaults(
		existingProfile.Driver,
		existingProfile.Name,
		existingProfile.Database,
		existingProfile.Host,
		existingProfile.Port,
		existingProfile.Username,
		existingProfile.SSLMode,
		existingPassword, // May be empty for passwordless profiles
	)
	if err != nil {
		return fmt.Errorf("edit cancelled or error: %w", err)
	}

	// Check if profile name changed
	nameChanged := profileData.Name != profileName

	if nameChanged {
		// Remove old profile and credentials
		_ = cfg.RemoveProfile(profileName)
		_ = credStore.Delete(ctx, profileName)

		fmt.Printf("ℹ️  Profile renamed from '%s' to '%s'\n", profileName, profileData.Name)
	}

	// Update profile
	updatedProfile := &config.Profile{
		Driver:   profileData.Driver,
		Name:     profileData.Name,
		Host:     profileData.Host,
		Port:     profileData.Port,
		Database: profileData.Database,
		Username: profileData.Username,
		SSLMode:  profileData.SSLMode,
		ReadOnly: true,
		Disabled: existingProfile.Disabled,
	}

	// Save updated credentials (only if password provided)
	if profileData.Password != "" {
		if err := credStore.Save(ctx, profileData.Name, credentials.Credentials{
			Username: profileData.Username,
			Password: profileData.Password,
		}); err != nil {
			return fmt.Errorf("failed to save credentials: %w", err)
		}
	} else {
		// If password is now empty, delete any existing credentials
		_ = credStore.Delete(ctx, profileData.Name)
	}

	// Add/update profile in config
	cfg.AddProfile(updatedProfile)

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("\n✓ Profile '%s' updated successfully\n", profileData.Name)
	if profileData.Password != "" {
		fmt.Printf("✓ Credentials updated in %s\n", credStore.Type())
	} else {
		fmt.Printf("✓ Using passwordless authentication\n")
	}

	// Prompt to test connection
	var runTest bool
	if err := huh.NewConfirm().
		Title("Test connection?").
		Value(&runTest).
		WithTheme(form.CustomTheme()).
		Run(); err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	if runTest {
		result := runConnectionTest(ctx, profileData)
		printConnectionTestResult(result)
	}

	return nil
}

// newConfigManageCmd creates the 'config manage' command
func newConfigManageCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "manage",
		Short: "Interactive profile management menu",
		Long: `Interactively manage database connection profiles.

Provides a menu to enable/disable and delete profiles.
Requires --human flag for interactive mode.

Examples:
  dbridge --human config manage
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)

			if !formatter.HumanMode {
				return formatError(cmd, "invalid_mode",
					"Interactive mode requires --human flag",
					nil)
			}

			return runManageMenu(cmd)
		},
	}
}

// runManageMenu is the main loop for the interactive manage menu
func runManageMenu(cmd *cobra.Command) error {
	for {
		// Reload config each iteration to reflect changes
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		if len(cfg.Profiles) == 0 {
			fmt.Println("No profiles configured.")
			fmt.Println("\nAdd a profile with: dbridge config add <name>")
			return nil
		}

		// Step 1: Select a profile
		profileOptions := buildProfileOptions(cfg)
		profileOptions = append(profileOptions, huh.NewOption("← Exit", "__exit"))

		var selectedProfile string
		err = huh.NewSelect[string]().
			Title("Select a profile to manage").
			Options(profileOptions...).
			Value(&selectedProfile).
			WithTheme(form.CustomTheme()).
			Run()
		if err != nil {
			return nil // User cancelled
		}

		if selectedProfile == "__exit" {
			return nil // Exit
		}

		// Step 2: Select an action
		profile, err := cfg.GetProfile(selectedProfile)
		if err != nil {
			return fmt.Errorf("profile not found: %w", err)
		}

		action, err := selectProfileAction(profile)
		if err != nil || action == "back" {
			continue // Back to profile list
		}

		// Step 3: Execute action
		switch action {
		case "edit":
			if err := runEditFlow(cfg, selectedProfile); err != nil {
				return err
			}
		case "toggle":
			if err := toggleProfile(cfg, selectedProfile); err != nil {
				return err
			}
			if profile.Disabled {
				fmt.Printf("✓ Profile '%s' disabled\n\n", selectedProfile)
			} else {
				fmt.Printf("✓ Profile '%s' enabled\n\n", selectedProfile)
			}
		case "delete":
			ctx := context.Background()
			deleted, err := deleteProfileConfirm(ctx, cfg, selectedProfile)
			if err != nil {
				return err
			}
			if deleted {
				fmt.Printf("✓ Profile '%s' deleted\n\n", selectedProfile)
			}
		}
	}
}

// buildProfileOptions builds huh select options for all profiles
func buildProfileOptions(cfg *config.Config) []huh.Option[string] {
	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	options := make([]huh.Option[string], 0, len(names))
	for _, name := range names {
		profile := cfg.Profiles[name]
		icon := "✓"
		status := "enabled"
		if profile.Disabled {
			icon = "✗"
			status = "disabled"
		}
		driverTag := profile.Driver
		if driverTag == "" {
			driverTag = "postgres"
		}
		label := fmt.Sprintf("%s %s - %s:%d/%s [%s] (%s)",
			icon, name, profile.Host, profile.Port, profile.Database, driverTag, status)
		options = append(options, huh.NewOption(label, name))
	}
	return options
}

// selectProfileAction shows the action menu for a profile
func selectProfileAction(profile *config.Profile) (string, error) {
	toggleLabel := "Disable profile"
	if profile.Disabled {
		toggleLabel = "Enable profile"
	}

	var action string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("Action for '%s'", profile.Name)).
		Options(
			huh.NewOption("← Back to profile list", "back"),
			huh.NewOption("Edit profile", "edit"),
			huh.NewOption(toggleLabel, "toggle"),
			huh.NewOption("Delete profile", "delete"),
		).
		Value(&action).
		WithTheme(form.CustomTheme()).
		Run()
	if err != nil {
		return "back", err
	}
	return action, nil
}

// toggleProfile flips the Disabled state and saves
func toggleProfile(cfg *config.Config, name string) error {
	profile := cfg.Profiles[name]
	profile.Disabled = !profile.Disabled
	return cfg.Save()
}

// deleteProfileConfirm confirms and deletes a profile and its credentials
func deleteProfileConfirm(ctx context.Context, cfg *config.Config, name string) (bool, error) {
	var confirm bool
	err := huh.NewConfirm().
		Title(fmt.Sprintf("Delete profile '%s'? This cannot be undone.", name)).
		Affirmative("Delete").
		Negative("Cancel").
		Value(&confirm).
		WithTheme(form.CustomTheme()).
		Run()
	if err != nil || !confirm {
		return false, nil
	}

	// Delete credentials
	credStore, err := credentials.NewStore("dbridge")
	if err == nil {
		_ = credStore.Delete(ctx, name)
	}

	// Remove profile
	if err := cfg.RemoveProfile(name); err != nil {
		return false, fmt.Errorf("failed to remove profile: %w", err)
	}

	return true, cfg.Save()
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

// runConnectionTest tests a database connection using the given profile data
func runConnectionTest(ctx context.Context, data *form.ProfileData) *connectionTestResult {
	connConfig := &dbpkg.ConnectionConfig{
		Driver:   data.Driver,
		Host:     data.Host,
		Port:     data.Port,
		Database: data.Database,
		Username: data.Username,
		Password: data.Password,
		SSLMode:  data.SSLMode,
		ReadOnly: true,
	}
	conn, err := dbpkg.NewConnection(ctx, connConfig)
	if err != nil {
		return &connectionTestResult{Success: false, Error: err.Error()}
	}
	conn.Close(ctx)
	return &connectionTestResult{Success: true}
}

// printConnectionTestResult prints the connection test result for human-readable output
func printConnectionTestResult(result *connectionTestResult) {
	if result.Success {
		fmt.Println("✓ Connection test successful")
	} else {
		fmt.Printf("⚠ Connection test failed: %s\n", result.Error)
	}
}
