## Install

```bash
curl -fsSL https://gwuah.github.io/mono/install.sh | bash
```

## Setup

conductor.json:

```json
{
  "scripts": {
    "setup": "mono init",
    "run": "mono run",
    "archive": "mono destroy"
  }
}
```

Mono automatically reads `CONDUCTOR_WORKSPACE_PATH` when no path is provided. You can also pass an explicit path: `mono init /path/to/workspace`.

## Architecture

```
                        CONDUCTOR
                 (worktrees + Claude Code)
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
   ┌─────────┐         ┌─────────┐         ┌─────────┐
   │ ws: foo │         │ ws: bar │         │ ws: baz │
   └────┬────┘         └────┬────┘         └────┬────┘
        │                   │                   │
        └───────────────────┼───────────────────┘
                            ▼
                          MONO
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
   ┌─────────┐         ┌─────────┐         ┌─────────┐
   │ mono-foo│         │ mono-bar│         │ mono-baz│
   │ :19100  │         │ :19200  │         │ :19300  │
   └─────────┘         └─────────┘         └─────────┘
   docker+tmux         docker+tmux         docker+tmux
```

Each workspace gets isolated ports, containers, and tmux session.
