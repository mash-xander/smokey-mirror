package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	for _, args := range [][]string{{"file", "-o", "out", "-k", "key"}, {"--out", "out", "--key-file", "key", "file"}} {
		o, err := Parse(args)
		if err != nil || o.Input != "file" || o.Output != "out" || o.KeyFile != "key" {
			t.Fatalf("%+v %v", o, err)
		}
	}
	if o, err := Parse([]string{"--", "-file"}); err != nil || o.Input != "-file" {
		t.Fatal(o, err)
	}
	for _, args := range [][]string{nil, {"a", "b"}, {"file", "--oops"}, {"file", "-o"}, {"file", "-k", "key", "--password-stdin"}, {"file", "-o", "a", "-o", "b"}} {
		if _, err := Parse(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestReadPassphrase(t *testing.T) {
	for _, text := range []string{"  preserve spaces  ", "  preserve spaces  \n", "  preserve spaces  \r\n"} {
		p, err := ReadPassphrase(strings.NewReader(text))
		if err != nil || p != "  preserve spaces  " {
			t.Fatal(p, err)
		}
	}
	for _, text := range []string{"", "\n", "one\ntwo", strings.Repeat("a", 4097)} {
		if _, err := ReadPassphrase(strings.NewReader(text)); err == nil {
			t.Fatal("invalid password accepted")
		}
	}
}

func TestCommands(t *testing.T) {
	dir := t.TempDir()
	key, in, out := filepath.Join(dir, "key"), filepath.Join(dir, "file"), filepath.Join(dir, "restored")
	if err := os.WriteFile(in, []byte("cli end to end"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	for _, args := range [][]string{{"keygen", key}, {"encrypt", in, "-k", key}, {"decrypt", in + ".smoke", "-o", out, "-k", key}} {
		if err := Run(context.Background(), args, nil, &stdout, &stderr); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(out)
	if err != nil || string(data) != "cli end to end" {
		t.Fatalf("%s %v", data, err)
	}
	if err := Run(context.Background(), []string{"unknown"}, nil, &stdout, &stderr); err == nil {
		t.Fatal("unknown command accepted")
	}
}
