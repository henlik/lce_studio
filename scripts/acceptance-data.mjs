#!/usr/bin/env node
// Independent references for versioned data contracts and pure structured values.
// Use a disposable database: immutable definitions and validation evidence remain.
import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';

const base = (process.env.STUDIO_URL || 'http://127.0.0.1:8080').replace(/\/$/, '');
const prefix = `acceptance.${randomUUID()}`;
const clone = value => structuredClone(value);
let checks = 0;
const report = message => { checks++; console.log(`PASS ${message}`); };

async function request(path, { method = 'GET', body, raw, expected = 200 } = {}) {
  const response = await fetch(`${base}${path}`, {
    method, headers: body === undefined && raw === undefined ? {} : { 'Content-Type': 'application/json' },
    body: raw ?? (body === undefined ? undefined : JSON.stringify(body)), signal: AbortSignal.timeout(30000),
  });
  const text = await response.text();
  assert.ok((Array.isArray(expected) ? expected : [expected]).includes(response.status), `${method} ${path}: ${response.status}: ${text.slice(0, 1200)}`);
  assert.match(response.headers.get('content-type') || '', /application\/json/);
  return { status: response.status, data: JSON.parse(text) };
}
const get = async (path, expected = 200) => (await request(path, { expected })).data;
const post = async (path, body, expected = 200) => (await request(path, { method: 'POST', body, expected })).data;
const prepareData = contract => post('/api/data/prepare', contract);
const check = (contract, fixture) => post('/api/data/check', { contract, fixture });
const recordCheck = (contract, fixture) => post('/api/data/validate', { contract, fixture }, 201);
const compare = (old, candidate) => post('/api/data/compare', { old, new: candidate });
const prepare = (pkg, dependencies = []) => post('/api/components/prepare', { package: pkg, dependencies });
const evaluate = (bundle, inputs) => post('/api/components/evaluate', { bundle, inputs }, 201);
const v = (type, value) => ({ type, [type]: value });
const bool = value => v('boolean', value);
const int = value => v('integer', value);
const str = value => v('string', value);
const input = name => ({ input: name });
const source = (node, port = 'result') => ({ node, port });
const literal = value => ({ literal: value });
const scalarPort = (name, type) => ({ name, type });
const typedPort = (name, type, recordRef) => ({ name, type, recordRef: clone(recordRef) });
const node = (id, op, inputs, metadata = {}) => ({ id, op, inputs, position: { x: 0, y: 0 }, ...metadata });
const ref = (lock, record) => ({ contractId: lock.contract.id, version: lock.contract.version, digest: lock.digest, record });
const record = (recordRef, fields) => ({ type: 'record', recordRef: clone(recordRef), record: fields });
const collection = (recordRef, values) => ({ type: 'collection', recordRef: clone(recordRef), collection: values });
const rootPackage = bundle => bundle.packages.find(l => l.package.id === bundle.root.packageId && l.package.version === bundle.root.version).package;
const canonicalData = c => ({
  schemaVersion: c.schemaVersion, id: c.id, version: c.version, name: c.name,
  description: c.description, author: c.author, license: c.license,
  records: c.records.map(r => ({ name: r.name, fields: r.fields.map(f => ({ name: f.name, type: f.type })) })),
  entities: c.entities.map(e => ({ name: e.name, record: e.record, primaryKey: e.primaryKey, uniqueKeys: e.uniqueKeys, position: { x: e.position.x, y: e.position.y } })),
  relationships: c.relationships.map(r => ({ name: r.name, fromEntity: r.fromEntity, fromFields: r.fromFields, toEntity: r.toEntity, toFields: r.toFields, minSources: r.minSources, maxSources: r.maxSources })),
});
// Go's typed JSON keeps declaration order and escapes HTML-sensitive characters
// and Unicode line separators; these are content hashes, not sorted RFC8785 JSON.
const typedJSON = value => JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, char => `\\u${char.charCodeAt(0).toString(16).padStart(4, '0')}`);
const digestData = c => createHash('sha256').update(typedJSON(canonicalData(c))).digest('hex');
function contract(name, records, entities = [], relationships = []) {
  return { schemaVersion: 1, id: `${prefix}.${name}`, version: '1.0.0', name,
    description: 'Independent synthetic acceptance reference.', author: 'LCE acceptance', license: 'MIT', records, entities, relationships };
}
const fields = entries => entries.map(([name, type]) => ({ name, type }));
const entity = (name, recordName, primaryKey, uniqueKeys = []) => ({ name, record: recordName, primaryKey, uniqueKeys, position: { x: 0, y: 0 } });
const relation = (name, fromEntity, fromFields, toEntity, toFields, minSources = 0, maxSources = 0) => ({ name, fromEntity, fromFields, toEntity, toFields, minSources, maxSources });
function component(name, behavior, dataContracts = [], tests = []) {
  const zero = () => ({ default: 0, min: 0, max: 0 });
  return { schemaVersion: 3, id: `${prefix}.${name}`, version: '1.0.0', name, description: 'Independent structured behavior reference.', author: 'LCE acceptance', license: 'MIT', kernel: 'composite', capabilities: ['design', 'evaluate'], config: { processingMs: zero(), capacity: zero(), memoryMb: zero(), deduplicate: false }, behavior, ...(tests.length ? { tests } : {}), ...(dataContracts.length ? { dataContracts } : {}) };
}
function assertInvalid(result, needle) {
  assert.equal(result.valid, false);
  assert.ok(result.issues.length > 0);
  if (needle) assert.ok(result.issues.some(i => `${i.path} ${i.message}`.toLowerCase().includes(needle.toLowerCase())), JSON.stringify(result));
}

