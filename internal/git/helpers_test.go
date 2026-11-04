package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mustGit runs a git command in the repo and fails the test if it errors. It returns stdout.
func mustGit(t *testing.T, r *Repo, args ...string) string {
	t.Helper()
	out, err := r.run(args...)
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return out
}

// newTestRepo creates a throw-away repository with one empty commit on "main".
func newTestRepo(t *testing.T) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	r := &Repo{Dir: t.TempDir()}
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		mustGit(t, r, args...)
	}
	return r
}

// commitFile writes a file in the repo's working tree and commits it on the current branch.
func commitFile(t *testing.T, r *Repo, name, content, msg string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.Dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, r, "add", name)
	mustGit(t, r, "commit", "-m", msg)
}

// newRemote creates a bare repository, wires it up as "origin" of r and pushes main to it.
func newRemote(t *testing.T, r *Repo) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", "-b", "main", dir).CombinedOutput(); err != nil {
		t.Fatalf("init bare: %v\n%s", err, out)
	}
	mustGit(t, r, "remote", "add", "origin", dir)
	mustGit(t, r, "push", "-u", "origin", "main")
	return dir
}

// cloneOf clones a remote into a new directory with a usable identity.
func cloneOf(t *testing.T, remote string) *Repo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", remote, dir).CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
	c := &Repo{Dir: dir}
	mustGit(t, c, "config", "user.email", "test@example.com")
	mustGit(t, c, "config", "user.name", "Test")
	mustGit(t, c, "config", "commit.gpgsign", "false")
	return c
}

// requireMergeTree skips the test on git versions that predate `merge-tree --write-tree` (2.38).
func requireMergeTree(t *testing.T, r *Repo) {
	t.Helper()
	if _, err := r.run("merge-tree", "--write-tree", "HEAD", "HEAD"); err != nil {
		t.Skipf("git too old for merge-tree --write-tree: %v", err)
	}
}

// branchNames returns the sorted-by-git names of all local branches.
func branchNames(t *testing.T, r *Repo) string {
	t.Helper()
	bs, err := r.LocalBranches()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, b := range bs {
		names = append(names, b.Name)
	}
	return strings.Join(names, ",")
}
