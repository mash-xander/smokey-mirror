package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"smokeymirror/internal/cli"
	"smokeymirror/internal/install"
	"smokeymirror/internal/ui"
)

var version = "0.1.0-dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Smokey Mirror:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		ui.DetachConsole()
		return ui.Run("", false)
	}
	switch args[0] {
	case "install":
		if len(args) != 1 {
			return fmt.Errorf("usage: smokey install")
		}
		self, err := os.Executable()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(self)
		if err != nil {
			return err
		}
		dir, err := install.Install(data, version)
		if err == nil {
			fmt.Println("Installed to " + dir + ". Restart your terminal to refresh PATH.")
		}
		return err
	case "uninstall":
		gui := len(args) == 2 && args[1] == "--gui"
		if len(args) != 1 && !gui {
			return fmt.Errorf("usage: smokey uninstall")
		}
		if gui {
			ui.DetachConsole()
			if !install.Confirm("Uninstall Smokey Mirror", "Remove Smokey Mirror and its Windows integration? Your encrypted files and keys will be kept.") {
				return nil
			}
		}
		err := install.Uninstall()
		if gui {
			if err != nil {
				install.Message("Uninstall failed", err.Error(), true)
			} else {
				install.Message("Smokey Mirror", "Windows integration removed. Installed files will be removed when this window closes.", false)
			}
		} else if err == nil {
			fmt.Println("Uninstalled. Encrypted files and keys are unchanged.")
		}
		return err
	case "help", "--help", "-h":
		fmt.Print(cli.Help)
		return nil
	case "version", "--version":
		fmt.Println("Smokey Mirror " + version)
		return nil
	case "gui":
		decrypt, input, literal := false, "", false
		for _, a := range args[1:] {
			if !literal {
				if a == "--" {
					literal = true
					continue
				}
				if a == "--decrypt" {
					decrypt = true
					continue
				}
				if a == "--encrypt" {
					decrypt = false
					continue
				}
				if strings.HasPrefix(a, "-") {
					return fmt.Errorf("unknown GUI option: %s", a)
				}
			}
			if input != "" {
				return fmt.Errorf("select one file")
			}
			input = a
		}
		ui.DetachConsole()
		return ui.Run(input, decrypt)
	}
	return cli.Run(ctx, args, os.Stdin, os.Stdout, os.Stderr)
}
