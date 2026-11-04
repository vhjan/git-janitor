package git

import (
	"os/exec"
	"testing"
)

func TestParseBranchLine(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		want   Branch
		wantOK bool
	}{
		{"plain branch", "main||||||2 weeks ago|||1700000000", Branch{Name: "main", CommitRelative: "2 weeks ago", CommitUnix: 1700000000}, true},
		{"gone upstream", "old|||[gone]|||3 months ago|||1600000000", Branch{Name: "old", UpstreamTrack: "[gone]", CommitRelative: "3 months ago", CommitUnix: 1600000000}, true},
		{"bad timestamp keeps branch", "x||||||now|||nope", Branch{Name: "x", CommitRelative: "now"}, true},
		{"too few fields", "x|||y", Branch{}, false},
		{"empty name", "||||||a|||1", Branch{}, false},
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

func TestUpstreamGone(t *testing.T) {
	if !(Branch{UpstreamTrack: "[gone]"}).UpstreamGone() {
		t.Error("expected [gone] to be detected")
	}
	if (Branch{UpstreamTrack: "[ahead 1]"}).UpstreamGone() {
		t.Error("[ahead 1] must not count as gone")
	}
}

// newTestRepo creates a throw-away repository with one commit on "main".
func newTestRepo(t *testing.T) *Repo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	r := &Repo{Dir: dir}
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		if _, err := r.run(args...); err != nil {
			t.Fatalf("setup git %v: %v", args, err)
		}
	}
	return r
}

func TestOpenRejectsNonRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if _, err := Open(t.TempDir()); err != ErrNotRepo {
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

	if _, err := r.run("branch", "feature"); err != nil {
		t.Fatal(err)
	}

	branches, err := r.LocalBranches()
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 2 {
		t.Fatalf("LocalBranches returned %d branches, want 2: %+v", len(branches), branches)
	}

	// "feature" points at the same commit as main, so it counts as merged.
	if _, ok := r.MergedBranches("main")["feature"]; !ok {
		t.Error("feature should be reported as merged into main")
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
