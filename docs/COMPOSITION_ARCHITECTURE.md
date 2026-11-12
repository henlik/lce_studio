# Recursive component architecture

Architecture recommendation agreed for development on 12 November 2026.

## Purpose and status

LCE Studio should let engineers define a small behaviour precisely, compose it into a reusable capability, combine capabilities into an application, and connect independently governed applications into larger systems. Each level exposes a contract and retains an inspectable internal design. A hospital ecosystem, a school management application and a national data exchange are examples built with this mechanism; they are not new kernel primitives.

The implementation stack remains Vue, TypeScript, Go and PostgreSQL. This architecture does not prescribe a VM, container technology, cloud provider or infrastructure orchestrator. Runtime targets are selected through explicit adapters.

This document distinguishes three stages:

| Stage | Meaning |
| --- | --- |
| Delivered v0.3 | Processing profiles, portable locks, weighted exclusive workflows, deployment/resource experiments and recorded evidence described in the existing contracts. |
| v0.4 increment | A separate Components workbench for bounded, typed, pure evaluation and recursive composition. Package schema 2 extends the existing catalog; project schemas 1–3 and their timed simulation stay unchanged. [COMPOSITION_CONTRACT.md](COMPOSITION_CONTRACT.md) defines the exact syntax, limits, endpoints and evidence. |
| Target architecture | Stateful workflows, richer data contracts, application generation, runtime adapters, federation and calibrated simulation. These require subsequent implementation and acceptance evidence. |

The proposal is inspired by UML, BPMN, C4, ER modelling and flowcharts. It does not claim full conformance, import/export compatibility or execution support for those languages. A familiar diagram shape never implies support for all semantics associated with that shape.

The v0.4 slice supports three scalar types (`boolean`, bounded `integer`, `string`) and eleven pure operations: identity, equality, integer less-than/less-or-equal, Boolean and/or/not, eager value selection, integer addition/subtraction and string concatenation. Nested components use exact version/digest references. Bundles carry all reachable definitions, and successful evaluations retain immutable PostgreSQL snapshots of their bundle, inputs and trace. Definition tests compare typed expected and actual outputs. Records, collections, mutable state, business effects and conditional application routing remain future work.

Composite definitions cannot yet be placed into the existing application graph. There is no project schema 4, nested deployment or timed-resource integration in this slice. A functional evaluation proves only the supported input/output computation. These explicit boundaries keep the new component foundation usable without changing the meaning of earlier resource experiments.

## 1. One model, several coordinated views

The canonical model should own identities, definitions, instances, types, behaviour, relationships and deployment bindings. A diagram is a projection of that model. Layout, zoom, colour and collapsed state are presentation data and must not change execution.

| View | Primary question | Useful inspiration |
| --- | --- | --- |
| Landscape and context | Which people and independently owned systems interact? | C4 system landscape/context |
| Architecture | Which runtime units contain which modules, and which interfaces connect them? | C4 containers/components; UML components |
| Data | What information exists, how is it identified, and which relationships and constraints hold? | ERDs; UML types |
| Behaviour | How do values, decisions, events and state changes produce an outcome? | Flowcharts; UML activities/state machines |
| Collaboration | How do participants coordinate work and handle messages and exceptions? | BPMN processes/collaboration |
| Deployment | Where do instances run and which resources and failure domains do they share? | UML deployment; LCE continuum model |
| Evidence | Which version, assumptions, inputs and measurements support a claim? | LCE experiments and test results |

The same `AccessRequest` definition should be referenced by a port, an expression, a workflow and a generated API. Editing its type must trigger compatibility checks everywhere it is used. A visual editor may expose only a subset of a complex model, but hidden unsupported content must be preserved or explicitly rejected; it must not silently disappear.

C4 terminology is narrower than LCE's general reuse mechanism. A C4 container is an application or data store and represents a runtime boundary; a C4 component groups functionality within that boundary. LCE should retain explicit kinds such as `behaviour`, `module`, `application` and `system` rather than relabelling all of them as the same C4 element. A collapsed hospital ecosystem can be reused through an LCE system contract while its C4 landscape still shows independently owned applications.

