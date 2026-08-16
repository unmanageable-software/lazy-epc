package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/unmanageable-software/lazy-epc/internal/config"
	"github.com/unmanageable-software/lazy-epc/internal/document"
	"github.com/unmanageable-software/lazy-epc/internal/epc"
	"github.com/unmanageable-software/lazy-epc/internal/qr"
	"github.com/unmanageable-software/lazy-epc/internal/store"
	"github.com/unmanageable-software/lazy-epc/internal/tui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "giro: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	cfg, err := config.LoadDefault()
	if err != nil {
		return err
	}

	switch args[0] {
	case "demo":
		if len(args) != 1 {
			return usageError()
		}
		return runDemo(cfg)
	case "create":
		return createCommandWithConfig(args[1:], cfg, "")
	case "tui":
		return tuiCommand(args[1:], cfg)
	default:
		return usageError()
	}
}

func usageError() error {
	return fmt.Errorf("usage: giro create --recipient \"Example GmbH\" --iban DE89370400440532013000 --amount 12.34 [--reference \"Invoice 2026-001\"] [--notes \"Optional internal note\"] | giro demo | giro tui")
}

func runDemo(cfg config.Config) error {
	outputPath := cfg.PaymentOutputPath(time.Now().UTC())
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory %s: %w", cfg.OutputDir, err)
	}
	payment := epc.Payment{
		Recipient:              "Demo Recipient",
		IBAN:                   "DE89370400440532013000",
		Amount:                 1234,
		UnstructuredRemittance: "Invoice 2026-001",
	}
	_, _, err := generatePaymentDocument(payment, outputPath, time.Now().UTC())
	return err
}

func createCommand(args []string, outputPath string) error {
	cfg, err := config.LoadDefault()
	if err != nil {
		return err
	}
	return createCommandWithConfig(args, cfg, outputPath)
}

func createCommandWithConfig(args []string, cfg config.Config, outputPath string) error {
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		recipient      string
		iban           string
		amountStr      string
		reference      string
		notes          string
		overrideDB     string
		overrideOutput string
		overrideTS     bool
	)
	fs.StringVar(&recipient, "recipient", "", "recipient name")
	fs.StringVar(&iban, "iban", "", "IBAN")
	fs.StringVar(&amountStr, "amount", "", "amount in EUR, for example 12.34")
	fs.StringVar(&reference, "reference", "", "optional payment reference")
	fs.StringVar(&notes, "notes", "", "optional local note stored with the payment")
	fs.StringVar(&overrideDB, "database", cfg.Database, "database path")
	fs.StringVar(&overrideOutput, "output-dir", cfg.OutputDir, "output directory")
	fs.BoolVar(&overrideTS, "timestamp-output", cfg.TimestampOutput, "include a timestamp in the output filename")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse arguments: %w", err)
	}

	cfg.Database = config.ExpandPath(overrideDB)
	cfg.OutputDir = config.ExpandPath(overrideOutput)
	cfg.TimestampOutput = overrideTS
	if outputPath == "" {
		outputPath = cfg.PaymentOutputPath(time.Now().UTC())
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory %s: %w", filepath.Dir(outputPath), err)
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory %s: %w", cfg.OutputDir, err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Database), 0o755); err != nil {
		return fmt.Errorf("create database directory %s: %w", filepath.Dir(cfg.Database), err)
	}

	if strings.TrimSpace(recipient) == "" {
		return fmt.Errorf("recipient is required")
	}
	if strings.TrimSpace(iban) == "" {
		return fmt.Errorf("iban is required")
	}
	if strings.TrimSpace(amountStr) == "" {
		return fmt.Errorf("amount is required")
	}

	cents, err := parseAmount(amountStr)
	if err != nil {
		return fmt.Errorf("parse amount %q: %w", amountStr, err)
	}

	payment := epc.Payment{
		Recipient: recipient,
		IBAN:      iban,
		Amount:    cents,
		Reference: reference,
	}

	generatedAt := time.Now().UTC()
	payload, html, err := generatePaymentDocument(payment, outputPath, generatedAt)
	if err != nil {
		return err
	}

	dbPath := cfg.Database
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open payment database: %w", err)
	}
	defer db.Close()

	id, err := db.Save(payment, payload, html, generatedAt, notes)
	if err != nil {
		return fmt.Errorf("store payment record: %w", err)
	}

	fmt.Printf("generated %s\nstored payment #%d\n", outputPath, id)
	return nil
}

func tuiCommand(args []string, cfg config.Config) error {
	fs := flag.NewFlagSet("tui", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		overrideDB     string
		overrideOutput string
		overrideTS     bool
	)
	fs.StringVar(&overrideDB, "database", cfg.Database, "database path")
	fs.StringVar(&overrideOutput, "output-dir", cfg.OutputDir, "output directory")
	fs.BoolVar(&overrideTS, "timestamp-output", cfg.TimestampOutput, "include a timestamp in the output filename")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse arguments: %w", err)
	}
	cfg.Database = config.ExpandPath(overrideDB)
	cfg.OutputDir = config.ExpandPath(overrideOutput)
	cfg.TimestampOutput = overrideTS
	return tui.Run(cfg)
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

func parseAmount(raw string) (int64, error) {
	amount := strings.TrimSpace(raw)
	if amount == "" {
		return 0, fmt.Errorf("amount is required")
	}
	if strings.HasPrefix(amount, "-") {
		return 0, fmt.Errorf("amount must be positive")
	}

	amount = strings.TrimPrefix(amount, "+")

	parts := strings.Split(amount, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("amount must be a number with at most 2 decimal places")
	}

	whole := parts[0]
	if whole == "" {
		whole = "0"
	}
	if whole != "0" && strings.TrimLeft(whole, "0") == "" {
		whole = "0"
	}
	if whole == "" || !isDigits(whole) {
		return 0, fmt.Errorf("amount is invalid")
	}

	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) > 2 {
			return 0, fmt.Errorf("amount must be a number with at most 2 decimal places")
		}
		if fraction != "" && !isDigits(fraction) {
			return 0, fmt.Errorf("amount is invalid")
		}
	}

	wholeCents, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount is invalid")
	}

	cents := wholeCents * 100
	if fraction != "" {
		if len(fraction) == 1 {
			fraction += "0"
		}
		fracCents, err := strconv.ParseInt(fraction, 10, 64)
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
