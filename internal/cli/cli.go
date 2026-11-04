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
Package cli is the command-line front end: it parses flags, dispatches subcommands and launches the TUI when no
subcommand is given.

Adding a subcommand:
 1. create a file with a `Command` value and a run function,
 2. add it to the registry in init() below.

It then shows up in `--help` automatically.
*/
package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/vhjan/git-janitor/internal/buildinfo"
	"github.com/vhjan/git-janitor/internal/config"
	"github.com/vhjan/git-janitor/internal/theme"
)

// Context carries everything a command needs. Writers are injected so commands are testable.
type Context struct {
	Stdout, Stderr io.Writer
	Config         config.Config
	Program        string // Name the user invoked (git-janitor or gj), used in messages
}

// Command is a named subcommand such as `update`.
type Command struct {
	Name    string
	Summary string
	Run     func(ctx *Context, args []string) int
}

// registry holds all subcommands. It is filled in init (rather than a var initializer) because the help command
// lists the registry, which would otherwise be an initialization cycle.
var registry []*Command

func init() {
	registry = []*Command{
		helpCommand,
		versionCommand,
		updateCommand,
	}
}

func findCommand(name string) *Command {
	for _, c := range registry {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// Execute runs the program with the given arguments (without the program name) and returns the exit code.
func Execute(arg0 string, args []string, stdout, stderr io.Writer) int {
	ctx := &Context{Stdout: stdout, Stderr: stderr, Program: programName(arg0)}

	flags := newRootFlags()
	if err := flags.fs.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Fprintln(stderr, theme.Warning.Render("❌ Error: ")+err.Error())
			fmt.Fprintf(stderr, "Run '%s --help' for usage.\n", ctx.Program)
		}
		return 2
	}
	ctx.Config = flags.config()

	switch {
	case flags.help:
		return helpCommand.Run(ctx, nil)
	case flags.version:
		return versionCommand.Run(ctx, nil)
	}

	if flags.fs.NArg() > 0 {
		name := flags.fs.Arg(0)
		cmd := findCommand(name)
		if cmd == nil {
			fmt.Fprintln(stderr, theme.Warning.Render("❌ Error: ")+fmt.Sprintf("unknown command %q", name))
			fmt.Fprintf(stderr, "Run '%s --help' for usage.\n", ctx.Program)
			return 2
		}
		return cmd.Run(ctx, flags.fs.Args()[1:])
	}

	return runTUI(ctx)
}

// programName derives the name to show in messages from argv[0], so `gj --help` talks about gj.
func programName(arg0 string) string {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(arg0)), ".exe")
	if base == buildinfo.ShortName {
		return buildinfo.ShortName
	}
	return buildinfo.Name
}
