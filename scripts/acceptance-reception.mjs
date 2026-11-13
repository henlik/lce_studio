#!/usr/bin/env node
// Independent workflow and reception protocol references. Use a disposable database:
// catalog entries and complete experiment evidence are intentionally immutable.
import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';

const base = (process.env.STUDIO_URL || 'http://127.0.0.1:8080').replace(/\/$/, '');
const suffix = randomUUID();
const prefix = `acceptance.reception.${suffix}`;
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

const prepareWorkflow = workflow => post('/api/workflow/prepare', workflow);
const checkWorkflow = (workflow, scenario, expected = 200) => post('/api/workflow/check', { workflow, scenario }, expected);
const saveWorkflow = (workflow, scenario, expected = 201) => post('/api/workflow/runs', { workflow, scenario }, expected);
const variable = name => ({ variable: name });
const workflowVariable = (name, type, initial, recordRef) => ({ ...port(name, type, recordRef), initial });
const workflowDefinition = (name, options = {}) => ({ schemaVersion: 1, id: `${prefix}.${name}`, version: '1.0.0', name, description: 'Independent bounded workflow reference with explicit synthetic human and adapter inputs.', author: 'Acceptance suite', license: 'MIT', dataContracts: [], decisions: [], machines: [], owners: [], inputs: [], outputs: [], variables: [], entry: '', nodes: [], signals: [], ...options });
const workflowNode = (id, kind, options = {}) => ({ id, name: id, kind, position: { x: 0, y: 0 }, ...options });
const eventResponse = (id, taskId, values) => ({ id, kind: 'response', taskId, values });
const eventSignal = (id, signal, values = {}) => ({ id, kind: 'signal', signal, values });
const workflowScenario = (name, inputs, events = [], expected) => ({ schemaVersion: 1, name, inputs, events, ...(expected ? { expected } : {}) });
const workflowExpectation = (status, outcome, activeTaskNode, variables, tasks, completedTasks, cancelledTasks, eventReceipts, owners = []) => ({ status, outcome, activeTaskNode, variables, owners, tasks, completedTasks, cancelledTasks, eventReceipts });
const orderedWorkflowBinding = b => ({ ...(b.input ? { input: b.input } : {}), ...(b.variable ? { variable: b.variable } : {}), ...(b.literal ? { literal: orderedValue(b.literal) } : {}) });
const orderedWorkflowExpectation = e => ({ status: e.status, outcome: e.outcome, activeTaskNode: e.activeTaskNode, variables: map(e.variables, orderedValue), owners: e.owners.map(v => ({ instanceId: v.instanceId, state: map(v.state, orderedValue), revision: v.revision, receipts: v.receipts, effects: v.effects })), tasks: e.tasks, completedTasks: e.completedTasks, cancelledTasks: e.cancelledTasks, eventReceipts: e.eventReceipts });
function orderedWorkflow(w) {
  return { schemaVersion: w.schemaVersion, id: w.id, version: w.version, name: w.name, description: w.description, author: w.author, license: w.license,
    dataContracts: w.dataContracts.map(orderedDataLock), decisions: w.decisions.map(orderedBundle), machines: w.machines.map(l => ({ machine: orderedMachine(l.machine), digest: l.digest })),
    owners: w.owners.map(o => ({ id: o.id, name: o.name, machine: { machineId: o.machine.machineId, version: o.machine.version, digest: o.machine.digest }, initial: map(o.initial, orderedWorkflowBinding) })),
    inputs: w.inputs.map(orderedPort), outputs: w.outputs.map(orderedPort), variables: w.variables.map(v => ({ ...orderedPort(v), initial: orderedValue(v.initial) })), entry: w.entry,
    nodes: w.nodes.map(n => ({ id: n.id, name: n.name, kind: n.kind, position: { x: n.position.x, y: n.position.y },
      ...(n.task ? { task: { kind: n.task.kind, title: n.task.title, instructions: n.task.instructions, request: map(n.task.request, orderedWorkflowBinding), response: n.task.response.map(orderedPort), assign: map(n.task.assign) } } : {}),
      ...(n.decision ? { decision: { component: orderedRef(n.decision.component), inputs: map(n.decision.inputs, orderedWorkflowBinding), assign: map(n.decision.assign) } } : {}),
      ...(n.command ? { command: { owner: n.command.owner, command: n.command.command, inputs: map(n.command.inputs, orderedWorkflowBinding), assign: map(n.command.assign), acceptedVariable: n.command.acceptedVariable, errorNext: n.command.errorNext } } : {}),
      ...(n.end ? { end: { outcome: n.end.outcome, outputs: map(n.end.outputs, orderedWorkflowBinding) } } : {}),
      ...(n.next ? { next: n.next } : {}), ...(n.branch ? { branch: { variable: n.branch.variable, then: n.branch.then, else: n.branch.else } } : {}),
    })),
    signals: w.signals.map(s => ({ name: s.name, inputs: s.inputs.map(orderedPort), ...(s.when ? { when: s.when } : {}), set: map(s.set, orderedWorkflowBinding), target: s.target })),
  };
}
function orderedWorkflowScenario(s) {
  return { schemaVersion: s.schemaVersion, name: s.name, inputs: map(s.inputs, orderedValue), events: s.events.map(e => ({ id: e.id, kind: e.kind, ...(e.taskId ? { taskId: e.taskId } : {}), ...(e.signal ? { signal: e.signal } : {}), values: map(e.values, orderedValue) })), ...(s.expected ? { expected: orderedWorkflowExpectation(s.expected) } : {}) };
}

