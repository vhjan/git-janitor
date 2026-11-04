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
	Proceed        key.Binding

	// Confirm dialog
	Yes key.Binding
	No  key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Quit:           key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("q / Ctrl+C", "Quit the application")),
		Toggle:         key.NewBinding(key.WithKeys(" "), key.WithHelp("Space", "Toggle selection for the current branch")),
		SelectAll:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "Select ALL unprotected branches")),
		SelectMerged:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "Select MERGED branches (those already merged into the default branch)")),
		SelectGone:     key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "Select GONE branches (those whose upstream remote branch was deleted)")),
		SelectStale:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "Select STALE branches (no commits within the stale-days threshold)")),
		ClearSelection: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "CLEAR all current selections")),
		CycleSort:      key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "Cycle through sort orders (Alphabetical, Latest Commits, Oldest Commits)")),
		Proceed:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Proceed to deletion confirmation screen")),

		Yes: key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "Confirm")),
		No:  key.NewBinding(key.WithKeys("n", "N", "q", "esc", "enter", "ctrl+c"), key.WithHelp("n", "Cancel")),
	}
}

// bindings returns every binding, used to validate that no two actions share a key.
func (k keyMap) bindings() []key.Binding {
	return []key.Binding{
		k.Quit, k.Toggle, k.SelectAll, k.SelectMerged, k.SelectGone, k.SelectStale,
		k.ClearSelection, k.CycleSort, k.Proceed,
	}
}

// footer renders the one-line hint bar shown below the branch list.
func (k keyMap) footer(sortLabel string) string {
	hints := []string{
		hint(k.SelectAll, "all"),
		hint(k.SelectMerged, "merged"),
		hint(k.SelectGone, "gone"),
		hint(k.SelectStale, "stale"),
		hint(k.ClearSelection, "clear"),
		hint(k.CycleSort, "sort ("+sortLabel+")"),
	}
	return "  " + strings.Join(hints, " • ")
}

func hint(b key.Binding, label string) string {
	return b.Help().Key + ": " + label
}

// HelpRows returns the interactive keybindings as {key, description} rows for the `--help` output.
func HelpRows() [][2]string {
	k := defaultKeyMap()
	rows := [][2]string{
		{"↑ / k", "Move cursor up"},
		{"↓ / j", "Move cursor down"},
	}
	for _, b := range []key.Binding{
		k.Toggle, k.SelectAll, k.SelectMerged, k.SelectGone, k.SelectStale,
		k.ClearSelection, k.CycleSort, k.Proceed, k.Quit,
	} {
		rows = append(rows, [2]string{b.Help().Key, b.Help().Desc})
	}
	return rows
}
