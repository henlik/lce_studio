#!/usr/bin/env node
// Independent HTTP references for pure recursive components. No elapsed wall
// time or evaluator operation count is presented as a physical simulation.
// Use a disposable database: immutable catalog/evaluation evidence is retained.
import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';

const base = (process.env.STUDIO_URL || 'http://127.0.0.1:8080').replace(/\/$/, '');
const suffix = randomUUID();
const prefix = `acceptance.${suffix}`;
const clone = value => structuredClone(value);
let checks = 0;
const report = message => { checks++; console.log(`PASS ${message}`); };

async function request(path, { method = 'GET', body, expected = 200 } = {}) {
  const response = await fetch(`${base}${path}`, {
    method, headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(30000),
  });
  const text = await response.text();
  assert.ok((Array.isArray(expected) ? expected : [expected]).includes(response.status), `${method} ${path}: ${response.status}: ${text.slice(0, 900)}`);
  assert.match(response.headers.get('content-type') || '', /application\/json/);
  return JSON.parse(text);
}
const post = (path, body, expected = 200) => request(path, { method: 'POST', body, expected });
const evaluate = (bundle, inputs) => post('/api/components/evaluate', { bundle, inputs }, 201);
async function validate(bundle) {
  const checked = await post('/api/components/validate-bundle', bundle);
  if (checked.valid) assert.ok(checked.bundle, 'successful validation returns the canonical typed bundle');
  return { valid: checked.valid, issues: checked.issues };
}
const test = bundle => post('/api/components/test', { bundle });
const install = (bundle, expected = 201) => post('/api/components/bundles', bundle, expected);
const value = (type, v) => ({ type, [type]: v });
const boolean = v => value('boolean', v);
const integer = v => value('integer', v);
const string = v => value('string', v);
const input = name => ({ input: name });
const literal = v => ({ literal: v });
const source = (node, port = 'result') => ({ node, port });
const port = (name, type) => ({ name, type });
const primitive = (id, op, inputs) => ({ id, op, inputs, position: { x: 0, y: 0 } });
const call = (id, ref, inputs) => ({ id, component: clone(ref), inputs, position: { x: 0, y: 0 } });
const sortedMap = (map, transform = x => x) => Object.fromEntries(Object.keys(map || {}).sort().map(key => [key, transform(map[key])]));
const ref = lock => ({ packageId: lock.package.id, version: lock.package.version, digest: lock.digest });

