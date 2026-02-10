package bastion

import "time"

// SessionEvent represents a single bastion session event.
type SessionEvent struct {
	SessionID string    `json:"session_id"`
	GrantID   string    `json:"grant_id"`
	Principal string    `json:"principal"`
	Target    string    `json:"target"`
	Event     string    `json:"event"` // connect, disconnect, command
	Timestamp time.Time `json:"timestamp"`
	Data      string    `json:"data,omitempty"`
}

const (
	SessionEventConnect    = "connect"
	SessionEventDisconnect = "disconnect"
	SessionEventCommand    = "command"
)
