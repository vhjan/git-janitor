package git

import (
	"errors"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseBranchLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   Branch
		wantOK bool
	}{
		{
			"plain branch",
			"main||||||||||||2 weeks ago|||1700000000|||",
			Branch{Name: "main", CommitRelative: "2 weeks ago", CommitUnix: 1700000000},
			true,
		},
		{
			"tracking branch checked out in a worktree",
			"feat|||[ahead 1, behind 2]|||origin|||refs/heads/feat|||1 day ago|||1700000000|||/tmp/wt",
			Branch{Name: "feat", UpstreamTrack: "[ahead 1, behind 2]", Remote: "origin", RemoteRef: "refs/heads/feat",
				CommitRelative: "1 day ago", CommitUnix: 1700000000, WorktreePath: "/tmp/wt"},
			true,
		},
		{
			"gone upstream",
			"old|||[gone]|||origin|||refs/heads/old|||3 months ago|||1600000000|||",
			Branch{Name: "old", UpstreamTrack: "[gone]", Remote: "origin", RemoteRef: "refs/heads/old",
				CommitRelative: "3 months ago", CommitUnix: 1600000000},
			true,
		},
		{
			"bad timestamp keeps branch",
			"x||||||||||||now|||nope|||",
			Branch{Name: "x", CommitRelative: "now"},
			true,
		},
		{"too few fields", "x|||y", Branch{}, false},
		{"empty name", "||||||||||||a|||1|||", Branch{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseBranchLine(tt.line)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("parseBranchLine(%q) = %+v, %v; want %+v, %v", tt.line, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestBranchHelpers(t *testing.T) {
	if !(Branch{UpstreamTrack: "[gone]"}).UpstreamGone() {
		t.Error("expected [gone] to be detected")
	}
	if (Branch{UpstreamTrack: "[ahead 1]"}).UpstreamGone() {
		t.Error("[ahead 1] must not count as gone")
	}

	for track, want := range map[string][2]int{
		"":                     {0, 0},
		"[ahead 3]":            {3, 0},
		"[behind 4]":           {0, 4},
		"[ahead 12, behind 5]": {12, 5},
		"[gone]":               {0, 0},
	} {
		a, b := Branch{UpstreamTrack: track}.AheadBehind()
		if a != want[0] || b != want[1] {
			t.Errorf("AheadBehind(%q) = %d,%d; want %d,%d", track, a, b, want[0], want[1])
		}
	}

	b := Branch{Remote: "origin", RemoteRef: "refs/heads/feature/x"}
	if !b.HasRemoteUpstream() || b.RemoteBranch() != "feature/x" {
		t.Errorf("remote upstream parsing wrong: %v %q", b.HasRemoteUpstream(), b.RemoteBranch())
	}
	if (Branch{Remote: "."}).HasRemoteUpstream() || (Branch{}).HasRemoteUpstream() {
		t.Error("local or missing upstream must not count as a remote upstream")
	}
}

func TestParsePruned(t *testing.T) {
	stderr := "From github.com:me/repo\n - [deleted]         (none)     -> origin/old-feature\n - [deleted]         (none)     -> origin/fix/x\n * [new branch]      other      -> origin/other\n"
	got := parsePruned(stderr)
	want := []string{"origin/old-feature", "origin/fix/x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePruned = %v, want %v", got, want)
	}
}

func TestOpenRejectsNonRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNotRepo) {
		t.Errorf("Open on empty dir = %v, want ErrNotRepo", err)
	}
}

func TestBranchLifecycle(t *testing.T) {
	r := newTestRepo(t)

	if got := r.CurrentBranch(); got != "main" {
		t.Errorf("CurrentBranch = %q, want main", got)
	}
	if got := r.DefaultBranch(); got != "main" {
		t.Errorf("DefaultBranch = %q, want main", got)
	}

	mustGit(t, r, "branch", "feature")

	if got := branchNames(t, r); got != "feature,main" {
		t.Fatalf("LocalBranches = %q, want feature,main", got)
	}

	// "feature" points at the same commit as main, so it counts as merged.
	if _, ok := r.MergedBranches("main")["feature"]; !ok {
		t.Error("feature should be reported as merged into main")
	}

	if sha := r.ShortSHA("feature"); len(sha) < 7 {
		t.Errorf("ShortSHA = %q, want an abbreviated commit id", sha)
	}
	if !r.RefExists("feature") || r.RefExists("nope") {
		t.Error("RefExists wrong")
	}

	if err := r.DeleteBranch("feature", true); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
	if err := r.DeleteBranch("feature", true); err == nil {
		t.Error("deleting a missing branch should fail")
	}
}

func TestObserverSeesCommands(t *testing.T) {
	r := newTestRepo(t)

	var seen [][]string
	r.Observer = func(args []string, err error) { seen = append(seen, args) }

	r.CurrentBranch()
	if len(seen) != 1 || seen[0][0] != "branch" {
		t.Errorf("observer saw %v, want one 'branch' invocation", seen)
	}
}

func TestErrorReason(t *testing.T) {
	r := newTestRepo(t)
	err := r.DeleteBranch("does-not-exist", true)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := Reason(err); !strings.Contains(got, "does-not-exist") {
		t.Errorf("Reason = %q, want git's own message mentioning the branch", got)
	}
}

func TestIsSquashMerged(t *testing.T) {
	r := newTestRepo(t)
	commitFile(t, r, "base.txt", "base\n", "base")

	// squashed: two commits on a branch, merged into main as ONE commit
	mustGit(t, r, "checkout", "-b", "squashed")
	commitFile(t, r, "s.txt", "one\n", "s1")
	commitFile(t, r, "s2.txt", "two\n", "s2")
	// rebased: two commits cherry-picked onto main individually
	mustGit(t, r, "checkout", "-b", "rebased", "main")
	commitFile(t, r, "r.txt", "one\n", "r1")
	commitFile(t, r, "r2.txt", "two\n", "r2")
	// open: work that never reached main
	mustGit(t, r, "checkout", "-b", "open", "main")
	commitFile(t, r, "o.txt", "wip\n", "o1")

	mustGit(t, r, "checkout", "main")
	mustGit(t, r, "merge", "--squash", "squashed")
	mustGit(t, r, "commit", "-m", "squash of squashed")
	mustGit(t, r, "cherry-pick", "rebased~1", "rebased")
	commitFile(t, r, "later.txt", "unrelated\n", "main moves on") // main keeps moving after the merges

	merged := r.MergedBranches("main")
	for _, name := range []string{"squashed", "rebased", "open"} {
		if _, ok := merged[name]; ok {
			t.Fatalf("test setup: git already reports %q as merged", name)
		}
	}

	for name, want := range map[string]bool{"squashed": true, "rebased": true, "open": false} {
		if got := r.IsSquashMerged(name, "main"); got != want {
			t.Errorf("IsSquashMerged(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestUnpushedCommits(t *testing.T) {
	r := newTestRepo(t)
	newRemote(t, r)

	mustGit(t, r, "checkout", "-b", "feature")
	commitFile(t, r, "a.txt", "a\n", "a")
	commitFile(t, r, "b.txt", "b\n", "b")

	if got := r.UnpushedCommits("feature", "main"); got != 2 {
		t.Errorf("before push: %d unpushed, want 2", got)
	}

	mustGit(t, r, "push", "-u", "origin", "feature")
	if got := r.UnpushedCommits("feature", "main"); got != 0 {
		t.Errorf("after push: %d unpushed, want 0", got)
	}

	commitFile(t, r, "c.txt", "c\n", "c")
	if got := r.UnpushedCommits("feature", "main"); got != 1 {
		t.Errorf("one new local commit: %d unpushed, want 1", got)
	}
}

func TestFetchPruneAndRemoteDelete(t *testing.T) {
	r := newTestRepo(t)
	remote := newRemote(t, r)

	mustGit(t, r, "checkout", "-b", "feature")
	commitFile(t, r, "f.txt", "f\n", "f")
	mustGit(t, r, "push", "-u", "origin", "feature")
	mustGit(t, r, "checkout", "main")

	bs, err := r.LocalBranches()
	if err != nil {
		t.Fatal(err)
	}
	var feature Branch
	for _, b := range bs {
		if b.Name == "feature" {
			feature = b
		}
	}
	if feature.Remote != "origin" || feature.RemoteBranch() != "feature" || feature.UpstreamGone() {
		t.Fatalf("unexpected tracking info: %+v", feature)
	}

	// Someone else deletes the branch on the remote; a fetch --prune must report it and mark ours [gone].
	other := cloneOf(t, remote)
	if err := other.DeleteRemoteBranch("origin", "feature"); err != nil {
		t.Fatalf("DeleteRemoteBranch: %v", err)
	}

	pruned, err := r.Fetch(true)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !reflect.DeepEqual(pruned, []string{"origin/feature"}) {
		t.Errorf("pruned = %v, want [origin/feature]", pruned)
	}

	bs, _ = r.LocalBranches()
	for _, b := range bs {
		if b.Name == "feature" && !b.UpstreamGone() {
			t.Error("feature should now be reported as gone")
		}
	}

	if pruned, err := r.Fetch(true); err != nil || len(pruned) != 0 {
		t.Errorf("second fetch: pruned=%v err=%v, want nothing", pruned, err)
	}
}

func TestWorktreePath(t *testing.T) {
	r := newTestRepo(t)
	mustGit(t, r, "branch", "elsewhere")
	wt := filepath.Join(t.TempDir(), "wt")
	mustGit(t, r, "worktree", "add", wt, "elsewhere")

	bs, err := r.LocalBranches()
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bs {
		switch b.Name {
		case "elsewhere":
			if b.WorktreePath == "" {
				t.Error("elsewhere is checked out in a worktree but WorktreePath is empty")
			}
		}
	}
	if err := r.DeleteBranch("elsewhere", true); err == nil {
		t.Error("git must refuse to delete a branch checked out in a worktree")
	}
}
