# VHDL-style source authoring experiment

Branch: `codex/vhdl-style-authoring`, based on M1 commit
`25cf7d0cc6d47799570246945e9c3001694610b4`.

This experiment tests whether engineers can assemble existing components more
comfortably through text. **LCE Assembly 0.1 is a small VHDL-inspired frontend,
not IEEE VHDL.** It compiles into the existing `SystemDesign` format and reuses
catalog resolution, provider checks, scenario execution and saved replay.
It introduces no second runtime or new command/message semantics.

M1 remains **100%**, overall roadmap **33%**. This experiment earns no additional
milestone credit. Source editing is available on this branch; browser interaction
and authoring usability still need manual evaluation.

## Do we need a new language?

Not for configuration alone: the existing system JSON already describes the
model, and the studio imports and exports it. A friendlier frontend is a UX choice.

| Option | What we gain | What we must maintain |
| --- | --- | --- |
| Existing JSON | Exact current model; established tooling; no new grammar | Verbose configuration and limited domain-specific editing assistance |
| Standard VHDL | Established syntax, types, concurrency, simulators and testbenches | Domain libraries for software transactions/messages/failures; a constrained mapping or bridge into LCE; decisions about two execution models |
| Small LCE frontend | Familiar component assembly and source diagnostics; direct lowering to the current model | A bounded grammar, compiler tests and language versioning |

VHDL can express algorithms, and it is not limited to synthesizable circuits.
GHDL describes its execution as highly concurrent and event driven, oriented
around hardware. Standard VHDL does not automatically supply LCE's particular
receipt, atomic intent, correlated acknowledgement or offline-provider contracts.
Those would need explicit modeling and integration. Using a standard language
would therefore reduce syntax/tooling work, but would not remove domain work.

Sources, reviewed 15 November 2026:

