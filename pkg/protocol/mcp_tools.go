package protocol

// MCPToolSchema defines the JSON Schema for a single MCP tool.
type MCPToolSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolTaskGet is the schema for the task_get tool (executor).
var ToolTaskGet = MCPToolSchema{
	Name:        "task_get",
	Description: "Get the next pending task for a project. Used by executor agents to claim work.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project": map[string]interface{}{
				"type":        "string",
				"description": "Project name",
			},
		},
		"required": []string{"project"},
	},
}

// ToolTaskComplete is the schema for the task_complete tool (executor).
var ToolTaskComplete = MCPToolSchema{
	Name:        "task_complete",
	Description: "Mark a task as completed and submit results.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project": map[string]interface{}{
				"type":        "string",
				"description": "Project name",
			},
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "Summary of what was done",
			},
			"commit": map[string]interface{}{
				"type":        "string",
				"description": "Git commit hash or message",
			},
			"test_output": map[string]interface{}{
				"type":        "string",
				"description": "Output from running tests",
			},
			"issues": map[string]interface{}{
				"type":        "string",
				"description": "Any issues encountered (optional)",
			},
		},
		"required": []string{"project", "summary"},
	},
}

// ToolTaskBlock is the schema for the task_block tool (executor).
var ToolTaskBlock = MCPToolSchema{
	Name:        "task_block",
	Description: "Mark a task as blocked and report the blocking reason.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project": map[string]interface{}{
				"type":        "string",
				"description": "Project name",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "Short reason for blocking",
			},
			"details": map[string]interface{}{
				"type":        "string",
				"description": "Detailed description of the blocker",
			},
		},
		"required": []string{"project", "reason"},
	},
}

// ToolTaskStatus is the schema for the task_status tool (executor).
var ToolTaskStatus = MCPToolSchema{
	Name:        "task_status",
	Description: "Query tasks by project and status.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project": map[string]interface{}{
				"type":        "string",
				"description": "Project name",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Task status filter (e.g. pending, executing, completed)",
			},
		},
		"required": []string{"project"},
	},
}

// ToolProjectRegister is the schema for the project_register tool (scheduler).
var ToolProjectRegister = MCPToolSchema{
	Name:        "project_register",
	Description: "Register a new project with the coordination server.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Unique project name",
			},
			"dir": map[string]interface{}{
				"type":        "string",
				"description": "Absolute path to project directory",
			},
			"tech": map[string]interface{}{
				"type":        "string",
				"description": "Technology stack (e.g. go, node, python)",
			},
			"test_cmd": map[string]interface{}{
				"type":        "string",
				"description": "Command to run tests",
			},
			"executor": map[string]interface{}{
				"type":        "string",
				"description": "Executor agent ID or 'any'",
			},
			"timeout_minutes": map[string]interface{}{
				"type":        "integer",
				"description": "Default task timeout in minutes",
			},
		},
		"required": []string{"name", "dir"},
	},
}

// ToolProjectList is the schema for the project_list tool (scheduler).
var ToolProjectList = MCPToolSchema{
	Name:        "project_list",
	Description: "List all registered projects and their current status.",
	InputSchema: map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	},
}

// ToolTaskDispatch is the schema for the task_dispatch tool (scheduler).
var ToolTaskDispatch = MCPToolSchema{
	Name:        "task_dispatch",
	Description: "Dispatch a new task to a project queue.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project": map[string]interface{}{
				"type":        "string",
				"description": "Project name",
			},
			"task": map[string]interface{}{
				"type":        "object",
				"description": "Task object to dispatch",
			},
		},
		"required": []string{"project", "task"},
	},
}

// ToolTaskAdvance is the schema for the task_advance tool (scheduler).
var ToolTaskAdvance = MCPToolSchema{
	Name:        "task_advance",
	Description: "Advance a project to its next pending task.",
	InputSchema: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project": map[string]interface{}{
				"type":        "string",
				"description": "Project name",
			},
		},
		"required": []string{"project"},
	},
}

// ToolAgentList is the schema for the agent_list tool (scheduler).
var ToolAgentList = MCPToolSchema{
	Name:        "agent_list",
	Description: "List all registered agents and their current status.",
	InputSchema: map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	},
}

// AllTools returns all MCP tool schemas.
func AllTools() []MCPToolSchema {
	return []MCPToolSchema{
		ToolTaskGet,
		ToolTaskComplete,
		ToolTaskBlock,
		ToolTaskStatus,
		ToolProjectRegister,
		ToolProjectList,
		ToolTaskDispatch,
		ToolTaskAdvance,
		ToolAgentList,
	}
}
