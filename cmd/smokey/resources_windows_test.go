package main

import (
	"testing"

	"github.com/lxn/win"
	"smokeymirror/internal/ui"
)

func TestEmbeddedApplicationIcon(t *testing.T) {
	module := win.GetModuleHandle(nil)
	for _, size := range []int32{16, 24, 32, 48, 64, 256} {
		icon := win.LoadImage(module, win.MAKEINTRESOURCE(ui.IconResourceID), win.IMAGE_ICON, size, size, 0)
		if icon == 0 {
			t.Fatalf("application icon is missing at %d pixels", size)
		}
		win.DestroyIcon(win.HICON(icon))
	}
}
