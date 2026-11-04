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
This file implements the git version check. Git Janitor relies on for-each-ref atoms such as %(worktreepath), which
appeared in git 2.23, so older versions are rejected up front with a clear message instead of failing later.
*/

package git

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a git release, reduced to the parts that matter for feature checks.
type Version struct {
	Major, Minor int
}

// MinVersion is the oldest git that Git Janitor supports.
var MinVersion = Version{Major: 2, Minor: 23}

func (v Version) String() string { return fmt.Sprintf("%d.%d", v.Major, v.Minor) }

// Less reports whether v is older than o.
func (v Version) Less(o Version) bool {
	return v.Major < o.Major || (v.Major == o.Major && v.Minor < o.Minor)
}

// TooOldError is returned by Open when the installed git is older than MinVersion.
type TooOldError struct {
	Found Version
}

func (e *TooOldError) Error() string {
	return fmt.Sprintf("git %s is too old: Git Janitor needs git %s or newer", e.Found, MinVersion)
}

// ParseVersion parses the output of `git --version`, e.g. "git version 2.55.0.windows.4" or
// "git version 2.39.3 (Apple Git-145)".
func ParseVersion(s string) (Version, error) {
	fields := strings.Fields(s)
	if len(fields) < 3 || fields[0] != "git" || fields[1] != "version" {
		return Version{}, fmt.Errorf("unrecognised git version output %q", strings.TrimSpace(s))
	}
	parts := strings.Split(fields[2], ".")
	if len(parts) < 2 {
		return Version{}, fmt.Errorf("unrecognised git version %q", fields[2])
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return Version{}, fmt.Errorf("unrecognised git version %q", fields[2])
	}
	return Version{Major: major, Minor: minor}, nil
}

// Version returns the installed git's version.
func (r *Repo) Version() (Version, error) {
	out, err := r.run("--version")
	if err != nil {
		return Version{}, err
	}
	return ParseVersion(out)
}
