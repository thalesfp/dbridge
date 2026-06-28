package cli

import "github.com/charmbracelet/lipgloss"

// Shared TUI color constants
const (
	ColorAccent  = lipgloss.Color("81")  // cyan/teal
	ColorDim     = lipgloss.Color("243") // gray
	ColorSuccess = lipgloss.Color("82")  // green
	ColorError   = lipgloss.Color("196") // red
	ColorWarning = lipgloss.Color("214") // orange
	ColorMuted   = lipgloss.Color("240") // dark gray
	ColorViolet  = lipgloss.Color("170") // violet/magenta
)

// Shared TUI styles
var (
	TitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent)
	HelpStyle   = lipgloss.NewStyle().Foreground(ColorDim)
	CursorStyle = lipgloss.NewStyle().Foreground(ColorAccent)
)
