package branches

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vhjan/git-janitor/internal/git"
)

// sh runs git in dir and fails the test on error.
func sh(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commit(t *testing.T, dir, file, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sh(t, dir, "add", file)
	sh(t, dir, "commit", "-m", "update "+file)
}

// newRepo creates a repository with a bare "origin" that already has main pushed.
func newRepo(t *testing.T) (*git.Repo, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	remote := filepath.Join(t.TempDir(), "remote.git")
	sh(t, dir, "init", "-b", "main")
	sh(t, dir, "config", "user.email", "test@example.com")
	sh(t, dir, "config", "user.name", "Test")
	sh(t, dir, "config", "commit.gpgsign", "false")
	commit(t, dir, "base.txt", "base\n")
	sh(t, dir, "init", "--bare", "-b", "main", remote)
	sh(t, dir, "remote", "add", "origin", remote)
	sh(t, dir, "push", "-u", "origin", "main")

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return repo, dir
}

func byName(snap Snapshot) map[string]Branch {
	m := map[string]Branch{}
	for _, b := range snap.Branches {
		m[b.Name] = b
	}
	return m
}

func TestLoadClassifiesBranches(t *testing.T) {
	repo, dir := newRepo(t)

	sh(t, dir, "branch", "merged") // same commit as main

	sh(t, dir, "checkout", "-b", "squashed")
	commit(t, dir, "s.txt", "s\n")
	commit(t, dir, "s2.txt", "s2\n")

	sh(t, dir, "checkout", "-b", "wip", "main") // local-only work
	commit(t, dir, "w.txt", "w\n")

	sh(t, dir, "checkout", "-b", "pushed", "main") // pushed work, then one more local commit
	commit(t, dir, "p.txt", "p\n")
	sh(t, dir, "push", "-u", "origin", "pushed")
	commit(t, dir, "p2.txt", "p2\n")

	sh(t, dir, "branch", "elsewhere")
	sh(t, dir, "worktree", "add", filepath.Join(t.TempDir(), "wt"), "elsewhere")

	sh(t, dir, "checkout", "main")
	sh(t, dir, "merge", "--squash", "squashed")
	sh(t, dir, "commit", "-m", "squash merge")

	snap, err := Load(repo, Options{StaleDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Current != "main" || snap.Default != "main" {
		t.Errorf("current/default = %q/%q, want main/main", snap.Current, snap.Default)
	}
	b := byName(snap)

	if !b["main"].Protected || b["main"].ProtectedReason == "" {
		t.Errorf("main should be protected with a reason: %+v", b["main"])
	}
	if got := b["merged"]; !got.Merged || got.Squashed || got.AtRisk() || !got.FullyMerged() {
		t.Errorf("merged: %+v", got)
	}
	if got := b["squashed"]; got.Merged || !got.Squashed || got.AtRisk() || !got.FullyMerged() {
		t.Errorf("squashed: %+v", got)
	}
	if got := b["wip"]; got.Unpushed != 1 || !got.AtRisk() || got.Remote != "" || got.RemoteDeletable() {
		t.Errorf("wip: %+v", got)
	}
	if got := b["pushed"]; got.Unpushed != 1 || got.Ahead != 1 || got.Remote != "origin" || got.RemoteBranch != "pushed" || !got.RemoteDeletable() {
		t.Errorf("pushed: %+v", got)
	}
	if got := b["elsewhere"]; !got.Protected || !strings.Contains(got.ProtectedReason, "worktree") || got.Worktree == "" {
		t.Errorf("elsewhere should be protected because it is checked out in a worktree: %+v", got)
	}
}

func TestDelete(t *testing.T) {
	repo, dir := newRepo(t)
	sh(t, dir, "checkout", "-b", "feature")
	commit(t, dir, "f.txt", "f\n")
	sh(t, dir, "push", "-u", "origin", "feature")
	sh(t, dir, "checkout", "-b", "local-only", "main")
	sh(t, dir, "checkout", "main")

	snap, err := Load(repo, Options{StaleDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	b := byName(snap)

	t.Run("dry run changes nothing", func(t *testing.T) {
		res := Delete(repo, []Branch{b["feature"]}, DeleteOptions{DryRun: true, DeleteRemote: true})
		if len(res.Deleted) != 1 || !res.Deleted[0].RemoteDeleted || res.Deleted[0].SHA == "" {
			t.Errorf("dry run result: %+v", res)
		}
		if !repo.RefExists("feature") {
			t.Error("dry run deleted the branch")
		}
	})

	t.Run("local only keeps the remote branch", func(t *testing.T) {
		res := Delete(repo, []Branch{b["local-only"]}, DeleteOptions{DeleteRemote: true})
		if len(res.Errors) != 0 || len(res.Deleted) != 1 || res.Deleted[0].RemoteDeleted {
			t.Errorf("result: %+v", res)
		}
		if repo.RefExists("local-only") {
			t.Error("branch still exists")
		}
	})

	t.Run("also deletes the remote branch on request", func(t *testing.T) {
		res := Delete(repo, []Branch{b["feature"]}, DeleteOptions{DeleteRemote: true})
		if len(res.Errors) != 0 || len(res.Deleted) != 1 || !res.Deleted[0].RemoteDeleted {
			t.Fatalf("result: %+v", res)
		}
		if repo.RefExists("feature") {
			t.Error("local branch still exists")
		}
		if _, err := repo.Fetch(true); err != nil {
			t.Fatal(err)
		}
		if repo.RefExists("origin/feature") {
			t.Error("remote branch still exists after remote delete")
		}
	})

	t.Run("reports failures instead of panicking", func(t *testing.T) {
		res := Delete(repo, []Branch{{Name: "ghost"}}, DeleteOptions{})
		if len(res.Deleted) != 0 || len(res.Errors) != 1 || !strings.Contains(res.Errors[0], "ghost") {
			t.Errorf("result: %+v", res)
		}
	})
}

func TestMergeAndRebasePreconditions(t *testing.T) {
	repo, dir := newRepo(t)
	sh(t, dir, "checkout", "-b", "feature")
	commit(t, dir, "f.txt", "f\n")
	sh(t, dir, "checkout", "main")

	t.Run("dirty tree is refused", func(t *testing.T) {
		if err := os.WriteFile(filepath.Join(dir, "base.txt"), []byte("changed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		defer sh(t, dir, "checkout", "--", "base.txt")

		if err := Merge(repo, "feature", git.MergeDefault); !errors.Is(err, ErrDirtyTree) {
			t.Errorf("Merge on dirty tree = %v, want ErrDirtyTree", err)
		}
		if err := Rebase(repo, "feature"); !errors.Is(err, ErrDirtyTree) {
			t.Errorf("Rebase on dirty tree = %v, want ErrDirtyTree", err)
		}
	})

	t.Run("up to date is reported, not run", func(t *testing.T) {
		if err := Merge(repo, "main", git.MergeNoFF); !errors.Is(err, ErrUpToDate) {
			t.Errorf("Merge of itself = %v, want ErrUpToDate", err)
		}
	})

	t.Run("detached HEAD is refused", func(t *testing.T) {
		sh(t, dir, "checkout", "--detach")
		defer sh(t, dir, "checkout", "main")
		if err := Merge(repo, "feature", git.MergeDefault); !errors.Is(err, ErrDetached) {
			t.Errorf("Merge on detached HEAD = %v, want ErrDetached", err)
		}
	})

	t.Run("merge then update target", func(t *testing.T) {
		if err := Merge(repo, "feature", git.MergeFFOnly); err != nil {
			t.Fatalf("fast-forward merge: %v", err)
		}
		if got := UpdateTarget(repo, "main"); got != "origin/main" {
			t.Errorf("UpdateTarget = %q, want origin/main (it exists after the initial push)", got)
		}
		if got := UpdateTarget(repo, "nonexistent"); got != "nonexistent" {
			t.Errorf("UpdateTarget fallback = %q, want the local name", got)
		}
	})
}

func TestProtectionReasons(t *testing.T) {
	cases := []struct {
		branch, wantSub string
	}{
		{"feature", "current branch"},
		{"main", "default branch"},
		{"dev", "built-in"},
		{"release-1", "release-*"},
		{"other", ""},
	}
	for _, c := range cases {
		got := ProtectionReason(c.branch, "feature", "main", []string{"release-*"})
		if c.wantSub == "" && got != "" || !strings.Contains(got, c.wantSub) {
			t.Errorf("ProtectionReason(%q) = %q, want it to mention %q", c.branch, got, c.wantSub)
		}
	}
}

func TestRemoteBranchIsNotOfferedWhenRemoteIsAhead(t *testing.T) {
	b := Branch{Name: "f", Remote: "origin", RemoteBranch: "f"}
	if !b.RemoteDeletable() {
		t.Fatal("a branch in sync with its remote should be deletable there")
	}
	b.Behind = 3 // a teammate pushed commits we don't have
	if b.RemoteDeletable() {
		t.Error("deleting the remote branch would destroy commits that exist only there")
	}
}

func TestUpdateTargetFollowsConfiguredRemote(t *testing.T) {
	repo, dir := newRepo(t)
	// Rename the remote: "origin/main" no longer exists, but main still tracks the (renamed) remote.
	sh(t, dir, "remote", "rename", "origin", "upstream")

	if got := UpdateTarget(repo, "main"); got != "upstream/main" {
		t.Errorf("UpdateTarget = %q, want upstream/main", got)
	}
}

// When the remote's default branch moves ahead and the user has not updated their local one, Load must say so:
// the "merged" checks keep using the local default branch.
func TestLoadReportsStaleLocalDefaultBranch(t *testing.T) {
	repo, dir := newRepo(t)

	snap, err := Load(repo, Options{StaleDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if snap.DefaultUpstream != "origin/main" || snap.DefaultBehind != 0 {
		t.Errorf("in sync: upstream=%q behind=%d, want origin/main and 0", snap.DefaultUpstream, snap.DefaultBehind)
	}

	// A teammate merges something into main on the remote.
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "teammate")
	sh(t, t.TempDir(), "clone", strings.TrimSpace(string(out)), other)
	sh(t, other, "config", "user.email", "mate@example.com")
	sh(t, other, "config", "user.name", "Mate")
	sh(t, other, "config", "commit.gpgsign", "false")
	commit(t, other, "merged-pr.txt", "pr\n")
	commit(t, other, "merged-pr2.txt", "pr2\n")
	sh(t, other, "push", "origin", "main")

	if _, err := repo.Fetch(true); err != nil {
		t.Fatal(err)
	}
	snap, err = Load(repo, Options{StaleDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if snap.DefaultBehind != 2 || snap.DefaultUpstream != "origin/main" {
		t.Errorf("after the remote moved: upstream=%q behind=%d, want origin/main and 2", snap.DefaultUpstream, snap.DefaultBehind)
	}

	// A repo without any upstream for main has nothing to compare with, so nothing to warn about.
	sh(t, dir, "branch", "--unset-upstream", "main")
	snap, _ = Load(repo, Options{StaleDays: 30})
	if snap.DefaultUpstream != "" || snap.DefaultBehind != 0 {
		t.Errorf("no upstream: upstream=%q behind=%d, want empty and 0", snap.DefaultUpstream, snap.DefaultBehind)
	}
}

// A branch created with `git checkout -b feature origin/main` has origin/main as its upstream. "Also delete the
// remote branch" must never turn that into `git push origin --delete main`.
func TestRemoteDeletionNeverTouchesSomeoneElsesBranch(t *testing.T) {
	repo, dir := newRepo(t)

	sh(t, dir, "checkout", "-b", "tracks-default", "origin/main") // upstream is the default branch
	commit(t, dir, "a.txt", "a\n")
	sh(t, dir, "checkout", "main")

	sh(t, dir, "checkout", "-b", "topic", "main")
	commit(t, dir, "t.txt", "t\n")
	sh(t, dir, "push", "-u", "origin", "topic")
	sh(t, dir, "checkout", "-b", "topic-copy", "topic") // shares origin/topic as its upstream
	sh(t, dir, "branch", "--set-upstream-to=origin/topic", "topic-copy")
	sh(t, dir, "checkout", "main")

	snap, err := Load(repo, Options{StaleDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	b := byName(snap)

	for name, wantReason := range map[string]string{
		"tracks-default": "default branch",
		"topic":          "shared",
		"topic-copy":     "shared",
	} {
		got := b[name]
		if got.RemoteDeletable() || !strings.Contains(got.RemoteKeep, wantReason) {
			t.Errorf("%s: RemoteDeletable=%v RemoteKeep=%q, want it kept because %q", name, got.RemoteDeletable(), got.RemoteKeep, wantReason)
		}
	}

	// Even if a caller asks for remote deletion, nothing is pushed.
	res := Delete(repo, []Branch{b["tracks-default"]}, DeleteOptions{DeleteRemote: true})
	if len(res.Errors) != 0 || len(res.Deleted) != 1 || res.Deleted[0].RemoteDeleted {
		t.Fatalf("result: %+v", res)
	}
	if _, err := repo.Fetch(true); err != nil {
		t.Fatal(err)
	}
	if !repo.RefExists("origin/main") {
		t.Fatal("origin/main was deleted on the remote")
	}
}

func TestProtectedUpstreamsAreKept(t *testing.T) {
	repo, dir := newRepo(t)
	sh(t, dir, "checkout", "-b", "release-1", "main")
	commit(t, dir, "r.txt", "r\n")
	sh(t, dir, "push", "-u", "origin", "release-1")
	sh(t, dir, "checkout", "-b", "mine", "release-1")
	sh(t, dir, "branch", "--set-upstream-to=origin/release-1", "mine")
	sh(t, dir, "checkout", "main")

	snap, err := Load(repo, Options{StaleDays: 30, ProtectPatterns: []string{"release-*"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := byName(snap)["mine"]; got.RemoteDeletable() || !strings.Contains(got.RemoteKeep, "protected") {
		t.Errorf("mine: RemoteDeletable=%v RemoteKeep=%q, want its protected upstream kept", got.RemoteDeletable(), got.RemoteKeep)
	}
}
