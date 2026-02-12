package toolkit

import (
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// --- test helpers ---

type addParams struct {
	A int `json:"a" description:"first number" required:"true"`
	B int `json:"b" description:"second number" required:"true"`
}

func addFunc(p addParams) (interface{}, error) {
	return p.A + p.B, nil
}

type failParams struct {
	Msg string `json:"msg" description:"error message" required:"true"`
}

func failFunc(p failParams) (interface{}, error) {
	return nil, errors.New(p.Msg)
}

func newTestToolkit() *Toolkit {
	tk := NewToolkit()
	tk.Name = "TestTool"
	tk.Description = "A test toolkit"
	tk.Register("Add", "Adds two numbers", &tk, addFunc, addParams{})
	tk.Register("Fail", "Always fails", &tk, failFunc, failParams{})
	return &tk
}

func makeInput(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// --- Basic Registration & Execution ---

func TestRegisterAndExecute(t *testing.T) {
	tk := newTestToolkit()

	result, err := tk.Execute("TestTool_Add", makeInput(map[string]interface{}{"a": 2, "b": 3}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.(int) != 5 {
		t.Fatalf("expected 5, got %v", result)
	}
}

func TestExecuteNotFound(t *testing.T) {
	tk := newTestToolkit()

	_, err := tk.Execute("TestTool_Unknown", makeInput(map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected error for unknown method")
	}
}

// --- Hook System ---

func TestPreHookAborts(t *testing.T) {
	tk := newTestToolkit()

	tk.AddPreHook(func(methodName string, input json.RawMessage) error {
		return errors.New("blocked by hook")
	})

	_, err := tk.Execute("TestTool_Add", makeInput(map[string]interface{}{"a": 1, "b": 2}))
	if err == nil {
		t.Fatal("expected pre-hook to abort execution")
	}
	if err.Error() != "Execute: pre-hook error: blocked by hook" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPreHookAllows(t *testing.T) {
	tk := newTestToolkit()

	var called int32
	tk.AddPreHook(func(methodName string, input json.RawMessage) error {
		atomic.AddInt32(&called, 1)
		return nil // allow
	})

	result, err := tk.Execute("TestTool_Add", makeInput(map[string]interface{}{"a": 10, "b": 20}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.(int) != 30 {
		t.Fatalf("expected 30, got %v", result)
	}
	if atomic.LoadInt32(&called) != 1 {
		t.Fatal("pre-hook was not called")
	}
}

func TestPostHookReceivesResult(t *testing.T) {
	tk := newTestToolkit()

	var hookResult interface{}
	var hookErr error
	tk.AddPostHook(func(methodName string, input json.RawMessage, result interface{}, err error) {
		hookResult = result
		hookErr = err
	})

	tk.Execute("TestTool_Add", makeInput(map[string]interface{}{"a": 3, "b": 4}))
	if hookResult.(int) != 7 {
		t.Fatalf("post-hook expected result 7, got %v", hookResult)
	}
	if hookErr != nil {
		t.Fatalf("post-hook expected nil error, got %v", hookErr)
	}
}

func TestPostHookReceivesError(t *testing.T) {
	tk := newTestToolkit()

	var hookErr error
	tk.AddPostHook(func(methodName string, input json.RawMessage, result interface{}, err error) {
		hookErr = err
	})

	tk.Execute("TestTool_Fail", makeInput(map[string]interface{}{"msg": "boom"}))
	if hookErr == nil || hookErr.Error() != "boom" {
		t.Fatalf("post-hook expected error 'boom', got %v", hookErr)
	}
}

func TestMethodLevelHooks(t *testing.T) {
	tk := NewToolkit()
	tk.Name = "HookTool"
	tk.Description = "Test method-level hooks"

	var preHookCalled, postHookCalled int32

	tk.RegisterWithOptions("Add", "Adds two numbers", &tk, addFunc, addParams{},
		WithMethodPreHook(func(methodName string, input json.RawMessage) error {
			atomic.AddInt32(&preHookCalled, 1)
			return nil
		}),
		WithMethodPostHook(func(methodName string, input json.RawMessage, result interface{}, err error) {
			atomic.AddInt32(&postHookCalled, 1)
		}),
	)

	result, err := tk.Execute("HookTool_Add", makeInput(map[string]interface{}{"a": 5, "b": 5}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.(int) != 10 {
		t.Fatalf("expected 10, got %v", result)
	}
	if atomic.LoadInt32(&preHookCalled) != 1 {
		t.Fatal("method pre-hook was not called")
	}
	if atomic.LoadInt32(&postHookCalled) != 1 {
		t.Fatal("method post-hook was not called")
	}
}

// --- Result Caching ---

func TestCachingReturnsStoredResult(t *testing.T) {
	tk := newTestToolkit()
	tk.Cache = CacheConfig{Enabled: true, TTL: 5 * time.Second}

	input := makeInput(map[string]interface{}{"a": 1, "b": 2})

	result1, _ := tk.Execute("TestTool_Add", input)
	result2, _ := tk.Execute("TestTool_Add", input)

	if result1.(int) != result2.(int) {
		t.Fatalf("cached result mismatch: %v != %v", result1, result2)
	}
}

func TestCacheTTLExpires(t *testing.T) {
	tk := newTestToolkit()
	tk.Cache = CacheConfig{Enabled: true, TTL: 50 * time.Millisecond}

	input := makeInput(map[string]interface{}{"a": 1, "b": 2})

	tk.Execute("TestTool_Add", input)

	time.Sleep(100 * time.Millisecond)

	// After TTL, the cache should be expired - should still compute correctly
	result, err := tk.Execute("TestTool_Add", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.(int) != 3 {
		t.Fatalf("expected 3, got %v", result)
	}
}

func TestClearCache(t *testing.T) {
	tk := newTestToolkit()
	tk.Cache = CacheConfig{Enabled: true, TTL: 5 * time.Second}

	input := makeInput(map[string]interface{}{"a": 1, "b": 2})
	tk.Execute("TestTool_Add", input)

	tk.ClearCache()

	// After clearing, it should recompute
	result, err := tk.Execute("TestTool_Add", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.(int) != 3 {
		t.Fatalf("expected 3, got %v", result)
	}
}

func TestCacheDisabledByDefault(t *testing.T) {
	tk := newTestToolkit()
	// Cache.Enabled is false by default

	var callCount int32
	tk.AddPreHook(func(methodName string, input json.RawMessage) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	})

	input := makeInput(map[string]interface{}{"a": 1, "b": 2})
	tk.Execute("TestTool_Add", input)
	tk.Execute("TestTool_Add", input)

	if atomic.LoadInt32(&callCount) != 2 {
		t.Fatalf("expected 2 calls without cache, got %d", callCount)
	}
}

// --- Tool Filtering ---

func TestIncludeTools(t *testing.T) {
	tk := newTestToolkit()
	tk.IncludeTools("Add")

	methods := tk.GetMethods()
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}
	if _, ok := methods["TestTool_Add"]; !ok {
		t.Fatal("expected TestTool_Add to be included")
	}
}

func TestExcludeTools(t *testing.T) {
	tk := newTestToolkit()
	tk.ExcludeTools("Fail")

	methods := tk.GetMethods()
	if len(methods) != 1 {
		t.Fatalf("expected 1 method, got %d", len(methods))
	}
	if _, ok := methods["TestTool_Add"]; !ok {
		t.Fatal("expected TestTool_Add to remain")
	}
	if _, ok := methods["TestTool_Fail"]; ok {
		t.Fatal("expected TestTool_Fail to be excluded")
	}
}

func TestNoFilterReturnsAll(t *testing.T) {
	tk := newTestToolkit()

	methods := tk.GetMethods()
	if len(methods) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(methods))
	}
}

// --- RequiresConfirmation & StopAfterCall ---

func TestRegisterWithOptions_Confirmation(t *testing.T) {
	tk := NewToolkit()
	tk.Name = "ConfTool"
	tk.Description = "Test confirmation"

	tk.RegisterWithOptions("Delete", "Deletes something", &tk, addFunc, addParams{},
		WithConfirmation(),
		WithStopAfterCall(),
	)

	method := tk.methods["ConfTool_Delete"]
	if !method.RequiresConfirmation {
		t.Fatal("expected RequiresConfirmation to be true")
	}
	if !method.StopAfterCall {
		t.Fatal("expected StopAfterCall to be true")
	}
}

// --- Schema Generation ---

func TestSchemaGeneration(t *testing.T) {
	tk := newTestToolkit()
	schema := tk.GetParameterStruct("TestTool_Add")

	props, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties in schema")
	}

	if _, ok := props["a"]; !ok {
		t.Fatal("expected field 'a' in schema")
	}
	if _, ok := props["b"]; !ok {
		t.Fatal("expected field 'b' in schema")
	}

	required := schema["required"].([]string)
	if len(required) != 2 {
		t.Fatalf("expected 2 required fields, got %d", len(required))
	}
}
