package learning

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devalexandre/agno-golang/agno/document"
	"github.com/devalexandre/agno-golang/agno/knowledge"
	"github.com/google/uuid"
)

const (
	metaNamespaceKey   = "learning_namespace"
	metaNamespaceValue = "learning"

	metaUserIDKey  = "learning_user_id"
	metaTypeKey    = "learning_type"
	metaTopicKey   = "learning_topic"
	metaTagsKey    = "learning_tags"
	metaVersionKey = "learning_version"
	metaStreakKey  = "learning_streak"

	metaStatusKey     = "learning_status"
	metaConfidenceKey = "learning_confidence"
	metaHitsKey       = "learning_hits"
	metaUpdatedAtKey  = "learning_updated_at"
	metaCreatedAtKey  = "learning_created_at"

	metaSimHashKey = "learning_simhash64"

	metaSourceKey            = "learning_source"
	metaSourceSessionIDKey   = "learning_source_session_id"
	metaSourceUserMsgIDKey   = "learning_source_user_message_id"
	metaSourceAsstMsgIDKey   = "learning_source_assistant_message_id"
	metaPromotedFromDocIDKey = "learning_promoted_from_document_id"

	metaDeprecatedByDocIDKey = "learning_deprecated_by_document_id"
	metaMergedFromDocIDKey   = "learning_merged_from_document_id"
)

type ManagerConfig struct {
	TopK                        int
	DedupeTopK                  int
	DedupeMaxHamming            int
	WriteGate                   WriteGateConfig
	Canonicalize                CanonicalizeConfig
	ContextBuilder              ContextBuilderConfig
	AutoPromoteStreak           int
	AutoPromoteHits             int
	AutoPromoteConfidenceStreak float64
	AutoPromoteConfidenceHits   float64
}

func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		TopK:                        6,
		DedupeTopK:                  5,
		DedupeMaxHamming:            5,
		WriteGate:                   DefaultWriteGateConfig(),
		Canonicalize:                DefaultCanonicalizeConfig(),
		ContextBuilder:              DefaultContextBuilderConfig(),
		AutoPromoteStreak:           3,
		AutoPromoteHits:             5,
		AutoPromoteConfidenceStreak: 0.75,
		AutoPromoteConfidenceHits:   0.68,
	}
}

type Manager struct {
	store         knowledge.Knowledge
	cfg           ManagerConfig
	retrievedMu   sync.Mutex
	lastRetrieved map[string][]*document.Document
}

func NewManager(store knowledge.Knowledge, cfg ManagerConfig) *Manager {
	if cfg.TopK <= 0 {
		cfg.TopK = DefaultManagerConfig().TopK
	}
	if cfg.DedupeTopK <= 0 {
		cfg.DedupeTopK = DefaultManagerConfig().DedupeTopK
	}
	if cfg.DedupeMaxHamming <= 0 {
		cfg.DedupeMaxHamming = DefaultManagerConfig().DedupeMaxHamming
	}
	if cfg.WriteGate.MaxCanonicalChars <= 0 {
		cfg.WriteGate = DefaultWriteGateConfig()
	}
	if cfg.Canonicalize.MaxTotalChars <= 0 {
		cfg.Canonicalize = DefaultCanonicalizeConfig()
	}
	if cfg.ContextBuilder.MaxItems <= 0 {
		cfg.ContextBuilder = DefaultContextBuilderConfig()
	}
	if cfg.AutoPromoteStreak <= 0 {
		cfg.AutoPromoteStreak = DefaultManagerConfig().AutoPromoteStreak
	}
	if cfg.AutoPromoteHits <= 0 {
		cfg.AutoPromoteHits = DefaultManagerConfig().AutoPromoteHits
	}
	if cfg.AutoPromoteConfidenceStreak <= 0 {
		cfg.AutoPromoteConfidenceStreak = DefaultManagerConfig().AutoPromoteConfidenceStreak
	}
	if cfg.AutoPromoteConfidenceHits <= 0 {
		cfg.AutoPromoteConfidenceHits = DefaultManagerConfig().AutoPromoteConfidenceHits
	}

	return &Manager{
		store:         store,
		cfg:           cfg,
		lastRetrieved: make(map[string][]*document.Document),
	}
}

func (m *Manager) RetrieveContext(ctx context.Context, userID, query string) (string, error) {
	return m.RetrieveContextWithFilters(ctx, userID, query, nil)
}

