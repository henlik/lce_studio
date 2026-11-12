# v0.4 typed composite contract

This document is the implementation contract for the first recursive-composition increment. It adds pure functional components, a graphical workbench, portable version locks, evaluation/tests and immutable PostgreSQL evaluation evidence. It does not change project schemas 1–3 or their resource simulation. Stateful workflows, conditional application routing, deployment of nested leaves, full UML/BPMN/C4 interchange and application generation remain later increments. The complete target is described in COMPOSITION_ARCHITECTURE.md.

## Package representation and compatibility

Append `Behavior *Behavior json:"behavior,omitempty"` and `Tests []BehaviorCase json:"tests,omitempty"` to the END of the existing Go ComponentPackage struct. Keep every existing field, order and tag unchanged; schema1 bytes and digests must remain identical. Schema1 rejects non-nil behavior/nonempty tests. Schema2 requires kernel `composite`, capabilities exactly `["design","evaluate"]`, Behavior non-nil, and the existing Config entirely zero (all three parameter triples zero and Deduplicate false). Config remains in typed canonical JSON solely for backward representation compatibility; the editor hides it for composite definitions. Schema2 metadata obeys the same identity/version/text bounds as schema1.

Profiles and composites share the immutable component catalog, but composite packages cannot be inserted as application nodes or referenced by schema1–3 projects. The existing profile picker must filter to schema1. The new Components workbench edits schema2. Names and canvas positions belong to versioned definitions and therefore affect the digest. This is exact-content identity, not behavioral equivalence or a publisher signature.

Go model types / exact JSON names:

- `TypedValue {Type string json:"type"; Boolean *bool json:"boolean,omitempty"; Integer *int64 json:"integer,omitempty"; String *string json:"string,omitempty"}`. Exactly one non-nil member matches Type (`boolean`, `integer`, `string`). Integers are within ±9007199254740991 inclusive (browser exact integer range). Strings: valid UTF-8, at most 4096 bytes, no NUL or unsupported control characters (newline/tab/CR allowed). No floats, null values, automatic conversions or truthiness.
- `BehaviorPort {Name string json:"name"; Type string json:"type"}`. Port/local IDs use `^[A-Za-z][A-Za-z0-9_]{0,63}$`. Types are exactly the three scalar types.
- `BehaviorBinding {Input string json:"input,omitempty"; Node string json:"node,omitempty"; Port string json:"port,omitempty"; Literal *TypedValue json:"literal,omitempty"}`. Exactly one source: outer input; node AND port; typed literal. No mixed forms or dangling port.
- `BehaviorNode {ID string json:"id"; Op string json:"op,omitempty"; Component *ComponentRef json:"component,omitempty"; Inputs map[string]BehaviorBinding json:"inputs"; Position Position json:"position"}`. Exactly one of Op or Component. Position is finite within existing canvas coordinate bounds. A component call refers to a schema2 package by exact ID/version/digest.
- `Behavior {Inputs []BehaviorPort json:"inputs"; Outputs []BehaviorPort json:"outputs"; Nodes []BehaviorNode json:"nodes"; Bindings map[string]BehaviorBinding json:"bindings"}`. Bindings maps each public output to its source; all declared outputs must be bound with exact type. Every node input must be bound exactly once, with no extra inputs. Inputs, Outputs and Nodes must be JSON arrays, never null; use [] for empty Inputs or Nodes. Bindings and every node Inputs must be JSON objects, never null. Empty nodes are allowed for direct/literal output components.
- `BehaviorCase {Name string json:"name"; Inputs map[string]TypedValue json:"inputs"; Expected map[string]TypedValue json:"expected"}`. Inputs/expected exactly match the public ports; unique nonblank names≤100 bytes.
- `ComponentBundle {SchemaVersion int json:"schemaVersion"; Root ComponentRef json:"root"; Packages []PackageLock json:"packages"}`. Bundle schema1, 1–50 unique ID/version locks, root is schema2, full transitive dependency closure embedded. All locks must be schema2 and reachable from root; reject extras to avoid ambiguous installs. No mutable catalog lookup during validation/evaluation. Digest-check every manifest and every reference.

## Pure primitive signatures

Every primitive has one output named `result`. `T` means one of the three scalar types, unified within that invocation only; public port types cannot be T.

