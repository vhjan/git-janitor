package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vhjan/git-janitor/internal/branches"
)

// ---- tabs ------------------------------------------------------------------------------------------------------

func TestTabSwitching(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"})
	if m.tab != tabCleanup {
		t.Fatal("the Cleanup tab must be the default")
	}

	steps := []struct {
		key  string
		want viewTab
	}{
		{"tab", tabBranches},
		{"tab", tabCleanup}, // wraps around
		{"shift+tab", tabBranches},
		{"shift+tab", tabCleanup},
		{"2", tabBranches},
		{"2", tabBranches},
		{"1", tabCleanup},
	}
	for _, s := range steps {
		m = press(m, s.key)
		if m.tab != s.want {
			t.Fatalf("after %q: tab = %v, want %v", s.key, m.tab, s.want)
		}
	}
}

func TestTabKeysAreIgnoredWhileTypingAFilter(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"})
	m.list.SetFilterState(list.Filtering)

	for _, k := range []string{"tab", "2", "?"} {
		next := press(m, k)
		if next.tab != tabCleanup || next.state != stateList {
			t.Errorf("%q while typing a filter changed the view (tab=%v state=%v)", k, next.tab, next.state)
		}
	}
}

func rowNamed(t *testing.T, m Model, name string) item {
	t.Helper()
	for _, li := range m.list.Items() {
		if i := li.(item); i.Name == name {
			return i
		}
	}
	t.Fatalf("no item %s", name)
	return item{}
}

func TestRowsRenderPerTab(t *testing.T) {
	m := testModel(
		branches.Branch{Name: "cur", Current: true, Protected: true, ProtectedReason: "current branch", Subject: "wip stuff"},
		branches.Branch{Name: "other", Subject: "Fix the login bug", Remote: "origin", Merged: true},
	)

	if title := rowNamed(t, m, "other").Title(); !strings.Contains(title, "[ ]") {
		t.Errorf("Cleanup rows need a checkbox, got %q", title)
	}
	if strings.Contains(rowNamed(t, m, "other").Description(), "Fix the login bug") {
		t.Error("the commit subject belongs to the Branches tab only")
	}

	m = press(m, "tab")
	other, cur := rowNamed(t, m, "other"), rowNamed(t, m, "cur")
	if strings.Contains(other.Title(), "[") {
		t.Errorf("Branches rows must not have a checkbox, got %q", other.Title())
	}
	if !strings.Contains(cur.Title(), "●") {
		t.Errorf("the current branch should be marked, got %q", cur.Title())
	}
	if !strings.Contains(other.Description(), "“Fix the login bug”") {
		t.Errorf("Branches rows show the last commit subject: %q", other.Description())
	}
}

func TestSelectionSurvivesTabSwitches(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"}, branches.Branch{Name: "b"})
	m = press(m, "a")
	m = press(press(m, "tab"), "tab")
	if got := selected(m); len(got) != 2 {
		t.Errorf("selection lost while visiting the Branches tab: %v", got)
	}

	// A reload while on the other tab keeps both the selection and the tab.
	m = press(m, "2")
	m, _ = feed(m, branchesLoadedMsg{
		items:   []list.Item{item{Branch: branches.Branch{Name: "a"}}, item{Branch: branches.Branch{Name: "b"}}},
		current: "cur", defaultBranch: "main",
	})
	if m.tab != tabBranches || len(selected(m)) != 2 {
		t.Errorf("reload changed tab=%v or selection=%v", m.tab, selected(m))
	}
	if !m.list.Items()[0].(item).plain {
		t.Error("reloaded rows must be rendered for the active tab")
	}
}

func TestTabKeysBelongToTheirTab(t *testing.T) {
	m := testModel(branches.Branch{Name: "other", Merged: true})

	// Cleanup: the Branches-only keys do nothing, and `m` keeps its meaning of "select merged".
	for _, k := range []string{"r", "u"} {
		if next := press(m, k); next.state != stateList || len(selected(next)) != 0 {
			t.Errorf("%q must do nothing in the Cleanup tab (state=%v selected=%v)", k, next.state, selected(next))
		}
	}
	next := press(m, "m")
	if next.state != stateList || !selected(next)["other"] {
		t.Errorf("m in Cleanup should select merged branches without opening anything (state=%v selected=%v)", next.state, selected(next))
	}

	// Branches: the very same `m` now merges, and the selection keys do nothing at all.
	b := press(m, "2")
	for _, k := range []string{"a", "g", "s", " ", "c"} {
		b = press(b, k)
	}
	if len(selected(b)) != 0 || b.state != stateList {
		t.Errorf("selection keys must do nothing in the Branches tab: selected=%v state=%v", selected(b), b.state)
	}
	if merge := press(b, "m"); merge.state != stateOp || len(selected(merge)) != 0 {
		t.Errorf("m in Branches should open the merge dialog, not select (state=%v selected=%v)", merge.state, selected(merge))
	}
	if b = press(b, "enter"); b.state != stateMenu {
		t.Errorf("Enter in the Branches tab should open the action menu (state=%v)", b.state)
	}
}