func (m *Manager) RetrieveContextWithFilters(ctx context.Context, userID, query string, filters map[string]interface{}) (string, error) {
	contextStr, _, err := m.RetrieveContextWithMeta(ctx, userID, query, filters)
	return contextStr, err
}

func (m *Manager) RetrieveContextWithMeta(ctx context.Context, userID, query string, filters map[string]interface{}) (string, []string, error) {
	if m == nil || m.store == nil {
		return "", nil, nil
	}
	if strings.TrimSpace(query) == "" {
		return "", nil, nil
	}
	results, err := searchKnowledge(ctx, m.store, query, m.cfg.TopK, buildSearchFilters(userID, filters))
	if err != nil {
		return "", nil, err
	}

	contextStr, selected := BuildContextWithSelection(userID, results, m.cfg.ContextBuilder)
	m.setLastRetrieved(userID, selected)
	var ids []string
	for _, doc := range selected {
		if doc == nil || doc.ID == "" {
			continue
		}
		ids = append(ids, doc.ID)
	}
	if len(selected) > 0 {
		_ = m.incrementHits(ctx, selected)
	}
	return contextStr, ids, nil
}

func (m *Manager) ObserveAndLearn(ctx context.Context, userID, userMsg, assistantMsg string, meta map[string]interface{}) error {
	_, err := m.ObserveAndLearnResult(ctx, userID, userMsg, assistantMsg, meta)
	return err
}

func (m *Manager) ObserveAndLearnResult(ctx context.Context, userID, userMsg, assistantMsg string, meta map[string]interface{}) (ObserveResult, error) {
	if m == nil || m.store == nil {
		return ObserveResult{Skipped: true, Reason: "no_store"}, nil
	}
	if strings.TrimSpace(userID) == "" {
		return ObserveResult{Skipped: true, Reason: "no_user_id"}, nil
	}

	prevAssistant := getMetaString(meta, "previous_assistant_msg")
	if IsUserRejection(userMsg) && strings.TrimSpace(prevAssistant) != "" {
		return m.deprecatePrevious(ctx, userID, meta)
	}
	if IsUserConfirmation(userMsg) && strings.TrimSpace(prevAssistant) != "" {
		return m.promoteCandidate(ctx, userID, meta)
	}
	if getMetaBool(meta, "validation_passed") && strings.TrimSpace(prevAssistant) != "" {
		return m.promoteCandidate(ctx, userID, meta)
	}

	if err := m.applyEvidence(ctx, userID, meta); err != nil {
		// Best-effort: do not fail the run due to evidence updates.
	}

	c := Canonicalize(userMsg, assistantMsg, m.cfg.Canonicalize)
	if strings.TrimSpace(c.Content) == "" {
		return ObserveResult{Skipped: true, Reason: "no_canonical"}, nil
	}

	decision := ShouldWrite(userMsg, assistantMsg, c.Content, m.cfg.WriteGate)
	if !decision.Allow {
		return ObserveResult{Skipped: true, Reason: decision.Reason, Canonical: c.Content, CanonicalType: c.Type}, nil
	}

	hash := simHash64(c.Content)
	filters := buildSearchFilters(userID, getMetaMap(meta, "knowledge_filters"))
	if match, ok := m.findDuplicate(ctx, userID, c.Content, hash, filters); ok {
		if match.exact {
			return ObserveResult{Skipped: true, Reason: "duplicate_exact", DocumentID: match.doc.ID, Canonical: c.Content, CanonicalType: c.Type}, nil
		}

		switch strings.ToLower(strings.TrimSpace(getMetaString(meta, "dedupe_action"))) {
		case string(DedupeActionSkip):
			return ObserveResult{Skipped: true, Reason: "duplicate_forced_skip", DocumentID: match.doc.ID, Canonical: c.Content, CanonicalType: c.Type}, nil
		case string(DedupeActionNewVersion):
			return m.newVersionAndDeprecate(ctx, userID, match.doc, c, hash, meta, filters)
		case string(DedupeActionMerge):
			return m.mergeIntoExisting(ctx, userID, match.doc, c, hash, meta, filters)
		}

		if IsUserRejection(userMsg) {
			return m.newVersionAndDeprecate(ctx, userID, match.doc, c, hash, meta, filters)
		}

		return m.mergeIntoExisting(ctx, userID, match.doc, c, hash, meta, filters)
	}

	return m.saveNewCandidate(ctx, userID, c, hash, meta, filters)
}

