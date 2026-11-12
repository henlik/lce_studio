# LCE Studio v0.4 integration contract

This document defines the shared contract between the Vue 3 + TypeScript editor and the Go API/simulator. See README.md for setup and SIMULATION.md for execution semantics. CONTINUUM_CONTRACT.md defines the additive schema v2 infrastructure model and result fields. WORKFLOW_CONTRACT.md defines schema3 exclusive branching, declarative packages, portable locks and catalog APIs. Schema1 and schema2 remain supported. COMPOSITION_CONTRACT.md defines package schema2, typed pure components, bundles, functional evaluation/tests and immutable evaluation history. Project schemas and timed simulation semantics remain unchanged; the resource simulator still reports engine version 0.3.0.

## Scope

Single-user local-first engineering studio. Editable requirements, topology, node/link properties and scenario; deterministic discrete-event simulation; requirements verdicts; event replay; persistent projects and run history; JSON import/export. Schema1/2 application models use one directed linear pipeline: exactly one client, one or more services, one terminal database. Schema3 additionally permits routers that select one branch, converging paths and multiple terminal stores in an acyclic graph. It does not implement parallel fan-out or synchronization. UI can freely edit but validation rejects unsupported topology with useful errors. Components model queues and bounded processing capacity. Stores can deduplicate writes by logical request ID (scoped to each store in schema3). Responses traverse the selected reverse path without extra processing. Drop-first-response applies once per logical request after a successful database write, exercising timeout/retry. A timed-out attempt may continue processing; no implicit cancellation. Schema2 requires infrastructure; schema3 permits it optionally. It binds nodes to geographic hosts and physical links, with shared CPU/bandwidth, static memory admission, power/network outages and estimated host energy; see CONTINUUM_CONTRACT.md.

## Base schema1 JSON model (camelCase)

The additive schema2 and schema3 fields are specified in CONTINUUM_CONTRACT.md and WORKFLOW_CONTRACT.md. They use optional fields so legacy serialized projects retain their shape.

Project {schemaVersion:1,id:string,name:string,description:string,nodes:Node[],edges:Edge[],requirements:Requirement[],scenario:Scenario}
Node {id:string,type:'client'|'service'|'database',name:string,position:{x:number,y:number},config:{processingMs:number,capacity:number,deduplicate:boolean}}
Edge {id:string,source:string,target:string,latencyMs:number}
Requirement {id:string,text:string,rule:'no_duplicates'|'all_complete'|'latency_p95',targetMs:number}
Scenario {requestCount:number,intervalMs:number,timeoutMs:number,maxRetries:number,dropFirstResponse:boolean,jitterMs:number,seed:number}

Limits: max 30 nodes, 30 edges, 50 requirements, 1..200 requests, intervalMs 1..60000, timeoutMs 1..60000, maxRetries 0..5, jitterMs 0..1000, seed integer 0..2147483647; processingMs and latencyMs 0..60000; capacity 1..100; targetMs 0..3600000. Max 100000 trace events and bounded simulation time 3600000ms; fail with explicit error instead of silent truncation. All numbers finite; integer where applicable. Stable unique IDs; reject dangling references and invalid strings. UI units milliseconds. Use int64 ticks and explicit tie-break event sequence. No wall-clock sleeping or unseeded randomness in simulation.

ValidationIssue {path:string,message:string}; Validation {valid:boolean,issues:ValidationIssue[]}
TraceEvent {sequence:number,timeMs:number,kind:string,nodeId:string,requestId:string,attempt:number,message:string}
Verdict {requirementId:string,text:string,rule:string,passed:boolean,actual:number,expected:number,message:string}
Metrics {requests:number,completed:number,failed:number,attempts:number,writes:number,duplicateWrites:number,deduplicated:number,p95LatencyMs:number,averageLatencyMs:number,durationMs:number}
Run {id:string,projectId:string,createdAt:string,modelHash:string,engineVersion:string,seed:number,metrics:Metrics,verdicts:Verdict[],events:TraceEvent[],project:Project}
Arrays always [] not null. Failures never fabricated as passing. No-duplicates based on writes beyond one per request; all_complete based on client-received responses. Latency measured first send to first received response for completed requests; latency verdict also fails if requests did not complete.

## Go package API

internal/model defines structs above and `func Validate(Project) []ValidationIssue`, `func ExampleProject() Project`.
internal/simulation exposes `func Run(context.Context, model.Project) (model.Run, error)`; sets engineVersion, metrics, verdicts, events, seed, project; server fills id/createdAt/modelHash if not already set. Bad model returns descriptive error. Avoid external simulation deps.

## HTTP API (same-origin)

