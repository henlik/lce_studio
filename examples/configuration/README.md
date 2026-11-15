# Configuration language examples

Start with the [hands-on tutorial](../../docs/CONFIGURATION_TUTORIAL.md), then use
[the language reference](../../docs/CONFIGURATION_LANGUAGE.md) for syntax,
constructors, API operations and limits.

Each row below is an independent project. In the studio, open **Components →
Component definitions → Configuration language → Import folder** and select that
row's folder, containing its `main.lce`. Do not import this entire examples directory
as one project. Imports keep the nested source paths.

| Order | Project | What it teaches | Expected result |
| --- | --- | --- | --- |
| 1 | [basics](basics/main.lce) | One integer identity primitive, public ports, bindings, and an adjacent testbench | 1 component; 3 tests pass |
| 2 | [expected-failure](expected-failure/main.lce) | Read and repair a deliberately wrong independent expectation | Compiles; 1 test fails, expected 24 / observed 23 |
| 3 | [counter](counter/main.lce) | Counter → Identity, source and testbench at both levels | 2 components; 4 tests pass |
| 4 | [hierarchy](hierarchy/main.lce) | Reception has Queue and Identification; Queue has Ticket | 4 components across 3 levels; 4 tests pass |
| 5 | [reuse](reuse/main.lce) | Module parameters, two specializations and a shared fixture | 2 component variants; 2 tests pass |
| 6 | [inspection](inspection/main.lce) | All eight current declaration kinds and bounded offline engines | 56 artifacts; 43 top-level experiments pass |

Counter forwards a value; its name does not imply incrementing or stored state.
The small hierarchy example demonstrates ticket and identity propagation. The
larger inspection project models reception steps and explicit external responses;
it does not make clinical decisions.

## Running locally

Run from the repository root with the Go version specified in the main README.
These source experiments do not require a running studio or database:

```sh
go run ./cmd/configcheck -file examples/configuration/basics/main.lce
go run ./cmd/configcheck -file examples/configuration/counter/main.lce
go run ./cmd/configcheck -file examples/configuration/hierarchy/main.lce
go run ./cmd/configcheck -file examples/configuration/reuse/main.lce
go run ./cmd/configcheck -file examples/configuration/inspection/main.lce
```

Each command prints a JSON report with `passed=true` and the table's case count.
To inspect the negative control using the CLI's own exit status:

```sh
go build -o /tmp/lce-configcheck ./cmd/configcheck
/tmp/lce-configcheck -file examples/configuration/expected-failure/main.lce
```

Expect `passed=false`, one assertion with expected `24` and actual `23`, and exit
status **1**. In `expected-failure/echo/testbench.lce`, change only the expectation
to `23`; the same command then passes. An execution error or invalid compilation
is not the intended failure. `go run` wraps program exit statuses; use the built
binary when you need to distinguish CLI statuses precisely.

To verify the untouched example collection, including its intentional failure:

```sh
go test ./cmd/configcheck -run TestDocumentedConfigurationExamples -count=1
```

## Component file organization

For new projects, each component owns `component.lce` and `testbench.lce` in its
folder. Nested children live in that folder's `components/` directory. The project
entry imports the testbenches explicitly; the compiler does not discover tests
by filename. Multiple experiments can share a testbench. Shared components are
imported once and referenced from multiple parents rather than copied.

The `counter` folder is a complete version of the UI creation/extraction lesson:

| Owner | Source | Testbench |
| --- | --- | --- |
| Workspace | [main.lce](counter/main.lce) | Imports both levels' tests |
| Counter | [component.lce](counter/components/Counter/component.lce) | [testbench.lce](counter/components/Counter/testbench.lce) |
| Identity | [component.lce](counter/components/Counter/components/Identity/component.lce) | [testbench.lce](counter/components/Counter/components/Identity/testbench.lce) |

The older compact `reuse` project keeps experiments in its entry file. The exported
`inspection` project uses flat filenames to make each canonical artifact easy to
locate. **Organize folders** can reorganize an imported project while preserving
artifact digests and independent expectations.

## Inspection project map

Open `inspection/main.lce` for the complete import list. Each row below links the
first test in the corresponding sequence; files are named `test_caseNN.lce`.

| Test files | Subject | Useful cases |
| --- | --- | --- |
| [01–18](inspection/test_case01.lce) | Reception workflow | Registration, no affiliation, review, emergency, unavailable registry/ECU, stale responses, capacity reservation |
| [19](inspection/test_case19.lce) | Service desk | Reuses ticket and assignment owners outside the hospital example |
| [20–22](inspection/test_case20.lce) | Data definitions | Healthcare and education fixtures; broken affiliation reference is correctly rejected |
| [23–24](inspection/test_case23.lce) | Scalar access components | Healthcare and school reuse |
| [25–26](inspection/test_case25.lce) | Structured access components | Nominal record contracts across the same two domains |
| [27–28](inspection/test_case27.lce) | Owned state | Pharmacy stock and school-library reservation scenarios |
| [29–30](inspection/test_case29.lce) | Messages | Reservation delivery and acknowledgement scenarios |
| [31–33](inspection/test_case31.lce) | Timing/resource simulation | Pipeline, exclusive routes, edge/cloud placement |
| [34–37](inspection/test_case34.lce) | Infrastructure constraints | Power loss, link loss, remote cloud, one-CPU burst |
| [38–43](inspection/test_case38.lce) | Shared M1 system | Complete journey, lost acknowledgement, stock rejection, rollback, review/duplicates, cancellation |

Definitions group aspects; test their executable aspects or their composed system.
Logical system execution and infrastructure timing are currently separate engines.
The examples do not establish a coupled logical/resource proof or deploy live
services. For implementation scope and evidence, see
[Configuration v1 acceptance](../../docs/CONFIGURATION_V1_ACCEPTANCE.md).
