package form

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thalesfp/dbridge/internal/config"
)

// Colors (matching the shared palette in cli/styles.go)
const (
	colorAccent = lipgloss.Color("81")
	colorDim    = lipgloss.Color("243")
	colorError  = lipgloss.Color("196")
	colorMuted  = lipgloss.Color("240")
)

var (
	formTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	formLabelStyle     = lipgloss.NewStyle().Width(16).Foreground(colorDim)
	formActiveLabel    = lipgloss.NewStyle().Width(16).Foreground(colorAccent).Bold(true)
	formErrorStyle     = lipgloss.NewStyle().Foreground(colorError)
	formHelpStyle      = lipgloss.NewStyle().Foreground(colorDim)
	formSelectActive   = lipgloss.NewStyle().Foreground(colorAccent).Bold(true)
	formSelectInactive = lipgloss.NewStyle().Foreground(colorMuted)
	formCursorStyle    = lipgloss.NewStyle().Foreground(colorAccent)
	formDialogSuccess  = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("82")).
				Padding(1, 3)
	formDialogFail     = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("214")).
				Padding(1, 3)
	formDialogTesting  = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAccent).
				Padding(1, 3)
	formDialogTitle    = lipgloss.NewStyle().Bold(true)
	formDialogHintStyle = lipgloss.NewStyle().Foreground(colorDim)
)

type selectOption struct {
	label string
	value string
}

type fieldKind int

const (
	fieldText   fieldKind = iota
	fieldSelect
)

// Field labels used for lookup
const (
	labelDriver   = "Driver"
	labelMode     = "Mode"
	labelDatabase = "Database Name"
	labelHost     = "Host"
	labelPort     = "Port"
	labelName     = "Connection Name"
	labelUsername  = "Username"
	labelPassword  = "Password"
	labelSSLMode  = "SSL Mode"
	labelEnv      = "Environment"
	labelDesc     = "Description"
)

// Mode values for MongoDB connection
const (
	modeStandard = "standard"
	modeSRV      = "srv"
)

type formField struct {
	kind     fieldKind
	label    string
	input    textinput.Model
	options  []selectOption
	selected int
	validate func(string) error
}

// FormOption configures the connection form.
type FormOption func(*connectionFormModel)

// WithTestConnection adds a "test connection" keybind (ctrl+t) to the form.
// The callback receives a context and current field values, and returns "" on success or an error message.
// The context is cancelled if the user presses esc during testing.
func WithTestConnection(fn func(context.Context, *ConnectionData) string) FormOption {
	return func(m *connectionFormModel) {
		m.testFn = fn
	}
}

// WithSave adds an in-form save callback. On submit, the form calls fn with the connection data.
// Returns "" on success or an error message. The result is shown as a dialog before the form exits.
func WithSave(fn func(*ConnectionData) string) FormOption {
	return func(m *connectionFormModel) {
		m.saveFn = fn
	}
}

type connectionFormModel struct {
	fields     []formField
	focusIndex int
	err        string
	submitted  bool
	cancelled  bool

	// Password (separate screen)
	password   string
	editingPw  bool
	pwInput    textinput.Model
	pwConfirm  textinput.Model
	pwFocus    int
	pwVisible  bool
	origPw     string

	// Callbacks
	testFn func(context.Context, *ConnectionData) string
	saveFn func(*ConnectionData) string

	// Connection test state
	testing      bool
	testCancel   context.CancelFunc
	testGen      int

	// Dialog overlay
	dialogMsg string // non-empty = show centered dialog
	dialogOk  bool   // true = success style, false = failure style
	saved     bool   // true = dismiss dialog should quit

	editMode    bool
	defaults    *ConnectionData
	lastDriver  string
	width       int
	height      int
}

func (m *connectionFormModel) cancelTest() {
	if m.testCancel != nil {
		m.testCancel()
		m.testCancel = nil
	}
	m.testing = false
}

