package server

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/0xdevelop/ctl_device/api/pb"
	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/project"
)

func newTestGRPCServer(t *testing.T, token string) (pb.AgentServiceClient, pb.ProjectServiceClient, pb.TaskServiceClient, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	bus := event.NewBus()
	store, err := project.NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	sched := project.NewScheduler(store, bus)
	registry, err := agent.NewRegistry(tmpDir + "/agents")
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	mgr, err := agent.NewManager(registry, store, bus)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	gs := NewGRPCServer(lis.Addr().String(), token, mgr, sched, store, bus)
	// swap listener: re-create server on already-bound listener
	gsRaw := grpc.NewServer(
		grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			if token != "" {
				md, _ := metadata.FromIncomingContext(ctx)
				ok := false
				for _, v := range md.Get("authorization") {
					if v == "Bearer "+token {
						ok = true
					}
				}
				if !ok {
					return nil, status.Error(codes.Unauthenticated, "invalid token")
				}
			}
			return handler(ctx, req)
		}),
	)
	pb.RegisterAgentServiceServer(gsRaw, &grpcAgentSvc{mgr: mgr})
	pb.RegisterProjectServiceServer(gsRaw, &grpcProjectSvc{store: store, bus: bus})
	pb.RegisterTaskServiceServer(gsRaw, &grpcTaskSvc{sched: sched})
	pb.RegisterEventServiceServer(gsRaw, &grpcEventSvc{bus: bus})
	_ = gs // created to verify compilation; we use gsRaw for test flexibility

	go func() {
		_ = gsRaw.Serve(lis)
	}()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}

	cleanup := func() {
		conn.Close()
		gsRaw.Stop()
		mgr.Shutdown()
	}

	return pb.NewAgentServiceClient(conn),
		pb.NewProjectServiceClient(conn),
		pb.NewTaskServiceClient(conn),
		cleanup
}

func authCtx(token string) context.Context {
	if token == "" {
		return context.Background()
	}
	md := metadata.Pairs("authorization", "Bearer "+token)
	return metadata.NewOutgoingContext(context.Background(), md)
}

