// Read verbs with native-tool ergonomics: line-numbered grep and
// range/section slicing for show. Reading stays cheaper through canopy
// than through raw files — slug addressing, no full-page dumps — which
// is what keeps agents on the observed path (docs/checkout-design.md §2).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/wiki"
)

// cmdGrep is search, not reading: like H5's search-exposure rule, a
// grep sweep must not mark every matching page as accessed (H11).
func cmdGrep() *cobra.Command {
	var ignoreCase bool
	var maxN int
	c := &cobra.Command{
		Use:   "grep <pattern> [page]",
		Short: "Regexp search with line numbers (slug:line: text) — across the wiki or inside one page",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pat := args[0]
			if ignoreCase {
				pat = "(?i)" + pat
			}
			re, err := regexp.Compile(pat)
			if err != nil {
				return fmt.Errorf("bad pattern: %w", err)
			}
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			pages := scan.Pages
			if len(args) == 2 {
				p, ok := scan.BySlug[wiki.NormalizeLink(args[1])]
				if !ok {
					return fmt.Errorf("page not found: %s", args[1])
				}
				pages = []*wiki.Page{p}
			}

			type match struct {
				Slug string `json:"slug"`
				Line int    `json:"line"`
				Text string `json:"text"`
			}
			var matches []match
			capped := false
		outer:
			for _, p := range pages {
				data, err := os.ReadFile(filepath.Join(w.Root, p.RelPath))
				if err != nil {
					continue
				}
				for i, line := range strings.Split(string(data), "\n") {
					if !re.MatchString(line) {
						continue
					}
					if len(matches) >= maxN {
						capped = true
						break outer
					}
					matches = append(matches, match{Slug: p.Slug, Line: i + 1, Text: line})
				}
			}
			if flagJSON {
				return emitJSON(map[string]any{"matches": matches, "capped": capped})
			}
			for _, m := range matches {
				fmt.Printf("%s:%d: %s\n", m.Slug, m.Line, m.Text)
			}
			if capped {
				fmt.Fprintf(os.Stderr, "…capped at %d matches (raise with -n)\n", maxN)
			}
			if len(matches) == 0 {
				fmt.Fprintln(os.Stderr, "no matches")
			}
			return nil
		},
	}
	c.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false, "case-insensitive match")
	c.Flags().IntVarP(&maxN, "max", "n", 200, "maximum matches to print")
	return c
}

// sliceLines returns the requested 1-based inclusive line range.
func sliceLines(content, spec string) (lines []string, start int, err error) {
	all := strings.Split(content, "\n")
	parts := strings.SplitN(spec, "-", 2)
	from, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, 0, fmt.Errorf("bad --lines %q (want N or N-M)", spec)
	}
	to := from
	if len(parts) == 2 {
		if to, err = strconv.Atoi(strings.TrimSpace(parts[1])); err != nil {
			return nil, 0, fmt.Errorf("bad --lines %q (want N or N-M)", spec)
		}
	}
	if from < 1 || to < from {
		return nil, 0, fmt.Errorf("bad --lines %q (want 1 ≤ N ≤ M)", spec)
	}
	if from > len(all) {
		return nil, 0, fmt.Errorf("--lines %q starts past end of file (%d lines)", spec, len(all))
	}
	if to > len(all) {
		to = len(all)
	}
	return all[from-1 : to], from, nil
}

var headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)

// sliceSection returns the section whose heading contains query
// (case-insensitive), up to the next heading of the same or higher
// level. Headings inside code fences don't count.
func sliceSection(content, query string) (lines []string, start int, err error) {
	all := strings.Split(content, "\n")
	q := strings.ToLower(query)
	inFence := false
	startIdx, level := -1, 0
	var headings []string
	for i, line := range all {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		m := headingRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if startIdx == -1 {
			headings = append(headings, m[2])
			if strings.Contains(strings.ToLower(line), q) {
				startIdx, level = i, len(m[1])
			}
			continue
		}
		if len(m[1]) <= level { // section ended
			return all[startIdx:i], startIdx + 1, nil
		}
	}
	if startIdx == -1 {
		hint := ""
		if len(headings) > 0 {
			hint = " — headings: " + strings.Join(headings, " | ")
		}
		return nil, 0, fmt.Errorf("no heading matches %q%s", query, hint)
	}
	return all[startIdx:], startIdx + 1, nil
}

func printNumbered(lines []string, start int) {
	for i, line := range lines {
		fmt.Printf("%d:%s\n", start+i, line)
	}
}
