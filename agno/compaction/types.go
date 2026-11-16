package compaction

import (
	"encoding/json"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Record is what one compaction did: the summary and the anchor of the first
// message kept verbatim. It is an alias of models.CompactionRecord so it can
// be carried on models.RunResponse without an import cycle.
type Record = models.CompactionRecord

// Status explains why a compaction attempt ended the way it did.
type Status string

const (
	// StatusCompacted: the fold happened.
	StatusCompacted Status = "compacted"
	// StatusNotEnabled: compaction is not configured on this agent.
	StatusNotEnabled Status = "not_enabled"
	// StatusNoHistory: the session has no stored history yet.
	StatusNoHistory Status = "no_history"
	// StatusNothingToFold: the kept tail covers the whole conversation.
	StatusNothingToFold Status = "nothing_to_fold"
	// StatusAlreadyCompacted: a previous fold already covers everything up to the only safe cut.
	StatusAlreadyCompacted Status = "already_compacted"
	// StatusNotWorthIt: folding would cost more than it reclaims (EnforceMinFoldRatio only).
	StatusNotWorthIt Status = "not_worth_it"
	// StatusSummaryFailed: the summarizer returned nothing or failed.
	StatusSummaryFailed Status = "summary_failed"
)

// Result is the outcome of a compaction attempt.
type Result struct {
	Status  Status  `json:"status"`
	Message string  `json:"message"`
	Record  *Record `json:"record,omitempty"`
	// Kept is the history kept verbatim after the fold (the messages from the
	// boundary on). Only set when Status is StatusCompacted.
	Kept []models.Message `json:"-"`
}

// Compacted reports whether the fold happened.
func (r *Result) Compacted() bool { return r != nil && r.Status == StatusCompacted }

// Stats are running totals for one agent.
type Stats struct {
	Compactions       int `json:"compactions"`
	MessagesCompacted int `json:"messages_compacted"`
	TokensBefore      int `json:"tokens_before"`
	TokensAfter       int `json:"tokens_after"`
}

// Add accumulates a record into the stats.
func (s *Stats) Add(r *Record) {
	if r == nil {
		return
	}
	s.Compactions++
	s.MessagesCompacted += r.MessagesCompacted
	s.TokensBefore += r.TokensBefore
	s.TokensAfter += r.TokensAfter
}

// RecordToMap converts a record into a JSON-compatible map for session storage.
func RecordToMap(r *Record) map[string]interface{} {
	if r == nil {
		return nil
	}
	data, err := json.Marshal(r)
	if err != nil {
		return nil
	}
	out := map[string]interface{}{}
	_ = json.Unmarshal(data, &out)
	return out
}

// RecordFromValue rebuilds a record from a stored value (a map, a *Record or
// raw JSON). It returns nil when the value is not a usable record.
func RecordFromValue(v interface{}) *Record {
	switch val := v.(type) {
	case nil:
		return nil
	case *Record:
		return val
	case Record:
		return &val
	}
	var data []byte
	switch val := v.(type) {
	case []byte:
		data = val
	case string:
		data = []byte(val)
	default:
		var err error
		data, err = json.Marshal(val)
		if err != nil {
			return nil
		}
	}
	var r Record
	if err := json.Unmarshal(data, &r); err != nil || r.Summary == "" {
		return nil
	}
	return &r
}
