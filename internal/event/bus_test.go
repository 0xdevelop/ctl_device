package event

import (
	"testing"
	"time"
)

func TestBus_PublishAndSubscribe(t *testing.T) {
	bus := NewBus()
	ch := make(chan Event, 10)
	unsubscribe := bus.Subscribe(ch)
	defer unsubscribe()

	expected := Event{
		Type:      EventTaskStatusChanged,
		Project:   "test-project",
		TaskID:    "test-01",
		Payload:   map[string]string{"status": "executing"},
		Timestamp: time.Now(),
	}

	bus.Publish(expected)

	select {
	case received := <-ch:
		if received.Type != expected.Type {
			t.Errorf("expected type %v, got %v", expected.Type, received.Type)
		}
		if received.Project != expected.Project {
			t.Errorf("expected project %v, got %v", expected.Project, received.Project)
		}
		if received.TaskID != expected.TaskID {
			t.Errorf("expected taskID %v, got %v", expected.TaskID, received.TaskID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := NewBus()
	ch1 := make(chan Event, 10)
	ch2 := make(chan Event, 10)
	unsubscribe1 := bus.Subscribe(ch1)
	unsubscribe2 := bus.Subscribe(ch2)
	defer unsubscribe1()
	defer unsubscribe2()

	event := Event{
		Type:      EventAgentOnline,
		AgentID:   "agent-1",
		Timestamp: time.Now(),
	}

	bus.Publish(event)

	for i, ch := range []chan Event{ch1, ch2} {
		select {
		case received := <-ch:
			if received.AgentID != event.AgentID {
				t.Errorf("subscriber %d: expected agentID %v, got %v", i, event.AgentID, received.AgentID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("subscriber %d: timeout waiting for event", i)
		}
	}
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus()
	ch := make(chan Event, 10)
	unsubscribe := bus.Subscribe(ch)

	event := Event{
		Type:      EventTaskCompleted,
		Timestamp: time.Now(),
	}
	bus.Publish(event)

	select {
	case <-ch:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for first event")
	}

	unsubscribe()

	bus.Publish(event)

	time.Sleep(50 * time.Millisecond)
	select {
	case val, ok := <-ch:
		if ok {
			t.Fatalf("received event after unsubscribe: %v", val)
		}
	default:
	}
}

func TestBus_FilterByEventType(t *testing.T) {
	bus := NewBus()
	ch := make(chan Event, 10)
	unsubscribe := bus.Subscribe(ch, EventAgentOnline, EventAgentOffline)
	defer unsubscribe()

	event1 := Event{
		Type:      EventAgentOnline,
		AgentID:   "agent-1",
		Timestamp: time.Now(),
	}
	event2 := Event{
		Type:      EventTaskCompleted,
		Timestamp: time.Now(),
	}

	bus.Publish(event1)
	bus.Publish(event2)

	select {
	case received := <-ch:
		if received.Type != EventAgentOnline {
			t.Errorf("expected EventAgentOnline, got %v", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for filtered event")
	}

	select {
	case <-ch:
		t.Fatal("received event that should have been filtered out")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestBus_AgentOfflineEvent(t *testing.T) {
	bus := NewBus()
	ch := make(chan Event, 10)
	unsubscribe := bus.Subscribe(ch, EventAgentOffline)
	defer unsubscribe()

	event := Event{
		Type:      EventAgentOffline,
		AgentID:   "agent-1",
		TaskID:    "project-01",
		Timestamp: time.Now(),
	}

	bus.Publish(event)

	select {
	case received := <-ch:
		if received.Type != EventAgentOffline {
			t.Errorf("expected EventAgentOffline, got %v", received.Type)
		}
		if received.AgentID != event.AgentID {
			t.Errorf("expected agentID %v, got %v", event.AgentID, received.AgentID)
		}
		if received.TaskID != event.TaskID {
			t.Errorf("expected taskID %v, got %v", event.TaskID, received.TaskID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}
}