assert.equal((await get('/api/health')).version, '0.8.1');
const gatePackage = packageDefinition('approval-gate', { inputs: [port('approved', 'boolean'), port('authorized', 'boolean')], outputs: [port('permitted', 'boolean')], nodes: [primitive('gate', 'and', { left: input('approved'), right: input('authorized') })], bindings: { permitted: source('gate') } });
const gate = await prepareComponent(gatePackage);
const reservationPackage = packageDefinition('bounded-reservation', {
  inputs: [port('available', 'integer'), port('amount', 'integer')], outputs: [port('accepted', 'boolean'), port('next', 'integer'), port('remaining', 'integer')],
  nodes: [primitive('enough', 'less_or_equal', { left: input('amount'), right: input('available') }), primitive('positive', 'less_than', { left: literal(int(0)), right: input('amount') }), primitive('permit', 'and', { left: source('enough'), right: source('positive') }), primitive('subtract', 'subtract', { left: input('available'), right: input('amount') }), primitive('remaining', 'select', { condition: source('permit'), trueValue: source('subtract'), falseValue: input('available') })],
  bindings: { accepted: source('permit'), next: source('subtract'), remaining: source('remaining') },
});
const reservation = await prepareComponent(reservationPackage);
const capacityDefinition = machineDefinition('capacity-owner', [slot('available', 'integer', 'durable', int(0))], [{ name: 'reserve', inputs: [port('amount', 'integer')], outputs: [port('remaining', 'integer')], decision: reservation, bindings: { available: { state: 'available' }, amount: { input: 'amount' } }, acceptedOutput: 'accepted', writes: { available: 'next' }, results: { remaining: 'remaining' }, effects: [] }]);
const capacity = await prepare(capacityDefinition);
const outputBindings = { requestId: input('requestId'), remaining: variable('remaining'), handoffAccepted: variable('handoffAccepted'), emergency: variable('emergency') };
const terminal = (id, outcome) => workflowNode(id, 'end', { end: { outcome, outputs: clone(outputBindings) } });
const taskNode = (id, response, assign, options = {}) => workflowNode(id, 'task', { task: { kind: 'human', title: `Explicit ${id}`, instructions: 'Supply an explicit synthetic result; the engine performs no human work.', request: { requestId: input('requestId'), actor: variable('actor') }, response, assign }, ...options });
const definition = workflowDefinition('generic-fulfilment', {
  decisions: [gate], machines: [capacity], owners: [{ id: 'inventory', name: 'Independent inventory owner', machine: { machineId: capacity.machine.id, version: capacity.machine.version, digest: capacity.digest }, initial: { available: input('capacity') } }],
  inputs: [port('requestId', 'string'), port('amount', 'integer'), port('capacity', 'integer'), port('authorized', 'boolean')],
  outputs: [port('requestId', 'string'), port('remaining', 'integer'), port('handoffAccepted', 'boolean'), port('emergency', 'boolean')],
  variables: [workflowVariable('approved', 'boolean', bool(false)), workflowVariable('permitted', 'boolean', bool(false)), workflowVariable('reserved', 'boolean', bool(false)), workflowVariable('remaining', 'integer', int(0)), workflowVariable('handoffAccepted', 'boolean', bool(false)), workflowVariable('urgentAccepted', 'boolean', bool(false)), workflowVariable('emergency', 'boolean', bool(false)), workflowVariable('canInterrupt', 'boolean', bool(true)), workflowVariable('actor', 'string', str(''))],
  entry: 'review',
  nodes: [
    taskNode('review', [port('approved', 'boolean'), port('actor', 'string')], { approved: 'approved', actor: 'actor' }, { next: 'gate' }),
    workflowNode('gate', 'decision', { decision: { component: gate.root, inputs: { approved: variable('approved'), authorized: input('authorized') }, assign: { permitted: 'permitted' } }, branch: { variable: 'permitted', then: 'reserve', else: 'rejected' } }),
    workflowNode('reserve', 'command', { command: { owner: 'inventory', command: 'reserve', inputs: { amount: input('amount') }, assign: { remaining: 'remaining' }, acceptedVariable: 'reserved', errorNext: 'commandError' }, branch: { variable: 'reserved', then: 'handoff', else: 'reservationDenied' } }),
    taskNode('handoff', [port('accepted', 'boolean')], { handoffAccepted: 'accepted' }, { branch: { variable: 'handoffAccepted', then: 'accepted', else: 'declined' } }),
    taskNode('urgent', [port('accepted', 'boolean')], { urgentAccepted: 'accepted' }, { branch: { variable: 'urgentAccepted', then: 'urgentAccepted', else: 'urgent' } }),
    terminal('accepted', 'handoffAccepted'), terminal('rejected', 'approvalDenied'), terminal('reservationDenied', 'reservationDenied'), terminal('declined', 'handoffDeclined'), terminal('urgentAccepted', 'urgentHandoffAccepted'), terminal('cancelled', 'cancelled'), terminal('commandError', 'commandError'),
  ],
  signals: [{ name: 'urgent', inputs: [port('actor', 'string')], when: 'canInterrupt', set: { emergency: literal(bool(true)), canInterrupt: literal(bool(false)), actor: input('actor') }, target: 'urgent' }, { name: 'cancel', inputs: [], when: 'canInterrupt', set: { canInterrupt: literal(bool(false)) }, target: 'cancelled' }],
});
const workflow = await prepareWorkflow(definition);
assert.equal(workflow.digest, sha(orderedWorkflow(definition))); assert.deepEqual(workflow.workflow, definition);
const rootInputs = (amount = 2, capacity = 5, authorized = true, requestId = 'order1') => ({ requestId: str(requestId), amount: int(amount), capacity: int(capacity), authorized: bool(authorized) });
const approval = (id = 'approve', taskId = 'task1', approved = true) => eventResponse(id, taskId, { approved: bool(approved), actor: str('reviewer1') });
const handoff = (id = 'accept', taskId = 'task2', accepted = true) => eventResponse(id, taskId, { accepted: bool(accepted) });
const inventoryExpectation = (available, revision, receipts) => expectation('inventory', { available: int(available) }, revision, receipts, 0);
const normal = workflowScenario('Independent approval then accepted handoff', rootInputs(), [approval(), handoff()], workflowExpectation('completed', 'handoffAccepted', '', { approved: bool(true), permitted: bool(true), reserved: bool(true), remaining: int(3), handoffAccepted: bool(true), emergency: bool(false), actor: str('reviewer1') }, 2, 2, 0, 2, [inventoryExpectation(3, 1, 1)]));
const normalCheck = await checkWorkflow(workflow, normal), normalResult = normalCheck.result;
assert.deepEqual(normalCheck.scenario, normal); assert.equal(normalResult.workflowHash, workflow.digest); assert.equal(normalResult.scenarioHash, sha(orderedWorkflowScenario(normal))); assert.equal(normalResult.engineVersion, '0.8.0'); assert.equal(normalResult.status, 'completed'); assert.equal(normalResult.outcome, 'handoffAccepted'); assert.equal(normalResult.verified, true); assert.equal(normalResult.passed, true);
assert.equal(normalResult.operations, 6); assert.equal(normalResult.invocations, 2); assert.equal(normalResult.task, undefined);
assert.deepEqual(normalResult.tasks.map(t => [t.id, t.nodeId, t.status, t.completedBy]), [['task1', 'review', 'completed', 'approve'], ['task2', 'handoff', 'completed', 'accept']]);
assert.deepEqual(normalResult.outputs, { requestId: str('order1'), remaining: int(3), handoffAccepted: bool(true), emergency: bool(false) });
assert.equal(normalResult.owners[0].revision, 1); assert.deepEqual(normalResult.owners[0].state, { available: int(3) }); assert.equal(normalResult.owners[0].receipts.length, 1); assert.equal(normalResult.owners[0].receipts[0].commandId, 'wf_command1');
assert.equal(normalResult.steps.find(s => s.kind === 'command').state.decision.engineVersion, '0.5.0'); assert.equal(normalResult.steps.find(s => s.kind === 'command').state.index, 0);
assert.deepEqual((await checkWorkflow(workflow, normal)).result, normalResult);
report('generic workflow composes exact pure and owned-state semantics with independently computed task, capacity, work and handoff outcomes');

const untouched = workflowScenario('No event means a pending human task', rootInputs(), [], workflowExpectation('waiting', '', 'review', { approved: bool(false), reserved: bool(false) }, 1, 0, 0, 0, [inventoryExpectation(5, 0, 0)]));
const untouchedResult = (await checkWorkflow(workflow, untouched)).result; assert.equal(untouchedResult.passed, true); assert.equal(untouchedResult.task.id, 'task1'); assert.equal(untouchedResult.tasks[0].status, 'pending'); assert.deepEqual(untouchedResult.outputs, {}); assert.equal(untouchedResult.operations, 0);
const pendingHandoff = workflowScenario('A reservation does not complete a receiving handoff', rootInputs(), [approval()], workflowExpectation('waiting', '', 'handoff', { reserved: bool(true), handoffAccepted: bool(false), remaining: int(3) }, 2, 1, 0, 1, [inventoryExpectation(3, 1, 1)]));
const pendingResult = (await checkWorkflow(workflow, pendingHandoff)).result; assert.equal(pendingResult.passed, true); assert.deepEqual(pendingResult.outputs, {}); assert.equal(pendingResult.task.nodeId, 'handoff');
const unverified = clone(untouched); delete unverified.expected; const unverifiedResult = (await checkWorkflow(workflow, unverified)).result; assert.equal(unverifiedResult.verified, false); assert.equal(unverifiedResult.passed, false);
const denied = workflowScenario('A denied capacity reservation has no handoff task', rootInputs(2, 1), [approval()], workflowExpectation('completed', 'reservationDenied', '', { reserved: bool(false), remaining: int(1), handoffAccepted: bool(false) }, 1, 1, 0, 1, [inventoryExpectation(1, 0, 1)]));
const deniedResult = (await checkWorkflow(workflow, denied)).result; assert.equal(deniedResult.passed, true); assert.equal(deniedResult.owners[0].receipts[0].accepted, false); assert.equal(deniedResult.tasks.length, 1);
const unauthorized = workflowScenario('Explicit authorization facts gate ordinary processing', rootInputs(2, 5, false), [approval()], workflowExpectation('completed', 'approvalDenied', '', { permitted: bool(false), reserved: bool(false) }, 1, 1, 0, 1, [inventoryExpectation(5, 0, 0)])); assert.equal((await checkWorkflow(workflow, unauthorized)).result.passed, true);
report('incomplete tasks remain waiting, unasserted runs remain unverified, and reservation or authority denial never invents a receiving acceptance');

const commandFailureScenario = workflowScenario('Ordinary owned reducer error follows explicit error edge', rootInputs(-Number.MAX_SAFE_INTEGER), [approval()], workflowExpectation('completed', 'commandError', '', { reserved: bool(false), remaining: int(0) }, 1, 1, 0, 1, [inventoryExpectation(5, 0, 0)]));
const commandFailure = (await checkWorkflow(workflow, commandFailureScenario)).result;
assert.equal(commandFailure.passed, true); assert.equal(commandFailure.steps.find(s => s.kind === 'command').state.status, 'evaluation_error'); assert.equal(commandFailure.operations, 1); assert.equal(commandFailure.invocations, 1);
const failingPureDefinition = workflowDefinition('ordinary-pure-failure', { decisions: [reservation], entry: 'overflow', nodes: [workflowNode('overflow', 'decision', { decision: { component: reservation.root, inputs: { available: literal(int(Number.MAX_SAFE_INTEGER)), amount: literal(int(-1)) }, assign: {} }, next: 'done' }), workflowNode('done', 'end', { end: { outcome: 'unreachable', outputs: {} } })], signals: [{ name: 'later', inputs: [], set: {}, target: 'done' }] });
const failingPure = await prepareWorkflow(failingPureDefinition), failingPureScenario = workflowScenario('Ordinary pure error remains terminal under later signals', {}, [eventSignal('late', 'later'), eventSignal('late', 'later')], workflowExpectation('failed', '', '', {}, 0, 0, 0, 1));
const pureFailure = (await checkWorkflow(failingPure, failingPureScenario)).result;
assert.equal(pureFailure.passed, true); assert.equal(pureFailure.steps.length, 1); assert.equal(pureFailure.steps[0].status, 'failed'); assert.deepEqual(pureFailure.events.map(e => e.status), ['rejected', 'replayed']); assert.deepEqual(pureFailure.outputs, {});
report('ordinary reducer failures preserve owner state and explicit error routing; terminal pure failures reject new signals and replay receipts');