// Independent producer of the typed encoding documented in COMPOSITION_CONTRACT.
// Fixtures use ASCII strings, avoiding Go's additional HTML escaping. Maps sort
// lexicographically; declared struct fields and arrays preserve their order.
function orderedValue(v) {
  return { type: v.type, ...Object.fromEntries(['boolean', 'integer', 'string'].filter(k => v[k] !== undefined && v[k] !== null).map(k => [k, v[k]])) };
}
function orderedRef(v) { return { packageId: v.packageId, version: v.version, digest: v.digest }; }
function orderedBinding(v) {
  return { ...(v.input ? { input: v.input } : {}), ...(v.node ? { node: v.node } : {}), ...(v.port ? { port: v.port } : {}), ...(v.literal ? { literal: orderedValue(v.literal) } : {}) };
}
function orderedPackage(p) {
  const parameter = v => ({ default: v.default, min: v.min, max: v.max });
  return {
    schemaVersion: p.schemaVersion, id: p.id, version: p.version, name: p.name,
    description: p.description, author: p.author, license: p.license,
    kernel: p.kernel, capabilities: p.capabilities,
    config: { processingMs: parameter(p.config.processingMs), capacity: parameter(p.config.capacity), memoryMb: parameter(p.config.memoryMb), deduplicate: p.config.deduplicate },
    ...(p.behavior ? { behavior: {
      inputs: p.behavior.inputs.map(v => port(v.name, v.type)),
      outputs: p.behavior.outputs.map(v => port(v.name, v.type)),
      nodes: p.behavior.nodes.map(n => ({ id: n.id, ...(n.op ? { op: n.op } : {}), ...(n.component ? { component: orderedRef(n.component) } : {}), inputs: sortedMap(n.inputs, orderedBinding), position: { x: n.position.x, y: n.position.y } })),
      bindings: sortedMap(p.behavior.bindings, orderedBinding),
    } } : {}),
    ...(p.tests?.length ? { tests: p.tests.map(c => ({ name: c.name, inputs: sortedMap(c.inputs, orderedValue), expected: sortedMap(c.expected, orderedValue) })) } : {}),
  };
}
const sha = v => createHash('sha256').update(JSON.stringify(v)).digest('hex');
const lock = p => ({ package: clone(p), digest: sha(orderedPackage(p)) });
const bundleOf = (p, dependencies = []) => { const root = lock(p); return { schemaVersion: 1, root: ref(root), packages: [root, ...clone(dependencies)] }; };
const bundleHash = b => sha({ schemaVersion: b.schemaVersion, root: orderedRef(b.root), packages: b.packages.map(l => ({ package: orderedPackage(l.package), digest: l.digest })) });
function manifest(name, behavior, tests = []) {
  const zero = () => ({ default: 0, min: 0, max: 0 });
  return { schemaVersion: 2, id: `${prefix}.${name}`, version: '1.0.0', name, description: 'Independent pure component reference', author: 'Acceptance suite', license: 'MIT', kernel: 'composite', capabilities: ['design', 'evaluate'], config: { processingMs: zero(), capacity: zero(), memoryMb: zero(), deduplicate: false }, behavior, ...(tests.length ? { tests } : {}) };
}
async function prepare(p, dependencies = []) {
  const bundle = await post('/api/components/prepare', { package: p, dependencies });
  assert.equal(bundle.root.digest, sha(orderedPackage(p)), 'prepare must use the documented typed digest');
  assert.deepEqual(await validate(bundle), { valid: true, issues: [] });
  return bundle;
}
async function reject(bundle) {
  const result = await validate(bundle);
  assert.equal(result.valid, false);
  assert.ok(result.issues.length > 0);
  const error = await post('/api/components/evaluate', { bundle, inputs: {} }, 422);
  assert.equal(error.id, undefined, 'a rejected evaluation must not return saved successful evidence');
}
const sameLockSet = (a, b) => assert.deepEqual([...a].sort((x, y) => x.package.id.localeCompare(y.package.id)), [...b].sort((x, y) => x.package.id.localeCompare(y.package.id)));
const primitiveEvents = record => record.result.events.filter(e => e.kind === 'primitive');

function policyInputs(domain, flags = [true, true, true, true]) {
  return { subjectTenant: string('tenant-a'), resourceTenant: string(flags[0] ? 'tenant-a' : 'tenant-b'), role: string(flags[1] ? domain.role : 'outsider'), requestedScope: string(flags[2] ? domain.scope : 'unrelated:write'), active: boolean(flags[3]) };
}
function flatPolicy() {
  return manifest('flat-policy', {
    inputs: [port('subjectTenant', 'string'), port('resourceTenant', 'string'), port('role', 'string'), port('requestedScope', 'string'), port('active', 'boolean')],
    outputs: [port('permitted', 'boolean')],
    nodes: [
      primitive('tenant', 'equal', { left: input('subjectTenant'), right: input('resourceTenant') }),
      primitive('role', 'equal', { left: input('role'), right: literal(string('clinician')) }),
      primitive('scope', 'equal', { left: input('requestedScope'), right: literal(string('record:read')) }),
      primitive('both', 'and', { left: source('tenant'), right: source('role') }),
      primitive('scoped', 'and', { left: source('both'), right: source('scope') }),
      primitive('enabled', 'and', { left: source('scoped'), right: input('active') }),
    ], bindings: { permitted: source('enabled') },
  });
}
function identityPackage(name, type = 'integer', extraStep = false) {
  return manifest(name, { inputs: [port('value', type)], outputs: [port('result', type)], nodes: [primitive('echo', 'identity', { value: input('value') }), ...(extraStep ? [primitive('again', 'identity', { value: source('echo') })] : [])], bindings: { result: source(extraStep ? 'again' : 'echo') } });
}
function chain(depth) {
  let p = identityPackage(`chain_${depth}_1`);
  const dependencies = [];
  for (let i = 2; i <= depth; i++) {
    const child = lock(p); dependencies.push(child);
    p = manifest(`chain_${depth}_${i}`, { inputs: [port('value', 'integer')], outputs: [port('result', 'integer')], nodes: [call('child', ref(child), { value: input('value') })], bindings: { result: source('child') } });
  }
  return bundleOf(p, dependencies);
}
function tree(name, { leafSteps = 1, firstWidth = 2, type = 'integer' } = {}) {
  let p = leafSteps === 0
    ? manifest(`${name}_1`, { inputs: [port('value', type)], outputs: [port('result', type)], nodes: [], bindings: { result: input('value') } })
    : identityPackage(`${name}_1`, type, leafSteps === 2);
  const dependencies = [];
  for (let level = 2; level <= 8; level++) {
    const child = lock(p); dependencies.push(child);
    const width = level === 2 ? firstWidth : 2;
    const nodes = Array.from({ length: width }, (_, i) => call(`call_${i}`, ref(child), { value: input('value') }));
    let last = source('call_0');
    for (let i = 1; i < width; i++) {
      const id = `combine_${i}`;
      nodes.push(primitive(id, type === 'integer' ? 'add' : 'select', type === 'integer' ? { left: last, right: source(`call_${i}`) } : { condition: literal(boolean(true)), trueValue: last, falseValue: source(`call_${i}`) }));
      last = source(id);
    }
    p = manifest(`${name}_${level}`, { inputs: [port('value', type)], outputs: [port('result', type)], nodes, bindings: { result: last } });
  }
  return bundleOf(p, dependencies);
}

