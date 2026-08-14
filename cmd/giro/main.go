package main

import (
	"fmt"
	"os"
	"time"

	"github.com/unmanageable-software/lazy-epc/internal/document"
	"github.com/unmanageable-software/lazy-epc/internal/epc"
	"github.com/unmanageable-software/lazy-epc/internal/qr"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "giro: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] != "demo" {
		return fmt.Errorf("usage: giro demo")
	}

	payment := epc.Payment{
		Recipient:              "Demo Recipient",
		IBAN:                   "DE89370400440532013000",
		Amount:                 1234,
		UnstructuredRemittance: "Invoice 2026-001",
	}

	payload, err := payment.Payload()
	if err != nil {
		return fmt.Errorf("generate EPC payload: %w", err)
	}

	png, err := qr.Encode(payload)
	if err != nil {
		return fmt.Errorf("generate QR PNG: %w", err)
	}

	html, err := document.Render(payment, time.Now(), png)
	if err != nil {
		return fmt.Errorf("render payment document: %w", err)
	}

	if err := os.WriteFile("payment.html", html, 0o644); err != nil {
		return fmt.Errorf("write payment.html: %w", err)
	}

	fmt.Println("generated payment.html")
	return nil
}
