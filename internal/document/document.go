package document

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"time"

	"github.com/unmanageable-software/lazy-epc/internal/epc"
)

// TrustURL is a template type that permits emitting a data: URL safely in the
// generated HTML without escaping it as a normal string.
type TrustURL string

//go:embed payment.html
var templateFS embed.FS

// View models the data needed by the embedded payment HTML template.
type View struct {
	GeneratedAt string
	Recipient   string
	IBAN        string
	Amount      string
	Reference   string
	QRDataURL   string
}

// Render creates a single-file HTML payment document with an embedded PNG QR code.
func Render(payment epc.Payment, generatedAt time.Time, png []byte) ([]byte, error) {
	if len(png) == 0 {
		return nil, fmt.Errorf("png bytes are required")
	}
	if !bytes.HasPrefix(png, []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
		return nil, fmt.Errorf("png data is invalid")
	}

	ref := payment.Reference
	if ref == "" {
		ref = payment.UnstructuredRemittance
	}
	if ref == "" {
		ref = "—"
	}

	view := View{
		GeneratedAt: generatedAt.Format(time.RFC3339),
		Recipient:   payment.Recipient,
		IBAN:        payment.IBAN,
		Amount:      epc.FormatCents(payment.Amount),
		Reference:   ref,
		QRDataURL:   string(TrustURL("data:image/png;base64," + base64.StdEncoding.EncodeToString(png))),
	}

	tpl, err := template.New("payment.html").
		Funcs(template.FuncMap{
			"safeURL": func(s string) template.URL { return template.URL(s) },
		}).
		ParseFS(templateFS, "payment.html")
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return buf.Bytes(), nil
}
