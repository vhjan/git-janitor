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
This file implements the action menu: a small popup listing everything that can be done with the highlighted branch
in the Branches view. Entries state exactly what they will do ("Merge feature into main") and are greyed out with a
reason when unavailable. Each entry also has a letter that triggers it directly.

Adding an action: append a menuItem in openMenu. Nothing else is needed.
*/

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vhjan/git-janitor/internal/theme"
)

// menuItem is one entry of the action menu.
type menuItem struct {
	label    string
	accel    string // Letter that triggers the entry, shown on the right
	disabled string // Why the entry is unavailable; "" when it is available
	run      func(m Model) (Model, tea.Cmd)
}

// menuState is the open action menu.
type menuState struct {
	branch item // The branch the menu was opened for
	items  []menuItem
	cursor int // Index of the highlighted entry; -1 when no entry can be selected
}

// openMenu builds the action menu for the highlighted branch and switches to it.
func (m *Model) openMenu() {
	sel, ok := m.list.SelectedItem().(item)
	if !ok {
		return
	}

	// Merge and rebase need a second, different branch to work against.
	otherProblem := ""
	switch {
	case m.current == "":
		otherProblem = "HEAD is detached"
	case sel.Name == m.current:
		otherProblem = "this is the current branch"
	}
	updateProblem := ""
	if m.current == "" {
		updateProblem = "HEAD is detached"
	}
	deleteProblem := ""
	if sel.Protected {
		deleteProblem = "protected: " + sel.ProtectedReason
	}

	current := m.current
	if current == "" {
		current = "HEAD"
	}

	startOp := func(kind opKind) func(Model) (Model, tea.Cmd) {
		return func(m Model) (Model, tea.Cmd) {
			m.menu = nil
			m.state = stateList
			return m.beginOp(kind)
		}
	}

	m.menu = &menuState{
		branch: sel,
		items: []menuItem{
			{label: fmt.Sprintf("Merge %s into %s", sel.Name, current), accel: "m", disabled: otherProblem, run: startOp(opMerge)},
			{label: fmt.Sprintf("Rebase %s onto %s", current, sel.Name), accel: "r", disabled: otherProblem, run: startOp(opRebase)},
			{label: fmt.Sprintf("Update %s from %s", current, m.defaultBranch), accel: "u", disabled: updateProblem, run: startOp(opUpdate)},
			{label: fmt.Sprintf("Delete branch %s…", sel.Name), accel: "d", disabled: deleteProblem, run: func(m Model) (Model, tea.Cmd) {
				target := sel
				m.menu = nil
				m.deleteOnly = &target
				m.deleteRemote = false
				m.state = stateConfirm
				return m, nil
			}},
		},
	}
	// Start on the first entry that can actually be used (-1 if there is none).
	m.menu.cursor = -1
	for idx, it := range m.menu.items {
		if it.disabled == "" {
			m.menu.cursor = idx
			break
		}
	}
	m.state = stateMenu
}

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok || m.menu == nil {
		return m, nil
	}
	menu := m.menu

	run := func(idx int) (tea.Model, tea.Cmd) {
		if idx < 0 || idx >= len(menu.items) {
			return m, nil // nothing is selectable
		}
		item := menu.items[idx]
		if item.disabled != "" {
			return m, nil
		}
		return item.run(m)
	}

	switch {
	case key.Matches(k, m.keys.OpCancel):
		m.menu = nil
		m.state = stateList
		return m, nil

	case key.Matches(k, m.keys.OpUp):
		menu.move(-1)
		return m, nil

	case key.Matches(k, m.keys.OpDown):
		menu.move(1)
		return m, nil

	case key.Matches(k, m.keys.OpRun):
		return run(menu.cursor)
	}

	// Letters trigger their entry directly; digits move the cursor to that entry if it is available.
	s := k.String()
	for idx, it := range menu.items {
		if strings.EqualFold(s, it.accel) {
			return run(idx)
		}
	}
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		if idx := int(s[0] - '1'); idx < len(menu.items) && menu.items[idx].disabled == "" {
			menu.cursor = idx
		}
	}
	return m, nil
}

func (m Model) viewMenu() string {
	if m.menu == nil {
		return ""
	}
	menu := m.menu
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorTitle)).Bold(true)
	selected := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorPrimary)).Bold(true)
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorTextMuted))

	width := 0
	for _, it := range menu.items {
		width = max(width, lipgloss.Width(it.label))
	}

	lines := []string{title.Render(menu.branch.Name), ""}
	for idx, it := range menu.items {
		label := lipgloss.NewStyle().Width(width).Render(it.label)
		accel := muted.Render(it.accel)
		switch {
		case it.disabled != "":
			lines = append(lines, "  "+muted.Render(label+"  "+it.accel+"  ("+it.disabled+")"))
			continue
		case idx == menu.cursor:
			lines = append(lines, "> "+selected.Render(label)+"  "+accel)
		default:
			lines = append(lines, "  "+label+"  "+accel)
		}
	}
	lines = append(lines, "", muted.Render("↑/↓ choose · enter select · esc close"))

	box := confirmStyle.Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// move puts the cursor on the next available entry in direction dir (+1 down, -1 up), wrapping around and skipping
// entries that are disabled. If no other entry is available the cursor stays where it is, so the selection never
// lands on (or disappears into) something that can't be chosen.
func (s *menuState) move(dir int) {
	n := len(s.items)
	start := s.cursor
	if start < 0 {
		start = 0
	}
	for step := 1; step <= n; step++ {
		idx := ((start+dir*step)%n + n) % n
		if s.items[idx].disabled == "" {
			s.cursor = idx
			return
		}
	}
}
