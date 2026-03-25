package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/client"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/project"
	"github.com/0xdevelop/ctl_device/internal/server"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ctl_device",
		Short: "ctl_device - multi-agent task coordination server",
		Long:  "ctl_device is a task coordination system for multi-agent workflows via MCP protocol.",
	}

	var addr string
	var token string
	var stateDir string

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the coordination server",
		Long:  "Start the ctl_device coordination server (JSON-RPC HTTP server).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(addr, token, stateDir)
		},
	}
	serverCmd.Flags().StringVarP(&addr, "addr", "a", ":3711", "Server address")
	serverCmd.Flags().StringVarP(&token, "token", "t", "", "Authentication token (optional)")
	serverCmd.Flags().StringVarP(&stateDir, "state-dir", "s", "", "State directory")

	var serverURL string
	var agentID string
	var projectFilter string
	var taskFile string
	var follow bool

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
			return runStatus(serverURL, token, agentID, projectFilter)
		},
	}
	statusCmd.Flags().StringVarP(&serverURL, "server", "s", "http://localhost:3711", "Server URL")
	statusCmd.Flags().StringVarP(&token, "token", "t", "", "Authentication token")
	statusCmd.Flags().StringVarP(&agentID, "agent", "a", "", "Agent ID")
	statusCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Project filter")

	dispatchCmd := &cobra.Command{
		Use:   "dispatch",
		Short: "Dispatch a task to an agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDispatch(serverURL, token, agentID, projectFilter, taskFile)
		},
	}
	dispatchCmd.Flags().StringVarP(&serverURL, "server", "s", "http://localhost:3711", "Server URL")
	dispatchCmd.Flags().StringVarP(&token, "token", "t", "", "Authentication token")
	dispatchCmd.Flags().StringVarP(&agentID, "agent", "a", "", "Agent ID")
	dispatchCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Project name")
	dispatchCmd.Flags().StringVarP(&taskFile, "task-file", "f", "", "Task file to dispatch")
	_ = dispatchCmd.MarkFlagRequired("project")
	_ = dispatchCmd.MarkFlagRequired("task-file")

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Show server logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(serverURL, token, projectFilter, follow)
		},
	}
	logsCmd.Flags().StringVarP(&serverURL, "server", "s", "http://localhost:3711", "Server URL")
	logsCmd.Flags().StringVarP(&token, "token", "t", "", "Authentication token")
	logsCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Project filter")
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow logs (SSE)")

	clientCmd.AddCommand(mcpCmd, statusCmd, dispatchCmd, logsCmd)
	rootCmd.AddCommand(serverCmd, clientCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServer(addr, token, stateDir string) error {
	store, err := project.NewFileStore(stateDir)
	if err != nil {
		return fmt.Errorf("failed to create file store: %w", err)
	}

	eventBus := event.NewBus()

	scheduler := project.NewScheduler(store, eventBus)

	registry, err := agent.NewRegistry(stateDir)
	if err != nil {
		return fmt.Errorf("failed to create agent registry: %w", err)
	}
	manager, err := agent.NewManager(registry, store, eventBus)
	if err != nil {
		return fmt.Errorf("failed to create agent manager: %w", err)
	}

	jsonrpcServer, err := server.NewServer(addr, token, manager, scheduler, store, eventBus)
	if err != nil {
		return fmt.Errorf("failed to create JSON-RPC server: %w", err)
	}

	jsonrpcServer.SubscribeToEvents()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler.StartSnapshotLoop(ctx, 5*time.Minute)
	scheduler.CheckTimeouts(ctx, func(msg string) {
		fmt.Fprintf(os.Stderr, "TIMEOUT: %s\n", msg)
	})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nShutting down server...")
		cancel()
		manager.Shutdown()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		jsonrpcServer.Shutdown(shutdownCtx)
	}()

	fmt.Printf("Starting ctl_device server on %s\n", addr)
	if token != "" {
		fmt.Printf("Token authentication enabled\n")
	}
	fmt.Printf("State directory: %s\n", store.Dir())

	if err := jsonrpcServer.Start(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func runStatus(serverURL, token, agentID, projectFilter string) error {
	c := client.NewClient(serverURL, token, agentID)

	resp, err := c.ProjectList()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	if len(resp.Projects) == 0 {
		fmt.Println("No projects registered")
		return nil
	}

	for _, proj := range resp.Projects {
		if projectFilter != "" && proj.Name != projectFilter {
			continue
		}

		fmt.Printf("\nProject: %s\n", proj.Name)
		fmt.Printf("  Directory: %s\n", proj.Dir)
		fmt.Printf("  Tech: %s\n", proj.Tech)
		fmt.Printf("  Executor: %s\n", proj.Executor)
		fmt.Printf("  Timeout: %d minutes\n", proj.TimeoutMinutes)

		tasks := resp.Tasks[proj.Name]
		if len(tasks) == 0 {
			fmt.Printf("  Tasks: none\n")
			continue
		}

		fmt.Printf("  Tasks:\n")
		for _, task := range tasks {
			fmt.Printf("    - [%s] %s: %s\n", task.Status, task.Num, task.Name)
			if task.AssignedTo != "" {
				fmt.Printf("      Assigned to: %s\n", task.AssignedTo)
			}
			if task.Commit != "" {
				fmt.Printf("      Commit: %s\n", task.Commit)
			}
		}
	}

	return nil
}

func runDispatch(serverURL, token, agentID, project, taskFile string) error {
	c := client.NewClient(serverURL, token, agentID)

	data, err := os.ReadFile(taskFile)
	if err != nil {
		return fmt.Errorf("failed to read task file: %w", err)
	}

	var task interface{}
	if err := json.Unmarshal(data, &task); err != nil {
		return fmt.Errorf("failed to parse task file: %w", err)
	}

	if err := c.TaskDispatch(project, task); err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	fmt.Printf("Task dispatched to project %s\n", project)
	return nil
}

func runLogs(serverURL, token, projectFilter string, follow bool) error {
	c := client.NewClient(serverURL, token, "")

	if !follow {
		return fmt.Errorf("logs without --follow not yet implemented")
	}

	eventCh, errCh, err := c.SubscribeEvents(projectFilter)
	if err != nil {
		return fmt.Errorf("failed to subscribe to events: %w", err)
	}

	for {
		select {
		case event := <-eventCh:
			data, _ := json.MarshalIndent(event, "", "  ")
			fmt.Printf("%s\n", string(data))
		case err := <-errCh:
			return fmt.Errorf("event stream error: %w", err)
		}
	}
}
