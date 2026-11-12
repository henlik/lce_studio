# Owned state and transition contract — v0.6

This is the implementation contract for the next increment. Vue/Go/PostgreSQL remain the stack. State machines are separate portable artifacts; existing pure package schemas, exact digests, primitive semantics, data contracts and timed simulation engine 0.3.0 remain unchanged. The app and new state engine use 0.6.0; the pure/data engines keep 0.5.0. No new dependencies.

## Meaning and boundary

Each scenario initializes independent instances of one locked machine and executes an explicit ordered sequence. An instance owns its state, revision, command receipts and effect intents. Commands use existing exact pure component bundles as decision functions. No reducer can mutate another instance or perform external I/O.

Durable state means surviving an explicit simulated restart within this scenario. Every new run starts from its declared initialization. PostgreSQL saves the machine and complete immutable experiment evidence; it does not execute the modeled business transactions. Effects are committed intentions only: no delivery, acknowledgement, external exactly-once claim, clock, asynchronous messaging, deployment or resource timing.

## Model types and typed JSON order

All JSON field names below are lower camel case as shown. Go exported names capitalize these names; ID is spelled ID, InstanceID, CommandID. Struct field order is exactly the order listed. Required collections/maps are nonnull unless an exception is stated. Optional members use omitempty.

```
StateSlot { name:string, type:string, recordRef?:DataTypeRef,
            durability:string, initial:TypedValue }
StateEffect { name:string, payload:BehaviorPort[] }
StateBinding { state?:string, input?:string, literal?:TypedValue }
StateEffectBinding { effect:string, whenOutput?:string, payload:map[string]string }
StateCommand { name:string, inputs:BehaviorPort[], outputs:BehaviorPort[],
               decision:ComponentBundle, bindings:map[string]StateBinding,
               acceptedOutput:string, writes:map[string]string,
               results:map[string]string, effects:StateEffectBinding[] }
StateMachine { schemaVersion:int, id:string, version:string, name:string,
               description:string, author:string, license:string,
               dataContracts:DataContractLock[], state:StateSlot[],
               effects:StateEffect[], commands:StateCommand[] }
StateMachineLock { machine:StateMachine, digest:string }
StateInstance { id:string, name:string, initial:map[string]TypedValue }
StateStep { kind:string, instanceId:string, commandId?:string, command?:string,
            expectedRevision?:*int64, inputs:map[string]TypedValue, fault?:string }
StateExpectation { instanceId:string, state:map[string]TypedValue,
                   revision:int64, receipts:int, effects:int }
StateScenario { schemaVersion:int, name:string, instances:StateInstance[],
                steps:StateStep[], expected:StateExpectation[] }
StateExample { machine:StateMachineLock, scenario:StateScenario }
```

Machine/scenario schemaVersion is exactly 1. Machine metadata uses the same validation as component metadata (namespaced safe ID, exact three-part decimal version, text bounds). Local slot/command/effect/port/instance/command IDs use the existing letter-first 1–64 character behavior name rule. Instance name and scenario name are required valid text <=200 bytes. Every slot has exact boolean/integer/string/record/collection type and valid initial value; durability is exactly `durable` or `volatile`.

State bindings have exactly one source: named owned slot, named command input, or literal. Every root decision input has exactly one binding with matching nominal type. `acceptedOutput` names a Boolean root output. `writes` maps owned slot names to root decision output names; unwritten slots retain their values. `results` maps every declared public output to a same-type decision output. An effect binding names a declared effect at most once per command. Its payload maps every declared effect payload port to a same-type decision output; optional `whenOutput` names a Boolean output. Empty whenOutput means true. Only accepted commands may write or emit effects. Reusing one output in several destinations is allowed. Unused decision outputs/command inputs/state slots and valid unused machine data locks are allowed.

Each decision is a complete independently valid ComponentBundle. Data types resolve from machine dataContracts plus all command decision bundle data contracts. Component package definitions validate using their existing per-bundle rules, never machine-only locks. Local machine dataContracts must have unique ID/version pairs. Repeated exact package/data locks across decisions are allowed; conflicting ID/version digests anywhere in the machine fail. Changing an embedded definition requires explicit version/lock updates. Machine installation does not install any dependencies in other catalogs.

`StateMachineDigest` validates then SHA-256 hashes Go json.Marshal(machine), like the existing typed JSON contracts. It is not RFC8785. Full immutable contents, including decision bundles and layout, participate. Unknown HTTP/CLI fields fail. Existing optional-pointer JSON null-to-absent normalization remains; required values/collections must still be present and valid.

## Scenario validation

An instance's `initial` is a nonnull map of zero or more declared slot overrides; defaults fill omitted slots. Unknown slots and wrong nominal types fail. Restart resets volatile slots to this effective per-instance initial value, including overrides. State revision initially equals zero; receipts and effects initially empty.

