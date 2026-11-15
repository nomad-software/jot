package ui

import (
	"maps"
	"slices"
	"strings"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtwidgets"

	"notepad/internal/config"
)

// themeColourFields are a theme's colours as the Colours page edits them,
// in the group each is shown under.
var themeColourFields = []struct {
	group, label string
	get          func(*config.ThemeSpec) *string
	resolved     func(config.Theme) string
}{
	{"Text", "Background", func(s *config.ThemeSpec) *string { return &s.Background }, func(t config.Theme) string { return t.Background }},
	{"Text", "Text", func(s *config.ThemeSpec) *string { return &s.Foreground }, func(t config.Theme) string { return t.Foreground }},
	{"Text", "Current line", func(s *config.ThemeSpec) *string { return &s.CurrentLine }, func(t config.Theme) string { return t.CurrentLine }},
	{"Text", "Selection", func(s *config.ThemeSpec) *string { return &s.Selection }, func(t config.Theme) string { return t.Selection }},
	{"Text", "Selected text", func(s *config.ThemeSpec) *string { return &s.SelectionText }, func(t config.Theme) string { return t.SelectionText }},
	{"Line numbers", "Background", func(s *config.ThemeSpec) *string { return &s.Gutter }, func(t config.Theme) string { return t.Gutter }},
	{"Line numbers", "Numbers", func(s *config.ThemeSpec) *string { return &s.GutterText }, func(t config.Theme) string { return t.GutterText }},
	{"Line numbers", "Current line's number", func(s *config.ThemeSpec) *string { return &s.GutterCurrent }, func(t config.Theme) string { return t.GutterCurrent }},
	{"Search", "Matches", func(s *config.ThemeSpec) *string { return &s.Match }, func(t config.Theme) string { return t.Match }},
	{"Search", "Match text", func(s *config.ThemeSpec) *string { return &s.MatchText }, func(t config.Theme) string { return t.MatchText }},
}

// themeGroups are the groups of themeColourFields, in order.
var themeGroups = []string{"Text", "Line numbers", "Search"}

// swatchStyle colours a button with the colour hex, with readable text on
// it.
func swatchStyle(hex string) string {
	c, _ := config.ParseColor(hex)
	r, g, b := c>>16&0xff, c>>8&0xff, c&0xff
	text := "#000000"
	if 299*r+587*g+114*b < 128000 {
		text = "#ffffff"
	}
	return "QPushButton { background: " + hex + "; color: " + text +
		"; border: 1px solid #808080; border-radius: 3px; padding: 2px 8px; }"
}