| op | inputs | output |
| --- | --- | --- |
| identity | value:T | T |
| equal | left:T, right:T | boolean |
| less_than | left:integer, right:integer | boolean |
| less_or_equal | left:integer, right:integer | boolean |
| and | left:boolean, right:boolean | boolean |
| or | left:boolean, right:boolean | boolean |
| not | value:boolean | boolean |
| select | condition:boolean, trueValue:T, falseValue:T | T |
| add | left:integer, right:integer | integer |
| subtract | left:integer, right:integer | integer |
| concat | left:string, right:string | string |

`select` is eager pure value selection: both inputs are computed; it is not conditional control flow. Arithmetic outside the safe integer range and concatenation exceeding the string limit fail explicitly. Equal compares exact typed scalar values; no coercion. All operations are deterministic and pure; no clock, random input, filesystem, network, secret access, script, database effect or implicit authorization enforcement.

Primitive discovery: `PrimitiveSpec {Op,Name,Description string; Inputs,Outputs []BehaviorPort}` with JSON `op,name,description,inputs,outputs` and T in generic metadata. Expose model.PrimitiveSpecs() and model.PrimitiveOutputType(op string, inputs map[string]string) (string,error).

## Validation and execution bounds

Each schema2 definition is limited to 128 KiB of typed canonical JSON, including tests. This also bounds the shared catalog response size. Each definition: ≤16 inputs, 1–16 outputs, 0–32 nodes, ≤32 test cases. Bundle≤50 definitions, nesting depth≤8 including root, ≤256 expanded primitive executions AND ≤256 component invocations per evaluation. Count repeated call occurrences, not unique packages; reject cycles in local dataflow and package dependencies. Validate all nodes, including unused outputs; require each node to contribute transitively to a public output (no unobservable disconnected execution). Stable topological order uses lexicographically smallest ready node ID; independent of source node array order. Dependency calls execute depth-first at their call node. Input/output map traversal is sorted when producing diagnostics or traces. Validation work is bounded; reject oversized collections before expensive expansion. HTTP body limit stays 2 MiB. Cancellation is checked between steps/calls.

No computational step is assumed transactionally atomic in a future distributed runtime; 'primitive' here means indivisible in this evaluator's documented semantics. It has no virtual clock or hardware cost metrics. Evaluation duration is not a simulated latency prediction.

Model APIs: `ValidateComponentPackage(p)` remains local manifest validation (schema2 validates local syntax, metadata, literals and structural bounds). `ComponentDigest(p)` unchanged algorithm. `ValidateComponentBundle(bundle) []ValidationIssue` performs closure/type/DAG/bounds checks including tests. `PrepareComponentBundle(p ComponentPackage, dependencies []PackageLock) (ComponentBundle, []ValidationIssue)` computes root lock, expects dependency-only closure, and validates it. `ExampleComponentBundles() []ComponentBundle` returns healthcare and education examples with shared authorization core and nested tenant-check. `PrimitiveSpecs`, `PrimitiveOutputType` as above.

## Evaluation and tests

New Go package internal/behavior, Version="0.4.0". `Evaluate(ctx context.Context, bundle model.ComponentBundle, inputs map[string]model.TypedValue) (Result,error)`; validates before execution and never mutates arguments. Result fields:
- `engineVersion` string; `bundleHash` lowercase SHA-256 over typed json.Marshal(bundle); `root` ComponentRef; `outputs` map typed values; `operations` primitive count; `invocations` component count; `events` []Event.
- Event fields `sequence` integer from 1, `path` string, `kind` (`component_enter`,`primitive`,`component_exit`), `op` optional string, `component` optional ref, `inputs` optional map typed values, `outputs` optional map typed values. Root path `$`; child node paths `$/nodeID` recursively. Events≤768 implied by256+2×256 caps. Enforce total serialized result≤1MiB; exceeding it aborts, no successful partial evidence.
- Errors include path and reason. Invalid input, overflow, missing attrs, cancellation and limits are errors, never implicit permit/deny. Boolean false is a valid decision output, distinct from an execution failure.
- `Test(ctx,bundle) (TestResult,error)` evaluates root package Tests only in definition order, sharing request cancellation but resetting operation bounds per case. TestResult has `passed` bool, `cases` []CaseResult; CaseResult has `name`, `passed`, `expected`, optional `actual`, optional `error`. Per-case evaluation failures are failed cases; outer cancellation aborts. Empty tests => passed false with cases[] (not 'verified'). No full trace per test case. Total serialized TestResult is also capped at 1 MiB; overflow returns an outer limit error without a successful partial suite. Each result retains typed expected/actual. Digest/version includes the tests.

