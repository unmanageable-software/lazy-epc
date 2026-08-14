package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr string
	}{
		{name: "whole number", input: "12", want: 1200},
		{name: "single decimal", input: "12.3", want: 1230},
		{name: "two decimals", input: "12.34", want: 1234},
		{name: "small amount", input: "0.01", want: 1},
		{name: "leading zero", input: "000.10", want: 10},
		{name: "too many decimals", input: "12.345", wantErr: "decimal"},
		{name: "empty", input: "", wantErr: "amount"},
		{name: "non numeric", input: "abc", wantErr: "amount"},
		{name: "negative", input: "-1.00", wantErr: "positive"},
		{name: "zero", input: "0", wantErr: "positive"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseAmount(tc.input)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("parseAmount(%q) = %d, nil; want error containing %q", tc.input, got, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseAmount(%q) error = %q, want substring %q", tc.input, err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAmount(%q) returned error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseAmount(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestCreateCommand(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "payment.html")

	err := createCommand([]string{
		"--recipient", "Example GmbH",
		"--iban", "DE89370400440532013000",
		"--amount", "12.34",
		"--reference", "Invoice 2026-001",
	}, outputPath)
	if err != nil {
		t.Fatalf("createCommand returned error: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(content), "Example GmbH") {
		t.Fatalf("output HTML missing recipient: %s", content)
	}
	if !strings.Contains(string(content), "EUR12.34") {
		t.Fatalf("output HTML missing formatted amount: %s", content)
	}
}

func TestCreateCommandRequiresFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing recipient", args: []string{"--iban", "DE89370400440532013000", "--amount", "12.34"}, want: "recipient"},
		{name: "missing iban", args: []string{"--recipient", "Example GmbH", "--amount", "12.34"}, want: "iban"},
		{name: "missing amount", args: []string{"--recipient", "Example GmbH", "--iban", "DE89370400440532013000"}, want: "amount"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := createCommand(tc.args, filepath.Join(t.TempDir(), "payment.html"))
			if err == nil {
				t.Fatal("createCommand() expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("createCommand() error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}
