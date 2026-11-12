# LCE Studio

A local software engineering workspace built with **Vue 3, TypeScript, Vue Flow, Go, and PostgreSQL**. Define requirements, assemble processing workflows, place them on computing resources, simulate their behaviour, and inspect test verdicts and event traces together.

Version **0.3.0** adds reusable declarative component packages and exclusive branching workflows. Install parameter profiles, pin their exact definitions in projects, and route each logical request through one weighted branch. Existing deployment models still simulate shared CPU/bandwidth, scheduled outages and estimated host energy. The backend performs the simulation in Go; the browser edits models and replays traces. There is no Python dependency. Existing schema1/2 projects retain their pipeline semantics.

The [product roadmap](docs/ROADMAP.md) describes the general-purpose engineering studio. Healthcare is a reference use case, not a kernel dependency. The [component authoring guide](docs/COMPONENT_SDK.md) explains the implemented package subset and the future SDK; external executable plugins and application generation are not yet implemented.

## Start locally

To start the complete app and PostgreSQL 17 with Docker Compose:

```sh
docker compose up --build
```

Open `http://127.0.0.1:8080`. Compose waits for PostgreSQL to become healthy and retains its data in the `postgres-data` volume. Both published ports bind to localhost. The example database credentials are for local development.

Compose uses the explicit subnet `10.203.0.0/24` to avoid Docker's `all predefined address pools have been fully subnetted` error on hosts with many networks. If this range overlaps another Docker network, your LAN, or a VPN, choose an unused private subnet and set `STUDIO_DOCKER_SUBNET` in your shell or Compose's `.env` file. For example, after checking that `10.204.0.0/24` is available:

```sh
STUDIO_DOCKER_SUBNET=10.204.0.0/24 docker compose up --build
```

