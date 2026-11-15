import type {Artifact,Compilation,ConfigExperiment} from './configuration';
export function sameTarget(a:Artifact,b:ConfigExperiment['target']){return !!b&&a.kind===b.kind&&a.id===b.id&&a.version===b.version&&a.digest===b.digest}
export function componentTests(c:Compilation|null,a:Artifact){return (c?.experiments||[]).filter(e=>sameTarget(a,e.target))}
export function componentChildren(c:Compilation|null,symbol:string){return (c?.workspace?.nodes.find(n=>n.symbol===symbol)?.children||[]).flatMap(link=>{const artifact=c?.artifacts.find(a=>a.symbol===link.symbol);return artifact?[{artifact,relation:link.relation}]:[]})}
export function componentRoots(c:Compilation|null){const roots=new Set(c?.workspace?.roots||c?.artifacts.map(a=>a.symbol));return (c?.artifacts||[]).filter(a=>roots.has(a.symbol)).sort((a,b)=>{const order=['system','definition','workflow','messages','state','behavior','data','timing'];return order.indexOf(a.kind)-order.indexOf(b.kind)||String(a.body.name||a.id).localeCompare(String(b.body.name||b.id))})}
export function parentCount(c:Compilation|null,symbol:string){return c?.workspace?.nodes.filter(n=>n.children.some(ch=>ch.symbol===symbol)).length||0}
export function matchesComponent(c:Compilation|null,a:Artifact,query:string,seen=new Set<string>()):boolean{if(seen.has(a.symbol))return false;seen.add(a.symbol);const text=[a.body.name,a.id,a.kind,a.origin.file].join(' ').toLowerCase();return text.includes(query.toLowerCase())||componentChildren(c,a.symbol).some(ch=>matchesComponent(c,ch.artifact,query,seen))}
export interface ConfigLocation {file:string;symbol:string;chain:string[];view:string}
export function pushLocation(history:ConfigLocation[],index:number,next:ConfigLocation){const result=history.slice(0,index+1);if(JSON.stringify(result.at(-1))!==JSON.stringify(next))result.push({...next,chain:[...next.chain]});return result.slice(-80)}
// Tolerant lexical context for suggestions while an object is unfinished.
// Strings/comments cannot masquerade as declarations or property delimiters.
export function completionContext(source:string,offset:number){
 const tokens=source.slice(0,offset).match(/--[^\n]*|"(?:\\.|[^"\\])*"|[A-Za-z_][A-Za-z0-9_]*|[{}[\]=,;:]|\S/g)||[];
 const stack:{kind:string;path:string;index:number}[]=[];let declaration='',kind='',last='',before='';
 const declarations=new Set(['data','behavior','state','messages','workflow','definition','system','timing','experiment','fixture']);
 for(const token of tokens){if(token.startsWith('--'))continue;
  if(stack.length===0&&declarations.has(before)&&/^[A-Za-z]/.test(last)){kind=before;declaration=last;}
  if(token==='{'||token==='['){let key=last==='='?before.replace(/^"|"$/g,''):'';const parent=stack.at(-1);const path=parent?(parent.kind==='['?`${parent.path}[${parent.index}]`:[parent.path,key].filter(Boolean).join('.')):'';stack.push({kind:token,path,index:0})}
  else if(token==='}'||token===']')stack.pop();else if(token===','&&stack.at(-1)?.kind==='[')stack.at(-1)!.index++;
  before=last;last=token;
 }
 return {declaration,kind,path:stack.at(-1)?.path||''};
}
