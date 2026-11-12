import type { TypedValue } from "./behaviorTypes";
import {
  stateDataContracts,
  type StateMachineLock,
  type StateExpectation,
  type StateInstanceResult,
  type StateReceipt,
  type StateStepResult,
} from "./stateTypes";
import { dataKey, type DataContractLock } from "./dataTypes";
import type { ValidationIssue } from "./types";
export interface DeliveryMachineRef {
  machineId: string;
  version: string;
  digest: string;
}
export interface DeliveryInstance {
  id: string;
  name: string;
  machine: DeliveryMachineRef;
  position: { x: number; y: number };
}
export interface DeliveryRoute {
  id: string;
  name: string;
  sourceInstanceId: string;
  effect: string;
  targetInstanceId: string;
  command: string;
  inputs: Record<string, string>;
}
export interface DeliveryModel {
  schemaVersion: 1;
  id: string;
  version: string;
  name: string;
  description: string;
  author: string;
  license: string;
  machines: StateMachineLock[];
  instances: DeliveryInstance[];
  routes: DeliveryRoute[];
}
export interface DeliveryModelLock {
  model: DeliveryModel;
  digest: string;
}
export interface InstalledDeliveryModel extends DeliveryModelLock {
  installedAt: string;
}
export interface DeliveryInitial {
  instanceId: string;
  initial: Record<string, TypedValue>;
}
export interface DeliveryStep {
  kind: "command" | "restart" | "dispatch" | "deliver";
  instanceId?: string;
  commandId?: string;
  command?: string;
  expectedRevision?: number;
  inputs: Record<string, TypedValue> | null;
  fault?:
    | "before_commit"
    | "after_commit_before_reply"
    | "drop_before_receive"
    | "before_recipient_commit"
    | "after_recipient_commit_before_ack";
  routeId?: string;
  sourceCommandId?: string;
  sourceMessage?: string;
  message?: string;
}
export interface DeliveryMessageExpectation {
  name: string;
  status: "absent" | "pending" | "acknowledged";
  attempts: number;
  accepted?: boolean;
}
export interface DeliveryScenario {
  schemaVersion: 1;
  name: string;
  initial: DeliveryInitial[];
  steps: DeliveryStep[];
  expected: StateExpectation[];
  expectedMessages: DeliveryMessageExpectation[];
}
export interface DeliveryExample {
  model: DeliveryModelLock;
  scenario: DeliveryScenario;
}
export interface DeliveryMessage {
  name: string;
  id: string;
  routeId: string;
  sourceIntentId: string;
  sourceInstanceId: string;
  sourceCommandId: string;
  effect: string;
  targetInstanceId: string;
  command: string;
  expectedRevision: number;
  inputs: Record<string, TypedValue>;
  status: string;
  attempts: number;
  acknowledgement?: StateReceipt;
}
export interface DeliveryStepResult {
  index: number;
  kind: string;
  status: string;
  message: string;
  instanceId?: string;
  messageName?: string;
  messageId?: string;
  faultApplied: boolean;
  beforeStatus: string;
  afterStatus: string;
  beforeAttempts: number;
  afterAttempts: number;
  state?: StateStepResult;
}
export interface DeliveryResult {
  engineVersion: string;
  modelHash: string;
  scenarioHash: string;
  operations: number;
  invocations: number;
  instances: StateInstanceResult[];
  messages: DeliveryMessage[];
  steps: DeliveryStepResult[];
  verified: boolean;
  passed: boolean;
  expectations: {
    instanceId: string;
    passed: boolean;
    issues: ValidationIssue[];
  }[];
  messageExpectations: {
    name: string;
    passed: boolean;
    issues: ValidationIssue[];
  }[];
}
export interface DeliveryRun {
  id: string;
  createdAt: string;
  model: DeliveryModelLock;
  scenario: DeliveryScenario;
  result: DeliveryResult;
}
export interface DeliveryRunSummary {
  id: string;
  createdAt: string;
  modelId: string;
  modelVersion: string;
  stepCount: number;
  messageCount: number;
  verified: boolean;
  passed: boolean;
}
export const deliveryKey = (lock: DeliveryModelLock) =>
  `${lock.model.id}@${lock.model.version}:${lock.digest}`;
