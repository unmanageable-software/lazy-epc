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
	Notes       string
	EPCPayload  string
	HTML        string
}

// Filters are optional query constraints used by the archive layer for later
// filtering by the most common payment fields.
type Filters struct {
	Recipient string
	IBAN      string
	Reference string
	Notes     string
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
			notes TEXT NOT NULL DEFAULT '',
			epc_payload TEXT NOT NULL,
			html TEXT NOT NULL
		)
	`); err != nil {
		db.Close()
		return nil, fmt.Errorf("create payments schema: %w", err)
	}

	if err := ensureNotesColumn(db); err != nil {
		db.Close()
		return nil, err
	}

	return &DB{db: db}, nil
}

func ensureNotesColumn(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(payments)`)
	if err != nil {
		return fmt.Errorf("read payments schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int64
		var name string
		var typ string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return fmt.Errorf("scan payments schema: %w", err)
		}
		if name == "notes" {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate payments schema: %w", err)
	}

	if _, err := db.Exec(`ALTER TABLE payments ADD COLUMN notes TEXT NOT NULL DEFAULT ''`); err != nil {
		return fmt.Errorf("upgrade payments schema to include notes: %w", err)
	}
	return nil
}

// Save inserts a payment record and returns the generated row ID.
func (db *DB) Save(payment epc.Payment, payload string, html string, createdAt time.Time, notes ...string) (int64, error) {
	if db == nil || db.db == nil {
		return 0, fmt.Errorf("database is not open")
	}
	if strings.TrimSpace(payload) == "" {
		return 0, fmt.Errorf("epc payload is required")
	}
	if strings.TrimSpace(html) == "" {
		return 0, fmt.Errorf("html is required")
	}

	noteText := ""
	if len(notes) > 0 {
		noteText = notes[0]
	}

	res, err := db.db.Exec(`
		INSERT INTO payments (created_at, recipient, iban, amount_cents, reference, notes, epc_payload, html)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, createdAt.Format(time.RFC3339), payment.Recipient, payment.IBAN, payment.Amount, payment.Reference, noteText, payload, html)
	if err != nil {
		return 0, fmt.Errorf("insert payment: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted payment id: %w", err)
	}
	return id, nil
}

func (db *DB) Update(id int64, payment epc.Payment, payload string, html string, createdAt time.Time, notes ...string) error {
	if db == nil || db.db == nil {
		return fmt.Errorf("database is not open")
	}
	if strings.TrimSpace(payload) == "" {
		return fmt.Errorf("epc payload is required")
	}
	if strings.TrimSpace(html) == "" {
		return fmt.Errorf("html is required")
	}

	noteText := ""
	if len(notes) > 0 {
		noteText = notes[0]
	}

	res, err := db.db.Exec(`
		UPDATE payments
		SET created_at = ?, recipient = ?, iban = ?, amount_cents = ?, reference = ?, notes = ?, epc_payload = ?, html = ?
		WHERE id = ?
	`, createdAt.Format(time.RFC3339), payment.Recipient, payment.IBAN, payment.Amount, payment.Reference, noteText, payload, html, id)
	if err != nil {
		return fmt.Errorf("update payment %d: %w", id, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read updated payment rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("payment %d not found", id)
	}
	return nil
}

func (db *DB) Delete(id int64) error {
	if db == nil || db.db == nil {
		return fmt.Errorf("database is not open")
	}

	res, err := db.db.Exec(`DELETE FROM payments WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete payment %d: %w", id, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted payment rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("payment %d not found", id)
	}
	return nil
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
		SELECT id, created_at, recipient, iban, amount_cents, reference, notes, epc_payload, html
		FROM payments WHERE id = ?
	`, id)

	var record PaymentRecord
	var createdAt string
	if err := row.Scan(&record.ID, &createdAt, &record.Recipient, &record.IBAN, &record.AmountCents, &record.Reference, &record.Notes, &record.EPCPayload, &record.HTML); err != nil {
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

// Query searches the archive using optional filters. Empty filters are ignored.
func (db *DB) Query(filters Filters) ([]PaymentRecord, error) {
	clauses := []string{}
	args := []any{}

	fields := []struct {
		name  string
		value string
	}{
		{name: "recipient", value: filters.Recipient},
		{name: "iban", value: filters.IBAN},
		{name: "reference", value: filters.Reference},
		{name: "notes", value: filters.Notes},
	}

	nonEmpty := make([]struct {
		name  string
		value string
	}, 0, len(fields))
	for _, field := range fields {
		if field.value != "" {
			nonEmpty = append(nonEmpty, field)
		}
	}

	if len(nonEmpty) > 0 {
		if len(nonEmpty) > 1 && allSameValue(nonEmpty) {
			matches := make([]string, 0, len(nonEmpty))
			for _, field := range nonEmpty {
				matches = append(matches, "LOWER("+field.name+") LIKE ?")
				args = append(args, "%"+strings.ToLower(field.value)+"%")
			}
			clauses = append(clauses, "("+strings.Join(matches, " OR ")+")")
		} else {
			for _, field := range nonEmpty {
				clauses = append(clauses, "LOWER("+field.name+") LIKE ?")
				args = append(args, "%"+strings.ToLower(field.value)+"%")
			}
		}
	}

	query := `SELECT id, created_at, recipient, iban, amount_cents, reference, notes, epc_payload, html FROM payments`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY id ASC"

	rows, err := db.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query payments: %w", err)
	}
	defer rows.Close()

	var records []PaymentRecord
	for rows.Next() {
		var record PaymentRecord
		var createdAt string
		if err := rows.Scan(&record.ID, &createdAt, &record.Recipient, &record.IBAN, &record.AmountCents, &record.Reference, &record.Notes, &record.EPCPayload, &record.HTML); err != nil {
			return nil, fmt.Errorf("scan payment: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse created_at: %w", err)
		}
		record.CreatedAt = parsed
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate payments: %w", err)
	}
	return records, nil
}

func allSameValue(fields []struct {
	name  string
	value string
}) bool {
	if len(fields) < 2 {
		return true
	}
	first := strings.TrimSpace(fields[0].value)
	for _, field := range fields[1:] {
		if strings.TrimSpace(field.value) != first {
			return false
		}
	}
	return true
}
