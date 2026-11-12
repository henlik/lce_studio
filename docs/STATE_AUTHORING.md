# Authoring owned state and transitions

The State workspace connects pure component decisions to an explicit owner of mutable state. Use it to answer practical questions: what changes when a command succeeds, what survives a restart, what happens when the reply is lost, and whether retrying creates another effect intent. The exact machine, scenario and API contract is in [STATE_CONTRACT.md](STATE_CONTRACT.md).

## Start with the reservation examples

The pharmacy and school-library scenarios use the same exact resource-reservation definition. Each instance owns a tenant identifier, resource identifier, durable available quantity and a volatile accepted-command counter that increments from a configurable initial baseline and resets to that baseline on restart. A pure component checks the supplied identity and requested quantity. A successful decision reserves units and produces a notice intent. A denial returns an explicit result without changing the business state.

1. Open State and choose a reference scenario. Inspect the machine identity and the exact decision bundle before running it.
2. Inspect both instances. They share one definition but have separate initial values, revisions, receipts and effects. The same command ID can be used independently on each instance.
3. Read the ordered steps and the expected final states. Commands and restarts run in listed order; there is no hidden background activity or automatic retry.
4. Run the scenario and inspect each step's before/after state, decision trace, receipt and effect intents. Compare the original returned values with the current state after a replay.
5. Change a quantity, revision, fault or initialization and rerun. Keep expectations independent: a failed expectation is useful evidence about a changed design, not an instruction to copy the observed result into the expected field.

## Define the state owner

Create a new machine ID/version or edit an exported definition. Give each owned slot a unique name, exact type, initial value and durability. Scalar, record and ordered record-collection values use the same type rules as Components and Data. A record reference includes the exact data-contract version and digest.

A durable slot preserves its last committed value across a simulated restart. A volatile slot resets to the effective initial value of that instance, including any scenario override. The machine's revision, receipts and committed effect intents are durable protocol state; they are not editable user slots. A restart never restores an old receipt's values into current state.

The scenario can override a subset of initial slot values per instance. Omitted overrides use definition defaults. This allows a common reservation machine to start with different quantities or resource identifiers without duplicating the definition.

## Connect a pure decision

A command declares its public input/output ports and contains an exact portable pure component bundle. Bind every decision input to one owned slot, one public command input, or a typed literal. The names of these sources remain distinct; an input does not implicitly shadow a state slot.

Select a Boolean decision output as the acceptance result. Map decision outputs to the slots to write and to all declared public result ports. Unwritten slots retain their values. A false acceptance value is a normal denied decision, not an execution error: all proposed writes and effects are discarded, but the denied response is recorded in a durable receipt.

An accepted decision increments the instance revision once, even if the written values happen to be unchanged. A denied decision preserves its revision. Both may create a receipt, so receipt count and revision count need not be equal.

Decisions remain pure. They receive explicit values, execute only the existing bounded operations, and return proposed values. The State engine stages the proposal and owns the commit boundary. Application deployment and business database writes are separate future adapters.

## Declare effects explicitly

An effect declaration names a typed payload. A command binds its payload fields to decision outputs and may supply another Boolean output as an emission condition. Effects are emitted only when the command is accepted and its transaction commits.

Each intent has a deterministic identity derived from the instance ID, command ID and effect name. Replaying the same committed command returns the same receipt and effect IDs without producing additional intents. The engine retains their typed payloads for inspection.

An intent is a request to do something. This release does not deliver it, send a message, update an external database or certify exactly-once execution by another service. The experiment proves only the supported intent creation and retention protocol.

## Write a recovery scenario

Each command step needs an instance, command name, command ID, expected revision and complete typed input map. The revision expresses which state the caller expected. A fresh command with an old revision produces a conflict before decision evaluation. It creates no receipt and can be resubmitted with the correct revision.

A committed command ID binds its command name, expected revision and typed inputs. An identical retry replays its historical receipt. Changing any of those fields under that ID produces an ID conflict. Fault injection is excluded from this fingerprint so a failed attempt can be retried without repeating the fault.

| Step or fault | State and receipt consequence |
| --- | --- |
| Normal accepted command | Atomically commit writes, incremented revision, receipt and enabled effects |
| Normal denied command | Store denied receipt; preserve state/revision; emit no effects |
| `before_commit` | Discard the entire staged transaction; preserve even volatile slots; store no receipt |
| `after_commit_before_reply` | Commit normally, then mark the reply as lost |
| Identical committed retry | Return original receipt; do not evaluate, change state or emit effects |
| Explicit restart | Preserve durable slots and protocol records; reset volatile slots to effective initial values |
| Ordinary decision execution error | Record an error outcome with no mutation or receipt |

A failure point does not itself restart the instance. Add a separate restart step to test that consequence. A matching retry bypasses commit faults because there is no new commit. Cancellation and execution-budget failures abort the whole run without saving partial successful evidence.

For example, reserving two of five units yields remaining quantity three. If another command later reserves the remaining three units, retrying the first command still reports its historical remaining quantity of three while current availability remains zero. This is the cached response protocol working as defined; the UI presents the historical receipt and current state separately.

## Verify and keep evidence

Add final expectations for each instance you want to verify: all owned state values, revision, receipt count and effect count. An empty expectation list is labeled unverified and never counted as passing. A scenario can intentionally contain denied commands, conflicts and rollback steps and still pass its independently specified final expectations.

The engine retains deterministic per-step state snapshots and successful pure decision traces, plus final receipts and effect intentions. Editing the machine or scenario marks earlier evidence as belonging to another snapshot. Opening history does not replace a draft; loading its snapshot is a separate action.

Install a valid machine to keep an immutable version, or export its complete lock and scenario. Each command's decision bundle and all required data locks travel with the machine. Machine installation does not modify the component or data catalogs. Changing installed content requires a new version.

## Offline checks

```sh
go run ./cmd/statecheck machine.json > machine-lock.json
go run ./cmd/statecheck --lock machine-lock.json
go run ./cmd/statecheck --run scenario-example.json
```

A scenario example contains `{machine,scenario}` with a locked machine. The CLI accepts `-` for standard input, rejects unknown/trailing/oversized JSON and needs no database or network. It prints complete scenario evidence and exits unsuccessfully if declared expectations fail. Empty expectations produce an explicitly unverified result, not a passing result.

Current bounds include eight state slots, eight commands, eight instances and sixty-four ordered steps. Values, locks, execution work, receipts, effects and evidence also have strict budgets. See the implementation contract for exact limits. This release models serialized ownership and an explicit atomic commit assumption; it does not yet couple these transitions to the continuum simulator's host, network or power model.
