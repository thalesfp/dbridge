package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thalesgelinger/pgmcp/internal/config"
	"github.com/thalesgelinger/pgmcp/internal/credentials"
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

	return cmd
}

// newConfigAddCmd creates the 'config add' command
func newConfigAddCmd() *cobra.Command {
	var (
		host     string
		port     int
		database string
		username string
		sslMode  string
		poolSize int
		readOnly bool
	)

	cmd := &cobra.Command{
		Use:   "add [profile-name]",
		Short: "Add a new connection profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Get password from user
			fmt.Printf("Enter password for %s@%s: ", username, host)
			var password string
			fmt.Scanln(&password)

			// Create profile
			profile := &config.Profile{
				Name:     profileName,
				Host:     host,
				Port:     port,
				Database: database,
				Username: username,
				SSLMode:  sslMode,
				PoolSize: poolSize,
				ReadOnly: readOnly,
			}

			// Save credentials to keychain
			credStore, err := credentials.NewStore("pgmcp")
			if err != nil {
				return fmt.Errorf("failed to open credential store: %w", err)
			}

			ctx := context.Background()
			if err := credStore.Save(ctx, profileName, credentials.Credentials{
				Username: username,
				Password: password,
			}); err != nil {
				return fmt.Errorf("failed to save credentials: %w", err)
			}

			// Add profile to config
			cfg.AddProfile(profile)

			// Save config
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("✓ Profile '%s' added successfully\n", profileName)
			fmt.Printf("✓ Credentials stored in %s\n", credStore.Type())

			if cfg.Settings.DefaultProfile == profileName {
				fmt.Printf("✓ Set as default profile\n")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&host, "host", "localhost", "Database host")
	cmd.Flags().IntVar(&port, "port", 5432, "Database port")
	cmd.Flags().StringVar(&database, "database", "", "Database name")
	cmd.Flags().StringVar(&username, "username", "", "Database username")
	cmd.Flags().StringVar(&sslMode, "ssl-mode", "prefer", "SSL mode (disable, require, prefer)")
	cmd.Flags().IntVar(&poolSize, "pool-size", 5, "Connection pool size")
	cmd.Flags().BoolVar(&readOnly, "readonly", true, "Read-only mode (v1.0 is read-only only)")

	cmd.MarkFlagRequired("database")
	cmd.MarkFlagRequired("username")

	return cmd
}

// newConfigListCmd creates the 'config list' command
func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all connection profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			if len(cfg.Profiles) == 0 {
				fmt.Println("No profiles configured")
				fmt.Println("\nAdd a profile with: pgmcp config add <name>")
				return nil
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
			profileName := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			profile, err := cfg.GetProfile(profileName)
			if err != nil {
				return err
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
			profileName := args[0]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Remove credentials from keychain
			credStore, err := credentials.NewStore("pgmcp")
			if err != nil {
				return fmt.Errorf("failed to open credential store: %w", err)
			}

			ctx := context.Background()
			if err := credStore.Delete(ctx, profileName); err != nil {
				fmt.Printf("Warning: failed to delete credentials: %v\n", err)
			}

			// Remove profile from config
			if err := cfg.RemoveProfile(profileName); err != nil {
				return err
			}

			// Save config
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Printf("✓ Profile '%s' removed\n", profileName)

			return nil
		},
	}
}
