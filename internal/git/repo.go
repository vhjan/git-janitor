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
Package git is a thin wrapper around the git executable. It is the only package in the project that spawns git
processes, so every future feature (merge, rebase, push, ...) goes through the single run helper below. That gives
us one place to add logging, dry-run handling, timeouts or multi-remote support.
*/
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// ErrNotRepo is returned by Open when the directory is not inside a Git work tree.
var ErrNotRepo = errors.New("not inside a git repository")

// Error is returned when a git command exits unsuccessfully. It keeps the captured stderr so callers can surface
// the real reason to the user.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	msg := "git " + strings.Join(e.Args, " ") + ": " + e.Err.Error()
	if e.Stderr != "" {
		msg += ": " + e.Stderr
	}
	return msg
}

func (e *Error) Unwrap() error { return e.Err }

// Reason returns the most useful human-readable explanation of the failure: git's own message when it printed
// one, otherwise the underlying error.
func (e *Error) Reason() string {
	if e.Stderr != "" {
		return e.Stderr
	}
	return e.Err.Error()
}

// Reason extracts a short explanation from any error returned by this package.
func Reason(err error) string {
	var ge *Error
	if errors.As(err, &ge) {
		return ge.Reason()
	}
	return err.Error()
}

// exitCode returns the process exit code behind err, or -1 when err is not an exit error.
func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// Repo represents a Git repository on disk. The zero value operates on the current working directory.
type Repo struct {
	// Dir is the working directory for all git commands. Empty means the process' current directory.
	Dir string

	// Observer, when set, is called after every git invocation. It is the hook for a future operation-log panel.
	Observer func(args []string, err error)

	observerMu sync.Mutex // serialises Observer calls: branches.Load runs git from several goroutines
}

// Open verifies that git is new enough and that dir (or the current directory when empty) is inside a work tree,
// and returns a Repo for it.
func Open(dir string) (*Repo, error) {
	r := &Repo{Dir: dir}

	v, err := r.Version()
	if err != nil {
		return nil, fmt.Errorf("could not run git (is it installed and on your PATH?): %w", err)
	}
	if v.Less(MinVersion) {
		return nil, &TooOldError{Found: v}
	}

	if _, err := r.run("rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, ErrNotRepo
	}
	return r, nil
}

// run executes `git <args...>` and returns its stdout. On failure the returned error is an *Error.
func (r *Repo) run(args ...string) (string, error) {
	out, _, err := r.runIO(args...)
	return out, err
}

// runIO is like run but also returns stderr, which some commands (e.g. fetch) use for their progress report.
//
// Git is started non-interactively (it must never block the TUI waiting for credentials or an editor) and with the
// C locale, because some output (e.g. "[deleted]" in fetch --prune) is parsed and git translates it otherwise.
func (r *Repo) runIO(args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_EDITOR=true", "LC_ALL=C")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	if err = cmd.Run(); err != nil {
		err = &Error{Args: args, Stderr: strings.TrimSpace(errBuf.String()), Err: err}
	}
	if r.Observer != nil {
		r.observerMu.Lock()
		defer r.observerMu.Unlock()
		r.Observer(args, err)
	}
	return outBuf.String(), errBuf.String(), err
}

// splitLines splits command output into trimmed, non-empty lines (tolerating Windows line endings).
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var lines []string
	for l := range strings.SplitSeq(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
