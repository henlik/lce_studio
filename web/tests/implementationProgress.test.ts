import test from 'node:test';
import assert from 'node:assert/strict';
import {existsSync} from 'node:fs';
import {calculateProgress,progressMilestones,roadmapProgress,progressHistory} from '../src/implementationProgress.ts';
import {milestones} from '../src/studioBlueprint.ts';
import {parseDefinition,definitionStamp} from '../src/definitionTypes.ts';
test('roadmap score reflects accepted gates, and history matches the current baseline',()=>{
 assert.deepEqual(progressMilestones.map(m=>m.id),milestones.map(m=>m.id));
 assert.equal(roadmapProgress.total,33);
 assert.equal(roadmapProgress.milestones[0]!.percentage,100);
 assert.equal(progressHistory.at(-1)!.percentage,roadmapProgress.total);
 const before=structuredClone(progressMilestones);before[0]!.checkpoints[3]!.done=false;
 assert.equal(calculateProgress(before).total,28);
 before[0]!.checkpoints[2]!.done=false;
 assert.equal(calculateProgress(before).total,23);
 for(const m of progressMilestones)for(const c of m.checkpoints)if(c.done)for(const e of c.evidence)assert.ok(existsSync(new URL('../../'+e,import.meta.url)),e);
});
test('progress cannot advance without evidence and weights cannot drift silently',()=>{
 const noEvidence=structuredClone(progressMilestones);noEvidence[5]!.checkpoints[0]!.done=true;
 assert.throws(()=>calculateProgress(noEvidence),/evidence/);
 const wrongWeight=structuredClone(progressMilestones);wrongWeight[0]!.weight++;
 assert.throws(()=>calculateProgress(wrongWeight),/100/);
 const all=structuredClone(progressMilestones);for(const m of all)for(const c of m.checkpoints){c.done=true;c.evidence=['test'];}
 assert.equal(calculateProgress(all).total,100);
 for(const m of all)for(const c of m.checkpoints)c.done=false;
 assert.equal(calculateProgress(all).total,0);
});
test('definition import rejects malformed ownership without coercing pins',()=>{
 const definition={schemaVersion:1,id:'test.reception',name:'Reception',version:'1.0.0',description:'',aspects:[{slot:'patient',kind:'data',id:'test.patient',version:'1.0.0',digest:'a'.repeat(64)}]};
 assert.deepEqual(parseDefinition(JSON.stringify(definition)),definition);
 for(const bad of [{...definition,aspects:null},{...definition,schemaVersion:2},{...definition,aspects:[{...definition.aspects[0],version:1}]}])assert.throws(()=>parseDefinition(JSON.stringify(bad)),/Invalid/);
});

test('definition import preserves typed link endpoints and rejects malformed links',()=>{
 const d={schemaVersion:1,id:'demo.linked',version:'1.0.0',name:'Linked',description:'',aspects:[],links:[{id:'copy',source:{slot:'a',interface:'outputs',port:'result'},target:{slot:'b',interface:'inputs',port:'value'}}]};
 assert.deepEqual(parseDefinition(JSON.stringify(d)),d);
 for(const links of [null,{},[{id:'copy',source:null,target:{}}],[{...d.links[0],target:{slot:'b',interface:1,port:'value'}}]])assert.throws(()=>parseDefinition(JSON.stringify({...d,links})),/Invalid/);
});

test('installed comparison normalizes empty optional links and JSON key ordering',()=>{
 const d={schemaVersion:1,id:'demo.empty-links',version:'1.0.0',name:'Example',description:'',aspects:[]};
 assert.equal(definitionStamp(d),definitionStamp({...d,links:[]}));
 assert.equal(definitionStamp(d),definitionStamp({aspects:[],description:'',name:d.name,version:d.version,id:d.id,schemaVersion:1}));
 assert.notEqual(definitionStamp(d),definitionStamp({...d,name:'Changed'}));
});
