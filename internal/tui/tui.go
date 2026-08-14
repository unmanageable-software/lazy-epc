package tui

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/unmanageable-software/lazy-epc/internal/store"
)

// Row is a TUI-friendly view model for a stored payment.
type Row struct {
	ID        int64
	Date      string
	Recipient string
	Amount    string
	Reference string
	Notes     string
	IBAN      string
}

// ViewModel contains the rows shown in the table and the active filter state.
type ViewModel struct {
	Rows      []Row
	Filter    string
	Selected  int
	Filtered  bool
	TotalRows int
}

// NewViewModel builds a view model from a database and optional filter text.
func NewViewModel(db *store.DB, filter string) (*ViewModel, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}

	records, err := db.Query(store.Filters{Recipient: filter, Reference: filter, Notes: filter, IBAN: filter})
	if err != nil {
		return nil, fmt.Errorf("query payments: %w", err)
	}

	rows := make([]Row, 0, len(records))
	for _, record := range records {
		rows = append(rows, Row{
			ID:        record.ID,
			Date:      record.CreatedAt.Format(time.RFC3339),
			Recipient: record.Recipient,
			Amount:    fmt.Sprintf("EUR%.2f", float64(record.AmountCents)/100.0),
			Reference: record.Reference,
			Notes:     record.Notes,
			IBAN:      record.IBAN,
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].ID > rows[j].ID })

	selected := 0
	if len(rows) > 0 {
		selected = 0
	}

	return &ViewModel{
		Rows:      rows,
		Filter:    filter,
		Selected:  selected,
		Filtered:  filter != "",
		TotalRows: len(rows),
	}, nil
}

// Run opens the SQLite database and starts the read-only TUI.
func Run() error {
	dbPath := filepath.Join(".", "payments.db")
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open payment database: %w", err)
	}
	defer db.Close()

	app := tview.NewApplication()
	list := tview.NewTable().SetSelectable(true, false)
	selectedRow := 0
	filterField := tview.NewInputField().SetPlaceholder("filter recipient / reference / notes / IBAN")
	filterField.SetLabel("/")
	filterField.SetChangedFunc(func(text string) {
		vm, err := NewViewModel(db, text)
		if err == nil {
			renderTable(list, vm)
			if len(vm.Rows) > 0 {
				selectedRow = 0
				list.Select(1, 0)
			}
		}
	})

	filterField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEsc:
			filterField.SetText("")
			vm, err := NewViewModel(db, "")
			if err == nil {
				renderTable(list, vm)
				selectedRow = 0
				if len(vm.Rows) > 0 {
					list.Select(1, 0)
				}
			}
			app.SetFocus(list)
		case tcell.KeyEnter:
			app.SetFocus(list)
		}
	})

	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.AddItem(filterField, 1, 0, false)
	root.AddItem(list, 0, 1, true)

	vm, err := NewViewModel(db, "")
	if err != nil {
		return err
	}
	renderTable(list, vm)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			if filterField.HasFocus() {
				filterField.SetText("")
				app.SetFocus(list)
				vm, err := NewViewModel(db, "")
				if err == nil {
					renderTable(list, vm)
					selectedRow = 0
					if len(vm.Rows) > 0 {
						list.Select(1, 0)
					}
				}
				return nil
			}
			return event
		case tcell.KeyRune:
			if event.Rune() == '/' && !filterField.HasFocus() {
				app.SetFocus(filterField)
				filterField.SetText("")
				return nil
			}
			if event.Rune() == 'q' && !filterField.HasFocus() {
				app.Stop()
				return nil
			}
		case tcell.KeyEnter:
			return nil
		case tcell.KeyUp:
			if list.GetRowCount() > 1 {
				if selectedRow > 0 {
					selectedRow--
					list.Select(selectedRow+1, 0)
				}
			}
			return nil
		case tcell.KeyDown:
			if list.GetRowCount() > 1 {
				if selectedRow < list.GetRowCount()-2 {
					selectedRow++
					list.Select(selectedRow+1, 0)
				}
			}
			return nil
		}
		return event
	})

	app.SetRoot(root, true)
	app.SetFocus(list)
	return app.Run()
}

func renderTable(table *tview.Table, vm *ViewModel) {
	table.Clear()
	table.SetBorder(true).SetTitle("Payments")

	head := []string{"ID", "Date", "Recipient", "Amount", "Reference", "Notes"}
	for i, col := range head {
		table.SetCell(0, i, tview.NewTableCell(col).SetTextColor(tcell.ColorYellow).SetSelectable(false))
	}

	for i, row := range vm.Rows {
		idx := i + 1
		table.SetCell(idx, 0, tview.NewTableCell(fmt.Sprintf("%d", row.ID)))
		table.SetCell(idx, 1, tview.NewTableCell(row.Date))
		table.SetCell(idx, 2, tview.NewTableCell(truncate(row.Recipient, 24)))
		table.SetCell(idx, 3, tview.NewTableCell(row.Amount))
		table.SetCell(idx, 4, tview.NewTableCell(truncate(row.Reference, 24)))
		table.SetCell(idx, 5, tview.NewTableCell(truncate(row.Notes, 32)))
	}

	if len(vm.Rows) == 0 {
		table.SetCell(1, 0, tview.NewTableCell("no payments").SetSelectable(false))
		table.SetSelectable(false, false)
		return
	}

	table.SetSelectable(true, false)
	table.Select(1, 0)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