assert.equal((await request('/api/health')).version, '0.5.0');
for (const [template, version] of [['retry', 1], ['continuum', 2], ['branching', 3]]) {
  const project = await request(`/api/example?template=${template}`);
  assert.equal(project.schemaVersion, version);
  assert.deepEqual(await post('/api/validate', project), { valid: true, issues: [] });
}
const specs = await request('/api/components/primitives');
assert.deepEqual(specs.map(s => s.op).sort(), ['add', 'and', 'concat', 'equal', 'identity', 'less_or_equal', 'less_than', 'not', 'or', 'select', 'subtract'].sort());
assert.ok(specs.every(s => s.outputs.length === 1 && s.outputs[0].name === 'result'));
const examples = await request('/api/components/composite-examples');
const domains = [
  { id: 'example.health-record-access', role: 'clinician', scope: 'record:read' },
  { id: 'example.school-record-access', role: 'teacher', scope: 'student:read' },
].map(d => ({ ...d, bundle: examples.find(b => b.root.packageId === d.id) }));
domains.forEach(d => assert.ok(d.bundle, `missing example ${d.id}`));
for (const shared of ['lce.access-policy', 'lce.same-tenant']) {
  const locks = domains.map(d => d.bundle.packages.find(l => l.package.id === shared));
  assert.ok(locks.every(Boolean));
  assert.deepEqual(locks[0], locks[1]);
}
report('legacy project schemas remain intact and two domain wrappers share exact nested authorization definitions');

