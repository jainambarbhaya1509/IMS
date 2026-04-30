package ingestion

import (
	"time"

	"github.com/jainambarbhaya1509/ims/models"
)

// Signal is what arrives at the ingestion endpoint
type Signal = models.Signal

const (
	DebounceWindow   = 10 * time.Second
	DebounceMaxCount = 100
)

// WindowEntry tracks the active debounce window for a component
type WindowEntry struct {
	WorkItemID  string
	FirstSignal time.Time
	Count       int
}

// DebounceState is an in-memory map: componentID → active window
// Why in-memory and not Redis? Because this runs in a hot loop and
// needs microsecond access. Redis adds network latency.
type DebounceState struct {
	windows map[string]*WindowEntry
}

func NewDebounceState() *DebounceState {
	return &DebounceState{windows: make(map[string]*WindowEntry)}
}

// ShouldCreateNewWorkItem returns true if no active window exists for this component
func (d *DebounceState) ShouldCreateNewWorkItem(componentID string, now time.Time) bool {
	entry, exists := d.windows[componentID]
	if !exists {
		return true
	}
	// Window expired
	if now.Sub(entry.FirstSignal) > DebounceWindow {
		return true
	}
	// Hit the cap — start a new work item
	if entry.Count >= DebounceMaxCount {
		return true
	}
	return false
}

func (d *DebounceState) OpenWindow(componentID, workItemID string, now time.Time) {
	d.windows[componentID] = &WindowEntry{
		WorkItemID:  workItemID,
		FirstSignal: now,
		Count:       1,
	}
}

func (d *DebounceState) IncrementWindow(componentID string) string {
	d.windows[componentID].Count++
	return d.windows[componentID].WorkItemID
}

func (d *DebounceState) GetWorkItemID(componentID string) string {
	if e, ok := d.windows[componentID]; ok {
		return e.WorkItemID
	}
	return ""
}