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
This file implements the confirmation dialog shown before the selected branches are deleted.
*/

package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vhjan/git-janitor/internal/branches"
	"github.com/vhjan/git-janitor/internal/theme"
)

// confirmStyle defines the appearance of the confirmation dialog.
var confirmStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(theme.ColorPrimary)).
	Padding(theme.DialogPaddingVertical, theme.ConfirmPaddingHorizontal).
	Align(lipgloss.Center)

func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(k, m.keys.No):
		m.state = stateList
		return m, nil

	case key.Matches(k, m.keys.Yes):
		res := branches.Delete(m.repo, m.selectedNames(), m.cfg.DryRun)
		m.result.Deleted = append(m.result.Deleted, res.Deleted...)
		m.result.Errors = append(m.result.Errors, res.Errors...)
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	count := len(m.selectedNames())

	prompt := fmt.Sprintf("Are you sure you want to force delete %d branches?\n\n(y/N)", count)
	if m.cfg.DryRun {
		prompt = fmt.Sprintf("DRY RUN: Would delete %d branches.\nProceed?\n\n(y/N)", count)
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, confirmStyle.Render(prompt))
}