const operations = [
  ['identity', { value: literal(boolean(false)) }, boolean(false)],
  ['equal', { left: literal(integer(7)), right: literal(integer(7)) }, boolean(true)],
  ['less_than', { left: literal(integer(3)), right: literal(integer(4)) }, boolean(true)],
  ['less_or_equal', { left: literal(integer(4)), right: literal(integer(4)) }, boolean(true)],
  ['and', { left: literal(boolean(false)), right: literal(boolean(true)) }, boolean(false)],
  ['or', { left: literal(boolean(false)), right: literal(boolean(true)) }, boolean(true)],
  ['not', { value: literal(boolean(false)) }, boolean(true)],
  ['select', { condition: literal(boolean(false)), trueValue: literal(string('unused')), falseValue: literal(string('chosen')) }, string('chosen')],
  ['add', { left: literal(integer(40)), right: literal(integer(2)) }, integer(42)],
  ['subtract', { left: literal(integer(5)), right: literal(integer(8)) }, integer(-3)],
  ['concat', { left: literal(string('LCE')), right: literal(string(' Studio')) }, string('LCE Studio')],
];
const primitivePackage = manifest('primitive-reference', {
  inputs: [],
  outputs: [...operations.map(([op, _inputs, expected]) => port(op, expected.type)), port('zero', 'integer'), port('empty', 'string')],
  nodes: [...operations.map(([op, inputs]) => primitive(op, op, inputs)), primitive('zero', 'identity', { value: literal(integer(0)) }), primitive('empty', 'identity', { value: literal(string('')) })].reverse(),
  bindings: Object.fromEntries([...operations.map(([op]) => [op, source(op)]), ['zero', source('zero')], ['empty', source('empty')]]),
});
const primitiveBundle = await prepare(primitivePackage);
const primitiveRecord = await evaluate(primitiveBundle, {});
const expectedValues = Object.fromEntries([...operations.map(([op, _inputs, expected]) => [op, expected]), ['zero', integer(0)], ['empty', string('')]]);
assert.deepEqual(primitiveRecord.result.outputs, expectedValues);
assert.equal(primitiveRecord.result.operations, 13);
assert.equal(primitiveRecord.result.invocations, 1);
assert.equal(primitiveRecord.result.engineVersion, '0.5.0');
assert.equal(primitiveRecord.result.bundleHash, bundleHash(primitiveBundle));
assert.deepEqual(primitiveEvents(primitiveRecord).map(e => e.path), primitivePackage.behavior.nodes.map(n => `$/` + n.id).sort());
primitiveRecord.result.events.forEach((e, i) => assert.equal(e.sequence, i + 1));
assert.equal(primitiveRecord.result.events[0].path, '$');
assert.equal(primitiveRecord.result.events.at(-1).kind, 'component_exit');
assert.equal(primitiveRecord.result.durationMs, undefined);
assert.equal(primitiveRecord.result.infrastructure, undefined);
const constantPackage = manifest('constant', { inputs: [], outputs: [port('result', 'boolean')], nodes: [], bindings: { result: literal(boolean(false)) } });
const constant = await evaluate(await prepare(constantPackage), {});
assert.deepEqual(constant.result.outputs, { result: boolean(false) });
assert.equal(constant.result.operations, 0);
assert.equal(constant.result.invocations, 1);
assert.equal(constant.result.events.length, 2);
const caseSensitive = identityPackage('canonical-import');
caseSensitive.behavior.inputs[0].name = 'Payload';
caseSensitive.behavior.outputs[0].name = 'Result';
caseSensitive.behavior.nodes[0].id = 'Echo';
caseSensitive.behavior.nodes[0].inputs.value = input('Payload');
caseSensitive.behavior.bindings = { Result: source('Echo') };
const canonical = bundleOf(caseSensitive);
const alternateCasing = clone(canonical);
const variant = alternateCasing.packages[0].package;
variant.Behavior = variant.behavior; delete variant.behavior;
variant.Behavior.Nodes = variant.Behavior.nodes; delete variant.Behavior.nodes;
variant.Behavior.Inputs = variant.Behavior.inputs; delete variant.Behavior.inputs;
const normalized = await post('/api/components/validate-bundle', alternateCasing);
assert.equal(normalized.valid, true);
assert.deepEqual(normalized.bundle, canonical);
assert.equal(normalized.bundle.packages[0].package.Behavior, undefined);
assert.deepEqual((await evaluate(normalized.bundle, { Payload: integer(7) })).result.outputs, { Result: integer(7) });
report('all eleven primitives produce independent typed references, preserving false, zero and empty strings');

