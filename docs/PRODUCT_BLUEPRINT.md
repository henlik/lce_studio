# LCE Studio implementation blueprint

The studio separates engineering authority from deployed execution. This is the
target product structure and a scoped inventory of the current implementation.
It does not claim that planned capabilities exist because they have menu entries.

Generated from [studioBlueprint.ts](../web/src/studioBlueprint.ts), the same
catalog used by the application navigation. Update that catalog as capabilities
change, then run `node --experimental-strip-types scripts/export-blueprint.mjs`.

## Status meanings

- **Available:** usable within the expressly stated scope.
- **Partial:** an implemented foundation with integration or semantics still missing.
- **Planned:** no working control is provided; the menu opens its blueprint details.

Counts in the studio are capability counts, not completion percentages. A status
change requires evidence against the acceptance condition. Developers maintain
these statuses in source control; test results do not automatically promote them.

## Authority and ownership

**Engineering plane:** engineers author reusable components, assemble system
designs, declare deployment environments, run experiments, implement software in
a sandbox, verify conformance and authorize releases. Deployment planning is an
engineering activity; it does not deploy software.

**Runtime plane:** deployed observers and controllers will check claims, interpret
context, propose adaptations and execute only actions permitted by engineering
authority. Runtime controllers must not change their own acceptance criteria,
security policies or release authority. These runtime services are planned.

**Shared foundations:** catalogs, security, trust, evidence and provenance support
both planes. Security work starts with the engineering model, before autonomous
runtime execution is introduced.

Component Data, State, Messages and Workflows are now grouped under Components.
Their artifacts remain independently versioned. Component definitions now own
named aspect pins and open exact artifacts in their editors with return context.
Definitions inspect dependency closures and check declared aspect-link types.
System-wide typed connections and dependency closures are now checked.
Bounded pure and selected-entry state graphs execute with deterministic token traces
and exact offline modeled-state provider bindings. Shared graph scenarios retain state
across repeated commands, faults and restarts with independent expectations. Saved
executions can be replay-verified from their pinned snapshots through the UI/API;
separate immutable checks distinguish reproduction, mismatch and unsupported replay.
Explicit owner-command steps now share state and receipts with graph entry calls.
Message/workflow integration, feedback/delivery scheduling and live provider integration remain unfinished. System architecture currently records references;
the timing/deployment simulator uses its existing project model under Simulation.
Design opens System architecture and its installed library. Library sections distinguish
components, supporting services and actors/workload. The separate timing-profile library
retains its existing simulation behavior; these models are not silently merged.

## Implementation sequence

**Weighted implementation: 28%**

Roadmap v1: six weighted milestones, five equal checkpoints within each. Only accepted checkpoints count. This is a scoped planning estimate, not test coverage or production readiness. Scope or weights changes require a new baseline; menu entries earn no credit.

- 2026-11-14: **18%** — Retrospective v1 baseline: shared model, direct resolution and bounded conformance with negative controls.
- 2026-11-14: **23%** — Versioned component ownership and contextual aspect editing (+5 percentage points).
- 2026-11-14: **23%** — Added manifest dependency inspection and typed aspect links. Compatibility gate remains open for system-wide connections; no additional checkpoint credit.
- 2026-11-14: **28%** — Completed static system-connection compatibility and closure checks with positive and negative reception-to-consultation evidence (+5 percentage points).
- 2026-11-14: **28%** — Added bounded pure component execution, token traces and saved replayable snapshots. Stateful/provider execution remains unfinished; no additional checkpoint credit.
- 2026-11-14: **28%** — Added single-command state handoffs and exact modeled-state provider bindings with commit/denial evidence. Multi-command, message/workflow and live-provider integration remain open; no additional checkpoint credit.
- 2026-11-14: **28%** — Added shared graph scenarios with repeated commands, explicit faults/restarts, blocked transfers and independent expectations. Broader joint-engine and live-provider checkpoints remain open; no additional checkpoint credit.
- 2026-11-14: **28%** — Corrected Design routing and library categories; separated timing simulation navigation and profiles. UX correction adds no engine checkpoint credit.
- 2026-11-14: **28%** — Added UI/API catalog-independent replay verification, stored plan/report hashes, bounded mismatch inspection and immutable verification history. Reproducibility does not close joint-engine or implementation-conformance checkpoints; no additional credit.
- 2026-11-14: **28%** — Added selected graph entry commands and explicit owner-command scenario steps sharing revisions and receipts, typed configuration, reference exercise and replay. Message/workflow and live-provider integration remain open; no additional checkpoint credit.
- 2026-11-14: **28%** — Connected pinned message actors to shared state owners, added the durable modeled broker, explicit transport scenarios, journey traces and portable replay. Workflow, continuum and live implementation/provider integration remain open; no additional checkpoint credit.

