package vault

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"filippo.io/age"
)

const Extension = ".smoke"
const MinPasswordLength = 12
const scryptWorkFactor = 18

type Credentials struct {
	Password string
	KeyFile  string
}

type Request struct {
	Input, Output string
	Decrypt       bool
	Credentials   Credentials
	Progress      func(done, total int64)
}

func DefaultOutput(input string, decrypt bool) string {
	if !decrypt {
		return input + Extension
	}
	for _, suffix := range []string{Extension, ".age"} {
		if strings.HasSuffix(strings.ToLower(input), suffix) && len(filepath.Base(input)) > len(suffix) {
			return input[:len(input)-len(suffix)]
		}
	}
	return input + ".decrypted"
}

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return fmt.Errorf("use a passphrase of at least %d characters", MinPasswordLength)
	}
	if strings.TrimSpace(password) == "" {
		return errors.New("passphrase cannot contain only whitespace")
	}
	return nil
}

func (c Credentials) identity(encrypt bool) (age.Identity, age.Recipient, error) {
	if (c.Password == "") == (c.KeyFile == "") {
		return nil, nil, errors.New("choose exactly one method: passphrase or key file")
	}
	if c.KeyFile != "" {
		id, err := readKey(c.KeyFile)
		if err != nil {
			return nil, nil, err
		}
		return id, id.Recipient(), nil
	}
	if encrypt {
		if err := ValidatePassword(c.Password); err != nil {
			return nil, nil, err
		}
		r, err := age.NewScryptRecipient(c.Password)
		if err == nil {
			r.SetWorkFactor(scryptWorkFactor)
		}
		return nil, r, err
	}
	id, err := age.NewScryptIdentity(c.Password)
	if err == nil {
		id.SetMaxWorkFactor(scryptWorkFactor)
	}
	return id, nil, err
}

func readKey(path string) (*age.X25519Identity, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open key file: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16*1024 {
		return nil, errors.New("key file must be a regular age X25519 identity file smaller than 16 KiB")
	}
	ids, err := age.ParseIdentities(io.LimitReader(f, 16*1024))
	if err != nil || len(ids) != 1 {
		return nil, errors.New("invalid key file: select a key created by Smokey Mirror or age-keygen")
	}
	id, ok := ids[0].(*age.X25519Identity)
	if !ok {
		return nil, errors.New("key file must contain one X25519 private key")
	}
	return id, nil
}

func GenerateKey(path string) error {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return err
	}
	return atomicWrite(path, func(f *os.File) error {
		_, err := fmt.Fprintf(f, "# Smokey Mirror key. Keep private and back up separately.\n# public key: %s\n%s\n", id.Recipient(), id.String())
		return err
	})
}

func Transform(ctx context.Context, req Request) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if req.Input == "" {
		return "", errors.New("select an input file")
	}
	if req.Output == "" {
		req.Output = DefaultOutput(req.Input, req.Decrypt)
	}
	source, err := os.Open(req.Input)
	if err != nil {
		return "", fmt.Errorf("open input: %w", err)
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("select a regular file; archive folders before encrypting")
	}
	if _, err := os.Lstat(req.Output); !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("output already exists or is inaccessible: %s", req.Output)
	}
	id, recipient, err := req.Credentials.identity(!req.Decrypt)
	if err != nil {
		return "", err
	}
	reader := &progressReader{ctx: ctx, src: source, total: info.Size(), report: req.Progress}
	err = atomicWrite(req.Output, func(out *os.File) error {
		if req.Decrypt {
			plaintext, err := age.Decrypt(reader, id)
			if err != nil {
				return fmt.Errorf("cannot decrypt: wrong passphrase/key, unsupported file, or damaged header: %w", err)
			}
			if _, err = io.Copy(out, plaintext); err != nil {
				return fmt.Errorf("decryption failed; no output saved: %w", err)
			}
		} else {
			ciphertext, err := age.Encrypt(out, recipient)
			if err != nil {
				return err
			}
			if _, err = io.Copy(ciphertext, reader); err != nil {
				return err
			}
			if err = ciphertext.Close(); err != nil {
				return err
			}
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		after, err := source.Stat()
		if err != nil {
			return err
		}
		if after.Size() != info.Size() || !after.ModTime().Equal(info.ModTime()) {
			return errors.New("input changed during processing; try again with a stable file")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return req.Output, nil
}

type progressReader struct {
	ctx         context.Context
	src         io.Reader
	done, total int64
	report      func(int64, int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	if err := p.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := p.src.Read(b)
	p.done += int64(n)
	if p.report != nil {
		p.report(p.done, p.total)
	}
	return n, err
}
