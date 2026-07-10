package writecli

import (
	"os"
	"strings"
	"testing"

	"github.com/thalesfp/dbridge/internal/config"
)

func TestWriteAuditEventStoresDigestOnly(t *testing.T) {
	path := t.TempDir() + "/audit.log"
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Settings.AuditLog = true
	cfg.Settings.AuditLogPath = path
	batch := "ALTER LOGIN admin WITH PASSWORD = 'do-not-log-this'"

	if err := writeAuditEvent(cfg, "production", batch); err != nil {
		t.Fatalf("writeAuditEvent() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), batch) || strings.Contains(string(data), "do-not-log-this") {
		t.Fatalf("audit log contains SQL text: %s", data)
	}
	if !strings.Contains(string(data), `"connection":"production"`) {
		t.Fatalf("audit log does not identify connection: %s", data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("audit log permissions = %04o, want 0600", info.Mode().Perm())
	}
}

func TestWriteAuditEventDisabled(t *testing.T) {
	path := t.TempDir() + "/audit.log"
	cfg := config.DefaultConfig()
	cfg.Settings.AuditLogPath = path

	if err := writeAuditEvent(cfg, "production", "DROP TABLE users"); err != nil {
		t.Fatalf("writeAuditEvent() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("audit file exists or returned unexpected error: %v", err)
	}
}
