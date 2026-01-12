## Install

```
go install github.com/gwuah/mono/cmd/mono@latest
```

## Setup

conductor.json:

```json
{
  "scripts": {
    "setup": "mono init \"$CONDUCTOR_ROOT_PATH\"",
    "run": "mono run \"$CONDUCTOR_ROOT_PATH\"",
    "archive": "mono destroy \"$CONDUCTOR_ROOT_PATH\""
  }
}
```

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
