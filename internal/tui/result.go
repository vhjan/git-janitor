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
This file renders the summary that is printed to the terminal after the TUI has closed.
*/

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vhjan/git-janitor/internal/theme"
)

// RenderResult formats what happened during the session: an optional update notice followed by a summary box of
// deleted branches and errors. It returns "" when there is nothing to report.
func RenderResult(r Result) string {
	var out strings.Builder

	if r.UpdateNotice != "" {
		out.WriteString(r.UpdateNotice + "\n\n")
	}

	if len(r.Deleted) > 0 || len(r.Errors) > 0 {
		out.WriteString(renderSummaryBox(r) + "\n")
	}

	return out.String()
}

func renderSummaryBox(r Result) string {
	var sb strings.Builder

	sb.WriteString(theme.Title.Render("* Git Janitor Summary *") + "\n\n")

	if len(r.Deleted) > 0 {
		header := "Deleted Branches:"
		if r.DryRun {
			header = "Dry Run - Would Delete (but not actually):"
		}
		sb.WriteString(theme.Success.Render(header) + "\n")
		for _, b := range r.Deleted {
			sb.WriteString(fmt.Sprintf("  - %s\n", b))
		}
		sb.WriteString("\n")
	}

	if len(r.Errors) > 0 {
		sb.WriteString(theme.Warning.Render("Errors & Warnings:") + "\n")
		for _, e := range r.Errors {
			sb.WriteString(fmt.Sprintf("  ! %s\n", e))
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.ColorSecondary)).
		Padding(theme.DialogPaddingVertical, theme.SummaryPaddingHorizontal).
		Width(theme.SummaryBoxWidth).
		Render(strings.TrimSpace(sb.String()))
}
