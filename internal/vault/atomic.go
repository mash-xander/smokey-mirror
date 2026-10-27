package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

func atomicWrite(path string, write func(*os.File) error) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".smokey-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := restrictFile(f.Name()); err != nil {
		return fmt.Errorf("protect temporary output: %w", err)
	}
	if err := write(f); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := commitFile(f.Name(), path); err != nil {
		return fmt.Errorf("save output (existing files are never replaced): %w", err)
	}
	return nil
}
