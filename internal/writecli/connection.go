package writecli

import (
	"context"
	"errors"
	"fmt"

	"github.com/thalesfp/dbridge/internal/config"
	"github.com/thalesfp/dbridge/internal/credentials"
	"github.com/thalesfp/dbridge/internal/writedb"
)

const credentialService = "dbridge-write"

func getConnection(ctx context.Context, cfg *config.Config, name string) (writedb.Connection, error) {
	writeConn, endpoint, err := cfg.GetWriteConnection(name)
	if err != nil {
		return nil, err
	}
	if writeConn.Disabled {
		return nil, fmt.Errorf("write connection '%s' is disabled", name)
	}
	if endpoint.Disabled {
		return nil, fmt.Errorf("referenced connection '%s' is disabled", writeConn.Connection)
	}

	store, err := credentials.NewStore(credentialService)
	if err != nil {
		return nil, fmt.Errorf("failed to open write credential store: %w", err)
	}
	creds, err := store.Load(ctx, name)
	if err != nil {
		if errors.Is(err, credentials.ErrNotFound) {
			return nil, fmt.Errorf("write credentials for '%s' not found", name)
		}

		return nil, fmt.Errorf("failed to load write credentials for '%s': %w", name, err)
	}

	conn, err := writedb.Connect(ctx, &writedb.Config{
		Driver:   endpoint.Driver,
		Host:     endpoint.Host,
		Port:     endpoint.Port,
		Database: endpoint.Database,
		Username: writeConn.Username,
		Password: creds.Password,
		SSLMode:  endpoint.SSLMode,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return conn, nil
}

func prepareExecution(ctx context.Context, name, batch string) (writedb.Connection, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if err := writeAuditEvent(cfg, name, batch); err != nil {
		return nil, err
	}

	return getConnection(ctx, cfg, name)
}
