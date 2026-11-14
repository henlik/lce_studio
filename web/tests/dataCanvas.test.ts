import { test } from 'node:test';
import assert from 'node:assert/strict';
import { dataEditorPlacement } from '../src/dataCanvas.ts';

test('context editor prefers beside its element and falls back to the left', () => {
 const anchor={left:100,top:40,right:300,bottom:160};
 assert.equal(dataEditorPlacement(anchor,1000,700).left,312);
 assert.equal(dataEditorPlacement({...anchor,left:600,right:800},1000,700).left,198);
});
test('mobile and offscreen selections keep editor inside the canvas', () => {
 for(const width of [320,390,768,1200]) {
  for(const anchor of [
   {left:-500,right:-200,top:-900,bottom:-600},
   {left:1800,right:2000,top:1000,bottom:1100},
  ]) {
   const panel=dataEditorPlacement(anchor,width,660);
   assert.ok(panel.left>=8);
   assert.ok(panel.left+panel.width<=width-8);
   assert.ok(panel.top>=8);
   assert.ok(panel.top+panel.maxHeight<=652);
   assert.ok(panel.maxHeight>=292);
  }
 }
});
