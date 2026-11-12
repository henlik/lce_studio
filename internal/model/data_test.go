package model

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func dataText(s string) TypedValue  { return TypedValue{Type: "string", String: &s} }
func dataNumber(n int64) TypedValue { return TypedValue{Type: "integer", Integer: &n} }
func dataBoolean(b bool) TypedValue { return TypedValue{Type: "boolean", Boolean: &b} }
func dataLock(t *testing.T, c DataContract) DataContractLock {
	t.Helper()
	if issues := ValidateDataContract(c); len(issues) > 0 {
		t.Fatalf("invalid fixture contract: %#v", issues)
	}
	digest, err := DataContractDigest(c)
	if err != nil {
		t.Fatal(err)
	}
	return DataContractLock{Contract: c, Digest: digest}
}
func dataClone[T any](t *testing.T, value T) T {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone T
	if err = json.Unmarshal(encoded, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}
func dataHas(issues []ValidationIssue, part string) bool {
	for _, issue := range issues {
		if strings.Contains(issue.Path, part) || strings.Contains(issue.Message, part) {
			return true
		}
	}
	return false
}

func TestDataExamplesShareExactContractAndOwnTheirValues(t *testing.T) {
	examples := ExampleDataFixtures()
	if len(examples) != 2 {
		t.Fatal("expected two domains")
	}
	if !reflect.DeepEqual(examples[0].Contract, examples[1].Contract) {
		t.Fatal("domains must use the same exact contract")
	}
	for _, example := range examples {
		result, err := CheckDataFixture(example.Contract, example.Fixture)
		if err != nil || !result.Valid || result.RowCount != 6 || result.EntityCount != 3 || result.EngineVersion != "0.5.0" {
			t.Fatalf("example verdict: %#v %v", result, err)
		}
	}
	examples[0].Contract.Contract.Records[0].Fields[0].Name = "changed"
	*examples[0].Fixture.Rows["Subjects"][0]["role"].String = "changed"
	if examples[1].Contract.Contract.Records[0].Fields[0].Name != "id" || *examples[1].Fixture.Rows["Subjects"][0]["role"].String != "teacher" {
		t.Fatal("example values alias across domains")
	}
	if ExampleDataFixtures()[0].Contract.Contract.Records[0].Fields[0].Name != "id" {
		t.Fatal("example mutation escaped the returned values")
	}
}

func TestDataContractValidationAndExactLocks(t *testing.T) {
	base := ExampleDataFixtures()[0].Contract
	cases := []struct {
		name   string
		mutate func(*DataContract)
		path   string
	}{
		{"schema", func(c *DataContract) { c.SchemaVersion = 2 }, "schemaVersion"},
		{"null records", func(c *DataContract) { c.Records = nil }, "records"},
		{"null entities", func(c *DataContract) { c.Entities = nil }, "entities"},
		{"null relationships", func(c *DataContract) { c.Relationships = nil }, "relationships"},
		{"duplicate record", func(c *DataContract) { c.Records = append(c.Records, c.Records[0]) }, "records[3].name"},
		{"null fields", func(c *DataContract) { c.Records[0].Fields = nil }, "records[0].fields"},
		{"duplicate field", func(c *DataContract) { c.Records[0].Fields = append(c.Records[0].Fields, c.Records[0].Fields[0]) }, "records[0].fields[2].name"},
		{"nested record", func(c *DataContract) { c.Records[0].Fields[0].Type = "record" }, "records[0].fields[0].type"},
		{"empty key", func(c *DataContract) { c.Entities[0].PrimaryKey = []string{} }, "primaryKey"},
		{"duplicate key field", func(c *DataContract) { c.Entities[0].PrimaryKey = []string{"id", "id"} }, "primaryKey"},
		{"null unique keys", func(c *DataContract) { c.Entities[0].UniqueKeys = nil }, "uniqueKeys"},
		{"repeated key", func(c *DataContract) { c.Entities[0].UniqueKeys = [][]string{{"id"}} }, "uniqueKeys"},
		{"missing field", func(c *DataContract) { c.Entities[0].PrimaryKey = []string{"missing"} }, "primaryKey"},
		{"missing record", func(c *DataContract) { c.Entities[0].Record = "missing" }, "record"},
		{"missing source", func(c *DataContract) { c.Relationships[0].FromEntity = "missing" }, "fromEntity"},
		{"nonunique target", func(c *DataContract) { c.Relationships[0].ToFields = []string{"name"} }, "toFields"},
		{"mismatched type", func(c *DataContract) { c.Relationships[0].FromFields = []string{"active"} }, "fromFields"},
		{"tuple length", func(c *DataContract) { c.Relationships[0].FromFields = []string{"id", "tenantId"} }, "same length"},
		{"invalid minimum", func(c *DataContract) { c.Relationships[0].MinSources = 2 }, "minSources"},
		{"invalid maximum", func(c *DataContract) { c.Relationships[0].MaxSources = -1 }, "maxSources"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := dataClone(t, base.Contract)
			tc.mutate(&c)
			issues := ValidateDataContract(c)
			if !dataHas(issues, tc.path) {
				t.Fatalf("missing %s issue: %#v", tc.path, issues)
			}
		})
	}
	changed := dataClone(t, base)
	changed.Contract.Name += " altered"
	if !dataHas(ValidateDataContractLock(changed), "digest") {
		t.Fatal("tampered data contract accepted")
	}
	changed = base
	changed.Digest = strings.Repeat("A", 64)
	if !dataHas(ValidateDataContractLock(changed), "digest") {
		t.Fatal("uppercase digest accepted")
	}
	ref := DataTypeRef{ContractID: base.Contract.ID, Version: base.Contract.Version, Digest: base.Digest, Record: "Subject"}
	if issues := ValidateDataTypeRef(ref); len(issues) > 0 {
		t.Fatal(issues)
	}
	ref.ContractID = "invalid"
	if !dataHas(ValidateDataTypeRef(ref), "contractId") {
		t.Fatal("invalid exact type reference accepted")
	}
}

