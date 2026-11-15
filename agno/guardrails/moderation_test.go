package guardrails

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// moderationServer serves a canned moderations response and records the request.
func moderationServer(t *testing.T, status int, body string, captured *map[string]interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if captured != nil {
			raw, _ := io.ReadAll(r.Body)
			decoded := map[string]interface{}{}
			_ = json.Unmarshal(raw, &decoded)
			decoded["_authorization"] = r.Header.Get("Authorization")
			decoded["_path"] = r.URL.Path
			*captured = decoded
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
}

const flaggedBody = `{
  "results": [{
    "flagged": true,
    "categories": {"violence": true, "hate": false, "sexual": false},
    "category_scores": {"violence": 0.91, "hate": 0.01, "sexual": 0.02}
  }]
}`

const cleanBody = `{
  "results": [{
    "flagged": false,
    "categories": {"violence": false, "hate": false},
    "category_scores": {"violence": 0.01, "hate": 0.0}
  }]
}`

func testModerator(server *httptest.Server) *OpenAIModerator {
	return NewOpenAIModerator(
		WithModerationAPIKey("test-key"),
		WithModerationBaseURL(server.URL+"/v1/moderations"),
		WithModerationHTTPClient(server.Client()),
	)
}

func TestOpenAIModeratorRequest(t *testing.T) {
	var captured map[string]interface{}
	server := moderationServer(t, http.StatusOK, flaggedBody, &captured)
	defer server.Close()

	moderator := testModerator(server)
	result, err := moderator.Moderate(context.Background(), "violent content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Flagged {
		t.Error("expected the result to be flagged")
	}
	if got := result.FlaggedCategories(); len(got) != 1 || got[0] != CategoryViolence {
		t.Errorf("unexpected flagged categories: %v", got)
	}
	if result.CategoryScores[CategoryViolence] != 0.91 {
		t.Errorf("unexpected score: %v", result.CategoryScores)
	}

	if captured["model"] != DefaultOpenAIModerationModel {
		t.Errorf("unexpected model: %v", captured["model"])
	}
	if captured["input"] != "violent content" {
		t.Errorf("unexpected input: %v", captured["input"])
	}
	if captured["_authorization"] != "Bearer test-key" {
		t.Errorf("unexpected authorization header: %v", captured["_authorization"])
	}
}

func TestModerationGuardrailFlagged(t *testing.T) {
	server := moderationServer(t, http.StatusOK, flaggedBody, nil)
	defer server.Close()

	g := NewModerationGuardrail(testModerator(server))
	err := g.Check(context.Background(), "violent content")
	if err == nil {
		t.Fatal("expected the check to fail")
	}
	checkErr, ok := AsCheckError(err)
	if !ok {
		t.Fatalf("expected *CheckError, got %T", err)
	}
	if checkErr.Trigger != TriggerModerationViolation {
		t.Errorf("unexpected trigger: %q", checkErr.Trigger)
	}
	flagged, _ := checkErr.AdditionalData["flagged_category"].([]string)
	if len(flagged) != 1 || flagged[0] != CategoryViolence {
		t.Errorf("unexpected flagged categories: %v", checkErr.AdditionalData)
	}
}

func TestModerationGuardrailClean(t *testing.T) {
	server := moderationServer(t, http.StatusOK, cleanBody, nil)
	defer server.Close()

	g := NewModerationGuardrail(testModerator(server))
	if err := g.Check(context.Background(), "hello there"); err != nil {
		t.Fatalf("expected clean content to pass, got %v", err)
	}
}

func TestModerationGuardrailRaiseForCategories(t *testing.T) {
	server := moderationServer(t, http.StatusOK, flaggedBody, nil)
	defer server.Close()

	moderator := testModerator(server)

	ignored := NewModerationGuardrail(moderator, WithRaiseForCategories(CategoryHate))
	if err := ignored.Check(context.Background(), "violent content"); err != nil {
		t.Errorf("violence must be ignored when only hate is watched, got %v", err)
	}

	watched := NewModerationGuardrail(moderator, WithRaiseForCategories(CategoryViolence))
	if err := watched.Check(context.Background(), "violent content"); err == nil {
		t.Error("expected the watched category to fail the check")
	}
}

func TestModerationGuardrailThreshold(t *testing.T) {
	server := moderationServer(t, http.StatusOK, cleanBody, nil)
	defer server.Close()

	// The provider did not flag the content, but the score is above the threshold.
	g := NewModerationGuardrail(testModerator(server), WithModerationThreshold(0.005))
	if err := g.Check(context.Background(), "borderline"); err == nil {
		t.Error("expected the threshold to fail the check")
	}
}

func TestModerationGuardrailProviderError(t *testing.T) {
	server := moderationServer(t, http.StatusInternalServerError, `{"error": {"message": "boom"}}`, nil)
	defer server.Close()

	g := NewModerationGuardrail(testModerator(server))
	err := g.Check(context.Background(), "content")
	if err == nil {
		t.Fatal("expected the provider error to surface")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected the status code in the error, got %v", err)
	}

	failOpen := NewModerationGuardrail(testModerator(server), WithModerationFailOpen())
	if err := failOpen.Check(context.Background(), "content"); err != nil {
		t.Errorf("fail-open must let the content through, got %v", err)
	}
}

func TestModerationGuardrailMissingAPIKey(t *testing.T) {
	moderator := NewOpenAIModerator(WithModerationAPIKey(""))
	if _, err := moderator.Moderate(context.Background(), "content"); err == nil {
		t.Fatal("expected an error when the API key is missing")
	}
}

func TestModerationGuardrailEmptyContent(t *testing.T) {
	server := moderationServer(t, http.StatusInternalServerError, "boom", nil)
	defer server.Close()

	// Blank content must not reach the provider.
	g := NewModerationGuardrail(testModerator(server))
	if err := g.Check(context.Background(), "   "); err != nil {
		t.Errorf("blank content must pass, got %v", err)
	}
}
