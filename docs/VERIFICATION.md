# Verification

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
