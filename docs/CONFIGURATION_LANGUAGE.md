# LCE Configuration Language 1.0

The configuration language is a second authoring surface for the studio's existing
models. Vue provides the editor; Go parses, resolves, validates, executes and
packages the configuration. PostgreSQL stores installed releases and artifacts.
It is not IEEE VHDL. The older scalar component syntax remains available under
**Source & testbench**, and its component/testbench files can be migrated.

## Nested component workspace

The Source view now uses Monaco, with locally bundled CSS, icons and worker code.
It provides syntax colours, line numbers, folding, bracket matching, undo/redo,
find/replace, multiple cursors, contextual suggestions and an outline. It is an
embedded editor, not a VS Code extension host, terminal or runtime debugger.

Start with **Nested component example** for this short authoring exercise:

| Level | Component | Source and independent testbench |
|---|---|---|
| 1 | Reception composition | `reception/component.lce`, `reception/testbench.lce` |
| 2 | Queue management | `reception/components/queue/component.lce`, adjacent `testbench.lce` |
| 3 | Ticket selection | `reception/components/queue/components/ticket/component.lce`, adjacent `testbench.lce` |
| 2 | Patient identification | `reception/components/identification/component.lce`, adjacent `testbench.lce` |

1. Expand **Reception composition** in the Components explorer. Its two direct
   subcomponents are Queue management and Patient identification.
2. Click **Queue management**, then **Ticket selection**. The breadcrumbs show
   the complete path. Click a parent breadcrumb or use Back/Forward to move between
   levels. Each source file keeps its own cursor, scroll position and undo history
   while the workspace remains open. Closing a tab does not delete its file.
3. Open **Testbench** under Ticket selection in the explorer to edit its independent
   expectation: input ticket `7` must produce output ticket `7`.
4. Use the **Testbench** view and **Run component tests**. Ticket has one experiment.
   Queue has one direct experiment, or two when **Include subcomponents and aspects**
   is checked. **Run all tests** runs all four. Results name their exact scope.
5. Change Ticket's expected output from `7` to `8`. Its test must fail; restore `7`.
   No assertion is inferred from the implementation's observed result.
6. Use **Files** to inspect actual nested folders. **Organize folders** can reorganize
   older flat projects: it rewrites local imports and checks that artifact digests
   and independent expectations are unchanged. Comments and multi-declaration files
   remain intact. Shared modules have one location under `shared/`, with references
   from their parents. Project entry modules keep their entry path. Multiple tests
   can live under a component's `testbench/` folder; unrelated modules remain under
   `modules/`. Deep paths are bounded, with shared locations used when necessary.
7. An untested component shows **No tests yet · Add testbench**. This creates a
   linked file with explicit TODOs and a deliberately failing missing-observation
   assertion. Supply the scenario and expected results yourself. Definitions are
   manifests: test their executable aspects. Parameterized specializations require
   an explicit test module with the same import parameters. If the component is in
   the entry file, a separate `test-suite.lce` entry avoids a circular import.
8. Export the JSON project to retain all paths, or use the native example at
   `examples/configuration/hierarchy/main.lce` with `configcheck`.

This example demonstrates composition and ticket/identity propagation only; it is
not the complete clinical reception process. Each engine primitive remains an
operation with implemented semantics, configured and tested through its owning
component. Data/state/messages are shown as aspects, not new abstraction levels.

Draft edits automatically refresh diagnostics after a short pause. Red markers
and the Problems list navigate to the exact source. The explorer retains the last
valid structure during an incomplete edit. Source, selected file, tabs and component
context autosave; editor undo history is intentionally scoped to the current session.

## Start the review

1. Start the branch's Go server and Vue app using the README instructions. Open
   **Components → Component definitions → Configuration language**. The same
   workspace is available at **Design → System architecture → Configuration
   language**. The buttons remain visible when the ribbon is collapsed.
