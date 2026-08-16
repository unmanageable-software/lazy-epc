package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/unmanageable-software/lazy-epc/internal/config"
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

func TestSelectedDataRowIndex(t *testing.T) {
	tests := []struct {
		name     string
		selected int
		want     int
		ok       bool
	}{
		{name: "header", selected: 0, want: -1, ok: false},
		{name: "first row", selected: 1, want: 0, ok: true},
		{name: "second row", selected: 2, want: 1, ok: true},
		{name: "past end", selected: 99, want: -1, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := selectedDataRowIndex(tt.selected, 2)
			if ok != tt.ok {
				t.Fatalf("selectedDataRowIndex(%d, 2) ok = %v, want %v", tt.selected, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("selectedDataRowIndex(%d, 2) = %d, want %d", tt.selected, got, tt.want)
			}
		})
	}
}

func TestListStateTracksCurrentRowsAfterRefresh(t *testing.T) {
	state := &listState{}
	state.setRows([]Row{{ID: 1, Recipient: "first"}})
	if got := len(state.currentRows()); got != 1 {
		t.Fatalf("len(state.currentRows()) = %d, want 1", got)
	}
	state.setRows([]Row{{ID: 2, Recipient: "second"}, {ID: 3, Recipient: "third"}})
	if got := len(state.currentRows()); got != 2 {
		t.Fatalf("len(state.currentRows()) after refresh = %d, want 2", got)
	}
	if got := state.currentRows()[1].Recipient; got != "third" {
		t.Fatalf("state.currentRows()[1].Recipient = %q, want %q", got, "third")
	}
}

func TestConfigLoadUsesDefaultPathAndDefaults(t *testing.T) {
	cfg, err := config.Load("/tmp/does-not-exist")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Database != "payments.db" {
		t.Fatalf("cfg.Database = %q, want %q", cfg.Database, "payments.db")
	}
	if cfg.OutputDir != "." {
		t.Fatalf("cfg.OutputDir = %q, want %q", cfg.OutputDir, ".")
	}
	if cfg.TimestampOutput {
		t.Fatal("cfg.TimestampOutput = true, want false")
	}
}