// Keys act on the visible tab only: nothing done in one tab may change the other tab's state behind the scenes.
func TestKeysNeverAffectTheInactiveTab(t *testing.T) {
	m := testModel(branches.Branch{Name: "a", Merged: true}, branches.Branch{Name: "b", Merged: true})

	m = press(m, "2")
	for _, k := range []string{"a", "m", "g", "s", "c", " "} {
		// every one of these either does nothing here or opens a dialog; none may change the selection
		m = press(press(m, k), "esc")
		m.state = stateList
	}
	if len(selected(m)) != 0 {
		t.Errorf("Branches-tab keys changed the Cleanup selection: %v", selected(m))
	}

	m = press(m, "1")
	m = press(m, "m")
	m = press(press(m, "2"), "tab") // away and back: the selection is still there
	if len(selected(m)) != 2 {
		t.Errorf("selection = %v, want both branches", selected(m))
	}
}

func TestViewShowsTabBarAndMatchingFooter(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"})
	m, _ = feed(m, tea.WindowSizeMsg{Width: 110, Height: 24})

	clean := m.View()
	for _, want := range []string{"1 Cleanup", "2 Branches", "a: all", "delete selected", "Tab: views"} {
		if !strings.Contains(clean, want) {
			t.Errorf("Cleanup view is missing %q:\n%s", want, clean)
		}
	}

	branchesView := press(m, "tab").View()
	for _, want := range []string{"m: merge", "r: rebase", "u: update", "Enter: actions"} {
		if !strings.Contains(branchesView, want) {
			t.Errorf("Branches view is missing %q:\n%s", want, branchesView)
		}
	}
	if strings.Contains(branchesView, "a: all") {
		t.Error("Branches footer must not advertise selection keys")
	}
}

// ---- action menu -----------------------------------------------------------------------------------------------

func menuLabels(m Model) []string {
	var out []string
	for _, it := range m.menu.items {
		out = append(out, it.label)
	}
	return out
}

func TestActionMenuContentAndDisabledStates(t *testing.T) {
	m := branchesModel(
		branches.Branch{Name: "cur", Current: true, Protected: true, ProtectedReason: "current branch"},
		branches.Branch{Name: "feature"},
	)

	// On the current branch: merge/rebase make no sense and delete is protected, only update works.
	m = press(m, "enter")
	if m.state != stateMenu || m.menu == nil {
		t.Fatalf("state=%v", m.state)
	}
	wantLabels := []string{"Merge cur into cur", "Rebase cur onto cur", "Update cur from main", "Delete branch cur…"}
	for i, want := range wantLabels {
		if got := menuLabels(m)[i]; got != want {
			t.Errorf("item %d = %q, want %q", i, got, want)
		}
	}
	for i, want := range []string{"this is the current branch", "this is the current branch", "", "protected: current branch"} {
		if got := m.menu.items[i].disabled; got != want {
			t.Errorf("item %d disabled = %q, want %q", i, got, want)
		}
	}
	if m.menu.cursor != 2 {
		t.Errorf("cursor starts on %d, want the first usable entry (2)", m.menu.cursor)
	}
	view := m.View()
	for _, want := range []string{"Update cur from main", "(this is the current branch)", "(protected: current branch)"} {
		if !strings.Contains(view, want) {
			t.Errorf("menu view is missing %q:\n%s", want, view)
		}
	}

	// Disabled entries can't be triggered by their letter.
	for _, k := range []string{"m", "r", "d"} {
		if next := press(m, k); next.state != stateMenu {
			t.Errorf("disabled accelerator %q left the menu (state=%v)", k, next.state)
		}
	}

	// On another branch every entry is available.
	m = press(press(press(m, "esc"), "j"), "enter")
	for i, it := range m.menu.items {
		if it.disabled != "" {
			t.Errorf("item %d (%s) unexpectedly disabled: %s", i, it.label, it.disabled)
		}
	}
	if got := menuLabels(m)[0]; got != "Merge feature into cur" {
		t.Errorf("first item = %q", got)
	}
}

func TestActionMenuDetachedHead(t *testing.T) {
	m := branchesModel(branches.Branch{Name: "feature"})
	m.current = ""
	m = press(m, "enter")
	for i := 0; i < 3; i++ {
		if m.menu.items[i].disabled != "HEAD is detached" {
			t.Errorf("item %d disabled = %q, want the detached-HEAD reason", i, m.menu.items[i].disabled)
		}
	}
	if m.menu.items[3].disabled != "" {
		t.Error("deleting another branch works fine with a detached HEAD")
	}
}

