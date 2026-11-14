# Verification

## Direct system catalog resolution — 14 November 2026

- System references resolve by exact kind, ID and version across component,
  data, state, message and workflow catalogs. Typed content digests and definition
  validity are checked. The built-in adapter namespace cannot be shadowed by
  installed packages.
- Tests cover all five installed kinds, missing versions, wrong kinds, ambiguous
  identities, content tampering, missing artifact error paths and unavailable
  storage. Unresolved references cannot start conformance execution; incomplete
  designs remain saveable as structurally valid drafts.
- New conformance evidence records resolved identities and digests. Older runs
  remain readable without this metadata. The reference library exposes installed
  definitions and explicit attachment to a selected component instance.
- Full Go race tests, vet, backend build and live system API acceptance passed.
  Frontend production build and all 21 frontend tests passed. Browser visual
  verification remains outstanding under the previously recorded restriction.
- This checks direct reference availability/integrity, not complete external
  dependency closure, connection type compatibility or executable provenance.

## Engineering/runtime navigation and blueprint — 14 November 2026

- Hierarchical navigation preserves all eleven existing workspace destinations,
  groups component aspects under Engineering → Components and system planning
  under Engineering → Design, and adds planned runtime/foundation destinations.
- The shared capability catalog contains 29 scoped entries: 7 available,
  9 partial and 13 planned. Each entry includes current scope, next work,
  dependencies and an acceptance condition; six milestone gates describe the
  implementation sequence. These counts do not measure product completion.
- Frontend production build and 21 frontend tests pass. New checks cover existing
  route reachability, planned-runtime non-executability, dependency integrity and
  the distinction between independent artifact editors and unified ownership.
- Existing backend behavior is unchanged. The prior full backend/conformance
  verification is recorded below. Interactive browser/mobile verification remains
  outstanding because Chromium startup was blocked by the environment's socket
  permissions in this session.

## Unified system and reception conformance increment — 14 November 2026

Verified with PostgreSQL 17.11, Go 1.25.5 and Node.js 24.22.1.

- Full `go test -race -count=1 ./...`, `go vet ./...`, and backend build passed.
- Frontend production build and all 17 frontend tests passed, including empty designs and safe import/nullable API normalization.
- Live `scripts/acceptance-system.mjs` passed: design persistence, seven scenario expectations, model/implementation verdicts, immutable evidence retrieval after design edits, and unsupported claim rejection.
- CLI execution of `examples/system/reception-intake.system.json` passed all seven scenario expectations against PostgreSQL. Deliberately faulty deduplication violates its contract; missing evidence is inconclusive.
- Browser smoke testing was attempted but Chromium startup was blocked by a socket permission restriction. No new visual/mobile-browser verification is claimed.
- CI now runs the new API acceptance script in both native and Compose jobs; remote CI status is separate from these local results.

This is the bounded reference adapter described in [SYSTEM_CONFORMANCE.md](SYSTEM_CONFORMANCE.md). Existing engine tests remain green; cross-engine execution, catalog resolution, signed implementation provenance, real process crashes and production orchestration are not supplied by this increment.

## v0.8 executable task workflows and reception reference — 12 November 2026

Verified with PostgreSQL 17.11, Go 1.25.5, Node.js 24.22.1, npm 11.9.0 and the production Vue build.

- Full `go test -race -count=1 ./...`, `go vet ./...` and backend build passed. Vue TypeScript checking and production build passed without new dependencies. The existing main JavaScript chunk advisory remains.
- All eight live API suites pass **122 acceptance groups**: the existing 101 plus 21 independent workflow/reception groups. Existing pure/data, state, delivery and timed simulation schema/digest fixtures remain valid.
- Nineteen handwritten reference scenarios pass: eighteen reception cases and one nonclinical service-desk reuse case. Forty-seven portable artifacts include raw definitions, exact locks, run files, decisions, state owners and a nominal data contract. The original EntryRoute digest remains unchanged.
- Reception normal completion requires explicit identity confirmation, current clinical release, payment disposition, an atomic owner reservation and receiving-doctor acceptance. Numeric tickets, new-patient registration, confirmed-no-affiliation, unresolved identity/affiliation, fever review, missing observations, stale release, no-doctor and reservation-denial paths are checked independently.
- Emergency interruption is checked at all eleven routine task activations. It cancels the exact task, releases a held routine reservation and waits for explicit ECU acceptance. An unavailable unit remains pending; a late physician response cannot restart routine routing. Separate arrivals can reuse a patient while retaining distinct encounter/case identities.
- Generic protocol checks cover exact dependency/type validation, rejected-event receipts, same-ID replay/conflict, structured snapshot isolation, simultaneous signal assignments, ordinary command error routes, immutable catalogs and complete failed/unverified evidence. Bound violations create no partial saved result.
- Exact limits are exercised: 256 transitions with 64 tasks and 64 event receipts succeeds; a 257th transition rejects. Shared pure/owned work reaches 4,096 operations and 160 component invocations before the next loop rejects. Context, task fan-out, package closure and complete-evidence byte limits are also tested.

