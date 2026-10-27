package ui

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows"
	"smokeymirror/internal/vault"
)

type window struct {
	main                                  *walk.MainWindow
	form, passwords, keys                 *walk.Composite
	input, output, password, confirm, key *walk.LineEdit
	mode, method                          *walk.ComboBox
	confirmLabel                          *walk.Label
	showPassword                          *walk.CheckBox
	status                                *walk.TextLabel
	progress                              *walk.ProgressBar
	action, cancel, reveal                *walk.PushButton
	busy                                  bool
	stop                                  context.CancelFunc
	result                                string
}

func DetachConsole() { windows.NewLazySystemDLL("kernel32.dll").NewProc("FreeConsole").Call() }

func Run(input string, decrypt bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	w := &window{}
	muted := walk.RGB(92, 101, 120)
	accent := walk.RGB(74, 67, 168)
	err := (MainWindow{
		AssignTo: &w.main, Title: "Smokey Mirror", Size: Size{Width: 720, Height: 720}, MinSize: Size{Width: 650, Height: 680},
		Font: Font{Family: "Segoe UI", PointSize: 10}, Background: SolidColorBrush{Color: walk.RGB(249, 250, 253)},
		Layout: VBox{Margins: Margins{Left: 28, Top: 24, Right: 28, Bottom: 24}, Spacing: 16},
		OnDropFiles: func(paths []string) {
			if w.busy {
				return
			}
			if len(paths) != 1 {
				w.fail(errors.New("drop one file at a time"))
				return
			}
			w.chooseInput(paths[0], true)
		},
		Children: []Widget{
			Label{Text: "SM  /  FILE PRIVACY", Font: Font{PointSize: 9, Bold: true}, TextColor: accent},
			Label{Text: "Smokey Mirror", Font: Font{PointSize: 28, Bold: true}, TextColor: walk.RGB(29, 34, 51)},
			Label{Text: "Your files. A little more private.", TextColor: muted},
			Composite{AssignTo: &w.form, Layout: VBox{MarginsZero: true, Spacing: 12}, Children: []Widget{
				Composite{Layout: HBox{MarginsZero: true, Spacing: 12}, Children: []Widget{
					ComboBox{AssignTo: &w.mode, Model: []string{"Encrypt a file", "Decrypt a file"}, CurrentIndex: 0, OnCurrentIndexChanged: w.updateMode},
					ComboBox{AssignTo: &w.method, Model: []string{"Passphrase", "Key file"}, CurrentIndex: 0, OnCurrentIndexChanged: w.updateMethod},
				}},
				Label{Text: "1   Choose a file", Font: Font{Bold: true}},
				Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
					LineEdit{AssignTo: &w.input, CueBanner: "Browse, paste a path, or drop a file here", OnTextChanged: func() {
						if w.output != nil && w.mode != nil {
							w.output.SetText(suggestOutput(w.input.Text(), w.mode.CurrentIndex() == 1))
						}
					}},
					PushButton{Text: "Browse…", OnClicked: w.browseInput},
				}},
				Label{Text: "2   Choose your protection", Font: Font{Bold: true}},
				Composite{AssignTo: &w.passwords, Layout: VBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
					LineEdit{AssignTo: &w.password, PasswordMode: true, CueBanner: "Passphrase · at least 12 characters", MaxLength: 1024},
					Label{AssignTo: &w.confirmLabel, Text: "Confirm passphrase", TextColor: muted},
					LineEdit{AssignTo: &w.confirm, PasswordMode: true, CueBanner: "Type the same passphrase again", MaxLength: 1024},
					CheckBox{AssignTo: &w.showPassword, Text: "Show passphrase", OnCheckedChanged: func() {
						if w.password != nil && w.confirm != nil {
							w.password.SetPasswordMode(!w.showPassword.Checked())
							w.confirm.SetPasswordMode(!w.showPassword.Checked())
						}
					}},
				}},
				Composite{AssignTo: &w.keys, Visible: false, Layout: VBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
					Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
						LineEdit{AssignTo: &w.key, CueBanner: "Select a .smokey-key or age identity file"},
						PushButton{Text: "Browse…", OnClicked: w.browseKey},
					}},
					PushButton{Text: "Create a new key file…", OnClicked: w.generateKey},
					TextLabel{Text: "Keep a backup of your key, separate from your encrypted files. Anyone with it can decrypt them.", TextColor: muted, MinSize: Size{Width: 100}},
				}},
				Label{Text: "3   Save the result", Font: Font{Bold: true}},
				Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
					LineEdit{AssignTo: &w.output, CueBanner: "A new file, next to the original"},
					PushButton{Text: "Save as…", OnClicked: w.browseOutput},
				}},
				Label{Text: "Your original stays untouched. Existing files are never replaced.", TextColor: muted},
			}},
			VSpacer{},
			ProgressBar{AssignTo: &w.progress, MinValue: 0, MaxValue: 100},
			TextLabel{AssignTo: &w.status, Text: "Ready when you are.", TextColor: muted, MinSize: Size{Width: 100, Height: 42}, NoPrefix: true},
			Composite{Layout: HBox{MarginsZero: true, Spacing: 10}, Children: []Widget{
				PushButton{AssignTo: &w.reveal, Text: "Show result", Enabled: false, OnClicked: w.showResult},
				HSpacer{},
				PushButton{AssignTo: &w.cancel, Text: "Cancel", Enabled: false, OnClicked: func() {
					if w.stop != nil {
						w.stop()
						w.status.SetText("Cancelling safely…")
						w.cancel.SetEnabled(false)
					}
				}},
				PushButton{AssignTo: &w.action, Text: "Encrypt file", MinSize: Size{Width: 150, Height: 36}, OnClicked: w.start},
			}},
		},
	}).Create()
	if err != nil {
		return err
	}
	defer w.main.Dispose()
	if icon, err := walk.NewIconFromResourceId(1); err == nil {
		defer icon.Dispose()
		w.main.SetIcon(icon)
	}
	w.main.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		if w.busy {
			*canceled = true
			w.stop()
			w.status.SetText("Cancelling safely… Close the window once the operation stops.")
		}
	})
	if decrypt {
		w.mode.SetCurrentIndex(1)
	}
	if input != "" {
		w.chooseInput(input, false)
	}
	w.updateMode()
	w.updateMethod()
	w.main.Run()
	return nil
}

