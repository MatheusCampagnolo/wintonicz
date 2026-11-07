# Wintonicz Repository Instructions

- Keep product-facing text and documentation in English.
- Preserve the shared-service architecture: CLI and TUI use the same feature and Windows platform layers.
- Keep Windows API access in `internal/platform/windows`; do not put operating-system operations in TUI rendering.
- Use Bubble Tea v2 commands for asynchronous work. Keep `View` a pure rendering function of model state and terminal size.
- Reuse the pinned Bubble Tea, Bubbles, and Lip Gloss dependencies before considering a new dependency.
- Treat system-changing operations as explicit workflows with a review step, clear impact, and a recovery path.
- Run `gofmt` on changed Go files. CI runs `go test ./...`, `go vet ./...`, and `go build ./...` on Windows.
