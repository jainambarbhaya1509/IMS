# Planning Prompts and Specification

Checked in per submission guidelines.

---

## Problem Decomposition

The assignment was distilled into three core problems before any code
was written:

1. Volume — handle 10,000 signals/sec without crashing
2. Noise — debounce duplicate signals into single incidents
3. Accountability — enforce RCA before closing an incident

Everything in the architecture is a consequence of solving these three
problems cleanly.

---

## Architecture Decisions Made Upfront

Before writing any code, the following decisions were locked in:

Go for backend
- Goroutines and channels handle the concurrency requirements natively
- No external threading library needed

Buffered channel as in-memory queue
- Same decoupling guarantee as Kafka without the operational overhead
- Non-blocking send means HTTP layer never hangs under load

Three storage layers
- MongoDB for raw signals: schemaless, high-write, flexible payloads
- PostgreSQL for work items and RCA: transactional, state machine
- Redis for dashboard cache: hot-path reads, TTL-based expiry

Single-goroutine debounce
- Only one goroutine touches the debounce state map
- Channel serialises access, no mutex needed

Strategy Pattern for alerting
- Each priority level is a separate struct
- Dispatcher picks the right one at runtime
- New strategies added without changing existing code

---

## Implementation Order

1. models/
   Data shapes first. Signal, WorkItem, RCA structs.
   Everything else depends on these.

2. storage/
   Database clients with retry logic.
   Postgres, MongoDB, Redis each in their own file.

3. ingestion/
   Rate limiter and debounce window state.
   HTTP handler that writes to the channel buffer.

4. worker/
   Background goroutine that reads from the channel.
   Runs debounce logic, writes to all three databases.

5. alerting/
   Strategy Pattern implementation.
   P0, P1, P2 structs and Dispatcher.

6. api/
   HTTP routes and CORS middleware.
   Redis to Postgres fallback on dashboard reads.

7. main.go
   Wire everything together.
   Graceful shutdown on SIGINT/SIGTERM.

8. tests
   RCA validation unit tests.
   State machine transition tests.
   Alerting strategy routing tests.

9. frontend
   React dashboard with polling loop.
   Three panels: live feed, incident detail, RCA form.

10. docs
    README with architecture, whys, setup, and non-functional items.
    DESIGN.md with decision rationale.
    SYSTEM_DESIGN.md with tech stack deep-dive.

---

## Design Patterns Used

Strategy Pattern
- Location: alerting/strategy.go
- Purpose: swap alert behaviour per severity without changing existing code

State Machine
- Location: storage/postgres.go
- Purpose: enforce valid incident lifecycle transitions

Factory
- Location: storage/NewPostgres(), NewMongo(), NewRedis()
- Purpose: encapsulate DB connection logic and retry behaviour

Repository
- Location: storage/*.go
- Purpose: separate database logic from business logic

Middleware
- Location: api/router.go
- Purpose: apply CORS headers across all routes uniformly

---

## Bonus Features Added Beyond Requirements

Timeseries aggregation endpoint
- GET /metrics/signals-per-hour
- MongoDB aggregation pipeline groups signals by component and hour
- Returns 24 hours of signal volume data

Runtime strategy registration
- dispatcher.Register() adds new alert strategies without recompilation
- Demonstrates Open/Closed Principle in practice

Automatic MTTR calculation
- Calculated inside the CLOSED transaction from RCA start and end times
- Never user-supplied, always accurate, impossible to fake

Redis TTL-based cache expiry
- 5 minute TTL per work item cache entry
- Stale data auto-clears even if explicit invalidation is missed

Graceful shutdown
- 10 second drain window on SIGINT and SIGTERM
- In-flight requests complete before process exits
- Safe for container orchestration systems like Kubernetes