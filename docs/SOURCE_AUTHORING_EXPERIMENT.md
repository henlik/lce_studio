# LCE source authoring experiment

Branch: `codex/vhdl-style-authoring`, based on M1 commit
`25cf7d0cc6d47799570246945e9c3001694610b4`.

**LCE Source 0.2** is a bounded, VHDL-inspired frontend, not IEEE VHDL. It accepts
either a complete scalar component with an optional testbench, or an assembly
of installed component definitions. It lowers to the existing data, behavior,
state and system model. It introduces no second execution engine.

Start with the [step-by-step component guide](COMPONENT_SOURCE_GUIDE.md) and
[complete stock example](../examples/source/stock-reservation.lce). The earlier
[M1 assembly example](../examples/source/m1-reservation.lce) remains supported
without source changes. Its prepared plan and all six scenario reports remain
equivalent to the reference M1 system.

M1 remains **100%**, overall roadmap **33%**. This authoring experiment earns no
new milestone credit. Browser interaction and authoring usability still need
manual evaluation.

## Why a small frontend?

JSON remains the existing model representation. Standard VHDL brings established
types, concurrent simulation and tooling, but integrating it still requires
software-domain semantics for transactions, receipts, messages and failures.
This experiment tests a direct frontend that removes repetitive declarations
and navigation while keeping the current model authoritative. It does not
implement or benchmark a GHDL bridge.

