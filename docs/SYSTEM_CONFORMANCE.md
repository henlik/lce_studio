# Unified system design and reception conformance

This increment connects a versioned system design, a bounded executable contract,
and a real Go/PostgreSQL reception-intake reference implementation. It is an
initial conformance experiment, not a production hospital system or a proof of
arbitrary generated software.

## What a system design contains

- A stable design ID, revision, and schema version.
- Named component instances referencing exact definition versions.
- References to component, data, state, message, and workflow artifacts.
- Connections between instances and bindings to deployment resources.
- Separate deployment plans and component placements.
- A contract with independently authored claims and declared assumptions.

The existing workspaces and their saved artifacts remain available. This schema
provides a common reference layer; it does not merge or rewrite their execution
engines. A locally valid artifact reference is not evidence that its referenced
catalog definition exists, that the implementation uses it, or that two ports
have compatible types.

## The first executable claims

| Kind | Bounded observation checked |
| --- | --- |
| `exactly_one_record` | One logical request must not produce multiple distinct encounter IDs; successful requests must have a record. Repeated observations of the same record are not duplicates. |
| `response_matches_record` | A success response identifies a matching previously observed persisted encounter for that request. |
| `no_false_success` | A success response is backed by persistence evidence for that request. |
| `required_evidence` | A request is observed and has terminal response evidence within the recorded run. |

In this first format, response matching and false-success checks share the same
observable persistence boundary. They are not independent database durability
proofs. All claims are evaluated over the run's request correlations; the schema
does not yet express arbitrary predicates, time bounds, or per-component scopes.

Verdicts are `satisfied`, `violated`, or `inconclusive`. Empty observations and
declared telemetry gaps cannot establish success. An observed duplicate remains
a violation even if subsequent telemetry is missing. Adapters must report known
loss with `evidence_gap`; a checker cannot discover every silently omitted event.
Event sequence numbers order a bounded run, not a distributed global clock.

Assumptions are declared conditions. Listing an assumption does not verify it.
Results are conditional on the reference adapter, evidence coverage, and those
conditions. This increment does not provide formal verification.

## Reference experiment

The reusable operation is: submit an intake request, persist an encounter, return
its identifier, and return that same identifier on retries. Inputs are synthetic.
The model adapter and the implementation adapter produce normalized observations
for the same claim evaluator. Their internal event sequences need not be equal.

| Scenario | Fault or input | Expected inspection |
| --- | --- | --- |
| Normal | One request | One encounter and a backed success response |
| Lost response | Discard response after persistence, then retry | Retry resolves to the existing encounter |
| Concurrent duplicates | Eight calls for one request | No duplicate records |
| Restart | Recreate the Go service instance over persistent storage | Request still resolves to its persisted encounter |
| Database unavailable | Use a closed SQL connection | No false success |
| Faulty deduplication | Use the intentionally unconstrained implementation | The uniqueness claim detects the defect |
| Missing evidence | Remove required observations | Inconclusive rather than silently successful |

The restart case recreates a Go service object; it does not kill an operating
system process or emulate power loss. The database failure case uses a closed
connection; it is not a network partition. A per-run temporary PostgreSQL schema
isolates the reference records and is removed after the experiment.

## Reproduce in the studio

1. Start the studio and PostgreSQL using the setup in [README.md](../README.md).
2. Open **System design** in the main navigation. A new design starts empty;
   example components are not inserted automatically.
3. Choose **File → Load reception example**.
4. Inspect the component cards in **Architecture**. Select a card to inspect its
   exact definition version and bindings in the canvas settings.
5. Open **Claims**. Read the four supported evaluators and the declared
   assumptions. The descriptions explain intent; the evaluator kind determines
   what is executed.
6. Open **Deployment**. Inspect the local compute resource, PostgreSQL resource,
   and component placements. These declarations do not provision infrastructure.
7. Choose **Validate**, then **Save system**. Invalid definitions are linked to
   their fields. Save stores the latest design in PostgreSQL; browser draft
   recovery also protects in-progress edits.
