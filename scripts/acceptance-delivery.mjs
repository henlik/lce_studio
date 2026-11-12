#!/usr/bin/env node
// Independent delivery protocol and numerical references. Use a disposable database:
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

const prepareDelivery = model => post('/api/delivery/prepare', model);
const checkDelivery = (model, scenario, expected = 200) => post('/api/delivery/check', { model, scenario }, expected);
const saveDelivery = (model, scenario, expected = 201) => post('/api/delivery/runs', { model, scenario }, expected);
const machineRef = lock => ({ machineId: lock.machine.id, version: lock.machine.version, digest: lock.digest });
const owner = (id, lock, x = 0) => ({ id, name: `Owner ${id}`, machine: machineRef(lock), position: { x, y: 0 } });
const route = (id, sourceInstanceId, effect, targetInstanceId, command, inputs) => ({ id, name: `Route ${id}`, sourceInstanceId, effect, targetInstanceId, command, inputs });
const network = (name, machines, instances, routes = []) => ({ schemaVersion: 1, id: `${prefix}.${name}`, version: '1.0.0', name, description: 'Independent deterministic delivery reference.', author: 'Acceptance suite', license: 'MIT', machines, instances, routes });
const initial = (instanceId, values = {}) => ({ instanceId, initial: values });
const experiment = (name, initial, steps, expected = [], expectedMessages = []) => ({ schemaVersion: 1, name, initial, steps, expected, expectedMessages });
const messageExpectation = (name, status, attempts, accepted) => ({ name, status, attempts, ...(accepted !== undefined ? { accepted } : {}) });
const dispatch = (routeId, message, expectedRevision, selector) => ({ kind: 'dispatch', expectedRevision, inputs: null, routeId, ...selector, message });
const deliver = (message, fault) => ({ kind: 'deliver', inputs: null, ...(fault ? { fault } : {}), message });
const direct = (instanceId, commandId, command, expectedRevision, inputs, fault) => commandStep(instanceId, commandId, expectedRevision, inputs, fault, command);
function orderedNetwork(m) {
  return { schemaVersion: m.schemaVersion, id: m.id, version: m.version, name: m.name, description: m.description, author: m.author, license: m.license,
    machines: m.machines.map(l => ({ machine: orderedMachine(l.machine), digest: l.digest })),
    instances: m.instances.map(i => ({ id: i.id, name: i.name, machine: { machineId: i.machine.machineId, version: i.machine.version, digest: i.machine.digest }, position: { x: i.position.x, y: i.position.y } })),
    routes: m.routes.map(r => ({ id: r.id, name: r.name, sourceInstanceId: r.sourceInstanceId, effect: r.effect, targetInstanceId: r.targetInstanceId, command: r.command, inputs: map(r.inputs) })),
  };
}
function orderedExperiment(s) {
  return { schemaVersion: s.schemaVersion, name: s.name,
    initial: s.initial.map(i => ({ instanceId: i.instanceId, initial: map(i.initial, orderedValue) })),
    steps: s.steps.map(t => ({ kind: t.kind, ...(t.instanceId ? { instanceId: t.instanceId } : {}), ...(t.commandId ? { commandId: t.commandId } : {}), ...(t.command ? { command: t.command } : {}), ...(t.expectedRevision !== undefined && t.expectedRevision !== null ? { expectedRevision: t.expectedRevision } : {}), inputs: t.inputs === undefined ? null : map(t.inputs, orderedValue), ...(t.fault ? { fault: t.fault } : {}), ...(t.routeId ? { routeId: t.routeId } : {}), ...(t.sourceCommandId ? { sourceCommandId: t.sourceCommandId } : {}), ...(t.sourceMessage ? { sourceMessage: t.sourceMessage } : {}), ...(t.message ? { message: t.message } : {}) })),
    expected: s.expected.map(e => ({ instanceId: e.instanceId, state: map(e.state, orderedValue), revision: e.revision, receipts: e.receipts, effects: e.effects })),
    expectedMessages: s.expectedMessages.map(e => ({ name: e.name, status: e.status, attempts: e.attempts, ...(e.accepted !== undefined && e.accepted !== null ? { accepted: e.accepted } : {}) })),
  };
}
function base32(bytes) {
  const alphabet = 'abcdefghijklmnopqrstuvwxyz234567'; let output = '', pending = 0, bits = 0;
  for (const byte of bytes) { pending = (pending << 8) | byte; bits += 8; while (bits >= 5) { bits -= 5; output += alphabet[(pending >>> bits) & 31]; } pending &= (1 << bits) - 1; }
  if (bits) output += alphabet[(pending << (5 - bits)) & 31];
  return output;
}
const messageID = (intent, routeID) => `msg_${base32(createHash('sha256').update(typedJSON([intent, routeID])).digest())}`;
const stateS = (sent, session) => ({ sent: int(sent), session: int(session) });
const stateR = (total, ticks) => ({ total: int(total), ticks: int(ticks) });
const sendInputs = (amount, allow = true, accept = true) => ({ amount: int(amount), allow: bool(allow), accept: bool(accept) });
const receiveInputs = (amount, allow = true) => ({ amount: int(amount), allow: bool(allow) });

assert.equal((await get('/api/health')).version, '0.7.0');
assert.equal((await get('/api/components/primitives')).length, 11); assert.equal((await get('/api/components/structured-primitives')).length, 16);
const sourcePackage = packageDefinition('delivery-source-decision', {
  inputs: [port('current', 'integer'), port('session', 'integer'), port('amount', 'integer'), port('allow', 'boolean'), port('accept', 'boolean')],
  outputs: [port('accepted', 'boolean'), port('nextSent', 'integer'), port('nextSession', 'integer'), port('amount', 'integer'), port('allow', 'boolean')],
  nodes: [primitive('sent', 'add', { left: input('current'), right: literal(int(1)) }), primitive('session', 'add', { left: input('session'), right: literal(int(1)) })],
  bindings: { accepted: input('accept'), nextSent: source('sent'), nextSession: source('session'), amount: input('amount'), allow: input('allow') },
});
const sourceBundle = await prepareComponent(sourcePackage);
assert.equal(sourceBundle.root.digest, sha(orderedPackage(sourcePackage)));
const sourceCommand = { name: 'emit', inputs: [port('amount', 'integer'), port('allow', 'boolean'), port('accept', 'boolean')], outputs: [port('sent', 'integer'), port('session', 'integer')], decision: sourceBundle,
  bindings: { current: { state: 'sent' }, session: { state: 'session' }, amount: { input: 'amount' }, allow: { input: 'allow' }, accept: { input: 'accept' } }, acceptedOutput: 'accepted', writes: { sent: 'nextSent', session: 'nextSession' }, results: { sent: 'nextSent', session: 'nextSession' }, effects: [{ effect: 'Transfer', payload: { amount: 'amount', allow: 'allow' } }] };