const eventHash = event => sha({ kind: event.kind, taskId: event.taskId || '', signal: event.signal || '', values: map(event.values, orderedValue) });
const duplicateEvents = [approval(), approval(), approval('approve', 'task1', false), approval('stale'), eventResponse('wrongPorts', 'task2', { accepted: str('true') }), handoff(), handoff('accept'), eventSignal('afterTerminal', 'urgent', { actor: str('dispatcher') }), eventSignal('afterTerminal', 'urgent', { actor: str('dispatcher') })];
const duplicateScenario = workflowScenario('Stable event receipts and stale task generations', rootInputs(), duplicateEvents, workflowExpectation('completed', 'handoffAccepted', '', { remaining: int(3), handoffAccepted: bool(true) }, 2, 2, 0, 5, [inventoryExpectation(3, 1, 1)]));
const duplicateResult = (await checkWorkflow(workflow, duplicateScenario)).result;
assert.equal(duplicateResult.passed, true); assert.deepEqual(duplicateResult.events.map(e => e.status), ['accepted', 'replayed', 'id_conflict', 'rejected', 'rejected', 'accepted', 'replayed', 'rejected', 'replayed']); assert.equal(duplicateResult.operations, 6); assert.equal(duplicateResult.invocations, 2);
assert.deepEqual(duplicateResult.owners, normalResult.owners); assert.deepEqual(duplicateResult.tasks, normalResult.tasks); assert.deepEqual(duplicateResult.receipts.map(r => r.id), ['approve', 'stale', 'wrongPorts', 'accept', 'afterTerminal']);
assert.equal(duplicateResult.receipts[0].requestHash, eventHash(approval())); assert.equal(duplicateResult.receipts[0].nextTaskId, 'task2'); assert.deepEqual(duplicateResult.events[1].receipt, duplicateResult.receipts[0]); assert.equal(duplicateResult.events[2].receipt, undefined); assert.equal(duplicateResult.events[8].receipt.status, 'rejected');
assert.equal(duplicateResult.events[1].beforeTaskId, 'task2'); assert.equal(duplicateResult.events[1].afterTaskId, 'task2'); assert.equal(duplicateResult.events[6].afterTaskId, ''); assert.equal(duplicateResult.tasks[1].request.actor.string, 'reviewer1'); assert.equal(duplicateResult.tasks[0].request.actor.string, '');
report('same-ID event replay preserves historical receipts, conflicting payloads do not rerun work, and stale or malformed task responses cannot advance the workflow');

for (const beforeHandoff of [false, true]) {
  const events = beforeHandoff ? [approval()] : [];
  const cancelledTask = beforeHandoff ? 'task2' : 'task1', urgentTask = beforeHandoff ? 'task3' : 'task2';
  events.push(eventSignal('escalate', 'urgent', { actor: str('emergencyReceiver') }), eventResponse('lateRoutine', cancelledTask, beforeHandoff ? { accepted: bool(true) } : { approved: bool(true), actor: str('lateReviewer') }), eventSignal('repeatUrgent', 'urgent', { actor: str('anotherActor') }), eventResponse('urgentAccept', urgentTask, { accepted: bool(true) }), eventSignal('escalate', 'urgent', { actor: str('emergencyReceiver') }));
  const e = workflowExpectation('completed', 'urgentHandoffAccepted', '', { emergency: bool(true), urgentAccepted: bool(true), handoffAccepted: bool(false), actor: str('emergencyReceiver'), remaining: int(beforeHandoff ? 3 : 0) }, beforeHandoff ? 3 : 2, beforeHandoff ? 2 : 1, 1, beforeHandoff ? 5 : 4, [inventoryExpectation(beforeHandoff ? 3 : 5, beforeHandoff ? 1 : 0, beforeHandoff ? 1 : 0)]);
  const interrupted = (await checkWorkflow(workflow, workflowScenario(`Interrupt ${beforeHandoff ? 'handoff' : 'initial review'} without administrative prerequisites`, rootInputs(), events, e))).result;
  assert.equal(interrupted.passed, true); assert.equal(interrupted.tasks.find(t => t.id === cancelledTask).status, 'cancelled'); assert.equal(interrupted.events.at(-1).status, 'replayed'); assert.equal(interrupted.events.at(-1).receipt.nextTaskId, urgentTask); assert.equal(interrupted.events.at(-1).afterTaskId, ''); assert.equal(interrupted.tasks.find(t => t.id === urgentTask).request.actor.string, 'emergencyReceiver');
}
const urgentRetry = workflowScenario('An unavailable receiver requests another explicit task', rootInputs(), [eventSignal('escalate', 'urgent', { actor: str('receiver') }), handoff('unavailable', 'task2', false)], workflowExpectation('waiting', '', 'urgent', { emergency: bool(true), urgentAccepted: bool(false) }, 3, 1, 1, 2, [inventoryExpectation(5, 0, 0)]));
const urgentRetryResult = (await checkWorkflow(workflow, urgentRetry)).result; assert.equal(urgentRetryResult.passed, true); assert.equal(urgentRetryResult.task.id, 'task3'); assert.equal(urgentRetryResult.tasks[1].status, 'completed'); assert.equal(urgentRetryResult.tasks[2].status, 'pending');
const cancelled = workflowScenario('Cancellation fences pending routine work', rootInputs(), [eventSignal('cancel', 'cancel'), approval('late')], workflowExpectation('completed', 'cancelled', '', { approved: bool(false), reserved: bool(false) }, 1, 0, 1, 2, [inventoryExpectation(5, 0, 0)])); assert.equal((await checkWorkflow(workflow, cancelled)).result.passed, true);
report('interruptions cancel the exact active task, fence late routine responses and keep unavailable urgent handoffs pending without automatic success');

const invalidDefinitions = [
  w => { w.schemaVersion = 2; }, w => { w.entry = 'missing'; }, w => { w.nodes[0].id = w.nodes[1].id; }, w => { w.nodes[0].kind = 'executeScript'; },
  w => { w.nodes[0].task.kind = 'automaticHuman'; }, w => { w.nodes[0].task.request = null; }, w => { w.nodes[0].task.assign.approved = 'missingResponse'; },
  w => { w.nodes[0].task.response[0].type = 'string'; }, w => { w.nodes[0].next = 'missing'; }, w => { w.nodes[0].branch = { variable: 'approved', then: 'accepted', else: 'rejected' }; },
  w => { w.nodes[1].decision.component.digest = '0'.repeat(64); }, w => { w.nodes[1].decision.inputs.approved = literal(str('true')); }, w => { w.nodes[1].branch.variable = 'actor'; },
  w => { w.nodes[2].command.owner = 'missingOwner'; }, w => { w.nodes[2].command.command = 'missingCommand'; }, w => { w.nodes[2].command.acceptedVariable = 'actor'; }, w => { w.nodes[2].command.errorNext = 'missing'; },
  w => { w.nodes[2].command.inputs.amount = variable('approved'); }, w => { w.nodes[5].end.outputs.remaining = variable('approved'); }, w => { w.nodes[5].next = 'review'; }, w => { w.nodes[5].end.outputs = null; },
  w => { w.signals[0].when = 'actor'; }, w => { w.signals[0].target = 'missing'; }, w => { w.signals[0].set.actor = input('undeclaredSignalInput'); }, w => { w.signals[0].set.emergency = literal(str('true')); },
  w => { w.owners[0].initial.available = input('requestId'); }, w => { w.owners[0].machine.digest = 'f'.repeat(64); }, w => { w.variables[0].initial = int(0); }, w => { w.variables.push(clone(w.variables[0])); },
  w => { w.decisions = []; }, w => { w.machines = []; }, w => { w.dataContracts = null; }, w => { w.nodes = []; }, w => { w.signals = null; },
];
for (const mutate of invalidDefinitions) { const invalid = clone(definition); mutate(invalid); await post('/api/workflow/prepare', invalid, 422); }
const canonical = await post('/api/workflow/validate-lock', workflow); assert.equal(canonical.valid, true); assert.deepEqual(canonical.workflow, workflow);
const tampered = clone(workflow); tampered.workflow.description += ' changed'; const rejectedLock = await post('/api/workflow/validate-lock', tampered); expectRejected(rejectedLock); assert.equal(rejectedLock.workflow, undefined);
const changedLayout = clone(definition); changedLayout.nodes[0].position.x = 220; const changedLock = await prepareWorkflow(changedLayout); assert.notEqual(changedLock.digest, workflow.digest);
const changedResult = (await checkWorkflow(changedLock, normal)).result; assert.deepEqual(changedResult.owners, normalResult.owners); assert.deepEqual(changedResult.tasks, normalResult.tasks); assert.deepEqual(changedResult.outputs, normalResult.outputs);
const escaped = clone(definition); escaped.description = 'A < B & C > D\u2028workflow\u2029response'; assert.equal((await prepareWorkflow(escaped)).digest, sha(orderedWorkflow(escaped)));
report('portable definitions reject unresolved or mistyped task, branch, decision, command and signal contracts while exact hashes include layout and metadata');

