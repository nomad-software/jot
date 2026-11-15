package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"

	"notepad/internal/config"
)

// recentLabels returns what the Open Recent menu lists.
func recentLabels(w *Window) []string {
	w.fillRecentMenu()
	var out []string
	for _, a := range w.recent.Actions() {
		if !a.IsSeparator() {
			out = append(out, a.Text())
		}
	}
	return out
}

// openButton returns the toolbar's Open button.
func openButton(w *Window) *qtwidgets.QToolButton {
	return qtwidgets.QToolButtonCast(w.toolbar.WidgetForAction(w.acts["open"]))
}

func TestOpenButtonHasRecentMenu(t *testing.T) {
	a := writeFile(t, "a.txt", []byte("a"))
	qutetest.Run(t, func() {
		w := newTestWindow(t, nil)
		w.openFiles([]string{a})
		b := openButton(w)
		if b.Menu() == nil || b.Menu().Ptr() != w.recent.Ptr() || b.PopupMode() != qtwidgets.QToolButtonMenuButtonPopup {
			t.Fatal("the Open button has no Open Recent menu beside it")
		}
		// Its arrow opens the menu, filled with the recent files.
		qutetest.WhenPopup(t, func(m *qtwidgets.QWidget) {
			menu := qtwidgets.QMenuCast(m)
			if got := menu.Actions()[0].Text(); !strings.HasSuffix(got, "a.txt") {
				t.Errorf("the menu's first item is %q", got)
			}
			menu.Close()
		})
		b.ShowMenu()
	})
}

func TestRecentFiles(t *testing.T) {
	a := writeFile(t, "a.txt", []byte("a"))
	b := writeFile(t, "b.txt", []byte("b"))
	qutetest.Run(t, func() {
		w := newTestWindow(t, func(c *config.Config) { c.RecentFilesMax = 2 })
		if got := recentLabels(w); len(got) != 1 || got[0] != "No Recent Files" {
			t.Errorf("empty menu: %q", got)
		}
		w.openFiles([]string{a, b})
		if got := w.recentFiles(); !slices.Equal(got, []string{b, a}) {
			t.Fatalf("recent %q", got)
		}
		labels := recentLabels(w)
		if len(labels) != 3 || !strings.HasPrefix(labels[0], "&1  ") || !strings.HasSuffix(labels[0], "b.txt") || labels[2] != "&Clear List" {
			t.Errorf("menu %q", labels)
		}

		// Saved under a new name, it is first; the oldest falls off.
		w.run("new_tab", false)
		c := writeFile(t, "c.txt", nil)
		if !w.saveTo(w.current(), c) {
			t.Fatal("not saved")
		}
		if got := w.recentFiles(); !slices.Equal(got, []string{c, b}) {
			t.Errorf("after Save As: %q", got)
		}

		// Choosing one opens it, or goes to its tab.
		w.closeDoc(w.docs[0]) // a.txt
		w.closeDoc(w.docs[0]) // b.txt
		w.fillRecentMenu()    // as opening the menu does
		w.recent.Actions()[1].Trigger()
		if w.current().path != b {
			t.Errorf("opened %q", w.current().path)
		}

		// One that has gone is said so, and taken off.
		os.Remove(c)
		answer(t, qtwidgets.QMessageBoxOk)
		w.openRecent(c)
		if got := w.recentFiles(); !slices.Equal(got, []string{b}) {
			t.Errorf("after a missing file: %q", got)
		}

		// Clear List empties it.
		recentLabels(w)
		w.recent.Actions()[len(w.recent.Actions())-1].Trigger()
		if got := w.recentFiles(); len(got) != 0 {
			t.Errorf("after Clear List: %q", got)
		}

		// None remembered hides the menu, and records nothing.
		cfg := *w.cfg
		cfg.RecentFilesMax = 0
		w.applyConfig(&cfg)
		w.openFiles([]string{a})
		if w.recent.MenuAction().IsVisible() || len(w.recentFiles()) != 0 {
			t.Error("the menu is shown, or files recorded, with none to remember")
		}
		if openButton(w).Menu() != nil {
			t.Error("the Open button has a menu with none to remember")
		}
	})
}

// TestRecentFilesAreShared checks that a window adds to the list another
// window (another process, with the same settings file) has written.
func TestRecentFilesAreShared(t *testing.T) {
	a := writeFile(t, "a.txt", []byte("a"))
	b := writeFile(t, "b.txt", []byte("b"))
	qutetest.Run(t, func() {
		w := newTestWindow(t, nil)
		w.openFiles([]string{a})
		// Another window opened b.
		if err := config.SetValues(w.cfgPath, map[string]any{"recent_files": []string{b, a}}); err != nil {
			t.Fatal(err)
		}
		w.run("new_tab", false)
		c := writeFile(t, "c.txt", nil)
		w.saveTo(w.current(), c)
		if got := w.recentFiles(); !slices.Equal(got, []string{c, b, a}) {
			t.Errorf("recent %q", got)
		}
	})
}

func TestSettingsRecentFilesMax(t *testing.T) {
	qutetest.Run(t, func() {
		w := newTestWindow(t, nil)
		w.setRecentFiles([]string{"/a", "/b", "/c"})
		s := w.newSettingsDialog()
		defer s.dlg.DeleteLater()
		qtwidgets.QSpinBoxCast(findChild(s.dlg, "recentFilesMax")).SetValue(1)
		if !s.apply() {
			t.Fatal("apply failed")
		}
		if w.cfg.RecentFilesMax != 1 || !slices.Equal(w.recentFiles(), []string{"/a"}) {
			t.Errorf("max %d, list %q", w.cfg.RecentFilesMax, w.recentFiles())
		}
	})
}

// TestSavingNewFileAddsItToRecent checks that a file named on the command
// line that is not there yet joins Open Recent once it is saved.
func TestSavingNewFileAddsItToRecent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.txt")
	qutetest.Run(t, func() {
		w := newTestWindow(t, nil, path)
		if len(w.recentFiles()) != 0 {
			t.Fatalf("recent before saving: %v", w.recentFiles())
		}
		qutetest.Type(w.current().edit, "hi")
		w.run("save", false)
		if got := w.recentFiles(); len(got) != 1 || got[0] != path {
			t.Errorf("recent = %v, want [%s]", got, path)
		}
	})
}
