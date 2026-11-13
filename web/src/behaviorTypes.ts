import {
  dataRefKey,
  resolveRecord,
  type DataTypeRef,
  type DataContractLock,
} from "./dataTypes";
import type {
  ComponentPackage,
  ComponentRef,
  InstalledComponent,
  PackageLock,
} from "./types";

export type ScalarType = "boolean" | "integer" | "string";
export type ValueType = ScalarType | "record" | "collection";
export interface ValueShape {
  type: ValueType | "T";
  recordRef?: DataTypeRef;
}
export interface TypedValue {
  type: ValueType;
  boolean?: boolean;
  integer?: number;
  string?: string;
  recordRef?: DataTypeRef;
  record?: Record<string, TypedValue>;
  collection?: TypedValue[];
}
export interface BehaviorPort {
  name: string;
  type: ValueType;
  recordRef?: DataTypeRef;
}
export interface BehaviorBinding {
  input?: string;
  node?: string;
  port?: string;
  literal?: TypedValue;
}
export interface BehaviorNode {
  color?: string;
  id: string;
  op?: string;
  component?: ComponentRef;
  inputs: Record<string, BehaviorBinding>;
  position: { x: number; y: number };
  recordRef?: DataTypeRef;
  field?: string;
}
export interface Behavior {
  inputs: BehaviorPort[];
  outputs: BehaviorPort[];
  nodes: BehaviorNode[];
  bindings: Record<string, BehaviorBinding>;
}
export interface BehaviorCase {
  name: string;
  inputs: Record<string, TypedValue>;
  expected: Record<string, TypedValue>;
}
export interface CompositePackage {
  designLevel?: "component" | "system";
  schemaVersion: 2 | 3;
  id: string;
  version: string;
  name: string;
  description: string;
  author: string;
  license: string;
  kernel: "composite";
  capabilities: ["design", "evaluate"];
  config: ComponentPackage["config"];
  behavior: Behavior;
  tests?: BehaviorCase[];
  dataContracts?: DataContractLock[];
}
export interface CompositeLock {
  package: CompositePackage;
  digest: string;
}
export interface InstalledComposite extends CompositeLock {
  installedAt: string;
}
export type CatalogRecord = InstalledComponent | InstalledComposite;
export interface ComponentBundle {
  schemaVersion: 1;
  root: ComponentRef;
  packages: CompositeLock[];
}
export interface PrimitiveSpec {
  op: string;
  name: string;
  description: string;
  inputs: { name: string; type: ValueType | "T" }[];
  outputs: { name: string; type: ValueType | "T" }[];
}
export interface BehaviorEvent {
  sequence: number;
  path: string;
  kind: "component_enter" | "primitive" | "component_exit";
  op?: string;
  component?: ComponentRef;
  inputs?: Record<string, TypedValue>;
  outputs?: Record<string, TypedValue>;
}
export interface EvaluationResult {
  engineVersion: string;
  bundleHash: string;
  root: ComponentRef;
  outputs: Record<string, TypedValue>;
  operations: number;
  invocations: number;
  events: BehaviorEvent[];
}
export interface EvaluationRecord {
  id: string;
  createdAt: string;
  bundle: ComponentBundle;
  inputs: Record<string, TypedValue>;
  result: EvaluationResult;
}
export interface EvaluationSummary {
  id: string;
  createdAt: string;
  componentId: string;
  componentVersion: string;
  operations: number;
  invocations: number;
}
export interface BehaviorTestResult {
  passed: boolean;
  cases: {
    name: string;
    passed: boolean;
    expected: Record<string, TypedValue>;
    actual?: Record<string, TypedValue>;
    error?: string;
  }[];
}
export interface BindingSource {
  node: string;
  port: string;
  type: ValueType | "T";
  label: string;
  recordRef?: DataTypeRef;
}
export const scalarTypes: ScalarType[] = ["boolean", "integer", "string"];
export const cloneBehavior = <T>(v: T): T => JSON.parse(JSON.stringify(v));
export const typedDefault = (
  type: ValueType,
  ref?: DataTypeRef,
  contracts: DataContractLock[] = [],
): TypedValue => {
  if (type === "record")
    return {
      type,
      recordRef: ref ? { ...ref } : undefined,
      record: Object.fromEntries(
        (resolveRecord(ref, contracts)?.fields || []).map((f) => [
          f.name,
          typedDefault(f.type),
        ]),
      ),
    };
  if (type === "collection")
    return { type, recordRef: ref ? { ...ref } : undefined, collection: [] };
  return {
    type,
    [type]: type === "boolean" ? false : type === "integer" ? 0 : "",
  };
};
export const valueText = (value?: TypedValue): string =>
  !value
    ? "missing"
    : value.type === "record" || value.type === "collection"
      ? JSON.stringify(
          value.type === "record" ? value.record : value.collection,
        )
      : value.type === "string"
        ? JSON.stringify(value.string)
        : String(value[value.type]);
