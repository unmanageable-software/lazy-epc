package qr

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEncodeSuccess(t *testing.T) {
	payload := "BCD\n002\n1\nSCT\n\nMax Mustermann\nDE89370400440532013000\nEUR12.34\n\nRF18539007547034\n"
	script, argsFile, payloadFile := makeFakeEncoder(t, "ok")
	oldPath := qrencodePath
	qrencodePath = script
	defer func() { qrencodePath = oldPath }()

	png, err := Encode(payload)
	if err != nil {
		t.Fatalf("Encode() returned error: %v", err)
	}
	if !bytes.HasPrefix(png, pngSignature) {
		t.Fatalf("Encode() output is not a PNG: %x", png[:8])
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("reading args file: %v", err)
	}
	if got := strings.TrimSpace(string(args)); got != "-l\nM\n-o\n-" {
		t.Fatalf("Encode() called with args %q, want %q", got, "-l\nM\n-o\n-")
	}

	inPayload, err := os.ReadFile(payloadFile)
	if err != nil {
		t.Fatalf("reading payload file: %v", err)
	}
	if string(inPayload) != payload {
		t.Fatalf("stdin payload = %q, want %q", string(inPayload), payload)
	}
}

func TestEncodeNotFound(t *testing.T) {
	oldPath := qrencodePath
	qrencodePath = filepath.Join(t.TempDir(), "missing-qrencode")
	defer func() { qrencodePath = oldPath }()

	_, err := Encode("BCD\n002\n1\nSCT\n")
	if err == nil {
		t.Fatal("Encode() expected error for missing qrencode")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Encode() error = %q, want 'not found'", err)
	}
}

func TestEncodeFailure(t *testing.T) {
	script, _, _ := makeFakeEncoder(t, "fail")
	oldPath := qrencodePath
	qrencodePath = script
	defer func() { qrencodePath = oldPath }()

	_, err := Encode("BCD\n002\n1\nSCT\n")
	if err == nil {
		t.Fatal("Encode() expected subprocess error")
	}
	if !strings.Contains(err.Error(), "qrencode failed") {
		t.Fatalf("Encode() error = %q, want qrencode failed", err)
	}
}

func TestEncodeInvalidOutput(t *testing.T) {
	script, _, _ := makeFakeEncoder(t, "invalid")
	oldPath := qrencodePath
	qrencodePath = script
	defer func() { qrencodePath = oldPath }()

	_, err := Encode("BCD\n002\n1\nSCT\n")
	if err == nil {
		t.Fatal("Encode() expected invalid PNG error")
	}
	if !strings.Contains(err.Error(), "not valid PNG") {
		t.Fatalf("Encode() error = %q, want invalid PNG", err)
	}
}

func TestEncodeEmptyOutput(t *testing.T) {
	script, _, _ := makeFakeEncoder(t, "empty")
	oldPath := qrencodePath
	qrencodePath = script
	defer func() { qrencodePath = oldPath }()

	_, err := Encode("BCD\n002\n1\nSCT\n")
	if err == nil {
		t.Fatal("Encode() expected empty output error")
	}
	if !strings.Contains(err.Error(), "no output") {
		t.Fatalf("Encode() error = %q, want no output", err)
	}
}

func TestEncodeRejectsEmptyPayload(t *testing.T) {
	_, err := Encode("   ")
	if err == nil {
		t.Fatal("Encode() expected empty payload error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("Encode() error = %q, want empty payload", err)
	}
}

func makeFakeEncoder(t *testing.T, mode string) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "qrencode")
	argsFile := filepath.Join(dir, "args.txt")
	payloadFile := filepath.Join(dir, "payload.txt")

	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > \"$QRENCODE_ARGS_FILE\"\n" +
		"cat > \"$QRENCODE_PAYLOAD_FILE\"\n" +
		"case \"${QRENCODE_MODE:-ok}\" in\n" +
		"  fail) echo 'qrencode failed' >&2; exit 1 ;;\n" +
		"  invalid) echo 'not a png'; exit 0 ;;\n" +
		"  empty) exit 0 ;;\n" +
		"  *) printf '\\211PNG\\r\\n\\032\\n'; printf 'fake-png'; exit 0 ;;\n" +
		"esac\n"

	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake encoder: %v", err)
	}
	if err := os.Setenv("QRENCODE_ARGS_FILE", argsFile); err != nil {
		t.Fatalf("set args env: %v", err)
	}
	if err := os.Setenv("QRENCODE_PAYLOAD_FILE", payloadFile); err != nil {
		t.Fatalf("set payload env: %v", err)
	}
	if err := os.Setenv("QRENCODE_MODE", mode); err != nil {
		t.Fatalf("set mode env: %v", err)
	}

	return scriptPath, argsFile, payloadFile
}

func TestTruncate(t *testing.T) {
	got := truncate("1234567890", 7)
	if !reflect.DeepEqual(got, "1234...") {
		t.Fatalf("truncate() = %q, want %q", got, "1234...")
	}
}
