// Package help provides a multi-tab searchable help dialog for the ClawGo TUI.
// It mirrors the TypeScript HelpV2 component with Commands, Keybindings, and Tips tabs.
package help

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Tab represents one of the help dialog tabs.
type Tab int

const (
	// TabCommands shows all registered slash commands.
	TabCommands Tab = iota
	// TabKeybindings shows keyboard shortcuts.
	TabKeybindings
	// TabTips shows usage tips.
	TabTips
)

// tabCount is the number of tabs available.
const tabCount = 3

// HelpEntry represents a single item displayed in the help list.
type HelpEntry struct {
	Name        string
	Description string
	Category    string
}

// HelpModel is the Bubble Tea model for the interactive help dialog.
type HelpModel struct {
	active        bool
	activeTab     Tab
	width         int
	height        int
	filter        string
	textinput     textinput.Model
	commands      []HelpEntry
	keybindings   []HelpEntry
	tips          []HelpEntry
	scrollOffset  int
	filteredItems []HelpEntry
}

// DismissHelpMsg is sent when the help dialog is dismissed.
type DismissHelpMsg struct{}

// tabNames provides display labels for each tab.
var tabNames = [tabCount]string{"Commands", "Keybindings", "Tips"}

// builtinTips provides a set of helpful tips for new users.
var builtinTips = []HelpEntry{
	{Name: "File references", Description: "Use @ to reference files in your prompt", Category: "input"},
	{Name: "Shell commands", Description: "Use ! prefix to run shell commands inline", Category: "input"},
	{Name: "Message jump", Description: "Use Ctrl+K to jump to a specific message", Category: "navigation"},
	{Name: "Compact context", Description: "Use /compact to compress the conversation context", Category: "commands"},
	{Name: "Model switch", Description: "Use /model to switch between AI models mid-conversation", Category: "commands"},
	{Name: "Vim mode", Description: "Use /vim to toggle vim-style navigation", Category: "navigation"},
	{Name: "Multi-line input", Description: "Use Shift+Enter to insert newlines in your prompt", Category: "input"},
	{Name: "Cost tracking", Description: "Use /cost to see session and total API costs", Category: "info"},
	{Name: "Session resume", Description: "Use /resume to continue a previous conversation", Category: "commands"},
	{Name: "Permission modes", Description: "Use --permission-mode flag to control tool approval behavior", Category: "config"},
	{Name: "Working directory", Description: "Use /add-dir to add extra directories to the context", Category: "commands"},
	{Name: "Tab completion", Description: "Type / and press Tab to cycle through command suggestions", Category: "input"},
}

// Styling for the help dialog.
var (
	tabStyle = lipgloss.NewStyle().
			Padding(0, 2)

	activeTabStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Bold(true).
			Foreground(lipgloss.Color("#61AFEF")).
			Underline(true)

	entryNameStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ABB2BF"))

	entryDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370"))

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#5C6370")).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#61AFEF"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#5C6370"))

	highlightStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#3E4451")).
			Foreground(lipgloss.Color("#ABB2BF"))
)

// NewHelpModel creates a new help dialog model populated with the given
// commands and keybindings. Tips are provided via built-in defaults.
func NewHelpModel(commands []HelpEntry, keybindings []HelpEntry) HelpModel {
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 80

	tips := make([]HelpEntry, len(builtinTips))
	copy(tips, builtinTips)

	m := HelpModel{
		active:      false,
		activeTab:   TabCommands,
		textinput:   ti,
		commands:    commands,
		keybindings: keybindings,
		tips:        tips,
	}
	m.filteredItems = m.activeEntries()
	return m
}

// Show activates the help dialog and focuses the search input.
func (m *HelpModel) Show() tea.Cmd {
	m.active = true
	m.activeTab = TabCommands
	m.filter = ""
	m.textinput.SetValue("")
	m.scrollOffset = 0
	m.filteredItems = m.activeEntries()
	return m.textinput.Focus()
}

// Dismiss deactivates the help dialog.
func (m *HelpModel) Dismiss() {
	m.active = false
	m.textinput.Blur()
}

// IsActive returns true if the help dialog is currently visible.
func (m *HelpModel) IsActive() bool {
	return m.active
}