func TestDataFixtureClosedScalarsKeysReferencesAndNulls(t *testing.T) {
	example := ExampleDataFixtures()[0]
	cases := []struct {
		name   string
		mutate func(*DataFixture)
		want   string
	}{
		{"unknown entity", func(f *DataFixture) { f.Rows["Unknown"] = []map[string]TypedValue{} }, "unknown entity"},
		{"null rows", func(f *DataFixture) { f.Rows = nil }, "nonnull object"},
		{"null entity rows", func(f *DataFixture) { f.Rows["Subjects"] = nil }, "not null"},
		{"null row", func(f *DataFixture) { f.Rows["Subjects"][0] = nil }, "row must be an object"},
		{"missing field", func(f *DataFixture) { delete(f.Rows["Subjects"][0], "active") }, "active"},
		{"extra field", func(f *DataFixture) { f.Rows["Subjects"][0]["extra"] = dataText("x") }, "closed record"},
		{"null scalar", func(f *DataFixture) { f.Rows["Subjects"][0]["active"] = TypedValue{} }, "active"},
		{"mixed scalar", func(f *DataFixture) {
			v := dataBoolean(false)
			v.String = new(string)
			f.Rows["Subjects"][0]["active"] = v
		}, "Exactly one"},
		{"unsafe integer", func(f *DataFixture) { f.Rows["Subjects"][0]["active"] = dataNumber(MaxBehaviorInteger + 1) }, "9007199254740991"},
		{"duplicate primary", func(f *DataFixture) { f.Rows["Subjects"][1]["id"] = dataText("subject-1") }, "Duplicate primary"},
		{"dangling reference", func(f *DataFixture) { f.Rows["Subjects"][0]["tenantId"] = dataText("absent") }, "no matching target"},
		{"ambiguous target", func(f *DataFixture) { f.Rows["Organizations"][1]["id"] = dataText("tenant-alpha") }, "multiple target"},
		{"invalid target key", func(f *DataFixture) { f.Rows["Organizations"][0]["id"] = dataBoolean(true) }, "no matching target"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := dataClone(t, example.Fixture)
			tc.mutate(&fixture)
			result, err := CheckDataFixture(example.Contract, fixture)
			if err != nil || result.Valid || !dataHas(result.Issues, tc.want) {
				t.Fatalf("verdict=%#v error=%v", result, err)
			}
		})
	}
	fixture := DataFixture{Name: "Empty complete dataset", Rows: map[string][]map[string]TypedValue{}}
	result, err := CheckDataFixture(example.Contract, fixture)
	if err != nil || !result.Valid || result.RowCount != 0 || result.EntityCount != 3 {
		t.Fatalf("omitted entities are empty: %#v %v", result, err)
	}
	fixture = dataClone(t, example.Fixture)
	fixture.Rows["Subjects"][0]["role"] = dataText("")
	fixture.Rows["Subjects"][0]["active"] = dataBoolean(false)
	result, err = CheckDataFixture(example.Contract, fixture)
	if err != nil || !result.Valid {
		t.Fatalf("false and empty string are values: %#v %v", result, err)
	}
}

