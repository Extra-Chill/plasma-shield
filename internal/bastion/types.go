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

// Grant represents a time-limited access grant for SSH bastion access.
type Grant struct {
	ID        string    `json:"id"`
	Principal string    `json:"principal"`   // who can use this grant
	Target    string    `json:"target"`      // agent or fleet pattern
	ExpiresAt time.Time `json:"expires_at"`
	CreatedBy string    `json:"created_by"`  // audit trail
	CreatedAt time.Time `json:"created_at"`
}
