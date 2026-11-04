package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vhjan/git-janitor/internal/branches"
	"github.com/vhjan/git-janitor/internal/config"
	"github.com/vhjan/git-janitor/internal/git"
)

// ---- helpers ---------------------------------------------------------------------------------------------------

// testModel returns a model in the list state with the given branches. "main" is the default branch and "cur" is
// the checked-out branch unless a branch list says otherwise.
func testModel(items ...branches.Branch) Model {
	m := New(config.Default(), nil)
	m.state = stateList
	m.current, m.defaultBranch = "cur", "main"
	m.list.SetSize(80, 24)
	li := make([]list.Item, len(items))
	for i, b := range items {
		li[i] = item{Branch: b}
	}
	m.list.SetItems(li)
	return m
}

func keyMsg(k string) tea.KeyMsg {
	switch k {
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
}

// pressCmd sends a key and returns the new model and the command the model asked for.
func pressCmd(m Model, k string) (Model, tea.Cmd) {
	next, cmd := m.Update(keyMsg(k))
	return next.(Model), cmd
}

func press(m Model, k string) Model {
	next, _ := pressCmd(m, k)
	return next
}

func feed(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

// runCmd executes a command (flattening batches) and returns the messages it produced. Only use it for commands
// that do not contain timers (spinner ticks resolve immediately; list status messages do not).
func runCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, runCmd(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func selected(m Model) map[string]bool {
	out := map[string]bool{}
	for _, li := range m.list.Items() {
		if i := li.(item); i.selected {
			out[i.Name] = true
		}
	}
	return out
}

func names(m Model) string {
	var out []string
	for _, li := range m.list.Items() {
		out = append(out, li.(item).Name)
	}
	return strings.Join(out, ",")
}

// ---- keymap ----------------------------------------------------------------------------------------------------

func TestKeyMapHasNoDuplicateKeys(t *testing.T) {
	k := defaultKeyMap()
	groups := map[string][]key.Binding{
		"list":    k.listBindings(),
		"confirm": {k.Yes, k.No, k.ToggleRemote},
		"op":      {k.OpUp, k.OpDown, k.OpRun, k.OpCancel},
	}
	for group, bindings := range groups {
		seen := map[string]string{}
		for _, b := range bindings {
			for _, bound := range b.Keys() {
				if other, dup := seen[bound]; dup {
					t.Errorf("%s view: key %q is bound to both %q and %q", group, bound, other, b.Help().Desc)
				}
				seen[bound] = b.Help().Desc
			}
		}
	}
}

func TestHelpRowsCoverAllListBindings(t *testing.T) {
	want := 2 + len(defaultKeyMap().listBindings()) // 2 navigation rows
	if got := len(HelpRows()); got != want {
		t.Errorf("HelpRows returned %d rows, want %d", got, want)
	}
	text := ""
	for _, r := range HelpRows() {
		text += r[0] + " " + r[1] + "\n"
	}
	for _, want := range []string{"FETCH", "MERGE", "REBASE", "UPDATE"} {
		if !strings.Contains(text, want) {
			t.Errorf("help is missing the %s action", want)
		}
	}
}

func TestFooterFitsTwoLines(t *testing.T) {
	f := defaultKeyMap().footer("Alphabetical")
	if got := strings.Count(f, "\n") + 1; got != footerLines {
		t.Errorf("footer has %d lines, footerLines says %d", got, footerLines)
	}
}

// ---- selection -------------------------------------------------------------------------------------------------

func TestSelectionKeys(t *testing.T) {
	m := testModel(
		branches.Branch{Name: "main", Protected: true},
		branches.Branch{Name: "merged", Merged: true},
		branches.Branch{Name: "squashed", Squashed: true},
		branches.Branch{Name: "gone", Gone: true},
		branches.Branch{Name: "stale", Stale: true},
		branches.Branch{Name: "plain"},
	)

	cases := []struct {
		key  string
		want []string
	}{
		{"m", []string{"merged", "squashed"}}, // squash/rebase merges count as merged
		{"g", []string{"gone"}},
		{"s", []string{"stale"}},
		{"a", []string{"merged", "squashed", "gone", "stale", "plain"}}, // never the protected branch
	}
	for _, c := range cases {
		got := selected(press(m, c.key))
		if len(got) != len(c.want) {
			t.Errorf("key %q selected %v, want %v", c.key, got, c.want)
			continue
		}
		for _, name := range c.want {
			if !got[name] {
				t.Errorf("key %q did not select %q (got %v)", c.key, name, got)
			}
		}
	}

	if got := selected(press(press(m, "a"), "c")); len(got) != 0 {
		t.Errorf("clear left selections behind: %v", got)
	}
}

func TestEnterOnlyConfirmsWithSelection(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"})

	if press(m, "enter").state != stateList {
		t.Error("enter without a selection must stay in the list view")
	}
	if press(press(m, "a"), "enter").state != stateConfirm {
		t.Error("enter with a selection must open the confirm view")
	}
}

// Regression test: toggling while a filter is applied must affect the highlighted branch, not the branch that
// happens to sit at the same index in the unfiltered list.
func TestToggleRespectsActiveFilter(t *testing.T) {
	m := testModel(
		branches.Branch{Name: "alpha"},
		branches.Branch{Name: "beta"},
		branches.Branch{Name: "gamma"},
	)
	m.list.SetFilterText("gamma")
	m.list.SetFilterState(list.FilterApplied)

	if sel, _ := m.list.SelectedItem().(item); sel.Name != "gamma" {
		t.Fatalf("test setup: highlighted %q, want gamma", sel.Name)
	}

	toggled := press(m, " ")

	got := selected(toggled)
	if len(got) != 1 || !got["gamma"] {
		t.Errorf("toggle selected %v, want only gamma", got)
	}
	// The buggy version overwrote the item at the filtered index (alpha) with a copy of gamma.
	if names(toggled) != "alpha,beta,gamma" {
		t.Errorf("toggle corrupted the item list: %v", names(toggled))
	}
}

func TestSortModeCycles(t *testing.T) {
	mode := sortAlphabetical
	for i := 0; i < int(sortModeCount); i++ {
		mode = mode.next()
	}
	if mode != sortAlphabetical {
		t.Errorf("cycling through all modes ended on %v, want alphabetical", mode)
	}
}

func TestReloadKeepsSelectionAndSort(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"}, branches.Branch{Name: "b"}, branches.Branch{Name: "c"})
	m = press(press(m, "a"), "o") // select all, then sort by latest commit

	reloaded := []list.Item{
		item{Branch: branches.Branch{Name: "a", LastCommitUnix: 1}},
		item{Branch: branches.Branch{Name: "c", LastCommitUnix: 3}},
		item{Branch: branches.Branch{Name: "new", LastCommitUnix: 9}},
		item{Branch: branches.Branch{Name: "main", Protected: true, LastCommitUnix: 5}},
	}
	m, _ = feed(m, branchesLoadedMsg{items: reloaded, current: "x", defaultBranch: "main"})

	if m.state != stateList || m.current != "x" {
		t.Fatalf("state=%v current=%q after reload", m.state, m.current)
	}
	if got := names(m); got != "new,main,c,a" {
		t.Errorf("order after reload = %s, want newest first: new,main,c,a", got)
	}
	got := selected(m)
	if len(got) != 2 || !got["a"] || !got["c"] {
		t.Errorf("selection after reload = %v, want a and c kept (b is gone, new was not selected)", got)
	}
}

// ---- rows ------------------------------------------------------------------------------------------------------

func TestItemDescriptions(t *testing.T) {
	cases := []struct {
		name string
		b    branches.Branch
		want []string
	}{
		{"merged", branches.Branch{Merged: true, Remote: "origin"}, []string{"Merged"}},
		{"squashed", branches.Branch{Squashed: true, Remote: "origin"}, []string{"squash"}},
		{"local only", branches.Branch{}, []string{"Local only"}},
		{"gone", branches.Branch{Gone: true, Remote: "origin"}, []string{"Gone"}},
		{"at risk", branches.Branch{Unpushed: 3}, []string{"⚠ 3 unpushed commits"}},
		{"singular", branches.Branch{Unpushed: 1}, []string{"⚠ 1 unpushed commit"}},
		{"sync", branches.Branch{Remote: "origin", Ahead: 2, Behind: 1}, []string{"↑2", "↓1"}},
		{"protected", branches.Branch{Protected: true, ProtectedReason: "checked out in worktree /x"}, []string{"Protected: checked out in worktree /x"}},
	}
	for _, c := range cases {
		desc := item{Branch: c.b}.Description()
		for _, want := range c.want {
			if !strings.Contains(desc, want) {
				t.Errorf("%s: description %q is missing %q", c.name, desc, want)
			}
		}
	}

	// A fully merged branch with leftover unpushed commits is not at risk, so no warning.
	if d := (item{Branch: branches.Branch{Merged: true, Remote: "origin", Unpushed: 2}}).Description(); strings.Contains(d, "⚠") {
		t.Errorf("merged branch must not warn: %q", d)
	}
}

// ---- delete dialog ---------------------------------------------------------------------------------------------

func TestConfirmRequiresCapitalYWhenCommitsWouldBeLost(t *testing.T) {
	m := testModel(
		branches.Branch{Name: "risky", Unpushed: 4},
		branches.Branch{Name: "safe", Merged: true},
	)
	m = press(press(m, "a"), "enter")
	if m.state != stateConfirm {
		t.Fatal("expected confirm view")
	}
	if view := m.View(); !strings.Contains(view, "risky") || !strings.Contains(view, "4 unpushed commits") {
		t.Errorf("dialog should name the at-risk branch and its commit count:\n%s", view)
	}
	if view := m.View(); !strings.Contains(view, "Delete 2 branches?") {
		t.Errorf("dialog should say 'Delete 2 branches?' (not 'branchs'):\n%s", view)
	}

	m, cmd := pressCmd(m, "y")
	if m.state != stateConfirm || cmd != nil {
		t.Fatal("lowercase y must not delete branches that have unpushed commits")
	}

	m, cmd = pressCmd(m, "Y")
	if m.state != stateLoading || cmd == nil {
		t.Errorf("capital Y should start the deletion (state=%v, cmd=%v)", m.state, cmd != nil)
	}
}

func TestConfirmLowercaseYIsEnoughForSafeBranches(t *testing.T) {
	m := testModel(branches.Branch{Name: "safe", Merged: true})
	m = press(press(m, "a"), "enter")
	if !strings.Contains(m.View(), "Delete 1 branch?") {
		t.Errorf("dialog should say 'Delete 1 branch?':\n%s", m.View())
	}

	m, cmd := pressCmd(m, "y")
	if m.state != stateLoading || cmd == nil {
		t.Errorf("y should delete merged branches (state=%v, cmd=%v)", m.state, cmd != nil)
	}
}

func TestConfirmRemoteToggle(t *testing.T) {
	m := testModel(
		branches.Branch{Name: "tracked", Merged: true, Remote: "origin", RemoteBranch: "tracked"},
		branches.Branch{Name: "local", Merged: true},
	)
	m = press(press(m, "a"), "enter")

	if m.deleteRemote {
		t.Fatal("remote deletion must be off by default")
	}
	m = press(m, "r")
	if !m.deleteRemote {
		t.Error("r should turn remote deletion on")
	}
	if !strings.Contains(m.View(), "[x] Also delete 1 remote branch") {
		t.Errorf("dialog does not show the toggle:\n%s", m.View())
	}
	if m = press(m, "r"); m.deleteRemote {
		t.Error("r should toggle it back off")
	}

	// With nothing eligible there is nothing to toggle.
	only := testModel(branches.Branch{Name: "local", Merged: true})
	only = press(press(only, "r"), "enter")
	if press(only, "r").deleteRemote {
		t.Error("r must do nothing when no selected branch has a remote")
	}

	// Cancelling and reopening resets the choice.
	m = press(press(m, "r"), "n")
	m = press(m, "enter")
	if m.deleteRemote {
		t.Error("reopening the dialog should reset remote deletion to off")
	}
}

// ---- merge / rebase dialog -------------------------------------------------------------------------------------

func TestOpsNeedACurrentBranch(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"})
	m.current = ""
	m = press(m, "M")
	if m.state != stateMessage || !strings.Contains(m.message, "detached") {
		t.Errorf("state=%v message=%q, want a 'detached HEAD' message", m.state, m.message)
	}
	if press(m, "x").state != stateList {
		t.Error("any key should dismiss the message")
	}
}

func TestOpsRefuseTheCurrentBranchItself(t *testing.T) {
	m := testModel(branches.Branch{Name: "cur", Protected: true})
	for _, k := range []string{"M", "R"} {
		next := press(m, k)
		if next.state != stateMessage || !strings.Contains(next.message, "current branch") {
			t.Errorf("%s on the current branch: state=%v message=%q", k, next.state, next.message)
		}
	}
}

func TestMergeDialogFlow(t *testing.T) {
	m := testModel(branches.Branch{Name: "feature"})

	m, cmd := pressCmd(m, "M")
	if m.state != stateOp || m.op == nil || !m.op.loading || cmd == nil {
		t.Fatalf("M should open the dialog and start the pre-flight (state=%v)", m.state)
	}
	if got := m.op.title(); got != "Merge feature into cur" {
		t.Errorf("title = %q", got)
	}
	if len(m.op.choices) != 4 {
		t.Errorf("merge offers %d choices, want 4 (default, ff-only, no-ff, squash)", len(m.op.choices))
	}

	// Enter is ignored until the pre-flight arrived.
	if next, c := pressCmd(m, "enter"); next.state != stateOp || c != nil {
		t.Error("enter while checking must not run anything")
	}

	m, _ = feed(m, preflightMsg{id: m.op.id, ref: "feature", check: git.MergeCheck{
		Incoming: 3, Local: 1, Checked: true, Conflicts: []string{"a.go", "b.go"},
	}})
	view := m.View()
	for _, want := range []string{"3 new commits", "Conflicts predicted in: a.go, b.go", "Squash"} {
		if !strings.Contains(view, want) {
			t.Errorf("dialog is missing %q:\n%s", want, view)
		}
	}

	m = press(m, "3") // number keys pick an option
	if m.op.cursor != 2 {
		t.Errorf("cursor = %d after pressing 3, want 2", m.op.cursor)
	}
	m = press(m, "j")
	if m.op.cursor != 3 {
		t.Errorf("cursor = %d after down, want 3", m.op.cursor)
	}
	m = press(m, "j")
	if m.op.cursor != 0 {
		t.Errorf("cursor should wrap to 0, got %d", m.op.cursor)
	}

	m, cmd = pressCmd(m, "enter")
	if m.state != stateLoading || cmd == nil {
		t.Errorf("enter should start the merge (state=%v)", m.state)
	}
}

func TestOpDialogCancelAndUpToDate(t *testing.T) {
	m := testModel(branches.Branch{Name: "feature"})
	m = press(m, "R")
	if m.state != stateOp || len(m.op.choices) != 1 || !m.op.choices[0].rebase {
		t.Fatalf("R should open a rebase-only dialog: %+v", m.op)
	}
	if got := m.op.title(); got != "Rebase cur onto feature" {
		t.Errorf("title = %q", got)
	}

	m, _ = feed(m, preflightMsg{id: m.op.id, ref: "feature", check: git.MergeCheck{UpToDate: true, Checked: true}})
	if !strings.Contains(m.View(), "nothing to do") {
		t.Errorf("up-to-date state not explained:\n%s", m.View())
	}

	m = press(m, "esc")
	if m.state != stateList || m.op != nil {
		t.Error("esc should close the dialog")
	}
}

func TestUpdateOffersRebaseAndMergeFromDefaultBranch(t *testing.T) {
	m := testModel(branches.Branch{Name: "whatever"})
	m, cmd := pressCmd(m, "U")
	if m.state != stateOp || cmd == nil {
		t.Fatal("U should open the update dialog")
	}
	if len(m.op.choices) != 2 || !m.op.choices[0].rebase || m.op.choices[1].rebase {
		t.Errorf("update should offer rebase then merge: %+v", m.op.choices)
	}
	if m.op.ref != "main" {
		t.Errorf("update source = %q before the pre-flight resolves it, want the default branch", m.op.ref)
	}

	// The pre-flight resolves the best ref (origin/main when it exists).
	m, _ = feed(m, preflightMsg{id: m.op.id, ref: "origin/main", check: git.MergeCheck{Incoming: 2, Checked: true}})
	if m.op.ref != "origin/main" || !strings.Contains(m.View(), "Rebase onto origin/main") {
		t.Errorf("ref=%q view:\n%s", m.op.ref, m.View())
	}
}

func TestOpResultHandling(t *testing.T) {
	m := testModel(branches.Branch{Name: "feature"})

	t.Run("conflict is explained and nothing is changed", func(t *testing.T) {
		got, _ := feed(m, opDoneMsg{err: &git.ConflictError{Op: "merge", Files: []string{"x.go", "y.go"}}})
		if got.state != stateMessage || !got.messageErr {
			t.Fatalf("state=%v err=%v", got.state, got.messageErr)
		}
		for _, want := range []string{"aborted", "x.go", "y.go", "nothing was changed"} {
			if !strings.Contains(got.message, want) {
				t.Errorf("message is missing %q:\n%s", want, got.message)
			}
		}
	})

	t.Run("friendly errors for fixable conditions", func(t *testing.T) {
		// ErrDirtyTree's own wording is already user-facing, so it must reach the user unchanged.
		for err, want := range map[error]string{
			branches.ErrUpToDate:  "Already up to date",
			branches.ErrDirtyTree: "uncommitted changes",
			branches.ErrDetached:  "detached",
		} {
			got, _ := feed(m, opDoneMsg{err: err})
			if got.state != stateMessage || !strings.Contains(got.message, want) {
				t.Errorf("message for %v = %q, want it to contain %q", err, got.message, want)
			}
		}
	})

	t.Run("success reloads and flashes the outcome", func(t *testing.T) {
		got, cmd := feed(m, opDoneMsg{text: "Merged feature into cur"})
		if got.state != stateLoading || got.flash != "Merged feature into cur" || cmd == nil {
			t.Fatalf("state=%v flash=%q", got.state, got.flash)
		}
		got, _ = feed(got, branchesLoadedMsg{items: []list.Item{item{Branch: branches.Branch{Name: "cur"}}}, current: "cur", defaultBranch: "main"})
		if got.state != stateList || got.flash != "" {
			t.Errorf("after reload: state=%v flash=%q (flash must be consumed)", got.state, got.flash)
		}
	})
}

func TestFetchKeyStartsAnnouncedFetch(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"})
	m, cmd := pressCmd(m, "f")
	if m.state != stateLoading || !strings.Contains(m.busy, "Fetching") || cmd == nil {
		t.Errorf("f should switch to a busy view (state=%v busy=%q)", m.state, m.busy)
	}
}

// ---- result summary --------------------------------------------------------------------------------------------

func TestRenderResult(t *testing.T) {
	out := RenderResult(Result{
		Deleted: []branches.Deleted{
			{Name: "old", SHA: "abc1234"},
			{Name: "tracked", SHA: "def5678", RemoteDeleted: true},
		},
		Errors: []string{"Failed to delete x: boom"},
	})
	for _, want := range []string{"old", "abc1234", "tracked", "+ remote", "git branch <name> <sha>", "Failed to delete x: boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary is missing %q:\n%s", want, out)
		}
	}

	dry := RenderResult(Result{DryRun: true, Deleted: []branches.Deleted{{Name: "old", SHA: "abc1234"}}})
	if strings.Contains(dry, "Restore one with") || !strings.Contains(dry, "Dry Run") {
		t.Errorf("dry-run summary wrong:\n%s", dry)
	}
	if RenderResult(Result{}) != "" {
		t.Error("an empty result should render nothing")
	}
}

// ---- end to end against real repositories ----------------------------------------------------------------------

type testRepo struct {
	t    *testing.T
	dir  string
	repo *git.Repo
}

func (r testRepo) git(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func (r testRepo) commit(file string) {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.dir, file), []byte(file+"\n"), 0o644); err != nil {
		r.t.Fatal(err)
	}
	r.git("add", file)
	r.git("commit", "-m", "add "+file)
}