### Production browser checks

- The Workflows workspace initially contained zero steps. Examples loaded only on request. Starting Interactive used root inputs with zero scripted events, paused at initial screening, then issued ticket 100 and waited for its call. Replaying the screening event left the same ticket/task active.
- An interactive emergency signal cancelled that queue task, waited for receiving acceptance and completed only after explicit synthetic ECU acceptance. Saving retained the trace as **unverified**.
- The complete ordinary scenario passed with eleven completed tasks, 152 pure operations and 24 component invocations. Changing its expected completed count to ten produced a saved failed verdict with `Expected 10, got 11.` Opening earlier passing evidence preserved the edited scenario.
- The fever scenario passed while waiting at `feverReview`, with no reservation. The post-reservation emergency scenario passed with the physician task cancelled and its late response rejected.
- Inspected EntryRoute down to its four primitive nodes. Hierarchy/inspector collapse, workflow expansion and Escape restore worked. Installed the exact workflow, exported it and reimported it through canonical server validation.
- No application-origin browser warnings or errors were observed. Mobile-specific visual verification remains incomplete because viewport resizing is not exposed by the available browser API.

### Implemented boundary

This is a bounded, deterministic task-workflow experiment runner. Interactive continuation reruns the declared event sequence from its initial owner configuration. PostgreSQL stores definitions and experiment evidence, not live patient sessions. Registry/affiliation/workforce/payment responses and professional assessments are explicit synthetic task responses. Queue numbering is scoped to a single-case experiment; doctor owners model individual slots. No live staff authentication, treatment automation, payment execution, persistent multi-patient scheduling, nested workflow calls or coupling to hardware/network/power simulation is claimed. These remain separate runtime integration work. See [TASK_WORKFLOW_CONTRACT.md](TASK_WORKFLOW_CONTRACT.md) and [HOSPITAL_RECEPTION_REPLICATION.md](HOSPITAL_RECEPTION_REPLICATION.md).

## v0.7.1 empty workspaces and canvas usability — 12 November 2026

Verified against PostgreSQL 17.11 with Go 1.25.5, Node.js 24.22.1 and npm 11.9.0.

- Full Go race tests, vet, production backend build and all seven live API suites pass: **101 acceptance groups**. Vue TypeScript checking and the production web build pass without new dependencies. The existing initial JavaScript chunk size advisory remains.
- Fresh application, component, data, state and message editors start empty. In the production browser, each definition editor's New action returned to an empty definition after explicitly loading an example. Refresh did not inject examples. An empty application project saved and reopened with zero components, connections and requirements.
- Collapsing the main navigation and both Design panels increased the measured application canvas width from 694 to 1,284 pixels at the tested desktop size. A node name survived hiding/reopening its inspector; a component operation survived changing workspaces and returning. An edited application prompted before replacement, while pristine blank drafts opened examples directly.
- Expand, zoom, Fit and Restore/Escape were exercised for application, pure component, data, state, message and deployment canvases. Expansion filled the viewport and released page scroll locking on restore. Deployment topology and geography both supported fitting; selecting a resource reopened its hidden inspector.
- The design guide displays the current artifact/view, explains top-down and bottom-up composition, and explicitly labels the unified hierarchy as planned. It does not change an artifact's kind or claim a stored parent/child tree.
- Read-only review identified and resolved scroll locking when a conditional canvas disappears, and an expanded-canvas minimum-height conflict on short mobile layouts. Mobile-specific visual verification remains incomplete because viewport resizing is not exposed by the available browser API.

This patch changes authoring defaults and presentation only. Application version is 0.7.1; delivery remains 0.7.0, state 0.6.0, pure/data 0.5.0 and timed simulation 0.3.0. Runtime schemas and canonical digest fixtures are unchanged. Component/data/state/message drafts remain separate artifacts; creating a new application project does not erase another workbench's draft or shared catalog. See [DESIGN_HIERARCHY.md](DESIGN_HIERARCHY.md) for the intended common hierarchy and current limits.