func (w *window) updateMode() {
	if w.action == nil {
		return
	}
	decrypt := w.mode.CurrentIndex() == 1
	w.confirm.SetVisible(!decrypt)
	w.confirmLabel.SetVisible(!decrypt)
	if decrypt {
		w.action.SetText("Decrypt file")
	} else {
		w.action.SetText("Encrypt file")
	}
	if w.input.Text() != "" {
		w.output.SetText(suggestOutput(w.input.Text(), decrypt))
	}
}

func (w *window) updateMethod() {
	if w.keys == nil {
		return
	}
	key := w.method.CurrentIndex() == 1
	w.keys.SetVisible(key)
	w.passwords.SetVisible(!key)
	if key {
		w.password.SetText("")
		w.confirm.SetText("")
	}
}

func (w *window) chooseInput(path string, detect bool) {
	w.input.SetText(path)
	if detect {
		lower := strings.ToLower(path)
		if strings.HasSuffix(lower, ".smoke") || strings.HasSuffix(lower, ".age") {
			w.mode.SetCurrentIndex(1)
		} else {
			w.mode.SetCurrentIndex(0)
		}
	}
	w.updateMode()
	w.status.SetText("Ready when you are.")
}

func (w *window) browseInput() {
	dlg := walk.FileDialog{Title: "Choose a file", Filter: "All files (*.*)|*.*"}
	ok, err := dlg.ShowOpen(w.main)
	if err != nil {
		w.fail(err)
	} else if ok {
		w.chooseInput(dlg.FilePath, true)
	}
}

func (w *window) browseOutput() {
	dlg := walk.FileDialog{Title: "Save to a new file", FilePath: w.output.Text(), Filter: "All files (*.*)|*.*"}
	ok, err := dlg.ShowSave(w.main)
	if err != nil {
		w.fail(err)
	} else if ok {
		w.output.SetText(dlg.FilePath)
	}
}

