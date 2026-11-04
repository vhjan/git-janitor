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
This file defines the 'item' type, which adapts a branches.Branch to the list.Item interface from the
charmbracelet/bubbles/list package. It represents a single branch row in the TUI list.
*/

package tui

import (
	"strings"

	"github.com/vhjan/git-janitor/internal/branches"
	"github.com/vhjan/git-janitor/internal/theme"
)

// item is a selectable branch row. The embedded Branch carries the git-derived state; selected is pure UI state.
type item struct {
	branches.Branch
	selected bool // Whether the user has marked this branch for deletion
}

// Title returns the primary text of the row, prefixed with a symbol for its state (protected, selected, ...).
func (i item) Title() string {
	if i.Protected {
		return theme.SymbolProtected + i.Name
	}
	if i.selected {
		return theme.SymbolSelected + i.Name
	}
	return theme.SymbolUnselected + i.Name
}

// Description provides the secondary text: status tags (Merged, Gone, Stale) and the last active date.
func (i item) Description() string {
	if i.Protected {
		d := "Protected or active branch (cannot be deleted)"
		if i.Age != "" {
			d += " | Last active: " + i.Age
		}
		return d
	}

	var tags []string
	if i.Merged {
		tags = append(tags, "Merged")
	}
	if i.Gone {
		tags = append(tags, "Gone (Upstream deleted)")
	}
	if i.Stale {
		tags = append(tags, "Stale")
	}

	desc := "Press space to toggle"
	if len(tags) > 0 {
		desc += " | " + strings.Join(tags, ", ")
	}
	if i.Age != "" {
		desc += " | Last active: " + i.Age
	}
	return desc
}

// FilterValue is the string the list's fuzzy finder matches against.
func (i item) FilterValue() string {
	return i.Name
}