// newE2ERepo creates a repository on main with a bare origin that already has main pushed.
func newE2ERepo(t *testing.T) testRepo {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	r := testRepo{t: t, dir: t.TempDir()}
	r.git("init", "-b", "main")
	r.git("config", "user.email", "test@example.com")
	r.git("config", "user.name", "Test")
	r.git("config", "commit.gpgsign", "false")
	r.commit("base.txt")

	remote := filepath.Join(t.TempDir(), "remote.git")
	r.git("init", "--bare", "-b", "main", remote)
	r.git("remote", "add", "origin", remote)
	r.git("push", "-u", "origin", "main")

	repo, err := git.Open(r.dir)
	if err != nil {
		t.Fatal(err)
	}
	r.repo = repo
	return r
}

// loaded builds a model and runs the real initial load against the repository.
func loaded(t *testing.T, r testRepo, opts loadOptions) Model {
	t.Helper()
	m := New(config.Default(), r.repo)
	m.list.SetSize(80, 24)
	m, _ = feed(m, loadBranchesCmd(r.repo, m.cfg, opts)())
	if m.state != stateList {
		t.Fatalf("after load: state=%v errors=%v", m.state, m.result.Errors)
	}
	return m
}

// settle feeds every message produced by cmd back into the model and returns the final model and whether the
// program asked to quit.
func settle(m Model, cmd tea.Cmd) (Model, bool) {
	quit := false
	for _, msg := range runCmd(cmd) {
		if _, isQuit := msg.(tea.QuitMsg); isQuit {
			quit = true
		}
		var next tea.Cmd
		m, next = feed(m, msg)
		for _, follow := range runCmd(next) {
			if _, isQuit := follow.(tea.QuitMsg); isQuit {
				quit = true
			}
		}
	}
	return m, quit
}

