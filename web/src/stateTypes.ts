import type {
  BehaviorPort,
  ComponentBundle,
  EvaluationResult,
  TypedValue,
  ValueShape,
  ValueType,
} from "./behaviorTypes";
import { scalarTypes, typeKey, typeLabel } from "./behaviorTypes";
import { dataKey, recordRef, type DataContractLock } from "./dataTypes";
import type { ValidationIssue } from "./types";
export interface StateSlot extends BehaviorPort {
  durability: "durable" | "volatile";
  initial: TypedValue;
}
export interface StateEffect {
  name: string;
  payload: BehaviorPort[];
}
export interface StateBinding {
  state?: string;
  input?: string;
  literal?: TypedValue;
}
export interface StateEffectBinding {
  effect: string;
  whenOutput?: string;
  payload: Record<string, string>;
}
export interface StateCommand {
  name: string;
  inputs: BehaviorPort[];
  outputs: BehaviorPort[];
  decision: ComponentBundle;
  bindings: Record<string, StateBinding>;
  acceptedOutput: string;
  writes: Record<string, string>;
  results: Record<string, string>;
  effects: StateEffectBinding[];
}
export interface StateMachine {
  schemaVersion: 1;
  id: string;
  version: string;
  name: string;
  description: string;
  author: string;
  license: string;
  dataContracts: DataContractLock[];
  state: StateSlot[];
  effects: StateEffect[];
  commands: StateCommand[];
}
export interface StateMachineLock {
  machine: StateMachine;
  digest: string;
}
export interface InstalledStateMachine extends StateMachineLock {
  installedAt: string;
}
export interface StateInstance {
  id: string;
  name: string;
  initial: Record<string, TypedValue>;
}
export interface StateStep {
  kind: "command" | "restart";
  instanceId: string;
  commandId?: string;
  command?: string;
  expectedRevision?: number;
  inputs: Record<string, TypedValue> | null;
  fault?: "before_commit" | "after_commit_before_reply";
}
export interface StateExpectation {
  instanceId: string;
  state: Record<string, TypedValue>;
  revision: number;
  receipts: number;
  effects: number;
}
export interface StateScenario {
  schemaVersion: 1;
  name: string;
  instances: StateInstance[];
  steps: StateStep[];
  expected: StateExpectation[];
}
export interface StateExample {
  machine: StateMachineLock;
  scenario: StateScenario;
}
export interface StateReceipt {
  commandId: string;
  requestHash: string;
  command: string;
  accepted: boolean;
  revision: number;
  outputs: Record<string, TypedValue>;
  effectIds: string[];
}
export interface StateEffectIntent {
  id: string;
  instanceId: string;
  commandId: string;
  effect: string;
  revision: number;
  payload: Record<string, TypedValue>;
}
export interface StateInstanceResult {
  id: string;
  revision: number;
  state: Record<string, TypedValue>;
  receipts: StateReceipt[];
  effects: StateEffectIntent[];
}
export interface StateStepResult {
  index: number;
  kind: string;
  instanceId: string;
  commandId?: string;
  status: string;
  replyLost: boolean;
  faultApplied: boolean;
  message: string;
  beforeRevision: number;
  afterRevision: number;
  before: Record<string, TypedValue>;
  after: Record<string, TypedValue>;
  receipt?: StateReceipt;
  decision?: EvaluationResult;
}
export interface StateResult {
  engineVersion: string;
  machineHash: string;
  scenarioHash: string;
  operations: number;
  invocations: number;
  instances: StateInstanceResult[];
  steps: StateStepResult[];
  verified: boolean;
  passed: boolean;
  expectations: {
    instanceId: string;
    passed: boolean;
    issues: ValidationIssue[];
  }[];
}
export interface StateRun {
  id: string;
  createdAt: string;
  machine: StateMachineLock;
  scenario: StateScenario;
  result: StateResult;
}
export interface StateRunSummary {
  id: string;
  createdAt: string;
  machineId: string;
  machineVersion: string;
  stepCount: number;
  verified: boolean;
  passed: boolean;
}
export const stateKey = (lock: StateMachineLock) =>
  `${lock.machine.id}@${lock.machine.version}:${lock.digest}`;
export function decisionRoot(command: StateCommand) {
  return command.decision.packages.find(
    (p) =>
      p.package.id === command.decision.root.packageId &&
      p.package.version === command.decision.root.version &&
      p.digest === command.decision.root.digest,
  )?.package;
}
export function stateDataContracts(machine: StateMachine) {
  const locks = new Map<string, DataContractLock>();
  for (const lock of machine.dataContracts) locks.set(dataKey(lock), lock);
  for (const command of machine.commands)
    for (const p of command.decision.packages)
      for (const lock of p.package.dataContracts || [])
        locks.set(dataKey(lock), lock);
  return [...locks.values()];
}
export function stateTypeChoices(locks: DataContractLock[]) {
  const shapes: ValueShape[] = scalarTypes.map((type) => ({ type }));
  for (const lock of locks)
    for (const record of lock.contract.records)
      for (const type of ["record", "collection"] as const)
        shapes.push({ type, recordRef: recordRef(lock, record.name) });
  return shapes.map((shape) => ({
    ...shape,
    type: shape.type as ValueType,
    key: typeKey(shape),
    label: typeLabel(shape),
  }));
}
export function freshStateName(prefix: string, used: string[]) {
  let i = 1;
  while (used.includes(prefix + i)) i++;
  return prefix + i;
}

// UI identity never becomes part of a machine/scenario or its exact digest.
const uiKeys = new WeakMap<object, number>();
let nextUIKey = 0;
export function stateUIKey(value: object) {
  let key = uiKeys.get(value);
  if (key === undefined) {
    key = ++nextUIKey;
    uiKeys.set(value, key);
  }
  return key;
}
