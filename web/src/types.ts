export type NodeKind = "client" | "service" | "database";
export interface ComponentNode {
  id: string;
  type: NodeKind;
  name: string;
  position: { x: number; y: number };
  config: { processingMs: number; capacity: number; deduplicate: boolean };
}
export interface Link {
  id: string;
  source: string;
  target: string;
  latencyMs: number;
}
export type Rule = "no_duplicates" | "all_complete" | "latency_p95";
export interface Requirement {
  id: string;
  text: string;
  rule: Rule;
  targetMs: number;
}
export interface Scenario {
  requestCount: number;
  intervalMs: number;
  timeoutMs: number;
  maxRetries: number;
  dropFirstResponse: boolean;
  jitterMs: number;
  seed: number;
}
export interface Project {
  schemaVersion: 1;
  id: string;
  name: string;
  description: string;
  nodes: ComponentNode[];
  edges: Link[];
  requirements: Requirement[];
  scenario: Scenario;
}
export interface ValidationIssue {
  path: string;
  message: string;
}
export interface Validation {
  valid: boolean;
  issues: ValidationIssue[];
}
export interface TraceEvent {
  sequence: number;
  timeMs: number;
  kind: string;
  nodeId: string;
  requestId: string;
  attempt: number;
  message: string;
}
export interface Verdict {
  requirementId: string;
  text: string;
  rule: string;
  passed: boolean;
  actual: number;
  expected: number;
  message: string;
}
export interface Metrics {
  requests: number;
  completed: number;
  failed: number;
  attempts: number;
  writes: number;
  duplicateWrites: number;
  deduplicated: number;
  p95LatencyMs: number;
  averageLatencyMs: number;
  durationMs: number;
}
export interface Run {
  id: string;
  projectId: string;
  createdAt: string;
  modelHash: string;
  engineVersion: string;
  seed: number;
  metrics: Metrics;
  verdicts: Verdict[];
  events: TraceEvent[];
  project: Project;
}
