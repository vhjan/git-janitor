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
This file defines the global command-line flags. Their descriptions live next to the definitions and are reused
by the help output, so the two can never drift apart.
*/

package cli

import (
	"flag"
	"io"
	"sort"
	"strings"

	"github.com/vhjan/git-janitor/internal/config"
)

// rootFlags holds the parsed global flags.
type rootFlags struct {
	fs *flag.FlagSet

	help    bool
	version bool
	dryRun  bool
	protect string
	stale   float64
}

func newRootFlags() *rootFlags {
	f := &rootFlags{}
	fs := flag.NewFlagSet("git-janitor", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // Errors and help are rendered by Execute, not by the flag package
	f.fs = fs

	const (
		helpUsage    = "Shows this help menu"
		versionUsage = "Shows the version of Git Janitor"
	)
	fs.BoolVar(&f.help, "h", false, helpUsage)
	fs.BoolVar(&f.help, "help", false, helpUsage)
	fs.BoolVar(&f.version, "v", false, versionUsage)
	fs.BoolVar(&f.version, "version", false, versionUsage)
	fs.BoolVar(&f.dryRun, "dry-run", false, "Simulate deletion without actually removing branches")
	fs.StringVar(&f.protect, "protect", "", "Comma-separated list of branches to protect, supports * wildcards (e.g. 'qa,release-*')")
	fs.Float64Var(&f.stale, "stale-days", config.DefaultStaleDays, "Number of days before a branch is considered stale (default 30)")
	return f
}

// config converts the parsed flags into the shared Config.
func (f *rootFlags) config() config.Config {
	cfg := config.Default()
	cfg.DryRun = f.dryRun
	cfg.Protect = config.ParseList(f.protect)
	cfg.StaleDays = f.stale
	return cfg
}

// helpRows returns {flags, description} rows for the help output. Aliases sharing a description (-h/--help) are
// merged into one row.
func (f *rootFlags) helpRows() [][2]string {
	byUsage := map[string][]string{}
	var order []string
	f.fs.VisitAll(func(fl *flag.Flag) {
		prefix := "--"
		if len(fl.Name) == 1 {
			prefix = "-"
		}
		if _, seen := byUsage[fl.Usage]; !seen {
			order = append(order, fl.Usage)
		}
		byUsage[fl.Usage] = append(byUsage[fl.Usage], prefix+fl.Name)
	})

	rows := make([][2]string, 0, len(order))
	for _, usage := range order {
		names := byUsage[usage]
		sort.SliceStable(names, func(i, j int) bool { return len(names[i]) < len(names[j]) })
		rows = append(rows, [2]string{strings.Join(names, ", "), usage})
	}
	return rows
}
