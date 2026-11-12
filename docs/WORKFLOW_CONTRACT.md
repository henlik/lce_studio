# v0.3 workflow and declarative package contract

This is the implementation contract for the v0.3 increment. It adds exclusive branching and reusable, parameterized simulation components. External executable plugins, parallel fan-out/join, application generation and real deployment remain future work. v1/v2 serialized fields and execution semantics remain compatible.

## Schema 3 application graph

- `schemaVersion: 3` permits optional infrastructure, `router` nodes, weighted routing edges, package locks and node component references. Schema 1/2 reject these new declarations rather than ignoring them. Optional new fields use `omitempty` in Go so old JSON hashes retain their shape.
- Existing limits remain: at most 30 nodes, 30 application edges, 50 requirements, 200 requests, 5 retries, 100000 trace events, and one hour virtual time.
- Exactly one client has no incoming edges and exactly one outgoing edge. At least one processing node (service or router) and one terminal database are required. Every other node is reachable from the client. Every node lies on a path to a database. The graph is acyclic; self-links and duplicate source/target pairs are rejected, including in drafts.
- A service has one or more incoming edges and exactly one outgoing edge. A router has one or more incoming edges and at least two outgoing edges. A database has one or more incoming edges and no outgoing edges. Convergence is permitted: it receives one selected path per logical request, with no synchronization barrier. It is not a parallel join.
- Drafts may omit connections/placements, but malformed references, unsupported declarations, duplicate edges and cycles are rejected.
- `Edge.RouteWeight int json:"routeWeight,omitempty"`: on edges FROM routers, effective weight is 1 when omitted/zero; explicit values are 1..100. On non-router edges the field must be omitted/zero. Negative or >100 values are invalid.
- A router consumes processing time/capacity/host CPU like a service and then chooses exactly one outgoing edge. It never duplicates a request into multiple branches. Schema3 service/router deduplicate flags must be false; deduplication belongs to databases.
- Router output edges are sorted lexicographically by edge ID. Calculate SHA-256 of UTF-8 `decimalSeed + "\n" + requestID + "\n" + routerID`. Interpret the first eight hash bytes as unsigned big-endian, take modulo the sum of effective weights, and choose the corresponding cumulative-weight interval. Request IDs are `request-001`, etc. This is reproducible weighted selection, not a guarantee of exact proportions in a small sample.
- The route is independent of event timing, edge array order and jitter RNG. Each logical request reuses the same route on every retry. A response follows that attempt's selected path in reverse.
- Deduplication and duplicate-write counts are scoped to `(database node ID, logical request ID)`. Committed writes survive the stated power-loss model. Completion still means the client receives success, not merely that a store committed a write.
- Schema3 uses the v2 timeout-drain observation horizon. Infrastructure constraints, direct physical routes, resource sharing, outage ordering and energy accounting follow CONTINUUM_CONTRACT.md for every application edge. With no infrastructure, only per-node processing capacity and logical link delay/jitter apply: no host IDs, physical-link metrics, memory admission or energy estimates are fabricated.
- `TraceEvent.EdgeID string json:"edgeId,omitempty"` identifies a selected application edge. A `route_selected` event identifies router node, request, attempt and edge. Other environment/node events retain their existing identity fields.

## Declarative package schema

These packages customize bounded service/database/router kernels. They contain no scripts, URLs to execute, native plugins, arbitrary property panels, or deployment actions. Engineers can install a new parameter profile without modifying core source; defining a new execution primitive still requires future SDK work.

```go
type IntegerParameter struct {
    Default int64 `json:"default"`
    Min     int64 `json:"min"`
    Max     int64 `json:"max"`
}
type ComponentConfig struct {
    ProcessingMs IntegerParameter `json:"processingMs"`
    Capacity     IntegerParameter `json:"capacity"`
    MemoryMB     IntegerParameter `json:"memoryMb"`
    Deduplicate  bool             `json:"deduplicate"`
}
type ComponentPackage struct {
    SchemaVersion int             `json:"schemaVersion"`
    ID            string          `json:"id"`
    Version       string          `json:"version"`
    Name          string          `json:"name"`
    Description   string          `json:"description"`
    Author        string          `json:"author"`
    License       string          `json:"license"`
    Kernel        string          `json:"kernel"`
    Capabilities  []string        `json:"capabilities"`
    Config        ComponentConfig `json:"config"`
}
type PackageLock struct {
    Package ComponentPackage `json:"package"`
    Digest  string           `json:"digest"`
}
type ComponentRef struct {
    PackageID string `json:"packageId"`
    Version   string `json:"version"`
    Digest    string `json:"digest"`
}
// Project.Packages []PackageLock `json:"packages,omitempty"`
// Node.Component *ComponentRef `json:"component,omitempty"`
```

