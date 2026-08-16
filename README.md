# lazy-epc

A small Go application for generating, reusing, and archiving EPC QR / GiroCode payment documents.

`lazy-epc` is intentionally a **workable MVP**, not a polished end-user application.

If you're reasonably comfortable with Go, Linux, SQLite, and installing command-line dependencies, you should be able to clone it, build it, and use it.

## Why?

Some invoices still arrive on paper with all the information required for a bank transfer.

Typing the recipient, IBAN, amount, and reference into a banking app every time is tedious.

EPC QR codes already provide a standard way to encode that information.

`lazy-epc` turns those payment details into a QR code that compatible banking applications can scan.

For recurring payments, previous entries can be searched, cloned, edited, and regenerated from the terminal UI.

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

Responsibilities remain deliberately narrow:

* `cmd/giro` — CLI entry point
* `internal/epc` — EPC payment validation and payload generation
* `internal/qr` — QR generation through `qrencode`
* `internal/document` — self-contained HTML documents with embedded QR images
* `internal/store` — SQLite payment archive
* `internal/tui` — terminal interface for finding and reusing previous payments

The project prefers boring, inspectable components over unnecessary application complexity.

## Requirements

* Go
* `qrencode` available in `PATH`
* a reasonably modern Linux terminal

`qrencode` is currently an intentional external runtime dependency.

## CLI

Create a payment:

```bash
go run ./cmd/giro create \
  --recipient "Example GmbH" \
  --iban DE89370400440532013000 \
  --amount 12.34 \
  --reference "Invoice 2026-001"
```

Amounts are parsed directly into integer cents without floating-point arithmetic.

Values such as these are accepted:

```text
12
12.3
12.34
0.01
```

Malformed values, zero/negative amounts, and values with more than two decimal places are rejected.

Generated payments are stored in SQLite together with their EPC payload and self-contained HTML document.

A simple demo command is also available for smoke testing:

```bash
go run ./cmd/giro demo
```

## Terminal UI

Launch the TUI:

```bash
go run ./cmd/giro tui
```

The TUI provides a deliberately old-school database-terminal workflow.

Previous payments can be:

* browsed
* filtered
* opened
* cloned into new payments
* edited
* updated
* deleted with confirmation

A previous payment can therefore become the starting point for a new one: find it, change the amount or reference, and generate a new payment.

Notes can also be attached to database entries. Notes are local metadata and are never included in the EPC payment payload.

Generated documents can be opened from the TUI using the system's default HTML handler.

## Configuration

Optional configuration lives at:

```text
~/.config/lazy-epc/config.toml
```

Example:

```toml
database = "~/payments.db"
output_dir = "~/giro-output"
timestamp_output = true
```

When `timestamp_output = false`, generated documents use:

```text
payment.html
```

When enabled:

```text
payment-YYYYMMDD-HHMMSS.html
```

The generated HTML is completely self-contained. The QR PNG is embedded directly into the document as a Base64 data URL.

No web server is required to view it.

## Current scope

This repository provides the MVP that I personally wanted to use.

It works, but it assumes that the person installing it is reasonably comfortable around the Go/Linux ecosystem.

There are deliberately still rough edges.

Rather than hiding those behind premature packaging, installers, containers, or a large dependency stack, the current version stays small and understandable.

## Ideas / Backlog

The MVP does what it was built to do. The following are ideas for where it could go next, not commitments.

### Pure Go QR generation

Replace the external `qrencode` dependency with a lightweight Go QR implementation.

This would remove the external runtime dependency and make it easier to distribute `lazy-epc` as a self-contained binary.

### Better TUI navigation

The current terminal UI is intentionally minimal.

Possible improvements include:

* mouse navigation
* additional keyboard shortcuts
* more discoverable keybindings
* additional views for browsing or inspecting stored payments

The goal would remain a fast, keyboard-friendly, old-school database-terminal interface rather than turning the application into a graphical UI.

### Easier installation

Provide pre-built binaries and packaging for common platforms instead of requiring users to have a Go development environment.

### Desktop integration

Register `lazy-epc` as a custom protocol handler so links such as:

```text
lazyepc://payment/...
```

could open directly in the installed application.

### Service version

If there is enough interest, a future hosted version could provide payment requests that can be opened by the local application.

A custom protocol handler could allow the service to hand an opaque payment identifier or token to `lazy-epc` without embedding banking information directly into URLs.

### And probably other things

This project grew out of solving an actual annoyance rather than designing a product roadmap.

If continued use reveals another small feature that makes the workflow noticeably better, that is probably a better reason to build it than trying to predict everything upfront.

## Non-goals

For now, `lazy-epc` deliberately avoids:

* web frameworks
* frontend frameworks
* user accounts
* cloud storage
* ORMs
* unnecessary abstraction layers
* becoming a payment platform

Good enough and useful beats comprehensive.

## Status

The MVP works.

It can generate EPC payment QR codes, produce portable HTML documents, archive payments in SQLite, search previous payments, and reuse them through the terminal interface.

For now, that's enough.

**Turn annoying payment details into a QR code, remember them for next time, and get out of the way.**
