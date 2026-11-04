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
	"fmt"
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

// Description provides the secondary text: status tags, sync state with the upstream, a data-loss warning and the
// last active date.
func (i item) Description() string {
	var parts []string

	if i.Protected {
		parts = append(parts, "Protected: "+i.ProtectedReason)
	} else {
		switch {
		case i.Merged:
			parts = append(parts, "Merged")
		case i.Squashed:
			parts = append(parts, "Merged (squash/rebase)")
		}
		if i.Gone {
			parts = append(parts, "Gone (Upstream deleted)")
		}
		if i.Stale {
			parts = append(parts, "Stale")
		}
		if i.Remote == "" && !i.Gone {
			parts = append(parts, "Local only")
		}
		if i.AtRisk() {
			parts = append(parts, fmt.Sprintf("⚠ %d unpushed commit%s", i.Unpushed, plural(i.Unpushed)))
		}
	}

	if sync := i.syncInfo(); sync != "" {
		parts = append(parts, sync)
	}
	if i.Age != "" {
		parts = append(parts, "Last active: "+i.Age)
	}

	if len(parts) == 0 {
		return "Press space to toggle"
	}
	return strings.Join(parts, " | ")
}

// syncInfo renders the position relative to the upstream, e.g. "↑2 ↓1".
func (i item) syncInfo() string {
	var s []string
	if i.Ahead > 0 {
		s = append(s, fmt.Sprintf("↑%d", i.Ahead))
	}
	if i.Behind > 0 {
		s = append(s, fmt.Sprintf("↓%d", i.Behind))
	}
	return strings.Join(s, " ")
}

// FilterValue is the string the list's fuzzy finder matches against.
func (i item) FilterValue() string {
	return i.Name
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
