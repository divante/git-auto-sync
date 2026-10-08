package common

import (
	"github.com/rjeczalik/notify"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func ignoredRuntimeFixture(t *testing.T) RepoConfig {
	cfg := PrepareFixture(t, "new_file")
	assert.NilError(t, os.WriteFile(filepath.Join(cfg.RepoPath, ".gitignore"), []byte(".beads/embeddeddolt/\n"), 0600))
	runtime := filepath.Join(cfg.RepoPath, ".beads", "embeddeddolt", "obsidian", ".dolt", "noms")
	assert.NilError(t, os.MkdirAll(runtime, 0700))
	assert.NilError(t, os.Chmod(runtime, 0000))
	t.Cleanup(func() { os.Chmod(runtime, 0700); os.RemoveAll(cfg.RepoPath) })
	return cfg
}

func Test_IgnoreDoesNotTraverseExcludedRuntime(t *testing.T) {
	cfg := ignoredRuntimeFixture(t)
	ignored, err := ShouldIgnoreFile(cfg.RepoPath, "2.md")
	assert.NilError(t, err)
	assert.Equal(t, ignored, false)
}

func Test_CommitDoesNotTraverseExcludedRuntime(t *testing.T) {
	cfg := ignoredRuntimeFixture(t)
	assert.NilError(t, commit(cfg))
	status, err := GitCommand(cfg, []string{"status", "--porcelain"})
	assert.NilError(t, err)
	assert.Equal(t, status.String(), "")
}

func Test_WatcherSkipsExcludedRuntime(t *testing.T) {
	cfg := ignoredRuntimeFixture(t)
	events := make(chan notify.EventInfo, 100)
	defer notify.Stop(events)
	assert.NilError(t, watchDirectories(cfg.RepoPath, cfg.RepoPath, events))
	note := filepath.Join(cfg.RepoPath, "2.md")
	assert.NilError(t, os.WriteFile(note, []byte("updated note"), 0600))
	select {
	case event := <-events:
		assert.Equal(t, event.Path(), note)
	case <-time.After(3 * time.Second):
		t.Fatal("document edit was not observed")
	}
}

func Test_CommitHandlesUnusualNamesAndStagedRename(t *testing.T) {
	cfg := PrepareFixture(t, "no_changes")
	t.Cleanup(func() { os.RemoveAll(cfg.RepoPath) })
	name := "-note with space\nand newline.md"
	assert.NilError(t, os.WriteFile(filepath.Join(cfg.RepoPath, name), []byte("note"), 0600))
	assert.NilError(t, commit(cfg))
	_, err := GitCommand(cfg, []string{"mv", "--", name, "renamed note.md"})
	assert.NilError(t, err)
	assert.NilError(t, commit(cfg))
	status, err := GitCommand(cfg, []string{"status", "--porcelain"})
	assert.NilError(t, err)
	assert.Equal(t, status.String(), "")
	data, err := os.ReadFile(filepath.Join(cfg.RepoPath, "renamed note.md"))
	assert.NilError(t, err)
	assert.Equal(t, string(data), "note")
}
