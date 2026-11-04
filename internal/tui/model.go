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
Package tui is the interactive Bubble Tea front end.

This file contains the root model. It owns the shared state and routes every message to the view that belongs to
the current state.

Adding a new view:
 1. add a constant to the state enum below,
 2. create view_<name>.go with `update<Name>(msg)` and `view<Name>()` methods on Model,
 3. add a case for it in Update and View.
*/
package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/vhjan/git-janitor/internal/config"
	"github.com/vhjan/git-janitor/internal/git"
	"github.com/vhjan/git-janitor/internal/theme"
)

// state identifies the active view.
type state int

const (
	stateLoading state = iota // Initial loading view
	stateList                 // Primary view: selecting branches
	stateConfirm              // Confirmation dialog before deleting the selected branches
)

// docStyle defines the layout margins of the main window.
var docStyle = lipgloss.NewStyle().Margin(theme.DocMarginVertical, theme.DocMarginHorizontal)

// Result is what the TUI hands back to the caller once it exits.
type Result struct {
	Deleted      []string // Branches deleted (or that would be deleted in a dry run)
	Errors       []string // Errors and warnings encountered along the way
	DryRun       bool     // Whether Deleted is hypothetical
	UpdateNotice string   // Rendered "update available" notice, if any
}

// Model is the complete application state for Bubble Tea.
type Model struct {
	cfg  config.Config
	repo *git.Repo
	keys keyMap

	list    list.Model
	spinner spinner.Model

	state         state
	width, height int
	sort          sortMode

	result Result
}

// New builds the initial model.
func New(cfg config.Config, repo *git.Repo) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorPrimary))

	return Model{
		cfg:     cfg,
		repo:    repo,
		keys:    defaultKeyMap(),
		list:    newList(cfg),
		spinner: s,
		state:   stateLoading,
		result:  Result{DryRun: cfg.DryRun},
	}
}

// Run starts the TUI on the alternate screen and blocks until it exits.
func Run(cfg config.Config, repo *git.Repo) (Result, error) {
	final, err := tea.NewProgram(New(cfg, repo), tea.WithAltScreen()).Run()
	if err != nil {
		return Result{}, err
	}
	if m, ok := final.(Model); ok {
		return m.result, nil
	}
	return Result{}, nil
}

// newList creates the styled list component used for the branch view.
func newList(cfg config.Config) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color(theme.ColorPrimary)).
		BorderLeftForeground(lipgloss.Color(theme.ColorPrimary))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color(theme.ColorSecondary)).
		BorderLeftForeground(lipgloss.Color(theme.ColorPrimary))

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Git Janitor"
	if cfg.DryRun {
		l.Title += " (DRY RUN)"
	}
	l.Styles.Title = lipgloss.NewStyle().
		Background(lipgloss.Color(theme.ColorPrimary)).
		Foreground(lipgloss.Color(theme.ColorOnPrimary)).
		Padding(0, 1)
	l.FilterInput.PromptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorTitle))
	l.FilterInput.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorPrimary))
	return l
}

// Init starts the background work: spinner, branch loading and the update check.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		loadBranchesCmd(m.repo, m.cfg),
		checkUpdateCmd(),
	)
}

// Update is the central event loop. Messages that are independent of the active view are handled first; everything
// else is routed to the current view.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v-2)

	case branchesLoadedMsg:
		if msg.err != nil {
			m.result.Errors = append(m.result.Errors, fmt.Sprintf("Error fetching branches: %v", msg.err))
			return m, tea.Quit
		}
		cmd := m.list.SetItems(msg.items)
		m.state = stateList
		return m, cmd

	case updateCheckedMsg:
		m.result.UpdateNotice = msg.notice
		return m, nil
	}

	switch m.state {
	case stateLoading:
		return m.updateLoading(msg)
	case stateList:
		return m.updateList(msg)
	case stateConfirm:
		return m.updateConfirm(msg)
	}
	return m, nil
}

// View renders the active view.
func (m Model) View() string {
	switch m.state {
	case stateLoading:
		return m.viewLoading()
	case stateConfirm:
		return m.viewConfirm()
	default:
		return m.viewList()
	}
}
