# Execute connected components

Engineering → Design → System architecture → Execution runs a bounded, acyclic
graph of pure behavior components. Single-command state aspects are also supported
with explicit offline provider bindings; see [STATE_SYSTEM_EXECUTION.md](STATE_SYSTEM_EXECUTION.md). It routes actual typed model values and records
an ordered token trace. It is model execution, not deployed software, a resource
simulation, a clinical process or proof that a real implementation conforms.

Repeated graph invocations with shared state are documented in
[SHARED_SYSTEM_SCENARIOS.md](SHARED_SYSTEM_SCENARIOS.md).

## Reception-to-consultation walkthrough

1. Open **System architecture → File → Install connection example**. Existing
   fixture packages remain immutable; their original labels describe the static
   exercise in which they were introduced. The new runner can execute those same
   pure record boundaries without rewriting their versions.
2. Click **Check connections**. Reception's encounter output matches Consultation's
   input. The mismatch example still fails and cannot execute.
3. Choose the **Execution** perspective, then **Prepare execution**. The studio
   resolves and freezes exact behavior bundles, checks supported features,
   derives a deterministic task order and identifies external inputs.
4. The form shows only Reception's encounter input. Consultation receives its
   value from the handoff link, so it has no separate input form.
5. Click **Use example inputs** or enter `encounterId` and `patientId` in the
   generated record fields. All supplied identifiers here are synthetic.
6. Click **Run model**. The successful result is saved in PostgreSQL with
   its design, inputs, exact executable bundles and trace.
7. Use **Next**, **Previous** or the trace slider:

   | Step | Event | Meaning |
   | --- | --- | --- |
   | 1 | Reception started | The supplied encounter is its input |
   | 2 | Reception completed | Its pass-through output is available |
   | 3 | Transfer | The encounter moves through the handoff to Consultation |
   | 4 | Consultation started | Its input contains the transferred encounter |
   | 5 | Consultation completed | Its output contains the same encounter |

   The active task or receiving task is highlighted; transfers highlight the
   link. Click a task card to jump to its start event. The value panel shows the
   exact saved values at that step.
8. Expand **Outputs and snapshot identity** to inspect outputs, bundle hashes,
   operation counts, design/input hashes and engine version. `claimsEvaluated`
   is explicitly false: this runner does not evaluate the system contract.
9. Edit the form and run again to produce another immutable record. Input edits
   never alter the displayed saved trace. Design edits require preparing again.
10. Reload the studio and use **Saved executions** to reopen a run. **Export
    execution** downloads its portable JSON snapshot. Loading history uses the
    selected system ID; a trace from an older revision is labelled as stale.

Empty systems and new definitions remain empty. Installing or loading the
exercise is explicit. The example is a pair of record pass-through boundaries,
not the full reception workflow or a consultation implementation.

## Supported semantics

- Direct schema 2/3 pure behavior packages, including their nested composition
  dependencies and structured data contracts.
- Component definitions containing pure behavior aspects and optional data aspects.
  Internal links and system links are flattened into one task graph with distinct
  instance/slot identities. Data references are checked; they have no execution step.
- Typed `data` edges only, using behavior `outputs` → `inputs` interfaces.
- Acyclic fan-out and fan-in. Every input has exactly one linked producer or one
  external value. Missing, unknown, duplicate and overriding inputs are rejected.
- Each task executes once. Ready tasks use a stable lexical identity order;
  independent tasks are serialized. This order is not a performance or parallelism model.
- Scalar, nominal record and record-collection inputs. The UI renders scalar and
  record fields; collections use a JSON array of plain records which is converted
  to exact typed values before submission.

Rejected explicitly: cycles, timing profiles, built-in conformance adapters,
request/event edges, message/workflow aspects, multi-command state machines, non-data attachments,
unsupported provider bindings and deployment plans. Unsupported aspects are never silently skipped.

To execute a pure component model while retaining a deployment design, keep a
separate behavior-only design. The current state adapter is described in the companion guide. Later adapters will
connect delivery, workloads, live provider effects and deployment constraints.

## Bounds and failure behavior

At most 64 tasks, 256 links, 4 MiB of portable task bundles, 4096 aggregate behavior
operations and 4096 invocations are allowed. Per-component engine limits also apply.
The trace is capped at 2 MiB, the report at 4 MiB and the complete saved snapshot
at 8 MiB. The API uses the shared run-capacity guard and a 20-second request budget.

Validation/evaluation rejection returns no successful partial report and does not
persist a successful execution record. No application I/O is performed. PostgreSQL
writes store studio evidence only. A storage failure returns an error rather than
presenting an unsaved result as persisted evidence.

## Snapshot and replay

Each stored execution contains the design and input snapshots, their content
hashes, the resolved reference report, the exact portable plan, engine version,
typed outputs and ordered events. Existing rows are insert-only. The history list
returns up to 50 summaries; it does not delete older snapshots.

The Go `internal/systemrun.Run(ctx, snapshot.Plan, snapshot.Inputs)` entry point can
replay the portable plan without querying the installed catalog. It re-derives
and validates order and required inputs rather than trusting saved scheduling
metadata. Regression tests delete the catalog before replaying and compare the
entire resulting report. **Verify replay** now provides this comparison through
the UI/API and retains separate immutable verification evidence; follow
[SYSTEM_REPLAY.md](SYSTEM_REPLAY.md). New snapshots also retain plan/report hashes.
Unsupported future engines remain unverified; exported snapshots are not signed provenance.

## API

| Route | Purpose |
| --- | --- |
| `POST /api/systems/execution/prepare` | Body `{design}`; returns the checked plan, external input descriptions and resolved identities |
| `POST /api/systems/execution/run` | Body `{design, inputs}`; resolves afresh, executes and saves one immutable snapshot |
| `GET /api/systems/executions?systemId=…` | Latest 50 execution summaries for that design |
| `GET /api/systems/executions/:id` | Exact saved execution |

Each input set has `taskId` and a `values` map of existing studio `TypedValue`
objects. Use task identities returned by preparation. Callers cannot supply a
replacement plan, schedule or database URL through the run endpoint.

## Progress and manual checks

Roadmap v1 stays at **28% (+0 points)**; M1 remains **80%**. Pure execution is a
completed part of its final checkpoint, but broader aspect integration and live
provider bindings remain unfinished. No checkpoint weight or scope was changed.

Browser/mobile verification is still required for the walkthrough, input error
highlighting, trace stepping, stale-snapshot notices and history after reload.
Automated tests cover the engine, API, storage, typed-value form conversion and
portable replay; they do not establish visual UX quality.
