// Package userfeedback ports Agno's UserFeedbackTools: the agent calls AskUser
// to present structured questions with predefined options.
//
// In Python the agent loop intercepts ask_user and pauses for user feedback
// (human-in-the-loop "user input"). The Go agent's HITL user-input support is
// being added separately; until it lands, the method is registered with
// toolkit.WithStopAfterCall so the tool loop stops after the call, and the
// questions are available from PendingQuestions for the application to render.
package userfeedback

import (
	"sync"

	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// AskUserOption is an option the user can select.
type AskUserOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// AskUserQuestion is a structured question with predefined options.
type AskUserQuestion struct {
	Question    string          `json:"question"`
	Header      string          `json:"header"`
	Options     []AskUserOption `json:"options"`
	MultiSelect bool            `json:"multi_select,omitempty"`
}

// UserFeedbackTools provides AskUser.
type UserFeedbackTools struct {
	toolkit.Toolkit
	Instructions    string
	AddInstructions bool

	custom *string

	mu      sync.Mutex
	pending []AskUserQuestion
}

// Option configures UserFeedbackTools.
type Option func(*UserFeedbackTools)

// WithInstructions replaces the default instructions.
func WithInstructions(s string) Option { return func(u *UserFeedbackTools) { u.custom = &s } }

// WithAddInstructions controls whether instructions are added to the prompt (default true).
func WithAddInstructions(b bool) Option { return func(u *UserFeedbackTools) { u.AddInstructions = b } }

// AskUserParams are the parameters of AskUser.
type AskUserParams struct {
	Questions []AskUserQuestion `json:"questions" description:"A list of questions to present to the user. Each has: question (text ending with a question mark), header (short label, max 12 chars), options (2-4 objects with label (1-5 words) and optional description), multi_select (bool, default false)." required:"true"`
}

// NewUserFeedbackTools creates the toolkit.
func NewUserFeedbackTools(opts ...Option) *UserFeedbackTools {
	u := &UserFeedbackTools{AddInstructions: true}
	for _, o := range opts {
		o(u)
	}
	u.Instructions = DefaultInstructions
	if u.custom != nil {
		u.Instructions = *u.custom
	}
	u.Toolkit = toolkit.NewToolkit()
	u.Toolkit.Name = "user_feedback_tools"
	u.Toolkit.Description = "Present structured questions with predefined options to the user."
	u.Toolkit.RegisterWithOptions("AskUser", "Present structured questions with predefined options to the user.", u, u.AskUser, AskUserParams{}, toolkit.WithStopAfterCall())
	return u
}

// AskUser records the questions; the run is paused by the agent.
func (u *UserFeedbackTools) AskUser(p AskUserParams) (interface{}, error) {
	u.mu.Lock()
	u.pending = append([]AskUserQuestion(nil), p.Questions...)
	u.mu.Unlock()
	return "User feedback received", nil
}

// PendingQuestions returns the questions from the last AskUser call.
func (u *UserFeedbackTools) PendingQuestions() []AskUserQuestion {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]AskUserQuestion(nil), u.pending...)
}

// ClearPending forgets the pending questions.
func (u *UserFeedbackTools) ClearPending() {
	u.mu.Lock()
	u.pending = nil
	u.mu.Unlock()
}