func newTextInput(placeholder string, value string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.CharLimit = 256
	ti.Width = 30
	return ti
}

func newPasswordInput(placeholder string, value string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(value)
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.CharLimit = 256
	ti.Width = 30
	return ti
}

func (m *connectionFormModel) fieldByLabel(label string) *formField {
	for i := range m.fields {
		if m.fields[i].label == label {
			return &m.fields[i]
		}
	}
	return nil
}

func (m *connectionFormModel) fieldValue(label string) string {
	f := m.fieldByLabel(label)
	if f == nil {
		return ""
	}
	if f.kind == fieldSelect {
		return f.options[f.selected].value
	}
	return f.input.Value()
}

func selectIdxForValue(opts []selectOption, val string) int {
	for i, o := range opts {
		if o.value == val {
			return i
		}
	}
	return 0
}

func sslOptions(driver string) []selectOption {
	if driver == "mysql" {
		return []selectOption{
			{"Disable", "disable"}, {"Preferred", "preferred"}, {"Require", "require"}, {"Verify Full", "verify-full"},
		}
	}
	return []selectOption{
		{"Disable", "disable"}, {"Prefer", "prefer"}, {"Require", "require"}, {"Verify Full", "verify-full"},
	}
}

func driverOptions() []selectOption {
	return []selectOption{
		{"PostgreSQL", "postgres"}, {"MySQL", "mysql"}, {"MongoDB", "mongodb"},
	}
}

func modeOptions() []selectOption {
	return []selectOption{
		{"mongodb://", modeStandard}, {"mongodb+srv://", modeSRV},
	}
}

func envOptions() []selectOption {
	return []selectOption{
		{"production", "production"}, {"staging", "staging"}, {"development", "development"}, {"local", "local"},
	}
}

func buildFields(driver, mode string, vals map[string]string, isEdit bool) []formField {
	driverOpts := driverOptions()
	sslOpts := sslOptions(driver)

	fields := []formField{
		{kind: fieldSelect, label: labelDriver, options: driverOpts, selected: selectIdxForValue(driverOpts, driver)},
	}

	if driver == "mongodb" {
		modeOpts := modeOptions()
		fields = append(fields, formField{kind: fieldSelect, label: labelMode, options: modeOpts, selected: selectIdxForValue(modeOpts, mode)})
	}

	eOpts := envOptions()

	fields = append(fields,
		formField{kind: fieldText, label: labelName, input: newTextInput("production-db", vals[labelName]), validate: validateConnectionName},
		formField{kind: fieldSelect, label: labelEnv, options: eOpts, selected: selectIdxForValue(eOpts, vals[labelEnv])},
		formField{kind: fieldText, label: labelDesc, input: newTextInput("optional", vals[labelDesc])},
		formField{kind: fieldText, label: labelDatabase, input: newTextInput("myapp", vals[labelDatabase]), validate: validateNotEmpty("Database Name")},
		formField{kind: fieldText, label: labelHost, input: newTextInput("localhost", vals[labelHost]), validate: validateNotEmpty("Host")},
	)

	if driver != "mongodb" || mode != modeSRV {
		fields = append(fields, formField{kind: fieldText, label: labelPort, input: newTextInput("5432", vals[labelPort]), validate: validatePort})
	}

	fields = append(fields,
		formField{kind: fieldText, label: labelUsername, input: newTextInput("postgres", vals[labelUsername]), validate: validateNotEmpty("Username")},
	)

	if !isEdit {
		fields = append(fields, formField{kind: fieldText, label: labelPassword, input: newPasswordInput("optional", vals[labelPassword])})
	}

	fields = append(fields, formField{kind: fieldSelect, label: labelSSLMode, options: sslOpts, selected: selectIdxForValue(sslOpts, vals[labelSSLMode])})

	return fields
}

