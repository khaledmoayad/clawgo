package app

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/khaledmoayad/clawgo/internal/api"
	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/cost"
	"github.com/khaledmoayad/clawgo/internal/permissions"
	"github.com/khaledmoayad/clawgo/internal/query"
	"github.com/khaledmoayad/clawgo/internal/session"
	"github.com/khaledmoayad/clawgo/internal/tools"
	"github.com/khaledmoayad/clawgo/internal/tui"
)

// REPLParams holds all parameters needed to launch the interactive REPL.
type REPLParams struct {
	Client               *api.Client
	Registry             *tools.Registry
	PermCtx              *permissions.PermissionContext
	CostTracker          *cost.Tracker
	Messages             []api.Message
	SystemPromptSections []string        // Multi-section system prompt (sent as separate content blocks)
	SystemPrompt         string          // Joined system prompt string (for commands, compact)
	StreamConfig         api.StreamConfig // API request augmentation (betas, thinking, headers)
	MaxTurns             int
	WorkingDir           string
	ProjectRoot          string
	SessionID            string
	Version              string
	Model                string
	CmdRegistry          *commands.CommandRegistry
	ToolRules            *permissions.ToolPermissionRules
	MCPManager           any // *mcp.Manager, typed as any to avoid circular imports
}

// LaunchREPL starts the interactive Bubble Tea REPL.
// It wires the TUI model to the query loop via callbacks and channels.
func LaunchREPL(ctx context.Context, params *REPLParams) error {
	permissionCh := make(chan permissions.PermissionResult, 1)
	history := session.NewHistory()
	sessionPath := session.GetSessionPath(params.ProjectRoot, params.SessionID)

	model := tui.New(tui.Config{
		Version:    params.Version,
		Model:      params.Model,
		WorkingDir: params.WorkingDir,
	})

	p := tea.NewProgram(model)

	// Wire OnSubmit: starts query loop in goroutine or dispatches slash commands
	model.OnSubmit = func(text string) tea.Cmd {
		return func() tea.Msg {
			history.Add(text)

			// Check for slash commands before sending to query loop
			if commands.IsCommand(text) && params.CmdRegistry != nil {
				return handleSlashCommand(text, params, p)
			}

			// Persist user message to session file
			_ = session.AppendEntry(sessionPath, session.EntryFromUserMessage(text))

			// Add user message to conversation
			params.Messages = append(params.Messages, api.UserMessage(text))

			// Run query loop in goroutine
			go func() {
				loopParams := &query.LoopParams{
					Client:               params.Client,
					Registry:             params.Registry,
					PermCtx:              params.PermCtx,
					CostTracker:          params.CostTracker,
					Messages:             params.Messages,
					SystemPromptSections: params.SystemPromptSections,
					SystemPrompt:         params.SystemPrompt,
					StreamConfig:         params.StreamConfig,
					MaxTurns:             params.MaxTurns,
					WorkingDir:           params.WorkingDir,
					ProjectRoot:          params.ProjectRoot,
					SessionID:            params.SessionID,
					CmdRegistry:          params.CmdRegistry,
					ToolRules:            params.ToolRules,
					MCPManager:           params.MCPManager,
					Program:              p,
					PermissionCh:         permissionCh,
				}
				err := query.RunLoop(ctx, loopParams)
				if err != nil {
					p.Send(tui.ErrorMsg{Err: err})
				} else {
					// Signal TUI that the query is complete
					p.Send(tui.StreamEventMsg{
						Event: api.StreamEvent{Type: api.EventMessageComplete},
					})
				}
				// Update params.Messages from loopParams (may have grown)
				params.Messages = loopParams.Messages
			}()

			return nil // actual events come via p.Send from goroutine
		}
	}

	// Wire OnPermission: sends result to permission channel
	model.OnPermission = func(resp tui.PermissionResponseMsg) tea.Cmd {
		return func() tea.Msg {
			result := permissions.Deny
			if resp.Approved {
				result = permissions.Allow
				if resp.Always {
					params.PermCtx.MarkAlwaysApproved(resp.ToolName)
				}
			}
			permissionCh <- result
			return nil
		}
	}

	// Wire OnShellCommand: executes shell commands from ! prefix input
	model.OnShellCommand = func(command string) tea.Cmd {
		return func() tea.Msg {
			return executeShellCommand(command, params.WorkingDir, p)
		}
	}

	// Register cleanup
	RegisterCleanup(func() {
		fmt.Println() // clean newline on exit
	})

	// Run the TUI
	_, err := p.Run()
	return err
}