export const deliveryMachineKey = (ref: DeliveryMachineRef) =>
  `${ref.machineId}@${ref.version}:${ref.digest}`;
export const deliveryMachineRef = (
  lock: StateMachineLock,
): DeliveryMachineRef => ({
  machineId: lock.machine.id,
  version: lock.machine.version,
  digest: lock.digest,
});
export function instanceMachine(model: DeliveryModel, id?: string) {
  const ref = model.instances.find((i) => i.id === id)?.machine;
  return ref
    ? model.machines.find(
        (l) =>
          l.machine.id === ref.machineId &&
          l.machine.version === ref.version &&
          l.digest === ref.digest,
      )?.machine
    : undefined;
}
export function deliveryDataContracts(model: DeliveryModel) {
  const locks = new Map<string, DataContractLock>();
  for (const machine of model.machines)
    for (const lock of stateDataContracts(machine.machine))
      locks.set(dataKey(lock), lock);
  return [...locks.values()];
}
export function assertDeliveryNumbers(
  model: DeliveryModel,
  scenario?: DeliveryScenario,
) {
  for (const instance of model.instances)
    for (const axis of ["x", "y"] as const)
      if (
        !Number.isFinite(instance.position[axis]) ||
        Math.abs(instance.position[axis]) > 1000000
      )
        throw new Error(
          `${instance.id} ${axis.toUpperCase()} must be finite and within ±1,000,000.`,
        );
  if (!scenario) return;
  scenario.steps.forEach((step, i) => {
    if (
      (step.kind === "command" || step.kind === "dispatch") &&
      (!Number.isSafeInteger(step.expectedRevision) ||
        step.expectedRevision! < 0)
    )
      throw new Error(
        `Step ${i + 1} expected revision must be a nonnegative safe integer.`,
      );
  });
  for (const e of scenario.expected)
    for (const key of ["revision", "receipts", "effects"] as const)
      if (!Number.isSafeInteger(e[key]) || e[key] < 0)
        throw new Error(
          `Expected ${e.instanceId} ${key} must be a nonnegative safe integer.`,
        );
  for (const e of scenario.expectedMessages)
    if (!Number.isSafeInteger(e.attempts) || e.attempts < 0)
      throw new Error(
        `Expected message ${e.name} attempts must be a nonnegative safe integer.`,
      );
}
// UI stamps distinguish invalid numeric drafts without converting them to valid zeros/nulls.
export const deliveryStamp = (value: unknown) =>
  JSON.stringify(value, (_key, v) =>
    typeof v === "number" && !Number.isFinite(v)
      ? `[invalid number:${String(v)}]`
      : v && typeof v === "object" && !Array.isArray(v)
        ? Object.fromEntries(
            Object.entries(v).sort(([a], [b]) => a.localeCompare(b)),
          )
        : v,
  );

// JSON.stringify converts -0 to 0, which would change a Go-hashed immutable layout.
export function assertBrowserExactNumbers(value: unknown, path = "$"): void {
  if (typeof value === "number" && Object.is(value, -0)) {
    throw new Error(
      `Exact lock contains a negative-zero coordinate at ${path}. This browser cannot preserve that hash, so the lock was not loaded. Retain it through the CLI, or publish a new version with ordinary zero coordinates.`,
    );
  }
  if (Array.isArray(value))
    value.forEach((item, index) =>
      assertBrowserExactNumbers(item, `${path}[${index}]`),
    );
  else if (value && typeof value === "object")
    for (const [key, item] of Object.entries(value))
      assertBrowserExactNumbers(item, `${path}.${key}`);
}
