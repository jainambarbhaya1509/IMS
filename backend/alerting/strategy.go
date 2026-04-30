package alerting

import (
	"fmt"
	"log"
	"time"

	"github.com/jainambarbhaya1509/ims/models"
)

// AlertStrategy is the interface every alerting strategy must implement.
// This is the core of the Strategy Pattern.
// To add a new alert type, just create a new struct that implements this.
type AlertStrategy interface {
	Alert(wi models.WorkItem)
	Priority() string
}

// ─────────────────────────────────────────
// P0Strategy — Critical. RDBMS, core infra.
// In production: pages on-call via PagerDuty, sends war-room Slack message.
// ─────────────────────────────────────────
type P0Strategy struct{}

func (s *P0Strategy) Priority() string { return "P0" }

func (s *P0Strategy) Alert(wi models.WorkItem) {
	log.Printf(`
╔══════════════════════════════════════════╗
║          🔴 P0 CRITICAL INCIDENT         ║
║  ID:        %s
║  Component: %s
║  Status:    %s
║  Time:      %s
║  Action:    Paging on-call engineer NOW  ║
╚══════════════════════════════════════════╝`,
		wi.ID, wi.ComponentID, wi.Status,
		wi.StartTime.Format(time.RFC3339),
	)
	// Production: call PagerDuty API here
}

// ─────────────────────────────────────────
// P1Strategy — High. API Gateway, queues.
// In production: sends Slack alert to #incidents channel.
// ─────────────────────────────────────────
type P1Strategy struct{}

func (s *P1Strategy) Priority() string { return "P1" }

func (s *P1Strategy) Alert(wi models.WorkItem) {
	log.Printf(`
┌──────────────────────────────────────────┐
│         🟠 P1 HIGH SEVERITY              │
│  ID:        %s
│  Component: %s
│  Status:    %s
│  Action:    Slack alert → #incidents     │
└──────────────────────────────────────────┘`,
		wi.ID, wi.ComponentID, wi.Status,
	)
	// Production: call Slack API here
}

// ─────────────────────────────────────────
// P2Strategy — Medium. Cache, NoSQL.
// In production: creates a Jira ticket, no human paged.
// ─────────────────────────────────────────
type P2Strategy struct{}

func (s *P2Strategy) Priority() string { return "P2" }

func (s *P2Strategy) Alert(wi models.WorkItem) {
	log.Printf(`
┌──────────────────────────────────────────┐
│         🟡 P2 MEDIUM SEVERITY            │
│  ID:        %s
│  Component: %s
│  Status:    %s
│  Action:    Ticket created in Jira       │
└──────────────────────────────────────────┘`,
		wi.ID, wi.ComponentID, wi.Status,
	)
	// Production: call Jira API here
}

// ─────────────────────────────────────────
// Dispatcher — picks the right strategy
// This is the "context" in Strategy Pattern terminology.
// It holds a reference to whichever strategy is needed
// and delegates the Alert() call to it.
// ─────────────────────────────────────────
type Dispatcher struct {
	strategies map[string]AlertStrategy
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		strategies: map[string]AlertStrategy{
			"P0": &P0Strategy{},
			"P1": &P1Strategy{},
			"P2": &P2Strategy{},
		},
	}
}

// Dispatch picks the right strategy and fires the alert.
// If severity is unknown, defaults to P2 (safe default).
func (d *Dispatcher) Dispatch(wi models.WorkItem) {
	strategy, ok := d.strategies[wi.Severity]
	if !ok {
		log.Printf("Unknown severity %s for work item %s, defaulting to P2", wi.Severity, wi.ID)
		strategy = &P2Strategy{}
	}
	strategy.Alert(wi)
}

// Register lets you add new strategies at runtime without
// changing existing code. This is the Open/Closed Principle —
// open for extension, closed for modification.
func (d *Dispatcher) Register(priority string, strategy AlertStrategy) {
	d.strategies[priority] = strategy
	fmt.Printf("Registered new alert strategy for %s\n", priority)
}