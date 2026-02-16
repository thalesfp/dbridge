package database

import (
	"context"
	"fmt"
	"sort"
)

// Driver creates connections for a specific database engine.
type Driver interface {
	Connect(ctx context.Context, config *ConnectionConfig) (Connection, error)
}

var drivers = map[string]Driver{}

// RegisterDriver registers a named driver.
func RegisterDriver(name string, d Driver) {
	drivers[name] = d
}

// DriverNames returns registered driver names sorted alphabetically.
func DriverNames() []string {
	names := make([]string, 0, len(drivers))
	for n := range drivers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// NewConnection creates a new database connection using the registered driver.
func NewConnection(ctx context.Context, config *ConnectionConfig) (Connection, error) {
	name := config.Driver
	if name == "" {
		return nil, fmt.Errorf("driver is required")
	}
	d, ok := drivers[name]
	if !ok {
		return nil, fmt.Errorf("unsupported driver: %s", name)
	}
	return d.Connect(ctx, config)
}
