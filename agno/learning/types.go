package learning

import "time"

type ItemType string

const (
	ItemTypeFAQ       ItemType = "faq"
	ItemTypePattern   ItemType = "pattern"
	ItemTypeSnippet   ItemType = "snippet"
	ItemTypeDecision  ItemType = "decision"
	ItemTypeProcedure ItemType = "procedure"
)

type Status string

const (
	StatusCandidate  Status = "candidate"
	StatusVerified   Status = "verified"
	StatusDeprecated Status = "deprecated"
)

type DedupeAction string

const (
	DedupeActionSkip       DedupeAction = "skip"
	DedupeActionMerge      DedupeAction = "merge"
	DedupeActionNewVersion DedupeAction = "new_version"
)

type ObserveResult struct {
	Saved        bool
	Promoted     bool
	Skipped      bool
	Reason       string
	DocumentID   string
	Canonical    string
	CanonicalType ItemType
	Status       Status
	CreatedAt    time.Time
}
