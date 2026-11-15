package metrics

import "testing"

func usage(in, out int, cost float64) *MessageMetrics {
	return &MessageMetrics{Tokens: Tokens{InputTokens: in, OutputTokens: out, TotalTokens: in + out, Cost: &cost}}
}

func TestAccumulateAndMap(t *testing.T) {
	r := NewRunMetrics()
	r.AccumulateModel(usage(10, 5, 0.01), "gpt-4o", "openai", ModelTypeModel)
	r.AccumulateModel(usage(20, 5, 0.02), "gpt-4o", "openai", ModelTypeModel)
	r.AccumulateModel(usage(3, 1, 0), "mini", "openai", ModelTypeMemoryModel)
	r.AccumulateToolLoop([]*MessageMetrics{usage(1, 1, 0), nil, {}}, "mini", "openai", ModelTypeCompaction)
	s := r.Snapshot()
	if s.TotalTokens != 46 || len(s.Details["model"]) != 1 || s.Details["model"][0].TotalTokens != 40 {
		t.Fatalf("snapshot %+v", s)
	}
	if *s.Cost < 0.0299 || *s.Cost > 0.0301 {
		t.Fatalf("cost %v", *s.Cost)
	}
	m := r.ToMap()
	if m["total_tokens"] != 46 || m["details"].(map[string]interface{})["compaction_model"] == nil {
		t.Fatalf("map %v", m)
	}
	back, err := RunMetricsFromMap(m)
	if err != nil || back.TotalTokens != 46 || back.Details["memory_model"][0].ID != "mini" {
		t.Fatalf("from map %+v %v", back, err)
	}
}

func TestChildSessionAndAdd(t *testing.T) {
	parent, child := NewRunMetrics(), NewRunMetrics()
	child.AccumulateModel(usage(4, 4, 0), "judge", "anthropic", ModelTypeModel)
	parent.AccumulateChild(child, "eval")
	if parent.Snapshot().Details["eval_model"][0].TotalTokens != 8 {
		t.Fatal("child prefixing")
	}
	sum := parent.Add(child)
	if sum.TotalTokens != 16 || len(sum.Details) != 2 {
		t.Fatalf("add %+v", sum)
	}
	var sess SessionMetrics
	sess.AccumulateFromRun(parent)
	sess.AccumulateFromRun(child)
	if sess.TotalTokens != 16 {
		t.Fatal("session")
	}
	bg := NewRunMetrics()
	bg.AccumulateModel(usage(1, 1, 0), "m", "p", ModelTypeLearningModel)
	parent.MergeBackground(bg, nil)
	if parent.Snapshot().Details["learning_model"] == nil {
		t.Fatal("background merge")
	}
}
