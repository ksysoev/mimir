# Mimir — Product Roadmap (Simplified)

This roadmap focuses on **what to build**, **why it matters**, and **what to ship**.
It intentionally avoids low-level implementation detail.

---

## Principles

- Keep `kvStore` interface clean and extensible.
- Ship small, testable increments.
- Add observability before major complexity.
- Prefer proven libraries over custom infrastructure when possible.

---

## Delivery Order (Recommended)

| # | Feature | Why now | Effort | Depends on |
|---|---------|---------|--------|------------|
| 1 | DELETE operation | Completes key lifecycle; unblocks cleanup flows | 1–2 days | — |
| 2 | TTL + eviction policies | Controls memory growth and stale data | 1.5–2 weeks | #1 |
| 3 | Prometheus metrics | Needed to operate and tune safely | 2–3 days | #1–2 |
| 4 | Watch/SSE notifications | Removes polling overhead for clients | ~1.5 weeks | #1 |
| 5 | Namespaces | Multi-tenant isolation and limits | ~2 weeks | #1–3 |
| 6 | Replication (primary→replica) | High availability / shard redundancy | 4–6 weeks | #3 |
| 7 | Persistent backend (LSM via Pebble/Badger) | Durability beyond memory + replication | ~1 week (integration) | #3 |

---

## 1) DELETE Operation

**Goal:** add `DELETE /kv/{key}` support.

**Scope**
- Extend store interface with `Delete(ctx, key)`.
- Add API handler + router wiring.
- Return `204` on success, `404` if missing.

**Done when**
- Delete works on node and through router.
- Tests cover delete + recreate behavior.

**Notes**
- Recreated keys may restart version sequence (document this).

---

## 2) TTL + Eviction Policies

### 2A. TTL Expiry

**Goal:** keys can expire automatically.

**Scope**
- Support TTL on write (`?ttl=<seconds>`).
- Expired keys are hidden from reads immediately (lazy check).
- Background cleanup removes expired keys over time.

**Done when**
- Expired keys return `404`.
- Cleanup keeps memory bounded for expired data.
- Response may include expiry metadata (e.g., `X-Expires-At`).

### 2B. Capacity Eviction

**Goal:** behavior is configurable when `maxKeys` is reached.

**Initial policies**
- `no-eviction` (current behavior)
- `random`
- `ttl-first`
- `lru`

**Done when**
- Policy selectable via config.
- Writes under pressure follow selected policy.
- Metrics expose eviction counts/reasons.

**Key tradeoff**
- LRU gives better cache behavior but adds lock/contention overhead.

---

## 3) Observability (Prometheus)

**Goal:** make runtime behavior visible before HA/persistence work.

**Scope**
- Add `/metrics` endpoint.
- Export request count/latency, key count, eviction count, snapshot timings.

**Done when**
- Prometheus can scrape every node.
- Basic Grafana dashboard exists.

---

## 4) Key-Change Notifications (Watch/SSE)

**Goal:** allow clients to react to key updates without polling.

**Scope**
- Add `GET /kv/{key}/watch` (SSE).
- Emit events on `put` and `delete`.
- Router proxies watch to owning node.

**Done when**
- Long-lived SSE streams are stable.
- Backpressure/limits are in place (max watchers per key/node).

---

## 5) Namespace Isolation

**Goal:** support multi-tenant workloads safely.

**Scope**
- Introduce namespaced paths (`/kv/{namespace}/{key}`).
- Keep `/kv/{key}` as `default` namespace for compatibility.
- Per-namespace limits/config (capacity, default TTL, auth token).

**Done when**
- Namespaces are isolated in storage and limits.
- Router + auth honor namespace config.

---

## 6) Replication (Primary → Replica)

**Goal:** avoid shard data loss on single-node failure.

**Scope**
- Assign optional replica per primary shard.
- Stream writes asynchronously to replica.
- Router can fail over reads/writes on primary failure.

**Done when**
- Primary failure does not lose shard availability.
- Promotion flow includes split-brain protection (epoch/fencing).

**Known tradeoff**
- Async replication means replica lag is possible (RPO > 0).

---

## 7) Durable Storage Backend (LSM)

**Goal:** survive process/node restarts with persistent storage.

**Approach**
- Prefer integrating a proven engine (`pebble` or `badger`) behind `kvStore`.
- Avoid building a custom LSM unless absolutely required.

**Done when**
- Same API semantics with durable reads/writes.
- Crash/restart recovery is verified.

---

## Milestones

### Milestone A — Core Completeness
- #1 DELETE
- #2 TTL/eviction
- #3 Metrics

### Milestone B — Real-time + Multi-tenant
- #4 Watch/SSE
- #5 Namespaces

### Milestone C — Reliability
- #6 Replication
- #7 Durable backend

---

## Out of Scope (for now)

- Cross-region replication
- Strong consistency protocols (Raft/Paxos)
- Custom-built storage engine internals

---

## Success Criteria

- Operators can control memory growth (TTL/eviction) and observe behavior (metrics).
- Clients can perform full key lifecycle operations and optionally watch changes.
- System can evolve from in-memory single-copy to replicated + durable modes without API redesign.
