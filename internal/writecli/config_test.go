package writecli

import (
	"testing"

	"github.com/thalesfp/dbridge/internal/config"
)

func TestValidateWriteEndpoint(t *testing.T) {
	for _, driver := range []string{"postgres", "mysql", "mssql"} {
		if err := validateWriteEndpoint(&config.Connection{Driver: driver}); err != nil {
			t.Fatalf("validateWriteEndpoint(%q) error = %v", driver, err)
		}
	}
}

func TestValidateWriteEndpointRejectsUnsupportedDriver(t *testing.T) {
	for _, driver := range []string{"mongodb", "unknown"} {
		if err := validateWriteEndpoint(&config.Connection{Driver: driver}); err == nil {
			t.Fatalf("validateWriteEndpoint(%q) error = nil", driver)
		}
	}
}
