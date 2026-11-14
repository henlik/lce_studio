# M1 manual acceptance — connected offline engineering model

M1 completes the bounded **offline engineering integration** milestone: data,
behavior, state, messages and workflows can run together against typed modeled
providers, with saved evidence and independent expectations. Overall roadmap
progress is **33%**, with **M1 100%** after the automated acceptance gates pass.
Manual acceptance is **pending**. Use this checklist for the joint inspection;
record actual observations rather than treating this document as a passed test.

Live PostgreSQL/RabbitMQ execution adapters, calibrated network/power/geographic
simulation, general implementation conformance, signed releases and runtime
orchestration belong to subsequent milestones. PostgreSQL currently stores the
studio catalogs and evidence. It is not the modeled application's database adapter.

## 1. Start and preserve your work

1. Start PostgreSQL, the Go API and Vue UI using the repository README.
2. Open **Engineering → Design → System architecture**.
3. Save or export any current system before loading an example.
4. Open **File → Install complete M1 acceptance**. This explicitly installs a
   synthetic reservation example; new projects remain empty.
5. Check that the system name is **M1 acceptance · unified reservation**.
   Select **File → Save system**. Reinstalling the example preserves immutable
   versions; an existing version with different content must report a conflict.

This example is deliberately domain neutral. It exercises the same mechanics
needed by reception, pharmacy, procurement or another reusable component.

## 2. Inspect composition and ownership

Switch to **Architecture** and select **Stock owner**. Its definition is `studio.message-stock`
version `2.0.0`. Inspect its pinned aspects:

| Aspect | Slot | What it contributes |
| --- | --- | --- |
| Data | `data` | Exact typed reservation and notice records |
| Behavior | `input` | Reusable pure reservation input decision |
| State | `stock` | Owned availability, reserve command, receipts and effect intents |
| Messages | `notices` | Reservation-to-ledger transport contract |
| Workflow | `flow` | Human review, decision, command, broker wait, branching and cancellation |

1. Open each aspect from the selected definition, inspect it, and return to the
   source component. Check that the exact version/digest and return context remain.
2. In Data, inspect the reservation fields `tenantId`, `resourceId`, `quantity`.
3. In State, inspect the reserve command, atomic state update and emitted notice.
4. In Messages, inspect `reservationToLedger` and the ledger recipient command.
5. In Workflow, inspect Review request → Typed input decision → Reserve shared
   stock → Await ledger acknowledgement → terminal outcome. The false review
   branch loops to review; reserve denial and rollback take separate outcomes.
6. Select the **Reservation ledger** component. It owns a separate state machine. The message
   model and workflow bind to existing owners; they do not create extra copies.

Installed definitions are immutable. To change semantics, create a new aspect
version, install it, then create and install a new owning definition version.

## 3. Inspect typed provider bindings

Return to Stock's inspector and click **Load workflow bindings**.

| Requirement | Provider kind | Required mapping |
| --- | --- | --- |
| Stock `state/stock` | `modeled-state@1.0.0` | Stock owner |
| Stock `messages/notices` | `modeled-broker@1.0.0` | Source → Stock; recipient → Ledger |
| Stock `workflow/flow` | `modeled-workflow@1.0.0` | Workflow owner `stock` → Stock / stock |
| Ledger `state/ledger` | `modeled-state@1.0.0` | Ledger owner |

For the workflow's `handoff` node, check:

- Channel: Stock / notices; route: `reservationToLedger`.
- Source command node: `reserve`.
- Acknowledgement response field `accepted` maps to recipient output `accepted`.

Open **Execution → Prepare execution**. Expand **Provider compatibility**:
all four bindings must be satisfied, with their required and supplied capabilities.
Missing owners, wrong machine pins, incomplete response maps and unsupported
capabilities must block preparation with a specific issue path.

## 4. Run the complete journey