## v0.7 typed delivery between owned machines — 12 November 2026

Verified against PostgreSQL 17.11 with Go 1.25.5, Node.js 24.22.1 and npm 11.9.0.

- Full Go race tests, vet and the production backend build passed. Additional completed state-session/delivery tests passed with race detection. The Vue TypeScript and production build passed without new dependencies; Messages loads as a separate approximately 70 kB uncompressed chunk. The existing large initial JavaScript chunk advisory remains.
- Seven live API suites pass **101 acceptance groups**: 13 legacy, 11 continuum, 12 workflow, 12 scalar composite, 16 data/structured, 19 state and 18 delivery. The new suite uses independently calculated outcomes, hashes and capacity boundaries rather than copying engine results into expectations.
- The shared pharmacy/library model passes both 19-step reference scenarios at 172 primitive operations and eight component invocations. Six delivery attempts produce three recipient commits. A separate 27-step relay oracle checks downstream dispatch before upstream acknowledgement, explicit cycles, fanout, denial, immutable stale revisions and retry faults.
- Delivery checks distinguish dispatch from execution, drops from rollback, committed acknowledgement loss from acceptance, historical receipt replay from current state, and already acknowledged no-ops from additional attempts. Acknowledgement loss applies to replayed receipts too. Frozen input maps and target revisions remain unchanged across retries and redispatch conflicts.
- State sessions reuse the existing kernel, retain no growing history, isolate definitions/initial values and share conservative work, receipt and effect budgets. Both complete canonical v0.6 result digests remain unchanged after the refactor.
- Boundaries include 8 owners, 16 routes, 32 messages, 64 receipts, 128 effects, 4,096 operations/invocations, model-wide unions of 64 component and 32 data locks, 64 KiB mapped values and 2 MiB complete evidence. A 4,096-operation experiment across distinct machine sessions succeeds; the next command aborts. Failed work consumes shared reservations. Mapping and evidence amplification reject before retained copies or publication.
- Strict raw decoding checks explicit message attempt counts, safe revisions, unsupported fields, null maps and trailing input. Original raw JSON reaches Go before browser canonicalization. Model/scenario/body byte limits, cancellation and shared computation admission prevent rejected work from producing saved evidence.
- PostgreSQL checks cover an independent immutable model catalog, concurrent installation, version conflicts/capacity, complete run snapshots and newest-50 summaries. Installing delivery models does not populate other catalogs. Repeated run submissions initialize fresh simulated owners and transport state.
- The offline delivery checker validates raw models and exact locks, runs both portable examples without PostgreSQL, emits independent failed verdicts with a nonzero exit status, and explicitly labels empty expectation lists unverified.

### Production browser workflow

- Ran both reference experiments and inspected acknowledgement loss followed by receipt replay after restart. The historical recipient counter remains 11 while current state remains 10. The first envelope stays frozen at recipient revision zero.
- Added a graphical redispatch and an absent-message expectation: the resulting 20-step scenario passed with unchanged 172-operation work. New dispatch defaults reuse the existing alias/revision for the same selector; undeclared assertion aliases default to absent.
- Fractional revision text survives step reordering and blocks execution/duplication. Fractional attempt counts block export. Nonfinite coordinates remain visible across owner selection and block validation.
- Changed an expected attempt count and observed a failed saved verdict. A recipient tenant mismatch with empty assertion lists produced an unverified run showing the first message acknowledged and its business receipt denied; later stale messages remained pending.
- Opening immutable history preserved the graphical draft and hidden pending JSON. Applying JSON after a newer graphical edit prompted; cancelling preserved both. A confirmed scenario import replaced the scenario and cleared pending JSON only after canonical validation succeeded.
- Installed the original model, rejected changed content under the existing version, installed that edit as 1.0.1, and reimported the original exact 1.0.0 export. Exported/reimported the scenario and reproduced the passing result.
- Removed and graphically rebuilt the nominal notice-to-command route, then reproduced the reference verdict. Dragging the source owner changed its coordinates from (70, 100) to (124, 172), with the route following its new position.
- Imported a valid CLI-generated negative-zero lock: the browser displayed an explicit limitation and preserved the current route and coordinates. The retained draft still ran successfully. No application-origin browser warnings or errors were observed; browser-extension messages are outside this statement.

### Implemented boundary

