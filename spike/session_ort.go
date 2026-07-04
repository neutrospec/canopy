//go:build ORT

package main

import (
	"context"

	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/options"
)

func newSession(ctx context.Context) (*hugot.Session, error) {
	return hugot.NewORTSession(ctx,
		options.WithOnnxLibraryPath("/opt/homebrew/lib"))
}