## API and persistence (all local same-origin limits retained)

- GET /api/components/primitives → PrimitiveSpec[].
- GET /api/components/composite-examples → ComponentBundle[].
- POST /api/components/prepare body `{package,dependencies}` →200 prepared bundle,422 issues. No persistence.
- POST /api/components/validate-bundle body ComponentBundle →200 `{valid,issues,bundle?}`; no persistence. A valid response includes the canonical typed bundle. Clients must adopt that returned representation instead of parsing the original JSON independently, because decoder field-casing and duplicate-field behavior can differ. Invalid responses omit bundle.
- POST /api/components/bundles body ComponentBundle →201 if any versions newly installed,200 if all identical; response `{bundle,installed:[ComponentRecord...]}`. Atomic all-or-nothing installation of full closure under existing advisory catalog lock and global 200-version cap;409 conflict/full. Preserve POST /api/components raw schema1 only; schema2 responds422 directing bundle install.
- GET /api/components/bundle?id=...&version=...&digest=... →exact root+closure reconstructed from immutable installed versions,404 missing,422 invalid ref. No network resolution.
- POST /api/components/evaluate body `{bundle,inputs}` →201 EvaluationRecord saved only on successful evaluation. Record `{id,createdAt,bundle,inputs,result}`. Store immutable JSON snapshot in new `component_evaluations` table; no mutation/delete API. Same max 2 concurrent computations channel, 15 s timeout;422 validation/execution error,408 cancellation/timeout,429 busy.
- GET /api/components/evaluations → newest 50 summaries `{id,createdAt,componentId,componentVersion,operations,invocations}`. GET /api/components/evaluations/{id} →full immutable record.
- POST /api/components/test body `{bundle}` →200 TestResult,422 invalid bundle,408 cancellation,429 busy. Test outcomes are not automatically installed or saved.
- Health version 0.4.0. Existing run APIs and schemas unchanged. Existing simulation Version remains0.3.0 because its implementation/semantics are unchanged.

Store methods names owned by API/store agent; coordinate with root. No authentication added to studio itself. Modelled authorization is a pure decision demonstration, not a production IAM/security boundary.

## Examples / acceptance

Same `lce.access-policy@1.0.0` used unchanged by `example.health-record-access@1.0.0` and `example.school-record-access@1.0.0`. Core calls `lce.same-tenant@1.0.0`; exact nested lock reused. Inputs core: subjectTenant:string, resourceTenant:string, role:string, requiredRole:string, requestedScope:string, allowedScope:string, active:boolean. permitted output = sameTenant AND roleEqual AND scopeEqual AND active. Domain wrappers expose subjectTenant/resourceTenant/role/requestedScope/active; bind literals requiredRole (`clinician`/`teacher`) and allowedScope (`record:read`/`student:read`). Output permitted:boolean. This is a deliberately small policy, not comprehensive real-world access policy, consent management, authentication or RBAC provisioning. Tests include permit, wrong tenant, wrong role, wrong scope, inactive subject. Missing/type-invalid input is a rejected evaluation, not a granted decision. Demonstrate 3 component levels root→policy→tenant-check, independent paths, same-core reuse, portability without catalog, strict missing/tamper/cycle/type/expansion failures.

## Workbench

A separate Components workspace edits schema2 draft metadata, scalar inputs/outputs, primitive/call nodes and their bindings; shows a real graph derived from these bindings with selectable nodes and nested-definition navigation. Provide output bindings, typed literal/input/source selectors, add/remove operations, explicit dependency selection from valid installed bundles, import/export entire bundle, install, evaluate, tests, saved evaluation history with immutable snapshot inspection. Keep prior evidence separate and visibly stale after draft/input edits. Browser-local drafts may remain unsaved until installation; explain state. Evaluation/history is server-backed. Existing Design catalog remains profile-only and project v1–3 behavior unchanged. No fabricated ERD/BPMN/C4 full editors; document those next stages.
