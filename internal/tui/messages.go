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
(loading branches, checking for updates, and later: fetching, merging, ...) is expressed as a tea.Cmd returning a
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

// branchesLoadedMsg carries the result of loadBranchesCmd.
type branchesLoadedMsg struct {
	items []list.Item
	err   error
}

// loadBranchesCmd prunes stale remote-tracking refs, then reads and classifies the local branches.
func loadBranchesCmd(repo *git.Repo, cfg config.Config) tea.Cmd {
	return func() tea.Msg {
		// Prune so branches without an upstream show up as "gone". A failure (e.g. offline) is not fatal.
		_ = repo.Fetch(true)

		loaded, err := branches.Load(repo, branches.Options{
			ProtectPatterns: cfg.Protect,
			StaleDays:       cfg.StaleDays,
		})
		if err != nil {
			return branchesLoadedMsg{err: err}
		}

		items := make([]list.Item, len(loaded))
		for i, b := range loaded {
			items[i] = item{Branch: b}
		}
		return branchesLoadedMsg{items: items}
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