## 2. Composition levels and identity

| Level | Example | Contract visible to its consumer |
| --- | --- | --- |
| Primitive operation | Compare two values | Typed arguments, result and defined error behaviour |
| Composite behaviour | Evaluate a scoped access rule | Typed ports, locked internal operations and deterministic outcome |
| Capability | Authorization or inventory reservation | Operations/events, owned state, policy and failure guarantees |
| Application | Pharmacy or school administration | External APIs/events, data ownership, deployment and operational requirements |
| Ecosystem | Distributed healthcare services | Participant contracts, service-sharing relationships and cross-organization workflows |
| System of systems | National data exchange | Federation agreements, exchange schemas, identity/trust mappings and service-level assumptions |

These are useful levels of explanation, not a compulsory six-level tree. A small application may use primitives directly. A national platform may call a capability exposed by an independent provider. The same definition may be instantiated more than once with separate parameters, state and placement.

Four identities must remain distinct:

1. **Definition:** a versioned reusable specification with a stable package identity.
2. **Instance:** a specific use of that definition, with its own identity, parameters and eventual state.
3. **Reference:** a dependency on an existing instance or external service; a reference does not create a copy.
4. **Deployment binding:** a mapping of an executable instance or artifact to a runtime target.

Containment and references form different relationships. A shared authorization service can be referenced by several applications without being copied into each application or charged several times for the same physical resources. Recursive composition means nested composition of definitions; it does not require allowing infinite self-recursive definitions or unbounded execution.

## 3. Selecting the atomic foundation

An LCE primitive is the smallest operation for which the engine directly defines semantics. It is atomic relative to that engine contract, not necessarily an indivisible machine instruction, transaction or real-world action. The design goal is a small, understandable and testable foundation that composes successfully.

Use these criteria to decide whether something belongs in the kernel:

- Its inputs, outputs, state effects and errors can be stated without relying on domain vocabulary.
- Its behaviour cannot be expressed cleanly using existing supported primitives, or a documented boundary must remain externally implemented.
- It has bounded validation and execution costs under the supported model.
- It has independent reference cases, including invalid input and boundary conditions.
- At least two distinct domains need it, or it is necessary for a well-defined execution mechanism.

A profile, permission, patient, invoice or school is normally a model definition built from the foundation. A retry policy can be composed from invocation, failure handling, a counter and a timer once those operations exist. A human approval is an interaction with a participant and a lifecycle, not a Boolean primitive that silently invents a person's decision.

### Candidate semantic families

The following is a design vocabulary, not the list of operations already implemented in v0.4. The implementation contract must name the exact supported subset.

| Family | Candidate declarations or operations | Required distinction |
| --- | --- | --- |
| Values and types | Boolean, bounded number, text, identifier, timestamp, enumeration, record, collection, optional value | Missing, null, empty and zero need explicit meanings; no implicit coercion. |
| Pure expressions | Constant, input/field access, equality/order, Boolean combination, membership, value construction and transformation | Pure evaluation does not read a wall clock, random source, database or network implicitly. |
| Control | Sequence, conditional selection, call/return, bounded iteration, split and synchronization | A decision, a merge and a synchronization barrier have different semantics. |
| State | State declaration, read, update, transition, guard and invariant | State is owned by an instance; persistent and volatile state have different recovery rules. |
| Data effects | Create/read/update/delete an entity, establish/remove a relationship, enforce a constraint | A conceptual entity is not automatically a SQL table; transaction boundaries are explicit. |
| Interaction | Send, receive, request/reply, publish/subscribe, message correlation | A message exchange has delivery semantics and is separate from an internal control edge. |
| Time and failure | Timer, deadline, timeout, failure, cancellation and recovery | Virtual time, interruption and remaining work have exact scheduling rules. |
| Composition | Definition, instance, typed port, parameter, connection and public output | Encapsulation hides implementation details while retaining contract obligations. |
| Resources | Resource demand, capacity, placement, availability and energy profile | Software containment does not allocate or create physical capacity. |