const decisions = new Map();
for (const domain of domains) {
  assert.deepEqual(await validate(domain.bundle), { valid: true, issues: [] });
  for (let mask = 0; mask < 16; mask++) {
    const flags = [0, 1, 2, 3].map(bit => !!(mask & (1 << bit)));
    const inputs = policyInputs(domain, flags);
    const record = await evaluate(domain.bundle, inputs);
    assert.deepEqual(record.result.outputs, { permitted: boolean(mask === 15) });
    assert.equal(record.result.operations, 6); // Three exact equalities and three ANDs.
    assert.equal(record.result.invocations, 3); // Wrapper, shared policy, tenant comparison.
    assert.ok(primitiveEvents(record).some(e => e.path.split('/').length === 4));
    decisions.set(`${domain.id}:${mask}`, record);
  }
}
const flat = await prepare(flatPolicy());
for (const mask of [0, 7, 11, 13, 14, 15]) {
  const inputs = policyInputs(domains[0], [0, 1, 2, 3].map(bit => !!(mask & (1 << bit))));
  const record = await evaluate(flat, inputs);
  assert.deepEqual(record.result.outputs, decisions.get(`${domains[0].id}:${mask}`).result.outputs);
  assert.equal(record.result.operations, 6);
  assert.equal(record.result.invocations, 1);
}
report('exhaustive four-predicate truth tables hold in both domains and match a separately authored flat policy');

const leafDefinition = identityPackage('instance-leaf');
leafDefinition.tests = [{ name: 'Dependency assertion is deliberately wrong', inputs: { value: integer(5) }, expected: { result: integer(999) } }];
const leaf = lock(leafDefinition);
const pairPackage = manifest('instance-pair', { inputs: [], outputs: [port('left', 'integer'), port('right', 'integer')], nodes: [call('a', ref(leaf), { value: literal(integer(5)) }), call('b', ref(leaf), { value: literal(integer(9)) })], bindings: { left: source('a'), right: source('b') } });
const pair = lock(pairPackage);
const nestedPackage = manifest('instance-root', { inputs: [], outputs: [port('left', 'integer'), port('right', 'integer')], nodes: [call('pair', ref(pair), {})], bindings: { left: source('pair', 'left'), right: source('pair', 'right') } }, [{ name: 'Root assertion only', inputs: {}, expected: { left: integer(5), right: integer(9) } }]);
const nested = await prepare(nestedPackage, [pair, leaf]);
const nestedRecord = await evaluate(nested, {});
assert.deepEqual(nestedRecord.result.outputs, { left: integer(5), right: integer(9) });
assert.equal(nestedRecord.result.operations, 2);
assert.equal(nestedRecord.result.invocations, 4);
assert.deepEqual(primitiveEvents(nestedRecord).map(e => e.path), ['$/pair/a/echo', '$/pair/b/echo']);
const nestedAgain = await evaluate(nested, {});
assert.deepEqual(nestedAgain.result, nestedRecord.result);
assert.deepEqual(nestedAgain.bundle, nestedRecord.bundle);
assert.notEqual(nestedAgain.id, nestedRecord.id);
const reorderedPackage = clone(primitivePackage);
reorderedPackage.behavior.nodes.reverse();
const reorderedRecord = await evaluate(await prepare(reorderedPackage), {});
assert.deepEqual(primitiveEvents(reorderedRecord), primitiveEvents(primitiveRecord));
assert.notEqual(reorderedRecord.result.bundleHash, primitiveRecord.result.bundleHash);
report('repeated nested calls retain independent inputs and qualified paths with deterministic depth-first evidence');

