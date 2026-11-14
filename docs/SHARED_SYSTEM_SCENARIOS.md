# Shared command scenarios

A shared scenario runs several graph invocations and owner restarts against the
same in-memory state sessions. The sessions exist only inside this run; another
run starts from its pinned initial definitions. PostgreSQL retains the complete
experiment, not live business state.

## Walkthrough

1. Open **System architecture → File → Install state execution example**.
2. In **Execution**, click **Prepare execution**.
3. Find **Shared scenario** and click **Load stock retry scenario**. This action
   explicitly fills the six-step example and its independently written expectations.
   The single-run input form above is separate; each scenario invocation owns its
   own editable typed inputs.
4. Inspect the command IDs, expected revisions, input quantities and fault choices.
   Duplicate a step to model a retry. Keep the same ID, revision and input values
   for an identical retry; a different request under the same ID is a conflict.
5. Expand **Expected task outcomes** on each step. These assertions are entered
   before execution and are never inferred from observed results.
6. Inspect **Expected final state**: availability `2`, volatile counter `0`, revision
   `2`, two receipts and two pending effect intents. Every state slot and all three
   counts must be supplied for a final-state expectation.
7. Click **Run shared scenario**. Review the verdict, then use the trace controls.
   Each event identifies its scenario step. At state completion, inspect the current
   before/after values and the receipt separately; a replay can return historical
   outputs without restoring historical state.

| Step | Action | State-owner outcome | Stock | Summary |
| --- | --- | --- | --- | --- |
| 1 | `reserveA`, revision 0, quantity 2 | Committed | 3 | Receives 3 |
| 2 | Identical `reserveA` retry | Replayed | 3 | Receives original 3 |
| 3 | `stale`, revision 0, quantity 1 | Revision conflict | 3 | Skipped |
| 4 | `reserveB`, revision 1, quantity 1; lose reply after commit | Reply lost | 2 | Skipped |
| 5 | Restart the stock owner | Restarted | 2 | Not invoked |
| 6 | Identical `reserveB` retry | Replayed | 2 | Receives original 2 |

8. Change expected final availability to `99` and rerun. The run is retained with a
   **failed** verdict and its evidence; it is not a validation error or a successful
   verification. Restore `2` afterward.
9. Remove all step and final assertions to explore without a claim: the report is
   **unverified**, never passed. This does not evaluate the system contract.
10. Reopen either record from **Saved executions** or export it. The snapshot contains
    its own scenario, exact plan and result. Editing the scenario affects only the
    next run. Scenario drafts autosave per system ID; preparing a changed graph
    requires resetting the incompatible scenario or explicitly loading the example.

## Author a scenario

**Add graph invocation** adds one visit to the whole acyclic graph. Supply all external
inputs and one command envelope for every state task. Expected revision is explicit;
the studio never silently replaces it with the latest revision. **Move up/down**
changes the declared order. **Duplicate / retry** creates independent editable copies.

**Add owner restart** targets one state task. It preserves that owner's durable slots,
revision, receipts and effect intents, and resets volatile slots to their declared
initial values. Other owners remain unchanged. A restart does not invoke the graph.

Fault options are none, before commit, and after commit before reply. Before-commit
rollback creates no receipt or effect. A lost reply still commits state, but cannot
transfer a response token. A matching retry returns the saved receipt without applying
the operation again. Denied receipts do return their declared outputs and can route
downstream. Revision conflicts, identity conflicts and evaluation failures without
a receipt block their outgoing data; consumers missing an input are marked skipped.
Independent branches may still execute. This is not a cross-component transaction.

The scheduler explores one explicitly ordered interleaving. Two commands using the
same expected revision can demonstrate a stale-write conflict. It does not discover
all possible races, simulate CPU parallelism or prove global serializability.

## API and portable evidence

Use `POST /api/systems/execution/run` with `{design, scenario}`. Supplying both
nonempty single-run `inputs` and a scenario is rejected. A scenario has:

- `name`, ordered `steps`, and `expected` final-state assertions.
- An invoke step: `kind: "invoke"`, typed `inputs`, `calls`, optional
  `expectedStatuses` keyed by task ID. Every call has `taskId`, `commandId`, explicit
  `expectedRevision`, and optional `fault`.
- A restart step: `kind: "restart"`, `taskId`, optional expected status. No calls
  or input values are allowed.
- A final expectation: `taskId`, full typed `state`, and explicit `revision`,
  `receipts`, `effects`. Missing numeric fields are rejected, not treated as zero.

The saved record's `inputHash` hashes the scenario rather than single-run inputs.
The report identifies `system-scenario-1`; events/results have one-based
`scenarioStep` values. `report.scenario` contains verified/passed flags, independent
assertions and final owner snapshots. Existing single-run report shapes remain
compatible. `claimsEvaluated` remains false: scenario assertions are not implementation
conformance or system-contract verdicts.

`systemrun.RunScenario(ctx, saved.Plan, *saved.Scenario)` replays without catalog
queries. **Verify replay** now repeats this check through the UI/API, preserving
failed and unverified scenario outcomes; see [SYSTEM_REPLAY.md](SYSTEM_REPLAY.md).
Signed provenance remains future work.

Bounds: 1–64 steps, 256 total task activations, 1 MiB scenario, shared 4096-operation
and invocation budgets, state receipt/effect limits, 2 MiB trace, 4 MiB report and
8 MiB persisted snapshot. Invalid envelopes, cancellation and exhausted budgets
return no successful partial report. Expected business conflicts and failed
assertions are valid, recordable experiment outcomes.

## Remaining scope

Graph invocations use one selected entry per owner. Explicit owner-command steps
can now call other commands with the same owned state and receipt history; see
[MULTI_COMMAND_SYSTEMS.md](MULTI_COMMAND_SYSTEMS.md). Message delivery, workflow
adapters, cycles, live providers and distributed deployment remain outside this
runner. Effects are pending intents only.

Roadmap remains **28% overall, M1 80% (+0 points)**. This completes the shared-scenario
increment, not the broader joint-engine or shared workload/failure checkpoints.
Browser/mobile interaction checks remain outstanding.