const relayCommand = clone(sourceCommand); relayCommand.name = 'relay'; relayCommand.inputs = relayCommand.inputs.filter(p => p.name !== 'accept'); relayCommand.bindings.accept = literal(bool(true));
const sourceDefinition = machineDefinition('delivery-source', [slot('sent', 'integer', 'durable', int(0)), slot('session', 'integer', 'volatile', int(10))], [sourceCommand, relayCommand], [{ name: 'Transfer', payload: [port('amount', 'integer'), port('allow', 'boolean')] }]);
const sourceMachine = await prepare(sourceDefinition);
const targetPackage = packageDefinition('delivery-target-decision', {
  inputs: [port('current', 'integer'), port('session', 'integer'), port('amount', 'integer'), port('allow', 'boolean')],
  outputs: [port('accepted', 'boolean'), port('nextTotal', 'integer'), port('nextTicks', 'integer'), port('amount', 'integer'), port('allow', 'boolean')],
  nodes: [primitive('total', 'add', { left: input('current'), right: input('amount') }), primitive('ticks', 'add', { left: input('session'), right: literal(int(1)) })],
  bindings: { accepted: input('allow'), nextTotal: source('total'), nextTicks: source('ticks'), amount: input('amount'), allow: input('allow') },
});
const targetBundle = await prepareComponent(targetPackage);
const targetCommand = { name: 'receive', inputs: [port('amount', 'integer'), port('allow', 'boolean')], outputs: [port('total', 'integer'), port('ticks', 'integer')], decision: targetBundle, bindings: { current: { state: 'total' }, session: { state: 'ticks' }, amount: { input: 'amount' }, allow: { input: 'allow' } }, acceptedOutput: 'accepted', writes: { total: 'nextTotal', ticks: 'nextTicks' }, results: { total: 'nextTotal', ticks: 'nextTicks' }, effects: [{ effect: 'Forward', payload: { amount: 'amount', allow: 'allow' } }] };
const adjustCommand = clone(targetCommand); adjustCommand.name = 'adjust'; adjustCommand.effects = [];
const targetDefinition = machineDefinition('delivery-target', [slot('total', 'integer', 'durable', int(0)), slot('ticks', 'integer', 'volatile', int(10))], [targetCommand, adjustCommand], [{ name: 'Forward', payload: [port('amount', 'integer'), port('allow', 'boolean')] }]);
const targetMachine = await prepare(targetDefinition);
assert.equal(sourceMachine.digest, sha(orderedMachine(sourceDefinition))); assert.equal(targetMachine.digest, sha(orderedMachine(targetDefinition)));
const payloadMap = { amount: 'amount', allow: 'allow' };
const modelDefinition = network('delivery-reference', [sourceMachine, targetMachine], [owner('S', sourceMachine), owner('R', targetMachine, 400), owner('T', targetMachine, 800)], [route('sr', 'S', 'Transfer', 'R', 'receive', payloadMap), route('st', 'S', 'Transfer', 'T', 'receive', payloadMap), route('rt', 'R', 'Forward', 'T', 'receive', payloadMap), route('rs', 'R', 'Forward', 'S', 'relay', payloadMap), route('rr', 'R', 'Forward', 'R', 'receive', payloadMap)]);
const model = await prepareDelivery(modelDefinition);
assert.deepEqual(model.model, modelDefinition); assert.equal(model.digest, sha(orderedNetwork(modelDefinition)));
const oldState = await check(sourceMachine, scenario('Legacy reserved-prefix compatibility', [instance('S')], [direct('S', 'msg_direct', 'emit', 0, sendInputs(2))]));
assert.equal(oldState.result.engineVersion, '0.6.0'); assert.equal(oldState.result.steps[0].status, 'committed'); assert.equal(oldState.result.steps[0].decision.engineVersion, '0.5.0');
const dataExample = (await get('/api/data/examples'))[0]; assert.equal((await post('/api/data/check', { contract: dataExample.contract, fixture: dataExample.fixture })).result.engineVersion, '0.5.0');
report('exact portable source and recipient machines retain legacy schemas, digests, reserved-name behavior and engine versions');

const initialOwners = () => [initial('T'), initial('S', { session: int(100) }), initial('R')];
const primarySteps = [
  direct('S', 'sendTwo', 'emit', 0, sendInputs(2)), dispatch('sr', 'first', 0, { sourceCommandId: 'sendTwo' }), dispatch('sr', 'first', 0, { sourceCommandId: 'sendTwo' }),
  deliver('first', 'drop_before_receive'), deliver('first', 'before_recipient_commit'), deliver('first', 'after_recipient_commit_before_ack'),
  direct('R', 'adjustThree', 'adjust', 1, receiveInputs(3)), restart('R'), restart('S'), deliver('first', 'after_recipient_commit_before_ack'),
  dispatch('sr', 'first', 0, { sourceCommandId: 'sendTwo' }), deliver('first', 'before_recipient_commit'), deliver('first', 'drop_before_receive'),
  dispatch('sr', 'first', 0, { sourceCommandId: 'sendTwo' }), dispatch('sr', 'first', 1, { sourceCommandId: 'sendTwo' }),
];
const primary = experiment('Lost acknowledgement, frozen retry and owner restarts', initialOwners(), primarySteps,
  [expectation('S', stateS(1, 100), 1, 1, 1), expectation('R', stateR(5, 10), 2, 2, 1), expectation('T', stateR(0, 10), 0, 0, 0)], [messageExpectation('first', 'acknowledged', 5, true)]);
const primaryCheck = await checkDelivery(model, primary), result = primaryCheck.result;
assert.deepEqual(primaryCheck.scenario, primary); assert.equal(result.modelHash, model.digest); assert.equal(result.scenarioHash, sha(orderedExperiment(primary))); assert.equal(result.engineVersion, '0.7.0');
assert.equal(result.verified, true); assert.equal(result.passed, true); assert.equal(result.operations, 8); assert.equal(result.invocations, 4);
assert.deepEqual(result.steps.map(s => s.status), ['committed', 'dispatched', 'dispatch_replayed', 'dropped', 'recipient_unacknowledged', 'acknowledgement_lost', 'committed', 'restarted', 'restarted', 'acknowledgement_lost', 'dispatch_replayed', 'acknowledged', 'already_acknowledged', 'dispatch_replayed', 'dispatch_conflict']);
assert.deepEqual(result.instances.map(i => i.id), ['S', 'R', 'T']);
assert.deepEqual(result.instances[0].state, stateS(1, 100)); assert.deepEqual(result.instances[1].state, stateR(5, 10)); assert.deepEqual(result.instances[2].state, stateR(0, 10));
result.steps.forEach((s, i) => { assert.equal(s.index, i); if (s.state) assert.equal(s.state.index, i); assert.equal(s.faultApplied, [3, 4, 5, 9].includes(i)); });
assert.equal(result.steps[3].state, undefined); assert.equal(result.steps[4].state.status, 'rolled_back'); assert.equal(result.steps[5].state.status, 'committed'); assert.equal(result.steps[5].state.replyLost, false);
assert.equal(result.steps[9].state.status, 'replayed'); assert.equal(result.steps[9].state.decision, undefined); assert.equal(result.steps[11].state.status, 'replayed'); assert.equal(result.steps[11].state.faultApplied, false); assert.equal(result.steps[12].state, undefined);
for (const i of [1, 2, 10, 13, 14]) assert.equal(result.steps[i].state, undefined);
assert.deepEqual(result.steps[9].state.after, stateR(5, 10)); assert.deepEqual(result.steps[9].state.receipt.outputs, { total: int(2), ticks: int(11) });
assert.deepEqual(result.steps[11].state.after, stateR(5, 10)); assert.deepEqual(result.steps[4].state.before, result.steps[4].state.after);
assert.equal(result.steps[3].beforeAttempts, 0); assert.equal(result.steps[3].afterAttempts, 1); assert.equal(result.steps[11].afterAttempts, 5); assert.equal(result.steps[12].afterAttempts, 5);
assert.equal(result.steps[1].beforeStatus, ''); assert.equal(result.steps[1].afterStatus, 'pending'); assert.equal(result.steps[5].afterStatus, 'pending'); assert.equal(result.steps[11].afterStatus, 'acknowledged');
assert.deepEqual((await checkDelivery(model, primary)).result, result);
report('independent numerical outcomes distinguish dropped delivery, rollback, committed lost acknowledgements and historical receipt replay');