Initially keep pure expressions and composition separate from effects. Pure components are easier to validate, reproduce, nest and compare because their results depend only on explicit inputs and locked definitions. Stateful and asynchronous behaviour can then be introduced with an explicit event/state protocol instead of being smuggled into expression evaluation.

Foundational UML (fUML) is a useful reference for this work: it defines an executable subset with precise action and activity semantics. LCE can study and map a supported subset without adopting the full fUML metamodel or changing its Go implementation stack.

## 4. A common component contract

Every reusable definition should eventually expose the following contract. Capabilities are earned by implementation and evidence; unsupported fields or capabilities must be rejected clearly.

| Contract area | Required information |
| --- | --- |
| Identity | Namespaced ID, exact version, schema version, content digest, author, licence and dependency locks |
| Kind and capabilities | Behaviour/module/application/system kind; explicit design, evaluation, simulation, generation, deployment and measurement support |
| Ports | Input/output direction, type identity, cardinality, required/optional status and documented meaning |
| Configuration | Defaults, allowed ranges, units, validation and explicit migration rules |
| Behaviour | Internal definition or external adapter, input/output mapping, deterministic ordering and failure outcomes |
| State and ownership | Owned state, lifecycle, persistence, isolation and permitted effects |
| Assumptions and guarantees | Preconditions, invariants, postconditions, dependencies and limits of the guarantee |
| Resources | Demand/capacity model, measurement provenance, placement requirements and accounting boundaries |
| Evidence | Reference inputs/outputs, negative cases, conformance tests, supported environment and known limitations |

The first pure subset needs only the relevant portions. A declaration of future metadata must not suggest that enforcement exists. For example, writing `tenantIsolation: true` is not an isolation mechanism.

### Connection and relationship types

Model these separately even when several are drawn as lines:

- **Containment:** which instance belongs inside an assembly.
- **Dependency/reference:** which exact definition or existing service is required.
- **Data association:** how entity instances are related, including cardinality and ownership.
- **Value binding:** how a typed output supplies an input in a pure computation.
- **Control flow:** which action becomes eligible next.
- **Message exchange:** what participants send and receive across a declared communication boundary.
- **Deployment binding:** where an executable instance runs.
- **Physical connectivity:** what resources can transport data and under which constraints.

An ER relationship does not itself perform a query or transmit a message. A BPMN sequence flow does not itself create a network connection. A graphical parent does not automatically become a security, tenancy or deployment boundary.

### Type and execution rules

The bounded first contract should require exact compatible types and complete bindings. Unknown ports, multiple conflicting bindings, unresolved references and dependency cycles must fail validation. Value size, nesting depth, definition count and evaluation work require explicit limits. A valid design must not cause infinite expansion or unbounded allocation.

Each primitive specification should state its signature, accepted values, output, error cases, ordering, work accounting and version. Decisions must define whether unused branches are evaluated. Boolean operations must define missing-input handling rather than converting an error into a permissive result. Numeric operations must specify representation, overflow and precision. Execution errors must not be reported as successful application decisions.

Stateful contracts later need event correlation, concurrency rules, ordering, cancellation, retries, duplicate effects, persistence and restart semantics. A distributed transaction or consistent snapshot must never be inferred merely because operations are drawn inside one box.

## 5. Versioning, portability and reproducibility

Preserve existing project schemas and schema-1 profile semantics. New kinds of executable definition need explicit versioned contracts; a package name alone cannot change how an old model runs.

A portable composite must carry the transitive closure of its exact definition dependencies, with consistent version/digest references. Validation and evaluation of that bundle must work without catalog access or network downloads. Catalog installation is a distribution convenience, not a hidden execution dependency.

The canonical serialization and digest algorithm must be specified and tested, including supported text and value encodings. Digests identify content; they do not certify the publisher, runtime safety or quality of a model. Identical reinstallations are idempotent, conflicting content under the same identity/version is rejected, and upgrades are explicit.

