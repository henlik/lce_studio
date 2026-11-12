# Software Studio

A local software engineering workspace built with **Vue 3, TypeScript, Vue Flow, Go, and PostgreSQL**. Define requirements, edit a service pipeline, simulate its behaviour, and inspect test verdicts and event traces together.

Version **0.1.0** is a working model simulator. The backend performs the simulation in Go; the browser edits the model and replays the resulting trace. There is no Python dependency.

## Start locally

To start the complete app and PostgreSQL 17 with Docker Compose:

```sh
docker compose up --build
```

Open `http://127.0.0.1:8080`. Compose waits for PostgreSQL to become healthy and retains its data in the `postgres-data` volume. Both published ports bind to localhost. The example database credentials are for local development.

For development with live Vue reload, install Go 1.25 or newer, Node.js 22 or newer, and npm. Start just the database, then configure the app:

```sh
docker compose up -d --wait postgres
cp .env.example .env.local
go mod download
npm --prefix web ci
npm run dev
```

Open the Vite URL printed in the terminal (normally `http://localhost:5173`). The development command reads `.env.local`, builds and starts the Go backend on `127.0.0.1:8080`, then starts Vite with `/api` proxying. Vue changes reload automatically. Restart the command after Go source changes. If `.env.local` already exists, add `DATABASE_URL` to it instead of replacing it. To use an existing PostgreSQL server, set its connection URL and omit the Compose database command.

For a production build:

```sh
npm --prefix web run build
go build -trimpath -o bin/studio ./cmd/studio
export DATABASE_URL='postgres://studio:studio_dev_only@127.0.0.1:5432/studio?sslmode=disable'
./bin/studio
```

Open `http://127.0.0.1:8080`. Run from the repository root so the default frontend path resolves. The compiled binary reads environment variables directly; it does not load `.env.local`.

The editable sample model is in `examples/retry-pipeline.json`.

## PostgreSQL storage

`DATABASE_URL` is required. On startup, the Go service connects with a bounded timeout and initializes its tables and indexes in a transaction. The configured database must already exist; its role needs permission to create tables and indexes in the selected schema. Startup initialization is serialized across processes. The connection pool allows up to 10 open connections per app process.

Projects and immutable run snapshots are stored as PostgreSQL JSON, with indexed IDs and timestamp columns. Health checks return HTTP 503 when PostgreSQL is unavailable. Deleting a project retains its recorded run snapshots.

The old `STUDIO_DATA` setting and SQLite backend have been removed. Project JSON exports from the SQLite version can be imported into this version. Existing SQLite files and recorded history are not automatically migrated; retain the old database if you need that history.

This version has no accounts or authentication; use it locally. Remote access needs an authenticated reverse proxy and an appropriate deployment configuration. For a remote PostgreSQL server, use its own credentials and TLS settings in `DATABASE_URL` (for example `sslmode=verify-full`).

## First experiment: retries without duplicate writes

The included project contains a client, an order service, and a ledger. Twelve requests pass through the pipeline. The first response for each request is lost after the database processes it, so the client retries.

1. Run the supplied project. Duplicate protection on the ledger is enabled; all three requirements should pass.
2. Select the ledger, disable duplicate protection, and run again. The no-duplicate-writes requirement should fail.
3. Restore duplicate protection, set retries to zero, and run again. The completion requirement should fail because each first response is deliberately lost.
4. Inspect verdicts, metrics, and the event trace. Use replay controls to step through the recorded events and highlight the affected components.
5. Save the project, reload it from Projects, or export its JSON. Run history retains an immutable project snapshot for every run.

The demonstration is intentionally a model of request processing. It does not connect to a real order service or database, execute imported source code, or prove that a production implementation behaves the same way.

## Included in this version

- Requirement editing with no-duplicate-write, completion, and p95-latency assertions.
- Graphical topology editing: add/delete components, connect them, move them, and inspect node and link parameters.
- Client workload, service/database processing capacities, FIFO queues, link delay and jitter.
- Timeouts, bounded retries, lost responses, and request-ID-based database deduplication.
- Deterministic virtual-time execution with a bounded event queue.
- Metrics, requirement verdicts, and recorded event replay.
- PostgreSQL project persistence and run history, plus project JSON import/export.
- Strict model validation, bounded input, same-origin browser writes, and local defaults.
- Go unit/integration tests, an API acceptance script, and a GitHub Actions workflow.