func TestDataFixtureTupleEncodingIsUnambiguous(t *testing.T) {
	c := DataContract{SchemaVersion: 1, ID: "example.tuples", Version: "1.0.0", Name: "Tuples", Author: "Tests", License: "MIT", Records: []DataRecord{{Name: "Pair", Fields: []DataField{{Name: "left", Type: "string"}, {Name: "right", Type: "string"}, {Name: "n", Type: "integer"}, {Name: "flag", Type: "boolean"}}}}, Entities: []DataEntity{{Name: "Pairs", Record: "Pair", PrimaryKey: []string{"left", "right"}, UniqueKeys: [][]string{{"n", "flag"}}, Position: Position{}}}, Relationships: []DataRelationship{}}
	lock := dataLock(t, c)
	fixture := DataFixture{Name: "Distinct typed tuples", Rows: map[string][]map[string]TypedValue{"Pairs": {
		{"left": dataText("a|b"), "right": dataText("c"), "n": dataNumber(0), "flag": dataBoolean(false)},
		{"left": dataText("a"), "right": dataText("b|c"), "n": dataNumber(0), "flag": dataBoolean(true)},
		{"left": dataText(""), "right": dataText(""), "n": dataNumber(1), "flag": dataBoolean(false)},
	}}}
	result, err := CheckDataFixture(lock, fixture)
	if err != nil || !result.Valid {
		t.Fatalf("typed tuples collided: %#v %v", result, err)
	}
	fixture.Rows["Pairs"][1]["flag"] = dataBoolean(false)
	result, err = CheckDataFixture(lock, fixture)
	if err != nil || result.Valid || !dataHas(result.Issues, "Duplicate unique") {
		t.Fatalf("real unique duplicate missed: %#v %v", result, err)
	}
}

func dataJoinExample(t *testing.T) (DataContractLock, DataFixture) {
	t.Helper()
	c := DataContract{SchemaVersion: 1, ID: "example.membership", Version: "1.0.0", Name: "Membership", Author: "Tests", License: "MIT", Records: []DataRecord{{Name: "Person", Fields: []DataField{{Name: "id", Type: "string"}}}, {Name: "Team", Fields: []DataField{{Name: "id", Type: "string"}}}, {Name: "Membership", Fields: []DataField{{Name: "person", Type: "string"}, {Name: "team", Type: "string"}}}}, Entities: []DataEntity{{Name: "People", Record: "Person", PrimaryKey: []string{"id"}, UniqueKeys: [][]string{}}, {Name: "Teams", Record: "Team", PrimaryKey: []string{"id"}, UniqueKeys: [][]string{}}, {Name: "Memberships", Record: "Membership", PrimaryKey: []string{"person", "team"}, UniqueKeys: [][]string{}}}, Relationships: []DataRelationship{{Name: "PersonMemberships", FromEntity: "Memberships", FromFields: []string{"person"}, ToEntity: "People", ToFields: []string{"id"}, MinSources: 1}, {Name: "TeamMemberships", FromEntity: "Memberships", FromFields: []string{"team"}, ToEntity: "Teams", ToFields: []string{"id"}, MinSources: 1}}}
	f := DataFixture{Name: "Many to many join", Rows: map[string][]map[string]TypedValue{"People": {{"id": dataText("p1")}, {"id": dataText("p2")}}, "Teams": {{"id": dataText("t1")}, {"id": dataText("t2")}}, "Memberships": {{"person": dataText("p1"), "team": dataText("t1")}, {"person": dataText("p1"), "team": dataText("t2")}, {"person": dataText("p2"), "team": dataText("t1")}}}}
	return dataLock(t, c), f
}