- Package schema version is exactly 1. ID uses the existing safe ID alphabet and length and must contain at least one dot (example `example.queued-worker`). Version is an exact three-part non-negative decimal version, with no leading zero except `0`, no range/prerelease/build suffix; at most 32 bytes total.
- Name is required, at most 200 bytes; description at most 4000; author and licence are required, at most 200 each. Existing valid-text/control-character rules apply.
- Kernel is exactly `service`, `database`, or `router`. Capabilities must be exactly `["design","simulate"]` in this order. Unsupported kernels/capabilities are rejected; the studio does not advertise generation/deployment support for these packages.
- Configuration intervals satisfy `min <= default <= max`. Processing milliseconds are within 0..60000; capacity within 1..100; memory within 1..1048576 MiB. Memory bounds apply to effective memory (64 MiB when a node reservation is omitted/zero). Non-database package deduplicate defaults must be false. The database deduplicate default is editable on a node, rather than a locked guarantee.
- Instantiation copies the manifest defaults into ordinary node configuration, adds a component ref, and pins the full manifest once in the project. Node type must equal the referenced kernel. Existing package-backed node values must remain within the profile bounds; the simulator validates, never silently clamps or substitutes defaults.
- At most 50 embedded package locks. Each `(id,version)` is unique; every node reference must resolve to the matching embedded ID, version and digest. Unused valid locks are allowed for reuse. Locks are portable with exported projects and immutable run snapshots; catalog lookup is not needed to validate or simulate them.
- A digest is 64 lowercase hexadecimal characters: SHA-256 of Go `encoding/json.Marshal(ComponentPackage)` using the exact field order above and the declared integer types. This typed serialization is the package canonicalization, not RFC 8785. Whitespace and input JSON key ordering do not change it. A digest identifies content; it is not a publisher signature or trust certificate.
- Importing edited definitions requires internally consistent locks/refs and produces a different project hash. Catalog conflicts are handled by installation; simulation of portable embedded packages does not depend on a particular local catalog's contents.

## Go model API

The model package supplies `ValidateComponentPackage(ComponentPackage) []ValidationIssue`, `ComponentDigest(ComponentPackage) (string,error)`, `ExampleComponentPackages() []ComponentPackage`, and `BranchingExampleProject() Project`. Package validation integrates into both draft and execution validation. Examples use generic concepts and include reusable worker, durable-store and weighted-router packages.

## Catalog HTTP and persistence

- `GET /api/components`: all installed records ordered by package ID then version. Each record is `{package: ComponentPackage, digest: string, installedAt: RFC3339 string}`.
- `POST /api/components`: accepts one raw `ComponentPackage`, using existing strict JSON/body/origin constraints. New installation returns 201 and the record; identical canonical digest at the same ID/version returns 200 with the existing record; a different digest at the same ID/version returns 409 without overwriting. Invalid manifests return 422 with validation issues.
- `GET /api/components/examples`: bundled `PackageLock[]`, available to install through the same POST endpoint. These are starter profiles, not external network dependencies.
- Catalog supports at most 200 installed versions. Capacity is enforced transactionally; a new version beyond capacity returns 409, identical reinstallation remains successful. Versions are immutable; no delete or overwrite endpoint is introduced.
- `GET /api/example?template=branching`: portable schema3 example with package-backed nodes and edge/cloud placement. Other examples remain unchanged. Unknown templates remain 400.
- Projects and runs continue to persist JSON in PostgreSQL; a new table stores catalog versions with a unique `(id,version)` key. Concurrent identical/different installs cannot violate immutability or capacity.

## Editor requirements

Design supports the router and weight editing, catalog listing/import/export/installation, package identity/bounds in properties, and insertion from manifest defaults. The same worker profile is reusable at several nodes. Graph labels distinguish exclusive routing from parallel execution. Schema3 upgrades are explicit in the UI, preserve the current design, and are never silently downgraded when infrastructure is enabled/removed. Project comparison includes locks and component refs. Built-in v1/v2 editing remains available.

## Acceptance evidence

Independent cases must cover branch latency/counts, exact route choice, retry stability, convergence without parallel execution, per-store idempotency, branch resource/outage effects, deterministic snapshots, catalog conflict/concurrency/capacity, malformed/tampered/missing package refs, portable project import/export, and preservation of both existing acceptance suites. Documentation must distinguish this implemented declarative package subset from the future executable SDK.
