package bastion

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGrantStore_Add(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	store := NewGrantStoreWithClock("", func() time.Time { return now })

	grant := store.Add("alice", "sarai-chinwag", "admin", 30*time.Minute)

	if grant.ID == "" {
		t.Error("expected non-empty ID")
	}
	if grant.Principal != "alice" {
		t.Errorf("expected principal 'alice', got %q", grant.Principal)
	}
	if grant.Target != "sarai-chinwag" {
		t.Errorf("expected target 'sarai-chinwag', got %q", grant.Target)
	}
	if grant.CreatedBy != "admin" {
		t.Errorf("expected created_by 'admin', got %q", grant.CreatedBy)
	}
	if !grant.CreatedAt.Equal(now) {
		t.Errorf("expected created_at %v, got %v", now, grant.CreatedAt)
	}
	expectedExpiry := now.Add(30 * time.Minute)
	if !grant.ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("expected expires_at %v, got %v", expectedExpiry, grant.ExpiresAt)
	}
}

func TestGrantStore_Get(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	currentTime := now
	store := NewGrantStoreWithClock("", func() time.Time { return currentTime })

	grant := store.Add("alice", "sarai-chinwag", "admin", 30*time.Minute)

	// Get valid grant
	retrieved := store.Get(grant.ID)
	if retrieved == nil {
		t.Fatal("expected to retrieve grant")
	}
	if retrieved.ID != grant.ID {
		t.Errorf("expected ID %q, got %q", grant.ID, retrieved.ID)
	}

	// Get non-existent grant
	notFound := store.Get("nonexistent")
	if notFound != nil {
		t.Error("expected nil for non-existent grant")
	}

	// Get expired grant
	currentTime = now.Add(31 * time.Minute)
	expired := store.Get(grant.ID)
	if expired != nil {
		t.Error("expected nil for expired grant")
	}
}

func TestGrantStore_Delete(t *testing.T) {
	store := NewGrantStore("")

	grant := store.Add("alice", "sarai-chinwag", "admin", 30*time.Minute)

	// Delete existing grant
	if !store.Delete(grant.ID) {
		t.Error("expected Delete to return true for existing grant")
	}

	// Verify it's deleted
	if store.Get(grant.ID) != nil {
		t.Error("expected grant to be deleted")
	}

	// Delete non-existent grant
	if store.Delete("nonexistent") {
		t.Error("expected Delete to return false for non-existent grant")
	}
}

func TestGrantStore_List(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	currentTime := now
	store := NewGrantStoreWithClock("", func() time.Time { return currentTime })

	// Add grants
	grant1 := store.Add("alice", "sarai-chinwag", "admin", 30*time.Minute)
	grant2 := store.Add("bob", "star-fleet", "admin", 1*time.Hour)

	// List all grants
	all := store.List()
	if len(all) != 2 {
		t.Errorf("expected 2 grants, got %d", len(all))
	}

	// Advance time past first grant expiry
	currentTime = now.Add(45 * time.Minute)

	// List still returns all (including expired)
	all = store.List()
	if len(all) != 2 {
		t.Errorf("expected 2 grants from List(), got %d", len(all))
	}

	// ListActive returns only non-expired
	active := store.ListActive()
	if len(active) != 1 {
		t.Errorf("expected 1 active grant, got %d", len(active))
	}
	if active[0].ID != grant2.ID {
		t.Errorf("expected active grant to be %q, got %q", grant2.ID, active[0].ID)
	}

	_ = grant1 // use grant1 to avoid lint error
}

func TestGrantStore_ValidateAccess(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	currentTime := now
	store := NewGrantStoreWithClock("", func() time.Time { return currentTime })

	// Add grant for alice to sarai-chinwag
	store.Add("alice", "sarai-chinwag", "admin", 30*time.Minute)
	// Add wildcard grant for bob
	store.Add("bob", "*", "admin", 30*time.Minute)

	// Alice can access sarai-chinwag
	if store.ValidateAccess("alice", "sarai-chinwag") == nil {
		t.Error("expected alice to have access to sarai-chinwag")
	}

	// Alice cannot access star-fleet
	if store.ValidateAccess("alice", "star-fleet") != nil {
		t.Error("expected alice to NOT have access to star-fleet")
	}

	// Bob can access anything (wildcard)
	if store.ValidateAccess("bob", "sarai-chinwag") == nil {
		t.Error("expected bob to have access to sarai-chinwag")
	}
	if store.ValidateAccess("bob", "star-fleet") == nil {
		t.Error("expected bob to have access to star-fleet")
	}

	// Unknown user has no access
	if store.ValidateAccess("charlie", "sarai-chinwag") != nil {
		t.Error("expected charlie to have no access")
	}

	// Expired grants don't validate
	currentTime = now.Add(31 * time.Minute)
	if store.ValidateAccess("alice", "sarai-chinwag") != nil {
		t.Error("expected expired grant to not validate")
	}
}

func TestGrantStore_Cleanup(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	currentTime := now
	store := NewGrantStoreWithClock("", func() time.Time { return currentTime })

	store.Add("alice", "sarai-chinwag", "admin", 10*time.Minute)
	store.Add("bob", "star-fleet", "admin", 30*time.Minute)
	store.Add("charlie", "both", "admin", 1*time.Hour)

	// No cleanup needed yet
	if removed := store.Cleanup(); removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}

	// Advance time
	currentTime = now.Add(25 * time.Minute)

	// Cleanup should remove alice's grant
	if removed := store.Cleanup(); removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	// Only 2 grants remain
	if len(store.List()) != 2 {
		t.Errorf("expected 2 grants remaining, got %d", len(store.List()))
	}
}

func TestGrantStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "grants.json")

	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	// Create store and add grants
	store1 := NewGrantStoreWithClock(filePath, clock)
	grant1 := store1.Add("alice", "sarai-chinwag", "admin", 30*time.Minute)
	grant2 := store1.Add("bob", "star-fleet", "admin", 1*time.Hour)

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("expected grants file to be created")
	}

	// Create new store from same file
	store2 := NewGrantStoreWithClock(filePath, clock)

	// Verify grants were loaded
	loaded := store2.List()
	if len(loaded) != 2 {
		t.Errorf("expected 2 grants loaded, got %d", len(loaded))
	}

	// Verify grant data
	g1 := store2.Get(grant1.ID)
	if g1 == nil || g1.Target != "sarai-chinwag" {
		t.Error("grant1 not properly persisted")
	}
	g2 := store2.Get(grant2.ID)
	if g2 == nil || g2.Target != "star-fleet" {
		t.Error("grant2 not properly persisted")
	}

	// Delete a grant and verify persistence
	store2.Delete(grant1.ID)

	// Load again
	store3 := NewGrantStoreWithClock(filePath, clock)
	if len(store3.List()) != 1 {
		t.Errorf("expected 1 grant after delete, got %d", len(store3.List()))
	}
}

func TestMatchTarget(t *testing.T) {
	tests := []struct {
		grantTarget     string
		requestedTarget string
		expected        bool
	}{
		{"sarai-chinwag", "sarai-chinwag", true},
		{"sarai-chinwag", "star-fleet", false},
		{"*", "sarai-chinwag", true},
		{"*", "any-target", true},
		{"", "", true},
		{"", "something", false},
	}

	for _, tc := range tests {
		result := matchTarget(tc.grantTarget, tc.requestedTarget)
		if result != tc.expected {
			t.Errorf("matchTarget(%q, %q) = %v, want %v",
				tc.grantTarget, tc.requestedTarget, result, tc.expected)
		}
	}
}