assert.equal((await get('/api/health')).version, '0.8.1');
assert.equal((await get('/api/components/primitives')).length, 11);
const structuredPrimitives = await get('/api/components/structured-primitives');
assert.equal(structuredPrimitives.length, 16);
for (const op of ['record_make', 'record_get', 'collection_count', 'collection_at', 'collection_append']) assert.ok(structuredPrimitives.some(p => p.op === op));
const dataExamples = await get('/api/data/examples');
assert.equal(dataExamples.length, 2);
assert.deepEqual(dataExamples[0].contract, dataExamples[1].contract);
const sharedData = dataExamples[0].contract;
assert.equal(sharedData.contract.id, 'lce.organization-registry');
assert.equal(sharedData.digest, digestData(sharedData.contract));
for (const example of dataExamples) {
  const checked = await check(example.contract, example.fixture);
  assert.equal(checked.result.valid, true);
  assert.equal(checked.result.engineVersion, '0.5.0');
  assert.deepEqual(checked.fixture, example.fixture);
  assert.equal(checked.result.rowCount, Object.values(example.fixture.rows).reduce((n, rows) => n + rows.length, 0));
}
report('both domain datasets reuse one exact data contract and legacy primitive discovery remains stable');

const registryDefinition = contract('registry', [
  { name: 'Organization', fields: fields([['id', 'string'], ['name', 'string']]) },
  { name: 'Member', fields: fields([['id', 'string'], ['org', 'string'], ['badge', 'integer'], ['active', 'boolean']]) },
  { name: 'Profile', fields: fields([['id', 'string'], ['member', 'string']]) },
  { name: 'Course', fields: fields([['id', 'string'], ['title', 'string']]) },
  { name: 'Enrollment', fields: fields([['member', 'string'], ['course', 'string']]) },
  { name: 'Pair', fields: fields([['left', 'string'], ['right', 'string']]) },
], [
  entity('Organizations', 'Organization', ['id'], [['name']]), entity('Members', 'Member', ['id'], [['org', 'badge']]),
  entity('Profiles', 'Profile', ['id']), entity('Courses', 'Course', ['id']), entity('Enrollments', 'Enrollment', ['member', 'course']), entity('Pairs', 'Pair', ['left', 'right']),
], [
  relation('MemberOrganization', 'Members', ['org'], 'Organizations', ['id'], 1, 0),
  relation('ProfileMember', 'Profiles', ['member'], 'Members', ['id'], 0, 1),
  relation('EnrollmentMember', 'Enrollments', ['member'], 'Members', ['id']),
  relation('EnrollmentCourse', 'Enrollments', ['course'], 'Courses', ['id']),
]);
const registry = await prepareData(registryDefinition);
assert.deepEqual(registry.contract, registryDefinition);
assert.equal(registry.digest, digestData(registryDefinition));
const fixture = { name: 'Complete registry reference', rows: {
  Organizations: [{ id: str('org-a'), name: str('Alpha') }, { id: str('org-b'), name: str('Beta') }],
  Members: [{ id: str('m1'), org: str('org-a'), badge: int(0), active: bool(false) }, { id: str('m2'), org: str('org-a'), badge: int(2), active: bool(true) }, { id: str('m3'), org: str('org-b'), badge: int(0), active: bool(true) }],
  Profiles: [{ id: str('p1'), member: str('m1') }],
  Courses: [{ id: str('c1'), title: str('First') }, { id: str('c2'), title: str('Second') }],
  Enrollments: [{ member: str('m1'), course: str('c1') }, { member: str('m1'), course: str('c2') }, { member: str('m2'), course: str('c1') }],
  Pairs: [{ left: str('a|b'), right: str('c') }, { left: str('a'), right: str('b|c') }],
} };
const validRegistry = await check(registry, fixture);
assert.equal(validRegistry.result.valid, true);
assert.equal(validRegistry.result.entityCount, 6);
assert.equal(validRegistry.result.rowCount, 13);
assert.equal(validRegistry.result.issuesTruncated, false);
const missingOptionalEntity = clone(fixture); delete missingOptionalEntity.rows.Profiles;
assert.equal((await check(registry, missingOptionalEntity)).result.valid, true);
const noRows = await check(registry, { name: 'Empty complete snapshot', rows: {} });
assert.equal(noRows.result.valid, true); // Reverse minimum applies to target rows; there are none.
assert.equal(noRows.result.rowCount, 0);
report('complete fixtures enforce typed compound keys and explicit many-to-many joins without display-string collisions');

