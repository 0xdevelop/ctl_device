package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/0xdevelop/ctl_device/config"
	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/client"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/notify"
	"github.com/0xdevelop/ctl_device/internal/recovery"
	"github.com/0xdevelop/ctl_device/internal/project"
	"github.com/0xdevelop/ctl_device/internal/server"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "ctl_device",
		Short: "ctl_device - multi-agent task coordination server",
		Long:  "ctl_device is a task coordination system for multi-agent workflows via MCP protocol.",
	}

	var jsonrpcPort int
	var mcpPort int
	var dashboardPort int
	var token string
	var stateDir string
	var configFile string

	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "Start the coordination server",
		Long:  "Start the ctl_device coordination server (JSON-RPC HTTP server).",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(jsonrpcPort, mcpPort, dashboardPort, token, stateDir, configFile)
		},
	}
	serverCmd.Flags().IntVar(&jsonrpcPort, "jsonrpc-port", 0, "JSON-RPC server port (default 3711)")
	serverCmd.Flags().IntVar(&mcpPort, "mcp-port", 0, "MCP SSE server port (default 3710)")
	serverCmd.Flags().IntVar(&dashboardPort, "dashboard-port", 0, "Dashboard port (default 3712)")
	serverCmd.Flags().StringVarP(&token, "token", "t", "", "Authentication token (optional)")
	serverCmd.Flags().StringVarP(&stateDir, "state-dir", "s", "", "State directory")
	serverCmd.Flags().StringVarP(&configFile, "config", "c", "", "Config file path")

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

	var mcpServer string
	var mcpToken string
	var clientConfigPath string

	mcpCmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP stdio client (proxy to remote JSON-RPC server)",
		Long:  "Start MCP stdio client that proxies MCP requests to a remote JSON-RPC server.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPClient(mcpServer, mcpToken, clientConfigPath)
		},
	}
	mcpCmd.Flags().StringVarP(&mcpServer, "server", "s", "http://localhost:3711", "Remote JSON-RPC server URL")
	mcpCmd.Flags().StringVarP(&mcpToken, "token", "t", "", "Authentication token")
	mcpCmd.Flags().StringVarP(&clientConfigPath, "config", "c", "", "Client config file path")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show project/task status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(serverURL, token, agentID, projectFilter, clientConfigPath)
		},
	}
	statusCmd.Flags().StringVarP(&serverURL, "server", "s", "http://localhost:3711", "Server URL")
	statusCmd.Flags().StringVarP(&token, "token", "t", "", "Authentication token")
	statusCmd.Flags().StringVarP(&agentID, "agent", "a", "", "Agent ID")
	statusCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Project filter")
	statusCmd.Flags().StringVarP(&clientConfigPath, "config", "c", "", "Client config file path")

	dispatchCmd := &cobra.Command{
		Use:   "dispatch",
		Short: "Dispatch a task to an agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDispatch(serverURL, token, agentID, projectFilter, taskFile, clientConfigPath)
		},
	}
	dispatchCmd.Flags().StringVarP(&serverURL, "server", "s", "http://localhost:3711", "Server URL")
	dispatchCmd.Flags().StringVarP(&token, "token", "t", "", "Authentication token")
	dispatchCmd.Flags().StringVarP(&agentID, "agent", "a", "", "Agent ID")
	dispatchCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Project name")
	dispatchCmd.Flags().StringVarP(&taskFile, "task-file", "f", "", "Task file to dispatch")
	dispatchCmd.Flags().StringVarP(&clientConfigPath, "config", "c", "", "Client config file path")
	_ = dispatchCmd.MarkFlagRequired("project")
	_ = dispatchCmd.MarkFlagRequired("task-file")

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Show server logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(serverURL, token, projectFilter, follow, clientConfigPath)
		},
	}
	logsCmd.Flags().StringVarP(&serverURL, "server", "s", "http://localhost:3711", "Server URL")
	logsCmd.Flags().StringVarP(&token, "token", "t", "", "Authentication token")
	logsCmd.Flags().StringVarP(&projectFilter, "project", "p", "", "Project filter")
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow logs (SSE)")
	logsCmd.Flags().StringVarP(&clientConfigPath, "config", "c", "", "Client config file path")

	clientCmd.AddCommand(mcpCmd, statusCmd, dispatchCmd, logsCmd)
	rootCmd.AddCommand(serverCmd, clientCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runServer(jsonrpcPort, mcpPort, dashboardPort int, token, stateDir, configFile string) error {
	var cfg *config.ServerConfig
	var err error

	// Priority: CLI > env > config > default
	// 1. Load from config file or default
	if configFile != "" {
		cfg, err = config.LoadServerConfig(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	} else {
		cfg = config.DefaultServerConfig()
	}

	// 2. Override with environment variables
	if envToken := os.Getenv("CTL_DEVICE_TOKEN"); envToken != "" {
		cfg.Server.Token = envToken
	}
	if envAddr := os.Getenv("CTL_DEVICE_ADDR"); envAddr != "" {
		cfg.Server.Bind = envAddr
	}
	if envStateDir := os.Getenv("CTL_DEVICE_STATE_DIR"); envStateDir != "" {
		cfg.Server.StateDir = envStateDir
	}

	// 3. Override with CLI arguments (highest priority, 0 = not set)
	if token != "" {
		cfg.Server.Token = token
	}
	if jsonrpcPort != 0 {
		cfg.Server.JSONRPCPort = jsonrpcPort
	}
	if mcpPort != 0 {
		cfg.Server.MCPPort = mcpPort
	}
	if dashboardPort != 0 {
		cfg.Server.DashboardPort = dashboardPort
	}
	if stateDir != "" {
		cfg.Server.StateDir = stateDir
	}

	store, err := project.NewFileStore(cfg.Server.StateDir)
	if err != nil {
		return fmt.Errorf("failed to create file store: %w", err)
	}

	eventBus := event.NewBus()

	scheduler := project.NewScheduler(store, eventBus)

	registry, err := agent.NewRegistry(cfg.Server.StateDir)
	if err != nil {
		return fmt.Errorf("failed to create agent registry: %w", err)
	}
	manager, err := agent.NewManager(registry, store, eventBus)
	if err != nil {
		return fmt.Errorf("failed to create agent manager: %w", err)
	}

	jsonrpcServer, err := server.NewServer(
		fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.JSONRPCPort),
		cfg.Server.Token,
		manager,
		scheduler,
		store,
		eventBus,
	)
	if err != nil {
		return fmt.Errorf("failed to create JSON-RPC server: %w", err)
	}

	if cfg.Server.TLS.Enabled {
		jsonrpcServer.SetTLSConfig(
			cfg.Server.TLS.Enabled,
			cfg.Server.TLS.CertFile,
			cfg.Server.TLS.KeyFile,
			cfg.Server.TLS.AutoTLS,
			cfg.Server.TLS.Domain,
		)
	}

	jsonrpcServer.SubscribeToEvents()

	// Notifier
	notifier := notify.NewNotifier(cfg.Notify.Channel, cfg.Notify.Target)

	// Recovery Manager（断线/重启/token限制/超时恢复）
	recoveryMgr := recovery.NewManager(scheduler, manager, notifier, eventBus)
	if err := recoveryMgr.OnServerStart(); err != nil {
		fmt.Fprintf(os.Stderr, "Recovery warning: %v\n", err)
	}

	// MCP SSE Server (:3710)
	mcpSSEServer := server.NewMCPSSEServer(
		fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.MCPPort),
		scheduler,
		manager,
		store,
		eventBus,
	)

	dashboard := server.NewDashboard(
		fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.DashboardPort),
		manager,
		scheduler,
		eventBus,
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	snapshotInterval := time.Duration(cfg.Server.SnapshotIntervalSecs) * time.Second
	scheduler.StartSnapshotLoop(ctx, snapshotInterval)
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
		mcpSSEServer.Shutdown(shutdownCtx)
		dashboard.Shutdown(shutdownCtx)
	}()

	fmt.Printf("Starting ctl_device server on %s:%d\n", cfg.Server.Bind, cfg.Server.JSONRPCPort)
	if cfg.Server.Token != "" {
		fmt.Printf("Token authentication enabled\n")
	}
	fmt.Printf("Dashboard available at http://%s:%d\n", cfg.Server.Bind, cfg.Server.DashboardPort)
	fmt.Printf("State directory: %s\n", store.Dir())

	fmt.Printf("MCP SSE server at http://%s:%d/sse\n", cfg.Server.Bind, cfg.Server.MCPPort)
	recoveryMgr.Start(ctx)
	go func() {
		fmt.Fprintf(os.Stderr, "Starting MCP SSE on %s:%d...\n", cfg.Server.Bind, cfg.Server.MCPPort)
		if err := mcpSSEServer.Start(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "MCP SSE failed: %v\n", err)
		}
	}()
	go func() {
		fmt.Fprintf(os.Stderr, "Starting dashboard on %s:%d...\n", cfg.Server.Bind, cfg.Server.DashboardPort)
		if err := dashboard.Start(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Dashboard failed: %v\n", err)
		}
	}()

	if err := jsonrpcServer.Start(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

func runStatus(serverURL, token, agentID, projectFilter, clientConfigPath string) error {
	var cfg *config.ClientConfig
	var err error

	if clientConfigPath != "" {
		cfg, err = config.LoadClientConfig(clientConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	} else {
		cfg, err = config.LoadClientConfig("")
		if err != nil {
			cfg = config.DefaultClientConfig()
		}
	}

	config.ApplyClientConfigOverrides(cfg, serverURL, token, agentID)

	c := client.NewClient(cfg.Server, cfg.Token, cfg.AgentID)

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

func runDispatch(serverURL, token, agentID, project, taskFile, clientConfigPath string) error {
	var cfg *config.ClientConfig
	var err error

	if clientConfigPath != "" {
		cfg, err = config.LoadClientConfig(clientConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	} else {
		cfg, err = config.LoadClientConfig("")
		if err != nil {
			cfg = config.DefaultClientConfig()
		}
	}

	config.ApplyClientConfigOverrides(cfg, serverURL, token, agentID)

	c := client.NewClient(cfg.Server, cfg.Token, cfg.AgentID)

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

func runLogs(serverURL, token, projectFilter string, follow bool, clientConfigPath string) error {
	var cfg *config.ClientConfig
	var err error

	if clientConfigPath != "" {
		cfg, err = config.LoadClientConfig(clientConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	} else {
		cfg, err = config.LoadClientConfig("")
		if err != nil {
			cfg = config.DefaultClientConfig()
		}
	}

	config.ApplyClientConfigOverrides(cfg, serverURL, token, "")

	c := client.NewClient(cfg.Server, cfg.Token, cfg.AgentID)

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

func runMCPClient(serverURL, token, clientConfigPath string) error {
	var cfg *config.ClientConfig
	var err error

	if clientConfigPath != "" {
		cfg, err = config.LoadClientConfig(clientConfigPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	} else {
		cfg, err = config.LoadClientConfig("")
		if err != nil {
			cfg = config.DefaultClientConfig()
		}
	}

	config.ApplyClientConfigOverrides(cfg, serverURL, token, "")

	c := client.NewClient(cfg.Server, cfg.Token, cfg.AgentID)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var mcpReq map[string]interface{}
		if err := json.Unmarshal([]byte(line), &mcpReq); err != nil {
			sendMCPError(nil, -32700, "Parse error: "+err.Error())
			continue
		}

		if mcpReq["jsonrpc"] != "2.0" {
			sendMCPError(mcpReq["id"], -32600, "Invalid Request")
			continue
		}

		method, _ := mcpReq["method"].(string)
		params := mcpReq["params"]

		result, err := handleMCPMethod(method, params, c)
		if err != nil {
			sendMCPError(mcpReq["id"], -32000, err.Error())
			continue
		}

		sendMCPResponse(mcpReq["id"], result)
	}

	return scanner.Err()
}

func handleMCPMethod(method string, params interface{}, c *client.Client) (interface{}, error) {
	switch method {
	case "initialize":
		return map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "ctl_device",
				"version": "0.1.0",
			},
		}, nil

	case "notifications/initialized":
		return nil, nil

	case "tools/list":
		tools := []map[string]interface{}{
			toolToMap(protocol.ToolTaskGet),
			toolToMap(protocol.ToolTaskComplete),
			toolToMap(protocol.ToolTaskBlock),
			toolToMap(protocol.ToolTaskStatus),
			toolToMap(protocol.ToolProjectRegister),
			toolToMap(protocol.ToolProjectList),
			toolToMap(protocol.ToolTaskDispatch),
			toolToMap(protocol.ToolTaskAdvance),
			toolToMap(protocol.ToolAgentList),
		}
		return map[string]interface{}{
			"tools": tools,
		}, nil

	case "tools/call":
		paramsMap, ok := params.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid params")
		}

		name, _ := paramsMap["name"].(string)
		arguments, _ := paramsMap["arguments"].(map[string]interface{})

		return handleToolCall(name, arguments, c)

	default:
		return nil, fmt.Errorf("method not found: %s", method)
	}
}

func toolToMap(tool protocol.MCPToolSchema) map[string]interface{} {
	return map[string]interface{}{
		"name":        tool.Name,
		"description": tool.Description,
		"inputSchema": tool.InputSchema,
	}
}

func handleToolCall(name string, args map[string]interface{}, c *client.Client) (interface{}, error) {
	switch name {
	case "task_get":
		projectName, _ := args["project"].(string)
		if projectName == "" {
			return nil, fmt.Errorf("missing project")
		}

		resp, err := c.TaskGet(projectName)
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": fmt.Sprintf(`{"status": "%s", "task": %v}`, resp.Status, resp.Task),
				},
			},
		}, nil

	case "task_complete":
		projectName, _ := args["project"].(string)
		taskNum, _ := args["task_num"].(string)
		summary, _ := args["summary"].(string)
		commit, _ := args["commit"].(string)
		testOutput, _ := args["test_output"].(string)
		issues, _ := args["issues"].(string)

		report := &client.CompleteReport{
			Project:    projectName,
			TaskNum:    taskNum,
			Summary:    summary,
			Commit:     commit,
			TestOutput: testOutput,
			Issues:     issues,
		}

		if err := c.TaskComplete(report); err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"status": "ok"}`},
			},
		}, nil

	case "task_block":
		projectName, _ := args["project"].(string)
		taskNum, _ := args["task_num"].(string)
		reason, _ := args["reason"].(string)
		details, _ := args["details"].(string)

		report := &client.BlockReport{
			Project: projectName,
			TaskNum: taskNum,
			Reason:  reason,
			Details: details,
		}

		if err := c.TaskBlock(report); err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"status": "ok"}`},
			},
		}, nil

	case "task_status":
		projectName, _ := args["project"].(string)
		taskNum, _ := args["task_num"].(string)
		status, _ := args["status"].(string)

		if err := c.TaskStatus(projectName, taskNum, status); err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"status": "ok"}`},
			},
		}, nil

	case "project_register":
		req := &client.ProjectRegisterRequest{
			Name:           getString(args, "name"),
			Dir:            getString(args, "dir"),
			Tech:           getString(args, "tech"),
			TestCmd:        getString(args, "test_cmd"),
			Executor:       getString(args, "executor"),
			TimeoutMinutes: getInt(args, "timeout_minutes"),
			NotifyChannel:  getString(args, "notify_channel"),
			NotifyTarget:   getString(args, "notify_target"),
		}

		if err := c.ProjectRegister(req); err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"status": "ok"}`},
			},
		}, nil

	case "project_list":
		resp, err := c.ProjectList()
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf(`{"projects": %v, "tasks": %v}`, resp.Projects, resp.Tasks)},
			},
		}, nil

	case "task_dispatch":
		projectName, _ := args["project"].(string)
		task, _ := args["task"].(map[string]interface{})

		if err := c.TaskDispatch(projectName, task); err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"status": "ok"}`},
			},
		}, nil

	case "task_advance":
		projectName, _ := args["project"].(string)

		if err := c.TaskAdvance(projectName); err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": `{"status": "ok"}`},
			},
		}, nil

	case "agent_list":
		resp, err := c.AgentList()
		if err != nil {
			return nil, err
		}

		return map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": fmt.Sprintf(`{"agents": %v}`, resp.Agents)},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func getString(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func getInt(args map[string]interface{}, key string) int {
	if v, ok := args[key].(int); ok {
		return v
	}
	return 0
}

func sendMCPResponse(id interface{}, result interface{}) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(os.Stdout, "%s\n", data)
}

func sendMCPError(id interface{}, code int, message string) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	data, _ := json.Marshal(resp)
	fmt.Fprintf(os.Stdout, "%s\n", data)
}
