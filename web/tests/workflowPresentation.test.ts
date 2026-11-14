import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  workflowFlowLayout,
  workflowRoutes,
  workflowTokenVisits,
  flowNodeHeight,
  flowNodeWidth,
} from "../src/workflowPresentation.ts";
import type {
  WorkflowDefinition,
  WorkflowNode,
  WorkflowResult,
  WorkflowStepResult,
  WorkflowTaskResult,
} from "../src/workflowTypes.ts";
const node = (id: string, extra = {}): WorkflowNode => ({
  id,
  name: id,
  kind: "task",
  position: { x: 99, y: 99 },
  ...extra,
});
const workflow = (nodes: WorkflowNode[], extra = {}): WorkflowDefinition => ({
  schemaVersion: 1,
  id: "test",
  version: "1",
  name: "Test",
  description: "",
  author: "",
  license: "",
  dataContracts: [],
  decisions: [],
  machines: [],
  owners: [],
  inputs: [],
  outputs: [],
  variables: [],
  entry: nodes[0]?.id || "",
  nodes,
  signals: [],
  ...extra,
});
const step = (nodeId: string, extra = {}): WorkflowStepResult => ({
  index: 0,
  nodeId,
  kind: "decision",
  status: "evaluated",
  message: "",
  next: "",
  assignments: {},
  ...extra,
});
const result = (steps: WorkflowStepResult[], extra = {}): WorkflowResult => ({
  engineVersion: "",
  workflowHash: "",
  scenarioHash: "",
  status: "waiting",
  outcome: "",
  currentNode: "",
  variables: {},
  outputs: {},
  tasks: [],
  owners: [],
  receipts: [],
  events: [],
  steps,
  operations: 0,
  invocations: 0,
  verified: false,
  passed: false,
  issues: [],
  ...extra,
});
const bool = (value: boolean) => ({ type: "boolean", boolean: value });

test("flow places sequences downward, splits branches and rejoins after both predecessors without mutating definitions", () => {
  const w = workflow([
    node("start", { branch: { variable: "ok", then: "yes", else: "no" } }),
    node("yes", { next: "join" }),
    node("no", { next: "middle" }),
    node("middle", { next: "join" }),
    node("join", { kind: "end" }),
  ]);
  const snapshot = JSON.stringify(w);
  const p = workflowFlowLayout(w).positions;
  assert.equal(p.yes.y, p.no.y);
  assert.notEqual(p.yes.x, p.no.x);
  assert.ok(p.join.y > p.middle.y && p.join.y > p.yes.y);
  for (const e of workflowRoutes(w)) assert.ok(p[e.to].y > p[e.from].y);
  assert.equal(JSON.stringify(w), snapshot);
  const reordered = { ...w, nodes: [...w.nodes].reverse() };
  assert.deepEqual(workflowFlowLayout(reordered), workflowFlowLayout(w));
});

test("only loop returns are feedback; disconnected nodes, self loops and invalid targets remain safe", () => {
  const w = workflow([
    node("start", { next: "check" }),
    node("check", { branch: { variable: "ok", then: "done", else: "start" } }),
    node("done", { kind: "end" }),
    node("orphan", { next: "orphan" }),
    node("invalid", { next: "missing" }),
  ]);
  const layout = workflowFlowLayout(w);
  assert.deepEqual([...layout.returns].sort(), ["check:false", "orphan:next"]);
  assert.equal(Object.keys(layout.positions).length, w.nodes.length);
  assert.equal(workflowFlowLayout(workflow([])).order.length, 0);
});

test("reception layout has no node overlaps and all non-return routes move downward", () => {
  const w = JSON.parse(
    readFileSync(
      new URL(
        "../../examples/reception/hospital-reception.workflow.json",
        import.meta.url,
      ),
      "utf8",
    ),
  ) as WorkflowDefinition;
  const { positions, returns } = workflowFlowLayout(w);
  for (const e of workflowRoutes(w))
    if (!returns.has(e.id))
      assert.ok(positions[e.to].y > positions[e.from].y, e.id);
  const rectangles = Object.values(positions);
  for (let a = 0; a < rectangles.length; a++)
    for (let b = a + 1; b < rectangles.length; b++) {
      assert.ok(
        Math.abs(rectangles[a].x - rectangles[b].x) >= flowNodeWidth ||
          Math.abs(rectangles[a].y - rectangles[b].y) >= flowNodeHeight,
      );
    }
});