func TestEndToEndDeleteFlow(t *testing.T) {
	r := newE2ERepo(t)
	r.git("branch", "merged-one") // same commit as main -> merged
	r.git("branch", "merged-two")

	m := loaded(t, r, loadOptions{})
	if len(m.list.Items()) != 3 {
		t.Fatalf("loaded %d items, want 3", len(m.list.Items()))
	}

	m = press(m, "m")
	if got := m.selectedNames(); len(got) != 2 {
		t.Fatalf("selected %v, want the 2 merged branches (main is protected)", got)
	}
	m = press(m, "enter")
	if m.state != stateConfirm {
		t.Fatal("expected confirm view")
	}

	m, cmd := pressCmd(m, "y")
	if m.state != stateLoading {
		t.Fatalf("state = %v, want the busy view while deleting", m.state)
	}
	m, quit := settle(m, cmd)
	if !quit {
		t.Error("finishing the deletion should quit the program")
	}
	if len(m.result.Deleted) != 2 || len(m.result.Errors) != 0 {
		t.Fatalf("result = %+v, want 2 deleted and no errors", m.result)
	}
	for _, d := range m.result.Deleted {
		if d.SHA == "" || d.RemoteDeleted {
			t.Errorf("deleted entry %+v: want a SHA and no remote deletion", d)
		}
	}

	remaining, _ := r.repo.LocalBranches()
	if len(remaining) != 1 || remaining[0].Name != "main" {
		t.Errorf("branches left in repo: %+v, want only main", remaining)
	}
}

