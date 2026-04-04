// Package all provides RegisterAll which registers all 44 external slash commands.
// This lives in a separate sub-package to avoid circular imports between the
// commands package and individual command sub-packages.
package all

import (
	"github.com/khaledmoayad/clawgo/internal/commands"
	"github.com/khaledmoayad/clawgo/internal/commands/adddir"
	"github.com/khaledmoayad/clawgo/internal/commands/agents"
	"github.com/khaledmoayad/clawgo/internal/commands/branch"
	"github.com/khaledmoayad/clawgo/internal/commands/clear"
	"github.com/khaledmoayad/clawgo/internal/commands/color"
	"github.com/khaledmoayad/clawgo/internal/commands/compact"
	"github.com/khaledmoayad/clawgo/internal/commands/config"
	"github.com/khaledmoayad/clawgo/internal/commands/context"
	"github.com/khaledmoayad/clawgo/internal/commands/copy"
	"github.com/khaledmoayad/clawgo/internal/commands/cost"
	"github.com/khaledmoayad/clawgo/internal/commands/debug"
	"github.com/khaledmoayad/clawgo/internal/commands/diff"
	"github.com/khaledmoayad/clawgo/internal/commands/doctor"
	"github.com/khaledmoayad/clawgo/internal/commands/effort"
	"github.com/khaledmoayad/clawgo/internal/commands/env"
	"github.com/khaledmoayad/clawgo/internal/commands/exit"
	"github.com/khaledmoayad/clawgo/internal/commands/export"
	"github.com/khaledmoayad/clawgo/internal/commands/fast"
	"github.com/khaledmoayad/clawgo/internal/commands/feedback"
	"github.com/khaledmoayad/clawgo/internal/commands/files"
	"github.com/khaledmoayad/clawgo/internal/commands/githubactions"
	"github.com/khaledmoayad/clawgo/internal/commands/help"
	"github.com/khaledmoayad/clawgo/internal/commands/hooks"
	"github.com/khaledmoayad/clawgo/internal/commands/ide"
	"github.com/khaledmoayad/clawgo/internal/commands/keybindings"
	"github.com/khaledmoayad/clawgo/internal/commands/login"
	"github.com/khaledmoayad/clawgo/internal/commands/logout"
	"github.com/khaledmoayad/clawgo/internal/commands/mcp"
	"github.com/khaledmoayad/clawgo/internal/commands/memory"
	"github.com/khaledmoayad/clawgo/internal/commands/model"
	"github.com/khaledmoayad/clawgo/internal/commands/permissions"
	"github.com/khaledmoayad/clawgo/internal/commands/plan"
	"github.com/khaledmoayad/clawgo/internal/commands/plugin"
	"github.com/khaledmoayad/clawgo/internal/commands/resume"
	"github.com/khaledmoayad/clawgo/internal/commands/review"
	"github.com/khaledmoayad/clawgo/internal/commands/rewind"
	"github.com/khaledmoayad/clawgo/internal/commands/session"
	"github.com/khaledmoayad/clawgo/internal/commands/skills"
	"github.com/khaledmoayad/clawgo/internal/commands/stats"
	"github.com/khaledmoayad/clawgo/internal/commands/status"
	"github.com/khaledmoayad/clawgo/internal/commands/tag"
	"github.com/khaledmoayad/clawgo/internal/commands/tasks"
	"github.com/khaledmoayad/clawgo/internal/commands/theme"
	"github.com/khaledmoayad/clawgo/internal/commands/upgrade"
	"github.com/khaledmoayad/clawgo/internal/commands/usage"
	"github.com/khaledmoayad/clawgo/internal/commands/version"
	"github.com/khaledmoayad/clawgo/internal/commands/vim"
)

// RegisterAll registers all 44 external slash commands into the given registry.
// Commands are registered in a logical order: core first, then alphabetical.
func RegisterAll(registry *commands.CommandRegistry) {
	// Core commands (11)
	registry.Register(help.New())
	registry.Register(clear.New())
	registry.Register(compact.New())
	registry.Register(model.New())
	registry.Register(cost.New())
	registry.Register(exit.New())
	registry.Register(status.New())
	registry.Register(permissions.New())
	registry.Register(config.New())
	registry.Register(vim.New())
	registry.Register(theme.New())

	// Additional commands (33) - alphabetical
	registry.Register(adddir.New())
	registry.Register(agents.New())
	registry.Register(branch.New())
	registry.Register(color.New())
	registry.Register(context.New())
	registry.Register(copy.New())
	registry.Register(debug.New())
	registry.Register(diff.New())
	registry.Register(doctor.New())
	registry.Register(effort.New())
	registry.Register(env.New())
	registry.Register(export.New())
	registry.Register(fast.New())
	registry.Register(feedback.New())
	registry.Register(files.New())
	registry.Register(githubactions.New())
	registry.Register(hooks.New())
	registry.Register(ide.New())
	registry.Register(keybindings.New())
	registry.Register(login.New())
	registry.Register(logout.New())
	registry.Register(mcp.New())
	registry.Register(memory.New())
	registry.Register(plan.New())
	registry.Register(plugin.New())
	registry.Register(resume.New())
	registry.Register(review.New())
	registry.Register(rewind.New())
	registry.Register(session.New())
	registry.Register(skills.New())
	registry.Register(stats.New())
	registry.Register(tag.New())
	registry.Register(tasks.New())
	registry.Register(upgrade.New())
	registry.Register(usage.New())
	registry.Register(version.New())
}