test("token visits preserve repeated task activations and exact true/false edges even with a shared target", () => {
  const w = workflow(
    [
      node("ask", {
        task: { assign: { ok: "answer" } },
        branch: { variable: "ok", then: "check", else: "check" },
      }),
      node("check", { next: "ask" }),
    ],
    { variables: [{ name: "ok", initial: bool(true) }] },
  );
  const steps = [
    step("ask", { kind: "task", taskId: "t1", status: "waiting" }),
    step("check", { next: "ask" }),
    step("ask", { kind: "task", taskId: "t2", status: "waiting" }),
  ];
  const tasks = [
    {
      id: "t1",
      status: "completed",
      completedBy: "r1",
      values: { answer: bool(false) },
    },
    { id: "t2", status: "pending", values: {} },
  ] as WorkflowTaskResult[];
  const visits = workflowTokenVisits(w, result(steps, { tasks }));
  assert.equal(visits.length, 3);
  assert.equal(visits[0].route?.id, "ask:false");
  assert.equal(visits[1].route?.id, "check:next");
  assert.equal(visits[2].route, undefined);
});

test("accepted interrupt is a distinct jump, while rejected/replayed events do not move the token", () => {
  const w = workflow(
    [node("ask", { next: "normal" }), node("normal"), node("emergency")],
    { signals: [{ name: "urgent", target: "emergency" }] },
  );
  const tasks = [
    { id: "t1", status: "cancelled", completedBy: "s1", values: {} },
  ] as WorkflowTaskResult[];
  const r = result(
    [
      step("ask", { kind: "task", taskId: "t1", status: "waiting" }),
      step("emergency"),
    ],
    {
      tasks,
      receipts: [
        { id: "s1", kind: "signal", status: "accepted", signal: "urgent" },
      ],
      events: [{ status: "replayed" }, { status: "rejected" }],
    },
  );
  const visits = workflowTokenVisits(w, r);
  assert.equal(visits.length, 2);
  assert.equal(visits[0].route?.kind, "signal");
  assert.equal(visits[0].route?.to, "emergency");
  r.receipts[0].status = "rejected";
  assert.equal(workflowTokenVisits(w, r)[0].route, undefined);
});

test("command failure is not confused with a business false branch to the same destination", () => {
  const w = workflow([
    node("command", {
      kind: "command",
      command: { errorNext: "review" },
      branch: { variable: "ok", then: "done", else: "review" },
    }),
    node("review"),
    node("done"),
  ]);
  const failure = step("command", {
    next: "review",
    assignments: { ok: bool(false) },
    state: { status: "failed" },
  });
  assert.equal(
    workflowTokenVisits(w, result([failure]))[0].route?.kind,
    "error",
  );
  const denial = {
    ...failure,
    state: { receipt: { accepted: false } },
  } as WorkflowStepResult;
  assert.equal(
    workflowTokenVisits(w, result([denial]))[0].route?.kind,
    "false",
  );
});

test("signal assignments use that accepted event's inputs and never retain stale branch values", () => {
  const w = workflow(
    [
      node("ask"),
      node("check", { branch: { variable: "ok", then: "done", else: "done" } }),
      node("done"),
    ],
    {
      variables: [{ name: "ok", initial: bool(true) }],
      signals: [
        {
          name: "interrupt",
          target: "check",
          set: { ok: { input: "answer" } },
        },
      ],
    },
  );
  const r = result(
    [step("ask", { taskId: "t1" }), step("check", { next: "done" })],
    {
      tasks: [{ id: "t1", status: "cancelled", completedBy: "s1" }],
      receipts: [
        { id: "s1", kind: "signal", status: "accepted", signal: "interrupt" },
      ],
    },
  );
  assert.equal(workflowTokenVisits(w, r)[1].route, undefined);
  const scenario = {
    schemaVersion: 1 as const,
    name: "interrupt",
    inputs: {},
    events: [
      {
        id: "s1",
        kind: "signal" as const,
        signal: "interrupt",
        values: { answer: { type: "boolean" as const, boolean: false } },
      },
    ],
  };
  assert.equal(workflowTokenVisits(w, r, scenario)[1].route?.kind, "false");
});
