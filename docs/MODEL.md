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

The authoritative implementation contracts are [CONTINUUM_CONTRACT.md](CONTINUUM_CONTRACT.md), [WORKFLOW_CONTRACT.md](WORKFLOW_CONTRACT.md), and [COMPOSITION_CONTRACT.md](COMPOSITION_CONTRACT.md). Resource simulation details belong to [SIMULATION.md](SIMULATION.md). Pure component evaluation is a separate capability and does not add a project schema.

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

An engineer can define a reusable schema 1 profile for an existing `service`, `database`, or `router` kernel. A profile supplies metadata, bounded editable defaults for processing time, capacity, and memory, and a database deduplication default. It contains no scripts or external executable modules. Package schema 2 separately supports the pure typed compositions described below; these cannot be inserted into project schemas 1–3.

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

## Pure typed composite packages

Package schema 2 defines a recursively reusable pure function in the Components workbench. Its kernel is `composite` and its capabilities are exactly `["design", "evaluate"]`. It retains the legacy `config` object with all values zero, solely to preserve the established typed representation; this object supplies no resource costs. The new `behavior` and optional `tests` fields are appended after the original fields. Omitting these additions keeps every schema 1 profile's bytes and digest unchanged. Schema 1 rejects nonempty tests or a behavior definition.

An executable composite can contain supported pure primitives and calls to other exact composite versions. No executable source code, network operation, storage effect, clock, random input, or implicit authorization enforcement is provided. The evaluator has no virtual time or hardware cost model. Composition adds functional behavior without claiming that nested nodes are deployed services or independently atomic steps in a distributed system.

| Behavior field | Meaning |
| --- | --- |
| `inputs`, `outputs` | Named public ports with an exact scalar type |
| `nodes` | Local primitive operations or pinned component calls, with explicit input bindings and canvas positions |
| `bindings` | One source binding for each public output |
| `tests` on the package | Named input/expected-output cases included in the package digest |

Port names and local node IDs use 1–64 letters, digits, and underscores, starting with a letter. A node declares exactly one `op` or `component` reference. A binding selects exactly one source: a public `input`, a local `node` plus output `port`, or a typed `literal`. Every input must be bound with no extras. Every local node must contribute transitively to a public output; unused internal execution is rejected. Behavior port/node collections must be arrays, including `[]` when empty; bindings and node inputs must be objects, including `{}` when empty.

Values are explicitly tagged: `{ "type": "boolean", "boolean": false }`, `{ "type": "integer", "integer": 0 }`, or `{ "type": "string", "string": "" }`. Exactly one matching value member is present. There are no null values, floating-point values, implicit conversions, or truthiness. Integers are bounded by ±9007199254740991; strings are valid UTF-8 of at most 4096 bytes with the existing text/control-character rules. False, zero, and the empty string are valid values.

Supported primitives are `identity`, `equal`, `less_than`, `less_or_equal`, `and`, `or`, `not`, `select`, `add`, `subtract`, and `concat`. Each has one output named `result`. Generic `T` in primitive discovery means one invocation-local scalar type; public ports must declare a concrete type. Equality requires identical types. `select` eagerly computes both candidate values before selecting; it is not conditional control flow. Arithmetic overflow and an oversized concatenated string fail explicitly.

## Portable composite bundles

A bundle has `schemaVersion: 1`, one exact `root` reference, and `packages` containing the full transitive closure of `{package, digest}` locks. All locks must be composite schema 2 packages, unique by ID/version, reachable from the root, and consistent with every reference. Missing dependencies, extra unused locks, digest mismatches, local graph cycles, and dependency cycles are rejected. Validation/evaluation never resolves a mutable catalog or an external network location.

The package digest still hashes typed Go JSON. The behavior, tests, display metadata, and canvas positions all participate; this identifies exact content rather than behavioral equivalence or publisher trust. A bundle hash separately hashes the complete typed bundle, including lock order. Catalog installation keeps the full closure immutable and checks conflicts atomically. Project schemas 1–3 reject composite locks and references, including unused locks, so their simulation cannot silently ignore functional behavior.

| Composite bound | Limit |
| --- | --- |
| Public inputs / outputs | `0..16` / `1..16` |
| Local nodes / declared tests | `0..32` / `0..32` |
| Canonical manifest size, including tests | 128 KiB |
| Bundle definitions | `1..50` |
| Nesting depth, including root | 8 |
| Expanded primitive executions per evaluation | 256 |
| Expanded component invocations per evaluation | 256 |
| Serialized successful evaluation result | 1 MiB |

Expansion counts repeated calls separately. Reusing one component many times therefore consumes the same work budget as separate calls, even when it has no primitives. Collection bounds are checked before recursive work. Local dataflow executes in stable topological order using the lexicographically smallest ready node ID; child calls execute depth-first at their call node. Diagnostic and evidence map traversal is sorted.