const intent = effectID('S', 'sendTwo', 'Transfer'), firstID = messageID(intent, 'sr');
assert.match(firstID, /^msg_[a-z2-7]{52}$/); assert.equal(firstID.length, 56);
const forwardID = effectID('R', firstID, 'Forward');
const originalReceipt = { commandId: firstID, requestHash: requestHash({ command: 'receive', expectedRevision: 0, inputs: receiveInputs(2) }), command: 'receive', accepted: true, revision: 1, outputs: { total: int(2), ticks: int(11) }, effectIds: [forwardID] };
assert.deepEqual(result.messages, [{ name: 'first', id: firstID, routeId: 'sr', sourceIntentId: intent, sourceInstanceId: 'S', sourceCommandId: 'sendTwo', effect: 'Transfer', targetInstanceId: 'R', command: 'receive', expectedRevision: 0, inputs: receiveInputs(2), status: 'acknowledged', attempts: 5, acknowledgement: originalReceipt }]);
assert.deepEqual(result.instances[1].receipts[0], originalReceipt); assert.deepEqual(result.steps[5].state.receipt, originalReceipt); assert.deepEqual(result.steps[11].state.receipt, originalReceipt);
assert.deepEqual(result.instances[0].effects[0], { id: intent, instanceId: 'S', commandId: 'sendTwo', effect: 'Transfer', revision: 1, payload: receiveInputs(2) });
assert.equal(result.instances[1].effects[0].id, forwardID); assert.equal(result.instances[1].effects[0].commandId, firstID);
const relabeled = clone(primary); relabeled.name = 'Different label';
const labeledResult = (await checkDelivery(model, relabeled)).result; assert.notEqual(labeledResult.scenarioHash, result.scenarioHash); assert.deepEqual(labeledResult.messages, result.messages); assert.deepEqual(labeledResult.instances, result.instances);
const aliasChanged = clone(primary); aliasChanged.steps.forEach(s => { if (s.message === 'first') s.message = 'renamed'; }); aliasChanged.expectedMessages[0].name = 'renamed';
assert.equal((await checkDelivery(model, aliasChanged)).result.messages[0].id, firstID);
report('full SHA-256/base32 message IDs and v0.6 request hashes are independent of aliases, faults, attempts and labels');

const denied = experiment('Business denial is acknowledged processing', initialOwners(), [direct('S', 'denyTarget', 'emit', 0, sendInputs(4, false)), dispatch('sr', 'denial', 0, { sourceCommandId: 'denyTarget' }), deliver('denial', 'before_recipient_commit'), deliver('denial', 'after_recipient_commit_before_ack'), restart('R'), deliver('denial', 'after_recipient_commit_before_ack'), deliver('denial', 'before_recipient_commit')], [expectation('R', stateR(0, 10), 0, 1, 0)], [messageExpectation('denial', 'acknowledged', 4, false)]);
const denial = (await checkDelivery(model, denied)).result;
assert.equal(denial.passed, true); assert.equal(denial.operations, 6); assert.equal(denial.invocations, 3);
assert.deepEqual(denial.steps.map(s => s.status), ['committed', 'dispatched', 'recipient_unacknowledged', 'acknowledgement_lost', 'restarted', 'acknowledgement_lost', 'acknowledged']);
assert.equal(denial.steps[2].state.status, 'rolled_back'); assert.equal(denial.steps[3].state.status, 'denied'); assert.equal(denial.steps[5].state.status, 'replayed'); assert.equal(denial.steps[5].faultApplied, true);
assert.equal(denial.messages[0].acknowledgement.accepted, false); assert.equal(denial.messages[0].acknowledgement.revision, 0); assert.deepEqual(denial.messages[0].acknowledgement.effectIds, []); assert.deepEqual(denial.messages[0].acknowledgement.outputs, { total: int(4), ticks: int(11) });
report('committed denials acknowledge delivery while preserving business state and acknowledgement loss also applies to replayed denials');

const stale = experiment('Explicit revisions never rebase during retries', initialOwners(), [direct('S', 'one', 'emit', 0, sendInputs(2)), direct('S', 'two', 'emit', 1, sendInputs(3)), dispatch('sr', 'older', 0, { sourceCommandId: 'one' }), dispatch('sr', 'newer', 0, { sourceCommandId: 'two' }), deliver('newer'), deliver('older'), restart('R'), deliver('older', 'before_recipient_commit'), dispatch('sr', 'older', 1, { sourceCommandId: 'one' }), deliver('older', 'after_recipient_commit_before_ack')], [expectation('R', stateR(3, 10), 1, 1, 1)], [messageExpectation('older', 'pending', 3), messageExpectation('newer', 'acknowledged', 1, true)]);
const staleResult = (await checkDelivery(model, stale)).result;
assert.equal(staleResult.passed, true); assert.equal(staleResult.operations, 6); assert.equal(staleResult.invocations, 3);
for (const i of [5, 7, 9]) { assert.equal(staleResult.steps[i].status, 'recipient_unacknowledged'); assert.equal(staleResult.steps[i].state.status, 'revision_conflict'); assert.equal(staleResult.steps[i].faultApplied, false); assert.equal(staleResult.steps[i].state.decision, undefined); }
assert.equal(staleResult.steps[8].status, 'dispatch_conflict'); assert.equal(staleResult.messages[0].expectedRevision, 0); assert.equal(staleResult.messages[0].acknowledgement, undefined);
const failingInitial = initialOwners(); failingInitial.find(i => i.instanceId === 'R').initial.total = int(9007199254740991);
const failing = (await checkDelivery(model, experiment('Ordinary recipient arithmetic errors', failingInitial, [direct('S', 'overflow', 'emit', 0, sendInputs(1)), dispatch('sr', 'overflow', 0, { sourceCommandId: 'overflow' }), deliver('overflow', 'after_recipient_commit_before_ack'), deliver('overflow')], [expectation('R', stateR(9007199254740991, 10), 0, 0, 0)], [messageExpectation('overflow', 'pending', 2)]))).result;
assert.equal(failing.passed, true); assert.equal(failing.operations, 2); for (const i of [2, 3]) { assert.equal(failing.steps[i].state.status, 'evaluation_error'); assert.equal(failing.steps[i].faultApplied, false); }
report('stale envelopes and ordinary recipient failures remain pending with no silent revision refresh or false acknowledgement');

const missing = experiment('Missing committed effects do not reserve message aliases', initialOwners(), [dispatch('sr', 'later', 0, { sourceCommandId: 'lateCommand' }), deliver('later'), dispatch('rt', 'downstreamMissing', 0, { sourceMessage: 'later' }), direct('S', 'lateCommand', 'emit', 0, sendInputs(2)), dispatch('sr', 'later', 0, { sourceCommandId: 'lateCommand' }), deliver('later'), direct('S', 'deniedSource', 'emit', 1, sendInputs(1, true, false)), dispatch('sr', 'never', 1, { sourceCommandId: 'deniedSource' }), deliver('never')], [], [messageExpectation('later', 'acknowledged', 1, true), messageExpectation('downstreamMissing', 'absent', 0), messageExpectation('never', 'absent', 0), messageExpectation('undeclaredAbsence', 'absent', 0)]);
const missingResult = (await checkDelivery(model, missing)).result;
assert.equal(missingResult.passed, true); assert.deepEqual(missingResult.steps.map(s => s.status), ['missing_intent', 'missing_message', 'missing_source_message', 'committed', 'dispatched', 'acknowledged', 'denied', 'missing_intent', 'missing_message']); assert.equal(missingResult.messages.length, 1);
report('missing and denied source effects produce explicit outcomes without materializing or poisoning later dispatch aliases');

