package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/cobra"

	"github.com/neutrospec/canopy/internal/embed"
	"github.com/neutrospec/canopy/internal/webui"
)

func cmdServe() *cobra.Command {
	var addr string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Serve the wiki as a read-only website (search-first browsing)",
		Long: `Serve renders the wiki over HTTP for humans: page views with
backlinks, hybrid search, and Wikipedia-style missing-page fallback.
It never writes — mutations still go through the CLI commands.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			w, err := loadWiki()
			if err != nil {
				return err
			}
			banner(w)
			// Engine failures degrade to keyword search, same as cmdSearch.
			var eng embed.Engine
			if embed.Available() && embed.ModelAvailable() {
				if eng, err = newEngine(); err != nil {
					fmt.Fprintf(os.Stderr, "search degrades to keyword only (%v)\n", err)
					eng = nil
				} else {
					defer eng.Close()
				}
			} else {
				fmt.Fprintln(os.Stderr, "embedding stack missing — search degrades to keyword only")
			}
			srv, err := webui.NewServer(w, eng)
			if err != nil {
				return err
			}
			hs := &http.Server{Addr: addr, Handler: srv.Handler()}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			go func() {
				<-ctx.Done()
				sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = hs.Shutdown(sctx)
			}()
			fmt.Printf("✓ serving %s on http://localhost%s\n", w.Root, addr)
			if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	c.Flags().StringVar(&addr, "addr", ":8737", "listen address")
	return c
}
