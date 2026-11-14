# Shared system messages: stock owner → reservation ledger

This exercise connects an installed message definition to existing component state
owners. Graph invocations, explicit owner commands and delivered commands use the
same state sessions, revisions, receipts and committed effect intents. All data is
synthetic. This is an offline model experiment, not a live broker integration or
proof that an implementation conforms.

## Reproduce in the studio

1. Open **Design → System architecture**.
2. In **File**, select **Install stock-to-ledger messages**. This installs exact
   reusable artifacts idempotently and opens a new example draft. It does not
   replace existing installed artifact versions.
3. Open **Architecture**. Select **Stock owner**. In its inspector, under
   **Message actors and broker**, select **Load message actors**.
4. Inspect the `notices` aspect. Its broker is `reservation-broker`.
   Actor `stock` maps to **Stock owner / stock**, and actor `ledger` maps to
   **Reservation ledger / ledger**. Only owners matching the pinned machine ID,
   version and digest are offered. Both mappings are required.
5. Open the left library’s **Supporting services** section. The example has
   `owned-state` (`modeled-state@1.0.0`) and `reservation-broker`
   (`modeled-broker@1.0.0`). These are execution providers, separate from
   application components and deployment resources.
6. Open **Execution**, select **Prepare execution**, then **Load stock-to-ledger
   scenario**. Message designs require a shared scenario; the single graph-run
   button is disabled. Scenario edits autosave under this design’s identity.
7. Review the fourteen steps below, including their expected transport outcomes.
   Expectations are handwritten and independently checked, not learned from the
   observed run. Select **Run shared scenario**.
8. Inspect the verdict and move through the token trace with **Next**, **Previous**
   or the slider. The message journey shows intent creation, dispatch, delivery,
   recipient commit and acknowledgement. Retry has its own recipient-replay event.
9. At **acknowledgement_lost**, inspect the transport snapshot: the ledger has
   committed, but the message is still pending and has no acknowledgement.
10. At the later retry, inspect the historical receipt and current owner revision.
    There is one ledger business write, not a second reservation.
11. Open **Saved executions**, reload this run, and use **Verify replay**. The
    pinned snapshot is sufficient even if the current artifact catalog changes.
    A matched replay means the recorded model report reproduced; it does not
    establish production correctness. Export the execution or verification report
    for inspection.
12. As a negative control, change expected attempts from `4` to `3` and run again.
    The scenario must fail. The earlier saved experiment remains unchanged.

## Ordered experiment

| Step | Action | Expected result |
| --- | --- | --- |
| 1 | Reserve two units, fault before commit | Rolled back; no receipt or intent |
| 2 | Dispatch that reservation | Missing intent; no deliverable message |
| 3 | Reserve two units as `reserveA` | Stock 5 → 3; revision 1; one intent |
| 4 | Dispatch as `reservationA`, recipient revision 0 | Frozen pending message |
| 5 | Deliver; drop before receipt | Pending, attempt 1 |
| 6 | Retry; rollback recipient before commit | Pending, attempt 2; no ledger write |
| 7 | Retry; lose acknowledgement after recipient processing | Ledger total 2, revision 1; pending, attempt 3 |
| 8 | Restart the modeled broker | Pending envelope and attempts retained |
| 9 | Restart ledger owner | Durable total and receipt retained; volatile counter reset |
| 10 | Restart stock owner | Durable stock and intent retained; volatile counter reset |
| 11 | Retry without fault | Historical recipient receipt replayed; acknowledged, attempt 4 |
| 12 | Retry acknowledged message | Already acknowledged; attempt count remains 4 |
| 13 | Repeat `reserveA` with original inputs/revision | Original stock receipt replayed |
| 14 | Repeat original dispatch | Existing immutable envelope reused |

Final expectations: stock available **3**, stock revision **1**, one stock receipt
and one intent; ledger total **2**, revision **1**, one ledger receipt, no ledger
intent. Both volatile counters are **0**. `noMessage` is absent;
`reservationA` is acknowledged after **4** attempts, with accepted business verdict.

## Build another message-connected system

Create and install the state machines and message model in Components. Include the
state and messages artifacts as pinned aspects of versioned component definitions.
Install those definitions, add their instances to a system, and add state and broker
providers in Supporting services. Bind `state/<slot>` to a modeled-state provider
and `messages/<slot>` to a modeled-broker provider. Load message actors in the
component inspector and map every actor to an exact matching state owner.

In Execution, prepare and start an empty shared scenario. Add owner commands or
graph invocations, then **dispatch**, **deliver**, **retry**, **broker restart** or
**owner restart** steps. Dispatch requires a route, exactly one source command ID
or upstream message alias, a new alias and an explicit recipient revision. Delivery
selects an earlier alias; it never creates a new request identity. Use an upstream
message alias to dispatch an effect created by a delivered command. Reordering or
removing prerequisites can invalidate a scenario; validation explains the missing
alias or binding. Add expected final state and message assertions before running.

## Execution contract and limits

- `modeled-broker@1.0.0` fixes durable transport semantics: explicit delivery,
  retryable pending messages, immutable requests, and receipt-based acknowledgement.
  A receipt may acknowledge a business rejection; acknowledgement is not acceptance.
- There is no automatic dispatch, retry timer, network, clock, live RabbitMQ,
  PostgreSQL business-state adapter, distributed transaction or exhaustive schedule
  exploration. Broker restart is an instantaneous modeled restart preserving its
  durable envelopes, attempts and acknowledgements; it does not model disk loss or
  downtime. Owner restart follows each state slot’s declared durability.
- A channel namespaces its generated recipient command IDs. The `msg_` prefix is
  reserved in message-enabled system scenarios. Actor mappings are one-to-one
  within a channel; different channels may reference the same real owner.
- Dispatch selectors and mapped types are validated against the pinned message
  model. Missing committed source effects cannot create messages. A frozen revision
  conflict stays pending; retry does not silently change the request to force success.
- At most 16 channels, 64 scenario steps, 256 activations and 32 messages per channel;
  shared state work/receipt/effect budgets and bounded transport/report sizes apply.
- New plans use `system-dataflow-4`; message scenarios use `system-scenario-3`.
  Existing pure/state/multi-command versions and report shapes remain supported.
  Saved executions retain channels, actor mappings, locks, inputs, expectations,
  plan/report hashes and transport evidence for catalog-independent replay.

Roadmap: **28% overall; M1 80% (+0 points)**. This completes the four-step message
increment. The existing joint-execution gate remains open for workflow integration
and broader provider execution; it has not been redefined to award partial credit.
