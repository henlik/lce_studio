#!/usr/bin/env node
// Independent state protocol and numerical references. Use a disposable database:
// catalog entries and complete experiment evidence are intentionally immutable.
import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';

const base = (process.env.STUDIO_URL || 'http://127.0.0.1:8080').replace(/\/$/, '');
const suffix = randomUUID();
const prefix = `acceptance.${suffix}`;
const clone = value => structuredClone(value);
let checks = 0;
const report = message => { checks++; console.log(`PASS ${message}`); };
async function request(path, { method = 'GET', body, raw, expected = 200 } = {}) {
  const response = await fetch(`${base}${path}`, {
    method, headers: body === undefined && raw === undefined ? {} : { 'Content-Type': 'application/json' },
    body: raw ?? (body === undefined ? undefined : JSON.stringify(body)), signal: AbortSignal.timeout(30000),
  });
  const text = await response.text();
  assert.ok((Array.isArray(expected) ? expected : [expected]).includes(response.status), `${method} ${path}: ${response.status}: ${text.slice(0, 1400)}`);
  assert.match(response.headers.get('content-type') || '', /application\/json/);
  return { status: response.status, data: JSON.parse(text) };
}
const get = async (path, expected = 200) => (await request(path, { expected })).data;
const post = async (path, body, expected = 200) => (await request(path, { method: 'POST', body, expected })).data;
const prepare = machine => post('/api/state/prepare', machine);
const check = (machine, scenario, expected = 200) => post('/api/state/check', { machine, scenario }, expected);
const save = (machine, scenario, expected = 201) => post('/api/state/runs', { machine, scenario }, expected);
const prepareComponent = (pkg, dependencies = []) => post('/api/components/prepare', { package: pkg, dependencies });
const int = integer => ({ type: 'integer', integer });
const bool = boolean => ({ type: 'boolean', boolean });
const str = string => ({ type: 'string', string });
const input = name => ({ input: name });
const literal = value => ({ literal: value });
const source = (node, port = 'result') => ({ node, port });
const port = (name, type, recordRef) => ({ name, type, ...(recordRef ? { recordRef: clone(recordRef) } : {}) });
const primitive = (id, op, inputs, metadata = {}) => ({ id, op, inputs, position: { x: 0, y: 0 }, ...metadata });
const call = (id, component, inputs = {}) => ({ id, component: clone(component), inputs, position: { x: 0, y: 0 } });
const dataRef = (lock, record) => ({ contractId: lock.contract.id, version: lock.contract.version, digest: lock.digest, record });
const record = (recordRef, values) => ({ type: 'record', recordRef: clone(recordRef), record: values });
const collection = (recordRef, values) => ({ type: 'collection', recordRef: clone(recordRef), collection: values });
const map = (values, transform = value => value) => values === null ? null : Object.fromEntries(Object.keys(values).sort().map(key => [key, transform(values[key])]));
const typedJSON = value => JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, char => `\\u${char.charCodeAt(0).toString(16).padStart(4, '0')}`);
const sha = value => createHash('sha256').update(typedJSON(value)).digest('hex');
const orderedRef = ref => ({ packageId: ref.packageId, version: ref.version, digest: ref.digest });
const orderedDataRef = ref => ({ contractId: ref.contractId, version: ref.version, digest: ref.digest, record: ref.record });
const orderedPort = p => ({ name: p.name, type: p.type, ...(p.recordRef ? { recordRef: orderedDataRef(p.recordRef) } : {}) });
function orderedValue(v) {
  return { type: v.type,
    ...Object.fromEntries(['boolean', 'integer', 'string'].filter(k => v[k] !== undefined && v[k] !== null).map(k => [k, v[k]])),
    ...(v.recordRef ? { recordRef: orderedDataRef(v.recordRef) } : {}),
    ...(v.record && Object.keys(v.record).length ? { record: map(v.record, orderedValue) } : {}),
    ...(v.collection !== undefined && v.collection !== null ? { collection: v.collection.map(orderedValue) } : {}),
  };
}
const orderedData = c => ({ schemaVersion: c.schemaVersion, id: c.id, version: c.version, name: c.name, description: c.description, author: c.author, license: c.license,
  records: c.records.map(r => ({ name: r.name, fields: r.fields.map(f => ({ name: f.name, type: f.type })) })),
  entities: c.entities.map(e => ({ name: e.name, record: e.record, primaryKey: e.primaryKey, uniqueKeys: e.uniqueKeys, position: { x: e.position.x, y: e.position.y } })),
  relationships: c.relationships.map(r => ({ name: r.name, fromEntity: r.fromEntity, fromFields: r.fromFields, toEntity: r.toEntity, toFields: r.toFields, minSources: r.minSources, maxSources: r.maxSources })),
});
const orderedDataLock = l => ({ contract: orderedData(l.contract), digest: l.digest });
const orderedBinding = v => ({ ...(v.input ? { input: v.input } : {}), ...(v.node ? { node: v.node } : {}), ...(v.port ? { port: v.port } : {}), ...(v.literal ? { literal: orderedValue(v.literal) } : {}) });
function orderedPackage(p) {
  const parameter = v => ({ default: v.default, min: v.min, max: v.max });
  return { schemaVersion: p.schemaVersion, id: p.id, version: p.version, name: p.name, description: p.description, author: p.author, license: p.license,
    kernel: p.kernel, capabilities: p.capabilities,
    config: { processingMs: parameter(p.config.processingMs), capacity: parameter(p.config.capacity), memoryMb: parameter(p.config.memoryMb), deduplicate: p.config.deduplicate },
    behavior: { inputs: p.behavior.inputs.map(orderedPort), outputs: p.behavior.outputs.map(orderedPort),
      nodes: p.behavior.nodes.map(n => ({ id: n.id, ...(n.op ? { op: n.op } : {}), ...(n.component ? { component: orderedRef(n.component) } : {}), inputs: map(n.inputs, orderedBinding), position: { x: n.position.x, y: n.position.y }, ...(n.recordRef ? { recordRef: orderedDataRef(n.recordRef) } : {}), ...(n.field ? { field: n.field } : {}) })),
      bindings: map(p.behavior.bindings, orderedBinding) },
    ...(p.tests?.length ? { tests: p.tests.map(t => ({ name: t.name, inputs: map(t.inputs, orderedValue), expected: map(t.expected, orderedValue) })) } : {}),
    ...(p.dataContracts?.length ? { dataContracts: p.dataContracts.map(orderedDataLock) } : {}),
  };
}
const orderedBundle = b => ({ schemaVersion: b.schemaVersion, root: orderedRef(b.root), packages: b.packages.map(l => ({ package: orderedPackage(l.package), digest: l.digest })) });
const orderedStateBinding = b => ({ ...(b.state ? { state: b.state } : {}), ...(b.input ? { input: b.input } : {}), ...(b.literal ? { literal: orderedValue(b.literal) } : {}) });
function orderedMachine(m) {
  return { schemaVersion: m.schemaVersion, id: m.id, version: m.version, name: m.name, description: m.description, author: m.author, license: m.license,
    dataContracts: m.dataContracts.map(orderedDataLock),
    state: m.state.map(s => ({ ...orderedPort(s), durability: s.durability, initial: orderedValue(s.initial) })),
    effects: m.effects.map(e => ({ name: e.name, payload: e.payload.map(orderedPort) })),
    commands: m.commands.map(c => ({ name: c.name, inputs: c.inputs.map(orderedPort), outputs: c.outputs.map(orderedPort), decision: orderedBundle(c.decision), bindings: map(c.bindings, orderedStateBinding), acceptedOutput: c.acceptedOutput, writes: map(c.writes), results: map(c.results), effects: c.effects.map(e => ({ effect: e.effect, ...(e.whenOutput ? { whenOutput: e.whenOutput } : {}), payload: map(e.payload) })) })),
  };
}
function orderedScenario(s) {
  return { schemaVersion: s.schemaVersion, name: s.name,
    instances: s.instances.map(i => ({ id: i.id, name: i.name, initial: map(i.initial, orderedValue) })),
    steps: s.steps.map(t => ({ kind: t.kind, instanceId: t.instanceId, ...(t.commandId ? { commandId: t.commandId } : {}), ...(t.command ? { command: t.command } : {}), ...(t.expectedRevision !== undefined && t.expectedRevision !== null ? { expectedRevision: t.expectedRevision } : {}), inputs: t.inputs === undefined ? null : map(t.inputs, orderedValue), ...(t.fault ? { fault: t.fault } : {}) })),
    expected: s.expected.map(e => ({ instanceId: e.instanceId, state: map(e.state, orderedValue), revision: e.revision, receipts: e.receipts, effects: e.effects })),
  };
}
const requestHash = s => sha({ command: s.command, expectedRevision: s.expectedRevision, inputs: map(s.inputs, orderedValue) });
const effectID = (instance, command, effect) => sha([instance, command, effect]);
function packageDefinition(name, behavior, dataContracts = [], tests = []) {
  const zero = () => ({ default: 0, min: 0, max: 0 });
  return { schemaVersion: dataContracts.length ? 3 : 2, id: `${prefix}.${name}`, version: '1.0.0', name, description: 'Independent state acceptance decision.', author: 'Acceptance suite', license: 'MIT', kernel: 'composite', capabilities: ['design', 'evaluate'], config: { processingMs: zero(), capacity: zero(), memoryMb: zero(), deduplicate: false }, behavior, ...(tests.length ? { tests } : {}), ...(dataContracts.length ? { dataContracts } : {}) };
}
function bundleOf(p, dependencies = []) {
  const lock = { package: clone(p), digest: sha(orderedPackage(p)) };
  return { schemaVersion: 1, root: { packageId: p.id, version: p.version, digest: lock.digest }, packages: [lock, ...clone(dependencies)] };
}
const machineDefinition = (name, state, commands, effects = [], dataContracts = []) => ({ schemaVersion: 1, id: `${prefix}.${name}`, version: '1.0.0', name, description: 'Independent owned state and deterministic transaction reference.', author: 'Acceptance suite', license: 'MIT', dataContracts, state, effects, commands });
const slot = (name, type, durability, initial, ref) => ({ ...port(name, type, ref), durability, initial });
const instance = (id, initial = {}) => ({ id, name: `Instance ${id}`, initial });
const commandStep = (instanceId, commandId, expectedRevision, inputs, fault, command = 'change') => ({ kind: 'command', instanceId, commandId, command, expectedRevision, inputs, ...(fault ? { fault } : {}) });
const restart = instanceId => ({ kind: 'restart', instanceId, inputs: null });
const scenario = (name, instances, steps, expected = []) => ({ schemaVersion: 1, name, instances, steps, expected });
const expectation = (instanceId, state, revision, receipts, effects) => ({ instanceId, state, revision, receipts, effects });
const stateOf = (total, ticks) => ({ total: int(total), ticks: int(ticks), note: str(''), enabled: bool(false) });
const argumentsOf = (amount, allow = true, notify = false) => ({ amount: int(amount), allow: bool(allow), notify: bool(notify) });
function expectRejected(result) { assert.equal(result.valid, false); assert.ok(result.issues.length > 0); }