func TestActionMenuNavigationAndAccelerators(t *testing.T) {
	m := branchesModel(branches.Branch{Name: "feature"})
	m = press(m, "enter")

	m = press(press(m, "j"), "j")
	if m.menu.cursor != 2 {
		t.Errorf("cursor = %d after two downs, want 2", m.menu.cursor)
	}
	m = press(m, "k")
	if m.menu.cursor != 1 {
		t.Errorf("cursor = %d after up, want 1", m.menu.cursor)
	}
	m = press(m, "4")
	if m.menu.cursor != 3 {
		t.Errorf("cursor = %d after pressing 4, want 3", m.menu.cursor)
	}
	if press(m, "esc").state != stateList {
		t.Error("esc should close the menu")
	}

	for k, want := range map[string]opKind{"m": opMerge, "r": opRebase, "u": opUpdate} {
		next, cmd := pressCmd(m, k)
		if next.state != stateOp || next.op == nil || next.op.kind != want || cmd == nil {
			t.Errorf("accelerator %q: state=%v op=%+v", k, next.state, next.op)
		}
		if next.menu != nil {
			t.Errorf("accelerator %q left the menu open", k)
		}
	}

	// Enter runs the highlighted entry too.
	m = press(m, "2")
	if next := press(m, "enter"); next.state != stateOp || next.op.kind != opRebase {
		t.Errorf("Enter on the second entry should start a rebase (state=%v)", next.state)
	}
}

func TestMenuDeleteTargetsOnlyTheHighlightedBranch(t *testing.T) {
	m := testModel(
		branches.Branch{Name: "doomed", Merged: true, Remote: "origin", RemoteBranch: "doomed"},
		branches.Branch{Name: "bystander", Merged: true},
	)
	m = press(m, "a") // Cleanup: both selected
	m = press(m, "2")

	m = press(press(m, "enter"), "d")
	if m.state != stateConfirm || m.deleteOnly == nil || m.deleteOnly.Name != "doomed" {
		t.Fatalf("state=%v deleteOnly=%v, want the confirm dialog for the highlighted branch", m.state, m.deleteOnly)
	}

	plan := m.plan()
	if len(plan.items) != 1 || plan.items[0].Name != "doomed" {
		t.Errorf("plan = %v, want only doomed even though both are selected in Cleanup", plan.items)
	}
	if !strings.Contains(m.View(), "Delete 1 branch?") {
		t.Errorf("dialog should be about one branch:\n%s", m.View())
	}

	// Cancelling forgets the single target, so the Cleanup selection works again.
	m = press(m, "n")
	if m.state != stateList || m.deleteOnly != nil {
		t.Errorf("cancel: state=%v deleteOnly=%v", m.state, m.deleteOnly)
	}
	m = press(press(m, "1"), "enter")
	if len(m.plan().items) != 2 {
		t.Errorf("Cleanup delete should cover both selected branches, got %d", len(m.plan().items))
	}
}

// ---- help overlay ----------------------------------------------------------------------------------------------

func TestHelpOverlay(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"})
	m, _ = feed(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	m = press(m, "?")
	if m.state != stateHelp {
		t.Fatalf("? should open the help overlay (state=%v)", m.state)
	}
	view := strings.Join(strings.Fields(m.View()), " ")
	for _, want := range []string{"Keyboard shortcuts", "Navigation", "Next view", "scroll"} { // the top of the content
		if !strings.Contains(view, want) {
			t.Errorf("help overlay is missing %q:\n%s", want, m.View())
		}
	}

	for _, closeKey := range []string{"esc", "?", "q"} {
		if press(m, closeKey).state != stateList {
			t.Errorf("%q should close the help overlay", closeKey)
		}
	}
}

func TestHelpOverlayScrollsOnSmallTerminals(t *testing.T) {
	m := testModel(branches.Branch{Name: "a"})
	m, _ = feed(m, tea.WindowSizeMsg{Width: 100, Height: 16})
	m = press(m, "?")

	if m.help.AtBottom() {
		t.Fatal("test setup: the help should be taller than a 16-line terminal")
	}
	before := m.help.YOffset
	for i := 0; i < 5; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
	}
	if m.help.YOffset <= before {
		t.Error("down arrow should scroll the help")
	}
	if m.state != stateHelp {
		t.Error("scrolling must not close the overlay")
	}
}

// ---- end to end ------------------------------------------------------------------------------------------------

