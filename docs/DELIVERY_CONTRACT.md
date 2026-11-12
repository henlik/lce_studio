# Typed message delivery contract — v0.7

This frozen implementation contract defines a separate Messages workspace in Vue/Go/PostgreSQL, without new dependencies. It composes exact v0.6 owned-state machines through typed effect-to-command routes. App/delivery version becomes 0.7.0; state remains 0.6.0, pure/data remain 0.5.0 and timed application simulation remains 0.3.0. Existing artifact schemas, hashes and complete v0.6 execution results must remain unchanged.

## Meaning and boundary

Commands commit owned state and effect intentions. Dispatch explicitly materializes one intention on one route into an immutable message. Delivery explicitly attempts its frozen recipient command. A recipient receipt acknowledges processing, including a business denial; acknowledgement and business acceptance are separate facts. Transport metadata, messages and acknowledgements survive modeled owner restarts. This is a stated durable-transport assumption, not an implementation of a real broker, distributed transaction or external exactly-once delivery.

There is no clock, hidden retry, automatic routing, queue draining, parallel execution, external I/O or resource accounting. Cycles, self-routes and downstream effects are allowed, but every dispatch/delivery requires another listed step. Future continuum integration must introduce scheduling and outage semantics explicitly.

## Model and JSON types

JSON uses lower camel case; Go fields use ID/InstanceID/RouteID. Struct field order is the listed order. Required arrays/maps are nonnull. Optional fields use omitempty. Position is the existing model.Position and finite within ±1,000,000.

```
DeliveryMachineRef { machineId:string, version:string, digest:string }
DeliveryInstance { id:string, name:string, machine:DeliveryMachineRef, position:Position }
DeliveryRoute { id:string, name:string, sourceInstanceId:string, effect:string,
                targetInstanceId:string, command:string, inputs:map[string]string }
DeliveryModel { schemaVersion:int, id:string, version:string, name:string,
                description:string, author:string, license:string,
                machines:StateMachineLock[], instances:DeliveryInstance[], routes:DeliveryRoute[] }
DeliveryModelLock { model:DeliveryModel, digest:string }
DeliveryInitial { instanceId:string, initial:map[string]TypedValue }
DeliveryStep { kind:string, instanceId?:string, commandId?:string, command?:string,
               expectedRevision?:*int64, inputs:map[string]TypedValue, fault?:string,
               routeId?:string, sourceCommandId?:string, sourceMessage?:string, message?:string }
DeliveryMessageExpectation { name:string, status:string, attempts:int, accepted?:*bool }
DeliveryScenario { schemaVersion:int, name:string, initial:DeliveryInitial[],
                   steps:DeliveryStep[], expected:StateExpectation[],
                   expectedMessages:DeliveryMessageExpectation[] }
DeliveryExample { model:DeliveryModelLock, scenario:DeliveryScenario }
```

Both schema versions are 1. Metadata uses the existing component/machine constraints. Local IDs/names referring to slots, commands, routes, instances, message aliases and command IDs use the existing letter-first 1–64 behavior-name rule. Display names are ≤200 bytes. `msg_` is reserved for generated recipient command IDs in delivery scenarios only; direct command steps may not use that prefix. Existing State scenarios remain unchanged.

Every instance resolves an exact embedded machine lock. Unique local machine ID/version pairs; repeated exact component/data definitions inside different machines are legal, conflicting ID/version digests anywhere in the model are not. Every machine is independently valid; definitions cannot borrow missing dependencies from another machine. Valid unused embedded machines are allowed. All embedded definitions count toward budgets.

A route names a declared source effect and recipient command. `inputs` maps every recipient input port to one source effect payload port of the exact scalar/nominal type. Source payload ports may be unused or mapped more than once within aggregate bounds. No coercion, record-field extraction or arbitrary expression occurs here; use pure components for adaptation. Duplicate route tuples `(sourceInstanceId,effect,targetInstanceId,command)` are rejected even if their IDs/mappings differ. Fanout to distinct targets or commands is explicit and valid. No topology reachability or acyclicity requirement.

`DeliveryModelDigest` validates and SHA256-hashes Go json.Marshal(model), including all locks, metadata and layout. This is typed JSON hashing, not RFC8785. HTTP/CLI strict decoding preserves the prior casing/canonicalization rules.

