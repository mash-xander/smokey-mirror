//go:build windows && installer

package main

import (
	"embed"
	"os"

	"smokeymirror/internal/install"
)

//go:embed payload/smokey.exe
var payload embed.FS

var version = "0.1.1-dev"

func main() {
	destination, err := install.Destination()
	if err != nil {
		install.Message("Smokey Mirror", err.Error(), true)
		os.Exit(1)
	}
	if !install.Confirm("Install Smokey Mirror", "Install Smokey Mirror for your Windows account?\n\n"+destination+"\n\nAdds a Start menu shortcut, Explorer context menu actions, and the smokey command to PATH. No administrator rights needed.") {
		return
	}
	data, err := payload.ReadFile("payload/smokey.exe")
	if err == nil {
		_, err = install.Install(data, version)
	}
	if err != nil {
		install.Message("Installation failed", err.Error(), true)
		os.Exit(1)
	}
	install.Message("Smokey Mirror is installed", "Ready. Open Smokey Mirror from the Start menu.\n\nRestart your terminal to use the smokey command. On Windows 11, Explorer actions are under Show more options.", false)
}
