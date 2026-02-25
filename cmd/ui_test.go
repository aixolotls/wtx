package cmd

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderCreateProgress_NewBranchFromBase(t *testing.T) {
	m := model{
		creatingBranch:  "feature/test",
		creatingBaseRef: "origin/main",
	}
	got := renderCreateProgress(m)
	if !strings.Contains(got, "Provisioning") || !strings.Contains(got, "from") {
		t.Fatalf("expected provisioning-from message, got %q", got)
	}
	if !strings.Contains(got, "origin/main") {
		t.Fatalf("expected base ref in message, got %q", got)
	}
}

func TestRenderCreateProgress_ExistingBranch(t *testing.T) {
	m := model{
		creatingBranch:   "feature/test",
		creatingExisting: true,
	}
	got := renderCreateProgress(m)
	if !strings.Contains(got, "worktree for") {
		t.Fatalf("expected existing-branch provisioning message, got %q", got)
	}
}

func TestShouldFetchByBranch(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		loadedKey   string
		fetchingKey string
		want        bool
	}{
		{name: "new key", key: "a", loadedKey: "", fetchingKey: "", want: true},
		{name: "loaded key", key: "a", loadedKey: "a", fetchingKey: "", want: false},
		{name: "fetching key", key: "a", loadedKey: "", fetchingKey: "a", want: false},
		{name: "empty key", key: "", loadedKey: "", fetchingKey: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldFetchByBranch(tc.key, tc.loadedKey, tc.fetchingKey)
			if got != tc.want {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestNormalizeFetchForBaseRef(t *testing.T) {
	if got := normalizeFetchForBaseRef("main", true); got {
		t.Fatalf("expected local base ref to disable fetch")
	}
	if got := normalizeFetchForBaseRef("origin/main", true); !got {
		t.Fatalf("expected remote base ref to keep fetch enabled")
	}
	if got := normalizeFetchForBaseRef("main", false); got {
		t.Fatalf("expected local base ref to keep fetch disabled")
	}
}

func TestShouldPromptFetchDefault(t *testing.T) {
	if shouldPromptFetchDefault("main", false, true) {
		t.Fatalf("expected local base ref to suppress fetch-default prompt")
	}
	if !shouldPromptFetchDefault("origin/main", false, true) {
		t.Fatalf("expected remote base ref to prompt when fetch preference differs")
	}
	if shouldPromptFetchDefault("origin/main", true, true) {
		t.Fatalf("expected no prompt when fetch preference matches default")
	}
}

func TestLooksLikeLocalBranchRef(t *testing.T) {
	if !looksLikeLocalBranchRef("main") {
		t.Fatalf("expected main to be treated as local")
	}
	if looksLikeLocalBranchRef("origin/main") {
		t.Fatalf("expected origin/main to be treated as remote")
	}
}

func TestDraftBranchName(t *testing.T) {
	got := draftBranchName(time.Unix(1700000000, 0))
	if got != "draft-1700000000" {
		t.Fatalf("expected deterministic draft name, got %q", got)
	}
}

func TestModeBranchPick_AllowsTypingKAndJInFilter(t *testing.T) {
	m := newModel()
	m.mode = modeBranchPick
	m.branchOptions = []string{"main", "release/kilo", "feature/jump"}
	m.branchSuggestions = filterBranches(m.branchOptions, "")
	m.branchInput.Focus()

	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	updated := updatedModel.(model)
	if updated.branchInput.Value() != "k" {
		t.Fatalf("expected filter input to include k, got %q", updated.branchInput.Value())
	}

	updatedModel, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	updated = updatedModel.(model)
	if updated.branchInput.Value() != "kj" {
		t.Fatalf("expected filter input to include j, got %q", updated.branchInput.Value())
	}
}