## Supported models and limits

A valid executable topology is a **single directed linear pipeline** containing exactly one client, one or more services, and one terminal database. All nodes must belong to that path. Branching, cycles, fan-out, alternate routes, standalone queue nodes, arbitrary business logic, and runtime code execution are outside v0.1.

Services and the database contain bounded processing resources and FIFO waiting queues. A service's capacity is its number of simultaneous processing slots. Responses traverse the reverse path using link delays, without additional service processing. These are explicit assumptions, not calibrated predictions for arbitrary real infrastructure.

Projects may be saved as incomplete drafts, but simulation requires a complete valid pipeline. Limits include 30 nodes, 30 edges, 50 requirements, 200 logical requests, five retries per request, 100,000 trace events, and one hour of virtual time. Exceeding execution bounds returns an error; it never silently produces a passing partial result.

See [the data and API contract](docs/CONTRACT.md) and [simulation semantics](docs/SIMULATION.md).
The first-release verification record is in [docs/VERIFICATION.md](docs/VERIFICATION.md).

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `STUDIO_ADDR` | `127.0.0.1:8080` | Go HTTP listen address |
| `DATABASE_URL` | Required | PostgreSQL connection string; see `.env.example` |
| `TEST_DATABASE_URL` | Required for storage/API tests | PostgreSQL URL for a disposable test database |
| `STUDIO_WEB_DIR` | `web/dist` | Built Vue assets |
| `STUDIO_GO_BINARY` | `go` | Optional Go executable path for `npm run dev` |
| `PORT` | `5173` | Vite port for `npm run dev` |

The development launcher optionally reads an untracked `.env.local`. Production reads environment variables directly. Run history shows the most recent 50 matching runs. Database files and local credentials are not committed to Git.

## Verification

Start a dedicated test database with the Compose PostgreSQL service:

```sh
docker compose up -d --wait postgres
docker compose exec postgres createdb -U studio studio_test
export TEST_DATABASE_URL='postgres://studio:studio_dev_only@127.0.0.1:5432/studio_test?sslmode=disable'
go test -race -count=1 ./...
go vet ./...
npm --prefix web run build
```

With a server running, execute the end-to-end API checks:

```sh
node scripts/acceptance.mjs
```

Create `studio_test` only once. Storage and API tests create a unique schema per test and clean it up afterwards; the test role needs `CREATE` permission on the test database. Missing or unreachable PostgreSQL fails these tests instead of silently skipping them. Model and simulation tests can run without a database using `go test ./internal/model ./internal/simulation`.

Use `STUDIO_URL=http://127.0.0.1:18080` to target a separate test instance. The acceptance script creates a dedicated test project and deletes that project afterwards; recorded test runs remain. Start the server with `DATABASE_URL="$TEST_DATABASE_URL"` for clean acceptance testing. GitHub Actions provisions PostgreSQL 17 and runs all of these checks. A separate job builds the Docker image and runs the same acceptance script against the Compose stack.

## Repository structure

| Directory | Responsibility |
| --- | --- |
| `web/` | Vue application, graphical editor, result views |
| `internal/model/` | Versioned data model, example project, validation |
| `internal/simulation/` | Deterministic event engine, metrics, assertions |
| `internal/store/` | PostgreSQL projects and immutable run records |
| `internal/testutil/` | Isolated PostgreSQL test schemas |
| `internal/server/` | HTTP API and frontend serving |
| `cmd/studio/` | Application entry point and graceful shutdown |
| `scripts/` | Development launcher and API acceptance checks |

## Next increments

Extend the modelling semantics deliberately: explicit message contracts and state machines; branching and routing; failure/recovery schedules; run comparison; external implementation test adapters; model migrations and multi-user access. Each change should include a small reference scenario with an independently known outcome.

MIT licensed. Third-party dependencies retain their respective licences.