Messages are explicit finite correctness experiments, with transport metadata assumed durable across modeled owner restarts. There is no broker, automatic retry/routing, wall clock, external message delivery, distributed transaction or generated application. Coupling owned transitions and delivery to the existing hardware/network/power simulator requires a later scheduling contract. PostgreSQL retains definitions and experiment evidence, not live business instances. Delivery/app version is 0.7.0; state remains 0.6.0, pure/data remain 0.5.0 and timed application simulation remains 0.3.0. Existing schemas/digests are unchanged. Native and Docker Compose CI run all seven acceptance suites.

The browser rejects exact locks containing negative-zero coordinates before cloning, since JavaScript JSON serialization would change their hashes. The CLI can retain those artifacts; a graphical edit requires a new version with ordinary zero coordinates. Desktop browser verification is recorded above; mobile-specific visual verification remains incomplete because viewport resizing is not exposed by the available browser API.

## v0.6 owned state, receipts and recovery — 12 November 2026

Verified against PostgreSQL 17.11 with Go 1.25.5, Node.js 24.22.1 and npm 11.9.0.

- Full Go race tests, vet and production backend build passed. The Vue TypeScript and production build passed without new dependencies. Vite reports its advisory for the approximately 531 kB uncompressed JavaScript chunk.
- Six live API suites pass 83 acceptance groups: 13 legacy, 11 continuum, 12 workflow, 12 scalar composite, 16 data/structured and 19 state groups. The state suite includes 20 independently calculated pharmacy/library reservation cases.
- Independent protocol checks distinguish accepted commits, denied receipts, pre-commit rollback, committed reply loss, identical replay, command-ID conflicts, revision conflicts, ordinary decision errors and explicit restart. Historical replies never restore their old values into current state. Same command IDs remain independent across instances.
- SHA-256 receipt fingerprints and effect identities match independently constructed references. Structured snapshots, receipts and effect payloads remain isolated from later writes and caller mutations. Volatile restart values use each instance's effective initialization, including overrides.
- Boundaries include 64 distinct component locks, 32 distinct data locks, 64 receipts, 128 intents, 4,096 operation/invocation work budgets, 64 KiB public value maps, 512 KiB machine/scenario artifacts and 2 MiB complete evidence. Failed pure evaluations consume conservative global work reservations while public counters include only completed decision traces. Amplified snapshots and mapped values reject before successful evidence is saved.
- PostgreSQL tests cover independent immutable machine versions, concurrent installs, catalog capacity, complete run snapshots and reopening persisted evidence. Repeated HTTP submissions start fresh simulations. Machine installation does not populate the separate component/data catalogs.
- Raw decoding rejects absent/null expectation counts, unsafe or fractional integers, unknown fields and trailing JSON. Mixed-case supported field names return canonical values. Both check/run endpoints apply the shared admission limit and deadline to semantic validation and execution; rejected work creates no evidence.
- The offline state checker validates raw machines and exact locks, executes both portable examples without PostgreSQL, reports failed independent expectations with complete evidence, and labels an empty expectation list unverified.

### Production browser workflow

- Ran both 18-step reference scenarios. Each passed both instance expectations with 189 primitive operations and 9 component invocations. Inspected rollback snapshots, nested pure traces, restart outcomes and replay after restart, distinguishing current volatile state from the historical receipt.
- Added a duplicate command through the scenario editor: the resulting 19-step scenario still passed with unchanged work and effect counts. Reordering a step preserved invalid local revision text; duplicating that invalid revision remained disabled. Fractional expected counts were rejected before submission.
- Changed initial availability and observed earlier evidence become stale, followed by an independently failing verdict. Removing all final expectations produced an explicitly unverified result.
- Pending JSON survived hide/reopen. Applying it after a newer graphical edit prompted before replacement; cancelling retained both drafts. Opening earlier immutable history preserved the graphical scenario name and pending JSON.
- Installed and exported a machine, rejected changed content under its existing version, installed the edit as 1.0.1, and imported the original 1.0.0 export. Exported/reimported its scenario and reproduced the passing 189-operation result through the canonical server representation.
- Authored a new integer pass-through decision with Boolean acceptance in Components, installed it, and selected its exact version in a newly created State machine. Graphically bound its result to an owned quantity and a typed effect payload. An injected lost reply followed by restart passed an independently entered quantity 7 / revision 1 / receipt 1 / effect 1 expectation. Changing the slot to volatile and the expected quantity to 0 also passed; the retained effect payload remained 7.
- Inspected the ownership diagram, read/write directions, typed controls and production layout. No application-origin browser warnings or errors were observed. Mobile-specific visual verification remains incomplete because viewport resizing is not exposed by the available browser API.

