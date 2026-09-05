package guardrails

import (
	"context"
	"strings"
	"testing"
)

func containsType(types []PIIType, target PIIType) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

func TestPIIDetect(t *testing.T) {
	g := NewPIIDetectionGuardrail()

	tests := []struct {
		name     string
		text     string
		expected PIIType
	}{
		{"email", "reach me at jane.doe+work@example.co.uk please", PIITypeEmail},
		{"ssn", "my ssn is 123-45-6789", PIITypeSSN},
		{"credit card", "card 4111 1111 1111 1111 expires soon", PIITypeCreditCard},
		{"credit card dashes", "card 5500-0000-0000-0004", PIITypeCreditCard},
		{"amex", "amex 3782 822463 10005", PIITypeCreditCard},
		{"cpf formatted", "meu cpf e 529.982.247-25", PIITypeCPF},
		{"cpf raw", "cpf 52998224725", PIITypeCPF},
		{"cnpj formatted", "cnpj 11.222.333/0001-81", PIITypeCNPJ},
		{"cnpj raw", "cnpj 11222333000181", PIITypeCNPJ},
		{"phone br", "ligue para (11) 98765-4321", PIITypePhone},
		{"phone us", "call 415-555-0132 today", PIITypePhone},
		{"iban", "iban GB82WEST12345698765432 is mine", PIITypeIBAN},
		{"api key openai", "use sk-abc123DEF456ghi789JKL0 as key", PIITypeAPIKey},
		{"api key aws", "AKIAIOSFODNN7EXAMPLE", PIITypeAPIKey},
		{"api key inline", `api_key = "0123456789abcdef0123"`, PIITypeAPIKey},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detected := g.Detect(tt.text)
			if !containsType(detected, tt.expected) {
				t.Fatalf("expected %q in %v for %q", tt.expected, detected, tt.text)
			}
		})
	}
}

func TestPIIDetectNoFalsePositives(t *testing.T) {
	g := NewPIIDetectionGuardrail()

	clean := []string{
		"the meeting is at 10am, room 4",
		"order 1234 was shipped",
		"invalid card 4111 1111 1111 1112",
		"invalid cpf 111.111.111-11",
		"invalid cnpj 11.222.333/0001-99",
		"invalid iban GB00WEST12345698765432",
		"revenue grew 1234 percent in 2024",
	}

	for _, text := range clean {
		if detected := g.Detect(text); len(detected) > 0 {
			t.Errorf("unexpected PII %v detected in %q", detected, text)
		}
	}
}

func TestPIIBlockMode(t *testing.T) {
	g := NewPIIDetectionGuardrail()
	ctx := context.Background()

	err := g.Check(ctx, "contact me at john@example.com")
	if err == nil {
		t.Fatal("expected a check error")
	}

	checkErr, ok := AsCheckError(err)
	if !ok {
		t.Fatalf("expected *CheckError, got %T", err)
	}
	if checkErr.Trigger != TriggerPIIDetected {
		t.Errorf("expected trigger %q, got %q", TriggerPIIDetected, checkErr.Trigger)
	}
	if checkErr.Stage != StageInput {
		t.Errorf("expected input stage, got %q", checkErr.Stage)
	}
	if !IsTrigger(err, TriggerPIIDetected) {
		t.Error("expected IsTrigger to report a PII trigger")
	}
	if detected, ok := checkErr.AdditionalData["detected_pii"].([]string); !ok || len(detected) != 1 || detected[0] != string(PIITypeEmail) {
		t.Errorf("unexpected additional data: %v", checkErr.AdditionalData)
	}

	if err := g.Check(ctx, "nothing sensitive here"); err != nil {
		t.Errorf("expected clean content to pass, got %v", err)
	}
}

