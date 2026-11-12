# Typed task workflows — v0.8

This contract defines the finite workflow experiment introduced in v0.8. It is
separate from the workload-routing profiles in [WORKFLOW_CONTRACT.md](WORKFLOW_CONTRACT.md).
The portable Go/JSON types are in `internal/model/workflow.go`; the runner is
`internal/workflow`. Existing data, pure behavior, state and delivery artifact
schemas and engine result formats remain unchanged.

## Scope and execution boundary

A workflow binds public typed inputs to initialized variables, exact pure
components, named state owners and explicit human/adapter tasks. The runner
automatically follows nodes until one task is waiting, an end is reached, or an
ordinary pure decision fails. There is at most one active task. A scenario
provides an ordered list of explicit responses or signals. The runner drains
automatic nodes between events.

The interactive workbench continues an **experiment** by appending an event and
replaying the immutable definition and scenario from their initialization. It
does not resume a deployed clinical process. Each replay creates fresh owned
state sessions; within that replay, commands execute incrementally against
those sessions, using the same transition kernel as the State workspace.

Tasks are visible requests for input. Opening a task does not establish that its
work succeeded. Adapter tasks perform no network, registry, finance or other
external operation. The response is explicit synthetic evidence supplied by the
scenario or operator. Human responses are not authenticated professional
decisions. An application may describe a clinical handoff, but this engine does
not diagnose, choose treatment, enforce clinical authorization, or deploy a
hospital information system.

This slice has no parallel branches, child-workflow invocation, external I/O,
clock, timeout scheduler, transport fault injection, process restart event,
cross-run owner sharing, or hardware/network/power simulation. Those concerns
remain separate contracts. Pure `select` remains eager; workflow branches
control which subsequent nodes execute.

## Portable package and exact dependencies

`WorkflowLock` is `{ "workflow": WorkflowDefinition, "digest": SHA256 }`.
The digest hashes Go's typed JSON encoding of the complete validated definition.
Definition field order is the declaration order in `workflow.go`; map keys are
sorted by Go JSON encoding. Array ordering, metadata, positions and exact
dependency locks are therefore significant. Preparing a lock never silently
upgrades a dependency.

The definition has schema version 1, a namespaced ID, exact three-part version,
name, description, author, license, local data locks, decision bundles, machine
locks, named owners, public input/output ports, initialized variables, entry node,
nodes and signals. Collections must be present and nonnull; empty collections
are valid when the feature is unused. The graph has at least one node.

Decision references identify exact embedded `ComponentBundle` roots. Owners use
exact `{machineId, version, digest}` references into embedded State machine
locks. Every embedded closure must be independently valid. Conflicting digests
for one component or data ID/version across the whole workflow are rejected;
one closure cannot repair another closure's missing dependency. The studio
inspector can descend from a workflow node to its exact decision or owner
handler and the existing pure primitive hierarchy. A whole workflow cannot yet
be invoked as a child node.

Values use the existing exact `TypedValue` types: Boolean, browser-safe integer,
string, nominal required-field record, or ordered collection of nominal records.
There is no coercion, nullable field, implicit default, nested record schema or
optional field. Nominal identity includes the exact data contract digest.

## Bindings and variables

Every `WorkflowBinding` has exactly one source: `input`, `variable` or `literal`.
Variables have declared types and valid initial values. Public inputs and
variables occupy separate namespaces. Assignments are simultaneous: all sources
are read before any destination changes.

| Location | `input` reads | Assignment direction |
| --- | --- | --- |
| Node inputs, task request, end outputs | Root scenario inputs | Destination port → binding |
| Owner initial overrides | Root scenario inputs | Owned slot → binding; only input or literal allowed |
| Task `assign` | Not a binding map | Workflow variable → response port name |
| Decision `assign` | Not a binding map | Workflow variable → decision output port name |
| Command `assign` | Not a binding map | Workflow variable → public receipt output port name |
| Signal `set` | Current signal's explicit values | Workflow variable → binding |

In signal `set`, `variable` reads the pre-signal context. For example, setting
`left` from `right` and `right` from `left` swaps the values. Owner overrides are
partial; omitted owned slots keep the machine's declared initial value. Owned
state cannot otherwise be read or written directly by a workflow binding.

## Nodes and control flow

Every node has ID, name, kind, finite position and exactly one matching body.
Each non-end node declares exactly one `next` or
`branch: {variable, then, else}`. The branch variable must be Boolean and is read
after that node's assignments. There is no standalone branch node. Every edge
must name a node. Every node must be reachable from entry or a signal target and
have a structural route to an end; signal targets count as possible exits from
waiting task nodes. Static reachability does not prove a guard will ever allow
an exit. Runtime limits still bound cycles.

