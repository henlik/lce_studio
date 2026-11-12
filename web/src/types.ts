export type NodeKind = "client" | "service" | "database";
export interface ComponentNode {
  id: string;
  type: NodeKind;
  name: string;
  position: { x: number; y: number };
  config: {
    processingMs: number;
    capacity: number;
    deduplicate: boolean;
    memoryMb?: number;
  };
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
  schemaVersion: 1 | 2;
  id: string;
  name: string;
  description: string;
  nodes: ComponentNode[];
  edges: Link[];
  requirements: Requirement[];
  scenario: Scenario;
  infrastructure?: Infrastructure;
}
export interface Outage {
  startMs: number;
  endMs: number;
}
export interface Site {
  id: string;
  name: string;
  latitude: number;
  longitude: number;
}
export type HostTier = "device" | "edge" | "regional" | "cloud";
export interface Host {
  id: string;
  name: string;
  siteId: string;
  tier: HostTier;
  cpuCores: number;
  memoryMb: number;
  idleWatts: number;
  activeWatts: number;
  outages: Outage[];
}
export interface NetworkLink {
  id: string;
  sourceHostId: string;
  targetHostId: string;
  latencyMs: number;
  bandwidthKbps: number;
  outages: Outage[];
}
export interface Placement {
  nodeId: string;
  hostId: string;
}
export interface Infrastructure {
  sites: Site[];
  hosts: Host[];
  links: NetworkLink[];
  placements: Placement[];
  messageBytes: number;
}
export interface HostMetrics {
  hostId: string;
  energyWh: number;
  busyCoreMs: number;
  uptimeMs: number;
  interruptedJobs: number;
}
export interface LinkMetrics {
  linkId: string;
  transfers: number;
  bytesTransferred: number;
  droppedTransfers: number;
}
export interface InfrastructureMetrics {
  energyWh: number;
  hosts: HostMetrics[];
  links: LinkMetrics[];
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
  hostId?: string;
  linkId?: string;
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
  infrastructure?: InfrastructureMetrics;
}
