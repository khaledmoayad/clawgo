package commands

import "strings"

// CommandRegistry manages registered slash commands with alias resolution.
// It mirrors the tools.Registry pattern but adds alias support.
type CommandRegistry struct {
	commands map[string]Command
	aliases  map[string]string // alias -> canonical command name
	order    []string          // preserves registration order
}

// NewCommandRegistry creates an empty command registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]Command),
		aliases:  make(map[string]string),
		order:    make([]string, 0),
	}
}

// Register adds a command to the registry along with all its aliases.
func (r *CommandRegistry) Register(cmd Command) {
	name := cmd.Name()
	r.commands[name] = cmd
	r.order = append(r.order, name)
	for _, alias := range cmd.Aliases() {
		r.aliases[alias] = name
	}
}

// Find returns a command by name or alias.
// Returns the command and true if found, nil and false otherwise.
func (r *CommandRegistry) Find(name string) (Command, bool) {
	if cmd, ok := r.commands[name]; ok {
		return cmd, true
	}
	if target, ok := r.aliases[name]; ok {
		return r.commands[target], true
	}
	return nil, false
}

// All returns all registered commands in registration order.
func (r *CommandRegistry) All() []Command {
	result := make([]Command, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.commands[name])
	}
	return result
}

// ParseCommandInput parses a slash command input string into command name and arguments.
// If the input starts with "/", it extracts the command name (resolving aliases)
// and the remaining arguments. Returns ("", "") if input is not a command.
func (r *CommandRegistry) ParseCommandInput(input string) (name string, args string) {
	if !IsCommand(input) {
		return "", ""
	}

	// Remove the leading "/"
	input = input[1:]

	// Split on first whitespace
	var rawName string
	if idx := strings.IndexByte(input, ' '); idx >= 0 {
		rawName = input[:idx]
		args = strings.TrimSpace(input[idx+1:])
	} else {
		rawName = input
	}

	// Resolve alias to canonical name
	if target, ok := r.aliases[rawName]; ok {
		return target, args
	}
	return rawName, args
}

// IsCommand returns true if the input looks like a slash command.
func IsCommand(input string) bool {
	return len(input) > 0 && input[0] == '/'
}
