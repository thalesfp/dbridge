package writecli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/config"
)

// NewExecuteCmd creates the direct batch execution command.
func NewExecuteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "execute <connection> <sql>",
		Short: "Execute an arbitrary SQL batch",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			batch := args[1]

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("failed to load audit config: %w", err)
			}
			if err := writeAuditEvent(cfg, name, batch); err != nil {
				return err
			}

			conn, err := getConnection(cmd.Context(), name)
			if err != nil {
				return err
			}
			defer conn.Close()

			result, executeErr := conn.Execute(cmd.Context(), batch)
			if result == nil {
				if executeErr == nil {
					return fmt.Errorf("batch returned no result")
				}

				return executeErr
			}
			if executeErr != nil {
				result.Error = executeErr.Error()
			}

			data, err := json.Marshal(result)
			if err != nil {
				return err
			}
			fmt.Println(string(data))

			if executeErr != nil {
				return fmt.Errorf("batch execution failed")
			}

			return nil
		},
	}
}