### Implemented boundary

The runner models serialized ownership and atomic commit under explicit assumptions. Effects are retained intentions with no delivery or acknowledgement. PostgreSQL stores definitions and experiment evidence, not live business instances. State execution is not yet coupled to simulated hosts, network links or power outages and does not generate or deploy applications. State/app version is 0.6.0; pure/data engines remain 0.5.0, timed application simulation remains 0.3.0, and existing schemas/digests are preserved. GitHub Actions checks native and Docker Compose execution with all six acceptance suites.

## v0.5 data contracts and structured components — 12 November 2026

Verified against PostgreSQL 17.11 with Go 1.25.5, Node.js 24.22.1 and npm 11.9.0.

- Go race tests, vet and the production backend build passed. The Vue TypeScript and production build passed without adding dependencies.
- The five live API suites cover 64 acceptance groups: 13 legacy, 11 continuum, 12 workflow, 12 scalar composite and 16 data/structured groups.
- Independent data checks cover compound-key tuple collisions, primary/unique keys, required foreign keys, reverse cardinality, join entities, cyclic relationships, closed records, directional compatibility, deterministic diagnostics and row/byte/issue limits.
- Structured checks cover all five added operations, nominal equality and selection, empty/ordered collections, invalid indexes, capacity limits, deep-copy isolation, pre-copy value budgets, trace amplification and cancellation. Legacy package digests are pinned by regression tests; schema2 cannot transport structured values through nested calls.
- Both domain wrappers match all sixteen combinations of four access conditions. Each uses eleven primitive operations and three component invocations, retaining the exact earlier scalar policy and tenant-checker dependencies.
- Portable closure checks exercise inherited record references, repeated exact data locks, conflicting locks, eight locks per definition and thirty-two distinct locks per bundle. Catalog tests cover concurrent identical/conflicting installs, atomic capacity limits, separate component/data catalogs and reopening persisted definitions.
- Valid and invalid fixture verdicts retain immutable PostgreSQL snapshots; malformed and over-budget requests do not create evidence. The offline data CLI checks strict JSON, exact locks, fixtures and directional comparisons. Both bundled complete datasets pass with three entities and six rows.
- Additional bounded composition fuzzing completed 1,119 executions without failure during runtime review.

### Production browser workflow

- Opened both data examples and recorded valid six-row fixtures. Duplicating an organization ID produced deterministic duplicate-key and missing/ambiguous foreign-key findings. Two subjects referencing the same organization violated an edited one-to-one relationship.
- Changed a row and observed earlier evidence become stale; reopened saved valid and invalid snapshots. Installing a contract succeeded, while changing an installed ID/version returned an explicit conflict. Adding a required field produced a breaking-change comparison.
- Dragged an entity, exported its exact contract and reimported it with the changed coordinates intact. Inspected all relationship labels and the corrected direction of diagram paths.
- Verified pending JSON survives hide/reopen. Applying it after newer graphical edits prompts before replacement; cancelling preserves both pending text and graphical changes.
- Evaluated the structured healthcare wrapper as both permit and tenant-mismatch denial; a missing required Boolean returned an input error. Both healthcare and school definition suites passed five cases each.
- Built a fresh collection-count component through the graphical editor, embedded the installed organization contract, wired its nominal collection input and integer output, and obtained zero for an empty collection and one for a single record. Captured the result as a test, passed that test, and installed the component. An exact typed empty-collection literal also validated and evaluated to zero.
- Checked desktop presentation and application-origin browser logs; no application-origin console warnings or errors were observed. Browser extension messages are outside this statement. Mobile-specific visual verification remains incomplete because viewport resizing is not exposed by the browser API.

### Implemented boundary

Records contain required scalar fields; collections hold ordered records of one exact nominal type. These checks do not create SQL tables, enforce live writes, authenticate users, generate applications or deploy systems. Optional/null fields, nested records, scalar arrays, state/effects and migrations remain later increments. Package schema3 adds structured pure composition; application schemas 1–3 and timed simulation engine 0.3.0 remain unchanged. The app and pure evaluator use version 0.5.0. Docker execution is checked separately by the GitHub Actions Compose job using all five acceptance suites.

## v0.4 typed recursive components — 12 November 2026

Verified against PostgreSQL 17.11 with Go 1.25.5, Node.js 24.22.1 and npm 11.9.0.