const invalidScenarios = [
  s => { s.schemaVersion = 2; }, s => { s.inputs = null; }, s => { delete s.inputs.amount; }, s => { s.inputs.amount = str('2'); }, s => { s.inputs.extra = bool(false); },
  s => { s.events = null; }, s => { s.events[0].id = 'bad id'; }, s => { s.events[0].kind = 'automatic'; }, s => { s.events[0].values = null; },
  s => { s.expected.variables = null; }, s => { s.expected.variables.unknown = int(0); }, s => { s.expected.variables.remaining = str('3'); }, s => { s.expected.tasks = -1; }, s => { s.expected.eventReceipts = Number.MAX_SAFE_INTEGER + 1; },
];
for (const mutate of invalidScenarios) { const invalid = clone(normal); mutate(invalid); await checkWorkflow(workflow, invalid, 422); }
for (const key of ['tasks', 'completedTasks', 'cancelledTasks', 'eventReceipts']) {
  const missing = clone(normal); delete missing.expected[key]; await checkWorkflow(workflow, missing, 400);
  const nulled = clone(normal); nulled.expected[key] = null; await checkWorkflow(workflow, nulled, 400);
}
await request('/api/workflow/prepare', { method: 'POST', raw: JSON.stringify({ ...definition, externalScript: 'unsupported' }), expected: 400 });
await request('/api/workflow/prepare', { method: 'POST', raw: `${JSON.stringify(definition)} {}`, expected: 400 });
await request('/api/workflow/check', { method: 'POST', raw: JSON.stringify({ workflow, scenario: normal }).replace('"integer":2', '"integer":2.0000000000000001'), expected: 400 });
await request('/api/workflow/check', { method: 'POST', raw: JSON.stringify({ workflow, scenario: normal }).replace('"completedTasks":2', '"completedTasks":2.5'), expected: 400 });
const caseNormalized = await request('/api/workflow/validate-lock', { method: 'POST', raw: JSON.stringify(workflow).replace('"workflow":', '"Workflow":').replace('"nodes":', '"Nodes":') }); assert.equal(caseNormalized.data.valid, true); assert.deepEqual(caseNormalized.data.workflow, workflow);
const noExpectations = clone(normal); delete noExpectations.expected; const rawZero = JSON.stringify({ workflow, scenario: untouched }).replace('"completedTasks":0', '"CompletedTasks":0'); const normalizedScenario = (await request('/api/workflow/check', { method: 'POST', raw: rawZero })).data; assert.equal(normalizedScenario.result.passed, true); assert.equal(normalizedScenario.scenario.expected.completedTasks, 0);
report('raw imports preserve exact integer and explicit-zero semantics, normalize validated casing, and reject unknown fields or trailing input before execution');

const futureTask = workflowScenario('An early rejection is not rewritten when its task later exists', rootInputs(), [handoff('early', 'task2'), approval(), handoff('early', 'task2'), handoff('newRequest', 'task2')], workflowExpectation('completed', 'handoffAccepted', '', {}, 2, 2, 0, 3, [inventoryExpectation(3, 1, 1)]));
const futureTaskResult = (await checkWorkflow(workflow, futureTask)).result; assert.equal(futureTaskResult.passed, true); assert.deepEqual(futureTaskResult.events.map(e => e.status), ['rejected', 'accepted', 'replayed', 'accepted']); assert.equal(futureTaskResult.events[2].receipt.status, 'rejected'); assert.equal(futureTaskResult.tasks[1].completedBy, 'newRequest');
const invalidSignals = workflowScenario('Unsupported signals and malformed typed responses are recorded refusals', rootInputs(), [eventSignal('unknown', 'notDeclared'), eventSignal('badType', 'urgent', { actor: int(7) }), eventSignal('badType', 'urgent', { actor: str('fixed') }), eventSignal('extra', 'urgent', { actor: str('user'), extra: bool(true) })], workflowExpectation('waiting', '', 'review', { emergency: bool(false), actor: str('') }, 1, 0, 0, 3, [inventoryExpectation(5, 0, 0)]));
const invalidSignalResult = (await checkWorkflow(workflow, invalidSignals)).result; assert.equal(invalidSignalResult.passed, true); assert.deepEqual(invalidSignalResult.events.map(e => e.status), ['rejected', 'rejected', 'id_conflict', 'rejected']);
const swapDefinition = workflowDefinition('simultaneous-bindings', {
  inputs: [port('label', 'string')], outputs: [], variables: [workflowVariable('left', 'integer', int(1)), workflowVariable('right', 'integer', int(2)), workflowVariable('label', 'string', str('initial'))], entry: 'wait',
  nodes: [workflowNode('wait', 'task', { task: { kind: 'adapter', title: 'Explicit external response', instructions: 'No adapter call is performed by this model.', request: { left: variable('left'), right: variable('right'), rootLabel: input('label'), currentLabel: variable('label') }, response: [], assign: {} }, next: 'done' }), workflowNode('done', 'end', { end: { outcome: 'done', outputs: {} } })],
  signals: [{ name: 'swap', inputs: [port('label', 'string')], set: { left: variable('right'), right: variable('left'), label: input('label') }, target: 'wait' }],
});
const swap = await prepareWorkflow(swapDefinition);
const swapResult = (await checkWorkflow(swap, workflowScenario('Signal assignments read one snapshot and their own input scope', { label: str('root') }, [eventSignal('swap', 'swap', { label: str('signal') })], workflowExpectation('waiting', '', 'wait', { left: int(2), right: int(1), label: str('signal') }, 2, 0, 1, 1)))).result;
assert.equal(swapResult.passed, true); assert.deepEqual(swapResult.tasks.map(t => t.request), [{ left: int(1), right: int(2), rootLabel: str('root'), currentLabel: str('initial') }, { left: int(2), right: int(1), rootLabel: str('root'), currentLabel: str('signal') }]);
report('rejected event receipts stay historical, unsupported signals do not cancel work, and simultaneous signal bindings use the correct immutable scope');

const dataDefinition = { schemaVersion: 1, id: `${prefix}.payload`, version: '1.0.0', name: 'Typed task payload', description: 'A small independent nominal task record.', author: 'Acceptance suite', license: 'MIT', records: [{ name: 'Item', fields: [port('id', 'string'), port('amount', 'integer')] }], entities: [], relationships: [] };
const data = await post('/api/data/prepare', dataDefinition), itemRef = dataRef(data, 'Item');
const otherDataDefinition = clone(dataDefinition); otherDataDefinition.id += '-other'; const otherData = await post('/api/data/prepare', otherDataDefinition), otherRef = dataRef(otherData, 'Item');
const item = (id, amount, ref = itemRef) => record(ref, { id: str(id), amount: int(amount) });
const structuredDefinition = workflowDefinition('record-task', {
  dataContracts: [data, otherData], inputs: [port('initial', 'record', itemRef)], outputs: [port('result', 'record', itemRef)], variables: [workflowVariable('value', 'record', item('none', 0), itemRef)], entry: 'capture',
  nodes: [workflowNode('capture', 'task', { task: { kind: 'human', title: 'Capture', instructions: 'Explicit synthetic record.', request: { value: input('initial') }, response: [port('value', 'record', itemRef)], assign: { value: 'value' } }, next: 'review' }), workflowNode('review', 'task', { task: { kind: 'human', title: 'Review', instructions: 'Explicit synthetic review.', request: { value: variable('value') }, response: [port('value', 'record', itemRef)], assign: { value: 'value' } }, next: 'done' }), workflowNode('done', 'end', { end: { outcome: 'recorded', outputs: { result: variable('value') } } })],
});
const structured = await prepareWorkflow(structuredDefinition);
const structuredScenario = workflowScenario('Task snapshots preserve exact nominal values', { initial: item('initial', 1) }, [eventResponse('wrongNominal', 'task1', { value: item('wrong', 2, otherRef) }), eventResponse('capture', 'task1', { value: item('first', 3) }), eventResponse('review', 'task2', { value: item('second', 4) })], workflowExpectation('completed', 'recorded', '', { value: item('second', 4) }, 2, 2, 0, 3));
const structuredResult = (await checkWorkflow(structured, structuredScenario)).result; assert.equal(structuredResult.passed, true); assert.deepEqual(structuredResult.events.map(e => e.status), ['rejected', 'accepted', 'accepted']); assert.deepEqual(structuredResult.tasks[0].request, { value: item('initial', 1) }); assert.deepEqual(structuredResult.tasks[0].values, { value: item('first', 3) }); assert.deepEqual(structuredResult.tasks[1].request, { value: item('first', 3) }); assert.deepEqual(structuredResult.tasks[1].values, { value: item('second', 4) }); assert.deepEqual(structuredResult.outputs, { result: item('second', 4) });
const incompatibleAssign = clone(structuredDefinition); incompatibleAssign.nodes[0].task.response[0].recordRef = otherRef; await post('/api/workflow/prepare', incompatibleAssign, 422);
const unknownRecordInput = clone(structuredScenario); unknownRecordInput.inputs.initial.record.extra = bool(true); await checkWorkflow(structured, unknownRecordInput, 422);
report('structured task snapshots remain historical and nominally typed; matching record shape does not substitute for an exact contract reference');

