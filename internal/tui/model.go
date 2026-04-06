package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	tea "charm.land/bubbletea/v2"

	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/tui/diff"
	"github.com/khaledmoayad/clawgo/internal/tui/help"
	"github.com/khaledmoayad/clawgo/internal/tui/keybind"
	"github.com/khaledmoayad/clawgo/internal/tui/overlay"
	"github.com/khaledmoayad/clawgo/internal/tui/render"
	"github.com/khaledmoayad/clawgo/internal/tui/renderers"
	"github.com/khaledmoayad/clawgo/internal/tui/scroll"
	"github.com/khaledmoayad/clawgo/internal/tui/suggest"
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
	state     State
	prevState State // State before entering viewport, restored on exit
	input     InputModel
	output    OutputModel
	spinner   SpinnerModel

	// Permission subsystems
	permission PermissionModel
	specPerm   SpecializedPermissionModel // Tool-specific permission dialogs
	ruleList   PermissionRuleListModel    // Permission rule management UI

	// Wave 1: Integrated TUI subsystems
	virtualScroll *scroll.VirtualScrollViewport
	statusLine    StatusLineModel
	overlayMgr    *overlay.OverlayManager
	helpDialog    help.HelpModel
	suggestions   suggest.SuggestModel
	registry      *renderers.RendererRegistry
	notifs        NotificationModel // Toast notification system

	// Legacy viewport for diff display (kept for backward compat)
	viewport diff.ViewportModel

	// Core
	keys      KeyMap
	keyConfig keybind.KeyBindConfig
	vim       keybind.VimModel
	config    Config
	width     int
	height    int

	// Command infrastructure
	cmdRegistry *commands.CommandRegistry

	// Callback functions for query loop integration.
	OnSubmit       func(text string) tea.Cmd               // Called when user submits input
	OnPermission   func(resp PermissionResponseMsg) tea.Cmd // Called on permission response
	OnCompact      func() tea.Cmd                           // Called when /compact is executed
	OnModelChange  func(model string) tea.Cmd               // Called when /model changes the model
	OnShellCommand func(cmd string) tea.Cmd                 // Called when user submits a !shell command
	OnCommand      func(name, args string) tea.Cmd          // Called when a slash command is detected
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

	// Build command names for tab completion and help dialog
	var cmdNames []string
	var helpCommands []help.HelpEntry
	if cfg.CmdRegistry != nil {
		for _, cmd := range cfg.CmdRegistry.All() {
			cmdNames = append(cmdNames, cmd.Name())
			helpCommands = append(helpCommands, help.HelpEntry{
				Name:        "/" + cmd.Name(),
				Description: cmd.Description(),
				Category:    "command",
			})
		}
		inputModel.SetCommandNames(cmdNames)
	}

	// Build keybinding entries for help dialog
	helpKeybindings := []help.HelpEntry{
		{Name: "Ctrl+C", Description: "Quit", Category: "general"},
		{Name: "Ctrl+K", Description: "Jump to message (selector)", Category: "navigation"},
		{Name: "Ctrl+R", Description: "Search history", Category: "navigation"},
		{Name: "Ctrl+F", Description: "Search transcript", Category: "navigation"},
		{Name: "Ctrl+O", Description: "Fullscreen current message", Category: "navigation"},
		{Name: "?", Description: "Show help (non-input mode)", Category: "general"},
		{Name: "Escape", Description: "Dismiss overlay / cancel", Category: "general"},
		{Name: "Tab", Description: "Accept suggestion / next item", Category: "input"},
	}

	// Initialize renderer registry for virtual scroll and output rendering
	reg := renderers.NewRegistry()

	// Initialize virtual scroll with render function that dispatches through registry
	renderFn := func(msg scroll.DisplayMessage, width int) string {
		return reg.Render(renderers.DisplayMessage{
			Type:        mapRoleToType(msg.Role, msg.ToolName, ""),
			Role:        msg.Role,
			Content:     msg.Content,
			ToolName:    msg.ToolName,
			DiffContent: msg.DiffContent,
		}, width)
	}
	vs := scroll.New(80, 20, renderFn)

	// Initialize suggestion providers
	var providers []suggest.SuggestionProvider
	if len(cmdNames) > 0 {
		providers = append(providers, &suggest.CommandProvider{Commands: cmdNames})
	}
	providers = append(providers, &suggest.FilePathProvider{})

	// Status line initialized with model name from config
	statusLine := NewStatusLineModel()
	statusLine.SetModel(cfg.Model)
	statusLine.SetWidth(80)
	if cfg.VimMode {
		statusLine.SetVimMode("NORMAL")
	}

	return Model{
		state:         StateInput,
		input:         inputModel,
		output:        NewOutputModelWithRegistry(reg),
		spinner:       NewSpinnerModel(),
		permission:    NewPermissionModel(),
		specPerm:      NewSpecializedPermissionModel(),
		ruleList:      NewPermissionRuleListModel(),
		notifs:        NewNotificationModel(),
		virtualScroll: vs,
		statusLine:    statusLine,
		overlayMgr:    overlay.NewOverlayManager(),
		helpDialog:    help.NewHelpModel(helpCommands, helpKeybindings),
		suggestions:   suggest.NewSuggestModel(providers...),
		registry:      reg,
		viewport:      diff.NewViewportModel(80, 24),
		keys:          DefaultKeyMap(),
		keyConfig:     keyConfig,
		vim:           vim,
		config:        cfg,
		cmdRegistry:   cfg.CmdRegistry,
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

		// Global quit (Ctrl+C) always works
		if m.keys.IsQuit(k) {
			return m, tea.Quit
		}

		// Priority 1: Overlay manager eats all keys when active
		if m.overlayMgr.IsActive() {
			cmd := m.overlayMgr.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		// Priority 2: Help dialog eats all keys when active
		if m.helpDialog.IsActive() {
			var cmd tea.Cmd
			m.helpDialog, cmd = m.helpDialog.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		// Priority 3: Suggestion selection when active
		if m.suggestions.IsActive() {
			if k.Code == tea.KeyTab || k.Code == tea.KeyEnter ||
				k.Code == tea.KeyUp || k.Code == tea.KeyDown || k.Code == tea.KeyEscape {
				var cmd tea.Cmd
				m.suggestions, cmd = m.suggestions.Update(msg)
				if cmd != nil {
					cmds = append(cmds, cmd)
				}
				return m, tea.Batch(cmds...)
			}
		}

		// Priority 4: Rule list overlay intercepts keys when active
		if m.ruleList.IsActive() {
			var cmd tea.Cmd
			m.ruleList, cmd = m.ruleList.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
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

		// Overlay hotkeys (only when not in permission or viewport state)
		if m.state == StateInput || m.state == StateStreaming {
			switch {
			case k.Code == 'k' && k.Mod&tea.ModCtrl != 0:
				// Ctrl+K: Message selector overlay
				msgs := m.toOverlayMessages()
				sel := overlay.NewMessageSelector(msgs)
				m.overlayMgr.Push(sel)
				return m, nil

			case k.Code == 'r' && k.Mod&tea.ModCtrl != 0:
				// Ctrl+R: History search overlay
				hist := overlay.NewHistorySearch(nil) // Empty history; populated externally
				m.overlayMgr.Push(hist)
				return m, nil

			case k.Code == 'f' && k.Mod&tea.ModCtrl != 0:
				// Ctrl+F: Transcript search overlay
				msgs := m.toOverlayMessages()
				search := overlay.NewTranscriptSearch(msgs)
				m.overlayMgr.Push(search)
				return m, nil

			case k.Code == 'o' && k.Mod&tea.ModCtrl != 0:
				// Ctrl+O: Fullscreen overlay with current output
				content := m.output.View()
				fs := overlay.NewFullscreen("Conversation", content, m.width, m.height)
				m.overlayMgr.Push(fs)
				return m, nil
			}
		}

		// ? key shows help in non-input states (or when input is empty)
		if k.Code == '?' && m.state != StatePermission {
			if m.state != StateInput || m.input.Value() == "" {
				cmd := m.helpDialog.Show()
				return m, cmd
			}
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
				// Sync vim normal/insert state with input model and status line
				m.input.SetVimNormal(m.vim.IsNormal())
				if m.vim.IsEnabled() {
					m.statusLine.SetVimMode(m.vim.ModeString())
				}
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
		m.statusLine.SetWidth(msg.Width)
		m.suggestions.SetWidth(msg.Width)
		// Reserve 4 lines for header + status bar
		viewH := max(1, msg.Height-4)
		m.viewport.SetSize(msg.Width, viewH)
		m.virtualScroll.SetSize(msg.Width, viewH)
		return m, nil

	case VimToggleMsg:
		m.vim.Toggle()
		m.input.SetVimNormal(m.vim.IsNormal())
		if m.vim.IsEnabled() {
			m.statusLine.SetVimMode(m.vim.ModeString())
		} else {
			m.statusLine.SetVimMode("")
		}
		return m, nil

	case ShellCommandMsg:
		// Shell command via ! prefix: display as user input and execute
		m.output.AddMessage(DisplayMessage{Role: "user", Content: "$ " + msg.Command})
		m.syncMessagesToScroll()
		m.input.Reset()
		m.state = StateStreaming
		cmd := m.spinner.Start("Running shell command")
		cmds = append(cmds, cmd)
		if m.OnShellCommand != nil {
			cmds = append(cmds, m.OnShellCommand(msg.Command))
		}
		return m, tea.Batch(cmds...)

	case SubmitMsg:
		// Check if input is a slash command
		if m.cmdRegistry != nil && commands.IsCommand(msg.Text) {
			return m.handleSlashCommand(msg.Text)
		}

		// Normal user input: send to API
		m.output.AddMessage(DisplayMessage{Role: "user", Content: msg.Text})
		m.syncMessagesToScroll()
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
		m.syncMessagesToScroll()
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
		// Update status line with cost info
		m.statusLine.SetCost(msg.SessionCost)
		return m, nil

	case overlay.OverlayResultMsg:
		// Handle overlay results (message selection, search navigation, etc.)
		return m.handleOverlayResult(msg)

	case help.DismissHelpMsg:
		// Help was dismissed, return focus to input
		if m.state == StateInput {
			return m, m.input.Focus()
		}
		return m, nil

	case suggest.SuggestionsReadyMsg:
		m.suggestions, _ = m.suggestions.Update(msg)
		return m, nil

	case suggest.SuggestionAcceptMsg:
		// Inject accepted suggestion text into input
		m.input.InsertText(msg.Text)
		return m, nil

	// Wave 1: New message types
	case ShowOverlayMsg:
		return m.handleShowOverlay(msg)

	case ShowHelpMsg:
		cmd := m.helpDialog.Show()
		return m, cmd

	case SuggestionUpdateMsg:
		cmd := m.suggestions.OnInputChange(msg.Input, msg.CursorPos)
		return m, cmd

	case ContextUpdateMsg:
		m.statusLine.SetContext(msg.Percent, msg.Tokens)
		return m, nil

	case ModelChangeMsg:
		m.config.Model = msg.Name
		m.statusLine.SetModel(msg.Name)
		return m, nil

	case ToastMsg:
		notif := Notification{
			Text:     msg.Message,
			Color:    toastLevelColor(msg.Level),
			Priority: PriorityMedium,
		}
		cmd := m.notifs.Add(notif)
		return m, cmd

	case ErrorMsg:
		m.output.AddMessage(DisplayMessage{Role: "error", Content: msg.Err.Error()})
		m.syncMessagesToScroll()
		m.state = StateInput
		m.spinner.Stop()
		cmds = append(cmds, m.input.Focus())
		return m, tea.Batch(cmds...)

	case APIErrorMsg:
		m.output.FinishStreaming()
		m.syncMessagesToScroll()
		errText := "API error"
		if msg.ErrorInfo != nil {
			errText = msg.ErrorInfo.UserMessage
		}
		m.output.AddMessage(DisplayMessage{Role: "error", Content: errText})
		m.syncMessagesToScroll()
		m.state = StateInput
		m.spinner.Stop()
		return m, m.input.Focus()
	}

	// Delegate to active sub-model; also forward scroll keys to virtual scroll
	// when not in permission/viewport states so users can scroll history.
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		k := keyMsg.Key()
		if m.state == StateInput || m.state == StateStreaming {
			switch k.Code {
			case tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd:
				m.virtualScroll.Update(msg)
			}
		}
	}

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
		m.syncMessagesToScroll()
	case "tool_use_start":
		m.output.FinishStreaming()
		m.syncMessagesToScroll()
		if evt.ToolUse != nil {
			m.output.AddMessage(DisplayMessage{
				Role:     "tool_use",
				ToolName: evt.ToolUse.Name,
				Content:  string(evt.ToolUse.Input),
			})
			m.syncMessagesToScroll()
		}
	case "message_complete":
		m.output.FinishStreaming()
		m.syncMessagesToScroll()
		m.state = StateInput
		m.spinner.Stop()
		return m, m.input.Focus()
	case "error":
		m.output.FinishStreaming()
		m.syncMessagesToScroll()
		errText := "API error"
		if evt.Error != nil {
			errText = evt.Error.Error()
		}
		m.output.AddMessage(DisplayMessage{Role: "error", Content: errText})
		m.syncMessagesToScroll()
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

	_, found := m.cmdRegistry.Find(name)
	if !found {
		m.output.AddMessage(DisplayMessage{
			Role:    "error",
			Content: "Unknown command: /" + name + ". Type /help for available commands.",
		})
		m.syncMessagesToScroll()
		return m, m.input.Focus()
	}

	// Delegate to OnCommand callback for full-context execution
	if m.OnCommand != nil {
		return m, m.OnCommand(name, args)
	}

	// Fallback: build limited local context when no callback is wired
	cmdCtx := &commands.CommandContext{
		WorkingDir:  m.config.WorkingDir,
		Model:       m.config.Model,
		SessionID:   m.config.SessionID,
		Version:     m.config.Version,
		CmdRegistry: m.cmdRegistry,
	}

	cmd, _ := m.cmdRegistry.Find(name)
	result, err := cmd.Execute(args, cmdCtx)
	if err != nil {
		m.output.AddMessage(DisplayMessage{
			Role:    "error",
			Content: "Command error: " + err.Error(),
		})
		m.syncMessagesToScroll()
		return m, m.input.Focus()
	}

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
		m.syncMessagesToScroll()
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
		m.syncMessagesToScroll()
		return m, m.input.Focus()

	case "model_change":
		m.config.Model = msg.Value
		m.statusLine.SetModel(msg.Value)
		if m.OnModelChange != nil {
			return m, tea.Batch(m.OnModelChange(msg.Value), m.input.Focus())
		}
		m.output.AddMessage(DisplayMessage{
			Role:    "command",
			Content: "Model changed to: " + msg.Value,
		})
		m.syncMessagesToScroll()
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
			m.syncMessagesToScroll()
		}
		return m, m.input.Focus()
	}
}