## Scenario validation and step fields

`initial` contains exactly one entry for every declared model instance, in any order, with nonnull partial slot overrides. Validate actual values and merged effective state before executing; owned defaults fill omitted slots. Final instances follow model instance order. Each StateExpectation names a model instance and contains that machine's complete owned state and explicit nonnull final revision/receipt/effect counts, as in v0.6.

| kind | Required fields | Optional fields | All other fields |
| --- | --- | --- | --- |
| command | instanceId, commandId, command, expectedRevision, nonnull exact inputs | fault: before_commit or after_commit_before_reply | absent/zero |
| restart | instanceId | none | absent/zero; inputs omitted/null |
| dispatch | routeId, message alias, expectedRevision, exactly one of sourceCommandId/sourceMessage | none | absent/zero; inputs omitted/null |
| deliver | message alias | fault: drop_before_receive, before_recipient_commit, after_recipient_commit_before_ack | absent/zero; inputs omitted/null |

Command inputs and revision use existing v0.6 validation; dispatch expectedRevision is explicit, nonnegative and browser-safe. Canonical non-command inputs are null. sourceCommandId names a direct source command ID and may not start msg_. sourceMessage names a previously declared dispatch alias: its frozen recipient command ID identifies the source command, allowing downstream delivery without users typing generated hashes. sourceMessage must refer to a dispatch alias declared earlier in the ordered scenario, and that earlier route's target must equal this route's source. Deliver references must likewise have an earlier declared dispatch alias. These are static declarations; the earlier dispatch can still fail at runtime, producing a missing-message outcome later.

Repeated dispatch aliases are legal only for the same route and same source selector (sourceCommandId/sourceMessage); revision may differ to test a conflict. One source selector/route may be declared under only one alias. Never silently merge or rename aliases. Direct commands need not precede dispatch statically: missing committed effects are modeled outcomes. Empty steps are invalid.

ExpectedMessages has unique names and status absent/pending/acknowledged, nonnegative safe attempts explicitly supplied and nonnull in raw JSON. Absent requires attempts 0 and accepted omitted; pending requires accepted omitted; acknowledged permits an optional accepted Boolean to assert the recipient's business verdict. A named absence assertion need not refer to a declared dispatch. Acknowledged attempts must be ≥1. Expectations are independently supplied; no run result may fill them automatically. verified is true if either expectation list is nonempty; passed is verified AND every supplied assertion passes. Partial assertion lists verify only the named instances/messages.

Pending and acknowledged message expectations must name a declared dispatch alias; an absent expectation may name any valid alias.

## Dispatch protocol

1. Resolve the route and source selector. For sourceMessage, look up the referenced materialized message and use its generated ID as the source command ID. If missing, return missing_source_message. The message need not yet be acknowledged: recipient commit may already have produced its downstream effect despite acknowledgement loss.
2. Find a committed source effect matching that source instance, source command ID and route effect name. No effect (denial, rollback, no execution or ordinary error) returns missing_intent, with no envelope or alias binding.
3. The uniqueness key is `(sourceIntentId,routeId)`. Compute the generated message/recipient-command ID as `msg_` + lowercase unpadded RFC4648 base32(SHA256(json.Marshal([]string{sourceIntentId,routeId}))). The full 256-bit digest is retained; the 56-character result satisfies the existing command-name rule. No fault, attempt number, label or revision participates in identity. Identity is scoped to this independently initialized experiment, not a global cross-run broker namespace.
4. Map and type-check the recipient inputs, including the 64 KiB aggregate bound before copying. Freeze name(alias), ID, route, source identity, target instance, target command, expected revision and typed inputs. The recipient request fingerprint remains the existing v0.6 command/revision/inputs hash.
5. An existing alias/uniqueness key with exactly the same frozen envelope returns dispatch_replayed and changes nothing, including after acknowledgement. Any different binding, revision or mapped contents returns dispatch_conflict and preserves the original. Alias/key collisions never create another envelope. First materialization appends a pending message with attempts 0, no acknowledgement and status dispatched. No recipient execution occurs.

## Delivery and restart protocol

Deliver looks up the frozen message. If not materialized, missing_message; if already acknowledged, already_acknowledged with no attempt, work or applied fault. Otherwise increment attempts once for this explicit deliver step, including drops, conflicts, ordinary errors and rollback.