A `command` step requires commandId, a declared command, a nonnegative safe-integer expectedRevision pointer (zero is valid), and an exact nonnull typed inputs map. Fault is absent/empty, `before_commit`, or `after_commit_before_reply`. A `restart` step permits only kind and instanceId; command fields, expectedRevision and fault must be absent/zero and inputs nil (omitted or JSON null). Canonical restart JSON includes inputs:null.

Expected is a nonnull array with at most one entry per instance. Each entry supplies every owned state slot plus exact final revision, receipt count and effect count. Counts/revisions are explicit nonnull nonnegative browser-safe integers; JSON expectations must supply revision, receipts and effects even when they are zero. Empty expected means unverified, not passing. Failed expectations are normal recordable outcomes. There is no implicit generation of expected values from the executed model.

## Command/restart protocol

For each command, check in this order:

1. Existing receipt by (instanceId, commandId). A matching request replays its original receipt without evaluating, changing current state/revision, or emitting effects. A mismatch produces id_conflict. The request fingerprint is SHA-256 of typed JSON struct `{command,expectedRevision,inputs}` in that order; it excludes scenario position, fault, labels and instanceId (the receipt is already instance-scoped).
2. Compare expectedRevision with current instance revision. Mismatch produces revision_conflict, no evaluation or receipt.
3. Evaluate the exact pure decision against a copied immutable input snapshot. Ordinary evaluation errors (for example overflow) produce evaluation_error without mutation/receipt, allowing later steps. Cancellation and behavior.ErrLimit abort the entire run, returning no partial successful evidence.
4. Stage all typed results/writes/effect payloads. False acceptedOutput stages a denied receipt and results but discards every proposed write and effect. True stages writes, incremented revision (even if values are equal) and enabled effect intents.
5. before_commit aborts the entire staged transaction: rolled_back, no state/revision/receipt/effect change. It does not itself restart/reset volatile state. An explicit restart is a separate step.
6. Otherwise commit atomically: publish all staged owned state, revision, durable receipt and effect intents. Status is committed or denied. Denial keeps revision/state unchanged but commits its receipt. after_commit_before_reply sets replyLost=true while preserving all committed facts. An explicit restart may then test persistence.

Matching replays bypass injected commit faults: no new commit occurs. Before-commit aborts do not reserve IDs. A cached receipt always contains historical outputs and its original revision/effect IDs; replay must never restore old state or recompute the reply using current values. Receipts are never silently evicted. A restart preserves durable slots, revision, receipts and effects and resets only volatile slots; other instances are unchanged.

Effect IDs are lowercase SHA-256 of Go JSON string array `[instanceId,commandId,effectName]`. Effect intents retain the typed payload and originating command/revision. Iterate declared command effects in their listed order. They are pending intentions with no delivery status.

## Engine result

Package `internal/state` exports Version="0.6.0" and Run(ctx, StateMachineLock, StateScenario) (Result,error). Exported engine structs follow this order; all result arrays/maps are nonnull. Result deterministic content has no wall-clock timestamps or random IDs.

```
Receipt { commandId:string, requestHash:string, command:string,
          accepted:bool, revision:int64, outputs:map[string]TypedValue, effectIds:string[] }
EffectIntent { id:string, instanceId:string, commandId:string,
               effect:string, revision:int64, payload:map[string]TypedValue }
InstanceResult { id:string, revision:int64, state:map[string]TypedValue,
                 receipts:Receipt[], effects:EffectIntent[] }
StepResult { index:int, kind:string, instanceId:string, commandId?:string,
             status:string, replyLost:bool, faultApplied:bool, message:string,
             beforeRevision:int64, afterRevision:int64,
             before:map[string]TypedValue, after:map[string]TypedValue,
             receipt?:*Receipt, decision?:*behavior.Result }
ExpectationResult { instanceId:string, passed:bool, issues:ValidationIssue[] }
Result { engineVersion:string, machineHash:string, scenarioHash:string,
         operations:int, invocations:int, instances:InstanceResult[], steps:StepResult[],
         verified:bool, passed:bool, expectations:ExpectationResult[] }
```

Step index is zero-based. Status is one of committed, denied, replayed, id_conflict, revision_conflict, rolled_back, evaluation_error, restarted. Full decision trace exists only for evaluations that returned a result; failed pure evaluations have message only. Receipt on first successful commit/denial or replay; no receipt for other statuses. ReplyLost only for first-time after-commit injection. faultApplied true only at an actually reached injection point. Final instances follow scenario instance order; receipts/effects follow first-commit order. Before/after are current owned state, not cached historical outputs. Deep copies prevent aliasing across inputs, snapshots, receipts, effects and evidence.

operations/invocations sum completed pure decision traces, including proposals later rolled back; ordinary failed evaluations expose no partial work counts. To bound failed work as well, the runner conservatively reserves the statically expanded maximum operations/calls before every fresh decision. These internal reservations are not reported as measured execution counts.

scenarioHash is SHA-256(json.Marshal(scenario)); machineHash equals lock.digest. verified = expected nonempty, passed = verified AND all expectations pass. Ordinary modeled failures do not themselves force expectation failure; the expected final state may intentionally describe them.

