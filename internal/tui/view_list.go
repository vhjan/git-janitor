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
This file implements the main view: the filterable branch list with selection, bulk-select shortcuts, sorting and
the entry points to fetching, merging, rebasing and deleting.
*/

package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vhjan/git-janitor/internal/theme"
)

var footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorTextMuted))

func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Only intercept our own keys when the user is not typing into the filter input.
	if k, ok := msg.(tea.KeyMsg); ok && m.list.FilterState() != list.Filtering {
		if next, cmd, handled := m.handleListKey(k); handled {
			return next, cmd
		}
	}

	// Everything else (navigation, filtering, paging) is handled by the list component itself.
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// handleListKey processes the keys specific to branch operations. The bool reports whether the key was consumed.
func (m Model) handleListKey(msg tea.KeyMsg) (Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit, true

	case key.Matches(msg, m.keys.Toggle):
		if i, ok := m.list.SelectedItem().(item); ok && !i.Protected {
			i.selected = !i.selected
			// GlobalIndex (not Index) is the position in the unfiltered items, which is what SetItem expects.
			m.list.SetItem(m.list.GlobalIndex(), i)
		}
		return m, nil, true

	case key.Matches(msg, m.keys.SelectAll):
		return m, m.selectWhere(func(item) bool { return true }), true

	case key.Matches(msg, m.keys.SelectMerged):
		return m, m.selectWhere(func(i item) bool { return i.FullyMerged() }), true

	case key.Matches(msg, m.keys.SelectGone):
		return m, m.selectWhere(func(i item) bool { return i.Gone }), true

	case key.Matches(msg, m.keys.SelectStale):
		return m, m.selectWhere(func(i item) bool { return i.Stale }), true

	case key.Matches(msg, m.keys.ClearSelection):
		return m, m.updateVisible(func(i *item) { i.selected = false }), true

	case key.Matches(msg, m.keys.CycleSort):
		m.sort = m.sort.next()
		items := m.list.Items()
		sortItems(items, m.sort)
		return m, m.list.SetItems(items), true

	case key.Matches(msg, m.keys.Fetch):
		busy := m.startBusy("Fetching and pruning remotes...")
		return m, tea.Batch(busy, loadBranchesCmd(m.repo, m.cfg, loadOptions{fetch: true, announce: true})), true

	case key.Matches(msg, m.keys.Merge):
		next, cmd := m.beginOp(opMerge)
		return next, cmd, true

	case key.Matches(msg, m.keys.Rebase):
		next, cmd := m.beginOp(opRebase)
		return next, cmd, true

	case key.Matches(msg, m.keys.Update):
		next, cmd := m.beginOp(opUpdate)
		return next, cmd, true

	case key.Matches(msg, m.keys.Proceed):
		if len(m.selectedItems()) > 0 {
			m.state = stateConfirm
			m.deleteRemote = false
		}
		return m, nil, true
	}

	return m, nil, false
}

// selectWhere marks every visible, unprotected branch matching pred as selected.
func (m *Model) selectWhere(pred func(item) bool) tea.Cmd {
	return m.updateVisible(func(i *item) {
		if !i.Protected && pred(*i) {
			i.selected = true
		}
	})
}

// updateVisible applies fn to every item that is currently visible (i.e. respecting an active filter) and
// leaves hidden items untouched.
func (m *Model) updateVisible(fn func(*item)) tea.Cmd {
	visible := make(map[string]bool)
	for _, v := range m.list.VisibleItems() {
		if i, ok := v.(item); ok {
			visible[i.Name] = true
		}
	}

	all := m.list.Items()
	updated := make([]list.Item, len(all))
	for idx, li := range all {
		i := li.(item)
		if visible[i.Name] {
			fn(&i)
		}
		updated[idx] = i
	}
	return m.list.SetItems(updated)
}

// selectedItems returns all selected, unprotected branches in list order.
func (m Model) selectedItems() []item {
	var items []item
	for _, li := range m.list.Items() {
		if i, ok := li.(item); ok && i.selected && !i.Protected {
			items = append(items, i)
		}
	}
	return items
}

// selectedNames returns the names of all selected, unprotected branches in list order.
func (m Model) selectedNames() []string {
	var names []string
	for _, i := range m.selectedItems() {
		names = append(names, i.Name)
	}
	return names
}

// staleDefaultNotice warns when the local default branch lacks commits that its remote has. The "merged" checks
// compare against the local default branch (deliberately: users may not want to update it yet), so merges that
// happened on the remote, such as squash-merged pull requests, would not show up. Returns "" when in sync.
func (m Model) staleDefaultNotice() string {
	if m.defaultBehind <= 0 || m.defaultUpstream == "" {
		return ""
	}
	return fmt.Sprintf("⚠ Local %s is %d commit%s behind %s, so branches merged on the remote may not show as merged. "+
		"Checking against local %s as it is.",
		m.defaultBranch, m.defaultBehind, plural(m.defaultBehind), m.defaultUpstream, m.defaultBranch)
}

// renderNotice renders the stale-default notice wrapped to the window width, or "" if there is none.
func (m Model) renderNotice() string {
	text := m.staleDefaultNotice()
	if text == "" {
		return ""
	}
	h, _ := docStyle.GetFrameSize()
	width := max(m.width-h, 20)
	return lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorWarning)).PaddingLeft(2).Width(width).Render(text)
}

// resizeList gives the list the space left after the margins, the footer and the optional notice.
func (m *Model) resizeList() {
	h, v := docStyle.GetFrameSize()
	reserved := v + footerLines + 1
	if notice := m.renderNotice(); notice != "" {
		reserved += lipgloss.Height(notice)
	}
	m.list.SetSize(m.width-h, m.height-reserved)
}

func (m Model) viewList() string {
	body := m.list.View()
	if notice := m.renderNotice(); notice != "" {
		body += "\n" + notice
	}
	return docStyle.Render(body + "\n" + footerStyle.Render(m.keys.footer(m.sort.String())))
}
