# v0.5 structured data contract

This increment adds reusable immutable data contracts, a graphical entity/relationship view, complete-fixture validation with saved evidence, directional compatibility comparison, and structured values in pure components. It does not add database effects, SQL generation, optional/null fields, embedded recursive records, migrations, application generation or timed deployment of composites. Existing application schemas 1–3 and simulation engine 0.3.0 stay unchanged.

## Data definitions

Data contracts are separate from component packages. Field order in the Go structs below defines typed JSON hashing, using the existing SHA-256/json.Marshal convention (not RFC8785). All metadata validations match component metadata. Internal names use the existing behavior-name pattern (letter first, up to64 letters/digits/underscores).

```go
type DataField struct { Name string `json:"name"`; Type string `json:"type"` }
type DataRecord struct { Name string `json:"name"`; Fields []DataField `json:"fields"` }
type DataEntity struct {
 Name string `json:"name"`
 Record string `json:"record"`
 PrimaryKey []string `json:"primaryKey"`
 UniqueKeys [][]string `json:"uniqueKeys"`
 Position Position `json:"position"`
}
type DataRelationship struct {
 Name string `json:"name"`
 FromEntity string `json:"fromEntity"`
 FromFields []string `json:"fromFields"`
 ToEntity string `json:"toEntity"`
 ToFields []string `json:"toFields"`
 MinSources int `json:"minSources"`
 MaxSources int `json:"maxSources"`
}
type DataContract struct {
 SchemaVersion int `json:"schemaVersion"`
 ID string `json:"id"`
 Version string `json:"version"`
 Name string `json:"name"`
 Description string `json:"description"`
 Author string `json:"author"`
 License string `json:"license"`
 Records []DataRecord `json:"records"`
 Entities []DataEntity `json:"entities"`
 Relationships []DataRelationship `json:"relationships"`
}
type DataContractLock struct { Contract DataContract `json:"contract"`; Digest string `json:"digest"` }
type DataTypeRef struct {
 ContractID string `json:"contractId"`
 Version string `json:"version"`
 Digest string `json:"digest"`
 Record string `json:"record"`
}
type DataFixture struct { Name string `json:"name"`; Rows map[string][]map[string]TypedValue `json:"rows"` }
```

Contract schema1. All arrays above must be nonnull; uniqueKeys may be [], entities/relationships may be []. Require1–32 named records,1–16 unique fields/record,0–32 entities,0–64 relationships, each namespace unique. Each field is one required scalar (boolean/integer/string) with existing strict scalar value rules. Missing, extra fields and explicit null are rejected; zero/false/empty string remain values. No nested fields or scalar arrays in this increment. Contract canonical JSON ≤64KiB.

Each entity selects one record and one primary key of1–4 distinct field names. Up to8 additional unique keys, each1–4 distinct scalar fields. Duplicate primary/unique key tuples are rejected in definitions. Key equality is typed scalar tuple equality, not concatenated display strings. Entity positions follow existing finite ±1e6 bounds. Relationships may form cycles (including self references); they do not imply recursive record values or deletion behavior.

A relationship is a foreign-key tuple from one entity to one declared primary/unique key on another entity. From/to fields have the same length/order/type and distinct names. Every source row must match exactly one target row: foreign keys are required. `minSources` is0 or1; `maxSources` is0 (unbounded) or1. These constrain the number of source rows referencing EACH target row: min1 requires at least one, max1 implements one-to-one. Many-to-many uses an explicit join entity with two relationships. No implicit tenancy, cascade, transaction or access enforcement.

## Complete fixture validation

A fixture is a complete bounded snapshot for checking the selected contract; it does not prove the validity of an external database. Name is required valid text≤100bytes. Rows is a nonnull object keyed by entity name; missing entities mean empty arrays, unknown entities fail, present row arrays and row objects must be nonnull. Every row has exactly its record's required scalar fields. Maximum256 total rows, fixture canonical JSON≤512KiB. Validate bounds before expensive iteration/serialization. Validate types, primary/unique tuples, missing references, and both reverse-cardinality bounds. A nullable foreign key is not supported. Invalid keys must not accidentally become a valid match. Diagnostics use deterministic sorted entity/map order and definition relationship order; paths identify entity,row,field or relationship. Record at most200 issue details but keep `valid:false` and `issuesTruncated:true` when more issues exist.

Result fields: `engineVersion:"0.5.0", valid, issues:[], issuesTruncated:boolean, entityCount:int, rowCount:int`. entityCount is the number of declared entities; rowCount counts all supplied rows, including invalid rows. Validation of a well-formed bounded fixture produces a verdict even when invalid; successful recording stores both valid and invalid verdicts. Oversized/malformed HTTP input or invalid contract locks fail without recording evidence.