## Bounds

- Machine: 1–8 slots, 1–8 commands, 0–8 effect declarations, <=8 local data locks, <=512 KiB canonical typed JSON. All bounded arrays checked before iteration/encoding.
- Each command: <=8 public inputs, <=8 public outputs, <=16 decision bindings, <=8 writes, <=8 results, <=8 effect bindings; each effect payload <=8 ports. Existing component-bundle bounds apply; <=32 distinct data locks and <=64 distinct component locks across machine.
- State/command-input/public-result/effect-payload maps: <=64 KiB aggregate encoded typed values after bounded shape validation; existing per-value 64 KiB, 128-record collection and scalar bounds apply. No copying or full unbounded encoding before checking shape/value limits.
- Scenario: 1–8 instances, 1–64 steps, <=8 expectations, <=512 KiB canonical JSON. <=64 receipts and <=128 effect intents across the run; reject further first commits at capacity, never evict.
- Existing per-decision <=256 primitive operations/calls/depth8; global <=4096 operations and <=4096 invocations. Result <=2 MiB canonical JSON, bounded incrementally before retaining expanded trace/snapshots. Context cancellation checked between stages. Any outer budget/cancellation failure aborts entire run with no persisted partial result.
- HTTP/CLI body <=2 MiB. App's existing 2-computation admission and 15s deadline include state checks/runs.

## Model exports

```
ValidateStateMachine(StateMachine) []ValidationIssue
StateMachineDigest(StateMachine) (string,error)
ValidateStateMachineLock(StateMachineLock) []ValidationIssue
StateMachineDataContracts(StateMachine) ([]DataContractLock,[]ValidationIssue)
ValidateStateScenario(StateMachineLock,StateScenario) []ValidationIssue
ValidateStateValues(map[string]TypedValue,[]DataContractLock) []ValidationIssue
ExampleStateScenarios() []StateExample
```

Use existing structured validation helpers. Model validation must independently validate actual inputs/initial state/expectations, and enforce nominal mapping types without executing decisions. Empty issue slices are nonnull.

## HTTP/storage

Relative to /api; strict same-origin bounded JSON as before. Invalid HTTP JSON/unknown/trailing fields ->400, body >2MiB ->413, semantic locks/scenarios/outer execution limits ->422; busy429/cancellation408. Expected scenario conflicts/denials/rollback/ordinary execution errors and failed expectations return complete 200/201 evidence.

- GET /state/examples -> StateExample[2].
- POST /state/prepare raw StateMachine ->200 canonical StateMachineLock;422 issues.
- POST /state/validate-lock raw lock ->200 {valid,issues,machine?:StateMachineLock}, canonical lock only if valid.
- GET /state/machines -> records sorted ID/version, {machine,digest,installedAt}.
- POST /state/machines raw lock ->201 new/200 identical record;409 conflict/full.
- GET /state/machine?id=&version=&digest= -> exact record;404 missing/422 invalid ref.
- POST /state/check {machine:StateMachineLock,scenario:StateScenario} ->200 {scenario,result}, no persistence; raw scenario must go to server before browser numeric parsing.
- POST /state/runs same ->201 {id,createdAt,machine,scenario,result}; saves complete snapshot only.
- GET /state/runs -> newest50 summaries {id,createdAt,machineId,machineVersion,stepCount,verified,passed}.
- GET /state/runs/{id} -> full record.

Independent200-version machine catalog uses advisory lock (1279477075,4), no delete/mutate endpoints. Same reinstall preserves timestamp. Immutable state_runs history has no hidden live application state. HTTP run submission itself is not idempotent; scenario command receipts are a different protocol.

## Examples and UI

Two reference scenarios use ONE exact generic resource-reservation machine and pure reducer/data locks. Pharmacy stock and school-library copies differ in labels and instance values, not engine semantics. Required state includes tenant/resource identifiers, durable available quantity, and a volatile successful-command counter. One reserve command checks matching IDs, positive quantity and sufficient availability; use selected safe delta before eager subtraction. On acceptance write remaining quantity/increment volatile count and emit one typed reservation notice. Denials return explicit outputs and no business-state change/effect. Root scenario expectations are handwritten and checked independently.

The State workspace must support exact definition identity, slots/durability/initial values, handler bundle imports/catalog selection and type-safe bindings, effect declarations, ordered command/restart steps with failure injection, per-instance overrides, independent expected final state, recorded state/receipt/effect/decision trace and history. Provide a compact ownership/handler diagram, not only raw JSON. Graphical authoring should use existing TypedValueEditor and nominal selectors. Preserve drafts across navigation; pending JSON and newer graphical changes require explicit discard confirmation. Results must show stale status after any machine/scenario/pending-JSON change. Show historical receipt outputs separately from current state after replay.

Portable CLI (root-owned): `go run ./cmd/statecheck machine.json` -> lock; `--lock` validates lock; `--run` consumes StateExample and prints result, nonzero only on run error or verified failed expectations (empty expected succeeds as unverified and is labeled). No database or network.