- [GHDL: language and execution model](https://ghdl.github.io/ghdl/about)
- [GHDL: analyze, elaborate and run](https://ghdl.github.io/ghdl/quick_start/simulation/index.html)
- [IEEE: VHDL scope](https://standards.ieee.org/ieee/1076/12535/)

Recommendation for the experiment: keep the existing model authoritative and
try the small frontend. Treat choosing a permanent language as a later decision
based on authoring experience. Standard VHDL remains an integration option,
particularly for hardware/software co-simulation; this branch does not implement
or benchmark a GHDL bridge.

## Try it in the studio

1. Start the studio from this branch using the normal README instructions.
2. Open **Engineering → Design → System architecture → Source**.
3. Export the current design if you want to preserve a separate copy before applying.
4. Click **Load M1 source**. The source editor receives the 31-line reservation
   assembly. This does not change the architecture canvas or install artifacts.
5. Click **Install M1 library** once. This explicitly installs the existing
   immutable M1 dependencies; it does not replace the current design.
6. Click **Compile and check**. Review the component summary and generated JSON.
   Compilation checks the syntax and system structure. Preparation resolves exact
   catalog versions, owners, routes and providers without executing a scenario.
7. Click **Apply to canvas**. The architecture now contains Stock owner and
   Reservation ledger with the compiled mappings. The system ID is
   `learning.m1-reservation`, separate from the installed reference system.
8. Open **Execution → Prepare execution → Load M1 acceptance suite**.
9. Run all six cases and verify replay as described in
   [M1 manual acceptance](M1_MANUAL_ACCEPTANCE.md).
10. Return to Source. Change a provider target to `"missing"`, then compile.
    Click its diagnostic to select the corresponding source location. The canvas
    must stay unchanged and Apply must remain disabled.
11. Replace `port map` with `generic map`. Compilation must reject the unsupported
    construct explicitly. Restore it and compile again.
12. Download the `.lce` file, reload the browser and inspect source draft recovery.
    Open the downloaded source again to check file import.

The compiled M1 plan is checked for exact equality with the installed reference
plan. The six scenarios must produce identical reports and replay successfully.
All dependency installation is explicit. Compilation does not write to catalogs,
execute source code, connect to providers or evaluate contract claims.

## What the example configures

The source declares provider instances, component versions and their bindings.
`port map` in this prototype maps **provider requirements** to providers. This
is an LCE meaning; it is not VHDL signal association. Data-port wiring has an
explicit `connect` statement. The grammar keeps owner mappings and correlated
handoffs visible because they affect execution semantics.

The Stock component already owns its behavior, data, state, messages and workflow.
Its initial stock of five comes from the pinned immutable definition. A source
`generic map(initial_stock => 5)` is not implemented: supporting generics requires
an explicit parameter contract and deterministic specialization in the shared
model, not merely accepting another textual keyword.

The current example is [m1-reservation.lce](../examples/source/m1-reservation.lce).
A shorter declaration does not remove the need to design reusable components;
it removes navigation between their already defined interfaces and bindings.

## Supported grammar

Keywords are case insensitive. IDs, names, versions and descriptions are quoted
JSON strings, preserving their case. `--` starts a comment outside strings.
The parser accepts one complete system, with no trailing source.

```text
system "id" version "1.0.0" is
  name "Display name";
  provider "provider-id" : "modeled-state" version "1.0.0";
begin
  "instance-id" : component "namespace.definition" version "1.0.0" is
    name "Display name";
    port map ("state/slot" => "provider-id");
    entry "slot" => "command";
    messages "message-slot" owner "actor" => "instance"."state-slot";
    workflow "workflow-slot" owner "owner" => "instance"."state-slot";
    workflow "workflow-slot" handoff "task-node" from "command-node"
      via "channel-instance"."message-slot"."route"
      response map ("response-field" => "recipient-output");
  end component;
  connect "link-id" : "source"."slot"."interface"."port"
    => "target"."slot"."interface"."port";
  claim "claim-id" : "required_evidence" description "Declared requirement";
  assume "assumption-id" : "Declared precondition";
end system;
```

This grammar illustration is not itself an installed example. Components need
only the declarations relevant to their owned aspects. `connect` creates a typed
**data** connection. Interfaces and exact value types are resolved from catalog
artifacts, not guessed by the parser. Missing/unsupported versions and mappings
are rejected by the shared validators.

The source omits an explicit contract identity: it derives `<system-id>.contract`
and uses the system revision. This rule is part of frontend version 0.1. Additional
capability requirements, arbitrary artifact attachments, deployment declarations,
new component bodies, primitives, generics, nested system definitions and text
scenario/testbench authoring are outside this experiment. Unsupported statements
fail rather than silently disappear.

## CLI and API

```sh
go run ./cmd/sourcecheck examples/source/m1-reservation.lce > /tmp/m1.system.json
# Or read source from stdin:
go run ./cmd/sourcecheck < examples/source/m1-reservation.lce
```

The CLI checks syntax and local model structure, emitting system JSON on success.
It does not open a database or check whether the referenced catalog exists.
Errors go to stderr with line/column and return a nonzero exit code.

- `GET /api/systems/source/example` returns the version label and source.
- `POST /api/systems/source/compile` accepts only `{ "source": "..." }`.
- Compilation returns `language`, `valid`, `diagnostics`, `locations`, and a
  `design` only on success. Syntax/model errors return `valid:false` with HTTP 200;
  malformed HTTP request shapes return HTTP 400.
- Feed a successful design to the existing `/api/systems/execution/prepare` API
  for catalog-aware checks. Run via the existing scenario endpoint.

Source is bounded to 64 KiB and 8,192 tokens; the existing model/execution limits
still apply. The parser has no file imports, network access, shell execution,
user-defined evaluators or unbounded recursive grammar. It is covered by malformed
input tests and a fuzz target.

## Editing and evidence boundaries

- Source is an independently autosaved draft and can be downloaded as `.lce`.
- Compile and check never replace the canvas. Applying is an explicit action.
- A change to the source or the canvas invalidates approval to apply the previous
  checked result. Check again before applying.
- Canvas edits do not regenerate source. There is no bidirectional editing,
  formatting-preserving roundtrip or automatic merge in this prototype.
- Saved execution evidence contains the compiled model and pinned plan, as before.
  The `.lce` text and its comments are not attached to or hashed into execution
  evidence yet. Export source alongside evidence when inspecting this experiment.
- Diagnostics point to the nearest known declaration. An error within an embedded
  catalog workflow can fall back to the system header; it is not a complete language
  server. The editor is a plain textarea with diagnostic selection.

## Acceptance and next decision

Automated checks cover bounded parsing, syntax errors, duplicate declarations,
source locations, HTTP request validation, missing catalogs, wrong owner bindings,
exact plan/report equivalence, six M1 cases and saved replay. Frontend helpers
check Unicode selection and stale source/canvas guards.

Manual observations to record:

| Question | Observation |
| --- | --- |
| Can the reservation system be assembled with fewer navigation steps? | Pending |
| Are provider maps and owner/handoff declarations understandable? | Pending |
| Do diagnostics make correction easier? | Pending |
| Does explicit apply prevent accidental replacement? | Pending |
| Do source export and autosave recovery work comfortably? | Pending |
| Are source-to-evidence provenance and generics necessary next? | Pending |

If this authoring style helps, the next slice should be typed component parameters
and a text testbench format, backed by the existing model. New behavior-body syntax
and bidirectional editing should follow only after defining their exact semantics.
