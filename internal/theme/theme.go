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
Package theme centralises the look of the application: colours, layout dimensions, list symbols and a few shared
lipgloss styles. Change values here to restyle the whole tool.
*/
package theme

import "github.com/charmbracelet/lipgloss"

// Colours accept standard ANSI codes (e.g. "62") or hex codes (e.g. "#8839ef").
const (
	ColorPrimary   = "#38BDF8" // Primary borders, selected item titles, confirm dialog
	ColorSecondary = "#7DD3FC" // Selected item descriptions, summary/help box borders
	ColorTextMuted = "#64748B" // Footer help text and version details
	ColorSuccess   = "#4ADE80" // Success headers and help key columns
	ColorWarning   = "#FB923C" // Error and warning headers
	ColorTitle     = "#FBBF24" // Summary and help menu titles
	ColorOnPrimary = "#0F172A" // Text drawn on top of a primary-coloured background
)

// Layout and sizing constraints shared by the different views.
const (
	DocMarginVertical        = 1  // Vertical margin around the main list
	DocMarginHorizontal      = 2  // Horizontal margin around the main list
	DialogPaddingVertical    = 1  // Vertical padding inside dialog boxes
	ConfirmPaddingHorizontal = 4  // Horizontal padding inside the confirmation dialog
	SummaryPaddingHorizontal = 3  // Horizontal padding inside the final summary box
	SummaryBoxWidth          = 50 // Fixed width of the final summary box
)

// Prefixes for the interactive branch list.
const (
	SymbolProtected  = "🔒  "  // Branch that cannot be deleted
	SymbolSelected   = "[x] " // Branch marked for deletion
	SymbolUnselected = "[ ] " // Branch not selected
	SymbolCurrent    = "●   " // The checked-out branch (Branches view)
	SymbolPlain      = "    " // Any other branch (Branches view)
)

// Shared text styles for terminal output outside of the TUI (help, version, update, summary).
var (
	Title   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTitle)).Bold(true)
	Muted   = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorTextMuted))
	Success = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Bold(true)
	Warning = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)).Bold(true)
)