2. A new workspace contains an empty module. Click **Inspection example** to
   explicitly load the reference project. It contains 56 artifacts, 43 independent
   experiments, and all eight supported artifact kinds across 100 source files.
3. Click **Compile**. The left explorer has **Components** and **Files** views. In Files, select
   `workflow_health_reception_inspection.lce`, then **Canvas**. Reception starts
   with its stage overview. Select a stage to inspect its sequential flow; choose
   **All steps** to see the entire workflow. **Manual positions** permits dragging
   nodes whose model includes a position. Hide the explorer or inspector for space.
4. Select a node, change its name in the inspector, then open **Source**. The
   corresponding property has changed in that file. Comments and unrelated
   properties remain. A value obtained from another fixture/constructor must be
   changed at its definition; the editor reports this instead of silently copying it.
5. In Source, put the cursor on an imported symbol and use **Go to definition**
   or F12. **Suggestions** / Ctrl+Space opens Monaco suggestions for symbols, constructors and the current object's typed fields; required serialized fields are labelled `required`. Select a
   declaration from the jump menu. **Format** preserves semantics and comments.
6. Select an artifact, enter a new local symbol name and click **Rename symbol
   safely**. References across files update. Implicit artifact IDs and names are
   made explicit to preserve installed identity. Changing a parameter or experiment
   name changes specialization/evidence identity and is an explicit source edit.
7. Click **Run all tests**. All 43 reference experiments should pass. An expected
   invalid data fixture passes when the data engine reports `valid=false`; that is
   a negative control. Expand a case for expected/observed values and its trace.
8. Open `test_case01.lce`. Change its expected `passed` value from true to false,
   compile and rerun. That case must fail. Restore it and rerun. Missing observed
   paths also fail; an absent value is never treated as a successful assertion.
9. Click **Build package**, then **Packages → Export package**. A package contains
   source, parameters, exact file/artifact locks, canonical models and test evidence.
   **Replay evidence** executes it afresh without relying on an installed catalog.
10. **Install atomically** writes the complete catalog closure and release to one
    PostgreSQL transaction. Repeat installation: existing exact versions are reused.
    A conflicting immutable version rejects the entire transaction. Install does
    not overwrite an open design draft. Select a system artifact in Canvas and use
    **Open in system design** to create that draft explicitly.
11. Reload the app: the source and latest evidence recover through the existing
    IndexedDB draft system. **Installed packages** reopens the immutable server
    copy. Export is the portable backup. Evidence from older source is marked stale.
12. Import an exported JSON project/package to share it. **Import folder** retains
    nested `.lce` module paths. For flat module projects, select all `.lce` files together. Import a single legacy `.lce`
    component to migrate it, preserving its state scenarios and independent
    step/output/effect assertions. Compare a previous JSON package to review
    removals, immutable conflicts, data compatibility and behavior changes.

Manual browser/mobile layout and actual drag ergonomics are reviewed together;
mounted Vue interaction tests do not establish visual usability.

## A small example of reuse

Start with `examples/configuration/reuse/main.lce` and `limit.lce`. One capacity
component is specialized for pharmacy and library use. The same input fixture is
reused with two independent expectations:

```text
language "1.0";
module "examples.reuse" version "1.0.0";
import "limit.lce" as pharmacy with { capacity = 5; };
import "limit.lce" as library with { capacity = 2; };

fixture request = { inputs = { quantity = integer(3); }; };
experiment PharmacyAccepts {
  target = target(pharmacy.Limit);
  scenario = request;
  expect = { "outputs.allowed.boolean" = true; };
}
experiment LibraryDeclines {
  target = target(library.Limit);
  scenario = request;
  expect = { "outputs.allowed.boolean" = false; };
}
```

`limit.lce` declares `parameter capacity : integer = 5;` and uses
`integer(capacity)` in a `less_or_equal` primitive. The two instances have distinct
immutable IDs derived from their effective parameter values. An explicit value
that equals the default resolves to the same specialization. This avoids copying
components and manually calculating dependency digests.