Instance paths must remain distinguishable in traces so two uses of one definition can be inspected separately. Evaluation evidence records the root definition, complete locks, inputs, engine version, outcomes and relevant limits. Later simulation evidence additionally records workload, resource bindings, seed, event schedule and calibration assumptions.

Export/import tests must cover more than the root package: nested references, unused valid definitions where allowed, tampered content, missing dependencies and isolated execution without an installed catalog are all important cases.

## 6. Authorization as the first composition proof

ABAC determines whether a subject may perform an operation on a resource under stated conditions. Authentication establishes identity. A complete identity and access-management capability combines these concerns while retaining their separate contracts. Roles may be subject attributes; permissions and scopes describe allowed operations or boundaries according to a declared policy model.

The first example should evaluate explicit supplied attributes. It does not authenticate users, enforce access to production records or claim a complete security product. Real identity providers, attribute sources, enforcement points and audit storage require separate supported adapters and tests.

### Generic construction

1. Define typed inputs for subject, resource, requested action and environmental context. Start with the bounded types supported by the implemented contract.
2. Build generic checks such as equality, membership and Boolean combination from primitives.
3. Package reusable rules for permitted action, matching scope and allowed subject attributes.
4. Compose those rules into an authorization decision with declared combination behaviour.
5. Expose the decision through a stable public interface. Distinguish evaluation failure from an ordinary negative decision.
6. Reuse the same locked definition in separate domain examples with different inputs or policy configuration.

For a simple fail-closed demonstration, only a successful explicit permit result allows the simulated action. Missing, malformed or unavailable inputs must not accidentally produce permit. This is an example policy choice, not an assertion that every authorization policy can be reduced to one Boolean or that the example implements a security standard.

### Two reference domains

| Reused behaviour | Healthcare example | Education example |
| --- | --- | --- |
| Subject attribute check | A configured staff category is eligible for an operation. | A configured school staff category is eligible for an operation. |
| Resource scope check | The requested record belongs to an allowed organizational scope. | The requested student record belongs to an allowed school scope. |
| Action check | The requested operation appears in a supplied allowed-action set. | The same generic operation check uses a different action set. |
| Context check | Explicit supplied contextual conditions satisfy the example policy. | The same rule contract evaluates education-specific inputs. |
| Decision composition | Produce permit or deny for the supplied example. | Execute the same package and primitives without kernel changes. |

These examples are synthetic architecture tests, not legal, clinical or operational authorization policies. Cross-organization access later requires an explicit sharing agreement and policy evaluation; equality of local organization IDs is not a general federation solution.

### Exact v0.4 proof

The initial executable examples intentionally use only scalar inputs and supported operations. Both `example.health-record-access@1.0.0` and `example.school-record-access@1.0.0` call the exact same `lce.access-policy@1.0.0`, which itself calls `lce.same-tenant@1.0.0`. This gives three observable component levels. Each wrapper binds domain-specific required-role and allowed-scope strings while exposing the remaining inputs.

The shared policy accepts `subjectTenant`, `resourceTenant`, `role`, `requiredRole`, `requestedScope`, `allowedScope` and `active`. Its `permitted` output requires tenant equality, role equality, scope equality and an active subject. The healthcare wrapper supplies `clinician` and `record:read`; the education wrapper supplies `teacher` and `student:read`. Scope comparison here is exact string equality; scope hierarchies, action sets, claims retrieval and policy administration are not implemented.

Each wrapper includes permit, wrong-tenant, wrong-role, wrong-scope and inactive-subject tests. Missing or type-invalid inputs reject evaluation rather than becoming a permission result. Successful evaluations record the actual nested trace. This workbench does not connect the result to an actual application enforcement point, authenticate a user or protect the studio itself.

Acceptance cases should include permit, each independent reason for denial, malformed types, unknown inputs, missing dependencies, repeated nested instances and identical results after portable export/import. The test should prove real reuse of one definition, not two copied implementations with different names.

