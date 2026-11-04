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
This file contains the branch-related git operations: querying branches, the default branch and merge state,
fetching, and deleting.
*/

package git

import (
	"strconv"
	"strings"
)

// Fallback names for the default branch when origin/HEAD is unknown.
const (
	BranchMain   = "main"
	BranchMaster = "master"
)

// Branch is the raw information git reports about a local branch.
type Branch struct {
	Name           string // Short branch name
	UpstreamTrack  string // Output of %(upstream:track), e.g. "[ahead 1]" or "[gone]"
	CommitRelative string // Human-readable age of the last commit (e.g. "2 weeks ago")
	CommitUnix     int64  // Unix timestamp of the last commit
}

// UpstreamGone reports whether the branch tracks a remote branch that no longer exists.
func (b Branch) UpstreamGone() bool {
	return strings.Contains(b.UpstreamTrack, "[gone]")
}

// CurrentBranch returns the checked-out branch, or "" when HEAD is detached or the lookup fails.
func (r *Repo) CurrentBranch() string {
	out, err := r.run("branch", "--show-current")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// DefaultBranch identifies the repository's primary branch. It checks origin/HEAD first, then falls back to a
// local "main", and finally assumes "master".
func (r *Repo) DefaultBranch() string {
	if out, err := r.run("rev-parse", "--abbrev-ref", "origin/HEAD"); err == nil {
		ref := strings.TrimSpace(out)
		if parts := strings.Split(ref, "/"); len(parts) == 2 {
			return parts[1]
		}
	}

	if _, err := r.run("show-ref", "--verify", "--quiet", "refs/heads/"+BranchMain); err == nil {
		return BranchMain
	}

	return BranchMaster
}

// MergedBranches returns the set of local branches already merged into target.
func (r *Repo) MergedBranches(target string) map[string]struct{} {
	merged := make(map[string]struct{})
	out, err := r.run("branch", "--format=%(refname:short)", "--merged", target)
	if err != nil {
		return merged
	}
	for _, name := range splitLines(out) {
		merged[name] = struct{}{}
	}
	return merged
}

// Fetch updates remote information. With prune set, remote-tracking branches that no longer exist on the remote
// are removed, which makes local branches whose upstream vanished show up as "[gone]".
func (r *Repo) Fetch(prune bool) error {
	args := []string{"fetch"}
	if prune {
		args = append(args, "--prune")
	}
	_, err := r.run(args...)
	return err
}

// branchFormat is the for-each-ref format parsed by parseBranchLine. It extracts everything in a single call.
const (
	branchSeparator = "|||"
	branchFormat    = "%(refname:short)" + branchSeparator + "%(upstream:track)" + branchSeparator +
		"%(committerdate:relative)" + branchSeparator + "%(committerdate:unix)"
)

// LocalBranches lists all local branches (refs/heads).
func (r *Repo) LocalBranches() ([]Branch, error) {
	out, err := r.run("for-each-ref", "--format="+branchFormat, "refs/heads/")
	if err != nil {
		return nil, err
	}

	var branches []Branch
	for _, line := range splitLines(out) {
		if b, ok := parseBranchLine(line); ok {
			branches = append(branches, b)
		}
	}
	return branches, nil
}

// parseBranchLine parses one line produced with branchFormat.
func parseBranchLine(line string) (Branch, bool) {
	parts := strings.Split(line, branchSeparator)
	if len(parts) != 4 {
		return Branch{}, false
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return Branch{}, false
	}

	b := Branch{
		Name:           name,
		UpstreamTrack:  parts[1],
		CommitRelative: parts[2],
	}
	if u, err := strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64); err == nil {
		b.CommitUnix = u
	}
	return b, true
}

// DeleteBranch removes a local branch. With force it uses -D (delete even if unmerged), otherwise -d.
func (r *Repo) DeleteBranch(name string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := r.run("branch", flag, name)
	return err
}
