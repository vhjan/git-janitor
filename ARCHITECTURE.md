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
  branches/                 UI-independent domain logic: classify, protect, delete, merge/rebase policy
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
| A new git operation (push, checkout, create, ...)   | A method on `git.Repo` (`internal/git`), plus a function in `branches` if it has logic around it |
| A new branch status tag (e.g. PR state)             | A field on `branches.Branch`, set in `branches.Load`; show it in `tui/item.go`                  |
| A new key                                           | A binding in `tui/keys.go` (global or in a tab group + `tabBindings`), a case in `handleListKey` / `handleCleanupKey` / `handleBranchesKey`. Footer, help overlay and `--help` update automatically |
| A new action on a branch                            | A `menuItem` in `openMenu` (`tui/view_menu.go`); it gets a menu entry, a letter shortcut and disabled-with-reason handling for free |
| A new tab (remotes, stashes, tags)                  | A `viewTab` constant + name in `model.go`, its key group in `keys.go`, a case in `handleListKey` and `footer` |
| A new screen (prompt, details pane, command palette)| A `state` constant and `view_<name>.go` in `internal/tui`; add cases to `Update`/`View` in `model.go` |
| A new CLI subcommand (`new`, `co`, `clean`)         | A `Command` in `internal/cli`, registered in `init()` in `cli.go`. It shows up in `--help` automatically |
| A new global flag                                   | `internal/cli/flags.go` (+ a field in `config.Config` if the TUI needs it). Help is generated   |
| A new setting that should be persistent             | `config.Config`; load it from a file in `config` and merge with flags in `cli`                 |
| Colours / symbols / spacing                         | `internal/theme`                                                                               |

## The TUI in a nutshell

`tui.Model` holds shared state and a `state` enum: `stateLoading` (spinner, text in `Model.busy`), `stateList`,
`stateConfirm` (delete dialog), `stateOp` (merge/rebase/update dialog), `stateMenu` (action menu), `stateHelp` (help
overlay) and `stateMessage` (notice or error). `Update` first handles messages that don't depend on the view (window
size, branches loaded, delete/operation finished) and then forwards everything else to `update<State>`. `View` does the
same for rendering. Slow work is always a `tea.Cmd` in `tui/messages.go` that returns a message; call
`Model.startBusy` (in its own statement, it mutates the model) to show the spinner while it runs.

`stateList` itself has **tabs** (`viewTab`): *Cleanup* (checkboxes, selection keys, delete) and *Branches* (plain rows,
`Enter` opens the action menu). Both show the same items; `item.plain` switches the rendering and `Model.setTab`
re-renders them. Keys are grouped in `keys.go` as global bindings plus one group per tab, `handleListKey` dispatches
accordingly, and the footer, the help overlay (`?`) and `--help` are all generated from the same bindings
(`HelpSections`).

## Safety model

These rules are what make the destructive features safe to extend; keep them when adding more.

- **Nothing deletes what git can't get back, silently.** `branches.Load` computes `Unpushed` (commits reachable from the
  branch but from neither the default branch nor any remote) and `Squashed` (changes already in the default branch via
  squash/rebase merge, detected with `git cherry` patch ids). `Branch.AtRisk()` is true for unmerged branches with
  unpushed commits; the delete dialog then lists them and only accepts a capital `Y`. Every deleted branch's SHA is
  printed afterwards so it can be restored.
- **Failed merges/rebases roll themselves back.** `git.Repo.Merge/Rebase` abort when they stop on conflicts and return a
  `*git.ConflictError`; the TUI shows it in the message view. `git.Repo.CheckMerge` predicts conflicts read-only with
  `git merge-tree --write-tree` (git 2.38+; on older git the dialog says conflicts can't be predicted and relies on the
  automatic abort).
- **git never blocks the UI.** Every invocation runs with `GIT_TERMINAL_PROMPT=0` and `GIT_EDITOR=true`, so a missing
  credential or an editor request fails fast instead of hanging behind the alternate screen.
- **Merge/rebase act on the current branch only.** They never check out another branch behind the user's back, require a
  clean tracked working tree, and honour `--dry-run` by only reporting the command.
## Binaries and aliases

GoReleaser builds the same `main` twice: `git-janitor` and the short alias `gj`. Both ship in the archives, the Scoop
manifest and the deb/rpm/apk packages. The Homebrew formula (built from source in `release.yml`) installs `git-janitor`
and symlinks `gj` to it. Because the executable is named `git-janitor`, Git also exposes it as `git janitor`.
The program detects which name it was started under (`cli.programName`) so its messages say `gj` when you typed `gj`.

## Testing

`go test ./...` — the `git` and `tui` packages create throw-away repositories in temp directories (skipped if `git`
isn't installed), so no mocking is needed for end-to-end flows.
