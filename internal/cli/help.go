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
This file renders the help menu. Commands, flags and keybindings are all read from their respective registries.
*/

package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/vhjan/git-janitor/internal/buildinfo"
	"github.com/vhjan/git-janitor/internal/theme"
	"github.com/vhjan/git-janitor/internal/tui"
)

var helpCommand = &Command{
	Name:    "help",
	Summary: "Shows this help menu",
	Run: func(ctx *Context, _ []string) int {
		fmt.Fprintln(ctx.Stdout, renderHelp(ctx.Program))
		return 0
	},
}

func renderHelp(program string) string {
	var sb strings.Builder

	headingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorPrimary)).Bold(true).Underline(true)
	keyColumn := lipgloss.NewStyle().Foreground(lipgloss.Color(theme.ColorSuccess)).Width(18)
	section := func(title string, rows [][2]string) {
		sb.WriteString("\n" + headingStyle.Render(title) + "\n")
		for _, row := range rows {
			sb.WriteString(fmt.Sprintf("  %s %s\n", keyColumn.Render(row[0]), row[1]))
		}
	}

	sb.WriteString(theme.Title.Render("🧹 Git Janitor") + "\n\n")
	sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("252")).
		Render("A fast, interactive TUI for cleaning up local Git branches.") + "\n\n")

	sb.WriteString(headingStyle.Render("USAGE:") + "\n")
	sb.WriteString(fmt.Sprintf("  %s [command] [flags]\n", program))
	sb.WriteString(theme.Muted.Render(fmt.Sprintf("  Also available as: %s, git janitor", otherName(program))) + "\n")

	var cmdRows [][2]string
	for _, c := range registry {
		cmdRows = append(cmdRows, [2]string{c.Name, c.Summary})
	}
	section("COMMANDS:", cmdRows)
	section("FLAGS:", newRootFlags().helpRows())
	section("INTERACTIVE KEYBINDINGS:", tui.HelpRows())

	sb.WriteString("\n" + headingStyle.Render("EXAMPLES:") + "\n")
	sb.WriteString(fmt.Sprintf("  %s update\n", program))
	sb.WriteString(fmt.Sprintf("  %s --dry-run\n", program))
	sb.WriteString(fmt.Sprintf("  %s --protect \"production,release-*\"\n", program))
	sb.WriteString(fmt.Sprintf("  %s --stale-days 14\n", program))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(theme.ColorSecondary)).
		Padding(1, 4).
		Render(sb.String())
}

// otherName returns the alias to advertise next to the name currently in use.
func otherName(program string) string {
	if program == buildinfo.ShortName {
		return buildinfo.Name
	}
	return buildinfo.ShortName
}
