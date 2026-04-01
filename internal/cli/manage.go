package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thalesfp/dbridge/internal/cli/form"
	"github.com/thalesfp/dbridge/internal/config"
	"github.com/thalesfp/dbridge/internal/credentials"
)

var (
	manageEnabledStyle  = lipgloss.NewStyle().Foreground(ColorSuccess)
	manageDisabledStyle = lipgloss.NewStyle().Foreground(ColorMuted)
	manageNameStyle     = lipgloss.NewStyle().Bold(true)
	manageStatusStyle   = lipgloss.NewStyle().Foreground(ColorSuccess)
	manageDialogStyle   = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorWarning).
				Padding(1, 3)
	manageDialogTitle = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	manageDialogHint  = lipgloss.NewStyle().Foreground(ColorDim)
)

type profileItem struct {
	name    string
	profile *config.Profile
}

type clearStatusMsg struct{}

type manageModel struct {
	profiles    []profileItem
	cursor      int
	maxName     int
	cfg         *config.Config
	confirming  bool
	confirmName string
	statusMsg   string
	quitting    bool
	editProfile string
	addProfile  bool
	width       int
	height      int
}

func newManageModel(cfg *config.Config) manageModel {
	m := manageModel{cfg: cfg}
	m.loadProfiles()
	return m
}

func (m *manageModel) loadProfiles() {
	names := make([]string, 0, len(m.cfg.Profiles))
	for name := range m.cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	m.profiles = make([]profileItem, len(names))
	m.maxName = 0
	for i, name := range names {
		m.profiles[i] = profileItem{name: name, profile: m.cfg.Profiles[name]}
		if len(name) > m.maxName {
			m.maxName = len(name)
		}
	}

	if m.cursor >= len(m.profiles) {
		m.cursor = max(0, len(m.profiles)-1)
	}
}

func (m *manageModel) reloadFromDisk() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	m.cfg = cfg
	m.loadProfiles()
	return nil
}

func (m manageModel) Init() tea.Cmd {
	return nil
}

func (m manageModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case clearStatusMsg:
		m.statusMsg = ""
		return m, nil

	case tea.KeyMsg:
		if m.confirming {
			return m.updateConfirm(msg)
		}
		return m.updateNormal(msg)
	}

	return m, nil
}

func (m manageModel) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.profiles)-1 {
			m.cursor++
		}

	case "t":
		if len(m.profiles) == 0 {
			return m, nil
		}
		name := m.profiles[m.cursor].name
		wasDisabled := m.profiles[m.cursor].profile.Disabled
		if err := toggleProfile(m.cfg, name); err != nil {
			m.statusMsg = "Error: " + err.Error()
			return m, clearStatusAfter(2 * time.Second)
		}
		_ = m.reloadFromDisk()
		if wasDisabled {
			m.statusMsg = fmt.Sprintf("✓ '%s' enabled", name)
		} else {
			m.statusMsg = fmt.Sprintf("✓ '%s' disabled", name)
		}
		return m, clearStatusAfter(2 * time.Second)

	case "d":
		if len(m.profiles) == 0 {
			return m, nil
		}
		m.confirming = true
		m.confirmName = m.profiles[m.cursor].name

	case "e":
		if len(m.profiles) == 0 {
			return m, nil
		}
		m.editProfile = m.profiles[m.cursor].name
		return m, tea.Quit

	case "a":
		m.addProfile = true
		return m, tea.Quit
	}

	return m, nil
}

func (m manageModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		if err := deleteProfile(m.cfg, m.confirmName); err != nil {
			m.statusMsg = "Error: " + err.Error()
		} else {
			m.statusMsg = fmt.Sprintf("✓ '%s' deleted", m.confirmName)
		}
		m.confirming = false
		m.confirmName = ""
		_ = m.reloadFromDisk()

		return m, clearStatusAfter(2 * time.Second)

	case "n", "esc":
		m.confirming = false
		m.confirmName = ""
	}

	return m, nil
}

