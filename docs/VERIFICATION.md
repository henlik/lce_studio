# Verification

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