const chained = experiment('Explicit downstream delivery and bounded cycles', initialOwners(), [direct('S', 'begin', 'emit', 0, sendInputs(2)), dispatch('sr', 'upstream', 0, { sourceCommandId: 'begin' }), deliver('upstream', 'after_recipient_commit_before_ack'), dispatch('rt', 'tail', 0, { sourceMessage: 'upstream' }), deliver('tail'), dispatch('rs', 'back', 1, { sourceMessage: 'upstream' }), deliver('back'), dispatch('sr', 'cycle', 1, { sourceMessage: 'back' }), dispatch('rr', 'self', 1, { sourceMessage: 'upstream' }), deliver('self')], [expectation('S', stateS(2, 102), 2, 2, 2), expectation('R', stateR(4, 12), 2, 2, 2), expectation('T', stateR(2, 11), 1, 1, 1)], [messageExpectation('upstream', 'pending', 1), messageExpectation('tail', 'acknowledged', 1, true), messageExpectation('back', 'acknowledged', 1, true), messageExpectation('cycle', 'pending', 0), messageExpectation('self', 'acknowledged', 1, true)]);
const chainResult = (await checkDelivery(model, chained)).result;
assert.equal(chainResult.passed, true); assert.equal(chainResult.operations, 10); assert.equal(chainResult.invocations, 5); assert.equal(chainResult.messages.length, 5);
assert.equal(chainResult.messages[1].sourceCommandId, chainResult.messages[0].id); assert.equal(chainResult.messages[3].sourceCommandId, chainResult.messages[2].id);
assert.equal(chainResult.messages[1].sourceIntentId, effectID('R', chainResult.messages[0].id, 'Forward')); assert.equal(chainResult.messages[0].acknowledgement, undefined);
assert.equal(chainResult.messages[3].attempts, 0); assert.deepEqual(chainResult.instances[1].effects[0].payload, receiveInputs(2));
const fanout = experiment('One intent fans out to independent owners', initialOwners(), [direct('S', 'fan', 'emit', 0, sendInputs(3)), dispatch('sr', 'right', 0, { sourceCommandId: 'fan' }), dispatch('st', 'third', 0, { sourceCommandId: 'fan' }), deliver('right'), deliver('third')], [expectation('R', stateR(3, 11), 1, 1, 1), expectation('T', stateR(3, 11), 1, 1, 1)], [messageExpectation('right', 'acknowledged', 1, true), messageExpectation('third', 'acknowledged', 1, true)]);
const fan = (await checkDelivery(model, fanout)).result;
assert.equal(fan.passed, true); assert.equal(fan.instances[0].effects.length, 1); assert.equal(fan.messages[0].sourceIntentId, fan.messages[1].sourceIntentId); assert.notEqual(fan.messages[0].id, fan.messages[1].id); assert.equal(fan.messages[0].acknowledgement.requestHash, fan.messages[1].acknowledgement.requestHash);
report('downstream intents can dispatch before upstream acknowledgement while self-routes, cycles and fanout require explicit finite steps');

const invalidModels = [];
const mutateModel = fn => { const value = clone(modelDefinition); fn(value); invalidModels.push(value); };
mutateModel(m => { m.instances[1].machine.digest = '0'.repeat(64); });
mutateModel(m => { m.instances[1].id = 'S'; });
mutateModel(m => { m.instances[0].position.x = 1000001; });
mutateModel(m => { m.routes[0].sourceInstanceId = 'missing'; });
mutateModel(m => { m.routes[0].targetInstanceId = 'missing'; });
mutateModel(m => { m.routes[0].effect = 'missing'; });
mutateModel(m => { m.routes[0].command = 'missing'; });
mutateModel(m => { delete m.routes[0].inputs.allow; });
mutateModel(m => { m.routes[0].inputs.extra = 'amount'; });
mutateModel(m => { m.routes[0].inputs.amount = 'allow'; });
mutateModel(m => { m.routes[0].inputs.amount = 'amount.value'; });
mutateModel(m => { m.routes[0].inputs = null; });
mutateModel(m => { m.routes.push({ ...clone(m.routes[0]), id: 'duplicate', name: 'Same business destination' }); });
mutateModel(m => { m.routes[1].id = m.routes[0].id; });
mutateModel(m => { m.machines.push(clone(m.machines[0])); });
mutateModel(m => { m.machines[0].machine.description += ' unsigned'; });
mutateModel(m => { m.machines = null; });
mutateModel(m => { m.instances = []; });
mutateModel(m => { m.routes = null; });
for (const invalid of invalidModels) await post('/api/delivery/prepare', invalid, 422);
const noRoutes = clone(modelDefinition); noRoutes.routes = []; await prepareDelivery(noRoutes);
const reversedInitial = clone(primary); reversedInitial.initial.reverse();
assert.deepEqual((await checkDelivery(model, reversedInitial)).result.instances, result.instances);
report('models reject ambiguous duplicate routes, invalid exact references and incomplete or incompatible effect mappings');

const invalidScenarios = [];
const simple = () => experiment('Invalid scenario reference', initialOwners(), [clone(primarySteps[0]), clone(primarySteps[1]), deliver('first')]);
const mutateExperiment = fn => { const value = simple(); fn(value); invalidScenarios.push(value); };
mutateExperiment(s => { s.initial.pop(); });
mutateExperiment(s => { s.initial.push(clone(s.initial[0])); });
mutateExperiment(s => { s.initial[0].instanceId = 'unknown'; });
mutateExperiment(s => { s.initial[0].initial = null; });
mutateExperiment(s => { s.initial[0].initial.unknown = int(0); });
mutateExperiment(s => { s.initial[1].initial.session = str('100'); });
mutateExperiment(s => { s.steps[0].commandId = 'msg_spoof'; });
mutateExperiment(s => { s.steps[1].sourceCommandId = 'msg_spoof'; });
mutateExperiment(s => { s.steps[0].inputs.extra = int(0); });
mutateExperiment(s => { s.steps[0].expectedRevision = null; });
mutateExperiment(s => { delete s.steps[1].expectedRevision; });
mutateExperiment(s => { s.steps[1].expectedRevision = 9007199254740992; });
mutateExperiment(s => { s.steps[1].expectedRevision = -1; });
mutateExperiment(s => { s.steps[1].sourceMessage = 'first'; });
mutateExperiment(s => { delete s.steps[1].sourceCommandId; });
mutateExperiment(s => { s.steps[1].inputs = {}; });
mutateExperiment(s => { s.steps[1].fault = 'drop_before_receive'; });
mutateExperiment(s => { s.steps[1].instanceId = 'S'; });
mutateExperiment(s => { s.steps[2].expectedRevision = 0; });
mutateExperiment(s => { s.steps[2].inputs = {}; });
mutateExperiment(s => { s.steps[2].fault = 'before_commit'; });
mutateExperiment(s => { s.steps[2].message = 'undeclared'; });
mutateExperiment(s => { s.steps = [deliver('first'), s.steps[1]]; });
mutateExperiment(s => { s.steps[0].fault = 'drop_before_receive'; });
mutateExperiment(s => { s.steps.push({ ...restart('R'), inputs: {} }); });
mutateExperiment(s => { s.steps.push({ ...restart('R'), routeId: 'sr' }); });
mutateExperiment(s => { s.steps.push(dispatch('st', 'first', 0, { sourceCommandId: 'sendTwo' })); });
mutateExperiment(s => { s.steps.push(dispatch('sr', 'anotherAlias', 0, { sourceCommandId: 'sendTwo' })); });
mutateExperiment(s => { s.steps.push(dispatch('st', 'badChain', 0, { sourceMessage: 'first' })); });
mutateExperiment(s => { s.steps.push(dispatch('rt', 'future', 0, { sourceMessage: 'notYet' })); });
mutateExperiment(s => { s.expectedMessages = null; });
mutateExperiment(s => { s.expectedMessages = [messageExpectation('first', 'delivered', 1)]; });
mutateExperiment(s => { s.expectedMessages = [messageExpectation('first', 'pending', 1, false)]; });
mutateExperiment(s => { s.expectedMessages = [messageExpectation('nothing', 'absent', 1)]; });
mutateExperiment(s => { s.expectedMessages = [messageExpectation('first', 'acknowledged', 0)]; });
mutateExperiment(s => { s.expectedMessages = [messageExpectation('first', 'pending', -1)]; });
mutateExperiment(s => { s.expectedMessages = [messageExpectation('first', 'pending', 9007199254740992)]; });
mutateExperiment(s => { s.expectedMessages = [messageExpectation('first', 'pending', 0), messageExpectation('first', 'pending', 0)]; });
for (const invalid of invalidScenarios) await checkDelivery(model, invalid, 422);
for (const missing of [true, false]) { const invalid = simple(); invalid.expectedMessages = [messageExpectation('first', 'acknowledged', 1)]; if (missing) delete invalid.expectedMessages[0].attempts; else invalid.expectedMessages[0].attempts = null; await checkDelivery(model, invalid, 400); }
const mixedCount = simple(); mixedCount.expectedMessages = [{ name: 'first', status: 'acknowledged', Attempts: 1, accepted: true }];
const normalizedCount = await checkDelivery(model, mixedCount); assert.equal(normalizedCount.result.passed, true); assert.deepEqual(normalizedCount.scenario.expectedMessages, [messageExpectation('first', 'acknowledged', 1, true)]);
const noExpectations = simple(); const unverified = (await checkDelivery(model, noExpectations)).result; assert.equal(unverified.verified, false); assert.equal(unverified.passed, false); assert.deepEqual(unverified.expectations, []); assert.deepEqual(unverified.messageExpectations, []);
const wrongExpectations = clone(primary); wrongExpectations.expectedMessages[0].accepted = false;
const failed = await saveDelivery(model, wrongExpectations); assert.equal(failed.result.verified, true); assert.equal(failed.result.passed, false); assert.ok(failed.result.messageExpectations[0].issues.length > 0); assert.deepEqual(failed.result.instances, result.instances);
report('strict step fields, ordered aliases, reserved IDs and explicit message expectations reject unsupported inputs without inventing results');

