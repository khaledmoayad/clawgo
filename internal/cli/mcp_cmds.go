package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/khaledmoayad/clawgo/internal/config"
	"github.com/spf13/cobra"
)

// MCPServerEntry represents a single MCP server configuration.
type MCPServerEntry struct {
	Command      string            `json:"command,omitempty"`
	Args         []string          `json:"args,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	URL          string            `json:"url,omitempty"`
	Transport    string            `json:"transport,omitempty"`
	ClientSecret string            `json:"clientSecret,omitempty"`
}

// MCPConfig represents the .mcp.json configuration file format.
type MCPConfig struct {
	MCPServers map[string]MCPServerEntry `json:"mcpServers"`
}

// mcpConfigPath returns the path to the MCP config file for the given scope.
func mcpConfigPath(scope string) (string, error) {
	switch scope {
	case "user":
		return filepath.Join(config.ConfigDir(), "mcp.json"), nil
	case "local":
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		return filepath.Join(cwd, ".mcp.json"), nil
	case "project":
		// Use current directory as project root for now
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		return filepath.Join(cwd, ".mcp.json"), nil
	default:
		return "", fmt.Errorf("unknown scope: %s (valid: user, local, project)", scope)
	}
}

// loadMCPConfig reads and parses an MCP config file.
func loadMCPConfig(path string) (*MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &MCPConfig{MCPServers: make(map[string]MCPServerEntry)}, nil
		}
		return nil, err
	}

	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if cfg.MCPServers == nil {
		cfg.MCPServers = make(map[string]MCPServerEntry)
	}
	return &cfg, nil
}

// saveMCPConfig writes the MCP config to disk.
func saveMCPConfig(path string, cfg *MCPConfig) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// newMCPAddCmd creates the "mcp add" subcommand.
func newMCPAddCmd() *cobra.Command {
	var (
		transport string
		command   string
		mcpArgs   []string
		envVars   []string
		scope     string
	)

	cmd := &cobra.Command{
		Use:   "add NAME",
		Short: "Add an MCP server",
		Long:  "Add a new MCP server configuration.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			configPath, err := mcpConfigPath(scope)
			if err != nil {
				return err
			}

			cfg, err := loadMCPConfig(configPath)
			if err != nil {
				return err
			}

			entry := MCPServerEntry{
				Transport: transport,
				Command:   command,
				Args:      mcpArgs,
			}

			// Parse env vars from KEY=VALUE format
			if len(envVars) > 0 {
				entry.Env = make(map[string]string)
				for _, ev := range envVars {
					parts := strings.SplitN(ev, "=", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid env format %q, expected KEY=VALUE", ev)
					}
					entry.Env[parts[0]] = parts[1]
				}
			}

			cfg.MCPServers[name] = entry

			if err := saveMCPConfig(configPath, cfg); err != nil {
				return err
			}

			fmt.Printf("Added MCP server %q to %s\n", name, configPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&transport, "transport", "stdio", "Transport type: stdio, sse")
	cmd.Flags().StringVar(&command, "command", "", "Command to run for stdio transport")
	cmd.Flags().StringSliceVar(&mcpArgs, "args", nil, "Arguments for the command")
	cmd.Flags().StringSliceVar(&envVars, "env", nil, "Environment variables (KEY=VALUE)")
	cmd.Flags().StringVar(&scope, "scope", "local", "Config scope: local, project, user")

	return cmd
}

// newMCPRemoveCmd creates the "mcp remove" subcommand.
func newMCPRemoveCmd() *cobra.Command {
	var scope string

	cmd := &cobra.Command{
		Use:   "remove NAME",
		Short: "Remove an MCP server",
		Long:  "Remove an MCP server configuration.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			configPath, err := mcpConfigPath(scope)
			if err != nil {
				return err
			}

			cfg, err := loadMCPConfig(configPath)
			if err != nil {
				return err
			}

			if _, exists := cfg.MCPServers[name]; !exists {
				return fmt.Errorf("MCP server %q not found in %s", name, configPath)
			}

			delete(cfg.MCPServers, name)

			if err := saveMCPConfig(configPath, cfg); err != nil {
				return err
			}

			fmt.Printf("Removed MCP server %q from %s\n", name, configPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "local", "Config scope: local, project, user")

	return cmd
}

// newMCPListCmd creates the "mcp list" subcommand.
func newMCPListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured MCP servers",
		Long:  "List all configured MCP servers from all scopes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			scopes := []struct {
				name string
				path string
			}{
				{"user", filepath.Join(config.ConfigDir(), "mcp.json")},
			}

			// Add local/project scope
			if cwd, err := os.Getwd(); err == nil {
				scopes = append(scopes, struct {
					name string
					path string
				}{"local", filepath.Join(cwd, ".mcp.json")})
			}

			found := false
			for _, s := range scopes {
				cfg, err := loadMCPConfig(s.path)
				if err != nil {
					continue
				}
				for name, entry := range cfg.MCPServers {
					found = true
					transport := entry.Transport
					if transport == "" {
						transport = "stdio"
					}
					fmt.Printf("  %-20s %-10s %-10s %s\n", name, transport, s.name, entry.Command)
				}
			}

			if !found {
				fmt.Println("No MCP servers configured.")
				fmt.Println("Use 'clawgo mcp add NAME --command CMD' to add one.")
			}

			return nil
		},
	}

	return cmd
}

// newMCPGetCmd creates the "mcp get" subcommand.
func newMCPGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get NAME",
		Short: "Show details of an MCP server",
		Long:  "Display the full configuration of a specific MCP server.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Search all scopes
			scopes := []string{"user", "local"}
			for _, scope := range scopes {
				configPath, err := mcpConfigPath(scope)
				if err != nil {
					continue
				}
				cfg, err := loadMCPConfig(configPath)
				if err != nil {
					continue
				}
				if entry, exists := cfg.MCPServers[name]; exists {
					data, err := json.MarshalIndent(entry, "", "  ")
					if err != nil {
						return err
					}
					fmt.Printf("Server: %s (scope: %s)\n", name, scope)
					fmt.Println(string(data))
					return nil
				}
			}

			return fmt.Errorf("MCP server %q not found in any scope", name)
		},
	}

	return cmd
}

// newMCPAddJSONCmd creates the "mcp add-json" subcommand.
func newMCPAddJSONCmd() *cobra.Command {
	var (
		scope        string
		clientSecret string
	)

	cmd := &cobra.Command{
		Use:   "add-json NAME JSON",
		Short: "Add an MCP server from JSON",
		Long:  "Add an MCP server by providing its configuration as a JSON string.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			jsonStr := args[1]

			var entry MCPServerEntry
			if err := json.Unmarshal([]byte(jsonStr), &entry); err != nil {
				return fmt.Errorf("invalid JSON: %w", err)
			}

			if clientSecret != "" {
				entry.ClientSecret = clientSecret
			}

			configPath, err := mcpConfigPath(scope)
			if err != nil {
				return err
			}

			cfg, err := loadMCPConfig(configPath)
			if err != nil {
				return err
			}

			cfg.MCPServers[name] = entry

			if err := saveMCPConfig(configPath, cfg); err != nil {
				return err
			}

			fmt.Printf("Added MCP server %q from JSON to %s\n", name, configPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "local", "Config scope: local, project, user")
	cmd.Flags().StringVar(&clientSecret, "client-secret", "", "Client secret for the MCP server")

	return cmd
}

// newMCPAddFromDesktopCmd creates the "mcp add-from-claude-desktop" subcommand.
func newMCPAddFromDesktopCmd() *cobra.Command {
	var scope string

	cmd := &cobra.Command{
		Use:   "add-from-claude-desktop",
		Short: "Import MCP servers from Claude Desktop",
		Long:  "Import MCP server configurations from Claude Desktop's config file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Claude Desktop config locations
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			// Check common Claude Desktop config paths
			paths := []string{
				filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"),
				filepath.Join(home, ".config", "claude-desktop", "config.json"),
			}

			var desktopCfg MCPConfig
			var found bool
			for _, p := range paths {
				data, err := os.ReadFile(p)
				if err != nil {
					continue
				}
				if err := json.Unmarshal(data, &desktopCfg); err != nil {
					continue
				}
				found = true
				break
			}

			if !found {
				return fmt.Errorf("Claude Desktop config not found. Checked: %s", strings.Join(paths, ", "))
			}

			if len(desktopCfg.MCPServers) == 0 {
				fmt.Println("No MCP servers found in Claude Desktop config.")
				return nil
			}

			configPath, err := mcpConfigPath(scope)
			if err != nil {
				return err
			}

			cfg, err := loadMCPConfig(configPath)
			if err != nil {
				return err
			}

			count := 0
			for name, entry := range desktopCfg.MCPServers {
				cfg.MCPServers[name] = entry
				count++
			}

			if err := saveMCPConfig(configPath, cfg); err != nil {
				return err
			}

			fmt.Printf("Imported %d MCP server(s) from Claude Desktop to %s\n", count, configPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "local", "Config scope: local, project, user")

	return cmd
}

// newMCPResetProjectChoicesCmd creates the "mcp reset-project-choices" subcommand.
func newMCPResetProjectChoicesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset-project-choices",
		Short: "Reset approved/rejected project MCP servers",
		Long:  "Clear all approved and rejected choices for project-scoped MCP servers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// The choices are stored in the user-level config
			choicesPath := filepath.Join(config.ConfigDir(), "mcp-choices.json")

			if err := os.Remove(choicesPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to reset choices: %w", err)
			}

			fmt.Println("Project MCP server choices have been reset.")
			return nil
		},
	}

	return cmd
}
