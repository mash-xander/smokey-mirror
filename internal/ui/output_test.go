package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSuggestOutput(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "report.txt")
	if err := os.WriteFile(input, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := suggestOutput(input+".smoke", true); got != filepath.Join(dir, "report (2).txt") {
		t.Fatal(got)
	}
	if err := os.WriteFile(input+".smoke", []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := suggestOutput(input, false); got != filepath.Join(dir, "report (2).txt.smoke") {
		t.Fatal(got)
	}
	if got := suggestOutput("", false); got != "" {
		t.Fatal(got)
	}
}
