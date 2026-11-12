package workflow

import (
	"encoding/json"
	"software-studio/internal/model"
	"sort"
)

func keys[T any](values map[string]T) []string {
	result := make([]string, 0, len(values))
	for key := range values {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}
func copyValue(v model.TypedValue) model.TypedValue {
	out := v
	if v.Boolean != nil {
		b := *v.Boolean
		out.Boolean = &b
	}
	if v.Integer != nil {
		n := *v.Integer
		out.Integer = &n
	}
	if v.String != nil {
		s := *v.String
		out.String = &s
	}
	if v.RecordRef != nil {
		r := *v.RecordRef
		out.RecordRef = &r
	}
	if v.Record != nil {
		out.Record = copyValues(v.Record)
	}
	if v.Collection != nil {
		rows := make([]model.TypedValue, len(*v.Collection))
		for i, row := range *v.Collection {
			rows[i] = copyValue(row)
		}
		out.Collection = &rows
	}
	return out
}
func copyValues(values map[string]model.TypedValue) map[string]model.TypedValue {
	out := map[string]model.TypedValue{}
	for key, value := range values {
		out[key] = copyValue(value)
	}
	return out
}
func references(values map[string]model.TypedValue) map[string]model.TypedValue {
	out := map[string]model.TypedValue{}
	for key, value := range values {
		out[key] = value
	}
	return out
}
func copyPorts(ports []model.BehaviorPort) []model.BehaviorPort {
	out := make([]model.BehaviorPort, len(ports))
	copy(out, ports)
	for i := range out {
		if out[i].RecordRef != nil {
			r := *out[i].RecordRef
			out[i].RecordRef = &r
		}
	}
	return out
}
func copyTask(task Task) Task {
	task.Request = copyValues(task.Request)
	task.Values = copyValues(task.Values)
	task.Response = copyPorts(task.Response)
	return task
}
func size(value any) int {
	data, err := json.Marshal(value)
	if err != nil {
		return MaxResultBytes + 1
	}
	return len(data)
}
func commas(count int) int {
	if count == 0 {
		return 0
	}
	return count - 1
}
