# Multiple commands on one system state owner

A component state aspect can expose several commands with different typed inputs
and outputs. A graph invocation uses one explicitly selected **entry command**;
an **owner command** scenario step can call any command on the same owner.
Both paths share that owner's durable state, revision, receipt namespace and
effect intents. A separate state aspect or component instance remains isolated,
even when it uses the same offline provider.

## Inspect the complete example

1. Open **Design → System architecture → File → Install multi-command example**.
   This explicitly installs stock machine/definition version `2.0.0`, reusable
   request/summary packages and a pure adjustment component. Version `1.0.0`
   remains unchanged. New projects are still empty.
2. Open **Architecture**, select **Stock owner**, and find **State graph entry
   commands** in the inspector. Click **Load command choices**. Slot `stock` has
   `reserve` and `adjust`; its selected graph entry is `reserve`.
3. The graph is Request → Stock owner → Summary. Its typed connections use
   `command/reserve/inputs` and `command/reserve/outputs`. The state provider is
   still `stock-model`, of kind `modeled-state@1.0.0`.
4. Open **Execution → Prepare execution**. Single-run mode invokes the selected
   entry and starts from the definition's initial state. It is separate from
   the shared scenario below.
5. In **Shared scenario**, click **Load multi-command scenario**. Inspect its
   request identities, revisions, faults and handwritten expectations before running.
6. Click **Run shared scenario**. Expected final state is availability `7`,
   revision `3`, four receipts, two reservation intents and volatile reservation
   count `0`. The two commands are:

   | Command | Input | Behavior |
   | --- | --- | --- |
   | `reserve` | Typed reservation record | Existing matching-target, positive-quantity and sufficient-stock decision; emits a reservation intent on acceptance |
   | `adjust` | Integer `count` | Replaces availability with the supplied nonnegative count; emits no intent and does not change the reservation counter |

   `adjust` is a synthetic inventory-count operation. It is not an authorization
   mechanism or a live inventory integration.

   | Step | Action | Owner outcome | Availability / revision |
   | --- | --- | --- | --- |
   | 1 | Graph reserve 2, ID `reserveA`, expected revision 0 | Committed | 3 / 1 |
   | 2 | Owner `adjust` to 8, ID `adjustA`, expected revision 1 | Committed | 8 / 2 |
   | 3 | Repeat step 2 with the same identity and payload | Replayed | 8 / 2 |
   | 4 | Graph reserve 1, ID `reserveB`, expected revision 2; lose reply | Reply lost; Summary skipped | 7 / 3 |
   | 5 | Restart owner | Durable state retained; volatile counter resets | 7 / 3 |
   | 6 | Owner `reserve`, repeating step 4's exact request | Replayed | 7 / 3 |
   | 7 | Owner `adjust` reusing `reserveA` | Identity conflict across commands | 7 / 3 |
   | 8 | Owner `adjust`, expected revision 2 | Revision conflict | 7 / 3 |
   | 9 | Owner `adjust`, expected revision 3; fault before commit | Rolled back | 7 / 3 |
   | 10 | Owner `adjust` to −1, expected revision 3 | Denied; receipt retained | 7 / 3 |

7. Step through the token trace. Graph invocations show the graph and actual
   transfers. Direct command and restart steps focus on the targeted owner;
   they do not run other tasks or traverse graph links. At command completion,
   inspect the command name, receipt, current state and before/after revision.
8. Change the final availability expectation to `99` and run again: the scenario
   must fail. Expectations are not inferred from observations. Removing all
   expectations produces an unverified result.
9. Use **Saved executions**, **Verify replay** and both JSON exports. Each record
   contains the exact selected entry, all command definitions and scenario calls.
   Replay works after catalog changes; it does not promote a failed assertion.

## Author your own calls

Use **Add owner command** to target one existing state owner. Select its command,
enter every required typed input, a request **Command ID**, explicit expected
revision and optional fault. The command name identifies the operation; Command
ID identifies a particular request and is shared across that owner's commands.

Changing the owner or command resets the step's inputs, expected revision and
outcome assertions. Review them before running. **Duplicate / retry** retains
an independent copy of the envelope; keep the ID, revision, command and payload
unchanged for an identical retry. Changing any of these under an existing ID
is an identity conflict. Fault selection is not part of request identity.

Graph invocations use the selected entry; their envelopes cannot override it.
Owner command steps take explicit scenario inputs and record responses without
feeding the graph. To feed outputs into other components, choose that operation
as the graph entry and connect its exact typed ports. Arbitrary automatic command
routing and nested workflow/message scheduling remain future work.

For a new component, install a state machine containing multiple commands, pin it
as a state aspect in an immutable component definition, and add that definition
to a system. Bind `state/<slot>` to an offline modeled-state provider. Load the
command choices in its inspector and select a graph entry. Switching entry does
not rewrite connections: **Prepare execution** rejects links to another command's
interfaces or incompatible input types. Unknown/duplicate/non-state slot choices
are rejected. Sole-command legacy owners can keep the automatic entry selection.

## Portable format and versions

An optional component instance field selects graph entries:

```json
"stateEntries": [{"slot":"stock","command":"reserve"}]
```

The prepared task carries `state.entryCommand`. A direct scenario command uses:

```json
{
  "kind": "command",
  "taskId": "<prepared owner task ID>",
  "calls": [{
    "taskId": "<prepared owner task ID>",
    "command": "adjust",
    "commandId": "adjustA",
    "expectedRevision": 1
  }],
  "inputs": [{
    "taskId": "<prepared owner task ID>",
    "values": {"count": {"type":"integer","integer":8}}
  }],
  "expectedStatuses": {"<prepared owner task ID>":"committed"}
}
```

There must be exactly one matching owner, envelope and input set. Use an empty
`values` object for a command with no inputs. Unknown commands, extra/missing
fields, wrong record locks and unrelated expected task statuses are rejected.
The existing execution API accepts these scenarios. The explicit fixture endpoint
is `POST /api/systems/multi-command-example`.

New selected-entry plans use `system-dataflow-3`; scenarios with these plans or
direct command steps use `system-scenario-2`. Older unselected sole-command plans
and invocation/restart-only scenarios retain their engine identifiers and report
shapes, including replay support. All engines remain deterministic and bounded.

## Scope and progress

The scheduler executes one declared sequence. It detects stale revisions and
request-identity conflicts; it does not explore every race or simulate parallel
CPU execution. Each run creates fresh in-memory ownership. Effects remain intents,
and restarts model process-local recovery assumptions, not destruction of a site.
Existing limits remain: 64 scenario steps, 256 activations, shared 4096-operation
and invocation budgets, receipt/effect caps and bounded traces/snapshots.

Roadmap remains **28% overall, M1 80% (+0 points)**. This completes selected-entry
and explicit multi-command scenario support. Message/workflow integration, live
providers and general implementation conformance remain open; milestone weights
and acceptance gates are unchanged. Browser/mobile interaction remains unverified.