If this project's network already exists, run `docker compose down` before changing its subnet, then start it again. This preserves the database volume. See Docker's [Compose IPAM configuration](https://docs.docker.com/reference/compose-file/networks/#ipam).

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

Projects and immutable run snapshots are stored as PostgreSQL JSON, with indexed IDs and timestamp columns. The component catalog stores immutable package versions in the same database. Health checks return HTTP 503 when PostgreSQL is unavailable. Deleting a project retains its recorded run snapshots.

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

## Deployment experiment: edge or cloud

1. Load the **Continuum example** and open **Deployment**. The client runs on a device; the service and durable store run on an edge host. The remote host and three direct physical links are available for comparison.
2. Inspect site coordinates, host cores/memory/power, physical-link bandwidth/latency, and application placements. Run the healthy baseline.
3. Move the service and database placements to the cloud host and run again. Compare the recorded results; they retain their own model snapshots and metrics.
4. Restore edge placements and add a power outage on the edge host, such as `[0,5000)` ms. Run again to see unavailable transfers, bounded retries and failed requirements.
5. Change host cores, physical bandwidth or network outage windows and rerun. These inputs affect processing admission and communication rather than merely changing diagram labels.
6. Save/export the project. `examples/continuum-placement.json` is also available for import.

The topology and geographic views are schematics. Coordinates and host tiers do not infer latency or processor speed. Host energy is an uncalibrated estimate based on idle/active wattage and CPU occupancy. All defined powered hosts contribute energy, including unused hosts. Battery discharge, solar generation, migration and multihop routing are future milestones.

## Reusable components and exclusive workflows

1. Open the **Branching example** from Projects. Both processing branches reuse the same worker and store packages, with complete definitions embedded in the project.
2. Run the example and inspect `route_selected` events. Each request follows one branch; retries keep that branch. This is weighted dispatch, not simultaneous execution of every outgoing connection.
3. Select a router's outgoing connection and change its weight. Repeat with the same seed to compare the resulting request choices and resource effects.
4. Open the component catalog in Design. Install a bundled profile or import a raw component package JSON, then add it to a design. Its defaults and allowed ranges appear in the inspector.
5. Export the project. Its exact component definitions travel with it, so a different studio can simulate it without installing those versions in its catalog first.

Engineers can start with the raw manifests in `examples/components/`. Validate a new profile offline using `go run ./cmd/componentcheck path/to/package.json`. Installation rejects unsupported kernels/capabilities and conflicts with an existing ID/version; changing a package requires a new version. See [the authoring guide](docs/COMPONENT_SDK.md).

These profiles describe bounded processing, storage or routing behavior. They do not run a real hospital, logistics or finance application's business logic. Typed data contracts, arbitrary component behavior and application construction are future milestones.

## Included in this version

- Requirement editing with no-duplicate-write, completion, and p95-latency assertions.
- Graphical topology editing: add/delete components, connect them, move them, and inspect node and link parameters.
- Client workload, service/database processing capacities, FIFO queues, link delay and jitter.
- Timeouts, bounded retries, lost responses, and request-ID-based database deduplication.
- Deterministic virtual-time execution with a bounded event queue.
- Metrics, requirement verdicts, and recorded event replay.
- PostgreSQL project persistence and run history, plus project JSON import/export.
- Deployment editing with geographic sites, host resources, direct physical links and node placements.
- Shared host CPU capacity, static resident-memory admission and FIFO physical-link bandwidth.
- Scheduled power/network outages, interrupted processing, retry effects and durable completed writes.
- Per-host estimated energy, uptime and busy core time; physical-link transfer/drop counters; recorded-run comparison.
- Reusable declarative service/store/router profiles with versioned catalog installation and offline author validation.
- Portable component definitions and exact content locks in project exports and run evidence.
- Exclusive weighted routers, multiple terminal stores and converging paths, with deterministic retry routing.
- Strict model validation, bounded input, same-origin browser writes, and local defaults.
- Go unit/integration tests, three API acceptance suites, and a GitHub Actions workflow.

## Supported models and limits

Schema1/2 executable topologies remain **single directed linear pipelines** containing one client, one or more services and one terminal database. Schema3 supports an **acyclic exclusive workflow** with one client, services, routers and one or more terminal databases. All nodes must be reachable. Services forward to one successor; routers select one weighted successor. Paths may converge without a synchronization barrier. Cycles, parallel fan-out/join, arbitrary business logic and runtime code execution are outside v0.3.

Services and the database contain bounded processing resources and FIFO waiting queues. A service's capacity is its number of simultaneous processing slots. Responses traverse the reverse path using link delays, without additional service processing. These are explicit assumptions, not calibrated predictions for arbitrary real infrastructure.

Schema2 requires infrastructure; schema3 supports it optionally. When infrastructure is present, every node must be placed on one host to execute. Each application edge must stay on one host or cross one explicit undirected physical link. Host cores are shared across colocated processing nodes; memory is checked as a static reservation. A physical link serializes transmissions from both directions on one channel. Without infrastructure, schema3 uses node processing capacity and logical links without inventing hardware energy or physical network measurements. See [CONTINUUM_CONTRACT.md](docs/CONTINUUM_CONTRACT.md), [WORKFLOW_CONTRACT.md](docs/WORKFLOW_CONTRACT.md) and [SIMULATION.md](docs/SIMULATION.md).

Projects may be saved as incomplete drafts, but simulation requires a valid executable graph. Limits include 30 nodes, 30 edges, 50 requirements, 50 embedded component definitions, 200 logical requests, five retries per request, 100,000 trace events, and one hour of virtual time. The catalog holds up to 200 installed versions. Exceeding execution bounds returns an error; it never silently produces a passing partial result.

See [the data and API contract](docs/CONTRACT.md) and [simulation semantics](docs/SIMULATION.md).
The verification record is in [docs/VERIFICATION.md](docs/VERIFICATION.md).

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `STUDIO_ADDR` | `127.0.0.1:8080` | Go HTTP listen address |
| `DATABASE_URL` | Required | PostgreSQL connection string; see `.env.example` |
| `TEST_DATABASE_URL` | Required for storage/API tests | PostgreSQL URL for a disposable test database |
| `STUDIO_WEB_DIR` | `web/dist` | Built Vue assets |
| `STUDIO_GO_BINARY` | `go` | Optional Go executable path for `npm run dev` |
| `PORT` | `5173` | Vite port for `npm run dev` |
| `STUDIO_DOCKER_SUBNET` | `10.203.0.0/24` | Compose network subnet; set via the shell or Compose's `.env`, rather than `.env.local` |

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
node scripts/acceptance-continuum.mjs
node scripts/acceptance-workflows.mjs
```

Create `studio_test` only once. Storage and API tests create a unique schema per test and clean it up afterwards; the test role needs `CREATE` permission on the test database. Missing or unreachable PostgreSQL fails these tests instead of silently skipping them. Model and simulation tests can run without a database using `go test ./internal/model ./internal/simulation`.

Use `STUDIO_URL=http://127.0.0.1:18080` to target a separate test instance. The acceptance scripts create dedicated test projects and delete those projects afterwards; recorded test runs remain. Start the server with `DATABASE_URL="$TEST_DATABASE_URL"` for clean acceptance testing. GitHub Actions provisions PostgreSQL 17 and runs all of these checks. A separate job builds the Docker image and runs both acceptance suites against the Compose stack.

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
| `cmd/componentcheck/` | Offline package validation and portable lock generation |
| `scripts/` | Development launcher and API acceptance checks |

## Next increments

Extend the modelling semantics deliberately: typed message contracts and state machines; bounded parallel fork/join; isolated external component adapters; application generation and measured implementation tests; model migrations and multi-user access. Each change should include a small reference scenario with an independently known outcome. See [ROADMAP.md](docs/ROADMAP.md) for the full product direction.

MIT licensed. Third-party dependencies retain their respective licences.
