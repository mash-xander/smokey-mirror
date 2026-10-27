package install

import (
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type config struct {
	dir, prefix, shortcut string
}

func currentConfig() (config, error) {
	local, err := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0)
	if err != nil {
		return config{}, err
	}
	programs, err := windows.KnownFolderPath(windows.FOLDERID_Programs, 0)
	if err != nil {
		return config{}, err
	}
	return config{dir: filepath.Join(local, "Programs", "Smokey Mirror"), prefix: `Software`, shortcut: filepath.Join(programs, "Smokey Mirror.lnk")}, nil
}

func Destination() (string, error) { c, err := currentConfig(); return c.dir, err }

type entry struct{ key, name, value string }

func entries(c config, version string) []entry {
	exe := filepath.Join(c.dir, "smokey.exe")
	qexe := `"` + exe + `"`
	classes := c.prefix + `\Classes\`
	uninstall := c.prefix + `\Microsoft\Windows\CurrentVersion\Uninstall\SmokeyMirror`
	return []entry{
		{classes + `*\shell\SmokeyMirror.Encrypt`, "", "Encrypt with Smokey Mirror"},
		{classes + `*\shell\SmokeyMirror.Encrypt`, "Icon", exe + ",0"},
		{classes + `*\shell\SmokeyMirror.Encrypt\command`, "", qexe + ` gui --encrypt -- "%1"`},
		{classes + `*\shell\SmokeyMirror.Decrypt`, "", "Decrypt with Smokey Mirror"},
		{classes + `*\shell\SmokeyMirror.Decrypt`, "Icon", exe + ",0"},
		{classes + `*\shell\SmokeyMirror.Decrypt\command`, "", qexe + ` gui --decrypt -- "%1"`},
		{classes + `SmokeyMirror.File`, "", "Smokey Mirror encrypted file"},
		{classes + `SmokeyMirror.File\DefaultIcon`, "", exe + ",0"},
		{classes + `SmokeyMirror.File\shell\open\command`, "", qexe + ` gui --decrypt -- "%1"`},
		{uninstall, "DisplayName", "Smokey Mirror"},
		{uninstall, "DisplayVersion", version},
		{uninstall, "Publisher", "Smokey Mirror contributors"},
		{uninstall, "InstallLocation", c.dir},
		{uninstall, "DisplayIcon", exe},
		{uninstall, "UninstallString", qexe + " uninstall --gui"},
		{c.prefix + `\SmokeyMirror`, "InstallDir", c.dir},
	}
}

func ownKeys(c config) []string {
	p := c.prefix + `\Classes\`
	return []string{
		p + `*\shell\SmokeyMirror.Encrypt\command`, p + `*\shell\SmokeyMirror.Encrypt`,
		p + `*\shell\SmokeyMirror.Decrypt\command`, p + `*\shell\SmokeyMirror.Decrypt`,
		p + `SmokeyMirror.File\shell\open\command`, p + `SmokeyMirror.File\shell\open`, p + `SmokeyMirror.File\shell`,
		p + `SmokeyMirror.File\DefaultIcon`, p + `SmokeyMirror.File`,
		c.prefix + `\Microsoft\Windows\CurrentVersion\Uninstall\SmokeyMirror`, c.prefix + `\SmokeyMirror`,
	}
}

func writeValue(e entry) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, e.key, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(e.name, e.value)
}

func Install(binary []byte, version string) (string, error) {
	c, err := currentConfig()
	if err != nil {
		return "", err
	}
	if err = installAt(c, binary, version, true); err != nil {
		return "", err
	}
	notifyShell()
	return c.dir, nil
}

func installAt(c config, binary []byte, version string, shortcut bool) (err error) {
	if len(binary) < 2 || string(binary[:2]) != "MZ" {
		return errors.New("invalid application payload")
	}
	if _, e := os.Lstat(c.dir); !errors.Is(e, os.ErrNotExist) {
		return errors.New("installation folder already exists; uninstall the previous version first")
	}
	for _, path := range ownKeys(c) {
		k, e := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
		if e == nil {
			k.Close()
			return errors.New("an existing registration was found; uninstall the previous version first")
		}
		if !errors.Is(e, registry.ErrNotExist) {
			return e
		}
	}
	if shortcut {
		if _, e := os.Lstat(c.shortcut); !errors.Is(e, os.ErrNotExist) {
			return errors.New("a Start menu shortcut already exists")
		}
	}
	if err = os.MkdirAll(filepath.Dir(c.dir), 0755); err != nil {
		return err
	}
	if err = os.Mkdir(c.dir, 0755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			cleanupRegistration(c)
			os.Remove(filepath.Join(c.dir, "smokey.exe"))
			os.Remove(c.dir)
			if shortcut {
				os.Remove(c.shortcut)
			}
		}
	}()
	if err = os.WriteFile(filepath.Join(c.dir, "smokey.exe"), binary, 0755); err != nil {
		return err
	}
	for _, e := range entries(c, version) {
		if err = writeValue(e); err != nil {
			return err
		}
	}
	if err = changePath(c, true); err != nil {
		return err
	}
	if shortcut {
		script := "$w = New-Object -ComObject WScript.Shell; $s = $w.CreateShortcut(" + psQuote(c.shortcut) + "); $s.TargetPath = " + psQuote(filepath.Join(c.dir, "smokey.exe")) + "; $s.Arguments = 'gui'; $s.WorkingDirectory = " + psQuote(c.dir) + "; $s.IconLocation = " + psQuote(filepath.Join(c.dir, "smokey.exe")+",0") + "; $s.Save()"
		if output, e := powershell(script).CombinedOutput(); e != nil {
			return fmt.Errorf("create Start menu shortcut: %w: %s", e, output)
		}
	}
	return nil
}

func pathKey(c config) string {
	if c.prefix == "Software" {
		return "Environment"
	}
	return c.prefix + `\Environment`
}

func hasPath(path, dir string) bool {
	for _, part := range strings.Split(path, ";") {
		if strings.EqualFold(strings.TrimRight(strings.Trim(strings.TrimSpace(part), `"`), `\/`), strings.TrimRight(dir, `\/`)) {
			return true
		}
	}
	return false
}