assert.equal((await get('/api/health')).version, '0.6.0');
assert.equal((await get('/api/components/primitives')).length, 11);
assert.equal((await get('/api/components/structured-primitives')).length, 16);
const reducerPackage = packageDefinition('increment', {
  inputs: [port('current', 'integer'), port('session', 'integer'), port('amount', 'integer'), port('allow', 'boolean'), port('notify', 'boolean')],
  outputs: [port('accepted', 'boolean'), port('nextTotal', 'integer'), port('nextTicks', 'integer'), port('emit', 'boolean')],
  nodes: [primitive('total', 'add', { left: input('current'), right: input('amount') }), primitive('ticks', 'add', { left: input('session'), right: literal(int(1)) })],
  bindings: { accepted: input('allow'), nextTotal: source('total'), nextTicks: source('ticks'), emit: input('notify') },
});
const reducer = await prepareComponent(reducerPackage);
assert.equal(reducer.root.digest, sha(orderedPackage(reducerPackage)));
const definition = machineDefinition('owned-counter', [slot('total', 'integer', 'durable', int(10)), slot('ticks', 'integer', 'volatile', int(100)), slot('note', 'string', 'durable', str('')), slot('enabled', 'boolean', 'durable', bool(false))], [{
  name: 'change', inputs: [port('amount', 'integer'), port('allow', 'boolean'), port('notify', 'boolean')], outputs: [port('total', 'integer'), port('ticks', 'integer')],
  decision: reducer, bindings: { current: { state: 'total' }, session: { state: 'ticks' }, amount: { input: 'amount' }, allow: { input: 'allow' }, notify: { input: 'notify' } },
  acceptedOutput: 'accepted', writes: { total: 'nextTotal', ticks: 'nextTicks' }, results: { total: 'nextTotal', ticks: 'nextTicks' },
  effects: [{ effect: 'Notice', whenOutput: 'emit', payload: { total: 'nextTotal', ticks: 'nextTicks' } }, { effect: 'Audit', payload: { total: 'nextTotal', ticks: 'nextTicks' } }],
}], [{ name: 'Audit', payload: [port('total', 'integer'), port('ticks', 'integer')] }, { name: 'Notice', payload: [port('total', 'integer'), port('ticks', 'integer')] }]);
const machine = await prepare(definition);
assert.deepEqual(machine.machine, definition);
assert.equal(machine.digest, sha(orderedMachine(definition)));
const pure = await post('/api/components/evaluate', { bundle: reducer, inputs: { current: int(10), session: int(100), ...argumentsOf(2) } }, 201);
assert.equal(pure.result.engineVersion, '0.5.0');
assert.deepEqual(pure.result.outputs, { accepted: bool(true), nextTotal: int(12), nextTicks: int(101), emit: bool(false) });
report('separate state artifacts preserve existing pure digests, primitive catalogs and engine versions');

