package ui

import (
	"testing"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"

	"jot/internal/config"
)

func TestColoursTab(t *testing.T) {
	qutetest.Run(t, func() {
		w := newTestWindow(t, nil)
		s := w.newSettingsDialog()
		defer s.dlg.DeleteLater()
		list := qtwidgets.QListWidgetCast(findChild(s.dlg, "themeList"))
		button := func(name string) *qtwidgets.QPushButton {
			return qtwidgets.QPushButtonCast(findChild(s.dlg, name))
		}
		if list.Count() != 3 || list.CurrentItem().Text() != "Freya" {
			t.Fatalf("%d themes, current %q", list.Count(), list.CurrentItem().Text())
		}

		// Add makes a theme from the current one's background and text,
		// with the rest auto, and selects it to be named.
		qutetest.Click(button("themeAdd"))
		if list.Count() != 4 || list.CurrentItem().Text() != "New Theme" {
			t.Fatalf("after Add: %d themes, current %q", list.Count(), list.CurrentItem().Text())
		}
		list.CurrentItem().SetText("My Theme")
		if got := button("themeColourTextCurrentline").Text(); got[len(got)-6:] != "(auto)" {
			t.Errorf("a new theme's current line is %q", got)
		}

		// A colour is chosen in the colour dialog.
		qutetest.WhenModal(t, func(m *qtwidgets.QWidget) {
			d := qtwidgets.QColorDialogCast(m)
			d.SetCurrentColor(mustColor("#102030"))
			d.Accept()
		})
		qutetest.Click(button("themeColourTextBackground"))
		if got := button("themeColourTextBackground").Text(); got != "#102030" {
			t.Errorf("the background button says %q", got)
		}
		qutetest.WhenModal(t, func(m *qtwidgets.QWidget) {
			d := qtwidgets.QColorDialogCast(m)
			d.SetCurrentColor(mustColor("#ff0000"))
			d.Accept()
		})
		qutetest.Click(button("themeColourSearchMatches"))
		// Right-click makes it auto again.
		qtcore.QCoreApplicationSendEvent(button("themeColourSearchMatches"),
			qtgui.NewQContextMenuEventWithReason(qtgui.QContextMenuEventMouse, qtcore.QPoint{X: 2, Y: 2}, qtcore.QPoint{X: 2, Y: 2}))
		if got := button("themeColourSearchMatches").Text(); got != "#6c71c4 (auto)" {
			t.Errorf("after right-click, matches are %q", got)
		}

		if !s.apply() {
			t.Fatal("apply failed")
		}
		if w.cfg.EditorTheme != "my-theme" || w.cfg.Theme().Background != "#102030" || w.cfg.Theme().Name != "My Theme" {
			t.Errorf("applied %q: %+v", w.cfg.EditorTheme, w.cfg.Theme())
		}
		c, err := config.Load(w.cfgPath)
		if err != nil || len(c.Themes) != 4 || c.Themes["my-theme"].Match != "" || c.Themes["my-theme"].CurrentLine != "" {
			t.Errorf("saved %+v, %v", c.Themes["my-theme"], err)
		}
		view := w.current().edit.Viewport()
		if got := pixel(t, view, view.Width()-10, view.Height()/2); got != "#102030" {
			t.Errorf("the text's background is %s", got)
		}

		// Remove takes the selected theme off; the last cannot go.
		s.tabs.SetCurrentIndex(2)
		for list.Count() > 1 {
			qutetest.Click(button("themeRemove"))
		}
		if button("themeRemove").IsEnabled() {
			t.Error("the last theme can be removed")
		}
	})
}
