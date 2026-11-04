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
This file defines every key binding of the TUI in one place. The same definitions drive the key handling, the
footer hints and the keybinding table of `--help`, so a new action only has to be declared here once.
*/

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

// keyMap groups all bindings by the view that uses them.
type keyMap struct {
	// List view
	Quit           key.Binding
	Toggle         key.Binding
	SelectAll      key.Binding
	SelectMerged   key.Binding
	SelectGone     key.Binding
	SelectStale    key.Binding
	ClearSelection key.Binding
	CycleSort      key.Binding
	Fetch          key.Binding
	Merge          key.Binding
	Rebase         key.Binding
	Update         key.Binding
	Proceed        key.Binding

	// Delete confirmation dialog
	Yes          key.Binding
	No           key.Binding
	ToggleRemote key.Binding

	// Merge / rebase dialog
	OpUp     key.Binding
	OpDown   key.Binding
	OpRun    key.Binding
	OpCancel key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Quit:           key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("q / Ctrl+C", "Quit the application")),
		Toggle:         key.NewBinding(key.WithKeys(" "), key.WithHelp("Space", "Toggle selection for the current branch")),
		SelectAll:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "Select ALL unprotected branches")),
		SelectMerged:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "Select MERGED branches (merged into the default branch, including squash/rebase merges)")),
		SelectGone:     key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "Select GONE branches (those whose upstream remote branch was deleted)")),
		SelectStale:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "Select STALE branches (no commits within the stale-days threshold)")),
		ClearSelection: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "CLEAR all current selections")),
		CycleSort:      key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "Cycle through sort orders (Alphabetical, Latest Commits, Oldest Commits)")),
		Fetch:          key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "FETCH from the remotes and prune remote-tracking branches that no longer exist")),
		Merge:          key.NewBinding(key.WithKeys("M"), key.WithHelp("M", "MERGE the highlighted branch into the current branch (conflicts are predicted first)")),
		Rebase:         key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "REBASE the current branch onto the highlighted branch")),
		Update:         key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "UPDATE the current branch from the default branch (rebase or merge)")),
		Proceed:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Proceed to deletion confirmation screen")),

		Yes:          key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "Confirm")),
		No:           key.NewBinding(key.WithKeys("n", "N", "q", "esc", "enter", "ctrl+c"), key.WithHelp("n", "Cancel")),
		ToggleRemote: key.NewBinding(key.WithKeys("r", "R"), key.WithHelp("r", "Also delete the remote branches")),

		OpUp:     key.NewBinding(key.WithKeys("up", "k", "shift+tab"), key.WithHelp("↑/k", "Previous option")),
		OpDown:   key.NewBinding(key.WithKeys("down", "j", "tab"), key.WithHelp("↓/j", "Next option")),
		OpRun:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Run")),
		OpCancel: key.NewBinding(key.WithKeys("esc", "q", "n", "ctrl+c"), key.WithHelp("Esc", "Cancel")),
	}
}

// listBindings returns the bindings of the main list view in the order they are documented. It is also what the
// duplicate-key test checks: two actions in the same view must never share a key.
func (k keyMap) listBindings() []key.Binding {
	return []key.Binding{
		k.Toggle, k.SelectAll, k.SelectMerged, k.SelectGone, k.SelectStale, k.ClearSelection, k.CycleSort,
		k.Fetch, k.Merge, k.Rebase, k.Update, k.Proceed, k.Quit,
	}
}

// footer renders the hint bar shown below the branch list (two lines: selection, then operations).
func (k keyMap) footer(sortLabel string) string {
	selection := []string{
		hint(k.SelectAll, "all"),
		hint(k.SelectMerged, "merged"),
		hint(k.SelectGone, "gone"),
		hint(k.SelectStale, "stale"),
		hint(k.ClearSelection, "clear"),
		hint(k.CycleSort, "sort ("+sortLabel+")"),
	}
	operations := []string{
		hint(k.Fetch, "fetch"),
		hint(k.Merge, "merge"),
		hint(k.Rebase, "rebase"),
		hint(k.Update, "update"),
		hint(k.Proceed, "delete selected"),
	}
	return "  " + strings.Join(selection, " • ") + "\n  " + strings.Join(operations, " • ")
}

// footerLines is the number of lines footer() renders.
const footerLines = 2

func hint(b key.Binding, label string) string {
	return b.Help().Key + ": " + label
}

// HelpRows returns the interactive keybindings as {key, description} rows for the `--help` output.
func HelpRows() [][2]string {
	rows := [][2]string{
		{"↑ / k", "Move cursor up"},
		{"↓ / j", "Move cursor down"},
	}
	for _, b := range defaultKeyMap().listBindings() {
		rows = append(rows, [2]string{b.Help().Key, b.Help().Desc})
	}
	return rows
}
