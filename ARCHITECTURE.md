# Architecture

Git Janitor is a single Go binary. The repository root only contains a tiny `main.go` (so that
`go install github.com/vhjan/git-janitor@latest` keeps working); everything else lives in `internal/`.

```
main.go                     entry point, calls cli.Execute
internal/
  buildinfo/                Version/Commit/Date (set via ldflags) and the binary names
  config/                   Config struct shared by everything (flags today, config file later)
  theme/                    colours, layout constants, list symbols, shared lipgloss styles
  git/                      the ONLY package that spawns git
  branches/                 UI-independent domain logic: classify, protect, delete
  updater/                  GitHub release check + package-manager detection
  tui/                      Bubble Tea front end
  cli/                      flag parsing, command registry, help/version/update
```

## Dependency direction

```
main -> cli -> tui -> branches -> git
              |  \-> updater      ^
              \----------------> config, theme, buildinfo (leaf packages, import nothing of ours)
```

Rules of thumb:

- **Only `internal/git` runs git.** Everything goes through `Repo.run`, which gives one place for error handling and the
  `Observer` hook (intended for a future "operation log" panel).
- **`branches` has no UI imports.** The TUI and any future non-interactive command (`git-janitor clean --merged -y`) share
  the same logic.
- **`tui` never shells out directly.** It calls `branches`/`git` from `tea.Cmd`s and receives the outcome as a message.

## Where does a new feature go?

| I want to add...                                    | Touch                                                                                          |
|-----------------------------------------------------|------------------------------------------------------------------------------------------------|
| A new git operation (merge, rebase, push, create)   | A method on `git.Repo` (`internal/git`), plus a function in `branches` if it has logic around it |
| A new branch status tag (squash-merged, unpushed)   | A field on `branches.Branch`, set in `branches.Load`; show it in `tui/item.go`                  |
| A new key in the list                               | A binding in `tui/keys.go`, a case in `handleListKey` (`tui/view_list.go`). Footer and `--help` update automatically |
| A new screen (prompt, details pane, command palette)| A `state` constant and `view_<name>.go` in `internal/tui`; add cases to `Update`/`View` in `model.go` |
| A new CLI subcommand (`new`, `co`, `clean`)         | A `Command` in `internal/cli`, registered in `init()` in `cli.go`. It shows up in `--help` automatically |
| A new global flag                                   | `internal/cli/flags.go` (+ a field in `config.Config` if the TUI needs it). Help is generated   |
| A new setting that should be persistent             | `config.Config`; load it from a file in `config` and merge with flags in `cli`                 |
| Colours / symbols / spacing                         | `internal/theme`                                                                               |

## The TUI in a nutshell

`tui.Model` holds shared state and a `state` enum (`stateLoading`, `stateList`, `stateConfirm`). `Update` first handles
messages that don't depend on the view (window size, data loaded) and then forwards everything else to
`updateLoading` / `updateList` / `updateConfirm`. `View` does the same for rendering. Slow work is always a `tea.Cmd`
in `tui/messages.go` that returns a message.

## Binaries and aliases

GoReleaser builds the same `main` twice: `git-janitor` and the short alias `gj`. Both ship in the archives, the Scoop
manifest and the deb/rpm/apk packages. The Homebrew formula (built from source in `release.yml`) installs `git-janitor`
and symlinks `gj` to it. Because the executable is named `git-janitor`, Git also exposes it as `git janitor`.
The program detects which name it was started under (`cli.programName`) so its messages say `gj` when you typed `gj`.

## Testing

`go test ./...` — the `git` and `tui` packages create throw-away repositories in temp directories (skipped if `git`
isn't installed), so no mocking is needed for end-to-end flows.