const aliasLock = clone(model); aliasLock.Model = aliasLock.model; delete aliasLock.model; aliasLock.Model.Routes = aliasLock.Model.routes; delete aliasLock.Model.routes;
const canonicalLock = await post('/api/delivery/validate-lock', aliasLock); assert.equal(canonicalLock.valid, true); assert.deepEqual(canonicalLock.model, model);
const aliasScenario = clone(primary); aliasScenario.Steps = aliasScenario.steps; delete aliasScenario.steps;
assert.deepEqual((await checkDelivery(model, aliasScenario)).scenario, primary);
const canonicalNulls = experiment('Canonical non-command inputs', initialOwners(), [{ kind: 'restart', instanceId: 'S' }, { kind: 'dispatch', routeId: 'sr', message: 'absent', expectedRevision: 0, sourceCommandId: 'never' }, { kind: 'deliver', message: 'absent' }]);
assert.ok((await checkDelivery(model, canonicalNulls)).scenario.steps.every(s => s.inputs === null));
const tampered = clone(model); tampered.model.name += ' changed'; const invalidLock = await post('/api/delivery/validate-lock', tampered); expectRejected(invalidLock); assert.equal(invalidLock.model, undefined); await checkDelivery(tampered, primary, 422);
await post('/api/delivery/prepare', { ...modelDefinition, networkScript: 'execute' }, 400);
await request('/api/delivery/prepare', { method: 'POST', raw: `${JSON.stringify(modelDefinition)} {}`, expected: 400 });
const rawFraction = JSON.stringify({ model, scenario: primary }).replace('"integer":2', '"integer":9007199254740990.5');
await request('/api/delivery/check', { method: 'POST', raw: rawFraction, expected: 400 });
const rawRevision = JSON.stringify({ model, scenario: simple() }).replace('"expectedRevision":0', '"expectedRevision":0.5');
await request('/api/delivery/check', { method: 'POST', raw: rawRevision, expected: 400 });
const rawAttempts = JSON.stringify({ model, scenario: { ...simple(), expectedMessages: [messageExpectation('first', 'acknowledged', 1)] } }).replace('"attempts":1', '"attempts":1.5');
await request('/api/delivery/check', { method: 'POST', raw: rawAttempts, expected: 400 });
const escapedModel = clone(modelDefinition); escapedModel.description = 'A < B & C > D\u2028delivery\u2029model';
assert.equal((await prepareDelivery(escapedModel)).digest, sha(orderedNetwork(escapedModel)));
report('raw delivery imports normalize validated casing and nulls while preserving exact content hashes and rejecting numeric rounding');

const dataDefinition = { schemaVersion: 1, id: `${prefix}.delivery-data`, version: '1.0.0', name: 'Delivery payload', description: 'Nominal portable payload.', author: 'Acceptance suite', license: 'MIT', records: [{ name: 'Item', fields: [port('id', 'string'), port('n', 'integer'), port('active', 'boolean')] }], entities: [], relationships: [] };
const data = await post('/api/data/prepare', dataDefinition), itemRef = dataRef(data, 'Item');
const item = (id, n) => record(itemRef, { id: str(id), n: int(n), active: bool(false) });
const structuredDecisionPackage = packageDefinition('structured-forward', { inputs: [port('value', 'record', itemRef)], outputs: [port('accepted', 'boolean'), port('value', 'record', itemRef)], nodes: [], bindings: { accepted: literal(bool(true)), value: input('value') } }, [data]);
const structuredDecision = await prepareComponent(structuredDecisionPackage);
const structuredCommand = { name: 'accept', inputs: [port('value', 'record', itemRef)], outputs: [port('value', 'record', itemRef)], decision: structuredDecision, bindings: { value: { input: 'value' } }, acceptedOutput: 'accepted', writes: { value: 'value' }, results: { value: 'value' }, effects: [{ effect: 'Changed', payload: { value: 'value' } }] };
const structuredDefinition = machineDefinition('structured-message-owner', [slot('value', 'record', 'durable', item('initial', 0), itemRef)], [structuredCommand], [{ name: 'Changed', payload: [port('value', 'record', itemRef)] }]);
const structuredMachine = await prepare(structuredDefinition);
const structuredModelDefinition = network('structured-message-model', [structuredMachine], [owner('A', structuredMachine), owner('B', structuredMachine, 400)], [route('recordRoute', 'A', 'Changed', 'B', 'accept', { value: 'value' })]);
const structuredModel = await prepareDelivery(structuredModelDefinition);
const structuredScenario = experiment('Closed nominal records survive immutable routing', [initial('A'), initial('B')], [direct('A', 'first', 'accept', 0, { value: item('one', 1) }), dispatch('recordRoute', 'recordMessage', 0, { sourceCommandId: 'first' }), direct('A', 'next', 'accept', 1, { value: item('two', 2) }), deliver('recordMessage', 'after_recipient_commit_before_ack'), restart('B'), deliver('recordMessage')], [expectation('A', { value: item('two', 2) }, 2, 2, 2), expectation('B', { value: item('one', 1) }, 1, 1, 1)], [messageExpectation('recordMessage', 'acknowledged', 2, true)]);
const structuredResult = (await checkDelivery(structuredModel, structuredScenario)).result;
assert.equal(structuredResult.passed, true); assert.deepEqual(structuredResult.messages[0].inputs, { value: item('one', 1) }); assert.deepEqual(structuredResult.messages[0].acknowledgement.outputs, { value: item('one', 1) }); assert.deepEqual(structuredResult.instances[0].effects[0].payload, { value: item('one', 1) });
const otherDataDefinition = clone(dataDefinition); otherDataDefinition.id += '-other'; const otherData = await post('/api/data/prepare', otherDataDefinition), otherRef = dataRef(otherData, 'Item');
const otherStructuredPackage = clone(structuredDecisionPackage); otherStructuredPackage.id += '-other'; otherStructuredPackage.dataContracts = [otherData]; otherStructuredPackage.behavior.inputs[0].recordRef = otherRef; otherStructuredPackage.behavior.outputs[1].recordRef = otherRef;
const otherStructuredDecision = await prepareComponent(otherStructuredPackage);
const otherStructuredDefinition = clone(structuredDefinition); otherStructuredDefinition.id += '-other'; otherStructuredDefinition.state[0].recordRef = otherRef; otherStructuredDefinition.state[0].initial = record(otherRef, item('initial', 0).record); otherStructuredDefinition.commands[0].decision = otherStructuredDecision; otherStructuredDefinition.commands[0].inputs[0].recordRef = otherRef; otherStructuredDefinition.commands[0].outputs[0].recordRef = otherRef; otherStructuredDefinition.effects[0].payload[0].recordRef = otherRef;
const otherStructuredMachine = await prepare(otherStructuredDefinition);
const nominalMismatch = clone(structuredModelDefinition); nominalMismatch.machines.push(otherStructuredMachine); nominalMismatch.instances[1].machine = machineRef(otherStructuredMachine); await post('/api/delivery/prepare', nominalMismatch, 422);
const badPayloadReference = clone(structuredScenario); badPayloadReference.steps[0].inputs.value = record(otherRef, item('one', 1).record); await checkDelivery(structuredModel, badPayloadReference, 422);
const changedPure = clone(sourcePackage); changedPure.name += ' alternate'; const changedPureBundle = await prepareComponent(changedPure);
const conflictingOwnerDef = clone(sourceDefinition); conflictingOwnerDef.id += '-conflict'; conflictingOwnerDef.commands.forEach(c => { c.decision = changedPureBundle; }); const conflictingOwner = await prepare(conflictingOwnerDef);
const conflictingModel = clone(modelDefinition); conflictingModel.machines.push(conflictingOwner); await post('/api/delivery/prepare', conflictingModel, 422);
const badIndependent = clone(structuredMachine); delete badIndependent.machine.commands[0].decision.packages[0].package.dataContracts; const badRoot = badIndependent.machine.commands[0].decision.packages[0]; badRoot.digest = sha(orderedPackage(badRoot.package)); badIndependent.machine.commands[0].decision.root.digest = badRoot.digest; badIndependent.digest = sha(orderedMachine(badIndependent.machine));
const borrowForbidden = clone(structuredModelDefinition); borrowForbidden.machines[0] = badIndependent; borrowForbidden.instances.forEach(i => { i.machine = machineRef(badIndependent); }); borrowForbidden.machines.push(otherStructuredMachine); await post('/api/delivery/prepare', borrowForbidden, 422);
report('structured payloads keep historical values and exact nominal identity; independently invalid or conflicting machine dependencies are rejected');