func (m *Manager) promoteCandidate(ctx context.Context, userID string, meta map[string]interface{}) (ObserveResult, error) {
	query := strings.TrimSpace(getMetaString(meta, "previous_user_msg"))
	if query == "" {
		query = strings.TrimSpace(getMetaString(meta, "previous_assistant_msg"))
	}
	if query == "" {
		return ObserveResult{Skipped: true, Reason: "no_promotion_query"}, nil
	}

	filters := buildSearchFilters(userID, getMetaMap(meta, "knowledge_filters"))
	results, err := searchKnowledge(ctx, m.store, query, m.cfg.DedupeTopK, filters)
	if err != nil {
		return ObserveResult{}, err
	}

	var best *knowledge.SearchResult
	for _, r := range results {
		if r == nil || r.Document == nil {
			continue
		}
		if !isLearningDocForUser(r.Document, userID) {
			continue
		}
		if getMetaString(r.Document.Metadata, metaStatusKey) != string(StatusCandidate) {
			continue
		}
		best = r
		break
	}
	if best == nil {
		return ObserveResult{Skipped: true, Reason: "no_candidate_to_promote"}, nil
	}

	now := time.Now()
	if best.Document.Content == "" {
		return ObserveResult{Skipped: true, Reason: "empty_candidate"}, nil
	}

	if best.Document.Metadata == nil {
		best.Document.Metadata = make(map[string]interface{})
	}

	best.Document.Metadata[metaStatusKey] = string(StatusVerified)
	best.Document.Metadata[metaUpdatedAtKey] = now.Unix()
	best.Document.Metadata[metaConfidenceKey] = 0.80

	version := getMetaInt(best.Document.Metadata, metaVersionKey, 1)
	best.Document.Metadata[metaVersionKey] = version + 1

	if err := upsertDocument(ctx, m.store, *best.Document); err != nil {
		return ObserveResult{Skipped: true, Reason: "promote_requires_upsert", DocumentID: best.Document.ID}, nil
	}

	return ObserveResult{
		Saved:         true,
		Promoted:      true,
		Reason:        "promoted_verified",
		DocumentID:    best.Document.ID,
		Canonical:     best.Document.Content,
		CanonicalType: ItemType(getMetaString(best.Document.Metadata, metaTypeKey)),
		Status:        StatusVerified,
		CreatedAt:     now,
	}, nil
}

type duplicateMatch struct {
	doc      *document.Document
	exact    bool
	distance int
}

func (m *Manager) findDuplicate(ctx context.Context, userID, canonical string, hash uint64, filters map[string]interface{}) (duplicateMatch, bool) {
	results, err := searchKnowledge(ctx, m.store, canonical, m.cfg.DedupeTopK, filters)
	if err != nil {
		return duplicateMatch{}, false
	}

	normalized := normalizeText(canonical)

	for _, r := range results {
		if r == nil || r.Document == nil {
			continue
		}
		if !isLearningDocForUser(r.Document, userID) {
			continue
		}
		existingHash := parseHash(r.Document.Metadata)
		if existingHash == 0 {
			continue
		}
		dist := hammingDistance64(hash, existingHash)
		if dist > m.cfg.DedupeMaxHamming {
			continue
		}
		exact := normalizeText(r.Document.Content) == normalized
		return duplicateMatch{doc: r.Document, exact: exact, distance: dist}, true
	}

	return duplicateMatch{}, false
}

func parseHash(meta map[string]interface{}) uint64 {
	s := getMetaString(meta, metaSimHashKey)
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Accept hex (preferred) or decimal.
	if strings.HasPrefix(s, "0x") {
		s = strings.TrimPrefix(s, "0x")
	}
	if v, err := strconv.ParseUint(s, 16, 64); err == nil {
		return v
	}
	if v, err := strconv.ParseUint(s, 10, 64); err == nil {
		return v
	}
	return 0
}