Evaluation returns typed outputs and bounded evidence with `component_enter`, `primitive`, and `component_exit` events. Root path is `$`; nested nodes append `/nodeID`. Errors, cancellation, invalid values, and bound violations return no successful partial evaluation. Tests run the root's declared cases in definition order; a false decision can pass a test, while an evaluation error fails that case. An empty test list does not count as verification.

## Shared access-policy examples

[health-record-access.json](../examples/composites/health-record-access.json) and [school-record-access.json](../examples/composites/school-record-access.json) are complete portable bundles. Their wrappers call the identical `lce.access-policy@1.0.0`, which calls the identical `lce.same-tenant@1.0.0`. This demonstrates three composition levels and reuse across healthcare and education without domain-specific evaluator primitives.

The shared policy permits only when tenant equality, role equality, scope equality, and active status are all true. The wrappers bind `clinician`/`record:read` and `teacher`/`student:read` respectively. Each has cases for permit, wrong tenant, wrong role, wrong scope, and inactive subject. Missing or mistyped values fail evaluation instead of producing an implicit decision. Each wrapper performs six primitives across three component invocations.

These synthetic examples illustrate executable composition. They do not implement real authentication, consent management, IAM provisioning, or a production authorization boundary. The studio's own access control is separate from the policy being modeled.

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

## Reusable data contracts (v0.5)

Data contracts are separately versioned immutable definitions, with exact ID/version/digest locks. They describe closed named records, entity identity and relationships. They do not allocate databases, execute queries or enforce production tenancy. Both reference datasets use `lce.organization-registry@1.0.0` unchanged; healthcare and education supply different rows.

A record has 1–16 required scalar fields (`boolean`, browser-safe `integer`, `string`). Unknown, missing and null fields are invalid. False, zero and empty strings remain values. Each entity selects one record, a primary-key tuple and optional unique-key tuples. Keys contain 1–4 distinct fields and use exact typed tuple equality; the validator never joins display strings to infer identity.

A relationship maps a required source tuple to one declared primary/unique target tuple with matching types and order. Each source row must find exactly one target. `minSources` is 0 or 1; `maxSources` is 0 (unbounded) or 1, and both apply to the number of source rows per target. Many-to-many relationships use an explicit join entity. Relationship cycles and self references are legal because checks inspect a finite snapshot. They imply no cascades, recursive record values, transactions or effects.

Fixture validation checks a complete bounded snapshot. Omitted entities are empty; unknown entity names, null row arrays and null row objects are invalid. The checker reports field, uniqueness, required-reference and reverse-cardinality failures. Invalid keys are never indexed, and duplicate target keys never silently select a row. At most 256 rows and 512 KiB of typed JSON are accepted. Diagnostics follow sorted entity/field order and declared relationship order. Only the first 200 issue details are retained; `issuesTruncated` and `valid:false` preserve the failed verdict. Invalid contract locks and over-budget inputs are errors rather than recorded verdicts. Valid and invalid bounded snapshots can both be saved as immutable evidence.

The data contract limit is 32 records, 32 entities, 64 relationships and 64 KiB of typed JSON. Every required definition array must be present and nonnull, including empty `entities`, `relationships` and `uniqueKeys` arrays. Metadata and names follow the component conventions. Hashing uses the declared Go struct order and `json.Marshal`, with sorted map keys where maps appear; it is exact content identity, not RFC8785, publisher trust or behavioral equivalence.

Compatibility comparison is explicitly `old-to-new`: it conservatively reports whether values and complete datasets accepted by the old declaration remain accepted by the new one. Required-field changes, removed records/entities, new constraints and tightened cardinalities are breaking. Removed constraints and relaxed cardinalities can be nonbreaking; metadata and display ordering are informational. Exact nominal locks remain mandatory regardless of the report. No data migration or automatic dependency upgrade occurs.

Portable files are under `examples/data/`: one contract lock, two fixture files, and combined `healthcare-registry.check.json` / `education-registry.check.json` inputs for `datacheck --fixture`. Exact fields, HTTP responses and structured-component integration are specified in [DATA_CONTRACT.md](DATA_CONTRACT.md).

## Owned state and transitions (v0.6)

A state machine is a separate immutable artifact containing owned slots, public commands and effect declarations. Each command embeds a complete exact pure component bundle as its decision function. A decision reads explicitly bound command inputs, owned state or literals and returns typed outputs. The command maps those outputs to its Boolean acceptance decision, proposed state writes, public results and optional effect intentions. Pure components retain their existing schemas, digests and evaluation semantics; they cannot perform state writes or external I/O.

