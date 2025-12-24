# Architecture

## Scope
This document expands the data-plane module design for the first milestone.
It focuses on three core modules:
- Config loading
- Protocol conversion
- Forwarding

It complements `README.md` and serves as the longer-term source of truth
as implementation starts.

## Data Plane Flow
1. Config loading compiles Gateway API + AIGateway CRD into snapshots.
2. Router and policy logic select target provider(s).
3. Protocol conversion maps the unified LLM API to provider protocol.
4. Forwarder executes upstream calls with retry, timeout, and observability.
5. Protocol conversion normalizes responses and streams data back.

```
+-------------------+       +---------------------+       +------------------+
| Config Loading    | --->  | Router/Policy       | --->  | Protocol Convert |
| (snapshots)       |       | (select provider)   |       | (encode/decode)  |
+-------------------+       +---------------------+       +------------------+
                                                                    |
                                                                    v
                                                         +------------------+
                                                         | Forwarder        |
                                                         | (upstream call)  |
                                                         +------------------+
```

## Module 1: Config Loading (reference: envoyproxy/gateway)
Goal: compile Gateway API + AIGateway CRD into immutable snapshots consumable
by the data plane, with validation and safe rollout.

Responsibilities:
- Watch K8s resources (Gateway, HTTPRoute, BackendRefs, AIGateway CRDs).
- Validate schema and cross-resource references.
- Merge and normalize into internal models (listeners, routes, providers,
  policies, secrets).
- Publish versioned snapshots with rollback safety.
- Distribute updates to data-plane instances via subscription or push.

Inputs:
- Gateway API resources
- AIGateway CRD resources
- Optional static config (file/env overrides)

Outputs:
- `ConfigSnapshot` containing:
  - `listeners[]`, `routes[]`, `providers[]`, `policies[]`, `secrets[]`
  - `version`, `checksum`, `generated_at`

Internal interfaces (concept):
- `ConfigSource`
  - `List()` returns current resources
  - `Watch()` returns resource events
- `ConfigCompiler`
  - `Compile(resources) -> ConfigSnapshot`
- `ConfigStore`
  - `GetLatest()` and `Get(version)`

Update lifecycle:
1. Build resource index on events.
2. Compile: parse -> validate -> link -> default -> internal model.
3. Publish: reject on validation error; keep last valid snapshot.
4. Hot reload: atomic swap in data plane.

Failure handling:
- Any invalid resource blocks snapshot publication.
- Previous snapshot remains active.
- Emit diagnostics with resource references and reasons.

## Module 2: Protocol Conversion
Goal: translate between unified LLM API and provider-specific protocols while
preserving semantics, streaming behavior, and error codes.

Responsibilities:
- Map unified request model to provider request payloads.
- Normalize provider responses into unified response structure.
- Stream translation (SSE or chunked) and event typing.
- Capability negotiation per model/provider.

Internal interfaces (concept):
- `Codec`
  - `Encode( UnifiedRequest ) -> ProviderRequest`
  - `Decode( ProviderResponse ) -> UnifiedResponse`
  - `DecodeStream( chunk ) -> StreamEvent`
- `CapabilityMatrix`
  - Supported params, defaults, and compatibility fallbacks.
- `ErrorMapper`
  - Provider errors -> unified error codes and messages.

Workflow:
1. Request normalization and validation.
2. `Encode` based on provider + endpoint.
3. Streaming or non-streaming response handling.
4. `Decode` and return unified response; enrich usage if needed.

Extensibility:
- Each provider registers a `Codec` and a `CapabilityMatrix`.
- Registry lookup by provider and endpoint.

## Module 3: Forwarder
Goal: execute upstream requests with high performance, reliability, and
observability.

Responsibilities:
- Connection pooling and HTTP/2 support.
- Timeouts, retries, circuit breaking, concurrency limits.
- Authentication injection per provider (API key, JWT, mTLS).
- Metrics, tracing, and structured logs.

Internal components (concept):
- `UpstreamClient` for a single provider
- `TransportPolicy` for timeout/retry/circuit settings
- `RetryPlanner` for error-based decisions and fallback
- `Forwarder` exposing `Do(ctx, ProviderRequest) -> ProviderResponse`

Workflow:
1. Select upstream target from routing result.
2. Inject auth, send request, manage retries and timeouts.
3. Return response or fallback result with standardized error.

Performance notes:
- Use `net/http` transport with tuned idle conns and TLS reuse.
- Minimize re-encoding and avoid duplicate buffers.
- Mask sensitive headers and payloads in logs.
