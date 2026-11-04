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
This file launches the interactive TUI, which is what runs when no subcommand is given.
*/

package cli

import (
	"errors"
	"fmt"

	"github.com/vhjan/git-janitor/internal/git"
	"github.com/vhjan/git-janitor/internal/theme"
	"github.com/vhjan/git-janitor/internal/tui"
)

func runTUI(ctx *Context) int {
	repo, err := git.Open("")
	if err != nil {
		var tooOld *git.TooOldError
		switch {
		case errors.Is(err, git.ErrNotRepo):
			fmt.Fprintln(ctx.Stdout, theme.Warning.Render("❌ Error: Not inside a git repository."))
		case errors.As(err, &tooOld):
			fmt.Fprintln(ctx.Stdout, theme.Warning.Render("❌ Error: ")+tooOld.Error())
			fmt.Fprintln(ctx.Stdout, theme.Muted.Render("   Please upgrade git: https://git-scm.com/downloads"))
		default:
			fmt.Fprintln(ctx.Stdout, theme.Warning.Render("❌ Error: ")+err.Error())
		}
		return 1
	}

	result, err := tui.Run(ctx.Config, repo)
	if err != nil {
		fmt.Fprintf(ctx.Stdout, "Error running program: %v\n", err)
		return 1
	}

	fmt.Fprint(ctx.Stdout, tui.RenderResult(result))
	return 0
}
