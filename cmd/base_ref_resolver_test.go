package cmd

import "testing"

func TestResolveNewBranchBaseRef_NoRemoteUsesMain(t *testing.T) {
	got := resolveNewBranchBaseRef("origin/develop", "feature/local-only", false)
	if got != "main" {
		t.Fatalf("expected main, got %q", got)
	}
}

func TestResolveNewBranchBaseRef_RemoteUsesConfig(t *testing.T) {
	got := resolveNewBranchBaseRef("origin/develop", "origin/main", true)
	if got != "origin/develop" {
		t.Fatalf("expected config base ref, got %q", got)
	}
}
