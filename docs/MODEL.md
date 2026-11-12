# Portable project model

LCE Studio models software independently from its deployment. Healthcare, payments, and other domains are reference projects or future component packages; none is a built-in simulator type.

## Supported schema versions

| Version | Supported content | Execution |
| --- | --- | --- |
| `1` | Requirements, client/service/database pipeline, workload | Original v0.1 timing and retry semantics |
| `2` | The same software model plus explicit infrastructure and placements | CPU sharing, memory admission, physical-link transfer costs, outages and host energy |
| `3` | Exclusive router workflows, reusable package profiles, optional infrastructure | Deterministic weighted routing, multiple terminal stores, optional deployment constraints |

Version 1 must not include infrastructure or positive memory reservations, since its engine does not interpret them. Version 2 requires an `infrastructure` object. Version 3 can run with or without infrastructure. Unset new optional fields are omitted from JSON serialization so legacy project snapshots retain their previous serialized shape and model hashes. Schema 1/2 reject router nodes, nonzero route weights, component references, and nonempty package locks.

Versions 1/2 execute one directed client → service(s) → database pipeline. Version 3 adds exclusive branches and convergence. Parallel fan-out, synchronized joins, cycles, executable third-party plugins, application generation, real deployment, batteries, migration, and multihop network routing are not implemented.

The authoritative implementation contracts are [CONTINUUM_CONTRACT.md](CONTINUUM_CONTRACT.md) and [WORKFLOW_CONTRACT.md](WORKFLOW_CONTRACT.md). Execution details belong to [SIMULATION.md](SIMULATION.md).

## Exclusive workflows

A schema 3 application is a bounded directed acyclic graph. It has exactly one client, at least one processing node (service or router), and at least one terminal database. Every node is reachable from the client and lies on a path to a database.

| Node type | Incoming edges | Outgoing edges | Behavior |
| --- | --- | --- | --- |
| Client | None | Exactly one | Generates logical requests and retries |
| Service | One or more | Exactly one | Processes and forwards the request |
| Router | One or more | At least two | Processes, then selects one outgoing edge |
| Database | One or more | None | Writes, optionally deduplicating by logical request ID |

Converging paths are permitted. Each logical request follows one selected path; convergence creates no synchronization barrier. Drafts may have missing connections, but self-links, duplicate source/target pairs, and cycles are rejected even before simulation. Service/router `config.deduplicate` must be false in schema 3; database deduplication remains editable.

`edge.routeWeight` applies only to edges from routers. Zero or omission means an effective weight of 1; explicit weights are integers `1..100`. All other edges must omit the field or use zero. The simulator sorts a router's outgoing edges by ID, hashes `decimalSeed + "\n" + requestID + "\n" + routerID` with SHA-256, and uses the first eight bytes as an unsigned big-endian number modulo the sum of weights. The chosen cumulative-weight interval identifies one edge. This gives a stable route for each logical request across retries, independent of jitter and input edge order. Small workloads need not match the weights' exact proportions. Changing an edge ID can change route selection.

Responses return along the selected attempt's path. Deduplication and duplicate-write evidence are scoped to `(database node ID, logical request ID)`. A committed write alone does not establish client completion. Schema 3 follows the deployment simulator's timeout-drain observation horizon, including when infrastructure is absent.

## Declarative component packages

An engineer can define a reusable profile for an existing `service`, `database`, or `router` kernel. A package supplies metadata, bounded editable defaults for processing time, capacity, and memory, and a database deduplication default. It contains no scripts or external executable modules. Adding a new execution primitive still requires core implementation work; this increment is the declarative subset of a future extension SDK.

The package manifest has its own `schemaVersion: 1`, independent of the project version. It declares `id`, `version`, `name`, `description`, `author`, `license`, `kernel`, `capabilities`, and `config`:

