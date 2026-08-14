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

## Demo

```bash
go run ./cmd/giro demo
```

This writes `payment.html` in the current working directory and prints `generated payment.html` when successful.

The project prefers boring, inspectable components over unnecessary application complexity.

## Roadmap

1. Define the payment model and EPC payload structure.
2. Generate QR code images through `qrencode`.
3. Assemble the self-contained HTML payment document.
4. Persist archived documents in SQLite.
5. Add focused CLI and document-generation checks as real requirements appear.

## Current status

This repository is intentionally a skeleton for the architecture and package boundaries only. It does not yet implement EPC generation, SQLite persistence, QR generation, or any production payment logic.
