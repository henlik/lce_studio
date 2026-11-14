import type {
  WorkflowDefinition,
  WorkflowNode,
  WorkflowResult,
  WorkflowScenario,
} from "./workflowTypes";

export const flowNodeWidth = 244;
export const flowNodeHeight = 112;
export const flowRowGap = 112;
export const flowColumnGap = 116;
export type WorkflowRoute = {
  id: string;
  from: string;
  to: string;
  kind: "next" | "true" | "false" | "error" | "signal";
  label: string;
};
export function workflowRoutes(workflow: WorkflowDefinition): WorkflowRoute[] {
  return workflow.nodes.flatMap((n) => {
    const routes: WorkflowRoute[] = [];
    const add = (kind: WorkflowRoute["kind"], to: string, label: string) => {
      if (to)
        routes.push({ id: `${n.id}:${kind}`, from: n.id, to, kind, label });
    };
    if (n.next) add("next", n.next, "Next");
    if (n.branch) {
      add("true", n.branch.then, `${n.branch.variable} = true`);
      add("false", n.branch.else, `${n.branch.variable} = false`);
    }
    if (n.command?.errorNext)
      add("error", n.command.errorNext, "Command failure");
    return routes;
  });
}

/** Deterministic top-down layers. Only DFS return edges are removed for ranking;
 * forward edges in a loop remain forward, and unrelated stored coordinates do not
 * influence the result. Definitions (including node order) are never modified. */
export function workflowFlowLayout(workflow: WorkflowDefinition) {
  const nodes = new Map(workflow.nodes.map((n) => [n.id, n]));
  const routes = workflowRoutes(workflow).filter((r) => nodes.has(r.to));
  const outgoing = new Map<string, WorkflowRoute[]>();
  for (const r of routes)
    outgoing.set(r.from, [...(outgoing.get(r.from) || []), r]);
  const state = new Map<string, number>(),
    returns = new Set<string>(),
    ordered: string[] = [];
  function visit(id: string) {
    if (!nodes.has(id) || state.has(id)) return;
    state.set(id, 1);
    ordered.push(id);
    for (const r of outgoing.get(id) || []) {
      if (state.get(r.to) === 1) returns.add(r.id);
      else visit(r.to);
    }
    state.set(id, 2);
  }
  visit(workflow.entry);
  for (const s of workflow.signals) visit(s.target);
  for (const id of [...nodes.keys()].sort()) visit(id);
  const order = new Map(ordered.map((id, i) => [id, i]));
  const forward = routes.filter((r) => !returns.has(r.id));
  const degree = new Map(ordered.map((id) => [id, 0]));
  const rank = new Map(ordered.map((id) => [id, 0]));
  for (const r of forward) degree.set(r.to, degree.get(r.to)! + 1);
  const ready = ordered.filter((id) => degree.get(id) === 0);
  while (ready.length) {
    const id = ready.shift()!;
    for (const r of (outgoing.get(id) || []).filter(
      (r) => !returns.has(r.id),
    )) {
      rank.set(r.to, Math.max(rank.get(r.to)!, rank.get(id)! + 1));
      degree.set(r.to, degree.get(r.to)! - 1);
      if (degree.get(r.to) === 0) ready.push(r.to);
    }
  }
  const layers = new Map<number, string[]>();
  for (const id of ordered)
    layers.set(rank.get(id)!, [...(layers.get(rank.get(id)!) || []), id]);
  const positions: Record<string, { x: number; y: number }> = {};
  const columns = Math.max(1, ...[...layers.values()].map((row) => row.length));
  for (const [level, row] of [...layers.entries()].sort(
    (a, b) => a[0] - b[0],
  )) {
    // Keep merges near their predecessors; IDs break ties independently of array order.
    const desired = (id: string) => {
      const parents = forward
        .filter((r) => r.to === id)
        .map((r) => positions[r.from]?.x)
        .filter((x): x is number => x !== undefined);
      return parents.length
        ? parents.reduce((a, b) => a + b, 0) / parents.length
        : order.get(id)!;
    };
    row.sort(
      (a, b) => desired(a) - desired(b) || order.get(a)! - order.get(b)!,
    );
    row.forEach((id, column) => {
      positions[id] = {
        x:
          80 +
          ((columns - row.length) / 2 + column) *
            (flowNodeWidth + flowColumnGap),
        y: 76 + level * (flowNodeHeight + flowRowGap),
      };
    });
  }
  return {
    positions,
    returns,
    order: [...ordered].sort(
      (a, b) => rank.get(a)! - rank.get(b)! || positions[a].x - positions[b].x,
    ),
  };
}

