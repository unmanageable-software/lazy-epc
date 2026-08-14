package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unmanageable-software/lazy-epc/internal/epc"
)

func TestSchemaCreation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "payments.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer store.Close()

	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='payments'`).Scan(&count); err != nil {
		t.Fatalf("query schema metadata: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 payments table, got %d", count)
	}
}

func TestSaveAndReadStoredValues(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "payments.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer store.Close()

	createdAt := time.Date(2026, 8, 14, 23, 57, 12, 0, time.UTC)
	payment := epc.Payment{
		Recipient: "Example GmbH",
		IBAN:      "DE89370400440532013000",
		Amount:    1234,
		Reference: "Invoice 2026-001",
	}

	id, err := store.Save(payment, "BCD\n002\n1\nSCT\n", "<html>ok</html>", createdAt)
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if id != 1 {
		t.Fatalf("Save() id = %d, want 1", id)
	}

	record, err := store.GetByID(id)
	if err != nil {
		t.Fatalf("GetByID() returned error: %v", err)
	}
	if record.Recipient != payment.Recipient {
		t.Fatalf("Recipient = %q, want %q", record.Recipient, payment.Recipient)
	}
	if record.IBAN != payment.IBAN {
		t.Fatalf("IBAN = %q, want %q", record.IBAN, payment.IBAN)
	}
	if record.AmountCents != payment.Amount {
		t.Fatalf("AmountCents = %d, want %d", record.AmountCents, payment.Amount)
	}
	if record.Reference != payment.Reference {
		t.Fatalf("Reference = %q, want %q", record.Reference, payment.Reference)
	}
	if record.EPCPayload != "BCD\n002\n1\nSCT\n" {
		t.Fatalf("EPCPayload = %q, want payload", record.EPCPayload)
	}
	if record.HTML != "<html>ok</html>" {
		t.Fatalf("HTML = %q, want encoded HTML", record.HTML)
	}
	if !record.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreatedAt = %s, want %s", record.CreatedAt.Format(time.RFC3339), createdAt.Format(time.RFC3339))
	}
}

func TestReopenExistingDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "payments.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}

	_, err = store.Save(epc.Payment{Recipient: "A", IBAN: "DE89370400440532013000", Amount: 1234}, "payload", "html", time.Now().UTC())
	if err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	store, err = Open(dbPath)
	if err != nil {
		t.Fatalf("re-open returned error: %v", err)
	}
	defer store.Close()

	var count int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM payments`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row after reopen, got %d", count)
	}
}

func TestSaveRejectsMissingPayloadOrHTML(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "payments.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer store.Close()

	_, err = store.Save(epc.Payment{Recipient: "A", IBAN: "DE89370400440532013000", Amount: 1234}, "", "html", time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "epc payload") {
		t.Fatalf("Save() with empty payload error = %v, want substring %q", err, "epc payload")
	}

	_, err = store.Save(epc.Payment{Recipient: "A", IBAN: "DE89370400440532013000", Amount: 1234}, "payload", "", time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "html") {
		t.Fatalf("Save() with empty HTML error = %v, want substring %q", err, "html")
	}
}

func TestMain(m *testing.M) {
	_ = os.RemoveAll("/tmp/lazy-epc-tests")
	os.Exit(m.Run())
}
