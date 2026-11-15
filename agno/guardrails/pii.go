package guardrails

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// PIIType identifies a class of personally identifiable information.
type PIIType string

const (
	PIITypeEmail      PIIType = "email"
	PIITypePhone      PIIType = "phone"
	PIITypeSSN        PIIType = "ssn"
	PIITypeCreditCard PIIType = "credit_card"
	PIITypeCPF        PIIType = "cpf"
	PIITypeCNPJ       PIIType = "cnpj"
	PIITypeIBAN       PIIType = "iban"
	PIITypeAPIKey     PIIType = "api_key"
)

// PIIMode defines what the guardrail does when PII is found.
type PIIMode string

const (
	// PIIModeBlock rejects the content with a CheckError.
	PIIModeBlock PIIMode = "block"
	// PIIModeRedact replaces each match with a type label, e.g. [REDACTED_EMAIL].
	PIIModeRedact PIIMode = "redact"
	// PIIModeMask replaces each character of the match with an asterisk.
	PIIModeMask PIIMode = "mask"
)

// piiPattern is a single detection rule.
type piiPattern struct {
	Type     PIIType
	Regexp   *regexp.Regexp
	Validate func(match string) bool
}

var defaultPIIPatterns = []piiPattern{
	{
		Type:   PIITypeEmail,
		Regexp: regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`),
	},
	{
		Type:   PIITypeSSN,
		Regexp: regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	},
	{
		Type:     PIITypeCreditCard,
		Regexp:   regexp.MustCompile(`\b(?:\d{4}[\s\-]?){3}\d{4}\b`),
		Validate: isValidLuhn,
	},
	{
		// American Express uses a 15 digit, 4-6-5 grouped number.
		Type:     PIITypeCreditCard,
		Regexp:   regexp.MustCompile(`\b3[47]\d{2}[\s\-]?\d{6}[\s\-]?\d{5}\b`),
		Validate: isValidLuhn,
	},
	{
		Type:     PIITypeCPF,
		Regexp:   regexp.MustCompile(`\b\d{3}\.\d{3}\.\d{3}-\d{2}\b|\b\d{11}\b`),
		Validate: isValidCPF,
	},
	{
		Type:     PIITypeCNPJ,
		Regexp:   regexp.MustCompile(`\b\d{2}\.\d{3}\.\d{3}/\d{4}-\d{2}\b|\b\d{14}\b`),
		Validate: isValidCNPJ,
	},
	{
		// Brazilian phone numbers, with optional country code and area code.
		Type:   PIITypePhone,
		Regexp: regexp.MustCompile(`(?:\+55[\s\-]?)?\(\d{2}\)[\s\-]?9?\d{4}[\s\-.]?\d{4}\b`),
	},
	{
		// North American phone numbers.
		Type:   PIITypePhone,
		Regexp: regexp.MustCompile(`\b(?:\+1[\s\-.]?)?\(?\d{3}\)?[\s\-.]?\d{3}[\s\-.]?\d{4}\b`),
	},
	{
		Type:     PIITypeIBAN,
		Regexp:   regexp.MustCompile(`\b[A-Z]{2}\d{2}[A-Z0-9]{10,30}\b`),
		Validate: isValidIBAN,
	},
	{
		// Provider style API keys and access tokens.
		Type:   PIITypeAPIKey,
		Regexp: regexp.MustCompile(`\b(?:sk|pk|rk)-[A-Za-z0-9_\-]{16,}\b|\bAKIA[0-9A-Z]{16}\b|\bgh[pousr]_[A-Za-z0-9]{20,}\b|\bxox[baprs]-[A-Za-z0-9\-]{10,}\b`),
	},
	{
		// Credentials assigned inline, e.g. api_key = "0123456789abcdef".
		Type:   PIITypeAPIKey,
		Regexp: regexp.MustCompile(`(?i)\b(?:api[_\-]?key|secret[_\-]?key|access[_\-]?token|auth[_\-]?token)\b\s*[:=]\s*["']?[A-Za-z0-9_\-]{16,}["']?`),
	},
}

// PIIConfig configures the PII detection guardrail.
type PIIConfig struct {
	// Mode defines what happens when PII is detected. Defaults to PIIModeBlock.
	Mode PIIMode
	// Types restricts detection to the given types. Empty means all types.
	Types []PIIType
	// DisabledTypes removes types from detection.
	DisabledTypes []PIIType
	// CustomPatterns adds extra detection rules, mapping a type name to a regex.
	CustomPatterns map[string]string
}

// PIIDetectionGuardrail detects personally identifiable information in content.
type PIIDetectionGuardrail struct {
	mode     PIIMode
	patterns []piiPattern
}

// PIIOption configures a PIIDetectionGuardrail.
type PIIOption func(*PIIConfig)

// WithPIIMode sets the guardrail mode (block, redact or mask).
func WithPIIMode(mode PIIMode) PIIOption {
	return func(c *PIIConfig) { c.Mode = mode }
}

// WithPIITypes restricts detection to the given PII types.
func WithPIITypes(types ...PIIType) PIIOption {
	return func(c *PIIConfig) { c.Types = types }
}

// WithoutPIITypes removes the given PII types from detection.
func WithoutPIITypes(types ...PIIType) PIIOption {
	return func(c *PIIConfig) { c.DisabledTypes = append(c.DisabledTypes, types...) }
}

// WithPIICustomPattern adds a custom detection rule.
func WithPIICustomPattern(name, pattern string) PIIOption {
	return func(c *PIIConfig) {
		if c.CustomPatterns == nil {
			c.CustomPatterns = map[string]string{}
		}
		c.CustomPatterns[name] = pattern
	}
}

// NewPIIDetectionGuardrail creates a PII detection guardrail. Custom patterns
// are compiled eagerly, so an invalid regex panics at construction time rather
// than failing on the first check.
func NewPIIDetectionGuardrail(opts ...PIIOption) *PIIDetectionGuardrail {
	cfg := PIIConfig{Mode: PIIModeBlock}
	for _, opt := range opts {
		opt(&cfg)
	}
	return NewPIIDetectionGuardrailWithConfig(cfg)
}

// NewPIIDetectionGuardrailWithConfig creates a PII detection guardrail from a config.
func NewPIIDetectionGuardrailWithConfig(cfg PIIConfig) *PIIDetectionGuardrail {
	if cfg.Mode == "" {
		cfg.Mode = PIIModeBlock
	}

	enabled := map[PIIType]bool{}
	if len(cfg.Types) > 0 {
		for _, t := range cfg.Types {
			enabled[t] = true
		}
	}
	disabled := map[PIIType]bool{}
	for _, t := range cfg.DisabledTypes {
		disabled[t] = true
	}

	patterns := make([]piiPattern, 0, len(defaultPIIPatterns))
	for _, p := range defaultPIIPatterns {
		if len(enabled) > 0 && !enabled[p.Type] {
			continue
		}
		if disabled[p.Type] {
			continue
		}
		patterns = append(patterns, p)
	}

	names := make([]string, 0, len(cfg.CustomPatterns))
	for name := range cfg.CustomPatterns {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		patterns = append(patterns, piiPattern{
			Type:   PIIType(name),
			Regexp: regexp.MustCompile(cfg.CustomPatterns[name]),
		})
	}

	return &PIIDetectionGuardrail{mode: cfg.Mode, patterns: patterns}
}

// GetName returns the guardrail name.
func (p *PIIDetectionGuardrail) GetName() string { return "PIIDetectionGuardrail" }

// GetDescription returns a human-readable description.
func (p *PIIDetectionGuardrail) GetDescription() string {
	return fmt.Sprintf("Detects personally identifiable information (mode: %s)", p.mode)
}

// Detect returns the PII types found in the text, sorted for stable output.
func (p *PIIDetectionGuardrail) Detect(text string) []PIIType {
	found := map[PIIType]bool{}
	for _, pattern := range p.patterns {
		for _, match := range pattern.Regexp.FindAllString(text, -1) {
			if pattern.Validate != nil && !pattern.Validate(match) {
				continue
			}
			found[pattern.Type] = true
			break
		}
	}

	types := make([]PIIType, 0, len(found))
	for t := range found {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	return types
}

// Sanitize applies the configured mode to the text and returns the sanitized
// text together with the PII types that were found. In block mode the text is
// returned unchanged.
func (p *PIIDetectionGuardrail) Sanitize(text string) (string, []PIIType) {
	detected := p.Detect(text)
	if len(detected) == 0 || p.mode == PIIModeBlock {
		return text, detected
	}

	sanitized := text
	for _, pattern := range p.patterns {
		mode := p.mode
		piiType := pattern.Type
		validate := pattern.Validate
		sanitized = pattern.Regexp.ReplaceAllStringFunc(sanitized, func(match string) string {
			if validate != nil && !validate(match) {
				return match
			}
			if mode == PIIModeMask {
				return strings.Repeat("*", len([]rune(match)))
			}
			return fmt.Sprintf("[REDACTED_%s]", strings.ToUpper(string(piiType)))
		})
	}
	return sanitized, detected
}

// Check validates arbitrary data. In redact/mask mode a plain string cannot be
// rewritten in place, so the check passes; use CheckInput/CheckOutput or
// Sanitize to obtain the sanitized content.
func (p *PIIDetectionGuardrail) Check(ctx context.Context, data interface{}) error {
	if input, ok := data.(*Input); ok {
		return p.CheckInput(ctx, input)
	}
	if output, ok := data.(*Output); ok {
		return p.CheckOutput(ctx, output)
	}

	text, ok := ExtractText(data)
	if !ok {
		return nil
	}
	if detected := p.Detect(text); len(detected) > 0 && p.mode == PIIModeBlock {
		return p.blockError(StageInput, detected)
	}
	return nil
}

// CheckInput validates the input, rewriting its content in redact/mask mode.
func (p *PIIDetectionGuardrail) CheckInput(ctx context.Context, input *Input) error {
	if input == nil {
		return nil
	}
	sanitized, detected := p.Sanitize(input.Content)
	if len(detected) == 0 {
		return nil
	}
	if p.mode == PIIModeBlock {
		return p.blockError(StageInput, detected)
	}
	input.SetText(sanitized)
	return nil
}

// CheckOutput validates the output, rewriting its content in redact/mask mode.
func (p *PIIDetectionGuardrail) CheckOutput(ctx context.Context, output *Output) error {
	if output == nil {
		return nil
	}
	sanitized, detected := p.Sanitize(output.Content)
	if len(detected) == 0 {
		return nil
	}
	if p.mode == PIIModeBlock {
		return p.blockError(StageOutput, detected)
	}
	output.SetText(sanitized)
	return nil
}

func (p *PIIDetectionGuardrail) blockError(stage Stage, detected []PIIType) *CheckError {
	names := make([]string, 0, len(detected))
	for _, t := range detected {
		names = append(names, string(t))
	}

	message := "Potential PII detected in input"
	if stage == StageOutput {
		message = "Potential PII detected in output"
	}
	return &CheckError{
		Stage:          stage,
		Trigger:        TriggerPIIDetected,
		Message:        message,
		Guardrail:      p.GetName(),
		AdditionalData: map[string]interface{}{"detected_pii": names},
	}
}

// ===== VALIDATORS =====

// digitsOnly keeps only the decimal digits of s.
func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isValidLuhn validates a number using the Luhn checksum.
func isValidLuhn(value string) bool {
	digits := digitsOnly(value)
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// isValidCPF validates a Brazilian CPF using its two check digits.
func isValidCPF(value string) bool {
	digits := digitsOnly(value)
	if len(digits) != 11 || allSameDigits(digits) {
		return false
	}

	for _, check := range []int{9, 10} {
		sum := 0
		weight := check + 1
		for i := 0; i < check; i++ {
			sum += int(digits[i]-'0') * weight
			weight--
		}
		remainder := (sum * 10) % 11
		if remainder == 10 {
			remainder = 0
		}
		if remainder != int(digits[check]-'0') {
			return false
		}
	}
	return true
}

// isValidCNPJ validates a Brazilian CNPJ using its two check digits.
func isValidCNPJ(value string) bool {
	digits := digitsOnly(value)
	if len(digits) != 14 || allSameDigits(digits) {
		return false
	}

	weightsFirst := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	weightsSecond := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}

	for idx, weights := range [][]int{weightsFirst, weightsSecond} {
		sum := 0
		for i, weight := range weights {
			sum += int(digits[i]-'0') * weight
		}
		remainder := sum % 11
		expected := 0
		if remainder >= 2 {
			expected = 11 - remainder
		}
		if expected != int(digits[12+idx]-'0') {
			return false
		}
	}
	return true
}

// isValidIBAN validates an IBAN using the ISO 7064 mod-97 checksum.
func isValidIBAN(value string) bool {
	iban := strings.ToUpper(strings.ReplaceAll(value, " ", ""))
	if len(iban) < 15 || len(iban) > 34 {
		return false
	}

	rearranged := iban[4:] + iban[:4]
	remainder := 0
	for _, r := range rearranged {
		var chunk string
		switch {
		case r >= '0' && r <= '9':
			chunk = string(r)
		case r >= 'A' && r <= 'Z':
			chunk = strconv.Itoa(int(r-'A') + 10)
		default:
			return false
		}
		for _, d := range chunk {
			remainder = (remainder*10 + int(d-'0')) % 97
		}
	}
	return remainder == 1
}

func allSameDigits(digits string) bool {
	for i := 1; i < len(digits); i++ {
		if digits[i] != digits[0] {
			return false
		}
	}
	return true
}
