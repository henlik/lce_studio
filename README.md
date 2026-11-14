# LCE Studio

A local software engineering workspace built with **Vue 3, TypeScript, Vue Flow, Go, and PostgreSQL**. Define requirements, assemble processing workflows, place them on computing resources, simulate their behaviour, and inspect test verdicts and event traces together.

The **System design** workspace adds a shared design/reference model, four executable contract checks, and a reception-intake conformance experiment against real Go/PostgreSQL code. Load the example explicitly, validate it, and compare model and implementation evidence across seven scenarios. See the [step-by-step conformance guide](docs/SYSTEM_CONFORMANCE.md). This first adapter supports the reference topology; it does not generate arbitrary applications or replace the existing engines.

Version **0.8.1** adds a searchable left-side component palette, a right-side binding inspector, visual node/pin/feedback conventions, validation navigation, action notifications and automatic browser draft recovery with protection against conflicting edits from multiple tabs. See [visual authoring and recovery](docs/STUDIO_UX.md) for the workflow and its limits. Portable bundles and publishing to the local catalog are available; a public community marketplace is still planned.

The studio also provides **Structure / Behavior / Execution** perspectives, nested workflow stage focus, token-following replay, and a **Regional care ecosystem inspection lab**. Open the lab to run reference exercises across all implemented engines and track manual UX checks. See the [step-by-step inspection guide](docs/STAGED_INSPECTION.md). Stages organize the workflow view; executable child-workflow calls and cross-engine orchestration are not implemented.

Executable task workflows connect human/adapter tasks, exact pure decisions and owned-state commands; follow automatic branches, interrupt an active task, and retain complete experiment evidence. The **Workflows** workspace includes hospital reception and a service-desk reuse example. See [the reception replication guide](docs/HOSPITAL_RECEPTION_REPLICATION.md).

Navigation and side panels remain collapsible, diagrams can expand, and new projects/definitions start empty with examples loaded only on request. The design-context guide distinguishes primitives, components, applications and systems from their data/state/deployment views. See [DESIGN_HIERARCHY.md](docs/DESIGN_HIERARCHY.md).

The [hospital reception design guide](docs/HOSPITAL_RECEPTION_DESIGN.md) walks top-down from a reusable reception contract through queueing, identity, affiliation, clinical tasks, payment policy and dispatch to pure primitives and explicit state/adapter boundaries. The executable reference remains a bounded synthetic experiment: it does not connect to a live registry, authenticate staff, prescribe treatment, settle payments or generate a deployed hospital application.

The **Messages workspace** connects exact owned-state machines through typed routes, freezes message payloads and recipient revisions at dispatch, and tests delivery drops, rollback, lost acknowledgements and safe receipt replay. Pharmacy and school-library experiments share one reservation-to-ledger model. The delivery runner remains 0.7.0; state remains 0.6.0, pure/data remain 0.5.0 and application/deployment simulation remains 0.3.0.

The [product roadmap](docs/ROADMAP.md) describes the general-purpose engineering studio. The [recursive component architecture](docs/COMPOSITION_ARCHITECTURE.md) develops the full recommendation from UML, BPMN, C4, ERDs and flowcharts; [COMPOSITION_CONTRACT.md](docs/COMPOSITION_CONTRACT.md) defines the precise v0.4 subset. Healthcare is a reference use case, not a kernel dependency. The [component authoring guide](docs/COMPONENT_SDK.md) explains the implemented package subset and the future SDK; external executable plugins and application generation are not yet implemented.

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

Projects and immutable run snapshots are stored as PostgreSQL JSON, with indexed IDs and timestamp columns. Separate component, data, state-machine and delivery-model catalogs store immutable versions in the same database. Pure evaluations, fixture validations, state scenarios and message experiments retain their exact inputs, definitions and results. State-scenario persistence saves experimental evidence; modeled instances do not become live application sessions. Health checks return HTTP 503 when PostgreSQL is unavailable. Deleting a project retains its recorded run snapshots.

The old `STUDIO_DATA` setting and SQLite backend have been removed. Project JSON exports from the SQLite version can be imported into this version. Existing SQLite files and recorded history are not automatically migrated; retain the old database if you need that history.

