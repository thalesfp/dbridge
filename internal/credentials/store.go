package credentials

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/99designs/keyring"
)

// Credentials holds database authentication information
type Credentials struct {
	Username string
	Password string
}

// Store defines the interface for credential storage
type Store interface {
	// Save stores credentials for a connection
	Save(ctx context.Context, connection string, creds Credentials) error

	// Load retrieves credentials for a connection
	Load(ctx context.Context, connection string) (Credentials, error)

	// Delete removes credentials for a connection
	Delete(ctx context.Context, connection string) error

	// List returns all connection names with stored credentials
	List(ctx context.Context) ([]string, error)

	// Available returns true if the store is usable on this platform
	Available() bool

	// Type returns the store type (keyring, encrypted-file, etc.)
	Type() string
}

// KeyringStore implements Store using OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)
type KeyringStore struct {
	keyring keyring.Keyring
	service string
}

// backendTypeToName maps each OS-native keychain backend to its display name.
var backendTypeToName = map[keyring.BackendType]string{
	keyring.KeychainBackend:      "keychain",
	keyring.WinCredBackend:       "wincred",
	keyring.SecretServiceBackend: "secret-service",
}

// allowedBackends returns the single OS-native keychain backend for the current platform.
// If the OS keychain is unavailable, keyring.Open returns an error instead of falling back.
func allowedBackends() []keyring.BackendType {
	switch runtime.GOOS {
	case "darwin":
		return []keyring.BackendType{keyring.KeychainBackend}
	case "windows":
		return []keyring.BackendType{keyring.WinCredBackend}
	default:
		return []keyring.BackendType{keyring.SecretServiceBackend}
	}
}

// NewKeyringStore creates a new keyring-based credential store
func NewKeyringStore(serviceName string) (*KeyringStore, error) {
	kr, err := keyring.Open(keyring.Config{
		ServiceName:              serviceName,
		KeychainName:             serviceName,
		KeychainTrustApplication: true,
		AllowedBackends:          allowedBackends(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open keyring: %w", err)
	}

	return &KeyringStore{
		keyring: kr,
		service: serviceName,
	}, nil
}

// Save stores credentials in the keychain
func (s *KeyringStore) Save(ctx context.Context, connection string, creds Credentials) error {
	// Store username and password as a single item with JSON
	data := fmt.Sprintf("%s:%s", creds.Username, creds.Password)

	err := s.keyring.Set(keyring.Item{
		Key:   s.connectionKey(connection),
		Data:  []byte(data),
		Label: fmt.Sprintf("dbridge-%s", connection),
	})
	if err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	return nil
}

// Load retrieves credentials from the keychain
func (s *KeyringStore) Load(ctx context.Context, connection string) (Credentials, error) {
	item, err := s.keyring.Get(s.connectionKey(connection))
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to load credentials: %w", err)
	}

	// Parse username:password format
	data := string(item.Data)

	// Split by first colon
	parts := strings.SplitN(data, ":", 2)
	if len(parts) == 2 {
		return Credentials{
			Username: parts[0],
			Password: parts[1],
		}, nil
	}

	// Fallback: treat whole data as password (for backward compatibility)
	return Credentials{
		Password: data,
	}, nil
}

// Delete removes credentials from the keychain
func (s *KeyringStore) Delete(ctx context.Context, connection string) error {
	err := s.keyring.Remove(s.connectionKey(connection))
	if err != nil {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}
	return nil
}

// List returns all stored connection names
func (s *KeyringStore) List(ctx context.Context) ([]string, error) {
	keys, err := s.keyring.Keys()
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	var connections []string
	prefix := s.service + "-"
	for _, key := range keys {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			name := key[len(prefix):]
			connections = append(connections, name)
		}
	}

	return connections, nil
}

// Available returns true if keyring is available
func (s *KeyringStore) Available() bool {
	return s.keyring != nil
}

// Type returns the OS-native keychain backend name
func (s *KeyringStore) Type() string {
	return backendTypeToName[allowedBackends()[0]]
}

// connectionKey generates the keychain key for a connection
func (s *KeyringStore) connectionKey(connection string) string {
	return fmt.Sprintf("%s-%s", s.service, connection)
}

// NewStore creates the appropriate credential store for the platform
func NewStore(serviceName string) (Store, error) {
	// Try keyring first
	store, err := NewKeyringStore(serviceName)
	if err == nil && store.Available() {
		return store, nil
	}

	// TODO: Implement encrypted file fallback
	return nil, fmt.Errorf("no credential store available: keyring error: %w", err)
}
