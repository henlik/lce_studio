# Component SDK contract proposal

Status: design proposal for the next milestones. v0.2 implements built-in application and infrastructure models. It does not load arbitrary component packages, execute external simulation plugins, generate applications or deploy infrastructure. This document defines the boundary to build toward; it must not be read as a list of available APIs.

## Extension principle

An engineer should be able to contribute a component without modifying studio source. Domain packages assemble these components into reusable patterns. Healthcare-specific concepts remain outside the generic simulation kernel.

Component categories include software, infrastructure, behaviour, orchestration policy, domain workflow, integration adapter and verification package. A component can be atomic or a composition with explicitly exposed interfaces.

## Package contents

| Item | Contract |
| --- | --- |
| Manifest | Stable namespaced ID, package version, SDK compatibility, display name, category, author, licence and dependencies |
| Configuration | Versioned schema, defaults, units, bounds, validation and upgrade rules |
| Ports | Typed inputs/outputs, direction, delivery assumptions and message/data contracts |
| Behaviour | States, transitions, preconditions, side effects, cancellation and failure semantics |
| Resource model | Compute/memory/storage/network/energy demands and parameter provenance |
| Presentation | Editor representation and property panels; accessible controls and no hidden semantics |
| Capability declaration | Explicit support for design, simulation, generation, deployment and measurement |
| Implementations | Versioned adapters for supported capabilities; no capability inferred from an icon or a template |
| Verification | Independent examples, conformance tests, invariants and known limitations |
| Documentation | Usage, assumptions, evidence, compatibility and migration guidance |

A design-only package cannot be simulated until a compatible behaviour model exists. A simulation-only component cannot be deployed. A wrapper around an existing app may have approximate simulation support; the UI must expose that approximation.

## Proposed simulation adapter boundary

The kernel owns virtual time, stable event ordering, the scenario seed, cancellation and execution limits. Components receive immutable configuration and an explicit event/context. They return state transitions, future events and declared resource requests; they do not independently sleep, change the kernel clock or bypass capacity accounting.

All randomness is derived from reproducible named streams. State snapshots and emitted events use documented, versioned serialization. Components must declare how queued work, volatile state, persistent state and timers behave on interruption and restart. Resource demands have explicit units and cannot be negative or non-finite.

A plugin's runtime code is separate from its simulation adapter. Sharing a model schema helps reduce divergence but is not proof of behavioural equivalence. Conformance tests compare supported scenarios across both implementations.

## Security and execution boundaries

External executable packages require explicit installation/trust decisions and isolated execution with bounded CPU, memory and event budgets. File, network, credential and deployment access are granted only to capabilities that require them. Declarative schemas and visual assets must not silently execute code. A signature identifies an artifact publisher; it does not replace sandboxing or validation.

The transport could be a versioned process/RPC boundary or another portable isolated runtime. Do not freeze a language-specific native plugin ABI before testing portability, determinism and isolation requirements. The Go core and Vue editor remain the primary application stack; specialist adapters may use their own implementation languages.

## Generation and deployment adapters

Generation declares supported targets, generated-file ownership, stable extension points and migration behaviour. Regeneration must not overwrite handwritten code. Deployment adapters consume a reviewed deployment plan and credentials supplied for that target; simulation must never cause real deployment side effects.

Artifacts need reproducible provenance, compatible dependency locks and a mapping back to the model revision. Hardware models describe capabilities and measured profiles; a deployment adapter must verify that real resources satisfy the required constraints rather than assuming the model created physical capacity.

## Registry and compatibility

The future catalog stores package metadata and versioned artifacts. Projects lock exact package versions. Schema upgrades and compatibility changes are explicit and reviewable. Removal of a package leaves a readable unresolved component instead of silently replacing its behaviour. Private and shared catalogs use the same contract.

## SDK acceptance gate

1. Build an external component using only published SDK interfaces.
2. Install and validate it without changing studio source.
3. Add it to an existing project and reject incompatible connections.
4. Run known input/output and resource-accounting reference cases.
5. Reproduce a run from the locked package, model and seed.
6. Reject an unsupported capability and terminate a component that exceeds its budget.
7. Upgrade the package while preserving a reproducible copy of the earlier project.
8. Demonstrate reuse in two domains.

The first SDK implementation should satisfy this gate with a small generic queued-processing component and an infrastructure profile before expanding to complex domain packages.
