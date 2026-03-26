package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

// Client is a JSON-RPC client for interacting with ctl_device server.
type Client struct {
	ServerURL  string
	Token      string
	AgentID    string
	client     *http.Client
	sseClient  *http.Client // no timeout, for SSE long-lived connections
}

// NewClient creates a new JSON-RPC client.
func NewClient(serverURL, token, agentID string) *Client {
	return &Client{
		ServerURL: serverURL,
		Token:     token,
		AgentID:   agentID,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		sseClient: &http.Client{
			Timeout: 0, // no timeout for SSE
		},
	}
}

// call invokes a JSON-RPC method and unmarshals the result into out.
func (c *Client) call(method string, params interface{}, out interface{}) error {
	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
		Params:  params,
	}

	if c.Token != "" {
		req.Auth = &protocol.AuthToken{Token: c.Token}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest("POST", c.ServerURL+"/rpc", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var rpcResp protocol.Response
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return err
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("RPC error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if out != nil && rpcResp.Result != nil {
		resultData, err := json.Marshal(rpcResp.Result)
		if err != nil {
			return err
		}
		return json.Unmarshal(resultData, out)
	}

	return nil
}

// TaskGetResponse represents the response from task_get.
type TaskGetResponse struct {
	Task   *protocol.Task `json:"task"`
	Status string         `json:"status"`
}

// TaskGet gets the current task for a project.
func (c *Client) TaskGet(project string) (*TaskGetResponse, error) {
	var resp TaskGetResponse
	err := c.call("bridge.task.get", map[string]interface{}{
		"project": project,
	}, &resp)
	return &resp, err
}

// TaskStatusUpdate represents a task status update request.
type TaskStatusUpdate struct {
	Project  string `json:"project"`
	TaskNum  string `json:"task_num"`
	Status   string `json:"status"`
}

// TaskStatus updates the status of a task.
func (c *Client) TaskStatus(project, taskNum, status string) error {
	return c.call("bridge.task.status", TaskStatusUpdate{
		Project: project,
		TaskNum: taskNum,
		Status:  status,
	}, nil)
}

// CompleteReport represents a task completion report.
type CompleteReport struct {
	Project    string `json:"project"`
	TaskNum    string `json:"task_num"`
	Summary    string `json:"summary"`
	Commit     string `json:"commit,omitempty"`
	TestOutput string `json:"test_output,omitempty"`
	Issues     string `json:"issues,omitempty"`
}

// TaskComplete marks a task as completed.
func (c *Client) TaskComplete(report *CompleteReport) error {
	return c.call("bridge.task.complete", report, nil)
}

// BlockReport represents a task block report.
type BlockReport struct {
	Project string `json:"project"`
	TaskNum string `json:"task_num"`
	Reason  string `json:"reason"`
	Details string `json:"details,omitempty"`
}

// TaskBlock marks a task as blocked.
func (c *Client) TaskBlock(report *BlockReport) error {
	return c.call("bridge.task.block", report, nil)
}

// RegisterRequest represents an agent registration request.
type RegisterRequest struct {
	AgentID      string   `json:"agent_id"`
	Role         string   `json:"role"`
	Capabilities []string `json:"capabilities"`
	Projects     []string `json:"projects,omitempty"`
	Resume       bool     `json:"resume,omitempty"`
}

// RegisterResponse represents an agent registration response.
type RegisterResponse struct {
	OK           bool             `json:"ok"`
	PendingTasks []*protocol.Task `json:"pending_tasks,omitempty"`
}

// AgentRegister registers an agent with the server.
func (c *Client) AgentRegister(req *RegisterRequest) (*RegisterResponse, error) {
	var resp RegisterResponse
	err := c.call("bridge.agent.register", req, &resp)
	return &resp, err
}

// Heartbeat sends a heartbeat from an agent.
func (c *Client) Heartbeat(agentID string) error {
	return c.call("bridge.agent.heartbeat", map[string]interface{}{
		"agent_id": agentID,
	}, nil)
}

// AgentInfo represents agent information.
type AgentInfo struct {
	ID            string    `json:"id"`
	Role          string    `json:"role"`
	Capabilities  []string  `json:"capabilities"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
	Online        bool      `json:"online"`
	CurrentTask   string    `json:"current_task,omitempty"`
}

// AgentListResponse represents the response from agent_list.
type AgentListResponse struct {
	Agents []*AgentInfo `json:"agents"`
}

// AgentList lists all registered agents.
func (c *Client) AgentList() (*AgentListResponse, error) {
	var resp AgentListResponse
	err := c.call("bridge.agent.list", nil, &resp)
	return &resp, err
}

// ProjectInfo represents project information.
type ProjectInfo struct {
	Name           string `json:"name"`
	Dir            string `json:"dir"`
	Tech           string `json:"tech"`
	TestCmd        string `json:"test_cmd"`
	Executor       string `json:"executor"`
	TimeoutMinutes int    `json:"timeout_minutes"`
	NotifyChannel  string `json:"notify_channel"`
	NotifyTarget   string `json:"notify_target,omitempty"`
}

// ProjectRegisterRequest represents a project registration request.
type ProjectRegisterRequest struct {
	Name           string `json:"name"`
	Dir            string `json:"dir"`
	Tech           string `json:"tech,omitempty"`
	TestCmd        string `json:"test_cmd,omitempty"`
	Executor       string `json:"executor,omitempty"`
	TimeoutMinutes int    `json:"timeout_minutes,omitempty"`
	NotifyChannel  string `json:"notify_channel,omitempty"`
	NotifyTarget   string `json:"notify_target,omitempty"`
}

// ProjectRegister registers a new project.
func (c *Client) ProjectRegister(req *ProjectRegisterRequest) error {
	return c.call("bridge.project.register", req, nil)
}

// ProjectListResponse represents the response from project_list.
type ProjectListResponse struct {
	Projects []*ProjectInfo          `json:"projects"`
	Tasks    map[string][]*protocol.Task `json:"tasks"`
}

// ProjectList lists all registered projects.
func (c *Client) ProjectList() (*ProjectListResponse, error) {
	var resp ProjectListResponse
	err := c.call("bridge.project.list", nil, &resp)
	return &resp, err
}

// TaskDispatchRequest represents a task dispatch request.
type TaskDispatchRequest struct {
	Project string          `json:"project"`
	Task    json.RawMessage `json:"task"`
}

// TaskDispatch dispatches a new task to a project.
func (c *Client) TaskDispatch(project string, task interface{}) error {
	taskData, err := json.Marshal(task)
	if err != nil {
		return err
	}
	return c.call("bridge.task.dispatch", map[string]interface{}{
		"project": project,
		"task":    json.RawMessage(taskData),
	}, nil)
}

// TaskAdvance advances a project to its next pending task.
func (c *Client) TaskAdvance(project string) error {
	return c.call("bridge.task.advance", map[string]interface{}{
		"project": project,
	}, nil)
}

// SSEEvent represents an SSE event.
type SSEEvent struct {
	Type      string      `json:"type"`
	Project   string      `json:"project,omitempty"`
	AgentID   string      `json:"agent_id,omitempty"`
	TaskID    string      `json:"task_id,omitempty"`
	Payload   interface{} `json:"payload"`
	Timestamp time.Time   `json:"timestamp"`
}

// SubscribeEvents connects to the SSE endpoint and yields events.
func (c *Client) SubscribeEvents(project string) (<-chan SSEEvent, <-chan error, error) {
	eventCh := make(chan SSEEvent, 100)
	errCh := make(chan error, 1)

	u, err := url.Parse(c.ServerURL + "/events")
	if err != nil {
		return nil, nil, err
	}

	q := u.Query()
	if project != "" {
		q.Set("project", project)
	}
	u.RawQuery = q.Encode()

	httpReq, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, nil, err
	}
	if c.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.sseClient.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}

	go func() {
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue // skip SSE comments (": ping") and empty lines
			}
			data := strings.TrimPrefix(line, "data: ")
			var evt SSEEvent
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue // skip malformed lines
			}
			eventCh <- evt
		}
		if err := scanner.Err(); err != nil && err != io.EOF {
			errCh <- err
		}
	}()

	return eventCh, errCh, nil
}
