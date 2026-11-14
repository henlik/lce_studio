import type {AspectEndpoint,AspectPort} from './definitionTypes';
export interface SystemDesign {
 schemaVersion: number; id: string; name: string; revision: string;
 executionProviders?: {id:string;kind:string;version:string}[];
 components: ComponentInstance[]; connections: SystemConnection[]; deploymentPlans: DeploymentPlan[]; contract: SystemContract;
}
export interface WorkflowOwnerBinding {slot:string;ownerId:string;componentId:string;stateSlot:string}
export interface WorkflowMessageBinding {slot:string;nodeId:string;channelComponentId:string;channelSlot:string;routeId:string;sourceNodeId:string;response:Record<string,string>}
export interface ComponentInstance {workflowOwners?:WorkflowOwnerBinding[];workflowMessages?:WorkflowMessageBinding[]; id:string; name:string; definition:{kind?:'component'|'definition';id:string;version:string}; artifacts:{kind:string;id:string;version:string}[]; bindings:{requirement:string;resourceId:string;requiredCapabilities?:string[]}[];messageOwners?:{slot:string;instanceId:string;componentId:string;stateSlot:string}[];stateEntries?:{slot:string;command:string}[] }
export interface SystemConnection {id:string; source:string; target:string; kind:string;sourcePort?:AspectEndpoint;targetPort?:AspectEndpoint}
export interface DeploymentPlan {id:string; name:string; resources:{id:string;kind:string}[]; placements:{componentId:string;resourceId:string}[]}
export interface SystemContract {id:string; revision:string; claims:{id:string;kind:string;description:string}[]; assumptions:{id:string;description:string}[]}
export interface SystemVerdict {claimId:string;status:string;message:string;evidenceSequences:number[]}
export interface SystemEvent {sequence:number;kind:string;requestId:string;encounterId?:string;detail?:string}
export interface SystemCatalogEntry {kernel?:string;kind:string;id:string;version:string;name:string;digest:string;source:string;ready:boolean;issue?:string;aspects?:{slot:string;kind:string;id:string;version:string;digest:string}[]}
export interface SystemPort extends AspectPort {componentId:string;bound?:boolean}
export interface SystemResolution {compatibility?:string;ports?:SystemPort[];connections?:{id:string;status:string;message:string}[];dependencies?:{path:string;kind:string;id:string;version:string;digest:string}[];valid:boolean;issues:{path:string;message:string}[];references:(SystemCatalogEntry & {path:string})[]}
export function emptySystem():SystemDesign {return {schemaVersion:1,id:crypto.randomUUID(),name:'Untitled system',revision:'1.0.0',components:[],connections:[],deploymentPlans:[],contract:{id:'system-contract',revision:'1.0.0',claims:[],assumptions:[]}};}
export const claimKinds = [
 {id:'exactly_one_record',label:'One record per accepted request'},
 {id:'response_matches_record',label:'Successful response matches persisted record'},
 {id:'no_false_success',label:'No success without persistence'},
 {id:'required_evidence',label:'Required execution evidence is present'},
];
// Structural guard protects the editor before server-side semantic validation.
export function parseSystemImport(source:string):SystemDesign {
 const value:unknown=JSON.parse(source);
 const object=(x:unknown):x is Record<string,unknown>=>!!x&&typeof x==='object'&&!Array.isArray(x);
 const stringFields=(x:unknown,fields:string[])=>object(x)&&fields.every(key=>typeof x[key]==='string');
 const endpoint=(x:unknown)=>stringFields(x,['slot','interface','port']);
 const arrayOf=(x:unknown,check:(v:unknown)=>boolean)=>Array.isArray(x)&&x.every(check);
 if(!object(value)||value.schemaVersion!==1||!stringFields(value,['id','name','revision'])||
 (value.executionProviders!==undefined&&!arrayOf(value.executionProviders,p=>stringFields(p,['id','kind','version'])))||
 !arrayOf(value.components,c=>object(c)&&stringFields(c,['id','name'])&&object(c.definition)&&stringFields(c.definition,['id','version'])&&(!('kind' in c.definition)||c.definition.kind==='component'||c.definition.kind==='definition')&&arrayOf(c.artifacts,a=>stringFields(a,['kind','id','version']))&&arrayOf(c.bindings,b=>object(b)&&stringFields(b,['requirement','resourceId'])&&(b.requiredCapabilities===undefined||arrayOf(b.requiredCapabilities,x=>typeof x==='string')))&&(c.workflowOwners===undefined||arrayOf(c.workflowOwners,o=>stringFields(o,['slot','ownerId','componentId','stateSlot'])))&&(c.workflowMessages===undefined||arrayOf(c.workflowMessages,r=>object(r)&&stringFields(r,['slot','nodeId','channelComponentId','channelSlot','routeId','sourceNodeId'])&&object(r.response)&&Object.values(r.response).every(v=>typeof v==='string')))&&(c.messageOwners===undefined||arrayOf(c.messageOwners,o=>stringFields(o,['slot','instanceId','componentId','stateSlot'])))&&(c.stateEntries===undefined||arrayOf(c.stateEntries,e=>stringFields(e,['slot','command']))))||
 !arrayOf(value.connections,c=>object(c)&&stringFields(c,['id','source','target','kind'])&&((c.sourcePort===undefined&&c.targetPort===undefined)||(endpoint(c.sourcePort)&&endpoint(c.targetPort))))||
 !arrayOf(value.deploymentPlans,p=>object(p)&&stringFields(p,['id','name'])&&arrayOf(p.resources,r=>stringFields(r,['id','kind']))&&arrayOf(p.placements,r=>stringFields(r,['componentId','resourceId'])))||
 !object(value.contract)||!stringFields(value.contract,['id','revision'])||!arrayOf(value.contract.claims,c=>stringFields(c,['id','kind','description']))||!arrayOf(value.contract.assumptions,a=>stringFields(a,['id','description']))) {
  throw new Error('Import requires a complete schema version 1 system design. The current draft was kept.');
 }
 return value as unknown as SystemDesign;
}
// Go encodes nil slices as null. Normalize only known collection fields; the
// parser still rejects malformed objects and missing scalar identifiers.
export function normalizeSystemResponse(value:unknown):SystemDesign {
 const object=(x:unknown):x is Record<string,unknown>=>!!x&&typeof x==='object'&&!Array.isArray(x);
 if(!object(value))return parseSystemImport(JSON.stringify(value));
 const list=(x:unknown)=>x==null?[]:x;
 const mapList=(x:unknown,fn:(item:unknown)=>unknown)=>Array.isArray(x)?x.map(fn):list(x);
 const normalized={...value,
  components:mapList(value.components,c=>object(c)?{...c,artifacts:list(c.artifacts),bindings:list(c.bindings)}:c),
  connections:list(value.connections),
  deploymentPlans:mapList(value.deploymentPlans,p=>object(p)?{...p,resources:list(p.resources),placements:list(p.placements)}:p),
  contract:object(value.contract)?{...value.contract,claims:list(value.contract.claims),assumptions:list(value.contract.assumptions)}:value.contract,
 };
 return parseSystemImport(JSON.stringify(normalized));
}