func changePath(c config, add bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, pathKey(c), registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	path, typ, err := k.GetStringValue("Path")
	absent := errors.Is(err, registry.ErrNotExist)
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	if add {
		if hasPath(path, c.dir) {
			return nil
		}
		before := path
		if path != "" && !strings.HasSuffix(path, ";") {
			path += ";"
		}
		path += c.dir
		beforeType := fmt.Sprint(typ)
		if absent {
			beforeType = "absent"
		}
		for _, e := range []entry{
			{c.prefix + `\SmokeyMirror`, "PathBefore", before},
			{c.prefix + `\SmokeyMirror`, "PathAfter", path},
			{c.prefix + `\SmokeyMirror`, "PathBeforeType", beforeType},
			{c.prefix + `\SmokeyMirror`, "PathAdded", "1"},
		} {
			if err := writeValue(e); err != nil {
				return err
			}
		}
	} else {
		owner, e := registry.OpenKey(registry.CURRENT_USER, c.prefix+`\SmokeyMirror`, registry.QUERY_VALUE)
		if e != nil {
			return nil
		}
		added, _, e := owner.GetStringValue("PathAdded")
		before, _, _ := owner.GetStringValue("PathBefore")
		after, _, _ := owner.GetStringValue("PathAfter")
		beforeType, _, _ := owner.GetStringValue("PathBeforeType")
		owner.Close()
		if e != nil || added != "1" {
			return nil
		}
		if path == after || path == before {
			if beforeType == "absent" {
				err := k.DeleteValue("Path")
				if errors.Is(err, registry.ErrNotExist) {
					return nil
				}
				return err
			}
			if beforeType == fmt.Sprint(registry.SZ) {
				return k.SetStringValue("Path", before)
			}
			return k.SetExpandStringValue("Path", before)
		}
		parts := strings.Split(path, ";")
		kept := make([]string, 0, len(parts))
		for _, part := range parts {
			if !hasPath(part, c.dir) {
				kept = append(kept, part)
			}
		}
		path = strings.Join(kept, ";")
	}
	if typ == registry.SZ {
		return k.SetStringValue("Path", path)
	}
	return k.SetExpandStringValue("Path", path)
}

func cleanupRegistration(c config) error {
	var errs []error
	if err := changePath(c, false); err != nil {
		errs = append(errs, err)
	}
	for _, path := range ownKeys(c) {
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil && !errors.Is(err, registry.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func Uninstall() error {
	c, err := currentConfig()
	if err != nil {
		return err
	}
	if err = cleanupRegistration(c); err != nil {
		return err
	}
	if err = os.Remove(c.shortcut); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	notifyShell()
	current, err := os.Executable()
	if err != nil {
		return err
	}
	target := filepath.Join(c.dir, "smokey.exe")
	if !strings.EqualFold(filepath.Clean(current), filepath.Clean(target)) {
		if err = os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		os.Remove(c.dir)
		return nil
	}
	script := fmt.Sprintf("Wait-Process -Id %d -ErrorAction SilentlyContinue; Remove-Item -LiteralPath %s -ErrorAction Stop; if ((Get-ChildItem -LiteralPath %s -Force | Measure-Object).Count -eq 0) { Remove-Item -LiteralPath %s }", os.Getpid(), psQuote(target), psQuote(c.dir), psQuote(c.dir))
	cmd := powershell(script)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func powershell(script string) *exec.Cmd {
	script = "$ErrorActionPreference = 'Stop'; " + script
	runes := utf16.Encode([]rune(script))
	data := make([]byte, len(runes)*2)
	for i, r := range runes {
		binary.LittleEndian.PutUint16(data[i*2:], r)
	}
	cmd := exec.Command(filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-EncodedCommand", base64.StdEncoding.EncodeToString(data))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	return cmd
}

func notifyShell() {
	windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify").Call(0x08000000, 0, 0, 0)
	value, _ := windows.UTF16PtrFromString("Environment")
	var result uintptr
	windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW").Call(0xffff, 0x001A, 0, uintptr(unsafe.Pointer(value)), 2, 1000, uintptr(unsafe.Pointer(&result)))
}

func Confirm(title, message string) bool {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(message)
	r, _, _ := windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x24)
	return r == 6
}

func Message(title, message string, failed bool) {
	t, _ := windows.UTF16PtrFromString(title)
	m, _ := windows.UTF16PtrFromString(message)
	flags := uintptr(0x40)
	if failed {
		flags = 0x10
	}
	windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), flags)
}
