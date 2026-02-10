package bastion

import (
	"testing"
	"time"
)

func TestLoggerConnectDisconnect(t *testing.T) {
	store := NewLogStore(10)
	now := time.Date(2025, 2, 10, 18, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	logger := NewLoggerWithClock(store, clock)

	logger.LogConnect("session-1", "grant-1", "alice", "agent-1")
	now = now.Add(2 * time.Second)
	logger.LogDisconnect("session-1", "grant-1", "alice", "agent-1")

	events, total := store.List(0, 10)
	if total != 2 {
		t.Fatalf("expected 2 events, got %d", total)
	}
	if events[0].Event != SessionEventConnect {
		t.Fatalf("expected first event connect, got %q", events[0].Event)
	}
	if events[1].Event != SessionEventDisconnect {
		t.Fatalf("expected second event disconnect, got %q", events[1].Event)
	}
	if events[1].Data != "2s" {
		t.Fatalf("expected duration 2s, got %q", events[1].Data)
	}
}

func TestLoggerCommand(t *testing.T) {
	store := NewLogStore(10)
	logger := NewLoggerWithClock(store, func() time.Time {
		return time.Date(2025, 2, 10, 18, 0, 0, 0, time.UTC)
	})

	logger.LogCommand("session-2", "grant-2", "bob", "agent-2", "ls -la")

	events, total := store.List(0, 10)
	if total != 1 {
		t.Fatalf("expected 1 event, got %d", total)
	}
	if events[0].Event != SessionEventCommand {
		t.Fatalf("expected command event, got %q", events[0].Event)
	}
	if events[0].Data != "ls -la" {
		t.Fatalf("expected command data, got %q", events[0].Data)
	}
}

func TestLogStoreLimit(t *testing.T) {
	store := NewLogStore(2)

	store.Add(SessionEvent{SessionID: "one"})
	store.Add(SessionEvent{SessionID: "two"})
	store.Add(SessionEvent{SessionID: "three"})

	events, total := store.List(0, 10)
	if total != 2 {
		t.Fatalf("expected 2 events, got %d", total)
	}
	if events[0].SessionID != "two" || events[1].SessionID != "three" {
		t.Fatalf("unexpected session order: %+v", events)
	}
}