func (m *connectionFormModel) snapshotValues() map[string]string {
	vals := map[string]string{}
	for i := range m.fields {
		f := &m.fields[i]
		switch f.kind {
		case fieldText:
			vals[f.label] = f.input.Value()
		case fieldSelect:
			vals[f.label] = f.options[f.selected].value
		}
	}
	return vals
}

func (m *connectionFormModel) onSelectChanged(f *formField) {
	if f.label == labelDriver || f.label == labelMode {
		m.rebuildFields(m.fieldValue(labelDriver), m.fieldValue(labelMode))
	}
}

func (m *connectionFormModel) rebuildFields(driver, mode string) {
	vals := m.snapshotValues()

	if driver != m.lastDriver {
		defaults, ok := config.DriverDefaultsMap()[driver]
		oldDefaults, oldOk := config.DriverDefaultsMap()[m.lastDriver]

		if ok {
			if !oldOk || vals[labelPort] == strconv.Itoa(oldDefaults.Port) {
				vals[labelPort] = strconv.Itoa(defaults.Port)
			}
			if !oldOk || vals[labelSSLMode] == oldDefaults.SSLMode {
				vals[labelSSLMode] = defaults.SSLMode
			}
		}

		m.lastDriver = driver
	}

	if vals[labelPort] == "" {
		if defaults, ok := config.DriverDefaultsMap()[driver]; ok {
			vals[labelPort] = strconv.Itoa(defaults.Port)
		}
	}

	m.fields = buildFields(driver, mode, vals, m.editMode)

	if m.focusIndex >= len(m.fields) {
		m.focusIndex = len(m.fields) - 1
	}

	m.focusField(m.focusIndex)
}

func newConnectionFormModel(initial *ConnectionData) connectionFormModel {
	pgDefaults := config.DriverDefaultsMap()["postgres"]
	data := &ConnectionData{
		Driver:  "postgres",
		Host:    "localhost",
		Port:    pgDefaults.Port,
		SSLMode: pgDefaults.SSLMode,
	}
	origPw := ""
	if initial != nil {
		if initial.Driver != "" {
			data.Driver = initial.Driver
		}
		if initial.Name != "" {
			data.Name = initial.Name
		}
		data.Database = initial.Database
		if initial.Host != "" {
			data.Host = initial.Host
		}
		if initial.Port != 0 {
			data.Port = initial.Port
		}
		data.Username = initial.Username
		if initial.SSLMode != "" {
			data.SSLMode = initial.SSLMode
		}
		data.Password = initial.Password
		data.SRV = initial.SRV
		data.Environment = initial.Environment
		data.Description = initial.Description
		origPw = initial.Password
	}

	portStr := strconv.Itoa(data.Port)
	if data.Port == 0 {
		portStr = "5432"
	}

	mode := modeStandard
	if data.SRV {
		mode = modeSRV
	}

	isEdit := initial != nil && initial.Host != ""

	env := data.Environment
	if env == "" {
		env = "production"
	}

	vals := map[string]string{
		labelDatabase: data.Database,
		labelHost:     data.Host,
		labelPort:     portStr,
		labelName:     data.Name,
		labelUsername:  data.Username,
		labelPassword:  data.Password,
		labelSSLMode:  data.SSLMode,
		labelEnv:      env,
		labelDesc:     data.Description,
	}

	fields := buildFields(data.Driver, mode, vals, isEdit)

	return connectionFormModel{
		fields:     fields,
		focusIndex: 0,
		password:   data.Password,
		origPw:     origPw,
		editMode:   isEdit,
		defaults:   data,
		lastDriver: data.Driver,
	}
}

type testConnectionResultMsg struct {
	err string // "" means success
	gen int
}

