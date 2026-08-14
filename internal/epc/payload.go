package epc

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxRecipientLen  = 70
	maxIBANLen       = 34
	maxPurposeLen    = 4
	maxReferenceLen  = 35
	maxPayloadLen    = 250
	maxRemittanceLen = 140
)

var (
	ibanPattern = regexp.MustCompile(`^[A-Z]{2}[A-Z0-9]{11,30}$`)
	bicPattern  = regexp.MustCompile(`^[A-Z]{4}[A-Z]{2}[A-Z0-9]{2}([A-Z0-9]{3})?$`)
)

// Payment represents the structured payment data required to build an EPC QR payload.
type Payment struct {
	Recipient              string
	IBAN                   string
	BIC                    string
	Amount                 int64
	Purpose                string
	Reference              string
	UnstructuredRemittance string
}

// Payload generates the EPC QR / GiroCode payload using the BCD/002/1/SCT format.
func (p Payment) Payload() (string, error) {
	if err := p.validate(); err != nil {
		return "", err
	}

	fields := []string{
		"BCD",
		"002",
		"1",
		"SCT",
		p.normalizedBIC(),
		p.Recipient,
		p.normalizedIBAN(),
		formatCents(p.Amount),
		p.Purpose,
		p.Reference,
		p.UnstructuredRemittance,
	}

	payload := strings.Join(fields, "\n") + "\n"
	if len(payload) > maxPayloadLen {
		return "", fmt.Errorf("payload exceeds maximum allowed size of %d bytes", maxPayloadLen)
	}
	return payload, nil
}

func (p Payment) validate() error {
	if strings.TrimSpace(p.Recipient) == "" {
		return fmt.Errorf("recipient is required")
	}
	if err := validateTextField("recipient", p.Recipient, maxRecipientLen); err != nil {
		return err
	}
	if strings.TrimSpace(p.IBAN) == "" {
		return fmt.Errorf("IBAN is required")
	}
	if err := validateIBAN(p.IBAN); err != nil {
		return err
	}
	if p.Amount < 0 {
		return fmt.Errorf("amount must be zero or greater")
	}
	if p.Amount > 9999999999999 {
		return fmt.Errorf("amount exceeds EPC QR supported range")
	}
	if p.BIC != "" {
		if err := validateBIC(p.BIC); err != nil {
			return err
		}
	}
	if p.Purpose != "" {
		if err := validateTextField("purpose", p.Purpose, maxPurposeLen); err != nil {
			return err
		}
	}
	if p.Reference != "" && p.UnstructuredRemittance != "" {
		return fmt.Errorf("structured and unstructured remittance information are mutually exclusive")
	}
	if p.Reference != "" {
		if err := validateTextField("reference", p.Reference, maxReferenceLen); err != nil {
			return err
		}
	}
	if p.UnstructuredRemittance != "" {
		if err := validateTextField("unstructured remittance", p.UnstructuredRemittance, maxRemittanceLen); err != nil {
			return err
		}
	}
	if err := validateNoNewlines("recipient", p.Recipient); err != nil {
		return err
	}
	if err := validateNoNewlines("IBAN", p.IBAN); err != nil {
		return err
	}
	if err := validateNoNewlines("BIC", p.BIC); err != nil {
		return err
	}
	if err := validateNoNewlines("purpose", p.Purpose); err != nil {
		return err
	}
	if err := validateNoNewlines("reference", p.Reference); err != nil {
		return err
	}
	if err := validateNoNewlines("unstructured remittance", p.UnstructuredRemittance); err != nil {
		return err
	}

	payload := strings.Join([]string{
		"BCD",
		"002",
		"1",
		"SCT",
		p.normalizedBIC(),
		p.Recipient,
		p.normalizedIBAN(),
		formatCents(p.Amount),
		p.Purpose,
		p.Reference,
		p.UnstructuredRemittance,
	}, "\n") + "\n"
	if len(payload) > maxPayloadLen {
		return fmt.Errorf("payload exceeds maximum allowed size of %d bytes", maxPayloadLen)
	}
	return nil
}

func (p Payment) normalizedIBAN() string {
	return sanitizeIBAN(p.IBAN)
}

func (p Payment) normalizedBIC() string {
	if p.BIC == "" {
		return ""
	}
	return sanitizeBIC(p.BIC)
}

func sanitizeIBAN(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(s), ""))
}

func sanitizeBIC(s string) string {
	return strings.ToUpper(strings.Join(strings.Fields(s), ""))
}

func validateIBAN(s string) error {
	normalized := sanitizeIBAN(s)
	if normalized == "" {
		return fmt.Errorf("IBAN is required")
	}
	if utf8.RuneCountInString(normalized) > maxIBANLen {
		return fmt.Errorf("IBAN exceeds maximum length of %d characters", maxIBANLen)
	}
	if !ibanPattern.MatchString(normalized) {
		return fmt.Errorf("IBAN is invalid")
	}
	return nil
}

func validateBIC(s string) error {
	normalized := sanitizeBIC(s)
	if normalized == "" {
		return nil
	}
	if utf8.RuneCountInString(normalized) != 8 && utf8.RuneCountInString(normalized) != 11 {
		return fmt.Errorf("BIC is invalid")
	}
	if !bicPattern.MatchString(normalized) {
		return fmt.Errorf("BIC is invalid")
	}
	return nil
}

func validateTextField(name, value string, maxLen int) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	if utf8.RuneCountInString(value) > maxLen {
		return fmt.Errorf("%s exceeds maximum length of %d characters", name, maxLen)
	}
	if err := validateNoNewlines(name, value); err != nil {
		return err
	}
	return nil
}

func validateNoNewlines(field, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s contains newline characters which are not allowed", field)
	}
	return nil
}

func formatCents(cents int64) string {
	return FormatCents(cents)
}

// FormatCents converts a money value from integer euro cents into a standard
// EUR display string without using floating-point arithmetic.
func FormatCents(cents int64) string {
	if cents < 0 {
		return "EUR0.00"
	}
	euros := cents / 100
	centsPart := cents % 100
	return fmt.Sprintf("EUR%d.%02d", euros, centsPart)
}
