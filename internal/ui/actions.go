package ui

import (
	"strings"

	"notepad/internal/icons"
)

// actionSpec describes one of the window's actions: what menus and the
// toolbar run, and what a keyboard shortcut can be given to.
type actionSpec struct {
	id        string // its name in the settings' keybindings
	label     string // in menus
	short     string // on the toolbar, if it is there
	icon      string
	checkable bool
}

// actionSpecs are every action, in the order the Settings dialog lists
// their shortcuts.
var actionSpecs = []actionSpec{
	{id: "new_window", label: "New &Window", short: "New Window", icon: icons.NewWindow},
	{id: "new_tab", label: "New &Tab", short: "New Tab", icon: icons.NewTab},
	{id: "open", label: "&Open…", short: "Open", icon: icons.Open},
	{id: "save", label: "&Save", short: "Save", icon: icons.Save},
	{id: "save_as", label: "Save &As…", short: "Save As", icon: icons.SaveAs},
	{id: "close_tab", label: "&Close Tab"},
	{id: "quit", label: "&Quit", short: "Quit", icon: icons.Quit},
	{id: "undo", label: "&Undo"},
	{id: "redo", label: "&Redo"},
	{id: "cut", label: "Cu&t"},
	{id: "copy", label: "&Copy"},
	{id: "paste", label: "&Paste"},
	{id: "select_all", label: "Select &All"},
	{id: "find", label: "&Find…", short: "Find", icon: icons.Find},
	{id: "replace", label: "Find and &Replace…"},
	{id: "find_next", label: "Find &Next"},
	{id: "find_prev", label: "Find Pre&vious"},
	{id: "go_to_line", label: "&Go to Line…"},
	{id: "settings", label: "&Settings…", short: "Settings", icon: icons.Settings},
	{id: "toolbar", label: "&Toolbar", checkable: true},
	{id: "line_numbers", label: "&Line Numbers", checkable: true},
	{id: "wrap_text", label: "&Wrap Text", checkable: true},
	{id: "highlight_line", label: "&Highlight Current Line", checkable: true},
	{id: "next_tab", label: "&Next Tab"},
	{id: "prev_tab", label: "&Previous Tab"},
	{id: "about", label: "&About"},
}

// menuSpecs are the menu bar's menus and the actions in each, "" standing
// for a separator.
var menuSpecs = []struct {
	title   string
	actions []string
}{
	{"&File", []string{"new_window", "new_tab", "open", recentMenuID, "", "save", "save_as", "", "close_tab", "quit"}},
	{"&Edit", []string{"undo", "redo", "", "cut", "copy", "paste", "select_all", "", "find", "replace", "find_next", "find_prev", "go_to_line", "", "settings"}},
	{"&View", []string{"toolbar", "line_numbers", "wrap_text", "highlight_line", "", "next_tab", "prev_tab"}},
	{"&Help", []string{"about"}},
}

// toolbarActions are the toolbar's buttons in order, "" standing for a
// separator.
var toolbarActions = []string{"new_tab", "", "open", "save", "save_as", "", "find", "", "settings", "", "quit"}

// plainLabel returns an action's label as the Settings dialog shows it:
// without its mnemonic or ellipsis.
func plainLabel(label string) string {
	return strings.TrimSuffix(strings.ReplaceAll(label, "&", ""), "…")
}
