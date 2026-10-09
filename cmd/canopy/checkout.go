// Checkout editing: lend a page to the agent's native editor, validate
// on the way back in. Design: docs/checkout-design.md, invariants R.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/attention"
	"github.com/neutrospec/canopy/internal/checkout"
	"github.com/neutrospec/canopy/internal/mermaid"
	"github.com/neutrospec/canopy/internal/reconcile"
	"github.com/neutrospec/canopy/internal/wiki"
	"github.com/neutrospec/canopy/internal/wikilock"
	"github.com/neutrospec/canopy/internal/writeops"
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
					state := o.Status
					if state == "" {
						state = "unmodified"
					}
					if o.Modified {
						state = "modified"
					}
					fmt.Printf("%s  (%s, since %s)\n  %s\n", o.Slug, state, o.OpenedAt, o.Path)
					if o.Diagnostic != "" {
						fmt.Printf("  %s\n", o.Diagnostic)
					}
					if o.Rescue != "" {
						fmt.Printf("  rescue: %s\n", o.Rescue)
					}
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
	c := &cobra.Command{Use: "checkin <page>", Short: "Validate and apply a working copy; retain recovery until postprocessing completes", Args: cobra.ExactArgs(1)}
	c.RunE = func(cmd *cobra.Command, args []string) (retErr error) {
		r := newMutationResult("checkin", syncNow)
		defer func() {
			var stageErr *checkout.StageError
			if errors.As(retErr, &stageErr) {
				r.Stage = stageErr.Stage
			}
			retErr = r.finish(retErr)
		}()
		w, err := loadWiki()
		if err != nil {
			return err
		}
		banner(w)
		if discard {
			r.Stage = "cleanup"
			if err := checkout.Discard(w, args[0]); err != nil {
				return err
			}
			r.Result["discarded"] = args[0]
			r.noop = true
		} else {
			scan, err := wiki.Scan(w)
			if err != nil {
				return err
			}
			res, err := checkout.CheckinPending(w, scan, args[0], mermaid.NewValidator())
			if res != nil {
				r.Result = map[string]any{"rel_path": res.RelPath, "unchanged": res.Unchanged, "old_lines": res.OldLines, "new_lines": res.NewLines, "warnings": res.Warnings, "applied_hash": res.AppliedHash, "cleanup_pending": res.CleanupPending, "resumed": res.Resumed}
				if res.Applied {
					r.Applied = "applied"
				}
				r.noop = res.Unchanged
			}
			r.Repair = "preserve the listed working copy; inspect current page and checkout --list; checkin resumes only the same recorded applied hash; use --discard only after rescuing edits"
			if err != nil {
				return err
			}
			for _, warn := range res.Warnings {
				fmt.Fprintln(os.Stderr, "⚠ "+warn)
			}
			if !res.Unchanged {
				r.Stage = "lock"
				g, err := wikilock.Acquire(context.Background(), w.Root)
				if err != nil {
					return err
				}
				err = func() error {
					defer g.Unlock()
					r.Stage = "validate"
					current, err := os.ReadFile(filepath.Join(w.Root, res.RelPath))
					if err != nil {
						return err
					}
					if reconcile.HashBytes(current) != res.AppliedHash {
						return fmt.Errorf("page changed after checkin publication; retained checkout must be inspected before repair")
					}
					p, err := scan.Find(args[0])
					if err != nil {
						return err
					}
					scan, err = writeops.RunLockedWithID(w, "update", res.RelPath, p.Tags, "", reconcile.Effects{Written: []string{res.RelPath}}, res.OperationID)
					return err
				}()
				if err != nil {
					return err
				}
				r.Stage = "reindex"
				if err := writeops.Refresh(w, scan); err != nil {
					return err
				}
			}
			r.Stage = "cleanup"
			if err := checkout.Complete(w, args[0], res); err != nil {
				return err
			}
			r.Result["cleanup_pending"] = false
			r.Repair = ""
		}
		if syncNow {
			r.Sync, err = performSync(w, syncMsg)
			r.Stage = r.Sync.Stage
			if err != nil {
				r.Repair = "inspect Git status and preserve any autostash/conflicted files; repair the requested sync stage and run canopy sync; checkin has already completed its local steps"
				return err
			}
		}
		if !flagJSON {
			if discard {
				fmt.Printf("✓ discarded checkout of %s\n", args[0])
			} else if r.noop {
				fmt.Printf("no changes — working copy of %s reclaimed\n", args[0])
			} else {
				fmt.Printf("✓ checked in %s\n", args[0])
			}
			if !syncNow && !discard {
				fmt.Println("NEXT: canopy sync")
			}
		}
		return nil
	}
	c.Flags().BoolVar(&discard, "discard", false, "drop the working copy without touching the wiki")
	addSyncFlags(c, &syncNow, &syncMsg)
	return c
}