export interface TokenVisit {
  index: number;
  nodeId: string;
  status: string;
  message: string;
  route?: WorkflowRoute;
}
/** Read recorded results only. No simulation, inferred concurrency, or synthetic
 * transition is introduced. Repeated task IDs, rejected signals and replayed
 * event receipts must not invent a second movement of the token. */
export function workflowTokenVisits(
  workflow: WorkflowDefinition,
  result: WorkflowResult,
  scenario?: WorkflowScenario,
): TokenVisit[] {
  const routes = workflowRoutes(workflow);
  const variables = Object.fromEntries(
    workflow.variables.map((v) => [v.name, v.initial]),
  );
  return result.steps.map((step, index) => {
    const node = workflow.nodes.find((n) => n.id === step.nodeId);
    Object.assign(variables, step.assignments);
    const task = step.taskId
      ? result.tasks.find((t) => t.id === step.taskId)
      : undefined;
    let next = step.next,
      signalName = "",
      status = step.status;
    let message = step.message;
    if (task?.status === "completed" && node?.task) {
      for (const [variable, source] of Object.entries(node.task.assign)) {
        if (task.values[source]) variables[variable] = task.values[source];
      }
      next = result.steps[index + 1]?.nodeId || "";
      status = "response received";
      message = `Task ${task.id} completed by ${task.completedBy}.`;
    } else if (task?.status === "cancelled") {
      const receipt = result.receipts.find(
        (r) =>
          r.id === task.completedBy &&
          r.kind === "signal" &&
          r.status === "accepted",
      );
      const signal = workflow.signals.find((s) => s.name === receipt?.signal);
      if (signal && result.steps[index + 1]?.nodeId === signal.target) {
        signalName = signal.name;
        next = signal.target;
        const event = scenario?.events.find(
          (e) =>
            e.id === task.completedBy &&
            e.kind === "signal" &&
            e.signal === signal.name,
        );
        const changes = Object.entries(signal.set || {}).map(
          ([name, binding]) =>
            [
              name,
              binding.literal ||
                (binding.variable
                  ? variables[binding.variable]
                  : binding.input
                    ? event?.values[binding.input]
                    : undefined),
            ] as const,
        );
        for (const [name, value] of changes) {
          if (value) variables[name] = value;
          else delete variables[name];
        }
      }
      status = "interrupted";
      message = `Task ${task.id} cancelled by ${task.completedBy}.`;
    }
    let route: WorkflowRoute | undefined;
    if (signalName)
      route = {
        id: `${step.nodeId}:signal:${signalName}`,
        from: step.nodeId,
        to: next,
        kind: "signal",
        label: `Interrupt · ${signalName}`,
      };
    else if (node && next) {
      // Error routes and branch routes can share a target; preserve the exact cause.
      const kind =
        node.command && step.state && !step.state.receipt
          ? "error"
          : node.branch
            ? variables[node.branch.variable]?.boolean === true
              ? "true"
              : variables[node.branch.variable]?.boolean === false
                ? "false"
                : undefined
            : "next";
      route = routes.find(
        (r) => r.from === node.id && r.to === next && r.kind === kind,
      );
      // A recorded target can disambiguate an unknown branch value, but never
      // choose between two branches that both point at that target.
      if (!route && !kind) {
        const candidates = routes.filter(
          (r) =>
            r.from === node.id &&
            r.to === next &&
            (r.kind === "true" || r.kind === "false"),
        );
        if (candidates.length === 1) route = candidates[0];
      }
    }
    return { index, nodeId: step.nodeId, status, message, route };
  });
}

export function workflowTitleLines(
  node: Pick<WorkflowNode, "name" | "id">,
): string[] {
  const words = (node.name || node.id).match(/.{1,28}(?:\s|$)|.{1,28}/g) || [];
  return words
    .slice(0, 2)
    .map((line, i) =>
      i === 1 && words.length > 2
        ? `${line.trim().slice(0, 25)}…`
        : line.trim(),
    );
}