// Update processes key events for the help dialog.
func (m HelpModel) Update(msg tea.Msg) (HelpModel, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		k := keyMsg.Key()
		switch {
		case k.Code == tea.KeyEscape:
			m.active = false
			m.textinput.Blur()
			return m, func() tea.Msg { return DismissHelpMsg{} }

		case k.Code == tea.KeyTab && k.Mod&tea.ModShift != 0:
			// Shift+Tab: previous tab
			m.activeTab = (m.activeTab + tabCount - 1) % tabCount
			m.scrollOffset = 0
			m.applyFilter()
			return m, nil

		case k.Code == tea.KeyTab:
			// Tab: next tab
			m.activeTab = (m.activeTab + 1) % tabCount
			m.scrollOffset = 0
			m.applyFilter()
			return m, nil

		case k.Code == tea.KeyUp:
			if m.scrollOffset > 0 {
				m.scrollOffset--
			}
			return m, nil

		case k.Code == tea.KeyDown:
			maxScroll := len(m.filteredItems) - 1
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollOffset < maxScroll {
				m.scrollOffset++
			}
			return m, nil
		}
	}

	// Delegate to textinput and re-filter
	var cmd tea.Cmd
	m.textinput, cmd = m.textinput.Update(msg)

	newFilter := m.textinput.Value()
	if newFilter != m.filter {
		m.filter = newFilter
		m.applyFilter()
	}

	return m, cmd
}

// View renders the help dialog within the given width and height.
func (m HelpModel) View(width, height int) string {
	if !m.active {
		return ""
	}

	boxWidth := width - 4
	if boxWidth > 80 {
		boxWidth = 80
	}
	if boxWidth < 30 {
		boxWidth = 30
	}

	// Title
	title := titleStyle.Render("Help")

	// Tab bar
	tabBar := m.renderTabBar()

	// Search input
	inputView := m.textinput.View()

	// Calculate visible list area
	listHeight := height - 8 // Reserve for title, tabs, input, status, borders
	if listHeight < 3 {
		listHeight = 3
	}
	if listHeight > 20 {
		listHeight = 20
	}

	// Render filtered items
	var listLines []string
	endIdx := m.scrollOffset + listHeight
	if endIdx > len(m.filteredItems) {
		endIdx = len(m.filteredItems)
	}
	for i := m.scrollOffset; i < endIdx; i++ {
		entry := m.filteredItems[i]
		name := entryNameStyle.Render(entry.Name)
		desc := entryDescStyle.Render(entry.Description)
		line := fmt.Sprintf("  %s  %s", name, desc)
		if i == m.scrollOffset {
			line = highlightStyle.Render(line)
		}
		listLines = append(listLines, line)
	}

	if len(listLines) == 0 {
		listLines = append(listLines, entryDescStyle.Render("  No matching entries"))
	}

	// Status bar
	status := statusStyle.Render(
		fmt.Sprintf(" %d items  Tab:switch  Esc:close", len(m.filteredItems)),
	)

	// Assemble
	var sb strings.Builder
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString(tabBar)
	sb.WriteString("\n")
	sb.WriteString(inputView)
	sb.WriteString("\n")
	sb.WriteString(strings.Join(listLines, "\n"))
	sb.WriteString("\n")
	sb.WriteString(status)

	return borderStyle.Width(boxWidth).Render(sb.String())
}

// renderTabBar renders the tab bar with the active tab highlighted.
func (m HelpModel) renderTabBar() string {
	var parts []string
	for i := 0; i < tabCount; i++ {
		tab := Tab(i)
		name := tabNames[tab]
		if tab == m.activeTab {
			parts = append(parts, activeTabStyle.Render("["+name+"]"))
		} else {
			parts = append(parts, tabStyle.Render(" "+name+" "))
		}
	}
	return strings.Join(parts, "")
}

// activeEntries returns the unfiltered entries for the current tab.
func (m HelpModel) activeEntries() []HelpEntry {
	switch m.activeTab {
	case TabCommands:
		return m.commands
	case TabKeybindings:
		return m.keybindings
	case TabTips:
		return m.tips
	default:
		return nil
	}
}

// SetFilter programmatically sets the search filter text. Useful for testing.
func (m *HelpModel) SetFilter(text string) {
	m.textinput.SetValue(text)
	m.filter = text
	m.applyFilter()
}

// FilteredCount returns the number of currently filtered items.
func (m *HelpModel) FilteredCount() int {
	return len(m.filteredItems)
}

// applyFilter filters the active tab's entries by case-insensitive substring match.
func (m *HelpModel) applyFilter() {
	entries := m.activeEntries()
	query := strings.ToLower(strings.TrimSpace(m.filter))

	if query == "" {
		m.filteredItems = make([]HelpEntry, len(entries))
		copy(m.filteredItems, entries)
	} else {
		m.filteredItems = m.filteredItems[:0]
		for _, entry := range entries {
			if strings.Contains(strings.ToLower(entry.Name), query) ||
				strings.Contains(strings.ToLower(entry.Description), query) ||
				strings.Contains(strings.ToLower(entry.Category), query) {
				m.filteredItems = append(m.filteredItems, entry)
			}
		}
	}

	// Reset scroll if out of range
	if m.scrollOffset >= len(m.filteredItems) {
		m.scrollOffset = 0
	}
}