### M1. Connect the engineering model

Capabilities: Component definitions, System architecture, Databases & brokers.

**Exit gate:** A selected component owns its aspects, references resolve, and compatible system connections execute.

**Milestone progress: 80% · roadmap weight: 25%**

- [x] Persist the shared design, contract and separate deployment schema. Evidence: [internal/system/types.go](../internal/system/types.go), [internal/store/systems_test.go](../internal/store/systems_test.go).
- [x] Reject missing or changed direct catalog references before a run. Evidence: [internal/system/resolution_test.go](../internal/system/resolution_test.go).
- [x] Install immutable component definitions with pinned aspects and open their editors in context. Evidence: [internal/store/definitions_test.go](../internal/store/definitions_test.go), [docs/COMPONENT_DEFINITIONS.md](../docs/COMPONENT_DEFINITIONS.md).
- [x] Validate full dependency closures and compatible ports across aspects. Evidence: [internal/store/system_compatibility_test.go](../internal/store/system_compatibility_test.go), [internal/system/connections_test.go](../internal/system/connections_test.go), [docs/SYSTEM_CONNECTIONS.md](../docs/SYSTEM_CONNECTIONS.md).
- [ ] Execute connected component aspects and typed provider bindings together.

### M2. Generalize conformance

Capabilities: Simulation requirements, Deployment simulation, Simulation results, Contracts & conformance.

**Exit gate:** The same scenarios and independent claims evaluate linked models and identifiable implementations.

**Milestone progress: 40% · roadmap weight: 20%**

- [x] Run and persist the bounded reception model/Go/PostgreSQL comparison. Evidence: [internal/conformance/run_test.go](../internal/conformance/run_test.go).
- [x] Distinguish a deliberate defect from missing observation evidence. Evidence: [scripts/acceptance-system.mjs](../scripts/acceptance-system.mjs).
- [ ] Evaluate user-defined designs through identifiable implementation adapters.
- [ ] Apply shared workload and failure scenarios across linked engines.
- [ ] Validate resource, network, power and consistency assumptions against measurements.

### M3. Build and authorize releases

Capabilities: Developers & AI agents, Build & sandbox, Release authority.

**Exit gate:** An isolated build produces an identifiable, authorized artifact with verified provenance and evidence.

**Milestone progress: 0% · roadmap weight: 20%**

- [ ] Enforce identities, tenant boundaries and scoped implementation permissions.
- [ ] Link developer and agent changes to approved design revisions.
- [ ] Run reproducible builds within enforced isolation and resource budgets.
- [ ] Bind source, build artifacts and evidence with verifiable provenance.
- [ ] Reject unauthorized or nonconforming signed releases.

### M4. Observe real execution

Capabilities: Sites & resources, Observer, Runtime verification.

**Exit gate:** Runtime evidence detects violations, changed assumptions and observation gaps without taking corrective actions.

**Milestone progress: 0% · roadmap weight: 15%**

- [ ] Discover authenticated resources and distinguish stale capacity.
- [ ] Collect correlated execution metadata within an overhead budget.
- [ ] Buffer observations offline and explicitly report loss.
- [ ] Evaluate live claims and changing assumptions with three-state verdicts.
- [ ] Validate observer and monitor behavior under real failures.

### M5. Evaluate adaptation in shadow mode

Capabilities: Contextualisation, Recovery & optimization.

**Exit gate:** Proposed adaptations respect policies and are evaluated before automatic execution is enabled.

**Milestone progress: 0% · roadmap weight: 10%**

- [ ] Track context freshness, uncertainty and supporting evidence.
- [ ] Generate costed recovery and optimization alternatives.
- [ ] Reject plans outside engineering delegation.
- [ ] Evaluate proposed changes before activating them.
- [ ] Demonstrate cooldowns and stable decisions under changing load.

### M6. Enable bounded autonomy

