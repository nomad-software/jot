package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"

	"jot/internal/config"
)

// pixel returns the colour at (x, y) of a picture of w, as "#rrggbb".
func pixel(t *testing.T, w qtwidgets.QWidgetLike, x, y int) string {
	t.Helper()
	img := qute.Screenshot(w)
	b := img.Bounds()
	// The picture may be at a higher pixel ratio than the widget.
	sx := b.Dx() / max(w.AsQWidget().Width(), 1)
	r, g, bl, _ := img.At(b.Min.X+x*sx, b.Min.Y+y*sx).RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, bl>>8)
}

// TestThemeSwitchRepaintsEverything switches the editor theme and the
// window theme while documents are open, and checks the colours of the
// text, its gutter, the tabs and the scroll bar, in the current tab and
// another: once, parts of each kept the old theme's colours.
func TestThemeSwitchRepaintsEverything(t *testing.T) {
	path := writeFile(t, "n.txt", []byte(strings.Repeat("short\n", 200)))
	steps := []struct{ window, editor string }{
		{"light", "solarized-dark"}, // the editor theme alone
		{"dark", "solarized-dark"},  // the window theme alone
		{"light", "solarized-light"},
		{"dark", "solarized-dark"}, // both at once
	}
	qutetest.Run(t, func() {
		w := newTestWindow(t, func(c *config.Config) { c.WindowTheme = "light"; c.EditorTheme = "solarized-light" }, path)
		w.run("new_tab", false)
		w.run("prev_tab", false)
		qutetest.Wait(20)
		for _, step := range steps {
			c := *w.cfg
			c.WindowTheme, c.EditorTheme = step.window, step.editor
			w.applyConfig(&c)
			th, _ := config.BuiltinThemes[step.editor].Resolve()
			for i := range w.docs {
				w.tabs.SetCurrentIndex(i)
				qutetest.Wait(20)
				d := w.current()
				name := fmt.Sprintf("%s/%s tab %d", step.window, step.editor, i)
				view := d.edit.Viewport()
				if got := pixel(t, view, view.Width()-10, view.Height()/2); got != th.Background {
					t.Errorf("%s: text background %s, want %s", name, got, th.Background)
				}
				if got := pixel(t, d.editor.gutter, 2, d.editor.gutter.Height()-5); got != th.Gutter {
					t.Errorf("%s: gutter %s, want %s", name, got, th.Gutter)
				}
				window := windowPalettes[step.window][qtgui.QPaletteWindow]
				bar := w.tabs.TabBar()
				other := 1 - i
				r := bar.TabRect(other)
				if got := pixel(t, bar, r.X+r.Width/2, r.Y+3); got != window {
					t.Errorf("%s: the other tab %s, want %s", name, got, window)
				}
				if d == w.docs[0] {
					sb := d.edit.VerticalScrollBar()
					if got := pixel(t, sb, sb.Width()/2, sb.Height()-30); got == "#ffffff" && step.window == "dark" {
						t.Errorf("%s: the scroll bar is still light", name)
					}
				}
			}
		}
	})
}

// TestPadding checks that the padding is in the text's colour on every
// side, and is no change to the text: Qt counts a change of the document's
// margin as one, to undo.
func TestPadding(t *testing.T) {
	qutetest.Run(t, func() {
		w := newTestWindow(t, func(c *config.Config) { c.Padding = 20 })
		d := w.current()
		th := w.cfg.Theme()
		doc := d.edit.Document()
		if doc.IsUndoAvailable() || d.modified() {
			t.Error("a new document has something to undo, or is modified")
		}
		e := d.edit
		gutter := d.editor.gutterWidth()
		for name, at := range map[string][2]int{
			"top":          {e.Width() / 2, 5},
			"left of text": {gutter + 10, e.Height() / 2},
			"bottom":       {e.Width() / 2, e.Height() - 5},
		} {
			if got := pixel(t, e, at[0], at[1]); got != th.Background {
				t.Errorf("the %s padding is %s, want %s", name, got, th.Background)
			}
		}
		if got := pixel(t, e, 5, e.Height()/2); got != th.Gutter {
			t.Errorf("the gutter is %s, want %s", got, th.Gutter)
		}
		if v := d.edit.Viewport(); v.X() != gutter+20 || v.Y() != 20 {
			t.Errorf("the text starts at %d,%d", v.X(), v.Y())
		}

		// Changing it while editing leaves the history alone.
		qutetest.Type(e, "abc")
		c := *w.cfg
		c.Padding = 4
		w.applyConfig(&c)
		e.Undo()
		if d.text() != "" || doc.IsUndoAvailable() {
			t.Errorf("one undo left %q, more to undo %v", d.text(), doc.IsUndoAvailable())
		}
	})
}