func (w *window) browseKey() {
	dlg := walk.FileDialog{Title: "Select your private key", Filter: "Key files (*.smokey-key;*.txt)|*.smokey-key;*.txt|All files (*.*)|*.*"}
	ok, err := dlg.ShowOpen(w.main)
	if err != nil {
		w.fail(err)
	} else if ok {
		w.key.SetText(dlg.FilePath)
	}
}

func (w *window) generateKey() {
	dlg := walk.FileDialog{Title: "Create a private key", FilePath: "my.smokey-key", Filter: "Smokey Mirror key (*.smokey-key)|*.smokey-key"}
	ok, err := dlg.ShowSave(w.main)
	if err != nil {
		w.fail(err)
		return
	}
	if !ok {
		return
	}
	if err = vault.GenerateKey(dlg.FilePath); err != nil {
		w.fail(err)
		return
	}
	w.key.SetText(dlg.FilePath)
	w.status.SetText("Key created. Back it up separately before using it for important files.")
}

func (w *window) fail(err error) {
	w.status.SetTextColor(walk.RGB(169, 43, 52))
	w.status.SetText(err.Error())
}

func (w *window) start() {
	if w.busy {
		return
	}
	req := vault.Request{Input: w.input.Text(), Output: w.output.Text(), Decrypt: w.mode.CurrentIndex() == 1}
	if w.method.CurrentIndex() == 1 {
		req.Credentials.KeyFile = w.key.Text()
	} else {
		req.Credentials.Password = w.password.Text()
		if !req.Decrypt && w.password.Text() != w.confirm.Text() {
			w.fail(errors.New("passphrases do not match"))
			return
		}
	}
	w.busy = true
	w.result = ""
	w.form.SetEnabled(false)
	w.action.SetEnabled(false)
	w.cancel.SetEnabled(true)
	w.reveal.SetEnabled(false)
	w.progress.SetValue(0)
	w.progress.SetMarqueeMode(true)
	w.status.SetTextColor(walk.RGB(92, 101, 120))
	w.status.SetText("Preparing protection…")
	ctx, cancel := context.WithCancel(context.Background())
	w.stop = cancel
	last := time.Now()
	req.Progress = func(done, total int64) {
		if time.Since(last) < 100*time.Millisecond {
			return
		}
		last = time.Now()
		percent := 0
		if total > 0 {
			percent = int(float64(done) / float64(total) * 100)
			if percent > 99 {
				percent = 99
			}
		}
		w.main.Synchronize(func() {
			w.progress.SetMarqueeMode(false)
			w.progress.SetValue(percent)
			w.status.SetText(fmt.Sprintf("Processing… %d%%", percent))
		})
	}
	go func() {
		result, err := vault.Transform(ctx, req)
		cancel()
		w.main.Synchronize(func() {
			w.busy = false
			w.stop = nil
			w.form.SetEnabled(true)
			w.action.SetEnabled(true)
			w.cancel.SetEnabled(false)
			w.progress.SetMarqueeMode(false)
			w.password.SetText("")
			w.confirm.SetText("")
			w.showPassword.SetChecked(false)
			if err != nil {
				w.progress.SetValue(0)
				if errors.Is(err, context.Canceled) {
					w.status.SetText("Cancelled. No result was saved.")
				} else {
					w.fail(err)
				}
				return
			}
			w.progress.SetValue(100)
			w.result = result
			w.reveal.SetEnabled(true)
			w.status.SetTextColor(walk.RGB(33, 119, 91))
			w.status.SetText("Done. Saved " + filepath.Base(result))
		})
	}()
}

func (w *window) showResult() {
	if w.result == "" {
		return
	}
	path, err := filepath.Abs(w.result)
	if err != nil {
		w.fail(err)
		return
	}
	cmd := exec.Command("explorer.exe", "/select,"+path)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		w.fail(err)
		return
	}
	go cmd.Wait()
}
