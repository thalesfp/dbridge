package writecli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/thalesfp/dbridge/internal/cli"
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

			conn, err := prepareExecution(cmd.Context(), name, batch)
			if err != nil {
				return err
			}
			defer conn.Close()

			result, executeErr := conn.Execute(cmd.Context(), batch)
			data, failed, err := renderBatchResult(result, executeErr)
			if err != nil {
				return err
			}
			fmt.Println(string(data))

			if failed {
				return &cli.HandledError{Message: executeErr.Error()}
			}

			return nil
		},
	}
}
