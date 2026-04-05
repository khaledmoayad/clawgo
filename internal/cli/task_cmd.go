package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newTaskCmd creates the "task" subcommand group for task management.
func newTaskCmd() *cobra.Command {
	taskCmd := &cobra.Command{
		Use:   "task",
		Short: "Manage task list tasks",
		Long:  "Create, list, and manage tasks.",
	}

	taskCmd.AddCommand(newTaskCreateCmd())
	taskCmd.AddCommand(newTaskListCmd())
	taskCmd.AddCommand(newTaskGetCmd())
	taskCmd.AddCommand(newTaskUpdateCmd())
	taskCmd.AddCommand(newTaskDirCmd())

	return taskCmd
}

// newTaskCreateCmd creates the "task create" subcommand.
func newTaskCreateCmd() *cobra.Command {
	var (
		description string
		list        string
	)

	cmd := &cobra.Command{
		Use:   "create SUBJECT",
		Short: "Create a new task",
		Long:  "Create a new task with the given subject.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Creating task: %s\n", args[0])
			if description != "" {
				fmt.Printf("Description: %s\n", description)
			}
			if list != "" {
				fmt.Printf("List: %s\n", list)
			}
			fmt.Println("Task creation is not yet available. Coming in a future release.")
			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Task description")
	cmd.Flags().StringVar(&list, "list", "", "Task list name")

	return cmd
}

// newTaskListCmd creates the "task list" subcommand.
func newTaskListCmd() *cobra.Command {
	var (
		list       string
		pending    bool
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		Long:  "List all tasks, optionally filtered by list or status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				fmt.Println("[]")
				return nil
			}
			status := "all"
			if pending {
				status = "pending"
			}
			fmt.Printf("Listing tasks (list: %s, status: %s)\n", list, status)
			fmt.Println("Task listing is not yet available. Coming in a future release.")
			return nil
		},
	}

	cmd.Flags().StringVar(&list, "list", "", "Filter by task list")
	cmd.Flags().BoolVar(&pending, "pending", false, "Show only pending tasks")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	return cmd
}

// newTaskGetCmd creates the "task get" subcommand.
func newTaskGetCmd() *cobra.Command {
	var list string

	cmd := &cobra.Command{
		Use:   "get ID",
		Short: "Get task details",
		Long:  "Display details of a specific task by ID.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Getting task: %s\n", args[0])
			if list != "" {
				fmt.Printf("List: %s\n", list)
			}
			fmt.Println("Task retrieval is not yet available. Coming in a future release.")
			return nil
		},
	}

	cmd.Flags().StringVar(&list, "list", "", "Task list name")

	return cmd
}

// newTaskUpdateCmd creates the "task update" subcommand.
func newTaskUpdateCmd() *cobra.Command {
	var (
		list        string
		status      string
		subject     string
		description string
		owner       string
		clearOwner  bool
	)

	cmd := &cobra.Command{
		Use:   "update ID",
		Short: "Update a task",
		Long:  "Update fields of an existing task.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Updating task: %s\n", args[0])
			fmt.Println("Task update is not yet available. Coming in a future release.")
			return nil
		},
	}

	cmd.Flags().StringVar(&list, "list", "", "Task list name")
	cmd.Flags().StringVar(&status, "status", "", "New task status")
	cmd.Flags().StringVar(&subject, "subject", "", "New task subject")
	cmd.Flags().StringVar(&description, "description", "", "New task description")
	cmd.Flags().StringVar(&owner, "owner", "", "New task owner")
	cmd.Flags().BoolVar(&clearOwner, "clear-owner", false, "Clear the task owner")

	return cmd
}

// newTaskDirCmd creates the "task dir" subcommand.
func newTaskDirCmd() *cobra.Command {
	var list string

	cmd := &cobra.Command{
		Use:   "dir",
		Short: "Show task directory",
		Long:  "Display the directory where task data is stored.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if list != "" {
				fmt.Printf("Task list: %s\n", list)
			}
			fmt.Println("Task directory is not yet available. Coming in a future release.")
			return nil
		},
	}

	cmd.Flags().StringVar(&list, "list", "", "Task list name")

	return cmd
}
