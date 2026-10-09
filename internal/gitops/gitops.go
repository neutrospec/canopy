// Package gitops shells out to the git binary so the user's existing
// git configuration and credentials apply unchanged.
package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/neutrospec/canopy/internal/safefile"
)

type Status struct {
	IsRepo  bool     `json:"is_repo"`
	Branch  string   `json:"branch"`
	Dirty   int      `json:"dirty"`  // modified + untracked paths
	Ahead   int      `json:"ahead"`  // commits not pushed
	Behind  int      `json:"behind"` // commits not pulled
	Changed []string `json:"changed,omitempty"`
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

var branchRe = regexp.MustCompile(`^## (\S+?)(?:\.\.\.(\S+))?(?: \[(.*)\])?$`)

// GetStatus inspects the wiki repo. A non-repo directory returns
// Status{IsRepo: false} without error.
func GetStatus(dir string) (*Status, error) {
	out, err := run(dir, "status", "--porcelain=v1", "-b")
	if err != nil {
		if strings.Contains(err.Error(), "not a git repository") {
			return &Status{}, nil
		}
		return nil, err
	}
	st := &Status{IsRepo: true}
	for i, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if i == 0 && strings.HasPrefix(line, "## ") {
			if m := branchRe.FindStringSubmatch(line); m != nil {
				st.Branch = m[1]
				for _, part := range strings.Split(m[3], ", ") {
					if n, ok := strings.CutPrefix(part, "ahead "); ok {
						fmt.Sscanf(n, "%d", &st.Ahead)
					}
					if n, ok := strings.CutPrefix(part, "behind "); ok {
						fmt.Sscanf(n, "%d", &st.Behind)
					}
				}
			}
			continue
		}
		if line == "" {
			continue
		}
		if len(line) > 3 && strings.HasPrefix(filepath.Base(strings.Trim(line[3:], "\"")), safefile.TempPrefix) {
			continue
		}
		st.Dirty++
		if len(line) > 3 {
			st.Changed = append(st.Changed, strings.TrimSpace(line[3:]))
		}
	}
	return st, nil
}

// Pull rebases local work onto origin. Called before committing so a
// user's direct edits (e.g. via Obsidian on another machine) merge in.
func Pull(dir string) (string, error) {
	if err := RequireSafe(dir); err != nil {
		return "", err
	}
	out, err := run(dir, "pull", "--rebase", "--autostash")
	if err != nil && strings.Contains(err.Error(), "no tracking information") {
		return "no upstream configured, skipping pull", nil
	}
	if err == nil {
		err = RequireSafe(dir)
	}
	return strings.TrimSpace(out), err
}

func CommitAll(dir, message string) (string, error) {
	if err := RequireSafe(dir); err != nil {
		return "", err
	}
	if _, err := run(dir, "add", "-A", "--", ".", ":(glob,exclude)**/"+safefile.TempPrefix+"*"); err != nil {
		return "", err
	}
	if err := RequireSafe(dir); err != nil {
		return "", err
	}
	out, err := run(dir, "commit", "-m", message)
	return strings.TrimSpace(out), err
}

func Push(dir string) (string, error) {
	if err := RequireSafe(dir); err != nil {
		return "", err
	}
	out, err := run(dir, "push")
	if err != nil && strings.Contains(err.Error(), "No configured push destination") {
		return "no remote configured, skipping push", nil
	}
	return strings.TrimSpace(out), err
}

// RequireSafe refuses unresolved index entries and operations that need a
// human decision. In particular, autostash conflicts may leave exit 0 from pull;
// the index, not Git's stdout, is the authority. This check precedes add -A.
func RequireSafe(dir string) error {
	unmerged, err := run(dir, "ls-files", "-u")
	if err != nil {
		return err
	}
	if strings.TrimSpace(unmerged) != "" {
		return fmt.Errorf("unresolved Git index entries in %s; resolve them manually; local files and autostash are preserved", dir)
	}
	for _, name := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "rebase-merge", "rebase-apply", "sequencer"} {
		p, err := run(dir, "rev-parse", "--git-path", name)
		if err != nil {
			return err
		}
		path := strings.TrimSpace(p)
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		if _, err := os.Lstat(path); err == nil {
			return fmt.Errorf("Git operation in progress (%s) in %s; finish or abort it manually before sync", name, dir)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func Head(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--verify", "HEAD")
	return strings.TrimSpace(out), err
}

// RemoteTip observes the tracked destination after an ambiguous push failure.
// A failed observation remains unknown; subprocess failure alone cannot prove
// that a remote rejected a commit (the acknowledgement may have been lost).
func RemoteTip(dir string) (string, error) {
	branch, err := run(dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return "", err
	}
	branch = strings.TrimSpace(branch)
	remote, err := run(dir, "config", "--get", "branch."+branch+".remote")
	if err != nil {
		return "", err
	}
	ref, err := run(dir, "config", "--get", "branch."+branch+".merge")
	if err != nil {
		return "", err
	}
	out, err := run(dir, "ls-remote", "--exit-code", strings.TrimSpace(remote), strings.TrimSpace(ref))
	if err != nil {
		return "", err
	}
	fields := strings.Fields(out)
	if len(fields) < 2 {
		return "", fmt.Errorf("remote destination cannot be observed")
	}
	return fields[0], nil
}

// Banner renders the unsynced-state warning shown at the start of every
// command. Empty string means the wiki is clean and pushed.
func (s *Status) Banner() string {
	if !s.IsRepo || (s.Dirty == 0 && s.Ahead == 0 && s.Behind == 0) {
		return ""
	}
	var parts []string
	if s.Dirty > 0 {
		parts = append(parts, fmt.Sprintf("%d uncommitted change(s)", s.Dirty))
	}
	if s.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("%d unpushed commit(s)", s.Ahead))
	}
	if s.Behind > 0 {
		parts = append(parts, fmt.Sprintf("%d commit(s) behind origin", s.Behind))
	}
	return "⚠ wiki has " + strings.Join(parts, ", ") + " — run `canopy sync`"
}
