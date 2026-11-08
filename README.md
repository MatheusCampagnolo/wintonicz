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
./bin/wintonicz.exe network check example.com:443
./bin/wintonicz.exe service list
./bin/wintonicz.exe usb list
./bin/wintonicz.exe storage list
./bin/wintonicz.exe resource snapshot
./bin/wintonicz.exe support disk-check C:
./bin/wintonicz.exe panel list
```

Add `--json` to structured data commands. Successful JSON output uses a `schemaVersion`, `command`, and `data` envelope. On failure, Wintonicz writes a readable error to stderr and does not emit an error envelope to stdout. `support disk-check C:` prints a preview only; add `--yes` to request UAC approval and run the exact `chkdsk C:` command in a temporary elevated console. The console shows CHKDSK output and closes when it finishes; Wintonicz returns the duration and exit status. A nonzero CHKDSK result is returned as the process exit code. On an active volume, Windows may report inconclusive errors if it cannot lock the drive. `service list` reports the service name, display name, state, and PID without changing service configuration. `panel open <key>` launches a fixed, listed Windows panel; it never builds a command from an arbitrary executable or shell string.

## Try the TUI

Run the following in Windows Terminal or Console Host:

```powershell
./scripts/build.ps1 -Version "0.1.0-dev"
./bin/wintonicz.exe --help
./bin/wintonicz.exe --version
./bin/wintonicz.exe
```

Running with no arguments in an interactive terminal opens the TUI without an elevation prompt. Home groups the tools into Overview, Diagnostics, Windows tools, and Support. Switch areas with Tab/Shift+Tab or keys `1`–`4`; each area's selectable tools appear immediately below the tabs. Use Up/Down or `j`/`k` to select an option, then Enter to open it. Windows tools shows the panel catalog directly. Support → Read-only disk check shows the selected drive and exact command beside the drive list. Press Enter once on the selected drive to request administrator consent and run it in a temporary elevated console. Read the CHKDSK output there; when it closes, Wintonicz shows the command duration and exit status. Canceling UAC cancels the action. CLI diagnostics keep the caller's privilege level; `support disk-check C: --yes` requests elevation for that action. Press Esc or Backspace to return to Home. In Processes, press `/` to edit a process-name or PID filter; Backspace/Delete remove text, Ctrl+U clears it, Enter applies it, and Esc cancels it. In Network, press `c`, enter a `host:port`, then press Enter for a DNS/TCP check or Esc to cancel. This explicitly contacts only the target you entered and uses a three-second timeout. Press `r` to refresh a diagnostic view, `?` for contextual key help, and `q` to exit. If output is redirected, the program prints help instead of taking over the terminal.

If Windows denies access to the TCP listener table, `port list` and the TCP Ports TUI page display the collection error; they do not claim that there are no listeners. Listener access can depend on the Windows environment, so validate this command on the target machine.

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
- Network status reports interfaces, assigned addresses, gateways, and DNS servers. `network check <host:port>` or the TUI's `c` action explicitly resolves the target and attempts a TCP connection with a timeout; it does not measure throughput or internet speed.
- System summary reports Windows product/build, uptime, logical processor count, memory, and quota-aware free/total space for the Windows system drive. Hardware model details are not included yet.
- Service inventory is read-only and reports current Windows service state and PID; Windows may omit services the current account cannot query. Startup configuration and service controls are not included.
- USB inventory is an on-demand snapshot of currently present USB devices. It does not monitor connection events or expose device serial numbers.
- Storage inventory lists Windows logical drives, drive type, total capacity, and free space available to the current user. Empty or inaccessible drives stay visible with an unavailable status.
- Resource snapshots sample CPU activity over about 500 ms and read physical memory at the end of the sample. They are on-demand snapshots, not continuous monitoring.
- Windows panel shortcuts are an allowlist; availability can differ by Windows edition/version.
- System-changing tweaks and repair commands are not available. The read-only disk check is the first guided support action. The release launcher and public release are not available yet.

The implementation roadmap and cloned reference projects are maintained outside this public repository. This keeps planning notes and upstream reference checkouts out of the product source tree.

## License

Wintonicz is distributed under the MIT License. See [LICENSE](LICENSE).