const mutations = [
  ['duplicate primary key', f => f.rows.Members.push(clone(f.rows.Members[0])), 'Members'],
  ['duplicate compound unique key', f => { f.rows.Members[1].badge = int(0); }, 'Members'],
  ['dangling foreign key', f => { f.rows.Members[1].org = str('missing'); }, 'MemberOrganization'],
  ['required reverse cardinality', f => f.rows.Organizations.push({ id: str('unreferenced'), name: str('Unused') }), 'MemberOrganization'],
  ['one-to-one reverse maximum', f => f.rows.Profiles.push({ id: str('p2'), member: str('m1') }), 'ProfileMember'],
  ['duplicate join tuple', f => f.rows.Enrollments.push(clone(f.rows.Enrollments[0])), 'Enrollments'],
  ['dangling join endpoint', f => { f.rows.Enrollments[0].course = str('missing'); }, 'EnrollmentCourse'],
  ['missing required field', f => { delete f.rows.Members[0].active; }, 'active'],
  ['extra record field', f => { f.rows.Members[0].unexpected = int(1); }, 'unexpected'],
  ['wrong key scalar type', f => { f.rows.Members[0].id = int(1); }, 'id'],
  ['mixed typed value', f => { f.rows.Members[0].active = { type: 'boolean', boolean: false, integer: 0 }; }, 'active'],
  ['unknown entity', f => { f.rows.Unknown = []; }, 'Unknown'],
  ['null row array', f => { f.rows.Profiles = null; }, 'Profiles'],
  ['null row', f => { f.rows.Members[0] = null; }, 'Members'],
];
for (const [name, mutate, needle] of mutations) {
  const changed = clone(fixture); changed.name = name; mutate(changed);
  assertInvalid((await check(registry, changed)).result, needle);
}
const repeatFailure = clone(fixture); repeatFailure.rows.Members[0].org = str('missing');
assert.deepEqual((await check(registry, repeatFailure)).result, (await check(registry, repeatFailure)).result);
report('invalid identity, required references, one-to-one bounds and record shape produce deterministic diagnostic paths');

const invalidDefinitions = [];
const wrongFK = clone(registryDefinition); wrongFK.relationships[0].fromFields = ['badge']; invalidDefinitions.push(wrongFK);
const notAKey = clone(registryDefinition); notAKey.relationships[1].toFields = ['org']; invalidDefinitions.push(notAKey);
const repeatedKeyField = clone(registryDefinition); repeatedKeyField.entities[0].primaryKey = ['id', 'id']; invalidDefinitions.push(repeatedKeyField);
const duplicateKey = clone(registryDefinition); duplicateKey.entities[0].uniqueKeys.push(['id']); invalidDefinitions.push(duplicateKey);
const unknownRecord = clone(registryDefinition); unknownRecord.entities[0].record = 'Absent'; invalidDefinitions.push(unknownRecord);
const optionalType = clone(registryDefinition); optionalType.records[0].fields[0].type = 'optional'; invalidDefinitions.push(optionalType);
const invalidCardinality = clone(registryDefinition); invalidCardinality.relationships[0].minSources = 2; invalidDefinitions.push(invalidCardinality);
const nullRecords = clone(registryDefinition); nullRecords.records = null; invalidDefinitions.push(nullRecords);
const nullKeys = clone(registryDefinition); nullKeys.entities[0].uniqueKeys = null; invalidDefinitions.push(nullKeys);
for (const invalid of invalidDefinitions) await post('/api/data/prepare', invalid, 422);
const selfDefinition = contract('self-reference', [{ name: 'Entry', fields: fields([['id', 'string'], ['parent', 'string']]) }], [entity('Entries', 'Entry', ['id'])], [relation('Parent', 'Entries', ['parent'], 'Entries', ['id'])]);
const selfLock = await prepareData(selfDefinition);
assert.equal((await check(selfLock, { name: 'Finite self-reference', rows: { Entries: [{ id: str('self'), parent: str('self') }] } })).result.valid, true);
report('data definitions reject malformed constraints while finite self-referential relationships remain valid');

