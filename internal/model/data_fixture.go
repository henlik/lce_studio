package model

import (
	"encoding/json"
	"fmt"
)

func dataScalarIssues(v TypedValue) []ValidationIssue {
	if !scalarType(v.Type) || v.RecordRef != nil || v.Record != nil || v.Collection != nil {
		return []ValidationIssue{{"", "Entity fields require a scalar value without a record reference or structured members."}}
	}
	return ValidateTypedValue(v)
}

// dataFixturePreflight bounds malformed nested values before encoding or cloning.
// Valid fixtures contain only scalar cells; these extra guards protect callers
// that supply malformed in-memory values as well as strict HTTP JSON callers.
func dataFixturePreflight(fixture DataFixture) (int, error) {
	rows, visited, lowerBytes := 0, 0, len(fixture.Name)
	var value func(TypedValue, int) error
	value = func(v TypedValue, depth int) error {
		visited++
		lowerBytes += len(v.Type) + 1
		if visited > 8192 || depth > 8 || len(v.Record) > 16 || (v.Collection != nil && len(*v.Collection) > 128) {
			return fmt.Errorf("fixture typed values exceed the bounded shape budget")
		}
		if v.String != nil {
			lowerBytes += len(*v.String)
		}
		if v.RecordRef != nil {
			lowerBytes += len(v.RecordRef.ContractID) + len(v.RecordRef.Version) + len(v.RecordRef.Digest) + len(v.RecordRef.Record)
		}
		if lowerBytes > MaxDataFixtureBytes {
			return fmt.Errorf("fixture typed JSON must not exceed 512 KiB")
		}
		for key, child := range v.Record {
			lowerBytes += len(key)
			if err := value(child, depth+1); err != nil {
				return err
			}
		}
		if v.Collection != nil {
			for _, child := range *v.Collection {
				if err := value(child, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for name, list := range fixture.Rows {
		lowerBytes += len(name) + 1
		rows += len(list)
		if rows > MaxDataFixtureRows {
			return rows, fmt.Errorf("fixture must contain at most 256 rows in total")
		}
		if lowerBytes > MaxDataFixtureBytes {
			return rows, fmt.Errorf("fixture typed JSON must not exceed 512 KiB")
		}
		for _, row := range list {
			for key, v := range row {
				lowerBytes += len(key)
				if err := value(v, 0); err != nil {
					return rows, err
				}
			}
		}
	}
	encoded, err := json.Marshal(fixture)
	if err != nil || len(encoded) > MaxDataFixtureBytes {
		return rows, fmt.Errorf("fixture typed JSON must not exceed 512 KiB")
	}
	return rows, nil
}

// dataKey encodes a typed tuple, avoiding collisions between concatenated text,
// scalar types and separators. An invalid or missing key is never indexed.
func dataKey(row map[string]TypedValue, fields []string, types map[string]string) (string, bool) {
	values := make([]TypedValue, len(fields))
	for i, field := range fields {
		v, found := row[field]
		if !found || v.Type != types[field] || len(dataScalarIssues(v)) != 0 {
			return "", false
		}
		values[i] = v
	}
	encoded, err := json.Marshal(values)
	return string(encoded), err == nil
}

// CheckDataFixture produces a verdict for a bounded complete snapshot. Invalid
// contracts and over-budget input are errors; invalid row content is evidence.
func CheckDataFixture(lock DataContractLock, fixture DataFixture) (DataCheckResult, error) {
	if issues := ValidateDataContractLock(lock); len(issues) != 0 {
		return DataCheckResult{}, fmt.Errorf("invalid data contract at %s: %s", issues[0].Path, issues[0].Message)
	}
	rowCount, err := dataFixturePreflight(fixture)
	if err != nil {
		return DataCheckResult{}, err
	}
	c := lock.Contract
	result := DataCheckResult{EngineVersion: "0.5.0", Valid: true, Issues: []ValidationIssue{}, EntityCount: len(c.Entities), RowCount: rowCount}
	add := func(path, message string) {
		result.Valid = false
		if len(result.Issues) < MaxDataIssueDetails {
			result.Issues = append(result.Issues, ValidationIssue{path, message})
		} else {
			result.IssuesTruncated = true
		}
	}
	if !behaviorTextValid(fixture.Name, 100, true) {
		add("name", "Fixture name must be nonblank valid text of at most 100 bytes.")
	}
	if fixture.Rows == nil {
		add("rows", "Fixture rows must be a nonnull object.")
	}
	records := make(map[string]map[string]string, len(c.Records))
	for _, record := range c.Records {
		fields := make(map[string]string, len(record.Fields))
		for _, field := range record.Fields {
			fields[field.Name] = field.Type
		}
		records[record.Name] = fields
	}
	entities := make(map[string]DataEntity, len(c.Entities))
	for _, entity := range c.Entities {
		entities[entity.Name] = entity
	}
	for _, name := range sortedBehaviorKeys(fixture.Rows) {
		if _, found := entities[name]; !found {
			add("rows."+name, "Rows name an unknown entity.")
		}
		if fixture.Rows[name] == nil {
			add("rows."+name, "Entity rows must be an array, not null.")
		}
	}
	// Per entity and key tuple, retain all matching rows so a duplicate target
	// never silently supplies an arbitrary valid foreign-key match.
	indexes := make(map[string]map[string]map[string][]int, len(entities))
	for _, name := range sortedBehaviorKeys(entities) {
		entity := entities[name]
		types := records[entity.Record]
		list := fixture.Rows[name]
		allKeys := append([][]string{entity.PrimaryKey}, entity.UniqueKeys...)
		indexes[name] = make(map[string]map[string][]int, len(allKeys))
		for _, key := range allKeys {
			indexes[name][dataTupleName(key)] = make(map[string][]int)
		}
		for i, row := range list {
			at := fmt.Sprintf("rows.%s[%d]", name, i)
			if row == nil {
				add(at, "Entity row must be an object, not null.")
				continue
			}
			for _, field := range sortedBehaviorKeys(types) {
				v, found := row[field]
				if !found {
					add(at+"."+field, "A value is required for this record field.")
					continue
				}
				for _, issue := range dataScalarIssues(v) {
					path := at + "." + field
					if issue.Path != "" {
						path += "." + issue.Path
					}
					add(path, issue.Message)
				}
				if v.Type != types[field] {
					add(at+"."+field+".type", "Value type must match the declared record field.")
				}
			}
			for _, field := range sortedBehaviorKeys(row) {
				if _, found := types[field]; !found {
					add(at+"."+field, "Field does not exist in the closed record definition.")
				}
			}
			for k, key := range allKeys {
				encoded, ok := dataKey(row, key, types)
				if !ok {
					continue
				}
				index := indexes[name][dataTupleName(key)]
				if len(index[encoded]) > 0 {
					kind := "primary key"
					if k > 0 {
						kind = "unique key"
					}
					add(at, fmt.Sprintf("Duplicate %s %s; first appears at row %d.", kind, dataTupleName(key), index[encoded][0]))
				}
				index[encoded] = append(index[encoded], i)
			}
		}
	}
	for ri, relation := range c.Relationships {
		at := fmt.Sprintf("relationships[%d]", ri)
		targets := fixture.Rows[relation.ToEntity]
		counts := make([]int, len(targets))
		index := indexes[relation.ToEntity][dataTupleName(relation.ToFields)]
		fromTypes := records[entities[relation.FromEntity].Record]
		for i, row := range fixture.Rows[relation.FromEntity] {
			encoded, valid := dataKey(row, relation.FromFields, fromTypes)
			if !valid {
				continue
			}
			matches := index[encoded]
			if len(matches) != 1 {
				message := "Required foreign key has no matching target row."
				if len(matches) > 1 {
					message = "Required foreign key matches multiple target rows with duplicate keys."
				}
				add(fmt.Sprintf("rows.%s[%d].%s", relation.FromEntity, i, relation.FromFields[0]), fmt.Sprintf("%s (%s)", message, relation.Name))
				continue
			}
			counts[matches[0]]++
		}
		toTypes := records[entities[relation.ToEntity].Record]
		for i, row := range targets {
			encoded, valid := dataKey(row, relation.ToFields, toTypes)
			if !valid || len(index[encoded]) != 1 {
				continue
			}
			if counts[i] < relation.MinSources {
				add(fmt.Sprintf("%s.targets[%d]", at, i), fmt.Sprintf("Target row %s[%d] requires at least one source row for %s.", relation.ToEntity, i, relation.Name))
			}
			if relation.MaxSources == 1 && counts[i] > 1 {
				add(fmt.Sprintf("%s.targets[%d]", at, i), fmt.Sprintf("Target row %s[%d] has %d source rows; %s permits at most one.", relation.ToEntity, i, counts[i], relation.Name))
			}
		}
	}
	return result, nil
}