Capabilities: Orchestration, Updates & recovery, Disconnected federation.

**Exit gate:** Approved recovery and upgrades remain safe through retries, partitions and reconnection.

**Milestone progress: 0% · roadmap weight: 10%**

- [ ] Execute bounded idempotent actions with fencing.
- [ ] Verify staged upgrades, schema compatibility and recovery limits.
- [ ] Enforce disconnected authority scope and expiry.
- [ ] Reconcile returning sites without silent conflict loss.
- [ ] Pass end-to-end security and recovery trials across constrained sites.

Runtime findings return to engineering as evidence and proposed changes.
Source modifications, schema migrations and updated policies pass through the
implementation and release gates. Disconnected runtime autonomy must be scoped
by explicit delegation, consistency rules and expiry behavior.

## Capability inventory

### Engineering plane

Define intent, build reusable components, assemble systems and establish evidence before release.

#### Components

**Interface & behavior — Available** (`component-behavior`)

Create reusable pure components from primitive operations and exact dependencies.

- Today: Typed ports, composite evaluation, tests and versioned bundles are available.
- Next: Resolve every component aspect through one selected component identity.
- Acceptance: An existing pure component can be composed, tested and installed without changing its contract.
- Dependencies: None in this inventory.

**Data — Available** (`component-data`)

Define reusable records, entities, keys and relationships.

- Today: Data contracts, fixtures and compatibility checks have their own editor.
- Next: Bind owned data contracts to the selected component and system provider.
- Acceptance: An entity and relationship can be authored, validated and reused as an exact data contract.
- Dependencies: None in this inventory.

**State — Available** (`component-state`)

Define owned state, commands and transition rules.

- Today: Owned-state scenarios, receipts and decision dependencies are executable.
- Next: Connect owner lifetimes to unified component instances.
- Acceptance: Command replay and rejected transitions retain the expected state and receipts.
- Dependencies: Interface & behavior, Data

**Messages — Available** (`component-messages`)

Define component message contracts and delivery behavior.

- Today: Typed routes bind to existing system owners; explicit dispatch/retry, lost acknowledgements and durable broker restarts run in shared scenarios.
- Next: Add live broker adapters and measured delivery/timing behavior beyond explicit offline scenarios.
- Acceptance: Duplicate delivery and retry scenarios retain the declared business result.
- Dependencies: State

**Workflows — Available** (`component-workflows`)

Compose tasks, decisions, owned commands and handoffs.

- Today: Staged workflow authoring, interactive tasks and bounded execution are available.
- Next: Execute nested workflow calls and connect their resource costs to system simulation.
- Acceptance: Reception and service-desk reference workflows run with independently checked expectations.
- Dependencies: Interface & behavior, State

**Component definitions — Partial** (`component-context`)

Own all aspects under one selected, versioned component.

- Today: Immutable component manifests own digest-pinned aspects and checked links; pure behavior and selected state entry commands execute together with saved token traces; owner commands and broker deliveries share revisions and receipts.
- Next: Integrate workflow adapters, automatic command routing and live provider bindings.
- Acceptance: A selected definition exposes its pinned aspects; opening an editor preserves the source context and changes require a new artifact and definition version.
- Dependencies: System architecture

#### Design

**System architecture — Partial** (`system-model`)

Assemble exact component references, bindings, contracts and deployment plans.

- Today: System designs check typed connections and dependencies; bounded pure/state dataflow and shared multi-command scenarios record routed values, faults, restarts, message journeys and independent assertions.
- Next: Integrate workflows, live providers and feedback/timed delivery scheduling.
- Acceptance: Changing a referenced artifact detects unresolved or incompatible dependencies before execution.
- Dependencies: Installed component library

**Databases & brokers — Partial** (`service-bindings`)

Bind component requirements to storage, cache, routing and messaging services.

- Today: Exact modeled-state@1.0.0 and modeled-broker@1.0.0 bindings execute shared owner/delivery semantics offline; deployment resources are not provisioned.
- Next: Add typed provider capabilities and executable PostgreSQL, cache and broker adapters.
- Acceptance: An incompatible provider is rejected and a supported binding executes through its adapter.
- Dependencies: System architecture, Component definitions

**Inspection lab — Available** (`inspection`)

Explore existing capabilities with independent reference exercises.

