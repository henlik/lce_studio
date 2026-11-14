# Check connections between reusable components

Engineering → Design → System architecture now supports typed output-to-input
connections. It checks dependency integrity and declared port types before
execution. These static checks do not implement clinical workflows. Pure behavior
graphs can now run separately using [SYSTEM_EXECUTION.md](SYSTEM_EXECUTION.md).

## Reception-to-consultation exercise

1. Open **Engineering → Design → System architecture**.
2. In **File**, choose **Install connection example**. This explicitly installs
   three reusable synthetic behavior packages and opens a Reception → Consultation
   design. Empty projects remain empty; this exercise is never seeded automatically.
   Installation is atomic and idempotent. Existing conflicting versions are not replaced.
3. Click **Check connections**. The summary reads **Declared connection types
   checked**, and the encounter handoff reads **checked**.
4. Click the handoff on the canvas. In its inspector, see Reception's
   `root / outputs / encounter` linked to Consultation's
   `root / inputs / encounter`. Both use the exact `Encounter` record in
   `example.encounter-contract@1.0.0` with the same digest.
5. Expand **Dependency evidence** in the summary. The report includes behavior
   roots and embedded data contracts, with instance paths, versions and digests.
6. In **File**, choose **Install mismatch example**, then **Check connections**.
   Consultation now references package version `2.0.0`, whose encounter contract
   is version `2.0.0`. Even though the record fields have the same shape, the
   nominal types differ. The handoff becomes red with a `targetPort` issue.
7. Click the issue. The inspector opens and scrolls to that connection. To repair
   it, select Consultation's component card and choose the installed
   `example.consultation-boundary@1.0.0` definition. Check again: the existing
   named endpoints resolve and the connection passes.
8. To test multiple producers, add a second connection using the same source
   output and target input. Check again: the second edge is rejected. Remove it.
9. Save the design, reload it, and export/import JSON. Typed endpoint bindings are
   preserved. Saving checks structure; dependency and type checks remain explicit.

These are record pass-through boundaries, not the complete reception component,
clinical rules, patient dispatch implementation or medical validation. The example's
claim is a declared placeholder, not evaluated evidence. **Run reception conformance**
continues to support only the separate durable-intake reference experiment and
rejects this new static exercise.

## Connect your own components

1. Install reusable behavior packages or component definitions in Components.
2. Add their exact versions from the system reference library.
3. Add a connection, choose its source and target instances, and click
   **Check connections / refresh ports** in the inspector.
4. Choose an output and input. Direct behavior packages use the `root` slot.
   Unified component definitions expose their named aspect slots and interfaces.
5. Check again. Issues identify the exact instance, dependency or connection.

Port discovery works on incomplete drafts without requiring executable claims,
a deployment plan or both connection endpoints. It is a separate inspection
operation and never certifies the whole system as valid. Changing a component
reference clears the cached port list; checking reloads the current exact ports.

Attached legacy artifacts are included in dependency checking but do not become
public ports automatically. Use a component definition to give an artifact an
owned aspect slot and expose its declared interface. Data contracts and timing
profiles have no executable ports; built-in conformance adapters retain their
separate fixed semantics.

## Verdicts and boundaries

| Verdict | Meaning |
| --- | --- |
| `checked` | All declared edges have compatible typed endpoints and the inspected dependencies are valid |
| `incomplete` | References resolve, but one or more legacy edges omit both typed endpoints |
| `invalid` | Structural, reference, closure, port or type checks failed |

The existing `valid` field still permits legacy untyped diagrams to resolve.
Read `compatibility` and each connection's status before claiming typed compatibility.
No connections means there are zero edge checks; a checked result does not imply
that a useful or complete system has been designed.

Checks include exact scalar types or nominal record/collection identity (contract
ID, version, digest and record name), endpoint direction, missing ports, transitive
behavior dependencies, embedded state/message/workflow dependencies and conflicting
content for the same artifact identity across instances and attachments. An input
already bound by a manifest-internal link cannot acquire another system producer.

Not checked yet: all required inputs being supplied, control-flow sequencing,
cycles/liveness, scheduling, request/reply or event transport semantics, provider
capabilities or deployment feasibility. Pure execution is a separate operation
with stricter completeness and acyclicity checks. Connection kinds remain
intent labels. Type-compatible cycles are not a proof of executable behavior.

Limits: 64 instances, 256 connections, 32 attached artifacts per instance and
8192 inspected dependency occurrences. Existing omitted endpoint fields keep their
previous design serialization; old evidence remains readable.

## API

- `POST /api/systems/interfaces`: inspect a design's instance ports independently
  of its connections, claims and deployment; returns ports and issues, not a
  whole-system verdict.
- `POST /api/systems/resolve`: returns reference resolution, instance ports,
  dependency occurrences, per-connection checks and `compatibility`.
- `POST /api/systems/connection-example`: explicitly installs the synthetic packages
  and returns valid and mismatched designs. It does not save over a system design.

Connections retain `source` and `target` instance IDs and add optional endpoint
objects `sourcePort` and `targetPort`, each with `slot`, `interface` and `port`.
Both must be present for a typed edge. Omitting both preserves legacy diagrams.

## Progress

Roadmap v1: **28%**, up from **23%** (+5 percentage points). M1 is **80%**:
static compatibility and dependency closure are accepted; joint execution and typed
provider bindings remain its final checkpoint. This is scoped roadmap progress,
not test coverage, deployment readiness or a formal correctness proof.