func (m connectionFormModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m connectionFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case testConnectionResultMsg:
		if !m.testing || msg.gen != m.testGen {
			return m, nil
		}
		m.cancelTest()
		if msg.err == "" {
			m.dialogMsg = "✓ Connection successful"
			m.dialogOk = true
		} else {
			m.dialogMsg = "⚠ " + msg.err
			m.dialogOk = false
		}
		return m, nil

	case tea.KeyMsg:
		if m.testing {
			switch msg.String() {
			case "ctrl+c":
				m.cancelTest()
				m.cancelled = true
				return m, tea.Quit
			case "esc":
				m.cancelTest()
			}
			return m, nil
		}

		// Dismiss dialog overlay on any key
		if m.dialogMsg != "" {
			if m.saved {
				m.submitted = true
				return m, tea.Quit
			}
			m.dialogMsg = ""
			return m, nil
		}
		if m.editingPw {
			return m.updatePassword(msg)
		}
		return m.updateMain(msg)
	}

	// Forward non-key messages to active text input
	if m.editingPw {
		return m.forwardToPwInput(msg)
	}

	f := &m.fields[m.focusIndex]
	if f.kind == fieldText {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m connectionFormModel) forwardToPwInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.pwFocus == 0 {
		var cmd tea.Cmd
		m.pwInput, cmd = m.pwInput.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.pwConfirm, cmd = m.pwConfirm.Update(msg)
	return m, cmd
}

func (m connectionFormModel) updateMain(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit

	case "esc":
		m.cancelled = true
		return m, tea.Quit

	case "tab", "down":
		if m.focusIndex < len(m.fields)-1 {
			m.blurField(m.focusIndex)
			m.focusIndex++
			m.err = ""
			m.focusField(m.focusIndex)
		}
		return m, nil

	case "shift+tab", "up":
		if m.focusIndex > 0 {
			m.blurField(m.focusIndex)
			m.focusIndex--
			m.err = ""
			m.focusField(m.focusIndex)
		}
		return m, nil

	case "left":
		f := &m.fields[m.focusIndex]
		if f.kind == fieldSelect && f.selected > 0 {
			f.selected--
			m.onSelectChanged(f)
			return m, nil
		}

	case "right":
		f := &m.fields[m.focusIndex]
		if f.kind == fieldSelect && f.selected < len(f.options)-1 {
			f.selected++
			m.onSelectChanged(f)
			return m, nil
		}

	case "ctrl+p":
		if m.editMode {
			m.enterPasswordScreen()
		}
		return m, nil

	case "ctrl+t":
		if m.testFn != nil {
			data := m.toConnectionData()
			ctx, cancel := context.WithCancel(context.Background())
			m.testGen++
			m.testing = true
			m.testCancel = cancel
			m.err = ""
			testFn := m.testFn
			gen := m.testGen
			cmd := func() tea.Msg {
				result := testFn(ctx, data)
				return testConnectionResultMsg{err: result, gen: gen}
			}
			return m, cmd
		}
		return m, nil

	case "enter":
		if err := m.validateAll(); err != "" {
			m.err = err
			return m, nil
		}
		if m.saveFn != nil {
			data := m.toConnectionData()
			result := m.saveFn(data)
			if result == "" {
				m.dialogMsg = fmt.Sprintf("✓ Connection '%s' saved", data.Name)
				m.dialogOk = true
				m.saved = true
			} else {
				m.dialogMsg = "⚠ " + result
				m.dialogOk = false
			}
			return m, nil
		}
		m.submitted = true
		return m, tea.Quit
	}

	// Forward to active text input (must be after special key handling)
	f := &m.fields[m.focusIndex]
	if f.kind == fieldText {
		var cmd tea.Cmd
		f.input, cmd = f.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *connectionFormModel) enterPasswordScreen() {
	m.editingPw = true
	m.pwInput = newPasswordInput("password", "")
	m.pwConfirm = newPasswordInput("confirm password", "")
	m.pwFocus = 0
	m.pwVisible = false
	m.pwInput.Focus()
	m.err = ""
}

