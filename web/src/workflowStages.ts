import type { WorkflowDefinition } from './workflowTypes';
import { workflowFlowLayout, workflowRoutes } from './workflowPresentation.ts';

// Paths organize a definition; they never change routing or engine semantics.
export function stagePath(value?: string): string {
  return (value || '').split('/').map(s => s.trim()).filter(Boolean).join('/');
}
export function withinStage(path: string, parent: string): boolean {
  return !parent || path === parent || path.startsWith(parent + '/');
}
export function workflowStages(workflow: WorkflowDefinition, parent = '') {
  const paths = new Map(workflow.nodes.map(n => [n.id, stagePath(n.stage)]));
  const groups = new Map<string, { path: string; name: string; nodes: string[] }>();
  for (const id of workflowFlowLayout(workflow).order) {
    const path = paths.get(id) || '';
    if (!withinStage(path, parent)) continue;
    const remainder = parent ? path.slice(parent.length + 1) : path;
    const child = remainder.split('/')[0];
    const key = child ? (parent ? parent + '/' : '') + child : parent;
    if (!groups.has(key)) groups.set(key, { path: key, name: child || 'Direct steps', nodes: [] });
    groups.get(key)!.nodes.push(id);
  }
  return [...groups.values()].sort((a,b) => {
    const rank = (path: string) => /^\d+/.test(path) ? parseInt(path,10) : Number.MAX_SAFE_INTEGER;
    return rank(a.name) - rank(b.name);
  });
}
export function stageBoundary(workflow: WorkflowDefinition, path: string) {
  const ids = new Set(workflow.nodes.filter(n => withinStage(stagePath(n.stage), path)).map(n => n.id));
  const routes = workflowRoutes(workflow);
  return {
    ids: [...ids],
    incoming: routes.filter(r => !ids.has(r.from) && ids.has(r.to)),
    outgoing: routes.filter(r => ids.has(r.from) && !ids.has(r.to)),
    signals: workflow.signals,
  };
}
export function stageProgress(ids: string[], visits: { nodeId: string }[], cursor: number) {
  if (ids.includes(visits[cursor]?.nodeId || '')) return 'current';
  if (visits.slice(0, cursor).some(v => ids.includes(v.nodeId))) return 'visited';
  return 'not visited';
}