const alternate = clone(registry);
alternate.Contract = alternate.contract; delete alternate.contract;
alternate.Contract.Records = alternate.Contract.records; delete alternate.Contract.records;
alternate.Contract.Records[0].Fields = alternate.Contract.Records[0].fields; delete alternate.Contract.Records[0].fields;
const canonicalLock = await post('/api/data/validate-lock', alternate);
assert.equal(canonicalLock.valid, true);
assert.deepEqual(canonicalLock.lock, registry);
const fixtureAlias = clone(fixture); fixtureAlias.Rows = fixtureAlias.rows; delete fixtureAlias.rows;
const canonicalFixture = await check(registry, fixtureAlias);
assert.deepEqual(canonicalFixture.fixture, fixture);
const escapedDefinition = clone(registryDefinition); escapedDefinition.description = 'A < B & C > D\u2028data\u2029contract';
const escapedLock = await prepareData(escapedDefinition);
assert.equal(escapedLock.digest, digestData(escapedDefinition));
const tampered = clone(registry); tampered.contract.name += ' changed';
assertInvalid(await post('/api/data/validate-lock', tampered));
assert.equal((await post('/api/data/validate-lock', tampered)).lock, undefined);
await post('/api/data/check', { contract: tampered, fixture }, 422);
await post('/api/data/prepare', { ...registryDefinition, unexpected: true }, 400);
await request('/api/data/prepare', { method: 'POST', raw: `${JSON.stringify(registryDefinition)} {}`, expected: 400 });
report('raw contract and fixture imports use the canonical validated representation and exact content digests');

const changedLock = async modify => { const c = clone(registryDefinition); c.version = '1.0.1'; modify(c); return prepareData(c); };
const compareCases = [
  ['metadata', c => { c.description = 'Revised documentation'; c.entities[0].position.x = 250; }, true],
  ['definition order', c => { c.records.reverse(); c.entities.reverse(); c.relationships.reverse(); c.records.forEach(r => r.fields.reverse()); }, true],
  ['unused record', c => c.records.push({ name: 'Extra', fields: fields([['id', 'string']]) }), true],
  ['unused entity', c => c.entities.push(entity('ExtraOrganizations', 'Organization', ['id'])), true],
  ['required field addition', c => c.records[0].fields.push({ name: 'code', type: 'string' }), false],
  ['required field removal', c => { c.records[3].fields = c.records[3].fields.filter(f => f.name !== 'title'); }, false],
  ['field type change', c => { c.records[3].fields.find(f => f.name === 'title').type = 'integer'; }, false],
  ['record removal', c => { c.records = c.records.filter(r => r.name !== 'Pair'); c.entities = c.entities.filter(e => e.name !== 'Pairs'); }, false],
  ['constraint removal', c => { c.relationships = c.relationships.filter(r => r.name !== 'MemberOrganization'); }, true],
  ['minimum relaxation', c => { c.relationships[0].minSources = 0; }, true],
  ['maximum tightening', c => { c.relationships[0].maxSources = 1; }, false],
  ['minimum tightening', c => { c.relationships[1].minSources = 1; }, false],
];
for (const [name, modify, compatible] of compareCases) {
  const candidate = await changedLock(modify);
  const result = await compare(registry, candidate);
  assert.equal(result.direction, 'old-to-new');
  assert.equal(result.compatible, compatible, `${name}: ${JSON.stringify(result)}`);
  assert.ok(result.changes.length > 0);
  if (!compatible) assert.ok(result.changes.some(c => c.kind === 'breaking'));
}
const relaxed = await changedLock(c => { c.relationships[0].minSources = 0; });
assert.equal((await compare(relaxed, registry)).compatible, false);
await post('/api/data/compare', { old: registry, new: selfLock }, 422);
report('compatibility comparison is directional and distinguishes metadata, required-field changes and cardinality relaxation');

const smallDefinition = contract('bounded', [{ name: 'Item', fields: fields([['id', 'string']]) }], [entity('Items', 'Item', ['id'])]);
const small = await prepareData(smallDefinition);
const rows256 = { name: 'Bounded rows', rows: { Items: Array.from({ length: 256 }, (_, i) => ({ id: str(String(i)) })) } };
assert.equal((await check(small, rows256)).result.valid, true);
const rows257 = clone(rows256); rows257.rows.Items.push({ id: str('256') });
await post('/api/data/check', { contract: small, fixture: rows257 }, 422);
const lotsOfIssues = { name: 'Bounded diagnostics', rows: { Items: Array.from({ length: 256 }, () => ({})) } };
const truncated = (await check(small, lotsOfIssues)).result;
assert.equal(truncated.valid, false); assert.equal(truncated.rowCount, 256);
assert.equal(truncated.issuesTruncated, true); assert.ok(truncated.issues.length > 0 && truncated.issues.length <= 200);
const largeRows = { name: 'Byte budget', rows: { Items: Array.from({ length: 130 }, (_, i) => ({ id: str(String(i).padStart(4096, 'x')) })) } };
await post('/api/data/check', { contract: small, fixture: largeRows }, 422);
report('fixture row, byte and diagnostic budgets are enforced without turning truncated errors into a passing verdict');

