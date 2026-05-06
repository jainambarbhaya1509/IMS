# System Design — Tech Stack Justification

---

## Signal Ingestion

Protocol chosen: HTTP/JSON over REST

Why not gRPC:
HTTP is universally supported by all monitoring agents and services.
gRPC requires generated client code on every producer. When producers
are diverse — APIs, caches, queues, databases, MCP hosts — HTTP is
the only practical choice. Any language with an HTTP library can send
a signal with a single curl command.

Why not a message queue at the ingestion boundary:
The rate limiter plus buffered channel provides the same decoupling
guarantee at this scale. Kafka adds ZooKeeper or KRaft, topic
management, consumer group configuration, and offset management.
None of that complexity is justified when a Go channel achieves the
same result in-process with nanosecond latency.

---

## In-Memory Buffer

Technology chosen: Go buffered channel, capacity 50,000

Why not Redis list as external queue:
Redis adds approximately 1ms of network latency per operation.
At 10,000 signals per second, that is 10 seconds of accumulated
latency per second of input — completely unacceptable. An in-memory
Go channel operation takes nanoseconds and never leaves the process.

Why not an unbuffered channel:
An unbuffered channel blocks the sender until the receiver is ready.
At 10,000 signals/sec with a slow database, every HTTP handler would
hang waiting for the worker. The buffered channel decouples them
completely — the HTTP handler writes and returns immediately as long
as there is space in the buffer.

---

## Source of Truth

Technology chosen: PostgreSQL 16

Why not MySQL:
Postgres has superior FOR UPDATE row locking semantics. The SKIP
LOCKED option is useful for future queue-like workloads. RETURNING
clauses simplify audit trail logic. Postgres is the industry standard
for transactional workloads requiring strict consistency.

Why not MongoDB:
State transitions require ACID guarantees. The RCA existence check
and CLOSED status write must be atomic. MongoDB multi-document
transactions exist but are slower, more complex to reason about,
and less battle-tested than Postgres native transactions for strict
ordering guarantees.

Why not SQLite:
SQLite is single-writer. At any real concurrency level, writers
queue up behind each other. Not suitable for a system handling
thousands of concurrent status checks.

---

## Audit Log

Technology chosen: MongoDB 7

Why not Postgres:
Signal payloads are unstructured. Each component type sends different
fields. Postgres would require either a JSONB column (loses type
safety and structured indexing), or a rigid schema with dozens of
nullable columns where most are null for any given signal, or a new
migration every time a new component is onboarded. MongoDB's
schemaless model handles this natively.

Why not Elasticsearch:
Elasticsearch is optimised for full-text search and log analytics
at the Kibana level. The timeseries queries required here — group
signals by component and hour — are handled perfectly by MongoDB's
aggregation pipeline without the operational overhead of running
an ES cluster with dedicated heap management.

Why not ClickHouse or TimescaleDB:
Both are excellent for timeseries data but introduce additional
infrastructure. MongoDB is already in the stack for the schemaless
signal storage. Reusing it for aggregations avoids adding a fourth
database.

---

## Dashboard Cache

Technology chosen: Redis 7

Why not Memcached:
Redis supports per-key TTL, atomic increment operations, and optional
persistence to disk. Per-key TTL is central to this design. Each work
item cache entry expires after 5 minutes automatically. Memcached
applies TTL at the instance level, not per key.

Why not in-process Go map:
An in-process map does not survive process restarts. Every deployment
or crash would wipe the cache and cause a thundering herd on Postgres
as every dashboard client simultaneously misses. Redis persists across
backend process restarts.

Why not a CDN or edge cache:
The dashboard data is personalised and changes frequently. CDN caching
is designed for static or semi-static content served to anonymous
users. Not applicable here.

---

## Frontend

Technology chosen: React 18 with Vite

Why not Vue:
React's unidirectional data flow maps cleanly to the three-panel
dashboard layout. The selected incident in the left panel drives
both the detail panel and the RCA form on the right. Vue works
equally well but React has a slightly larger ecosystem for the
component patterns needed here.

Why not HTMX:
HTMX suits server-rendered UIs where HTML fragments are swapped in
on user interaction. The dashboard requires meaningful client-side
state: the selected incident, real-time polling, optimistic UI
updates on status transitions, and multi-field form validation before
any API call. HTMX would require significant server-side rendering
complexity to achieve the same result.

Why not Next.js:
Server-side rendering adds complexity not needed here. The dashboard
is an authenticated internal tool, not a public-facing page requiring
SEO or initial load performance optimisation. Plain React with Vite
is simpler and faster to develop.

Why Vite over Create React App:
Vite uses native ES modules for development, resulting in near-instant
hot module replacement. Create React App bundles everything on every
change. For a dashboard with many components, the developer experience
difference is significant.

---

## Scalability Path

If this system needed to scale to 1,000,000 signals per second:

Replace Go channel with Apache Kafka
Persistent, distributed, and replayable. Multiple consumer groups
allow independent debounce workers for different component categories.
Signals are durable across process crashes. Kafka Connect can fan out
signals to additional sinks like ClickHouse for analytics.

Shard MongoDB by component_id
Even write distribution across shards. Each shard handles signals
for a subset of components. The compound index on (component_id,
received_at) aligns perfectly with the shard key.

Add Postgres read replicas
Work item list reads go to replicas. Status transition writes go to
the primary. This separates read and write load, allowing the primary
to focus on transactional writes.

Use Redis Cluster
Horizontal cache scaling across multiple nodes. The wi: key namespace
distributes evenly across cluster slots. No single Redis node becomes
a bottleneck.

Partition debounce workers by component_id hash
Hash each component_id to assign it to exactly one worker process.
No cross-worker coordination or shared state needed. Each worker
owns its subset of components completely.

Replace polling with Server-Sent Events or WebSockets
Push work item updates to dashboard clients the moment they occur.
Eliminates the 5-second polling latency and reduces server load
from constant polling across many browser sessions.

Add circuit breakers on all DB clients
Use a circuit breaker pattern on MongoDB, Postgres, and Redis clients.
If a database becomes unresponsive, the circuit opens and requests
fail fast rather than piling up waiting for timeouts. The system
degrades gracefully rather than cascading.