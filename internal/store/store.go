package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/unmanageable-software/lazy-epc/internal/epc"

	_ "modernc.org/sqlite"
)

// PaymentRecord stores a generated payment with all fields needed to reconstruct
// or re-open the document later.
type PaymentRecord struct {
	ID          int64
	CreatedAt   time.Time
	Recipient   string
	IBAN        string
	AmountCents int64
	Reference   string
	EPCPayload  string
	HTML        string
}

// DB is the SQLite-backed payment archive.
type DB struct {
	db *sql.DB
}

// Open creates a SQLite DB at the given path and ensures the schema exists.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS payments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			created_at TEXT NOT NULL,
			recipient TEXT NOT NULL,
			iban TEXT NOT NULL,
			amount_cents INTEGER NOT NULL,
			reference TEXT,
			epc_payload TEXT NOT NULL,
			html TEXT NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create payments schema: %w", err)
	}

	return &DB{db: db}, nil
}

// Save inserts a payment record and returns the generated row ID.
func (db *DB) Save(payment epc.Payment, payload string, html string, createdAt time.Time) (int64, error) {
	if db == nil || db.db == nil {
		return 0, fmt.Errorf("database is not open")
	}
	if strings.TrimSpace(payload) == "" {
		return 0, fmt.Errorf("epc payload is required")
	}
	if strings.TrimSpace(html) == "" {
		return 0, fmt.Errorf("html is required")
	}

	res, err := db.db.Exec(`
		INSERT INTO payments (created_at, recipient, iban, amount_cents, reference, epc_payload, html)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, createdAt.Format(time.RFC3339), payment.Recipient, payment.IBAN, payment.Amount, payment.Reference, payload, html)
	if err != nil {
		return 0, fmt.Errorf("insert payment: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted payment id: %w", err)
	}
	return id, nil
}

// Close closes the underlying SQLite connection.
func (db *DB) Close() error {
	if db == nil || db.db == nil {
		return nil
	}
	return db.db.Close()
}

// GetByID fetches a stored payment by ID.
func (db *DB) GetByID(id int64) (*PaymentRecord, error) {
	row := db.db.QueryRow(`
		SELECT id, created_at, recipient, iban, amount_cents, reference, epc_payload, html
		FROM payments WHERE id = ?
	`, id)

	var record PaymentRecord
	var createdAt string
	if err := row.Scan(&record.ID, &createdAt, &record.Recipient, &record.IBAN, &record.AmountCents, &record.Reference, &record.EPCPayload, &record.HTML); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("payment %d not found", id)
		}
		return nil, fmt.Errorf("read payment: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	record.CreatedAt = parsed
	return &record, nil
}
