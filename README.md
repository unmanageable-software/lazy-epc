# lazy-epc

A small Go project for generating and archiving EPC QR / GiroCode payment documents.

## Architecture

The project intentionally follows a boring, inspectable flow:

```text
payment fields
    ↓
EPC payload
    ↓
qrencode
    ↓
PNG
    ↓
self-contained HTML
    ↓
SQLite
```

The design keeps responsibilities narrow and explicit:

- `cmd/giro` provides the CLI entry point.
- `internal/epc` holds the EPC payment structure and payload generation boundary.
- `internal/qr` converts a validated EPC payload into PNG bytes using the external `qrencode` executable.
- `internal/document` renders a self-contained payment document as plain HTML using a Base64 `data:` image URL.
- `internal/store` is the archive boundary for storing generated documents in SQLite.

`qrencode` is an intentional runtime dependency for this project stage. The QR package accepts a validated payload string and returns PNG bytes without introducing a Go QR library or a higher-level abstraction. Install `qrencode` and ensure it is available in `PATH` before running the demo.

The document package builds a portable HTML file from payment metadata and embedded PNG bytes. It does not yet add SQLite persistence or CLI workflows.

## CLI usage

Create a payment document from concrete payment data:

```bash
go run ./cmd/giro create \
  --recipient "Example GmbH" \
  --iban DE89370400440532013000 \
  --amount 12.34 \
  --reference "Invoice 2026-001"
```

This writes `payment.html` in the current working directory and prints `generated payment.html` when successful.

Optional TOML configuration is supported at `~/.config/lazy-epc/config.toml`.

```toml
# ~/.config/lazy-epc/config.toml
# optional values; defaults remain in effect when the file is absent

database = "~/payments.db"
output_dir = "~/giro-output"
timestamp_output = true
```

The `create` command accepts required `--recipient`, `--iban`, and `--amount` values and an optional `--reference` field. The amount parser accepts values like `12`, `12.3`, `12.34`, and `0.01`, and rejects malformed input, negatives, zero, and more than two decimal places.

When `timestamp_output = false`, generated files are named `payment.html`. When `timestamp_output = true`, they become `payment-YYYYMMDD-HHMMSS.html` in the configured `output_dir`. The config values are optional; if the config file is absent, the current default behavior stays the same. CLI flags override config values for the current run.

On successful generation, the CLI also opens or creates a local SQLite database named `payments.db` in the working directory and stores the generated payment record, including the EPC payload, rendered HTML, integer amount in cents, and a stable RFC3339 timestamp.

A simple demo command remains available for smoke testing:

```bash
go run ./cmd/giro demo
```

The project prefers boring, inspectable components over unnecessary application complexity.

## Roadmap

1. Define the payment model and EPC payload structure.
2. Generate QR code images through `qrencode`.
3. Assemble the self-contained HTML payment document.
4. Persist archived documents in SQLite.
5. Add focused CLI and document-generation checks as real requirements appear.

## Current status

This repository is intentionally a skeleton for the architecture and package boundaries only. It does not yet implement EPC generation, SQLite persistence, QR generation, or any production payment logic.
