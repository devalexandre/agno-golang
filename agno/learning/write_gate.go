package learning

import (
	"regexp"
	"strings"
)

type WriteGateConfig struct {
	MaxCanonicalChars int
}

type WriteGateDecision struct {
	Allow  bool
	Reason string
}

func DefaultWriteGateConfig() WriteGateConfig {
	return WriteGateConfig{
		MaxCanonicalChars: 900,
	}
}

func ShouldWrite(userMsg, assistantMsg, canonical string, cfg WriteGateConfig) WriteGateDecision {
	if strings.TrimSpace(canonical) == "" {
		return WriteGateDecision{Allow: false, Reason: "empty_canonical"}
	}
	if cfg.MaxCanonicalChars > 0 && len(canonical) > cfg.MaxCanonicalChars {
		return WriteGateDecision{Allow: false, Reason: "canonical_too_long"}
	}

	if looksSensitive(userMsg) || looksSensitive(assistantMsg) || looksSensitive(canonical) {
		return WriteGateDecision{Allow: false, Reason: "sensitive_content"}
	}
	if looksTooSpecific(userMsg) || looksTooSpecific(assistantMsg) || looksTooSpecific(canonical) {
		return WriteGateDecision{Allow: false, Reason: "too_specific"}
	}
	if looksUnstable(canonical) {
		return WriteGateDecision{Allow: false, Reason: "unstable_information"}
	}
	if !looksReusable(assistantMsg) && !looksReusable(canonical) {
		return WriteGateDecision{Allow: false, Reason: "not_reusable"}
	}

	return WriteGateDecision{Allow: true, Reason: "ok"}
}

func looksReusable(text string) bool {
	t := strings.ToLower(text)
	if strings.Contains(t, "```") {
		return true
	}
	if strings.Contains(t, "\n- ") || strings.Contains(t, "\n* ") {
		return true
	}
	if regexp.MustCompile(`(?m)^\s*\d+[.)]\s+`).FindStringIndex(text) != nil {
		return true
	}
	reusableHints := []string{
		"step", "steps", "how to", "do this", "do it", "do the following", "procedure",
		"passo", "passos", "etapa", "como ", "como fazer", "faça", "faça assim",
		"rule", "tip", "recommend", "avoid", "always", "never", "use ", "don't ", "do not ",
		"regra", "dica", "recomend", "evite", "sempre", "nunca", "use ", "não ",
	}
	for _, h := range reusableHints {
		if strings.Contains(t, h) {
			return true
		}
	}
	return false
}

func looksUnstable(text string) bool {
	t := strings.ToLower(text)
	unstable := []string{
		"price", "cost", "promo", "promotion", "quote", "news", "latest",
		"today", "now", "yesterday", "tomorrow", "currently", "right now",
		"preço", "cust", "promo", "cotação", "notícia", "noticias", "últimas", "ultimas",
		"hoje", "agora", "ontem", "amanhã", "amanha", "atualmente", "neste momento",
	}
	for _, u := range unstable {
		if strings.Contains(t, u) {
			return true
		}
	}
	return false
}

var sensitiveRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(api[_-]?key|secret|token|access[_-]?token|refresh[_-]?token)\b`),
	regexp.MustCompile(`(?i)\b(bearer)\s+[a-z0-9._\\-]{10,}`),
	regexp.MustCompile(`(?i)\b(cookie|set-cookie)\b`),
	regexp.MustCompile(`(?i)\b(sk-[a-z0-9]{10,})\b`),        // OpenAI-like
	regexp.MustCompile(`(?i)\b(ghp_[a-z0-9]{10,})\b`),       // GitHub PAT
	regexp.MustCompile(`(?i)\b(xox[baprs]-[a-z0-9-]{10,})\b`), // Slack tokens
	regexp.MustCompile(`(?i)\bAKIA[0-9A-Z]{16}\b`),          // AWS access key id
	regexp.MustCompile(`(?i)\b(set-cookie|authorization|proxy-authorization|x-api-key)\s*:\s*.+`),
	regexp.MustCompile(`(?i)[a-z0-9._%+\\-]+@[a-z0-9.\\-]+\\.[a-z]{2,}`),
	regexp.MustCompile(`(?i)\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`),     // CPF
	regexp.MustCompile(`(?i)\b\d{2}\.?\d{3}\.?\d{3}/?\d{4}-?\d{2}\b`), // CNPJ
}

func looksSensitive(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, re := range sensitiveRegexes {
		if re.FindStringIndex(text) != nil {
			return true
		}
	}
	return false
}

var tooSpecificRegexes = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b\d{8,}\b`), // long numeric IDs
	regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`), // UUID
	regexp.MustCompile(`(?i)(^|\\s)(/[a-z0-9._\\-]+){3,}`), // deep unix paths
	regexp.MustCompile(`(?i)([a-zA-Z]:\\\\[^\\s]+)`),       // windows paths
	regexp.MustCompile(`(?i)\\b(customer_id|client_id|tenant_id|account_id)\\b\\s*[:=]\\s*\\S+`),
}

func looksTooSpecific(text string) bool {
	if strings.TrimSpace(text) == "" {
		return false
	}
	for _, re := range tooSpecificRegexes {
		if re.FindStringIndex(text) != nil {
			return true
		}
	}
	return false
}

func IsUserConfirmation(userMsg string) bool {
	t := strings.ToLower(strings.TrimSpace(userMsg))
	if t == "" {
		return false
	}
	confirmations := []string{
		"that worked", "it worked", "works now", "fixed", "solved", "resolved", "perfect",
		"thanks, it worked", "thank you, it worked", "awesome", "great, thanks",
		"funcionou", "resolveu", "resolvido", "deu certo", "ok agora", "era isso",
		"perfeito", "valeu, resolveu", "obrigado, resolveu", "fechou", "show",
	}
	for _, c := range confirmations {
		if strings.Contains(t, c) {
			return true
		}
	}
	return false
}

func IsUserRejection(userMsg string) bool {
	t := strings.ToLower(strings.TrimSpace(userMsg))
	if t == "" {
		return false
	}
	rejections := []string{
		"didn't work", "did not work", "doesn't work", "does not work",
		"wrong", "incorrect", "not correct", "broken", "still broken",
		"changed", "no longer works",
		"não funcionou", "nao funcionou", "não deu", "nao deu", "errado", "incorreto",
		"mudou", "não é mais", "nao e mais",
	}
	for _, r := range rejections {
		if strings.Contains(t, r) {
			return true
		}
	}
	return false
}
