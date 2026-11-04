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
This file implements the help overlay (`?`): every keybinding, grouped by view, in a scrollable box. The content is
generated from HelpSections(), so it can never drift from the real key handling.
*/

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vhjan/git-janitor/internal/theme"
)

const (
	helpMaxWidth = 76 // Widest the help box gets, however large the terminal
	helpKeyWidth = 14 // Width of the key column
	helpChrome   = 11 // Lines used by the box border, padding, title and hint around the scrollable content
)

// openHelp shows the help overlay.
func (m *Model) openHelp() {
	m.help = viewport.New(0, 0)
	m.resizeHelp()
	m.state = stateHelp
}

// resizeHelp fits the help box to the window and rebuilds its content for the new width.
func (m *Model) resizeHelp() {
	m.help.Width = max(30, min(helpMaxWidth, m.width-14))
	m.help.Height = max(5, m.height-helpChrome)
	m.help.SetContent(helpContent(m.help.Width))
}

// helpContent renders all help sections for the given text width.
func helpContent(width int) string {
	heading := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorPrimary)).Bold(true)
	keyCol := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorSuccess)).Width(helpKeyWidth)
	descCol := lipgloss.NewStyle().Width(max(10, width-helpKeyWidth-1))

	var out []string
	for _, section := range HelpSections() {
		out = append(out, heading.Render(section.Title))
		for _, row := range section.Rows {
			out = append(out, lipgloss.JoinHorizontal(lipgloss.Top, keyCol.Render(row[0]), " ", descCol.Render(row[1])))
		}
		out = append(out, "")
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func (m Model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && (key.Matches(k, m.keys.Help) || key.Matches(k, m.keys.OpCancel)) {
		m.state = stateList
		return m, nil
	}

	var cmd tea.Cmd
	m.help, cmd = m.help.Update(msg)
	return m, cmd
}

func (m Model) viewHelp() string {
	title := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorTitle)).Bold(true).Render("Keyboard shortcuts")
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorTextMuted)).
		Render(fmt.Sprintf("↑/↓ scroll · ?/esc close · %d%%", int(m.help.ScrollPercent()*100)))

	box := confirmStyle.Render(title + "\n\n" + m.help.View() + "\n\n" + hint)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
