package form

import (
	"fmt"
	"regexp"
	"strconv"
)

// ProfileData holds the form input values
type ProfileData struct {
	Driver   string
	Name     string
	Database string
	Host     string
	Port     int
	Username string
	SSLMode  string
	Password string
}

// Validation functions

var profileNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func validateProfileName(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("profile name cannot be empty")
	}
	if !profileNameRe.MatchString(s) {
		return fmt.Errorf("only alphanumeric characters, dashes, and underscores allowed")
	}
	return nil
}

func validateNotEmpty(fieldName string) func(string) error {
	return func(s string) error {
		if len(s) == 0 {
			return fmt.Errorf("%s cannot be empty", fieldName)
		}
		return nil
	}
}

func validatePort(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("port cannot be empty")
	}
	port, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("port must be a number")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}