func TestDataJoinRelationshipsCardinalityAndCycles(t *testing.T) {
	lock, fixture := dataJoinExample(t)
	result, err := CheckDataFixture(lock, fixture)
	if err != nil || !result.Valid {
		t.Fatalf("join fixture: %#v %v", result, err)
	}
	strict := dataClone(t, lock.Contract)
	strict.Relationships[0].MaxSources = 1
	result, err = CheckDataFixture(dataLock(t, strict), fixture)
	if err != nil || result.Valid || !dataHas(result.Issues, "permits at most one") {
		t.Fatalf("one-to-one: %#v %v", result, err)
	}
	missing := dataClone(t, fixture)
	missing.Rows["Teams"] = append(missing.Rows["Teams"], map[string]TypedValue{"id": dataText("t3")})
	result, err = CheckDataFixture(lock, missing)
	if err != nil || result.Valid || !dataHas(result.Issues, "requires at least one") {
		t.Fatalf("minimum cardinality: %#v %v", result, err)
	}
	cycle := DataContract{SchemaVersion: 1, ID: "example.cycle", Version: "1.0.0", Name: "Cycle", Author: "Tests", License: "MIT", Records: []DataRecord{{Name: "Item", Fields: []DataField{{Name: "id", Type: "string"}, {Name: "parent", Type: "string"}}}}, Entities: []DataEntity{{Name: "Items", Record: "Item", PrimaryKey: []string{"id"}, UniqueKeys: [][]string{}}}, Relationships: []DataRelationship{{Name: "Parent", FromEntity: "Items", FromFields: []string{"parent"}, ToEntity: "Items", ToFields: []string{"id"}, MinSources: 1, MaxSources: 1}}}
	cyclicRows := DataFixture{Name: "Finite relationship cycle", Rows: map[string][]map[string]TypedValue{"Items": {{"id": dataText("a"), "parent": dataText("b")}, {"id": dataText("b"), "parent": dataText("a")}}}}
	result, err = CheckDataFixture(dataLock(t, cycle), cyclicRows)
	if err != nil || !result.Valid {
		t.Fatalf("cycles should be legal relationships: %#v %v", result, err)
	}
}

func TestDataFixtureBoundsTruncationAndStableDiagnostics(t *testing.T) {
	example := ExampleDataFixtures()[0]
	fixture := DataFixture{Name: "Many invalid rows", Rows: map[string][]map[string]TypedValue{"Subjects": make([]map[string]TypedValue, 256)}}
	first, err := CheckDataFixture(example.Contract, fixture)
	if err != nil || first.Valid || !first.IssuesTruncated || len(first.Issues) != 200 || first.RowCount != 256 {
		t.Fatalf("bounded details: %#v %v", first, err)
	}
	second, err := CheckDataFixture(example.Contract, fixture)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatal("diagnostics changed for identical invalid data")
	}
	fixture.Rows["Subjects"] = append(fixture.Rows["Subjects"], nil)
	if _, err = CheckDataFixture(example.Contract, fixture); err == nil {
		t.Fatal("257 rows accepted")
	}
	fixture = DataFixture{Name: "Oversized strings", Rows: map[string][]map[string]TypedValue{"Organizations": {}}}
	for i := 0; i < 130; i++ {
		fixture.Rows["Organizations"] = append(fixture.Rows["Organizations"], map[string]TypedValue{"id": dataText(strings.Repeat("x", 4096)), "name": dataText(strings.Repeat("y", 4096))})
	}
	if _, err = CheckDataFixture(example.Contract, fixture); err == nil {
		t.Fatal("oversized fixture accepted")
	}
	badLock := example.Contract
	badLock.Digest = strings.Repeat("0", 64)
	if _, err = CheckDataFixture(badLock, example.Fixture); err == nil {
		t.Fatal("tampered lock produced a fixture verdict")
	}
}

func TestDataCompatibilityIsDirectionalAndConservative(t *testing.T) {
	base, _ := dataJoinExample(t)
	cases := []struct {
		name       string
		change     func(*DataContract)
		compatible bool
		want       string
	}{
		{"metadata", func(c *DataContract) { c.Name = "Renamed title" }, true, "metadata"},
		{"layout", func(c *DataContract) { c.Entities[0].Position.X = 50 }, true, "position"},
		{"reorder", func(c *DataContract) { c.Records[0], c.Records[1] = c.Records[1], c.Records[0] }, true, "display order"},
		{"required field", func(c *DataContract) {
			c.Records[0].Fields = append(c.Records[0].Fields, DataField{Name: "active", Type: "boolean"})
		}, false, "required field"},
		{"new unused record", func(c *DataContract) {
			c.Records = append(c.Records, DataRecord{Name: "Unused", Fields: []DataField{{Name: "id", Type: "string"}}})
		}, true, "additional record"},
		{"new empty entity", func(c *DataContract) {
			c.Entities = append(c.Entities, DataEntity{Name: "Visitors", Record: "Person", PrimaryKey: []string{"id"}, UniqueKeys: [][]string{}})
		}, true, "additional entity"},
		{"remove relationship", func(c *DataContract) { c.Relationships = c.Relationships[1:] }, true, "Removed a relationship"},
		{"relax minimum", func(c *DataContract) { c.Relationships[0].MinSources = 0 }, true, "relaxed"},
		{"tighten maximum", func(c *DataContract) { c.Relationships[0].MaxSources = 1 }, false, "tightened"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := dataClone(t, base.Contract)
			changed.Version = "1.0.1"
			tc.change(&changed)
			result, err := CompareDataContracts(base, dataLock(t, changed))
			if err != nil || result.Compatible != tc.compatible || result.Direction != "old-to-new" {
				t.Fatalf("comparison: %#v %v", result, err)
			}
			found := false
			for _, change := range result.Changes {
				if strings.Contains(change.Path, tc.want) || strings.Contains(change.Kind, tc.want) || strings.Contains(change.Message, tc.want) {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing change %q: %#v", tc.want, result.Changes)
			}
		})
	}
	loose := dataClone(t, base.Contract)
	loose.Relationships[0].MinSources = 0
	result, err := CompareDataContracts(dataLock(t, loose), base)
	if err != nil || result.Compatible {
		t.Fatalf("tightening min should break: %#v %v", result, err)
	}
	strict := dataClone(t, base.Contract)
	strict.Relationships[0].MaxSources = 1
	result, err = CompareDataContracts(dataLock(t, strict), base)
	if err != nil || !result.Compatible {
		t.Fatalf("relaxing max should preserve data: %#v %v", result, err)
	}
	other := dataClone(t, base.Contract)
	other.ID = "example.other"
	if _, err = CompareDataContracts(base, dataLock(t, other)); err == nil {
		t.Fatal("different contract identities compared as interchangeable")
	}
}

