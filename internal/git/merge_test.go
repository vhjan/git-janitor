package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// divergedRepo builds: main and "other" both changed shared.txt differently after a common base (conflicting), and
// "clean" changed a different file (mergeable). main is checked out.
func divergedRepo(t *testing.T) *Repo {
	t.Helper()
	r := newTestRepo(t)
	commitFile(t, r, "shared.txt", "base\n", "base")

	mustGit(t, r, "checkout", "-b", "other")
	commitFile(t, r, "shared.txt", "from other\n", "other edits shared")
	mustGit(t, r, "checkout", "-b", "clean", "main")
	commitFile(t, r, "clean.txt", "clean\n", "clean adds file")

	mustGit(t, r, "checkout", "main")
	commitFile(t, r, "shared.txt", "from main\n", "main edits shared")
	return r
}

func TestCheckMerge(t *testing.T) {
	r := divergedRepo(t)
	requireMergeTree(t, r)

	t.Run("conflicting", func(t *testing.T) {
		c, err := r.CheckMerge("main", "other")
		if err != nil {
			t.Fatal(err)
		}
		if !c.Checked || len(c.Conflicts) != 1 || c.Conflicts[0] != "shared.txt" || c.UpToDate || c.FastForward {
			t.Errorf("unexpected check: %+v", c)
		}
		if c.Incoming != 1 || c.Local != 1 {
			t.Errorf("counts = %d incoming / %d local, want 1/1", c.Incoming, c.Local)
		}
	})

	t.Run("clean divergence", func(t *testing.T) {
		c, err := r.CheckMerge("main", "clean")
		if err != nil {
			t.Fatal(err)
		}
		if !c.Checked || len(c.Conflicts) != 0 || c.FastForward || c.UpToDate {
			t.Errorf("unexpected check: %+v", c)
		}
	})

	t.Run("fast-forward", func(t *testing.T) {
		mustGit(t, r, "branch", "ahead", "main")
		mustGit(t, r, "checkout", "ahead")
		commitFile(t, r, "ahead.txt", "x\n", "ahead")
		mustGit(t, r, "checkout", "main")
		c, _ := r.CheckMerge("main", "ahead")
		if !c.FastForward || c.UpToDate || c.Incoming != 1 {
			t.Errorf("unexpected check: %+v", c)
		}
	})

	t.Run("up to date", func(t *testing.T) {
		c, _ := r.CheckMerge("main", "main")
		if !c.UpToDate {
			t.Errorf("unexpected check: %+v", c)
		}
	})
}

