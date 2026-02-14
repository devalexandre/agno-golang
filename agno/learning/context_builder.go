package learning

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge"
)

type ContextBuilderConfig struct {
	MaxItems      int
	MaxLineChars  int
	MaxTotalChars int
	IncludeStatus bool
}

func DefaultContextBuilderConfig() ContextBuilderConfig {
	return ContextBuilderConfig{
		MaxItems:      6,
		MaxLineChars:  260,
		MaxTotalChars: 1500,
		IncludeStatus: true,
	}
}

func BuildContext(userID string, results []*knowledge.SearchResult, cfg ContextBuilderConfig) string {
	ctx, _ := BuildContextWithSelection(userID, results, cfg)
	return ctx
}

func BuildContextWithSelection(userID string, results []*knowledge.SearchResult, cfg ContextBuilderConfig) (string, []*document.Document) {
	type row struct {
		doc          *document.Document
		score        float64
		status       string
		statusWeight int
		updated      int64
		hits         int
	}

	var rows []row

	for _, r := range results {
		if r == nil || r.Document == nil {
			continue
		}
		if !isLearningDocForUser(r.Document, userID) {
			continue
		}

		status := getMetaString(r.Document.Metadata, metaStatusKey)
		if status == "" {
			status = string(StatusCandidate)
		}
		if status == string(StatusDeprecated) {
			continue
		}

		updated := int64(0)
		if v := getMetaString(r.Document.Metadata, metaUpdatedAtKey); v != "" {
			// meta is unix seconds; parse loosely
			if parsed, err := parseUnix(v); err == nil {
				updated = parsed
			}
		}
		if updated == 0 {
			if v := getMetaString(r.Document.Metadata, metaCreatedAtKey); v != "" {
				if parsed, err := parseUnix(v); err == nil {
					updated = parsed
				}
			}
		}

		statusWeight := 0
		switch status {
		case string(StatusVerified):
			statusWeight = 2
		case string(StatusCandidate):
			statusWeight = 1
		}

		hits := 0
		if v := getMetaString(r.Document.Metadata, metaHitsKey); v != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				hits = parsed
			}
		}
		rows = append(rows, row{
			doc:          r.Document,
			score:        r.Score,
			status:       status,
			statusWeight: statusWeight,
			updated:      updated,
			hits:         hits,
		})
	}

	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].statusWeight != rows[j].statusWeight {
			return rows[i].statusWeight > rows[j].statusWeight
		}
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		if rows[i].updated != rows[j].updated {
			return rows[i].updated > rows[j].updated
		}
		return rows[i].hits > rows[j].hits
	})

	var selected []*document.Document
	var lines []string
	totalChars := 0

	for _, rr := range rows {
		line := oneLineSummary(rr.doc, cfg)
		if line == "" {
			continue
		}
		entry := "- " + line
		if cfg.MaxTotalChars > 0 && totalChars+len(entry)+1 > cfg.MaxTotalChars {
			break
		}
		lines = append(lines, entry)
		selected = append(selected, rr.doc)
		totalChars += len(entry) + 1
		if cfg.MaxItems > 0 && len(lines) >= cfg.MaxItems {
			break
		}
	}

	if len(lines) == 0 {
		return "", nil
	}

	return fmt.Sprintf("<learning_memories>\nRelevant memories (from your history):\n%s\n</learning_memories>\n", strings.Join(lines, "\n")), selected
}

func isLearningDocForUser(doc *document.Document, userID string) bool {
	if doc == nil || doc.Metadata == nil {
		return false
	}
	if getMetaString(doc.Metadata, metaNamespaceKey) != metaNamespaceValue {
		return false
	}
	if userID != "" && getMetaString(doc.Metadata, metaUserIDKey) != userID {
		return false
	}
	status := getMetaString(doc.Metadata, metaStatusKey)
	if status == string(StatusDeprecated) {
		return false
	}
	return true
}

func oneLineSummary(doc *document.Document, cfg ContextBuilderConfig) string {
	title := strings.TrimSpace(doc.Name)
	if title == "" {
		title = firstNonEmptyLine(doc.Content)
	}

	summary := firstSummaryLine(doc.Content)
	if summary != "" && summary != title {
		title = fmt.Sprintf("%s — %s", title, summary)
	}

	s := title

	if cfg.IncludeStatus {
		status := getMetaString(doc.Metadata, metaStatusKey)
		if status == "" {
			status = string(StatusCandidate)
		}
		typ := getMetaString(doc.Metadata, metaTypeKey)
		if typ != "" {
			s = fmt.Sprintf("[%s/%s] %s", status, typ, s)
		} else {
			s = fmt.Sprintf("[%s] %s", status, s)
		}
	}

	return clampLen(s, cfg.MaxLineChars)
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		t = strings.TrimPrefix(t, "- ")
		return t
	}
	return ""
}

func firstSummaryLine(s string) string {
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "```") {
			continue
		}
		if strings.EqualFold(t, "- snippet:") || strings.EqualFold(t, "snippet:") {
			continue
		}
		if strings.HasPrefix(t, "- ") {
			t = strings.TrimSpace(strings.TrimPrefix(t, "- "))
		}
		if t == "" {
			continue
		}
		return t
	}
	return ""
}

func getMetaString(meta map[string]interface{}, key string) string {
	if meta == nil {
		return ""
	}
	v, ok := meta[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func parseUnix(s string) (int64, error) {
	// Support int-like values and float-like strings.
	if strings.Contains(s, ".") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f), nil
		}
	}
	return strconv.ParseInt(s, 10, 64)
}
