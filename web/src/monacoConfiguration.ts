import * as monaco from 'monaco-editor/editor';
import 'monaco-editor/features/register.all';
import EditorWorker from 'monaco-editor/editor/editor.worker?worker';
(globalThis as any).MonacoEnvironment={getWorker:()=>new EditorWorker()};
monaco.languages.register({id:'lce',extensions:['.lce'],aliases:['LCE Configuration']});
monaco.languages.setLanguageConfiguration('lce',{
 comments:{lineComment:'--'},brackets:[['{','}'],['[',']'],['(',')']],
 autoClosingPairs:[{open:'{',close:'}'},{open:'[',close:']'},{open:'(',close:')'},{open:'"',close:'"',notIn:['string','comment']}],
 surroundingPairs:[{open:'{',close:'}'},{open:'[',close:']'},{open:'(',close:')'},{open:'"',close:'"'}],
 indentationRules:{increaseIndentPattern:/^.*[\{\[\(]\s*$/,decreaseIndentPattern:/^\s*[\}\]\)]/},
});
monaco.languages.setMonarchTokensProvider('lce',{
 keywords:['language','module','version','import','as','with','parameter','fixture','experiment','data','behavior','state','messages','workflow','definition','system','timing'],
 constructors:['lock','ref','bundle','use','aspect','target','record','value','integer','boolean','string','body','merge','concat','get'],
 tokenizer:{root:[[/--.*$/,'comment'],[/"(?:\\.|[^"\\])*"/,'string'],[/-?\d+(?:\.\d+)?/,'number'],[/\b(?:true|false|null)\b/,'constant'],[/[A-Za-z_][\w]*/,{cases:{'@keywords':'keyword','@constructors':'type.identifier','@default':'identifier'}}],[/[{}()[\]]/,'@brackets'],[/[=;,:.]/,'delimiter']]},
});
monaco.editor.defineTheme('lce-studio',{base:'vs-dark',inherit:true,rules:[{token:'keyword',foreground:'86BFFF'},{token:'type.identifier',foreground:'79D6C5'},{token:'string',foreground:'E7BA8E'},{token:'comment',foreground:'89A58A'}],colors:{'editor.background':'#18232F','editorGutter.background':'#18232F','editor.lineHighlightBackground':'#223140','editorLineNumber.foreground':'#728493','editor.selectionBackground':'#355374','editorCursor.foreground':'#B1E5D9'}});
export {monaco};