const steps = [
  commandStep('A', 'c1', 0, argumentsOf(2)),
  commandStep('A', 'c1', 0, argumentsOf(2), 'before_commit'),
  commandStep('A', 'c2', 1, argumentsOf(3)),
  commandStep('A', 'c1', 0, argumentsOf(2), 'after_commit_before_reply'),
  commandStep('A', 'c3', 2, argumentsOf(4), 'before_commit'),
  commandStep('A', 'c3', 2, argumentsOf(4)),
  commandStep('A', 'c4', 3, argumentsOf(5), 'after_commit_before_reply'),
  restart('A'),
  commandStep('A', 'c4', 3, argumentsOf(5), 'before_commit'),
  commandStep('A', 'c5', 4, argumentsOf(1)),
  commandStep('A', 'c1', 0, argumentsOf(9)),
  commandStep('A', 'stale', 4, argumentsOf(7), 'after_commit_before_reply'),
  commandStep('B', 'c1', 0, argumentsOf(2)),
  restart('A'),
  commandStep('A', 'c6', 5, argumentsOf(0, true, true)),
];
const protocol = scenario('Commit, rollback, receipt replay and independent restart', [instance('A'), instance('B', { total: int(20), ticks: int(200) })], steps, [expectation('A', stateOf(25, 101), 6, 6, 7), expectation('B', stateOf(22, 201), 1, 1, 1)]);
const checked = await check(machine, protocol), result = checked.result;
assert.deepEqual(checked.scenario, protocol);
assert.equal(result.engineVersion, '0.6.0'); assert.equal(result.machineHash, machine.digest); assert.equal(result.scenarioHash, sha(orderedScenario(protocol)));
assert.equal(result.verified, true); assert.equal(result.passed, true);
assert.equal(result.operations, 16); assert.equal(result.invocations, 8);
assert.deepEqual(result.steps.map(s => s.status), ['committed', 'replayed', 'committed', 'replayed', 'rolled_back', 'committed', 'committed', 'restarted', 'replayed', 'committed', 'id_conflict', 'revision_conflict', 'committed', 'restarted', 'committed']);
const states = [[12, 101], [12, 101], [15, 102], [15, 102], [15, 102], [19, 103], [24, 104], [24, 100], [24, 100], [25, 101], [25, 101], [25, 101], [22, 201], [25, 100], [25, 101]];
const revisions = [1, 1, 2, 2, 2, 3, 4, 4, 4, 5, 5, 5, 1, 5, 6];
result.steps.forEach((s, i) => {
  assert.equal(s.index, i); assert.deepEqual(s.after, stateOf(...states[i])); assert.equal(s.afterRevision, revisions[i]);
  assert.equal(s.replyLost, i === 6); assert.equal(s.faultApplied, i === 4 || i === 6);
  if (['replayed', 'id_conflict', 'revision_conflict', 'restarted'].includes(s.status)) assert.equal(s.decision, undefined);
  if (['id_conflict', 'revision_conflict', 'rolled_back', 'restarted'].includes(s.status)) assert.equal(s.receipt, undefined);
  if (s.decision) assert.equal(s.decision.engineVersion, '0.5.0');
});
assert.deepEqual(result.instances.map(i => i.id), ['A', 'B']);
assert.deepEqual(result.instances[0].state, stateOf(25, 101)); assert.deepEqual(result.instances[1].state, stateOf(22, 201));
assert.deepEqual(result.steps[3].receipt.outputs, { total: int(12), ticks: int(101) });
assert.deepEqual(result.steps[8].receipt.outputs, { total: int(24), ticks: int(104) });
assert.deepEqual(result.steps[8].after, stateOf(24, 100));
assert.deepEqual((await check(machine, protocol)).result, result);
report('independent numerical transitions distinguish rollback, committed reply loss, historical replay and per-instance restart');

const committed = [[0, 'c1', 1, 12, 101], [2, 'c2', 2, 15, 102], [5, 'c3', 3, 19, 103], [6, 'c4', 4, 24, 104], [9, 'c5', 5, 25, 101], [14, 'c6', 6, 25, 101]];
const expectedEffects = [];
for (const [index, commandId, revision, total, ticks] of committed) {
  const names = commandId === 'c6' ? ['Notice', 'Audit'] : ['Audit'];
  const outputs = { total: int(total), ticks: int(ticks) };
  const receipt = { commandId, requestHash: requestHash(steps[index]), command: 'change', accepted: true, revision, outputs, effectIds: names.map(n => effectID('A', commandId, n)) };
  assert.deepEqual(result.steps[index].receipt, receipt);
  assert.deepEqual(result.instances[0].receipts[expectedEffects.length === 0 ? 0 : committed.findIndex(c => c[1] === commandId)], receipt);
  for (const effect of names) expectedEffects.push({ id: effectID('A', commandId, effect), instanceId: 'A', commandId, effect, revision, payload: outputs });
}
assert.deepEqual(result.instances[0].effects, expectedEffects);
assert.notEqual(result.instances[1].effects[0].id, result.instances[0].effects[0].id);
assert.equal(result.instances[1].receipts[0].requestHash, result.instances[0].receipts[0].requestHash);
assert.deepEqual(result.steps[0].after, stateOf(12, 101));
assert.deepEqual(result.steps[4].before, result.steps[4].after);
assert.deepEqual(result.steps[1].receipt, result.steps[0].receipt);
assert.deepEqual(result.steps[6].receipt, result.steps[8].receipt);
report('receipt fingerprints and ordered effect identities match independent SHA-256 references and immutable historical payloads');

const denialSteps = [commandStep('A', 'denied', 0, argumentsOf(5, false, true), 'after_commit_before_reply'), restart('A'), commandStep('A', 'denied', 0, argumentsOf(5, false, true), 'before_commit'), commandStep('A', 'denied', 0, argumentsOf(6, false, true)), commandStep('A', 'accepted', 0, argumentsOf(2)), commandStep('A', 'denied', 0, argumentsOf(5, false, true)), commandStep('A', 'rolled', 1, argumentsOf(7, false, true), 'before_commit'), commandStep('A', 'rolled', 1, argumentsOf(1))];
const denial = await check(machine, scenario('Denied receipts and unreserved rollback IDs', [instance('A')], denialSteps, [expectation('A', stateOf(13, 102), 2, 3, 2)]));
assert.equal(denial.result.passed, true); assert.equal(denial.result.operations, 8);
assert.deepEqual(denial.result.steps.map(s => s.status), ['denied', 'restarted', 'replayed', 'id_conflict', 'committed', 'replayed', 'rolled_back', 'committed']);
assert.equal(denial.result.steps[0].replyLost, true); assert.equal(denial.result.steps[0].receipt.accepted, false); assert.equal(denial.result.steps[0].receipt.revision, 0);
assert.deepEqual(denial.result.steps[0].receipt.effectIds, []); assert.deepEqual(denial.result.steps[0].after, stateOf(10, 100));
assert.deepEqual(denial.result.steps[5].receipt.outputs, { total: int(15), ticks: int(101) }); assert.deepEqual(denial.result.steps[5].after, stateOf(12, 101));
report('denials retain replayable receipts without proposed writes/effects and rolled-back IDs remain available');