// handleShowOverlay handles ShowOverlayMsg by pushing the appropriate overlay.
func (m Model) handleShowOverlay(msg ShowOverlayMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case "selector":
		msgs := m.toOverlayMessages()
		sel := overlay.NewMessageSelector(msgs)
		m.overlayMgr.Push(sel)
	case "history":
		hist := overlay.NewHistorySearch(nil)
		m.overlayMgr.Push(hist)
	case "search":
		msgs := m.toOverlayMessages()
		search := overlay.NewTranscriptSearch(msgs)
		m.overlayMgr.Push(search)
	case "fullscreen":
		content := m.output.View()
		fs := overlay.NewFullscreen("Conversation", content, m.width, m.height)
		m.overlayMgr.Push(fs)
	}
	return m, nil
}

// handleOverlayResult processes results from overlay dismissals.
func (m Model) handleOverlayResult(msg overlay.OverlayResultMsg) (tea.Model, tea.Cmd) {
	switch msg.Source {
	case "selector":
		// Message selected -- could scroll to it in the future
		if msg.Result.Action == "select" && msg.Result.Index >= 0 {
			// Scroll virtual viewport to show the selected message
			// For now, just dismiss and return to input
		}
	case "search":
		// Transcript search result
		if msg.Result.Action == "select" && msg.Result.Index >= 0 {
			// Could scroll to match location
		}
	case "history":
		// History search selected a past prompt -- inject into input
		if msg.Result.Action == "select" && msg.Result.Value != "" {
			m.input.InsertText(msg.Result.Value)
		}
	}
	if m.state == StateInput {
		return m, m.input.Focus()
	}
	return m, nil
}

