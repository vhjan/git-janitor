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
This file implements the merge / rebase / update dialog. Opening it runs a read-only pre-flight (commit counts,
fast-forward possibility and predicted conflicts); choosing an option then performs the operation on the current
branch. Conflicts abort the operation automatically, so the repository is never left half-merged.
*/

package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vhjan/git-janitor/internal/branches"
	"github.com/vhjan/git-janitor/internal/git"
	"github.com/vhjan/git-janitor/internal/theme"
)

// opKind is the operation the dialog was opened for.
type opKind int

const (
	opMerge  opKind = iota // Merge the highlighted branch into the current branch
	opRebase               // Rebase the current branch onto the highlighted branch
	opUpdate               // Update the current branch from the default branch (rebase or merge)
)

// maxListedConflicts caps how many conflicting files are spelled out.
const maxListedConflicts = 5

// opChoice is one selectable way to perform the operation.
type opChoice struct {
	rebase bool          // Rebase instead of merge
	mode   git.MergeMode // Merge mode when !rebase
}

// opDialog is the state of the merge/rebase dialog.
type opDialog struct {
	id       int // Unique per opened dialog, echoed by its pre-flight result
	kind     opKind
	pushed   bool   // The current branch already exists on a remote, so rebasing it rewrites published history
	current  string // Branch the operation applies to
	ref      string // The other side: merged in / rebased onto / updated from
	choices  []opChoice
	cursor   int
	loading  bool // Pre-flight still running
	check    git.MergeCheck
	checkErr error
}

func newOpDialog(kind opKind, current, ref string) *opDialog {
	d := &opDialog{kind: kind, current: current, ref: ref, loading: true}
	switch kind {
	case opMerge:
		d.choices = []opChoice{
			{mode: git.MergeDefault}, {mode: git.MergeFFOnly}, {mode: git.MergeNoFF}, {mode: git.MergeSquash},
		}
	case opRebase:
		d.choices = []opChoice{{rebase: true}}
	case opUpdate:
		d.choices = []opChoice{{rebase: true}, {mode: git.MergeDefault}}
	}
	return d
}

func (d *opDialog) title() string {
	switch d.kind {
	case opMerge:
		return fmt.Sprintf("Merge %s into %s", d.ref, d.current)
	case opRebase:
		return fmt.Sprintf("Rebase %s onto %s", d.current, d.ref)
	default:
		return fmt.Sprintf("Update %s from %s", d.current, d.ref)
	}
}

func (d *opDialog) label(c opChoice) string {
	if c.rebase {
		return fmt.Sprintf("Rebase onto %s", d.ref)
	}
	if d.kind == opUpdate {
		return fmt.Sprintf("Merge %s (fast-forward when possible)", d.ref)
	}
	return c.mode.Label()
}

// beginOp opens the dialog for the highlighted branch and starts the pre-flight.
func (m Model) beginOp(kind opKind) (Model, tea.Cmd) {
	if m.current == "" {
		m.showMessage("HEAD is detached.\nCheck out a branch first; merging and rebasing act on the current branch.", true)
		return m, nil
	}

	ref := m.defaultBranch
	if kind != opUpdate {
		sel, ok := m.list.SelectedItem().(item)
		if !ok {
			return m, nil
		}
		if sel.Name == m.current {
			m.showMessage(fmt.Sprintf("%s is the current branch.\nHighlight a different branch first.", sel.Name), true)
			return m, nil
		}
		ref = sel.Name
	}

	m.opSeq++
	m.op = newOpDialog(kind, m.current, ref)
	m.op.id = m.opSeq
	m.op.pushed = m.currentIsPushed()
	m.state = stateOp
	return m, preflightCmd(m.repo, m.op.id, kind, m.current, ref, m.defaultBranch)
}