const loopDefinition = workflowDefinition('explicit-task-loop', { variables: [workflowVariable('again', 'boolean', bool(true))], entry: 'repeat', nodes: [workflowNode('repeat', 'task', { task: { kind: 'adapter', title: 'Explicit retry', instructions: 'Only a supplied response may repeat this task.', request: {}, response: [port('again', 'boolean')], assign: { again: 'again' } }, branch: { variable: 'again', then: 'repeat', else: 'done' } }), workflowNode('done', 'end', { end: { outcome: 'done', outputs: {} } })] });
const loop = await prepareWorkflow(loopDefinition);
const loopEvents = Array.from({ length: 63 }, (_, i) => eventResponse(`event${i + 1}`, `task${i + 1}`, { again: bool(true) }));
const fullTasks = workflowScenario('Sixty-four retained task activations', {}, loopEvents, workflowExpectation('waiting', '', 'repeat', {}, 64, 63, 0, 63));
const fullTasksResult = (await checkWorkflow(loop, fullTasks)).result; assert.equal(fullTasksResult.passed, true); assert.equal(fullTasksResult.task.id, 'task64'); assert.equal(fullTasksResult.tasks[0].id, 'task1');
const completedCapacity = clone(fullTasks); completedCapacity.events.push(eventResponse('event64', 'task64', { again: bool(false) })); completedCapacity.expected = workflowExpectation('completed', 'done', '', {}, 64, 64, 0, 64); assert.equal((await checkWorkflow(loop, completedCapacity)).result.passed, true);
const tooManyTasks = clone(fullTasks); tooManyTasks.events.push(eventResponse('event64', 'task64', { again: bool(true) })); assert.match(JSON.stringify(await saveWorkflow(loop, tooManyTasks, 422)), /task|64/i);
const tooManyEvents = clone(completedCapacity); tooManyEvents.events.push(eventResponse('event65', 'task64', { again: bool(false) })); await checkWorkflow(loop, tooManyEvents, 422);
const ninetySixNodes = clone(loopDefinition); ninetySixNodes.id += '-nodes'; ninetySixNodes.entry = 'node0'; ninetySixNodes.nodes = Array.from({ length: 95 }, (_, i) => workflowNode(`node${i}`, 'task', { task: { kind: 'human', title: `Task ${i}`, instructions: '', request: {}, response: [], assign: {} }, next: i === 94 ? 'done' : `node${i + 1}` })); ninetySixNodes.nodes.push(workflowNode('done', 'end', { end: { outcome: 'done', outputs: {} } })); await prepareWorkflow(ninetySixNodes); ninetySixNodes.nodes.splice(94, 0, workflowNode('extra', 'end', { end: { outcome: 'extra', outputs: {} } })); await post('/api/workflow/prepare', ninetySixNodes, 422);
const fortyEightVars = clone(loopDefinition); fortyEightVars.id += '-variables'; for (let i = 1; i < 48; i++) fortyEightVars.variables.push(workflowVariable(`spare${i}`, 'boolean', bool(false))); await prepareWorkflow(fortyEightVars); fortyEightVars.variables.push(workflowVariable('spare48', 'boolean', bool(false))); await post('/api/workflow/prepare', fortyEightVars, 422);
const alwaysPackage = packageDefinition('always', { inputs: [], outputs: [port('again', 'boolean')], nodes: [], bindings: { again: literal(bool(true)) } }), always = await prepareComponent(alwaysPackage);
const spinDefinition = workflowDefinition('bounded-automatic-loop', { decisions: [always], variables: [workflowVariable('again', 'boolean', bool(true))], entry: 'spin', nodes: [workflowNode('spin', 'decision', { decision: { component: always.root, inputs: {}, assign: { again: 'again' } }, branch: { variable: 'again', then: 'spin', else: 'done' } }), workflowNode('done', 'end', { end: { outcome: 'done', outputs: {} } })] });
const spin = await prepareWorkflow(spinDefinition); assert.match(JSON.stringify(await saveWorkflow(spin, workflowScenario('Automatic cycles have a finite transition cap', {}), 422)), /transition|256/i);
report('task, event, node and variable capacities preserve boundary cases while automatic cycles stop at a finite transition budget');

const exactTransitionsDefinition = clone(loopDefinition); exactTransitionsDefinition.id += '-transitions'; exactTransitionsDefinition.entry = 'initialA'; exactTransitionsDefinition.decisions = [always];
const zeroDecision = (id, next) => workflowNode(id, 'decision', { decision: { component: always.root, inputs: {}, assign: {} }, next });
exactTransitionsDefinition.nodes[0].branch.then = 'cycleA';
exactTransitionsDefinition.nodes.push(zeroDecision('initialA', 'initialB'), zeroDecision('initialB', 'repeat'), zeroDecision('cycleA', 'cycleB'), zeroDecision('cycleB', 'cycleC'), zeroDecision('cycleC', 'repeat'));
const exactTransitions = await prepareWorkflow(exactTransitionsDefinition);
const exactTransitionsResult = (await checkWorkflow(exactTransitions, completedCapacity)).result;
assert.equal(exactTransitionsResult.passed, true); assert.equal(exactTransitionsResult.steps.length, 256); assert.equal(exactTransitionsResult.tasks.length, 64); assert.equal(exactTransitionsResult.receipts.length, 64); assert.equal(exactTransitionsResult.operations, 0); assert.equal(exactTransitionsResult.invocations, 191);
const oneMoreTransitionDefinition = clone(exactTransitionsDefinition); oneMoreTransitionDefinition.id += '-overflow'; oneMoreTransitionDefinition.entry = 'initialExtra'; oneMoreTransitionDefinition.nodes.push(zeroDecision('initialExtra', 'initialA')); const oneMoreTransition = await prepareWorkflow(oneMoreTransitionDefinition);
assert.match(JSON.stringify(await saveWorkflow(oneMoreTransition, completedCapacity, 422)), /transition|256/i);
report('exactly 256 transitions, 64 tasks and 64 event receipts succeed together; one extra transition rejects the whole experiment');

const workLeafPackage = packageDefinition('work-leaf', { inputs: [port('value', 'integer')], outputs: [port('value', 'integer')], nodes: Array.from({ length: 32 }, (_, i) => primitive(`add${i}`, 'add', { left: i ? source(`add${i - 1}`) : input('value'), right: literal(int(0)) })), bindings: { value: source('add31') } });
const workLeaf = await prepareComponent(workLeafPackage);
const workRootPackage = packageDefinition('work-root', { inputs: [port('value', 'integer')], outputs: [port('accepted', 'boolean'), port('value', 'integer')], nodes: Array.from({ length: 4 }, (_, i) => call(`child${i}`, workLeaf.root, { value: i ? source(`child${i - 1}`, 'value') : input('value') })), bindings: { accepted: literal(bool(true)), value: source('child3', 'value') } });
const workRoot = await prepareComponent(workRootPackage, workLeaf.packages);
const workOwner = await prepare(machineDefinition('global-work-owner', [slot('value', 'integer', 'durable', int(0))], [{ name: 'commit', inputs: [], outputs: [port('value', 'integer')], decision: workRoot, bindings: { value: { state: 'value' } }, acceptedOutput: 'accepted', writes: { value: 'value' }, results: { value: 'value' }, effects: [] }]));
const workDefinition = workflowDefinition('shared-pure-and-state-work', { decisions: [workRoot], machines: [workOwner], owners: [{ id: 'owner', name: 'Owned work', machine: { machineId: workOwner.machine.id, version: workOwner.machine.version, digest: workOwner.digest }, initial: {} }], variables: [workflowVariable('value', 'integer', int(0)), workflowVariable('accepted', 'boolean', bool(false)), workflowVariable('again', 'boolean', bool(true))], outputs: [port('value', 'integer')], entry: 'pure', nodes: [workflowNode('pure', 'decision', { decision: { component: workRoot.root, inputs: { value: variable('value') }, assign: { value: 'value' } }, next: 'owned' }), workflowNode('owned', 'command', { command: { owner: 'owner', command: 'commit', inputs: {}, assign: { value: 'value' }, acceptedVariable: 'accepted', errorNext: 'done' }, next: 'continue' }), workflowNode('continue', 'task', { task: { kind: 'human', title: 'Continue explicitly', instructions: '', request: {}, response: [port('again', 'boolean')], assign: { again: 'again' } }, branch: { variable: 'again', then: 'pure', else: 'done' } }), workflowNode('done', 'end', { end: { outcome: 'done', outputs: { value: variable('value') } } })] });
const work = await prepareWorkflow(workDefinition);
const workEvents = Array.from({ length: 16 }, (_, i) => eventResponse(`work${i + 1}`, `task${i + 1}`, { again: bool(i !== 15) }));
const workScenario = workflowScenario('One budget spans pure decisions and owned commands', {}, workEvents, workflowExpectation('completed', 'done', '', { value: int(0), accepted: bool(true) }, 16, 16, 0, 16, [expectation('owner', { value: int(0) }, 16, 16, 0)]));
const workResult = (await checkWorkflow(work, workScenario)).result; assert.equal(workResult.passed, true); assert.equal(workResult.operations, 4096); assert.equal(workResult.invocations, 160); assert.equal(workResult.owners[0].receipts[15].commandId, 'wf_command16');
const tooMuchWork = clone(workScenario); tooMuchWork.events[15].values.again = bool(true); assert.match(JSON.stringify(await saveWorkflow(work, tooMuchWork, 422)), /work|operation|4096/i);
report('expanded work is counted once and reserved globally across pure workflow decisions and reused owned-state kernels');