const max = 9007199254740991;
const overflowSteps = [commandStep('A', 'retry', 0, argumentsOf(1), 'before_commit'), commandStep('A', 'retry', 0, argumentsOf(-1)), commandStep('A', 'retry', 0, argumentsOf(1))];
const overflow = await check(machine, scenario('Ordinary pure errors remain noncommitting outcomes', [instance('A', { total: int(max) })], overflowSteps, [expectation('A', stateOf(max - 1, 101), 1, 1, 1)]));
assert.equal(overflow.result.passed, true); assert.deepEqual(overflow.result.steps.map(s => s.status), ['evaluation_error', 'committed', 'id_conflict']);
assert.ok(overflow.result.steps[0].message.length > 0); assert.equal(overflow.result.steps[0].decision, undefined); assert.equal(overflow.result.steps[0].receipt, undefined); assert.equal(overflow.result.steps[0].faultApplied, false);
assert.deepEqual(overflow.result.steps[0].before, overflow.result.steps[0].after);
const badRevision = clone(protocol); badRevision.steps = [commandStep('A', 'fresh', 99, argumentsOf(1), 'before_commit')]; badRevision.expected = [];
const conflict = (await check(machine, badRevision)).result;
assert.equal(conflict.steps[0].status, 'revision_conflict'); assert.equal(conflict.steps[0].faultApplied, false); assert.equal(conflict.operations, 0);
const changedRevision = clone(protocol); changedRevision.steps = [steps[0], commandStep('A', 'c1', 1, argumentsOf(2))]; changedRevision.expected = [];
assert.equal((await check(machine, changedRevision)).result.steps[1].status, 'id_conflict');
report('receipt identity precedes optimistic revisions and faults while arithmetic errors neither commit nor reserve command IDs');

const unverified = clone(protocol); unverified.expected = [];
const noExpectations = (await check(machine, unverified)).result;
assert.equal(noExpectations.verified, false); assert.equal(noExpectations.passed, false); assert.deepEqual(noExpectations.expectations, []);
const wrongExpected = clone(protocol); wrongExpected.expected[0].state.total = int(999); wrongExpected.expected[0].effects = 0;
const failedExpectations = await save(machine, wrongExpected);
assert.equal(failedExpectations.result.verified, true); assert.equal(failedExpectations.result.passed, false);
assert.equal(failedExpectations.result.expectations[0].passed, false); assert.ok(failedExpectations.result.expectations[0].issues.length >= 1);
assert.equal(failedExpectations.result.expectations[1].passed, true);
assert.deepEqual(failedExpectations.result.instances, result.instances);
const changedLabels = clone(protocol); changedLabels.name += ' alternate'; changedLabels.instances[0].name = 'Another label';
const relabeled = (await check(machine, changedLabels)).result;
assert.notEqual(relabeled.scenarioHash, result.scenarioHash); assert.deepEqual(relabeled.instances, result.instances); assert.deepEqual(relabeled.steps, result.steps);
report('handwritten expectations distinguish verified failures from unverified runs and labels cannot alter command semantics');

const dataDefinition = { schemaVersion: 1, id: `${prefix}.state-data`, version: '1.0.0', name: 'State values', description: 'Exact record values for independent isolation checks.', author: 'Acceptance suite', license: 'MIT', records: [{ name: 'Item', fields: [port('id', 'string'), port('n', 'integer'), port('active', 'boolean')] }], entities: [], relationships: [] };
const data = await post('/api/data/prepare', dataDefinition), itemRef = dataRef(data, 'Item');
const item = (id, n, active = false) => record(itemRef, { id: str(id), n: int(n), active: bool(active) });
const starter = item('start', 0), first = item('one', 1), second = item('two', 2), third = item('three', 3);
const listDecisionPackage = packageDefinition('append-owned', { inputs: [port('items', 'collection', itemRef), port('value', 'record', itemRef)], outputs: [port('accepted', 'boolean'), port('next', 'collection', itemRef), port('value', 'record', itemRef)], nodes: [primitive('append', 'collection_append', { values: input('items'), value: input('value') })], bindings: { accepted: literal(bool(true)), next: source('append'), value: input('value') } }, [data]);
const listDecision = await prepareComponent(listDecisionPackage);
const structuredDefinition = machineDefinition('structured-owned', [slot('latest', 'record', 'durable', starter, itemRef), slot('items', 'collection', 'volatile', collection(itemRef, []), itemRef)], [{ name: 'append', inputs: [port('value', 'record', itemRef)], outputs: [port('items', 'collection', itemRef)], decision: listDecision, bindings: { items: { state: 'items' }, value: { input: 'value' } }, acceptedOutput: 'accepted', writes: { items: 'next', latest: 'value' }, results: { items: 'next' }, effects: [{ effect: 'Appended', payload: { value: 'value', items: 'next' } }] }], [{ name: 'Appended', payload: [port('value', 'record', itemRef), port('items', 'collection', itemRef)] }]);
const structuredMachine = await prepare(structuredDefinition);
const appendStep = (id, commandId, revision, value) => commandStep(id, commandId, revision, { value }, undefined, 'append');
const reorderedFirst = record(itemRef, { active: bool(false), n: int(1), id: str('one') });
const structuredScenario = scenario('Record copies and volatile collection reset', [instance('A', { items: collection(itemRef, [starter]) }), instance('B', { items: collection(itemRef, [starter]) })], [appendStep('A', 'x', 0, first), appendStep('B', 'x', 0, second), appendStep('A', 'y', 1, third), restart('A'), appendStep('A', 'x', 0, reorderedFirst)], [expectation('A', { latest: third, items: collection(itemRef, [starter]) }, 2, 2, 2), expectation('B', { latest: second, items: collection(itemRef, [starter, second]) }, 1, 1, 1)]);
const structure = (await check(structuredMachine, structuredScenario)).result;
assert.equal(structure.passed, true); assert.equal(structure.operations, 3); assert.equal(structure.invocations, 3);
assert.equal(structure.steps[4].status, 'replayed'); assert.deepEqual(structure.steps[0].after.items, collection(itemRef, [starter, first]));
assert.deepEqual(structure.steps[4].receipt.outputs, { items: collection(itemRef, [starter, first]) });
assert.deepEqual(structure.instances[0].effects[0].payload, { value: first, items: collection(itemRef, [starter, first]) });
assert.equal(structure.instances[0].receipts[0].requestHash, requestHash(structuredScenario.steps[4]));
assert.deepEqual((await check(structuredMachine, structuredScenario)).result, structure);
report('structured snapshots and receipts remain isolated and restart restores each instance override rather than shared defaults');