1. In Execution, click **Load M1 acceptance suite**.
2. Select **Complete M1 journey**.
3. Inspect the scenario before running. It has 13 explicit steps and separately
   authored final expectations. Start input is quantity **2**, initial stock **5**.
4. Click **Run shared scenario**. Expect **Scenario expectations passed**.
5. Use **Previous / Next** or the trace slider. Each automatic workflow transition
   and transport event has its own evidence, so one scenario step can produce
   several trace entries.

| Scenario step | Action | Observation to inspect |
| --- | --- | --- |
| 1 | Start `reservation` | Human review waits on `task1` |
| 2 | Approve `task1` | Pure decision runs; stock commits once; `task2` waits; notice dispatched |
| 3 | Drop delivery | No ledger commit; workflow remains waiting |
| 4 | Retry with recipient rollback | No ledger write or acknowledgement |
| 5 | Retry and lose acknowledgement | Ledger commits; broker remains pending; workflow still waiting |
| 6–8 | Restart ledger, stock and broker | Durable revisions/receipts/intents/envelope retained; volatile counters reset |
| 9 | Restart workflow | Same waiting `task2`; reserve is not run again |
| 10 | Retry delivery successfully | Recipient receipt replays; matching acknowledgement resumes workflow |
| 11 | Retry after acknowledgement | No new ledger write or workflow completion |
| 12 | Restart completed workflow | Completion retained |
| 13 | Repeat identical start | Existing completion reused |

Final expected result:

| Item | Expected |
| --- | --- |
| Workflow | `completed`, outcome `reservationRecorded` |
| Outputs | `remaining=3`, `recorded=true` |
| Stock | availability 3; revision 1; one receipt; one effect intent |
| Ledger | totalReserved 2; revision 1; one receipt; no effect intents |
| Volatile counters | Both zero after the modeled owner restarts |
| Message | acknowledged; four delivery attempts; accepted=true |

A broker acknowledgement is evidence of the recipient's durable command receipt,
including business acceptance or rejection. A lost acknowledgement cannot finish
a waiting task. A manually supplied human response cannot impersonate that receipt.

## 5. Run the other five scenarios

Choose each case in **M1 acceptance scenario** and click **Run shared scenario**.
All six are expected to pass their assertions, including cases that intentionally
reject or cancel the business operation.

| Scenario | Expected result |
| --- | --- |
| Lost acknowledgement stays waiting | Ledger total 2, pending message, workflow waiting on handoff, recorded=false |
| Stock rejection | Quantity 99 rejected; stock stays 5; ledger 0; no notice; outcome reservationRejected |
| Atomic rollback | Stock stays 5; no receipt or intent; ledger 0; outcome reservationFailed |
| Review loop and duplicate event | Second review has a new task identity; duplicate old event cannot finish it; final stock 3 and ledger 2 |
| Cancel and ignore late acknowledgement | Workflow remains reservationCancelled after late delivery; ledger may commit 2; recorded=false |

Cancellation stops the workflow but does not recall an already dispatched message
or compensate committed work. The final case makes that distinction observable.

## 6. Check rejection and failed assertions

Save a copy first and restore it after each experiment.

1. Change **Expected final workflows → Outcome** to `wrongOutcome`, then run.
   Execution should succeed with **Scenario expectations failed**. Verify replay:
   it should match the original failed report, without turning it into a pass.
2. Clear the Stock workflow's shared owner mapping. Prepare: expect rejection.
   Restore the mapping and prepare again.
3. Add `live-network` to a binding's extra required capabilities. Prepare: expect
   rejection naming the missing capability. Remove it and prepare again.
4. Clear a required acknowledgement response mapping. Prepare: expect rejection.
5. In a copy of the complete scenario, use an unknown correlated task ID for a
   delivery. Run: expect rejection, with no successful execution evidence saved.
6. Remove a required start input. Run: expect typed-input rejection.

