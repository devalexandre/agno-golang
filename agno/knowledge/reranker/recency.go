package reranker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/devalexandre/agno-golang/agno/vectordb"
)

const (
	// RecencyMetadataKey is the default metadata key holding a document's own
	// timestamp (Python RECENCY_METADATA_KEY).
	RecencyMetadataKey = "updated_at"
	// StoreRecencyMetadataKey is the fallback key a store may set with its own
	// last-modified time (Python STORE_RECENCY_METADATA_KEY).
	StoreRecencyMetadataKey = "_agno_updated_at"

	secondsPerDay = 86400.0
)

// RecencyReranker surfaces recently updated documents without discarding
// relevance. Port of agno-python knowledge/reranker/recency.py.
//
// It blends the search score with an exponential decay on a timestamp:
//
//	score = (1-Weight)*relevance + Weight*0.5^(ageDays/HalfLifeDays)
//
// The timestamp is read from Document.Metadata[TimestampKey] (default
// "updated_at"), falling back to Metadata["_agno_updated_at"]; it may be an
// RFC 3339/ISO-8601 string, a time.Time, or epoch seconds/milliseconds.
// Documents without a usable timestamp receive no recency term and rank purely
// on relevance. Results are shallow copies with the blended Score.
type RecencyReranker struct {
	BaseReranker
	// TimestampKey is the metadata key holding the document timestamp.
	TimestampKey string
	// HalfLifeDays is the age at which the recency term has decayed to half.
	// Values <= 0 fall back to 30.
	HalfLifeDays float64
	// Weight is 0.0 for relevance alone, 1.0 for age alone.
	Weight float64
	// UseDocumentUpdatedAt additionally falls back to Document.UpdatedAt when
	// no metadata timestamp exists. Off by default: NewDocument stamps
	// time.Now(), which would make every unpersisted document look fresh.
	UseDocumentUpdatedAt bool
	// Now overrides the clock (tests). Nil means time.Now.
	Now func() time.Time
}

// NewRecencyReranker creates a recency reranker with Python's defaults:
// "updated_at" timestamp key, 30-day half-life, weight 0.3, x3 candidates.
func NewRecencyReranker() *RecencyReranker {
	return &RecencyReranker{
		BaseReranker: BaseReranker{ID: "recency", Model: "recency"},
		TimestampKey: RecencyMetadataKey,
		HalfLifeDays: 30,
		Weight:       0.3,
	}
}

// Rerank implements Reranker.
func (r *RecencyReranker) Rerank(_ context.Context, _ string, results []*vectordb.SearchResult, limit int) ([]*vectordb.SearchResult, error) {
	if len(results) == 0 {
		return []*vectordb.SearchResult{}, nil
	}
	if r.Weight < 0 || r.Weight > 1 || math.IsNaN(r.Weight) {
		return nil, fmt.Errorf("%w: RecencyReranker weight must be between 0.0 and 1.0, got %v", ErrInvalidInput, r.Weight)
	}
	halfLife := r.HalfLifeDays
	if halfLife <= 0 || math.IsNaN(halfLife) {
		halfLife = 30
	}
	key := r.TimestampKey
	if key == "" {
		key = RecencyMetadataKey
	}
	nowFn := r.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := float64(nowFn().UnixNano()) / 1e9

	// Go results always carry a float Score, so "the store reported no score"
	// is detected as every Score being zero; then rank stands in for relevance
	// across the whole pool (never mixing scales).
	scores := make([]float64, len(results))
	useRank := true
	for i, res := range results {
		if res != nil {
			scores[i] = res.Score
			if res.Score != 0 {
				useRank = false
			}
		}
	}
	if !useRank {
		scores = rescaled(scores)
	}

	type entry struct {
		score    float64
		position int
		res      *vectordb.SearchResult
	}
	scored := make([]entry, 0, len(results))
	dated := 0
	for pos, res := range results {
		if res == nil {
			continue
		}
		relevance := scores[pos]
		if useRank {
			relevance = 1.0 / (1.0 + float64(pos))
		}
		score := (1.0 - r.Weight) * relevance
		if ts, ok := r.timestamp(res); ok {
			dated++
			ageDays := math.Max((now-ts)/secondsPerDay, 0)
			score += r.Weight * math.Exp(-math.Ln2*ageDays/halfLife)
		}
		scored = append(scored, entry{score, pos, res})
	}

	if dated == 0 {
		log.Printf("[RERANKER] RecencyReranker found no usable %q on any search result, so ordering is unchanged. Set it in metadata when adding content.", key)
	}

	// The original position breaks ties, so equal scores keep the vector db order.
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].position < scored[j].position
	})

	n := effectiveLimit(limit, r.TopN)
	if n <= 0 || n > len(scored) {
		n = len(scored)
	}
	out := make([]*vectordb.SearchResult, 0, n)
	for _, e := range scored[:n] {
		c := copyResult(e.res)
		c.Score = e.score
		out = append(out, c)
	}
	return out, nil
}