export const typeKey = (p: ValueShape) =>
  `${p.type}:${dataRefKey(p.recordRef)}`;
export const typeLabel = (p: ValueShape) =>
  p.recordRef
    ? `${p.type}<${p.recordRef.record}> · ${p.recordRef.contractId}@${p.recordRef.version} · ${p.recordRef.digest.slice(0, 8)}`
    : p.type;
export const compatibleType = (a: ValueShape, b: ValueShape) =>
  a.type === "T" ||
  b.type === "T" ||
  (a.type === b.type &&
    (!a.recordRef ||
      !b.recordRef ||
      dataRefKey(a.recordRef) === dataRefKey(b.recordRef)));
export function componentDataContracts(
  root: CompositePackage,
  dependencies: CompositeLock[],
) {
  const found = new Map<string, DataContractLock>();
  for (const p of [root, ...dependencies.map((d) => d.package)])
    for (const l of p.dataContracts || [])
      found.set(`${l.contract.id}@${l.contract.version}:${l.digest}`, l);
  return [...found.values()];
}
export const referenceKey = (r: {
  packageId: string;
  version: string;
  digest: string;
}) => `${r.packageId}@${r.version}:${r.digest}`;
export const lockKey = (p: CompositeLock | PackageLock) =>
  `${p.package.id}@${p.package.version}:${p.digest}`;
export const stableBehavior = (value: unknown) =>
  JSON.stringify(value, (_k, v) =>
    v && typeof v === "object" && !Array.isArray(v)
      ? Object.fromEntries(
          Object.entries(v).sort(([a], [b]) => a.localeCompare(b)),
        )
      : v,
  );

