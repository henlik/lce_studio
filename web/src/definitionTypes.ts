export interface ComponentAspect {slot:string;kind:string;id:string;version:string;digest:string}
export interface ComponentDefinition {schemaVersion:number;id:string;version:string;name:string;description:string;aspects:ComponentAspect[];links?:AspectLink[]}
export interface DefinitionRecord {definition:ComponentDefinition;digest:string}
export interface AspectContext {id:string;name:string;version:string;slot:string;workspace:string}
export interface AspectEditorRequest {workspace:string;body:Record<string,unknown>;context:AspectContext}
export function emptyDefinition():ComponentDefinition{return {schemaVersion:1,id:'local.component-'+crypto.randomUUID().slice(0,8),version:'1.0.0',name:'Untitled component',description:'',aspects:[]};}
export function parseDefinition(source:string):ComponentDefinition{
 const d=JSON.parse(source);
 const strings=(v:any,keys:string[])=>v&&typeof v==='object'&&!Array.isArray(v)&&keys.every(k=>typeof v[k]==='string');
 if(!d||d.schemaVersion!==1||!strings(d,['id','version','name','description'])||!Array.isArray(d.aspects)||!d.aspects.every((a:any)=>strings(a,['slot','kind','id','version','digest']))||(d.links!==undefined&&(!Array.isArray(d.links)||!d.links.every((l:any)=>strings(l,['id'])&&strings(l.source,['slot','interface','port'])&&strings(l.target,['slot','interface','port'])))))throw new Error('Invalid component definition format. The current draft was kept.');
 return d;
}

export interface AspectEndpoint {slot:string;interface:string;port:string}
export interface AspectLink {id:string;source:AspectEndpoint;target:AspectEndpoint}
export interface AspectPort extends AspectEndpoint {direction:'input'|'output';type:string;recordRef?:{contractId:string;version:string;digest:string;record:string}}
export interface DefinitionCompatibility {valid:boolean;issues:{path:string;message:string}[];ports:AspectPort[];dependencies:{path:string;kind:string;id:string;version:string;digest:string}[]}
export function endpointLabel(p:AspectPort):string{return `${p.slot} / ${p.interface} / ${p.port} · ${p.type}${p.recordRef?' ('+p.recordRef.contractId+'@'+p.recordRef.version+':'+p.recordRef.record+')':''}`;}

// Compare semantic manifest fields; omitted and empty optional links have the
// same Go digest, and JSON property ordering is not part of installation identity.
export function definitionStamp(d:ComponentDefinition):string {
 const endpoint=(e:AspectEndpoint)=>({slot:e.slot,interface:e.interface,port:e.port});
 return JSON.stringify({schemaVersion:d.schemaVersion,id:d.id,version:d.version,name:d.name,description:d.description,aspects:d.aspects.map(a=>({slot:a.slot,kind:a.kind,id:a.id,version:a.version,digest:a.digest})),...(d.links?.length?{links:d.links.map(l=>({id:l.id,source:endpoint(l.source),target:endpoint(l.target)}))}:{})});
}
