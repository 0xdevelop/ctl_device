package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/0xdevelop/ctl_device/api/pb"
	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/project"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

// GRPCServer is the top-level container that owns the grpc.Server and auth logic.
type GRPCServer struct {
	addr     string
	token    string
	server   *grpc.Server
	eventSvc *grpcEventSvc
}

// NewGRPCServer creates and configures a new GRPCServer.
func NewGRPCServer(addr string, token string, mgr *agent.Manager, sched *project.Scheduler, store *project.FileStore, bus *event.Bus) *GRPCServer {
	g := &GRPCServer{addr: addr, token: token}

	checkAuth := func(ctx context.Context) error {
		if token == "" {
			return nil
		}
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return status.Error(codes.Unauthenticated, "missing metadata")
		}
		for _, v := range md.Get("authorization") {
			if strings.HasPrefix(v, "Bearer ") && strings.TrimPrefix(v, "Bearer ") == token {
				return nil
			}
		}
		return status.Error(codes.Unauthenticated, "invalid or missing token")
	}

	g.server = grpc.NewServer(
		grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			if err := checkAuth(ctx); err != nil {
				return nil, err
			}
			return handler(ctx, req)
		}),
		grpc.StreamInterceptor(func(srv interface{}, ss grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			if err := checkAuth(ss.Context()); err != nil {
				return err
			}
			return handler(srv, ss)
		}),
	)

	pb.RegisterAgentServiceServer(g.server, &grpcAgentSvc{mgr: mgr})
	pb.RegisterProjectServiceServer(g.server, &grpcProjectSvc{store: store, bus: bus})
	pb.RegisterTaskServiceServer(g.server, &grpcTaskSvc{sched: sched})
	g.eventSvc = &grpcEventSvc{bus: bus}
	pb.RegisterEventServiceServer(g.server, g.eventSvc)

	return g
}

// Start begins listening and serving gRPC connections.
func (g *GRPCServer) Start() error {
	lis, err := net.Listen("tcp", g.addr)
	if err != nil {
		return fmt.Errorf("grpc listen: %w", err)
	}
	return g.server.Serve(lis)
}

// Shutdown gracefully stops the gRPC server.
func (g *GRPCServer) Shutdown(_ context.Context) error {
	g.server.GracefulStop()
	return nil
}

// --- AgentService ---

type grpcAgentSvc struct {
	pb.UnimplementedAgentServiceServer
	mgr *agent.Manager
}

func (s *grpcAgentSvc) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	resp, err := s.mgr.Register(&agent.RegisterRequest{
		AgentID:      req.AgentId,
		Role:         req.Role,
		Capabilities: req.Capabilities,
		Projects:     req.Projects,
		Resume:       req.Resume,
	})
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	pbResp := &pb.RegisterResponse{Ok: resp.OK}
	for _, t := range resp.PendingTasks {
		pbResp.PendingTasks = append(pbResp.PendingTasks, protocolTaskToPB(t))
	}
	return pbResp, nil
}

func (s *grpcAgentSvc) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if err := s.mgr.Heartbeat(req.AgentId); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &pb.HeartbeatResponse{Status: "ok"}, nil
}

func (s *grpcAgentSvc) ListAgents(ctx context.Context, req *pb.ListAgentsRequest) (*pb.ListAgentsResponse, error) {
	agents := s.mgr.ListAgents()
	resp := &pb.ListAgentsResponse{}
	for _, a := range agents {
		resp.Agents = append(resp.Agents, &pb.Agent{
			Id:            a.ID,
			Role:          string(a.Role),
			Capabilities:  a.Capabilities,
			LastHeartbeat: a.LastHeartbeat.Format(time.RFC3339),
			Online:        a.Online,
			CurrentTask:   a.CurrentTask,
		})
	}
	return resp, nil
}

// --- ProjectService ---

type grpcProjectSvc struct {
	pb.UnimplementedProjectServiceServer
	store *project.FileStore
	bus   *event.Bus
}

func (s *grpcProjectSvc) Register(ctx context.Context, req *pb.ProjectRegisterRequest) (*pb.ProjectRegisterResponse, error) {
	proj := &protocol.Project{
		Name:           req.Name,
		Dir:            req.Dir,
		Tech:           req.Tech,
		TestCmd:        req.TestCmd,
		Executor:       req.Executor,
		TimeoutMinutes: int(req.TimeoutMinutes),
		NotifyChannel:  req.NotifyChannel,
		NotifyTarget:   req.NotifyTarget,
	}
	if err := s.store.SaveProject(proj); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	s.bus.Publish(event.Event{
		Type:      event.EventProjectRegistered,
		Project:   req.Name,
		Timestamp: time.Now(),
	})
	return &pb.ProjectRegisterResponse{Status: "ok"}, nil
}

func (s *grpcProjectSvc) List(ctx context.Context, req *pb.ProjectListRequest) (*pb.ProjectListResponse, error) {
	projects, err := s.store.ListProjects()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	resp := &pb.ProjectListResponse{Tasks: make(map[string]*pb.TaskList)}
	for _, p := range projects {
		resp.Projects = append(resp.Projects, &pb.Project{
			Name:           p.Name,
			Dir:            p.Dir,
			Tech:           p.Tech,
			TestCmd:        p.TestCmd,
			Executor:       p.Executor,
			TimeoutMinutes: int32(p.TimeoutMinutes),
			NotifyChannel:  p.NotifyChannel,
			NotifyTarget:   p.NotifyTarget,
		})
		tasks, _ := s.store.ListTasks(p.Name)
		tl := &pb.TaskList{}
		for _, t := range tasks {
			tl.Tasks = append(tl.Tasks, protocolTaskToPB(t))
		}
		resp.Tasks[p.Name] = tl
	}
	return resp, nil
}

