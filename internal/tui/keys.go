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
per-tab footer hints, the help overlay and the keybinding section of `--help`, so a new action only has to be
declared here once.
*/

package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

// keyMap groups all bindings by where they apply. Global keys work in every tab; the tab groups only in their tab.
type keyMap struct {
	// Global (list views)
	Quit      key.Binding
	Help      key.Binding
	NextTab   key.Binding
	PrevTab   key.Binding
	TabClean  key.Binding
	TabBranch key.Binding
	Fetch     key.Binding
	CycleSort key.Binding

	// Cleanup tab
	Toggle         key.Binding
	SelectAll      key.Binding
	SelectMerged   key.Binding
	SelectGone     key.Binding
	SelectStale    key.Binding
	ClearSelection key.Binding
	Proceed        key.Binding

	// Branches tab
	Actions key.Binding
	Merge   key.Binding
	Rebase  key.Binding
	Update  key.Binding

	// Delete confirmation dialog
	Yes          key.Binding
	No           key.Binding
	ToggleRemote key.Binding

	// Menus and the merge / rebase dialog
	OpUp     key.Binding
	OpDown   key.Binding
	OpRun    key.Binding
	OpCancel key.Binding
}

func defaultKeyMap() keyMap {
	return keyMap{
		Quit:      key.NewBinding(key.WithKeys("ctrl+c", "q"), key.WithHelp("q / Ctrl+C", "Quit the application")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "Show this help")),
		NextTab:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("Tab", "Next view")),
		PrevTab:   key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("Shift+Tab", "Previous view")),
		TabClean:  key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "Jump to the Cleanup view")),
		TabBranch: key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "Jump to the Branches view")),
		Fetch:     key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "FETCH from the remotes and prune remote-tracking branches that no longer exist")),
		CycleSort: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "Cycle through sort orders (Alphabetical, Latest Commits, Oldest Commits)")),

		Toggle:         key.NewBinding(key.WithKeys(" "), key.WithHelp("Space", "Toggle selection for the current branch")),
		SelectAll:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "Select ALL unprotected branches")),
		SelectMerged:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "Select MERGED branches (merged into the default branch, including squash/rebase merges)")),
		SelectGone:     key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "Select GONE branches (those whose upstream remote branch was deleted)")),
		SelectStale:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "Select STALE branches (no commits within the stale-days threshold)")),
		ClearSelection: key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "CLEAR all current selections")),
		Proceed:        key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Delete the selected branches (asks for confirmation first)")),

		Actions: key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Open the action menu for the highlighted branch")),
		Merge:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "MERGE the highlighted branch into the current branch (conflicts are predicted first)")),
		Rebase:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "REBASE the current branch onto the highlighted branch")),
		Update:  key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "UPDATE the current branch from the default branch (rebase or merge)")),

		Yes:          key.NewBinding(key.WithKeys("y", "Y"), key.WithHelp("y", "Confirm")),
		No:           key.NewBinding(key.WithKeys("n", "N", "q", "esc", "enter", "ctrl+c"), key.WithHelp("n", "Cancel")),
		ToggleRemote: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "Also delete the remote branches")),

		OpUp:     key.NewBinding(key.WithKeys("up", "k", "shift+tab"), key.WithHelp("↑/k", "Previous option")),
		OpDown:   key.NewBinding(key.WithKeys("down", "j", "tab"), key.WithHelp("↓/j", "Next option")),
		OpRun:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("Enter", "Run")),
		OpCancel: key.NewBinding(key.WithKeys("esc", "q", "n", "ctrl+c"), key.WithHelp("Esc", "Cancel")),
	}
}

// globalBindings are active in every list tab.
func (k keyMap) globalBindings() []key.Binding {
	return []key.Binding{k.Quit, k.Help, k.NextTab, k.PrevTab, k.TabClean, k.TabBranch, k.Fetch, k.CycleSort}
}

// tabBindings are the bindings that only apply in the given tab.
func (k keyMap) tabBindings(t viewTab) []key.Binding {
	switch t {
	case tabBranches:
		return []key.Binding{k.Actions, k.Merge, k.Rebase, k.Update}
	default:
		return []key.Binding{k.Toggle, k.SelectAll, k.SelectMerged, k.SelectGone, k.SelectStale, k.ClearSelection, k.Proceed}
	}
}

// footer renders the two-line hint bar below the list, tailored to the active tab.
func (k keyMap) footer(t viewTab, sortLabel string) string {
	var first, second []string
	switch t {
	case tabBranches:
		first = []string{
			hint(k.Actions, "actions"), hint(k.Merge, "merge"), hint(k.Rebase, "rebase"), hint(k.Update, "update"),
		}
		second = []string{
			hint(k.Fetch, "fetch"), hint(k.CycleSort, "sort ("+sortLabel+")"), "Tab: views", hint(k.Help, "help"),
		}
	default:
		first = []string{
			hint(k.SelectAll, "all"), hint(k.SelectMerged, "merged"), hint(k.SelectGone, "gone"),
			hint(k.SelectStale, "stale"), hint(k.ClearSelection, "clear"),
		}
		second = []string{
			hint(k.Proceed, "delete selected"), hint(k.Fetch, "fetch"), hint(k.CycleSort, "sort ("+sortLabel+")"),
			"Tab: views", hint(k.Help, "help"),
		}
	}
	return "  " + strings.Join(first, " • ") + "\n  " + strings.Join(second, " • ")
}

// footerLines is the number of lines footer() renders.
const footerLines = 2

func hint(b key.Binding, label string) string {
	return b.Help().Key + ": " + label
}

// HelpSection is a titled group of {key, description} rows.
type HelpSection struct {
	Title string
	Rows  [][2]string
}

func rows(bindings ...key.Binding) [][2]string {
	out := make([][2]string, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, [2]string{b.Help().Key, b.Help().Desc})
	}
	return out
}

// HelpSections returns all interactive keybindings, grouped, for the help overlay and `--help`.
func HelpSections() []HelpSection {
	k := defaultKeyMap()
	return []HelpSection{
		{"Navigation", append([][2]string{
			{"↑ / k", "Move cursor up"},
			{"↓ / j", "Move cursor down"},
			{"/", "Filter branches by name (Esc clears the filter)"},
		}, rows(k.NextTab, k.PrevTab, k.TabClean, k.TabBranch)...)},
		{"Everywhere in the lists", rows(k.Fetch, k.CycleSort, k.Help, k.Quit)},
		{"Cleanup view: select and delete", rows(
			k.Toggle, k.SelectAll, k.SelectMerged, k.SelectGone, k.SelectStale, k.ClearSelection, k.Proceed,
		)},
		{"Branches view: act on one branch", rows(k.Actions, k.Merge, k.Rebase, k.Update)},
		{"Delete confirmation", [][2]string{
			{"y", "Confirm (a capital Y is required when commits would be lost)"},
			{"r", "Also delete the remote branches"},
			{"n / Esc", "Cancel"},
		}},
		{"Menus and merge / rebase dialogs", [][2]string{
			{"↑ ↓ / j k", "Choose an option"},
			{"1-9", "Pick an option directly"},
			{"Enter", "Run the chosen option"},
			{"Esc", "Cancel"},
		}},
	}
}