- drop_before_receive: return dropped; faultApplied=true; recipient is untouched and no state trace/work exists.
- before_recipient_commit: invoke the existing state command protocol with before_commit. A fresh decision rolls back; matching historical receipts bypass this commit fault exactly as v0.6 and can acknowledge normally.
- after_recipient_commit_before_ack: invoke the recipient with no state commit fault. If a committed/denied/replayed receipt exists, suppress its acknowledgement, return acknowledgement_lost, leave message pending, and faultApplied=true. This transport fault also applies to a replayed receipt. No receipt means the loss point was not reached.
- no fault (or before-commit bypassed on replay): invoke the frozen command. A committed, denied or replayed receipt acknowledges the message; status acknowledged, and a deep copy of the original receipt becomes the durable acknowledgement. accepted=false is successful acknowledgement of a denied business request.
- revision_conflict, id_conflict, rolled_back or evaluation_error produces recipient_unacknowledged and leaves message pending; preserve the underlying state status/message/trace. A stale frozen revision remains stale on retries: no automatic rebasing. A new intent represents a new business request.

Recipient execution uses generated commandId=message.ID and the exact frozen command/revision/inputs. Direct command and restart steps use the same v0.6 kernel. Restart resets only that owner's volatile slots; messages, acknowledgement status, attempts, revisions, receipts and effect intents remain. Source intents are never consumed or rewritten by dispatch, retry, acknowledgement or fanout.

Ordinary modeled errors are recordable outcomes. Context cancellation and any shape/work/receipt/effect/message/evidence capacity limit abort the whole run with no persisted partial evidence. Do not rerun growing histories to emulate incremental execution.

## Engine evidence

`internal/delivery` exports Version="0.7.0" and Run(ctx,model.DeliveryModelLock,model.DeliveryScenario)(Result,error). All result arrays/maps nonnull; deterministic result contains no wall clock or random IDs. Struct order:

```
Message { name:string, id:string, routeId:string, sourceIntentId:string,
          sourceInstanceId:string, sourceCommandId:string, effect:string,
          targetInstanceId:string, command:string, expectedRevision:int64,
          inputs:map[string]TypedValue, status:string, attempts:int,
          acknowledgement?:*state.Receipt }
StepResult { index:int, kind:string, status:string, message:string,
             instanceId?:string, messageName?:string, messageId?:string,
             faultApplied:bool, beforeStatus:string, afterStatus:string,
             beforeAttempts:int, afterAttempts:int, state?:*state.StepResult }
MessageExpectationResult { name:string, passed:bool, issues:ValidationIssue[] }
Result { engineVersion:string, modelHash:string, scenarioHash:string,
         operations:int, invocations:int, instances:state.InstanceResult[],
         messages:Message[], steps:StepResult[], verified:bool, passed:bool,
         expectations:state.ExpectationResult[], messageExpectations:MessageExpectationResult[] }
```

Outer step status for direct command/restart is its state status. Dispatch/delivery statuses are those above. `message` is a human-readable diagnostic, not the alias. A state trace is present only when the kernel was called; its index equals the outer zero-based index. Recipient status remains in state.status, separate from outer transport status. beforeStatus/afterStatus use empty string for absent/unrelated messages; attempts use 0 there. Messages follow first-materialization order. Acknowledgement contains historical outputs, not current recipient state. Final states/receipts/effects remain machine-owned snapshots. Count completed operations/invocations once across every kernel call; reserve failed work globally as v0.6 does.

## Limits and shared kernel

- Model:≤4 exact machine locks,1–8 instances,0–16 routes,≤1 MiB canonical model JSON. Whole model union≤64 distinct component locks and≤32 distinct data locks. Existing per-machine/bundle limits still apply.
- Scenario:1–64 steps, exactly 1 initialization per instance,≤8 state expectations,≤32 message expectations,≤512 KiB canonical JSON. All bounded shape checks precede full encoding/copying. Actual merged state and mapped inputs each obey existing 64 KiB public-map limits.
- Run:≤32 materialized messages,≤64 total owned receipts and≤128 total owned effect intents,≤4096 reserved operations and≤4096 reserved invocations across all machines,≤2 MiB full retained result including all final states, envelopes, acknowledgements, traces and expectations. No silent eviction or partial-success return.
- Pre-copy incremental accounting must cover candidate state changes and outer message/acknowledgement/step evidence before publication. All machine sessions share one work/capacity budget. Do not duplicate retained histories inside sessions and outer results.
- HTTP/CLI input≤2 MiB, existing shared two-computation admission and 15-second deadline for checks/runs including semantic validation. Existing strict JSON status handling remains.

