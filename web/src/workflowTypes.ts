import {
  referenceKey,
  type BehaviorPort,
  type ComponentBundle,
  type TypedValue,
  type EvaluationResult,
} from "./behaviorTypes";
import { dataKey, type DataContractLock } from "./dataTypes";
import {
  stateDataContracts,
  type StateMachineLock,
  type StateExpectation,
  type StateInstanceResult,
  type StateStepResult,
} from "./stateTypes";
import type { ComponentRef, ValidationIssue } from "./types";
export interface WorkflowBinding {
  input?: string;
  variable?: string;
  literal?: TypedValue;
}
export interface WorkflowVariable extends BehaviorPort {
  initial: TypedValue;
}
export interface WorkflowOwner {
  id: string;
  name: string;
  machine: { machineId: string; version: string; digest: string };
  initial: Record<string, WorkflowBinding>;
}
export interface WorkflowTask {
  kind: "human" | "adapter";
  title: string;
  instructions: string;
  request: Record<string, WorkflowBinding>;
  response: BehaviorPort[];
  assign: Record<string, string>;
}
export interface WorkflowDecision {
  component: ComponentRef;
  inputs: Record<string, WorkflowBinding>;
  assign: Record<string, string>;
}
export interface WorkflowCommand {
  owner: string;
  command: string;
  inputs: Record<string, WorkflowBinding>;
  assign: Record<string, string>;
  acceptedVariable: string;
  errorNext: string;
}
export interface WorkflowNode {
  id: string;
  name: string;
  kind: "task" | "decision" | "command" | "end";
  position: { x: number; y: number };
  task?: WorkflowTask;
  decision?: WorkflowDecision;
  command?: WorkflowCommand;
  end?: { outcome: string; outputs: Record<string, WorkflowBinding> };
  next?: string;
  branch?: { variable: string; then: string; else: string };
}
export interface WorkflowSignal {
  name: string;
  inputs: BehaviorPort[];
  when?: string;
  set: Record<string, WorkflowBinding>;
  target: string;
}
export interface WorkflowDefinition {
  schemaVersion: 1;
  id: string;
  version: string;
  name: string;
  description: string;
  author: string;
  license: string;
  dataContracts: DataContractLock[];
  decisions: ComponentBundle[];
  machines: StateMachineLock[];
  owners: WorkflowOwner[];
  inputs: BehaviorPort[];
  outputs: BehaviorPort[];
  variables: WorkflowVariable[];
  entry: string;
  nodes: WorkflowNode[];
  signals: WorkflowSignal[];
}
export interface WorkflowLock {
  workflow: WorkflowDefinition;
  digest: string;
}
export interface InstalledWorkflow extends WorkflowLock {
  installedAt: string;
}
export interface WorkflowEvent {
  id: string;
  kind: "response" | "signal";
  taskId?: string;
  signal?: string;
  values: Record<string, TypedValue>;
}
export interface WorkflowExpectation {
  status: string;
  outcome: string;
  activeTaskNode: string;
  variables: Record<string, TypedValue>;
  owners: StateExpectation[];
  tasks: number;
  completedTasks: number;
  cancelledTasks: number;
  eventReceipts: number;
}
export interface WorkflowScenario {
  schemaVersion: 1;
  name: string;
  inputs: Record<string, TypedValue>;
  events: WorkflowEvent[];
  expected?: WorkflowExpectation;
}
export interface WorkflowExample {
  workflow: WorkflowLock;
  scenario: WorkflowScenario;
}
export const workflowKey = (lock: WorkflowLock) =>
  `${lock.workflow.id}@${lock.workflow.version}:${lock.digest}`;
export const workflowStamp = (value: unknown): string =>
  JSON.stringify(value, (_key, v) =>
    typeof v === "number" && !Number.isFinite(v)
      ? `!invalid:${String(v)}`
      : v && typeof v === "object" && !Array.isArray(v)
        ? Object.fromEntries(
            Object.keys(v)
              .sort()
              .map((k) => [k, v[k]]),
          )
        : v,
  );
