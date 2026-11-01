package ui

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

var (
	backgroundColor = walk.RGB(16, 19, 19)
	surfaceColor    = walk.RGB(25, 31, 30)
	borderColor     = walk.RGB(55, 68, 66)
	textColor       = walk.RGB(235, 242, 240)
	mutedColor      = walk.RGB(164, 180, 176)
	turquoiseColor  = walk.RGB(64, 224, 208)
	goldColor       = walk.RGB(255, 197, 61)
	errorColor      = walk.RGB(255, 151, 145)
	disabledColor   = walk.RGB(112, 128, 123)

	commonControls  = windows.NewLazySystemDLL("comctl32.dll")
	setSubclass     = commonControls.NewProc("SetWindowSubclass")
	removeSubclass  = commonControls.NewProc("RemoveWindowSubclass")
	defaultSubclass = commonControls.NewProc("DefSubclassProc")
	createBrush     = windows.NewLazySystemDLL("gdi32.dll").NewProc("CreateSolidBrush")
	fillRectangle   = windows.NewLazySystemDLL("user32.dll").NewProc("FillRect")
	getWindowDC     = windows.NewLazySystemDLL("user32.dll").NewProc("GetWindowDC")
	themeCallback   uintptr
	themedWindows   = map[win.HWND]*darkTheme{}
)

func init() { themeCallback = syscall.NewCallback(themeWndProc) }

type darkTheme struct {
	ui         *window
	controls   map[win.HWND]walk.Window
	brushes    map[walk.Color]win.HBRUSH
	fieldBrush *walk.SolidColorBrush
}

