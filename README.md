# Git Janitor

A fast, interactive Terminal User Interface (TUI) for cleaning up local Git branches.

[![git-janitor-showcase.gif](assets/git-janitor-showcase.gif)](/assets/git-janitor-showcase.gif)

## The Problem

If you actively review PRs locally or juggle multiple feature branches, your local repository quickly accumulates stale,
merged, or abandoned branches. Cleaning this up manually is tedious. You either have to delete branches one by one in
the JetBrains IDE Git tool window, or manually look up and type out every single branch name you want to remove in the
terminal:

```bash
git branch -D feature/one feature/two bugfix/three stale/four
```

To avoid typing out a massive list, you *can* technically bulk-delete merged branches using native Git alongside bash
utilities, but it requires chaining together a terrifying command pipeline like this:

```bash
git branch --merged | egrep -v "(^\*|main|master|dev)" | xargs git branch -d
```

This native workaround has significant drawbacks:

* **Dangerous:** It runs completely blind. A slight typo in your regular expression could instantly wipe out the wrong
  branches.
* **Hard to remember:** Nobody wants to memorize `egrep -v` syntax and pipe operators just to clean up their workspace.
* **Not cross-platform:** This relies on Unix tools (`grep`, `xargs`) that do not work natively for developers using
  standard Windows Command Prompt or PowerShell.

## The Solution

**Git Janitor** streamlines this entire process by replacing manual typing and dangerous bash scripts with a fast,
keyboard-driven UI directly in your terminal.

Here is why it is a better approach:

* **Visual & Interactive:** You see exactly which branches are queued for deletion before any destructive commands are
  run.
* **Smart & Safe:** It dynamically queries your repository to find the true default branch and hard-locks it, along with
  your currently active branch, so they cannot be accidentally deleted.
* **Zero Memorization:** Just type `git-janitor`, press `m` to select all merged branches, and hit Enter.
* **Cross-Platform:** Compiled as a single Go binary, it works flawlessly across macOS, Linux, and Windows without
  relying on external shell utilities.

## Features

* **Keyboard-driven UI:** Navigate and toggle branches quickly without leaving the terminal.
* **Merged Branch Detection:** Automatically flags branches that have already been safely merged into your default
  branch.
* **Bulk Selection:** Select all unprotected branches or exclusively merged branches with a single keystroke.
* **Branch Protection:** Built-in safeguards prevent the accidental deletion of your currently active branch, as well as
  standard branches like `main`, `master`, and `dev`.

## Installation

**Option 1: Package Managers (Recommended)**

**macOS / Linux (Homebrew)**

```bash
brew install vhjan/tap/git-janitor
```

**Windows (Scoop)**

```bash
scoop bucket add vhjan https://github.com/vhjan/scoop-bucket.git
scoop install git-janitor
```

**Option 2: Download Pre-compiled Binaries**

You can download the pre-compiled `.zip` or `.tar.gz` binaries for Windows, macOS, and Linux directly from
the [GitHub Releases page](https://github.com/vhjan/git-janitor/releases). Extract the
executable and move it to a directory in your system's `$PATH`.

**Option 3: Install via Go**

If you have [Go](https://go.dev/) 1.24.2 or later installed:

```bash
go install github.com/vhjan/git-janitor@latest
```

*(Ensure your `~/go/bin` directory is in your system's `$PATH`)*

**Option 4: Build from source**

```bash
git clone https://github.com/vhjan/git-janitor.git
cd git-janitor
go build -o git-janitor .
# Move the binary to your PATH, e.g., sudo mv git-janitor /usr/local/bin/
```

> If you're running on Windows, use `go build -o git-janitor.exe .` instead.

## Usage

Navigate to any local Git repository in your terminal and run:

```bash
git-janitor
```

> If you're on Windows and you haven't added the git-janitor executable to your PATH use `.\git-janitor.exe`.

### Shorter ways to run it

Typing `git-janitor` gets old quickly, so the same program is available under shorter names:

| Command         | Notes                                                                                   |
|-----------------|-----------------------------------------------------------------------------------------|
| `gj`            | Short alias, installed next to `git-janitor` by Scoop, Homebrew and the release archives |
| `git janitor`   | Works automatically: Git runs any `git-<name>` executable on your PATH as `git <name>`  |

Neither overrides an existing Git command. If you installed with `go install` (which only produces `git-janitor`), you
can still get the short forms yourself:

```bash
# bash / zsh
alias gj=git-janitor
# or as a git alias, so that `git j` works
git config --global alias.j janitor
```

```powershell
# PowerShell profile
Set-Alias gj git-janitor
```

### Keybindings

| Key            | Action                                                       |
|----------------|--------------------------------------------------------------|
| `↑` / `k`      | Move cursor up                                               |
| `↓` / `j`      | Move cursor down                                             |
| `Space`        | Toggle selection for the current branch                      |
| `a`            | Select **all** unprotected branches                          |
| `m`            | Select only **merged** branches                              |
| `g`            | Select only **gone** branches (upstream deleted on remote)   |
| `s`            | Select only **stale** branches (see `--stale-days`)          |
| `c`            | **Clear** all selections                                     |
| `o`            | Cycle the sort order                                         |
| `Enter`        | Proceed to deletion confirmation                             |
| `q` / `Ctrl+C` | Quit                                                         |

## Configuration

The UI is built using `bubbletea` and `lipgloss`. Colors, margins and list symbols are centralized in
`internal/theme/theme.go`, and the built-in protected branch names live in `internal/branches/branches.go`. You can
modify these and rebuild the binary to adjust the tool to your preferences. Run `git-janitor --help` for the available
flags.

## Contributing

Contributions, issues, and feature requests are welcome. Feel free to check
the [issues page](https://github.com/vhjan/git-janitor/issues) if you want to contribute.

See [`ARCHITECTURE.md`](ARCHITECTURE.md) for how the code is organised and where new features belong.

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/NewFeature`)
3. Commit your Changes (`git commit -m 'Add NewFeature'`)
4. Push to the Branch (`git push origin feature/NewFeature`)
5. Open a Pull Request

## Prior Art

This project shares a name and core mission with an earlier [Git Janitor](https://github.com/grvm/Git-Janitor) built
by Gaurav Manchanda in 2015. While this Go-based TUI is a completely independent tool designed from scratch for modern
workflows, I want to acknowledge their original work!

## License

Distributed under the MIT License. See [`LICENSE`](https://github.com/vhjan/git-janitor/blob/main/LICENSE) for more
information.
