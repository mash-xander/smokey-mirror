//go:build !windows

package ui

import "errors"

func DetachConsole() {}
func Run(string, bool) error {
	return errors.New("the desktop interface requires Windows; use the CLI commands on this platform")
}
