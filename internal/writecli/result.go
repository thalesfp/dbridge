package writecli

import (
	"encoding/json"
	"fmt"

	"github.com/thalesfp/dbridge/internal/writedb"
)

func renderBatchResult(result *writedb.BatchResult, executeErr error) ([]byte, bool, error) {
	if result == nil {
		if executeErr == nil {
			return nil, false, fmt.Errorf("batch returned no result")
		}

		return nil, true, executeErr
	}
	if executeErr != nil {
		result.Error = executeErr.Error()
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, executeErr != nil, err
	}

	return data, executeErr != nil, nil
}