const malformedMachines = [];
const mutateMachine = fn => { const m = clone(definition); fn(m); malformedMachines.push(m); };
mutateMachine(m => { m.state[0].durability = 'persistent'; });
mutateMachine(m => { m.state[0].initial = str('10'); });
mutateMachine(m => { m.state[0].initial = null; });
mutateMachine(m => { m.commands[0].bindings.current = { state: 'total', input: 'amount' }; });
mutateMachine(m => { delete m.commands[0].bindings.current; });
mutateMachine(m => { m.commands[0].bindings.unknown = { state: 'total' }; });
mutateMachine(m => { m.commands[0].bindings.current = { state: 'other' }; });
mutateMachine(m => { m.commands[0].acceptedOutput = 'nextTotal'; });
mutateMachine(m => { m.commands[0].writes.total = 'accepted'; });
mutateMachine(m => { m.commands[0].writes.outside = 'nextTotal'; });
mutateMachine(m => { delete m.commands[0].results.total; });
mutateMachine(m => { m.commands[0].effects[0].whenOutput = 'nextTotal'; });
mutateMachine(m => { m.commands[0].effects[0].payload.total = 'accepted'; });
mutateMachine(m => { m.commands[0].effects.push(clone(m.commands[0].effects[0])); });
mutateMachine(m => { m.commands[0].effects[0].effect = 'Unknown'; });
mutateMachine(m => { m.dataContracts = null; });
mutateMachine(m => { m.effects = null; });
mutateMachine(m => { m.commands[0].writes = null; });
mutateMachine(m => { m.commands[0].inputs = null; });
mutateMachine(m => { m.commands[0].decision.packages[0].package.name += ' tampered'; });
for (const invalid of malformedMachines) await prepareInvalid(invalid);
async function prepareInvalid(invalid) { return post('/api/state/prepare', invalid, 422); }
const otherDataDef = clone(dataDefinition); otherDataDef.id += '-other';
const otherData = await post('/api/data/prepare', otherDataDef), otherItemRef = dataRef(otherData, 'Item');
const wrongNominal = clone(structuredDefinition); wrongNominal.dataContracts.push(otherData); wrongNominal.state[0].recordRef = otherItemRef; wrongNominal.state[0].initial = record(otherItemRef, starter.record);
await prepareInvalid(wrongNominal);
const invalidUnused = clone(definition); invalidUnused.dataContracts.push({ ...data, digest: '0'.repeat(64) }); await prepareInvalid(invalidUnused);
report('machine mappings reject unsupported ownership, ambiguous bindings, invalid effect signatures and nominally incompatible values');

const malformedScenarios = [];
const mutateScenario = fn => { const s = scenario('Invalid input reference', [instance('A')], [clone(steps[0])]); fn(s); malformedScenarios.push(s); };
mutateScenario(s => { s.instances[0].initial.unknown = int(1); });
mutateScenario(s => { s.instances[0].initial.total = str('10'); });
mutateScenario(s => { s.instances[0].initial = null; });
mutateScenario(s => { s.steps[0].instanceId = 'Unknown'; });
mutateScenario(s => { s.steps[0].kind = 'send'; });
mutateScenario(s => { s.steps[0].command = 'unknown'; });
mutateScenario(s => { s.steps[0].commandId = ''; });
mutateScenario(s => { delete s.steps[0].expectedRevision; });
mutateScenario(s => { s.steps[0].expectedRevision = null; });
mutateScenario(s => { s.steps[0].expectedRevision = -1; });
mutateScenario(s => { s.steps[0].expectedRevision = 9007199254740992; });
mutateScenario(s => { s.steps[0].inputs = null; });
mutateScenario(s => { delete s.steps[0].inputs.amount; });
mutateScenario(s => { s.steps[0].inputs.extra = int(0); });
mutateScenario(s => { s.steps[0].inputs.amount = bool(false); });
mutateScenario(s => { s.steps[0].fault = 'drop_network'; });
mutateScenario(s => { s.steps = [{ ...restart('A'), expectedRevision: 0 }]; });
mutateScenario(s => { s.steps = [{ ...restart('A'), inputs: {} }]; });
mutateScenario(s => { s.steps = [{ ...restart('A'), commandId: 'x' }]; });
mutateScenario(s => { s.steps = [{ ...restart('A'), fault: 'before_commit' }]; });
mutateScenario(s => { s.instances.push(clone(s.instances[0])); });
mutateScenario(s => { s.expected = null; });
mutateScenario(s => { s.expected = [expectation('A', { total: int(12) }, 1, 1, 1)]; });
mutateScenario(s => { s.expected = [expectation('A', stateOf(12, 101), 1, -1, 1)]; });
mutateScenario(s => { s.expected = [expectation('A', stateOf(12, 101), 1, 1, 1), expectation('A', stateOf(12, 101), 1, 1, 1)]; });
for (const invalid of malformedScenarios) await check(machine, invalid, 422);
for (const field of ['revision', 'receipts', 'effects']) {
  for (const missing of [true, false]) {
    const invalid = clone(protocol);
    if (missing) delete invalid.expected[0][field]; else invalid.expected[0][field] = null;
    await check(machine, invalid, 400);
  }
}
const reordered = clone(protocol); reordered.steps[0].inputs = { notify: bool(false), allow: bool(true), amount: int(2) };
assert.deepEqual((await check(machine, reordered)).result, result);
const canonicalRestart = await check(machine, scenario('Canonical restart', [instance('A')], [{ kind: 'restart', instanceId: 'A' }]));
assert.deepEqual(canonicalRestart.scenario.steps, [restart('A')]);
assert.deepEqual(canonicalRestart.result.instances[0].state, stateOf(10, 100));
report('scenario validation checks actual values, complete expectations and restart-only fields before any execution');

const lockAlias = clone(machine); lockAlias.Machine = lockAlias.machine; delete lockAlias.machine; lockAlias.Machine.State = lockAlias.Machine.state; delete lockAlias.Machine.state;
const canonical = await post('/api/state/validate-lock', lockAlias);
assert.equal(canonical.valid, true); assert.deepEqual(canonical.machine, machine);
const scenarioAlias = clone(protocol); scenarioAlias.Steps = scenarioAlias.steps; delete scenarioAlias.steps;
assert.deepEqual((await check(machine, scenarioAlias)).scenario, protocol);
const tampered = clone(machine); tampered.machine.name += ' changed';
const invalidLock = await post('/api/state/validate-lock', tampered); expectRejected(invalidLock); assert.equal(invalidLock.machine, undefined);
await check(tampered, protocol, 422);
await post('/api/state/prepare', { ...definition, execute: 'shell' }, 400);
await request('/api/state/prepare', { method: 'POST', raw: `${JSON.stringify(definition)} {}`, expected: 400 });
const fraction = JSON.stringify({ machine, scenario: scenario('Raw fraction', [instance('A')], [steps[0]]) }).replace('"integer":2', '"integer":9007199254740990.5');
await request('/api/state/check', { method: 'POST', raw: fraction, expected: 400 });
const fractionalRevision = JSON.stringify({ machine, scenario: scenario('Raw revision', [instance('A')], [steps[0]]) }).replace('"expectedRevision":0', '"expectedRevision":0.5');
await request('/api/state/check', { method: 'POST', raw: fractionalRevision, expected: 400 });
const escaped = clone(definition); escaped.description = 'A < B & C > D\u2028state\u2029effects';
assert.equal((await prepare(escaped)).digest, sha(orderedMachine(escaped)));
report('raw imports retain canonical exact identities and reject unsafe numeric parsing, unknown fields and trailing JSON');

const changedReducer = clone(reducerPackage); changedReducer.name += ' conflicting content';
const conflictingDecision = await prepareComponent(changedReducer);
const conflictDefinitions = clone(definition); conflictDefinitions.commands.push({ ...clone(conflictDefinitions.commands[0]), name: 'other', decision: conflictingDecision });
await prepareInvalid(conflictDefinitions);
const repeatedDecision = clone(definition); repeatedDecision.commands.push({ ...clone(repeatedDecision.commands[0]), name: 'other' });
await prepare(repeatedDecision);
const commandConflictScenario = scenario('Command name is part of receipt identity', [instance('A')], [steps[0], commandStep('A', 'c1', 0, argumentsOf(2), undefined, 'other')]);
assert.equal((await check(await prepare(repeatedDecision), commandConflictScenario)).result.steps[1].status, 'id_conflict');
const conflictDataDef = clone(dataDefinition); conflictDataDef.description += ' changed';
const conflictData = await post('/api/data/prepare', conflictDataDef);
const conflictDataMachine = clone(structuredDefinition); conflictDataMachine.dataContracts = [conflictData]; await prepareInvalid(conflictDataMachine);
const missingDecisionData = clone(listDecisionPackage); delete missingDecisionData.dataContracts;
const machineOnlyData = clone(structuredDefinition); machineOnlyData.dataContracts = [data]; machineOnlyData.commands[0].decision = bundleOf(missingDecisionData);
await prepareInvalid(machineOnlyData);
report('pure/data definitions resolve from exact portable closures and conflicting cross-command locks cannot hide behind identical shapes');

