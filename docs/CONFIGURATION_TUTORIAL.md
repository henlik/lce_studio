# Learn LCE configuration by building and testing components

Use branch `codex/vhdl-style-authoring`. The examples below are complete projects
that run with the current compiler and engines. Read the lessons in order, then
use the [language reference](CONFIGURATION_LANGUAGE.md#syntax-and-meaning) while
working on your own design. The [example index](../examples/configuration/README.md)
lists every project and its expected result.

## 1. Open a small working project

Start the studio using the [repository setup instructions](../README.md). Open
**Components → Component definitions → Configuration language**. Select
**Import folder**, then choose `examples/configuration/basics` from your checkout.
Choose that one project folder, which contains `main.lce`; the parent
`configuration` directory contains several independent projects.

Click **Compile**, then **Run all tests**. Expect **3 experiments passed**.
The explorer shows one component, **Echo**, with its source and three testbench
entries. Several experiments can share the same `testbench.lce` file.

The same project runs without the browser or PostgreSQL:

```sh
go run ./cmd/configcheck -file examples/configuration/basics/main.lce
```

Run commands from the repository root. The CLI prints a JSON report. Check its
root `passed` field and the individual `cases`; a successful compilation alone
does not mean the tests passed.

| File | Purpose |
| --- | --- |
| [main.lce](../examples/configuration/basics/main.lce) | Imports the component and its tests into the project |
| [echo/component.lce](../examples/configuration/basics/echo/component.lce) | Declares the component interface and implementation |
| [echo/testbench.lce](../examples/configuration/basics/echo/testbench.lce) | Supplies inputs and independent expected outputs |

## 2. Read the component from its interface inward

Open Echo's **Source** in the explorer. These are the important parts of the
complete file:

```lce
behavior Echo {
  behavior = {
    inputs = [{ name = "value"; type = "integer"; }];
    outputs = [{ name = "result"; type = "integer"; }];
    nodes = [{
      id = "evaluate";
      op = "identity";
      inputs = { value = { input = "value"; }; };
    }];
    bindings = { result = { node = "evaluate"; port = "result"; }; };
  };
}
```

Read it in this order:

1. `inputs` declares the public input port: an integer named `value`.
2. `outputs` declares the public output port: an integer named `result`.
3. `nodes` configures one built-in operation, `identity`, with local node ID
   `evaluate`. Identity returns its input unchanged.
4. In the node's `inputs`, the key `value` names the primitive's input port.
   `{ input = "value"; }` reads Echo's public input port. The two names happen to
   be the same here; they can be different when wired explicitly.
5. `bindings.result` exposes the local node's `result` port as Echo's public
   `result` output.

The `language` and `module` headers at the start of the actual file identify the
syntax version and reusable module. `Echo` is a declaration name. Its implicit
artifact ID is `learning.basics.echo.Echo`, and its version comes from the module.
`echo`, the import alias in `main.lce`, is only a local name for that module.

The useful connection to your VHDL experience is interface, implementation,
instances and testbench:

| Familiar idea | LCE location |
| --- | --- |
| Public ports | `behavior.inputs` and `behavior.outputs` |
| Internal implementation | `behavior.nodes` and output `bindings` |
| Reusable instance | A node with `component = ref(alias.Component);` |
| Configuration before execution | Module `parameter` and import `with` arguments |
| Testbench stimulus and assertion | Experiment `scenario` and `expect` |

Pure behavior nodes run in dependency order. Their order in the source array is
not a timeline, and `select` evaluates its inputs eagerly. Use the workflow,
state and message engines when modeling waiting, events or durable changes.
A box containing `op = "identity"` configures an existing engine primitive;
it does not define a new engine operation.

## 3. Read, break and repair the testbench

Click **Testbench · EchoPositive** in the explorer. It opens the source file where
the test is written. The **Testbench** view above the editor runs tests and displays
results; it is not the input form for `scenario` and `expect`.

```lce
experiment EchoPositive {
  target = target(subject.Echo);
  scenario = { inputs = { value = integer(23); }; };
  expect = { "outputs.result.integer" = 23; };
}
```

`subject` is the test file's import alias for `component.lce`. `target(...)` pins
the exact compiled component. `integer(23)` constructs a typed runtime input.
The expectation uses the ordinary number `23` because it compares the integer
field in the observed result:

```json
{"outputs":{"result":{"type":"integer","integer":23}}}
```

1. Change only the expected result from `23` to `24`.
2. Open the **Testbench** view and click **Run component tests**.
3. Expand EchoPositive. Expect a failed assertion: **Expected 24, Observed 23**.
   EchoZero and EchoNegative should still pass.
4. Restore the expectation to `23` and rerun. All three tests pass again.

For a ready-made failure, import
[expected-failure](../examples/configuration/expected-failure/main.lce). It compiles
successfully and has **one failing assertion**, with the same expected/observed
values. Correct `echo/testbench.lce` to `23` to make it pass.

Choose expectations from the component's intended contract before inspecting an
execution. Do not copy observed results just to make a test green. A negative
business outcome can be correct: a test expecting `allowed=false` should pass when
access is denied. That differs from an assertion expecting the wrong result.

## 4. Compose a parent and a child

Import [counter](../examples/configuration/counter/main.lce). This is the completed
**Counter → Identity** exercise from the creation walkthrough. Counter is the
starter's name: this example forwards a value; it does not count requests,
increment the value or retain state.

| Level | Source | Own testbench |
| --- | --- | --- |
| Counter | `components/Counter/component.lce` | Adjacent `testbench.lce`: 2 tests |
| Identity | `components/Counter/components/Identity/component.lce` | Adjacent `testbench.lce`: 2 tests |

Counter imports Identity and replaces its primitive node with a component call:

```lce
import "components/Identity/component.lce" as identity;

-- Inside Counter's behavior.nodes array:
{
  id = "evaluate";
  component = ref(identity.Identity);
  inputs = { value = { input = "value"; }; };
}
```

The snippet shows the relevant pieces; the linked source file contains the complete
module. Counter's public interface stays the same. Identity owns the primitive and
can be reused by another parent through an import and `ref(...)`.

1. Click Counter, then Identity. The breadcrumb records both levels.
2. Use the parent breadcrumb, **Back/Forward**, or open-file tabs to move between
   the files. **F12** on `identity.Identity` opens the referenced declaration.
3. Select Counter and open **Testbench**. With **Include subcomponents and aspects**
   checked, expect **4 experiments passed**. Unchecked, expect Counter's **2**.
4. Select Identity and run its tests. Expect **2 experiments passed**.
5. Open **Files** to see the physical folders. Closing an editor tab does not
   delete a file or a component.

A parent test verifies composition through the parent's public interface. A child
has its own independent tests. Child tests are not substitutes for parent tests.
Shared children keep one source identity even when reached from several parents;
the Components explorer describes composition, while Files shows physical storage.

To build this yourself, follow
[Create your own component hierarchy](CONFIGURATION_LANGUAGE.md#create-your-own-component-hierarchy).
**+ New component** creates an identity starter. **Extract primitive…** moves a
supported scalar operation into a child and wires the parent. The generated
TODO testbench intentionally fails until you supply a scenario and expectation.
Extraction adds a component invocation, so rerun tests that assert trace paths,
call counts or behavior near execution limits.

## 5. Navigate three levels

Click **Nested component example**, or import
[hierarchy](../examples/configuration/hierarchy/main.lce). This matches your proposed
structure: the top component has two children, and one child has its own child.

| Owner | Direct children | Own test |
| --- | --- | --- |
| Reception composition | Queue management, Patient identification | Ticket `17` and patient `PAT-001` pass through |
| Queue management | Ticket selection | Ticket `42` passes through |
| Ticket selection | Built-in identity operation | Ticket `7` passes through |
| Patient identification | Built-in identity operation | Patient `PAT-002` passes through |

Run all tests: **4 pass**. Queue with descendants runs **2**; Queue alone runs **1**.
Each component has an adjacent `testbench.lce`. The built-in operations are tested
through their owning leaf components. A fourth composition level uses the same
pattern, subject to the engine's existing depth and execution limits.

This small project teaches hierarchy and value propagation. For the reception
workflow with queue ownership, encounters, review steps and exceptional routes,
use the inspection project in lesson 7. Neither example supplies clinical judgment.

## 6. Reuse one implementation with different configuration

Import [reuse](../examples/configuration/reuse/main.lce). Its compact layout keeps
two experiments in `main.lce` and the parameterized implementation in `limit.lce`.
The language permits this; separate component/testbench files are the recommended
organization as a project grows.

```lce
import "limit.lce" as pharmacy with { capacity = 5; };
import "limit.lce" as library with { capacity = 2; };
fixture request = { inputs = { quantity = integer(3); }; };
```

| Setting | When it is supplied | Effect |
| --- | --- | --- |
| `capacity` parameter | During module specialization | Configures a distinct immutable component variant |
| `quantity` input | For an execution/scenario | Supplies the value evaluated by that variant |
| `request` fixture | During compilation | Reuses configuration data in both tests |

Run all tests: **2 pass**. The pharmacy variant permits quantity `3`; the library
variant declines it. Change the library capacity to `3` and rerun: its existing
`false` expectation must fail. Restore `2` after observing the mismatch.

The example tests only `quantity <= capacity`; it is not a complete stock policy
and does not reject negative quantities. Additional rules must be modeled and
specified in independent tests. Different module parameters produce different
specialization IDs; repeated imports with the same effective values reuse the same
specialization. Parameterized modules require explicit test modules/imports;
automatic testbench scaffolding and primitive extraction do not infer those choices.

## 7. Explore each supported design view

Load **Inspection example**, or import
[inspection](../examples/configuration/inspection/main.lce). It contains **56 artifacts
and 43 top-level experiments**; some experiments also run authored engine scenarios
or component test cases. Use the Files search box to open one feature at a time.

| What to learn | Definition file in `inspection/` | Test file to start with |
| --- | --- | --- |
| `data`: entities and relationships | `data_lce_organization_registry.lce` | `test_case20.lce`; compare invalid affiliation in `test_case22.lce` |
| `behavior`: reusable access decisions | `behavior_example_health_record_access.lce` | `test_case23.lce`; compare school reuse in `test_case24.lce` |
| Typed record interfaces | `behavior_example_structured_health_access.lce` | `test_case25.lce` |
| `state`: revisions, receipts and modeled restart | `state_lce_resource_reservation.lce` | `test_case27.lce` |
| `messages`: delivery, retry and acknowledgement | `messages_lce_reservation_delivery.lce` | `test_case29.lce` |
| `workflow`: reception stages and events | `workflow_health_reception_inspection.lce` | `test_case01.lce`; emergency in `test_case07.lce` |
| `definition`: a component's owned aspects | `definition_studio_message_stock.lce` | Test its composed system in `test_case38.lce` |
| `system`: composed logical execution | `system_m1_unified_reservation.lce` | `test_case38.lce`; lost acknowledgement in `test_case39.lce` |
| `timing`: placement and resource/network models | `timing_continuum_placement.lce` | `test_case33.lce`; power interruption in `test_case34.lce` |

The [inspection index](../examples/configuration/README.md#inspection-project-map)
groups the remaining scenarios, including rollback, review loops and cancellation.
Run all 43 before changing an example. Then change one input or expectation, run
again, and inspect its trace rather than editing many parts at once.

A `definition` groups pinned aspects; it has no execution engine of its own.
Components can own data/state/message/workflow aspects and be assembled into a
system. The current timing/resource simulator is a separate experiment from the
logical workflow/state/message engine. Geographic metadata alone does not supply
network delay; the modeled links do. These examples do not implement live
PostgreSQL/Redis/RabbitMQ adapters or autonomous production deployment.

## 8. Package, reopen and replay your design

After the passing Counter exercise:

1. Open **Packages**, set **Package ID** to `learning.counter-release` and
   **Version** to `1.0.0`, then click **Build package**. Choose an unused release
   version when repeating this with changes. Inspect its source snapshot, exact
   artifacts and test evidence, then export the package JSON for portability.
2. Click **Replay evidence**. The engine reruns the packaged source and compares
   the complete report. Running only one component is scoped evidence; building
   a release runs the full project's experiments.
3. Click **Install atomically** when you want that release in the studio catalog.
   Installation requires the studio's PostgreSQL connection. A changed immutable
   artifact version is a conflict; update the affected artifact/module versions
   before installation. Changing only the outer package version is insufficient.
4. Reload to check draft recovery. Use **Installed packages** to reopen the stored
   immutable release, or import its exported JSON. Draft recovery belongs to this
   browser; export or installation supplies a separate portable/server copy.

For the same portable package workflow without a database:

```sh
go build -o /tmp/lce-configcheck ./cmd/configcheck
/tmp/lce-configcheck -file examples/configuration/counter/main.lce -mode pack \
  -id learning.counter-release -version 1.0.0 -out /tmp/counter-package.json
/tmp/lce-configcheck -mode verify -file /tmp/counter-package.json
/tmp/lce-configcheck -mode replay -file /tmp/counter-package.json
```

The expected result is a verified package and **4 passing replayed experiments**.
Package hashes bind content; they are not signatures or authorization to deploy.
Finite tests establish the stated observations for those scenarios, not a universal
proof about an implementation or every possible environment.

## 9. Diagnose common problems

| What you see | Where to act |
| --- | --- |
| Syntax/type diagnostic | Click it to jump to source; check `=` and `;`, field names and typed values |
| Import cannot be resolved | Import the complete project folder; paths are relative to the importing file |
| Valid compilation but no tests | Import the testbench from `main.lce`; declarations alone do not provide evidence |
| Missing input or wrong type | Edit the experiment's `scenario.inputs`; `integer(23)` and `string("23")` differ |
| Expected/observed mismatch | Open the failed assertion; compare the intended contract, source behavior and fixture |
| Missing observation | Fix the expected result path; absence is never a passing match |
| New testbench fails | Replace its scenario and TODO expectation with independently chosen values |
| Old evidence after an edit | Run the current source again; prior evidence remains tied to its original snapshot |
| Structural edit says source changed | Compile the current draft; close/reopen a creation form prepared before the edit |
| Immutable version conflict | Bump the changed artifact's version and rebuild the complete release |
| Definition has no direct tests | Test its executable aspects or an assembled system |

Executable checks for the files in this tutorial run in CI:

```sh
go test ./cmd/configcheck -run TestDocumentedConfigurationExamples -count=1
```

The check expects the deliberate failure to report **24 versus 23**, then verifies
its documented repair. It checks the actual files linked here, without modifying
them. M1 remains **100%**; weighted roadmap progress remains **33%**.