// View renders the complete TUI as a tea.View.
func (m Model) View() tea.View {
	return tea.NewView(m.viewString())
}

// viewString builds the raw string content for the view -- layered rendering.
func (m Model) viewString() string {
	var sb strings.Builder

	// 1. Header with optional vim mode indicator
	header := "ClawGo " + m.config.Version + " (" + m.config.Model + ")"
	if m.vim.IsEnabled() {
		header += " [" + m.vim.ModeString() + "]"
	}
	sb.WriteString(DimStyle.Render(header))
	sb.WriteString("\n\n")

	// 2. Message area via virtualScroll.View() (uses renderer registry)
	sb.WriteString(m.virtualScroll.View())

	// Also render streaming text if active (not yet in messages list).
	// To avoid broken rendering from incomplete markdown (e.g. an unclosed
	// code fence), only pass complete lines (up to the last newline) through
	// the markdown renderer; any partial trailing line is appended as raw text.
	if m.output.isStreaming && m.output.streamingText.Len() > 0 {
		sb.WriteString("\n")
		sb.WriteString(RoleLabel("assistant"))
		sb.WriteString("\n")
		streamRaw := m.output.streamingText.String()
		lastNL := strings.LastIndex(streamRaw, "\n")
		var streamRendered string
		if lastNL >= 0 {
			complete := streamRaw[:lastNL+1]
			partial := streamRaw[lastNL+1:]
			rendered, err := render.RenderMarkdown(complete, m.width)
			if err != nil {
				streamRendered = MessagePadding.Render(streamRaw)
			} else {
				streamRendered = rendered
				if partial != "" {
					streamRendered += MessagePadding.Render(partial)
				}
			}
		} else {
			// No newline yet — show partial line as raw text.
			streamRendered = MessagePadding.Render(streamRaw)
		}
		sb.WriteString(streamRendered)
		sb.WriteString("\n")
	}

	// 3. State-specific content
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

	// 4. Suggestions dropdown (if active, positioned below input)
	if sugView := m.suggestions.View(); sugView != "" {
		sb.WriteString("\n")
		sb.WriteString(sugView)
	}

	// 5. Status line at bottom
	sb.WriteString("\n")
	sb.WriteString(m.statusLine.View())

	// 6. Toast notifications (right-aligned, above status line conceptually)
	if notifView := m.notifs.View(); notifView != "" {
		sb.WriteString("\n")
		sb.WriteString(notifView)
	}

	// 7. Permission rules overlay
	if m.ruleList.IsActive() {
		sb.WriteString(m.ruleList.View())
	}

	base := sb.String()

	// 8. Overlay (if active) — composited ON TOP of base content via lipgloss.Place.
	// This prevents the overlay from appending below and scrolling the terminal.
	if m.overlayMgr.IsActive() && m.width > 0 && m.height > 0 {
		overlayContent := m.overlayMgr.View(m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlayContent,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#3E4451"))),
		)
	}

	// 9. Help dialog (if active) — composited on top of base content.
	if m.helpDialog.IsActive() && m.width > 0 && m.height > 0 {
		helpContent := m.helpDialog.View(m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpContent,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#3E4451"))),
		)
	}

	return base
}