func (m connectionFormModel) updatePassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.cancelled = true
		return m, tea.Quit

	case "esc":
		m.editingPw = false
		m.err = ""
		return m, nil

	case "tab", "down":
		if m.pwFocus == 0 {
			m.pwInput.Blur()
			m.pwFocus = 1
			m.pwConfirm.Focus()
		}
		return m, nil

	case "shift+tab", "up":
		if m.pwFocus == 1 {
			m.pwConfirm.Blur()
			m.pwFocus = 0
			m.pwInput.Focus()
		}
		return m, nil

	case "ctrl+g":
		m.pwVisible = !m.pwVisible
		if m.pwVisible {
			m.pwInput.EchoMode = textinput.EchoNormal
			m.pwConfirm.EchoMode = textinput.EchoNormal
		} else {
			m.pwInput.EchoMode = textinput.EchoPassword
			m.pwConfirm.EchoMode = textinput.EchoPassword
		}
		return m, nil

	case "enter":
		pw := m.pwInput.Value()
		confirm := m.pwConfirm.Value()
		if pw != "" && pw != confirm {
			m.dialogMsg = "⚠ Passwords do not match"
			m.dialogOk = false
			return m, nil
		}
		m.password = pw
		m.editingPw = false
		m.err = ""
		if pw != "" {
			m.dialogMsg = "✓ Password updated"
		} else {
			m.dialogMsg = "✓ Password cleared"
		}
		m.dialogOk = true
		return m, nil
	}

	return m.forwardToPwInput(msg)
}

func (m *connectionFormModel) focusField(idx int) {
	f := &m.fields[idx]
	if f.kind == fieldText {
		f.input.Focus()
	}
}

func (m *connectionFormModel) blurField(idx int) {
	f := &m.fields[idx]
	if f.kind == fieldText {
		f.input.Blur()
	}
}

func (m connectionFormModel) validateAll() string {
	for i := range m.fields {
		f := &m.fields[i]
		if f.validate != nil && f.kind == fieldText {
			if err := f.validate(f.input.Value()); err != nil {
				return fmt.Sprintf("%s: %s", f.label, err.Error())
			}
		}
	}
	return ""
}

