package model

import (
	"fmt"
	"reflect"
)

// CompareDataContracts is directional and conservative. It reports acceptance
// of old values/datasets by the new declaration, not interchangeable locks,
// a migration, or evidence about data held in an external database.
func CompareDataContracts(oldLock, newLock DataContractLock) (DataCompatibility, error) {
	for _, input := range []struct {
		name string
		lock DataContractLock
	}{{"old", oldLock}, {"new", newLock}} {
		if issues := ValidateDataContractLock(input.lock); len(issues) > 0 {
			return DataCompatibility{}, fmt.Errorf("invalid %s data contract at %s: %s", input.name, issues[0].Path, issues[0].Message)
		}
	}
	old, new := oldLock.Contract, newLock.Contract
	if old.ID != new.ID {
		return DataCompatibility{}, fmt.Errorf("comparison requires the same data contract ID")
	}
	result := DataCompatibility{Compatible: true, Direction: "old-to-new", Changes: []DataChange{}}
	add := func(path, kind, message string) {
		result.Changes = append(result.Changes, DataChange{path, kind, message})
		if kind == "breaking" {
			result.Compatible = false
		}
	}
	for _, field := range []struct{ name, before, after string }{{"version", old.Version, new.Version}, {"name", old.Name, new.Name}, {"description", old.Description, new.Description}, {"author", old.Author, new.Author}, {"license", old.License, new.License}} {
		if field.before != field.after {
			add(field.name, "metadata", "Metadata changed; exact version/digest references still remain distinct.")
		}
	}
	oldRecords, newRecords := map[string]DataRecord{}, map[string]DataRecord{}
	for _, r := range old.Records {
		oldRecords[r.Name] = r
	}
	for _, r := range new.Records {
		newRecords[r.Name] = r
	}
	for _, name := range sortedBehaviorKeys(oldRecords) {
		before := oldRecords[name]
		after, exists := newRecords[name]
		at := "records." + name
		if !exists {
			add(at, "breaking", "Record was removed.")
			continue
		}
		oldFields, newFields := map[string]string{}, map[string]string{}
		for _, f := range before.Fields {
			oldFields[f.Name] = f.Type
		}
		for _, f := range after.Fields {
			newFields[f.Name] = f.Type
		}
		for _, field := range sortedBehaviorKeys(oldFields) {
			kind, found := newFields[field]
			if !found {
				add(at+".fields."+field, "breaking", "Removing a field makes old values invalid for this closed record.")
			} else if kind != oldFields[field] {
				add(at+".fields."+field+".type", "breaking", "The required scalar field type changed.")
			}
		}
		for _, field := range sortedBehaviorKeys(newFields) {
			if _, found := oldFields[field]; !found {
				add(at+".fields."+field, "breaking", "A new required field is absent from old values.")
			}
		}
		if reflect.DeepEqual(oldFields, newFields) && !reflect.DeepEqual(before.Fields, after.Fields) {
			add(at+".fields", "metadata", "Field display order changed; values are matched by field name.")
		}
	}
	for _, name := range sortedBehaviorKeys(newRecords) {
		if _, exists := oldRecords[name]; !exists {
			add("records."+name, "nonbreaking", "An additional record definition does not change existing values.")
		}
	}
	oldEntities, newEntities := map[string]DataEntity{}, map[string]DataEntity{}
	for _, e := range old.Entities {
		oldEntities[e.Name] = e
	}
	for _, e := range new.Entities {
		newEntities[e.Name] = e
	}
	for _, name := range sortedBehaviorKeys(oldEntities) {
		before := oldEntities[name]
		after, exists := newEntities[name]
		at := "entities." + name
		if !exists {
			add(at, "breaking", "Removing an entity rejects old datasets that contain its rows.")
			continue
		}
		if before.Record != after.Record {
			add(at+".record", "breaking", "Entity record identity changed; review the old rows against the new record.")
		}
		if !reflect.DeepEqual(before.PrimaryKey, after.PrimaryKey) {
			add(at+".primaryKey", "breaking", "The primary-key tuple changed.")
		}
		beforeKeys, afterKeys := map[string]bool{}, map[string]bool{}
		for _, key := range before.UniqueKeys {
			beforeKeys[dataTupleName(key)] = true
		}
		for _, key := range after.UniqueKeys {
			afterKeys[dataTupleName(key)] = true
		}
		for _, key := range sortedBehaviorKeys(beforeKeys) {
			if !afterKeys[key] {
				add(at+".uniqueKeys", "nonbreaking", "Removed unique constraint "+key+".")
			}
		}
		for _, key := range sortedBehaviorKeys(afterKeys) {
			if !beforeKeys[key] {
				add(at+".uniqueKeys", "breaking", "Added unique constraint "+key+" can reject old rows.")
			}
		}
		if before.Position != after.Position {
			add(at+".position", "metadata", "Entity diagram position changed.")
		}
		if reflect.DeepEqual(beforeKeys, afterKeys) && !reflect.DeepEqual(before.UniqueKeys, after.UniqueKeys) {
			add(at+".uniqueKeys", "metadata", "Unique-key declaration order changed; tuple order within each key is unchanged.")
		}
	}
	for _, name := range sortedBehaviorKeys(newEntities) {
		if _, exists := oldEntities[name]; !exists {
			add("entities."+name, "nonbreaking", "An additional entity is empty when omitted from an old complete fixture; relationship changes are checked separately.")
		}
	}
	oldRelationships, newRelationships := map[string]DataRelationship{}, map[string]DataRelationship{}
	for _, r := range old.Relationships {
		oldRelationships[r.Name] = r
	}
	for _, r := range new.Relationships {
		newRelationships[r.Name] = r
	}
	for _, name := range sortedBehaviorKeys(oldRelationships) {
		before := oldRelationships[name]
		after, exists := newRelationships[name]
		at := "relationships." + name
		if !exists {
			add(at, "nonbreaking", "Removed a relationship constraint.")
			continue
		}
		if before.FromEntity != after.FromEntity || before.ToEntity != after.ToEntity || !reflect.DeepEqual(before.FromFields, after.FromFields) || !reflect.DeepEqual(before.ToFields, after.ToFields) {
			add(at, "breaking", "Foreign-key endpoints or ordered field tuples changed.")
		}
		if before.MinSources != after.MinSources {
			kind, message := "nonbreaking", "Minimum source cardinality was relaxed."
			if after.MinSources > before.MinSources {
				kind, message = "breaking", "Minimum source cardinality was tightened."
			}
			add(at+".minSources", kind, message)
		}
		if before.MaxSources != after.MaxSources {
			kind, message := "nonbreaking", "Maximum source cardinality was relaxed to unbounded."
			if after.MaxSources == 1 {
				kind, message = "breaking", "Maximum source cardinality was tightened to one."
			}
			add(at+".maxSources", kind, message)
		}
	}
	for _, name := range sortedBehaviorKeys(newRelationships) {
		if _, exists := oldRelationships[name]; !exists {
			add("relationships."+name, "breaking", "Added a relationship constraint that can reject old datasets.")
		}
	}
	// Array positions are presentation except for explicit ordered key tuples.
	for _, pair := range []struct {
		path          string
		before, after any
	}{{"records", dataRecordNames(old.Records), dataRecordNames(new.Records)}, {"entities", dataEntityNames(old.Entities), dataEntityNames(new.Entities)}, {"relationships", dataRelationshipNames(old.Relationships), dataRelationshipNames(new.Relationships)}} {
		before, after := pair.before.([]string), pair.after.([]string)
		if len(before) == len(after) && !reflect.DeepEqual(before, after) {
			a, b := map[string]bool{}, map[string]bool{}
			for _, name := range before {
				a[name] = true
			}
			for _, name := range after {
				b[name] = true
			}
			if reflect.DeepEqual(a, b) {
				add(pair.path, "metadata", "Definition display order changed; references still resolve by name.")
			}
		}
	}
	return result, nil
}

func dataRecordNames(records []DataRecord) []string {
	names := make([]string, len(records))
	for i, r := range records {
		names[i] = r.Name
	}
	return names
}
func dataEntityNames(entities []DataEntity) []string {
	names := make([]string, len(entities))
	for i, e := range entities {
		names[i] = e.Name
	}
	return names
}
func dataRelationshipNames(relations []DataRelationship) []string {
	names := make([]string, len(relations))
	for i, r := range relations {
		names[i] = r.Name
	}
	return names
}
