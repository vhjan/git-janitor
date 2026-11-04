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
This file implements the sort orders of the branch list.
*/

package tui

import (
	"sort"

	"github.com/charmbracelet/bubbles/list"
)

// sortMode defines the available sort orders for the branch list.
type sortMode int

const (
	sortAlphabetical sortMode = iota
	sortLatestCommits
	sortOldestCommits
	sortModeCount // Number of modes; keep last
)

// next returns the mode that follows m, wrapping around.
func (m sortMode) next() sortMode {
	return (m + 1) % sortModeCount
}

// String returns the label shown in the footer.
func (m sortMode) String() string {
	switch m {
	case sortLatestCommits:
		return "Latest Commits"
	case sortOldestCommits:
		return "Oldest Commits"
	default:
		return "Alphabetical"
	}
}

// sortItems sorts list items in place according to the given mode.
func sortItems(items []list.Item, mode sortMode) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].(item), items[j].(item)
		switch mode {
		case sortLatestCommits:
			return a.LastCommitUnix > b.LastCommitUnix
		case sortOldestCommits:
			return a.LastCommitUnix < b.LastCommitUnix
		default:
			return a.Name < b.Name
		}
	})
}
