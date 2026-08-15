package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/unmanageable-software/lazy-epc/internal/document"
	"github.com/unmanageable-software/lazy-epc/internal/epc"
	"github.com/unmanageable-software/lazy-epc/internal/qr"
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

type listState struct {
	rows []Row
}

func (s *listState) setRows(rows []Row) {
	s.rows = rows
}

func (s *listState) currentRows() []Row {
	return s.rows
}

// FormValues describes the data entry fields on the clone form.
type FormValues struct {
	Recipient string
	IBAN      string
	Amount    string
	Reference string
	Notes     string
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
			Amount:    formatAmountDisplay(record.AmountCents),
			Reference: record.Reference,
			Notes:     record.Notes,
			IBAN:      record.IBAN,
		})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].ID > rows[j].ID })

	return &ViewModel{
		Rows:      rows,
		Filter:    filter,
		Selected:  0,
		Filtered:  filter != "",
		TotalRows: len(rows),
	}, nil
}

// Run opens the SQLite database and starts the TUI.
func Run() error {
	dbPath := filepath.Join(".", "payments.db")
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open payment database: %w", err)
	}
	defer db.Close()

	app := tview.NewApplication()
	list, filterField, rowsState, refreshList := buildList(db)
	pages := tview.NewPages()
	pages.AddPage("list", buildListPage(list, filterField), true, true)
	app.SetRoot(pages, true)
	app.SetFocus(list)

	list.SetSelectedFunc(func(row, column int) {
		currentRows := rowsState.currentRows()
		idx, ok := selectedDataRowIndex(row, len(currentRows))
		if !ok {
			return
		}
		selectedID := currentRows[idx].ID
		selected, err := db.GetByID(selectedID)
		if err != nil {
			showFormError(app, pages, list, "could not load payment: "+err.Error())
			return
		}
		showCloneForm(app, pages, db, list, refreshList, selected)
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			if filterField.HasFocus() {
				filterField.SetText("")
				refreshList()
				app.SetFocus(list)
				return nil
			}
			return event
		case tcell.KeyRune:
			if event.Rune() == '/' && !filterField.HasFocus() {
				app.SetFocus(filterField)
				return nil
			}
			if event.Rune() == 'q' && !filterField.HasFocus() {
				app.Stop()
				return nil
			}
		case tcell.KeyUp:
			row, _ := list.GetSelection()
			if row > 1 {
				list.Select(row-1, 0)
			}
			return nil
		case tcell.KeyDown:
			row, _ := list.GetSelection()
			if row == 0 {
				if list.GetRowCount() > 1 {
					list.Select(1, 0)
				}
				return nil
			}
			if row+1 < list.GetRowCount() {
				list.Select(row+1, 0)
			}
			return nil
		}
		return event
	})

	filterField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEsc:
			filterField.SetText("")
			refreshList()
			app.SetFocus(list)
		case tcell.KeyEnter:
			refreshList()
			app.SetFocus(list)
		}
	})

	filterField.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			filterField.SetText("")
			refreshList()
			app.SetFocus(list)
			return nil
		case tcell.KeyEnter:
			refreshList()
			app.SetFocus(list)
			return nil
		}
		return event
	})

	filterField.SetChangedFunc(func(text string) {
		refreshList()
	})

	refreshList()
	app.SetFocus(list)
	return app.Run()
}

func buildList(db *store.DB) (*tview.Table, *tview.InputField, *listState, func()) {
	list := tview.NewTable().SetSelectable(true, false)
	filterField := tview.NewInputField().SetPlaceholder("filter recipient / reference / notes / IBAN")
	filterField.SetLabel("/")

	state := &listState{}
	refreshList := func() {
		vm, err := NewViewModel(db, filterField.GetText())
		if err != nil {
			return
		}
		state.setRows(vm.Rows)
		renderTable(list, vm)
		if len(vm.Rows) > 0 {
			list.Select(1, 0)
		}
		list.SetSelectable(true, false)
	}

	return list, filterField, state, refreshList
}

func buildListPage(list *tview.Table, filterField *tview.InputField) *tview.Flex {
	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.AddItem(list, 0, 1, true)
	root.AddItem(filterField, 1, 0, false)
	footer := tview.NewTextView().SetText("↑↓ Navigate   Enter Open   / Filter   q Quit").SetTextColor(tcell.ColorGray)
	root.AddItem(footer, 1, 0, false)
	return root
}