const wideDefinition = clone(dataDefinition); wideDefinition.id += '-wide'; wideDefinition.records = [{ name: 'Wide', fields: Array.from({ length: 16 }, (_, i) => port(`field${i}`, 'string')) }]; const wideData = await post('/api/data/prepare', wideDefinition), wideRef = dataRef(wideData, 'Wide');
const wide = size => record(wideRef, Object.fromEntries(Array.from({ length: 16 }, (_, i) => [`field${i}`, str('w'.repeat(size))])));
const wideTaskDefinition = workflowDefinition('bounded-task-payload', { dataContracts: [wideData], inputs: [port('value', 'record', wideRef)], entry: 'repeat', nodes: [workflowNode('repeat', 'task', { task: { kind: 'adapter', title: 'Bounded retained payload', instructions: '', request: { value: input('value') }, response: [], assign: {} }, next: 'repeat' }), workflowNode('done', 'end', { end: { outcome: 'done', outputs: {} } })], signals: [{ name: 'stop', inputs: [], set: {}, target: 'done' }] });
const wideTask = await prepareWorkflow(wideTaskDefinition);
const mappedRequestDefinition = clone(wideTaskDefinition); mappedRequestDefinition.id += '-fanout'; mappedRequestDefinition.nodes[0].task.request = Object.fromEntries(Array.from({ length: 5 }, (_, i) => [`value${i}`, input('value')])); const mappedRequest = await prepareWorkflow(mappedRequestDefinition);
const mappedRequestScenario = workflowScenario('One valid source cannot amplify task request size without bounds', { value: wide(1000) }); assert.match(JSON.stringify(await saveWorkflow(mappedRequest, mappedRequestScenario, 422)), /request|65536|64 KiB|map|limit/i);
const variableFanoutDefinition = clone(wideTaskDefinition); variableFanoutDefinition.id += '-assign'; variableFanoutDefinition.variables = Array.from({ length: 5 }, (_, i) => workflowVariable(`value${i}`, 'record', wide(0), wideRef)); variableFanoutDefinition.nodes[0].task.response = [port('value', 'record', wideRef)]; variableFanoutDefinition.nodes[0].task.assign = Object.fromEntries(Array.from({ length: 5 }, (_, i) => [`value${i}`, 'value'])); const variableFanout = await prepareWorkflow(variableFanoutDefinition);
const variableFanoutScenario = workflowScenario('One bounded response cannot overflow aggregate workflow variables', { value: wide(0) }, [eventResponse('assign', 'task1', { value: wide(3990) })]); assert.match(JSON.stringify(await saveWorkflow(variableFanout, variableFanoutScenario, 422)), /variable|262144|256 KiB|map|limit/i);
const evidenceScenario = workflowScenario('Retained task history counts toward the whole evidence budget', { value: wide(3990) }, Array.from({ length: 63 }, (_, i) => eventResponse(`wide${i}`, `task${i + 1}`, {})));
assert.match(JSON.stringify(await saveWorkflow(wideTask, evidenceScenario, 422)), /result|evidence|4194304|4 MiB/i);
await request('/api/workflow/prepare', { method: 'POST', raw: ' '.repeat(2 * 1024 * 1024 + 1), expected: 413 });
report('task request fan-out, variable assignment amplification and retained evidence are bounded before partial publication');

const catalogPaths = ['/api/components', '/api/data/contracts', '/api/state/machines', '/api/delivery/models'];
const catalogsBefore = await Promise.all(catalogPaths.map(path => get(path)));
const installations = await Promise.all([request('/api/workflow/packages', { method: 'POST', body: workflow, expected: [200, 201] }), request('/api/workflow/packages', { method: 'POST', body: workflow, expected: [200, 201] })]);
assert.deepEqual(installations.map(r => r.status).sort(), [200, 201]); assert.deepEqual(installations[0].data, installations[1].data); const installed = installations[0].data;
assert.deepEqual(installed.workflow, definition); assert.equal(installed.digest, workflow.digest); assert.deepEqual(await post('/api/workflow/packages', workflow), installed);
const exactPath = lock => `/api/workflow/package?id=${encodeURIComponent(lock.workflow.id)}&version=${encodeURIComponent(lock.workflow.version)}&digest=${lock.digest}`;
assert.deepEqual(await get(exactPath(workflow)), installed); await get(exactPath({ ...workflow, digest: 'f'.repeat(64) }), 404); await get('/api/workflow/package', 422);
const conflictDefinition = clone(definition); conflictDefinition.description += ' conflict'; const conflict = await prepareWorkflow(conflictDefinition); await post('/api/workflow/packages', conflict, 409);
const newerDefinition = clone(definition); newerDefinition.version = '1.0.1'; newerDefinition.name += ' newer'; const newer = await prepareWorkflow(newerDefinition); await post('/api/workflow/packages', newer, 201);
const installedCatalog = await get('/api/workflow/packages'), installedKeys = installedCatalog.map(r => `${r.workflow.id}\u0000${r.workflow.version}`); assert.deepEqual(installedKeys, [...installedKeys].sort()); assert.equal(installedCatalog.filter(r => r.workflow.id === definition.id).length, 2);
assert.deepEqual(await Promise.all(catalogPaths.map(path => get(path))), catalogsBefore); await request('/api/workflow/packages', { method: 'DELETE', expected: 405 });
report('workflow installation is atomic, immutable and exact without installing or changing its independent dependency catalogs');

const historyBefore = await get('/api/workflow/runs');
await checkWorkflow(workflow, normal); await saveWorkflow(work, tooMuchWork, 422); await saveWorkflow(wideTask, evidenceScenario, 422); await saveWorkflow(loop, tooManyTasks, 422); assert.deepEqual(await get('/api/workflow/runs'), historyBefore);
const failedScenario = clone(normal); failedScenario.expected.variables.remaining = int(99); const failed = await saveWorkflow(workflow, failedScenario); assert.equal(failed.result.verified, true); assert.equal(failed.result.passed, false); assert.equal(failed.result.status, 'completed'); assert.deepEqual(failed.result.owners, normalResult.owners); assert.ok(failed.result.issues.some(i => i.path === 'expected.variables.remaining'));
const saved = await saveWorkflow(workflow, normal), savedAgain = await saveWorkflow(workflow, normal); assert.notEqual(saved.id, savedAgain.id); assert.deepEqual(saved.result, normalResult); assert.deepEqual(savedAgain.result, normalResult); assert.deepEqual(saved.workflow, workflow); assert.deepEqual(saved.scenario, normal);
assert.deepEqual(await get(`/api/workflow/runs/${saved.id}`), saved); assert.deepEqual(await get(`/api/workflow/runs/${failed.id}`), failed);
const history = await get('/api/workflow/runs'); assert.deepEqual(history.slice(0, 2).map(r => r.id), [savedAgain.id, saved.id]); assert.deepEqual(history[0], { id: savedAgain.id, createdAt: savedAgain.createdAt, workflowId: definition.id, workflowVersion: definition.version, status: 'completed', outcome: 'handoffAccepted', eventCount: 2, taskCount: 2, verified: true, passed: true });
const newerRun = await saveWorkflow(newer, normal); assert.equal(newerRun.result.workflowHash, newer.digest); assert.deepEqual(newerRun.result.tasks, normalResult.tasks); assert.deepEqual(newerRun.result.owners, normalResult.owners); assert.deepEqual(await get(`/api/workflow/runs/${saved.id}`), saved);
const unverifiedSaved = await saveWorkflow(workflow, noExpectations); assert.equal(unverifiedSaved.result.verified, false); assert.equal(unverifiedSaved.result.passed, false);
await get('/api/workflow/runs/not-present', 404); await request(`/api/workflow/runs/${saved.id}`, { method: 'DELETE', expected: 405 }); await request('/api/workflow/runs', { method: 'DELETE', expected: 405 });
assert.equal(capacity.digest, sha(orderedMachine(capacityDefinition))); assert.equal(gate.root.digest, sha(orderedPackage(gatePackage)));
const oldState = await check(capacity, scenario('Legacy state semantics after workflow extension', [instance('inventory', { available: int(5) })], [directStateCommand()], [inventoryExpectation(3, 1, 1)]));
function directStateCommand() { return { kind: 'command', instanceId: 'inventory', commandId: 'msg_direct', command: 'reserve', expectedRevision: 0, inputs: { amount: int(2) } }; }
assert.equal(oldState.result.engineVersion, '0.6.0'); assert.equal(oldState.result.passed, true); assert.equal(oldState.result.steps[0].decision.engineVersion, '0.5.0'); assert.equal((await get('/api/components/primitives')).length, 11); assert.equal((await get('/api/components/structured-primitives')).length, 16);
const legacyDelivery = (await get('/api/delivery/examples'))[0]; assert.equal((await post('/api/delivery/check', legacyDelivery)).result.engineVersion, '0.7.0');
report('immutable evidence keeps failed and unverified outcomes, rejects partial persistence on limits, and preserves existing state, delivery and pure engines');

