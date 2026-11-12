# Authoring typed message delivery

The Messages workspace connects independently owned state machines. Use it to test what happens between a committed intention and a recipient's acknowledged response. It builds on [State](STATE_AUTHORING.md) and its pure decision components; the exact implementation rules are in [DELIVERY_CONTRACT.md](DELIVERY_CONTRACT.md).

## Start with the reference experiments

The pharmacy and school-library experiments share one exact model. A resource owner reserves stock or copies and commits a typed reservation notice. A separate ledger accepts that notice through a declared command and updates its own total. The scenario deliberately drops a delivery, rolls back a recipient attempt, loses an acknowledgement and restarts both owners before retrying.

1. Open Messages and choose a reference experiment.
2. Inspect each instance's pinned machine and the route from its source effect to the recipient command. A route maps typed payload ports to typed command inputs.
3. Review the initial overrides and ordered steps. Commands, dispatches, deliveries and restarts are separate actions.
4. Read the independently entered final state and message expectations before running.
5. Run and inspect each transition. Compare recipient state, historical receipts and message acknowledgement status separately.

The expected final reserved total is five for the pharmacy and four for the library. All three messages are acknowledged. The first message needs four delivery attempts; the other two need one each. Domain names and initial quantities change, while the model and machine definitions remain identical.

## Compose exact state owners

Create a new delivery model and import or select immutable state-machine versions. Add named instances of those machines. Two instances may share a definition while owning completely separate state, revision, receipts and intents. Position the instances in the diagram and add routes between their declared boundaries.

For each route, choose a source instance and effect, a recipient instance and command, and a source payload port for every recipient input. Scalar types must match exactly; records and collections must reference the same exact nominal contract. A route does not convert types or inspect fields inside a record. Put that adaptation in a pure component within an emitting or receiving machine.

Fanout uses separate routes to distinct recipients or commands. A duplicate source/effect/recipient/command route is rejected. Self-routes and cycles are permitted, but they do not trigger background execution. Every action remains bounded by the explicit scenario.

Machine definitions and their pure/data dependencies travel with the model. Installing a delivery model does not install them into the other catalogs. Editing installed content requires a new version. Neither a model's digest nor its installation proves publisher trust or business correctness.

The browser rejects exact model/machine locks containing negative-zero coordinates to avoid changing their hashes during JSON serialization. Retain those artifacts through the CLI, or publish a new version using ordinary zero coordinates before graphical authoring.

## Keep dispatch separate from delivery

A source command may commit an effect intention. Dispatch chooses that committed intention, a route, a readable message alias and the recipient revision expected by the sender. It freezes the exact target command, revision and typed inputs. Dispatch itself never executes the recipient.

Delivery refers to that message alias. Every retry sends the same frozen request. Repeating dispatch returns the existing message when its frozen fields match; changing the expected revision conflicts. Retrying cannot silently update a stale revision to the recipient's latest state.

For example, two messages dispatched for recipient revision zero cannot both create new accepted commits sequentially. Once one commits revision one, the other remains conflicted unless its original command already has a matching receipt. A new business request must have a new originating intent; the tool does not invent a new request during retry.

Use a source command ID to select an intention from a direct scenario command. For a downstream intention produced by a delivered command, select its earlier source-message alias. This follows the generated recipient command identity without requiring a hash to be typed. A recipient may produce a downstream intention even while its upstream acknowledgement is lost. Its commit and its sender's knowledge of that commit are distinct.

## Choose a failure point

| Delivery fault | Observable consequence |
| --- | --- |
| None | A durable committed, denied or replayed receipt acknowledges the message |
| Drop before receive | No recipient evaluation or mutation; the message remains pending |
| Before recipient commit | A fresh proposal rolls back without a receipt; the message remains pending |
| After recipient commit, before acknowledgement | Recipient work remains committed, while the sender still sees a pending message |

An acknowledgement can be lost again when a retry returns a cached receipt. A before-commit fault cannot roll back an already committed historical receipt, so a matching replay bypasses that fault and can acknowledge normally. Delivering an already acknowledged message is a no-op and does not count another attempt.

A denied business request can still be acknowledged: the recipient durably recorded its response with `accepted=false`. Check acceptance explicitly when the experiment requires successful business action. A revision conflict, ordinary evaluation error or rollback has no durable receipt and remains unacknowledged.

Restarts reset only the selected owner's volatile slots. Durable state, receipts, effect intentions, materialized messages, attempts and acknowledgements survive under this simulator's explicit durability assumptions. They do not represent a real database or broker crash-recovery implementation.

## Specify independent expectations

State expectations describe complete final owned values, revision, receipt count and effect count for each selected instance. Message expectations name a message, its final absent/pending/acknowledged status and exact attempt count. An acknowledged message can additionally assert accepted true or false.

An empty pair of expectation lists produces an unverified result. Partial lists verify only the listed facts. Do not copy observed results into expectations merely to make a run pass; use independent invariants and arithmetic to decide what should happen.

Evidence retains immutable model/scenario definitions, all state transitions, historical receipts and frozen message envelopes. Editing the draft marks earlier evidence as belonging to another snapshot. Opening history preserves the draft; loading a recorded snapshot is an explicit replacement action. Pending raw JSON is preserved when hidden and cannot silently overwrite newer graphical edits.

## Export and check offline

Keep the exact model lock with its scenario export. A complete run input is `{model,scenario}`. Bundled examples under `examples/delivery/` contain both.

```sh
go run ./cmd/deliverycheck model.json > model-lock.json
go run ./cmd/deliverycheck --lock model-lock.json
go run ./cmd/deliverycheck --run experiment.json
```

The checker needs no server, broker, database or network. Failed declared expectations print their complete verdict and return a nonzero exit status. Empty expectations are explicitly unverified. Oversized or malformed work returns an error rather than partial successful evidence.

This increment models communication correctness across explicit ordered steps. It does not send real messages, estimate latency, run owners concurrently, generate applications or deploy them. Coupling these transitions to hardware, network, geographic and power models requires a later clock and scheduling contract.
