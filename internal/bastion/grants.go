package bastion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// GrantStore stores access grants in memory with JSON file persistence.
type GrantStore struct {
	mu       sync.RWMutex
	grants   map[string]*Grant
	filePath string
	now      func() time.Time
	counter  int64
}

// NewGrantStore creates a new GrantStore with optional file persistence.
// If filePath is empty, grants are only stored in memory.
func NewGrantStore(filePath string) *GrantStore {
	return NewGrantStoreWithClock(filePath, func() time.Time { return time.Now().UTC() })
}

// NewGrantStoreWithClock creates a GrantStore with a custom clock (for testing).
func NewGrantStoreWithClock(filePath string, now func() time.Time) *GrantStore {
	if now == nil {
		panic("bastion: nil clock")
	}
	s := &GrantStore{
		grants:   make(map[string]*Grant),
		filePath: filePath,
		now:      now,
	}
	if filePath != "" {
		s.load()
	}
	return s
}

// Add creates a new grant and persists it.
func (s *GrantStore) Add(principal, target, createdBy string, duration time.Duration) *Grant {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.counter++
	grant := &Grant{
		ID:        generateGrantID(now, s.counter),
		Principal: principal,
		Target:    target,
		ExpiresAt: now.Add(duration),
		CreatedBy: createdBy,
		CreatedAt: now,
	}

	s.grants[grant.ID] = grant
	s.persist()
	return grant
}

// Get retrieves a grant by ID. Returns nil if not found or expired.
func (s *GrantStore) Get(id string) *Grant {
	s.mu.RLock()
	defer s.mu.RUnlock()

	grant, exists := s.grants[id]
	if !exists {
		return nil
	}
	if s.now().After(grant.ExpiresAt) {
		return nil
	}
	return grant
}

// Delete removes a grant by ID. Returns true if the grant existed.
func (s *GrantStore) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.grants[id]; !exists {
		return false
	}

	delete(s.grants, id)
	s.persist()
	return true
}

// List returns all grants (including expired ones).
func (s *GrantStore) List() []*Grant {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Grant, 0, len(s.grants))
	for _, g := range s.grants {
		result = append(result, g)
	}
	return result
}

// ListActive returns only non-expired grants.
func (s *GrantStore) ListActive() []*Grant {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := s.now()
	result := make([]*Grant, 0)
	for _, g := range s.grants {
		if now.Before(g.ExpiresAt) {
			result = append(result, g)
		}
	}
	return result
}

// ValidateAccess checks if a principal has an active grant for a target.
func (s *GrantStore) ValidateAccess(principal, target string) *Grant {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := s.now()
	for _, g := range s.grants {
		if now.After(g.ExpiresAt) {
			continue
		}
		if g.Principal == principal && matchTarget(g.Target, target) {
			return g
		}
	}
	return nil
}

// Cleanup removes all expired grants.
func (s *GrantStore) Cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	removed := 0
	for id, g := range s.grants {
		if now.After(g.ExpiresAt) {
			delete(s.grants, id)
			removed++
		}
	}
	if removed > 0 {
		s.persist()
	}
	return removed
}

// persist saves grants to the JSON file (must be called with lock held).
func (s *GrantStore) persist() {
	if s.filePath == "" {
		return
	}

	grants := make([]*Grant, 0, len(s.grants))
	for _, g := range s.grants {
		grants = append(grants, g)
	}

	data, err := json.MarshalIndent(grants, "", "  ")
	if err != nil {
		return
	}

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	os.MkdirAll(dir, 0755)

	// Write atomically
	tmpFile := s.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return
	}
	os.Rename(tmpFile, s.filePath)
}

// load reads grants from the JSON file.
func (s *GrantStore) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}

	var grants []*Grant
	if err := json.Unmarshal(data, &grants); err != nil {
		return
	}

	for _, g := range grants {
		s.grants[g.ID] = g
	}
}

// matchTarget checks if a grant target matches a requested target.
// Supports exact match and wildcard (*) patterns.
func matchTarget(grantTarget, requestedTarget string) bool {
	if grantTarget == "*" {
		return true
	}
	if grantTarget == requestedTarget {
		return true
	}
	// Could extend to support glob patterns later
	return false
}

func generateGrantID(t time.Time, counter int64) string {
	return "grant-" + strconv.FormatInt(t.UnixNano(), 36) + "-" + strconv.FormatInt(counter, 36)
}
