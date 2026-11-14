# Connect stateful components to an offline provider

The system graph can now execute pure components and owned state aspects through selected graph
entry commands. This walkthrough uses the original sole-command example.
For multiple commands on one owner, see [MULTI_COMMAND_SYSTEMS.md](MULTI_COMMAND_SYSTEMS.md). The state transition reuses the existing state engine. Its
provider is explicitly **modeled-state@1.0.0**: an offline semantic adapter with
atomic state updates, revision checks, durable receipts and pending effect intents
inside one experiment. It is not PostgreSQL, Redis, a live application or deployment.

For repeated commands, faults and restarts in one run, follow
[SHARED_SYSTEM_SCENARIOS.md](SHARED_SYSTEM_SCENARIOS.md). The single-run mode below
still starts fresh for each invocation.

## Inspect the example

1. Open **Engineering → Design → System architecture → File**.
2. Choose **Install state execution example**. Installation explicitly adds reusable
   request and summary packages, a stock state machine and a component definition.
   New projects remain empty. Installation is idempotent per artifact; if a later
   artifact conflicts, earlier successful installations remain and retry is safe.
   The current draft is replaced only after the complete installation succeeds.
3. The **Execution** perspective opens. Click **Prepare execution**.
4. Inspect the provider notice for **Stock owner / stock**. Its binding is
   `state/stock` → `stock-model`, with type `modeled-state@1.0.0`.
5. Click **Use example inputs**. The generated request contains `tenantDefault`,
   `resourceDefault`, and quantity `2`. These are synthetic identifiers.
6. Click **Run model**. The initial stock is `5`; the accepted reservation leaves
   `3`. The graph executes Request → Stock owner → Summary.
7. Step through the eight events. At **Stock owner completed**, inspect:

   | Evidence | Expected value |
   | --- | --- |
   | Status | `committed` |
   | Available stock | `5` → `3` |
   | Revision | `0` → `1` |
   | Receipt | One accepted command with its original outputs |
   | Effect intents | One reservation intent, not delivered |
   | Summary output | `remaining=3` |

8. Change quantity to `9` and run again. Status is `denied`, stock stays `5`,
   revision stays `0`, one denied receipt is recorded, and no effect is emitted.
   The summary receives `remaining=5`. Each run starts from the declared initial
   state; it does not continue the preceding run's inventory.
9. Reopen **Saved executions**, compare the two immutable records, then export
   either JSON snapshot. The old record retains its own exact inputs and state.
10. For a negative check, open **Architecture**, select **Stock owner**, and remove
    its binding or change the requirement to `state/wrong`. Preparation must reject
    it and locate the component. Restore `state/stock` → `stock-model` to run again.

## Build your own component

Create and install a state machine in the component State editor, with one or more
commands, declared inputs/outputs and initial state. For multiple commands, select
the graph entry in the component inspector after adding it to your system. Its decision uses a locked pure
behavior bundle. Create a component definition with a named `state` aspect pinned
to that exact machine digest, then install the definition and add it to your system.

In **System settings → Offline execution providers**, add a modeled-state provider.
Select the component and bind requirement `state/<aspect-slot>` to that provider ID.
Connect `command/<command-name>/inputs` and `command/<command-name>/outputs` using
the existing typed port selectors and `data` links. Pure aspects continue to use
`inputs` and `outputs`. Only unconnected inputs appear on the execution form.

Changing initial state requires a new machine version and updated definition pin.
Separate state aspects have isolated ownership even when bound to the same provider.
Their effect identities are scoped by task identity. Sharing a provider does not
share mutable application state between components.

The optional system field `executionProviders` contains `{id, kind, version}`.
Provider IDs must be unique and distinct from deployment resources. The runner
requires exact `modeled-state` / `1.0.0`, rejects unused providers and unrecognized
bindings, and does not infer provider behavior from a resource's name.

## Semantics and limits

- One invocation per task, deterministic topological order, fresh state per aspect.
  A sole command is selected automatically for legacy owners. Multi-command owners
  require an explicit entry selection; direct scenario commands share that owner.
- The invocation uses a run-local `execute` command ID and expected revision `0`.
  The single-run mode does not exercise retries or faults. The separate shared-scenario
  mode now supports repeated invocations, explicit faults and owner restarts. The
  dedicated State workspace remains available for its existing ordered scenarios.
- Accepted commands commit writes, increment revision and retain effect intents.
  Denied commands retain a receipt and outputs without changing state or emitting
  effects. Both outcomes can feed downstream typed ports. An evaluation failure
  without a receipt aborts the graph and produces no successful partial evidence.
- Message/workflow aspects, effect-delivery links, cycles, external providers,
  deployment plans and arbitrary application code remain unsupported. Every rejected
  feature is explicit; no such aspect is silently skipped.
- Existing graph limits apply: 64 tasks, 256 links, 4 MiB task definitions, 4096
  operations/invocations, 2 MiB trace, 4 MiB report and 8 MiB saved snapshot. State
  sessions share receipt/effect budgets. PostgreSQL saves studio evidence only.
- State reports identify `system-dataflow-2`; existing pure graphs keep
  `system-pure-1` and their report shape. Exact machine/provider definitions are
  saved with the plan. Catalog-free replay is available through **Verify replay**
  in the UI/API; see [SYSTEM_REPLAY.md](SYSTEM_REPLAY.md). Signed provenance is
  not implemented. Claims remain unevaluated.

## Progress and verification boundary

Roadmap v1 remains **28% (+0 points), M1 80%**. This advances the joint-execution
checkpoint without closing message/workflow and live-provider work. Selected-entry
and direct multi-command steps are documented in the companion guide.
No milestone scope or weights changed. Browser and mobile interaction checks for
this walkthrough remain outstanding; engine, persistence, API and form tests are
recorded in [VERIFICATION.md](VERIFICATION.md).
