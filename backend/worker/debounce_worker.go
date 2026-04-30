package worker

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jainambarbhaya1509/ims/alerting"
	"github.com/jainambarbhaya1509/ims/ingestion"
	"github.com/jainambarbhaya1509/ims/models"
	"github.com/jainambarbhaya1509/ims/storage"
)

type DebounceWorker struct {
	buffer     chan ingestion.Signal
	pg         *storage.Postgres
	mongo      *storage.Mongo
	redis      *storage.Redis
	state      *ingestion.DebounceState
	dispatcher *alerting.Dispatcher
	processed  atomic.Int64 // thread-safe counter for metrics
}

func NewDebounceWorker(
	buffer chan ingestion.Signal,
	pg *storage.Postgres,
	mongo *storage.Mongo,
	redis *storage.Redis,
) *DebounceWorker {
	return &DebounceWorker{
		buffer:     buffer,
		pg:         pg,
		mongo:      mongo,
		redis:      redis,
		state:      ingestion.NewDebounceState(),
		dispatcher: alerting.NewDispatcher(),
	}
}

// Start reads signals from the buffer in an infinite loop.
// This runs as a single goroutine — no mutex needed on DebounceState
// because only this goroutine ever reads/writes it.
func (w *DebounceWorker) Start() {
	log.Println("Debounce worker started")
	for signal := range w.buffer {
		w.process(signal)
		w.processed.Add(1)
	}
}

func (w *DebounceWorker) process(s ingestion.Signal) {
	ctx := context.Background()
	now := time.Now()

	// Always store raw signal in MongoDB first (audit log)
	// Do this regardless of debounce outcome
	if err := w.mongo.InsertSignal(ctx, s); err != nil {
		log.Printf("Mongo insert failed for signal %s: %v", s.ID, err)
		// Don't return — we still want to process the work item
	}

	if w.state.ShouldCreateNewWorkItem(s.ComponentID, now) {
		// New incident — create a Work Item in Postgres
		severity := models.SeverityByComponent[s.ComponentID]
		if severity == "" {
			severity = "P2" // default
		}

		wi := models.WorkItem{
			ID:          uuid.NewString(),
			ComponentID: s.ComponentID,
			Severity:    severity,
			Status:      "OPEN",
			SignalCount: 1,
			StartTime:   now,
			UpdatedAt:   now,
		}

		if err := w.pg.CreateWorkItem(ctx, wi); err != nil {
			log.Printf("Postgres work item creation failed: %v", err)
			return
		}

		w.dispatcher.Dispatch(wi)

		// Cache the new work item in Redis
		w.redis.SetWorkItem(ctx, wi)

		// Open the debounce window
		w.state.OpenWindow(s.ComponentID, wi.ID, now)

		// Link signal to work item in Mongo
		w.mongo.LinkSignalToWorkItem(ctx, s.ID, wi.ID)

		log.Printf("[NEW INCIDENT] %s | %s | %s", wi.ID, s.ComponentID, severity)

	} else {
		// Existing incident — just increment the signal count
		workItemID := w.state.IncrementWindow(s.ComponentID)

		if err := w.pg.IncrementSignalCount(ctx, workItemID); err != nil {
			log.Printf("Signal count increment failed: %v", err)
		}

		w.mongo.LinkSignalToWorkItem(ctx, s.ID, workItemID)

		log.Printf("[GROUPED] signal %s → work item %s", s.ID, workItemID)
	}
}

// Processed returns total signals processed (for metrics)
func (w *DebounceWorker) Processed() int64 {
	return w.processed.Load()
}
