package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/tasks"
	"github.com/neutrospec/canopy/internal/wiki"
)

// canopy tasks — the delegated-work queue (docs/agent-tasks.md).
// Doors file tasks, agent loops perform them, and `done` re-checks the
// outcome in code (invariant T2): a connect only closes once the mutual
// links exist, an edit only once the page actually changed.

func cmdTasks() *cobra.Command {
	c := &cobra.Command{
		Use:   "tasks",
		Short: "Delegated task queue (web → agent): list, add, done (verified), dismiss",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTasksList("", "", false)
		},
	}
	c.AddCommand(cmdTasksList(), cmdTasksAdd(), cmdTasksClose("done", tasks.StatusDone),
		cmdTasksClose("dismiss", tasks.StatusDismissed), cmdTasksGC())
	return c
}

func cmdTasksList() *cobra.Command {
	var all bool
	var status, page string
	c := &cobra.Command{
		Use:   "list",
		Short: "List tasks (default: pending only — what the agent loop should pick up)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && status != "" {
				return fmt.Errorf("--all and --status are mutually exclusive")
			}
			return runTasksList(status, page, all)
		},
	}
	c.Flags().BoolVar(&all, "all", false, "include done/dismissed tasks")
	c.Flags().StringVar(&status, "status", "", "filter by status (pending|done|dismissed)")
	c.Flags().StringVar(&page, "page", "", "only tasks involving this page")
	return c
}

func runTasksList(status, page string, all bool) error {
	w, err := loadWiki()
	if err != nil {
		return err
	}
	banner(w)
	if status == "" && !all {
		status = tasks.StatusPending
	}
	list, err := tasks.Load(w)
	if err != nil {
		return err
	}
	var out []*tasks.Task
	for _, t := range list {
		if status != "" && t.Status != status {
			continue
		}
		if page != "" && !t.Involves(wiki.NormalizeLink(page)) {
			continue
		}
		out = append(out, t)
	}
	if flagJSON {
		if out == nil {
			out = []*tasks.Task{}
		}
		return emitJSON(map[string]any{"tasks": out, "count": len(out)})
	}
	if len(out) == 0 {
		fmt.Println("no tasks")
		return nil
	}
	for _, t := range out {
		fmt.Printf("%-10s %-8s %s\n", t.Status, t.Type, t.ID)
		fmt.Printf("           %s\n", taskLine(t))
	}
	fmt.Fprintf(os.Stderr, "%d task(s)\n", len(out))
	return nil
}

func taskLine(t *tasks.Task) string {
	var desc string
	switch t.Type {
	case tasks.TypeConnect:
		desc = strings.Join(t.Pages, " ↔ ")
		if t.Sim > 0 {
			desc += fmt.Sprintf(" (sim %.2f)", t.Sim)
		}
	case tasks.TypeEdit:
		desc = strings.Join(t.Pages, ",") + ": " + t.Request
		if t.Request == "" && t.Body != "" {
			desc = strings.Join(t.Pages, ",") + ": (proposed body, " + fmt.Sprintf("%d", len(t.Body)) + " bytes)"
		}
	default:
		desc = strings.Join(t.Pages, ",")
	}
	if len(desc) > 120 {
		desc = desc[:120] + "…"
	}
	when := t.Created
	if i := strings.Index(when, "T"); i > 0 {
		when = when[:i]
	}
	line := desc + " — " + when
	if t.Note != "" {
		line += " · " + t.Note
	}
	return line
}

func cmdTasksAdd() *cobra.Command {
	c := &cobra.Command{Use: "add", Short: "File a task by hand (the web UI files them via its buttons)"}

	connect := &cobra.Command{
		Use:   "connect <page-a> <page-b>",
		Short: "File a connect task: agent should mutually [[link]] the pair",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			a, b := wiki.NormalizeLink(args[0]), wiki.NormalizeLink(args[1])
			for _, s := range []string{a, b} {
				if _, ok := scan.BySlug[s]; !ok {
					return fmt.Errorf("page not found: %s", s)
				}
			}
			t, created, err := tasks.FileConnect(w, a, b, 0, "cli", time.Now())
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"task": t, "created": created})
			}
			if !created {
				fmt.Printf("already filed: %s (%s)\n", t.ID, t.Status)
				return nil
			}
			fmt.Printf("✓ filed %s\n", t.ID)
			return nil
		},
	}

	var request string
	edit := &cobra.Command{
		Use:   "edit <page>",
		Short: "File an edit request: agent should revise the page per --request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			p, ok := scan.BySlug[wiki.NormalizeLink(args[0])]
			if !ok {
				return fmt.Errorf("page not found: %s", args[0])
			}
			raw, err := os.ReadFile(filepath.Join(w.Root, p.RelPath))
			if err != nil {
				return err
			}
			sum := sha256.Sum256(raw)
			t, err := tasks.FileEdit(w, p.Slug, request, "", hex.EncodeToString(sum[:]), "cli", time.Now())
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"task": t, "created": true})
			}
			fmt.Printf("✓ filed %s\n", t.ID)
			return nil
		},
	}
	edit.Flags().StringVar(&request, "request", "", "what to change (required)")
	_ = edit.MarkFlagRequired("request")

	c.AddCommand(connect, edit)
	return c
}

func cmdTasksClose(use, status string) *cobra.Command {
	var note string
	short := "Close a task as done — refused unless the outcome is verifiably in the wiki"
	if status == tasks.StatusDismissed {
		short = "Close a task as dismissed (judged not worth doing; it won't resurface)"
	}
	c := &cobra.Command{
		Use:   use + " <id>",
		Short: short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			t, err := tasks.Close(w, scan, args[0], status, note, attention.DoorAgent, time.Now())
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"task": t})
			}
			fmt.Printf("✓ %s %s\n", t.Status, t.ID)
			return nil
		},
	}
	c.Flags().StringVar(&note, "note", "", "one-line result (done) or reason (dismiss)")
	return c
}

func cmdTasksGC() *cobra.Command {
	var days int
	c := &cobra.Command{
		Use:   "gc",
		Short: "Remove closed tasks older than --days (pending tasks are never touched)",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			removed, err := tasks.GC(w, time.Duration(days)*24*time.Hour, time.Now())
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"removed": removed})
			}
			fmt.Printf("✓ removed %d closed task(s)\n", removed)
			return nil
		},
	}
	c.Flags().IntVar(&days, "days", 90, "keep closed tasks this many days")
	return c
}