const emptyDecision = (name, dataLocks = []) => packageDefinition(name, { inputs: [], outputs: [port('accepted', 'boolean')], nodes: [], bindings: { accepted: literal(bool(true)) } }, dataLocks);
const simpleCommand = (name, decision) => ({ name, inputs: [], outputs: [], decision, bindings: {}, acceptedOutput: 'accepted', writes: {}, results: {}, effects: [] });
const simpleMachine = (name, commands, state = [slot('value', 'integer', 'durable', int(0))], effects = []) => machineDefinition(name, state, commands, effects);
const closureCommands = [];
for (let group = 0; group < 8; group++) {
  let bundle = bundleOf(emptyDecision(`closure_${group}_0`));
  for (let depth = 1; depth < 8; depth++) {
    const p = emptyDecision(`closure_${group}_${depth}`); p.behavior.nodes = [call('child', bundle.root)]; p.behavior.bindings.accepted = source('child', 'accepted');
    bundle = bundleOf(p, bundle.packages);
  }
  closureCommands.push(simpleCommand(`handler${group}`, bundle));
}
const closureMachine = simpleMachine('sixty-four-locks', closureCommands);
await prepare(closureMachine);
const tooManyLocks = clone(closureMachine);
const extra = bundleOf(emptyDecision('closure_extra'));
const originalRoot = tooManyLocks.commands[0].decision.packages[0].package; originalRoot.behavior.nodes.push(call('extra', extra.root), primitive('both', 'and', { left: source('child', 'accepted'), right: source('extra', 'accepted') })); originalRoot.behavior.bindings.accepted = source('both');
tooManyLocks.commands[0].decision = bundleOf(originalRoot, [...tooManyLocks.commands[0].decision.packages.slice(1), ...extra.packages]);
const pureCapError = await prepareInvalid(tooManyLocks);
assert.ok(pureCapError.issues.some(i => i.path === 'commands' && /64 distinct component/.test(i.message)));
const spareData = Array.from({ length: 33 }, (_, i) => { const contract = clone(dataDefinition); contract.id = `${prefix}.spare_${i}`; return { contract, digest: sha(orderedData(contract)) }; });
const dataUnionMachine = simpleMachine('thirty-two-data-locks', Array.from({ length: 4 }, (_, i) => simpleCommand(`handler${i}`, bundleOf(emptyDecision(`union_${i}`, spareData.slice(i * 8, i * 8 + 8))))));
await prepare(dataUnionMachine);
dataUnionMachine.dataContracts.push(spareData[32]); await prepareInvalid(dataUnionMachine);
const tooManyLocal = clone(definition); tooManyLocal.dataContracts = spareData.slice(0, 9); await prepareInvalid(tooManyLocal);
report('machine-wide dependency limits count distinct exact pure and data locks across otherwise valid decision bundles');

const tooManySlots = clone(definition); tooManySlots.state = Array.from({ length: 9 }, (_, i) => slot(`slot${i}`, 'integer', 'durable', int(0))); await prepareInvalid(tooManySlots);
const tooManyCommands = clone(definition); tooManyCommands.commands = Array.from({ length: 9 }, (_, i) => ({ ...clone(definition.commands[0]), name: `command${i}` })); await prepareInvalid(tooManyCommands);
const tooManyEffects = clone(definition); tooManyEffects.effects = Array.from({ length: 9 }, (_, i) => ({ name: `Effect${i}`, payload: [] })); await prepareInvalid(tooManyEffects);
const tooManyInstances = clone(protocol); tooManyInstances.instances = Array.from({ length: 9 }, (_, i) => instance(`I${i}`)); await check(machine, tooManyInstances, 422);
const manySteps = scenario('Sixty-four immutable receipts', [instance('A')], Array.from({ length: 64 }, (_, i) => commandStep('A', `c${i}`, i, argumentsOf(0))), [expectation('A', stateOf(10, 164), 64, 64, 64)]);
const fullReceipts = (await check(machine, manySteps)).result;
assert.equal(fullReceipts.passed, true); assert.equal(fullReceipts.instances[0].receipts.length, 64);
const sixtyFive = clone(manySteps); sixtyFive.steps.push(commandStep('A', 'extra', 64, argumentsOf(0))); await check(machine, sixtyFive, 422);
const effectsDef = simpleMachine('effect-capacity', [simpleCommand('commit', bundleOf(emptyDecision('effect-accepted')))], undefined, Array.from({ length: 8 }, (_, i) => ({ name: `Intent${i}`, payload: [] })));
effectsDef.commands[0].effects = effectsDef.effects.map(e => ({ effect: e.name, payload: {} }));
const effectMachine = await prepare(effectsDef);
const effectSteps = count => scenario('Bounded effect journal', [instance('A')], Array.from({ length: count }, (_, i) => commandStep('A', `c${i}`, i, {}, undefined, 'commit')));
const fullEffects = (await check(effectMachine, effectSteps(16))).result;
assert.equal(fullEffects.instances[0].effects.length, 128); assert.equal(fullEffects.instances[0].revision, 16);
await check(effectMachine, effectSteps(17), 422);
report('bounded scenarios retain every receipt and intent at capacity and reject overflow without eviction');

