package userfeedback

import "testing"

func TestAskUser(t *testing.T) {
	u := NewUserFeedbackTools()
	m, ok := u.GetMethods()["user_feedback_tools_AskUser"]
	if !ok || !m.StopAfterCall {
		t.Fatal("AskUser must be registered with StopAfterCall")
	}
	out, err := u.Execute("user_feedback_tools_AskUser", []byte(`{"questions":[{"question":"Where?","header":"Dest","options":[{"label":"Paris"},{"label":"Rome"}]}]}`))
	if err != nil || out != "User feedback received" {
		t.Fatalf("%v %v", out, err)
	}
	q := u.PendingQuestions()
	if len(q) != 1 || len(q[0].Options) != 2 || q[0].Header != "Dest" {
		t.Fatalf("pending %+v", q)
	}
	u.ClearPending()
	if len(u.PendingQuestions()) != 0 {
		t.Fatal("clear failed")
	}
}
