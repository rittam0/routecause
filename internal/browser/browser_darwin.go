//go:build darwin

package browser

import "os/exec"

func Open(url string) error { return exec.Command("open", url).Start() }