## Syntax and meaning

```ebnf
file        = "language" string ";" "module" string "version" string ";"
              { import | parameter | fixture | artifact | experiment } ;
import      = "import" string "as" identifier ["with" object] ";" ;
parameter   = "parameter" identifier ":" scalarType "=" expression ";" ;
fixture     = "fixture" identifier "=" expression ";" ;
artifact    = artifactKind identifier (object | call) [";"] ;
experiment  = "experiment" identifier (object | call) [";"] ;
expression  = literal | object | array | reference | call ;
object      = "{" { (identifier | string) "=" expression ";" } "}" ;
array       = "[" [expression {"," expression} [","]] "]" ;
reference   = identifier {"." identifier} ;
call        = reference "(" [expression {"," expression} [","]] ")" ;
```

Keywords are case-insensitive; identifiers and field names are case-sensitive.
Names begin with a letter and contain letters, digits or underscores (64 characters
maximum). Modules use namespaced IDs and exact three-part versions. Strings use
JSON escapes and UTF-8; comments begin with `--`. Literals include decimal numbers,
Booleans and null. Object fields use `=` and terminate with `;`. Unknown fields,
wrong types and duplicate fields are rejected. Null is not a scalar default.

Parameters support integer, number, Boolean and string. Runtime typed values support
integer, Boolean, string, nominal record and ordered collection. Integer values are
restricted to the existing safe range. Parameter defaults may use earlier parameters.
Declarations and imported symbols may be referenced before their textual declaration;
cycles are rejected. Imports are local to the closed project. File paths cannot leave
it. Module namespaces are unique. No network fetches, environment variables, arbitrary
code, loops or external side effects occur during compilation.

The compiler fills omitted identity/name/version metadata and omitted required empty
collections. Explicit IDs and names take precedence. Optional omitted collections
retain historic model digest behavior. Parameterized artifacts append a deterministic
specialization suffix to their IDs, including explicitly supplied IDs. A component
version is immutable: changing content requires a new version before installation.

| Declaration | Canonical model and supported semantics |
| --- | --- |
| `data` | Records, entities, keys, relationships and nominal record types |
| `behavior` | Pure primitives, typed bindings and bounded nested component composition; existing timing profiles can also be represented |
| `state` | Owned durable/volatile slots, commands, revisions, receipts, effect intents and modeled faults |
| `messages` | Exact state actors, typed routes, frozen messages, retries and acknowledgements |
| `workflow` | Tasks, branches, decision bundles, owned commands, signals and staged presentation |
| `definition` | Reusable component manifest with pinned aspects and typed internal links |
| `system` | Component instances, connections, owned aspect mappings, modeled providers and claims |
| `timing` | Existing application/deployment simulator: resource placement, workload, routes, links, sites, power/network outages and requirements |

Bodies use the existing model field names and validators. Reflection-driven field
suggestions follow those Go types. The compiler rejects missing exact dependencies
rather than borrowing arbitrary installed versions. Model diagnostics map to the
closest source property, with file, UTF-8 byte offsets, line, column, code and path.
A failure produces no runnable artifact list.

| Constructor | Result |
| --- | --- |
| `lock(A)` | Exact embedded data/behavior/state/message/workflow lock |
| `ref(A)` | Exact behavior or state reference |
| `bundle(A)` | Behavior root and its reachable dependency closure |
| `use(A)` | Behavior/component-definition reference for a system instance |
| `aspect("slot", A)` | Named exact aspect pin |
| `target(A)` | Experiment target identity and digest |
| `record(Data, "Record")` | Nominal record type reference |
| `integer(n)`, `boolean(b)`, `string(s)` | Typed scalar values |
| `value(recordRef, fieldsOrRecords)` | Typed record or collection |
| `body(A)` | Canonical artifact body for a new explicit derivative |
| `merge(base, overrides)` | Deep object merge; arrays/scalars replace |
| `get(fixture, "path")` | Object/array path selection |
| `concat(strings...)` | Configuration string concatenation |

