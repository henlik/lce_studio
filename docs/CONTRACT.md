# Software Studio v0.1 integration contract

This document defines the shared contract between the Vue 3 + TypeScript editor and the Go API/simulator. See README.md for setup and docs/SIMULATION.md for execution semantics.

## Scope

Single-user local-first engineering studio. Editable requirements, topology, node/link properties and scenario; deterministic discrete-event simulation; requirements verdicts; event replay; persistent projects and run history; JSON import/export. This version models a single directed linear pipeline: exactly one client, one or more services, exactly one terminal database. Every node participates in the path. No branching/cycles. UI can freely edit but validator rejects unsupported topology with useful errors. Components model queues and bounded processing capacity. Data store can deduplicate writes by logical request ID. Responses traverse the reverse path using link latency only (no extra service processing). Drop-first-response applies once per logical request after a successful database write, exercising timeout/retry. A timed-out attempt may continue processing; no implicit cancellation.

## JSON model (camelCase)

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

GET /api/health -> {status:'ok',version:'0.1.0'}
GET /api/example -> Project
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

Serve built web/dist from Go with SPA fallback; API paths must not become HTML. Go server localhost default 127.0.0.1:8080 configurable STUDIO_ADDR, STUDIO_DATA, STUDIO_WEB_DIR. SQLite stores projects and immutable runs. This is local single-user software with no built-in authentication; it simulates models rather than executing application source code.

## Demo defaults

Client at (60,160) -> Order service at (360,160) -> Ledger at (660,160). Edge latencies 10ms each. Service processingMs 15 capacity 2. Database processingMs 5 capacity 1 deduplicate true. Scenario requestCount 12 intervalMs 40 timeoutMs 100 maxRetries 2 dropFirstResponse true jitterMs 0 seed 42. Requirements: no duplicates, all complete, p95<=300ms. Demo must pass with dedup enabled, fail no_duplicates with dedup disabled, fail completion if retries disabled.

## Frontend

Use actual Go API only, no mock simulation. Vite proxy /api to http://127.0.0.1:8080. Professional dark ink/navy canvas, cyan/teal accents, light readable panels or consistent dark style. Desktop split editor with component palette/project explorer, main Vue Flow canvas, property inspector, bottom results/event timeline. Plan/Design/Simulate views or tabs. Responsive mobile stacks. Functional controls only. Save status/error messages; save project; import/export JSON; select node/edge edit; add/remove/connect nodes; scenario fields; requirement CRUD; validate/run; run list select; playback/step/reset saved trace with node highlighting explicitly labelled replay. Provide request status and metrics. Include demo reset with confirmation if dirty. API errors visible. Handle offline backend cleanly.