- `go test -race -count=1 ./...`, `go vet ./...`, the production Go build and Vue TypeScript/production build passed.
- All four live API suites passed: 13 legacy, 11 continuum, 12 workflow and 12 composite acceptance groups (48 total).
- Component model tests cover exact scalar representations, safe integer bounds, type/signature compatibility, nonnull collections, graph reachability, cycles, missing/tampered/extra dependency locks, immutable schema-1 digests, 128 KiB manifests, nesting depth and repeated expansion limits.
- Evaluator tests cover all eleven operations, stable topological order, independent repeated invocations, eager selection, cancellation, overflow, bounded result/test evidence and no successful partial result. Bounded composition fuzzing completed 9,715 executions in ten seconds without failure.
- Independent flat policies and all sixteen combinations of the four Boolean access conditions were compared with nested execution in both healthcare and education examples. Outputs and primitive counts agree; nested traces retain separate instance identities.
- PostgreSQL tests verify atomic installation and rollback on dependency conflicts, concurrent installs, exact closure export and immutable evaluation persistence. API checks confirm failed evaluation creates no saved success record and portable evaluation needs no catalog lookup.
- CLI checks cover legacy profile locks, portable bundles, root tests, failed/empty suites and malformed input. Both example bundles are included under `examples/composites/`.

### Reference cases

| Controlled case | Observed result |
| --- | --- |
| Matching tenant, role, scope and active subject | `permitted: true`, six primitive executions, three component invocations |
| Any one of those four conditions fails | `permitted: false`; still a successful pure evaluation |
| Missing or invalid typed input | Explicit error; no recorded evaluation |
| Both domain wrapper test suites | Five cases each pass with shared exact policy and tenant-check locks |
| Zero-node constant-output component | Zero primitive executions, one component invocation |
| Integer overflow or oversized concatenation | Explicit error, including in an unselected eager `select` input |

### Production browser workflow

- Inspected the healthcare wrapper, nested access policy and nested tenant comparison. Evaluated permit and tenant-mismatch denial; inspected six operations, three invocations and qualified trace paths. Missing input returned an error while preserving the earlier snapshot.
- Ran both domain suites: five cases passed in each. Imported a school bundle whose JSON struct fields used alternate casing; the canonical server response safely populated the editor and passed the tests. Unknown fields and invalid locks remain rejected.
- Created a string component through the graphical interface, added concatenation, bound its public input and a literal suffix, connected the public output, and obtained `Hello from LCE`. Captured the expected output as a definition test and ran it successfully.
- Switched between Design and Components without losing the draft. Installed and exported the component, rejected conflicting content under version 1.0.0, installed the edit as 1.0.1, and reimported the original export with version 1.0.0 and its bindings intact.
- Verified that input changes, definition edits and added tests mark earlier evidence as belonging to an earlier snapshot. Test verdicts are also marked stale when the definition changes.
- Unapplied advanced JSON survived hide/reopen and triggered a discard confirmation when replacing the draft. A zero-input definition with a null test-input map remained editable after adding a public input; the updated case passed. The integer editor rejected `9007199254740990.5` before numeric conversion.

Desktop browser checks use the production Vue build and real Go/PostgreSQL backend. Mobile-specific visual verification remains incomplete because the browser API does not expose viewport resizing. Docker execution is delegated to the separate GitHub Actions Compose job, which runs all four acceptance suites.

### Implemented boundary

This release verifies finite pure composition and its separate workbench. It does not add state, effects, parallel workflow execution, authentication, production authorization enforcement, nested timed deployment or application generation. Project schemas 1–3 and simulation engine 0.3.0 remain unchanged; the pure evaluator is 0.4.0. Root test verdicts are session results, while individual successful evaluations have immutable PostgreSQL history. Content digests identify exact definitions, not trusted publishers or behavior equivalence.

## v0.3 components and exclusive workflows — 12 November 2026

Verified against PostgreSQL 17.11 with Go 1.25.5, Node.js 24.22.1, npm 11.9.0 and the production Vue build.

- `go test -race -count=1 ./...`, `go vet ./...`, the Go build and Vue TypeScript/production build passed.
- All three live API suites passed: 13 legacy, 11 continuum and 12 workflow acceptance groups (36 total).
- PostgreSQL tests exercise concurrent identical/conflicting installs, version coexistence, transactional catalog capacity and reopening persisted definitions. No catalog entry can be silently overwritten through the API.
- Simulator tests include full-run schema 1/2 golden snapshots (only engine version excluded), route vectors, nested/converging workflows, retry identity, independent node resources, shared physical resources and branch-specific faults.
- The schema 3 fuzz target completed 46,212 executions in ten seconds without failure. All 30 simulator tests and three fuzz seed suites passed with race detection.
- The offline `componentcheck` command produced a digest independently checked against Python SHA-256 and rejected unknown fields, trailing JSON, null, unsupported capabilities and oversized input. This independent test is tooling evidence, not a Python runtime dependency.

