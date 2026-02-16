package form

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/thalesfp/dbridge/internal/config"
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

// NewProfileForm creates an interactive form for profile creation
func NewProfileForm(initialName string) (*ProfileData, error) {
	data := &ProfileData{
		Driver:  "postgres",
		Name:    initialName,
		Host:    "localhost",
		Port:    5432,
		SSLMode: "prefer",
	}

	portStr := "5432"

	// Group 1: Driver & Profile Identification
	group1 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Database Driver").
			Description("Type of database to connect to").
			Options(
				huh.NewOption("PostgreSQL", "postgres"),
				huh.NewOption("MySQL", "mysql"),
			).
			Value(&data.Driver),

		huh.NewInput().
			Title("Profile Name").
			Description("Unique identifier for this database profile").
			Placeholder("production-db").
			Value(&data.Name).
			Validate(validateProfileName),

		huh.NewInput().
			Title("Database Name").
			Description("Name of the database").
			Placeholder("myapp_production").
			Value(&data.Database).
			Validate(validateNotEmpty("Database name")),
	).Title("Profile Identification (Step 1/3)")

	// Group 2: Connection Details
	group2 := huh.NewGroup(
		huh.NewInput().
			Title("Database Host").
			Description("Hostname or IP address of the database server").
			Placeholder("localhost").
			Value(&data.Host).
			Validate(validateNotEmpty("Host")),

		huh.NewInput().
			Title("Database Port").
			Description("Database port number").
			Placeholder("5432").
			Value(&portStr).
			Validate(validatePort),

		huh.NewInput().
			Title("Username").
			Description("Database user for authentication").
			Placeholder("postgres").
			Value(&data.Username).
			Validate(validateNotEmpty("Username")),
	).Title("Connection Details (Step 2/3)")

	// Group 3: Security
	group3 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("SSL Mode").
			Description("SSL/TLS connection security level").
			Options(
				huh.NewOption("Disable (no encryption)", "disable"),
				huh.NewOption("Prefer (try SSL, fallback to plain)", "prefer"),
				huh.NewOption("Require (force SSL)", "require"),
			).
			Value(&data.SSLMode),

		huh.NewInput().
			Title("Password").
			Description("Database password (optional - leave empty for passwordless auth)").
			EchoMode(huh.EchoModePassword).
			Value(&data.Password),
	).Title("Security (Step 3/3)")

	form := huh.NewForm(group1, group2, group3).
		WithTheme(CustomTheme())

	err := form.Run()
	if err != nil {
		return nil, err
	}

	// Password confirmation (only if password was provided)
	if data.Password != "" {
		var confirmPassword string
		confirmErr := huh.NewInput().
			Title("Confirm Password").
			Description("Re-enter your password to confirm").
			EchoMode(huh.EchoModePassword).
			Value(&confirmPassword).
			Validate(func(s string) error {
				if s != data.Password {
					return fmt.Errorf("passwords do not match")
				}
				return nil
			}).
			WithTheme(CustomTheme()).
			Run()

		if confirmErr != nil {
			return nil, confirmErr
		}
	}

	data.Port, _ = strconv.Atoi(portStr)

	// Apply driver-specific defaults when user switched driver but left port/SSL unchanged
	if data.Driver != "postgres" {
		if defaults, ok := config.DriverDefaultsMap()[data.Driver]; ok {
			if data.Port == 5432 {
				data.Port = defaults.Port
			}
			if data.SSLMode == "prefer" {
				data.SSLMode = defaults.SSLMode
			}
		}
	}

	return data, nil
}

