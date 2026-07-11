package writedb

import (
	"context"
	"fmt"
)

// SupportsDriver reports whether writable batch execution is implemented.
func SupportsDriver(driver string) bool {
	switch driver {
	case "postgres", "", "mysql", "mssql":
		return true
	default:
		return false
	}
}

// Connect opens a writable connection for a supported SQL driver.
func Connect(ctx context.Context, config *Config) (Connection, error) {
	switch config.Driver {
	case "postgres", "":
		return connectPostgres(ctx, config)
	case "mysql":
		return connectMySQL(ctx, config)
	case "mssql":
		return connectMSSQL(ctx, config)
	default:
		return nil, fmt.Errorf("unsupported write driver: %s", config.Driver)
	}
}