This version has no accounts or authentication; use it locally. Remote access needs an authenticated reverse proxy and an appropriate deployment configuration. For a remote PostgreSQL server, use its own credentials and TLS settings in `DATABASE_URL` (for example `sslmode=verify-full`).

## First experiment: retries without duplicate writes

The studio opens with an empty project. To try the included pipeline, open **Projects** and load the retry example. It contains a client, an order service, and a ledger. Twelve requests pass through the pipeline. The first response for each request is lost after the database processes it, so the client retries.

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

These profiles describe bounded processing, storage or routing behavior. They do not run a real hospital, logistics or finance application's business logic. The separate Components workspace adds typed pure logic; data contracts are available in Data and bounded owned-state scenarios in State. Application construction remains a future milestone.

## Typed recursive components

1. Open **Components** and load the healthcare access example. Inspect its typed inputs and `permitted` Boolean output.
2. Open its nested access-policy instance, then its tenant-check instance. Each definition has its own exact package identity; the same definitions are used by the education example.
3. Evaluate the supplied inputs, change the resource tenant, and evaluate again. The policy changes from permit to deny. A missing or wrongly typed input produces an error instead of a permission decision.
4. Run the component's test cases. They include valid access, a different tenant, a different role/scope, and an inactive subject. These are examples of a deliberately small policy, not authentication or production access enforcement.
5. Create or edit a component: define scalar ports, add primitive or nested-component nodes, bind inputs and public outputs, and inspect the resulting graph. Draft changes require preparation/validation before evaluation or installation.
6. Export the complete bundle to retain every exact dependency. Install a valid bundle in the shared catalog; changing an installed definition requires a new version. Evaluation history retains the original bundle, inputs, outputs and nested execution trace.

The pure kernel supports Boolean, safe-integer and string values; identity, equality, ordering, Boolean logic, eager value selection, addition, subtraction and concatenation. It has no state, external I/O, hidden clock or scripts. Component evaluation reports functional results and operation counts, not simulated latency, hardware use or energy. Composite packages are edited/evaluated in Components; they cannot yet be placed on the application simulator canvas. Resource-profile packages remain available in Design.

Validate and test an exported bundle offline:

```sh
go run ./cmd/componentcheck --bundle path/to/bundle.json
go run ./cmd/componentcheck --test path/to/bundle.json
```

These commands perform no catalog or network lookup. Test mode returns a nonzero exit status for failed or empty test suites. See [COMPONENT_SDK.md](docs/COMPONENT_SDK.md) for authoring and [COMPOSITION_ARCHITECTURE.md](docs/COMPOSITION_ARCHITECTURE.md) for subsequent state, workflow, data, deployment and federation milestones.

## Data contracts and structured components

1. Open **Data** and choose a bundled healthcare or education fixture. Both use the same exact organization-registry contract.
2. Inspect the records, entity keys and relationship diagram. `Subjects.tenantId` and `Resources.tenantId` reference `Organizations.id`; every source must have a matching organization.
3. Validate the complete fixture, then duplicate an ID or reference a missing organization and validate again. The result identifies the row or relationship that failed. Both verdicts retain their own contract and data snapshots.
4. Create or edit a contract. Add required scalar fields, entity primary/unique keys and foreign-key relationships. Set the reverse cardinality to require at least one source or allow at most one source per target.
5. Install an exact immutable contract version or export its lock. Compare an edited version with an earlier version before upgrading consumers; compatibility is directional and does not change existing locks.
6. Open **Components**, load a structured access example, and inspect the `subject` and `resource` record inputs. Field-extraction nodes feed the existing reusable scalar policy. Change a supplied tenant or role and inspect the decision and nested trace.
7. Build new record/collection components using `record_make`, `record_get`, `collection_count`, `collection_at` and `collection_append`. Identity, equality and eager selection also support exact structured types in component schema3.

Records currently contain **required scalar fields**. Collections are ordered sequences of one exact record type. Optional/null fields, embedded nested records, live database writes, SQL generation and migrations are later increments. A valid fixture is evidence about that supplied complete snapshot; it does not certify an external database. Data-contract and component versions are separate identities with exact locks.

