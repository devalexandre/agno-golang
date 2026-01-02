package learning

import (
	"regexp"
	"strings"
)

type Canonical struct {
	Title   string
	Content string
	Type    ItemType
	Tags    []string
	Topic   string
}

type CanonicalizeConfig struct {
	MaxBullets       int
	MaxBulletChars   int
	MaxCodeBlockLines int
	MaxTotalChars    int
	MaxLines         int
}

func DefaultCanonicalizeConfig() CanonicalizeConfig {
	return CanonicalizeConfig{
		MaxBullets:        8,
		MaxBulletChars:    220,
		MaxCodeBlockLines: 10,
		MaxTotalChars:     900,
		MaxLines:          12,
	}
}

func Canonicalize(userMsg, assistantMsg string, cfg CanonicalizeConfig) Canonical {
	topic := topicFromUserMsg(userMsg)

	lowerAssistant := strings.ToLower(assistantMsg)
	if strings.Contains(lowerAssistant, "decision:") || strings.Contains(lowerAssistant, "we decided") || strings.Contains(lowerAssistant, "we chose") {
		sentBullets := bulletsFromSentences(assistantMsg, cfg.MaxBullets, cfg.MaxBulletChars)
		if len(sentBullets) > 0 {
			content := joinLines(sentBullets, cfg.MaxLines)
			return Canonical{
				Title:   topic,
				Topic:   topic,
				Content: clampLen(content, cfg.MaxTotalChars),
				Type:    ItemTypeDecision,
			}
		}
	}

	codeBlock := firstCodeBlock(assistantMsg, cfg.MaxCodeBlockLines)
	if codeBlock != "" {
		lines := []string{"- Snippet:", codeBlock}
		content := joinLines(lines, cfg.MaxLines)
		return Canonical{
			Title:   topic,
			Topic:   topic,
			Content: clampLen(content, cfg.MaxTotalChars),
			Type:    ItemTypeSnippet,
		}
	}

	bullets := extractBullets(assistantMsg, cfg.MaxBullets, cfg.MaxBulletChars)
	if len(bullets) > 0 {
		content := joinLines(bullets, cfg.MaxLines)
		return Canonical{
			Title:   topic,
			Topic:   topic,
			Content: clampLen(content, cfg.MaxTotalChars),
			Type:    ItemTypeProcedure,
		}
	}

	sentBullets := bulletsFromSentences(assistantMsg, cfg.MaxBullets, cfg.MaxBulletChars)
	if len(sentBullets) > 0 {
		content := joinLines(sentBullets, cfg.MaxLines)
		typ := ItemTypePattern
		if strings.Contains(strings.TrimSpace(userMsg), "?") && len(sentBullets) <= 4 {
			typ = ItemTypeFAQ
		}
		return Canonical{
			Title:   topic,
			Topic:   topic,
			Content: clampLen(content, cfg.MaxTotalChars),
			Type:    typ,
		}
	}

	return Canonical{Title: topic, Topic: topic, Content: "", Type: ItemTypePattern}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(strings.TrimPrefix(s, "User:"))
}

func topicFromUserMsg(userMsg string) string {
	line := firstLine(userMsg)
	if line == "" {
		return "General"
	}
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, "?")
	line = strings.TrimSuffix(line, ".")
	if len(line) > 60 {
		line = strings.TrimSpace(line[:60]) + "…"
	}
	return line
}

func clampLen(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max]) + "…"
}

var codeBlockRe = regexp.MustCompile("(?s)```[a-zA-Z0-9_+-]*\\n(.*?)\\n```")

func firstCodeBlock(s string, maxLines int) string {
	m := codeBlockRe.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	body := strings.TrimSpace(m[1])
	if body == "" {
		return ""
	}
	lines := strings.Split(body, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
		body = strings.Join(lines, "\n") + "\n…"
	}
	return "```\n" + body + "\n```"
}

func extractBullets(text string, maxBullets, maxBulletChars int) []string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var out []string

	numbered := regexp.MustCompile(`^\s*\d+[.)]\s+`)

	for _, line := range lines {
		t := strings.TrimSpace(strings.TrimPrefix(line, ">"))
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
			out = append(out, "- "+clampLen(strings.TrimSpace(t[2:]), maxBulletChars))
		} else if numbered.MatchString(t) {
			t = numbered.ReplaceAllString(t, "")
			out = append(out, "- "+clampLen(strings.TrimSpace(t), maxBulletChars))
		}
		if maxBullets > 0 && len(out) >= maxBullets {
			break
		}
	}
	return out
}

func bulletsFromSentences(text string, maxBullets, maxBulletChars int) []string {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return nil
	}
	clean = stripCodeBlocks(clean)
	clean = strings.ReplaceAll(clean, "\r\n", "\n")

	parts := regexp.MustCompile(`[.?!]\s+`).Split(clean, -1)
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if len(p) < 20 {
			continue
		}
		out = append(out, "- "+clampLen(p, maxBulletChars))
		if maxBullets > 0 && len(out) >= maxBullets {
			break
		}
	}
	return out
}

func stripCodeBlocks(s string) string {
	return codeBlockRe.ReplaceAllString(s, "")
}

func joinLines(lines []string, maxLines int) string {
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return strings.Join(lines, "\n")
}