### Numerical reference cases

| Controlled fixture | Result |
| --- | --- |
| Two exclusive processing paths | 27 / 49 ms latency |
| Paths converging into one store | 29 / 49 ms; one commit per request |
| Lost first response and route-stable retry | 127 / 149 ms |
| Two routers with converging intermediate paths | 41 / 47 / 61 / 67 ms |
| Mixed terminal-store deduplication policies | 20 writes, 8 duplicates, 4 deduplicated retries |
| Shared CPU contention, one then two cores | 29 ms then 18 ms p95 |

### Production browser workflow

- The bundled branching model completed 12/12 requests at 277 ms p95. Its recorded routing table showed eight edge-path requests and four cloud-path requests for weights 3:1 and seed 42.
- Editing the edge branch weight to 1 produced six requests on each branch. Routing evidence distinguishes unique requests from retry selection events and from successful completion.
- Installed and exported the bundled worker profile. Importing changed content with the same ID/version displayed a conflict; changing to version 1.0.1 installed it. A new node used its 35 ms default and rejected 41 ms outside the 30–40 ms profile interval.
- Inspected embedded profile identity, version, digest and processing/capacity/memory bounds. Removing the test node left its valid unused package lock intact.
- Removed infrastructure and ran the portable workflow: 12/12 completions, 47 ms p95, and no invented physical-resource or energy metrics. Export retained schema 3, four package locks, six nodes and the 128 MiB worker reservation.
- Reimported that JSON, enabled infrastructure again and completed 12/12 requests. Saved/reloaded projects kept their embedded versions and placements.
- Added a profile to a fresh legacy design: schema 3 was selected while preserving its original client/service/database and adding the new profile-backed node.
- Compared earlier and later saved runs: the UI reported changes in embedded profiles, application, infrastructure and placement and retained both original model hashes and results.
- Stepped recorded replay, verified the version footer and inspected automatic canvas fitting after opening a project. All six example nodes remained visible. Ordinary property changes do not reset user zoom.

No application-origin browser console errors were observed. Mobile-specific visual verification remains incomplete because the available browser API does not expose viewport resizing. Container behavior is checked by the separate GitHub Actions Compose job, which runs all three acceptance suites.

### Implemented boundary

Packages are declarative profiles of three supported kernels, not external executable plugins. Router weights choose one path per logical request; there is no parallel fan-out/join, payload-dependent business predicate, general state machine or application generation. Resource/energy predictions remain subject to the assumptions in SIMULATION.md. The hospital remains a future reference application rather than a dependency of the kernel.

## v0.2 continuum increment — 12 November 2026

Verified with PostgreSQL 17.11, Go 1.25.5, Node.js 24.22.1, npm 11.9.0, and the production Vue build on Linux amd64.

- `go test -race -count=1 ./...` passed against a real PostgreSQL database, including legacy model, engine, HTTP and storage tests.
- `go vet ./...`, the Go production build, and `npm --prefix web run build` passed.
- `scripts/acceptance.mjs` passed all 13 legacy acceptance groups; `scripts/acceptance-continuum.mjs` passed all 11 continuum groups against the built server.
- The continuum engine fuzz target completed 46,536 executions without a failing case.
- An independent acceptance review checked scheduling, outage ordering, cancelled-event horizons, memory admission, shared-link accounting, energy integration and immutable run comparisons.
- CI runs both acceptance suites in the native-build job and the Docker Compose job. Local Docker execution is unavailable; CI is the container verification path.

### Independent numerical references

These are controlled fixtures in the acceptance suite, with inputs distinct from the interactive example.

| Scenario | Result |
| --- | --- |
| Move the same application from edge to cloud | 27 ms to 117 ms latency |
| Increase shared CPU cores under contention | 302 ms to 202 ms p95 |
| Bidirectional contention on one physical link | 3999 ms p95 |
| Retry after power interruption | 87 ms completion latency |
| CPU occupancy, idle hosts and outage energy | 19000 / 3600000 Wh total |

### Browser workflow

