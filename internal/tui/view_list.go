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
This file implements the main view: the filterable branch list with selection, bulk-select shortcuts and sorting.
*/

package tui

import (
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
		return m, m.selectWhere(func(i item) bool { return i.Merged }), true

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

	case key.Matches(msg, m.keys.Proceed):
		if len(m.selectedNames()) > 0 {
			m.state = stateConfirm
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

// selectedNames returns the names of all selected, unprotected branches in list order.
func (m Model) selectedNames() []string {
	var names []string
	for _, li := range m.list.Items() {
		if i, ok := li.(item); ok && i.selected && !i.Protected {
			names = append(names, i.Name)
		}
	}
	return names
}

func (m Model) viewList() string {
	return docStyle.Render(m.list.View() + "\n" + footerStyle.Render(m.keys.footer(m.sort.String())))
}