// themesTab builds the Settings dialog's Colours page: a list of themes to
// choose from, with an editor for the selected one. read returns the
// edited themes and the id of the selected theme, which is used from OK or
// Apply.
func (w *Window) themesTab(dlg *qtwidgets.QDialog) (page *qtwidgets.QWidget, read func() (map[string]config.ThemeSpec, string)) {
	specs := maps.Clone(w.cfg.Themes)
	ids := w.cfg.ThemeIDs()
	current := -1
	loading := false

	page = qtwidgets.NewQWidget(nil)
	outer := qtwidgets.NewQHBoxLayout(page)

	// ---- left: the list and its buttons
	left := qtwidgets.NewQVBoxLayout(nil)
	_, lf := settingsSection(left, "Themes")
	note := qtwidgets.NewQLabelWithText("The selected theme is used when you press OK or Apply.", nil)
	note.SetWordWrap(true)
	lf.AddRowWidget(note)
	// A theme is renamed in the list itself: double-click it, press F2 or
	// use Rename.
	list := qtwidgets.NewQListWidget(page)
	list.SetObjectName("themeList")
	list.SetMinimumWidth(170)
	list.SetEditTriggers(qtwidgets.QAbstractItemViewDoubleClicked | qtwidgets.QAbstractItemViewEditKeyPressed)
	lf.AddRowWidget(list)
	addBtn := qtwidgets.NewQPushButtonWithText("Add", nil)
	addBtn.SetObjectName("themeAdd")
	dupBtn := qtwidgets.NewQPushButtonWithText("Duplicate", nil)
	renameBtn := qtwidgets.NewQPushButtonWithText("Rename", nil)
	removeBtn := qtwidgets.NewQPushButtonWithText("Remove", nil)
	removeBtn.SetObjectName("themeRemove")
	grid := qtwidgets.NewQGridLayout(nil)
	grid.AddWidget(addBtn, 0, 0)
	grid.AddWidget(dupBtn, 0, 1)
	grid.AddWidget(renameBtn, 1, 0)
	grid.AddWidget(removeBtn, 1, 1)
	lf.AddRowSpanLayout(grid)
	restoreBtn := qtwidgets.NewQPushButtonWithText("Restore Built-in Themes", nil)
	lf.AddRowWidget(restoreBtn)
	outer.AddLayoutFull(left, 0)

	// ---- right: the editor
	right := qtwidgets.NewQVBoxLayout(nil)
	colourBtns := make([]*qtwidgets.QPushButton, len(themeColourFields))
	// Every label is as wide as the widest, so that the colours line up
	// from one group to the next.
	var labels []*qtwidgets.QLabel
	labelWidth := 0
	for _, group := range themeGroups {
		_, form := settingsSection(right, group)
		for i, f := range themeColourFields {
			if f.group != group {
				continue
			}
			b := qtwidgets.NewQPushButton(page)
			b.SetObjectName("themeColour" + strings.ReplaceAll(group+f.label, " ", ""))
			b.SetMinimumWidth(120)
			b.SetContextMenuPolicy(qtcore.CustomContextMenu)
			colourBtns[i] = b
			label := qtwidgets.NewQLabelWithText(f.label+":", nil)
			labelWidth = max(labelWidth, label.SizeHint().Width)
			labels = append(labels, label)
			form.AddRow(label, b)
		}
	}
	for _, l := range labels {
		l.SetMinimumWidth(labelWidth)
	}
	auto := qtwidgets.NewQLabelWithText("Colours marked auto are made from the background and text. Right-click a colour to make it auto again.", nil)
	auto.SetWordWrap(true)
	right.AddWidget(auto)
	right.AddStretch()
	outer.AddLayoutFull(right, 1)

	cur := func() *config.ThemeSpec {
		if current < 0 || current >= len(ids) {
			return nil
		}
		s := specs[ids[current]]
		return &s
	}
	store := func(s *config.ThemeSpec) { specs[ids[current]] = *s }

	refresh := func() {
		s := cur()
		if s == nil {
			return
		}
		th, _ := s.Resolve()
		for i, f := range themeColourFields {
			c := f.resolved(th)
			label := c
			if *f.get(s) == "" {
				label += " (auto)"
			}
			colourBtns[i].SetText(label)
			colourBtns[i].SetStyleSheet(swatchStyle(c))
		}
	}
	load := func(row int) {
		current = row
		refresh()
	}
	rebuildList := func(selectID string) {
		loading = true
		list.Clear()
		sel := 0
		for i, id := range ids {
			list.AddItemText(specs[id].Name)
			item := list.Item(i)
			item.SetFlags(item.Flags() | qtcore.ItemIsEditable)
			if id == selectID {
				sel = i
			}
		}
		loading = false
		list.SetCurrentRow(sel)
		load(sel)
	}
	list.OnCurrentRowChanged(func(r int) {
		if !loading && r >= 0 {
			load(r)
		}
		removeBtn.SetEnabled(len(ids) > 1)
	})
	// rename starts editing the selected theme's name in the list.
	rename := func() {
		if item := list.CurrentItem(); item != nil {
			list.EditItem(item)
		}
	}
	list.OnItemChanged(func(item *qtwidgets.QListWidgetItem) {
		r := list.Row(item)
		if loading || r < 0 || r >= len(ids) {
			return
		}
		s := specs[ids[r]]
		if text := strings.TrimSpace(item.Text()); text != "" {
			s.Name = text
			specs[ids[r]] = s
		}
		if item.Text() != s.Name { // blank, or with spaces around it
			loading = true
			item.SetText(s.Name)
			loading = false
		}
	})
	renameBtn.OnClicked(func(bool) { rename() })

	for i, f := range themeColourFields {
		colourBtns[i].OnClicked(func(bool) {
			s := cur()
			if s == nil {
				return
			}
			th, _ := s.Resolve()
			c := qtwidgets.QColorDialogGetColorFull(mustColor(f.resolved(th)), dlg, f.group+": "+f.label, 0)
			if c.IsValid() {
				*f.get(s) = c.Name()
				store(s)
				refresh()
			}
		})
		// Right-click makes a colour auto again, except the two the rest
		// are made from.
		colourBtns[i].OnCustomContextMenuRequested(func(qtcore.QPoint) {
			s := cur()
			if s == nil || f.get(s) == &s.Background || f.get(s) == &s.Foreground {
				return
			}
			*f.get(s) = ""
			store(s)
			refresh()
		})
	}

	addCopy := func(name string, from config.ThemeSpec) {
		from.Name = name
		id := config.NewThemeID(specs, name)
		specs[id] = from
		ids = append(ids, id)
		rebuildList(id)
		rename()
	}
	addBtn.OnClicked(func(bool) {
		// A new theme has the current one's background and text, and the
		// rest made from them.
		n := config.ThemeSpec{Background: "#ffffff", Foreground: "#000000"}
		if s := cur(); s != nil {
			th, _ := s.Resolve()
			n.Background, n.Foreground = th.Background, th.Foreground
		}
		addCopy("New Theme", n)
	})
	dupBtn.OnClicked(func(bool) {
		if s := cur(); s != nil {
			addCopy(s.Name+" Copy", *s)
		}
	})
	removeBtn.OnClicked(func(bool) {
		if len(ids) <= 1 || cur() == nil {
			return // at least one theme must remain
		}
		delete(specs, ids[current])
		ids = slices.Delete(ids, current, current+1)
		rebuildList(ids[min(current, len(ids)-1)])
	})
	restoreBtn.OnClicked(func(bool) {
		msg := "Restore the built-in themes? Changes made to them will be lost; your own themes are kept."
		if qtwidgets.QMessageBoxQuestion(dlg, "Restore Built-in Themes", msg) != qtwidgets.QMessageBoxYes {
			return
		}
		selected := ""
		if current >= 0 && current < len(ids) {
			selected = ids[current]
		}
		for _, id := range slices.Sorted(maps.Keys(config.BuiltinThemes)) { // the same order each time
			if _, ok := specs[id]; !ok {
				ids = append(ids, id)
			}
			specs[id] = config.BuiltinThemes[id]
		}
		rebuildList(selected)
	})
	rebuildList(w.cfg.EditorTheme)

	// read returns the edited themes. Built-in themes keep their ids; the
	// user's own are stored under ids made from their names as they are
	// now, so a renamed copy doesn't keep the id of the theme it was
	// copied from.
	read = func() (map[string]config.ThemeSpec, string) {
		out := map[string]config.ThemeSpec{}
		newID := map[string]string{}
		// A theme of the user's never takes a built-in theme's id, even one
		// removed: it would be taken for that theme, and restoring the
		// built-in ones would replace it.
		taken := maps.Clone(config.BuiltinThemes)
		for _, id := range ids {
			if _, builtin := config.BuiltinThemes[id]; builtin {
				out[id], newID[id] = specs[id], id
			}
		}
		for _, id := range ids {
			if _, builtin := config.BuiltinThemes[id]; !builtin {
				nid := config.NewThemeID(taken, specs[id].Name)
				out[nid], newID[id] = specs[id], nid
				taken[nid] = specs[id]
			}
		}
		sel := config.DefaultTheme
		if current >= 0 && current < len(ids) {
			sel = newID[ids[current]]
		}
		return out, sel
	}
	return page, read
}
