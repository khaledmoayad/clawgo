package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/khaledmoayad/clawgo/internal/config"
	"github.com/spf13/cobra"
)

// newAuthCmd creates the "auth" subcommand group for authentication management.
func newAuthCmd() *cobra.Command {
	authCmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long:  "Manage authentication credentials for the Anthropic API.",
	}
	authCmd.AddCommand(newAuthLoginCmd())
	authCmd.AddCommand(newAuthStatusCmd())
	authCmd.AddCommand(newAuthLogoutCmd())
	return authCmd
}

// newAuthLoginCmd creates the "auth login" subcommand.
func newAuthLoginCmd() *cobra.Command {
	var (
		email    string
		sso      bool
		console  bool
		claudeai bool
	)

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to your Anthropic account",
		Long:  "Authenticate with Anthropic via OAuth. Opens a browser for the login flow.",
		RunE: func(cmd *cobra.Command, args []string) error {
			baseURL := config.ResolveAPIBaseURL(nil)
			loginURL := baseURL + "/oauth/authorize"

			if email != "" {
				loginURL += "?login_hint=" + email
			}

			// Full OAuth PKCE flow will be implemented in Phase 12 (MCP-09).
			// For now, print the login URL and instruct user to visit it.
			fmt.Println("To log in, visit the following URL in your browser:")
			fmt.Println()
			fmt.Printf("  %s\n", loginURL)
			fmt.Println()
			fmt.Println("After logging in, your credentials will be stored in:")
			fmt.Printf("  %s\n", config.CredentialsPath())
			fmt.Println()
			fmt.Println("Note: Full OAuth PKCE flow coming in a future release.")
			fmt.Println("For now, set ANTHROPIC_API_KEY environment variable for authentication.")

			return nil
		},
	}

	cmd.Flags().StringVar(&email, "email", "", "Email hint for login")
	cmd.Flags().BoolVar(&sso, "sso", false, "Use SSO login flow")
	cmd.Flags().BoolVar(&console, "console", false, "Log in to Anthropic Console")
	cmd.Flags().BoolVar(&claudeai, "claudeai", false, "Log in to claude.ai")

	return cmd
}

// authStatusOutput represents the JSON output of auth status.
type authStatusOutput struct {
	Authenticated bool   `json:"authenticated"`
	AuthMethod    string `json:"authMethod"`
	Model         string `json:"model,omitempty"`
}

// newAuthStatusCmd creates the "auth status" subcommand.
func newAuthStatusCmd() *cobra.Command {
	var (
		jsonOutput bool
		textOutput bool
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		Long:  "Display current authentication status and method.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				cfg = &config.Config{}
			}

			apiKey := config.ResolveAPIKey(cfg)
			model := config.Env(config.EnvModel)

			status := authStatusOutput{
				Authenticated: apiKey != "",
				Model:         model,
			}

			// Determine auth method
			if config.Env(config.EnvAPIKey) != "" {
				status.AuthMethod = "api_key"
			} else if config.Env(config.EnvAuthToken) != "" {
				status.AuthMethod = "oauth"
			} else if cfg.PrimaryAPIKey != "" {
				status.AuthMethod = "config"
			} else if apiKey != "" {
				status.AuthMethod = "credentials_file"
			} else {
				status.AuthMethod = "none"
			}

			if textOutput {
				if status.Authenticated {
					fmt.Printf("Authenticated: yes\n")
					fmt.Printf("Method: %s\n", status.AuthMethod)
					if status.Model != "" {
						fmt.Printf("Model: %s\n", status.Model)
					}
				} else {
					fmt.Println("Authenticated: no")
					fmt.Println("Run 'clawgo auth login' or set ANTHROPIC_API_KEY to authenticate.")
				}
				return nil
			}

			// Default: JSON output
			data, err := json.MarshalIndent(status, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal status: %w", err)
			}
			fmt.Println(string(data))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format (default)")
	cmd.Flags().BoolVar(&textOutput, "text", false, "Output in human-readable text format")

	return cmd
}

// newAuthLogoutCmd creates the "auth logout" subcommand.
func newAuthLogoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out and clear stored credentials",
		Long:  "Remove stored OAuth tokens and credentials.",
		RunE: func(cmd *cobra.Command, args []string) error {
			credsPath := config.CredentialsPath()

			// Check if credentials file exists
			if _, err := os.Stat(credsPath); os.IsNotExist(err) {
				fmt.Println("No stored credentials found.")
				return nil
			}

			// Also check for OAuth tokens directory
			oauthDir := filepath.Join(config.ConfigDir(), "oauth")

			// Remove credentials file
			if err := os.Remove(credsPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove credentials: %w", err)
			}

			// Remove OAuth tokens if they exist
			if _, err := os.Stat(oauthDir); err == nil {
				if err := os.RemoveAll(oauthDir); err != nil {
					return fmt.Errorf("failed to remove OAuth tokens: %w", err)
				}
			}

			fmt.Println("Successfully logged out. Stored credentials have been removed.")
			fmt.Printf("Note: Environment variables (ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN) are not affected.\n")
			return nil
		},
	}

	return cmd
}
