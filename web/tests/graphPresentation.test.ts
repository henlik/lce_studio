import { test } from "node:test";
import assert from "node:assert/strict";
import { feedbackEdges } from "../src/graphPresentation.ts";
test("feedback is a directed cycle, never just reverse layout or reordered nodes", () => {
  assert.deepEqual(
    [
      ...feedbackEdges([
        { id: "b-a", from: "b", to: "a" },
        { id: "a-c", from: "a", to: "c" },
      ]),
    ],
    [],
  );
  assert.deepEqual(
    [
      ...feedbackEdges([
        { id: "a-b", from: "a", to: "b" },
        { id: "b-a", from: "b", to: "a" },
        { id: "b-c", from: "b", to: "c" },
      ]),
    ].sort(),
    ["a-b", "b-a"],
  );
  assert.deepEqual(
    [...feedbackEdges([{ id: "self", from: "a", to: "a" }])],
    ["self"],
  );
});