const wideDef = clone(dataDefinition); wideDef.id += '-wide'; wideDef.records = [{ name: 'Wide', fields: Array.from({ length: 16 }, (_, i) => port(`f${i}`, 'string')) }];
const wide = await post('/api/data/prepare', wideDef), wideRef = dataRef(wide, 'Wide');
const wideValue = size => record(wideRef, Object.fromEntries(wideDef.records[0].fields.map(f => [f.name, str('x'.repeat(size))])));
const validWide = wideValue(1000);
const wideDecision = bundleOf(packageDefinition('wide-decision', { inputs: [port('value', 'record', wideRef)], outputs: [port('accepted', 'boolean'), port('next', 'record', wideRef)], nodes: [], bindings: { accepted: literal(bool(true)), next: input('value') } }, [wide]));
const wideCommand = { ...simpleCommand('commit', wideDecision), bindings: { value: { state: 'value' } }, writes: { value: 'next' } };
const wideMachineDefinition = simpleMachine('wide-owned', [wideCommand], [slot('value', 'record', 'durable', validWide, wideRef)]);
const wideMachine = await prepare(wideMachineDefinition);
const mapTooWide = clone(wideMachineDefinition); mapTooWide.state = Array.from({ length: 5 }, (_, i) => slot(`v${i}`, 'record', 'durable', validWide, wideRef)); mapTooWide.commands[0].bindings.value = { state: 'v0' }; mapTooWide.commands[0].writes = {};
await prepareInvalid(mapTooWide);
const inputMapDef = clone(wideMachineDefinition); inputMapDef.state = [slot('value', 'record', 'durable', starter, itemRef)]; inputMapDef.dataContracts = [data]; inputMapDef.commands[0].bindings.value = { input: 'v0' }; inputMapDef.commands[0].writes = {}; inputMapDef.commands[0].inputs = Array.from({ length: 5 }, (_, i) => port(`v${i}`, 'record', wideRef));
const inputMapMachine = await prepare(inputMapDef);
await check(inputMapMachine, scenario('Oversized exact command map', [instance('A')], [commandStep('A', 'c', 0, Object.fromEntries(Array.from({ length: 5 }, (_, i) => [`v${i}`, validWide])), undefined, 'commit')]), 422);
const payloadTooWide = clone(wideMachineDefinition); payloadTooWide.effects = [{ name: 'Large', payload: Array.from({ length: 5 }, (_, i) => port(`v${i}`, 'record', wideRef)) }]; payloadTooWide.commands[0].effects = [{ effect: 'Large', payload: Object.fromEntries(Array.from({ length: 5 }, (_, i) => [`v${i}`, 'next'])) }];
const payloadMachine = await prepare(payloadTooWide);
await check(payloadMachine, scenario('Mapped payload amplification', [instance('A')], [commandStep('A', 'c', 0, {}, undefined, 'commit')]), 422);
const resultTooWide = clone(wideMachineDefinition); resultTooWide.commands[0].outputs = Array.from({ length: 5 }, (_, i) => port(`v${i}`, 'record', wideRef)); resultTooWide.commands[0].results = Object.fromEntries(Array.from({ length: 5 }, (_, i) => [`v${i}`, 'next']));
await check(await prepare(resultTooWide), scenario('Mapped result amplification', [instance('A')], [commandStep('A', 'c', 0, {}, undefined, 'commit')]), 422);
const effectiveTooWide = clone(wideMachineDefinition); effectiveTooWide.state = ['v0', 'v1'].map(name => slot(name, 'record', 'durable', validWide, wideRef)); effectiveTooWide.commands[0].bindings.value = { state: 'v0' }; effectiveTooWide.commands[0].writes = {};
await check(await prepare(effectiveTooWide), scenario('Effective state combines defaults with overrides', [instance('A', { v0: wideValue(3900) })], [commandStep('A', 'c', 0, {}, undefined, 'commit')]), 422);
const stagedTooWide = clone(effectiveTooWide); stagedTooWide.commands[0].inputs = [port('value', 'record', wideRef)]; stagedTooWide.commands[0].bindings.value = { input: 'value' }; stagedTooWide.commands[0].writes = { v0: 'next', v1: 'next' };
await check(await prepare(stagedTooWide), scenario('Staged writes amplify one result', [instance('A')], [commandStep('A', 'c', 0, { value: wideValue(2500) }, undefined, 'commit')]), 422);
const malformedCollection = clone(structuredDefinition); malformedCollection.state[1].initial = collection(itemRef, Array.from({ length: 129 }, () => starter)); await prepareInvalid(malformedCollection);
const traceScenario = scenario('Incremental evidence limit', [instance('A')], Array.from({ length: 64 }, (_, i) => commandStep('A', `c${i}`, i, {}, undefined, 'commit')));
const historyBeforeLimits = await get('/api/state/runs');
await save(wideMachine, traceScenario, 422);
assert.deepEqual(await get('/api/state/runs'), historyBeforeLimits);
report('state/input/effect map budgets and incremental evidence bounds prevent repeated-value amplification before persisted evidence');

const leafPackage = packageDefinition('work-leaf', { inputs: [port('value', 'integer')], outputs: [port('value', 'integer')], nodes: Array.from({ length: 32 }, (_, i) => primitive(`n${i}`, 'identity', { value: i ? source(`n${i - 1}`) : input('value') })), bindings: { value: source('n31') } });
const leaf = bundleOf(leafPackage);
const workPackage = packageDefinition('work-root', { inputs: [port('value', 'integer')], outputs: [port('accepted', 'boolean'), port('value', 'integer')], nodes: Array.from({ length: 4 }, (_, i) => call(`call${i}`, leaf.root, { value: i ? source(`call${i - 1}`, 'value') : input('value') })), bindings: { accepted: literal(bool(true)), value: source('call3', 'value') } });
const workBundle = bundleOf(workPackage, leaf.packages);
const workDefinition = simpleMachine('global-operations', [{ ...simpleCommand('commit', workBundle), bindings: { value: { state: 'value' } }, writes: { value: 'value' } }]);
const workMachine = await prepare(workDefinition);
const workScenario = count => scenario('Global work accounting', [instance('A')], Array.from({ length: count }, (_, i) => commandStep('A', `c${i}`, i, {}, undefined, 'commit')));
const workBoundary = (await check(workMachine, workScenario(32))).result;
assert.equal(workBoundary.operations, 4096); assert.equal(workBoundary.invocations, 160);
await check(workMachine, workScenario(33), 422);
const shortPackage = (name, children = []) => {
  const p = emptyDecision(name); p.id = `a.${suffix.slice(0, 8)}.${name}`;
  p.behavior.nodes = children;
  p.behavior.outputs = (children.length ? children : [null]).map((_, i) => port(String.fromCharCode(97 + i), 'boolean'));
  p.behavior.bindings = Object.fromEntries(p.behavior.outputs.map((v, i) => [v.name, children.length ? source(children[i].id, 'a') : literal(bool(true))]));
  return p;
};
const invLeaf = bundleOf(shortPackage('l'));
const invMid = bundleOf(shortPackage('m', Array.from({ length: 16 }, (_, i) => call(`n${i}`, invLeaf.root))), invLeaf.packages);
const invRoot = bundleOf(shortPackage('r', Array.from({ length: 15 }, (_, i) => call(`n${i}`, invMid.root))), invMid.packages);
const invMachine = await prepare(simpleMachine('global-invocations', [{ ...simpleCommand('commit', invRoot), acceptedOutput: 'a' }]));
assert.equal((await check(invMachine, workScenario(16))).result.invocations, 4096);
await check(invMachine, workScenario(17), 422);
report('global operation and invocation budgets include every decision execution across scenario steps');

const long = str('x'.repeat(4096));
const largeDecisionPackage = packageDefinition('large-manifest', { inputs: [port('value', 'string')], outputs: [port('accepted', 'boolean'), port('value', 'string')], nodes: [], bindings: { accepted: literal(bool(true)), value: input('value') } }, [], Array.from({ length: 10 }, (_, i) => ({ name: `case${i}`, inputs: { value: long }, expected: { accepted: bool(true), value: long } })));
const largeDecision = await prepareComponent(largeDecisionPackage);
const hugeMachine = simpleMachine('manifest-limit', Array.from({ length: 8 }, (_, i) => ({ ...simpleCommand(`command${i}`, largeDecision), bindings: { value: { literal: str('') } } })));
await prepareInvalid(hugeMachine);
const hugeScenario = scenario('Scenario byte limit', Array.from({ length: 8 }, (_, i) => instance(`I${i}`, { value: wideValue(3900) })), Array.from({ length: 64 }, (_, i) => commandStep(`I${i % 8}`, `c${i}`, Math.floor(i / 8), {}, undefined, 'commit')));
hugeScenario.expected = [expectation('I0', { value: wideValue(3900) }, 8, 8, 0)];
await check(wideMachine, hugeScenario, 422);
await request('/api/state/prepare', { method: 'POST', raw: JSON.stringify({ ...definition, description: 'x'.repeat(2 * 1024 * 1024) }), expected: 413 });
report('machine/scenario canonical byte ceilings and the HTTP body limit bound complete portable artifacts');

