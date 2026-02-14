package cli

import (
	"context"
	"fmt"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli/form"
	"github.com/thalesfp/dbridge/internal/cli/output"
	"github.com/thalesfp/dbridge/internal/config"
	"github.com/thalesfp/dbridge/internal/credentials"
	"golang.org/x/term"
)

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
	cmd.AddCommand(newConfigEditCmd())

	return cmd
}

// newConfigAddCmd creates the 'config add' command
func newConfigAddCmd() *cobra.Command {
	var (
		host     string
		port     int
		database string
		username string
		password string
		sslMode  string
		poolSize int
		readOnly bool
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
			flagMode := cmd.Flags().Changed("database") || cmd.Flags().Changed("username")

			// JSON mode requires flag mode (no interactive TUI)
			if formatter.JSONMode && !flagMode {
				return formatError(cmd, "invalid_mode",
					"--json mode requires using flags (--database, --username, etc.) instead of interactive form",
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
				if database == "" {
					return fmt.Errorf("--database is required")
				}
				if username == "" {
					return fmt.Errorf("--username is required")
				}

				// Get password if not provided via flag
				if !cmd.Flags().Changed("password") {
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

				// Apply defaults
				if port == 0 {
					port = 5432
				}
				if poolSize == 0 {
					poolSize = 5
				}
				if sslMode == "" {
					sslMode = "prefer"
				}

				profileData = &form.ProfileData{
					Name:     profileName,
					Host:     host,
					Port:     port,
					Database: database,
					Username: username,
					SSLMode:  sslMode,
					PoolSize: poolSize,
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
				Name:     profileData.Name,
				Host:     profileData.Host,
				Port:     profileData.Port,
				Database: profileData.Database,
				Username: profileData.Username,
				SSLMode:  profileData.SSLMode,
				PoolSize: profileData.PoolSize,
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

			// Output success message
			if formatter.JSONMode {
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
						"name":      profileData.Name,
						"host":      profileData.Host,
						"port":      profileData.Port,
						"database":  profileData.Database,
						"username":  profileData.Username,
						"ssl_mode":  profileData.SSLMode,
						"pool_size": profileData.PoolSize,
						"read_only": true,
					},
					"credentials_stored": profileData.Password != "",
					"credential_store":   credStore,
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
			}

			return nil
		},
	}

	// Add flags for non-interactive mode
	cmd.Flags().StringVar(&host, "host", "localhost", "Database host")
	cmd.Flags().IntVar(&port, "port", 5432, "Database port")
	cmd.Flags().StringVar(&database, "database", "", "Database name (required for flag mode)")
	cmd.Flags().StringVar(&username, "username", "", "Database username (required for flag mode)")
	cmd.Flags().StringVar(&password, "password", "", "Database password (optional, will prompt if not provided)")
	cmd.Flags().StringVar(&sslMode, "ssl-mode", "prefer", "SSL mode (disable, require, prefer)")
	cmd.Flags().IntVar(&poolSize, "pool-size", 5, "Connection pool size")
	cmd.Flags().BoolVar(&readOnly, "readonly", true, "Read-only mode")

	return cmd
}

// readPassword reads a password from stdin without echoing
func readPassword() ([]byte, error) {
	return term.ReadPassword(int(syscall.Stdin))
}

// getFormatter creates a formatter instance based on the JSON output flag
func getFormatter(cmd *cobra.Command) *output.Formatter {
	jsonMode := false
	if val := cmd.Context().Value("json_output"); val != nil {
		jsonMode = val.(bool)
	}
	return output.NewFormatter(jsonMode)
}

// formatError outputs a structured error in JSON mode
func formatError(cmd *cobra.Command, code, message string, details interface{}) error {
	formatter := getFormatter(cmd)
	if formatter.JSONMode {
		formatter.Error(code, message, details)
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
				if formatter.JSONMode {
					return formatter.Success("config_list", map[string]interface{}{
						"profiles":        []interface{}{},
						"default_profile": "",
						"total_count":     0,
					}, "No profiles configured")
				}

				fmt.Println("No profiles configured")
				fmt.Println("\nAdd a profile with: dbridge config add <name>")
				return nil
			}

			if formatter.JSONMode {
				profiles := make([]map[string]interface{}, 0, len(cfg.Profiles))
				for name, profile := range cfg.Profiles {
					profiles = append(profiles, map[string]interface{}{
						"name":       name,
						"host":       profile.Host,
						"port":       profile.Port,
						"database":   profile.Database,
						"username":   profile.Username,
						"ssl_mode":   profile.SSLMode,
						"pool_size":  profile.PoolSize,
						"read_only":  profile.ReadOnly,
						"is_default": name == cfg.Settings.DefaultProfile,
					})
				}

				return formatter.Success("config_list", map[string]interface{}{
					"profiles":        profiles,
					"default_profile": cfg.Settings.DefaultProfile,
					"total_count":     len(cfg.Profiles),
				}, fmt.Sprintf("Found %d profile(s)", len(cfg.Profiles)))
			}

			fmt.Println("Profiles:")
			for name, profile := range cfg.Profiles {
				marker := ""
				if name == cfg.Settings.DefaultProfile {
					marker = " (default)"
				}

				readOnlyMarker := ""
				if profile.ReadOnly {
					readOnlyMarker = " [read-only]"
				} else {
					readOnlyMarker = " [read-write]"
				}

				fmt.Printf("  %s%s - %s:%d/%s%s\n",
					name,
					marker,
					profile.Host,
					profile.Port,
					profile.Database,
					readOnlyMarker,
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

			if formatter.JSONMode {
				data := map[string]interface{}{
					"profile": map[string]interface{}{
						"name":       profile.Name,
						"host":       profile.Host,
						"port":       profile.Port,
						"database":   profile.Database,
						"username":   profile.Username,
						"ssl_mode":   profile.SSLMode,
						"pool_size":  profile.PoolSize,
						"read_only":  profile.ReadOnly,
						"is_default": profile.Name == cfg.Settings.DefaultProfile,
					},
					"has_credentials": true,
				}

				return formatter.Success("config_show", data,
					fmt.Sprintf("Profile '%s' details", profileName))
			}

			fmt.Printf("Profile: %s\n", profile.Name)
			fmt.Printf("Host: %s\n", profile.Host)
			fmt.Printf("Port: %d\n", profile.Port)
			fmt.Printf("Database: %s\n", profile.Database)
			fmt.Printf("Username: %s\n", profile.Username)
			fmt.Printf("SSL Mode: %s\n", profile.SSLMode)
			fmt.Printf("Pool Size: %d\n", profile.PoolSize)
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
			if credDeleteErr != nil && !formatter.JSONMode {
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

			if formatter.JSONMode {
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

			// JSON mode not supported for clone (requires interactive TUI)
			if formatter.JSONMode {
				return formatError(cmd, "invalid_mode",
					"--json mode is not supported for config clone (requires interactive form)",
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
				newProfileName,
				sourceProfile.Database,
				sourceProfile.Host,
				sourceProfile.Port,
				sourceProfile.Username,
				sourceProfile.SSLMode,
				sourceProfile.PoolSize,
				sourcePassword, // Pre-fill password from source (may be empty)
			)
			if err != nil {
				return fmt.Errorf("clone cancelled or error: %w", err)
			}

			// Create new profile
			newProfile := &config.Profile{
				Name:     profileData.Name,
				Host:     profileData.Host,
				Port:     profileData.Port,
				Database: profileData.Database,
				Username: profileData.Username,
				SSLMode:  profileData.SSLMode,
				PoolSize: profileData.PoolSize,
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

// newConfigEditCmd creates the 'config edit' command
func newConfigEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit <profile-name>",
		Short: "Edit an existing connection profile",
		Long: `Edit an existing database connection profile.

Opens an interactive form pre-filled with the current profile settings.
You can modify any field and save the changes.

Note: Changing the profile name will create a new profile and remove the old one.

Examples:
  dbridge config edit production
  dbridge config edit local
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			formatter := getFormatter(cmd)
			profileName := args[0]

			// JSON mode not supported for edit (requires interactive TUI)
			if formatter.JSONMode {
				return formatError(cmd, "invalid_mode",
					"--json mode is not supported for config edit (requires interactive form)",
					nil)
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

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
				existingProfile.Name,
				existingProfile.Database,
				existingProfile.Host,
				existingProfile.Port,
				existingProfile.Username,
				existingProfile.SSLMode,
				existingProfile.PoolSize,
				existingPassword, // May be empty for passwordless profiles
			)
			if err != nil {
				return fmt.Errorf("edit cancelled or error: %w", err)
			}

			// Check if profile name changed
			nameChanged := profileData.Name != profileName

			if nameChanged {
				// Remove old profile and credentials
				cfg.RemoveProfile(profileName)
				credStore.Delete(ctx, profileName)

				fmt.Printf("ℹ️  Profile renamed from '%s' to '%s'\n", profileName, profileData.Name)
			}

			// Update profile
			updatedProfile := &config.Profile{
				Name:     profileData.Name,
				Host:     profileData.Host,
				Port:     profileData.Port,
				Database: profileData.Database,
				Username: profileData.Username,
				SSLMode:  profileData.SSLMode,
				PoolSize: profileData.PoolSize,
				ReadOnly: true,
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
				credStore.Delete(ctx, profileData.Name)
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

			return nil
		},
	}
}