func applyDarkTheme(w *window) (*darkTheme, error) {
	fieldBrush, err := walk.NewSolidColorBrush(surfaceColor)
	if err != nil {
		return nil, err
	}
	t := &darkTheme{ui: w, controls: make(map[win.HWND]walk.Window), brushes: make(map[walk.Color]win.HBRUSH), fieldBrush: fieldBrush}
	var visit func(walk.Window) error
	visit = func(control walk.Window) error {
		hwnd := control.Handle()
		t.controls[hwnd] = control
		if _, exists := themedWindows[hwnd]; !exists {
			ok, _, _ := setSubclass.Call(uintptr(hwnd), themeCallback, 1, 0)
			if ok == 0 {
				return fmt.Errorf("apply window theme")
			}
			themedWindows[hwnd] = t
		}
		switch c := control.(type) {
		case *walk.LineEdit:
			c.SetBackground(fieldBrush)
			c.SetTextColor(textColor)
			c.GraphicsEffects().Clear()
			c.SendMessage(win.EM_SETMARGINS, 3, uintptr(pixels(c, 8))|uintptr(pixels(c, 8))<<16)
			win.SetWindowTheme(hwnd, syscall.StringToUTF16Ptr("DarkMode_CFD"), nil)
		case *walk.ComboBox:
			win.SetWindowTheme(hwnd, syscall.StringToUTF16Ptr("DarkMode_CFD"), nil)
		case *walk.ProgressBar:
			empty := syscall.StringToUTF16Ptr("")
			win.SetWindowTheme(hwnd, empty, empty)
			win.SetWindowLong(hwnd, win.GWL_STYLE, win.GetWindowLong(hwnd, win.GWL_STYLE)&^win.WS_BORDER)
			win.SetWindowLong(hwnd, win.GWL_EXSTYLE, win.GetWindowLong(hwnd, win.GWL_EXSTYLE)&^(win.WS_EX_CLIENTEDGE|win.WS_EX_STATICEDGE))
			win.SetWindowPos(hwnd, 0, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE|win.SWP_FRAMECHANGED)
			c.SendMessage(win.PBM_SETBARCOLOR, 0, uintptr(turquoiseColor))
			c.SendMessage(win.PBM_SETBKCOLOR, 0, uintptr(surfaceColor))
		case *walk.CheckBox:
			c.GraphicsEffects().Clear()
		case *walk.PushButton:
			c.GraphicsEffects().Clear()
		}
		if container, ok := control.(walk.Container); ok {
			for i := 0; i < container.Children().Len(); i++ {
				child := container.Children().At(i)
				// Walk's MainWindow has an internal client container that also receives notifications.
				parent := child.Parent()
				if parent != nil && parent.Handle() != hwnd {
					ph := parent.Handle()
					if _, exists := themedWindows[ph]; !exists {
						ok, _, _ := setSubclass.Call(uintptr(ph), themeCallback, 1, 0)
						if ok == 0 {
							return fmt.Errorf("apply container theme")
						}
						themedWindows[ph] = t
					}
				}
				if err := visit(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visit(w.main); err != nil {
		t.dispose()
		return nil, err
	}
	dwm := windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	dark := int32(1)
	dwm.Call(uintptr(w.main.Handle()), 20, uintptr(unsafe.Pointer(&dark)), 4)
	caption := uint32(backgroundColor)
	dwm.Call(uintptr(w.main.Handle()), 35, uintptr(unsafe.Pointer(&caption)), 4)
	return t, nil
}

func (t *darkTheme) dispose() {
	for hwnd, theme := range themedWindows {
		if theme == t {
			removeSubclass.Call(uintptr(hwnd), themeCallback, 1)
			delete(themedWindows, hwnd)
		}
	}
	for _, brush := range t.brushes {
		win.DeleteObject(win.HGDIOBJ(brush))
	}
	t.fieldBrush.Dispose()
}

func themeWndProc(hwnd win.HWND, msg uint32, wParam, lParam, id, data uintptr) uintptr {
	t := themedWindows[hwnd]
	if t != nil {
		switch msg {
		case win.WM_NOTIFY:
			// WM_NOTIFY supplies a native pointer whose lifetime is limited to this callback.
			nm := *(**win.NMCUSTOMDRAW)(unsafe.Pointer(&lParam))
			if nm.Hdr.Code == win.NM_CUSTOMDRAW && nm.DwDrawStage == win.CDDS_PREPAINT {
				switch c := t.controls[nm.Hdr.HwndFrom].(type) {
				case *walk.PushButton:
					t.paintButton(c, nm)
					return win.CDRF_SKIPDEFAULT
				case *walk.CheckBox:
					t.paintCheckBox(c, nm)
					return win.CDRF_SKIPDEFAULT
				}
			}
		case win.WM_CTLCOLORLISTBOX:
			dc := win.HDC(wParam)
			win.SetTextColor(dc, win.COLORREF(textColor))
			win.SetBkColor(dc, win.COLORREF(surfaceColor))
			return uintptr(t.brush(surfaceColor))
		case win.WM_PAINT:
			var paint func(win.HDC)
			switch c := t.controls[hwnd].(type) {
			case *walk.ComboBox:
				paint = func(dc win.HDC) { t.paintCombo(c, dc) }
			case *walk.LineEdit:
				if c.Text() == "" && !c.Focused() {
					paint = func(dc win.HDC) { t.paintPlaceholder(c, dc) }
				}
			case *walk.ProgressBar:
				paint = func(dc win.HDC) { t.paintProgress(c, dc) }
			}
			if paint != nil {
				var ps win.PAINTSTRUCT
				dc := win.BeginPaint(hwnd, &ps)
				paint(dc)
				win.EndPaint(hwnd, &ps)
				return 0
			}
		case win.WM_NCPAINT:
			if c, ok := t.controls[hwnd].(*walk.LineEdit); ok {
				t.paintEditBorder(c)
				return 0
			}
		case win.WM_TIMER:
			if c, ok := t.controls[hwnd].(*walk.ProgressBar); ok && c.MarqueeMode() {
				win.InvalidateRect(hwnd, nil, false)
				return 0
			}
		case win.WM_ERASEBKGND:
			if _, ok := t.controls[hwnd].(*walk.ComboBox); ok {
				return 1
			}
		case win.WM_NCDESTROY:
			removeSubclass.Call(uintptr(hwnd), themeCallback, id)
			delete(themedWindows, hwnd)
		}
	}
	result, _, _ := defaultSubclass.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	if t != nil && (msg == win.PBM_SETPOS || msg == win.PBM_SETMARQUEE) {
		if _, ok := t.controls[hwnd].(*walk.ProgressBar); ok {
			win.InvalidateRect(hwnd, nil, false)
		}
	}
	if t != nil && (msg == win.WM_SETFOCUS || msg == win.WM_KILLFOCUS || msg == win.WM_ENABLE) {
		win.RedrawWindow(hwnd, nil, 0, win.RDW_INVALIDATE|win.RDW_FRAME)
	}
	return result
}

func (t *darkTheme) brush(color walk.Color) win.HBRUSH {
	if b, ok := t.brushes[color]; ok {
		return b
	}
	b, _, _ := createBrush.Call(uintptr(color))
	t.brushes[color] = win.HBRUSH(b)
	return win.HBRUSH(b)
}

func (t *darkTheme) fill(dc win.HDC, r win.RECT, color walk.Color) {
	fillRectangle.Call(uintptr(dc), uintptr(unsafe.Pointer(&r)), uintptr(t.brush(color)))
}

func (t *darkTheme) rounded(dc win.HDC, r win.RECT, color walk.Color, radius int32) {
	previousBrush := win.SelectObject(dc, win.HGDIOBJ(t.brush(color)))
	previousPen := win.SelectObject(dc, win.GetStockObject(win.NULL_PEN))
	win.RoundRect(dc, r.Left, r.Top, r.Right+1, r.Bottom+1, radius, radius)
	win.SelectObject(dc, previousPen)
	win.SelectObject(dc, previousBrush)
}

func drawLabel(dc win.HDC, hwnd win.HWND, label string, r win.RECT, color walk.Color, flags uint32) {
	font := win.SendMessage(hwnd, win.WM_GETFONT, 0, 0)
	if font != 0 {
		win.SelectObject(dc, win.HGDIOBJ(font))
	}
	win.SetBkMode(dc, win.TRANSPARENT)
	win.SetTextColor(dc, win.COLORREF(color))
	win.DrawTextEx(dc, syscall.StringToUTF16Ptr(label), -1, &r, flags|win.DT_SINGLELINE|win.DT_VCENTER|win.DT_END_ELLIPSIS, nil)
}

func inset(r win.RECT, n int32) win.RECT {
	r.Left += n
	r.Top += n
	r.Right -= n
	r.Bottom -= n
	return r
}
func pixels(w walk.Window, n int) int32 { return int32(n * w.DPI() / 96) }

func controlEnabled(w walk.Window) bool {
	if !w.Enabled() {
		return false
	}
	if widget, ok := w.(walk.Widget); ok && widget.Parent() != nil {
		return controlEnabled(widget.Parent())
	}
	return true
}

func (t *darkTheme) paintButton(b *walk.PushButton, nm *win.NMCUSTOMDRAW) {
	dc := nm.Hdc
	saved := win.SaveDC(dc)
	defer win.RestoreDC(dc, saved)
	fill, foreground, edge := surfaceColor, textColor, borderColor
	accent := walk.Color(0)
	if b == t.ui.action {
		accent = turquoiseColor
	} else if b == t.ui.createKey {
		accent = goldColor
	}
	if accent != 0 {
		fill = accent
		foreground = backgroundColor
		edge = accent
	}
	if accent == goldColor {
		fill = walk.RGB(42, 36, 22)
		foreground = goldColor
		edge = walk.RGB(119, 95, 43)
	}
	enabled := controlEnabled(b)
	if !enabled {
		fill = surfaceColor
		foreground = disabledColor
		edge = borderColor
	} else {
		if nm.UItemState&win.CDIS_HOT != 0 {
			if accent == turquoiseColor {
				fill = walk.RGB(101, 232, 218)
			} else if accent == goldColor {
				fill = walk.RGB(59, 50, 30)
				edge = goldColor
			} else {
				fill = walk.RGB(38, 49, 46)
				edge = mutedColor
			}
		}
		if nm.UItemState&win.CDIS_SELECTED != 0 {
			if accent == turquoiseColor {
				fill = walk.RGB(46, 193, 178)
			} else if accent == goldColor {
				fill = walk.RGB(33, 29, 19)
			} else {
				fill = backgroundColor
			}
		}
	}
	r := nm.Rc
	t.fill(dc, r, backgroundColor)
	radius := pixels(b, 10)
	t.rounded(dc, inset(r, 1), edge, radius)
	t.rounded(dc, inset(r, 2), fill, radius-2)
	if b.Focused() && enabled {
		t.rounded(dc, inset(r, 3), foreground, radius-2)
		t.rounded(dc, inset(r, 4), fill, radius-3)
	}
	drawLabel(dc, b.Handle(), b.Text(), inset(r, 5), foreground, win.DT_CENTER)
}

func (t *darkTheme) paintCheckBox(c *walk.CheckBox, nm *win.NMCUSTOMDRAW) {
	dc := nm.Hdc
	saved := win.SaveDC(dc)
	defer win.RestoreDC(dc, saved)
	var r win.RECT
	win.GetClientRect(c.Handle(), &r)
	t.fill(dc, r, backgroundColor)
	size := pixels(c, 16)
	box := win.RECT{Left: r.Left + 1, Top: r.Top + (r.Bottom-r.Top-size)/2, Right: r.Left + 1 + size, Bottom: r.Top + (r.Bottom-r.Top+size)/2}
	edge, fg := borderColor, mutedColor
	if c.Focused() || nm.UItemState&win.CDIS_HOT != 0 {
		edge = turquoiseColor
	}
	if !controlEnabled(c) {
		fg = disabledColor
	}
	if c.Checked() {
		t.rounded(dc, box, turquoiseColor, pixels(c, 4))
		drawLabel(dc, c.Handle(), "✓", box, backgroundColor, win.DT_CENTER)
	} else {
		t.rounded(dc, box, edge, pixels(c, 4))
		t.rounded(dc, inset(box, 1), surfaceColor, pixels(c, 3))
	}
	r.Left = box.Right + pixels(c, 8)
	drawLabel(dc, c.Handle(), c.Text(), r, fg, win.DT_LEFT)
}

func (t *darkTheme) paintCombo(c *walk.ComboBox, dc win.HDC) {
	saved := win.SaveDC(dc)
	defer win.RestoreDC(dc, saved)
	var r win.RECT
	win.GetClientRect(c.Handle(), &r)
	edge, fg := borderColor, textColor
	if c.Focused() {
		edge = turquoiseColor
	}
	if !controlEnabled(c) {
		fg = disabledColor
	}
	t.fill(dc, r, backgroundColor)
	t.rounded(dc, r, edge, pixels(c, 8))
	t.rounded(dc, inset(r, 1), surfaceColor, pixels(c, 7))
	label := r
	label.Left += pixels(c, 10)
	label.Right -= pixels(c, 28)
	drawLabel(dc, c.Handle(), c.Text(), label, fg, win.DT_LEFT)
	arrow := r
	arrow.Left = arrow.Right - pixels(c, 26)
	drawLabel(dc, c.Handle(), "⌄", arrow, mutedColor, win.DT_CENTER)
}

func (t *darkTheme) paintPlaceholder(c *walk.LineEdit, dc win.HDC) {
	saved := win.SaveDC(dc)
	defer win.RestoreDC(dc, saved)
	var r win.RECT
	win.GetClientRect(c.Handle(), &r)
	t.fill(dc, r, surfaceColor)
	r.Left += pixels(c, 8)
	r.Right -= pixels(c, 8)
	drawLabel(dc, c.Handle(), c.CueBanner(), r, mutedColor, win.DT_LEFT|win.DT_NOPREFIX)
}

func (t *darkTheme) paintEditBorder(c *walk.LineEdit) {
	nativeDC, _, _ := getWindowDC.Call(uintptr(c.Handle()))
	dc := win.HDC(nativeDC)
	if dc == 0 {
		return
	}
	defer win.ReleaseDC(c.Handle(), dc)
	var r win.RECT
	win.GetWindowRect(c.Handle(), &r)
	r.Right -= r.Left
	r.Bottom -= r.Top
	r.Left = 0
	r.Top = 0
	thickness := pixels(c, 2)
	win.ExcludeClipRect(dc, thickness, thickness, r.Right-thickness, r.Bottom-thickness)
	edge := borderColor
	if c.Focused() {
		edge = turquoiseColor
	}
	t.fill(dc, r, edge)
	t.fill(dc, inset(r, 1), surfaceColor)
}

func (t *darkTheme) paintProgress(c *walk.ProgressBar, dc win.HDC) {
	var r win.RECT
	win.GetClientRect(c.Handle(), &r)
	t.fill(dc, r, backgroundColor)
	t.rounded(dc, r, surfaceColor, pixels(c, 6))
	width := r.Right - r.Left
	if c.MarqueeMode() {
		segment := max(width/5, 1)
		r.Left = int32(time.Now().UnixMilli()/12%int64(max(width+segment, 1))) - segment
		r.Right = r.Left + segment
		r.Left = max(r.Left, 0)
		r.Right = min(r.Right, width)
	} else {
		r.Right = width * int32(c.Value()) / 100
	}
	if r.Right > r.Left {
		t.rounded(dc, r, turquoiseColor, pixels(c, 6))
	}
}
