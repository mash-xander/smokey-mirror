package vault

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"filippo.io/age"
)

func fixture(t *testing.T) (string, Credentials) {
	t.Helper()
	dir := t.TempDir()
	key := filepath.Join(dir, "test.smokey-key")
	if err := GenerateKey(key); err != nil {
		t.Fatal(err)
	}
	return dir, Credentials{KeyFile: key}
}

func put(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func noTemps(t *testing.T, dir string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, ".smokey-*.tmp"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary files left behind: %v, %v", files, err)
	}
}

func TestKeyRoundTripAndAgeCompatibility(t *testing.T) {
	dir, creds := fixture(t)
	for _, size := range []int{0, 1, 65535, 65536, 65537, 3 * 1024 * 1024} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			input := filepath.Join(dir, fmt.Sprintf("данные %d.bin", size))
			plain := bytes.Repeat([]byte{0x37, 0xfe, 0, 0x81}, (size+3)/4)[:size]
			put(t, input, plain)
			encrypted, err := Transform(context.Background(), Request{Input: input, Credentials: creds})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(get(t, input), plain) {
				t.Fatal("source modified")
			}
			id, err := readKey(creds.KeyFile)
			if err != nil {
				t.Fatal(err)
			}
			reader, err := age.Decrypt(bytes.NewReader(get(t, encrypted)), id)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := io.ReadAll(reader)
			if err != nil || !bytes.Equal(decoded, plain) {
				t.Fatalf("age interoperability: %v", err)
			}
			out, err := Transform(context.Background(), Request{Input: encrypted, Output: input + ".restored", Decrypt: true, Credentials: creds})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(get(t, out), plain) {
				t.Fatal("round trip mismatch")
			}
			noTemps(t, dir)
		})
	}
}

func TestPasswordRoundTrip(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "note.txt")
	put(t, input, []byte("private message"))
	creds := Credentials{Password: "дым и зеркало — длинная фраза"}
	enc, err := Transform(context.Background(), Request{Input: input, Credentials: creds})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Transform(context.Background(), Request{Input: enc, Output: input + ".wrong", Decrypt: true, Credentials: Credentials{Password: "incorrect passphrase"}})
	if err == nil {
		t.Fatal("wrong password accepted")
	}
	if _, err := os.Stat(input + ".wrong"); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("failed decryption published plaintext")
	}
	out, err := Transform(context.Background(), Request{Input: enc, Output: input + ".restored", Decrypt: true, Credentials: creds})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(get(t, out), get(t, input)) {
		t.Fatal("password round trip mismatch")
	}
	noTemps(t, dir)
}

func TestTamperingTruncationAndTrailingData(t *testing.T) {
	dir, creds := fixture(t)
	input := filepath.Join(dir, "secret.bin")
	put(t, input, bytes.Repeat([]byte("secret"), 30000))
	enc, err := Transform(context.Background(), Request{Input: input, Credentials: creds})
	if err != nil {
		t.Fatal(err)
	}
	original := get(t, enc)
	mutated := bytes.Clone(original)
	mutated[len(mutated)-20] ^= 1
	cases := map[string][]byte{
		"empty": {}, "header": original[:16], "one-byte-short": original[:len(original)-1],
		"chunk-short": original[:len(original)-65536], "corrupt": mutated,
		"trailing": append(bytes.Clone(original), 0),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			bad, out := filepath.Join(dir, name+".smoke"), filepath.Join(dir, name+".out")
			put(t, bad, data)
			if _, err := Transform(context.Background(), Request{Input: bad, Output: out, Decrypt: true, Credentials: creds}); err == nil {
				t.Fatal("damaged file accepted")
			}
			if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("partial output published")
			}
			noTemps(t, dir)
		})
	}
}

func TestWrongKeyAndMethod(t *testing.T) {
	dir, creds := fixture(t)
	_, other := fixture(t)
	input := filepath.Join(dir, "plain")
	put(t, input, []byte("secret"))
	enc, err := Transform(context.Background(), Request{Input: input, Credentials: creds})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []Credentials{other, {Password: "a different passphrase"}} {
		if _, err := Transform(context.Background(), Request{Input: enc, Output: input + ".out", Decrypt: true, Credentials: c}); err == nil {
			t.Fatal("wrong credentials accepted")
		}
	}
	noTemps(t, dir)
}