For a manual broker-forgery check, make a new scenario with Start, a human approval
response to `task1`, then another human response aimed at `task2`. The broker task
must reject that response regardless of the supplied value.

## 7. Verify persistence and evidence

1. Export an execution. Inspect design/input/plan/report hashes, exact artifact
   pins, scenario, workflow trace, state transitions, broker receipts and outcomes.
2. Click **Verify replay**. Expect **matched**, identical report hashes and a
   separate immutable verification record.
3. Refresh the browser. Reopen **Saved executions** and its replay history.
   The saved run must remain unchanged by later scenario or design edits.
4. Edit a scenario name or input; wait for draft-save confirmation; refresh.
   Prepare the same design again if required. The scenario draft must recover.
5. Export the system JSON; import it into a fresh draft. Check workflow owner,
   message handoff, provider capability and exact version bindings survive.
6. Open the saved system through File. Check design and catalog ownership context.

Saved replay uses frozen plans, not mutable current catalogs. An automated test
removes the artifact catalogs and still reproduces the M1 report. Do not delete
catalogs manually in a workspace containing your designs.

## 8. Inspect the other M1 gates and UX

The unified exercise closes joint execution. Use these existing examples to
inspect the other M1 gates during the same session:

| Action | Inspect |
| --- | --- |
| File → New empty system | No example data/components inserted automatically |
| File → Install connection example | Compatible typed data arrows; Prepare; Use example inputs; Run model; token transfers |
| File → Install mismatch example | Validation rejects incompatible ports before execution |
| File → Install multi-command example | Selected graph entry; direct owner commands; shared revision conflicts and receipt replay |
| Open a definition and change a pin to a missing version in a copy | Dependency closure fails with its exact location |
| Change aspect content under the same installed version | Immutable install rejects changed content |

For UI acceptance, record pass/fail for:

- Left library search/categories/add; right inspector; contextual aspect navigation.
- Collapsible panels, ribbon, canvas expansion and narrow-screen overflow.
- Required fields, issue location, loading indicators and notifications.
- Sequential token trace, loops, unvisited branches and exact waiting task identity.
- Refresh recovery, saved evidence, import/export and stale-plan warnings.

## Scope and limits to keep visible

- Pure/state **data** connections execute as acyclic dataflow. Workflow control
  executes through its pinned nodes and explicit owner/message bindings. An arbitrary
  request/event arrow is not automatically an executable adapter.
- Human responses, signals, delivery and retry are explicitly ordered scenario
  steps. This explores an interleaving, not every race or a real network clock.
- Up to 64 scenario steps, eight started workflow instances, and 256 combined
  task/workflow activations; existing state work, receipt, trace and report limits
  also apply. Oversized runs fail; they do not produce partial success evidence.
- State lives across steps of one model run. A new run starts fresh. Modeled
  restarts retain modeled durable state, not a tested live process or disk image.
- Workflow command fault injection currently supports before-commit rollback.
  Direct owner/graph scenarios separately cover lost command replies.
- Scenario assertions and exact replay are evidence, not a proof of all possible
  executions. System contract claims remain unevaluated in these engine runs.
- Nested workflow calls, automatic clock-driven delivery, live service adapters,
  calibrated continuum deployment and production security are subsequent work.

## Joint inspection record

| Area | Result | Observation / reproduction steps |
| --- | --- | --- |
| Installation and five owned aspects | Pending | |
| Provider and owner bindings | Pending | |
| Complete journey and trace | Pending | |
| Five alternative scenarios | Pending | |
| Invalid bindings / failed expectation | Pending | |
| Persistence / recovery / export / replay | Pending | |
| Other M1 gates | Pending | |
| Desktop / narrow layout and interaction | Pending | |

For repeatable API coverage, run `node scripts/acceptance-system.mjs` against a
**disposable** running studio database. It includes the existing conformance,
connection, state, multi-command and message tests plus all six M1 cases and replay.
