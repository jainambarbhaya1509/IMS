package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"github.com/jainambarbhaya1509/ims/models"
)

type Postgres struct {
	db *sql.DB
}

func NewPostgres() *Postgres {
	connStr := "host=localhost port=5433 user=ims password=ims dbname=ims sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Postgres connect failed:", err)
	}
	// Retry logic — DB might not be ready immediately on docker compose up
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("Postgres not ready, retrying (%d/10)...", i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatal("Postgres never became ready:", err)
	}
	p := &Postgres{db: db}
	p.migrate()
	return p
}

// migrate creates tables if they don't exist
func (p *Postgres) migrate() {
	_, err := p.db.Exec(`
		CREATE TABLE IF NOT EXISTS work_items (
			id TEXT PRIMARY KEY,
			component_id TEXT NOT NULL,
			severity TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'OPEN',
			signal_count INT NOT NULL DEFAULT 1,
			start_time TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			mttr_minutes DOUBLE PRECISION
		);

		CREATE TABLE IF NOT EXISTS rcas (
			id TEXT PRIMARY KEY,
			work_item_id TEXT NOT NULL REFERENCES work_items(id),
			start_time TIMESTAMPTZ NOT NULL,
			end_time TIMESTAMPTZ NOT NULL,
			root_cause_category TEXT NOT NULL,
			fix_applied TEXT NOT NULL,
			prevention_steps TEXT NOT NULL,
			submitted_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		log.Fatal("Migration failed:", err)
	}
	log.Println("Postgres tables ready")
}

func (p *Postgres) CreateWorkItem(ctx context.Context, wi models.WorkItem) error {
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO work_items (id, component_id, severity, status, signal_count, start_time, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, wi.ID, wi.ComponentID, wi.Severity, wi.Status, wi.SignalCount, wi.StartTime, wi.UpdatedAt)
	return err
}

func (p *Postgres) IncrementSignalCount(ctx context.Context, workItemID string) error {
	_, err := p.db.ExecContext(ctx,
		`UPDATE work_items SET signal_count = signal_count + 1, updated_at = $1 WHERE id = $2`,
		time.Now(), workItemID,
	)
	return err
}

// TransitionStatus enforces valid state transitions
// This is the State Pattern — invalid transitions are rejected here
func (p *Postgres) TransitionStatus(ctx context.Context, workItemID, newStatus string) error {
	validTransitions := map[string]string{
		"OPEN":          "INVESTIGATING",
		"INVESTIGATING": "RESOLVED",
		"RESOLVED":      "CLOSED",
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Read current status inside the transaction (prevents race conditions)
	var currentStatus string
	err = tx.QueryRowContext(ctx,
		`SELECT status FROM work_items WHERE id = $1 FOR UPDATE`, workItemID,
	).Scan(&currentStatus)
	if err != nil {
		return fmt.Errorf("work item not found: %w", err)
	}

	if validTransitions[currentStatus] != newStatus {
		return fmt.Errorf("invalid transition: %s → %s", currentStatus, newStatus)
	}

	// If closing, RCA must exist
	if newStatus == "CLOSED" {
		var rcaCount int
		tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM rcas WHERE work_item_id = $1`, workItemID,
		).Scan(&rcaCount)
		if rcaCount == 0 {
			return fmt.Errorf("cannot close work item: RCA is missing")
		}

		// Calculate MTTR
		var startTime time.Time
		tx.QueryRowContext(ctx,
			`SELECT start_time FROM work_items WHERE id = $1`, workItemID,
		).Scan(&startTime)

		var endTime time.Time
		tx.QueryRowContext(ctx,
			`SELECT end_time FROM rcas WHERE work_item_id = $1`, workItemID,
		).Scan(&endTime)

		mttr := endTime.Sub(startTime).Minutes()
		tx.ExecContext(ctx,
			`UPDATE work_items SET status = $1, updated_at = $2, mttr_minutes = $3 WHERE id = $4`,
			newStatus, time.Now(), mttr, workItemID,
		)
	} else {
		tx.ExecContext(ctx,
			`UPDATE work_items SET status = $1, updated_at = $2 WHERE id = $3`,
			newStatus, time.Now(), workItemID,
		)
	}

	return tx.Commit()
}

func (p *Postgres) GetAllWorkItems(ctx context.Context) ([]models.WorkItem, error) {
	rows, err := p.db.QueryContext(ctx,
		`SELECT id, component_id, severity, status, signal_count, start_time, updated_at, mttr_minutes
		 FROM work_items ORDER BY start_time DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.WorkItem
	for rows.Next() {
		var wi models.WorkItem
		rows.Scan(&wi.ID, &wi.ComponentID, &wi.Severity, &wi.Status,
			&wi.SignalCount, &wi.StartTime, &wi.UpdatedAt, &wi.MTTR)
		items = append(items, wi)
	}
	return items, nil
}

func (p *Postgres) SubmitRCA(ctx context.Context, rca models.RCA) error {
	// Validate all fields are present
	if rca.RootCauseCategory == "" || rca.FixApplied == "" || rca.PreventionSteps == "" {
		return fmt.Errorf("RCA is incomplete: all fields are required")
	}
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO rcas (id, work_item_id, start_time, end_time, root_cause_category, fix_applied, prevention_steps, submitted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, rca.ID, rca.WorkItemID, rca.StartTime, rca.EndTime,
		rca.RootCauseCategory, rca.FixApplied, rca.PreventionSteps, rca.SubmittedAt)
	return err
}