// syncMessagesToScroll converts output messages to scroll-package format
// and updates the virtual scroll viewport.
func (m *Model) syncMessagesToScroll() {
	outMsgs := m.output.Messages()
	scrollMsgs := make([]scroll.DisplayMessage, len(outMsgs))
	for i, msg := range outMsgs {
		scrollMsgs[i] = scroll.DisplayMessage{
			Role:        msg.Role,
			Content:     msg.Content,
			ToolName:    msg.ToolName,
			DiffContent: msg.DiffContent,
		}
	}
	m.virtualScroll.SetMessages(scrollMsgs)
	m.virtualScroll.OnNewMessage()
}

// toOverlayMessages converts output messages to overlay-package format.
func (m Model) toOverlayMessages() []overlay.DisplayMessage {
	outMsgs := m.output.Messages()
	result := make([]overlay.DisplayMessage, len(outMsgs))
	for i, msg := range outMsgs {
		result[i] = overlay.DisplayMessage{
			Role:     msg.Role,
			Content:  msg.Content,
			ToolName: msg.ToolName,
		}
	}
	return result
}

// mapRoleToType maps a DisplayMessage role to a renderer registry type key.
// Falls back to the role itself if no specific mapping exists.
func mapRoleToType(role, toolName, status string) string {
	switch role {
	case "user":
		return "user_text"
	case "assistant":
		return "assistant_text"
	case "thinking":
		return "assistant_thinking"
	case "tool_use":
		return "assistant_tool_use"
	case "tool_result":
		switch status {
		case "error":
			return "tool_error"
		case "rejected":
			return "tool_rejected"
		case "canceled":
			return "tool_canceled"
		case "success":
			return "tool_success"
		default:
			return "tool_result"
		}
	case "error":
		return "system_api_error"
	case "command":
		return "user_command"
	default:
		return role
	}
}

// toastLevelColor returns a color for the given toast level string.
func toastLevelColor(level string) color.Color {
	switch level {
	case "error":
		return color.RGBA{R: 224, G: 108, B: 117, A: 255} // Red
	case "warning":
		return color.RGBA{R: 229, G: 192, B: 123, A: 255} // Amber
	default:
		return nil // Default color for info
	}
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

// VirtualScroll returns the virtual scroll viewport for external access.
func (m *Model) VirtualScroll() *scroll.VirtualScrollViewport { return m.virtualScroll }

// StatusLine returns the status line model for external access.
func (m *Model) StatusLine() *StatusLineModel { return &m.statusLine }

// OverlayManager returns the overlay manager for external access.
func (m *Model) OverlayManager() *overlay.OverlayManager { return m.overlayMgr }

// HelpDialog returns the help dialog model for external access.
func (m *Model) HelpDialog() *help.HelpModel { return &m.helpDialog }

// Suggestions returns the suggestion model for external access.
func (m *Model) Suggestions() *suggest.SuggestModel { return &m.suggestions }

// Registry returns the renderer registry for external access.
func (m *Model) Registry() *renderers.RendererRegistry { return m.registry }