const counterDefinition = contract('counter-shape', [{ name: 'Counter', fields: fields([['id', 'string'], ['n', 'integer'], ['enabled', 'boolean']]) }]);
const counter = await prepareData(counterDefinition), counterRef = ref(counter, 'Counter');
const a = record(counterRef, { id: str('a'), n: int(1), enabled: bool(true) });
const b = record(counterRef, { enabled: bool(false), n: int(2), id: str('b') });
const subject = record(counterRef, { id: str('s'), n: int(7), enabled: bool(true) });
const group = collection(counterRef, [a, b]);
const opsPackage = component('structured-reference', {
  inputs: [typedPort('subject', 'record', counterRef), typedPort('group', 'collection', counterRef)],
  outputs: [typedPort('made', 'record', counterRef), scalarPort('field', 'integer'), scalarPort('count', 'integer'), typedPort('at', 'record', counterRef), typedPort('appended', 'collection', counterRef), scalarPort('equal', 'boolean'), typedPort('selected', 'record', counterRef), typedPort('copied', 'collection', counterRef)],
  nodes: [
    node('get', 'record_get', { value: input('subject') }, { field: 'n' }),
    node('make', 'record_make', { id: literal(str('new')), n: source('get'), enabled: literal(bool(false)) }, { recordRef: counterRef }),
    node('count', 'collection_count', { values: input('group') }),
    node('at', 'collection_at', { values: input('group'), index: literal(int(1)) }),
    node('append', 'collection_append', { values: input('group'), value: input('subject') }),
    node('same', 'equal', { left: input('subject'), right: literal(record(counterRef, { enabled: bool(true), n: int(7), id: str('s') })) }),
    node('select', 'select', { condition: literal(bool(false)), trueValue: input('subject'), falseValue: source('make') }),
    node('copy', 'identity', { value: input('group') }),
  ].reverse(),
  bindings: { made: source('make'), field: source('get'), count: source('count'), at: source('at'), appended: source('append'), equal: source('same'), selected: source('select'), copied: source('copy') },
}, [counter]);
const ops = await prepare(opsPackage);
const outputRecord = await evaluate(ops, { subject, group });
const made = record(counterRef, { id: str('new'), n: int(7), enabled: bool(false) });
assert.deepEqual(outputRecord.result.outputs, { made, field: int(7), count: int(2), at: b, appended: collection(counterRef, [a, b, subject]), equal: bool(true), selected: made, copied: group });
assert.equal(outputRecord.result.operations, 8); assert.equal(outputRecord.result.invocations, 1);
assert.equal(outputRecord.result.engineVersion, '0.5.0');
assert.deepEqual(outputRecord.inputs, { subject, group });
assert.deepEqual((await evaluate(ops, { subject, group })).result, outputRecord.result);
report('all five structured operations and generic identity, equality and selection match independent typed references');

const equalityPackage = (name, type) => component(name, { inputs: [typedPort('left', type, counterRef), typedPort('right', type, counterRef)], outputs: [scalarPort('result', 'boolean')], nodes: [node('same', 'equal', { left: input('left'), right: input('right') })], bindings: { result: source('same') } }, [counter]);
const recordEquality = await prepare(equalityPackage('record-equality', 'record'));
const listEquality = await prepare(equalityPackage('collection-equality', 'collection'));
assert.deepEqual((await evaluate(recordEquality, { left: a, right: record(counterRef, { enabled: bool(true), n: int(1), id: str('a') }) })).result.outputs, { result: bool(true) });
assert.deepEqual((await evaluate(recordEquality, { left: a, right: b })).result.outputs, { result: bool(false) });
assert.deepEqual((await evaluate(listEquality, { left: group, right: collection(counterRef, [b, a]) })).result.outputs, { result: bool(false) });
assert.deepEqual((await evaluate(listEquality, { left: collection(counterRef, []), right: collection(counterRef, []) })).result.outputs, { result: bool(true) });
const sameShape = clone(counterDefinition); sameShape.id += '-other';
const otherCounter = await prepareData(sameShape), otherRef = ref(otherCounter, 'Counter');
const mismatch = equalityPackage('nominal-mismatch', 'record');
mismatch.dataContracts.push(otherCounter); mismatch.behavior.inputs[1].recordRef = otherRef;
await post('/api/components/prepare', { package: mismatch, dependencies: [] }, 422);
const nominalInputs = { subject: record(otherRef, clone(subject.record)), group };
await post('/api/components/evaluate', { bundle: ops, inputs: nominalInputs }, 422);
report('record equality ignores field order, collection equality preserves element order, and equal-shaped nominal contracts remain distinct');

