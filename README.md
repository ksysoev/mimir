# Mimir

[![Tests](https://github.com/ksysoev/mimir/actions/workflows/tests.yml/badge.svg)](https://github.com/ksysoev/mimir/actions/workflows/tests.yml)
[![codecov](https://codecov.io/gh/ksysoev/mimir/graph/badge.svg?token=PE8DPSCWQR)](https://codecov.io/gh/ksysoev/mimir)
[![Go Reference](https://pkg.go.dev/badge/github.com/ksysoev/mimir.svg)](https://pkg.go.dev/github.com/ksysoev/mimir)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

In-memory key-value store with versioning, optimistic locking, and support for **any content type**.

---

## Features

- **Any content type** — store JSON, plain text, images, binary blobs, or any other format. The `Content-Type` header you send on `PUT` is preserved and returned verbatim on `GET`.
- **Versioning & optimistic locking** — every write increments a monotonic version counter. Supply `?ifVersion=<n>` on `PUT` or `PATCH` to perform a conditional update; mismatches return `409 Conflict`.
- **JSON merge-patch** — `PATCH` performs a shallow JSON object merge, allowing partial updates without overwriting unrelated fields.
- **Lean response format** — `GET` returns only the raw stored bytes in the body. Metadata (key name, version) is delivered as HTTP headers so that the body is always machine-readable without unwrapping an envelope.

---

## Installation

### Building from Source

```sh
CGO_ENABLED=0 go build -o mimir -ldflags "-X main.version=dev -X main.name=mimir" ./cmd/mimir/main.go
```

### Using Go

```sh
go install github.com/ksysoev/mimir/cmd/mimir@latest
```

---

## Running

```sh
mimir --log-level=debug --log-text=true --config=runtime/config.yml
```

---

## API Reference

### Health check

```
GET /livez
```

Returns `200 Ok` when the service is healthy.

---

### Store a value — `PUT /kv/{key}`

Store any payload under `{key}`. The `Content-Type` header is preserved and
returned on subsequent `GET` requests. If no `Content-Type` is provided,
`application/octet-stream` is assumed.

**Request**

| Element | Details |
|---|---|
| Header `Content-Type` | MIME type of the payload (optional, defaults to `application/octet-stream`) |
| Query `?ifVersion=<n>` | Conditional write: only succeeds if the current version equals `n` |
| Body | Raw payload bytes — any format |

**Response**

| Header | Description |
|---|---|
| `Content-Type` | Echoes the stored content type |
| `X-Key` | The key that was written |
| `X-Version` | New version number after the write |

Body contains the stored value as-is.

**Status codes**

| Code | Meaning |
|---|---|
| `200` | Write successful |
| `409` | `ifVersion` guard failed (version mismatch) |
| `400` | Malformed `ifVersion` query parameter |

**Examples**

```sh
# Store JSON
curl -X PUT http://localhost:8080/kv/config \
  -H "Content-Type: application/json" \
  -d '{"timeout":30,"retries":3}'

# Store plain text
curl -X PUT http://localhost:8080/kv/greeting \
  -H "Content-Type: text/plain" \
  -d "Hello, world"

# Store binary (e.g. an image)
curl -X PUT http://localhost:8080/kv/logo \
  -H "Content-Type: image/png" \
  --data-binary @logo.png

# Conditional update (only if current version is 2)
curl -X PUT "http://localhost:8080/kv/config?ifVersion=2" \
  -H "Content-Type: application/json" \
  -d '{"timeout":60}'
```

---

### Retrieve a value — `GET /kv/{key}`

Fetch the stored value. The response body contains **only the raw bytes** that
were written. Key metadata is delivered through response headers, not wrapped
in a JSON envelope, so the body can be piped directly into other tools.

**Response**

| Header | Description |
|---|---|
| `Content-Type` | The content type recorded at write time |
| `X-Key` | The key that was read |
| `X-Version` | Current version of the key |

Body contains the stored value as-is.

**Status codes**

| Code | Meaning |
|---|---|
| `200` | Key found |
| `404` | Key does not exist |

**Examples**

```sh
# Retrieve JSON — body is ready to pipe into jq
curl -s http://localhost:8080/kv/config | jq .

# Retrieve binary — save directly to a file
curl -s http://localhost:8080/kv/logo -o logo.png

# Inspect metadata headers
curl -I http://localhost:8080/kv/config
# X-Key: config
# X-Version: 3
# Content-Type: application/json
```

---

### Partially update a value — `PATCH /kv/{key}`

Apply a **shallow JSON merge** to the existing value. Only top-level fields
present in the request body are updated; all other fields are left untouched.

> **Requires `Content-Type: application/json`.**  
> Sending any other content type returns `415 Unsupported Media Type`. This
> restriction exists because merge-patch semantics are only well-defined for
> JSON objects.

**Request**

| Element | Details |
|---|---|
| Header `Content-Type` | Must be `application/json` |
| Query `?ifVersion=<n>` | Optional conditional write |
| Body | Any valid JSON. When both the stored value and this body are JSON objects, top-level fields are shallow-merged (delta wins). Otherwise the stored value is replaced entirely. |

**Response**

Same headers and body format as `PUT`.

**Status codes**

| Code | Meaning |
|---|---|
| `200` | Patch applied |
| `400` | Body is not valid JSON |
| `409` | `ifVersion` guard failed |
| `415` | Content-Type is not `application/json` |

**Example**

```sh
# Initial write
curl -X PUT http://localhost:8080/kv/settings \
  -H "Content-Type: application/json" \
  -d '{"theme":"dark","lang":"en","fontSize":14}'

# Patch: only update fontSize, leave other fields intact
curl -X PATCH http://localhost:8080/kv/settings \
  -H "Content-Type: application/json" \
  -d '{"fontSize":18}'

# GET returns: {"theme":"dark","lang":"en","fontSize":18}
```

---

## Design decisions

### Why move away from a JSON-only API?

The original API accepted and returned only `json.RawMessage`. This worked
well for structured configuration data but created unnecessary friction for
common real-world use cases:

- **Binary assets** (images, compiled artefacts, certificates) had to be
  base64-encoded before storage, inflating payload size and adding
  encode/decode steps on both sides.
- **Plaintext values** (simple flags, tokens, templates) were forced into
  JSON string syntax (`"value"`) even when the consumer had no use for JSON
  parsing.
- **Content negotiation was impossible** — consumers could not distinguish a
  JSON document from a plain string without inspecting the value itself.

By storing payloads as `[]byte` alongside the original `Content-Type`, Mimir
becomes a general-purpose byte store. The cost is zero: binary data is stored
exactly as received and returned without transcoding.

### Why keep PATCH JSON-only?

Merge-patch (RFC 7396) is defined exclusively over JSON objects. Allowing
`PATCH` with arbitrary content types would require implementing separate merge
semantics for each type (XML, CBOR, etc.) or silently falling back to a
full replace, which defeats the purpose of `PATCH`. The `415 Unsupported
Media Type` response makes this constraint explicit and standard — clients
receive a clear error rather than unexpected behaviour.

### Why move key/version metadata to response headers?

The original design returned a JSON envelope:

```json
{ "key": "...", "value": ..., "version": 3 }
```

This forced every consumer to:
1. Parse the outer JSON wrapper, even when the stored value was not JSON.
2. Extract the inner `value` field before using the actual content.

For binary or plaintext payloads this was particularly awkward. Moving
metadata to `X-Key` and `X-Version` headers means the response body is
**always** the verbatim stored value — ready to be saved to disk, piped
into another tool, or deserialized directly, with no unwrapping step. Header
access is O(1) and adds no parsing overhead for callers that do not need
the metadata.

---

## Roadmap

See [ROADMAP.md](ROADMAP.md) for the technical design plans — including TTL & eviction, fork-based consistent snapshots, replication, LSM persistence, and more. Each item covers motivation, component diagrams, expected benefits, tradeoffs, and a rough effort estimate.

---

## License

Mimir is licensed under the MIT License. See the LICENSE file for more details.