func copyMap(in map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func buildSearchFilters(userID string, knowledgeFilters map[string]interface{}) map[string]interface{} {
	filters := make(map[string]interface{}, 4+len(knowledgeFilters))
	filters[metaNamespaceKey] = metaNamespaceValue
	if strings.TrimSpace(userID) != "" {
		filters[metaUserIDKey] = userID
	}
	for k, v := range knowledgeFilters {
		filters[k] = v
	}
	return filters
}

func searchKnowledge(ctx context.Context, kb knowledge.Knowledge, query string, topK int, filters map[string]interface{}) ([]*knowledge.SearchResult, error) {
	if kb == nil {
		return nil, nil
	}
	if s, ok := kb.(interface {
		SearchWithFilters(ctx context.Context, query string, numDocuments int, filters map[string]interface{}) ([]*knowledge.SearchResult, error)
	}); ok {
		return s.SearchWithFilters(ctx, query, topK, filters)
	}
	return kb.Search(ctx, query, topK)
}

func getMetaMap(meta map[string]interface{}, key string) map[string]interface{} {
	if meta == nil {
		return nil
	}
	v, ok := meta[key]
	if !ok || v == nil {
		return nil
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func getMetaBool(meta map[string]interface{}, key string) bool {
	if meta == nil {
		return false
	}
	v, ok := meta[key]
	if !ok || v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func (m *Manager) saveNewCandidate(ctx context.Context, userID string, c Canonical, hash uint64, meta map[string]interface{}, filters map[string]interface{}) (ObserveResult, error) {
	now := time.Now()
	docID := "learn_" + uuid.NewString()

	topic := strings.TrimSpace(getMetaString(meta, "topic"))
	if topic == "" {
		topic = strings.TrimSpace(c.Topic)
	}
	if topic == "" {
		topic = strings.TrimSpace(c.Title)
	}
	if topic == "" {
		topic = "General"
	}

	tags := tagsFromMeta(meta, c.Tags)

	doc := document.NewDocument(c.Content)
	doc.ID = docID
	doc.Name = topic
	doc.Source = metaSourceKey
	doc.Metadata = map[string]interface{}{
		metaNamespaceKey:       metaNamespaceValue,
		metaUserIDKey:          userID,
		metaTypeKey:            string(c.Type),
		metaTopicKey:           topic,
		metaTagsKey:            tags,
		metaVersionKey:         1,
		metaStatusKey:          string(StatusCandidate),
		metaConfidenceKey:      0.55,
		metaHitsKey:            0,
		metaUpdatedAtKey:       now.Unix(),
		metaCreatedAtKey:       now.Unix(),
		metaSimHashKey:         fmt.Sprintf("%016x", hash),
		metaSourceSessionIDKey: getMetaString(meta, "session_id"),
		metaSourceUserMsgIDKey: getMetaString(meta, "user_message_id"),
		metaSourceAsstMsgIDKey: getMetaString(meta, "assistant_message_id"),
	}

	for k, v := range filters {
		if _, exists := doc.Metadata[k]; exists {
			continue
		}
		doc.Metadata[k] = v
	}

	if err := m.store.LoadDocument(ctx, *doc); err != nil {
		return ObserveResult{}, err
	}

	return ObserveResult{
		Saved:         true,
		Reason:        "saved_candidate",
		DocumentID:    docID,
		Canonical:     c.Content,
		CanonicalType: c.Type,
		Status:        StatusCandidate,
		CreatedAt:     now,
	}, nil
}

func (m *Manager) mergeIntoExisting(ctx context.Context, userID string, existing *document.Document, c Canonical, incomingHash uint64, meta map[string]interface{}, filters map[string]interface{}) (ObserveResult, error) {
	if existing == nil {
		return ObserveResult{Skipped: true, Reason: "no_existing"}, nil
	}

	merged := mergeCanonicalText(existing.Content, c.Content, m.cfg.Canonicalize)
	if normalizeText(merged) == normalizeText(existing.Content) {
		return ObserveResult{Skipped: true, Reason: "duplicate_after_merge", DocumentID: existing.ID, Canonical: c.Content, CanonicalType: c.Type}, nil
	}

	now := time.Now()
	existing.Content = merged
	if existing.Metadata == nil {
		existing.Metadata = make(map[string]interface{})
	}

	version := getMetaInt(existing.Metadata, metaVersionKey, 1)
	existing.Metadata[metaVersionKey] = version + 1
	existing.Metadata[metaUpdatedAtKey] = now.Unix()
	existing.Metadata[metaSimHashKey] = fmt.Sprintf("%016x", simHash64(merged))

	hits := getMetaInt(existing.Metadata, metaHitsKey, 0)
	existing.Metadata[metaHitsKey] = hits + 1

	tags := tagsFromMeta(meta, nil)
	if len(tags) > 0 {
		existing.Metadata[metaTagsKey] = tags
	}
	if topic := strings.TrimSpace(getMetaString(meta, "topic")); topic != "" {
		existing.Metadata[metaTopicKey] = topic
		existing.Name = topic
	}

	if err := upsertDocument(ctx, m.store, *existing); err != nil {
		return ObserveResult{Skipped: true, Reason: "merge_requires_upsert", DocumentID: existing.ID}, nil
	}

	_ = incomingHash // deterministic input for dedupe; merged hash recomputed from merged content

	return ObserveResult{
		Saved:         true,
		Reason:        "merged",
		DocumentID:    existing.ID,
		Canonical:     merged,
		CanonicalType: ItemType(getMetaString(existing.Metadata, metaTypeKey)),
		Status:        Status(getMetaString(existing.Metadata, metaStatusKey)),
		CreatedAt:     now,
	}, nil
}

func (m *Manager) newVersionAndDeprecate(ctx context.Context, userID string, existing *document.Document, c Canonical, hash uint64, meta map[string]interface{}, filters map[string]interface{}) (ObserveResult, error) {
	if existing == nil {
		return ObserveResult{Skipped: true, Reason: "no_existing"}, nil
	}

	now := time.Now()
	newID := "learn_" + uuid.NewString()

	// Deprecate old
	if existing.Metadata == nil {
		existing.Metadata = make(map[string]interface{})
	}
	existing.Metadata[metaStatusKey] = string(StatusDeprecated)
	existing.Metadata[metaUpdatedAtKey] = now.Unix()
	existing.Metadata[metaDeprecatedByDocIDKey] = newID
	if err := upsertDocument(ctx, m.store, *existing); err != nil {
		return ObserveResult{Skipped: true, Reason: "deprecate_requires_upsert", DocumentID: existing.ID}, nil
	}

	// Save new version
	version := getMetaInt(existing.Metadata, metaVersionKey, 1)
	topic := strings.TrimSpace(getMetaString(meta, "topic"))
	if topic == "" {
		topic = strings.TrimSpace(c.Topic)
	}
	if topic == "" {
		topic = strings.TrimSpace(existing.Name)
	}
	if topic == "" {
		topic = "General"
	}

	tags := tagsFromMeta(meta, c.Tags)

	doc := document.NewDocument(c.Content)
	doc.ID = newID
	doc.Name = topic
	doc.Source = metaSourceKey
	doc.Metadata = map[string]interface{}{
		metaNamespaceKey:       metaNamespaceValue,
		metaUserIDKey:          userID,
		metaTypeKey:            string(c.Type),
		metaTopicKey:           topic,
		metaTagsKey:            tags,
		metaVersionKey:         version + 1,
		metaStatusKey:          string(StatusCandidate),
		metaConfidenceKey:      0.55,
		metaHitsKey:            0,
		metaUpdatedAtKey:       now.Unix(),
		metaCreatedAtKey:       now.Unix(),
		metaSimHashKey:         fmt.Sprintf("%016x", hash),
		metaSourceSessionIDKey: getMetaString(meta, "session_id"),
		metaSourceUserMsgIDKey: getMetaString(meta, "user_message_id"),
		metaSourceAsstMsgIDKey: getMetaString(meta, "assistant_message_id"),
	}
	for k, v := range filters {
		if _, exists := doc.Metadata[k]; exists {
			continue
		}
		doc.Metadata[k] = v
	}

	if err := m.store.LoadDocument(ctx, *doc); err != nil {
		return ObserveResult{}, err
	}

	return ObserveResult{
		Saved:         true,
		Reason:        "new_version_deprecated_old",
		DocumentID:    newID,
		Canonical:     c.Content,
		CanonicalType: c.Type,
		Status:        StatusCandidate,
		CreatedAt:     now,
	}, nil
}

func (m *Manager) deprecatePrevious(ctx context.Context, userID string, meta map[string]interface{}) (ObserveResult, error) {
	query := strings.TrimSpace(getMetaString(meta, "previous_user_msg"))
	if query == "" {
		query = strings.TrimSpace(getMetaString(meta, "previous_assistant_msg"))
	}
	if query == "" {
		return ObserveResult{Skipped: true, Reason: "no_deprecation_query"}, nil
	}

	filters := buildSearchFilters(userID, getMetaMap(meta, "knowledge_filters"))
	results, err := searchKnowledge(ctx, m.store, query, m.cfg.DedupeTopK, filters)
	if err != nil {
		return ObserveResult{}, err
	}

	var best *knowledge.SearchResult
	for _, r := range results {
		if r == nil || r.Document == nil {
			continue
		}
		if !isLearningDocForUser(r.Document, userID) {
			continue
		}
		status := getMetaString(r.Document.Metadata, metaStatusKey)
		if status == string(StatusDeprecated) {
			continue
		}
		best = r
		break
	}
	if best == nil {
		return ObserveResult{Skipped: true, Reason: "no_memory_to_deprecate"}, nil
	}

	now := time.Now()
	best.Document.Metadata[metaStatusKey] = string(StatusDeprecated)
	best.Document.Metadata[metaUpdatedAtKey] = now.Unix()

	if err := upsertDocument(ctx, m.store, *best.Document); err != nil {
		return ObserveResult{Skipped: true, Reason: "deprecate_requires_upsert", DocumentID: best.Document.ID}, nil
	}

	return ObserveResult{Saved: true, Reason: "deprecated", DocumentID: best.Document.ID, Status: StatusDeprecated, CreatedAt: now}, nil
}

func (m *Manager) incrementHits(ctx context.Context, docs []*document.Document) error {
	if m == nil || m.store == nil {
		return nil
	}
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if doc.Metadata == nil {
			continue
		}
		hits := getMetaInt(doc.Metadata, metaHitsKey, 0)
		doc.Metadata[metaHitsKey] = hits + 1
		doc.Metadata[metaUpdatedAtKey] = time.Now().Unix()
		if err := upsertDocument(ctx, m.store, *doc); err != nil {
			// Best effort: do not fail retrieval due to counter updates.
			return nil
		}
	}
	return nil
}

func upsertDocument(ctx context.Context, kb knowledge.Knowledge, doc document.Document) error {
	if kb == nil {
		return fmt.Errorf("no_store")
	}
	if u, ok := kb.(interface {
		UpsertDocument(ctx context.Context, doc document.Document) error
	}); ok {
		return u.UpsertDocument(ctx, doc)
	}
	if u, ok := kb.(interface {
		Upsert(ctx context.Context, documents []document.Document) error
	}); ok {
		return u.Upsert(ctx, []document.Document{doc})
	}
	return fmt.Errorf("upsert_not_supported")
}

func (m *Manager) setLastRetrieved(userID string, docs []*document.Document) {
	if m == nil {
		return
	}
	m.retrievedMu.Lock()
	defer m.retrievedMu.Unlock()
	if len(docs) == 0 {
		delete(m.lastRetrieved, userID)
		return
	}
	copies := make([]*document.Document, 0, len(docs))
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		copies = append(copies, doc)
	}
	m.lastRetrieved[userID] = copies
}

func (m *Manager) consumeLastRetrieved(userID string) []*document.Document {
	if m == nil {
		return nil
	}
	m.retrievedMu.Lock()
	defer m.retrievedMu.Unlock()
	docs := m.lastRetrieved[userID]
	delete(m.lastRetrieved, userID)
	return docs
}

func (m *Manager) applyEvidence(ctx context.Context, userID string, meta map[string]interface{}) error {
	retrievedDocs := m.consumeLastRetrieved(userID)
	if len(retrievedDocs) == 0 {
		return nil
	}

	ids := getMetaStringSlice(meta, "learning_retrieved_ids")
	if len(ids) > 0 {
		retrievedDocs = filterDocsByIDs(retrievedDocs, ids)
	}
	if len(retrievedDocs) == 0 {
		return nil
	}

	now := time.Now().Unix()
	for _, doc := range retrievedDocs {
		if doc == nil || doc.Metadata == nil {
			continue
		}
		status := getMetaString(doc.Metadata, metaStatusKey)
		if status == string(StatusDeprecated) {
			continue
		}

		streak := getMetaInt(doc.Metadata, metaStreakKey, 0) + 1
		doc.Metadata[metaStreakKey] = streak
		doc.Metadata[metaUpdatedAtKey] = now

		if status == string(StatusCandidate) {
			hits := getMetaInt(doc.Metadata, metaHitsKey, 0)
			shouldPromote := false
			confidence := getMetaFloat(doc.Metadata, metaConfidenceKey, 0.55)

			if m.cfg.AutoPromoteStreak > 0 && streak >= m.cfg.AutoPromoteStreak {
				shouldPromote = true
				confidence = m.cfg.AutoPromoteConfidenceStreak
			} else if m.cfg.AutoPromoteHits > 0 && hits >= m.cfg.AutoPromoteHits {
				shouldPromote = true
				confidence = m.cfg.AutoPromoteConfidenceHits
			}

			if shouldPromote {
				doc.Metadata[metaStatusKey] = string(StatusVerified)
				doc.Metadata[metaConfidenceKey] = confidence
				version := getMetaInt(doc.Metadata, metaVersionKey, 1)
				doc.Metadata[metaVersionKey] = version + 1
			}
		}

		if err := upsertDocument(ctx, m.store, *doc); err != nil {
			return nil
		}
	}

	return nil
}

func filterDocsByIDs(docs []*document.Document, ids []string) []*document.Document {
	if len(ids) == 0 {
		return docs
	}
	allowed := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}
	var out []*document.Document
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		if _, ok := allowed[doc.ID]; ok {
			out = append(out, doc)
		}
	}
	return out
}