const scalarSamples = { id: str(''), n: int(0), enabled: bool(false) };
assert.equal((await post('/api/data/values', { contracts: [counter], values: { Value: record(counterRef, scalarSamples), Empty: collection(counterRef, []) } })).valid, true);
const invalidValues = [];
const noField = clone(subject); delete noField.record.n; invalidValues.push(noField);
const extraField = clone(subject); extraField.record.extra = str('x'); invalidValues.push(extraField);
const nullFields = clone(subject); nullFields.record = null; invalidValues.push(nullFields);
const nullScalar = clone(subject); nullScalar.record.n = null; invalidValues.push(nullScalar);
const mixedRecord = clone(subject); mixedRecord.string = 'x'; invalidValues.push(mixedRecord);
const wrongType = clone(subject); wrongType.record.n = str('7'); invalidValues.push(wrongType);
const unsafeInteger = clone(subject); unsafeInteger.record.n = int(9007199254740992); invalidValues.push(unsafeInteger);
const nestedRecord = clone(subject); nestedRecord.record.n = clone(a); invalidValues.push(nestedRecord);
const nullList = collection(counterRef, []); nullList.collection = null; invalidValues.push(nullList);
const wrongListMember = collection(counterRef, [record(otherRef, clone(a.record))]); invalidValues.push(wrongListMember);
for (const invalid of invalidValues) {
  const response = await post('/api/data/values', { contracts: [counter, otherCounter], values: { Value: invalid } });
  assertInvalid(response); assert.equal(response.values, undefined);
}
const canonicalValues = { contracts: [counter], values: { Payload: clone(subject) } };
canonicalValues.values.Payload.Record = canonicalValues.values.Payload.record; delete canonicalValues.values.Payload.record;
const normalized = await post('/api/data/values', canonicalValues);
assert.equal(normalized.valid, true); assert.deepEqual(normalized.values, { Payload: subject });
const fractionRaw = JSON.stringify({ contracts: [counter], values: { Value: subject } }).replace('"integer":7', '"integer":9007199254740990.5');
await request('/api/data/values', { method: 'POST', raw: fractionRaw, expected: 400 });
report('structured values preserve false, zero and empty collections while rejecting coercion, null, missing and extra fields');

const indexPackage = component('index', { inputs: [typedPort('group', 'collection', counterRef), scalarPort('index', 'integer')], outputs: [typedPort('result', 'record', counterRef)], nodes: [node('at', 'collection_at', { values: input('group'), index: input('index') })], bindings: { result: source('at') } }, [counter]);
const indexed = await prepare(indexPackage);
for (const index of [-1, 2, 9007199254740991]) await post('/api/components/evaluate', { bundle: indexed, inputs: { group, index: int(index) } }, 422);
await post('/api/components/evaluate', { bundle: indexed, inputs: { group: collection(counterRef, []), index: int(0) } }, 422);
const tinyRef = ref(small, 'Item');
const tinyItems = Array.from({ length: 128 }, (_, i) => record(tinyRef, { id: str(String(i)) }));
assert.equal((await post('/api/data/values', { contracts: [small], values: { Values: collection(tinyRef, tinyItems) } })).valid, true);
assertInvalid(await post('/api/data/values', { contracts: [small], values: { Values: collection(tinyRef, [...tinyItems, tinyItems[0]]) } }));
const appendPackage = component('append-bound', { inputs: [typedPort('group', 'collection', tinyRef)], outputs: [typedPort('result', 'collection', tinyRef)], nodes: [node('append', 'collection_append', { values: input('group'), value: literal(tinyItems[0]) })], bindings: { result: source('append') } }, [small]);
await post('/api/components/evaluate', { bundle: await prepare(appendPackage), inputs: { group: collection(tinyRef, tinyItems) } }, 422);
const badField = clone(indexPackage); badField.behavior.nodes[0] = node('at', 'record_get', { value: literal(subject) }, { field: 'absent' });
badField.behavior.outputs = [scalarPort('result', 'integer')];
await post('/api/components/prepare', { package: badField, dependencies: [] }, 422);
const forbiddenMetadata = clone(opsPackage); forbiddenMetadata.behavior.nodes.find(n => n.id === 'copy').field = 'n';
await post('/api/components/prepare', { package: forbiddenMetadata, dependencies: [] }, 422);
report('collection indexing, append capacity, schema-derived field access and operation metadata fail explicitly at their boundaries');