func TestEndToEndDeleteWithRemoteAndSquashDetection(t *testing.T) {
	r := newE2ERepo(t)

	// A feature branch that was squash-merged into main and pushed before.
	r.git("checkout", "-b", "feature")
	r.commit("f1.txt")
	r.commit("f2.txt")
	r.git("push", "-u", "origin", "feature")
	r.git("checkout", "main")
	r.git("merge", "--squash", "feature")
	r.git("commit", "-m", "squash feature")

	m := loaded(t, r, loadOptions{})
	m = press(m, "m") // squash-merged counts as merged
	if got := m.selectedNames(); len(got) != 1 || got[0] != "feature" {
		t.Fatalf("m selected %v, want the squash-merged feature branch", got)
	}

	m = press(m, "enter")
	if !strings.Contains(m.View(), "squash/rebase-merged") {
		t.Errorf("dialog should say the branch is squash-merged:\n%s", m.View())
	}
	m = press(m, "r") // also delete the remote branch
	m, cmd := pressCmd(m, "y")
	m, _ = settle(m, cmd)

	if len(m.result.Deleted) != 1 || !m.result.Deleted[0].RemoteDeleted || len(m.result.Errors) != 0 {
		t.Fatalf("result = %+v", m.result)
	}
	if r.repo.RefExists("feature") {
		t.Error("local branch still exists")
	}
	if _, err := r.repo.Fetch(true); err != nil {
		t.Fatal(err)
	}
	if r.repo.RefExists("origin/feature") {
		t.Error("remote branch still exists")
	}
}

