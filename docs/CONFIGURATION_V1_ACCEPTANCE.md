# Configuration Language v1 acceptance

Scope: complete configuration authoring for the existing bounded offline engines.
Live adapters, nested workflow execution, coupled logical/resource scheduling,
replication and autonomous runtime retain their existing separate roadmap gates.

The release is reviewed against six gates, not an arbitrary line-count score:

| Gate | Required evidence | State |
| --- | --- | --- |
| 1. Foundation | Versioned syntax, AST, separate resolution/validation, typed diagnostics and deterministic compilation | Automated checks passed |
| 2. Reuse | Local modules/imports, typed parameters/defaults, fixtures, pinned source/artifact closure; cycle and ambiguity rejection | Automated checks passed |
| 3. Components | Data, behavior, state, messages, workflows and definitions authored through typed configuration; reception and contrasting examples | Automated checks passed |
| 4. Systems/experiments | Systems, deployment/timing models and shared tests use existing engines with independent assertions | Automated checks passed |
| 5. Editors | Completion, symbol navigation, formatting, safe rename and conflict-checked graphical edits preserve semantics/comments | Automated checks passed |
| 6. Packages | Portable provenance, compatibility review, migration and all-or-nothing immutable installation; clean-environment reproduction | Automated checks passed |

M1 remains **100%** and overall roadmap progress remains **33%**. Language gates
are tracked separately. A gate is complete only after its executable acceptance
checks pass. Manual product review follows the implementation.

## Evidence

1. `internal/configuration/parser.go`, `artifacts.go`, `compile.go` and tests cover
   syntax, source-mapped type errors, deterministic output, unknown fields, cycles,
   resource bounds and negative controls. Fuzzing exercised 3,713 inputs.
2. `TestModuleReuseAndTypedDiagnostics`, `examples/configuration/reuse`, and CLI
   confinement tests check explicit/default specialization equivalence, distinct
   parameter variants, local imports and exact locks.
3. `TestInspectionProject` compiles 56 artifacts covering all eight kinds and runs
   43 independently expected experiments. Reception and service-desk behavior
   reuse existing engine primitives. The exporter emits human-readable modules.
4. `internal/assembly` shares the installed-catalog resolver and planner with the
   source compiler. Six M1 scenarios join the reference suite; timed resource
   experiments remain explicitly separate from logical execution.
5. `TestEditorSemanticGuards`, Unicode source-location tests and mounted Vue
   interaction checks cover source/graph property edits, comment-preserving format,
   safe artifact renames, navigation, conflict rejection and visible navigation
   with the ribbon collapsed. Manual browser/mobile visual acceptance is pending.
6. Package replay, tamper controls, legacy migration with nine retained test cases,
   PostgreSQL rollback/concurrency/reopen tests, and `acceptance-configuration.mjs`
   cover portable evidence and atomic installation. Packages are unsigned; runtime
   authorization and implementation provenance remain later milestones.

**Configuration-language implementation: 6/6 automated gates (100% of this scope).**
This percentage does not represent production readiness or replace manual UX review.


## Authoring UX follow-up — 15 November 2026

Monaco replaces the textarea. Components now expose source, testbench, child
components and aspects by composition; Files exposes their actual folders.
The three-level example has four independent testbenches. Scoped evidence is
labelled and cannot be substituted for the whole-project report in a package.
Folder reorganization checks semantic identity before accepting a move.

Review with the nested-workspace steps in `CONFIGURATION_LANGUAGE.md`. The
browser acceptance script passed on code commit `2ecfa23`: production CSS, real
Monaco, typed-space retention, nested navigation, undo, find, scoped/full execution,
desktop/mobile width, and mobile navigation containment. CI desktop/mobile
screenshots were inspected; local Chromium cannot start under this workspace's
socket policy. Manual usability review with the engineer remains separate.
M1 stays 100% and the weighted roadmap stays 33%.

## Create and extract acceptance

Follow **Create your own component hierarchy** in `CONFIGURATION_LANGUAGE.md`.
Start empty, create Counter, independently specify its test, extract Identity,
complete Identity's test, and run both together and separately. Confirm the child
folder, breadcrumbs and Back/Forward navigation. A fresh testbench must fail;
changing an expected result must fail without changing the component.

Automated coverage: `create_test.go` exercises all eleven scalar operations,
upstream component outputs, literal/fixture bindings, standalone child compilation,
unchanged parent expectations, stale hashes, invalid names and folder collisions.
The live API acceptance covers empty-to-nested authoring with independently supplied
parent and child expectations. The Chromium exercise covers the same controls and
editing the generated TODO testbench into a passing independent test.