func (m manageModel) View() string {
	if m.quitting {
		return ""
	}

	if len(m.profiles) == 0 {
		var b strings.Builder
		b.WriteString("\n")
		b.WriteString(TitleStyle.Render("  Manage Profiles"))
		b.WriteString("\n\n")
		b.WriteString("  " + HelpStyle.Render("No profiles configured."))
		b.WriteString("\n\n")
		b.WriteString("  " + HelpStyle.Render("a add · esc quit"))
		b.WriteString("\n")
		if m.statusMsg != "" {
			b.WriteString("  " + manageStatusStyle.Render(m.statusMsg))
			b.WriteString("\n")
		}
		return b.String()
	}

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(TitleStyle.Render("  Manage Profiles"))
	b.WriteString("\n\n")

	for i, item := range m.profiles {
		cursor := "  "
		if i == m.cursor {
			cursor = CursorStyle.Render("▸ ")
		}

		icon := manageEnabledStyle.Render("✓")
		if item.profile.Disabled {
			icon = manageDisabledStyle.Render("✗")
		}

		name := manageNameStyle.Render(fmt.Sprintf("%-*s", m.maxName, item.name))
		driver := HelpStyle.Render(driverShort(item.profile.Driver))
		conn := HelpStyle.Render(fmt.Sprintf("%s:%d/%s", item.profile.Host, item.profile.Port, item.profile.Database))

		b.WriteString(fmt.Sprintf("  %s%s %s  %s  %s\n", cursor, icon, name, driver, conn))
	}

	b.WriteString("\n")
	b.WriteString("  " + HelpStyle.Render("a add · e edit · d delete · t toggle · esc quit"))
	b.WriteString("\n")

	if m.statusMsg != "" {
		b.WriteString("  " + manageStatusStyle.Render(m.statusMsg))
		b.WriteString("\n")
	}

	content := b.String()

	if m.confirming {
		dialog := manageDialogTitle.Render(fmt.Sprintf("Delete '%s'?", m.confirmName)) +
			"\n\n" +
			manageDialogHint.Render("y confirm · n cancel")
		box := manageDialogStyle.Render(dialog)

		w := m.width
		h := m.height
		if w == 0 {
			w = 60
		}
		if h == 0 {
			h = 20
		}
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, box)
	}

	return content
}

func driverShort(driver string) string {
	switch driver {
	case "postgres":
		return "pg"
	case "mysql":
		return "my"
	default:
		if len(driver) > 2 {
			return driver[:2]
		}
		return driver
	}
}

func deleteProfile(cfg *config.Config, name string) error {
	ctx := context.Background()
	credStore, err := credentials.NewStore("dbridge")
	if err == nil {
		_ = credStore.Delete(ctx, name)
	}
	if err := cfg.RemoveProfile(name); err != nil {
		return fmt.Errorf("failed to remove profile: %w", err)
	}
	return cfg.Save()
}

func runAddFlow() error {
	_, err := form.RunProfileForm(nil, testConnectionOption(), form.WithSave(saveProfileCallback))
	return err
}

func saveProfileCallback(d *form.ProfileData) string {
	cfg, err := config.Load()
	if err != nil {
		return "failed to load config: " + err.Error()
	}

	profile := &config.Profile{
		Driver:   d.Driver,
		Name:     d.Name,
		Host:     d.Host,
		Port:     d.Port,
		Database: d.Database,
		Username: d.Username,
		SSLMode:  d.SSLMode,
		ReadOnly: true,
	}

	if d.Password != "" {
		credStore, err := credentials.NewStore("dbridge")
		if err != nil {
			return "failed to open credential store: " + err.Error()
		}
		if err := credStore.Save(context.Background(), d.Name, credentials.Credentials{
			Username: d.Username,
			Password: d.Password,
		}); err != nil {
			return "failed to save credentials: " + err.Error()
		}
	}

	cfg.AddProfile(profile)
	if err := cfg.Save(); err != nil {
		return "failed to save config: " + err.Error()
	}
	return ""
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func runManageTUI() error {
	for {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		m := newManageModel(cfg)
		p := tea.NewProgram(m, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			return err
		}

		result := finalModel.(manageModel)
		if result.quitting {
			return nil
		}

		if result.addProfile {
			if err := runAddFlow(); err != nil {
				fmt.Printf("\n  Error: %s\n", err)
			}
			fmt.Println()
			continue
		}

		if result.editProfile != "" {
			if err := runEditFlow(cfg, result.editProfile); err != nil {
				fmt.Printf("\n  Error: %s\n", err)
			}
			fmt.Println()
			continue
		}

		return nil
	}
}
