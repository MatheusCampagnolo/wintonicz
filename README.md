# Wintonicz

A Windows support toolbox that runs **without installing**. One Go executable gives you a keyboard-driven terminal UI, a scriptable command line, and JSON output that scripts and AI agents can rely on.

> **Status: early alpha.** The read-only diagnostics below work from source. The one-command launcher script exists but needs a published release to download (not available yet). No telemetry, no background service, no silent updates.

## Run it without installing (once a release is published)

```powershell
irm https://raw.githubusercontent.com/MatheusCampagnolo/wintonicz/main/scripts/run.ps1 | iex
```

The launcher downloads the release executable quietly, verifies its SHA-256 against the release's `checksums.txt`, caches it under `%LOCALAPPDATA%\Wintonicz\cache\<version>\`, and starts it. Later runs start from the cache. It changes nothing else on your system.

Pass commands with the script-block form (`irm | iex` cannot forward arguments):

```powershell
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/MatheusCampagnolo/wintonicz/main/scripts/run.ps1))) port find 8080 --json
```

Options: `-Version 0.2.0` (pin), `-Admin` (start elevated), `-NoCache`, `-NoUpdate` (skip the newer-version check), `-ClearCache`. Each run checks for a newer release first and downloads it silently when there is one. Read the script before you run it — `irm <url>` prints it — and see its header comment for details. Remove everything Wintonicz stored with `-ClearCache`.

## Project direction

- Windows 10/11 x64 first; ARM64 after validation.
- One executable, three faces over one feature catalog: TUI, CLI, and a machine-readable manifest.
- Read-only diagnostics first; every system change is previewed, confirmed in proportion to its risk, logged locally, and has a documented undo path.
- Administrator rights only when a tool needs them, requested once by restarting the whole app elevated.
- No bulk software installation, no "optimize everything" button, no telemetry.

## Build from source

Install Go 1.26 or newer, clone this repository, and run:

```powershell
go build ./...
./scripts/build.ps1 -Version "0.1.0-dev"   # writes bin/wintonicz.exe (ignored by Git)
./bin/wintonicz.exe --help
```

## Read-only commands

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
./bin/wintonicz.exe service list --key
./bin/wintonicz.exe storage health
./bin/wintonicz.exe security status
./bin/wintonicz.exe startup list
./bin/wintonicz.exe program list
./bin/wintonicz.exe network quick
./bin/wintonicz.exe network ping 1.1.1.1 -n 4
./bin/wintonicz.exe why explorer          # also: why 1234, why --port 8080
./bin/wintonicz.exe report support --dry-run   # md/txt/json, redacted by default
./bin/wintonicz.exe report gpresult --dry-run

# Repairs, cleanup, reports: each previews first; add --yes to run (administrator where noted)
./bin/wintonicz.exe power set saver --dry-run   # records the old plan; `undo` switches back
./bin/wintonicz.exe undo list --yes
./bin/wintonicz.exe repair sfc --dry-run
./bin/wintonicz.exe repair dism --stage scan --dry-run
./bin/wintonicz.exe clean temp --dry-run
./bin/wintonicz.exe process end 1234 --dry-run
./bin/wintonicz.exe report eventlogs --dry-run
./bin/wintonicz.exe report download https://example.com/file.zip --sha256 <hash> --dry-run
./bin/wintonicz.exe panel list
./bin/wintonicz.exe panel open <key>
./bin/wintonicz.exe network flushdns --dry-run
./bin/wintonicz.exe tools list --json
```

Add `--json` to structured commands. `panel open` launches only a fixed, listed Windows panel — never an arbitrary executable or shell string. `service list` is read-only and never starts, stops, or reconfigures a service.

Every `--json` call prints one document: `{schemaVersion: 2, ok, tool, command, data, error, meta}`; failures use the same shape on stdout. Exit codes: 0 ok, 1 error, 2 usage, 3 needs administrator, 4 not run (preview, declined, read-only), 10 the Windows command failed. `wintonicz tools list --json` describes every tool; see [docs/ai.md](docs/ai.md) and [llms.txt](llms.txt). Preferences: `wintonicz config get|set` (`readOnly`, `jsonDefault`, `actionLog`, `defaultArea`, `alwaysAdmin`).

**Administrator:** by default the interface restarts itself as administrator at launch (one UAC prompt, new window). A declined prompt continues as a normal user. To stay as your normal user: `wintonicz --no-admin`, `WTZ_ADMIN=0`, or `wintonicz config set alwaysAdmin false`. Subcommands such as `port list` never elevate on their own.

`wintonicz pin --yes` (opt-in) copies the executable to `%LOCALAPPDATA%Wintoniczbin` and adds that folder to your user PATH; `wintonicz unpin --yes` reverses it.

## Try the TUI

Run in Windows Terminal or Console Host:

```powershell
./bin/wintonicz.exe
```

The window has a tool list on the left (grouped as Inspect, Network, Windows), the selected tool in the middle, and details on the right. Up/Down or `j`/`k` moves, Enter opens a tool, Esc or Left returns to the list, `1`–`4` jump to an area, `Ctrl+K` searches every tool (try "no internet"), `?` shows keys and the `[R]`/`[C]`/`A` risk symbols, `a` restarts as administrator, `r` refreshes, `/` filters a list, and `q` quits. Connectivity check asks for a `host:port`. "Flush DNS cache" is the first action: it shows its exact command, asks before running, and needs administrator. With redirected output the program prints help instead of taking over the terminal.


If Windows denies access to the TCP listener table, `port list` and the TCP Ports page show the collection error; they do not claim there are no listeners.

## Current limitations

- Process snapshots show PID, parent PID, executable name, and executable path when Windows allows access.
- Network status reports interfaces, addresses, gateways, and DNS servers. `network check` resolves the target and attempts one TCP connection with a timeout; it does not measure throughput.
- The system summary reports Windows product/build, uptime, logical processors, memory, and free/total space for the system drive.
- Service inventory reports state and PID; Windows may omit services the current account cannot query.
- USB inventory is a snapshot of devices present now; no event monitoring, no serial numbers.
- Storage inventory lists logical drives with type, capacity, and free space available to the current user.
- Resource snapshots sample CPU over about 500 ms; they are not continuous monitoring.
- Windows panel shortcuts are an allowlist; availability differs by Windows edition and version.
- Repairs, cleanup, and reversible changes ask for confirmation, show the exact command first, and write what they changed to `%LOCALAPPDATA%\Wintonicz\undo`. `wintonicz undo` reverses the newest one (restore points are rolled back with System Restore). They are alpha: several have not been run on a clean VM yet. (A CHKDSK-based "disk check" existed briefly and was removed; read-only disk health replaced it.)

## Tests

```powershell
go test ./...
```

The implementation roadmap and cloned reference projects are maintained outside this public repository, which keeps planning notes and upstream checkouts out of the product source tree.

## License

Wintonicz is distributed under the MIT License. See [LICENSE](LICENSE).