const inputs = policyInputs(domains[0]);
const missing = clone(inputs); delete missing.role;
const mistyped = clone(inputs); mistyped.active = string('true');
const mixed = clone(inputs); mixed.active = { type: 'boolean', boolean: false, integer: 0 };
const extra = { ...inputs, unexpected: boolean(true) };
for (const badInputs of [missing, mistyped, mixed, extra]) {
  const failure = await post('/api/components/evaluate', { bundle: domains[0].bundle, inputs: badInputs }, 422);
  assert.equal(failure.id, undefined);
}
const invalidTypedValue = clone(primitiveBundle);
const invalidNode = invalidTypedValue.packages[0].package.behavior.nodes.find(n => n.id === 'zero');
invalidNode.inputs.value.literal.integer = 9007199254740992;
const invalidRange = bundleOf(invalidTypedValue.packages[0].package);
await reject(invalidRange);
const mismatch = clone(primitivePackage);
mismatch.behavior.nodes.find(n => n.id === 'equal').inputs.right = literal(string('7'));
await reject(bundleOf(mismatch));
const wrongOutput = clone(primitivePackage);
wrongOutput.behavior.outputs.find(p => p.name === 'equal').type = 'string';
await reject(bundleOf(wrongOutput));
const missingArgument = clone(primitivePackage);
delete missingArgument.behavior.nodes.find(n => n.id === 'add').inputs.left;
await reject(bundleOf(missingArgument));
const mixedBinding = clone(primitivePackage);
mixedBinding.behavior.nodes.find(n => n.id === 'not').inputs.value.input = 'not_an_input';
await reject(bundleOf(mixedBinding));
const wrongCallType = clone(pairPackage);
wrongCallType.behavior.nodes[0].inputs.value = literal(string('5'));
await reject(bundleOf(wrongCallType, [leaf]));
const hiddenPort = clone(nestedPackage);
hiddenPort.behavior.bindings.left = source('pair', 'hidden');
await reject(bundleOf(hiddenPort, [pair, leaf]));
for (const field of ['inputs', 'nodes']) {
  const malformedCollection = clone(constantPackage);
  malformedCollection.behavior[field] = null;
  const invalid = await post('/api/components/prepare', { package: malformedCollection, dependencies: [] }, 422);
  assert.ok(invalid.issues?.some(issue => issue.path.includes(field)));
}
const nullCallInputs = clone(nestedPackage);
nullCallInputs.behavior.nodes[0].inputs = null;
await post('/api/components/prepare', { package: nullCallInputs, dependencies: [pair, leaf] }, 422);
report('missing, extra, mismatched and multiply represented values or bindings are rejected without coercion');

const max = 9007199254740991;
const overflowPackage = manifest('overflow', { inputs: [port('value', 'integer')], outputs: [port('result', 'integer')], nodes: [primitive('sum', 'add', { left: input('value'), right: literal(integer(1)) })], bindings: { result: source('sum') } }, [{ name: 'overflow becomes a failed case', inputs: { value: integer(max) }, expected: { result: integer(0) } }]);
const overflowBundle = await prepare(overflowPackage);
await post('/api/components/evaluate', { bundle: overflowBundle, inputs: { value: integer(max) } }, 422);
const eager = manifest('eager-select', { inputs: [], outputs: [port('result', 'integer')], nodes: [primitive('overflow', 'add', { left: literal(integer(max)), right: literal(integer(1)) }), primitive('choose', 'select', { condition: literal(boolean(false)), trueValue: source('overflow'), falseValue: literal(integer(0)) })], bindings: { result: source('choose') } });
await post('/api/components/evaluate', { bundle: await prepare(eager), inputs: {} }, 422);
const concatenate = manifest('concat-limit', { inputs: [port('value', 'string')], outputs: [port('result', 'string')], nodes: [primitive('join', 'concat', { left: input('value'), right: input('value') })], bindings: { result: source('join') } });
await post('/api/components/evaluate', { bundle: await prepare(concatenate), inputs: { value: string('x'.repeat(2049)) } }, 422);
report('arithmetic and string overflow fail explicitly, including an eager select input that is not selected');

for (const domain of domains) {
  const result = await test(domain.bundle);
  assert.equal(result.passed, true);
  assert.ok(result.cases.length >= 5);
  assert.ok(result.cases.every(c => c.passed && c.actual && !c.error));
}
const intentionalFailure = clone(flatPolicy());
intentionalFailure.tests = [{ name: 'deliberately incorrect expected decision', inputs, expected: { permitted: boolean(false) } }];
const failedAssertions = await test(await prepare(intentionalFailure));
assert.equal(failedAssertions.passed, false);
assert.equal(failedAssertions.cases[0].passed, false);
assert.deepEqual(failedAssertions.cases[0].actual, { permitted: boolean(true) });
assert.deepEqual(failedAssertions.cases[0].expected, { permitted: boolean(false) });
const executionFailure = await test(overflowBundle);
assert.equal(executionFailure.passed, false);
assert.equal(executionFailure.cases[0].passed, false);
assert.ok(executionFailure.cases[0].error);
assert.equal(executionFailure.cases[0].actual, undefined);
const rootTests = await test(nested);
assert.equal(rootTests.passed, true);
assert.deepEqual(rootTests.cases.map(c => c.name), ['Root assertion only']);
assert.deepEqual(await test(primitiveBundle), { passed: false, cases: [] });
report('tests distinguish verified cases, incorrect expectations, execution failures and an untested definition');

