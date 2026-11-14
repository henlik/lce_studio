import test from 'node:test';
import assert from 'node:assert/strict';
import {blueprint, capabilities, capabilityLocation, workspaceLocation, canOpenCapability, milestones} from '../src/studioBlueprint.ts';

test('all existing editors retain one canonical destination under the intended parent', () => {
  const groups = {components:['components','data','state','messages','workflows'],design:['plan','system','design','deployment','results','inspection']};
  for (const [group, views] of Object.entries(groups)) for (const view of views) {
    const matches = capabilities.filter(c => c.view === view);
    assert.equal(matches.length, 1, view);
    assert.equal(capabilityLocation(matches[0]!.id)?.group.id, group);
    assert.equal(capabilityLocation(matches[0]!.id)?.plane.id, 'engineering');
    assert.ok(canOpenCapability(matches[0]!));
  }
  assert.equal(workspaceLocation('blueprint'), undefined);
});

test('planned runtime capabilities never expose an executable destination', () => {
  const runtime = blueprint.find(p => p.id === 'runtime')!;
  for (const c of runtime.groups.flatMap(g => g.capabilities)) {
    assert.equal(c.status, 'planned');
    assert.equal(canOpenCapability(c), false);
    assert.equal(c.view, undefined);
    assert.equal(c.action, undefined);
  }
  for (const c of capabilities.filter(c=>c.status==='planned')) assert.equal(canOpenCapability(c), false);
});

test('the blueprint has unique identities and resolvable acyclic dependencies and milestone gates', () => {
  assert.equal(new Set(capabilities.map(c=>c.id)).size, capabilities.length);
  function visit(id:string, ancestors:string[]) {
    const c = capabilityLocation(id)?.capability;
    assert.ok(c, `Unknown capability ${id}`);
    assert.ok(!ancestors.includes(id), `Dependency cycle: ${[...ancestors,id].join(' → ')}`);
    for (const dependency of c.dependsOn) visit(dependency,[...ancestors,id]);
  }
  for (const c of capabilities) {
    visit(c.id,[]);
    for (const field of ['purpose','today','next','acceptance'] as const) assert.ok(c[field].trim(),`${c.id}.${field}`);
  }
  for (const milestone of milestones) {
    assert.ok(milestone.gate.trim());
    for (const id of milestone.features) assert.ok(capabilityLocation(id));
  }
});

test('integration foundations stay distinct from existing artifact editors', () => {
  assert.equal(capabilityLocation('component-context')!.capability.status,'planned');
  assert.equal(capabilityLocation('system-model')!.capability.status,'partial');
  assert.equal(capabilityLocation('conformance')!.capability.status,'partial');
  assert.equal(canOpenCapability(capabilityLocation('conformance')!.capability),true);
  assert.equal(capabilityLocation('local-catalog')!.capability.action,'catalog');
});
