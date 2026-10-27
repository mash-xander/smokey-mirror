package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
	"smokeymirror/internal/vault"
)

const Help = `Smokey Mirror — file encryption, without the fuss.

  smokey                         Open the desktop app
  smokey gui [--decrypt] [FILE]   Open a prefilled window
  smokey encrypt FILE [OPTIONS]  Create FILE.smoke
  smokey decrypt FILE [OPTIONS]  Restore the original filename
  smokey keygen OUTPUT           Create a reusable private key file
  smokey install                Install for the current Windows user
  smokey uninstall              Remove integration and installed files
  smokey version                Print the version
  smokey licenses               Show bundled open-source licenses

Options for encrypt/decrypt (before or after FILE):
  -o, --out PATH       Destination; existing files are never replaced
  -k, --key-file PATH  Use a Smokey Mirror / age X25519 private key
  --password-stdin    Read a passphrase from stdin, for automation
  --                  Treat remaining arguments as filenames

Without --key-file or --password-stdin, a hidden passphrase prompt opens.
Encryption requires 12+ characters and confirmation. Originals are kept.
Keep your key file separate from encrypted files. There is no recovery key.
`

type Options struct {
	Input, Output, KeyFile string
	PasswordStdin          bool
}

func Parse(args []string) (Options, error) {
	var o Options
	positional := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !positional {
			switch a {
			case "--":
				positional = true
				continue
			case "--password-stdin":
				if o.PasswordStdin {
					return o, errors.New("duplicate --password-stdin")
				}
				o.PasswordStdin = true
				continue
			case "-o", "--out", "-k", "--key-file":
				if i+1 == len(args) || args[i+1] == "" {
					return o, fmt.Errorf("%s requires a value", a)
				}
				i++
				if a == "-o" || a == "--out" {
					if o.Output != "" {
						return o, errors.New("duplicate output option")
					}
					o.Output = args[i]
				} else {
					if o.KeyFile != "" {
						return o, errors.New("duplicate key-file option")
					}
					o.KeyFile = args[i]
				}
				continue
			}
			if strings.HasPrefix(a, "-") {
				return o, fmt.Errorf("unknown option: %s", a)
			}
		}
		if o.Input != "" {
			return o, errors.New("select one input file")
		}
		o.Input = a
	}
	if o.Input == "" {
		return o, errors.New("missing input file")
	}
	if o.PasswordStdin && o.KeyFile != "" {
		return o, errors.New("choose either --key-file or --password-stdin")
	}
	return o, nil
}

func Run(ctx context.Context, args []string, stdin *os.File, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing command")
	}
	if args[0] == "keygen" {
		if len(args) != 2 || args[1] == "" {
			return errors.New("usage: smokey keygen OUTPUT")
		}
		if err := vault.GenerateKey(args[1]); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Created key: %s\nBack it up separately. Anyone with this key can decrypt your files.\n", args[1])
		return nil
	}
	if args[0] != "encrypt" && args[0] != "decrypt" {
		return fmt.Errorf("unknown command %q; run smokey --help", args[0])
	}
	o, err := Parse(args[1:])
	if err != nil {
		return err
	}
	decrypt := args[0] == "decrypt"
	creds := vault.Credentials{KeyFile: o.KeyFile}
	if o.PasswordStdin {
		creds.Password, err = ReadPassphrase(stdin)
	} else if o.KeyFile == "" {
		creds.Password, err = promptPassword(stdin, stderr, !decrypt)
	}
	if err != nil {
		return err
	}
	out, err := vault.Transform(ctx, vault.Request{Input: o.Input, Output: o.Output, Decrypt: decrypt, Credentials: creds})
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, out)
	return nil
}

func ReadPassphrase(r io.Reader) (string, error) {
	b, err := io.ReadAll(io.LimitReader(bufio.NewReader(r), 4097))
	if err != nil {
		return "", err
	}
	defer clear(b)
	if len(b) > 4096 {
		return "", errors.New("passphrase input exceeds 4096 bytes")
	}
	s := strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r")
	if s == "" {
		return "", errors.New("passphrase cannot be empty")
	}
	if strings.ContainsAny(s, "\r\n") {
		return "", errors.New("passphrase must be a single line")
	}
	return s, nil
}

func promptPassword(stdin *os.File, stderr io.Writer, confirm bool) (string, error) {
	if !term.IsTerminal(int(stdin.Fd())) {
		return "", errors.New("no interactive terminal; use --password-stdin or --key-file")
	}
	fmt.Fprint(stderr, "Passphrase: ")
	b, err := term.ReadPassword(int(stdin.Fd()))
	fmt.Fprintln(stderr)
	if err != nil {
		return "", err
	}
	defer clear(b)
	if confirm {
		if err := vault.ValidatePassword(string(b)); err != nil {
			return "", err
		}
		fmt.Fprint(stderr, "Confirm passphrase: ")
		other, err := term.ReadPassword(int(stdin.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", err
		}
		defer clear(other)
		if string(other) != string(b) {
			return "", errors.New("passphrases do not match")
		}
	}
	return string(b), nil
}
