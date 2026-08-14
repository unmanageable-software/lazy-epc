package document

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/unmanageable-software/lazy-epc/internal/epc"
)

func TestRender(t *testing.T) {
	payment := epc.Payment{
		Recipient: "Max Mustermann",
		IBAN:      "DE89370400440532013000",
		Amount:    1234,
		Reference: "RF18539007547034",
	}
	generatedAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	png := validPNGBytes()

	html, err := Render(payment, generatedAt, png)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}

	got := string(html)
	for _, want := range []string{
		"Max Mustermann",
		"DE89370400440532013000",
		"EUR12.34",
		"RF18539007547034",
		"2026-01-02T03:04:05Z",
		"data:image/png;base64,",
		"<html lang=\"en\">",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render() missing %q in HTML: %s", want, got)
		}
	}
	if strings.Contains(got, "http://") || strings.Contains(got, "https://") {
		t.Fatalf("Render() should not include external asset references: %s", got)
	}
}

func TestRenderUsesUnstructuredRemittance(t *testing.T) {
	payment := epc.Payment{
		Recipient:              "A. Person",
		IBAN:                   "DE89370400440532013000",
		Amount:                 500,
		UnstructuredRemittance: "Invoice 42",
	}
	png := validPNGBytes()

	html, err := Render(payment, time.Unix(0, 0).UTC(), png)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}
	if !strings.Contains(string(html), "Invoice 42") {
		t.Fatalf("Render() did not show the unstructured remittance: %s", html)
	}
}

func TestRenderEscapesUserInput(t *testing.T) {
	payment := epc.Payment{
		Recipient: "<script>alert(1)</script>",
		IBAN:      "DE89370400440532013000",
		Amount:    100,
		Reference: "<b>oops</b>",
	}
	png := validPNGBytes()

	html, err := Render(payment, time.Unix(0, 0).UTC(), png)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}
	got := string(html)
	if strings.Contains(got, "<script>") || strings.Contains(got, "<b>") {
		t.Fatalf("Render() did not escape HTML input: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("Render() did not escape recipient: %s", got)
	}
}

func TestRenderInvalidPNG(t *testing.T) {
	payment := epc.Payment{Recipient: "A", IBAN: "DE89370400440532013000", Amount: 100}
	if _, err := Render(payment, time.Now(), []byte("not-a-png")); err == nil {
		t.Fatal("Render() expected invalid PNG error")
	}
}

func validPNGBytes() []byte {
	return append([]byte{}, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 'x', 'y'}...)
}

func TestRenderUsesStructuredReferenceWhenPresent(t *testing.T) {
	payment := epc.Payment{
		Recipient: "A. Person",
		IBAN:      "DE89370400440532013000",
		Amount:    2500,
		Reference: "RF18539007547034",
	}
	png := validPNGBytes()

	html, err := Render(payment, time.Unix(0, 0).UTC(), png)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}
	if !strings.Contains(string(html), "RF18539007547034") {
		t.Fatalf("Render() did not show the structured reference: %s", html)
	}
}

func TestRenderContainsBase64PNGDataURL(t *testing.T) {
	payment := epc.Payment{Recipient: "A", IBAN: "DE89370400440532013000", Amount: 10}
	png := validPNGBytes()
	dataURL := "data:image/png;base64,"

	html, err := Render(payment, time.Unix(0, 0).UTC(), png)
	if err != nil {
		t.Fatalf("Render() returned error: %v", err)
	}
	if !bytes.Contains(html, []byte(dataURL)) {
		t.Fatalf("Render() should contain a data:image/png;base64 URL: %s", html)
	}
}