Refactor the state engine only as needed to expose an incremental session/host API. Preserve state.Run's complete canonical v0.6 output and error semantics, engine version, existing budgets and all hashes; pin canonical example result digests before refactoring and test them afterward. Existing pure evaluator remains unchanged.

## Model exports

```
ValidateDeliveryModel(DeliveryModel) []ValidationIssue
DeliveryModelDigest(DeliveryModel)(string,error)
ValidateDeliveryModelLock(DeliveryModelLock) []ValidationIssue
ValidateDeliveryScenario(DeliveryModelLock,DeliveryScenario) []ValidationIssue
DeliveryModelDataContracts(DeliveryModel)([]DataContractLock,[]ValidationIssue)
ExampleDeliveryScenarios() []DeliveryExample
```

## HTTP and PostgreSQL

Relative to /api; same strict canonical raw-JSON import flow as State. JSON 400 / body 413 / semantic-or-execution 422 / busy 429 / cancel 408. Modeled failures and failed expectations are recordable 200/201.

- GET /delivery/examples -> two DeliveryExamples.
- POST /delivery/prepare raw model -> canonical DeliveryModelLock.
- POST /delivery/validate-lock lock -> {valid,issues,model?:DeliveryModelLock}.
- GET /delivery/models -> sorted {model,digest,installedAt} records.
- POST /delivery/models lock ->201 new / 200 same / 409 conflict-or-full.
- GET /delivery/model?id=&version=&digest= -> exact record.
- POST /delivery/check {model:DeliveryModelLock,scenario} -> {scenario,result}, no persistence.
- POST /delivery/runs same ->201 {id,createdAt,model,scenario,result}.
- GET /delivery/runs -> newest 50 {id,createdAt,modelId,modelVersion,stepCount,messageCount,verified,passed}.
- GET /delivery/runs/{id} -> complete immutable record.

New delivery_models and delivery_runs tables. Independent 200-version catalog, advisory key (1279477075,5); installation never implicitly populates state/component/data catalogs. Same reinstall retains timestamp. This is experiment persistence only, not live message delivery. HTTP run submission itself is not idempotent.

## Examples, user workflow and implementation ownership

One exact generic delivery model is reused for pharmacy and library domains. It embeds the exact v0.6 resource-reservation source machine plus a generic typed reservation ledger recipient; no healthcare-specific engine branches. Include explicit drops, rollback, lost acknowledgement, replay after both owner restarts, sender replay/redispatch, business denial and independent state/message expectations. A separate small downstream example or acceptance case must prove sourceMessage chaining and explicitly bounded cycles/fanout without hidden delivery. Keep handwritten numeric oracles.

Messages workspace: diagram of pinned machine instances and typed routes; editable positions and route endpoints; exact state-machine catalog/import selection; mapping selectors with incompatible nominal types disabled; per-instance typed initial overrides; ordered command/dispatch/deliver/restart editor with named message selectors and faults; independent state/message expectations; step evidence with separate current state, historical receipt and transport status; immutable history; exact model/scenario export/import. Preserve drafts/pending JSON/stale evidence and strict pre-serialization number guards. Reuse existing typed controls. Drafts are session-local until exported, installed or recorded, as before.

CLI(root): deliverycheck raw model ->lock; --lock validates; --run DeliveryExample ->result with nonzero exit for error/verified failed expectations, unverified labelled. No database/network.

Root owns this contract, CLI, release docs/version files/CI and integration. Model agent owns new internal/model/delivery*.go except examples. Runtime agent owns internal/state refactor/tests and new internal/delivery. API agent owns server/store delivery files/routes/health/schema/tests. Frontend agent owns web/src updates. Architecture agent owns delivery example model/files/tests and example README. Review agent owns independent acceptance-delivery.mjs and read-only cross-review. No commits by agents.
