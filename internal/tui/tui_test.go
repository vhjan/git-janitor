package tui

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/vhjan/git-janitor/internal/git"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/vhjan/git-janitor/internal/branches"
	"github.com/vhjan/git-janitor/internal/config"
)

func TestKeyMapHasNoDuplicateKeys(t *testing.T) {
	seen := map[string]string{}
	for _, b := range defaultKeyMap().bindings() {
		for _, k := range b.Keys() {
			if other, dup := seen[k]; dup {
				t.Errorf("key %q is bound to both %q and %q", k, other, b.Help().Desc)
			}
			seen[k] = b.Help().Desc
		}
	}
}

func TestHelpRowsCoverAllListBindings(t *testing.T) {
	// 2 navigation rows + 9 list-view bindings
	if got := len(HelpRows()); got != 11 {
		t.Errorf("HelpRows returned %d rows, want 11", got)
	}
}

// testModel returns a model in the list state with the given branches.
func testModel(items ...branches.Branch) Model {
	m := New(config.Default(), nil)
	m.state = stateList
	m.list.SetSize(80, 24)
	li := make([]list.Item, len(items))
	for i, b := range items {
		li[i] = item{Branch: b}
	}
	m.list.SetItems(li)
	return m
}

func press(m Model, k string) Model {
	var msg tea.KeyMsg
	switch k {
	case " ":
		msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	next, _ := m.Update(msg)
	return next.(Model)
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

func TestSelectionKeys(t *testing.T) {
	m := testModel(
		branches.Branch{Name: "main", Protected: true},
		branches.Branch{Name: "merged", Merged: true},
		branches.Branch{Name: "gone", Gone: true},
		branches.Branch{Name: "stale", Stale: true},
		branches.Branch{Name: "plain"},
	)

	cases := []struct {
		key  string
		want []string
	}{
		{"m", []string{"merged"}},
		{"g", []string{"gone"}},
		{"s", []string{"stale"}},
		{"a", []string{"merged", "gone", "stale", "plain"}}, // never the protected branch
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

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if next.(Model).state != stateList {
		t.Error("enter without a selection must stay in the list view")
	}

	m = press(m, "a")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if next.(Model).state != stateConfirm {
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
	var names []string
	for _, li := range toggled.list.Items() {
		names = append(names, li.(item).Name)
	}
	if strings.Join(names, ",") != "alpha,beta,gamma" {
		t.Errorf("toggle corrupted the item list: %v", names)
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

// TestEndToEndDeleteFlow drives the real model against a real repository: load -> select merged -> confirm -> delete.
func TestEndToEndDeleteFlow(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}

	dir := t.TempDir()
	g := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	g("init", "-b", "main")
	g("config", "user.email", "test@example.com")
	g("config", "user.name", "Test")
	g("config", "commit.gpgsign", "false")
	g("commit", "--allow-empty", "-m", "initial")
	g("branch", "merged-one") // same commit as main -> merged
	g("branch", "merged-two")

	repo, err := git.Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Load through the real command, then feed the message into the model.
	m := New(config.Default(), repo)
	m.list.SetSize(80, 24)
	msg := loadBranchesCmd(repo, m.cfg)()
	next, _ := m.Update(msg)
	m = next.(Model)
	if m.state != stateList || len(m.list.Items()) != 3 {
		t.Fatalf("after load: state=%v items=%d, want list view with 3 items", m.state, len(m.list.Items()))
	}

	m = press(m, "m")
	if got := m.selectedNames(); len(got) != 2 {
		t.Fatalf("selected %v, want the 2 merged branches (main is protected)", got)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(Model)
	if m.state != stateConfirm {
		t.Fatal("expected confirm view")
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	m = next.(Model)
	if cmd == nil {
		t.Error("confirming should quit the program")
	}
	if len(m.result.Deleted) != 2 || len(m.result.Errors) != 0 {
		t.Fatalf("result = %+v, want 2 deleted and no errors", m.result)
	}

	remaining, _ := repo.LocalBranches()
	if len(remaining) != 1 || remaining[0].Name != "main" {
		t.Errorf("branches left in repo: %+v, want only main", remaining)
	}
}
