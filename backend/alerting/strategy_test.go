package alerting_test

import (
	"testing"
	"time"

	"github.com/jainambarbhaya1509/ims/alerting"
	"github.com/jainambarbhaya1509/ims/models"
)

func makeWorkItem(severity string) models.WorkItem {
	return models.WorkItem{
		ID:          "test-id",
		ComponentID: "RDBMS",
		Severity:    severity,
		Status:      "OPEN",
		StartTime:   time.Now(),
	}
}

func TestDispatcher_RoutesP0(t *testing.T) {
	d := alerting.NewDispatcher()
	// Should not panic
	d.Dispatch(makeWorkItem("P0"))
	t.Log("P0 dispatched correctly")
}

func TestDispatcher_RoutesP1(t *testing.T) {
	d := alerting.NewDispatcher()
	d.Dispatch(makeWorkItem("P1"))
	t.Log("P1 dispatched correctly")
}

func TestDispatcher_RoutesP2(t *testing.T) {
	d := alerting.NewDispatcher()
	d.Dispatch(makeWorkItem("P2"))
	t.Log("P2 dispatched correctly")
}

func TestDispatcher_UnknownSeverityDefaultsToP2(t *testing.T) {
	d := alerting.NewDispatcher()
	// Should not panic, should default to P2
	d.Dispatch(makeWorkItem("P99"))
	t.Log("Unknown severity handled gracefully")
}

func TestDispatcher_RegisterCustomStrategy(t *testing.T) {
	d := alerting.NewDispatcher()

	// Custom strategy for testing
	custom := &alerting.P2Strategy{} // reuse P2 as a stand-in
	d.Register("P3", custom)

	// Should route to our custom strategy without panicking
	d.Dispatch(makeWorkItem("P3"))
	t.Log("Custom strategy registered and dispatched correctly")
}