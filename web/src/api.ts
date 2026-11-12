import type { ValidationIssue } from "./types";
export class APIError extends Error {
  issues: ValidationIssue[];
  constructor(message: string, issues: ValidationIssue[] = []) {
    super(message);
    this.issues = issues;
  }
}
export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api${path}`, {
      method,
      headers: body === undefined ? {} : { "Content-Type": "application/json" },
      body: body === undefined ? undefined : JSON.stringify(body),
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
    );
  return payload as T;
}
