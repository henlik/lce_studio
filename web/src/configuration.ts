import {workflowFlowLayout} from './workflowPresentation.ts';
import {workflowStages,withinStage,stagePath} from './workflowStages.ts';
import type {WorkflowDefinition} from './workflowTypes';
export interface ConfigProject {language:string;entry:string;files:Record<string,string>;parameters?:Record<string,unknown>;lock?:unknown}
export interface Span {file:string;start:number;end:number;line:number;column:number}
export interface ConfigDiagnostic extends Span {code:string;path?:string;message:string}
export interface Artifact {symbol:string;kind:string;id:string;version:string;digest:string;body:Record<string,any>;origin:Span}
export interface ConfigDeclaration extends Span {kind:string;name:string;nameSpan:Span;value:{start:number;end:number}}
export interface Compilation {valid:boolean;sourceHash:string;diagnostics:ConfigDiagnostic[];artifacts:Artifact[];experiments:unknown[];lock:unknown;documents:{file:string;declarations:ConfigDeclaration[]}[]}
export interface ConfigReport {passed:boolean;sourceHash:string;cases:{name:string;passed:boolean;error?:string;assertions:{path:string;observed:boolean;passed:boolean;actual:unknown;expected:unknown}[];result?:unknown}[]}
export interface ConfigPackage {schemaVersion:number;id:string;version:string;digest:string;project:ConfigProject;artifacts:Artifact[];report?:ConfigReport}
export interface Completion {label:string;kind:string;type?:string;required?:boolean}
export function emptyConfiguration():ConfigProject{return {language:'1.0',entry:'main.lce',files:{'main.lce':'language "1.0";\nmodule "local.project" version "1.0.0";\n\n-- Add declarations or load the inspection project.\n'}}}
// UTF-8 offsets from Go are translated at the editor boundary. Emoji/non-Latin
// comments must not shift diagnostic, rename or navigation selections.
export function configOffset(text:string,bytes:number){let used=0,index=0;for(const char of text){const size=new TextEncoder().encode(char).length;if(used+size>bytes)break;used+=size;index+=char.length;}return index;}
export function configByteOffset(text:string,index:number){return new TextEncoder().encode(text.slice(0,index)).length;}
export function unlockedProject(p:ConfigProject){const result=structuredClone(p);delete result.lock;return result;}
export function sourceDeclaration(a:Artifact){return a.symbol.split('#')[1]?.replace(/\.p[0-9a-f]{12}$/,'')||'';}
export function configGraph(a:Artifact,stage='',manual=false){const b=a.body;let values:any[]=[],prefix='';switch(a.kind){case 'workflow':case 'timing':values=b.nodes||[];prefix='nodes';break;case 'behavior':values=b.behavior?.nodes||[];prefix='behavior.nodes';break;case 'data':values=b.entities?.length?b.entities:b.records||[];prefix=b.entities?.length?'entities':'records';break;case 'state':values=b.commands||[];prefix='commands';break;case 'messages':values=b.instances||[];prefix='instances';break;case 'definition':values=b.aspects||[];prefix='aspects';break;case 'system':values=b.components||[];prefix='components';break;}
 const workflow={...b,signals:b.signals||[],entry:b.entry||values[0]?.id} as WorkflowDefinition;
 const layout=a.kind==='workflow'?workflowFlowLayout(workflow):null;
 const occupied=new Set<string>();
 const nodes=values.map((v,i)=>({id:String(v.id||v.slot||v.name||i),label:String(v.name||v.slot||v.id||i),position:(()=>{const pos=layout&&!manual?layout.positions[v.id]:v.position;const key=JSON.stringify(pos);if(pos&&!occupied.has(key)){occupied.add(key);return pos};return {x:(i%3)*250,y:Math.floor(i/3)*150}})(),data:{path:`${prefix}[${i}]`,value:v},type:'default',draggable:!!v.position&&(!layout||manual),style:{borderColor:a.kind==='workflow'?'#178477':a.kind==='data'?'#6f5bc4':'#3c719c',borderRadius:v.kind==='decision'||v.kind==='branch'?'24px':'10px'}}));
 const edges:{id:string;source:string;target:string;label?:string;style?:Record<string,string>}[]=[];const add=(source:string,target:string,label='',feedback=false)=>{if(nodes.some(n=>n.id===source)&&nodes.some(n=>n.id===target))edges.push({id:`e${edges.length}`,source,target,label,style:{stroke:feedback?'#c77928':'#178477'}})};
 if(a.kind==='workflow')for(const v of values){if(v.next)add(v.id,v.next);if(v.branch){add(v.id,v.branch.then,`${v.branch.variable}: true`);add(v.id,v.branch.else,`${v.branch.variable}: false`)};if(v.command?.errorNext)add(v.id,v.command.errorNext,'error',true)}
 if(a.kind==='behavior')for(const v of values)for(const [port,binding]of Object.entries(v.inputs||{})as [string,any][]){if(binding.node)add(binding.node,v.id,port)}
 for(const e of b.edges||b.connections||[])add(e.source,e.target,e.kind||'');
 for(const e of b.routes||[])add(e.sourceInstanceId,e.targetInstanceId,e.name||e.id);
 for(const e of b.relationships||[])add(e.fromEntity,e.toEntity,e.name);
 for(const e of b.links||[])add(e.source.slot,e.target.slot,e.id);
 if(layout){for(const e of edges){if([...layout.returns].some(id=>id.startsWith(e.source+':'))&&layout.positions[e.target]?.y<=layout.positions[e.source]?.y)e.style={stroke:'#c77928',strokeDasharray:'6 4'};}
 const groups=workflowStages(workflow);if(!stage&&groups.some(g=>g.path)){
  const groupNodes=groups.map((g,i)=>({id:'stage:'+g.path,label:`${g.name} · ${g.nodes.length} steps`,position:{x:80,y:i*150},data:{path:'',value:{stage:g.path},stage:g.path},type:'default',draggable:false,style:{borderColor:'#178477',borderRadius:'14px'}}));
  const owners=new Map(groups.flatMap(g=>g.nodes.map(id=>[id,'stage:'+g.path]as const)));const seen=new Set<string>();const grouped=edges.flatMap(e=>{const from=owners.get(e.source),to=owners.get(e.target);const key=from+'>'+to;if(!from||!to||from===to||seen.has(key))return [];seen.add(key);return [{...e,id:key,source:from,target:to}]});return {nodes:groupNodes,edges:grouped};
 }
 if(stage&&stage!=='*'){const ids=new Set(values.filter(v=>withinStage(stagePath(v.stage),stage)).map(v=>v.id));return {nodes:nodes.filter(n=>ids.has(n.id)),edges:edges.filter(e=>ids.has(e.source)&&ids.has(e.target))};}
 }
 return {nodes,edges};}
export function scalarProperties(value:Record<string,any>,prefix=''): {path:string;value:string|number|boolean;type:string}[]{const out:{path:string;value:string|number|boolean;type:string}[]=[];for(const [k,v]of Object.entries(value)){const path=prefix?`${prefix}.${k}`:k;if(['string','number','boolean'].includes(typeof v))out.push({path,value:v,type:typeof v});else if(v&&typeof v==='object'&&!Array.isArray(v)&&prefix.split('.').length<4)out.push(...scalarProperties(v,path))}return out;}
