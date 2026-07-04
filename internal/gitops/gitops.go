// Package gitops shells out to the git binary so the user's existing
// git configuration and credentials apply unchanged.
package gitops

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

type Status struct {
	IsRepo  bool   `json:"is_repo"`
	Branch  string `json:"branch"`
	Dirty   int    `json:"dirty"`  // modified + untracked paths
	Ahead   int    `json:"ahead"`  // commits not pushed
	Behind  int    `json:"behind"` // commits not pulled
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
		st.Dirty++
		if len(line) > 3 {
			st.Changed = append(st.Changed, strings.TrimSpace(line[3:]))
		}
	}
	return st, nil
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
