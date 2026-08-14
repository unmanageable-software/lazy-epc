package store

import (
	"database/sql"
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

	id, err := store.Save(payment, "BCD\n002\n1\nSCT\n", "<html>ok</html>", createdAt, "internal follow-up")
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
	if record.Notes != "internal follow-up" {
		t.Fatalf("Notes = %q, want %q", record.Notes, "internal follow-up")
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

	_, err = store.Save(epc.Payment{Recipient: "A", IBAN: "DE89370400440532013000", Amount: 1234}, "payload", "html", time.Now().UTC(), "note")
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

func TestUpgradeOldSchemaToIncludeNotes(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "payments.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() returned error: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE payments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TEXT NOT NULL,
			recipient TEXT NOT NULL,
			iban TEXT NOT NULL,
			amount_cents INTEGER NOT NULL,
			reference TEXT,
			epc_payload TEXT NOT NULL,
			html TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	_, err = db.Exec(`INSERT INTO payments (created_at, recipient, iban, amount_cents, reference, epc_payload, html) VALUES (?, ?, ?, ?, ?, ?, ?)`, time.Now().UTC().Format(time.RFC3339), "Legacy", "DE89370400440532013000", 1234, "Ref-1", "payload", "html")
	if err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() upgrade returned error: %v", err)
	}
	defer store.Close()

	row := store.db.QueryRow(`SELECT notes FROM payments WHERE recipient = ?`, "Legacy")
	var notes string
	if err := row.Scan(&notes); err != nil {
		t.Fatalf("scan notes after upgrade: %v", err)
	}
	if notes != "" {
		t.Fatalf("upgraded notes should default to empty string, got %q", notes)
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

func TestFilterQueries(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "payments.db")
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer store.Close()

	_, err = store.Save(epc.Payment{Recipient: "Acme GmbH", IBAN: "DE89370400440532013000", Amount: 1234, Reference: "INV-001"}, "payload-1", "html-1", time.Now().UTC(), "first note")
	if err != nil {
		t.Fatalf("Save() first row: %v", err)
	}
	_, err = store.Save(epc.Payment{Recipient: "Beta Ltd", IBAN: "FR1420041010050000000000013", Amount: 2500, Reference: "INV-002"}, "payload-2", "html-2", time.Now().UTC(), "second note")
	if err != nil {
		t.Fatalf("Save() second row: %v", err)
	}

	rows, err := store.Query(Filters{Recipient: "acme"})
	if err != nil {
		t.Fatalf("Query(recipient) returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Recipient != "Acme GmbH" {
		t.Fatalf("Query(recipient) = %#v, want one acme row", rows)
	}

	rows, err = store.Query(Filters{Reference: "INV-002"})
	if err != nil {
		t.Fatalf("Query(reference) returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Reference != "INV-002" {
		t.Fatalf("Query(reference) = %#v, want one INV-002 row", rows)
	}

	rows, err = store.Query(Filters{IBAN: "FR14"})
	if err != nil {
		t.Fatalf("Query(iban) returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].IBAN != "FR1420041010050000000000013" {
		t.Fatalf("Query(iban) = %#v, want one FR14 row", rows)
	}

	rows, err = store.Query(Filters{Notes: "second"})
	if err != nil {
		t.Fatalf("Query(notes) returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].Notes != "second note" {
		t.Fatalf("Query(notes) = %#v, want one second note row", rows)
	}
}

func TestMain(m *testing.M) {
	_ = os.RemoveAll("/tmp/lazy-epc-tests")
	os.Exit(m.Run())
}
