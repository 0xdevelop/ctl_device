package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ctl_device",
		Short: "ctl_device - multi-agent task coordination server",
		Long:  "ctl_device is a task coordination system for multi-agent workflows via MCP protocol.",
	}

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the coordination server",
		Long:  "Start the ctl_device coordination server (MCP SSE or stdio mode).",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "server not yet implemented")
			return nil
		},
	}

	clientCmd := &cobra.Command{
		Use:   "client",
		Short: "Client commands for interacting with the server",
		Long:  "Client commands to interact with a running ctl_device server.",
	}

	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP stdio client",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "mcp client not yet implemented")
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show project/task status",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "status not yet implemented")
			return nil
		},
	}

	dispatchCmd := &cobra.Command{
		Use:   "dispatch",
		Short: "Dispatch a task to an agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "dispatch not yet implemented")
			return nil
		},
	}

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Show server logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stderr, "logs not yet implemented")
			return nil
		},
	}

	clientCmd.AddCommand(mcpCmd, statusCmd, dispatchCmd, logsCmd)
	rootCmd.AddCommand(serverCmd, clientCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
