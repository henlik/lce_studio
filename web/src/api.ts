import { pendingMaintenance } from "./draftStorage";
import type { ValidationIssue } from "./types";
export class APIError extends Error {
  issues: ValidationIssue[];
  status: number;
  constructor(message: string, issues: ValidationIssue[] = [], status = 0) {
    super(message);
    this.issues = issues;
    this.status = status;
  }
}
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  return request<T>(
    path,
    method,
    body === undefined ? undefined : JSON.stringify(body),
  );
}
export async function apiRawJson<T>(
  path: string,
  body: string,
  method = "POST",
): Promise<T> {
  return request<T>(path, method, body);
}
let epoch: Promise<string> | undefined;
let paused = false;
const active = new Set<Promise<unknown>>();
export async function pauseStudioRequests() { paused = true; await Promise.allSettled([...active]); }
export function resumeStudioRequests() { paused = false; }
function studioEpoch() {
  return epoch ||= send<{epoch: string}>("/studio/status", "GET").then(v => v.epoch).catch(e => { epoch = undefined; throw e; });
}
function request<T>(path: string, method: string, body?: string): Promise<T> {
  if (path.startsWith("/studio/")) return send<T>(path, method, body);
  const task = (async () => {
    if (paused || await pendingMaintenance()) throw new APIError("Backup/reset is in progress. Finish or cancel it before continuing.");
    const version = await studioEpoch();
    if (paused) throw new APIError("Studio requests are paused for backup/reset.");
    return send<T>(path, method, body, version);
  })();
  active.add(task); void task.then(() => active.delete(task), () => active.delete(task));
  return task;
}
async function send<T>(
  path: string,
  method: string,
  body?: string,
  version?: string,
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api${path}`, {
      method,
      headers: { ...(body === undefined ? {} : { "Content-Type": "application/json" }), ...(version ? { "X-Studio-Epoch": version } : {}) },
      body,
    });
  } catch {
    throw new APIError(
      "Cannot reach the Go server. Start the backend, then reconnect.",
    );
  }
  if (response.status === 204) return undefined as T;
  let payload: any;
  try {
    payload = await response.json();
  } catch {
    throw new APIError(
      "The backend returned an unexpected response. Check that the Go server is running.",
    );
  }
  if (!response.ok)
    throw new APIError(
      payload.error || `Request failed (${response.status})`,
      payload.issues || [],
      response.status,
    );
  return payload as T;
}
