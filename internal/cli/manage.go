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
	manageDialogTitle    = lipgloss.NewStyle().Foreground(ColorWarning).Bold(true)
	manageDialogHint     = lipgloss.NewStyle().Foreground(ColorDim)
	manageDriverPgStyle  = lipgloss.NewStyle().Foreground(ColorAccent)
	manageDriverMyStyle  = lipgloss.NewStyle().Foreground(ColorWarning)
	manageDriverMoStyle  = lipgloss.NewStyle().Foreground(ColorSuccess)
	manageDriverMsStyle  = lipgloss.NewStyle().Foreground(ColorViolet)
	manageDriverColStyle = lipgloss.NewStyle().Width(2)
	manageEnvProdStyle   = lipgloss.NewStyle().Foreground(ColorError)
	manageEnvStagStyle   = lipgloss.NewStyle().Foreground(ColorWarning)
	manageEnvDevStyle    = lipgloss.NewStyle().Foreground(ColorSuccess)
)

type connectionItem struct {
	name string
	conn *config.Connection
}

type clearStatusMsg struct{}

type manageModel struct {
	connections    []connectionItem
	cursor         int
	maxName        int
	maxEnv         int
	envColStyle    lipgloss.Style
	headerLine     string
	cfg            *config.Config
	confirming     bool
	confirmName    string
	statusMsg      string
	quitting       bool
	editConnection string
	addConnection  bool
	width          int
	height         int
}

func newManageModel(cfg *config.Config) manageModel {
	m := manageModel{cfg: cfg}
	m.loadConnections()
	return m
}

func (m *manageModel) loadConnections() {
	names := make([]string, 0, len(m.cfg.Connections))
	for name := range m.cfg.Connections {
		names = append(names, name)
	}
	sort.Strings(names)

	m.connections = make([]connectionItem, len(names))
	m.maxName = 0
	maxEnv := 0
	for i, name := range names {
		conn := m.cfg.Connections[name]
		m.connections[i] = connectionItem{name: name, conn: conn}
		if len(name) > m.maxName {
			m.maxName = len(name)
		}
		if len(conn.Environment) > maxEnv {
			maxEnv = len(conn.Environment)
		}
	}
	m.maxEnv = maxEnv
	m.envColStyle = lipgloss.NewStyle().Width(maxEnv)

	hName := HelpStyle.Render(fmt.Sprintf("%-*s", m.maxName, "name"))
	hEnvLabel := ""
	if maxEnv >= 3 {
		hEnvLabel = fmt.Sprintf("%-*s", maxEnv, "env")
	} else if maxEnv > 0 {
		hEnvLabel = fmt.Sprintf("%-*s", maxEnv, "env"[:maxEnv])
	}
	m.headerLine = fmt.Sprintf("      %s  %-2s  %s  %s\n",
		hName, "",
		HelpStyle.Render(hEnvLabel),
		HelpStyle.Render("connection"),
	)

	if m.cursor >= len(m.connections) {
		m.cursor = max(0, len(m.connections)-1)
	}
}

func (m *manageModel) reloadFromDisk() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	m.cfg = cfg
	m.loadConnections()
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
		if m.cursor < len(m.connections)-1 {
			m.cursor++
		}

	case "t":
		if len(m.connections) == 0 {
			return m, nil
		}
		name := m.connections[m.cursor].name
		wasDisabled := m.connections[m.cursor].conn.Disabled
		if err := toggleConnection(m.cfg, name); err != nil {
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
		if len(m.connections) == 0 {
			return m, nil
		}
		m.confirming = true
		m.confirmName = m.connections[m.cursor].name

	case "enter":
		if len(m.connections) == 0 {
			return m, nil
		}
		m.editConnection = m.connections[m.cursor].name
		return m, tea.Quit

	case "a":
		m.addConnection = true
		return m, tea.Quit
	}

	return m, nil
}

