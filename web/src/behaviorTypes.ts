import type {
  ComponentPackage,
  ComponentRef,
  InstalledComponent,
  PackageLock,
} from "./types";

export type ScalarType = "boolean" | "integer" | "string";
export interface TypedValue {
  type: ScalarType;
  boolean?: boolean;
  integer?: number;
  string?: string;
}
export interface BehaviorPort {
  name: string;
  type: ScalarType;
}
export interface BehaviorBinding {
  input?: string;
  node?: string;
  port?: string;
  literal?: TypedValue;
}
export interface BehaviorNode {
  id: string;
  op?: string;
  component?: ComponentRef;
  inputs: Record<string, BehaviorBinding>;
  position: { x: number; y: number };
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
  schemaVersion: 2;
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
  inputs: { name: string; type: ScalarType | "T" }[];
  outputs: { name: string; type: ScalarType | "T" }[];
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
  type: ScalarType | "T";
  label: string;
}
export const scalarTypes: ScalarType[] = ["boolean", "integer", "string"];
export const cloneBehavior = <T>(v: T): T => JSON.parse(JSON.stringify(v));
export const typedDefault = (type: ScalarType): TypedValue => ({
  type,
  [type]: type === "boolean" ? false : type === "integer" ? 0 : "",
});
export const valueText = (value?: TypedValue) =>
  value
    ? value.type === "string"
      ? JSON.stringify(value.string)
      : String(value[value.type])
    : "missing";
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
) {
  return node.component
    ? nestedPackage(node, dependencies)?.behavior.inputs || []
    : primitives.find((p) => p.op === node.op)?.inputs || [];
}
export function nodeOutputs(
  node: BehaviorNode,
  behavior: Behavior,
  dependencies: CompositeLock[],
  primitives: PrimitiveSpec[],
  seen = new Set<string>(),
): BindingSource[] {
  if (seen.has(node.id)) return [];
  const visited = new Set(seen).add(node.id);
  const outputs = node.component
    ? nestedPackage(node, dependencies)?.behavior.outputs || []
    : primitives.find((p) => p.op === node.op)?.outputs || [];
  const generic = nodeInputs(node, dependencies, primitives).find(
    (p) => p.type === "T",
  );
  const inferred = generic
    ? bindingType(
        node.inputs[generic.name],
        behavior,
        dependencies,
        primitives,
        visited,
      )
    : undefined;
  return outputs.map((port) => ({
    node: node.id,
    port: port.name,
    type: port.type === "T" ? inferred || "T" : port.type,
    label: `${node.id}.${port.name}`,
  }));
}
export function bindingType(
  binding: BehaviorBinding | undefined,
  behavior: Behavior,
  dependencies: CompositeLock[],
  primitives: PrimitiveSpec[],
  seen = new Set<string>(),
): ScalarType | "T" | undefined {
  if (!binding) return undefined;
  if (binding.literal) return binding.literal.type;
  if (binding.input)
    return behavior.inputs.find((p) => p.name === binding.input)?.type;
  const source = behavior.nodes.find((n) => n.id === binding.node);
  return source
    ? nodeOutputs(source, behavior, dependencies, primitives, seen).find(
        (p) => p.port === binding.port,
      )?.type
    : undefined;
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
