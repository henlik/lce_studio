// One catalog drives navigation, implementation status and the product blueprint.
// Status describes the scope in `today`, not the completeness of the whole vision.
export type Workspace = 'design' | 'plan' | 'deployment' | 'results' | 'components' | 'data' | 'state' | 'messages' | 'workflows' | 'inspection' | 'system';
export type StudioView = Workspace | 'blueprint';
export type CapabilityStatus = 'available' | 'partial' | 'planned';
export type PlaneID = 'engineering' | 'runtime' | 'foundations';
export interface Capability {
  id: string; label: string; status: CapabilityStatus; purpose: string;
  today: string; next: string; acceptance: string; dependsOn: string[];
  view?: Workspace; action?: 'catalog';
}
export interface BlueprintGroup { id: string; label: string; capabilities: Capability[] }
export interface BlueprintPlane { id: PlaneID; label: string; description: string; groups: BlueprintGroup[] }
export const statusLabels: Record<CapabilityStatus, string> = {available: 'Available', partial: 'Partial', planned: 'Planned'};

export const blueprint: BlueprintPlane[] = [
  {id:'engineering',label:'Engineering plane',description:'Define intent, build reusable components, assemble systems and establish evidence before release.',groups:[
    {id:'components',label:'Components',capabilities:[
      {id:'component-behavior',label:'Interface & behavior',status:'available',view:'components',purpose:'Create reusable pure components from primitive operations and exact dependencies.',today:'Typed ports, composite evaluation, tests and versioned bundles are available.',next:'Resolve every component aspect through one selected component identity.',acceptance:'An existing pure component can be composed, tested and installed without changing its contract.',dependsOn:[]},
      {id:'component-data',label:'Data',status:'available',view:'data',purpose:'Define reusable records, entities, keys and relationships.',today:'Data contracts, fixtures and compatibility checks have their own editor.',next:'Bind owned data contracts to the selected component and system provider.',acceptance:'An entity and relationship can be authored, validated and reused as an exact data contract.',dependsOn:[]},
      {id:'component-state',label:'State',status:'available',view:'state',purpose:'Define owned state, commands and transition rules.',today:'Owned-state scenarios, receipts and decision dependencies are executable.',next:'Connect owner lifetimes to unified component instances.',acceptance:'Command replay and rejected transitions retain the expected state and receipts.',dependsOn:['component-behavior','component-data']},
      {id:'component-messages',label:'Messages',status:'available',view:'messages',purpose:'Define component message contracts and delivery behavior.',today:'Typed owner routes, delivery retries and acknowledgements run as bounded experiments.',next:'Bind these requirements to concrete broker adapters in system designs.',acceptance:'Duplicate delivery and retry scenarios retain the declared business result.',dependsOn:['component-state']},
      {id:'component-workflows',label:'Workflows',status:'available',view:'workflows',purpose:'Compose tasks, decisions, owned commands and handoffs.',today:'Staged workflow authoring, interactive tasks and bounded execution are available.',next:'Execute nested workflow calls and connect their resource costs to system simulation.',acceptance:'Reception and service-desk reference workflows run with independently checked expectations.',dependsOn:['component-behavior','component-state']},
      {id:'component-context',label:'Unified component context',status:'planned',purpose:'Own all aspects under one selected, versioned component.',today:'The editors above remain independent artifacts; menu grouping does not bind them automatically.',next:'Introduce an aspect manifest, ownership rules, navigation into referenced definitions and compatibility checks.',acceptance:'Selecting Reception exposes only its linked data, state, messages and workflows; a new version preserves exact dependencies.',dependsOn:['system-model']},
    ]},
    {id:'design',label:'Design',capabilities:[
      {id:'requirements',label:'Requirements',status:'partial',view:'plan',purpose:'Capture intent and measurable acceptance conditions.',today:'Project requirements and bounded simulation checks are available.',next:'Link requirement IDs to system claims, implementation evidence and runtime monitors.',acceptance:'A requirement can be traced to its evaluator and the evidence supporting a verdict.',dependsOn:['system-model']},
      {id:'system-model',label:'System architecture',status:'partial',view:'system',purpose:'Assemble exact component references, bindings, contracts and deployment plans.',today:'The shared system schema, structured editor, PostgreSQL persistence and evidence snapshots are available.',next:'Resolve catalog definitions, verify compatible ports and execute linked engines together.',acceptance:'Changing a referenced artifact detects unresolved or incompatible dependencies before execution.',dependsOn:['local-catalog']},
      {id:'application-flow',label:'Application flow',status:'available',view:'design',purpose:'Explore processing flows and resource demand.',today:'The application canvas runs bounded timed processing simulations.',next:'Derive demand from actual component behavior instead of a separate timing model.',acceptance:'The existing timed simulation preserves routing, retries and independent requirements.',dependsOn:[]},
      {id:'service-bindings',label:'Databases & brokers',status:'partial',purpose:'Bind component requirements to storage, cache, routing and messaging services.',today:'The system model records resource references; actual providers are not provisioned.',next:'Add typed provider capabilities and executable PostgreSQL, cache and broker adapters.',acceptance:'An incompatible provider is rejected and a supported binding executes through its adapter.',dependsOn:['system-model','component-context']},
      {id:'deployment-model',label:'Deployment planning',status:'partial',view:'deployment',purpose:'Model sites, compute resources, networks and placement before deployment.',today:'Hosts, links, geographic placement, outages and resource experiments are available in the timed model.',next:'Link unified designs and add calibrated replication, consistency, battery and multi-hop network models.',acceptance:'A site outage produces traceable application consequences and reports unmet placement assumptions.',dependsOn:['system-model']},
      {id:'experiments',label:'Experiments & results',status:'partial',view:'results',purpose:'Test workload, resource and failure scenarios against a design.',today:'Timed simulation runs, trace replay and results are stored for the application model.',next:'Use shared scenarios across component behavior, data, messages and deployment.',acceptance:'One reproducible scenario links a deployment event to its application-level outcome.',dependsOn:['deployment-model','system-model']},
      {id:'inspection',label:'Inspection lab',status:'available',view:'inspection',purpose:'Explore existing capabilities with independent reference exercises.',today:'Reference exercises and separate manual UX checks span the existing engines.',next:'Add each new integrated capability to the inspection pack.',acceptance:'Expected rejection is distinguished from successful execution and missing evidence.',dependsOn:[]},
    ]},
    {id:'implementation',label:'Implementation',capabilities:[
      {id:'implementation-workspace',label:'Developers & AI agents',status:'planned',purpose:'Turn an approved design into reviewable source changes.',today:'Developers and agents can work outside the studio; there is no in-studio implementation workflow.',next:'Add repository connections, scoped work items, agent permissions and model-to-code mappings.',acceptance:'A task produces a reviewable change linked to its design revision without changing its acceptance criteria.',dependsOn:['system-model','security']},
      {id:'sandbox',label:'Build & sandbox',status:'planned',purpose:'Build and exercise implementations in an isolated environment.',today:'The reception reference executes Go methods and PostgreSQL queries; it is not a general sandbox.',next:'Introduce bounded isolated runners, reproducible build inputs and artifact identification.',acceptance:'Untrusted implementation code cannot access studio credentials or exceed its execution budget.',dependsOn:['implementation-workspace','security']},
    ]},
    {id:'assurance',label:'Assurance & release',capabilities:[
      {id:'conformance',label:'Contracts & conformance',status:'partial',view:undefined,purpose:'Compare model and implementation observations against independent claims.',today:'Design → System architecture includes four executable checks and seven reception scenarios with saved evidence.',next:'Generalize adapters, claim scopes and time bounds; record implementation identity and assumption evidence.',acceptance:'Correct code passes, an intentional defect violates its claim and lost evidence remains inconclusive.',dependsOn:['system-model']},
      {id:'release-gate',label:'Release authority',status:'planned',purpose:'Authorize identifiable artifacts with contracts, evidence and adaptation limits.',today:'Evidence hashes exist; signed releases and promotion policies are not implemented.',next:'Add provenance, signatures, trust roots and evidence-gated promotion.',acceptance:'A release with mismatched artifacts, failing checks or an unauthorized signer is rejected.',dependsOn:['sandbox','conformance','security']},
    ]},
  ]},
  {id:'runtime',label:'Runtime plane',description:'Observe deployed systems and adapt only within engineering-authorized policies. No live runtime controller is connected yet.',groups:[
    {id:'observe',label:'Observe & verify',capabilities:[
      {id:'fleet',label:'Sites & resources',status:'planned',purpose:'Discover the real computing continuum and resource ownership.',today:'Sites exist in design simulations only; no live fleet inventory is connected.',next:'Add authenticated discovery, resource capability reports and freshness indicators.',acceptance:'Offline or stale nodes cannot be mistaken for available placement capacity.',dependsOn:['security','release-gate']},
      {id:'observer',label:'Observer',status:'planned',purpose:'Collect the execution evidence required by deployed contracts.',today:'Bounded conformance traces exist; continuous runtime telemetry does not.',next:'Add correlated events, resource budgets, offline buffering and explicit loss reporting.',acceptance:'Telemetry loss is visible and observation overhead stays within its configured budget.',dependsOn:['fleet','conformance']},
      {id:'runtime-verifier',label:'Runtime verification',status:'planned',purpose:'Check live claims and assumptions using trustworthy observations.',today:'Three-state verdicts exist only in the bounded engineering experiment.',next:'Add incremental monitors, observation windows and assumption-change detection.',acceptance:'Stale or missing observations produce inconclusive results instead of a healthy status.',dependsOn:['observer','release-gate']},
    ]},
    {id:'adapt',label:'Context & adaptation',capabilities:[
      {id:'context',label:'Contextualisation',status:'planned',purpose:'Interpret workload, topology, power, location and policy together.',today:'No runtime context engine exists.',next:'Build a context model with freshness, uncertainty and constraint provenance.',acceptance:'An insight identifies its supporting evidence, affected claims and applicable constraints.',dependsOn:['runtime-verifier','fleet']},
      {id:'planner',label:'Recovery & optimization',status:'planned',purpose:'Propose resource changes and fault recovery within delegated limits.',today:'No autonomous planner exists.',next:'Start in shadow mode with costed alternatives, cooldowns and policy checks.',acceptance:'A proposed action explains its expected benefit and cannot cross an authorization boundary.',dependsOn:['context','release-gate']},
      {id:'orchestrator',label:'Orchestration',status:'planned',purpose:'Execute approved plans and verify their outcomes.',today:'Deployment is simulated, not executed on live resources.',next:'Add idempotent actuators, fencing, health gates and safe fallback.',acceptance:'Failed or repeated actions cannot leave conflicting controllers in charge.',dependsOn:['planner','security']},
      {id:'updates',label:'Updates & recovery',status:'planned',purpose:'Roll out engineering-authorized updates and fixes safely.',today:'No runtime release delivery or migration management exists.',next:'Add staged rollout, data compatibility checks and explicit rollback limits.',acceptance:'An incompatible migration blocks promotion; a failed rollout follows its approved recovery policy.',dependsOn:['orchestrator','release-gate']},
      {id:'federation',label:'Disconnected federation',status:'planned',purpose:'Let sites operate safely with intermittent connectivity and power.',today:'Outages can be simulated; delegated site controllers and reconciliation are not implemented.',next:'Specify scoped local authority, consistency policies, expiry and reconnection protocols.',acceptance:'Sites perform only permitted disconnected operations and reconcile without silent conflict loss.',dependsOn:['orchestrator','runtime-verifier']},
    ]},
  ]},
  {id:'foundations',label:'Shared foundations',description:'Cross-cutting capabilities used by both planes; security work starts before runtime implementation.',groups:[
    {id:'reuse',label:'Reuse & governance',capabilities:[
      {id:'local-catalog',label:'Local catalog',status:'partial',action:'catalog',purpose:'Install and reuse exact component packages.',today:'The local catalog and portable bundles work; the unified system resolver is not connected.',next:'Resolve component and aspect references by version and digest across designs.',acceptance:'Missing or changed packages cannot silently substitute for a pinned dependency.',dependsOn:[]},
      {id:'marketplace',label:'Community marketplace',status:'planned',purpose:'Publish, discover and review reusable components.',today:'No remote community publishing service is connected.',next:'Add package provenance, review, compatibility metadata and trust policies.',acceptance:'A package is verified before installation and its permissions are visible.',dependsOn:['local-catalog','security','release-gate']},
      {id:'security',label:'Security & trust',status:'partial',purpose:'Enforce identity, authorization, isolation and trustworthy changes by default.',today:'Local API safeguards and isolated synthetic experiment schemas exist; accounts and production authorization do not.',next:'Define the threat model, workload identities, tenant boundaries, least privilege and signing-key lifecycle.',acceptance:'Unauthorized data access, execution and release promotion are rejected and audited.',dependsOn:[]},
      {id:'evidence-governance',label:'Evidence & provenance',status:'partial',purpose:'Trace intent through implementation, release and runtime decisions.',today:'Runs store design and contract snapshots with hashes; release provenance is not yet recorded.',next:'Link source revisions, build artifacts, signatures and runtime observations with retention policies.',acceptance:'Every deployed release and adaptation can be traced to its authority and supporting evidence.',dependsOn:['conformance','security']},
    ]},
  ]},
];

