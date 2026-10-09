package main

import (
	"context"
	"errors"
	"fmt"
	"net"
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
	var allowHosts []string
	c := &cobra.Command{
		Use:   "serve",
		Short: "Serve the wiki as a website (search, browsing, and edit proposals)",
		Long: `Serve renders the wiki over HTTP for humans: page views with
backlinks, hybrid search, faceted browsing, and Wikipedia-style
missing-page fallback. The body editor files proposals for an agent to
review and apply through the CLI; it never writes wiki pages directly.`,
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
			// Non-loopback binds require the auth wall (plan-2 D1/D2):
			// a public address can never serve the wiki unauthenticated.
			if !webui.IsLoopbackAddr(addr) {
				code, err := srv.EnableAuth()
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "⚠ %s is reachable beyond localhost — authentication required\n", addr)
				if code != "" {
					fmt.Fprintf(os.Stderr, "\n  최초 1회 계정 설정: 브라우저에서 /setup 을 열고 아래 설정 코드를 입력하세요\n")
					fmt.Fprintf(os.Stderr, "  ┌──────────────────────────┐\n")
					fmt.Fprintf(os.Stderr, "  │  setup code: %s    │\n", code)
					fmt.Fprintf(os.Stderr, "  └──────────────────────────┘\n")
					fmt.Fprintln(os.Stderr, "  (이 코드는 이 터미널에만 표시되며 1회만 사용됩니다)")
				}
				fmt.Fprintln(os.Stderr, "  note: 전송 암호화(TLS)는 tailscale 또는 리버스 프록시 사용을 권장합니다")
			}
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return err
			}
			defer listener.Close()
			if err := srv.ConfigureListener(listener.Addr().String(), allowHosts); err != nil {
				return err
			}
			hs := &http.Server{Addr: addr, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()
			go func() {
				<-ctx.Done()
				sctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				_ = hs.Shutdown(sctx)
			}()
			fmt.Printf("✓ serving %s on http://%s\n", w.Root, addr)
			if err := hs.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		},
	}
	c.Flags().StringVar(&addr, "addr", "localhost:8737", "listen address (non-loopback binds require authentication)")
	c.Flags().StringSliceVar(&allowHosts, "allow-host", nil, "additional trusted Host authorities for a domain/proxy, e.g. wiki.example:443; forwarded headers are ignored")
	return c
}
