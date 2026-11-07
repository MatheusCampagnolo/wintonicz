# Wintonicz

Wintonicz is an early-stage Windows support toolbox planned as one Go application with a terminal UI and a scriptable command-line interface. Its first capabilities will focus on read-only diagnostics and useful Windows settings shortcuts.

The first read-only diagnostic pages and CLI commands are implemented. TCP listener collection is present; where Windows or the execution environment denies access, Wintonicz reports that error explicitly instead of showing an empty list.

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

## Available read-only commands

```powershell
./bin/wintonicz.exe system info
./bin/wintonicz.exe process list --name pwsh
./bin/wintonicz.exe process ports pwsh.exe
./bin/wintonicz.exe port list
./bin/wintonicz.exe port find 8080
./bin/wintonicz.exe network status
./bin/wintonicz.exe panel list
```

Add `--json` to structured data commands. JSON uses a `schemaVersion` and `command` envelope. `panel open <key>` launches a fixed, listed Windows panel; it never builds a command from an arbitrary executable or shell string.

## Try the TUI

Run the following in Windows Terminal or Console Host:

```powershell
./scripts/build.ps1 -Version "0.1.0-dev"
./bin/wintonicz.exe --help
./bin/wintonicz.exe --version
./bin/wintonicz.exe
```

Running with no arguments opens the TUI when both standard input and output are interactive. Use `1`–`5` or Tab to move through Overview, Processes, TCP Ports, Network, and Windows Panels. Use arrow keys or `j`/`k` in tables, `r` to refresh, Enter to open a selected panel, `?` for key help, and `q` to exit. If output is redirected, the program prints help instead of taking over the terminal.

If Windows denies access to the TCP listener table, `port list` and the Ports TUI page display the collection error; they do not claim that there are no listeners. Listener access can depend on the Windows environment, so validate this command on the target machine.

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

## Current limitations

- Process snapshots show PID, parent PID, executable name, and executable path when Windows allows access.
- Network status reports interfaces and assigned addresses; gateway, DNS, throughput, and internet-speed measurements are not included yet.
- System summary reports Windows product/build, uptime, logical processor count, and memory; disk and hardware model details are not included yet.
- Windows panel shortcuts are an allowlist; availability can differ by Windows edition/version.
- No system-changing tweaks, repair commands, release launcher, or public release are available yet.

The implementation roadmap and cloned reference projects are maintained outside this public repository. This keeps planning notes and upstream reference checkouts out of the product source tree.

## License

Wintonicz is distributed under the MIT License. See [LICENSE](LICENSE).