func TestIsClean(t *testing.T) {
	r := newTestRepo(t)
	commitFile(t, r, "f.txt", "v1\n", "f")

	if clean, err := r.IsClean(); err != nil || !clean {
		t.Fatalf("fresh repo: clean=%v err=%v", clean, err)
	}

	// Untracked files must not count: they don't block a merge or rebase.
	if err := os.WriteFile(filepath.Join(r.Dir, "untracked.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if clean, _ := r.IsClean(); !clean {
		t.Error("untracked file made the tree dirty")
	}

	if err := os.WriteFile(filepath.Join(r.Dir, "f.txt"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if clean, _ := r.IsClean(); clean {
		t.Error("modified tracked file should make the tree dirty")
	}
}

func parentCount(t *testing.T, r *Repo, ref string) int {
	t.Helper()
	return len(strings.Fields(mustGit(t, r, "log", "-1", "--format=%P", ref)))
}

func TestMergeModes(t *testing.T) {
	t.Run("default fast-forwards when possible", func(t *testing.T) {
		r := newTestRepo(t)
		mustGit(t, r, "checkout", "-b", "feat")
		commitFile(t, r, "a.txt", "a\n", "a")
		mustGit(t, r, "checkout", "main")
		before := mustGit(t, r, "rev-parse", "main")

		if err := r.Merge("feat", MergeDefault); err != nil {
			t.Fatal(err)
		}
		if mustGit(t, r, "rev-parse", "main") != mustGit(t, r, "rev-parse", "feat") {
			t.Error("main should have fast-forwarded to feat")
		}
		if before == mustGit(t, r, "rev-parse", "main") {
			t.Error("main did not move")
		}
	})

	t.Run("no-ff always creates a merge commit", func(t *testing.T) {
		r := newTestRepo(t)
		mustGit(t, r, "checkout", "-b", "feat")
		commitFile(t, r, "a.txt", "a\n", "a")
		mustGit(t, r, "checkout", "main")

		if err := r.Merge("feat", MergeNoFF); err != nil {
			t.Fatal(err)
		}
		if n := parentCount(t, r, "main"); n != 2 {
			t.Errorf("merge commit has %d parents, want 2", n)
		}
	})

	t.Run("ff-only refuses diverged history", func(t *testing.T) {
		r := divergedRepo(t)
		err := r.Merge("clean", MergeFFOnly)
		if err == nil {
			t.Fatal("ff-only merge of diverged branches must fail")
		}
		var ce *ConflictError
		if errors.As(err, &ce) {
			t.Error("a refused fast-forward is not a conflict")
		}
		if clean, _ := r.IsClean(); !clean {
			t.Error("repository left dirty")
		}
	})

	t.Run("squash creates one single-parent commit", func(t *testing.T) {
		r := newTestRepo(t)
		mustGit(t, r, "checkout", "-b", "feat")
		commitFile(t, r, "a.txt", "a\n", "a")
		commitFile(t, r, "b.txt", "b\n", "b")
		mustGit(t, r, "checkout", "main")
		before := strings.TrimSpace(mustGit(t, r, "rev-parse", "main"))

		if err := r.Merge("feat", MergeSquash); err != nil {
			t.Fatal(err)
		}
		if n := parentCount(t, r, "main"); n != 1 {
			t.Errorf("squash commit has %d parents, want 1", n)
		}
		if got := strings.TrimSpace(mustGit(t, r, "rev-list", "--count", before+"..main")); got != "1" {
			t.Errorf("squash added %s commits, want 1", got)
		}
		for _, f := range []string{"a.txt", "b.txt"} {
			if _, err := os.Stat(filepath.Join(r.Dir, f)); err != nil {
				t.Errorf("%s missing after squash merge", f)
			}
		}
	})
}

func TestMergeConflictIsAborted(t *testing.T) {
	for _, mode := range []MergeMode{MergeDefault, MergeSquash} {
		r := divergedRepo(t)
		head := mustGit(t, r, "rev-parse", "HEAD")

		err := r.Merge("other", mode)
		var ce *ConflictError
		if !errors.As(err, &ce) {
			t.Fatalf("mode %d: err = %v, want *ConflictError", mode, err)
		}
		if len(ce.Files) != 1 || ce.Files[0] != "shared.txt" {
			t.Errorf("mode %d: conflict files = %v", mode, ce.Files)
		}
		if clean, _ := r.IsClean(); !clean {
			t.Errorf("mode %d: working tree not restored after abort", mode)
		}
		if mustGit(t, r, "rev-parse", "HEAD") != head {
			t.Errorf("mode %d: HEAD moved", mode)
		}
		if _, err := os.Stat(filepath.Join(r.Dir, ".git", "MERGE_HEAD")); err == nil {
			t.Errorf("mode %d: merge still in progress", mode)
		}
	}
}

func TestRebase(t *testing.T) {
	t.Run("replays commits onto the new base", func(t *testing.T) {
		r := newTestRepo(t)
		commitFile(t, r, "base.txt", "b\n", "base")
		mustGit(t, r, "checkout", "-b", "feat")
		commitFile(t, r, "feat.txt", "f\n", "feat work")
		mustGit(t, r, "checkout", "main")
		commitFile(t, r, "main.txt", "m\n", "main moves")
		mustGit(t, r, "checkout", "feat")

		if err := r.Rebase("main"); err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(mustGit(t, r, "rev-list", "--count", "main..feat")); got != "1" {
			t.Errorf("feat is %s commits ahead of main, want 1", got)
		}
		if got := strings.TrimSpace(mustGit(t, r, "rev-list", "--count", "feat..main")); got != "0" {
			t.Errorf("feat should contain main after rebase, behind by %s", got)
		}
	})

	t.Run("conflict is aborted and branch untouched", func(t *testing.T) {
		r := divergedRepo(t)
		mustGit(t, r, "checkout", "other")
		head := mustGit(t, r, "rev-parse", "HEAD")

		err := r.Rebase("main")
		var ce *ConflictError
		if !errors.As(err, &ce) || ce.Op != "rebase" {
			t.Fatalf("err = %v, want rebase *ConflictError", err)
		}
		if mustGit(t, r, "rev-parse", "HEAD") != head {
			t.Error("branch moved despite aborted rebase")
		}
		if r.rebaseInProgress() {
			t.Error("rebase still in progress")
		}
		if clean, _ := r.IsClean(); !clean {
			t.Error("working tree not restored")
		}
		if got := r.CurrentBranch(); got != "other" {
			t.Errorf("current branch = %q, want other", got)
		}
	})
}

func TestMergeAndRebaseCommandStrings(t *testing.T) {
	if got := MergeCommand("feat", MergeNoFF); got != "git merge --no-edit --no-ff feat" {
		t.Errorf("MergeCommand = %q", got)
	}
	if got := MergeCommand("feat", MergeDefault); got != "git merge --no-edit feat" {
		t.Errorf("MergeCommand = %q", got)
	}
	if got := RebaseCommand("origin/main"); got != "git rebase origin/main" {
		t.Errorf("RebaseCommand = %q", got)
	}
}
