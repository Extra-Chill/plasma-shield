package bastion

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

const DefaultLogLimit = 10000

// LogStore stores session events in memory.
type LogStore struct {
	mu     sync.RWMutex
	events []SessionEvent
	limit  int
}

// NewLogStore creates a new LogStore with a limit.
func NewLogStore(limit int) *LogStore {
	if limit <= 0 {
		limit = DefaultLogLimit
	}
	return &LogStore{
		events: make([]SessionEvent, 0, limit),
		limit:  limit,
	}
}

// Add stores a session event and logs it as JSON.
func (s *LogStore) Add(event SessionEvent) {
	data, err := json.Marshal(event)
	if err == nil {
		log.Println(string(data))
	}

	s.mu.Lock()
	s.events = append(s.events, event)
	if len(s.events) > s.limit {
		s.events = s.events[len(s.events)-s.limit:]
	}
	s.mu.Unlock()
}

// List returns a paginated list of events.
func (s *LogStore) List(offset, limit int) ([]SessionEvent, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.events)
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = total
	}

	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	result := make([]SessionEvent, end-start)
	copy(result, s.events[start:end])
	return result, total
}

// Logger logs bastion session activity.
type Logger struct {
	store         *LogStore
	mu            sync.Mutex
	sessionStarts map[string]time.Time
	now           func() time.Time
}

// NewLogger creates a new Logger.
func NewLogger(store *LogStore) *Logger {
	return NewLoggerWithClock(store, func() time.Time { return time.Now().UTC() })
}

// NewLoggerWithClock creates a Logger with a custom clock.
func NewLoggerWithClock(store *LogStore, now func() time.Time) *Logger {
	if store == nil {
		panic("bastion: nil LogStore")
	}
	if now == nil {
		panic("bastion: nil clock")
	}
	return &Logger{
		store:         store,
		sessionStarts: make(map[string]time.Time),
		now:           now,
	}
}

// LogConnect logs a new session connection.
func (l *Logger) LogConnect(sessionID, grantID, principal, target string) {
	now := l.now()

	l.mu.Lock()
	l.sessionStarts[sessionID] = now
	l.mu.Unlock()

	l.store.Add(SessionEvent{
		SessionID: sessionID,
		GrantID:   grantID,
		Principal: principal,
		Target:    target,
		Event:     SessionEventConnect,
		Timestamp: now,
	})
}

// LogDisconnect logs a session disconnect with duration data.
func (l *Logger) LogDisconnect(sessionID, grantID, principal, target string) {
	now := l.now()
	var duration time.Duration

	l.mu.Lock()
	start, ok := l.sessionStarts[sessionID]
	if ok {
		duration = now.Sub(start)
		delete(l.sessionStarts, sessionID)
	}
	l.mu.Unlock()

	l.store.Add(SessionEvent{
		SessionID: sessionID,
		GrantID:   grantID,
		Principal: principal,
		Target:    target,
		Event:     SessionEventDisconnect,
		Timestamp: now,
		Data:      duration.String(),
	})
}

// LogCommand logs a command executed during a session.
func (l *Logger) LogCommand(sessionID, grantID, principal, target, command string) {
	l.store.Add(SessionEvent{
		SessionID: sessionID,
		GrantID:   grantID,
		Principal: principal,
		Target:    target,
		Event:     SessionEventCommand,
		Timestamp: l.now(),
		Data:      command,
	})
}
