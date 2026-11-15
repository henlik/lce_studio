# Create a complete component and its testbench

Use branch `codex/vhdl-style-authoring`. This guide uses the executable
[stock-reservation.lce](../examples/source/stock-reservation.lce) example.
It is a reusable resource reservation model, not production pharmacy software.

## 1. Open the component editor

Open **Engineering → Components → Component definitions → Source & testbench**.
Click **Load complete stock component**. This fills the source editor; it does
not install artifacts or replace a system design. The editor starts empty until
you load an example or open your own file.

Component source and system assembly have separate autosaved drafts. Reloading
restores the source and its last testbench evidence from this browser. Export a
`.lce` file and the evidence JSON for sharing or independent backup.

## 2. Read the component's identity and state

```text
component "learning.stock-reservation" version "1.0.0" is
  state tenantId : string := "hospitalA" durable;
  state resourceId : string := "supplyA" durable;
  state available : integer := 5 durable;
```

One instance owns one tenant/resource pair and its available stock. These values
survive a **modeled** restart. The default stock is five. Change defaults before
installing, or publish a new version after installation. A test case's `initial`
declaration changes only that case's fixture, not the reusable definition.

## 3. Read the public command

```text
command reserve (tenantId : string, resourceId : string, quantity : integer)
  returns (remaining : integer) is
```

These are the only public values you configure. The compiler generates the input
schema, output schema, decision ports and state bindings. Every command also
returns an automatic Boolean `accepted` output. Command ID and expected revision
belong to the engine's request envelope, so you do not redeclare them as inputs.

## 4. Read the behavior

```text
accept when tenantId = state.tenantId and resourceId = state.resourceId
  and state.available >= 0 and quantity > 0 and quantity <= state.available;
let taken := select(accepted, quantity, 0);
set available := state.available - taken;
return remaining := next.available;
```

`state.available` is the value before the command. `next.available` is the
proposed accepted value, or the unchanged value when denied. Assignments and
event intents commit atomically only when accepted. Denial records a receipt
without increasing the state revision. A successful command increases it once.

Expressions use the existing **eager** pure engine. `select` does not prevent
evaluation of its arguments. The example selects a safe `taken` value *before*
subtracting; even an extreme invalid negative quantity cannot overflow that
subtraction. The compiler rejects unsupported operations and type mismatches.

The tenant/resource comparisons route a request to its owner. They do not
authenticate the caller or establish authorization.

## 5. Read the event

The `reserved` declaration defines its typed payload. `emit reserved (...)`
creates one intent alongside an accepted commit. The compiler generates payload
bindings and the acceptance condition. Retries reuse the receipt and do not
create a second intent. No live broker delivery is implied; a system must bind
and orchestrate delivery separately.

## 6. Compile

Click **Compile and check**. Expand **Inspect generated artifacts and testbench**.
You should see a data contract with four records, one pure decision package,
one state machine and a definition owning the `data` and `owner` aspects.
The decision package is embedded in the state command and is also independently
installable. It is not added as a second executable task in the demonstration
system.

Compilation needs no catalog or installed example library. A syntax/type error
shows a line and column; click it to select the location. No artifacts are
installed by compilation.

## 7. Run the testbench

Click **Run testbench**. All **nine cases** should pass. Expand a case to see the
ordered calls, outcome, before/after revision and receipt outputs; then expand
its final state and committed event intents.

| Case | What it checks |
| --- | --- |
| Reserve, retry and exhaust stock | 5 → 3 → 0; duplicate call changes nothing; denial at zero |
| Invalid quantities | Insufficient, zero and extreme negative values are denied |
| Different tenant/resource | Requests for another owner cannot change this stock |
| Failure before commit | No write, receipt or event persists; retry succeeds |
| Lost reply | Commit survives restart; retry recovers the original receipt |
| Competing revision | After the first commit, a stale request is rejected |
| Changed request under the same ID | Reusing an ID for different content is rejected |
| Different domain | The same command works with a school/book fixture |
| Invalid initial stock | Negative availability is denied without mutation |

Each case starts fresh. Within one case, calls and restarts share the same owner.
The competing-revision case checks one declared ordering, not all concurrent
schedules or distributed replica consistency.

## 8. Understand a test line

```text
call reserve (tenantId => "hospitalA", resourceId => "supplyA", quantity => 2)
  id "first" revision 0 expect committed
  outputs (accepted => true, remaining => 3);
```

This submits the request with ID `first`, expects revision zero and independently
asserts a committed result with three units remaining. Repeat the same request,
ID and revision with `expect replayed` to check duplicate suppression.

`fault before_commit` models rollback. `fault after_commit_before_reply`
models a commit whose response is lost; the expected internal status is
`committed reply_lost`. The receipt in that trace is internal evidence, not a
response received by the caller. In the system execution view, the same situation
has the combined display status `reply_lost`.

The final `expect state (...) revision ... receipts ... effects ...;` checks the
complete owner state and counts. `expect event ... from "first";` checks the
specific committed payload. These expectations are handwritten, not generated
from execution results.

## 9. Make it fail deliberately

In the first call, change expected `remaining => 3` to `remaining => 4`.
Compile and run again. Compilation should succeed, while the testbench fails
at that assertion. Select the diagnostic. Restore three, compile and rerun.
You can also change an event expectation to see its payload assertion fail.

Changing source marks previous evidence as belonging to an earlier version.
It remains downloadable with the exact source, SHA-256 source hash, language
version, compiled artifacts, scenarios and engine traces that produced it.

## 10. Install and reuse

After reviewing the passing results, click **Install component version**.
The generated schema, pure decision, state machine and definition are installed
as immutable artifacts. Installation is explicit and does not itself run tests.
Reinstalling identical content is safe. Changed content under the same version
is rejected; change the component version, for example to `1.0.1`, and rerun tests.
The generated child artifacts inherit that version.

Return to **Aspects** to inspect the generated definition and open its data or
state editor. Those editors create separate drafts; graphical edits do not
rewrite the source. Go to **Design → System architecture**, refresh its component
library, and add **Reusable stock reservation** to another system.

For a ready-to-run demonstration design, open **Design → System architecture →
Source**, load the complete stock component, compile, install, and click **Apply
to canvas**. This replaces the current system draft after provider checks, so
export it first if you want a separate copy. The generated design has one subject
component and one modeled state provider. Its state entry is the first declared
command; other commands are available through explicit owner calls.

## CLI equivalent

```sh
# Emit the generated artifact pack, testbench and demonstration design.
go run ./cmd/sourcecheck examples/source/stock-reservation.lce

# Execute offline, including the exact source in the resulting evidence.
go run ./cmd/sourcecheck -test examples/source/stock-reservation.lce > stock-evidence.json
```

A syntax error or failed assertion returns a nonzero exit code. No database is
needed for these commands. Only installation needs PostgreSQL. See
[language scope and limits](SOURCE_AUTHORING_EXPERIMENT.md) before extending the
example. **M1 remains 100%; overall roadmap 33%.** Manual usability acceptance
is still a separate exercise.