Offline authoring checks require no server or database:

```sh
go run ./cmd/datacheck path/to/contract.json
go run ./cmd/datacheck --lock path/to/contract-lock.json
go run ./cmd/datacheck --fixture path/to/fixture-example.json
go run ./cmd/datacheck --compare path/to/old-new-locks.json
```

Fixture files have `{contract,fixture}`; comparison files have `{old,new}` locked contracts. Invalid fixtures and potentially breaking comparisons print their verdict and exit unsuccessfully. See [DATA_AUTHORING.md](docs/DATA_AUTHORING.md) for the workflow and [DATA_CONTRACT.md](docs/DATA_CONTRACT.md) for exact semantics and limits.

## Owned state, commands and recovery

1. Open **State** and choose the pharmacy or school-library scenario. Both use the same locked reservation machine with independently owned instances.
2. Inspect durable available quantity and the volatile accepted-command counter. Bindings connect state and command inputs to an existing pure decision bundle.
3. Read the ordered commands, explicit restarts and named failure points. Run the scenario and compare the independent expected final states with the result.
4. Inspect rollback before commit, a lost reply after commit, and identical retries. Replays return the original receipt without changing current state or creating another effect intent.
5. Add or change an instance, command input, expected revision or failure point. Edit expected final values independently; empty expectations remain unverified.
6. Install/export the exact machine, export the scenario, and reopen recorded evidence. Earlier evidence retains its original definition, initialization, steps and expectations.

Effects are typed committed intentions. They do not send messages or perform external writes. Durable slots and receipts survive simulated restarts within a run; every separate run starts from its own initialization. This state model has no clock or infrastructure resource costs yet.

```sh
go run ./cmd/statecheck path/to/machine.json
go run ./cmd/statecheck --lock path/to/machine-lock.json
go run ./cmd/statecheck --run path/to/scenario-example.json
```

Scenario files have `{machine,scenario}`. See [STATE_AUTHORING.md](docs/STATE_AUTHORING.md) for the workflow and [STATE_CONTRACT.md](docs/STATE_CONTRACT.md) for exact ownership, commit, replay and recovery semantics.

## Typed messages between state owners

1. Open **Messages** and load a reference experiment. Inspect the exact reservation owner, ledger owner and typed route between them.
2. Review initial values and the ordered command, dispatch, deliver and restart steps. Dispatch freezes a recipient request; delivery attempts it without changing its payload or expected revision.
3. Run the experiment and compare the independent state and message expectations. Acknowledgement confirms a durable recipient response; its separate acceptance flag tells you whether the business request succeeded.
4. Inspect a lost acknowledgement, restart the recipient and retry the same message. The original receipt replays without repeating its state change or effect intentions.
5. Add instances of installed/imported machines, map effect payloads to command inputs, arrange the diagram and author another scenario. Downstream dispatch can reference an earlier message alias.
6. Install an immutable model version, export its exact lock and scenario, or reopen a saved evidence snapshot. Draft edits never change previously recorded results.

Messages are simulated within one bounded experiment. There is no live broker, automatic retry, hidden routing, external delivery, clock or infrastructure timing. See [DELIVERY_AUTHORING.md](docs/DELIVERY_AUTHORING.md) and [DELIVERY_CONTRACT.md](docs/DELIVERY_CONTRACT.md).

```sh
go run ./cmd/deliverycheck path/to/model.json
go run ./cmd/deliverycheck --lock path/to/model-lock.json
go run ./cmd/deliverycheck --run path/to/experiment.json
```

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
- Typed pure composite components, nested exact dependencies, graphical editing, functional evaluation, tests and immutable evaluation history.
- Atomic multi-package installation in the shared versioned catalog.
- Reusable data contracts, entity/relationship editing, complete-fixture checks and directional compatibility comparison.
- Exact record/collection ports and five structured operations in pure components.
- Owned durable/volatile state, command decisions, revision checks, idempotent receipts, explicit effect intents and deterministic recovery scenarios.
- Independent state expectations, exact machine catalog versions and immutable scenario evidence.
- Typed routes between independent state owners, frozen message envelopes, explicit delivery faults and separate acknowledgement/business outcomes.
- Graphical delivery authoring, downstream message references, independent transport expectations and immutable model/run history.
- Go unit/integration tests, seven API acceptance suites, and a GitHub Actions workflow.