| Field | Constraint |
| --- | --- |
| `id` | Safe identifier, at most 128 bytes, containing a dot, such as `example.queued-worker` |
| `version` | Exact three-part decimal version, at most 32 bytes; no leading zeroes, ranges, or suffixes |
| `name`, `author`, `license` | Required valid text, at most 200 bytes each |
| `description` | Optional valid text, at most 4000 bytes |
| `kernel` | `service`, `database`, or `router` |
| `capabilities` | Exactly `["design", "simulate"]` in that order |
| `config.processingMs` | `{default, min, max}` integers within `0..60000` |
| `config.capacity` | `{default, min, max}` integers within `1..100` |
| `config.memoryMb` | `{default, min, max}` integers within `1..1048576` MiB |
| `config.deduplicate` | Boolean default; false for service/router packages |

Each interval must satisfy `min <= default <= max`. Instantiation copies defaults into an ordinary node and adds a `component` reference containing `packageId`, `version`, and `digest`. The project embeds one `{package, digest}` lock per exact package ID/version in `packages`, with at most 50 locks. Several nodes can reference the same lock; unused valid locks are allowed.

Package-backed node types must match their kernel, and their configuration must remain within the declared intervals. Memory checks use the effective 64 MiB default when a node's memory is omitted/zero. Validation does not clamp values or replace them with defaults. A database node can change its deduplication setting independently from the profile's initial value.

The digest is 64 lowercase hexadecimal characters: SHA-256 of Go `encoding/json.Marshal(ComponentPackage)` using the exact typed field order in [WORKFLOW_CONTRACT.md](WORKFLOW_CONTRACT.md). Whitespace and input JSON key order do not affect it. This encoding is not RFC 8785 canonicalization, and the digest is a content identifier rather than proof of publisher trust. Both the lock and every referencing node must agree with the embedded content.

The PostgreSQL catalog stores immutable installed versions, but project validation and simulation resolve embedded locks without consulting that catalog. Exports and saved run snapshots therefore carry the complete package definitions. A self-consistent edited definition can be imported as a new project snapshot; installation of conflicting content at an already installed ID/version returns a conflict.

Starter manifests are available in [queued-worker.json](../examples/components/queued-worker.json), [durable-store.json](../examples/components/durable-store.json), and [weighted-router.json](../examples/components/weighted-router.json), and through `GET /api/components/examples`.

## Infrastructure entities

| Entity | Definition | Meaning |
| --- | --- | --- |
| Site | `id`, `name`, `latitude`, `longitude` | A named geographical location |
| Host | `id`, `name`, `siteId`, `tier`, `cpuCores`, `memoryMb`, `idleWatts`, `activeWatts`, `outages` | A powered computing resource at a site |
| Network link | `id`, `sourceHostId`, `targetHostId`, `latencyMs`, `bandwidthKbps`, `outages` | One direct undirected connection between hosts |
| Placement | `nodeId`, `hostId` | Assignment of one software node to a computing resource |
| Outage | `startMs`, `endMs` | Unavailability on the half-open interval `[startMs, endMs)` |
| Infrastructure | `sites`, `hosts`, `links`, `placements`, `messageBytes` | The deployment environment and fixed payload size for link crossings |

Sites, hosts, and links have unique IDs across their infrastructure namespace and cannot reuse application node IDs. Application edges, requirements, and their original identifiers remain unchanged. Host tiers are `device`, `edge`, `regional`, or `cloud`; neither tier nor geographic distance automatically generates network latency or processing speed.

Arrays are explicit, including empty arrays: use `outages: []` for an always-powered host or always-available link. An omitted or null infrastructure collection is invalid. All quantities have explicit units: integer milliseconds, MiB of memory, bytes of payload, decimal kbit/s of bandwidth, watts of power, and watt-hours of estimated energy.

## Drafts and executable placements

Saving and importing a draft validates bounded values, identities, and references, while allowing incomplete software topology or missing placements. This lets an engineer save work before every application has been placed or all resource requirements have been satisfied.

When infrastructure is present, simulation additionally requires:

- Exactly one placement for every application node, including the client.
- A direct physical link for every application edge whose endpoints are placed on different hosts. A same-host edge needs no physical link. Indirect paths are not inferred.
- Sufficient host memory for the sum of all resident application reservations. `node.config.memoryMb` omitted or zero means **64 MiB** in versions 2/3.
- A complete application topology for the selected schema and all workload constraints.

Memory is reserved statically; it is not a dynamic allocator or an out-of-memory model. Node `capacity` limits processing concurrency, while colocated nodes share host CPU cores. A total of node capacities greater than the host core count is permitted so CPU contention can be simulated.

