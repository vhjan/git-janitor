/*
MIT License

Copyright (c) 2026 Jan Van Herck (https://github.com/vhjan)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

/*
This file defines the asynchronous messages of the TUI and the commands that produce them. All background work
(loading branches, fetching, merging, deleting, checking for updates) is expressed as a tea.Cmd returning a
message, which keeps the Update loop free of blocking calls.
*/

package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vhjan/git-janitor/internal/branches"
	"github.com/vhjan/git-janitor/internal/buildinfo"
	"github.com/vhjan/git-janitor/internal/config"
	"github.com/vhjan/git-janitor/internal/git"
	"github.com/vhjan/git-janitor/internal/theme"
	"github.com/vhjan/git-janitor/internal/updater"
)

// loadOptions controls what loadBranchesCmd does before reading the branches.
type loadOptions struct {
	fetch    bool // Run `git fetch --prune` first so "[gone]" upstreams are accurate
	announce bool // Report the fetch outcome to the user even when nothing was pruned (explicit user action)
}

// branchesLoadedMsg carries the result of loadBranchesCmd.
type branchesLoadedMsg struct {
	items           []list.Item
	current         string // Checked-out branch
	defaultBranch   string
	defaultUpstream string // Remote-tracking branch the default branch follows, "" if none
	defaultBehind   int    // Commits the local default branch is behind that upstream
	note            string // Short status line about the fetch, may be empty
	err             error
}

// loadBranchesCmd optionally fetches/prunes, then reads and classifies the local branches.
func loadBranchesCmd(repo *git.Repo, cfg config.Config, opts loadOptions) tea.Cmd {
	return func() tea.Msg {
		var note string
		if opts.fetch {
			pruned, err := repo.Fetch(true)
			switch {
			case err != nil && opts.announce:
				// Typically offline; the list is still useful, just possibly out of date.
				note = "Fetch failed: " + git.Reason(err)
			case err != nil:
				// Silent on startup: working offline is normal.
			case len(pruned) > 0:
				note = fmt.Sprintf("Fetched. Pruned %d stale remote ref%s", len(pruned), plural(len(pruned)))
			case opts.announce:
				note = "Fetched. Nothing to prune"
			}
		}

		snap, err := branches.Load(repo, branches.Options{
			ProtectPatterns: cfg.Protect,
			StaleDays:       cfg.StaleDays,
		})
		if err != nil {
			return branchesLoadedMsg{err: err}
		}

		items := make([]list.Item, len(snap.Branches))
		for i, b := range snap.Branches {
			items[i] = item{Branch: b}
		}
		return branchesLoadedMsg{
			items: items, current: snap.Current, defaultBranch: snap.Default, note: note,
			defaultUpstream: snap.DefaultUpstream, defaultBehind: snap.DefaultBehind,
		}
	}
}

// deleteDoneMsg carries the outcome of deleteCmd.
type deleteDoneMsg struct {
	result branches.DeleteResult
}

// deleteCmd deletes branches in the background (remote deletion involves the network).
func deleteCmd(repo *git.Repo, targets []branches.Branch, opts branches.DeleteOptions) tea.Cmd {
	return func() tea.Msg {
		return deleteDoneMsg{result: branches.Delete(repo, targets, opts)}
	}
}

// preflightMsg carries the read-only inspection shown in the merge/rebase dialog.
type preflightMsg struct {
	id    int    // Dialog that asked; results for an earlier, cancelled dialog are dropped
	ref   string // The ref the operation uses (resolved for "update")
	check git.MergeCheck
	err   error
}

// preflightCmd inspects what merging/rebasing would do, without touching the working tree.
func preflightCmd(repo *git.Repo, id int, kind opKind, current, ref, defaultBranch string) tea.Cmd {
	return func() tea.Msg {
		if kind == opUpdate {
			ref = branches.UpdateTarget(repo, defaultBranch)
		}
		check, err := repo.CheckMerge(current, ref)
		return preflightMsg{id: id, ref: ref, check: check, err: err}
	}
}

// opDoneMsg carries the outcome of opCmd.
type opDoneMsg struct {
	text string // Success message
	err  error
}

// opCmd performs the chosen merge or rebase of the current branch. In a dry run it only reports the command.
func opCmd(repo *git.Repo, choice opChoice, current, ref string, dryRun bool) tea.Cmd {
	return func() tea.Msg {
		command := git.MergeCommand(ref, choice.mode)
		if choice.rebase {
			command = git.RebaseCommand(ref)
		}
		if dryRun {
			return opDoneMsg{text: "DRY RUN: would run `" + command + "`"}
		}

		if choice.rebase {
			if err := branches.Rebase(repo, ref); err != nil {
				return opDoneMsg{err: err}
			}
			return opDoneMsg{text: fmt.Sprintf("Rebased %s onto %s", current, ref)}
		}
		if err := branches.Merge(repo, ref, choice.mode); err != nil {
			return opDoneMsg{err: err}
		}
		return opDoneMsg{text: fmt.Sprintf("Merged %s into %s", ref, current)}
	}
}

// updateCheckedMsg carries the (possibly empty) "update available" notice.
type updateCheckedMsg struct {
	notice string
}

// checkUpdateCmd asks GitHub for the latest release in the background.
func checkUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		latest := updater.CheckForUpdate()
		if latest == "" {
			return updateCheckedMsg{}
		}

		notice := theme.Title.Render("  ✦ Update available!") +
			theme.Muted.Render(fmt.Sprintf(" %s → %s  ", updater.Normalize(buildinfo.Version), latest)) +
			theme.Muted.Render("Run: "+buildinfo.Name+" update")
		return updateCheckedMsg{notice: notice}
	}
}
