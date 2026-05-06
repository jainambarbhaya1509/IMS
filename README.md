# IMS — Incident Management System

A mission-critical, production-grade Incident Management System built in Go + React. Ingests high-volume error signals from a distributed stack, deduplicates them into actionable incidents, and enforces a structured resolution workflow with mandatory Root Cause Analysis.

---

## What This System Solves

In a distributed system, when something breaks — say the RDBMS goes down — every service that depends on it starts screaming simultaneously. Without IMS, an engineer gets 10,000 alerts and is paralysed by noise. With IMS, they get one incident, with all signals linked to it, and a clear workflow to resolve it.

Three problems solved:
- **Volume** — accept 10,000 signals/sec without crashing
- **Noise** — collapse duplicate signals into one incident per component
- **Accountability** — force a documented root cause before closing

---

## Architecture

```
Signal Producers (APIs, MCP Hosts, Caches, Queues, RDBMS, NoSQL)
        |
        | POST /signals (HTTP/JSON)
        v
+--------------------------------------------------+
|              INGESTION LAYER                     |
|  Rate Limiter (10k req/s) -> Channel Buffer (50k)|
|  Non-blocking drop on full buffer                |
+--------------------------------------------------+
        |
        v
+--------------------------------------------------+
|           DEBOUNCE WORKER (goroutine)            |
|  10s window per component = 1 Work Item          |
|  Alert Strategy fired on new incident (P0/P1/P2) |
+--------------------------------------------------+
        |
   +----+------------------+------------------+
   |                       |                  |
   v                       v                  v
MongoDB               PostgreSQL            Redis
Raw Signals           Work Items            Dashboard Cache
Audit Log             RCA Records           TTL: 5 min
Schemaless            ACID Transactions     Hot-path reads
                      State Machine         <1ms latency
                      FOR UPDATE lock
        |
        v
REST API (Go net/http)
  GET  /work-items
  GET  /work-items/:id/signals
  PATCH /work-items/:id/status
  POST /work-items/:id/rca
  GET  /metrics/signals-per-hour
  GET  /health
        |
        v
React Dashboard
  Live Feed | Incident Detail | RCA Form
```

---

## Why This Tech Stack

### Go — Backend Language
Go was chosen over Node.js, Python, and Java for one reason: goroutines and channels are native language primitives. A goroutine costs ~2KB of memory versus ~1MB for an OS thread. At 10,000 signals/sec, every incoming HTTP request runs in its own goroutine — completely normal in Go. The built-in channel type provides producer-consumer communication with zero external dependencies.

Node.js requires worker threads for CPU-bound tasks. Python's GIL creates bottlenecks. Java works but carries significant boilerplate. Go is the right tool for high-throughput concurrent systems.

### Go Buffered Channel — In-Memory Queue
A production system at extreme scale would use Apache Kafka. For this system, a Go buffered channel provides the same core guarantee — decoupling the fast HTTP ingestion layer from the slower database write layer — without a Kafka cluster, ZooKeeper, and topic management overhead.

The channel holds 50,000 signals. At 10,000 signals/sec, that is 5 seconds of burst capacity. If the buffer fills, the HTTP handler returns 503 immediately rather than blocking. This is the backpressure mechanism.

**Why not Redis list as queue?** Redis adds ~1ms network latency per operation. At 10,000 signals/sec, that is 10 seconds of accumulated latency per second of input. In-memory channel operations take nanoseconds.

**Trade-off accepted:** Signals are lost if the process crashes while the buffer is non-empty. Kafka would persist to disk. This is acceptable because the rate limiter caps inflow and producers can re-send.

### PostgreSQL — Source of Truth
The single most important requirement is transactional correctness. When an incident is closed, two things must happen atomically: check that an RCA record exists, and update status to CLOSED. Postgres handles this with ACID transactions and FOR UPDATE row locking.

**Why not MongoDB for this?** MongoDB multi-document transactions are slower and more complex than Postgres native transactions for strict state machine guarantees.

**Why not MySQL?** Postgres has superior FOR UPDATE locking semantics, better JSONB support, and RETURNING clauses useful for audit trails.

