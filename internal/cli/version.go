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
This file implements the `version` command (and the -v/--version flag).
*/

package cli

import (
	"fmt"

	"github.com/vhjan/git-janitor/internal/buildinfo"
	"github.com/vhjan/git-janitor/internal/theme"
)

var versionCommand = &Command{
	Name:    "version",
	Summary: "Shows the version of Git Janitor",
	Run: func(ctx *Context, _ []string) int {
		fmt.Fprintln(ctx.Stdout, theme.Title.Render("🧹  Git Janitor")+fmt.Sprintf(" version %s", buildinfo.Version))
		fmt.Fprintln(ctx.Stdout, theme.Muted.Render(fmt.Sprintf("  Commit:  %s", buildinfo.Commit)))
		fmt.Fprintln(ctx.Stdout, theme.Muted.Render(fmt.Sprintf("  Built:   %s", buildinfo.Date)))
		return 0
	},
}