func mergeCanonicalText(existing, incoming string, cfg CanonicalizeConfig) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if existing == "" {
		return incoming
	}
	if incoming == "" {
		return existing
	}

	existingCode := firstCodeBlock(existing, cfg.MaxCodeBlockLines)
	incomingCode := firstCodeBlock(incoming, cfg.MaxCodeBlockLines)

	existingLines := splitNonEmptyLines(stripCodeBlocks(existing))
	incomingLines := splitNonEmptyLines(stripCodeBlocks(incoming))

	seen := make(map[string]struct{}, len(existingLines)+len(incomingLines))
	var merged []string
	for _, l := range existingLines {
		n := normalizeText(l)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		merged = append(merged, l)
	}
	for _, l := range incomingLines {
		n := normalizeText(l)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		merged = append(merged, l)
	}

	if incomingCode != "" {
		merged = append(merged, "- Snippet:", incomingCode)
	} else if existingCode != "" {
		merged = append(merged, "- Snippet:", existingCode)
	}

	if cfg.MaxLines > 0 && len(merged) > cfg.MaxLines {
		merged = merged[:cfg.MaxLines]
	}

	out := strings.Join(merged, "\n")
	return clampLen(out, cfg.MaxTotalChars)
}

func splitNonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

func normalizeText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "\r\n", "\n")
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func getMetaInt(meta map[string]interface{}, key string, def int) int {
	if meta == nil {
		return def
	}
	v, ok := meta[key]
	if !ok || v == nil {
		return def
	}
	switch x := v.(type) {
	case int:
		return x
	case int32:
		return int(x)
	case int64:
		return int(x)
	case float32:
		return int(x)
	case float64:
		return int(x)
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(x)); err == nil {
			return i
		}
		return def
	default:
		return def
	}
}

