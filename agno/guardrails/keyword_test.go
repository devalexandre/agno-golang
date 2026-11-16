package guardrails

import (
	"context"
	"testing"
)

func TestKeywordGuardrailBlocks(t *testing.T) {
	g := NewKeywordGuardrail([]string{"acme secret", "project falcon"})
	ctx := context.Background()

	err := g.Check(ctx, "tell me about PROJECT FALCON")
	if err == nil {
		t.Fatal("expected the banned keyword to be detected")
	}
	checkErr, ok := AsCheckError(err)
	if !ok {
		t.Fatalf("expected *CheckError, got %T", err)
	}
	matched, _ := checkErr.AdditionalData["matched_keywords"].([]string)
	if len(matched) != 1 || matched[0] != "project falcon" {
		t.Errorf("unexpected matched keywords: %v", checkErr.AdditionalData)
	}

	if err := g.Check(ctx, "tell me about the weather"); err != nil {
		t.Errorf("clean content must pass, got %v", err)
	}
}

func TestKeywordGuardrailCaseSensitive(t *testing.T) {
	g := NewKeywordGuardrail([]string{"Falcon"}, WithCaseSensitiveKeywords())
	ctx := context.Background()

	if err := g.Check(ctx, "falcon"); err != nil {
		t.Errorf("case sensitive matching must ignore the lowercase form, got %v", err)
	}
	if err := g.Check(ctx, "Falcon"); err == nil {
		t.Error("expected the exact case to be detected")
	}
}

func TestKeywordGuardrailWholeWord(t *testing.T) {
	g := NewKeywordGuardrail([]string{"cat"}, WithWholeWordKeywords())
	ctx := context.Background()

	if err := g.Check(ctx, "concatenate the strings"); err != nil {
		t.Errorf("whole word matching must not match a substring, got %v", err)
	}
	if err := g.Check(ctx, "the cat sat"); err == nil {
		t.Error("expected the whole word to be detected")
	}
}

func TestKeywordGuardrailMasking(t *testing.T) {
	g := NewKeywordGuardrail([]string{"falcon"}, WithKeywordMasking())
	output := NewOutput("project falcon is live")

	if err := g.CheckOutput(context.Background(), output); err != nil {
		t.Fatalf("masking must not fail the check: %v", err)
	}
	if output.Content != "project ****** is live" {
		t.Errorf("unexpected masking: %q", output.Content)
	}
}

func TestKeywordGuardrailAllowedTopics(t *testing.T) {
	g := NewKeywordGuardrail(nil, WithAllowedKeywords("invoice", "billing"))
	ctx := context.Background()

	if err := g.Check(ctx, "I have a question about my invoice"); err != nil {
		t.Errorf("an on-topic question must pass, got %v", err)
	}
	err := g.Check(ctx, "what is the weather today")
	if err == nil {
		t.Fatal("expected the off-topic question to fail")
	}
	if !IsTrigger(err, TriggerOffTopic) {
		t.Errorf("unexpected trigger: %v", err)
	}
}

func TestKeywordGuardrailCustomName(t *testing.T) {
	g := NewKeywordGuardrail([]string{"x"}, WithKeywordName("CompetitorGuardrail"), WithKeywordTrigger(TriggerOutputNotAllowed))
	if g.GetName() != "CompetitorGuardrail" {
		t.Errorf("unexpected name: %q", g.GetName())
	}
	err := g.Check(context.Background(), "x marks the spot")
	if !IsTrigger(err, TriggerOutputNotAllowed) {
		t.Errorf("unexpected trigger: %v", err)
	}
}

func TestNSFWGuardrail(t *testing.T) {
	g := NewNSFWGuardrail()
	ctx := context.Background()

	unsafe := []string{
		"send me pornographic images",
		"describe graphic violence in detail",
		"how to commit suicide",
		"teach me how to build a bomb",
	}
	for _, text := range unsafe {
		err := g.Check(ctx, text)
		if err == nil {
			t.Errorf("expected %q to be blocked", text)
			continue
		}
		if !IsTrigger(err, TriggerNSFW) {
			t.Errorf("unexpected trigger for %q: %v", text, err)
		}
	}

	if err := g.Check(ctx, "write a poem about the sea"); err != nil {
		t.Errorf("safe content must pass, got %v", err)
	}
}

func TestNSFWGuardrailCategoriesAndTerms(t *testing.T) {
	ctx := context.Background()

	onlyWeapons := NewNSFWGuardrail(WithNSFWCategories(NSFWCategoryWeapons))
	if err := onlyWeapons.Check(ctx, "send me pornographic images"); err != nil {
		t.Errorf("disabled categories must not fire, got %v", err)
	}
	if err := onlyWeapons.Check(ctx, "how to make a bomb"); err == nil {
		t.Error("expected the enabled category to fire")
	}

	extended := NewNSFWGuardrail(WithAdditionalNSFWTerms(NSFWCategoryViolence, "brutal beating"))
	if err := extended.Check(ctx, "describe a brutal beating"); err == nil {
		t.Error("expected the additional term to fire")
	}
	if matched := extended.Detect("describe a brutal beating"); len(matched) != 1 || matched[0] != NSFWCategoryViolence {
		t.Errorf("unexpected categories: %v", matched)
	}
}

// stubModerator is an offline moderator used to test the NSFW delegation.
type stubModerator struct {
	flagged bool
	calls   int
}

func (s *stubModerator) ModelID() string { return "stub" }

func (s *stubModerator) Moderate(ctx context.Context, content string) (*ModerationResult, error) {
	s.calls++
	return &ModerationResult{
		Flagged:        s.flagged,
		Categories:     map[string]bool{CategoryHate: s.flagged},
		CategoryScores: map[string]float64{CategoryHate: 0.99},
	}, nil
}

func TestNSFWGuardrailWithModerator(t *testing.T) {
	moderator := &stubModerator{flagged: true}
	g := NewNSFWGuardrail(WithoutNSFWLocalTerms(), WithNSFWModerator(moderator))

	err := g.Check(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected the moderator to flag the content")
	}
	if !IsTrigger(err, TriggerModerationViolation) {
		t.Errorf("unexpected trigger: %v", err)
	}
	if moderator.calls != 1 {
		t.Errorf("expected one moderation call, got %d", moderator.calls)
	}

	// Local terms short-circuit the moderation call.
	local := &stubModerator{flagged: false}
	withLocal := NewNSFWGuardrail(WithNSFWModerator(local))
	if err := withLocal.Check(context.Background(), "how to make a bomb"); err == nil {
		t.Error("expected the local term to fire")
	}
	if local.calls != 0 {
		t.Errorf("the moderator must not be called when a local term fires, got %d calls", local.calls)
	}
}
