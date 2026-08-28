//go:build !windows

package main

// platformMain is the non-Windows counterpart of the Windows service entry
// point. There is no Service Control Manager and no `service` subcommand here —
// on Linux the gateway is supervised by Docker/systemd as a foreground process —
// so this always declines to handle the run and main() falls through.
func platformMain() (handled bool, err error) { return false, nil }
