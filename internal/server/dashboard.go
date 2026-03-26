package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/0xdevelop/ctl_device/internal/agent"
	"github.com/0xdevelop/ctl_device/internal/event"
	"github.com/0xdevelop/ctl_device/internal/project"
	"github.com/0xdevelop/ctl_device/pkg/protocol"
)

//go:embed static/*
var staticFiles embed.FS

type DashboardState struct {
	StartTime    time.Time            `json:"start_time"`
	Agents       []*protocol.Agent    `json:"agents"`
	Projects     []*protocol.Project  `json:"projects"`
	Tasks        map[string][]*protocol.Task `json:"tasks"`
	RecentEvents []event.Event        `json:"recent_events"`
}

type Dashboard struct {
	addr      string
	manager   *agent.Manager
	scheduler *project.Scheduler
	eventBus  *event.Bus
	startTime time.Time
	server    *http.Server
	eventCh   chan event.Event
	mu        chan struct{}
}

func NewDashboard(addr string, manager *agent.Manager, scheduler *project.Scheduler, eventBus *event.Bus) *Dashboard {
	return &Dashboard{
		addr:      addr,
		manager:   manager,
		scheduler: scheduler,
		eventBus:  eventBus,
		startTime: time.Now(),
		eventCh:   make(chan event.Event, 100),
		mu:        make(chan struct{}, 1),
	}
}

func (d *Dashboard) Start() error {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("failed to create static FS: %w", err)
	}

	mux.HandleFunc("/", d.handleIndex)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/api/state", d.handleAPIState)
	mux.HandleFunc("/stream", d.handleStream)

	d.server = &http.Server{
		Addr:    d.addr,
		Handler: mux,
	}

	go d.subscribeToEvents()

	return d.server.ListenAndServe()
}

func (d *Dashboard) Shutdown(ctx context.Context) error {
	if d.server != nil {
		return d.server.Shutdown(ctx)
	}
	return nil
}

func (d *Dashboard) subscribeToEvents() {
	eventCh := make(chan event.Event, 100)
	unsubscribe := d.eventBus.Subscribe(eventCh)
	defer unsubscribe()

	for {
		select {
		case evt := <-eventCh:
			select {
			case d.eventCh <- evt:
			default:
			}
		case <-time.After(10 * time.Minute):
			continue
		}
	}
}

func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Failed to load index.html", http.StatusInternalServerError)
		return
	}
	w.Write(data)
}

func (d *Dashboard) handleAPIState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	state := d.buildState()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		http.Error(w, "Failed to encode state", http.StatusInternalServerError)
		return
	}
}

func (d *Dashboard) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-d.eventCh:
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (d *Dashboard) buildState() *DashboardState {
	agents := d.manager.ListAgents()

	projects, err := d.scheduler.GetStore().ListProjects()
	if err != nil {
		projects = []*protocol.Project{}
	}

	tasksByProject := make(map[string][]*protocol.Task)
	for _, proj := range projects {
		tasks, err := d.scheduler.GetStore().ListTasks(proj.Name)
		if err != nil {
			tasks = []*protocol.Task{}
		}
		tasksByProject[proj.Name] = tasks
	}

	recentEvents := d.getRecentEvents(10)

	return &DashboardState{
		StartTime:    d.startTime,
		Agents:       agents,
		Projects:     projects,
		Tasks:        tasksByProject,
		RecentEvents: recentEvents,
	}
}

func (d *Dashboard) getRecentEvents(limit int) []event.Event {
	events := make([]event.Event, 0, limit)

	select {
	case <-d.mu:
		d.mu <- struct{}{}
	default:
		d.mu <- struct{}{}
	}

	for i := 0; i < limit; i++ {
		select {
		case evt := <-d.eventCh:
			events = append(events, evt)
		default:
			break
		}
	}

	<-d.mu

	return events
}
