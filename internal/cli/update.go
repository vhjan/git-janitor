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
This file implements the `update` command: it checks GitHub for a newer release and upgrades through the package
manager that installed the tool.
*/

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/vhjan/git-janitor/internal/buildinfo"
	"github.com/vhjan/git-janitor/internal/theme"
	"github.com/vhjan/git-janitor/internal/updater"
)

var updateCommand = &Command{
	Name:    "update",
	Summary: "Update Git Janitor to the latest version",
	Run:     runUpdate,
}

func runUpdate(ctx *Context, _ []string) int {
	out := ctx.Stdout
	say := func(s string) { fmt.Fprintln(out, s) }

	say(theme.Title.Render("🧹  Git Janitor - Update"))
	say("")

	fmt.Fprint(out, theme.Muted.Render("  Checking for updates..."))
	latest, err := updater.LatestVersion()
	if err != nil {
		say("")
		say(theme.Warning.Render("  ✗ Could not reach GitHub: ") + err.Error())
		return 1
	}
	say("") // newline after "Checking..."

	current := updater.Normalize(buildinfo.Version)
	if current == latest && !buildinfo.IsDev() {
		say(theme.Success.Render("  ✓ You are already on the latest version (" + current + ")"))
		return 0
	}

	if !buildinfo.IsDev() {
		say(theme.Muted.Render(fmt.Sprintf("  Current version: %s", current)))
		say(theme.Muted.Render(fmt.Sprintf("  Latest version:  %s", latest)))
		say("")
	}

	pm := updater.Detect()
	if pm == nil {
		say(theme.Warning.Render("  ✗ Could not detect your package manager."))
		say(theme.Muted.Render("  Please upgrade manually:"))
		say(theme.Muted.Render("    " + updater.ManualCommand()))
		return 1
	}

	say(theme.Muted.Render(fmt.Sprintf("  Detected package manager: %s", pm.Name)))
	say(theme.Muted.Render(fmt.Sprintf("  Running: %s", strings.Join(pm.Command, " "))))
	say("")

	if err := pm.Run(os.Stdin, ctx.Stdout, ctx.Stderr); err != nil {
		say("")
		say(theme.Warning.Render("  ✗ Update failed: ") + err.Error())
		return 1
	}

	say("")
	say(theme.Success.Render("  ✓ Git Janitor updated successfully!"))
	say(theme.Success.Render("    Installed version v" + latest))
	return 0
}