## 7. Organization, tenancy and federation

Logical nesting must not silently transfer ownership or authority. Keep organizations, tenants, facilities, users, identities, administrative domains and geographic sites separate. A pharmacy may serve several hospitals while retaining its own users, state, policies, runtime and operating responsibility.

For larger systems, references cross boundaries through declared contracts. A national exchange can expose a common access surface while source systems retain responsibility for their data. Participation does not require one shared database, one deployment or one central policy authority.

Future federation contracts must specify identifier namespaces, schema/version mapping, resource ownership, trusted identity/attribute issuers, authorization policy composition, permitted purposes, audit responsibility and revocation behaviour. Scope values must carry a namespace and declared meaning rather than relying on identical strings across unrelated organizations.

Intermittent connectivity requires explicit rules for cached attributes, expiry, offline permissions, queued effects and reconciliation. An offline decision based on stale data is a different guarantee from an online decision using current attributes. Simulation should expose the distinction and test it under the configured failure model.

## 8. Resource accounting and simulation fidelity

Keep the software composition graph separate from the physical resource graph. An application assembly can span several hosts; several assemblies can share one host, link, storage service or energy source. A diagram's parent-child relationship neither allocates resources nor implies a physical location.

Resource requests must retain their leaf instance identity and resolve to the shared resource they consume. Charge shared CPU, memory reservations, storage and network transfer once at their defined accounting boundary. Avoid both charging a composite and its children for the same work and assigning private fictitious capacity to each collapsed component.

Use explicit fidelity modes:

| Mode | Behaviour and evidence |
| --- | --- |
| Expanded execution | Execute the supported internal model. Collapsing the drawing changes presentation only. |
| Declared resource simulation | Couple supported behaviour to supplied resource demands, queues, outages and energy assumptions. |
| Calibrated approximation | Replace internals with a documented surrogate for a defined workload and environment; retain error bounds and calibration evidence. |
| Execution-based verification | Run a supported real implementation under controlled constraints and compare observed behaviour with model predictions. |

The first pure evaluator provides functional results and bounded execution evidence. It must not invent CPU time, latency, availability or energy predictions from the number of visual nodes. A future bridge to the continuum simulator must define how evaluation is scheduled, which costs are charged, and what happens when a host fails during evaluation.

Scaling to a national platform will require selective detail, bounded workloads and explicit approximations. Aggregate latency is not generally the sum of child latencies when branches, contention, queues or failures interact. A surrogate must preserve the properties being tested and state which details it omits.

## 9. Application construction and runtime adapters

A simulation definition and a runtime implementation should share typed contracts and conformance cases. Their correspondence must be checked; matching names and diagrams alone do not establish equivalence.

Generation should begin with a small supported subset: typed Go interfaces/data structures, explicit operation implementations, Vue forms or views where defined, database migrations, tests and deployment manifests for supported targets. Generated files need ownership rules and stable extension points so regeneration preserves handwritten code.

Effect adapters should have separate model, simulation and runtime implementations. Examples include a persistent store, a message transport, an identity provider and an audit sink. Their capability declarations must identify supported error and recovery behaviour. Importing or simulating a component must never execute its real deployment effects.

External executable adapters require isolated execution and explicit capabilities, cancellation and CPU/memory/output limits. Authentication and cryptographic functions should use established implementations through these boundaries, with their correctness and security assumptions documented. A diagram of an authentication workflow is not evidence that a custom cryptographic implementation is sound.

Deployment plans bind artifacts to actual resources and administrative domains, preserving credentials and secret-management boundaries. Inventory, reservation, upgrade, rollback and disconnected recovery belong to runtime orchestration milestones, not to visual containment.

## 10. Delivery sequence and acceptance gates