export function workflowContracts(w: WorkflowDefinition) {
  const locks = new Map<string, DataContractLock>();
  for (const l of w.dataContracts) locks.set(dataKey(l), l);
  for (const b of w.decisions)
    for (const p of b.packages)
      for (const l of p.package.dataContracts || []) locks.set(dataKey(l), l);
  for (const m of w.machines)
    for (const l of stateDataContracts(m.machine)) locks.set(dataKey(l), l);
  return [...locks.values()];
}
export function workflowDecision(w: WorkflowDefinition, node: WorkflowNode) {
  return node.decision
    ? w.decisions.find(
        (b) => referenceKey(b.root) === referenceKey(node.decision!.component),
      )
    : undefined;
}
export function bundleRoot(b?: ComponentBundle) {
  return b?.packages.find(
    (p) =>
      `${p.package.id}@${p.package.version}:${p.digest}` ===
      referenceKey(b.root),
  )?.package;
}
export function ownerMachine(w: WorkflowDefinition, id: string) {
  const o = w.owners.find((o) => o.id === id);
  return o
    ? w.machines.find(
        (m) =>
          m.machine.id === o.machine.machineId &&
          m.machine.version === o.machine.version &&
          m.digest === o.machine.digest,
      )?.machine
    : undefined;
}
export function workflowCommand(w: WorkflowDefinition, n: WorkflowNode) {
  return n.command
    ? ownerMachine(w, n.command.owner)?.commands.find(
        (c) => c.name === n.command!.command,
      )
    : undefined;
}
export function bindingShape(
  w: WorkflowDefinition,
  b?: WorkflowBinding,
  inputs = w.inputs,
): BehaviorPort | undefined {
  if (b?.input) return inputs.find((p) => p.name === b.input);
  if (b?.variable) return w.variables.find((p) => p.name === b.variable);
  if (b?.literal)
    return {
      name: "literal",
      type: b.literal.type,
      recordRef: b.literal.recordRef,
    };
}
export function assertWorkflowNumbers(
  value: unknown,
  path = "$",
  exact = false,
): void {
  if (typeof value === "number") {
    if (!Number.isFinite(value))
      throw new Error(
        `Invalid number at ${path}. Correct the visible numeric field before running, exporting or replacing its draft.`,
      );
    if (exact && Object.is(value, -0))
      throw new Error(
        `This exact lock contains negative zero at ${path}, which browser JSON cannot preserve. Retain it with the CLI or publish a version using ordinary zero.`,
      );
    if (
      !path.endsWith(".x") &&
      !path.endsWith(".y") &&
      !Number.isSafeInteger(value)
    )
      throw new Error(`Use a safe integer at ${path}.`);
    if (
      (path.endsWith(".x") || path.endsWith(".y")) &&
      Math.abs(value) > 1000000
    )
      throw new Error(`Coordinate ${path} must be within ±1,000,000.`);
  } else if (Array.isArray(value))
    value.forEach((v, i) => assertWorkflowNumbers(v, `${path}[${i}]`, exact));
  else if (value && typeof value === "object")
    for (const [k, v] of Object.entries(value))
      assertWorkflowNumbers(v, `${path}.${k}`, exact);
}
export function downloadWorkflow(value: unknown, name: string) {
  assertWorkflowNumbers(value);
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(value, null, 2)], { type: "application/json" }),
  );
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}
export interface WorkflowTaskResult {
  id: string;
  nodeId: string;
  kind: string;
  title: string;
  instructions: string;
  request: Record<string, TypedValue>;
  response: BehaviorPort[];
  status: string;
  completedBy: string;
  values: Record<string, TypedValue>;
}
export interface WorkflowEventReceipt {
  id: string;
  requestHash: string;
  kind: string;
  taskId?: string;
  signal?: string;
  status: string;
  message: string;
  nextTaskId: string;
  outcome: string;
}
export interface WorkflowEventResult {
  index: number;
  id: string;
  status: string;
  message: string;
  beforeTaskId: string;
  afterTaskId: string;
  receipt?: WorkflowEventReceipt;
}
export interface WorkflowStepResult {
  index: number;
  nodeId: string;
  kind: string;
  status: string;
  message: string;
  next: string;
  taskId?: string;
  assignments: Record<string, TypedValue>;
  decision?: EvaluationResult;
  state?: StateStepResult;
}
export interface WorkflowResult {
  engineVersion: string;
  workflowHash: string;
  scenarioHash: string;
  status: string;
  outcome: string;
  currentNode: string;
  variables: Record<string, TypedValue>;
  outputs: Record<string, TypedValue>;
  task?: WorkflowTaskResult;
  tasks: WorkflowTaskResult[];
  owners: StateInstanceResult[];
  receipts: WorkflowEventReceipt[];
  events: WorkflowEventResult[];
  steps: WorkflowStepResult[];
  operations: number;
  invocations: number;
  verified: boolean;
  passed: boolean;
  issues: ValidationIssue[];
}
export interface WorkflowRun {
  id: string;
  createdAt: string;
  workflow: WorkflowLock;
  scenario: WorkflowScenario;
  result: WorkflowResult;
}
export interface WorkflowRunSummary {
  id: string;
  createdAt: string;
  workflowId: string;
  workflowVersion: string;
  status: string;
  outcome: string;
  eventCount: number;
  taskCount: number;
  verified: boolean;
  passed: boolean;
}