Comparison is directional: would old valid values/datasets remain accepted by the new contract? Compare only contracts with the same ID; exact versions/digests still remain mandatory in components. Return `{compatible,direction:"old-to-new",changes:[{path,kind,message}]}` where kind is breaking/nonbreaking/metadata. Be conservative: adding/removing/changing required fields, removing records/entities, changing entity record/key constraints or adding/changing relationship constraints is breaking; adding an unused record/entity, removing a constraint/relationship, metadata/layout changes and definition array reorder can be nonbreaking/metadata if old data is still accepted. If uncertain mark breaking for review; never claim migrations or runtime lock interchangeability. Record/field names are identities. Key/relationship order within tuples matters. Tests must cover tightening/relaxing minSources/maxSources.

## Structured component bridge

Append fields only; preserve old field order/tags. Component schema1 and2 reject new data-contract fields, structured types/values, new operations and node metadata. Schema3 is a pure composite with the same capabilities/config rules as2; supports old scalar operations plus the structured ones below. Composite package limit stays128KiB. Bundle schema1 admits schema2/3 composites; old application projects reject both.

Append `DataContracts []DataContractLock json:"dataContracts,omitempty"` at END of ComponentPackage. A schema3 manifest may embed up to8 locks; validate every lock even when unused. Across a component bundle allow at most32 unique data contract ID/version pairs. Identical repetitions across packages are allowed; different digests for the same ID/version fail. All nominal references resolve from this embedded union only; no catalog lookup. Data contracts have no external dependencies in v0.5. Component installation stores embedded definitions; it does not implicitly install separate data-catalog entries.

JSON decoding retains the established Go normalization for unused optional members: an explicit `null` becomes absent and is omitted from canonical output and hashing. Thus unused `recordRef:null`, `record:null`, `collection:null` or `dataContracts:null` additions do not change a legacy digest. Required structured members still fail when null: a record needs its reference and field object, a collection needs its reference and nonnull array, and each required scalar field needs its matching nonnull value member.

Append to TypedValue, in this order:
```go
RecordRef *DataTypeRef `json:"recordRef,omitempty"`
Record map[string]TypedValue `json:"record,omitempty"`
Collection *[]TypedValue `json:"collection,omitempty"`
```
Append `RecordRef *DataTypeRef json:"recordRef,omitempty"` to BehaviorPort. Append `RecordRef *DataTypeRef json:"recordRef,omitempty"` then `Field string json:"field,omitempty"` to BehaviorNode. Existing Type string accepts record/collection only in schema3, in addition to existing scalars. Scalars prohibit recordRef; record/collection require a valid exact reference. A record value has exactly recordRef+record members beyond Type, its required scalar fields, no extra scalar/collection member. A collection has recordRef+collection (nonnull JSON array including[]), at most128 record values, each with the same exact reference. No nested collection/record field is permitted. Each structured typed value's canonical JSON is at most64KiB; each public value map at most256KiB, in addition to existing16port limit. Bound structure before recursive clone/encoding. Literal/test values, primitive results and node/event snapshots use the same checks.

Type compatibility is nominal: Type plus every recordRef field must match, even for equal-shaped contracts. `identity`, `equal`, and eager `select` accept structured values under schema3. Equality ignores record map insertion order and preserves collection element order. Clone deeply before returning/storing evidence. Old schema2 outputs/digests remain unchanged; pure evaluator version becomes0.5.0 for all new runs.

New operations (all output result):
| op | Node metadata | Inputs | Output |
| --- | --- | --- | --- |
| record_make | recordRef required, field absent | one scalar input for EVERY field of referenced record | record with exact ref |
| record_get | field required, recordRef absent | value:record | selected declared scalar field |
| collection_count | no metadata | values:collection | integer length |
| collection_at | no metadata | values:collection,index:integer | record; negative/out-of-range is an error |
| collection_append | no metadata | values:collection,value:record with same ref | new ordered collection; bounds enforced |

Metadata is rejected on unrelated ops and calls. T in primitive discovery now includes structured values for identity/equal/select, but schema2 still supports scalar T only. `record_make` has a schema-derived dynamic signature; `record_get` result follows the selected field. UI must use full nominal type identity to disable incompatible wiring. Never infer scalar type merely from the string 'record'. Value/error/trace limits, cancellation,256 expanded operations/calls and depth8 remain. Structured inputs can amplify trace size; errors return no successful partial evaluation.

## HTTP and storage