// The portable reception definition is the subject under test. All event facts,
// task order, final variables and owner mutations below are authored here; no
// example scenario or result is used as an oracle.
const receptionExamples = await get('/api/workflow/examples');
assert.ok(receptionExamples.length >= 1);
const reception = clone(receptionExamples.find(e => e.workflow.workflow.id === 'health.hospital-reception').workflow);
assert.equal(reception.digest, sha(orderedWorkflow(reception.workflow)));
assert.deepEqual(await prepareWorkflow(reception.workflow), reception);
const candidateRef = dataRef(reception.workflow.dataContracts.find(l => l.contract.id === 'health.reception-values'), 'CandidateFacts');
const receptionInputs = () => ({ caseId: str('case-qa77'), arrivalId: str('arrival-qa77'), facilityId: str('hospital-west'), serviceId: str('general-medicine'), identifier: str('synthetic-qa77'), capacityA: int(1), capacityB: int(1), queueStart: int(700) });
const candidate = (resourceId, overrides = {}) => record(candidateRef, { resourceId: str(resourceId), facilityMatches: bool(true), serviceMatches: bool(true), qualified: bool(true), onDuty: bool(true), available: int(1), ...overrides });
const receptionResponse = (index, values, id = `answer${index}`) => eventResponse(id, `task${index}`, { ...values, actorRef: str('reviewer-qa'), evidenceRef: str(`independent-evidence-${index}`) });
const emergencySignal = (id = 'urgentQA') => eventSignal(id, 'escalateEmergency', { actorRef: str('urgent-reviewer'), assessmentRef: str('urgent-assessment-qa') });
const ecuResponse = (index, overrides = {}, id = `ecu${index}`) => receptionResponse(index, { accepted: bool(true), caseId: str('case-qa77'), receivingUnit: str('ecu-west'), encounterId: str('emergency:case-qa77'), reference: str('receiving-qa77'), ...overrides }, id);
const routineEvents = () => [
  receptionResponse(1, { assessmentRecorded: bool(true), emergencyIdentified: bool(false), assessmentRef: str('screen-qa77') }),
  receptionResponse(2, { ready: bool(true) }),
  receptionResponse(3, { completed: bool(true), matchCount: int(1), patientId: str('patient-shared') }),
  receptionResponse(4, { confirmed: bool(true), patientId: str('patient-shared') }),
  receptionResponse(5, { status: str('verified') }),
  receptionResponse(6, { observationsAccepted: bool(true), feverAlert: bool(false), observationRef: str('observations-qa77'), assessmentRef: str('assessment-qa77') }),
  receptionResponse(7, { disposition: str('released'), authorized: bool(true), assessmentRef: str('assessment-qa77') }),
  receptionResponse(8, { disposition: str('allow') }),
  receptionResponse(9, { candidateA: candidate('providerA'), candidateB: candidate('providerB') }),
  receptionResponse(10, { choice: str('candidateA') }),
  receptionResponse(11, { accepted: bool(true), resourceId: str('providerA'), caseId: str('case-qa77'), reference: str('physician-acceptance-qa77') }),
];
function receptionOwners({ issued = true, called = true, cancelled = false, opened = true, patient = 'patient-shared', doctor = 'A', doctorRevision = 0, doctorReceipts = 0, held = false, accepted = false, capacityA = 1, capacityB = 1, caseId = 'case-qa77', queueStart = 700 } = {}) {
  const queueRevision = Number(issued) + Number(called) + Number(cancelled);
  const owners = [
    expectation('queue', { caseId: str(issued ? caseId : ''), ticketId: str(issued ? `ticket:${caseId}` : ''), called: bool(called), cancelled: bool(cancelled), ticketNumber: int(issued ? queueStart : 0), nextNumber: int(queueStart + Number(issued)) }, queueRevision, queueRevision, 0),
    expectation('encounter', { caseId: str(opened ? caseId : ''), patientId: str(opened ? patient : ''), encounterId: str(opened ? `encounter:${caseId}` : '') }, Number(opened), Number(opened), 0),
  ];
  for (const which of ['A', 'B']) {
    const active = which === doctor;
    owners.push(expectation(`doctor${which}`, { resourceId: str(`provider${which}`), available: int(active && held ? 0 : which === 'A' ? capacityA : capacityB), holder: str(active && held ? caseId : ''), accepted: bool(active && accepted) }, active ? doctorRevision : 0, active ? doctorReceipts : 0, 0));
  }
  return owners;
}
const receptionExpected = (status, outcome, active, variables, tasks, completed, cancelled, receipts, owners) => workflowExpectation(status, outcome, active, variables, tasks, completed, cancelled, receipts, owners);
const runReception = async (name, events, expected, inputs = receptionInputs()) => {
  const experiment = workflowScenario(name, inputs, events, expected), result = (await checkWorkflow(reception, experiment)).result;
  assert.equal(result.verified, true, name); assert.equal(result.passed, true, `${name}: ${JSON.stringify(result.issues)}`);
  assert.equal(result.workflowHash, reception.digest); assert.equal(result.scenarioHash, sha(orderedWorkflowScenario(experiment)));
  return result;
};
const normalReceptionExpected = (patient = 'patient-shared', noAffiliation = false, ownerOptions = {}) => receptionExpected('completed', 'consultationAccepted', '', { patientId: str(patient), ticketNumber: int(ownerOptions.queueStart ?? 700), clinicalReleased: bool(true), noAffiliation: bool(noAffiliation), reservationHeld: bool(true), doctorAccepted: bool(true), emergencyActive: bool(false) }, 11, 11, 0, 11, receptionOwners({ patient, doctorRevision: 2, doctorReceipts: 2, held: true, accepted: true, ...ownerOptions }));
const routine = await runReception('Independent complete routine reception', routineEvents(), normalReceptionExpected());
assert.deepEqual(routine.tasks.map(t => t.nodeId), ['initialScreening', 'callNext', 'lookupIdentity', 'confirmIdentity', 'lookupAffiliation', 'recordVitals', 'clinicalReview', 'paymentPolicy', 'queryWorkforce', 'selectDoctor', 'physicianAcceptance']);
assert.deepEqual(routine.outputs, { patientId: str('patient-shared'), encounterId: str('encounter:case-qa77'), ticketId: str('ticket:case-qa77'), ticketNumber: int(700), handoffRef: str('physician-acceptance-qa77'), selectedResource: str('providerA'), noAffiliation: bool(false), reservationHeld: bool(true) });
assert.equal(routine.tasks[1].request.ticketNumber.integer, 700); assert.equal(routine.tasks[10].request.resourceId.string, 'providerA');
const beforeReceiving = await runReception('Reservation still requires receiving physician', routineEvents().slice(0, 10), receptionExpected('waiting', '', 'physicianAcceptance', { reservationHeld: bool(true), doctorAccepted: bool(false) }, 11, 10, 0, 10, receptionOwners({ doctorRevision: 1, doctorReceipts: 1, held: true })));
assert.deepEqual(beforeReceiving.outputs, {}); assert.equal(beforeReceiving.tasks.at(-1).status, 'pending');
let patientEvents = routineEvents(); patientEvents[2].values.matchCount = int(0); patientEvents[2].values.patientId = str(''); patientEvents[3].values.patientId = str('new-patient-qa');
const registered = await runReception('Confirmed zero match creates one named patient', patientEvents, normalReceptionExpected('new-patient-qa'));
assert.equal(registered.tasks[3].nodeId, 'registerPatient'); assert.equal(registered.tasks.filter(t => t.nodeId === 'registerPatient').length, 1);
patientEvents.splice(4, 0, clone(patientEvents[3])); const replayRegistrationExpected = normalReceptionExpected('new-patient-qa');
const registeredReplay = await runReception('Registration response replay does not open another encounter', patientEvents, replayRegistrationExpected);
assert.equal(registeredReplay.events[4].status, 'replayed'); assert.deepEqual(registeredReplay.owners, registered.owners);
report('complete reception uses explicit identity, clinical release, atomic reservation and receiving acceptance; registration replay creates no extra encounter');

for (const [completed, count, label] of [[false, 0, 'unavailable'], [true, 2, 'ambiguous'], [true, -1, 'invalid negative count']]) {
  const events = routineEvents().slice(0, 3); events[2].values = { ...events[2].values, completed: bool(completed), matchCount: int(count), patientId: str('') };
  const result = await runReception(`Identity ${label}`, events, receptionExpected('waiting', '', 'identityReview', { lookupRegister: bool(false), lookupConfirm: bool(false) }, 4, 3, 0, 3, receptionOwners({ opened: false })));
  assert.equal(result.tasks.some(t => t.nodeId === 'registerPatient'), false);
}
for (const affiliation of ['unknown', 'unavailable', 'expired', 'contradictory', '']) {
  const events = routineEvents().slice(0, 5); events[4].values.status = str(affiliation);
  await runReception(`Unresolved affiliation ${affiliation}`, events, receptionExpected('waiting', '', 'affiliationReview', { noAffiliation: bool(false), encounterId: str('') }, 6, 5, 0, 5, receptionOwners({ opened: false })));
}
for (const payment of ['allow', 'defer', 'waive']) {
  const events = routineEvents(); events[4].values.status = str('confirmedNone'); events[7].values.disposition = str(payment);
  const result = await runReception(`Confirmed no affiliation with ${payment}`, events, normalReceptionExpected('patient-shared', true));
  assert.equal(result.tasks[7].request.noAffiliation.boolean, true);
}
for (const disposition of ['review', 'policyError', 'unknown']) {
  const events = routineEvents().slice(0, 8); events[7].values.disposition = str(disposition);
  await runReception(`Payment ${disposition} is unresolved`, events, receptionExpected('waiting', '', 'paymentReview', { reservationHeld: bool(false) }, 9, 8, 0, 8, receptionOwners()));
}
report('unavailable or ambiguous identity and unknown affiliation remain distinct from confirmed absence; payment failures stay explicit and pending');