// --- TaskService ---

type grpcTaskSvc struct {
	pb.UnimplementedTaskServiceServer
	sched *project.Scheduler
}

func (s *grpcTaskSvc) Get(ctx context.Context, req *pb.TaskGetRequest) (*pb.TaskGetResponse, error) {
	task, err := s.sched.GetCurrentTask(req.Project)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if task == nil {
		return &pb.TaskGetResponse{Status: "no_pending_tasks"}, nil
	}
	return &pb.TaskGetResponse{Status: "ok", Task: protocolTaskToPB(task)}, nil
}

func (s *grpcTaskSvc) UpdateStatus(ctx context.Context, req *pb.TaskStatusRequest) (*pb.TaskStatusResponse, error) {
	if err := s.sched.UpdateTaskStatus(req.Project, req.TaskNum, protocol.TaskStatus(req.Status)); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &pb.TaskStatusResponse{Status: "ok"}, nil
}

func (s *grpcTaskSvc) Complete(ctx context.Context, req *pb.TaskCompleteRequest) (*pb.TaskCompleteResponse, error) {
	report := req.Summary
	if req.TestOutput != "" {
		report += "\n\nTest Output:\n" + req.TestOutput
	}
	if req.Issues != "" {
		report += "\n\nIssues:\n" + req.Issues
	}
	if err := s.sched.CompleteTask(req.Project, req.TaskNum, req.Commit, report); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &pb.TaskCompleteResponse{Status: "ok"}, nil
}

func (s *grpcTaskSvc) Block(ctx context.Context, req *pb.TaskBlockRequest) (*pb.TaskBlockResponse, error) {
	reason := req.Reason
	if req.Details != "" {
		reason += ": " + req.Details
	}
	if err := s.sched.BlockTask(req.Project, req.TaskNum, reason); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &pb.TaskBlockResponse{Status: "ok"}, nil
}

func (s *grpcTaskSvc) Dispatch(ctx context.Context, req *pb.TaskDispatchRequest) (*pb.TaskDispatchResponse, error) {
	if req.Task == nil {
		return nil, status.Error(codes.InvalidArgument, "missing task")
	}
	task := pbTaskToProtocol(req.Project, req.Task)
	if err := s.sched.Dispatch(req.Project, task); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pb.TaskDispatchResponse{Status: "ok"}, nil
}

func (s *grpcTaskSvc) Advance(ctx context.Context, req *pb.TaskAdvanceRequest) (*pb.TaskAdvanceResponse, error) {
	if err := s.sched.Advance(req.Project); err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}
	return &pb.TaskAdvanceResponse{Status: "ok"}, nil
}

// --- EventService ---

type grpcEventSvc struct {
	pb.UnimplementedEventServiceServer
	bus *event.Bus
}

func (s *grpcEventSvc) Subscribe(req *pb.SubscribeRequest, stream pb.EventService_SubscribeServer) error {
	eventCh := make(chan event.Event, 100)
	unsub := s.bus.Subscribe(eventCh)
	defer unsub()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case evt, ok := <-eventCh:
			if !ok {
				return nil
			}
			if req.Project != "" && evt.Project != "" && evt.Project != req.Project {
				continue
			}
			payloadBytes, _ := json.Marshal(evt.Payload)
			if err := stream.Send(&pb.EventResponse{
				Type:      string(evt.Type),
				Project:   evt.Project,
				AgentId:   evt.AgentID,
				TaskId:    evt.TaskID,
				Payload:   string(payloadBytes),
				Timestamp: evt.Timestamp.Format(time.RFC3339),
			}); err != nil {
				return err
			}
		}
	}
}

// --- helpers ---

func protocolTaskToPB(t *protocol.Task) *pb.Task {
	if t == nil {
		return nil
	}
	return &pb.Task{
		Id:                 t.ID,
		Project:            t.Project,
		Num:                t.Num,
		Name:               t.Name,
		Description:        t.Description,
		AcceptanceCriteria: t.AcceptanceCriteria,
		ContextFiles:       t.ContextFiles,
		Status:             string(t.Status),
		AssignedTo:         t.AssignedTo,
		StartedAt:          t.StartedAt.Format(time.RFC3339),
		UpdatedAt:          t.UpdatedAt.Format(time.RFC3339),
		Commit:             t.Commit,
		Report:             t.Report,
		TimeoutMinutes:     int32(t.TimeoutMinutes),
	}
}

func pbTaskToProtocol(projectName string, t *pb.Task) *protocol.Task {
	task := &protocol.Task{
		Project:            projectName,
		Num:                t.Num,
		Name:               t.Name,
		Description:        t.Description,
		AcceptanceCriteria: t.AcceptanceCriteria,
		ContextFiles:       t.ContextFiles,
		TimeoutMinutes:     int(t.TimeoutMinutes),
		UpdatedAt:          time.Now(),
	}
	task.ID = fmt.Sprintf("%s:%s", projectName, t.Num)
	return task
}
