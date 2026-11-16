// Typed errors and provider error classification (agno/exceptions).
//
// Shows how to classify model provider failures (rate limit, context window,
// non-retryable status codes), decide whether to retry, map errors to HTTP
// status codes (SessionBusyError -> 409), steer a run from a tool with
// StopAgentRun/RetryAgentRun, and redact secrets from error messages.
//
// Runs fully offline. Requires no env vars or services.
package main

import (
	"errors"
	"fmt"

	"github.com/devalexandre/agno-golang/agno/exceptions"
)

func callProvider(status int, msg string) error {
	return fmt.Errorf("openai call failed: %w", exceptions.NewModelProviderError(msg, status, "OpenAIChat", "gpt-4o"))
}

func main() {
	cases := []struct {
		status int
		msg    string
	}{
		{429, "Rate limit reached for requests"},
		{400, "This model's maximum context length is 128000 tokens"},
		{401, "Incorrect API key provided"},
		{503, "upstream unavailable"},
	}
	for _, c := range cases {
		err := callProvider(c.status, c.msg)
		var pe *exceptions.ModelProviderError
		errors.As(err, &pe)
		kind := exceptions.Classify(pe).Kind
		fmt.Printf("%d %-28s kind=%-30s retryable=%v rateLimit=%v contextWindow=%v\n",
			c.status, c.msg[:min(28, len(c.msg))], kind, exceptions.IsRetryable(err),
			exceptions.IsRateLimit(err), exceptions.IsContextWindowExceeded(err))
	}

	busy := exceptions.NewSessionBusyError("", "run-2", "session-1")
	fmt.Printf("\n%v -> HTTP %d (busy=%v)\n", busy, exceptions.HTTPStatusOf(busy), exceptions.IsSessionBusy(busy))

	stop := exceptions.NewStopAgentRun(errors.New("budget exhausted"), "", "I have to stop here.")
	fmt.Printf("tool returned %q -> stop=%v\n", stop, exceptions.IsStopAgentRun(stop))

	emb := exceptions.NewEmbeddingError("invalid_api_key: sk-proj-abcdefghijklmnopqrstuvwxyz", 401, "text-embedding-3-small", "openai")
	fmt.Printf("embedding error: reason=%s hint=%q safe=%q\n", emb.Reason(), emb.RecoveryHint(), emb.SafeMessage())
}
