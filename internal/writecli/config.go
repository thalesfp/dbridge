package writecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/config"
	"github.com/thalesfp/dbridge/internal/credentials"
	"github.com/thalesfp/dbridge/internal/writedb"
	"golang.org/x/term"
)

// NewConfigCmd manages the write-only namespace and credential store.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage writable database connections",
	}

	cmd.AddCommand(newConfigAddCmd())
	cmd.AddCommand(newConfigListCmd())
	cmd.AddCommand(newConfigRemoveCmd())

	return cmd
}

func newConfigAddCmd() *cobra.Command {
	var connection string
	var username string
	var password string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a writable identity for an existing read connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}
			if _, ok := cfg.WriteConnections[name]; ok {
				return fmt.Errorf("write connection '%s' already exists", name)
			}
			endpoint, err := cfg.GetConnection(connection)
			if err != nil {
				return err
			}
			if err := validateWriteEndpoint(endpoint); err != nil {
				return err
			}

			resolvedPassword := password
			if !cmd.Flags().Changed("password") {
				resolvedPassword, err = promptPassword()
				if err != nil {
					return err
				}
			}

			store, err := credentials.NewStore(credentialService)
			if err != nil {
				return fmt.Errorf("failed to open write credential store: %w", err)
			}
			if err := store.Save(cmd.Context(), name, credentials.Credentials{Username: username, Password: resolvedPassword}); err != nil {
				return fmt.Errorf("failed to save write credentials: %w", err)
			}

			cfg.AddWriteConnection(name, &config.WriteConnection{Connection: connection, Username: username})
			if err := cfg.Save(); err != nil {
				deleteErr := store.Delete(cmd.Context(), name)
				if deleteErr != nil && !errors.Is(deleteErr, credentials.ErrNotFound) {
					return fmt.Errorf("failed to save config: %v; failed to roll back credentials: %w", err, deleteErr)
				}

				return err
			}

			fmt.Printf("write connection '%s' added\n", name)

			return nil
		},
	}

	cmd.Flags().StringVar(&connection, "connection", "", "Existing read connection to reuse")
	cmd.Flags().StringVar(&username, "username", "", "Writable database username")
	cmd.Flags().StringVar(&password, "password", "", "Writable database password (prompted when omitted)")
	_ = cmd.MarkFlagRequired("connection")
	_ = cmd.MarkFlagRequired("username")

	return cmd
}

func validateWriteEndpoint(endpoint *config.Connection) error {
	if !writedb.SupportsDriver(endpoint.Driver) {
		return fmt.Errorf(
			"write connections support postgres, mysql, and mssql; referenced connection uses %s",
			endpoint.Driver,
		)
	}

	return nil
}

func promptPassword() (string, error) {
	fmt.Fprint(os.Stderr, "Write database password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	return string(password), nil
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List writable database connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			names := make([]string, 0, len(cfg.WriteConnections))
			for name := range cfg.WriteConnections {
				names = append(names, name)
			}
			sort.Strings(names)

			data, err := json.Marshal(names)
			if err != nil {
				return err
			}
			fmt.Println(string(data))

			return nil
		},
	}
}

func newConfigRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a writable database connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeWriteConnection(cmd.Context(), args[0])
		},
	}
}

func removeWriteConnection(ctx context.Context, name string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := cfg.RemoveWriteConnection(name); err != nil {
		return err
	}
	if err := cfg.Save(); err != nil {
		return err
	}

	store, err := credentials.NewStore(credentialService)
	if err != nil {
		return fmt.Errorf("write connection removed, but credential store could not be opened: %w", err)
	}
	if err := store.Delete(ctx, name); err != nil && !errors.Is(err, credentials.ErrNotFound) {
		return fmt.Errorf("write connection removed, but credentials could not be deleted: %w", err)
	}

	fmt.Printf("write connection '%s' removed\n", name)

	return nil
}
