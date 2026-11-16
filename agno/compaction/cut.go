package compaction

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/devalexandre/agno-golang/agno/models"
)

// Boundary selection: pair-safe, anchor-durable cut points. All functions are
// pure over an in-memory message list (port of Python compaction/_cut.py).

func isLeadingRole(role models.Role) bool {
	return role == models.TypeSystemRole || role == "developer"
}

// LeadingSystemCount is the number of leading system/developer messages.
func LeadingSystemCount(messages []models.Message) int {
	n := 0
	for _, m := range messages {
		if !isLeadingRole(m.Role) {
			break
		}
		n++
	}
	return n
}

// IsInjectedSummary reports whether m is a summary message injected by
// compaction. It is never a boundary candidate and never counted as foldable.
func IsInjectedSummary(m models.Message) bool {
	return strings.HasPrefix(m.Content, SummaryPrefix)
}

// MessageFingerprint is a stable identifier for a message. Go messages carry
// no id, so the boundary anchor is a hash of the message's role, content and
// tool-call identity. It resolves to the same message every time history is
// rebuilt from stored runs, or fails to resolve at all (fail-open).
func MessageFingerprint(m models.Message) string {
	h := sha256.New()
	h.Write([]byte(m.Role))
	h.Write([]byte{0})
	h.Write([]byte(m.Content))
	h.Write([]byte{0})
	if m.ToolCallID != nil {
		h.Write([]byte(*m.ToolCallID))
	}
	for _, tc := range m.ToolCalls {
		h.Write([]byte{0})
		h.Write([]byte(tc.ID))
		h.Write([]byte(tc.Function.Name))
	}
	return "msg_" + hex.EncodeToString(h.Sum(nil))[:24]
}

// FindMessage returns the index of the first message at or after start whose
// fingerprint is id, or -1.
func FindMessage(messages []models.Message, id string, start int) int {
	if id == "" {
		return -1
	}
	for i := start; i < len(messages); i++ {
		if MessageFingerprint(messages[i]) == id {
			return i
		}
	}
	return -1
}

// keepTailStart is the index of the first message of a kept tail of about
// keepTokens, walking backward from the end. Returns start when all fits.
func keepTailStart(messages []models.Message, keepTokens, start int) int {
	acc := 0
	idx := len(messages)
	for idx > start {
		cand := messages[idx-1]
		if !IsInjectedSummary(cand) {
			acc += EstimateMessageTokens(cand)
			if acc > keepTokens {
				break
			}
		}
		idx--
	}
	return idx
}

// owningBatchHead returns the index of the assistant message owning the tool
// message at index, or -1.
func owningBatchHead(messages []models.Message, index int) int {
	id := messages[index].ToolCallID
	if id == nil || *id == "" {
		return -1
	}
	for i := index - 1; i >= 0; i-- {
		for _, tc := range messages[i].ToolCalls {
			if tc.ID == *id {
				return i
			}
		}
	}
	return -1
}

func earliestOrphanHead(messages []models.Message, boundary int) int {
	earliest := -1
	for i := boundary; i < len(messages); i++ {
		if messages[i].Role != models.TypeToolRole {
			continue
		}
		head := owningBatchHead(messages, i)
		if head >= 0 && head < boundary && (earliest < 0 || head < earliest) {
			earliest = head
		}
	}
	return earliest
}

// ChooseBoundary chooses the index of the first message kept verbatim.
//
// With keepFromIndex >= 0 the tail starts there; otherwise a tail of about
// keepTokens is accumulated walking backward. The boundary then snaps toward
// minIndex to a pair-safe user/assistant message: a tool result never starts
// the tail and a tool batch whose head would fall behind the cut moves into
// the tail whole. Returns -1 when no valid boundary exists after minIndex.
func ChooseBoundary(messages []models.Message, keepTokens, keepFromIndex, minIndex int) int {
	lead := LeadingSystemCount(messages)
	floor := minIndex
	if lead > floor {
		floor = lead
	}
	var cand int
	if keepFromIndex >= 0 {
		cand = keepFromIndex
		if cand < floor {
			cand = floor
		}
	} else {
		cand = keepTailStart(messages, keepTokens, floor)
	}
	if cand <= floor {
		return -1
	}
	if cand > len(messages)-1 {
		cand = len(messages) - 1
	}
	idx := cand
	for idx > floor {
		m := messages[idx]
		if m.Role == models.TypeToolRole {
			head := owningBatchHead(messages, idx)
			if head >= floor {
				idx = head
			} else {
				idx--
			}
			continue
		}
		if isLeadingRole(m.Role) || IsInjectedSummary(m) {
			idx--
			continue
		}
		if orphan := earliestOrphanHead(messages, idx); orphan >= 0 {
			idx = orphan
			continue
		}
		return idx
	}
	return -1
}

// newestUserIndex returns the index of the last user message, or -1.
func newestUserIndex(messages []models.Message) int {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == models.TypeUserRole && !IsInjectedSummary(messages[i]) {
			return i
		}
	}
	return -1
}

// outsideNewestExchange moves cut back to the newest user message when it
// would fold inside that exchange.
func outsideNewestExchange(messages []models.Message, cut, minIndex int) int {
	newest := newestUserIndex(messages)
	if cut >= 0 && newest >= 0 && cut > newest {
		return ChooseBoundary(messages, 0, newest, minIndex)
	}
	return cut
}