Without infrastructure, schema 3 applies per-node capacity and logical-link delays/jitter. Memory profile bounds still validate, but the run fabricates no host memory admission, host IDs, physical-link metrics, or energy estimates.

Duplicate placements, self links, or duplicate unordered host pairs are invalid even in drafts. Deleting a referenced object requires updating or removing its dependent objects; dangling references are not silently repaired by the backend.

## Bounds

| Property | Limit |
| --- | --- |
| Application nodes / edges / requirements | 30 / 30 / 50 |
| Embedded package versions | 50 |
| Workload requests / retries | 200 / 5 |
| Router output weight | Integer `0..100`; zero means effective weight 1 |
| Sites / hosts / placements | 30 of each |
| Physical links | 60 |
| Outage windows | 20 per host or link |
| Latitude / longitude | Finite `[-90,90]` / `[-180,180]` |
| Host CPU cores | Integer `1..128` |
| Host memory | Integer `1..1048576` MiB |
| Node memory | Integer `0..1048576` MiB; zero means the default |
| Idle / active power | Finite `0..1000000` W; active must be at least idle |
| Link latency | Integer `0..60000` ms |
| Link bandwidth | Integer `1..10000000` kbit/s |
| Message size | Integer `1..10485760` bytes |
| Outage interval | `0 <= startMs < endMs <= 3600000` |

Outage windows may be supplied in any order. They cannot overlap; touching windows are allowed. Validation sorts a copy when checking overlaps and never mutates the project.

All original application and workload bounds also apply. A structurally valid experiment can still exceed the simulator's event or time budget; that condition returns an error rather than a passing partial result.

## Generic reference experiment

`GET /api/example?template=continuum` and [continuum-placement.json](../examples/continuum-placement.json) describe the same healthy baseline:

- A request client on `device-host`.
- A request service and durable store on `edge-host`.
- A separate `cloud-host` available for alternative placement.
- A local device/edge link and two explicit WAN links.
- Twelve requests, 1024-byte physical-link payloads, and bounded retries.
- No configured outages initially; both edge and cloud arrangements have valid direct connectivity.

Run the baseline, then move both `service` and `database` to `cloud-host` and run again. Compare immutable run snapshots to see the effect of physical-link delay, bandwidth, host placement, and energy profiles.

For a deliberate failing case, return the service and database to `edge-host` and add a host outage `{ "startMs": 0, "endMs": 5000 }`. This spans the requests and all their permitted retries, so successful completion is not expected. For a disconnected cloud case, use the same interval on `device-wan` after moving the application to the cloud.

The numerical hardware/power profiles are illustrative assumptions, not measured predictions. Geography supplies location metadata only. Total energy covers the reported run horizon, including configured environmental transitions and unused powered hosts; compare duration and availability as well as energy before drawing conclusions about placement efficiency.

## Reusable branching experiment

`GET /api/example?template=branching` and [branching-workflow.json](../examples/branching-workflow.json) describe a schema 3 workflow with a request client, one exclusive router, and separate edge/cloud worker-and-store paths. Both worker nodes share the `lce.queued-worker@1.0.0` profile; both stores share `lce.durable-store@1.0.0`. The router uses `lce.weighted-router@1.0.0`. All three exact definitions and digests are embedded.

The edge/cloud route weights are 3/1. Run it, change weights or host conditions, and compare snapshots. Removing the optional infrastructure gives the same application graph a per-node simulation without physical-resource estimates. The branching example is a generic reusable-component experiment and carries no healthcare-specific semantics.

## Run evidence

Version 2 and placed version 3 runs retain their exact project snapshot and add `infrastructure` results:

- Total host energy in Wh.
- Per-host energy, occupied core-milliseconds, uptime, and interrupted processing-job counts.
- Per-link attempted transfers, successfully delivered payload bytes, and dropped transfer counts.

Trace events can carry `hostId` or `linkId` for environmental events. They need not invent an application `nodeId` or `requestId` when none applies. A schema 3 `route_selected` event identifies the router, request, attempt, and selected application `edgeId`. The original application metrics, verdict types, and stored-run identity fields remain unchanged.