func (m connectionFormModel) renderOverlay(style lipgloss.Style, title string, titleColor lipgloss.Color, hint string) string {
	dialog := formDialogTitle.Foreground(titleColor).Render(title) +
		"\n\n" +
		formDialogHintStyle.Render(hint)
	box := style.Render(dialog)

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

func (m connectionFormModel) View() string {
	if m.submitted || m.cancelled {
		return ""
	}

	if m.testing {
		return m.renderOverlay(formDialogTesting, "Testing connection...", colorAccent, "esc to cancel")
	}

	if m.dialogMsg != "" {
		style := formDialogFail
		titleColor := lipgloss.Color("214")
		if m.dialogOk {
			style = formDialogSuccess
			titleColor = lipgloss.Color("82")
		}
		return m.renderOverlay(style, m.dialogMsg, titleColor, "press any key to continue")
	}

	if m.editingPw {
		return m.viewPassword()
	}

	return m.viewMain()
}

func (m connectionFormModel) viewMain() string {
	var b strings.Builder

	title := "Add Connection"
	if m.editMode {
		title = "Edit Connection"
	}
	b.WriteString(fmt.Sprintf("\n  %s\n\n", formTitleStyle.Render(title)))

	for i := range m.fields {
		f := &m.fields[i]
		active := i == m.focusIndex

		label := formLabelStyle.Render(f.label)
		if active {
			label = formActiveLabel.Render(f.label)
		}

		cursor := "  "
		if active {
			cursor = formCursorStyle.Render("▸ ")
		}

		switch f.kind {
		case fieldText:
			b.WriteString(fmt.Sprintf("  %s%s %s\n", cursor, label, f.input.View()))
		case fieldSelect:
			b.WriteString(fmt.Sprintf("  %s%s %s\n", cursor, label, m.renderSelect(f, active)))
		}
	}

	if m.err != "" {
		b.WriteString(fmt.Sprintf("\n  %s\n", formErrorStyle.Render(m.err)))
	}

	extras := ""
	if m.testFn != nil {
		extras += " · ctrl+t test"
	}
	if m.editMode {
		extras += " · ctrl+p password"
	}
	help := "↑↓ navigate · ←→ select" + extras + " · enter save · esc cancel"
	b.WriteString(fmt.Sprintf("\n  %s\n", formHelpStyle.Render(help)))

	return b.String()
}

func (m connectionFormModel) viewPassword() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  %s\n\n", formTitleStyle.Render("Change Password")))

	visLabel := "hidden"
	if m.pwVisible {
		visLabel = "visible"
	}

	// Password input
	pwLabel := formLabelStyle.Render("Password")
	pwCursor := "  "
	if m.pwFocus == 0 {
		pwLabel = formActiveLabel.Render("Password")
		pwCursor = formCursorStyle.Render("▸ ")
	}
	b.WriteString(fmt.Sprintf("  %s%s %s\n", pwCursor, pwLabel, m.pwInput.View()))

	// Confirm input
	cfLabel := formLabelStyle.Render("Confirm")
	cfCursor := "  "
	if m.pwFocus == 1 {
		cfLabel = formActiveLabel.Render("Confirm")
		cfCursor = formCursorStyle.Render("▸ ")
	}
	b.WriteString(fmt.Sprintf("  %s%s %s\n", cfCursor, cfLabel, m.pwConfirm.View()))

	if m.err != "" {
		b.WriteString(fmt.Sprintf("\n  %s\n", formErrorStyle.Render(m.err)))
	}

	b.WriteString(fmt.Sprintf("\n  %s\n",
		formHelpStyle.Render(fmt.Sprintf("tab next · ctrl+g show/hide (%s) · enter save · esc cancel", visLabel))))

	return b.String()
}

func (m connectionFormModel) renderSelect(f *formField, active bool) string {
	parts := make([]string, 0, len(f.options))
	for i, opt := range f.options {
		if i == f.selected {
			if active {
				parts = append(parts, formSelectActive.Render("▸ "+opt.label))
			} else {
				parts = append(parts, formSelectActive.Render(opt.label))
			}
		} else {
			parts = append(parts, formSelectInactive.Render(opt.label))
		}
	}
	return strings.Join(parts, "  ")
}

func (m connectionFormModel) toConnectionData() *ConnectionData {
	port, _ := strconv.Atoi(m.fieldValue(labelPort))

	driver := m.fieldValue(labelDriver)
	sslMode := m.fieldValue(labelSSLMode)
	mode := m.fieldValue(labelMode)

	pw := m.password
	if !m.editMode {
		pw = m.fieldValue(labelPassword)
	}

	return &ConnectionData{
		Driver:      driver,
		Name:        m.fieldValue(labelName),
		Database:    m.fieldValue(labelDatabase),
		Host:        m.fieldValue(labelHost),
		Port:        port,
		Username:    m.fieldValue(labelUsername),
		SSLMode:     sslMode,
		Password:    pw,
		SRV:         mode == modeSRV,
		Environment: m.fieldValue(labelEnv),
		Description: m.fieldValue(labelDesc),
	}
}

// RunConnectionForm runs the interactive connection form.
// Pass nil for creation mode, or a ConnectionData for edit/clone mode.
func RunConnectionForm(initial *ConnectionData, opts ...FormOption) (*ConnectionData, error) {
	m := newConnectionFormModel(initial)
	for _, opt := range opts {
		opt(&m)
	}
	p := tea.NewProgram(m)
	final, err := p.Run()
	if err != nil {
		return nil, err
	}

	result := final.(connectionFormModel)
	if result.cancelled {
		return nil, fmt.Errorf("form cancelled")
	}

	return result.toConnectionData(), nil
}
