import type {SystemDesign} from './systemTypes';
import type {StateMachineLock,StateResult} from './stateTypes';
import type {ComponentDefinition} from './definitionTypes';
export interface SourcePosition {line:number;column:number}
export interface SourceDiagnostic extends SourcePosition {path:string;message:string}
export interface SourceCompilation {language:string;valid:boolean;design?:SystemDesign;component?:{definition:ComponentDefinition;machine:StateMachineLock;data:unknown;packages:unknown[]};testbench?:{name:string;cases:unknown[]};diagnostics:SourceDiagnostic[];locations:Record<string,SourcePosition>}
export interface SourceTestReport {language:string;source:string;sourceHash:string;compilation:SourceCompilation;passed:boolean;cases:{name:string;passed:boolean;assertions:number;diagnostics:SourceDiagnostic[];result:StateResult}[]}
export function locateSource(locations:Record<string,SourcePosition>,path:string):SourcePosition {
 let result=locations['']||{line:1,column:1},length=-1;
 for(const [key,pos] of Object.entries(locations))if((path===key||path.startsWith(key+'.')||path.startsWith(key+'['))&&key.length>length){result=pos;length=key.length;}
 return result;
}
// Backend columns count Unicode code points; textarea offsets use UTF-16 units.
export function sourceOffset(source:string,position:SourcePosition):number {
 const lines=source.split('\n'),line=Math.max(0,Math.min(position.line-1,lines.length-1));
 return lines.slice(0,line).reduce((n,s)=>n+s.length+1,0)+Array.from(lines[line]||'').slice(0,Math.max(0,position.column-1)).join('').length;
}
export function sourceCanApply(source:string,checkedSource:string,designStamp:string,checkedDesignStamp:string,prepared:boolean):boolean {
 return prepared&&source===checkedSource&&designStamp===checkedDesignStamp;
}