export const capabilities = blueprint.flatMap(plane => plane.groups.flatMap(group => group.capabilities));
export function capabilityLocation(id: string) {
  for (const plane of blueprint) for (const group of plane.groups) {
    const capability = group.capabilities.find(item => item.id === id);
    if (capability) return {plane, group, capability};
  }
  return undefined;
}
export function workspaceLocation(view: StudioView) {
  return capabilities.find(item => item.view === view)?.id;
}
export function canOpenCapability(capability: Capability): boolean {
  return capability.status !== 'planned' && (!!capability.view || !!capability.action || capability.id === 'conformance');
}
export const milestones = [
  {id:'M1',name:'Connect the engineering model',features:['component-context','system-model','service-bindings'],gate:'A selected component owns its aspects, references resolve, and compatible system connections execute.'},
  {id:'M2',name:'Generalize conformance',features:['requirements','deployment-model','experiments','conformance'],gate:'The same scenarios and independent claims evaluate linked models and identifiable implementations.'},
  {id:'M3',name:'Build and authorize releases',features:['implementation-workspace','sandbox','release-gate'],gate:'An isolated build produces an identifiable, authorized artifact with verified provenance and evidence.'},
  {id:'M4',name:'Observe real execution',features:['fleet','observer','runtime-verifier'],gate:'Runtime evidence detects violations, changed assumptions and observation gaps without taking corrective actions.'},
  {id:'M5',name:'Evaluate adaptation in shadow mode',features:['context','planner'],gate:'Proposed adaptations respect policies and are evaluated before automatic execution is enabled.'},
  {id:'M6',name:'Enable bounded autonomy',features:['orchestrator','updates','federation'],gate:'Approved recovery and upgrades remain safe through retries, partitions and reconnection.'},
];