export function nestedPackage(
  node: BehaviorNode,
  dependencies: CompositeLock[],
) {
  return node.component
    ? dependencies.find((p) => lockKey(p) === referenceKey(node.component!))
        ?.package
    : undefined;
}
export function nodeInputs(
  node: BehaviorNode,
  dependencies: CompositeLock[],
  primitives: PrimitiveSpec[],
  contracts: DataContractLock[] = [],
  behavior?: Behavior,
): (
  | BehaviorPort
  | { name: string; type: ValueType | "T"; recordRef?: DataTypeRef }
)[] {
  if (node.component)
    return nestedPackage(node, dependencies)?.behavior.inputs || [];
  if (node.op === "record_make")
    return resolveRecord(node.recordRef, contracts)?.fields || [];
  if (node.op === "record_get") return [{ name: "value", type: "record" }];
  if (node.op === "collection_count")
    return [{ name: "values", type: "collection" }];
  if (node.op === "collection_at")
    return [
      { name: "values", type: "collection" },
      { name: "index", type: "integer" },
    ];
  if (node.op === "collection_append") {
    const shape = behavior
      ? bindingShape(
          node.inputs.values,
          behavior,
          dependencies,
          primitives,
          contracts,
        )
      : undefined;
    return [
      { name: "values", type: "collection", recordRef: shape?.recordRef },
      { name: "value", type: "record", recordRef: shape?.recordRef },
    ];
  }
  return primitives.find((p) => p.op === node.op)?.inputs || [];
}
export function nodeOutputs(
  node: BehaviorNode,
  behavior: Behavior,
  dependencies: CompositeLock[],
  primitives: PrimitiveSpec[],
  contracts: DataContractLock[] = [],
  seen = new Set<string>(),
): BindingSource[] {
  if (seen.has(node.id)) return [];
  const visited = new Set(seen).add(node.id);
  const input = (name: string) =>
    bindingShape(
      node.inputs[name],
      behavior,
      dependencies,
      primitives,
      contracts,
      visited,
    );
  let outputs: ValueShape[] = [];
  let names: string[] = ["result"];
  if (node.component) {
    const ports = nestedPackage(node, dependencies)?.behavior.outputs || [];
    outputs = ports;
    names = ports.map((p) => p.name);
  } else if (node.op === "record_make")
    outputs = [{ type: "record", recordRef: node.recordRef }];
  else if (node.op === "record_get") {
    const source = input("value");
    outputs = [
      {
        type:
          resolveRecord(source?.recordRef, contracts)?.fields.find(
            (f) => f.name === node.field,
          )?.type || "T",
      },
    ];
  } else if (node.op === "collection_count") outputs = [{ type: "integer" }];
  else if (node.op === "collection_at")
    outputs = [{ type: "record", recordRef: input("values")?.recordRef }];
  else if (node.op === "collection_append")
    outputs = [{ type: "collection", recordRef: input("values")?.recordRef }];
  else {
    const spec = primitives.find((p) => p.op === node.op);
    const generic = spec?.inputs.find((p) => p.type === "T");
    const inferred = generic ? input(generic.name) : undefined;
    outputs = (spec?.outputs || []).map((p) =>
      p.type === "T" ? inferred || { type: "T" } : p,
    );
    names = (spec?.outputs || []).map((p) => p.name);
  }
  return outputs.map((p, i) => ({
    ...p,
    node: node.id,
    port: names[i],
    label: `${node.id}.${names[i]}`,
  }));
}
export function bindingShape(
  binding: BehaviorBinding | undefined,
  behavior: Behavior,
  dependencies: CompositeLock[],
  primitives: PrimitiveSpec[],
  contracts: DataContractLock[] = [],
  seen = new Set<string>(),
): ValueShape | undefined {
  if (!binding) return;
  if (binding.literal)
    return { type: binding.literal.type, recordRef: binding.literal.recordRef };
  if (binding.input)
    return behavior.inputs.find((p) => p.name === binding.input);
  const source = behavior.nodes.find((n) => n.id === binding.node);
  return source
    ? nodeOutputs(
        source,
        behavior,
        dependencies,
        primitives,
        contracts,
        seen,
      ).find((p) => p.port === binding.port)
    : undefined;
}
export function bindingType(
  binding: BehaviorBinding | undefined,
  behavior: Behavior,
  dependencies: CompositeLock[],
  primitives: PrimitiveSpec[],
  contracts: DataContractLock[] = [],
  seen = new Set<string>(),
): ValueType | "T" | undefined {
  return bindingShape(
    binding,
    behavior,
    dependencies,
    primitives,
    contracts,
    seen,
  )?.type;
}
export function dependencyClosure(
  root: CompositePackage,
  dependencies: CompositeLock[],
): CompositeLock[] {
  const result: CompositeLock[] = [];
  const seen = new Set<string>();
  function visit(p: CompositePackage) {
    for (const node of p.behavior.nodes) {
      if (!node.component) continue;
      const key = referenceKey(node.component);
      if (seen.has(key)) continue;
      seen.add(key);
      const lock = dependencies.find((d) => lockKey(d) === key);
      if (lock) {
        result.push(lock);
        visit(lock.package);
      }
    }
  }
  visit(root);
  return result.sort((a, b) => lockKey(a).localeCompare(lockKey(b)));
}
