//go:build !ORT

package main

import (
	"context"

	"github.com/knights-analytics/hugot"
)

func newSession(ctx context.Context) (*hugot.Session, error) {
	return hugot.NewGoSession(ctx)
}