func getMetaFloat(meta map[string]interface{}, key string, def float64) float64 {
	if meta == nil {
		return def
	}
	v, ok := meta[key]
	if !ok || v == nil {
		return def
	}
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(x), 64); err == nil {
			return f
		}
		return def
	default:
		return def
	}
}

func getMetaStringSlice(meta map[string]interface{}, key string) []string {
	if meta == nil {
		return nil
	}
	v, ok := meta[key]
	if !ok || v == nil {
		return nil
	}
	switch x := v.(type) {
	case []string:
		return x
	case []interface{}:
		out := make([]string, 0, len(x))
		for _, it := range x {
			if it == nil {
				continue
			}
			s := strings.TrimSpace(fmt.Sprint(it))
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		return out
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return nil
		}
		return []string{s}
	default:
		return nil
	}
}

func tagsFromMeta(meta map[string]interface{}, fallback []string) []string {
	if meta == nil {
		return fallback
	}
	v, ok := meta["tags"]
	if !ok || v == nil {
		return fallback
	}
	switch x := v.(type) {
	case []string:
		return x
	case []interface{}:
		var out []string
		for _, it := range x {
			if it == nil {
				continue
			}
			s := strings.TrimSpace(fmt.Sprint(it))
			if s == "" {
				continue
			}
			out = append(out, s)
		}
		if len(out) > 0 {
			return out
		}
		return fallback
	default:
		s := strings.TrimSpace(fmt.Sprint(x))
		if s == "" {
			return fallback
		}
		return []string{s}
	}
}