- Today: Reference exercises and separate manual UX checks span the existing engines.
- Next: Add each new integrated capability to the inspection pack.
- Acceptance: Expected rejection is distinguished from successful execution and missing evidence.
- Dependencies: None in this inventory.

#### Simulation

**Timing flow — Available** (`application-flow`)

Explore processing flows and resource demand.

- Today: The application canvas runs bounded timed processing simulations.
- Next: Derive demand from actual component behavior instead of a separate timing model.
- Acceptance: The existing timed simulation preserves routing, retries and independent requirements.
- Dependencies: None in this inventory.

**Simulation requirements — Partial** (`requirements`)

Capture intent and measurable acceptance conditions.

- Today: Project requirements and bounded simulation checks are available.
- Next: Link requirement IDs to system claims, implementation evidence and runtime monitors.
- Acceptance: A requirement can be traced to its evaluator and the evidence supporting a verdict.
- Dependencies: System architecture

**Deployment simulation — Partial** (`deployment-model`)

Model sites, compute resources, networks and placement before deployment.

- Today: Hosts, links, geographic placement, outages and resource experiments are available in the timed model.
- Next: Link unified designs and add calibrated replication, consistency, battery and multi-hop network models.
- Acceptance: A site outage produces traceable application consequences and reports unmet placement assumptions.
- Dependencies: System architecture

**Simulation results — Partial** (`experiments`)

Test workload, resource and failure scenarios against a design.

- Today: Timed simulation runs, trace replay and results are stored for the application model.
- Next: Use shared scenarios across component behavior, data, messages and deployment.
- Acceptance: One reproducible scenario links a deployment event to its application-level outcome.
- Dependencies: Deployment simulation, System architecture

#### Implementation

**Developers & AI agents — Planned** (`implementation-workspace`)

Turn an approved design into reviewable source changes.

- Today: Developers and agents can work outside the studio; there is no in-studio implementation workflow.
- Next: Add repository connections, scoped work items, agent permissions and model-to-code mappings.
- Acceptance: A task produces a reviewable change linked to its design revision without changing its acceptance criteria.
- Dependencies: System architecture, Security & trust

**Build & sandbox — Planned** (`sandbox`)

Build and exercise implementations in an isolated environment.

- Today: The reception reference executes Go methods and PostgreSQL queries; it is not a general sandbox.
- Next: Introduce bounded isolated runners, reproducible build inputs and artifact identification.
- Acceptance: Untrusted implementation code cannot access studio credentials or exceed its execution budget.
- Dependencies: Developers & AI agents, Security & trust

#### Assurance & release

**Contracts & conformance — Partial** (`conformance`)

Compare model and implementation observations against independent claims.

- Today: Design → System architecture includes four executable checks and seven reception scenarios with saved evidence.
- Next: Generalize adapters, claim scopes and time bounds; record implementation identity and assumption evidence.
- Acceptance: Correct code passes, an intentional defect violates its claim and lost evidence remains inconclusive.
- Dependencies: System architecture

**Release authority — Planned** (`release-gate`)

Authorize identifiable artifacts with contracts, evidence and adaptation limits.

- Today: Evidence hashes exist; signed releases and promotion policies are not implemented.
- Next: Add provenance, signatures, trust roots and evidence-gated promotion.
- Acceptance: A release with mismatched artifacts, failing checks or an unauthorized signer is rejected.
- Dependencies: Build & sandbox, Contracts & conformance, Security & trust

### Runtime plane

Observe deployed systems and adapt only within engineering-authorized policies. No live runtime controller is connected yet.

#### Observe & verify

**Sites & resources — Planned** (`fleet`)

Discover the real computing continuum and resource ownership.

- Today: Sites exist in design simulations only; no live fleet inventory is connected.
- Next: Add authenticated discovery, resource capability reports and freshness indicators.
- Acceptance: Offline or stale nodes cannot be mistaken for available placement capacity.
- Dependencies: Security & trust, Release authority

**Observer — Planned** (`observer`)

Collect the execution evidence required by deployed contracts.

- Today: Bounded conformance traces exist; continuous runtime telemetry does not.
- Next: Add correlated events, resource budgets, offline buffering and explicit loss reporting.
- Acceptance: Telemetry loss is visible and observation overhead stays within its configured budget.
- Dependencies: Sites & resources, Contracts & conformance

**Runtime verification — Planned** (`runtime-verifier`)

Check live claims and assumptions using trustworthy observations.

