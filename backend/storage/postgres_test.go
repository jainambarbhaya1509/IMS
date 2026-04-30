package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jainambarbhaya1509/ims/models"
	"github.com/jainambarbhaya1509/ims/storage"
)

// setupTestDB creates a real Postgres connection for integration tests.
// Run with: go test ./storage/... (requires Docker running)
func setupTestDB(t *testing.T) *storage.Postgres {
	t.Helper()
	pg := storage.NewPostgres()
	return pg
}

// ─────────────────────────────────────────
// RCA Validation Tests
// ─────────────────────────────────────────

func TestSubmitRCA_RejectsEmptyRootCause(t *testing.T) {
	pg := setupTestDB(t)
	ctx := context.Background()

	// Create a work item first
	wi := models.WorkItem{
		ID:          uuid.NewString(),
		ComponentID: "RDBMS",
		Severity:    "P0",
		Status:      "OPEN",
		SignalCount:  1,
		StartTime:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	pg.CreateWorkItem(ctx, wi)

	// Try to submit RCA with empty RootCauseCategory
	rca := models.RCA{
		ID:                uuid.NewString(),
		WorkItemID:        wi.ID,
		StartTime:         time.Now().Add(-1 * time.Hour),
		EndTime:           time.Now(),
		RootCauseCategory: "", // ← intentionally empty
		FixApplied:        "Restarted the DB",
		PreventionSteps:   "Add connection pooling",
		SubmittedAt:       time.Now(),
	}

	err := pg.SubmitRCA(ctx, rca)

	// We EXPECT an error here
	if err == nil {
		t.Fatal("Expected error for empty RootCauseCategory, got nil")
	}
	t.Logf("Correctly rejected: %v", err)
}

func TestSubmitRCA_RejectsEmptyFixApplied(t *testing.T) {
	pg := setupTestDB(t)
	ctx := context.Background()

	wi := models.WorkItem{
		ID: uuid.NewString(), ComponentID: "CACHE_CLUSTER",
		Severity: "P2", Status: "OPEN",
		SignalCount: 1, StartTime: time.Now(), UpdatedAt: time.Now(),
	}
	pg.CreateWorkItem(ctx, wi)

	rca := models.RCA{
		ID:                uuid.NewString(),
		WorkItemID:        wi.ID,
		StartTime:         time.Now().Add(-30 * time.Minute),
		EndTime:           time.Now(),
		RootCauseCategory: "Infrastructure",
		FixApplied:        "", // ← intentionally empty
		PreventionSteps:   "Add monitoring",
		SubmittedAt:       time.Now(),
	}

	err := pg.SubmitRCA(ctx, rca)
	if err == nil {
		t.Fatal("Expected error for empty FixApplied, got nil")
	}
	t.Logf("Correctly rejected: %v", err)
}

func TestSubmitRCA_AcceptsCompleteRCA(t *testing.T) {
	pg := setupTestDB(t)
	ctx := context.Background()

	wi := models.WorkItem{
		ID: uuid.NewString(), ComponentID: "API_GATEWAY",
		Severity: "P1", Status: "OPEN",
		SignalCount: 1, StartTime: time.Now(), UpdatedAt: time.Now(),
	}
	pg.CreateWorkItem(ctx, wi)

	rca := models.RCA{
		ID:                uuid.NewString(),
		WorkItemID:        wi.ID,
		StartTime:         time.Now().Add(-2 * time.Hour),
		EndTime:           time.Now(),
		RootCauseCategory: "Configuration",
		FixApplied:        "Rolled back bad deployment",
		PreventionSteps:   "Add canary deployments",
		SubmittedAt:       time.Now(),
	}

	err := pg.SubmitRCA(ctx, rca)
	if err != nil {
		t.Fatalf("Expected no error for complete RCA, got: %v", err)
	}
	t.Log("Complete RCA accepted correctly")
}

// ─────────────────────────────────────────
// State Machine Tests
// ─────────────────────────────────────────

func TestTransition_ValidPath(t *testing.T) {
	pg := setupTestDB(t)
	ctx := context.Background()

	wi := models.WorkItem{
		ID: uuid.NewString(), ComponentID: "RDBMS",
		Severity: "P0", Status: "OPEN",
		SignalCount: 1, StartTime: time.Now(), UpdatedAt: time.Now(),
	}
	pg.CreateWorkItem(ctx, wi)

	// OPEN → INVESTIGATING should work
	err := pg.TransitionStatus(ctx, wi.ID, "INVESTIGATING")
	if err != nil {
		t.Fatalf("Valid transition OPEN→INVESTIGATING failed: %v", err)
	}
	t.Log("OPEN → INVESTIGATING: OK")

	// INVESTIGATING → RESOLVED should work
	err = pg.TransitionStatus(ctx, wi.ID, "RESOLVED")
	if err != nil {
		t.Fatalf("Valid transition INVESTIGATING→RESOLVED failed: %v", err)
	}
	t.Log("INVESTIGATING → RESOLVED: OK")
}

func TestTransition_RejectsSkippingStates(t *testing.T) {
	pg := setupTestDB(t)
	ctx := context.Background()

	wi := models.WorkItem{
		ID: uuid.NewString(), ComponentID: "RDBMS",
		Severity: "P0", Status: "OPEN",
		SignalCount: 1, StartTime: time.Now(), UpdatedAt: time.Now(),
	}
	pg.CreateWorkItem(ctx, wi)

	// OPEN → CLOSED should fail (skipping states)
	err := pg.TransitionStatus(ctx, wi.ID, "CLOSED")
	if err == nil {
		t.Fatal("Expected error skipping OPEN→CLOSED, got nil")
	}
	t.Logf("Correctly rejected skip: %v", err)
}

func TestTransition_RejectsCloseWithoutRCA(t *testing.T) {
	pg := setupTestDB(t)
	ctx := context.Background()

	wi := models.WorkItem{
		ID: uuid.NewString(), ComponentID: "RDBMS",
		Severity: "P0", Status: "OPEN",
		SignalCount: 1, StartTime: time.Now(), UpdatedAt: time.Now(),
	}
	pg.CreateWorkItem(ctx, wi)

	// Walk through to RESOLVED first
	pg.TransitionStatus(ctx, wi.ID, "INVESTIGATING")
	pg.TransitionStatus(ctx, wi.ID, "RESOLVED")

	// Try to close without submitting RCA
	err := pg.TransitionStatus(ctx, wi.ID, "CLOSED")
	if err == nil {
		t.Fatal("Expected error closing without RCA, got nil")
	}
	t.Logf("Correctly blocked close without RCA: %v", err)
}