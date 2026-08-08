// Checkout editing: lend a page to the agent's native editor, validate
// on the way back in. Design: docs/checkout-design.md, invariants R.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/checkout"
	"github.com/neutrospec/canopy/internal/mermaid"
	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/wiki"
)

func cmdCheckout() *cobra.Command {
	var list bool
	c := &cobra.Command{
		Use:   "checkout <page>",
		Short: "Materialize a working copy for native editing (outside the wiki; checkin brings it back)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			if list || len(args) == 0 {
				if !list && len(args) == 0 {
					return fmt.Errorf("usage: canopy checkout <page> (or --list)")
				}
				open, err := checkout.Open(w)
				if err != nil {
					return err
				}
				if flagJSON {
					return emitJSON(map[string]any{"checkouts": open})
				}
				if len(open) == 0 {
					fmt.Println("no open checkouts")
					return nil
				}
				for _, o := range open {
					state := "unmodified"
					if o.Modified {
						state = "modified"
					}
					fmt.Printf("%s  (%s, since %s)\n  %s\n", o.Slug, state, o.OpenedAt, o.Path)
				}
				return nil
			}
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			info, err := checkout.Checkout(w, scan, args[0])
			if err != nil {
				return err
			}
			// A checkout hands the agent the full page content — record it
			// like a show (H6/H7 apply; meta distinguishes it in events).
			touchAttention(w, []string{info.Slug}, attention.KindShow, "checkout")
			if flagJSON {
				return emitJSON(info)
			}
			if info.Modified {
				fmt.Fprintln(os.Stderr, "already checked out with edits in progress — continuing on the same copy")
			}
			fmt.Printf("✓ checked out %s\n", info.RelPath)
			fmt.Printf("  %s\n", info.Path)
			fmt.Println("edit that file with your own tools, then: canopy checkin " + info.Slug)
			return nil
		},
	}
	c.Flags().BoolVar(&list, "list", false, "list open checkouts (slug, state, path)")
	return c
}

func cmdCheckin() *cobra.Command {
	var discard, syncNow bool
	var syncMsg string
	c := &cobra.Command{
		Use:   "checkin <page>",
		Short: "Validate the working copy (schema, mermaid, base conflict) and write it into the wiki",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			if discard {
				if err := checkout.Discard(w, args[0]); err != nil {
					return err
				}
				if flagJSON {
					return emitJSON(map[string]any{"discarded": args[0]})
				}
				fmt.Printf("✓ discarded checkout of %s (wiki untouched)\n", args[0])
				return nil
			}
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			res, err := checkout.Checkin(w, scan, args[0], mermaid.NewValidator())
			if err != nil {
				return err
			}
			for _, warn := range res.Warnings {
				fmt.Fprintln(os.Stderr, "⚠ "+warn)
			}
			if res.Unchanged {
				if flagJSON {
					return emitJSON(res)
				}
				fmt.Printf("no changes — working copy of %s reclaimed\n", args[0])
				return nil
			}
			if !flagJSON {
				fmt.Printf("✓ checked in %s (%d → %d lines)\n", res.RelPath, res.OldLines, res.NewLines)
			}
			p := scan.BySlug[wiki.NormalizeLink(args[0])]
			if err := afterWrite(w, "update", res.RelPath, p.Tags, "", reconcile.Effects{Written: []string{res.RelPath}}, syncNow, syncMsg); err != nil {
				return err
			}
			if flagJSON {
				return emitJSON(res)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&discard, "discard", false, "drop the working copy without touching the wiki")
	c.Flags().BoolVar(&syncNow, "sync", false, "run canopy sync right after")
	c.Flags().StringVarP(&syncMsg, "message", "m", "", "commit message when using --sync")
	return c
}
