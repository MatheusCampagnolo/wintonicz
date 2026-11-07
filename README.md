# Wintonicz

Wintonicz is an early-stage Windows support toolbox planned as one Go application with a terminal UI and a scriptable command-line interface. Its first capabilities will focus on read-only diagnostics and useful Windows settings shortcuts.

The project is in its first interface milestone. The terminal shell and CLI help/version are available; diagnostic commands and the PowerShell release launcher are not implemented yet.

## Project direction

- Windows-first, with Windows x64 as the initial target.
- One executable with a keyboard-driven TUI and CLI over shared feature logic.
- Read-only diagnostics before system-changing maintenance actions.
- Clear permission limits, local-only reports, and no background service or telemetry by default.
- No bulk software installation or automatic “debloat” presets.

## Build from source

Install Go 1.26 or newer, clone this repository, and run:

```powershell
go build ./...
```

To build a local executable into the ignored `bin/` directory:

```powershell
./scripts/build.ps1
./bin/wintonicz.exe --help
```

Set a build version explicitly when needed:

```powershell
./scripts/build.ps1 -Version "0.1.0-alpha"
```

## Try the current shell

Run the following in Windows Terminal or Console Host:

```powershell
./scripts/build.ps1 -Version "0.1.0-dev"
./bin/wintonicz.exe --help
./bin/wintonicz.exe --version
./bin/wintonicz.exe
```

The first two commands print CLI help and the build version. Running with no arguments opens the TUI when both standard input and output are interactive; press `?` for key help and `q` to exit. If output is redirected, the program prints help instead of taking over the terminal.

To run the current automated checks:

```powershell
go test ./...
```

For a Windows cross-build from another operating system, set the Go target and build:

```powershell
$env:GOOS = "windows"
$env:GOARCH = "amd64"
go build -o bin/wintonicz.exe ./cmd/wintonicz
```

## Planned first diagnostic release

The first user-testable milestone is expected to cover a system overview, a read-only process browser, TCP port-to-process lookup in both directions, basic network status, and allowlisted Windows settings panels. The command set and support matrix will be documented as each capability is implemented.

The implementation roadmap and cloned reference projects are maintained outside this public repository. This keeps planning notes and upstream reference checkouts out of the product source tree.

## License

Wintonicz is distributed under the MIT License. See [LICENSE](LICENSE).
