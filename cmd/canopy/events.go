package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/digest"
)

// canopy events — the raw surface of the machine-local observation
// timeline (docs/events.md). Co-location is the point: an agent on this
// host answers "what happened in this wiki lately" with one call, no
// server. Events are observations, never truth (invariant N1).

func cmdEvents() *cobra.Command {
	var kind, slug, since string
	var limit int
	c := &cobra.Command{
		Use:   "events",
		Short: "Query the machine-local event log (attention + task/sync/reconcile lifecycle)",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			f := attention.Filter{Kind: kind, Slug: slug, Limit: limit}
			if since != "" {
				cutoff, err := digest.ParseSince(since, time.Now())
				if err != nil {
					return err
				}
				f.Since = cutoff
			}
			ev, err := attention.Open(w.AttentionDBPath())
			if err != nil {
				return err
			}
			defer ev.Close()
			events, err := ev.Query(f)
			if err != nil {
				return err
			}
			if flagJSON {
				if events == nil {
					events = []attention.Event{}
				}
				return emitJSON(map[string]any{"events": events, "count": len(events)})
			}
			for _, e := range events {
				ts := e.TS
				if t, err := time.Parse(time.RFC3339, e.TS); err == nil {
					ts = t.Local().Format("2006-01-02 15:04")
				}
				line := fmt.Sprintf("%s  %-22s %-5s %s", ts, e.Kind, e.Door, e.Slug)
				if e.Meta != "" {
					line += "  " + e.Meta
				}
				fmt.Println(strings.TrimRight(line, " "))
			}
			fmt.Fprintf(os.Stderr, "%d event(s)\n", len(events))
			return nil
		},
	}
	c.Flags().StringVar(&kind, "kind", "", `exact kind, or a prefix like "task.*"`)
	c.Flags().StringVar(&slug, "slug", "", "only events for this page")
	c.Flags().StringVar(&since, "since", "", "window (7d, 4w, 1m, or YYYY-MM-DD)")
	c.Flags().IntVarP(&limit, "limit", "n", 200, "maximum events")
	c.AddCommand(cmdEventsGC())
	return c
}

func cmdEventsGC() *cobra.Command {
	var days int
	c := &cobra.Command{
		Use:   "gc",
		Short: "Prune events older than --days (machine-local only; the wiki is never touched)",
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			ev, err := attention.Open(w.AttentionDBPath())
			if err != nil {
				return err
			}
			defer ev.Close()
			removed, err := ev.Prune(time.Now().Add(-time.Duration(days) * 24 * time.Hour))
			if err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(map[string]any{"removed": removed})
			}
			fmt.Printf("✓ pruned %d event(s) older than %dd\n", removed, days)
			return nil
		},
	}
	c.Flags().IntVar(&days, "days", 365, "keep events this many days")
	return c
}
