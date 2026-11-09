# Using Wintonicz from scripts and AI agents

Wintonicz is built so a program can use it as safely as a person can.

## Quick start

```powershell
wintonicz tools list --json          # what exists, how risky, who may run it
wintonicz port find 8080 --json      # a read tool
wintonicz network flushdns --dry-run # preview an action, exit 0
wintonicz network flushdns --yes     # run it (needs an administrator terminal)
```

Without installing, use the launcher's script-block form (see the README).

## Output

Every `--json` call prints exactly one document and nothing else on stdout:

```json
{
  "schemaVersion": 2,
  "ok": true,
  "tool": "inspect.ports",
  "command": "port find 8080",
  "data": [],
  "warnings": [],
  "meta": { "elevated": false, "version": "0.2.0" }
}
```

Failures use the same shape with `"ok": false` and an `error` object (`code`, `message`, optional `hint`). `schemaVersion` changes only for breaking changes.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | Error |
| 2 | Usage error (bad command, flag, or value) |
| 3 | Administrator rights required |
| 4 | Not run: preview only, declined, cancelled, or read-only mode |
| 10 | The underlying Windows command ran and returned non-zero |

## Safety contract

- Tools have a `risk`: `read`, `launch`, `change`, `destructive`; and an `admin` need: `none`, `optional`, `required`.
- `read` and `launch` tools never change settings.
- Actions print a preview and exit 4 unless `--yes` is given. `--dry-run` previews and exits 0.
- `--read-only`, `WTZ_READ_ONLY=1`, or `wintonicz config set readOnly true` refuses every action.
- Every action is recorded in a local log (`%LOCALAPPDATA%\Wintonicz\logs`). Output bodies are not stored. Nothing is uploaded.

## Etiquette for agents

1. Call `tools list --json` once and cache it.
2. Show the human the preview (`--dry-run`) before running any `change` or `destructive` tool, and get approval for that specific action.
3. An empty `data` with a `warnings` entry means the result was limited, not that nothing exists.
4. Do not retry `needs_admin` in a loop; it requires a person to approve a UAC prompt.

## Preferences

`wintonicz config get|set|path|reset`. Keys: `readOnly`, `jsonDefault`, `actionLog`, `defaultArea`, `alwaysAdmin`. Environment overrides: `WTZ_READ_ONLY=1`, `WTZ_JSON=1`, `WTZ_ADMIN=0|1`. `alwaysAdmin` is on by default and only affects opening the interface; subcommands never elevate.

## Destructive tools

`clean temp`, `clean systemtemp`, `clean recyclebin`, and `process end` delete data or stop programs. Their `--dry-run` output lists what would be touched (file counts and sizes, the process name and path). Show that to the human and get approval for that exact preview before passing `--yes`; for `clean temp`, a later run can match different files than the preview did.

## Investigating and reporting

- `why <pid|name|:port> --json` returns `reasons[]`, each with `confidence: confirmed|inferred`. Treat `inferred` as a hypothesis and report it as one. `limits[]` lists what the current account could not read.
- `report support` and `report gpresult` write to a local file only. Redaction masks user name, computer name, IPv4/IPv6, and MAC by default (`--redact none` disables it and the preview says so). Masking is pattern-based; tell the human to review the file before sharing it.
- Reversible changes (`power set`, `explorer pref`, `repair wureset`, `network reset`) record their prior state in `%LOCALAPPDATA%\Wintonicz\undo`. Run `undo list` to see it and `undo [id]` to reverse one. `repair restorepoint` is rolled back with System Restore. Always `--dry-run` first and show the human the preview; `network reset` needs a restart afterward.

## MCP server (read-only)

`wintonicz mcp --read-only` speaks the Model Context Protocol on stdio. Add it to an MCP client, for example:

```json
{ "mcpServers": { "wintonicz": { "command": "C:\path\to\wintonicz.exe", "args": ["mcp", "--read-only"] } } }
```

It exposes only tools whose risk is `read`, named with underscores (`inspect_system`, `inspect_why`, `net_quick`, `power_list`, ...). Each takes an optional `args` array of plain values (a host, PID, or process name); values that start with `-` are refused. Every call runs Wintonicz with `--json --read-only`, so repairs, cleanup, and changes are not reachable. Output is the normal JSON envelope and is not redacted: it can contain the computer name, user name, and addresses, so connect it only to a client you trust with that.