func TestEndToEndMerge(t *testing.T) {
	r := newE2ERepo(t)
	r.git("checkout", "-b", "feature")
	r.commit("feature.txt")
	r.git("checkout", "main")

	m := loaded(t, r, loadOptions{})
	// alphabetical: feature, main -> the cursor starts on feature
	if sel := m.list.SelectedItem().(item); sel.Name != "feature" {
		t.Fatalf("highlighted %q, want feature", sel.Name)
	}

	m, cmd := pressCmd(m, "M")
	m, _ = settle(m, cmd) // runs the real pre-flight
	if m.op == nil || m.op.loading {
		t.Fatal("pre-flight did not complete")
	}
	if !m.op.check.FastForward || m.op.check.Incoming != 1 {
		t.Errorf("pre-flight = %+v, want a fast-forward of 1 commit", m.op.check)
	}

	m, cmd = pressCmd(m, "enter")
	m, _ = settle(m, cmd) // runs the merge, then the reload it triggers
	for _, msg := range runCmd(loadBranchesCmd(r.repo, m.cfg, loadOptions{})) {
		m, _ = feed(m, msg)
	}

	if m.state != stateList {
		t.Fatalf("state = %v after the merge, want the list again (message: %q)", m.state, m.message)
	}
	if _, err := os.Stat(filepath.Join(r.dir, "feature.txt")); err != nil {
		t.Error("main does not contain the merged file")
	}
	// feature is now fully merged, so the refreshed list must say so.
	for _, li := range m.list.Items() {
		if i := li.(item); i.Name == "feature" && !i.Merged {
			t.Error("feature should be reported as merged after the merge")
		}
	}
}

