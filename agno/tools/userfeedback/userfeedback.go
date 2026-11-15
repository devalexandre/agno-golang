// Package userfeedback ports Agno's UserFeedbackTools: the agent calls AskUser
// to present structured questions with predefined options.
//
// As in Python, the call does not run the tool: it pauses the agent run
// (human-in-the-loop "user feedback"). The paused RunResponse carries one
// RunRequirement whose UserFeedbackSchema holds the questions; answer them with
// RunRequirement.ProvideUserFeedback (question text -> selected labels) and
// resume with Agent.ContinueRun. The model then receives the selections as the
// tool result ("User feedback received: [...]").
package userfeedback

import (
	"fmt"

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
	u.Toolkit.RegisterWithOptions("AskUser", "Present structured questions with predefined options to the user.", u, u.AskUser, AskUserParams{}, toolkit.WithUserFeedback(QuestionsFromArgs))
	return u
}

// QuestionsFromArgs turns AskUser's arguments into the feedback questions of
// the paused run (Python's ask_user handling in Model).
func QuestionsFromArgs(args map[string]interface{}) []toolkit.UserFeedbackQuestionSpec {
	raw, _ := args["questions"].([]interface{})
	var out []toolkit.UserFeedbackQuestionSpec
	for _, item := range raw {
		q, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		spec := toolkit.UserFeedbackQuestionSpec{Question: str(q["question"]), Header: str(q["header"])}
		spec.MultiSelect, _ = q["multi_select"].(bool)
		opts, _ := q["options"].([]interface{})
		for _, o := range opts {
			if om, ok := o.(map[string]interface{}); ok {
				spec.Options = append(spec.Options, toolkit.UserFeedbackOptionSpec{Label: str(om["label"]), Description: str(om["description"])})
			}
		}
		out = append(out, spec)
	}
	return out
}

func str(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

// AskUser is only executed when the call has no questions (or outside an
// agent run); otherwise the agent pauses the run instead of calling it.
func (u *UserFeedbackTools) AskUser(p AskUserParams) (interface{}, error) {
	return "User feedback received", nil
}