### MongoDB — Audit Log
Signal payloads are unstructured by definition. An RDBMS failure sends connection pool metrics. A cache failure sends memory utilisation. An API gateway sends latency percentiles. Forcing this into a Postgres schema would require nullable columns for every possible field, or a JSONB column that loses type safety, or migrations for every new component type. MongoDB's schemaless model handles variable payloads naturally.

The compound index on (component_id, received_at) makes debounce window queries — "find all signals for RDBMS in the last 10 seconds" — fast even with millions of documents.

**Why not Elasticsearch?** ES is optimised for full-text search. MongoDB's aggregation pipeline handles timeseries queries without the operational overhead.

### Redis — Dashboard Cache
The dashboard polls every 5 seconds. Without caching, every poll hits Postgres with a full table scan. Redis serves reads in under 1 millisecond.

**Why not Memcached?** Redis supports per-key TTL, which is central to this design. Each cache entry expires after 5 minutes automatically — no manual cleanup needed.

**Why not an in-process Go map?** Doesn't survive process restarts. Redis persists across backend restarts, keeping the dashboard responsive during deployments.

### React + Vite — Frontend
The dashboard requires client-side state management: the selected incident drives both the detail panel and the RCA form, status transitions update UI without page reloads, and the RCA form has multi-field validation. React's component model and unidirectional data flow handles all of this cleanly. HTMX suits simpler server-rendered UIs. Vite provides fast HMR during development.

### Docker Compose — Local Infrastructure
All three databases start with one command: docker compose up -d. No local database installation required. The entire system is reproducible on any machine.

---

## How Backpressure Works

```
HTTP Handlers (many goroutines, fast)
        |
        |   select {
        |     case signalBuffer <- signal:   // queued, 202 Accepted
        |     default:                       // buffer full, 503 immediately
        |   }
        v
Go Channel Buffer (capacity: 50,000 signals)
        |
        v
Debounce Worker (single goroutine, steady DB pace)
        |
        +---> MongoDB  (raw signal insert)
        +---> Postgres (work item create or increment)
```

Three layers of protection:

1. **Rate Limiter (429)** — Token bucket at 10,000 req/s. Rejects before any work is done.
2. **Channel Buffer (absorb)** — 50,000 capacity absorbs ~5 seconds of full load.
3. **Non-blocking drop (503)** — select/default means HTTP handler never waits.

Accepted trade-off: under sustained extreme load, some signals are dropped. A 503 tells the producer to retry. Better to lose 0.1% of signals than have IMS go down during a major incident.

---

## How Debouncing Works

```
Timeline:  0s               10s             15s
           |                 |               |
Signals:   ################  |               #######
           |                 |               |
           Window opens      Window expires  New window opens
           Work Item created                 New Work Item created
```

The worker maintains an in-memory map of component -> active window. For each signal:
- No window exists → create Work Item, open window
- Window active and under 100 signals → increment count, link signal
- Window expired (>10s) or hit 100 signals → create new Work Item

This map is only ever touched by one goroutine. Channel serialises all access. No mutex needed.

---

## How the State Machine Works

```
OPEN --> INVESTIGATING --> RESOLVED --> CLOSED
```

Enforced inside Postgres transactions with FOR UPDATE row locking:
- Cannot skip states (OPEN -> CLOSED = 400 Bad Request)
- Cannot go backwards (RESOLVED -> INVESTIGATING = 400)
- CLOSED requires RCA — check and write happen atomically in one transaction
- MTTR auto-calculated at close: RCA.end_time - RCA.start_time

Two engineers clicking simultaneously are serialised by the row lock.

---

## How Alerting Works (Strategy Pattern)

```go
type AlertStrategy interface {
    Alert(wi models.WorkItem)
    Priority() string
}
```

Three concrete strategies:
- P0Strategy — RDBMS failures — pages on-call immediately
- P1Strategy — API Gateway, Queue, MCP Host — Slack alert
- P2Strategy — Cache, NoSQL — creates Jira ticket

Adding a new alert type requires zero changes to existing code:
```go
dispatcher.Register("P3", &P3Strategy{})
```

This is the Open/Closed Principle: open for extension, closed for modification.

---

## Setup

