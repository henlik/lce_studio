import type { BehaviorNode, CompositeLock } from "./behaviorTypes";
export const nodeColors = [
  "#7957bb",
  "#2478ba",
  "#15846a",
  "#b77518",
  "#b34b72",
  "#526b82",
];
export function nodeColor(
  node: BehaviorNode,
  dependencies: CompositeLock[] = [],
): string {
  if (node.color && /^#[0-9a-fA-F]{6}$/.test(node.color)) return node.color;
  if (node.component)
    return dependencies.find((p) => p.digest === node.component?.digest)
      ?.package.designLevel === "system"
      ? "#b77518"
      : "#15846a";
  if (["add", "subtract", "less_than", "less_or_equal"].includes(node.op || ""))
    return "#2478ba";
  if (node.op === "concat") return "#b77518";
  if (node.op?.startsWith("record") || node.op?.startsWith("collection"))
    return "#15846a";
  return "#7957bb";
}
// A feedback edge is in a directed cycle; a wire merely drawn right-to-left is not.
export function feedbackEdges(
  edges: { from: string; to: string; id: string }[],
): Set<string> {
  const adjacency = new Map<string, string[]>();
  for (const e of edges)
    adjacency.set(e.from, [...(adjacency.get(e.from) || []), e.to]);
  return new Set(
    edges
      .filter((e) => {
        const seen = new Set<string>(),
          stack = [e.to];
        while (stack.length) {
          const n = stack.pop()!;
          if (n === e.from) return true;
          if (seen.has(n)) continue;
          seen.add(n);
          stack.push(...(adjacency.get(n) || []));
        }
        return false;
      })
      .map((e) => e.id),
  );
}
