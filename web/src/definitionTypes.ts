export interface ComponentAspect {slot:string;kind:string;id:string;version:string;digest:string}
export interface ComponentDefinition {schemaVersion:number;id:string;version:string;name:string;description:string;aspects:ComponentAspect[]}
export interface DefinitionRecord {definition:ComponentDefinition;digest:string}
export interface AspectContext {id:string;name:string;version:string;slot:string;workspace:string}
export interface AspectEditorRequest {workspace:string;body:Record<string,unknown>;context:AspectContext}
export function emptyDefinition():ComponentDefinition{return {schemaVersion:1,id:'local.component-'+crypto.randomUUID().slice(0,8),version:'1.0.0',name:'Untitled component',description:'',aspects:[]};}
export function parseDefinition(source:string):ComponentDefinition{
 const d=JSON.parse(source);
 const strings=(v:any,keys:string[])=>v&&typeof v==='object'&&!Array.isArray(v)&&keys.every(k=>typeof v[k]==='string');
 if(!d||d.schemaVersion!==1||!strings(d,['id','version','name','description'])||!Array.isArray(d.aspects)||!d.aspects.every((a:any)=>strings(a,['slot','kind','id','version','digest'])))throw new Error('Invalid component definition format. The current draft was kept.');
 return d;
}