func TestEndToEndMergeConflictIsReportedAndAborted(t *testing.T) {
	r := newE2ERepo(t)
	r.commit("shared.txt")
	r.git("checkout", "-b", "other")
	if err := os.WriteFile(filepath.Join(r.dir, "shared.txt"), []byte("other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.git("commit", "-am", "other edits")
	r.git("checkout", "main")
	if err := os.WriteFile(filepath.Join(r.dir, "shared.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.git("commit", "-am", "main edits")
	head := r.repo.ShortSHA("HEAD")

	m := loaded(t, r, loadOptions{})
	for m.list.SelectedItem().(item).Name != "other" {
		m = press(m, "j")
	}
	m, cmd := pressCmd(m, "M")
	m, _ = settle(m, cmd)
	m, cmd = pressCmd(m, "enter")
	m, _ = settle(m, cmd)

	if m.state != stateMessage || !m.messageErr || !strings.Contains(m.message, "shared.txt") {
		t.Fatalf("state=%v message=%q, want a conflict report naming shared.txt", m.state, m.message)
	}
	if r.repo.ShortSHA("HEAD") != head {
		t.Error("HEAD moved despite the aborted merge")
	}
	if clean, _ := r.repo.IsClean(); !clean {
		t.Error("working tree was left dirty")
	}
}

// A pre-flight result that arrives after its dialog was cancelled must not leak into the next dialog.
func TestStalePreflightIsIgnored(t *testing.T) {
	m := testModel(branches.Branch{Name: "alpha"}, branches.Branch{Name: "beta"})

	m = press(m, "M") // dialog for alpha (cursor starts on alpha)
	staleID := m.op.id
	m = press(m, "esc")
	m = press(press(m, "j"), "M") // dialog for beta
	if m.op.title() != "Merge beta into cur" || m.op.id == staleID {
		t.Fatalf("test setup: op = %+v", m.op)
	}

	m, _ = feed(m, preflightMsg{id: staleID, ref: "alpha", check: git.MergeCheck{Incoming: 9, Checked: true}})
	if !m.op.loading || m.op.ref != "beta" {
		t.Errorf("stale result was applied: loading=%v ref=%q", m.op.loading, m.op.ref)
	}

	m, _ = feed(m, preflightMsg{id: m.op.id, ref: "beta", check: git.MergeCheck{Incoming: 1, Checked: true}})
	if m.op.loading || m.op.ref != "beta" {
		t.Errorf("current result was dropped: loading=%v ref=%q", m.op.loading, m.op.ref)
	}
}

func TestRebaseWarnsWhenCurrentBranchIsPushed(t *testing.T) {
	m := testModel(
		branches.Branch{Name: "cur", Protected: true, Remote: "origin"},
		branches.Branch{Name: "other"},
	)
	m = press(m, "U")
	m, _ = feed(m, preflightMsg{id: m.op.id, ref: "main", check: git.MergeCheck{Incoming: 1, Checked: true}})
	if !strings.Contains(m.View(), "already pushed") {
		t.Errorf("rebase of a pushed branch should warn about rewriting history:\n%s", m.View())
	}
	if m = press(m, "j"); strings.Contains(m.View(), "already pushed") {
		t.Error("the warning only applies to the rebase option, not to merge")
	}

	unpushed := testModel(branches.Branch{Name: "cur", Protected: true})
	unpushed = press(unpushed, "U")
	unpushed, _ = feed(unpushed, preflightMsg{id: unpushed.op.id, ref: "main", check: git.MergeCheck{Incoming: 1, Checked: true}})
	if strings.Contains(unpushed.View(), "already pushed") {
		t.Error("a local-only branch can be rebased freely, no warning expected")
	}
}

func TestStaleDefaultBranchNotice(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"})
	m, _ = feed(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	baseHeight := m.list.Height()
	if strings.Contains(m.View(), "behind") {
		t.Fatal("no notice expected when the default branch is in sync")
	}

	// The remote moved ahead: the tool must say it keeps using the local branch, and why that matters.
	m, _ = feed(m, branchesLoadedMsg{
		items:           []list.Item{item{Branch: branches.Branch{Name: "a"}}},
		current:         "cur",
		defaultBranch:   "main",
		defaultUpstream: "origin/main",
		defaultBehind:   12,
	})
	view := strings.Join(strings.Fields(m.View()), " ") // the notice wraps, so compare ignoring line breaks
	for _, want := range []string{
		"Local main is 12 commits behind origin/main",
		"may not show as merged",
		"Checking against local main as it is",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("notice is missing %q:\n%s", want, view)
		}
	}
	if m.list.Height() >= baseHeight {
		t.Errorf("list height %d, want it smaller than %d to make room for the notice", m.list.Height(), baseHeight)
	}

	// Singular wording
	m.defaultBehind = 1
	if !strings.Contains(strings.Join(strings.Fields(m.View()), " "), "1 commit behind") {
		t.Errorf("singular wording wrong:\n%s", m.View())
	}

	// After the user syncs (or a fetch shows they are level) the notice and its reserved space go away.
	m, _ = feed(m, branchesLoadedMsg{
		items:           []list.Item{item{Branch: branches.Branch{Name: "a"}}},
		current:         "cur",
		defaultBranch:   "main",
		defaultUpstream: "origin/main",
	})
	if strings.Contains(m.View(), "behind") || m.list.Height() != baseHeight {
		t.Errorf("notice should disappear when in sync (height %d, want %d)", m.list.Height(), baseHeight)
	}
}
