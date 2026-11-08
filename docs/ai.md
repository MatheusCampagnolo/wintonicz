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
