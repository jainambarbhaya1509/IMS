# Design Decisions

## Why Go

Go was chosen over Node.js, Python, and Java for one reason:
goroutines and channels are native language primitives, not a library.

A goroutine costs ~2KB of memory at startup versus ~1MB for an OS thread.
At 10,000 signals/sec, every incoming HTTP request runs in its own goroutine
and this is completely normal in Go. The built-in channel type provides
producer-consumer communication with zero external dependencies.

Node.js requires careful async management and worker threads for CPU-bound
tasks. Python's GIL creates bottlenecks under concurrent load. Java works
but carries significant boilerplate overhead. Go is the right tool for
high-throughput concurrent systems.

---

## Why Three Databases

Each database was chosen for what it does best. Using one database for
everything would mean forcing the wrong tool onto every problem.

### MongoDB — Signals (Audit Log)

Signal payloads are unstructured by definition. An RDBMS failure sends
connection pool metrics. A cache failure sends memory utilisation. An API
gateway sends latency percentiles. Forcing this into a Postgres schema would
require nullable columns for every possible field, or a JSONB column that
loses type safety, or a migration every time a new component type is added.

MongoDB's schemaless model handles variable payloads naturally. The compound
index on (component_id, received_at) makes debounce window queries fast even
with millions of documents. The native aggregation pipeline handles timeseries
grouping for the metrics endpoint without application-level code.

Why not Elasticsearch: ES is optimised for full-text search. MongoDB's
aggregation pipeline handles the timeseries queries needed here without the
operational overhead of running an ES cluster.

### PostgreSQL — Work Items and RCA (Source of Truth)

The single most important requirement is transactional correctness. When an
incident is closed, two things must happen atomically: check that an RCA
record exists, and update the status to CLOSED. If these happen in two
separate operations, a race condition exists.

Inside a Postgres transaction with FOR UPDATE row locking, this is
structurally impossible. The second transaction waits for the first to
commit, then reads the already-updated status and fails with a clear error.
MTTR is also calculated inside this same transaction, from RCA start and end
times. Everything atomic. Everything consistent.

Why not MongoDB for this: MongoDB multi-document transactions are slower and
more complex than Postgres native transactions for strict state machine
guarantees.

Why not MySQL: Postgres has superior FOR UPDATE locking semantics, better
JSONB support, and RETURNING clauses useful for audit trails.

### Redis — Dashboard Cache (Hot Path)

The dashboard polls every 5 seconds. Without caching, every poll hits Postgres
with a full table scan. Redis serves reads in under 1 millisecond.

Per-key TTL (5 minutes) means each cache entry expires automatically even if
invalidation is missed. No manual cleanup required. An in-process Go map was
considered but does not survive process restarts. Redis persists across backend
restarts, keeping the dashboard responsive during deployments.

Why not Memcached: Redis supports per-key TTL, atomic operations, and
persistence. Per-key TTL is central to this design. Memcached does not
support it.

---

## Why Buffered Channel Instead of Kafka

A production system at true scale would use Apache Kafka. For this system,
a Go buffered channel provides the same core guarantee — decoupling the fast
HTTP ingestion layer from the slower database write layer — without requiring
a separate Kafka cluster, ZooKeeper, topic management, and consumer group
configuration.

The channel holds 50,000 signals. At 10,000 signals/sec, that is 5 seconds
of burst capacity. If the buffer fills, the HTTP handler returns 503
immediately rather than blocking.

Why not Redis list as the queue: Redis adds a network round-trip (~1ms) per
operation. At 10,000 signals/sec that is 10 seconds of accumulated latency
per second of input. An in-memory channel operation takes nanoseconds.

Trade-off accepted: signals are lost if the Go process crashes while the
buffer is non-empty. Kafka would persist signals to disk across crashes.
This is acceptable because the rate limiter caps inflow and producers can
re-send. Operational simplicity outweighs the durability cost at this scale.

---

## Why Single-Goroutine Debounce

The debounce state is an in-memory map of component ID to active window.
This map is only ever read and written by a single goroutine — the debounce
worker. The channel serialises all access. No mutex is needed, no contention
is possible.

The alternative would be a concurrent map with mutexes, which adds complexity
and lock contention at high throughput. The single-goroutine approach is
simpler, faster, and provably race-condition-free.

---

## Strategy Pattern for Alerting

The Strategy Pattern was chosen for alerting because:

- Swappable: change P0 from PagerDuty to OpsGenie by implementing a new
  struct, zero changes to existing code
- Testable: each strategy is independently unit-testable in isolation
- Open/Closed Principle: adding P3 never touches P0, P1, or P2 code
- Runtime registration: dispatcher.Register() allows adding strategies
  without recompilation

---

## State Machine in Postgres

The state machine is enforced inside a Postgres transaction with a row-level
FOR UPDATE lock. This means:

- Two engineers clicking simultaneously are serialised, not duplicated
- The RCA check and CLOSED write are atomic, impossible to race
- Invalid transitions return a clear error message, not a silent no-op
- MTTR is calculated at the exact moment of closure, not estimated

---

## Polling vs WebSockets

The dashboard polls every 5 seconds instead of using WebSockets.

Polling:
- Low complexity, no persistent connection infrastructure
- Predictable server load
- Suitable for incident dashboards where 5s latency is acceptable

WebSockets:
- Near-zero latency
- Requires persistent connection management
- Better suited for live chat, trading, real-time collaboration

For an incident management dashboard, 5-second latency is acceptable.
Incidents are not resolved in sub-second timeframes. Polling keeps the
architecture simple with no additional server infrastructure required.