func selectedDataRowIndex(tableRow int, totalRows int) (int, bool) {
	if tableRow <= 0 {
		return -1, false
	}
	idx := tableRow - 1
	if idx >= totalRows {
		return -1, false
	}
	return idx, true
}

func showFormError(app *tview.Application, pages *tview.Pages, list *tview.Table, msg string) {
	if msg == "" {
		pages.SwitchToPage("list")
		app.SetFocus(list)
		return
	}
	modal := tview.NewModal().SetText(msg).AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		pages.SwitchToPage("list")
		app.SetFocus(list)
	})
	pages.AddPage("error", modal, true, true)
	app.SetFocus(modal)
}

func showCloneForm(app *tview.Application, pages *tview.Pages, db *store.DB, list *tview.Table, refreshList func(), original *store.PaymentRecord) {
	if original == nil {
		showFormError(app, pages, list, "no payment selected")
		return
	}

	values := FormValues{
		Recipient: original.Recipient,
		IBAN:      original.IBAN,
		Amount:    formatAmountString(original.AmountCents),
		Reference: original.Reference,
		Notes:     original.Notes,
	}

	recipientField := tview.NewInputField().SetLabel("Recipient").SetText(values.Recipient)
	ibanField := tview.NewInputField().SetLabel("IBAN").SetText(values.IBAN)
	amountField := tview.NewInputField().SetLabel("Amount").SetText(values.Amount)
	referenceField := tview.NewInputField().SetLabel("Reference").SetText(values.Reference)
	notesField := tview.NewInputField().SetLabel("Notes").SetText(values.Notes)
	errorView := tview.NewTextView().SetTextColor(tcell.ColorRed)

	form := tview.NewForm()
	form.SetBorder(true).SetTitle("Clone payment")
	form.AddFormItem(recipientField)
	form.AddFormItem(ibanField)
	form.AddFormItem(amountField)
	form.AddFormItem(referenceField)
	form.AddFormItem(notesField)
	form.AddButton("Create", func() {
		formValues := FormValues{
			Recipient: strings.TrimSpace(recipientField.GetText()),
			IBAN:      strings.TrimSpace(ibanField.GetText()),
			Amount:    strings.TrimSpace(amountField.GetText()),
			Reference: strings.TrimSpace(referenceField.GetText()),
			Notes:     strings.TrimSpace(notesField.GetText()),
		}
		newPayment, err := paymentFromFormValues(formValues)
		if err != nil {
			errorView.SetText("validation: " + err.Error())
			return
		}

		generatedAt := time.Now().UTC()
		payload, html, err := generatePaymentDocument(newPayment, "payment.html", generatedAt)
		if err != nil {
			errorView.SetText("generate: " + err.Error())
			return
		}
		if _, err := db.Save(newPayment, payload, html, generatedAt, formValues.Notes); err != nil {
			errorView.SetText("save: " + err.Error())
			return
		}
		pages.SwitchToPage("list")
		app.SetFocus(list)
		refreshList()
	})
	form.AddButton("Update", func() {
		if original == nil {
			errorView.SetText("update: no payment selected")
			return
		}
		formValues := FormValues{
			Recipient: strings.TrimSpace(recipientField.GetText()),
			IBAN:      strings.TrimSpace(ibanField.GetText()),
			Amount:    strings.TrimSpace(amountField.GetText()),
			Reference: strings.TrimSpace(referenceField.GetText()),
			Notes:     strings.TrimSpace(notesField.GetText()),
		}
		updatedPayment, err := paymentFromFormValues(formValues)
		if err != nil {
			errorView.SetText("validation: " + err.Error())
			return
		}

		generatedAt := time.Now().UTC()
		payload, html, err := generatePaymentDocument(updatedPayment, "payment.html", generatedAt)
		if err != nil {
			errorView.SetText("generate: " + err.Error())
			return
		}
		if err := db.Update(original.ID, updatedPayment, payload, html, generatedAt, formValues.Notes); err != nil {
			errorView.SetText("update: " + err.Error())
			return
		}
		pages.SwitchToPage("list")
		app.SetFocus(list)
		refreshList()
	})
	form.AddButton("Delete", func() {
		if original == nil {
			errorView.SetText("delete: no payment selected")
			return
		}
		modal := tview.NewModal().SetText(fmt.Sprintf("Delete payment #%d for %s?\nThis action cannot be undone.", original.ID, original.Recipient)).AddButtons([]string{"Cancel", "Delete"})
		modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonIndex == 0 || buttonLabel == "Cancel" {
				pages.SwitchToPage("clone")
				app.SetFocus(form)
				return
			}
			if err := db.Delete(original.ID); err != nil {
				errorView.SetText("delete: " + err.Error())
				pages.SwitchToPage("clone")
				app.SetFocus(form)
				return
			}
			pages.SwitchToPage("list")
			app.SetFocus(list)
			refreshList()
		})
		pages.AddPage("delete-confirm", modal, true, true)
		app.SetFocus(modal)
	})
	form.AddButton("Cancel", func() {
		pages.SwitchToPage("list")
		app.SetFocus(list)
	})
	form.SetCancelFunc(func() {
		pages.SwitchToPage("list")
		app.SetFocus(list)
	})
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			pages.SwitchToPage("list")
			app.SetFocus(list)
			return nil
		}
		return event
	})

	page := tview.NewFlex().SetDirection(tview.FlexRow)
	page.AddItem(form, 0, 1, true)
	page.AddItem(errorView, 1, 0, false)
	pages.AddPage("clone", page, true, true)
	pages.SwitchToPage("clone")
	app.SetFocus(form)
}

