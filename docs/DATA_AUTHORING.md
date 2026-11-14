# Authoring reusable data contracts

Navigation update: Types/relationships are under **Structure**, compatibility comparison under **Behavior**, and fixture validation/history under **Execution**. See [staged inspection](STAGED_INSPECTION.md) for the complete use case.

A data contract gives an exact meaning to the records exchanged by components and to the complete sample datasets checked in the Data workspace. It is a reusable specification, not a database connection or a generated database schema. The precise JSON contract is in [DATA_CONTRACT.md](DATA_CONTRACT.md).

## Start from a shared definition

The bundled organization registry contains three record types: Organization, Subject and Resource. Their fields are required Boolean, safe-integer or string values. Healthcare and education examples use the same definitions with different organization names, roles and scopes. Domain vocabulary belongs in reusable contracts and components; it does not require a different execution engine.

1. Open Data and a bundled example. Inspect its record fields before editing the entity diagram.
2. Give your own definition a namespaced ID, exact three-part decimal version, author and license. An installed ID/version cannot be overwritten with different content.
3. Define record fields. Names are case-sensitive, unique within the record and begin with a letter. Each field has one exact scalar type. Empty text, zero and false are values; omitted or null fields are invalid.
4. Define an entity using one record. Select its primary key and any additional unique keys. A compound key compares a typed tuple of up to four fields; it is not a concatenated text identifier.
5. Define foreign-key relationships. The source fields must match a declared target key in order and type. Every source row must reference a target row. A reverse maximum of one prevents two source rows from referencing the same target; a reverse minimum of one requires every target row to have a source.
6. Use a join entity for many-to-many relationships. Its two foreign keys describe the two associations explicitly; no hidden many-to-many table or execution effect is created.

The diagram is a view of these same definitions. Moving an entity changes the versioned document layout, not its validation semantics. Relationship cycles are allowed because references between rows do not imply recursively embedded values.

## Check a complete fixture

Enter or import a small complete dataset. A missing entity in the fixture means it has no rows. This matters for required reverse relationships: a dataset containing an organization with no required member fails even if an omitted external system might contain that member.

Validation checks required fields, exact scalar values, primary/unique keys, foreign-key references and reverse cardinality. Each verdict includes paths to failing rows or relationships. A wrong scalar type and a valid value that violates a relationship remain different findings. Oversized requests are rejected before a result is recorded.

Saved validation evidence includes the exact locked contract, supplied fixture, engine version and verdict. Editing a contract or row does not alter earlier evidence. A valid result applies only to the supplied complete snapshot; checking live systems and enforcing database writes require later runtime adapters.

## Reuse records in Components

Component schema3 adds exact nominal record references to ports and values. A reference carries the data contract ID, version, digest and record name. Two equal-shaped records from different versions or contracts are different types until the engineer deliberately changes the consuming contract.

A portable component bundle embeds all data contracts needed by its definitions. Evaluation does not query the installed catalog. A nested component can expose a structured interface while its internal nodes extract scalar fields and call existing scalar components.

- `record_make` constructs a record from one bound scalar input per field.
- `record_get` extracts one explicitly selected field.
- `collection_count` returns the number of records.
- `collection_at` selects a record at a zero-based index; an invalid index is an error.
- `collection_append` produces a new ordered collection without mutating the input.

Identity, equality and eager value selection support these values. Record field order does not affect equality; collection element order does. Selection still evaluates both branches. These operations have no state, external I/O or implicit database effects.

The structured healthcare and school access examples each extract five fields and invoke the same scalar access policy. Their expected cost is eleven primitive operations and three component invocations. These are functional execution counts, not timing, energy or production access-control guarantees.

## Compare and publish versions

Compare an old and proposed contract before installing a changed version. The question is directional: would data accepted by the old contract still be accepted by the new one? Adding a required field, changing field types or tightening a relationship can be breaking. The checker is conservative and marks uncertain structural changes for review.

A compatible result does not make exact locks interchangeable and does not migrate saved datasets. Install the new version, update consuming components explicitly, rerun fixtures/tests, then version those components as well. Earlier evidence remains tied to the earlier definitions.

Separate data-catalog installation is optional for portable components: embedded contracts suffice for validation/evaluation. Installing a component bundle does not silently publish its embedded contracts into the data catalog.

## Portable command-line checks

```sh
go run ./cmd/datacheck contract.json > contract-lock.json
go run ./cmd/datacheck --lock contract-lock.json
go run ./cmd/datacheck --fixture fixture-example.json
go run ./cmd/datacheck --compare proposed-upgrade.json
go run ./cmd/componentcheck --test structured-component.bundle.json
```

A fixture example contains `{contract,fixture}`. A comparison contains `{old,new}` data-contract locks. The commands accept `-` for standard input and reject unknown fields, extra JSON values, tampered locks and oversized input. Invalid fixture and incompatible comparison verdicts are printed before returning a nonzero exit status.

Current limits include 32 record types, 16 required scalar fields per record, 32 entities, 64 relationships, 256 total fixture rows and 128 records per collection. Contract, fixture, value and trace byte budgets also apply. See DATA_CONTRACT.md for all bounds. Optional/null fields, nested records, scalar arrays, schema migrations and actual database constraints are outside this release.
