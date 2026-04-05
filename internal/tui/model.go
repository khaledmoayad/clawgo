package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/tui/diff"
	"github.com/khaledmoayad/clawgo/internal/tui/keybind"
)

// State represents the current TUI state.
type State int

const (
	StateInput      State = iota // Waiting for user input
	StateStreaming               // Receiving streaming response
	StatePermission              // Waiting for permission decision
	StateViewport                // Viewing large content in scrollable viewport
)

// viewportLargeThreshold is the line count above which content triggers viewport mode.
const viewportLargeThreshold = 50

// Config configures the TUI model.
type Config struct {
	Version     string
	Model       string // Current AI model name
	WorkingDir  string
	SessionID   string
	KeyBindings map[string]string // Custom key binding overrides from settings
	VimMode     bool              // Whether vim mode is enabled from settings

	// CmdRegistry is the slash command registry. If nil, slash commands are disabled.
	CmdRegistry *commands.CommandRegistry
}

// VimToggleMsg signals that vim mode should be toggled.
type VimToggleMsg struct{}

// Model is the root Bubble Tea model for the ClawGo REPL.
type Model struct {
	state      State
	prevState  State // State before entering viewport, restored on exit
	input      InputModel
	output     OutputModel
	spinner    SpinnerModel
	permission PermissionModel
	specPerm   SpecializedPermissionModel // Tool-specific permission dialogs
	ruleList   PermissionRuleListModel    // Permission rule management UI
	notifs     NotificationModel          // Toast notification system
	viewport   diff.ViewportModel
	keys       KeyMap
	keyConfig  keybind.KeyBindConfig
	vim        keybind.VimModel
	config     Config
	width      int
	height     int

	// Command infrastructure
	cmdRegistry *commands.CommandRegistry

	// Callback functions for query loop integration (set by Plan 07).
	OnSubmit      func(text string) tea.Cmd               // Called when user submits input
	OnPermission  func(resp PermissionResponseMsg) tea.Cmd // Called on permission response
	OnCompact     func() tea.Cmd                           // Called when /compact is executed
	OnModelChange func(model string) tea.Cmd               // Called when /model changes the model
}

// New creates a new TUI model.
func New(cfg Config) Model {
	keyConfig, err := keybind.LoadKeyBindings(cfg.KeyBindings)
	if err != nil {
		// Fall back to defaults on invalid config
		keyConfig = keybind.DefaultBindings()
	}

	vim := keybind.NewVimModel()
	if cfg.VimMode {
		vim.SetEnabled(true)
	}

	inputModel := NewInputModel()

	// Wire command names for tab completion
	if cfg.CmdRegistry != nil {
		names := make([]string, 0)
		for _, cmd := range cfg.CmdRegistry.All() {
			names = append(names, cmd.Name())
		}
		inputModel.SetCommandNames(names)
	}

	return Model{
		state:       StateInput,
		input:       inputModel,
		output:      NewOutputModel(),
		spinner:     NewSpinnerModel(),
		permission:  NewPermissionModel(),
		specPerm:    NewSpecializedPermissionModel(),
		ruleList:    NewPermissionRuleListModel(),
		notifs:      NewNotificationModel(),
		viewport:    diff.NewViewportModel(80, 24),
		keys:        DefaultKeyMap(),
		keyConfig:   keyConfig,
		vim:         vim,
		config:      cfg,
		cmdRegistry: cfg.CmdRegistry,
	}
}

// Init returns the initial command for the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.input.Focus(), m.spinner.spinner.Tick)
}

