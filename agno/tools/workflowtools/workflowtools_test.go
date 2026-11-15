package workflowtools

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestWorkflowTools(t *testing.T) {
	var gotIn string
	var gotExtra map[string]interface{}
	w, err := NewWorkflowToolsFromFunc(func(_ context.Context, in string, extra map[string]interface{}) (interface{}, error) {
		gotIn, gotExtra = in, extra
		if in == "fail" {
			return nil, errors.New("boom")
		}
		return map[string]string{"content": "done: " + in}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(w.GetMethods()) != 1 {
		t.Fatal("only RunWorkflow should be enabled by default")
	}
	out, err := w.Execute("workflow_tools_RunWorkflow", []byte(`{"input_data":"hi","additional_data":{"k":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.(string), "done: hi") || gotIn != "hi" || gotExtra["k"] != float64(1) {
		t.Fatalf("unexpected: %v %q %v", out, gotIn, gotExtra)
	}
	if out, _ := w.RunWorkflow(RunWorkflowParams{InputData: "fail"}); out != "Error running workflow: boom" {
		t.Fatalf("error: %v", out)
	}
	all, _ := NewWorkflowToolsFromFunc(func(context.Context, string, map[string]interface{}) (interface{}, error) { return nil, nil }, WithAll())
	if len(all.GetMethods()) != 3 {
		t.Fatal("WithAll should enable 3 tools")
	}
	if out, _ := all.Analyze(AnalyzeParams{Analysis: "a"}); out != "Workflow Analysis:\n- a" {
		t.Fatalf("analyze: %v", out)
	}
	if _, err := NewWorkflowTools(nil); err == nil {
		t.Fatal("expected error for nil workflow")
	}
}
