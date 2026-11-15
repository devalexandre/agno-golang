// HyDE (Hypothetical Document Embeddings) query transformer on its own.
//
// HyDE asks a model for a plausible answer to the question and searches with
// that passage instead (or with question + passage when IncludeQuery is set),
// since an answer embeds closer to real answers than a question does. If the
// model call fails the original query is used, so search degrades instead of
// breaking. A custom Prompt must contain {query}; MaxCharacters caps the
// passage on a sentence/word break.
//
// Runs offline with an in-file stub model. For the full pipeline (HyDE +
// reranker on a real vector db) see cookbook/knowledge/rerank_hyde; there it
// is wired as kb.QueryTransformer = querytransformer.NewHyDE(model).
//
//	go run ./cookbook/knowledge/querytransformer/hyde
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/devalexandre/agno-golang/agno/knowledge/querytransformer"
	"github.com/devalexandre/agno-golang/agno/models"
)

func main() {
	ctx := context.Background()
	question := "How does Go handle concurrency?"

	// 1. Default: search with the hypothetical passage only.
	hyde := querytransformer.NewHyDE(stubModel{})
	q, err := hyde.Transform(ctx, question)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("default:\n%s\n\n", q)

	// 2. IncludeQuery keeps the question's own words (keyword/hybrid stores)
	//    and MaxCharacters trims the passage.
	hyde.IncludeQuery = true
	hyde.MaxCharacters = 80
	q, _ = hyde.Transform(ctx, question)
	fmt.Printf("include query, max 80 chars:\n%s\n\n", q)

	// 3. A custom prompt must contain {query}.
	bad := &querytransformer.HyDE{Model: stubModel{}, Prompt: "Answer this question."}
	_, err = bad.Transform(ctx, question)
	fmt.Println("prompt without {query}:", errors.Is(err, querytransformer.ErrInvalidPrompt))

	// 4. Model failure: the question is returned unchanged with a nil error.
	q, err = querytransformer.NewHyDE(failingModel{}).Transform(ctx, question)
	fmt.Printf("model failure -> %q (err=%v)\n", q, err)

	// Any func can be a transformer too.
	var t querytransformer.QueryTransformer = querytransformer.Func(func(_ context.Context, s string) (string, error) {
		return "golang " + s, nil
	})
	q, _ = t.Transform(ctx, question)
	fmt.Println("Func transformer:", q)
}

// stubModel writes a fixed "hypothetical" passage.
type stubModel struct{}

func (stubModel) GetID() string { return "stub-model" }
func (stubModel) Invoke(context.Context, []models.Message, ...models.Option) (*models.MessageResponse, error) {
	return &models.MessageResponse{Role: "assistant", Content: "Go handles concurrency with goroutines, lightweight threads scheduled by the runtime. " +
		"Goroutines communicate over channels, and the select statement waits on several channel operations at once."}, nil
}
func (m stubModel) AInvoke(ctx context.Context, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return asyncInvoke(ctx, m, msgs, opts...)
}
func (stubModel) InvokeStream(context.Context, []models.Message, ...models.Option) error { return nil }
func (m stubModel) AInvokeStream(ctx context.Context, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	return asyncInvoke(ctx, m, msgs, opts...)
}

// failingModel simulates a provider outage.
type failingModel struct{ stubModel }

func (failingModel) Invoke(context.Context, []models.Message, ...models.Option) (*models.MessageResponse, error) {
	return nil, errors.New("provider unavailable")
}

func asyncInvoke(ctx context.Context, m models.AgnoModelInterface, msgs []models.Message, opts ...models.Option) (<-chan *models.MessageResponse, <-chan error) {
	ch, errCh := make(chan *models.MessageResponse, 1), make(chan error, 1)
	if r, err := m.Invoke(ctx, msgs, opts...); err != nil {
		errCh <- err
	} else {
		ch <- r
	}
	close(ch)
	close(errCh)
	return ch, errCh
}
