package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/unmanageable-software/lazy-epc/internal/epc"
	"github.com/unmanageable-software/lazy-epc/internal/store"
)

func TestNewViewModelSortsNewestFirst(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "payments.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer db.Close()

	first := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	second := time.Date(2026, 8, 2, 9, 0, 0, 0, time.UTC)

	_, err = db.Save(epc.Payment{Recipient: "Old Recipient", IBAN: "DE89370400440532013000", Amount: 100, Reference: "A"}, "payload-1", "html-1", first, "old note")
	if err != nil {
		t.Fatalf("Save() first record: %v", err)
	}
	_, err = db.Save(epc.Payment{Recipient: "New Recipient", IBAN: "DE89370400440532013000", Amount: 200, Reference: "B"}, "payload-2", "html-2", second, "new note")
	if err != nil {
		t.Fatalf("Save() second record: %v", err)
	}

	vm, err := NewViewModel(db, "")
	if err != nil {
		t.Fatalf("NewViewModel() returned error: %v", err)
	}
	if len(vm.Rows) != 2 {
		t.Fatalf("len(vm.Rows) = %d, want 2", len(vm.Rows))
	}
	if vm.Rows[0].Recipient != "New Recipient" {
		t.Fatalf("vm.Rows[0] = %q, want newest item first", vm.Rows[0].Recipient)
	}
	if vm.Rows[1].Recipient != "Old Recipient" {
		t.Fatalf("vm.Rows[1] = %q, want older item second", vm.Rows[1].Recipient)
	}
}

func TestNewViewModelFiltersAcrossFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "payments.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open() returned error: %v", err)
	}
	defer db.Close()

	_, err = db.Save(epc.Payment{Recipient: "Acme GmbH", IBAN: "DE89370400440532013000", Amount: 1234, Reference: "INV-001"}, "payload-1", "html-1", time.Now(), "follow-up")
	if err != nil {
		t.Fatalf("Save() first record: %v", err)
	}
	_, err = db.Save(epc.Payment{Recipient: "Beta Ltd", IBAN: "FR1420041010050000000000013", Amount: 2500, Reference: "INV-002"}, "payload-2", "html-2", time.Now(), "other note")
	if err != nil {
		t.Fatalf("Save() second record: %v", err)
	}

	vm, err := NewViewModel(db, "follow")
	if err != nil {
		t.Fatalf("NewViewModel() returned error: %v", err)
	}
	if len(vm.Rows) != 1 {
		t.Fatalf("len(vm.Rows) = %d, want 1", len(vm.Rows))
	}
	if vm.Rows[0].Recipient != "Acme GmbH" {
		t.Fatalf("filtered row recipient = %q, want %q", vm.Rows[0].Recipient, "Acme GmbH")
	}
}