func (r *RecencyReranker) timestamp(res *vectordb.SearchResult) (float64, bool) {
	if res.Document == nil {
		return 0, false
	}
	key := r.TimestampKey
	if key == "" {
		key = RecencyMetadataKey
	}
	if md := res.Document.Metadata; md != nil {
		if ts, ok := asTimestamp(md[key]); ok {
			return ts, true
		}
		if ts, ok := asTimestamp(md[StoreRecencyMetadataKey]); ok {
			return ts, true
		}
	}
	if r.UseDocumentUpdatedAt && !res.Document.UpdatedAt.IsZero() {
		return float64(res.Document.UpdatedAt.UnixNano()) / 1e9, true
	}
	return 0, false
}

var isoLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04",
	"2006-01-02",
}

// asTimestamp reads a metadata value as a POSIX timestamp in seconds.
func asTimestamp(value interface{}) (float64, bool) {
	var f float64
	switch v := value.(type) {
	case nil, bool:
		return 0, false
	case time.Time:
		if v.IsZero() {
			return 0, false
		}
		return float64(v.UnixNano()) / 1e9, true
	case *time.Time:
		if v == nil || v.IsZero() {
			return 0, false
		}
		return float64(v.UnixNano()) / 1e9, true
	case int:
		f = float64(v)
	case int32:
		f = float64(v)
	case int64:
		f = float64(v)
	case uint:
		f = float64(v)
	case uint32:
		f = float64(v)
	case uint64:
		f = float64(v)
	case float32:
		f = float64(v)
	case float64:
		f = v
	case json.Number:
		parsed, err := v.Float64()
		if err != nil {
			return 0, false
		}
		f = parsed
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return 0, false
		}
		for _, layout := range isoLayouts {
			// Naive timestamps are read as UTC, like Python.
			if t, err := time.ParseInLocation(layout, text, time.UTC); err == nil {
				return float64(t.UnixNano()) / 1e9, true
			}
		}
		return 0, false
	default:
		return 0, false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	// Milliseconds are common in JSON payloads and are centuries away in seconds.
	if f > 1e11 {
		f /= 1000.0
	}
	return f, true
}

// rescaled brings out-of-range scores onto 0-1 by dividing by the largest,
// preserving gaps (min-max would exaggerate narrow relevance gaps).
func rescaled(scores []float64) []float64 {
	if len(scores) == 0 {
		return scores
	}
	lowest, highest := scores[0], scores[0]
	for _, s := range scores[1:] {
		lowest = math.Min(lowest, s)
		highest = math.Max(highest, s)
	}
	if lowest >= 0 && highest <= 1 {
		return scores
	}
	out := make([]float64, len(scores))
	if highest <= 0 {
		return out
	}
	for i, s := range scores {
		out[i] = math.Max(s, 0) / highest
	}
	return out
}

var _ Reranker = (*RecencyReranker)(nil)
