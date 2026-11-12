# Component authoring and SDK direction

v0.3 implements **declarative component profiles** for the service, database and exclusive-router simulation kernels. An engineer can install, reuse and share a new profile without modifying studio source. A profile defines a name, exact version, parameter defaults and permitted ranges. It cannot introduce a new execution primitive or execute arbitrary code. The general executable SDK described later in this document is still a proposal.

## Author a usable component

1. Copy `examples/components/queued-worker.json`, `durable-store.json` or `weighted-router.json`. Choose the kernel whose stated semantics fit the component you intend to model.
2. Give it a namespaced ID such as `example.validation-worker`, an exact version such as `1.0.0`, a meaningful name, author and licence. Document assumptions in its description.
3. Set `processingMs`, `capacity` and `memoryMb` default/min/max values. These are modelling inputs, not measurements inferred from the component's name. A database profile can set its initial `deduplicate` option.
4. Validate locally without installing or contacting a server:

   ```sh
   go run ./cmd/componentcheck path/to/my-component.json
   ```

   The command prints a portable `{package,digest}` lock. It accepts `-` for standard input, rejects malformed/unknown fields and unsupported capabilities, and returns a nonzero exit status on failure.

5. Import the raw package JSON through the studio's component catalog. Installation validates it and stores its exact ID, version and content digest in PostgreSQL. Alternatively:

   ```sh
   curl -X POST http://localhost:8080/api/components \
     -H 'Content-Type: application/json' \
     --data-binary @path/to/my-component.json
   ```

6. Insert the installed profile into a project. Its defaults initialize editable node values; its profile bounds remain enforced. Connect it, place it if using infrastructure, and run a small scenario with a known outcome.
7. Export the project or share the raw package. Project exports include the full pinned definitions, so another studio can validate and simulate the project without installing the same catalog first.

Use a new package version when changing its contents. Reinstalling identical content is safe; installing different content under an existing ID/version returns a conflict. Existing project nodes keep their previous definitions and parameters. No automatic package upgrades or silent replacements occur.

## Implemented capabilities

| Capability | v0.3 behavior |
| --- | --- |
| Design | Display metadata; insert nodes; edit processing/capacity/memory within package bounds |
| Simulation | Use the declared bounded `service`, `database` or `router` kernel |
| Routing | A router chooses one weighted branch per logical request; retries keep that route |
| Portable locks | Exact manifest, ID, version and SHA-256 digest embedded in projects and run snapshots |
| Catalog | Install, list, export and reuse immutable versions; up to 200 installed versions |
| Compatibility | Older v1/v2 projects remain usable; package-backed projects use schema3 |
| Generation, deployment and executable plugins | Not implemented |

The accepted capabilities array is exactly `["design","simulate"]`. Unsupported declarations are rejected. A digest identifies package content; it does not certify the publisher or validate a performance claim. The full JSON schema, canonical hash rules, resource semantics and API responses are defined in [WORKFLOW_CONTRACT.md](WORKFLOW_CONTRACT.md).

For example, a validation worker profile could be reused for a logistics order or an administrative request by giving its instances different names and processing parameters. That reuse models queueing and processing demand; the studio does not execute the real validation business logic.

## Future executable SDK proposal

The remaining sections describe the broader extension boundary. They are design goals rather than available runtime APIs.

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
