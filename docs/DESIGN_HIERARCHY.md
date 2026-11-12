# Design levels and composition flow

## The distinction to preserve

LCE has two different coordinates: **what boundary is being designed** and **which view of that boundary is open**. Data, behavior, state, collaboration, deployment and evidence describe different aspects. They are not rungs on a hierarchy.

A useful design pyramid is:

| Level | Responsibility | Example |
| --- | --- | --- |
| Primitive | An operation whose semantics are directly defined by the engine | Equality, integer addition, record field access |
| Component | A reusable contract and its internal implementation; may contain other components | Access decision, reservation owner |
| Application | An independently runnable application with an operational boundary | Pharmacy management |
| System | Applications coordinated to achieve a larger purpose | Hospital information system |
| System of systems | Independently governed systems collaborating through agreements and interfaces | National exchange involving healthcare, education and agriculture |

These are responsibility levels, not five compulsory nesting depths. A reusable component can contain several layers of components. A system may reference an existing shared service without containing or duplicating it. An entire system can eventually be exposed as a reusable unit at a larger boundary while preserving its ownership and operational independence.

A **primitive made from primitives** is a *composite operation* or *composite component*. Calling it a new engine primitive would hide the distinction between composition and extending the kernel. A **component of components** remains a component at a deeper nesting level. Display kind, containment depth and execution capabilities separately.

An authorization capability is a component, not an engine primitive. Authentication establishes identity; authorization evaluates whether a subject may act on a resource in a context. Profiles, roles, permissions and scopes are authored domain concepts or policy data. Their presence in a diagram alone does not establish a working identity or security mechanism.

## Top-down design

1. Declare the target boundary and purpose: for example, a hospital ecosystem or reusable reservation component.
2. Define the public contract, ownership, assumptions and independent acceptance criteria.
3. Decompose responsibilities into smaller units. Reuse compatible definitions and identify the parts still needing design.
4. Refine each part until it is expressible through supported primitives or explicit external adapters.
5. Verify the parts, assemble exact versions, and verify the original larger contract.

Unimplemented parts in a future hierarchy editor must be visibly marked as placeholders. A placeholder may help planning but cannot silently pass execution or deployment checks. There is no automatic inference of an implementation from its name.

## Bottom-up design

1. Select primitives, types or existing tested components.
2. Compose a useful behavior and define its public interface.
3. Test it against independent expectations and publish an exact reusable version.
4. Instantiate it inside a larger boundary and bind its ports explicitly.
5. Repeat, adding state ownership, message contracts and operational boundaries where required.

The two flows meet at the same contract. A team can plan the larger system top-down while implementing and testing its parts bottom-up. Neither direction requires hiding the reusable catalog.

## Intended hierarchy editor

The eventual creation flow should ask **what are you designing?**, then start with an empty artifact of that kind. It should keep these visible:

- Artifact kind and supported capabilities: design, evaluate, simulate, generate or deploy only when implemented.
- Current containment path, such as `National exchange / Health ecosystem / Pharmacy / Reservation`.
- Public interface of the current boundary and a separate view of its internals.
- Definition versus instance, exact version references, and references to independently owned external services.
- Top-down placeholders versus implemented parts, with unresolved obligations visible before verification.
- A catalog available at every level, filtered by compatibility rather than an artificial rule that only adjacent levels may connect.

The path above is a target interaction, not an existing stored hierarchy. A unified artifact registry, project memberships, parent/child relationships, application/system contracts and navigation through those boundaries are required before it can be implemented honestly. Project membership must not imply shared ownership or copy a shared service into every parent.

## What v0.7.1 implements

- The design-context strip states the actual artifact or view currently open and provides both design-flow explanations.
- New application projects, pure components, data contracts, state machines and message models start empty. Examples are loaded explicitly. Required metadata and ordinary experiment settings remain; no sample design is injected.
- The main navigation and design/deployment side panels can be collapsed. Diagrams have expand/restore controls, and Escape restores the normal view. These presentation choices do not modify artifacts or execution semantics.
- Existing component, data, state and message catalogs remain shared and available. Those workbench drafts are currently separate from an application project; creating a project does not silently delete an unrelated draft or catalog entry. Use the corresponding New action for a new definition.

Nested pure components are executable within the existing depth and work bounds. State machines use pure command decisions. Message models connect state owners through explicit typed delivery. The older timed application/deployment model remains a separate model. Reusable generated application/system packages, a common project tree and navigation through recursively nested systems are not yet implemented.

## Workflow composition in v0.8

Workflows add a concrete reusable root contract that contains typed task nodes and exact references to pure decisions and owned-state components. Its execution follows declared control flow, pauses for explicit responses, and accepts declared interrupt signals. The workbench exposes the workflow, referenced decisions, their nested definitions and primitive operations for inspection. Exported dependencies can be reused separately; the whole workflow can be instantiated with different inputs and scenarios.

This is a workflow-local composition boundary. It does not create a common parent/child project registry, imply nested workflow calls, execute external medical services or generate applications. The [reception replication guide](HOSPITAL_RECEPTION_REPLICATION.md) distinguishes the working reference from those future capabilities.

The next UX/model milestone should extend these explicit identities into the common artifact hierarchy before coupling execution to hardware, networks and power simulation. See [COMPOSITION_ARCHITECTURE.md](COMPOSITION_ARCHITECTURE.md) for the broader architecture and the individual execution contracts for current limits.
