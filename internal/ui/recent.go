package ui

import (
	"fmt"
	"strings"

	"github.com/nomad-software/qute/qtwidgets"

	"notepad/internal/config"
)

// recentMenuID stands in menuSpecs for the Open Recent menu.
const recentMenuID = "@recent"

// The recent files are kept in the settings file. Each window is a process
// of its own, so the list is read from the file whenever it is shown or
// changed, rather than kept from start-up, which would lose the files the
// other windows opened.

// recentFiles returns the files opened lately, the latest first.
func (w *Window) recentFiles() []string {
	c, _ := config.Load(w.cfgPath) // what loads is used, problems or not
	return config.TrimRecent(c.RecentFiles, w.cfg.RecentFilesMax)
}

// setRecentFiles writes the list of recent files to the settings file.
func (w *Window) setRecentFiles(list []string) {
	if err := config.SetValues(w.cfgPath, map[string]any{"recent_files": list}); err != nil {
		w.message("The recent files could not be saved: " + err.Error())
		return
	}
	w.cfg.RecentFiles = list
}

// addRecent puts a file at the top of the recent files.
func (w *Window) addRecent(path string) {
	if w.cfg.RecentFilesMax > 0 {
		w.setRecentFiles(config.AddRecent(w.recentFiles(), path, w.cfg.RecentFilesMax))
	}
}

// makeRecentMenu adds the Open Recent menu to menu. It is filled each time
// it opens.
func (w *Window) makeRecentMenu(menu *qtwidgets.QMenu) {
	w.recent = menu.AddMenuTitle("Open &Recent")
	w.recent.SetObjectName("notepadRecentMenu")
	w.recent.OnAboutToShow(w.fillRecentMenu)
	w.fillRecentMenu()
}

// fillRecentMenu lists the recent files in the Open Recent menu, numbered,
// with the folder each is in, and Clear List beneath them.
func (w *Window) fillRecentMenu() {
	m := w.recent
	m.Clear()
	m.MenuAction().SetVisible(w.cfg.RecentFilesMax > 0)
	list := w.recentFiles()
	if len(list) == 0 {
		m.AddActionText("No Recent Files").SetEnabled(false)
		return
	}
	for i, path := range list {
		label := strings.ReplaceAll(displayPath(path), "&", "&&")
		if i < 9 {
			label = fmt.Sprintf("&%d  %s", i+1, label)
		} else {
			label = "    " + label
		}
		m.AddActionText(label).OnTriggered(func(bool) { w.openRecent(path) })
	}
	m.AddSeparator()
	m.AddActionText("&Clear List").OnTriggered(func(bool) { w.setRecentFiles([]string{}) })
}

// openRecent opens a recent file. One that is no longer there is said so
// and taken off the list, rather than opened empty as a new file.
func (w *Window) openRecent(path string) {
	if !statFile(path).exists {
		w.setRecentFiles(config.RemoveRecent(w.recentFiles(), path))
		w.warn(displayPath(path) + " no longer exists.")
		return
	}
	w.openFiles([]string{path})
}