func TestEndToEndDeleteFromTheMenu(t *testing.T) {
	r := newE2ERepo(t)
	r.git("branch", "merged-one")
	r.git("branch", "merged-two")

	m := loaded(t, r, loadOptions{})
	m = press(m, "a") // select everything in Cleanup: the menu delete must ignore this
	m = press(m, "2")
	for m.list.SelectedItem().(item).Name != "merged-one" {
		m = press(m, "j")
	}

	m = press(press(m, "enter"), "d")
	if m.state != stateConfirm {
		t.Fatalf("state = %v, want the confirm dialog", m.state)
	}
	m, cmd := pressCmd(m, "y")
	m, quit := settle(m, cmd)
	if !quit || len(m.result.Deleted) != 1 || m.result.Deleted[0].Name != "merged-one" {
		t.Fatalf("result = %+v (quit=%v), want only merged-one deleted", m.result, quit)
	}

	remaining, _ := r.repo.LocalBranches()
	var names []string
	for _, b := range remaining {
		names = append(names, b.Name)
	}
	if strings.Join(names, ",") != "main,merged-two" {
		t.Errorf("branches left: %v, want main and merged-two", names)
	}
}

func TestHelpContentListsEveryView(t *testing.T) {
	content := strings.Join(strings.Fields(helpContent(76)), " ")
	for _, want := range []string{
		"Cleanup view: select and delete", "Select ALL unprotected branches",
		"Branches view: act on one branch", "Open the action menu",
		"Delete confirmation", "Menus and merge / rebase dialogs",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("help content is missing %q", want)
		}
	}
}

// ---- menu cursor -----------------------------------------------------------------------------------------------

func TestMenuCursorSkipsUnavailableEntries(t *testing.T) {
	// A protected branch other than the current one: merge, rebase and update work, delete is blocked.
	m := branchesModel(
		branches.Branch{Name: "main", Protected: true, ProtectedReason: "default branch"},
	)
	m = press(m, "enter")
	if got := m.menu.items[3].disabled; got == "" {
		t.Fatal("test setup: delete should be disabled for a protected branch")
	}
	if m.menu.cursor != 0 {
		t.Fatalf("cursor starts on %d, want 0", m.menu.cursor)
	}

	// Down walks 0 -> 1 -> 2 and then wraps straight back to 0, never landing on the blocked entry.
	var seen []int
	for i := 0; i < 5; i++ {
		m = press(m, "j")
		seen = append(seen, m.menu.cursor)
	}
	if fmt.Sprint(seen) != "[1 2 0 1 2]" {
		t.Errorf("cursor path going down = %v, want [1 2 0 1 2]", seen)
	}

	// And the same going up.
	seen = nil
	for i := 0; i < 4; i++ {
		m = press(m, "k")
		seen = append(seen, m.menu.cursor)
	}
	if fmt.Sprint(seen) != "[1 0 2 1]" {
		t.Errorf("cursor path going up = %v, want [1 0 2 1]", seen)
	}

	// The highlight is always on a usable entry.
	if !strings.Contains(m.View(), "> ") {
		t.Errorf("a selectable entry must always be highlighted:\n%s", m.View())
	}
}

func TestMenuCursorStaysPutWhenNothingElseIsSelectable(t *testing.T) {
	// The current branch: only "Update" is available, everything else is blocked.
	m := branchesModel(branches.Branch{Name: "cur", Current: true, Protected: true, ProtectedReason: "current branch"})
	m = press(m, "enter")
	if m.menu.cursor != 2 {
		t.Fatalf("cursor starts on %d, want the only usable entry (2)", m.menu.cursor)
	}

	for _, k := range []string{"j", "k", "down", "up", "tab", "shift+tab", "1", "2", "4"} {
		m = press(m, k)
		if m.menu.cursor != 2 {
			t.Fatalf("after %q the cursor moved to %d; it must stay on the only selectable entry", k, m.menu.cursor)
		}
	}
	if !strings.Contains(m.View(), "> Update cur from main") {
		t.Errorf("the selection must stay visible:\n%s", m.View())
	}
}

func TestMenuWithNothingSelectable(t *testing.T) {
	// Detached HEAD plus a protected branch: every entry is blocked.
	m := branchesModel(branches.Branch{Name: "main", Protected: true, ProtectedReason: "default branch"})
	m.current = ""
	m = press(m, "enter")

	if m.menu.cursor != -1 {
		t.Fatalf("cursor = %d, want -1 (nothing selectable)", m.menu.cursor)
	}
	for _, k := range []string{"j", "k", "enter", "m", "r", "u", "d", "1"} {
		next := press(m, k)
		if next.state != stateMenu || next.menu.cursor != -1 {
			t.Errorf("%q with nothing selectable: state=%v cursor=%d", k, next.state, next.menu.cursor)
		}
	}
	if strings.Contains(m.View(), "> ") {
		t.Errorf("nothing should be highlighted when nothing can be selected:\n%s", m.View())
	}
	if press(m, "esc").state != stateList {
		t.Error("esc must still close the menu")
	}
}
