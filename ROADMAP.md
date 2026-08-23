# Mimir — Product Roadmap

## Delivery Order

| # | Feature | Why now | Effort |
|---|---------|---------|--------|
| 1 | DELETE operation | Completes key lifecycle; unblocks cleanup flows | 1–2 days |
| 2 | TTL expiry | Controls stale data lifecycle | 3–5 days |
| 3 | HTTP cache headers (configurable) | Simplifies client-side caching after TTL support | 2–4 days |
| 4 | Eviction policies | Controls memory growth under capacity pressure | 4–7 days |
| 5 | OpenTelemetry observability | Needed to operate and tune safely | 2–3 days |
| 6 | Watch/SSE notifications | Removes polling overhead for clients | ~1.5 weeks |
| 7 | Namespaces | Multi-tenant isolation and limits | ~2 weeks |
| 8 | Replication (primary→replica) | High availability / shard redundancy | 4–6 weeks |

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

## 2) TTL Expiry

**Goal:** keys can expire automatically.

**Scope**
- Support TTL on write (`?ttl=<seconds>`).
- Expired keys are hidden from reads immediately (lazy check).
- Background cleanup removes expired keys over time.

**Done when**
- Expired keys return `404`.
- Cleanup keeps memory bounded for expired data.
- Response may include expiry metadata (e.g., `X-Expires-At`).

---

## 3) HTTP Cache Headers

**Goal:** make client-side caching easier and consistent with TTL semantics.

**Scope**
- Add configurable cache header behavior on read responses.
- Support headers such as `Cache-Control`, `ETag`, and `Last-Modified`.
- Allow policy by config (e.g., disabled, ttl-based, custom max-age).
- Ensure `DELETE`/`PUT`/`PATCH` flows invalidate cached content correctly.

**Done when**
- Clients can cache GET responses using standard HTTP cache semantics.
- Header behavior is predictable and documented.
- Conditional requests (`If-None-Match` / `If-Modified-Since`) return `304` when appropriate.

---

## 4) Capacity Eviction Policies

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

## 5) Observability (OpenTelemetry)

**Goal:** make runtime behavior visible before HA work.

**Scope**
- Add OpenTelemetry SDK instrumentation.
- Export request count/latency, key count, eviction count, snapshot timings via OTLP (and optional Prometheus bridge if needed).

**Done when**
- Telemetry is exportable to an OpenTelemetry Collector.
- Basic dashboard exists in the chosen backend (e.g., Grafana, Datadog, Tempo-compatible stack).

---

## 6) Key-Change Notifications (Watch/SSE)

**Goal:** allow clients to react to key updates without polling.

**Scope**
- Add `GET /kv/{key}/watch` (SSE).
- Emit events on `put` and `delete`.
- Router proxies watch to owning node.

**Done when**
- Long-lived SSE streams are stable.
- Backpressure/limits are in place (max watchers per key/node).

---

## 7) Namespace Isolation

**Goal:** support multi-tenant workloads safely.

**Scope**
- Introduce namespaced paths (`/kv/{namespace}/{key}`).
- Keep `/kv/{key}` as `default` namespace for compatibility.
- Per-namespace limits/config (capacity, default TTL, auth token).

**Done when**
- Namespaces are isolated in storage and limits.
- Router + auth honor namespace config.

---

## 8) Replication (Primary → Replica)

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

## Milestones

### Milestone A — Core Completeness
- #1 DELETE
- #2 TTL expiry
- #3 HTTP cache headers
- #4 Eviction policies
- #5 Observability

### Milestone B — Real-time + Multi-tenant
- #6 Watch/SSE
- #7 Namespaces

### Milestone C — Reliability
- #8 Replication

---

## Out of Scope (for now)

- Cross-region replication
- Strong consistency protocols (Raft/Paxos)
- Custom-built storage engine internals

---

## Success Criteria

- Operators can control stale data and memory growth (TTL + eviction) and observe behavior (metrics).
- Clients can perform full key lifecycle operations and optionally watch changes.
- System can evolve from in-memory single-copy to replicated mode without API redesign.