The actual production Vue interface was exercised with the Go backend and PostgreSQL:

- Open the continuum example; inspect placements, host resources and topology.
- Run the healthy edge placement: 12/12 completions, 44 ms p95, 0.050478 Wh (rounded).
- Move service and store to the cloud host: 12/12 completions, 262 ms p95, 0.051153 Wh (rounded).
- Compare both recorded runs and verify their original placements, model hashes and metrics remain distinct.
- Add an edge power outage `[0,5000)`: 0/12 completions, 36 bounded attempts, 36 dropped transfers, and failed completion/latency requirements.
- Exceed edge memory capacity and observe the exact 4352 MiB reservation versus 4096 MiB capacity validation error.
- Edit geographic coordinates and observe the coordinate view; save, export and reimport the schema v2 model with its sites and placements intact.
- Remove infrastructure, save a legacy model, enable infrastructure again, save and reload the new deployment.
- Verify successful import clears an earlier invalid-file error. Separate curved physical-link paths make all three example links visible.

No application-origin console errors were observed. Browser automation extension logs are excluded from this statement. Desktop visual inspection passed. Mobile-specific visual QA remains incomplete: the available browser API does not expose viewport resizing, and the app correctly blocks embedding in an iframe. No security headers were relaxed for testing.

### Scope

This verifies the bounded v0.2 pipeline/resource model, not generated production applications, arbitrary plugins or clinical systems. Geographic coordinates are metadata; energy estimates are uncalibrated. See SIMULATION.md for implemented semantics and ROADMAP.md for future milestones.

## Previous PostgreSQL update

The PostgreSQL storage update was verified on 12 November 2026 using PostgreSQL 17.11, Go 1.25.5, Node.js 24.22.1, and npm 11.9.0 on Linux amd64.

## Completed checks

- `go test -race ./...` passed for model, simulation, server, and storage packages against a real PostgreSQL database. CI uses `-count=1` to prevent cached integration test results.
- `go vet ./...` passed.
- `npm --prefix web run build` passed TypeScript checks and built the Vue production assets.
- `go build -trimpath -o bin/studio ./cmd/studio` passed.
- `node scripts/acceptance.mjs` passed all 13 acceptance groups against the built Go server and a disposable PostgreSQL database.
- A server restart preserved a saved project and its full run snapshot.
- Database shutdown caused `/api/health` to return 503; restarting PostgreSQL restored HTTP 200 without restarting the Go process.
- Missing `DATABASE_URL` caused startup to fail with an explicit configuration error.
- Storage tests verified concurrent schema initialization and writes, Unicode and escaped-control-character JSON roundtrips, duplicate run rejection, chronological ordering across time-zone offsets, and the 50-summary limit without deleting older snapshots.
- The production server returned the built Vue index at `/` while API endpoints retained JSON responses.
- Engine fuzz testing completed 61,143 executions without a failing case during the initial review.

## Numerical reference cases

| Scenario | Observed result |
| --- | --- |
| Default lost-response experiment, deduplication on | 12 completions, 24 attempts, 12 writes, 12 deduplications, p95 160 ms; 3/3 requirements pass |
| Same experiment, deduplication off | 24 writes, including 12 duplicates; no-duplicate requirement fails |
| Same experiment, retries disabled | No client completions; completion and latency requirements fail |
| One request, no response loss | 60 ms latency: 15 + 5 ms processing and 40 ms round-trip link delay |
| Three closely spaced requests, capacity 1 | p95 298 ms |
| Same queue experiment, capacity 2 | p95 198 ms |

## Browser workflow (initial Vue release)

Checked the actual Vue interface connected to the Go backend: initial model rendering, validation, successful simulation, deduplication toggle and failed result, requirement threshold edits, save, independent project creation, component addition/removal, disconnected-node errors, recorded trace stepping and node highlighting, JSON export, and import of a structurally valid incomplete draft.

A draft import issue discovered during this review was fixed by adding draft-only validation to the API. Regular simulation validation remains strict. Results from earlier model snapshots are labelled when the current design changes.

## Limits of this verification

The storage update did not change the Vue interface; its browser checks above were recorded for the initial release. Browser checks used a desktop viewport; mobile-specific visual QA is not yet recorded. Docker is unavailable in the local workspace, so the GitHub Actions workflow includes a separate job that builds the image and runs the complete API acceptance suite against the Compose stack. The release remains a local single-user prototype with the linear-pipeline semantics documented in SIMULATION.md. Existing SQLite databases and their history are not automatically migrated.
