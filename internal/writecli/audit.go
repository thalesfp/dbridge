package writecli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/thalesfp/dbridge/internal/config"
)

type auditEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Connection string    `json:"connection"`
	SQLDigest  string    `json:"sql_digest"`
}

func writeAuditEvent(cfg *config.Config, connection, sql string) error {
	if !cfg.Settings.AuditLog {
		return nil
	}

	path := cfg.Settings.AuditLogPath
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to resolve audit log path: %w", err)
		}
		path = filepath.Join(home, ".config", "dbridge", "write-audit.log")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create audit log directory: %w", err)
	}

	digest := sha256.Sum256([]byte(sql))
	event := auditEvent{
		Timestamp:  time.Now().UTC(),
		Connection: connection,
		SQLDigest:  hex.EncodeToString(digest[:]),
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to encode audit event: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open audit log: %w", err)
	}
	if err := file.Chmod(0600); err != nil {
		file.Close()

		return fmt.Errorf("failed to secure audit log: %w", err)
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		file.Close()

		return fmt.Errorf("failed to write audit log: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close audit log: %w", err)
	}

	return nil
}
