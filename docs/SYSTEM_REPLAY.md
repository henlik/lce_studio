# Verify a saved system execution

Replay checks whether an experiment can reproduce its entire recorded report
using the saved executable plan, inputs or scenario. It queries no component,
state, data or definition catalog. It does not execute the current editor draft.
This is model reproducibility, not verification of a deployed implementation,
system-contract evaluation, signed provenance or exhaustive concurrency analysis.

## Walkthrough

1. Open **Design → System architecture → Execution**.
2. Run a supported model or choose **Saved executions** and open an existing run.
   The pure connection example, state execution example and shared stock scenario
   all support replay. No preparation is needed to replay an existing record.
3. In **Reproduce this execution**, click **Verify replay**. The action shows its
   running status and uses the saved snapshot even if the current design differs.
4. Inspect the result:

   | Result | Meaning |
   | --- | --- |
   | Reproduced exactly | The complete new report equals the recorded report |
   | Mismatch detected | A stored consistency check failed, the saved schedule differs from its pinned graph, or the replay report differs |
   | Replay unverified | The snapshot or engine version is unsupported, or the pinned experiment cannot complete |

5. Expand the checks. For a report mismatch, inspect the JSON locations and
   recorded/replayed value previews. At most 32 differences are displayed, with
   an explicit truncation notice. The verification retains the full replay report
   when execution completed; the original report remains in its source execution.
6. Use **Export replay verification** to download the evidence, source execution
   ID, snapshot hash, verifier version and both report hashes.
7. Reload, reopen the same execution and click **Previous verifications**. Open
   any verification to inspect or export it. Each attempt is a separate immutable
   record; the latest 50 are listed without deleting older evidence.

For an important negative exercise, load the shared stock scenario, set expected
final availability to `99`, and run it. Its scenario assertions fail. Replay that
execution: **Reproduced exactly** should appear alongside **Original scenario:
expectations failed**. Reproducing a failure never changes it into a passed test.
A scenario with no assertions remains unverified even when exactly reproduced.

## Integrity and version boundary

New execution snapshots use `snapshotVersion: 1`, retaining `planHash` and
`reportHash` alongside the existing design/input hashes. These hashes use the
server's deterministic typed JSON encoding and SHA-256; they are not signatures.
A party that can replace both content and hashes can forge internal consistency.
Authentication, external attestations and a trust authority remain separate work.

The verifier first checks stored hashes and input/scenario ambiguity. It validates
the pinned definitions and re-derives the graph's order and external inputs.
Mismatch at this stage stops execution. Unsupported snapshot versions, engine
versions, modes and claim semantics remain unverified; they are not silently
migrated. The current verifier recognizes `system-pure-1`, `system-dataflow-2`
and `system-scenario-1` in their corresponding modes.

Legacy executions without `snapshotVersion`, `planHash` or `reportHash` can still
be replayed. A visible unverified check states that stored plan/report integrity
hashes are unavailable. Exact reproduction compares the full recorded report;
it does not retroactively create those missing historical hashes.

Verifier `system-replay-1` saves the source snapshot hash and result separately.
Matched, mismatched and unverified attempts are retained. A cancelled request or
storage error produces no successful response; after a lost response, inspect
previous verifications before retrying. Another retry creates a new attempt.

## API

| Route | Purpose |
| --- | --- |
| `POST /api/systems/executions/:id/replays` | Verify the server-held execution; body must be `{}` |
| `GET /api/systems/executions/:id/replays` | Latest 50 verification summaries |
| `GET /api/systems/execution-replays/:id` | Complete immutable verification |

The API accepts no replacement design, inputs, plan, expected report or provider
URL. Missing source records return 404, unsupported methods 405, malformed request
bodies 400, exhausted execution capacity 429 and interrupted work 408. Completed
negative/unverified checks return 200 with their explicit status, not success claims.

The shared capacity guard and 20-second execution deadline apply. Existing graph,
state, scenario, operation and trace bounds remain unchanged. Source snapshots
are limited to 8 MiB and saved verification evidence to 6 MiB. Export/import of
arbitrary replay snapshots and historical engine binaries are not provided.

## Progress

Roadmap v1 remains **28% overall, M1 80% (+0 points)**. Replay makes existing
evidence inspectable through the API/UI; joint message/workflow execution and
general implementation adapters are still open. No milestone weights changed.
Automated persistence/API checks cover positive and negative outcomes; browser
and mobile interaction checks remain unverified in this environment.
