import test from 'node:test';
import assert from 'node:assert/strict';
import {sourceCanApply,locateSource,sourceOffset} from '../src/sourceAuthoring.ts';
test('source application requires successful checks on unchanged source and canvas',()=>{
 assert.equal(sourceCanApply('a','a','d','d',true),true);
 for(const args of [['a','b','d','d',true],['a','a','d','e',true],['a','a','d','d',false]] as const)assert.equal(sourceCanApply(...args),false);
});
test('model diagnostics choose the closest declaration rather than a similar instance prefix',()=>{
 const locations={'':{line:1,column:1},'components[1]':{line:3,column:2},'components[10]':{line:7,column:2},'components[1].bindings[0]':{line:4,column:4}};
 assert.deepEqual(locateSource(locations,'components[1].bindings[0].resourceId'),{line:4,column:4});
 assert.deepEqual(locateSource(locations,'components[10].definition'),{line:7,column:2});
 assert.deepEqual(locateSource(locations,'unknown'),{line:1,column:1});
});
test('source selections account for Unicode code points and clamp missing lines',()=>{
 assert.equal(sourceOffset('a\n😀x',{line:2,column:2}),4);
 assert.equal(sourceOffset('a\nbc',{line:99,column:99}),4);
 assert.equal(sourceOffset('a',{line:0,column:0}),0);
});