8. Choose **Run reception conformance**. Wait for the bounded experiment to
   finish; it needs a PostgreSQL role permitted to create an isolated schema.
9. Open **Evidence**. Compare the separate **Model** and **Go implementation**
   verdicts for each scenario and expand **Execution trace**.
10. Check that the faulty-deduplication case has a violated implementation claim
    and that the missing-evidence case is inconclusive. Their expected detection
    makes the experiment check pass; it does not make the faulty code conform.
11. Change a claim description or design name. The displayed result should be
    identified as belonging to an earlier snapshot. Use **Evidence history** to
    reopen a saved run without treating it as evidence for the edited design.

The first runner invokes real Go service methods and PostgreSQL queries. It does
not launch a separate deployed HTTP application or a subprocess.

## Run the reference implementation

Use the repository's Go toolchain and a disposable PostgreSQL database. The
database role needs permission to create and drop experiment schemas.

```sh
export DATABASE_URL='postgres://studio:studio_dev_only@127.0.0.1:5432/studio_test?sslmode=disable'
go run ./cmd/conformance
```

The command prints a JSON report for the default reference design. To evaluate a
saved design file with the reference adapter:

```sh
go run ./cmd/conformance -design examples/system/reception-intake.system.json
```

The reference adapter is a fixed implementation of reception intake. Changing a
design does not generate a new application or make arbitrary components execute.
The runner rejects unsupported topology or component-version changes; changing
descriptive labels and executable claims does not change the adapter's behavior.
Inspect the model and implementation results separately, including the negative
and incomplete-evidence cases. A negative case successfully detecting a defect
does not mean that the faulty implementation conforms.

## API and stored evidence

| Endpoint | Purpose |
| --- | --- |
| `GET /api/systems/example` | Retrieve the reference design explicitly |
| `GET /api/systems` | List saved system designs |
| `GET /api/systems/{id}` | Retrieve the latest saved design |
| `PUT /api/systems/{id}` | Validate and save a design with the matching ID |
| `POST /api/systems/validate` | Validate a design body without executing it |
| `POST /api/systems/conformance` | Run a body shaped as `{ "design": ... }` |
| `GET /api/systems/evidence?systemId={id}` | List recent evidence for a system |
| `GET /api/systems/evidence/{id}` | Retrieve one saved experiment |

The conformance API uses the server's configured PostgreSQL connection; clients
cannot supply a database connection string. An unavailable database produces an
execution error, not a fabricated simulation result.

Each saved experiment contains its design snapshot, contract, report, creation
time, and hashes of the design and contract. Later design edits do not rewrite
earlier stored evidence. The system-design record itself stores the latest saved
draft under its ID; its revision field is not an immutable release registry.
These hashes identify recorded content, not signed release authorization or
implementation provenance.

## Independent checks

```sh
go test ./internal/system
export TEST_DATABASE_URL='postgres://studio:studio_dev_only@127.0.0.1:5432/studio_test?sslmode=disable'
go test -race -count=1 ./internal/conformance
```

The external acceptance fixtures in `internal/system/acceptance_external_test.go`
are handwritten, not generated by the model or implementation adapters. They
exercise idempotent replay, duplicates despite telemetry gaps, empty evidence,
success before persistence, and cross-request evidence confusion. Database
integration checks require a working `TEST_DATABASE_URL`; missing infrastructure
must not be reported as a passing integration run.

## Deliberate limits

- No application generation, AI implementation workflow, release signing, or
  production deployment is performed.
- No federated controllers, automatic scaling, migrations, remote replication,
  or geographic/network/power fault orchestration are implemented here.
- Resource declarations do not provision PostgreSQL, Redis, brokers, or hosts.
- Evidence is scoped to this experiment; it is not a continuous runtime observer.
- The studio remains a local prototype; this work does not introduce accounts,
  production tenant authorization, or remote access security.
- Existing project schemas remain intact. This is an additive system-design
  layer, not automatic conversion of every old artifact into a unified system.