### Prerequisites
- Docker Desktop running
- Go 1.22+
- Node.js 18+

### Step 1 — Clone the repo
```bash
git clone https://github.com/jainambarbhaya1509/ims
cd ims
```

### Step 2 — Start the databases
```bash
docker compose up -d
```
Wait ~10 seconds for Postgres, MongoDB and Redis to initialise.

### Step 3 — Start the backend (Terminal 1)
```bash
cd backend
go mod download
go run main.go
```

Expected output:
```
Postgres tables ready
Mongo connected
Redis connected
Debounce worker started
Server running on :8080
```

### Step 4 — Start the frontend (Terminal 2)
```bash
cd frontend
npm install
npm run dev
```
Open http://localhost:5173

### Step 5 — Simulate a failure (Terminal 3)
```bash
chmod +x scripts/simulate_failure.sh
./scripts/simulate_failure.sh
```

The script:
1. Sends 5 RDBMS signals — P0 alert fires
2. Sends 3 MCP_HOST signals — P1 alert fires, cascade simulation
3. Transitions RDBMS incident through all states
4. Attempts CLOSED without RCA — gets 400 (enforcement working)
5. Submits complete RCA
6. Closes incident — MTTR calculated automatically

### Step 6 — Run tests
```bash
cd backend
go test ./... -v
```

---

## API Reference

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | /signals | Ingest error signal |
| GET | /work-items | List all incidents |
| GET | /work-items/:id/signals | Raw signals from MongoDB |
| PATCH | /work-items/:id/status | Transition status |
| POST | /work-items/:id/rca | Submit RCA |
| GET | /metrics/signals-per-hour | Timeseries data (24h) |
| GET | /health | Health check |

### Signal payload
```json
{
  "component_id": "RDBMS",
  "severity": "P0",
  "message": "Connection pool exhausted",
  "payload": {
    "active_connections": 100,
    "max_connections": 100,
    "waiting_requests": 847
  }
}
```

### RCA payload
```json
{
  "start_time": "2024-01-15T10:00:00Z",
  "end_time": "2024-01-15T11:30:00Z",
  "root_cause_category": "Infrastructure",
  "fix_applied": "Increased connection pool size from 100 to 500",
  "prevention_steps": "Add monitoring alert at 80% pool capacity"
}
```

---

## Non-Functional Considerations

These are production-grade additions beyond the functional requirements.

### Performance
- **Redis cache** serves dashboard reads in under 1ms versus ~5ms Postgres. Every dashboard poll hits Redis first.
- **MongoDB compound index** on (component_id, received_at) makes debounce window queries O(log n) instead of full collection scans.
- **MGet** fetches all Redis dashboard keys in a single network round-trip, not one per key.
- **Buffered channel** fully decouples HTTP ingestion speed from DB write speed. Slow DB writes never slow down signal acceptance.
- **Single-goroutine debounce** eliminates lock contention entirely. The channel itself is the synchronisation primitive.

### Resilience
- **DB retry on startup** — all three clients retry with 2 second delays up to 10 attempts. Handles slow Docker container initialisation without crashing.
- **Graceful shutdown** — 10 second drain window lets in-flight requests complete before the process exits. No dropped connections on deploy.
- **Redis to Postgres fallback** — if Redis is empty or unavailable, the dashboard transparently reads from Postgres. The UI never errors.
- **Signal drop over hang** — 503 on full buffer keeps the HTTP layer always responsive, even under extreme load.
- **Redis TTL (5 minutes)** — stale cache entries auto-expire even if cache invalidation is missed. No manual cleanup required.

### Security
- **Parameterized queries** — all Postgres queries use $1 placeholder syntax. SQL injection is structurally impossible, not just mitigated.
- **Rate limiting** — token bucket at 10,000 req/s prevents any client from exhausting the signal buffer or overwhelming the system.
- **Server-side assignment** — received_at timestamp and signal ID are assigned by the server, never trusted from client payloads.
- **CORS** — permissive (*) for development. In production, restrict to the specific frontend origin.