for (const [accepted, reference] of [[false, ''], [true, '']]) {
  const events = routineEvents().slice(0, 6); events[5].values.observationsAccepted = bool(accepted); events[5].values.observationRef = str(reference);
  await runReception('Missing observations cannot clear clinical review', events, receptionExpected('waiting', '', 'observationReview', { clinicalReleased: bool(false), reservationHeld: bool(false) }, 7, 6, 0, 6, receptionOwners()));
}
for (const [fever, disposition, authorized, reference] of [[true, 'review', true, 'assessment-qa77'], [false, 'released', false, 'assessment-qa77'], [false, 'released', true, 'old-assessment'], [false, 'released', true, '']]) {
  const events = routineEvents().slice(0, 7); events[5].values.feverAlert = bool(fever); events[6].values = { ...events[6].values, disposition: str(disposition), authorized: bool(authorized), assessmentRef: str(reference) };
  const result = await runReception('Clinical release requires current explicit authority', events, receptionExpected('waiting', '', fever ? 'feverReview' : 'clinicalReview', { clinicalReleased: bool(false), reservationHeld: bool(false) }, 8, 7, 0, 7, receptionOwners()));
  assert.equal(result.tasks.at(-1).request.assessmentRef.string, 'assessment-qa77');
}
const feverEvents = routineEvents(); feverEvents[5].values.feverAlert = bool(true);
const fever = await runReception('Fever requires explicit professional release', feverEvents, normalReceptionExpected()); assert.equal(fever.tasks[6].nodeId, 'feverReview');
const missingEvidence = routineEvents().slice(0, 1); delete missingEvidence[0].values.evidenceRef;
const malformedResponse = await runReception('Missing provenance is rejected without consuming task', missingEvidence, receptionExpected('waiting', '', 'initialScreening', { ticketNumber: int(0) }, 1, 0, 0, 1, receptionOwners({ issued: false, called: false, opened: false })));
assert.equal(malformedResponse.events[0].status, 'rejected');
report('missing observations, stale or unauthorized clinical release and incomplete task provenance cannot silently advance the reception case');

for (let prefixLength = 0; prefixLength <= 10; prefixLength++) {
  const events = routineEvents().slice(0, prefixLength), oldTask = prefixLength + 1, ecuTask = prefixLength + 2;
  events.push(emergencySignal(), ecuResponse(ecuTask), receptionResponse(oldTask, routineEvents()[prefixLength].values, `late${prefixLength}`));
  const result = await runReception(`Emergency preemption at routine stage ${prefixLength}`, events,
    receptionExpected('completed', 'emergencyAccepted', '', { emergencyActive: bool(true), routineActive: bool(false), clinicalReleased: bool(false), reservationHeld: bool(false), doctorAccepted: bool(false) }, prefixLength + 2, prefixLength + 1, 1, prefixLength + 3,
      receptionOwners({ issued: prefixLength > 0, called: prefixLength > 1, cancelled: prefixLength > 0, opened: prefixLength >= 5, doctorRevision: prefixLength === 10 ? 2 : 0, doctorReceipts: prefixLength === 10 ? 2 : 0 })));
  assert.equal(result.tasks[oldTask - 1].status, 'cancelled'); assert.equal(result.tasks[oldTask - 1].completedBy, 'urgentQA');
  assert.equal(result.events.at(-1).status, 'rejected'); assert.equal(result.outputs.encounterId.string, 'emergency:case-qa77'); assert.equal(result.outputs.selectedResource.string, 'ecu-west');
  assert.equal(result.tasks.filter(t => t.nodeId === 'ecuAcceptance').length, 1);
}
for (const mismatch of [{ accepted: bool(false) }, { caseId: str('another-case') }, { receivingUnit: str('') }, { encounterId: str('') }, { reference: str('') }]) {
  const result = await runReception('Emergency requires actual matching receiving acceptance', [emergencySignal(), ecuResponse(2, mismatch)], receptionExpected('waiting', '', 'ecuPending', { emergencyActive: bool(true), routineActive: bool(false) }, 3, 1, 1, 2, receptionOwners({ issued: false, called: false, opened: false })));
  assert.deepEqual(result.outputs, {});
}
const clinicalEmergency = routineEvents().slice(0, 7); clinicalEmergency[6].values = { ...clinicalEmergency[6].values, disposition: str('emergency'), authorized: bool(false), assessmentRef: str('') }; clinicalEmergency.push(ecuResponse(8));
await runReception('Clinical emergency bypasses routine administrative release', clinicalEmergency, receptionExpected('completed', 'emergencyAccepted', '', { clinicalReleased: bool(false), emergencyActive: bool(true), reservationHeld: bool(false) }, 8, 8, 0, 8, receptionOwners({ cancelled: true })));
report('emergency preempts all 11 routine stages, cancels stale responses, releases held capacity and remains pending until exact receiving acceptance');

for (const facts of [{ available: int(0) }, { available: int(-1) }, { onDuty: bool(false) }, { qualified: bool(false) }, { serviceMatches: bool(false) }, { facilityMatches: bool(false) }, { resourceId: str('unconfigured-provider') }]) {
  const events = routineEvents().slice(0, 10); events[8].values.candidateA = candidate('providerA', facts);
  await runReception('Ineligible or mismatched candidate cannot be reserved', events, receptionExpected('waiting', '', 'waitDoctor', { candidateAEligible: bool(false), reservationHeld: bool(false), doctorAccepted: bool(false) }, 11, 10, 0, 10, receptionOwners()));
}
const zeroCapacity = receptionInputs(); zeroCapacity.capacityA = int(0);
const deniedCapacity = await runReception('Owner denies stale availability snapshot', routineEvents().slice(0, 10), receptionExpected('waiting', '', 'waitDoctor', { candidateAEligible: bool(true), reservationHeld: bool(false), doctorAccepted: bool(false) }, 11, 10, 0, 10, receptionOwners({ capacityA: 0, doctorReceipts: 1 })), zeroCapacity);
assert.equal(deniedCapacity.owners.find(o => o.id === 'doctorA').receipts[0].accepted, false);
for (const mismatch of [{ accepted: bool(false) }, { resourceId: str('providerB') }, { caseId: str('other-case') }, { reference: str('') }]) {
  const events = routineEvents(); events[10].values = { ...events[10].values, ...mismatch };
  await runReception('Declined or mismatched physician handoff releases reservation', events, receptionExpected('waiting', '', 'waitDoctor', { reservationHeld: bool(false), doctorAccepted: bool(false) }, 12, 11, 0, 11, receptionOwners({ doctorRevision: 2, doctorReceipts: 2 })));
}
const chooseB = routineEvents(); chooseB[9].values.choice = str('candidateB'); chooseB[10].values.resourceId = str('providerB');
const reservedB = await runReception('Independent provider B can receive a consultation', chooseB, normalReceptionExpected('patient-shared', false, { doctor: 'B' }));
assert.equal(reservedB.outputs.selectedResource.string, 'providerB');
const repeatFinal = routineEvents(); repeatFinal.push(clone(repeatFinal.at(-1))); const replayFinal = await runReception('Receiving response retry does not reserve twice', repeatFinal, normalReceptionExpected()); assert.equal(replayFinal.events.at(-1).status, 'replayed'); assert.deepEqual(replayFinal.owners, routine.owners);
report('eligibility remains a proposal: authoritative reservation denial, receiving mismatch, release and independent provider ownership are verified');

const secondInputs = receptionInputs(); secondInputs.caseId = str('case-qa88'); secondInputs.arrivalId = str('arrival-qa88'); secondInputs.facilityId = str('hospital-east'); secondInputs.queueStart = int(900);
const secondEvents = routineEvents(); secondEvents[10].values.caseId = str('case-qa88');
const secondVisit = await runReception('Same patient new arrival at another facility', secondEvents, normalReceptionExpected('patient-shared', false, { caseId: 'case-qa88', queueStart: 900 }), secondInputs);
assert.equal(secondVisit.outputs.patientId.string, routine.outputs.patientId.string); assert.notEqual(secondVisit.outputs.encounterId.string, routine.outputs.encounterId.string); assert.notEqual(secondVisit.outputs.ticketId.string, routine.outputs.ticketId.string); assert.equal(secondVisit.tasks[0].request.facilityId.string, 'hospital-east');
assert.equal(secondVisit.workflowHash, routine.workflowHash); assert.notEqual(secondVisit.scenarioHash, routine.scenarioHash);
const evidenceBefore = await get('/api/workflow/runs'); const receptionScenario = workflowScenario('Independent complete routine reception', receptionInputs(), routineEvents(), normalReceptionExpected());
const receptionSaved = await saveWorkflow(reception, receptionScenario); assert.deepEqual(receptionSaved.result, routine); assert.deepEqual(await get(`/api/workflow/runs/${receptionSaved.id}`), receptionSaved); assert.equal((await get('/api/workflow/runs')).length, evidenceBefore.length + 1);
assert.deepEqual(await Promise.all(catalogPaths.map(path => get(path))), catalogsBefore);
report('the exact reception package reuses a patient across distinct arrivals and facilities while preserving isolated owners, encounter identity and immutable evidence');

console.log(`All ${checks} workflow and reception acceptance groups passed.`);