const examples = await get('/api/state/examples');
assert.equal(examples.length, 2); assert.deepEqual(examples[0].machine, examples[1].machine);
assert.equal(examples[0].machine.machine.id, 'lce.resource-reservation');
const reservationCases = [
  [5, 2, true, true, true, 3, 'reserved'], [5, 0, true, true, false, 5, 'invalid_quantity'],
  [5, -1, true, true, false, 5, 'invalid_quantity'], [5, 6, true, true, false, 5, 'insufficient'],
  [5, 1, false, true, false, 5, 'wrong_target'], [5, 1, true, false, false, 5, 'wrong_target'],
  [-1, 1, true, true, false, -1, 'invalid_state'], [max, -max, true, true, false, max, 'invalid_quantity'],
  [max, max, true, true, true, 0, 'reserved'], [0, 1, true, true, false, 0, 'insufficient'],
];
for (const [domain, example] of examples.entries()) {
  const machineLock = example.machine;
  const commandRef = machineLock.machine.commands[0].inputs.find(p => p.name === 'request').recordRef;
  const tenantId = domain ? 'SchoolAlpha' : 'HospitalAlpha', resourceId = domain ? 'BookCopies' : 'DrugUnits';
  for (const [available, quantity, sameTenant, sameResource, accepted, remaining, reason] of reservationCases) {
    const requestValue = record(commandRef, { tenantId: str(sameTenant ? tenantId : 'OtherTenant'), resourceId: str(sameResource ? resourceId : 'OtherResource'), quantity: int(quantity) });
    const initial = { tenantId: str(tenantId), resourceId: str(resourceId), available: int(available), acceptedSinceRestart: int(10) };
    const expectedState = { ...initial, available: int(remaining), acceptedSinceRestart: int(accepted ? 11 : 10) };
    const testScenario = scenario(`${domain ? 'Education' : 'Healthcare'} reservation reference`, [instance('Owner', initial)], [commandStep('Owner', 'reserveOne', 0, { request: requestValue }, undefined, 'reserve')], [expectation('Owner', expectedState, accepted ? 1 : 0, 1, accepted ? 1 : 0)]);
    const reference = (await check(machineLock, testScenario)).result;
    assert.equal(reference.passed, true); assert.equal(reference.steps[0].status, accepted ? 'committed' : 'denied');
    assert.deepEqual(reference.steps[0].receipt.outputs, { accepted: bool(accepted), remaining: int(remaining), acceptedSinceRestart: int(accepted ? 11 : 10), reason: str(reason) });
    assert.equal(reference.instances[0].effects.length, accepted ? 1 : 0);
  }
}
report('healthcare and education reuse one exact machine and match independent reservation, target and integer-boundary references');

const installations = await Promise.all([request('/api/state/machines', { method: 'POST', body: machine, expected: [200, 201] }), request('/api/state/machines', { method: 'POST', body: machine, expected: [200, 201] })]);
assert.deepEqual(installations.map(r => r.status).sort(), [200, 201]); assert.deepEqual(installations[0].data, installations[1].data);
const installed = installations[0].data;
assert.deepEqual(await post('/api/state/machines', machine), installed);
const changedDefinition = clone(definition); changedDefinition.name += ' changed';
const conflictingMachine = await prepare(changedDefinition); await post('/api/state/machines', conflictingMachine, 409);
const query = new URLSearchParams({ id: definition.id, version: definition.version, digest: machine.digest });
assert.deepEqual(await get(`/api/state/machine?${query}`), installed);
query.set('digest', '0'.repeat(64)); await get(`/api/state/machine?${query}`, 404);
query.set('digest', 'invalid'); await get(`/api/state/machine?${query}`, 422);
const raceDefinition = clone(definition); raceDefinition.id += '-race';
const raceA = await prepare(raceDefinition); raceDefinition.name += ' alternate'; const raceB = await prepare(raceDefinition);
const race = await Promise.all([raceA, raceB].map(body => request('/api/state/machines', { method: 'POST', body, expected: [201, 409] })));
assert.deepEqual(race.map(r => r.status).sort(), [201, 409]);
const machines = await get('/api/state/machines'); assert.deepEqual(machines.find(r => r.machine.id === raceA.machine.id), race.find(r => r.status === 201).data);
assert.ok(!(await get('/api/components')).some(r => r.package.id === reducerPackage.id));
assert.ok(!(await get('/api/data/contracts')).some(r => r.contract.id === data.contract.id));
await request('/api/state/machines', { method: 'DELETE', expected: 405 });
report('concurrent immutable machine installs retain exact versions without installing pure or data dependencies in other catalogs');

const saved = await save(machine, protocol), savedAgain = await save(machine, protocol);
assert.notEqual(saved.id, savedAgain.id); assert.deepEqual(saved.result, result); assert.deepEqual(savedAgain.result, result);
assert.deepEqual(saved.machine, machine); assert.deepEqual(saved.scenario, protocol);
const newer = clone(definition); newer.version = '1.0.1'; newer.state[0].initial = int(777);
await post('/api/state/machines', await prepare(newer), 201);
assert.deepEqual(await get(`/api/state/runs/${saved.id}`), saved);
assert.deepEqual(await get(`/api/state/runs/${failedExpectations.id}`), failedExpectations);
const history = await get('/api/state/runs'); assert.ok(history.length <= 50);
const summary = history.find(r => r.id === saved.id); assert.ok(summary); assert.equal(summary.machineId, definition.id); assert.equal(summary.machineVersion, '1.0.0'); assert.equal(summary.stepCount, 15); assert.equal(summary.verified, true); assert.equal(summary.passed, true); assert.equal(summary.scenario, undefined); assert.equal(summary.result, undefined);
const beforeCheck = await get('/api/state/runs'); await check(machine, protocol); assert.deepEqual(await get('/api/state/runs'), beforeCheck);
await save(machine, malformedScenarios[0], 422); await save(tampered, protocol, 422); assert.deepEqual(await get('/api/state/runs'), beforeCheck);
await request(`/api/state/runs/${saved.id}`, { method: 'DELETE', expected: 405 });
const oldProject = await get('/api/example?template=branching'); oldProject.machines = [machine]; await post('/api/validate?draft=true', oldProject, 400);
await post('/api/components/prepare', { package: { ...reducerPackage, state: definition.state }, dependencies: [] }, 400);
const dataExample = (await get('/api/data/examples'))[0]; assert.equal((await post('/api/data/check', { contract: dataExample.contract, fixture: dataExample.fixture })).result.engineVersion, '0.5.0');
report('PostgreSQL preserves complete run snapshots while repeated submissions restart simulations and old artifact schemas remain unchanged');

console.log(`\n${checks} state acceptance checks passed against ${base}.`);