Background references reviewed 15 November 2026:
[GHDL execution model](https://ghdl.github.io/ghdl/about),
[GHDL quick start](https://ghdl.github.io/ghdl/quick_start/simulation/index.html),
[IEEE VHDL scope](https://standards.ieee.org/ieee/1076/12535/).

## Component grammar

```text
component "namespace.id" version "1.0.0" is
  name "Display name";
  description "Optional description";
  author "Author";
  license "MIT";
  state available : integer := 5 durable;
  event reserved (quantity : integer, remaining : integer);
  command reserve (quantity : integer) returns (remaining : integer) is
    accept when quantity > 0 and quantity <= state.available;
    let taken := select(accepted, quantity, 0);
    set available := state.available - taken;
    return remaining := next.available;
    emit reserved (quantity => taken, remaining => next.available);
  end command;
end component;
```

The example above is a syntax illustration. The runnable stock example also
checks tenant/resource identity and nonnegative initial stock.

- Metadata, state and event declarations precede commands. Names and descriptions
  use quoted JSON strings. Local identifiers are case sensitive, start with a
  letter, and contain at most 48 letters, digits or underscores. Keywords are
  case insensitive. `--` begins a comment outside a string.
- Scalar types are `integer`, `boolean` and `string`. Integer literals range from
  −9007199254740991 to +9007199254740991. Strings use JSON escapes. State must
  explicitly say `durable` or `volatile`.
- Every command begins with one `accept when` Boolean expression. Its automatic
  public Boolean `accepted` output cannot be redeclared. Public input/output types
  are declared once. Each declared return requires exactly one assignment.
- Inputs and earlier `let` values can appear in expressions. `state.name` reads
  the prior value. `next.name` is available after that slot's single `set`.
  `accepted`, `state`, `next`, `select`, `not`, `true`, `false`, `and`, and `or`
  are reserved input/local names, including case variants.
- Expressions support parentheses, scalar literals, `+`, binary `-`, `=`, `<`,
  `<=`, `>`, `>=`, `and`, `or`, `not` and `select(condition, yes, no)`.
  Precedence, from strongest to weakest: `not`, arithmetic, comparisons, `and`,
  `or`. Unary minus is supported on integer literals only.
- Expressions are **eager**, including Boolean operators and selection. Arithmetic
  overflow is an evaluation error, not wraparound. Sanitize a quantity before
  performing arithmetic, as the runnable example does; an acceptance guard does
  not short-circuit the decision graph.
- `set` generates an accepted-value selection and a state-write binding. Denied
  commands leave state unchanged. `emit` creates an intent only on acceptance;
  each event can be emitted once per command, with its complete typed payload.
- State writes, receipt and intents follow the existing atomic command protocol.
  Accepted commands advance the revision; denial stores a receipt without advancing
  it. Duplicate requests replay a receipt; changed requests under the same ID
  conflict. These semantics are not reimplemented in the compiler.

Generation creates `<id>.data`, `<id>.state`, `<id>.<command>.decision`, and the
definition `<id>`, all at the component version. Public schemas and owned state
become data records; exact digests and decision bindings are generated. The
definition owns data and state aspects; its state commands embed the pure
decision packages. A one-instance demonstration design binds `state/owner` to
`modeled-state@1.0.0`, with the first command as the default entry.

## Testbench grammar

```text
testbench "Acceptance" for "namespace.id" is
  case "Reserve and retry" is
    initial (available => 5);
    call reserve (quantity => 2) id "first" revision 0
      expect committed outputs (accepted => true, remaining => 3);
    restart;
    call reserve (quantity => 2) id "first" revision 0
      expect replayed outputs (accepted => true, remaining => 3);
    expect state (available => 3) revision 1 receipts 1 effects 1;
    expect event reserved (quantity => 2, remaining => 3) from "first";
  end case;
end testbench;
```

The optional `initial` clause supplies all state slots and appears before steps.
Omit it to use component defaults. Each case starts a fresh instance; its steps
share that owner. Every case requires at least one step and an independent,
complete final state/revision/receipt/effect expectation.

Calls require all typed input literals, command ID, expected revision and status.
Statuses: `committed`, `denied`, `replayed`, `id_conflict`, `revision_conflict`,
`rolled_back`, `evaluation_error`. Optional `outputs (...)` compares all public
receipt outputs. Optional `fault before_commit` or
`fault after_commit_before_reply` uses the existing fault model. The latter is
asserted as `expect committed reply_lost`; receipt outputs in the engine trace
remain internal evidence, not a received reply. Omitted `reply_lost` asserts false.

Event assertions compare the complete payload and require exactly one matching
committed event under the specified command ID. Final effect counts detect extra
intents. The compiler validates expectations but never derives them from a run.
Failures retain the actual trace and map to the relevant test line. Restart
asserts `restarted` automatically. Tests cover selected schedules, not every
interleaving or a mathematical proof of the implementation.

## System assembly grammar

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

Include only declarations needed by the installed aspects. `port map` maps
provider requirements, not VHDL signals. `connect` creates a typed data connection.
Owners, handoffs, types and provider capability checks use the existing catalog.
The contract identity is `<system-id>.contract` at the system revision. Declared
system claims are separate from scenario/testbench assertions and are not
evaluated by these model runs.

## CLI, APIs and installation

```sh
go run ./cmd/sourcecheck examples/source/m1-reservation.lce
go run ./cmd/sourcecheck examples/source/stock-reservation.lce
go run ./cmd/sourcecheck -test examples/source/stock-reservation.lce
```

One file or stdin is accepted. Assembly compilation preserves the earlier system
JSON output. Component compilation emits the full compilation result. `-test`
emits portable source, source hash, compiled artifacts, expectations and traces.
Invalid source and failed testbenches return a nonzero exit code.

| Route under `/api/systems/source` | Behavior |
| --- | --- |
| `GET /example` | Existing M1 assembly text |
| `GET /component-example` | Complete stock component and testbench text |
| `POST /compile` | Pure compilation; `{source}` only |
| `POST /testbench` | Recompile and run offline; `{source}` only |
| `POST /install` | Recompile and install generated artifacts; `{source}` only |

Compile returns `valid`, `language`, `diagnostics`, `locations`, and successful
model artifacts. Invalid syntax/model returns HTTP 200 with `valid:false` from
compile; malformed request shapes return 400. Test/install require valid
component source (422 otherwise); failed assertions are valid execution evidence
with HTTP 200 and `passed:false`. Installation conflicts/capacity return 409.

Compilation and test execution perform no I/O. Installation is explicit,
immutable and idempotent per artifact. A later installation failure may leave
earlier artifacts installed; retry is safe. Installation does not run or require
passing tests. The editor recommends running and reviewing them first.

## Limits and boundaries

- 64 KiB UTF-8 source, 8,192 tokens, expression nesting at most 32.
- At most eight state slots, eight commands and eight events; public port limits
  and all existing model bounds still apply. Each command expands to at most
  32 primitive nodes and 16 total generated outputs.
- At most 16 test cases, 64 steps per case, 256 total steps; each case retains the
  state engine's work/receipt/effect bounds. Combined case evidence is bounded to
  8 MiB. HTTP computations use the existing admission/deadline guard.
- No imports, shell execution, network operations or arbitrary evaluator code.
- Generics, nested component bodies, record expressions, broker/workflow body
  syntax, deployment syntax and bidirectional canvas/source editing are not yet
  supported. Unsupported constructs fail explicitly.
- Browser drafts save source and the last testbench report independently of the
  architecture canvas. Changed source marks previous evidence as stale. Downloads
  preserve its exact source, SHA-256 hash, artifacts and engine traces.
- Existing saved **system** execution evidence still contains the compiled model
  and pinned plan, without the source text. Download source alongside that evidence.
- Diagnostics use the closest known declaration; this is not a language server.
  The plain-text editor has no completion or formatting-preserving roundtrip.

Manual acceptance should assess whether this removes meaningful configuration
work, whether failures are easy to fix, whether version installation is clear,
and whether source/evidence survive reload comfortably. Automated compiler and
engine checks do not establish that the UI feels good.
