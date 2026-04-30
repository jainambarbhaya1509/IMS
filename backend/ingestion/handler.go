package ingestion

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RateLimiter uses a token bucket approach
// Each second, N tokens are added. Each request consumes one token.
// If no tokens left, request is rejected with 429.
type RateLimiter struct {
	mu       sync.Mutex
	tokens   int
	max      int
	refillAt time.Time
}

func NewRateLimiter(perSecond int) *RateLimiter {
	return &RateLimiter{
		tokens:   perSecond,
		max:      perSecond,
		refillAt: time.Now().Add(time.Second),
	}
}

func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.After(r.refillAt) {
		r.tokens = r.max
		r.refillAt = now.Add(time.Second)
	}

	if r.tokens <= 0 {
		return false
	}
	r.tokens--
	return true
}

// IngestHandler is the HTTP handler for POST /signals
type IngestHandler struct {
	buffer      chan Signal
	rateLimiter *RateLimiter
}

func NewIngestHandler(buffer chan Signal) *IngestHandler {
	return &IngestHandler{
		buffer:      buffer,
		rateLimiter: NewRateLimiter(10000), // 10k signals/sec
	}
}

func (h *IngestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Rate limit check — fast, before any parsing
	if !h.rateLimiter.Allow() {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}

	var s Signal
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Assign ID and timestamp server-side (don't trust the client)
	s.ID = uuid.NewString()
	s.ReceivedAt = time.Now()

	// Non-blocking send to buffer
	// If the buffer is full (50k items), we drop the signal rather than blocking the HTTP handler
	// This is the backpressure mechanism — the HTTP layer never slows down
	select {
	case h.buffer <- s:
		w.WriteHeader(http.StatusAccepted) // 202 = accepted, not yet processed
	default:
		http.Error(w, "buffer full, try again", http.StatusServiceUnavailable)
	}
}

// StartMetricsLogger prints throughput every 5 seconds
func StartMetricsLogger(w interface{ Processed() int64 }) {
	ticker := time.NewTicker(5 * time.Second)
	var lastCount int64
	for range ticker.C {
		current := w.Processed()
		rate := (current - lastCount) / 5
		lastCount = current
		println("[METRICS] Signals/sec:", rate, "| Total:", current)
	}
}