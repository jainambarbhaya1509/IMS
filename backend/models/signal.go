package models

import (
	"time"
)

// Signal is a raw error event sent by a monitored component.
// Stored in MongoDB (audit log, never modified).
type Signal struct {
	ID          string    `json:"id" bson:"_id"`
	ComponentID string    `json:"component_id" bson:"component_id"` // e.g. "CACHE_CLUSTER_01"
	Severity    string    `json:"severity" bson:"severity"`         // "P0", "P1", "P2"
	Message     string    `json:"message" bson:"message"`
	Payload     any       `json:"payload" bson:"payload"`           // flexible, unstructured
	ReceivedAt  time.Time `json:"received_at" bson:"received_at"`
	WorkItemID  string    `json:"work_item_id,omitempty" bson:"work_item_id,omitempty"`
}

// WorkItem is a deduplicated incident. One work item can have many signals.
// Stored in PostgreSQL (transactional, source of truth).
type WorkItem struct {
	ID          string    `json:"id" db:"id"`
	ComponentID string    `json:"component_id" db:"component_id"`
	Severity    string    `json:"severity" db:"severity"`
	Status      string    `json:"status" db:"status"` // OPEN → INVESTIGATING → RESOLVED → CLOSED
	SignalCount  int       `json:"signal_count" db:"signal_count"`
	StartTime   time.Time `json:"start_time" db:"start_time"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	MTTR        *float64  `json:"mttr_minutes,omitempty" db:"mttr_minutes"` // set when CLOSED
}

// RCA is the Root Cause Analysis. Required before closing a WorkItem.
// Stored in PostgreSQL alongside the WorkItem.
type RCA struct {
	ID               string    `json:"id" db:"id"`
	WorkItemID       string    `json:"work_item_id" db:"work_item_id"`
	StartTime        time.Time `json:"start_time" db:"start_time"`
	EndTime          time.Time `json:"end_time" db:"end_time"`
	RootCauseCategory string   `json:"root_cause_category" db:"root_cause_category"`
	FixApplied       string    `json:"fix_applied" db:"fix_applied"`
	PreventionSteps  string    `json:"prevention_steps" db:"prevention_steps"`
	SubmittedAt      time.Time `json:"submitted_at" db:"submitted_at"`
}

// Severity priority mapping — used by the alerting strategy
var SeverityByComponent = map[string]string{
	"RDBMS":         "P0",
	"API_GATEWAY":   "P1",
	"CACHE_CLUSTER": "P2",
	"ASYNC_QUEUE":   "P1",
	"MCP_HOST":      "P1",
	"NOSQL":         "P2",
}