func (m manageModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		if err := deleteConnection(m.cfg, m.confirmName); err != nil {
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

	if len(m.connections) == 0 {
		var b strings.Builder
		b.WriteString("\n")
		b.WriteString(TitleStyle.Render("  dbridge") + HelpStyle.Render("  Manage Connections"))
		b.WriteString("\n\n")
		b.WriteString("  " + HelpStyle.Render("No connections configured."))
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
	b.WriteString(TitleStyle.Render("  dbridge") + HelpStyle.Render("  Manage Connections"))
	b.WriteString("\n\n")

	b.WriteString(m.headerLine)
	b.WriteString("\n")

	for i, item := range m.connections {
		cursor := "  "
		if i == m.cursor {
			cursor = CursorStyle.Render("▸ ")
		}

		icon := manageEnabledStyle.Render("✓")
		if item.conn.Disabled {
			icon = manageDisabledStyle.Render("✗")
		}

		name := manageNameStyle.Render(fmt.Sprintf("%-*s", m.maxName, item.name))
		driver := manageDriverColStyle.Render(driverBadge(item.conn.Driver))
		env := m.envColStyle.Render(envBadge(item.conn.Environment))
		connStr := HelpStyle.Render(fmt.Sprintf("%s:%d/%s", item.conn.Host, item.conn.Port, item.conn.Database))

		b.WriteString(fmt.Sprintf("  %s%s %s  %s  %s  %s\n", cursor, icon, name, driver, env, connStr))
	}

	b.WriteString("\n")
	b.WriteString("  " + HelpStyle.Render("a add · enter edit · d delete · t enable/disable · esc quit"))
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
	case "postgres", "":
		return "pg"
	case "mysql":
		return "my"
	case "mongodb":
		return "mo"
	case "mssql":
		return "ms"
	default:
		if len(driver) > 2 {
			return driver[:2]
		}
		return driver
	}
}

func driverBadge(driver string) string {
	switch driver {
	case "mysql":
		return manageDriverMyStyle.Render(driverShort(driver))
	case "mongodb":
		return manageDriverMoStyle.Render(driverShort(driver))
	case "mssql":
		return manageDriverMsStyle.Render(driverShort(driver))
	default:
		return manageDriverPgStyle.Render(driverShort(driver))
	}
}

func envBadge(env string) string {
	if env == "" {
		return ""
	}
	lower := strings.ToLower(env)
	switch {
	case strings.Contains(lower, "prod"):
		return manageEnvProdStyle.Render(env)
	case strings.Contains(lower, "stag"):
		return manageEnvStagStyle.Render(env)
	case strings.Contains(lower, "dev"), strings.Contains(lower, "local"):
		return manageEnvDevStyle.Render(env)
	default:
		return HelpStyle.Render(env)
	}
}

func deleteConnection(cfg *config.Config, name string) error {
	ctx := context.Background()
	credStore, err := credentials.NewStore("dbridge")
	if err == nil {
		_ = credStore.Delete(ctx, name)
	}
	if err := cfg.RemoveConnection(name); err != nil {
		return fmt.Errorf("failed to remove connection: %w", err)
	}
	return cfg.Save()
}

func runAddFlow() error {
	_, err := form.RunConnectionForm(nil, testConnectionOption(), form.WithSave(saveConnectionCallback))
	return err
}

func saveConnectionCallback(d *form.ConnectionData) string {
	cfg, err := config.Load()
	if err != nil {
		return "failed to load config: " + err.Error()
	}

	conn := &config.Connection{
		Driver:      d.Driver,
		Name:        d.Name,
		Host:        d.Host,
		Port:        d.Port,
		Database:    d.Database,
		Username:    d.Username,
		SSLMode:     d.SSLMode,
		SRV:         d.SRV,
		Environment: d.Environment,
		Description: d.Description,
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

	cfg.AddConnection(conn)
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

		if result.addConnection {
			if err := runAddFlow(); err != nil {
				fmt.Printf("\n  Error: %s\n", err)
			}
			fmt.Println()
			continue
		}

		if result.editConnection != "" {
			if err := runEditFlow(cfg, result.editConnection); err != nil {
				fmt.Printf("\n  Error: %s\n", err)
			}
			fmt.Println()
			continue
		}

		return nil
	}
}
