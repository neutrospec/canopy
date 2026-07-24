//go:build !ORT

package embed

import "errors"

func Available() bool { return false }

func New() (Engine, error) {
	return nil, errors.New("this canopy binary was built without embedding support (rebuild with -tags ORT)")
}