These constructors produce configuration values. Runtime operators remain the
existing engine primitives: e.g. a behavior node with `op = "less_or_equal"`.

## Experiments and evidence

Each experiment specifies a target, a scenario and one or more expected result
paths. Array paths accept `steps[0].status` or `steps.0.status`. Equality compares
canonical JSON values. The expected value can also be an explicit `$matches` object
to count array entries matching selected fields; legacy effect assertions migrate
to this form. Missing paths/fields fail. Expectations come from source, not from a
prior execution. The packaged report retains actual observations and assertions.

| Target kind | Scenario |
| --- | --- |
| data | `DataFixture` |
| behavior | `{ inputs = ...; }` or `{ tests = true; }` for authored model tests |
| state | `StateScenario` |
| messages | `DeliveryScenario` |
| workflow | `WorkflowScenario` |
| system | Shared `systemrun.Scenario`, including workflow/state/broker steps |
| timing | `{}`; workload and infrastructure faults belong to the timing model |

A definition is an ownership manifest: test an executable aspect or its composed
system. Shared system execution uses the same resolver/planner as installed designs.
A timing experiment remains separate from the logical workflow/state/message engine.
Latitude/longitude and tiers are metadata; explicit links determine network timing.
This release does not add a coupled scheduler, replication protocol, general live
implementation adapters, clinical decisions or autonomous deployment.

Source locks bind compiler version, all source files, parameter values and exact
artifacts. A supplied lock must match; regenerating it is explicit through source
editing/recompilation/package building. Package digests detect accidental changes;
they are not signatures or release authorization. Replay recomputes every result
and compares the complete report. Behavioral compatibility is conservatively marked
for review; a passing finite testbench is not a universal proof of equivalence.

## CLI and HTTP

Run from the repository root:

```sh
go run ./cmd/configcheck -file examples/configuration/reuse/main.lce
go run ./cmd/configcheck -file examples/configuration/inspection/main.lce -mode compile
go run ./cmd/configcheck -file examples/configuration/inspection/main.lce -mode pack \
  -id examples.inspection -version 1.0.0 -out /tmp/inspection-package.json
go run ./cmd/configcheck -mode replay -file /tmp/inspection-package.json
go run ./cmd/configcheck -mode migrate -file examples/source/stock-reservation.lce \
  -out /tmp/migrated-project.json
```

The CLI reads native imports beneath the entry file's directory, including symlink
confinement. An adjacent `lock.json`, when present, is checked. JSON project bundles
work too. Exit codes: 0 successful, 1 failed expectations/compilation verdict,
2 invalid input or execution failure. `-mode example` exports the reference project.

The `/api/configuration` routes expose `compile`, `run`, `organize`, `testbench`, `patch`, `format`, `rename`,
`definition`, `complete`, `pack`, `verify`, `replay`, `compare`, `migrate`, and `install`
as POST actions. Editing takes `{project, expectedHash, file, declaration, ...}`;
stale hashes return 409. GET `example`, `packages`, and `package?id=...&version=...`
provide the example and persisted releases. Mutations retain same-origin checks.

Limits include 128 files, 512 KiB per file, 4 MiB total source, 256 module
specializations, 512 artifacts, 128 experiments and 64 assertions per experiment.
Expanded values, total resolved declarations, execution counts and evidence sizes
also have explicit bounds. Existing individual engine limits continue to apply.

## Progress

Language implementation is tracked as six acceptance gates in
[CONFIGURATION_V1_ACCEPTANCE.md](CONFIGURATION_V1_ACCEPTANCE.md).
M1 remains **100%**; overall roadmap progress remains **33%**. Authoring conveniences
and source hashes do not satisfy later runtime security or implementation gates.