// NewProfileFormWithDefaults creates a form pre-filled with provided values
func NewProfileFormWithDefaults(driver, name, database, host string, port int, username, sslMode string, password string) (*ProfileData, error) {
	if driver == "" {
		driver = "postgres"
	}

	data := &ProfileData{
		Driver:   driver,
		Name:     name,
		Database: database,
		Host:     host,
		Port:     port,
		Username: username,
		SSLMode:  sslMode,
		Password: password,
	}

	portStr := strconv.Itoa(port)

	// Group 1: Driver & Profile Identification
	group1 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Database Driver").
			Description("Type of database to connect to").
			Options(
				huh.NewOption("PostgreSQL", "postgres"),
				huh.NewOption("MySQL", "mysql"),
			).
			Value(&data.Driver),

		huh.NewInput().
			Title("Profile Name").
			Description("Unique identifier for this database profile").
			Placeholder("production-db").
			Value(&data.Name).
			Validate(validateProfileName),

		huh.NewInput().
			Title("Database Name").
			Description("Name of the database").
			Placeholder("myapp_production").
			Value(&data.Database).
			Validate(validateNotEmpty("Database name")),
	).Title("Profile Identification (Step 1/3)")

	// Group 2: Connection Details
	group2 := huh.NewGroup(
		huh.NewInput().
			Title("Database Host").
			Description("Hostname or IP address of the database server").
			Placeholder("localhost").
			Value(&data.Host).
			Validate(validateNotEmpty("Host")),

		huh.NewInput().
			Title("Database Port").
			Description("Database port number").
			Placeholder("5432").
			Value(&portStr).
			Validate(validatePort),

		huh.NewInput().
			Title("Username").
			Description("Database user for authentication").
			Placeholder("postgres").
			Value(&data.Username).
			Validate(validateNotEmpty("Username")),
	).Title("Connection Details (Step 2/3)")

	// Group 3: Security
	group3 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("SSL Mode").
			Description("SSL/TLS connection security level").
			Options(
				huh.NewOption("Disable (no encryption)", "disable"),
				huh.NewOption("Prefer (try SSL, fallback to plain)", "prefer"),
				huh.NewOption("Require (force SSL)", "require"),
			).
			Value(&data.SSLMode),

		huh.NewInput().
			Title("Password").
			Description("Database password (optional - leave empty for passwordless auth)").
			EchoMode(huh.EchoModePassword).
			Value(&data.Password),
	).Title("Security (Step 3/3)")

	form := huh.NewForm(group1, group2, group3).
		WithTheme(CustomTheme())

	err := form.Run()
	if err != nil {
		return nil, err
	}

	// Password confirmation (only if password was provided and changed from original)
	if data.Password != "" && data.Password != password {
		var confirmPassword string
		confirmErr := huh.NewInput().
			Title("Confirm Password").
			Description("Re-enter your password to confirm").
			EchoMode(huh.EchoModePassword).
			Value(&confirmPassword).
			Validate(func(s string) error {
				if s != data.Password {
					return fmt.Errorf("passwords do not match")
				}
				return nil
			}).
			WithTheme(CustomTheme()).
			Run()

		if confirmErr != nil {
			return nil, confirmErr
		}
	}

	data.Port, _ = strconv.Atoi(portStr)

	// Apply driver-specific defaults when user switched driver but left port/SSL unchanged
	if data.Driver != driver {
		if defaults, ok := config.DriverDefaultsMap()[data.Driver]; ok {
			if data.Port == port {
				data.Port = defaults.Port
			}
			if data.SSLMode == sslMode {
				data.SSLMode = defaults.SSLMode
			}
		}
	}

	return data, nil
}

// Validation functions

func validateProfileName(s string) error {
	if len(s) == 0 {
		return fmt.Errorf("profile name cannot be empty")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(s) {
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

// CustomTheme returns a styled theme for the form
func CustomTheme() *huh.Theme {
	t := huh.ThemeCharm()

	t.Focused.Base = t.Focused.Base.Foreground(lipgloss.Color("205"))
	t.Focused.Title = t.Focused.Title.Foreground(lipgloss.Color("205")).Bold(true)
	t.Focused.Description = t.Focused.Description.Foreground(lipgloss.Color("243"))
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(lipgloss.Color("196"))
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(lipgloss.Color("205")).SetString("▸ ")
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(lipgloss.Color("205")).SetString("→")
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(lipgloss.Color("205")).SetString("←")
	t.Focused.Option = t.Focused.Option.Foreground(lipgloss.Color("255"))
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(lipgloss.Color("205")).SetString("☐ ")
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(lipgloss.Color("205"))
	t.Focused.SelectedPrefix = t.Focused.SelectedPrefix.Foreground(lipgloss.Color("205")).SetString("✓ ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(lipgloss.Color("240"))
	t.Focused.UnselectedPrefix = t.Focused.UnselectedPrefix.Foreground(lipgloss.Color("240")).SetString("○ ")
	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Foreground(lipgloss.Color("0")).
		Background(lipgloss.Color("205")).
		Bold(true).
		Padding(0, 3)
	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Foreground(lipgloss.Color("240")).
		Background(lipgloss.Color("236")).
		Padding(0, 3)

	t.Blurred.Base = t.Blurred.Base.Foreground(lipgloss.Color("240"))
	t.Blurred.Title = t.Blurred.Title.Foreground(lipgloss.Color("240"))
	t.Blurred.Description = t.Blurred.Description.Foreground(lipgloss.Color("238"))

	return t
}