const cycle = identityPackage('local-cycle');
cycle.behavior.nodes = [primitive('a', 'identity', { value: source('b') }), primitive('b', 'identity', { value: source('a') })];
cycle.behavior.bindings.result = source('a');
await reject(bundleOf(cycle));
const unused = identityPackage('unused-node');
unused.behavior.nodes.push(primitive('unobservable', 'not', { value: literal(boolean(true)) }));
await reject(bundleOf(unused));
const tampered = clone(nested); tampered.packages[0].package.name = 'Unhashed edit';
await reject(tampered);
const absent = clone(nested); absent.packages = absent.packages.filter(l => l.package.id !== leaf.package.id);
await reject(absent);
const unreferenced = clone(nested); unreferenced.packages.push(lock(identityPackage('extra-definition')));
await reject(unreferenced);
const wrongRef = clone(nestedPackage); wrongRef.behavior.nodes[0].component.digest = '0'.repeat(64);
await reject(bundleOf(wrongRef, [pair, leaf]));
const recursive = identityPackage('dependency-cycle');
recursive.behavior.nodes = [call('self', { packageId: recursive.id, version: recursive.version, digest: '0'.repeat(64) }, { value: input('value') })];
recursive.behavior.bindings.result = source('self');
// A content-addressed dependency cycle cannot practically have consistent
// fixed-point digests; both the cyclic reference and its mismatch must reject.
await reject(bundleOf(recursive));
report('dataflow cycles, unused nodes, missing or extra dependencies and tampered exact locks are rejected');

const depthEight = chain(8);
assert.deepEqual(await validate(depthEight), { valid: true, issues: [] });
const deepRecord = await evaluate(depthEight, { value: integer(7) });
assert.deepEqual(deepRecord.result.outputs, { result: integer(7) });
assert.equal(deepRecord.result.invocations, 8);
await reject(chain(9));
const boundedTree = tree('bounded-tree');
const expanded = await evaluate(boundedTree, { value: integer(1) });
// Seven binary call levels:128 leaf identities +127 adds;255 invocations.
assert.deepEqual(expanded.result.outputs, { result: integer(128) });
assert.equal(expanded.result.operations, 255);
assert.equal(expanded.result.invocations, 255);
assert.equal(expanded.result.events.length, 765);
await reject(tree('too-many-primitives', { leafSteps: 2 })); // 383 primitives,255 calls.
await reject(tree('too-many-calls', { leafSteps: 0, firstWidth: 3 })); // 191 primitives,319 calls.
const oversizedEvidence = tree('evidence-limit', { type: 'string' });
assert.deepEqual(await validate(oversizedEvidence), { valid: true, issues: [] });
const tooLarge = await post('/api/components/evaluate', { bundle: oversizedEvidence, inputs: { value: string('x'.repeat(4096)) } }, 422);
assert.equal(tooLarge.id, undefined);
report('depth, repeated expansion counts and serialized evidence limits bound work before successful evidence is saved');

const legacyProfile = (await request('/api/components/examples'))[0].package;
legacyProfile.id = `${prefix}.legacy-profile`;
const installedLegacy = await post('/api/components', legacyProfile, 201);
assert.equal(installedLegacy.digest, sha(orderedPackage(legacyProfile)));
assert.equal(installedLegacy.package.behavior, undefined);
assert.equal(installedLegacy.package.tests, undefined);
assert.deepEqual(await post('/api/components', legacyProfile), installedLegacy);
await post('/api/components', { ...legacyProfile, behavior: constantPackage.behavior }, 422);
await post('/api/components', nestedPackage, 422);
const profileBundle = bundleOf(nestedPackage, [pair, leaf, { package: legacyProfile, digest: installedLegacy.digest }]);
await reject(profileBundle);
const oldProject = await request('/api/example?template=branching');
oldProject.packages.push(nested.packages[0]);
assert.equal((await post('/api/validate?draft=true', oldProject)).valid, false);
report('schema1 catalog digests remain compatible and composites cannot silently become timed application nodes');

