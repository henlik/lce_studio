package behavior

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"software-studio/internal/model"
)

func FuzzCompositionRepeatability(f *testing.F) {
	f.Add([]byte{3, 2, 1, 7, 0, 0})
	f.Add([]byte{255, 3, 3, 128, 1, 6})
	f.Add([]byte{0, 0, 0, 0, 0, 2})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) != 6 {
			t.Skip()
		}
		amount, initial := int64(int8(data[0])), int64(int8(data[3]))
		middleCount, rootCount := int(data[1]%4)+1, int(data[2]%4)+1
		op := "add"
		if data[4]&1 != 0 {
			op = "subtract"
		}
		leaf := primitiveDefinition(t, op, map[string]model.TypedValue{"left": integer(0), "right": integer(amount)})
		leaf.ID = "test.step"
		leaf.Behavior.Inputs = []model.BehaviorPort{{Name: "value", Type: "integer"}}
		leaf.Behavior.Nodes[0].Inputs = map[string]model.BehaviorBinding{"left": input("value"), "right": literal(integer(amount))}
		leafLock := lockDefinition(t, leaf)
		middle := lockDefinition(t, callChain("test.middle", "integer", reference(leafLock), middleCount))
		root := callChain("test.root", "integer", reference(middle), rootCount)
		bundle := bundleDefinition(t, root, middle, leafLock)
		inputs := map[string]model.TypedValue{"value": integer(initial)}
		mutation := data[5] % 8
		switch mutation {
		case 1:
			delete(inputs, "value")
		case 2:
			inputs["value"] = boolean(false)
		case 3:
			inputs["value"] = model.TypedValue{Type: "integer"}
		case 4:
			bundle.Root.Digest = strings.Repeat("0", 64)
		case 5:
			bundle.Packages = bundle.Packages[:2]
		}
		result, err := Evaluate(context.Background(), bundle, inputs)
		if mutation >= 1 && mutation <= 5 {
			if err == nil || result.EngineVersion != "" {
				t.Fatalf("malformed inputs/closure produced evidence: %+v, %v", result, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		operations := middleCount * rootCount
		want := initial + int64(operations)*amount
		if op == "subtract" {
			want = initial - int64(operations)*amount
		}
		if result.Operations != operations || result.Invocations != 1+rootCount+operations || *result.Outputs["result"].Integer != want {
			t.Fatalf("independent expanded arithmetic/counts: %+v; expected%d", result, want)
		}
		if !reflect.DeepEqual(result, evaluateOK(t, bundle, inputs)) {
			t.Fatal("pure evaluation was not repeatable")
		}
		stack := []Event{}
		for _, event := range result.Events {
			switch event.Kind {
			case "component_enter":
				stack = append(stack, event)
			case "component_exit":
				if len(stack) == 0 || stack[len(stack)-1].Path != event.Path || !reflect.DeepEqual(stack[len(stack)-1].Component, event.Component) {
					t.Fatal("unbalanced depth-first trace")
				}
				stack = stack[:len(stack)-1]
			case "primitive":
				if len(stack) == 0 || !strings.HasPrefix(event.Path, stack[len(stack)-1].Path+"/") {
					t.Fatal("primitive escaped its component invocation")
				}
			default:
				t.Fatalf("unknown evidence kind: %s", event.Kind)
			}
		}
		if len(stack) != 0 {
			t.Fatal("unterminated component invocation")
		}
		flat := primitiveChain("test.flat", "integer", operations)
		for i := range flat.Behavior.Nodes {
			left := input("value")
			if i > 0 {
				left = output(fmt.Sprintf("node%02d", i-1))
			}
			flat.Behavior.Nodes[i].Op = op
			flat.Behavior.Nodes[i].Inputs = map[string]model.BehaviorBinding{"left": left, "right": literal(integer(amount))}
		}
		expanded := evaluateOK(t, bundleDefinition(t, flat), inputs)
		if !equalValues(result.Outputs, expanded.Outputs) || result.Operations != expanded.Operations {
			t.Fatal("flattened composition changed observable outputs or primitive count")
		}
		bundle.Packages[1], bundle.Packages[2] = bundle.Packages[2], bundle.Packages[1]
		reordered := evaluateOK(t, bundle, inputs)
		if !reflect.DeepEqual(result.Events, reordered.Events) || !equalValues(result.Outputs, reordered.Outputs) {
			t.Fatal("package array order changed evaluation")
		}
	})
}