| Kind | Behavior |
| --- | --- |
| `task` | Freeze a typed request snapshot, expose the declared response ports and wait. Task kind is `human` or `adapter`. A valid response assigns variables, then takes the continuation. |
| `decision` | Resolve typed inputs, evaluate the exact pure bundle, assign named outputs and take the continuation. An ordinary evaluation error records a `failed` step and terminal workflow status. |
| `command` | Invoke the named command on one named owner, assign public receipt outputs and acceptance, then route. The command's pure reducer cannot perform an external effect. |
| `end` | Resolve all declared public outputs and finish with the explicit outcome identifier. An end has no continuation. |

A command reads the owner's current revision immediately before invocation.
Its command ID is `wf_command1`, `wf_command2`, and so on, globally per workflow
owner-command invocation. No automatic retry or state transport fault is added.
A normal accepted or denied receipt supplies `assign` values and sets the
required Boolean `acceptedVariable`. That variable cannot also be an `assign`
destination. A denied receipt follows the ordinary continuation; branch on
acceptance when that matters. If an ordinary command error produces no receipt,
acceptance becomes false, other output destinations retain their previous
values, and the workflow takes the required `errorNext` edge. State effect
intents remain evidence; the workflow runner does not deliver them.

## Tasks, signals and event receipts

Task IDs are `task1`, `task2`, and so on, per activation across the experiment.
Re-entering the same node creates a new task ID. A task retains its frozen
request, response schema, status (`pending`, `completed`, `cancelled`), supplied
response values and `completedBy` event ID. `completedBy` is an event identifier,
including for cancellation; it is not an authenticated actor.

A response event is `{id, kind:"response", taskId, values}`. It is accepted only
for the current pending task and an exact response map with every declared port
and no extras. A noncurrent, cancelled, already completed or never issued task
ID is an ordinary rejected event. Well-formed typed values with incorrect port
names or types are also rejected events; malformed values or envelope fields
are invalid scenario input.

A signal event is `{id, kind:"signal", signal, values}`. A signal definition has
name, typed inputs, optional Boolean variable `when`, simultaneous `set` bindings
and target. While a task is waiting, a declared signal with matching input values
and a true guard cancels the active task, applies its assignments and jumps to
the target. Empty `when` is unconditional. Unknown signals, mismatched values
and false guards produce rejected events. Set a guard variable false to prevent
repeated escalation when desired. Signals cannot reopen completed or failed
workflows. Every response and signal is ordered by its position in the scenario;
there is no hidden emergency priority or mid-decision preemption.

The event request fingerprint is SHA-256 of the typed JSON struct below, in this
field order, with empty `taskId` or `signal` still included:

```json
{"kind":"response","taskId":"task1","signal":"","values":{}}
```

The fingerprint excludes event ID and event index. Every first-time event ID
stores one receipt, whether accepted or rejected, including terminal events,
false signal guards and stale task responses. The receipt stores its request
hash, accepted/rejected status, message, historical next task ID and outcome.

| Later use of an event ID | Event result | Mutation |
| --- | --- | --- |
| Same canonical request | `replayed`, with the original receipt | None; no new receipt or progress |
| Changed kind, task, signal or values | `id_conflict`, no receipt | None; the original receipt remains |

First-time event results use `accepted` or `rejected`. Event result
`beforeTaskId`/`afterTaskId` describe the current experiment at that event; a
replayed receipt's `nextTaskId` remains its historical value. Late routine
responses after a signal therefore cannot recreate a cancelled task.

## Results and independent assertions

The result includes engine version `0.8.0`, workflow/scenario hashes, final
status, outcome, current node, variables, public outputs, current task when
waiting, task history, owned snapshots, event receipts, ordered event results,
automatic node steps, completed operation/invocation counts and independent
assertion issues. Final status is `waiting`, `completed` or `failed`. Each
decision or command step retains the existing nested pure/state result so its
reasoning can be inspected. Returned snapshots do not alias the caller's
definition, scenario or other independently returned snapshots.

Scenario schema version 1 has a name, exact public inputs, zero to 64 events and
optional `expected`. Omitted or null `expected` means unverified: `verified` and
`passed` are false. A supplied expectation requires explicit `status`, `outcome`,
`activeTaskNode`, `tasks`, `completedTasks`, `cancelledTasks` and `eventReceipts`.
Empty outcome/active-task strings and zero counts are valid explicit assertions;
missing or null fields do not silently become zero or empty assertions. Counts
must be nonnegative browser-safe integers. `variables` is a nonnull map of named
partial assertions. `owners` is a nonnull subset array of complete existing
State expectations (owned state, revision, receipt and effect counts).

