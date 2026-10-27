package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"smokeymirror/internal/vault"
)

func suggestOutput(input string, decrypt bool) string {
	if input == "" {
		return ""
	}
	path := vault.DefaultOutput(input, decrypt)
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	name, suffix := path, ""
	if !decrypt {
		name = strings.TrimSuffix(name, vault.Extension)
		suffix = vault.Extension
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; i <= 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s%s", stem, i, ext, suffix)
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return path
}