const existingPackage = identityPackage('catalog-existing');
const existingBundle = await prepare(existingPackage);
const firstInstall = await install(existingBundle);
assert.equal(firstInstall.installed.length, 1);
assert.deepEqual(await install(existingBundle, 200), firstInstall);
const conflictingPackage = clone(existingPackage); conflictingPackage.name = 'Conflicting immutable version';
const conflictingLock = lock(conflictingPackage);
const freshLock = lock(identityPackage('catalog-new-child'));
const makeCatalogRoot = child => manifest('catalog-parent', { inputs: [], outputs: [port('result', 'integer')], nodes: [call('existing', ref(child), { value: literal(integer(5)) }), call('fresh', ref(freshLock), { value: literal(integer(7)) }), primitive('sum', 'add', { left: source('existing'), right: source('fresh') })], bindings: { result: source('sum') } });
const badInstall = await prepare(makeCatalogRoot(conflictingLock), [conflictingLock, freshLock]);
await install(badInstall, 409);
let catalog = await request('/api/components');
assert.ok(!catalog.some(r => r.package.id === freshLock.package.id));
assert.ok(!catalog.some(r => r.package.id === badInstall.root.packageId));
assert.deepEqual(catalog.find(r => r.package.id === existingPackage.id), firstInstall.installed[0]);
const goodInstall = await prepare(makeCatalogRoot(existingBundle.packages[0]), [existingBundle.packages[0], freshLock]);
const installed = await install(goodInstall);
assert.equal(installed.installed.length, 3);
assert.deepEqual(installed.installed.find(r => r.package.id === existingPackage.id), firstInstall.installed[0]);
const query = new URLSearchParams({ id: goodInstall.root.packageId, version: goodInstall.root.version, digest: goodInstall.root.digest });
const restored = await request(`/api/components/bundle?${query}`);
assert.deepEqual(restored.root, goodInstall.root);
sameLockSet(restored.packages, goodInstall.packages);
assert.deepEqual((await evaluate(restored, {})).result.outputs, { result: integer(12) });
query.set('digest', '0'.repeat(64));
await request(`/api/components/bundle?${query}`, { expected: 404 });
query.set('digest', 'not-a-digest');
await request(`/api/components/bundle?${query}`, { expected: 422 });
report('bundle installation is atomic on conflicts and reconstructs an exact immutable dependency closure');

const portable = JSON.parse(JSON.stringify(nested));
catalog = await request('/api/components');
assert.ok(!catalog.some(r => r.package.id === portable.root.packageId));
const portableRecord = await evaluate(portable, {});
assert.deepEqual(portableRecord.result, nestedRecord.result);
const modified = clone(nestedPackage);
modified.name = 'Later editor metadata'; modified.behavior.nodes[0].position.x = 50;
const changedBundle = await prepare(modified, [pair, leaf]);
const changedRecord = await evaluate(changedBundle, {});
assert.deepEqual(changedRecord.result.outputs, nestedRecord.result.outputs);
assert.notEqual(changedRecord.result.bundleHash, nestedRecord.result.bundleHash);
const detail = await request(`/api/components/evaluations/${portableRecord.id}`);
assert.deepEqual(detail, portableRecord);
assert.deepEqual(await request(`/api/components/evaluations/${nestedRecord.id}`), nestedRecord);
const history = await request('/api/components/evaluations');
assert.ok(history.length <= 50);
const summary = history.find(r => r.id === portableRecord.id);
assert.ok(summary);
assert.equal(summary.componentId, portable.root.packageId);
assert.equal(summary.componentVersion, portable.root.version);
assert.equal(summary.operations, 2);
assert.equal(summary.invocations, 4);
assert.equal(summary.bundle, undefined);
assert.equal(summary.result, undefined);
await request(`/api/components/evaluations/${portableRecord.id}`, { method: 'DELETE', expected: 405 });
report('uninstalled portable bundles reproduce outputs and immutable PostgreSQL evidence survives later definition edits');

console.log(`\n${checks} composite acceptance checks passed against ${base}.`);