Expected status, outcome, active task node and all four counts are compared
exactly. Variable assertions compare exact typed values. Expectations are never
derived from the actual result. A structurally valid wrong expectation produces
recorded issues, `verified:true` and `passed:false`; it does not rewrite the run.

## Resource admission

| Resource | Bound |
| --- | --- |
| Definition canonical JSON | 1 MiB |
| Definition nodes / variables | 96 / 48 |
| Context value map | 256 KiB |
| Embedded machine locks / named owners | 8 / 8 |
| Decision bundles / distinct pure locks across closure | 32 / 64 |
| Local data locks / distinct data locks across closure | 8 / 32 |
| Public input/output ports | 16 each |
| Task request/response ports / signal inputs | 8 each |
| Signals | 8 |
| Task request, response or signal value map | 64 KiB |
| Individual structured value / collection rows | Existing 64 KiB / 128 |
| Scenario events / canonical JSON | 64 / 512 KiB |
| Automatic node visits, including task openings and ends | 256 |
| Task activations / first-time event receipts / owner-command calls | 64 each |
| Owned command receipts / effect intents, across owners | 64 / 128 |
| Reserved expanded pure operations / invocations, across pure and owned commands | 4096 each |
| Result canonical JSON | 4 MiB |

Existing pure/state limits also apply. Failed evaluations retain their expanded
work reservation, so errors cannot reset the shared work budget; public counters
report completed decision results. Every event and automatic transition checks
cancellation. Limit, cancellation and validation errors return no partial result.

Evidence is admitted incrementally before repeated value copies and state
commits. The outer runner uses scalar projections from State sessions, including
receipt output sizes and Boolean routing metadata, without exposing mutable
owned values. A bounded envelope for the current event is conservatively
reserved during automatic routing and replaced by its exact receipt afterward.
Thus admission may reject a near-limit run before its eventual final encoding
reaches 4 MiB. No journal is evicted or silently truncated to make a run fit.

## API and CLI

The API prefix is `/api/workflow`. Preparing, checking or importing a package
does not create a saved run. Imports store the complete immutable lock and do
not install or upgrade dependencies implicitly.

| Endpoint | Input or result |
| --- | --- |
| `GET /examples` | Complete workflow/scenario examples |
| `POST /prepare` | Raw definition → canonical exact lock |
| `POST /validate-lock` | Lock → validation issues and canonical lock when valid |
| `GET /packages` | Installed workflow catalog, sorted by ID/version |
| `POST /packages` | Exact lock → 201 new, 200 identical, 409 conflicting identity or full catalog |
| `GET /package?id=&version=&digest=` | One exact installed package; no version fallback |
| `POST /check` | `{workflow: WorkflowLock, scenario}` → `{scenario,result}` without persistence |
| `POST /runs` | Same input → saved `{id,createdAt,workflow,scenario,result}` |
| `GET /runs` | Newest 50 run summaries |
| `GET /runs/{id}` | Saved immutable complete experiment snapshot |

Requests have a 2 MiB body bound and strict JSON decoding, including unknown
fields and trailing documents. Semantic validation and outer execution errors
return 422; busy admission returns 429 and cancellation/deadline returns 408.
There are two compute slots and a 15-second admission window covering semantic
validation, execution and saving. A modeled waiting/failed result or failed
independent expectation is still a successful check (200) or saved run (201).
The workflow catalog has its own 200-artifact limit and immutable identity rules.

```sh
go run ./cmd/workflowcheck examples/reception/hospital-reception.workflow.json
go run ./cmd/workflowcheck --lock examples/reception/hospital-reception.lock.json
go run ./cmd/workflowcheck --run examples/reception/scenarios/routine-existing.run.json
```

The default CLI action prepares a raw definition. `--lock` validates and emits
an exact lock; `--run` consumes a complete `{workflow,scenario}` experiment and
emits its result. Use `-` to read standard input. Invalid input, execution limits
or a verified failed expectation exit nonzero. Waiting/unverified runs exit
successfully but remain explicitly labeled in the result.

## Replication and further layers

The reception package uses this generic engine and exact reusable pure/state
parts. Its typed fixture responses make registry, affiliation, clinical release,
finance and handoff boundaries explicit. See
[HOSPITAL_RECEPTION_DESIGN.md](HOSPITAL_RECEPTION_DESIGN.md) for the domain design
and [HOSPITAL_RECEPTION_REPLICATION.md](HOSPITAL_RECEPTION_REPLICATION.md) for
authoring/running the implemented package. A different domain can provide its own graph, tasks, exact components
and state owners without changing the workflow engine. Further hierarchy,
concurrent cases, real adapters and continuum placement require their own
explicit contracts and tests.
