export type InspectionWorkspace = 'workflows' | 'components' | 'data' | 'state' | 'messages' | 'design' | 'deployment';
export interface InspectionCase {
  id: string; workspace: InspectionWorkspace; name: string; covers: string;
  endpoint: string; body: Record<string, any>; verdictPath: string; expected: unknown;
}
export interface InspectionPack { id: string; name: string; description: string; cases: InspectionCase[] }
export function inspectionVerdict(item: InspectionCase, result: unknown) {
  const actual = item.verdictPath.split('.').reduce<any>((v, key) => v?.[key], result);
  return { actual, passed: actual !== undefined && JSON.stringify(actual) === JSON.stringify(item.expected) };
}