### Correctness
- **Pessimistic locking (FOR UPDATE)** — status transition reads and writes happen inside a transaction with a row-level lock. Two simultaneous requests are serialised, not duplicated.
- **Atomic RCA enforcement** — the RCA existence check and CLOSED status write happen in a single transaction. There is no race condition window between the check and the write.
- **Single-goroutine debounce** — the debounce state map is only ever touched by one goroutine. The channel serialises all access. No mutex required, no contention possible.

### Observability
- **/health endpoint** — returns {"status":"ok"} — suitable for load balancer health checks and Docker healthcheck directives.
- **Throughput metrics** — signals/sec and total processed printed to stdout every 5 seconds. Pipeable to any log aggregator.
- **Structured logging** — every new incident creation, signal grouping decision, state transition, and error is logged with context.

### Bonus Additions
- **Timeseries aggregation endpoint** — GET /metrics/signals-per-hour uses MongoDB aggregation pipeline to group signal volume by component and hour over 24 hours.
- **Runtime strategy registration** — dispatcher.Register() allows adding new alert strategies without changing existing code. Open/Closed Principle in practice.
- **Automatic MTTR** — calculated at close time from RCA start/end times. Never user-supplied, always accurate.
- **Redis TTL-based cache expiry** — 5 minute TTL means stale data auto-clears even if invalidation is missed.
- **Graceful shutdown** — 10 second drain window on SIGINT/SIGTERM. Safe for container orchestration.

---

## Running Tests

```bash
cd backend
go test ./... -v
```

| Test | What it verifies |
|------|-----------------|
| TestSubmitRCA_RejectsEmptyRootCause | Blank category rejected |
| TestSubmitRCA_RejectsEmptyFixApplied | Blank fix rejected |
| TestSubmitRCA_AcceptsCompleteRCA | Valid RCA stored |
| TestTransition_ValidPath | OPEN->INVESTIGATING->RESOLVED works |
| TestTransition_RejectsSkippingStates | OPEN->CLOSED rejected |
| TestTransition_RejectsCloseWithoutRCA | Close without RCA rejected |
| TestDispatcher_RoutesP0 | P0 -> P0Strategy |
| TestDispatcher_RoutesP1 | P1 -> P1Strategy |
| TestDispatcher_RoutesP2 | P2 -> P2Strategy |
| TestDispatcher_UnknownSeverityDefaultsToP2 | Unknown severity safe default |
| TestDispatcher_RegisterCustomStrategy | Custom strategy registerable |

---

## Project Structure

```
ims/
├── backend/
│   ├── main.go                    Entry point, wires all components
│   ├── go.mod
│   ├── models/signal.go           Signal, WorkItem, RCA structs
│   ├── ingestion/
│   │   ├── handler.go             HTTP handler, rate limiter, metrics
│   │   └── debounce.go            In-memory 10s window state
│   ├── worker/
│   │   └── debounce_worker.go     Background goroutine, reads channel
│   ├── alerting/
│   │   ├── strategy.go            Strategy Pattern P0/P1/P2 + Dispatcher
│   │   └── strategy_test.go       Alerting unit tests
│   ├── storage/
│   │   ├── postgres.go            Work items, RCA, state machine, MTTR
│   │   ├── postgres_test.go       RCA + state machine unit tests
│   │   ├── mongo.go               Raw signal insert and fetch
│   │   ├── mongo_aggregations.go  Timeseries aggregation pipeline
│   │   └── redis.go               Dashboard cache with TTL
│   └── api/router.go              HTTP routes + CORS middleware
├── frontend/src/
│   ├── App.jsx                    Root, 5s polling loop
│   ├── api/client.js              All backend API calls
│   └── components/
│       ├── Header.jsx             Health indicator, live badge
│       ├── IncidentList.jsx       Left panel, sorted by severity
│       ├── IncidentDetail.jsx     Right panel, signals + transitions
│       ├── RCAForm.jsx            RCA form with validation
│       ├── SeverityBadge.jsx      P0/P1/P2 chip
│       └── StatusBadge.jsx        Status chip with pulse animation
├── scripts/simulate_failure.sh   End-to-end failure simulation
├── docs/
│   ├── DESIGN.md
│   ├── PROMPTS.md
│   └── SYSTEM_DESIGN.md
└── docker-compose.yml
```

---