package reasoning

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestThinkAndAnalyze(t *testing.T) {
	r := NewReasoningTools()
	if len(r.GetMethods()) != 2 {
		t.Fatalf("expected 2 methods, got %d", len(r.GetMethods()))
	}
	out, _ := r.Think(ThinkParams{Title: "Plan", Thought: "split the problem", Action: "search"})
	if out.(string) != "Step 1:\nTitle: Plan\nReasoning: split the problem\nAction: search\nConfidence: 0.8" {
		t.Fatalf("unexpected think output:\n%s", out)
	}
	raw, _ := json.Marshal(map[string]interface{}{"title": "Check", "result": "ok", "analysis": "fine", "next_action": "final_answer", "confidence": "1"})
	out2, err := r.Execute("reasoning_tools_Analyze", raw)
	if err != nil {
		t.Fatal(err)
	}
	s := out2.(string)
	if !strings.Contains(s, "Step 2:\nTitle: Check\nReasoning: fine\nAction: None\nConfidence: 1.0") {
		t.Fatalf("unexpected analyze output:\n%s", s)
	}
	steps := r.Steps()
	if len(steps) != 2 || steps[1].NextAction != NextActionFinalAnswer || steps[1].Result != "ok" {
		t.Fatalf("unexpected steps %+v", steps)
	}
	if got := r.SessionState()["reasoning_steps"].([]string); len(got) != 2 {
		t.Fatalf("session state %v", got)
	}
	r.Reset()
	if len(r.Steps()) != 0 {
		t.Fatal("reset failed")
	}
}

func TestOptionsAndInstructions(t *testing.T) {
	r := NewReasoningTools(WithAnalyze(false), WithFewShot(""))
	if len(r.GetMethods()) != 1 {
		t.Fatal("analyze should be disabled")
	}
	if !strings.HasPrefix(r.Instructions, "<reasoning_instructions>") || !strings.Contains(r.Instructions, FewShotExamples) {
		t.Fatal("instructions missing few-shot examples")
	}
	if NewReasoningTools(WithInstructions("x")).Instructions != "x" {
		t.Fatal("custom instructions ignored")
	}
}