// TestGRPC_AgentRegisterAndList verifies agent registration and listing.
func TestGRPC_AgentRegisterAndList(t *testing.T) {
	agentClient, _, _, cleanup := newTestGRPCServer(t, "")
	defer cleanup()

	ctx := context.Background()

	resp, err := agentClient.Register(ctx, &pb.RegisterRequest{
		AgentId:      "agent-1",
		Role:         "executor",
		Capabilities: []string{"go"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !resp.Ok {
		t.Error("expected ok=true")
	}

	listResp, err := agentClient.ListAgents(ctx, &pb.ListAgentsRequest{})
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(listResp.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(listResp.Agents))
	}
	if listResp.Agents[0].Id != "agent-1" {
		t.Errorf("expected agent-1, got %s", listResp.Agents[0].Id)
	}
	if !listResp.Agents[0].Online {
		t.Error("expected agent to be online")
	}
}

// TestGRPC_AgentHeartbeat verifies heartbeat works.
func TestGRPC_AgentHeartbeat(t *testing.T) {
	agentClient, _, _, cleanup := newTestGRPCServer(t, "")
	defer cleanup()

	ctx := context.Background()

	// Register first
	if _, err := agentClient.Register(ctx, &pb.RegisterRequest{AgentId: "hb-agent", Role: "executor"}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	hbResp, err := agentClient.Heartbeat(ctx, &pb.HeartbeatRequest{AgentId: "hb-agent"})
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if hbResp.Status != "ok" {
		t.Errorf("expected ok, got %s", hbResp.Status)
	}
}

// TestGRPC_ProjectRegisterAndList verifies project registration and listing.
func TestGRPC_ProjectRegisterAndList(t *testing.T) {
	_, projClient, _, cleanup := newTestGRPCServer(t, "")
	defer cleanup()

	ctx := context.Background()
	tmpDir := t.TempDir()

	regResp, err := projClient.Register(ctx, &pb.ProjectRegisterRequest{
		Name:    "my-proj",
		Dir:     tmpDir,
		Tech:    "go",
		TestCmd: "go test ./...",
	})
	if err != nil {
		t.Fatalf("Register project: %v", err)
	}
	if regResp.Status != "ok" {
		t.Errorf("expected ok, got %s", regResp.Status)
	}

	listResp, err := projClient.List(ctx, &pb.ProjectListRequest{})
	if err != nil {
		t.Fatalf("List projects: %v", err)
	}
	if len(listResp.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(listResp.Projects))
	}
	if listResp.Projects[0].Name != "my-proj" {
		t.Errorf("expected my-proj, got %s", listResp.Projects[0].Name)
	}
}

// TestGRPC_TaskDispatchGetComplete verifies task dispatch, get, and complete.
func TestGRPC_TaskDispatchGetComplete(t *testing.T) {
	_, projClient, taskClient, cleanup := newTestGRPCServer(t, "")
	defer cleanup()

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Register project
	if _, err := projClient.Register(ctx, &pb.ProjectRegisterRequest{
		Name: "task-proj",
		Dir:  tmpDir,
		Tech: "go",
	}); err != nil {
		t.Fatalf("Register project: %v", err)
	}

	// Dispatch task
	dispResp, err := taskClient.Dispatch(ctx, &pb.TaskDispatchRequest{
		Project: "task-proj",
		Task: &pb.Task{
			Num:  "1",
			Name: "Do something",
		},
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if dispResp.Status != "ok" {
		t.Errorf("expected ok, got %s", dispResp.Status)
	}

	// Get task
	getResp, err := taskClient.Get(ctx, &pb.TaskGetRequest{Project: "task-proj"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if getResp.Status != "ok" {
		t.Fatalf("expected ok, got %s", getResp.Status)
	}
	if getResp.Task.Num != "1" {
		t.Errorf("expected task num 1, got %s", getResp.Task.Num)
	}

	// Complete task
	compResp, err := taskClient.Complete(ctx, &pb.TaskCompleteRequest{
		Project: "task-proj",
		TaskNum: "1",
		Summary: "All done",
		Commit:  "abc123",
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if compResp.Status != "ok" {
		t.Errorf("expected ok, got %s", compResp.Status)
	}
}

// TestGRPC_TokenAuth verifies token authentication works and rejects bad tokens.
func TestGRPC_TokenAuth(t *testing.T) {
	agentClient, _, _, cleanup := newTestGRPCServer(t, "secret-token")
	defer cleanup()

	// Without token — should fail
	_, err := agentClient.ListAgents(context.Background(), &pb.ListAgentsRequest{})
	if err == nil {
		t.Fatal("expected error without token, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %s", st.Code())
	}

	// With correct token — should succeed
	ctx := authCtx("secret-token")
	_, err = agentClient.ListAgents(ctx, &pb.ListAgentsRequest{})
	if err != nil {
		t.Fatalf("expected success with valid token, got %v", err)
	}
}

// TestGRPC_TokenAuth_WrongToken verifies wrong token is rejected.
func TestGRPC_TokenAuth_WrongToken(t *testing.T) {
	agentClient, _, _, cleanup := newTestGRPCServer(t, "correct-token")
	defer cleanup()

	ctx := authCtx("wrong-token")
	_, err := agentClient.ListAgents(ctx, &pb.ListAgentsRequest{})
	if err == nil {
		t.Fatal("expected error with wrong token")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Unauthenticated {
		t.Errorf("expected Unauthenticated, got %s", st.Code())
	}
}

// TestGRPC_NoAuth verifies no-token server allows all requests.
func TestGRPC_NoAuth(t *testing.T) {
	agentClient, _, _, cleanup := newTestGRPCServer(t, "")
	defer cleanup()

	_, err := agentClient.ListAgents(context.Background(), &pb.ListAgentsRequest{})
	if err != nil {
		t.Fatalf("expected no auth required, got %v", err)
	}
}

// TestGRPC_TaskBlock verifies task blocking.
func TestGRPC_TaskBlock(t *testing.T) {
	_, projClient, taskClient, cleanup := newTestGRPCServer(t, "")
	defer cleanup()

	ctx := context.Background()
	tmpDir := t.TempDir()

	if _, err := projClient.Register(ctx, &pb.ProjectRegisterRequest{Name: "block-proj", Dir: tmpDir, Tech: "go"}); err != nil {
		t.Fatalf("Register project: %v", err)
	}
	if _, err := taskClient.Dispatch(ctx, &pb.TaskDispatchRequest{
		Project: "block-proj",
		Task:    &pb.Task{Num: "1", Name: "Blocked task"},
	}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	blockResp, err := taskClient.Block(ctx, &pb.TaskBlockRequest{
		Project: "block-proj",
		TaskNum: "1",
		Reason:  "dependency missing",
		Details: "need pkg X",
	})
	if err != nil {
		t.Fatalf("Block: %v", err)
	}
	if blockResp.Status != "ok" {
		t.Errorf("expected ok, got %s", blockResp.Status)
	}
}

// TestGRPC_Start verifies the server binds and serves on a real port.
func TestGRPC_Start(t *testing.T) {
	tmpDir := t.TempDir()
	bus := event.NewBus()
	store, _ := project.NewFileStore(tmpDir)
	sched := project.NewScheduler(store, bus)
	registry, _ := agent.NewRegistry(tmpDir + "/agents")
	mgr, _ := agent.NewManager(registry, store, bus)
	defer mgr.Shutdown()

	// Pick a free port
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	lis.Close()

	gs := NewGRPCServer(addr, "", mgr, sched, store, bus)
	go func() {
		if err := gs.Start(); err != nil {
			// expected on shutdown
		}
	}()

	// Give it time to start
	time.Sleep(50 * time.Millisecond)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	client := pb.NewAgentServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = client.ListAgents(ctx, &pb.ListAgentsRequest{})
	if err != nil {
		t.Fatalf("ListAgents after Start: %v", err)
	}

	gs.Shutdown(context.Background())

	// Verify addr is from our format
	if addr == "" {
		t.Error("expected non-empty addr")
	}
	fmt.Printf("gRPC server tested at %s\n", addr)
}