## Supported models and limits

Schema1/2 executable topologies remain **single directed linear pipelines** containing one client, one or more services and one terminal database. Schema3 supports an **acyclic exclusive workflow** with one client, services, routers and one or more terminal databases. All nodes must be reachable. Services forward to one successor; routers select one weighted successor. Paths may converge without a synchronization barrier. Cycles, parallel fan-out/join, effectful business logic and imported code execution are outside these application schemas. Pure composite logic is evaluated separately in Components.

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
node scripts/acceptance-composites.mjs
node scripts/acceptance-data.mjs
node scripts/acceptance-state.mjs
node scripts/acceptance-delivery.mjs
node scripts/acceptance-reception.mjs
```

Create `studio_test` only once. Storage and API tests create a unique schema per test and clean it up afterwards; the test role needs `CREATE` permission on the test database. Missing or unreachable PostgreSQL fails these tests instead of silently skipping them. Model, functional evaluator and simulation tests can run without a database using `go test ./internal/model ./internal/behavior ./internal/state ./internal/delivery ./internal/workflow ./internal/simulation`.

Use `STUDIO_URL=http://127.0.0.1:18080` to target a separate test instance. The acceptance scripts delete their test projects afterwards; recorded runs, component/data-contract/state-machine/delivery-model versions and evaluation/validation/scenario evidence remain. Start the server with `DATABASE_URL="$TEST_DATABASE_URL"` and use a disposable database for acceptance testing. GitHub Actions provisions PostgreSQL 17 and runs all of these checks. A separate job builds the Docker image and runs all eight acceptance suites against the Compose stack.

## Repository structure

| Directory | Responsibility |
| --- | --- |
| `web/` | Vue application, graphical editor, result views |
| `internal/model/` | Versioned data model, example project, validation |
| `internal/behavior/` | Bounded deterministic pure component evaluation and tests |
| `internal/state/` | Owned-state scenarios, receipts, effect intents and restart evidence |
| `internal/workflow/` | Typed tasks, automatic routes, decision/owner calls and interrupt evidence |
| `internal/simulation/` | Deterministic event engine, metrics, assertions |
| `internal/store/` | PostgreSQL projects, catalogs and immutable simulation/evaluation/validation evidence |
| `internal/testutil/` | Isolated PostgreSQL test schemas |
| `internal/server/` | HTTP API and frontend serving |
| `cmd/studio/` | Application entry point and graceful shutdown |
| `cmd/componentcheck/` | Offline profile/bundle validation and composite test execution |
| `cmd/datacheck/` | Offline data-contract, complete-fixture and compatibility checks |
| `cmd/statecheck/` | Offline state-machine validation and scenario execution |
| `cmd/workflowcheck/` | Offline workflow validation and independent scenario execution |
| `cmd/receptionexport/` | Reproducible reception/service-desk packages and handwritten scenarios |
| `cmd/deliverycheck/` | Offline delivery-model validation and message experiments |
| `internal/delivery/` | Typed dispatch, explicit delivery and acknowledgement simulation |
| `scripts/` | Development launcher and API acceptance checks |

## Next increments

First establish explicit artifact kinds and a common parent/child design navigator for top-down and bottom-up work, as described in [DESIGN_HIERARCHY.md](docs/DESIGN_HIERARCHY.md). Then couple owned-state and message transitions to resource/failure simulation under an explicit clock and scheduling contract; add event scheduling and bounded parallel fork/join; optional and nested data types; isolated external effect adapters; application generation and measured implementation tests; model migrations and multi-user access. Each change should include a small reference scenario with an independently known outcome. See [ROADMAP.md](docs/ROADMAP.md) for the full product direction.

MIT licensed. Third-party dependencies retain their respective licences.
