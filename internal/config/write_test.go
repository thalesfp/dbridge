package config

import (
	"testing"
)

func TestGetWriteConnection(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AddConnection(&Connection{
		Name:     "production-read",
		Driver:   "postgres",
		Host:     "db.example.com",
		Database: "app",
		Username: "reader",
	})
	cfg.AddWriteConnection("production", &WriteConnection{
		Connection: "production-read",
		Username:   "writer",
	})

	writeConn, endpoint, err := cfg.GetWriteConnection("production")
	if err != nil {
		t.Fatalf("GetWriteConnection() error = %v", err)
	}
	if writeConn.Username != "writer" {
		t.Fatalf("write username = %q, want writer", writeConn.Username)
	}
	if endpoint.Host != "db.example.com" {
		t.Fatalf("endpoint host = %q, want db.example.com", endpoint.Host)
	}
}

func TestGetWriteConnectionFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{name: "missing", cfg: DefaultConfig()},
		{name: "missing reference", cfg: configWithWriteConnection(&WriteConnection{Username: "writer"})},
		{name: "missing username", cfg: configWithWriteConnection(&WriteConnection{Connection: "read"})},
		{name: "unknown reference", cfg: configWithWriteConnection(&WriteConnection{Connection: "unknown", Username: "writer"})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := tt.cfg.GetWriteConnection("write"); err == nil {
				t.Fatal("GetWriteConnection() error = nil")
			}
		})
	}
}

func configWithWriteConnection(writeConn *WriteConnection) *Config {
	cfg := DefaultConfig()
	cfg.AddConnection(&Connection{Name: "read", Host: "localhost", Database: "app", Username: "reader"})
	cfg.AddWriteConnection("write", writeConn)

	return cfg
}

func TestWriteConnectionsSaveAndLoad(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg := DefaultConfig()
	cfg.AddConnection(&Connection{Name: "read", Host: "localhost", Database: "app", Username: "reader"})
	cfg.AddWriteConnection("write", &WriteConnection{Connection: "read", Username: "writer"})
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	writeConn, endpoint, err := loaded.GetWriteConnection("write")
	if err != nil {
		t.Fatalf("GetWriteConnection() error = %v", err)
	}
	if writeConn.Username != "writer" || endpoint.Username != "reader" {
		t.Fatalf("credentials identities were not kept separate: write=%q read=%q", writeConn.Username, endpoint.Username)
	}
}
