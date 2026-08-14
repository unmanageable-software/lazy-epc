package epc

import "testing"

func TestPaymentPayloadValid(t *testing.T) {
	payment := Payment{
		Recipient: "Max Mustermann",
		IBAN:      "DE89 3704 0044 0532 0130 00",
		Amount:    1234,
		Reference: "RF18539007547034",
	}

	want := "BCD\n002\n1\nSCT\n\nMax Mustermann\nDE89370400440532013000\nEUR12.34\n\nRF18539007547034\n\n"

	got, err := payment.Payload()
	if err != nil {
		t.Fatalf("Payload() returned error: %v", err)
	}
	if got != want {
		t.Fatalf("Payload() = %q, want %q", got, want)
	}
}

func TestAmountString(t *testing.T) {
	tests := []struct {
		name  string
		cents int64
		want  string
	}{
		{name: "one cent", cents: 1, want: "EUR0.01"},
		{name: "one euro", cents: 100, want: "EUR1.00"},
		{name: "twelve thirty four", cents: 1234, want: "EUR12.34"},
		{name: "eight hundred fifty", cents: 85000, want: "EUR850.00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatCents(tc.cents); got != tc.want {
				t.Fatalf("formatCents(%d) = %q, want %q", tc.cents, got, tc.want)
			}
		})
	}
}

func TestPaymentPayloadValidation(t *testing.T) {
	tests := []struct {
		name    string
		payment Payment
		wantErr string
	}{
		{name: "missing recipient", payment: Payment{IBAN: "DE89370400440532013000", Amount: 1234}, wantErr: "recipient"},
		{name: "invalid iban", payment: Payment{Recipient: "Max Mustermann", IBAN: "NOTANIBAN", Amount: 1234}, wantErr: "IBAN"},
		{name: "invalid amount", payment: Payment{Recipient: "Max Mustermann", IBAN: "DE89370400440532013000", Amount: -1}, wantErr: "amount"},
		{name: "newline injection", payment: Payment{Recipient: "Bad\nRecipient", IBAN: "DE89370400440532013000", Amount: 1234}, wantErr: "newline"},
		{name: "structured and unstructured both set", payment: Payment{Recipient: "Max Mustermann", IBAN: "DE89370400440532013000", Amount: 1234, Reference: "RF18539007547034", UnstructuredRemittance: "Invoice 1"}, wantErr: "mutually exclusive"},
		{name: "length violation recipient", payment: Payment{Recipient: repeatString("x", 71), IBAN: "DE89370400440532013000", Amount: 1234}, wantErr: "recipient"},
		{name: "length violation reference", payment: Payment{Recipient: "Max Mustermann", IBAN: "DE89370400440532013000", Amount: 1234, Reference: repeatString("x", 36)}, wantErr: "reference"},
		{name: "payload too large", payment: buildLargePayment(), wantErr: "payload"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.payment.Payload()
			if err == nil {
				t.Fatalf("Payload() expected error")
			}
			if tc.wantErr != "" && !contains(err.Error(), tc.wantErr) {
				t.Fatalf("Payload() error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestPaymentPayloadAllowsOptionalBIC(t *testing.T) {
	payment := Payment{
		Recipient: "Max Mustermann",
		IBAN:      "DE89 3704 0044 0532 0130 00",
		Amount:    1234,
		BIC:       "DEUTDEFFXXX",
	}

	payload, err := payment.Payload()
	if err != nil {
		t.Fatalf("Payload() returned error: %v", err)
	}
	if !contains(payload, "\nDEUTDEFFXXX\nMax Mustermann\n") {
		t.Fatalf("Payload() missing BIC field: %q", payload)
	}
}

func TestIBANNormalization(t *testing.T) {
	payment := Payment{
		Recipient: "Max Mustermann",
		IBAN:      "de89 3704 0044 0532 0130 00",
		Amount:    1234,
	}
	payload, err := payment.Payload()
	if err != nil {
		t.Fatalf("Payload() returned error: %v", err)
	}
	if !contains(payload, "DE89370400440532013000") {
		t.Fatalf("Payload() did not normalize IBAN: %q", payload)
	}
}

func buildLargePayment() Payment {
	return Payment{
		Recipient:              repeatString("x", 70),
		IBAN:                   "DE89370400440532013000",
		Amount:                 1234,
		UnstructuredRemittance: repeatString("y", 140),
	}
}

func repeatString(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})())
}
