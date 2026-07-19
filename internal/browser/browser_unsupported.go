//go:build !linux && !windows && !darwin

package browser

import "errors"

func Open(string) error { return errors.New("opening a browser is unsupported on this platform") }