Slots declare a nominal scalar, record or collection type, an initial value, and `durable` or `volatile` recovery behavior. A scenario creates independent instances, applies bounded initial overrides and runs an explicit ordered sequence of commands and restarts. No instance can address another instance's state. Machine validation checks nominal bindings without executing decisions. Scenario validation checks actual overrides, command inputs and independently supplied final expectations, including the complete state formed by merging defaults and overrides.

Each command carries an instance-scoped command ID and expected revision. Matching receipt retries return the original historical result without evaluating again or restoring old state; reuse with changed request content conflicts. Stale revisions conflict before evaluation. Accepted commands atomically publish proposed state, an incremented revision, a durable receipt and enabled effect intents. Denied commands retain their receipt and public result but leave state, revision and effects unchanged. A before-commit fault rolls back the entire proposal; an after-commit lost reply preserves committed facts for retry. Restart preserves durable slots, revisions, receipts and effects while resetting volatile slots to that instance's effective initial values.

Effects are typed pending intentions with deterministic IDs, not delivered messages or external writes. Durability models an explicit restart inside one experiment. Every new run starts from its declared initialization; PostgreSQL stores immutable machine and experiment evidence rather than the modeled application's live state. This increment adds no resource timing, deployment, asynchronous delivery or external exactly-once guarantee.

Machines contain 1–8 slots, 1–8 commands and at most 8 effect declarations. Scenarios contain 1–8 instances and 1–64 ordered steps. State, public command input/result and effect-payload maps are bounded to 8 values and 64 KiB, in addition to existing structured-value rules. Machine and scenario canonical JSON are each bounded to 512 KiB. Shape checks precede enclosing serialization. Exact data/package locks may repeat across decision bundles, but conflicting ID/version digests fail globally; machine-local data locks must be unique. Decision bundles must resolve independently without borrowing machine-only data locks.

Expected final state, revisions, receipt counts and effect counts are supplied independently. Empty expectations mean unverified; an intentional denial, conflict, rollback or ordinary evaluation error can be part of a passing experiment when its expected final state matches. See [STATE_CONTRACT.md](STATE_CONTRACT.md) for the complete protocol, model serialization order, engine budgets and APIs.

## Typed message delivery (v0.7)

A delivery model composes exact independently valid owned-state machines through named instances and typed routes. Each route maps every recipient command input to a source effect payload port of the same scalar or nominal type. It cannot extract fields or evaluate expressions; reusable pure components perform adaptations inside the owned machines. Valid unused machine locks are allowed, but conflicting component or data ID/version digests anywhere in the model are rejected. Duplicate source/effect/target/command route tuples are invalid. Explicit fanout, self-routes and cycles are supported.

A scenario supplies one initial override map per model instance and an ordered sequence of `command`, `restart`, `dispatch` and `deliver` steps. Dispatch selects one source command's committed intention on one route, freezes its mapped recipient inputs and expected revision, and names the resulting message. It never executes the recipient. A later delivery explicitly attempts that frozen command. Missing intentions, lost delivery, rollback, stale recipient revisions and lost acknowledgements are observable outcomes; no retry, revision rebasing or queue draining happens automatically.

The `sourceMessage` selector identifies a prior declared dispatch whose target owns a downstream intention. This permits explicit multi-hop sequences and cycles without asking users to enter generated command hashes. Alias declarations must preserve their route and source selector, and one selector/route cannot be declared under several aliases. A prior dispatch may still fail at runtime; later steps then record the missing message or intention. The `msg_` prefix is reserved for generated recipient command IDs in delivery scenarios only; existing state scenarios retain their original ID rules.

Acknowledgement proves that the recipient committed a receipt, including a business denial. It is distinct from business acceptance. A lost acknowledgement can be retried against the recipient's durable historical receipt, while before/after snapshots continue to show current state. Modeled owner restarts preserve transport messages, attempts and acknowledgements as an explicit durable-transport assumption, and reset only the owner's volatile slots. Source intentions remain available for retries and explicit fanout. This increment provides no broker, clock, external delivery, distributed transaction or resource scheduling.

Models contain at most 4 machine locks, 1–8 instances and 16 routes, with a 1 MiB canonical JSON limit. The complete union is bounded to 64 component locks and 32 data contracts. Scenarios have 1–64 steps, at most 8 state expectations and 32 message expectations, and 512 KiB canonical JSON. Initial overrides, merged state and actual command values retain the 64 KiB state-map bound. Shape validation precedes enclosing serialization and execution copies.

State and message expectations are supplied independently. Message assertions distinguish absent, pending and acknowledged envelopes and require an explicit nonnull attempt count; acknowledged assertions may also specify the recipient's business verdict. Partial assertion lists cover only the named owners/messages, while empty lists mean unverified. Exact fields, ownership rules, frozen-envelope semantics and shared execution budgets appear in [DELIVERY_CONTRACT.md](DELIVERY_CONTRACT.md).