| Increment | Deliverable | Gate before broadening support |
| --- | --- | --- |
| v0.4 composition foundation | Three scalar types, eleven pure operations, nested schema-2 packages, portable bundles, shared immutable catalog, separate workbench, functional tests and saved evaluation evidence | Three nested levels execute with inspectable traces; invalid wiring and dependencies fail; healthcare and education examples use the same locked policy; project schemas 1–3 and v0.3 simulation remain unchanged. |
| Structured data | Records, collections, entity identity, relationships and constraints, with coordinated data/interface views | Invalid values and incompatible schema changes are detected consistently; portable contracts roundtrip without hidden coercion. |
| Stateful effects | Owned state, effect requests, persistence and recovery protocol | Tests distinguish volatile/persistent state, duplicate effects, rollback boundaries and restart outcomes. |
| Event workflows | Messages, correlation, timers, decisions, bounded loops, parallel coordination and human-task lifecycle | Independent cases cover event ordering, timeouts, synchronization, cancellation and failure propagation. |
| Coupled behavioural simulation | Explicit scheduling/resource demands for richer component behaviour | Shared-resource conservation, outage ordering and reproducibility hold across nested compositions. |
| Runtime construction | First supported Go/Vue generation path and external adapters | Generated units run independently; regeneration preserves extension code; model/runtime conformance cases agree. |
| Distributed deployment | Inventory, domain adapters, placement, rollout and recovery | A supported application operates across separate domains and recovers according to stated partition/power assumptions. |
| Domain ecosystems | Growing healthcare reference plus education or another contrasting domain | Reuse crosses domains without kernel changes; tenancy, ownership and sharing policies remain explicit. |
| Federation and scale | System contracts, exchange adapters, selective fidelity and calibration | Independent systems exchange supported data under tested policy/failure conditions; approximations disclose measured limits. |

Testing, documentation, accessibility and threat modelling accompany every increment. The gates describe observable outcomes rather than calendar promises. Broader support follows evidence from the preceding subset, not the appearance of additional icons in the palette.

## 11. Decisions to preserve

- Keep a small semantic kernel and build domain behaviour through composition.
- Use coordinated views over shared identities and definitions.
- Keep pure computation, state effects, communication and physical resource accounting distinct.
- Treat nesting, instance reuse, external references and deployment as different operations.
- Preserve old schemas and exact portable dependency locks.
- Reject unsupported capabilities and invalid models explicitly.
- Make assumptions, failure modes and simulation fidelity visible in evidence.
- Test the architecture through multiple domains before freezing a general SDK.

## Primary references

1. [OMG UML 2.5.1](https://www.omg.org/spec/UML/2.5.1): structural and behavioural modelling constructs. [OMG introduction](https://www.omg.org/gettingstarted/what_is_uml.htm) explains coordinated structural and behavioural views.
2. [OMG BPMN 2.0.2](https://www.omg.org/spec/BPMN/2.0.2): process, event, gateway and participant/message semantics.
3. [C4 abstractions](https://c4model.com/abstractions), [containers](https://c4model.com/abstractions/container), [components](https://c4model.com/abstractions/component) and [system landscapes](https://c4model.com/diagrams/system-landscape): levels of architectural explanation and runtime boundaries.
4. Peter P. Chen, [The Entity-Relationship Model—Toward a Unified View of Data](https://doi.org/10.1145/320434.320440), ACM TODS 1(1), 1976. [Author-hosted excerpt](https://csc.lsu.edu/~chen/pdf/erd-5-pages.pdf).
5. [ISO 5807:1985](https://www.iso.org/standard/11955.html): documentation symbols and conventions for data, program and system flowcharts.
6. [OMG fUML 1.5](https://www.omg.org/spec/FUML/1.5) and its [reference implementation documentation](https://modeldriven.github.io/fUML-Reference-Implementation/): precise execution semantics for a foundational UML subset.
7. [NIST SP 800-162, Guide to Attribute Based Access Control](https://csrc.nist.gov/pubs/sp/800/162/upd2/final): subject, object, operation and environment attributes evaluated against policy.

These sources inform the proposed architecture. They do not certify LCE, imply full standards compatibility or replace the implemented contracts and their tests.