const capacityOwners = [owner('S', sourceMachine), ...Array.from({ length: 7 }, (_, i) => owner(`R${i + 1}`, targetMachine, (i + 1) * 100))];
const capacityRoutes = [...Array.from({ length: 7 }, (_, i) => route(`send${i + 1}`, 'S', 'Transfer', `R${i + 1}`, 'receive', { amount: 'amount', allow: 'allow' })), route('adjustOne', 'S', 'Transfer', 'R1', 'adjust', { amount: 'amount', allow: 'allow' })];
const capacityDefinition = network('capacity', [sourceMachine, targetMachine], capacityOwners, capacityRoutes);
const capacityModel = await prepareDelivery(capacityDefinition);
const capacityInitial = capacityOwners.map(i => initial(i.id));
const capacitySteps = [];
for (let i = 0; i < 4; i++) {
  capacitySteps.push(direct('S', `emit${i}`, 'emit', i, sendInputs(i)));
  capacityRoutes.forEach((r, j) => capacitySteps.push(dispatch(r.id, `message${i}route${j}`, 0, { sourceCommandId: `emit${i}` })));
}
capacitySteps.push(clone(capacitySteps[1]));
const atCapacity = experiment('Thirty-two immutable envelopes and a harmless redispatch', capacityInitial, capacitySteps, [], [messageExpectation('message0route0', 'pending', 0)]);
const capacityResult = (await checkDelivery(capacityModel, atCapacity)).result;
assert.equal(capacityResult.messages.length, 32); assert.equal(capacityResult.steps.at(-1).status, 'dispatch_replayed'); assert.equal(capacityResult.passed, true);
const overCapacity = clone(atCapacity); overCapacity.steps.push(direct('S', 'extra', 'emit', 4, sendInputs(9)), dispatch('send1', 'extraMessage', 0, { sourceCommandId: 'extra' }));
const overCapacityFailure = await saveDelivery(capacityModel, overCapacity, 422); assert.match(JSON.stringify(overCapacityFailure), /message capacity/i);
const tooManyInstances = clone(capacityDefinition); tooManyInstances.instances.push(owner('R8', targetMachine)); await post('/api/delivery/prepare', tooManyInstances, 422);
const sixteenRoutes = clone(capacityDefinition); sixteenRoutes.routes = [];
for (let i = 1; i <= 7; i++) for (const command of ['receive', 'adjust']) sixteenRoutes.routes.push(route(`route${i}${command}`, 'S', 'Transfer', `R${i}`, command, { amount: 'amount', allow: 'allow' }));
sixteenRoutes.routes.push(route('back', 'R1', 'Forward', 'S', 'relay', { amount: 'amount', allow: 'allow' }), route('self', 'R1', 'Forward', 'R1', 'receive', { amount: 'amount', allow: 'allow' }));
await prepareDelivery(sixteenRoutes); sixteenRoutes.routes.push(route('extra', 'R2', 'Forward', 'R2', 'receive', { amount: 'amount', allow: 'allow' })); await post('/api/delivery/prepare', sixteenRoutes, 422);
const sixtyFour = experiment('The explicit step boundary', initialOwners(), Array.from({ length: 64 }, () => restart('S')));
assert.equal((await checkDelivery(model, sixtyFour)).result.steps.length, 64); sixtyFour.steps.push(restart('S')); await checkDelivery(model, sixtyFour, 422);
const tooManyExpected = experiment('No more than thirty-two message assertions', initialOwners(), [restart('S')], [], Array.from({ length: 33 }, (_, i) => messageExpectation(`absent${i}`, 'absent', 0))); await checkDelivery(model, tooManyExpected, 422);
report('explicit topology, step and thirty-two-message boundaries hold without consuming source effects or rejecting harmless redispatches');

const sourceEightDefinition = clone(sourceDefinition); sourceEightDefinition.id += '-eight';
sourceEightDefinition.effects = Array.from({ length: 8 }, (_, i) => ({ name: `Effect${i}`, payload: [port('amount', 'integer'), port('allow', 'boolean')] }));
sourceEightDefinition.commands.forEach(c => { c.effects = sourceEightDefinition.effects.map(e => ({ effect: e.name, payload: { amount: 'amount', allow: 'allow' } })); });
const sourceEight = await prepare(sourceEightDefinition);
const targetEightDefinition = clone(targetDefinition); targetEightDefinition.id += '-eight'; targetEightDefinition.effects = clone(sourceEightDefinition.effects); targetEightDefinition.commands[0].effects = clone(sourceEightDefinition.commands[0].effects);
const targetEight = await prepare(targetEightDefinition);
const effectModel = await prepareDelivery(network('global-effect-capacity', [sourceEight, targetEight], [owner('A', sourceEight), owner('B', targetEight)]));
const effectSteps = [];
for (let i = 0; i < 8; i++) effectSteps.push(direct('A', `source${i}`, 'emit', i, sendInputs(1)), direct('B', `target${i}`, 'receive', i, receiveInputs(1)));
const effectScenario = experiment('Global intent capacity across distinct machines', [initial('A'), initial('B')], effectSteps);
const effectResult = (await checkDelivery(effectModel, effectScenario)).result;
assert.equal(effectResult.instances.reduce((sum, i) => sum + i.effects.length, 0), 128); assert.equal(effectResult.operations, 32); assert.equal(effectResult.invocations, 16);
const tooManyEffects = clone(effectScenario); tooManyEffects.steps.push(direct('B', 'extra', 'receive', 8, receiveInputs(1)));
assert.match(JSON.stringify(await saveDelivery(effectModel, tooManyEffects, 422)), /effect/i);
const workLeafPackage = packageDefinition('work-leaf', { inputs: [port('value', 'integer')], outputs: [port('value', 'integer')], nodes: Array.from({ length: 32 }, (_, i) => primitive(`add${i}`, 'add', { left: i ? source(`add${i - 1}`) : input('value'), right: literal(int(0)) })), bindings: { value: source('add31') } });
const workLeaf = await prepareComponent(workLeafPackage);
const workRootPackage = packageDefinition('work-root', { inputs: [port('value', 'integer')], outputs: [port('accepted', 'boolean'), port('value', 'integer')], nodes: Array.from({ length: 4 }, (_, i) => call(`child${i}`, workLeaf.root, { value: i ? source(`child${i - 1}`, 'value') : input('value') })), bindings: { accepted: literal(bool(true)), value: source('child3', 'value') } });
const workRoot = await prepareComponent(workRootPackage, workLeaf.packages);
const workCommand = { name: 'commit', inputs: [], outputs: [], decision: workRoot, bindings: { value: { state: 'value' } }, acceptedOutput: 'accepted', writes: { value: 'value' }, results: {}, effects: [] };
const workA = await prepare(machineDefinition('work-owner-a', [slot('value', 'integer', 'durable', int(0))], [workCommand]));
const workB = await prepare(machineDefinition('work-owner-b', [slot('value', 'integer', 'durable', int(0))], [workCommand]));
const workModel = await prepareDelivery(network('global-work', [workA, workB], [owner('A', workA), owner('B', workB)]));
const workScenario = experiment('Global work budget across independent machine sessions', [initial('A'), initial('B')], Array.from({ length: 32 }, (_, i) => direct(i % 2 ? 'B' : 'A', `commit${i}`, 'commit', Math.floor(i / 2), {})));
const workResult = (await checkDelivery(workModel, workScenario)).result; assert.equal(workResult.operations, 4096); assert.equal(workResult.invocations, 160); assert.deepEqual(workResult.instances.map(i => i.revision), [16, 16]);
workScenario.steps.push(direct('A', 'extra', 'commit', 16, {})); assert.match(JSON.stringify(await saveDelivery(workModel, workScenario, 422)), /work|operation|limit/i);
report('effect and expanded pure-work budgets are shared across machines rather than reset per owner');