// handleSlashCommand dispatches a slash command and returns an appropriate TUI message.
func handleSlashCommand(input string, params *REPLParams, p *tea.Program) tea.Msg {
	name, args := params.CmdRegistry.ParseCommandInput(input)

	cmd, ok := params.CmdRegistry.Find(name)
	if !ok {
		// Unknown command -- display error to user
		return tui.StreamEventMsg{
			Event: api.StreamEvent{
				Type: api.EventText,
				Text: fmt.Sprintf("Unknown command: /%s. Type /help for available commands.", name),
			},
		}
	}

	// Build command context
	cmdCtx := &commands.CommandContext{
		WorkingDir:   params.WorkingDir,
		Messages:     params.Messages,
		CostTracker:  params.CostTracker,
		Model:        params.Model,
		SessionID:    params.SessionID,
		Version:      params.Version,
		ToolRegistry: params.Registry,
		CmdRegistry:  params.CmdRegistry,
		SystemPrompt: params.SystemPrompt,
	}

	// Execute the command
	result, err := cmd.Execute(args, cmdCtx)
	if err != nil {
		return tui.StreamEventMsg{
			Event: api.StreamEvent{
				Type: api.EventText,
				Text: fmt.Sprintf("Command error: %v", err),
			},
		}
	}

	if result == nil {
		return tui.StreamEventMsg{
			Event: api.StreamEvent{Type: api.EventMessageComplete},
		}
	}

	// Handle result types
	switch result.Type {
	case "exit":
		// Request REPL exit
		p.Send(tea.Quit())
		return nil

	case "clear":
		// Reset conversation messages
		params.Messages = nil
		return tui.StreamEventMsg{
			Event: api.StreamEvent{
				Type: api.EventText,
				Text: "Conversation cleared.",
			},
		}

	case "model_change":
		// Update model on the client
		if result.Value != "" {
			params.Client.Model = result.Value
			params.Model = result.Value
		}
		return tui.StreamEventMsg{
			Event: api.StreamEvent{
				Type: api.EventText,
				Text: fmt.Sprintf("Model changed to: %s", result.Value),
			},
		}

	case "compact":
		// Compact is a Phase 3 feature -- stub for now
		return tui.StreamEventMsg{
			Event: api.StreamEvent{
				Type: api.EventText,
				Text: "Compact: conversation compaction will be available in a future update.",
			},
		}

	case "text":
		// Display text output then signal complete
		return tui.StreamEventMsg{
			Event: api.StreamEvent{
				Type: api.EventText,
				Text: result.Value,
			},
		}

	default:
		// For any other type, show the value
		if result.Value != "" {
			return tui.StreamEventMsg{
				Event: api.StreamEvent{
					Type: api.EventText,
					Text: result.Value,
				},
			}
		}
		return tui.StreamEventMsg{
			Event: api.StreamEvent{Type: api.EventMessageComplete},
		}
	}
}

// shellTimeout is the maximum duration for ! shell commands.
const shellTimeout = 30 * time.Second

// executeShellCommand runs a shell command from the ! prefix and sends
// the result back to the TUI as stream events. It runs synchronously
// and returns a message when complete.
func executeShellCommand(command, workDir string, p *tea.Program) tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), shellTimeout)
	defer cancel()

	if workDir == "" {
		workDir = "."
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Build combined output
	var output strings.Builder
	if stdout.Len() > 0 {
		output.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString(stderr.String())
	}

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// Send timeout error
			p.Send(tui.StreamEventMsg{
				Event: api.StreamEvent{
					Type: api.EventText,
					Text: fmt.Sprintf("Shell command timed out after %s", shellTimeout),
				},
			})
		} else {
			// Show output + error
			result := output.String()
			if result == "" {
				result = err.Error()
			}
			p.Send(tui.StreamEventMsg{
				Event: api.StreamEvent{
					Type: api.EventText,
					Text: result,
				},
			})
		}
	} else {
		result := output.String()
		if result == "" {
			result = "(no output)"
		}
		p.Send(tui.StreamEventMsg{
			Event: api.StreamEvent{
				Type: api.EventText,
				Text: result,
			},
		})
	}

	// Signal completion
	return tui.StreamEventMsg{
		Event: api.StreamEvent{Type: api.EventMessageComplete},
	}
}
