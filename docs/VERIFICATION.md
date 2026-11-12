# Verification

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