const wideDataDefinition = clone(dataDefinition); wideDataDefinition.id += '-wide'; wideDataDefinition.records = [{ name: 'Wide', fields: Array.from({ length: 16 }, (_, i) => port(`field${i}`, 'string')) }];
const wideData = await post('/api/data/prepare', wideDataDefinition), wideRef = dataRef(wideData, 'Wide');
const wide = count => record(wideRef, Object.fromEntries(Array.from({ length: 16 }, (_, i) => [`field${i}`, str('x'.repeat(count))])));
const widePure = await prepareComponent(packageDefinition('wide-decision', { inputs: [port('value', 'record', wideRef)], outputs: [port('accepted', 'boolean'), port('value', 'record', wideRef)], nodes: [], bindings: { accepted: literal(bool(true)), value: input('value') } }, [wideData]));
const wideCommand = { name: 'emit', inputs: [], outputs: [port('value', 'record', wideRef)], decision: widePure, bindings: { value: { state: 'value' } }, acceptedOutput: 'accepted', writes: { value: 'value' }, results: { value: 'value' }, effects: [{ effect: 'Changed', payload: { value: 'value' } }] };
const wideEffects = [{ name: 'Changed', payload: [port('value', 'record', wideRef)] }];
const wideSource = await prepare(machineDefinition('wide-source', [slot('value', 'record', 'durable', wide(1000), wideRef)], [wideCommand], wideEffects));
const wideReceiveCommand = clone(wideCommand); wideReceiveCommand.name = 'accept'; wideReceiveCommand.inputs = [port('value', 'record', wideRef)]; wideReceiveCommand.bindings = { value: { input: 'value' } };
const wideTarget = await prepare(machineDefinition('wide-target', [slot('value', 'record', 'durable', wide(0), wideRef)], [wideReceiveCommand], wideEffects));
const wideModel = await prepareDelivery(network('wide-messages', [wideSource, wideTarget], [owner('A', wideSource), owner('B', wideTarget)], [route('wideRoute', 'A', 'Changed', 'B', 'accept', { value: 'value' })]));
const fiveInputs = clone(wideReceiveCommand); fiveInputs.inputs = Array.from({ length: 5 }, (_, i) => port(`value${i}`, 'record', wideRef)); fiveInputs.bindings = { value: { input: 'value0' } };
const amplifyingTarget = await prepare(machineDefinition('amplifying-target', [slot('value', 'record', 'durable', wide(0), wideRef)], [fiveInputs], wideEffects));
const amplifyingModel = await prepareDelivery(network('mapped-input-amplification', [wideSource, amplifyingTarget], [owner('A', wideSource), owner('B', amplifyingTarget)], [route('amplify', 'A', 'Changed', 'B', 'accept', Object.fromEntries(Array.from({ length: 5 }, (_, i) => [`value${i}`, 'value'])))]));
const amplification = experiment('One bounded payload cannot amplify beyond mapped-input capacity', [initial('A'), initial('B')], [direct('A', 'one', 'emit', 0, {}), dispatch('amplify', 'tooWide', 0, { sourceCommandId: 'one' })]);
assert.match(JSON.stringify(await saveDelivery(amplifyingModel, amplification, 422)), /mapped inputs|65536|64 KiB/i);
const evidenceSteps = [];
for (let i = 0; i < 16; i++) evidenceSteps.push(direct('A', `emit${i}`, 'emit', i, {}), dispatch('wideRoute', `wide${i}`, i, { sourceCommandId: `emit${i}` }), deliver(`wide${i}`));
const evidenceScenario = experiment('Bound total evidence including envelopes, histories and acknowledgements', [initial('A'), initial('B')], evidenceSteps);
assert.match(JSON.stringify(await saveDelivery(wideModel, evidenceScenario, 422)), /evidence|result|snapshot/i);
report('typed payload fan-out and complete delivery evidence are bounded before publication and leave no partial saved run');

const emptyDecision = (name, dataLocks = []) => packageDefinition(name, { inputs: [], outputs: [port('accepted', 'boolean')], nodes: [], bindings: { accepted: literal(bool(true)) } }, dataLocks);
const simpleCommand = (name, decision) => ({ name, inputs: [], outputs: [], decision, bindings: {}, acceptedOutput: 'accepted', writes: {}, results: {}, effects: [] });
const simpleMachine = (name, commands) => machineDefinition(name, [slot('value', 'integer', 'durable', int(0))], commands);
const closureMachines = [];
for (let ownerIndex = 0; ownerIndex < 2; ownerIndex++) {
  const commands = [];
  for (let group = 0; group < 4; group++) {
    let bundle = bundleOf(emptyDecision(`closure_${ownerIndex}_${group}_0`));
    for (let depth = 1; depth < 8; depth++) {
      const pkg = emptyDecision(`closure_${ownerIndex}_${group}_${depth}`); pkg.behavior.nodes = [call('child', bundle.root)]; pkg.behavior.bindings.accepted = source('child', 'accepted'); bundle = bundleOf(pkg, bundle.packages);
    }
    commands.push(simpleCommand(`handler${group}`, bundle));
  }
  closureMachines.push(await prepare(simpleMachine(`closure-owner-${ownerIndex}`, commands)));
}
const closureDefinition = network('sixty-four-global-locks', closureMachines, [owner('A', closureMachines[0])]);
await prepareDelivery(closureDefinition);
const spareMachine = await prepare(simpleMachine('spare-machine', [simpleCommand('commit', bundleOf(emptyDecision('one-extra-lock')))]));
closureDefinition.machines.push(spareMachine);
const closureFailure = await post('/api/delivery/prepare', closureDefinition, 422); assert.ok(closureFailure.issues.some(i => i.path === 'machines' && /64 distinct component/.test(i.message)));
const spareData = Array.from({ length: 33 }, (_, i) => { const contract = clone(dataDefinition); contract.id = `${prefix}.delivery_spare_${i}`; return { contract, digest: sha(orderedData(contract)) }; });
const dataUnionMachine = await prepare(simpleMachine('data-union-owner', Array.from({ length: 4 }, (_, i) => simpleCommand(`handler${i}`, bundleOf(emptyDecision(`data_union_${i}`, spareData.slice(i * 8, i * 8 + 8)))))));
const extraDataMachine = await prepare(simpleMachine('extra-data-owner', [simpleCommand('commit', bundleOf(emptyDecision('data_union_extra', [spareData[32]])))]));
const dataUnionModel = network('thirty-two-global-data-locks', [dataUnionMachine], [owner('A', dataUnionMachine)]); await prepareDelivery(dataUnionModel); dataUnionModel.machines.push(extraDataMachine);
const dataUnionFailure = await post('/api/delivery/prepare', dataUnionModel, 422); assert.ok(dataUnionFailure.issues.some(i => i.path === 'machines' && /32 distinct data/.test(i.message)));
const fourMachines = [sourceMachine, targetMachine, spareMachine, structuredMachine]; const fourDefinition = network('four-machines', fourMachines, [owner('A', sourceMachine)]); await prepareDelivery(fourDefinition); fourDefinition.machines.push(otherStructuredMachine); await post('/api/delivery/prepare', fourDefinition, 422);
report('global dependency unions count independently valid unused machine locks and permit exact reuse at the advertised boundaries');