GET /api/health -> {status:'ok',version:'0.7.0'}
GET /api/example -> Project
GET /api/example?template=continuum -> schema v2 Project; template=retry is equivalent to the legacy default; unknown template returns 400
GET /api/example?template=branching -> schema3 Project with embedded component locks
GET /api/components -> installed immutable component records
GET /api/components/examples -> bundled portable package locks
POST /api/components body schema1 ComponentPackage -> installed record (201 new, 200 identical reinstall, 409 conflict/capacity, 422 invalid)
POST /api/validate body Project -> Validation (200 even invalid)
POST /api/validate?draft=true body Project -> Validation of fields/references only, permitting incomplete topology for JSON import
GET /api/projects -> Project[]
POST /api/projects body Project -> Project (upsert; generate id if empty; permit structurally valid drafts with unsupported topology, require bounded well-formed fields)
GET /api/projects/{id} -> Project
DELETE /api/projects/{id} -> 204
POST /api/runs body {project:Project} -> Run (201); simulate synchronously bounded, persist snapshot. 422 for invalid model, 400 for malformed JSON, 500 for internal errors. Does not silently save project.
GET /api/runs?projectId=... -> Run[] (can omit events/project in summaries; frontend fetch detail)
GET /api/runs/{id} -> Run
Errors {error:string,issues?:ValidationIssue[]}. Unknown API paths JSON 404. Body limit 2MiB. Reject unknown model fields via strict JSON decode; exactly one JSON value.

Serve built web/dist from Go with SPA fallback; API paths must not become HTML. Go server localhost default 127.0.0.1:8080 configurable STUDIO_ADDR, DATABASE_URL, STUDIO_WEB_DIR. DATABASE_URL is required. PostgreSQL stores projects and immutable runs as JSON, with TIMESTAMPTZ ordering columns. Startup checks the connection and initializes the schema transactionally; GET /api/health returns 503 if the database cannot be reached within two seconds. This is local single-user software with no built-in authentication; it simulates models rather than executing application source code.

## Composite component API

The exact request/response and persistence contract for `/api/components/primitives`, `/composite-examples`, `/prepare`, `/validate-bundle`, `/bundles`, `/bundle`, `/evaluate`, `/evaluations` and `/test` is in [COMPOSITION_CONTRACT.md](COMPOSITION_CONTRACT.md). Relative names here are under `/api/components`. The common component list contains both schema1 resource profiles and schema2 pure composites; callers must respect capabilities. Application projects reject schema2 package locks, including unused locks.

## Demo defaults

Client at (60,160) -> Order service at (360,160) -> Ledger at (660,160). Edge latencies 10ms each. Service processingMs 15 capacity 2. Database processingMs 5 capacity 1 deduplicate true. Scenario requestCount 12 intervalMs 40 timeoutMs 100 maxRetries 2 dropFirstResponse true jitterMs 0 seed 42. Requirements: no duplicates, all complete, p95<=300ms. Demo must pass with dedup enabled, fail no_duplicates with dedup disabled, fail completion if retries disabled.

## Frontend

Use actual Go API only, no mock simulation. Vite proxy /api to http://127.0.0.1:8080. Professional dark ink/navy canvas, cyan/teal accents, light readable panels or consistent dark style. Desktop split editor with component palette/project explorer, main Vue Flow canvas, property inspector, bottom results/event timeline. Plan/Design/Simulate views or tabs. Responsive mobile stacks. Functional controls only. Save status/error messages; save project; import/export JSON; select node/edge edit; add/remove/connect nodes; scenario fields; requirement CRUD; validate/run; run list select; playback/step/reset saved trace with node highlighting explicitly labelled replay. Provide request status and metrics. Include demo reset with confirmation if dirty. API errors visible. Handle offline backend cleanly.

## Structured data increment

The data-contract catalog, complete-fixture validation, immutable validation history, directional compatibility and structured component APIs are specified in [DATA_CONTRACT.md](DATA_CONTRACT.md). Data definitions are separate immutable artifacts; component schema3 embeds their exact locks. Existing scalar endpoints/examples remain available. `/components/structured-primitives` exposes all16 operations while `/components/primitives` retains the11 scalar operations for older clients.

## Owned-state scenarios (v0.6)

The app adds an independent `/api/state/*` machine catalog and deterministic scenario API. [STATE_CONTRACT.md](STATE_CONTRACT.md) defines strict typed bindings, per-instance ownership, command receipts, revision checks, explicit effect intents, commit faults, restart and immutable evidence. Machine and scenario schemas are 1; pure/data evaluation remains 0.5.0 and timed simulation remains 0.3.0. Scenario state is initialized per run; PostgreSQL stores complete experiment snapshots, not live modeled application sessions.

## Typed delivery scenarios (v0.7)

The independent `/api/delivery/*` model catalog and scenario API compose exact state-machine versions through typed effect-to-command routes. [DELIVERY_CONTRACT.md](DELIVERY_CONTRACT.md) defines immutable dispatch envelopes, explicit delivery attempts, recipient receipt replay, separate acknowledgements and business acceptance, downstream message references and aggregate limits. Model/scenario schemas are 1 and delivery engine is 0.7.0. State engine remains 0.6.0 with unchanged complete legacy results. PostgreSQL retains experimental evidence; no real broker or external message delivery occurs.
