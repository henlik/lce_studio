// Roadmap v1: checkpoint weights are estimates of scope, not measured engineering effort.
export interface ProgressCheckpoint {id:string; acceptance:string; done:boolean; evidence:string[]}
export interface ProgressMilestone {id:string; weight:number; checkpoints:ProgressCheckpoint[]}
const gate=(id:string,acceptance:string,...evidence:string[]):ProgressCheckpoint=>({id,acceptance,done:evidence.length>0,evidence});
export const progressMilestones:ProgressMilestone[]=[
 {id:'M1',weight:25,checkpoints:[
  gate('shared-model','Persist the shared design, contract and separate deployment schema.','internal/system/types.go','internal/store/systems_test.go'),
  gate('direct-resolution','Reject missing or changed direct catalog references before a run.','internal/system/resolution_test.go'),
  gate('aspect-ownership','Install immutable component definitions with pinned aspects and open their editors in context.','internal/store/definitions_test.go','docs/COMPONENT_DEFINITIONS.md'),
  gate('compatible-closure','Validate full dependency closures and compatible ports across aspects.','internal/store/system_compatibility_test.go','internal/system/connections_test.go','docs/SYSTEM_CONNECTIONS.md'),
  gate('joint-execution','Execute connected component aspects and typed provider bindings together.')
 ]},
 {id:'M2',weight:20,checkpoints:[
  gate('bounded-conformance','Run and persist the bounded reception model/Go/PostgreSQL comparison.','internal/conformance/run_test.go'),
  gate('negative-controls','Distinguish a deliberate defect from missing observation evidence.','scripts/acceptance-system.mjs'),
  gate('general-adapters','Evaluate user-defined designs through identifiable implementation adapters.'),
  gate('shared-scenarios','Apply shared workload and failure scenarios across linked engines.'),
  gate('calibrated-continuum','Validate resource, network, power and consistency assumptions against measurements.')
 ]},
 {id:'M3',weight:20,checkpoints:[
  gate('identity-isolation','Enforce identities, tenant boundaries and scoped implementation permissions.'),
  gate('repository-work','Link developer and agent changes to approved design revisions.'),
  gate('isolated-build','Run reproducible builds within enforced isolation and resource budgets.'),
  gate('provenance','Bind source, build artifacts and evidence with verifiable provenance.'),
  gate('release-authority','Reject unauthorized or nonconforming signed releases.')
 ]},
 {id:'M4',weight:15,checkpoints:[
  gate('fleet','Discover authenticated resources and distinguish stale capacity.'),
  gate('observer','Collect correlated execution metadata within an overhead budget.'),
  gate('evidence-gaps','Buffer observations offline and explicitly report loss.'),
  gate('runtime-claims','Evaluate live claims and changing assumptions with three-state verdicts.'),
  gate('runtime-validation','Validate observer and monitor behavior under real failures.')
 ]},
 {id:'M5',weight:10,checkpoints:[
  gate('context','Track context freshness, uncertainty and supporting evidence.'),
  gate('candidate-plans','Generate costed recovery and optimization alternatives.'),
  gate('policy-bounds','Reject plans outside engineering delegation.'),
  gate('shadow-evaluation','Evaluate proposed changes before activating them.'),
  gate('stability','Demonstrate cooldowns and stable decisions under changing load.')
 ]},
 {id:'M6',weight:10,checkpoints:[
  gate('actuation','Execute bounded idempotent actions with fencing.'),
  gate('rollout','Verify staged upgrades, schema compatibility and recovery limits.'),
  gate('offline-authority','Enforce disconnected authority scope and expiry.'),
  gate('reconciliation','Reconcile returning sites without silent conflict loss.'),
  gate('acceptance','Pass end-to-end security and recovery trials across constrained sites.')
 ]}
];
export function calculateProgress(items:ProgressMilestone[]){
 if(items.reduce((sum,m)=>sum+m.weight,0)!==100)throw new Error('Roadmap weights must total 100');
 const ids=new Set<string>();
 const milestones=items.map(m=>{
  if(ids.has(m.id)||m.weight<=0||!m.checkpoints.length)throw new Error('Invalid milestone');
  ids.add(m.id);
  const gates=new Set<string>();
  for(const c of m.checkpoints){
   if(gates.has(c.id)||!c.acceptance.trim()||(c.done&&!c.evidence.some(e=>e.trim())))throw new Error('Every completed checkpoint requires evidence');
   gates.add(c.id);
  }
  const percentage=100*m.checkpoints.filter(c=>c.done).length/m.checkpoints.length;
  return {...m,percentage};
 });
 return {milestones,total:Math.round(milestones.reduce((sum,m)=>sum+m.weight*m.percentage/100,0)*10)/10};
}
export const roadmapProgress=calculateProgress(progressMilestones);
export const progressHistory=[
 {date:'2026-11-14',percentage:18,reason:'Retrospective v1 baseline: shared model, direct resolution and bounded conformance with negative controls.'},
 {date:'2026-11-14',percentage:23,reason:'Versioned component ownership and contextual aspect editing (+5 percentage points).'},
 {date:'2026-11-14',percentage:23,reason:'Added manifest dependency inspection and typed aspect links. Compatibility gate remains open for system-wide connections; no additional checkpoint credit.'},
 {date:'2026-11-14',percentage:28,reason:'Completed static system-connection compatibility and closure checks with positive and negative reception-to-consultation evidence (+5 percentage points).'},
 {date:'2026-11-14',percentage:28,reason:'Added bounded pure component execution, token traces and saved replayable snapshots. Stateful/provider execution remains unfinished; no additional checkpoint credit.'},
 {date:'2026-11-14',percentage:28,reason:'Added single-command state handoffs and exact modeled-state provider bindings with commit/denial evidence. Multi-command, message/workflow and live-provider integration remain open; no additional checkpoint credit.'},
 {date:'2026-11-14',percentage:28,reason:'Added shared graph scenarios with repeated commands, explicit faults/restarts, blocked transfers and independent expectations. Broader joint-engine and live-provider checkpoints remain open; no additional checkpoint credit.'},
 {date:'2026-11-14',percentage:28,reason:'Corrected Design routing and library categories; separated timing simulation navigation and profiles. UX correction adds no engine checkpoint credit.'},
 {date:'2026-11-14',percentage:28,reason:'Added UI/API catalog-independent replay verification, stored plan/report hashes, bounded mismatch inspection and immutable verification history. Reproducibility does not close joint-engine or implementation-conformance checkpoints; no additional credit.'},
 {date:'2026-11-14',percentage:28,reason:'Added selected graph entry commands and explicit owner-command scenario steps sharing revisions and receipts, typed configuration, reference exercise and replay. Message/workflow and live-provider integration remain open; no additional checkpoint credit.'},
 {date:'2026-11-14',percentage:28,reason:'Connected pinned message actors to shared state owners, added the durable modeled broker, explicit transport scenarios, journey traces and portable replay. Workflow, continuum and live implementation/provider integration remain open; no additional checkpoint credit.'}
];
export const progressMethod='Roadmap v1: six weighted milestones, five equal checkpoints within each. Only accepted checkpoints count. This is a scoped planning estimate, not test coverage or production readiness. Scope or weights changes require a new baseline; menu entries earn no credit.';
