import type {SystemDesign} from './systemTypes';
export interface SourcePosition {line:number;column:number}
export interface SourceDiagnostic extends SourcePosition {path:string;message:string}
export interface SourceCompilation {language:string;valid:boolean;design?:SystemDesign;diagnostics:SourceDiagnostic[];locations:Record<string,SourcePosition>}
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