func TestPIIMaskMode(t *testing.T) {
	g := NewPIIDetectionGuardrail(WithPIIMode(PIIModeMask))
	input := NewInput("email john@example.com and ssn 123-45-6789")

	if err := g.CheckInput(context.Background(), input); err != nil {
		t.Fatalf("mask mode must not fail the check: %v", err)
	}
	if strings.Contains(input.Content, "john@example.com") || strings.Contains(input.Content, "123-45-6789") {
		t.Fatalf("content was not masked: %q", input.Content)
	}
	if !strings.Contains(input.Content, strings.Repeat("*", len("john@example.com"))) {
		t.Errorf("expected asterisks of the same length: %q", input.Content)
	}
	if !strings.HasPrefix(input.Content, "email ") {
		t.Errorf("non-PII content must be preserved: %q", input.Content)
	}
}

func TestPIIRedactMode(t *testing.T) {
	g := NewPIIDetectionGuardrail(WithPIIMode(PIIModeRedact))
	output := NewOutput("write to john@example.com")

	if err := g.CheckOutput(context.Background(), output); err != nil {
		t.Fatalf("redact mode must not fail the check: %v", err)
	}
	if output.Content != "write to [REDACTED_EMAIL]" {
		t.Fatalf("unexpected redaction: %q", output.Content)
	}
}

func TestPIISanitizeKeepsInvalidMatches(t *testing.T) {
	g := NewPIIDetectionGuardrail(WithPIIMode(PIIModeRedact))
	// The number matches the credit card shape but fails the Luhn checksum.
	sanitized, detected := g.Sanitize("card 4111 1111 1111 1112")
	if len(detected) != 0 {
		t.Fatalf("expected no PII, got %v", detected)
	}
	if sanitized != "card 4111 1111 1111 1112" {
		t.Errorf("invalid match must not be redacted: %q", sanitized)
	}
}

func TestPIITypeSelection(t *testing.T) {
	only := NewPIIDetectionGuardrail(WithPIITypes(PIITypeEmail))
	if detected := only.Detect("ssn 123-45-6789 and mail a@b.co"); len(detected) != 1 || detected[0] != PIITypeEmail {
		t.Errorf("expected only email, got %v", detected)
	}

	without := NewPIIDetectionGuardrail(WithoutPIITypes(PIITypeEmail))
	if detected := without.Detect("mail a@b.co"); len(detected) != 0 {
		t.Errorf("expected email detection to be disabled, got %v", detected)
	}
}

func TestPIICustomPattern(t *testing.T) {
	g := NewPIIDetectionGuardrail(
		WithPIICustomPattern("employee_id", `\bEMP-\d{6}\b`),
		WithPIIMode(PIIModeRedact),
	)
	if detected := g.Detect("employee EMP-123456"); !containsType(detected, PIIType("employee_id")) {
		t.Fatalf("expected custom type, got %v", detected)
	}
	sanitized, _ := g.Sanitize("employee EMP-123456")
	if sanitized != "employee [REDACTED_EMPLOYEE_ID]" {
		t.Errorf("unexpected redaction: %q", sanitized)
	}
}

func TestPIIChecksumValidators(t *testing.T) {
	if !isValidLuhn("4111111111111111") {
		t.Error("expected a valid Luhn number")
	}
	if isValidLuhn("4111111111111112") {
		t.Error("expected an invalid Luhn number")
	}
	if !isValidCPF("529.982.247-25") {
		t.Error("expected a valid CPF")
	}
	if isValidCPF("529.982.247-26") {
		t.Error("expected an invalid CPF")
	}
	if isValidCPF("111.111.111-11") {
		t.Error("repeated digits must be rejected")
	}
	if !isValidCNPJ("11.222.333/0001-81") {
		t.Error("expected a valid CNPJ")
	}
	if isValidCNPJ("11.222.333/0001-82") {
		t.Error("expected an invalid CNPJ")
	}
	if !isValidIBAN("GB82 WEST 1234 5698 7654 32") {
		t.Error("expected a valid IBAN")
	}
	if isValidIBAN("GB83WEST12345698765432") {
		t.Error("expected an invalid IBAN")
	}
}