func TestDataContractSizeAndStructuralBudgets(t *testing.T) {
	base := ExampleDataFixtures()[0].Contract.Contract
	tooMany := dataClone(t, base)
	tooMany.Records = make([]DataRecord, 33)
	if !dataHas(ValidateDataContract(tooMany), "records") {
		t.Fatal("record count bound absent")
	}
	tooMany = dataClone(t, base)
	tooMany.Relationships = make([]DataRelationship, 65)
	if !dataHas(ValidateDataContract(tooMany), "relationships") {
		t.Fatal("relationship count bound absent")
	}
	large := DataContract{SchemaVersion: 1, ID: "example.large", Version: "1.0.0", Name: "Large contract", Author: "Tests", License: "MIT", Records: []DataRecord{}, Entities: []DataEntity{}, Relationships: []DataRelationship{}}
	for i := 0; i < 32; i++ {
		record := DataRecord{Name: fmt.Sprintf("Record%02d", i), Fields: []DataField{}}
		for j := 0; j < 16; j++ {
			record.Fields = append(record.Fields, DataField{Name: fmt.Sprintf("Field%02d", j) + strings.Repeat("x", 57), Type: "string"})
		}
		large.Records = append(large.Records, record)
		entity := DataEntity{Name: fmt.Sprintf("Entity%02d", i), Record: record.Name, PrimaryKey: []string{record.Fields[0].Name, record.Fields[1].Name, record.Fields[2].Name, record.Fields[3].Name}, UniqueKeys: [][]string{}}
		for j := 4; j < 12; j++ {
			entity.UniqueKeys = append(entity.UniqueKeys, []string{record.Fields[j].Name})
		}
		large.Entities = append(large.Entities, entity)
	}
	if !dataHas(ValidateDataContract(large), "64 KiB") {
		t.Fatal("canonical data contract size bound absent")
	}
}

func TestDataFixturePreflightRejectsCyclicMalformedValues(t *testing.T) {
	example := ExampleDataFixtures()[0]
	cycle := map[string]TypedValue{}
	v := TypedValue{Type: "record", Record: cycle}
	cycle["again"] = v
	fixture := dataClone(t, example.Fixture)
	fixture.Rows["Subjects"][0]["role"] = v
	if _, err := CheckDataFixture(example.Contract, fixture); err == nil || !strings.Contains(err.Error(), "shape budget") {
		t.Fatalf("malformed in-memory value was not bounded: %v", err)
	}
}

func TestDataComparisonFieldRemovalAndUniqueConstraintDirection(t *testing.T) {
	base := ExampleDataFixtures()[0].Contract
	removed := dataClone(t, base.Contract)
	removed.Records[0].Fields = removed.Records[0].Fields[:1]
	result, err := CompareDataContracts(base, dataLock(t, removed))
	if err != nil || result.Compatible {
		t.Fatalf("removed closed-record field is breaking: %#v %v", result, err)
	}
	unique := dataClone(t, base.Contract)
	unique.Entities[0].UniqueKeys = [][]string{{"name"}}
	strict := dataLock(t, unique)
	result, err = CompareDataContracts(base, strict)
	if err != nil || result.Compatible {
		t.Fatalf("new uniqueness must break: %#v %v", result, err)
	}
	result, err = CompareDataContracts(strict, base)
	if err != nil || !result.Compatible {
		t.Fatalf("removing uniqueness must relax: %#v %v", result, err)
	}
}
