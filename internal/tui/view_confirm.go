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
This file implements the confirmation dialog shown before the selected branches are deleted. It summarises what
is about to happen, warns about commits that would be lost, and offers to delete the remote branches as well.
*/

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vhjan/git-janitor/internal/branches"
	"github.com/vhjan/git-janitor/internal/theme"
)

// maxListedRisks caps how many at-risk branches are spelled out in the dialog.
const maxListedRisks = 6

// confirmStyle defines the appearance of the confirmation dialog.
var confirmStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(theme.ColorPrimary)).
	Padding(theme.DialogPaddingVertical, theme.ConfirmPaddingHorizontal)

// deletePlan describes what confirming the dialog would delete.
type deletePlan struct {
	items    []item
	merged   int    // merged according to git
	squashed int    // merged by squash/rebase
	atRisk   []item // would lose commits that exist nowhere else
	remote   int    // have an upstream branch that could be deleted too
}

func (m Model) plan() deletePlan {
	p := deletePlan{items: m.selectedItems()}
	for _, i := range p.items {
		switch {
		case i.Merged:
			p.merged++
		case i.Squashed:
			p.squashed++
		}
		if i.AtRisk() {
			p.atRisk = append(p.atRisk, i)
		}
		if i.RemoteDeletable() {
			p.remote++
		}
	}
	return p
}

func (p deletePlan) branches() []branches.Branch {
	out := make([]branches.Branch, len(p.items))
	for i, it := range p.items {
		out[i] = it.Branch
	}
	return out
}

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	plan := m.plan()

	switch {
	case key.Matches(k, m.keys.ToggleRemote):
		if plan.remote > 0 {
			m.deleteRemote = !m.deleteRemote
		}
		return m, nil

	case key.Matches(k, m.keys.No):
		m.state = stateList
		return m, nil

	case key.Matches(k, m.keys.Yes):
		// Deleting commits that exist nowhere else needs a deliberate capital Y.
		if len(plan.atRisk) > 0 && k.String() != "Y" {
			return m, nil
		}
		opts := branches.DeleteOptions{DryRun: m.cfg.DryRun, DeleteRemote: m.deleteRemote && plan.remote > 0}
		busy := m.startBusy("Deleting branches...") // separate statement: it mutates m before m is returned
		return m, tea.Batch(busy, deleteCmd(m.repo, plan.branches(), opts))
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	plan := m.plan()
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorWarning)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorTextMuted))

	var lines []string
	if m.cfg.DryRun {
		lines = append(lines, fmt.Sprintf("DRY RUN: Would delete %s.", countBranches(len(plan.items))))
	} else {
		lines = append(lines, fmt.Sprintf("Delete %s?", countBranches(len(plan.items))))
	}

	// Breakdown of what is being deleted
	var breakdown []string
	if plan.merged > 0 {
		breakdown = append(breakdown, fmt.Sprintf("%d merged", plan.merged))
	}
	if plan.squashed > 0 {
		breakdown = append(breakdown, fmt.Sprintf("%d squash/rebase-merged", plan.squashed))
	}
	if other := len(plan.items) - plan.merged - plan.squashed; other > 0 {
		breakdown = append(breakdown, fmt.Sprintf("%d not merged", other))
	}
	lines = append(lines, "", muted.Render(strings.Join(breakdown, " · ")))

	// Data-loss warning
	if len(plan.atRisk) > 0 {
		lines = append(lines, "", warn.Render("⚠ These have commits that exist nowhere else:"))
		for idx, i := range plan.atRisk {
			if idx == maxListedRisks {
				lines = append(lines, fmt.Sprintf("  … and %d more", len(plan.atRisk)-maxListedRisks))
				break
			}
			lines = append(lines, fmt.Sprintf("  %s (%d unpushed commit%s)", i.Name, i.Unpushed, plural(i.Unpushed)))
		}
	}

	// Remote deletion toggle
	if plan.remote > 0 {
		box := "[ ]"
		if m.deleteRemote {
			box = "[x]"
		}
		noun := "branches"
		if plan.remote == 1 {
			noun = "branch"
		}
		lines = append(lines, "", fmt.Sprintf("%s Also delete %d remote %s  %s",
			box, plan.remote, noun, muted.Render("(r to toggle)")))
	}

	prompt := "(y/N)"
	if len(plan.atRisk) > 0 {
		prompt = warn.Render("Type capital Y to delete anyway · n to cancel")
	}
	lines = append(lines, "", prompt)

	box := confirmStyle.Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// countBranches renders "1 branch" / "3 branches".
func countBranches(n int) string {
	if n == 1 {
		return "1 branch"
	}
	return fmt.Sprintf("%d branches", n)
}