- Today: Three-state verdicts exist only in the bounded engineering experiment.
- Next: Add incremental monitors, observation windows and assumption-change detection.
- Acceptance: Stale or missing observations produce inconclusive results instead of a healthy status.
- Dependencies: Observer, Release authority

#### Context & adaptation

**Contextualisation — Planned** (`context`)

Interpret workload, topology, power, location and policy together.

- Today: No runtime context engine exists.
- Next: Build a context model with freshness, uncertainty and constraint provenance.
- Acceptance: An insight identifies its supporting evidence, affected claims and applicable constraints.
- Dependencies: Runtime verification, Sites & resources

**Recovery & optimization — Planned** (`planner`)

Propose resource changes and fault recovery within delegated limits.

- Today: No autonomous planner exists.
- Next: Start in shadow mode with costed alternatives, cooldowns and policy checks.
- Acceptance: A proposed action explains its expected benefit and cannot cross an authorization boundary.
- Dependencies: Contextualisation, Release authority

**Orchestration — Planned** (`orchestrator`)

Execute approved plans and verify their outcomes.

- Today: Deployment is simulated, not executed on live resources.
- Next: Add idempotent actuators, fencing, health gates and safe fallback.
- Acceptance: Failed or repeated actions cannot leave conflicting controllers in charge.
- Dependencies: Recovery & optimization, Security & trust

**Updates & recovery — Planned** (`updates`)

Roll out engineering-authorized updates and fixes safely.

- Today: No runtime release delivery or migration management exists.
- Next: Add staged rollout, data compatibility checks and explicit rollback limits.
- Acceptance: An incompatible migration blocks promotion; a failed rollout follows its approved recovery policy.
- Dependencies: Orchestration, Release authority

**Disconnected federation — Planned** (`federation`)

Let sites operate safely with intermittent connectivity and power.

- Today: Outages can be simulated; delegated site controllers and reconciliation are not implemented.
- Next: Specify scoped local authority, consistency policies, expiry and reconnection protocols.
- Acceptance: Sites perform only permitted disconnected operations and reconcile without silent conflict loss.
- Dependencies: Orchestration, Runtime verification

### Shared foundations

Cross-cutting capabilities used by both planes; security work starts before runtime implementation.

#### Reuse & governance

**Installed component library — Partial** (`local-catalog`)

Install and reuse exact component packages.

- Today: Installed packages and built-in adapters can be selected in System architecture; direct references are resolved by exact version with content integrity checks.
- Next: Extend verified model closures to implementation provenance, signed distribution and remote catalogs.
- Acceptance: Missing or changed packages cannot silently substitute for a pinned dependency.
- Dependencies: None in this inventory.

**Community marketplace — Planned** (`marketplace`)

Publish, discover and review reusable components.

- Today: No remote community publishing service is connected.
- Next: Add package provenance, review, compatibility metadata and trust policies.
- Acceptance: A package is verified before installation and its permissions are visible.
- Dependencies: Installed component library, Security & trust, Release authority

**Security & trust — Partial** (`security`)

Enforce identity, authorization, isolation and trustworthy changes by default.

- Today: Local API safeguards and isolated synthetic experiment schemas exist; accounts and production authorization do not.
- Next: Define the threat model, workload identities, tenant boundaries, least privilege and signing-key lifecycle.
- Acceptance: Unauthorized data access, execution and release promotion are rejected and audited.
- Dependencies: None in this inventory.

**Evidence & provenance — Partial** (`evidence-governance`)

Trace intent through implementation, release and runtime decisions.

- Today: Saved pure/state/message scenarios support catalog-independent replay verification, immutable verification history and bounded report differences. New snapshots retain plan/report hashes; release provenance remains unsigned.
- Next: Link source revisions, build artifacts, signatures and runtime observations with retention policies.
- Acceptance: Every deployed release and adaptation can be traced to its authority and supporting evidence.
- Dependencies: Contracts & conformance, Security & trust

## Using the blueprint

Open **Product blueprint** in the sidebar. Search or filter by plane and status.
Choose **View blueprint** for current scope, next work, dependencies and an
acceptance condition. **Open** appears only for a working destination. The
**Implementation sequence** tab shows milestone gates. Planned runtime entries
open details and never invoke deployment controls. Existing drafts and workspace
IDs are retained across the navigation change.
