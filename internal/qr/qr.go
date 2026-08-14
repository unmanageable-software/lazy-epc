package qr

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var (
	qrencodePath = "qrencode"
	pngSignature = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
)

// Encode turns a validated EPC payload into a PNG QR code.
func Encode(payload string) ([]byte, error) {
	if strings.TrimSpace(payload) == "" {
		return nil, errors.New("qr payload is empty")
	}

	cmd := exec.Command(qrencodePath, "-l", "M", "-o", "-")
	cmd.Stdin = strings.NewReader(payload)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) || strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "no such file or directory") {
			return nil, fmt.Errorf("qrencode not found in PATH: %w", err)
		}
		return nil, fmt.Errorf("qrencode failed: %s: %w", truncate(stderr.String(), 200), err)
	}

	data := stdout.Bytes()
	if len(data) == 0 {
		return nil, errors.New("qrencode produced no output")
	}
	if !bytes.HasPrefix(data, pngSignature) {
		return nil, errors.New("qrencode output is not valid PNG")
	}

	return data, nil
}

func truncate(s string, maxLen int) string {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) <= maxLen {
		return trimmed
	}
	return trimmed[:maxLen-3] + "..."
}
