package codegen

import (
	"sync"
	"testing"
)

const code = `
temp = Units.TemperatureQuantity.Create(25, "Celsius")
result = Conditions.Heat.GetByLabel("Heat")
nested = obj.method(getValue(1, 2), unit)
none = thing.reset()
`

func TestMethodCalls(t *testing.T) {
	tree, err := Parse([]byte(code))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close()

	calls, err := tree.MethodCalls()
	if err != nil {
		t.Fatalf("MethodCalls: %v", err)
	}

	want := []MethodCallInfo{
		{"Units.TemperatureQuantity", "Create", 2, 2},
		{"Conditions.Heat", "GetByLabel", 1, 3},
		{"obj", "method", 2, 4},
		{"thing", "reset", 0, 5},
	}
	if len(calls) != len(want) {
		t.Fatalf("got %d calls, want %d: %+v", len(calls), len(want), calls)
	}
	for i, w := range want {
		if calls[i] != w {
			t.Errorf("call %d = %+v, want %+v", i, calls[i], w)
		}
	}
}

// The crash came in under concurrent gRPC load, so exercise that path.
func TestConcurrentParse(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tree, err := Parse([]byte(code))
			if err != nil {
				t.Error(err)
				return
			}
			defer tree.Close()
			if _, err := tree.MethodCalls(); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}
