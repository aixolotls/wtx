package cmd

import (
	"path/filepath"
	"testing"
)

func TestConfigPath_UsesOverrideEnv(t *testing.T) {
	override := t.TempDir()
	t.Setenv(configDirOverrideEnv, override)
	t.Setenv("HOME", "")

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath with override: %v", err)
	}
	want := filepath.Join(override, "config.json")
	if path != want {
		t.Fatalf("expected %q, got %q", want, path)
	}
}

func TestConfigPath_UsesHomeByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv(configDirOverrideEnv, "")
	t.Setenv("HOME", home)

	path, err := configPath()
	if err != nil {
		t.Fatalf("configPath default: %v", err)
	}
	want := filepath.Join(home, ".wtx", "config.json")
	if path != want {
		t.Fatalf("expected %q, got %q", want, path)
	}
}
