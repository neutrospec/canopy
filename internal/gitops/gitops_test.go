package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	b, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}
func clones(t *testing.T) (root, peer, remote string) {
	t.Helper()
	dir := t.TempDir()
	remote = filepath.Join(dir, "origin.git")
	root = filepath.Join(dir, "local")
	peer = filepath.Join(dir, "peer")
	git(t, dir, "init", "--bare", remote)
	git(t, dir, "clone", remote, root)
	git(t, root, "config", "user.name", "fixture")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	os.MkdirAll(filepath.Join(root, "concepts"), 0o755)
	os.WriteFile(filepath.Join(root, "concepts", "alpha.md"), []byte("first\nshared baseline\nlast\n"), 0o644)
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "baseline")
	git(t, root, "push", "-u", "origin", "HEAD")
	git(t, dir, "clone", remote, peer)
	git(t, peer, "config", "user.name", "fixture")
	git(t, peer, "config", "user.email", "fixture@example.invalid")
	return
}

func TestAutostashConflictStopsBeforeAddCommitPush(t *testing.T) {
	root, peer, remote := clones(t)
	localTip := git(t, root, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(peer, "concepts", "alpha.md"), []byte("first\nREMOTE-SOURCE\nlast\n"), 0o644)
	git(t, peer, "commit", "-am", "remote edit")
	git(t, peer, "push")
	remoteTip := git(t, remote, "rev-parse", "HEAD")
	os.WriteFile(filepath.Join(root, "concepts", "alpha.md"), []byte("first\nLOCAL-SOURCE\nlast\n"), 0o644)
	if _, err := Pull(root); err == nil {
		t.Fatal("autostash conflict accepted")
	}
	pullTip := git(t, root, "rev-parse", "HEAD")
	if pullTip == localTip {
		t.Fatal("fixture did not pull peer commit")
	}
	if _, err := CommitAll(root, "must not commit markers"); err == nil {
		t.Fatal("committed unresolved entries")
	}
	if _, err := Push(root); err == nil {
		t.Fatal("pushed unresolved entries")
	}
	if got := git(t, remote, "rev-parse", "HEAD"); got != remoteTip {
		t.Fatal("remote changed")
	}
	if got := git(t, root, "rev-parse", "HEAD"); got != pullTip {
		t.Fatal("created conflict commit")
	}
	if got := git(t, root, "ls-files", "-u"); got == "" {
		t.Fatal("add resolved the index")
	}
	if text := git(t, root, "show", "stash@{0}:concepts/alpha.md"); !strings.Contains(text, "LOCAL-SOURCE") {
		t.Fatal("local source not recoverable")
	}
	if text := git(t, remote, "show", "HEAD:concepts/alpha.md"); !strings.Contains(text, "REMOTE-SOURCE") {
		t.Fatal("remote source not recoverable")
	}
}

func TestNormalAutostashAndScratchExclusion(t *testing.T) {
	root, peer, remote := clones(t)
	os.WriteFile(filepath.Join(peer, "concepts", "alpha.md"), []byte("REMOTE-FIRST\nshared baseline\nlast\n"), 0o644)
	git(t, peer, "commit", "-am", "remote")
	git(t, peer, "push")
	os.WriteFile(filepath.Join(root, "concepts", "alpha.md"), []byte("first\nshared baseline\nLOCAL-LAST\n"), 0o644)
	if _, err := Pull(root); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "concepts", ".canopy-tmp-crashed"), []byte("partial scratch"), 0o600)
	if _, err := CommitAll(root, "safe merge"); err != nil {
		t.Fatal(err)
	}
	if _, err := Push(root); err != nil {
		t.Fatal(err)
	}
	text := git(t, remote, "show", "HEAD:concepts/alpha.md")
	if !strings.Contains(text, "REMOTE-FIRST") || !strings.Contains(text, "LOCAL-LAST") {
		t.Fatal(text)
	}
	if files := git(t, remote, "ls-tree", "-r", "--name-only", "HEAD"); strings.Contains(files, ".canopy-tmp-") {
		t.Fatal("scratch committed")
	}
	st, err := GetStatus(root)
	if err != nil || st.Dirty != 0 {
		t.Fatalf("scratch polluted status: %+v %v", st, err)
	}
}

func TestPreexistingGitOperationRefused(t *testing.T) {
	root, _, _ := clones(t)
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "sequencer"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(root, ".git", name)
			if strings.Contains(name, "rebase") || name == "sequencer" {
				os.Mkdir(path, 0o755)
			} else {
				os.WriteFile(path, []byte("fixture"), 0o600)
			}
			defer os.RemoveAll(path)
			if _, err := Pull(root); err == nil {
				t.Fatal("pull started during existing operation")
			}
			if _, err := CommitAll(root, "unsafe"); err == nil {
				t.Fatal("commit started during existing operation")
			}
		})
	}
}
