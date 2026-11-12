import type { TypedValue, ScalarType } from "./behaviorTypes";
import type { ValidationIssue } from "./types";
export interface DataField {
  name: string;
  type: ScalarType;
}
export interface DataRecord {
  name: string;
  fields: DataField[];
}
export interface DataEntity {
  name: string;
  record: string;
  primaryKey: string[];
  uniqueKeys: string[][];
  position: { x: number; y: number };
}
export interface DataRelationship {
  name: string;
  fromEntity: string;
  fromFields: string[];
  toEntity: string;
  toFields: string[];
  minSources: number;
  maxSources: number;
}
export interface DataContract {
  schemaVersion: 1;
  id: string;
  version: string;
  name: string;
  description: string;
  author: string;
  license: string;
  records: DataRecord[];
  entities: DataEntity[];
  relationships: DataRelationship[];
}
export interface DataContractLock {
  contract: DataContract;
  digest: string;
}
export interface DataContractRecord extends DataContractLock {
  installedAt: string;
}
export interface DataTypeRef {
  contractId: string;
  version: string;
  digest: string;
  record: string;
}
export interface DataFixture {
  name: string;
  rows: Record<string, Record<string, TypedValue>[]>;
}
export interface DataCheckResult {
  engineVersion: string;
  valid: boolean;
  issues: ValidationIssue[];
  issuesTruncated: boolean;
  entityCount: number;
  rowCount: number;
}
export interface DataValidationRecord {
  id: string;
  createdAt: string;
  contract: DataContractLock;
  fixture: DataFixture;
  result: DataCheckResult;
}
export interface DataValidationSummary {
  id: string;
  createdAt: string;
  contractId: string;
  contractVersion: string;
  valid: boolean;
  rowCount: number;
}
export interface DataExample {
  contract: DataContractLock;
  fixture: DataFixture;
}
export interface DataComparison {
  compatible: boolean;
  direction: "old-to-new";
  changes: { path: string; kind: string; message: string }[];
}
export const dataKey = (l: DataContractLock) =>
  `${l.contract.id}@${l.contract.version}:${l.digest}`;
export const dataRefKey = (r?: DataTypeRef) =>
  r ? `${r.contractId}@${r.version}:${r.digest}/${r.record}` : "";
export const recordRef = (l: DataContractLock, r: string): DataTypeRef => ({
  contractId: l.contract.id,
  version: l.contract.version,
  digest: l.digest,
  record: r,
});
export function resolveRecord(
  r: DataTypeRef | undefined,
  locks: DataContractLock[],
) {
  return r
    ? locks
        .find(
          (l) =>
            l.contract.id === r.contractId &&
            l.contract.version === r.version &&
            l.digest === r.digest,
        )
        ?.contract.records.find((x) => x.name === r.record)
    : undefined;
}
export const splitFields = (s: string) =>
  s
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