const boundaryDefinition = network('scenario-size', [wideTarget], Array.from({ length: 8 }, (_, i) => owner(`Owner${i}`, wideTarget)));
const boundaryModel = await prepareDelivery(boundaryDefinition);
const boundaryInitial = boundaryDefinition.instances.map(i => initial(i.id, { value: wide(3900) }));
const boundaryScenario = experiment('Individually bounded maps cannot exceed total scenario capacity', boundaryInitial, [restart('Owner0')], [expectation('Owner0', { value: wide(3900) }, 0, 0, 0)]);
const boundaryFailure = await checkDelivery(boundaryModel, boundaryScenario, 422); assert.match(JSON.stringify(boundaryFailure), /512 KiB|524288|canonical JSON/i);
const largeTestString = str('t'.repeat(4096));
const testedPackage = packageDefinition('large-test-decision', { inputs: [port('value', 'string')], outputs: [port('accepted', 'boolean'), port('value', 'string')], nodes: [], bindings: { accepted: literal(bool(true)), value: input('value') } }, [], Array.from({ length: 8 }, (_, i) => ({ name: `case${i}`, inputs: { value: largeTestString }, expected: { accepted: bool(true), value: largeTestString } })));
const testedDecision = await prepareComponent(testedPackage);
const testedCommand = { ...simpleCommand('commit', testedDecision), bindings: { value: literal(str('')) } };
const largeMachines = [];
for (let i = 0; i < 4; i++) largeMachines.push(await prepare(simpleMachine(`model-byte-owner-${i}`, Array.from({ length: 4 }, (_, j) => ({ ...clone(testedCommand), name: `command${j}` })))));
const largeModel = network('model-byte-budget', largeMachines, [owner('A', largeMachines[0])]);
assert.ok(Buffer.byteLength(typedJSON(orderedNetwork(largeModel))) > 1024 * 1024);
assert.match(JSON.stringify(await post('/api/delivery/prepare', largeModel, 422)), /1 MiB|1048576/i);
await request('/api/delivery/prepare', { method: 'POST', raw: ' '.repeat(2 * 1024 * 1024 + 1), expected: 413 });
report('canonical model and scenario byte limits are distinct from individual typed values and the outer HTTP body cap');

const beforeCatalogs = await Promise.all(['/api/state/machines', '/api/components', '/api/data/contracts'].map(path => get(path)));
const installResults = await Promise.all([request('/api/delivery/models', { method: 'POST', body: model, expected: [200, 201] }), request('/api/delivery/models', { method: 'POST', body: model, expected: [200, 201] })]);
assert.deepEqual(installResults.map(r => r.status).sort(), [200, 201]); assert.deepEqual(installResults[0].data, installResults[1].data);
const installed = installResults[0].data; assert.deepEqual(installed.model, model.model); assert.equal(installed.digest, model.digest); assert.match(installed.installedAt, /^\d{4}-/);
assert.deepEqual(await post('/api/delivery/models', model), installed);
const exactPath = lock => `/api/delivery/model?id=${encodeURIComponent(lock.model.id)}&version=${encodeURIComponent(lock.model.version)}&digest=${lock.digest}`;
assert.deepEqual(await get(exactPath(model)), installed); await get(exactPath({ ...model, digest: 'f'.repeat(64) }), 404); await get('/api/delivery/model', 422);
const modelConflictDefinition = clone(modelDefinition); modelConflictDefinition.description += ' conflicting'; const modelConflict = await prepareDelivery(modelConflictDefinition); await post('/api/delivery/models', modelConflict, 409);
const secondVersionDefinition = clone(modelDefinition); secondVersionDefinition.version = '1.0.1'; secondVersionDefinition.instances[0].position.x = 20; const secondVersion = await prepareDelivery(secondVersionDefinition); await post('/api/delivery/models', secondVersion, 201);
const catalog = await get('/api/delivery/models'); const catalogKeys = catalog.map(r => `${r.model.id}\u0000${r.model.version}`); assert.deepEqual(catalogKeys, [...catalogKeys].sort()); assert.equal(catalog.filter(r => r.model.id === model.model.id).length, 2);
assert.deepEqual(await Promise.all(['/api/state/machines', '/api/components', '/api/data/contracts'].map(path => get(path))), beforeCatalogs);
await request('/api/delivery/models', { method: 'DELETE', expected: 405 });
report('immutable delivery catalogs are atomic, exact, sorted and independent of state, pure-component and data catalogs');

const beforeHistory = await get('/api/delivery/runs');
await checkDelivery(model, primary); await saveDelivery(capacityModel, overCapacity, 422); await saveDelivery(amplifyingModel, amplification, 422); await saveDelivery(wideModel, evidenceScenario, 422);
assert.deepEqual(await get('/api/delivery/runs'), beforeHistory);
const saved = await saveDelivery(model, primary), savedAgain = await saveDelivery(model, primary);
assert.notEqual(saved.id, savedAgain.id); assert.deepEqual(saved.result, result); assert.deepEqual(savedAgain.result, result); assert.deepEqual(saved.model, model); assert.deepEqual(saved.scenario, primary);
assert.deepEqual(await get(`/api/delivery/runs/${saved.id}`), saved); assert.deepEqual(await get(`/api/delivery/runs/${failed.id}`), failed);
const afterHistory = await get('/api/delivery/runs');
assert.deepEqual(afterHistory.slice(0, 2).map(r => r.id), [savedAgain.id, saved.id]);
assert.deepEqual(afterHistory[0], { id: savedAgain.id, createdAt: savedAgain.createdAt, modelId: model.model.id, modelVersion: model.model.version, stepCount: 15, messageCount: 1, verified: true, passed: true });
assert.equal(afterHistory.find(r => r.id === failed.id).passed, false);
const later = await saveDelivery(secondVersion, primary); assert.equal(later.result.modelHash, secondVersion.digest); assert.deepEqual(later.result.instances, result.instances); assert.deepEqual(later.result.messages, result.messages);
assert.deepEqual(await get(`/api/delivery/runs/${saved.id}`), saved);
await get('/api/delivery/runs/not-present', 404); await request(`/api/delivery/runs/${saved.id}`, { method: 'DELETE', expected: 405 });
await request('/api/delivery/runs', { method: 'DELETE', expected: 405 });
const examples = await get('/api/delivery/examples'); assert.equal(examples.length, 2);
for (const example of examples) { assert.equal((await post('/api/delivery/validate-lock', example.model)).valid, true); assert.equal((await checkDelivery(example.model, example.scenario)).result.passed, true); }
assert.deepEqual((await check(sourceMachine, scenario('Legacy engine remains isolated', [instance('S')], [direct('S', 'msg_direct', 'emit', 0, sendInputs(2))]))).result.instances, oldState.result.instances);
report('complete evidence survives later catalog versions; failed expectations persist, previews and aborted runs do not, and new submissions receive distinct IDs');

console.log(`${checks} delivery acceptance groups passed against ${base}`);
