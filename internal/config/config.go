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
Package config defines the runtime configuration shared by the CLI, the TUI and the domain logic.

Today the values only come from command-line flags. Keeping them in one struct means a config file (global or
per-repository) can later be layered in here without touching any consumer.
*/
package config

import "strings"

// DefaultStaleDays is the age after which a branch is flagged as stale.
const DefaultStaleDays = 30

// Config holds all user-tunable settings.
type Config struct {
	DryRun    bool     // Simulate destructive operations without performing them
	Protect   []string // Extra branch-name patterns that can never be deleted (supports * wildcards)
	StaleDays float64  // Days without commits before a branch counts as stale
}

// Default returns the configuration used when nothing is overridden.
func Default() Config {
	return Config{StaleDays: DefaultStaleDays}
}

// ParseList splits a comma-separated value into trimmed, non-empty items.
func ParseList(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