func paymentFromFormValues(values FormValues) (epc.Payment, error) {
	cents, err := parseDecimalAmount(values.Amount)
	if err != nil {
		return epc.Payment{}, err
	}

	payment := epc.Payment{
		Recipient: values.Recipient,
		IBAN:      values.IBAN,
		Amount:    cents,
		Reference: values.Reference,
	}
	if _, err := payment.Payload(); err != nil {
		return epc.Payment{}, err
	}
	return payment, nil
}

func generatePaymentDocument(payment epc.Payment, outputPath string, generatedAt time.Time) (string, string, error) {
	payload, err := payment.Payload()
	if err != nil {
		return "", "", fmt.Errorf("generate EPC payload: %w", err)
	}

	png, err := qr.Encode(payload)
	if err != nil {
		return "", "", fmt.Errorf("generate QR PNG: %w", err)
	}

	htmlBytes, err := document.Render(payment, generatedAt, png)
	if err != nil {
		return "", "", fmt.Errorf("render payment document: %w", err)
	}

	if err := os.WriteFile(outputPath, htmlBytes, 0o644); err != nil {
		return "", "", fmt.Errorf("write %s: %w", outputPath, err)
	}
	return payload, string(htmlBytes), nil
}

func parseDecimalAmount(raw string) (int64, error) {
	amount := strings.TrimSpace(raw)
	if amount == "" {
		return 0, fmt.Errorf("amount is required")
	}
	if strings.HasPrefix(amount, "+") {
		amount = strings.TrimPrefix(amount, "+")
	}
	if strings.HasPrefix(amount, "-") {
		return 0, fmt.Errorf("amount must be positive")
	}
	parts := strings.Split(amount, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("amount must be a number with at most 2 decimal places")
	}
	whole := parts[0]
	if whole == "" {
		whole = "0"
	}
	frac := ""
	if len(parts) == 2 {
		frac = parts[1]
		if len(frac) > 2 {
			return 0, fmt.Errorf("amount must be a number with at most 2 decimal places")
		}
	}
	if whole == "" || !isDigits(whole) {
		return 0, fmt.Errorf("amount is invalid")
	}
	if frac != "" && !isDigits(frac) {
		return 0, fmt.Errorf("amount is invalid")
	}
	wholeCents, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount is invalid")
	}
	cents := wholeCents * 100
	if frac != "" {
		if len(frac) == 1 {
			frac += "0"
		}
		fracCents, err := strconv.ParseInt(frac, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("amount is invalid")
		}
		cents += fracCents
	}
	if cents <= 0 {
		return 0, fmt.Errorf("amount must be positive")
	}
	return cents, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func formatAmountDisplay(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole := cents / 100
	frac := cents % 100
	return fmt.Sprintf("%sEUR%d.%02d", sign, whole, frac)
}

func formatAmountString(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	whole := cents / 100
	frac := cents % 100
	return fmt.Sprintf("%s%d.%02d", sign, whole, frac)
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