// Update processes messages and returns the updated model and commands.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		k := msg.Key()
		if m.keys.IsQuit(k) {
			return m, tea.Quit
		}
		// In viewport mode, q or Escape exits back to previous state
		if m.state == StateViewport {
			if k.Code == 'q' || m.keys.IsEscape(k) {
				m.state = m.prevState
				if m.state == StateInput {
					return m, m.input.Focus()
				}
				return m, nil
			}
			// All other keys delegated to viewport below
		}

		// Vim mode key handling: process before state-specific delegation
		if m.vim.IsEnabled() {
			action, consumed := m.vim.HandleKey(k)
			if consumed {
				// Handle scroll actions from vim navigation
				switch action {
				case keybind.ActionScrollUp, keybind.ActionScrollDown:
					if m.state == StateViewport {
						var cmd tea.Cmd
						m.viewport, cmd = m.viewport.Update(msg)
						cmds = append(cmds, cmd)
					}
				}
				// Sync vim normal/insert state with input model
				m.input.SetVimNormal(m.vim.IsNormal())
				return m, tea.Batch(cmds...)
			}
			// If vim is in Normal mode and state is input, block key forwarding to textarea
			if m.vim.IsNormal() && m.state == StateInput {
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.output.SetWidth(msg.Width)
		m.permission.SetWidth(msg.Width)
		m.specPerm.SetWidth(msg.Width)
		m.ruleList.SetWidth(msg.Width)
		m.notifs.SetWidth(msg.Width)
		// Reserve 4 lines for header + status bar
		m.viewport.SetSize(msg.Width, max(1, msg.Height-4))
		return m, nil

	case VimToggleMsg:
		m.vim.Toggle()
		m.input.SetVimNormal(m.vim.IsNormal())
		return m, nil

	case SubmitMsg:
		// Check if input is a slash command
		if m.cmdRegistry != nil && commands.IsCommand(msg.Text) {
			return m.handleSlashCommand(msg.Text)
		}

		// Normal user input: send to API
		m.output.AddMessage(DisplayMessage{Role: "user", Content: msg.Text})
		m.input.Reset()
		m.state = StateStreaming
		cmd := m.spinner.Start("Thinking")
		cmds = append(cmds, cmd)
		if m.OnSubmit != nil {
			cmds = append(cmds, m.OnSubmit(msg.Text))
		}
		return m, tea.Batch(cmds...)

	case CommandResultMsg:
		return m.handleCommandResult(msg)

	case StreamEventMsg:
		return m.handleStreamEvent(msg)

	case PermissionRequestMsg:
		m.state = StatePermission
		m.spinner.Stop()
		m.permission.Show(msg.ToolName, msg.ToolInput, msg.Description)
		return m, nil

	case DetailedPermissionRequestMsg:
		m.state = StatePermission
		m.spinner.Stop()
		m.specPerm.ShowDetailed(msg.Details)
		return m, nil

	case NotificationMsg:
		cmd := m.notifs.Add(msg.Notification)
		return m, cmd

	case notificationDismissMsg:
		var cmd tea.Cmd
		m.notifs, cmd = m.notifs.Update(msg)
		return m, cmd

	case ShowPermissionRulesMsg:
		m.ruleList.SetRules(msg.Rules)
		m.ruleList.Show()
		return m, nil

	case PermissionRuleRemoveMsg:
		// Rule was removed from the UI; could be forwarded to settings
		return m, nil

	case PermissionResponseMsg:
		m.permission.Hide()
		m.specPerm.Hide()
		if msg.Approved {
			m.state = StateStreaming
			cmd := m.spinner.Start("Running " + msg.ToolName)
			cmds = append(cmds, cmd)
		} else {
			m.state = StateInput
			cmds = append(cmds, m.input.Focus())
		}
		if m.OnPermission != nil {
			cmds = append(cmds, m.OnPermission(msg))
		}
		return m, tea.Batch(cmds...)

	case DiffDisplayMsg:
		// Add the diff as a tool_result message with diff flag set
		m.output.AddMessage(DisplayMessage{
			Role:        "tool_result",
			Content:     msg.Content,
			ToolName:    msg.ToolName,
			DiffContent: true,
		})
		// If content is large, enter viewport mode for scrolling
		rendered := diff.RenderDiff(msg.Content, m.width)
		lineCount := strings.Count(rendered, "\n") + 1
		if m.viewport.NeedsViewport(lineCount) || lineCount > viewportLargeThreshold {
			m.prevState = m.state
			m.state = StateViewport
			m.spinner.Stop()
			m.viewport.SetContent(rendered)
		}
		return m, nil

	case CostUpdateMsg:
		// Store cost info for status line rendering (future use)
		return m, nil

	case ErrorMsg:
		m.output.AddMessage(DisplayMessage{Role: "error", Content: msg.Err.Error()})
		m.state = StateInput
		m.spinner.Stop()
		cmds = append(cmds, m.input.Focus())
		return m, tea.Batch(cmds...)
	}

	// Rule list overlay intercepts keys when active
	if m.ruleList.IsActive() {
		var cmd tea.Cmd
		m.ruleList, cmd = m.ruleList.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	// Delegate to active sub-model
	switch m.state {
	case StateInput:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	case StateStreaming:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	case StatePermission:
		// Delegate to whichever permission model is active
		if m.specPerm.IsActive() {
			var cmd tea.Cmd
			m.specPerm.PermissionModel, cmd = m.specPerm.PermissionModel.Update(msg)
			cmds = append(cmds, cmd)
		} else {
			var cmd tea.Cmd
			m.permission, cmd = m.permission.Update(msg)
			cmds = append(cmds, cmd)
		}
	case StateViewport:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// handleStreamEvent processes a streaming event from the API.
func (m Model) handleStreamEvent(msg StreamEventMsg) (tea.Model, tea.Cmd) {
	evt := msg.Event
	switch evt.Type {
	case "text":
		if !m.output.isStreaming {
			m.output.StartStreaming()
			m.spinner.Stop()
		}
		m.output.AppendStreaming(evt.Text)
	case "thinking":
		m.output.AddMessage(DisplayMessage{Role: "thinking", Content: evt.Text})
	case "tool_use_start":
		m.output.FinishStreaming()
		if evt.ToolUse != nil {
			m.output.AddMessage(DisplayMessage{
				Role:     "tool_use",
				ToolName: evt.ToolUse.Name,
				Content:  string(evt.ToolUse.Input),
			})
		}
	case "message_complete":
		m.output.FinishStreaming()
		m.state = StateInput
		m.spinner.Stop()
		return m, m.input.Focus()
	case "error":
		m.output.FinishStreaming()
		errText := "API error"
		if evt.Error != nil {
			errText = evt.Error.Error()
		}
		m.output.AddMessage(DisplayMessage{Role: "error", Content: errText})
		m.state = StateInput
		m.spinner.Stop()
		return m, m.input.Focus()
	}
	return m, nil
}

// handleSlashCommand intercepts slash command input, parses it, and executes it
// via the command registry. Returns the result as a CommandResultMsg.
func (m Model) handleSlashCommand(text string) (tea.Model, tea.Cmd) {
	m.input.Reset()

	name, args := m.cmdRegistry.ParseCommandInput(text)

	cmd, found := m.cmdRegistry.Find(name)
	if !found {
		m.output.AddMessage(DisplayMessage{
			Role:    "error",
			Content: "Unknown command: /" + name + ". Type /help for available commands.",
		})
		return m, m.input.Focus()
	}

	// Build the command context from current TUI state
	cmdCtx := &commands.CommandContext{
		WorkingDir:  m.config.WorkingDir,
		Model:       m.config.Model,
		SessionID:   m.config.SessionID,
		Version:     m.config.Version,
		CmdRegistry: m.cmdRegistry,
	}

	result, err := cmd.Execute(args, cmdCtx)
	if err != nil {
		m.output.AddMessage(DisplayMessage{
			Role:    "error",
			Content: "Command error: " + err.Error(),
		})
		return m, m.input.Focus()
	}

	// Return the command result as a message for the update loop to handle
	return m, func() tea.Msg {
		return CommandResultMsg{
			Type:    result.Type,
			Value:   result.Value,
			Command: name,
		}
	}
}

// handleCommandResult processes the result of a slash command execution.
func (m Model) handleCommandResult(msg CommandResultMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case "text":
		m.output.AddMessage(DisplayMessage{
			Role:    "command",
			Content: msg.Value,
		})
		return m, m.input.Focus()

	case "clear":
		m.output.Clear()
		return m, m.input.Focus()

	case "compact":
		if m.OnCompact != nil {
			return m, tea.Batch(m.OnCompact(), m.input.Focus())
		}
		m.output.AddMessage(DisplayMessage{
			Role:    "command",
			Content: "Compacting conversation context...",
		})
		return m, m.input.Focus()

	case "model_change":
		m.config.Model = msg.Value
		if m.OnModelChange != nil {
			return m, tea.Batch(m.OnModelChange(msg.Value), m.input.Focus())
		}
		m.output.AddMessage(DisplayMessage{
			Role:    "command",
			Content: "Model changed to: " + msg.Value,
		})
		return m, m.input.Focus()

	case "exit":
		return m, tea.Quit

	case "skip", "rewind":
		// No-op for result types that are handled elsewhere
		return m, m.input.Focus()

	default:
		// Unknown result type, display as text
		if msg.Value != "" {
			m.output.AddMessage(DisplayMessage{
				Role:    "command",
				Content: msg.Value,
			})
		}
		return m, m.input.Focus()
	}
}

// View renders the complete TUI as a tea.View.
func (m Model) View() tea.View {
	return tea.NewView(m.viewString())
}

// viewString builds the raw string content for the view.
func (m Model) viewString() string {
	var sb strings.Builder

	// Header with optional vim mode indicator
	header := "ClawGo " + m.config.Version + " (" + m.config.Model + ")"
	if m.vim.IsEnabled() {
		header += " [" + m.vim.ModeString() + "]"
	}
	sb.WriteString(DimStyle.Render(header))
	sb.WriteString("\n\n")

	// Messages
	sb.WriteString(m.output.View())

	// State-specific content
	switch m.state {
	case StateInput:
		sb.WriteString("\n")
		sb.WriteString(m.input.View())
	case StateStreaming:
		if m.spinner.IsActive() {
			sb.WriteString("\n")
			sb.WriteString(m.spinner.View())
		}
	case StatePermission:
		// Prefer specialized permission dialog when active
		if m.specPerm.IsActive() {
			sb.WriteString(m.specPerm.View())
		} else {
			sb.WriteString(m.permission.View())
		}
	case StateViewport:
		sb.WriteString(m.viewport.View())
		sb.WriteString("\n")
		sb.WriteString(DimStyle.Render("  Scroll: arrows/j/k  Exit: q/Esc"))
	}

	// Permission rules overlay
	if m.ruleList.IsActive() {
		sb.WriteString(m.ruleList.View())
	}

	// Toast notifications (rendered at the bottom, above input)
	if notifView := m.notifs.View(); notifView != "" {
		sb.WriteString("\n")
		sb.WriteString(notifView)
	}

	return sb.String()
}

// ViewContent returns the rendered view as a string (for testing).
func (m Model) ViewContent() string { return m.viewString() }

// CurrentState returns the current TUI state (for testing).
func (m Model) CurrentState() State { return m.state }

// Notifications returns the notification model for external access.
func (m *Model) Notifications() *NotificationModel { return &m.notifs }

// SpecializedPermission returns the specialized permission model for external access.
func (m *Model) SpecializedPermission() *SpecializedPermissionModel { return &m.specPerm }

// PermissionRules returns the permission rule list model for external access.
func (m *Model) PermissionRules() *PermissionRuleListModel { return &m.ruleList }
