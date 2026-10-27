//go:build !windows

package vault

import "os"

func restrictFile(path string) error { return os.Chmod(path, 0600) }

func commitFile(source, target string) error { return os.Link(source, target) }
