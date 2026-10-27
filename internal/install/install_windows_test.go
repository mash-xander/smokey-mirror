package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows/registry"
)

func testConfig(t *testing.T) config {
	t.Helper()
	dir := t.TempDir()
	c := config{dir: filepath.Join(dir, "Smokey Mirror"), prefix: fmt.Sprintf(`Software\SmokeyMirrorTests\%d`, time.Now().UnixNano()), shortcut: filepath.Join(dir, "Smokey Mirror.lnk")}
	t.Cleanup(func() {
		deleteTree(t, c.prefix)
		registry.DeleteKey(registry.CURRENT_USER, `Software\SmokeyMirrorTests`)
	})
	return c
}

func deleteTree(t *testing.T, path string) {
	t.Helper()
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.READ)
	if errors.Is(err, registry.ErrNotExist) {
		return
	}
	if err != nil {
		t.Error(err)
		return
	}
	names, err := k.ReadSubKeyNames(-1)
	k.Close()
	if err != nil {
		t.Error(err)
		return
	}
	for _, name := range names {
		deleteTree(t, path+`\`+name)
	}
	if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
		t.Error(err)
	}
}

func TestInstallAndRemoveRegistration(t *testing.T) {
	c := testConfig(t)
	before := `%USERPROFILE%\bin;C:\Tools`
	k, _, err := registry.CreateKey(registry.CURRENT_USER, pathKey(c), registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	if err := k.SetExpandStringValue("Path", before); err != nil {
		t.Fatal(err)
	}
	if err := installAt(c, []byte("MZtest payload"), "0.1.0", true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(c.shortcut); err != nil {
		t.Fatal(err)
	}
	for _, e := range entries(c, "0.1.0") {
		r, err := registry.OpenKey(registry.CURRENT_USER, e.key, registry.QUERY_VALUE)
		if err != nil {
			t.Fatal(err)
		}
		got, _, err := r.GetStringValue(e.name)
		r.Close()
		if err != nil || got != e.value {
			t.Fatalf("registry mismatch: %q %v", got, err)
		}
	}
	path, _, err := k.GetStringValue("Path")
	if err != nil || !hasPath(path, c.dir) {
		t.Fatalf("PATH not installed: %q %v", path, err)
	}
	if err := installAt(c, []byte("MZother"), "0.1.0", false); err == nil {
		t.Fatal("overwrote existing install")
	}
	if err := cleanupRegistration(c); err != nil {
		t.Fatal(err)
	}
	path, typ, err := k.GetStringValue("Path")
	if err != nil || path != before || typ != registry.EXPAND_SZ {
		t.Fatalf("PATH not restored: %q %v", path, err)
	}
	for _, key := range ownKeys(c) {
		r, err := registry.OpenKey(registry.CURRENT_USER, key, registry.QUERY_VALUE)
		if err == nil {
			r.Close()
			t.Fatalf("key left behind: %s", key)
		}
	}
}

func TestPreserveExistingPath(t *testing.T) {
	c := testConfig(t)
	before := `C:\Other;"` + c.dir + `";C:\Last`
	if err := writeValue(entry{pathKey(c), "Path", before}); err != nil {
		t.Fatal(err)
	}
	if err := installAt(c, []byte("MZtest"), "0.1.0", false); err != nil {
		t.Fatal(err)
	}
	if err := cleanupRegistration(c); err != nil {
		t.Fatal(err)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, pathKey(c), registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	path, typ, err := k.GetStringValue("Path")
	if err != nil || path != before || typ != registry.SZ {
		t.Fatalf("existing PATH changed: %q %v", path, err)
	}
}

func TestRollbackOnShortcutFailure(t *testing.T) {
	c := testConfig(t)
	c.shortcut = filepath.Join(filepath.Dir(c.dir), "missing", "fail.lnk")
	if err := installAt(c, []byte("MZtest"), "0.1.0", true); err == nil {
		t.Fatal("expected shortcut failure")
	}
	if _, err := os.Stat(c.dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial installation left behind")
	}
	for _, key := range ownKeys(c) {
		r, err := registry.OpenKey(registry.CURRENT_USER, key, registry.QUERY_VALUE)
		if err == nil {
			r.Close()
			t.Fatal("registration not rolled back")
		}
	}
}

func TestQuotingAndExplorerCommands(t *testing.T) {
	if psQuote(`a'b $x`) != `'a''b $x'` {
		t.Fatal("unsafe PowerShell quoting")
	}
	c := config{dir: `C:\Users\Test User\Smokey Mirror`, prefix: "Software"}
	for _, e := range entries(c, "0.1.0") {
		if strings.HasSuffix(e.key, `\command`) && (!strings.HasPrefix(e.value, `"C:\Users\Test User\Smokey Mirror\smokey.exe" gui `) || !strings.HasSuffix(e.value, ` -- "%1"`)) {
			t.Fatal("unsafe shell command", e.value)
		}
	}
}