func TestNeverOverwrite(t *testing.T) {
	dir, creds := fixture(t)
	input := filepath.Join(dir, "plain")
	put(t, input, []byte("preserve me"))
	for _, out := range []string{input, creds.KeyFile} {
		before := get(t, out)
		if _, err := Transform(context.Background(), Request{Input: input, Output: out, Credentials: creds}); err == nil {
			t.Fatal("existing file overwritten")
		}
		if !bytes.Equal(before, get(t, out)) {
			t.Fatal("existing file changed")
		}
	}
	before := get(t, creds.KeyFile)
	if err := GenerateKey(creds.KeyFile); err == nil {
		t.Fatal("key overwritten")
	}
	if !bytes.Equal(before, get(t, creds.KeyFile)) {
		t.Fatal("key changed")
	}
	noTemps(t, dir)
}

func TestCancellation(t *testing.T) {
	dir, creds := fixture(t)
	input := filepath.Join(dir, "plain")
	put(t, input, bytes.Repeat([]byte("secret"), 100000))
	enc, err := Transform(context.Background(), Request{Input: input, Credentials: creds})
	if err != nil {
		t.Fatal(err)
	}
	for _, decrypt := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		source := input
		if decrypt {
			source = enc
		}
		out := source + ".cancelled"
		_, err := Transform(ctx, Request{Input: source, Output: out, Decrypt: decrypt, Credentials: creds, Progress: func(done, total int64) {
			if done > 100000 {
				cancel()
			}
		}})
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("want cancellation, got %v", err)
		}
		if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("cancelled output published")
		}
		noTemps(t, dir)
	}
}

func TestAtomicConcurrentPublish(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "winner")
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, value := range []string{"first", "second"} {
		wg.Go(func() {
			results <- atomicWrite(path, func(f *os.File) error { _, err := io.WriteString(f, value); return err })
		})
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("expected one winner, got %d", success)
	}
	if data := string(get(t, path)); data != "first" && data != "second" {
		t.Fatal("invalid output")
	}
	noTemps(t, dir)
}

func TestValidation(t *testing.T) {
	dir, creds := fixture(t)
	input := filepath.Join(dir, "input")
	put(t, input, []byte("data"))
	for _, c := range []Credentials{{}, {Password: "short"}, {Password: strings.Repeat(" ", 20)}, {Password: "valid long password", KeyFile: creds.KeyFile}, {KeyFile: input}, {KeyFile: dir}} {
		if _, err := Transform(context.Background(), Request{Input: input, Credentials: c}); err == nil {
			t.Fatal("invalid credentials accepted")
		}
	}
	if _, err := Transform(context.Background(), Request{Input: dir, Credentials: creds}); err == nil {
		t.Fatal("directory accepted")
	}
	noTemps(t, dir)
}

func TestDefaultOutput(t *testing.T) {
	for _, tc := range []struct {
		in, out string
		decrypt bool
	}{
		{"file.txt", "file.txt.smoke", false}, {"FILE.SMOKE", "FILE", true},
		{"file.age", "file", true}, {"file", "file.decrypted", true}, {".smoke", ".smoke.decrypted", true},
	} {
		if got := DefaultOutput(tc.in, tc.decrypt); got != tc.out {
			t.Fatalf("%s: %s", tc.in, got)
		}
	}
}

func TestFailedWriteNeverPublishes(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "result")
	want := errors.New("simulated full disk")
	err := atomicWrite(target, func(f *os.File) error {
		if _, err := f.Write([]byte("partial plaintext")); err != nil {
			t.Fatal(err)
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("partial file published")
	}
	noTemps(t, dir)
}

func TestInputModificationDetected(t *testing.T) {
	dir, creds := fixture(t)
	input := filepath.Join(dir, "input")
	put(t, input, bytes.Repeat([]byte("payload"), 100000))
	changed := false
	_, err := Transform(context.Background(), Request{Input: input, Credentials: creds, Progress: func(done, total int64) {
		if changed {
			return
		}
		changed = true
		f, err := os.OpenFile(input, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("changed")); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}})
	if err == nil || !strings.Contains(err.Error(), "input changed") {
		t.Fatal(err)
	}
	if _, err := os.Stat(input + Extension); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("changed input published")
	}
	noTemps(t, dir)
}
