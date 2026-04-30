package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jainambarbhaya1509/ims/ingestion"
	"github.com/jainambarbhaya1509/ims/models"
	"github.com/jainambarbhaya1509/ims/storage"
)

func NewRouter(
	buffer chan ingestion.Signal,
	pg *storage.Postgres,
	mongo *storage.Mongo,
	redis *storage.Redis,
) http.Handler {
	mux := http.NewServeMux()

	// Signal ingestion
	mux.Handle("POST /signals", ingestion.NewIngestHandler(buffer))

	// Work items (dashboard feed)
	mux.HandleFunc("GET /work-items", func(w http.ResponseWriter, r *http.Request) {
		// Try Redis first (hot path)
		items, err := redis.GetAllWorkItems(r.Context())
		if err != nil || len(items) == 0 {
			// Fall back to Postgres
			items, err = pg.GetAllWorkItems(r.Context())
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		json.NewEncoder(w).Encode(items)
	})

	// Raw signals for an incident
	mux.HandleFunc("GET /work-items/{id}/signals", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		signals, err := mongo.GetSignalsForWorkItem(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(signals)
	})

	// Status transition
	mux.HandleFunc("PATCH /work-items/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Status string `json:"status"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		if err := pg.TransitionStatus(r.Context(), id, body.Status); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Invalidate Redis cache so next dashboard fetch is fresh
		redis.InvalidateWorkItem(r.Context(), id)
		w.WriteHeader(http.StatusOK)
	})

	// RCA submission
	mux.HandleFunc("POST /work-items/{id}/rca", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var rca models.RCA
		json.NewDecoder(r.Body).Decode(&rca)

		rca.ID = uuid.NewString()
		rca.WorkItemID = id
		rca.SubmittedAt = time.Now()

		if err := pg.SubmitRCA(r.Context(), rca); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	})

	// Timeseries — signals per hour for last 24 hours
	mux.HandleFunc("GET /metrics/signals-per-hour", func(w http.ResponseWriter, r *http.Request) {
		since := time.Now().Add(-24 * time.Hour)
		buckets, err := mongo.GetSignalsPerHour(r.Context(), since)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buckets)
	})

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// CORS for React frontend
	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