const bigFields = Array.from({ length: 16 }, (_, i) => [`f${i}`, 'string']);
const wide = await prepareData(contract('wide', [{ name: 'Wide', fields: fields(bigFields) }]));
const wideRef = ref(wide, 'Wide');
const wideValue = size => record(wideRef, Object.fromEntries(bigFields.map(([name]) => [name, str('x'.repeat(size))])));
assertInvalid(await post('/api/data/values', { contracts: [wide], values: { Value: wideValue(4096) } }));
const amplification = component('trace-amplification', {
  inputs: [typedPort('value', 'record', wideRef)], outputs: [typedPort('result', 'record', wideRef)],
  nodes: Array.from({ length: 32 }, (_, i) => node(`step${i}`, 'identity', { value: i === 0 ? input('value') : source(`step${i - 1}`) })), bindings: { result: source('step31') },
}, [wide]);
const amplified = await prepare(amplification);
const beforeFailure = await get('/api/components/evaluations');
await post('/api/components/evaluate', { bundle: amplified, inputs: { value: wideValue(1024) } }, 422);
assert.deepEqual(await get('/api/components/evaluations'), beforeFailure);
const mapBound = component('map-budget', { inputs: Array.from({ length: 5 }, (_, i) => typedPort(`value${i}`, 'record', wideRef)), outputs: [typedPort('result', 'record', wideRef)], nodes: [], bindings: { result: input('value0') } }, [wide]);
const mapValues = Object.fromEntries(Array.from({ length: 5 }, (_, i) => [`value${i}`, wideValue(3800)]));
await post('/api/components/evaluate', { bundle: await prepare(mapBound), inputs: mapValues }, 422);
report('structured value, public-map and amplified trace byte budgets reject oversized work without saved success');

const structuredExamples = await get('/api/components/structured-examples');
assert.equal(structuredExamples.length, 2);
for (const bundle of structuredExamples) {
  const root = rootPackage(bundle);
  assert.equal(root.schemaVersion, 3);
  assert.ok(root.dataContracts.some(lock => lock.digest === sharedData.digest));
  const baseline = clone(root.tests[0].inputs);
  for (let mask = 0; mask < 16; mask++) {
    const values = clone(baseline);
    values.subject.record.tenantId = str(mask & 1 ? values.resource.record.tenantId.string : 'different');
    if (!(mask & 2)) values.subject.record.role = str('visitor');
    if (!(mask & 4)) values.resource.record.scope = str('unrelated:write');
    values.subject.record.active = bool(!!(mask & 8));
    const outcome = await evaluate(bundle, values);
    assert.deepEqual(outcome.result.outputs, { permitted: bool(mask === 15) });
    assert.equal(outcome.result.operations, 11); assert.equal(outcome.result.invocations, 3);
    assert.ok(outcome.result.events.some(e => e.path.includes('/tenant/compare')));
  }
  const tests = await post('/api/components/test', { bundle });
  assert.equal(tests.passed, true); assert.ok(tests.cases.length >= 5);
  const badInput = clone(baseline); delete badInput.subject.record.role;
  await post('/api/components/evaluate', { bundle, inputs: badInput }, 422);
}
report('structured healthcare and education policies match all sixteen independent access-condition combinations using unchanged scalar dependencies');

const oldComposite = clone(rootPackage((await get('/api/components/composite-examples'))[0]));
oldComposite.id = `${prefix}.old-scalar`;
oldComposite.dataContracts = [counter];
await post('/api/components/prepare', { package: oldComposite, dependencies: [] }, 422);
const oldStructured = clone(opsPackage); oldStructured.schemaVersion = 2;
await post('/api/components/prepare', { package: oldStructured, dependencies: [] }, 422);
const legacyProfile = clone((await get('/api/components/examples'))[0].package);
legacyProfile.id = `${prefix}.old-profile`; legacyProfile.dataContracts = [counter];
await post('/api/components', legacyProfile, 422);
const project = await get('/api/example?template=branching');
project.packages.push(ops.packages[0]);
assert.equal((await post('/api/validate?draft=true', project)).valid, false);
const missingContract = clone(opsPackage); delete missingContract.dataContracts;
await post('/api/components/prepare', { package: missingContract, dependencies: [] }, 422);
const changedCounterDefinition = clone(counterDefinition); changedCounterDefinition.name += ' conflict';
const conflictingCounter = await prepareData(changedCounterDefinition);
const conflictingLocks = clone(opsPackage); conflictingLocks.dataContracts.push(conflictingCounter);
await post('/api/components/prepare', { package: conflictingLocks, dependencies: [] }, 422);
// Resolve records from the embedded union, including locks declared by a child.
// Unused embedded definitions still count toward the exact bundle-wide bound.
const spareLocks = Array.from({ length: 39 }, (_, i) => {
  const definition = contract(`spare-${i}`, [{ name: 'Item', fields: fields([['id', 'string']]) }]);
  return { contract: definition, digest: digestData(definition) };
});
const lockGroups = [[counter, ...spareLocks.slice(0, 7)], ...Array.from({ length: 4 }, (_, i) => spareLocks.slice(7 + i * 8, 15 + i * 8))];
const dataWrapper = (name, locks, child) => component(name, {
  inputs: [typedPort('value', 'record', counterRef)], outputs: [typedPort('result', 'record', counterRef)],
  nodes: child ? [{ id: 'child', component: clone(child.root), inputs: { value: input('value') }, position: { x: 0, y: 0 } }] : [],
  bindings: { result: child ? source('child') : input('value') },
}, locks);
let contractChain = await prepare(dataWrapper('data-union-0', lockGroups[0]));
for (let i = 1; i < 4; i++) contractChain = await prepare(dataWrapper(`data-union-${i}`, lockGroups[i], contractChain), contractChain.packages);
assert.deepEqual((await evaluate(contractChain, { value: subject })).result.outputs, { result: subject });
const repeatedData = await prepare(dataWrapper('repeated-exact-data', [counter], contractChain), contractChain.packages);
assert.deepEqual((await evaluate(repeatedData, { value: subject })).result.outputs, { result: subject });
await post('/api/components/prepare', { package: dataWrapper('too-many-data', lockGroups[4], contractChain), dependencies: contractChain.packages }, 422);
await post('/api/components/prepare', { package: dataWrapper('cross-package-conflict', [conflictingCounter], contractChain), dependencies: contractChain.packages }, 422);
await post('/api/components/prepare', { package: dataWrapper('too-many-manifest-locks', [counter, ...spareLocks.slice(0, 8)]), dependencies: [] }, 422);
report('old schemas reject structured additions while exact embedded unions enforce nominal identity and manifest/bundle lock limits');