func (m Model) updateOp(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok || m.op == nil {
		return m, nil
	}
	d := m.op

	switch {
	case key.Matches(k, m.keys.OpCancel):
		m.op = nil
		m.state = stateList
		return m, nil

	case key.Matches(k, m.keys.OpUp):
		d.cursor = (d.cursor + len(d.choices) - 1) % len(d.choices)
		return m, nil

	case key.Matches(k, m.keys.OpDown):
		d.cursor = (d.cursor + 1) % len(d.choices)
		return m, nil

	case key.Matches(k, m.keys.OpRun):
		if d.loading {
			return m, nil
		}
		choice := d.choices[d.cursor]
		text := "Merging..."
		if choice.rebase {
			text = "Rebasing..."
		}
		busy := m.startBusy(text) // separate statement: it mutates m before m is returned
		return m, tea.Batch(busy, opCmd(m.repo, choice, d.current, d.ref, m.cfg.DryRun))
	}

	// Number keys pick an option directly.
	if s := k.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		if idx := int(s[0] - '1'); idx < len(d.choices) {
			d.cursor = idx
		}
	}
	return m, nil
}

// summary describes the pre-flight result, one line per fact.
func (d *opDialog) summary() []string {
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorWarning)).Bold(true)
	ok := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorSuccess))
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorTextMuted))

	switch {
	case d.loading:
		return []string{muted.Render("Checking...")}
	case d.checkErr != nil:
		return []string{warn.Render("Could not inspect the branches: " + git.Reason(d.checkErr))}
	case d.check.UpToDate:
		return []string{ok.Render(fmt.Sprintf("✓ %s already contains everything from %s: nothing to do", d.current, d.ref))}
	}

	var lines []string
	c := d.check
	lines = append(lines, fmt.Sprintf("%d new commit%s on %s, %d commit%s only on %s",
		c.Incoming, plural(c.Incoming), d.ref, c.Local, plural(c.Local), d.current))
	if c.FastForward {
		lines = append(lines, ok.Render("✓ Can fast-forward"))
	}

	switch {
	case !c.Checked:
		lines = append(lines, muted.Render("Conflicts can't be predicted (needs git 2.38+)"))
	case len(c.Conflicts) > 0:
		shown := c.Conflicts
		more := ""
		if len(shown) > maxListedConflicts {
			more = fmt.Sprintf(" and %d more", len(shown)-maxListedConflicts)
			shown = shown[:maxListedConflicts]
		}
		lines = append(lines, warn.Render("⚠ Conflicts predicted in: "+strings.Join(shown, ", ")+more))
		lines = append(lines, muted.Render("  The operation will be aborted if they occur."))
	default:
		lines = append(lines, ok.Render("✓ No conflicts predicted"))
	}
	return lines
}

func (m Model) viewOp() string {
	if m.op == nil {
		return ""
	}
	d := m.op
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorTitle)).Bold(true)
	selected := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorPrimary)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorTextMuted))

	lines := []string{title.Render(d.title()), ""}
	lines = append(lines, d.summary()...)
	lines = append(lines, "")
	for idx, c := range d.choices {
		prefix := "  "
		text := fmt.Sprintf("%d. %s", idx+1, d.label(c))
		if idx == d.cursor {
			prefix, text = "> ", selected.Render(text)
		}
		lines = append(lines, prefix+text)
	}
	if d.pushed && d.choices[d.cursor].rebase {
		warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorWarning)).Bold(true)
		lines = append(lines, "", warnStyle.Render("⚠ "+d.current+" is already pushed: rebasing rewrites its history,"),
			warnStyle.Render("  so the next push needs --force-with-lease."))
	}
	hints := "↑/↓ choose · enter run · esc cancel"
	if m.cfg.DryRun {
		hints = "DRY RUN · " + hints
	}
	lines = append(lines, "", muted.Render(hints))

	box := confirmStyle.Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// describeOpError turns an error from a merge/rebase into text for the message view.
func describeOpError(err error) string {
	var ce *git.ConflictError
	switch {
	case errors.As(err, &ce):
		files := strings.Join(ce.Files, "\n  ")
		return fmt.Sprintf("The %s hit conflicts and was aborted; nothing was changed.\n\nConflicting files:\n  %s\n\n"+
			"Resolve them outside git-janitor (e.g. run the %s yourself).", ce.Op, files, ce.Op)
	case errors.Is(err, branches.ErrUpToDate):
		return "Already up to date: there is nothing to do."
	default:
		return "The operation failed:\n\n" + git.Reason(err)
	}
}

// currentIsPushed reports whether the checked-out branch tracks a branch on a remote.
func (m Model) currentIsPushed() bool {
	for _, li := range m.list.Items() {
		if i, ok := li.(item); ok && i.Name == m.current {
			return i.Remote != ""
		}
	}
	return false
}