All paths below are relative to `/api`. JSON decoding is strict: bodies over 2 MiB return 413; malformed JSON, unknown fields and trailing values return 400. Domain-budget violations return 422. Responses use canonical server values. New data catalog holds≤200 exact immutable versions, independently of existing component catalog, serialized installs under its own PG advisory lock. Identical reinstall preserves installedAt; conflicting ID/version409; full409. No deletion/update endpoints. New evidence table retains immutable snapshots; list newest50 summaries. No new runtime dependencies.

- GET /data/examples → `[{contract:DataContractLock,fixture:DataFixture}]` two reference datasets.
- GET /data/contracts → `DataContractRecord[]`, shape `{contract,digest,installedAt}` sorted ID/version.
- POST /data/prepare raw DataContract →200 DataContractLock;422 issues.
- POST /data/validate-lock raw DataContractLock →200 `{valid,issues,lock?}` canonical lock only if valid.
- POST /data/contracts raw DataContractLock →201new/200identical DataContractRecord.
- GET /data/contract?id=&version=&digest= → exact DataContractRecord;404missing/422badref.
- POST /data/check `{contract:DataContractLock,fixture:DataFixture}` →200 `{result,fixture}` canonical fixture plus verdict, no persistence. Use this before JS-parsing raw fixture files or advanced JSON. Invalid lock or fixture budget→422. Invalid row content→result.valid=false; preserve it for correction.
- POST /data/validate same request →201 DataValidationRecord `{id,createdAt,contract,fixture,result}`. Record either verdict, not malformed/over-budget input.
- GET /data/validations → newest50 summaries `{id,createdAt,contractId,contractVersion,valid,rowCount}`.
- GET /data/validations/{id} → complete saved record.
- POST /data/compare `{old:DataContractLock,new:DataContractLock}` →200 comparison;422invalid lock/ID mismatch.
- POST /data/values `{contracts:DataContractLock[],values:map[string]TypedValue}` →200 `{valid,issues,values?}` canonical values only if valid. Limit 8 locks/16 values. Missing/null contracts or more than eight locks return 422; invalid individual locks or values return 200 with `valid:false`. Advanced structured value input validates source JSON before browser numeric parsing.
- GET /components/structured-examples → ComponentBundle[]; existing composite-examples remains the scalar examples for backward UI/tests.

Fixture checking/recording and behavior computations share the existing2-computation admission channel and15s deadline. Health/app version0.5.0. Keep existing scalar primitive discovery endpoint returning11 entries for backward compatibility; add GET /components/structured-primitives returning all16 for new editor. Old scalar workbench tests remain valid. Store validation/result JSON retains the original exact locked contract and supplied fixture; edits never relabel past evidence as current.

## Model functions and examples

Expose ValidateDataContract, DataContractDigest, ValidateDataContractLock, CheckDataFixture (bounded result+error), CompareDataContracts, ExampleDataFixtures, ExampleStructuredComponentBundles. Signature details and structured behavior inference helpers are coordinated by model owner with evaluator/server owners before implementation. Do not duplicate semantic checking in handlers.

Generic shared contract `lce.organization-registry@1.0.0`: records Organization(id:string,name:string), Subject(id:string,tenantId:string,role:string,active:boolean), Resource(id:string,tenantId:string,scope:string); entities Organizations/Subjects/Resources with primary key id and required subject/resource tenant foreign keys to Organizations.id (minSources0,maxSources0). Healthcare and education fixture examples use the SAME exact contract, with domain-specific names/role/scope values. No healthcare assumptions in kernel.

Schema3 wrappers `example.structured-health-access@1.0.0` and `example.structured-school-access@1.0.0` expose subject:Subject record and resource:Resource record, output permitted:boolean. Five record_get nodes extract subject.tenantId/role/active and resource.tenantId/scope, then call the existing exact scalar access policy with domain role/scope literals. Reuse existing schema2 policy+tenant-check dependencies unchanged. Expected execution:11primitive operations,3component invocations. At least five pass/deny tests/domain; missing/mismatched structured input is an error. Include JSON fixtures and offline command support for data lock/fixture validation while preserving existing componentcheck behavior.

## Acceptance gate

Unchanged schema1/2 digests and scalar/application results; one exact data contract reused in two ERDs/datasets and nested behavior; GUI contract/entity/relationship creation, fixture edits and portable import/export; invalid key/type/reference/cardinality evidence; one-to-one and join-entity cases; directional compatibility changes; field/record/collection primitives with independent expected outputs; record-order vs collection-order equality; safe integer/null/extra field rejection; value/trace/row bounds; catalog conflicts/concurrency and immutable evidence; raw JSON canonicalization; graphical draft preservation; production browser + native/Docker CI.