const installedResponses = await Promise.all([request('/api/data/contracts', { method: 'POST', body: registry, expected: [200, 201] }), request('/api/data/contracts', { method: 'POST', body: registry, expected: [200, 201] })]);
assert.deepEqual(installedResponses.map(r => r.status).sort(), [200, 201]);
assert.deepEqual(installedResponses[0].data, installedResponses[1].data);
const installed = installedResponses[0].data;
assert.deepEqual(await post('/api/data/contracts', registry), installed);
const conflictDefinition = clone(registryDefinition); conflictDefinition.name += ' changed';
await post('/api/data/contracts', await prepareData(conflictDefinition), 409);
const query = new URLSearchParams({ id: registry.contract.id, version: registry.contract.version, digest: registry.digest });
assert.deepEqual(await get(`/api/data/contract?${query}`), installed);
query.set('digest', '0'.repeat(64)); await get(`/api/data/contract?${query}`, 404);
query.set('digest', 'bad'); await get(`/api/data/contract?${query}`, 422);
const raceDefinition = clone(counterDefinition); raceDefinition.id = `${prefix}.catalog-race`;
const raceA = await prepareData(raceDefinition); raceDefinition.name += ' alternate'; const raceB = await prepareData(raceDefinition);
const race = await Promise.all([raceA, raceB].map(body => request('/api/data/contracts', { method: 'POST', body, expected: [201, 409] })));
assert.deepEqual(race.map(r => r.status).sort(), [201, 409]);
const catalog = await get('/api/data/contracts');
const winner = race.find(r => r.status === 201).data;
assert.deepEqual(catalog.find(r => r.contract.id === winner.contract.id), winner);
assert.ok(!catalog.some(r => r.contract.id === counter.contract.id));
report('concurrent data catalog installs preserve exact immutable versions and independent portable definitions');

const savedValid = await recordCheck(registry, fixture);
const invalidFixture = clone(fixture); invalidFixture.rows.Members[1].org = str('unknown');
const savedInvalid = await recordCheck(registry, invalidFixture);
assert.equal(savedValid.result.valid, true); assert.equal(savedInvalid.result.valid, false);
assert.deepEqual(savedValid.fixture, fixture); assert.deepEqual(savedInvalid.fixture, invalidFixture);
const newer = await changedLock(c => { c.description = 'Later version retained separately'; });
await post('/api/data/contracts', newer, 201);
assert.deepEqual(await get(`/api/data/validations/${savedValid.id}`), savedValid);
assert.deepEqual(await get(`/api/data/validations/${savedInvalid.id}`), savedInvalid);
const history = await get('/api/data/validations');
assert.ok(history.length <= 50);
const summary = history.find(r => r.id === savedInvalid.id);
assert.ok(summary); assert.equal(summary.valid, false); assert.equal(summary.rowCount, 13);
assert.equal(summary.contractId, registry.contract.id); assert.equal(summary.contractVersion, '1.0.0');
assert.equal(summary.fixture, undefined); assert.equal(summary.result, undefined);
const beforeOversize = await get('/api/data/validations');
await post('/api/data/validate', { contract: small, fixture: rows257 }, 422);
assert.deepEqual(await get('/api/data/validations'), beforeOversize);
await request(`/api/data/validations/${savedValid.id}`, { method: 'DELETE', expected: 405 });
assert.deepEqual(await get(`/api/components/evaluations/${outputRecord.id}`), outputRecord);
report('valid and invalid fixture verdicts retain immutable PostgreSQL evidence while malformed or oversized work creates no record');

console.log(`\n${checks} data acceptance checks passed against ${base}.`);
