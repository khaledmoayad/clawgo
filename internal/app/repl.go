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
		Version:     params.Version,
		Model:       params.Model,
		WorkingDir:  params.WorkingDir,
		CmdRegistry: params.CmdRegistry,
	})

	p := tea.NewProgram(model)

	// Wire OnSubmit: starts query loop in goroutine (slash commands are intercepted by TUI)
	model.OnSubmit = func(text string) tea.Cmd {
		return func() tea.Msg {
			history.Add(text)

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

	// Wire OnCommand: executes slash commands with full REPL context
	model.OnCommand = func(name, args string) tea.Cmd {
		return func() tea.Msg {
			cmd, ok := params.CmdRegistry.Find(name)
			if !ok {
				return tui.CommandResultMsg{Type: "text", Value: "Unknown command: /" + name, Command: name}
			}

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
				MCPManager:   params.MCPManager,
			}

			result, err := cmd.Execute(args, cmdCtx)
			if err != nil {
				return tui.CommandResultMsg{Type: "text", Value: "Command error: " + err.Error(), Command: name}
			}
			if result == nil {
				return tui.CommandResultMsg{Type: "text", Value: "", Command: name}
			}

			return tui.CommandResultMsg{
				Type:    result.Type,
				Value:   result.Value,
				Command: name,
			}
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
