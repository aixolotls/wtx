# wtx
wtx is a CLI that automates Git worktrees for AI agents. 

Works in any terminal (ghostty, iTerm, etc.) and with any AI agent (Claude, Codex, ...).

## why wtx
In large repos, creating and managing worktrees becomes slow and manual. Treating them as disposable per branch does not scale.

wtx keeps a reusable pool of worktrees and assigns them to branches automatically, so parallel AI agents can use them without repeated bootstrap.

![wtx screenshot](docs/assets/wtx-screenshot-v2.png)

## Quickstart

```sh
wtx checkout -b feat/first
# open another terminal
wtx checkout -b feat/second
```

interactively:
```sh
wtx
```

## Installation

```sh
curl -fsSL https://raw.githubusercontent.com/aixolotls/wtx/main/install.sh | bash
```

Alternative (requires Go):
```sh
go install github.com/aixolotls/wtx@latest
```

## Other Features
- Open your ide easily on a worktree's subfolder, to avoid indexing tax in large repos (requires tmux)
- Get an interactive shell quickly in the worktree (requires tmux)
- Terminal tab naming: keeps branch context visible while juggling many monorepo sessions (requires tmux)
- GitHub integration: surfaces merge, review, and CI status where you are already working

## License
[MIT](LICENSE)
