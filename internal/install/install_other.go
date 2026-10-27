//go:build !windows

package install

import "errors"

func Destination() (string, error)           { return "", errors.New("installation requires Windows") }
func Install([]byte, string) (string, error) { return "", errors.New("installation requires Windows") }
func Uninstall() error                       { return errors.New("uninstallation requires Windows") }
func Confirm(string, string) bool            { return false }
func Message(string, string, bool)           {}
