package ui

import (
	"fmt"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"notepad/internal/icons"
)

// maxHighlights is how many of a search's matches are highlighted at most,
// so that a search for "e" in a large file stays quick.
const maxHighlights = 2000

// findBar is the bar along the bottom of a window that finds text in the
// current document and, opened for replacing, replaces it.
type findBar struct {
	win        *Window
	box        *qtwidgets.QWidget
	find       *qtwidgets.QLineEdit
	replace    *qtwidgets.QLineEdit
	replaceRow *qtwidgets.QWidget
	matchCase  *qtwidgets.QCheckBox
	wholeWords *qtwidgets.QCheckBox
	count      *qtwidgets.QLabel
	// refresh highlights the matches again a moment after the text
	// changes, rather than at every key.
	refresh *qtcore.QTimer
	// highlighted is the document whose matches are highlighted.
	highlighted *Doc
	// more is whether it has more matches than are highlighted.
	more bool
}

// fieldKeys handles the keys of the bar's text fields: Escape closes the
// bar, and Enter finds the next match (with Shift, the previous one).
type fieldKeys struct {
	qtwidgets.QLineEditBase
	bar     *findBar
	replace bool
}

// KeyPressEvent overrides QWidget::keyPressEvent.
func (f *fieldKeys) KeyPressEvent(ev *qtgui.QKeyEvent) {
	switch ev.Key() {
	case qtcore.Key_Escape:
		f.bar.close()
		return
	case qtcore.Key_Return, qtcore.Key_Enter:
		switch {
		case f.replace && ev.Modifiers()&qtcore.ControlModifier != 0:
			f.bar.replaceAll()
		case f.replace:
			f.bar.replaceOne()
		default:
			f.bar.next(ev.Modifiers()&qtcore.ShiftModifier == 0)
		}
		return
	}
	f.Super().KeyPressEvent(ev)
}

func newFindBar(w *Window) *findBar {
	b := &findBar{win: w}
	b.box = qtwidgets.NewQWidget(nil)
	b.box.SetObjectName("notepadFindBar")
	outer := qtwidgets.NewQVBoxLayout(b.box)
	outer.SetContentsMargins(qtcore.QMargins{Left: 8, Top: 6, Right: 8, Bottom: 6})
	outer.SetSpacing(6)

	field := func(placeholder string, replace bool) *qtwidgets.QLineEdit {
		e := qtwidgets.NewQLineEditWith(&fieldKeys{bar: b, replace: replace}, nil)
		e.SetPlaceholderText(placeholder)
		e.SetClearButtonEnabled(true)
		e.SetMinimumWidth(220)
		return e
	}
	button := func(text string, fn func()) *qtwidgets.QPushButton {
		p := qtwidgets.NewQPushButtonWithText(text, nil)
		p.SetFocusPolicy(qtcore.TabFocus)
		p.OnClicked(func(bool) { fn() })
		return p
	}

	// Find row.
	row := qtwidgets.NewQHBoxLayout(nil)
	row.SetSpacing(6)
	b.find = field("Find", false)
	b.find.SetObjectName("notepadFindField")
	b.find.OnTextEdited(func(string) { b.incremental() })
	row.AddWidget(b.find)
	row.AddWidget(button("Previous", func() { b.next(false) }))
	row.AddWidget(button("Next", func() { b.next(true) }))
	b.matchCase = qtwidgets.NewQCheckBoxWithText("Match case", nil)
	b.wholeWords = qtwidgets.NewQCheckBoxWithText("Whole words", nil)
	for _, c := range []*qtwidgets.QCheckBox{b.matchCase, b.wholeWords} {
		c.SetFocusPolicy(qtcore.TabFocus)
		c.OnToggled(func(bool) { b.incremental() })
		row.AddWidget(c)
	}
	b.count = qtwidgets.NewQLabel(nil)
	row.AddWidgetFull(b.count, 1, 0)
	closeButton := qtwidgets.NewQToolButton(nil)
	closeButton.SetAutoRaise(true)
	closeButton.SetFocusPolicy(qtcore.NoFocus)
	closeButton.SetIcon(icon(icons.Close))
	closeButton.SetIconSize(qtcore.QSize{Width: 14, Height: 14})
	closeButton.SetToolTip("Close (Escape)")
	closeButton.OnClicked(func(bool) { b.close() })
	row.AddWidget(closeButton)
	outer.AddLayout(row)

	// Replace row.
	b.replaceRow = qtwidgets.NewQWidget(nil)
	rrow := qtwidgets.NewQHBoxLayout(b.replaceRow)
	rrow.SetContentsMargins(qtcore.QMargins{})
	rrow.SetSpacing(6)
	b.replace = field("Replace with", true)
	rrow.AddWidget(b.replace)
	rrow.AddWidget(button("Replace", b.replaceOne))
	rrow.AddWidget(button("Replace All", b.replaceAll))
	rrow.AddStretch()
	outer.AddWidget(b.replaceRow)

	b.refresh = qtcore.NewQTimer(b.box)
	b.refresh.SetSingleShot(true)
	b.refresh.SetInterval(250)
	b.refresh.OnTimeout(b.highlight)

	b.box.SetVisible(false)
	return b
}

// visible reports whether the bar is open.
func (b *findBar) visible() bool { return b.box.IsVisible() }

// open opens the bar, with the replace row if replace, for the current
// document, starting from the text selected in it if that is on one line.
func (b *findBar) open(replace bool) {
	b.show(replace)
	if d := b.win.current(); d != nil {
		if c := d.edit.TextCursor(); c.HasSelection() && !d.editor.selectionSpansLines(c) {
			b.find.SetText(c.SelectedText())
		}
	}
	b.find.SetFocus()
	b.find.SelectAll()
	b.highlight()
}

// show shows the bar, with the replace row if replace, leaving the focus
// where it is.
func (b *findBar) show(replace bool) {
	b.box.SetVisible(true)
	b.replaceRow.SetVisible(replace)
}

// close closes the bar, takes the highlights off, and gives the focus back
// to the document.
func (b *findBar) close() {
	b.box.SetVisible(false)
	b.refresh.Stop()
	b.clearHighlights()
	b.setNotFound(false)
	if d := b.win.current(); d != nil {
		d.edit.SetFocus()
	}
}

func (b *findBar) clearHighlights() {
	if b.highlighted != nil {
		b.highlighted.editor.setMatches(nil)
		b.highlighted = nil
	}
	b.count.SetText("")
}

// flags are the search's options as Qt takes them.
func (b *findBar) flags() qtgui.QTextDocumentFindFlags {
	var f qtgui.QTextDocumentFindFlags
	if b.matchCase.IsChecked() {
		f |= qtgui.QTextDocumentFindCaseSensitively
	}
	if b.wholeWords.IsChecked() {
		f |= qtgui.QTextDocumentFindWholeWords
	}
	return f
}

// setNotFound shows the search field's text in red when there is no match.
func (b *findBar) setNotFound(on bool) {
	setBoolProperty(b.find, "notFound", on)
	b.find.Style().Unpolish(b.find)
	b.find.Style().Polish(b.find)
}

// incremental searches as the text is typed: from the start of the
// selection, so that the match grows as the word is typed.
func (b *findBar) incremental() {
	d := b.win.current()
	if d == nil {
		return
	}
	c := d.edit.TextCursor()
	c.SetPosition(c.SelectionStart())
	d.edit.SetTextCursor(c)
	b.next(true)
	b.highlight()
}

// next selects the next match after the selection, or before it, going
// round past the end (or start) of the document.
func (b *findBar) next(forward bool) {
	d := b.win.current()
	text := b.find.Text()
	if d == nil {
		return
	}
	if text == "" {
		b.setNotFound(false)
		return
	}
	flags := b.flags()
	if !forward {
		flags |= qtgui.QTextDocumentFindBackward
	}
	doc := d.edit.Document()
	found := doc.FindWithTextCursorFull(text, d.edit.TextCursor(), flags)
	if found.IsNull() {
		from := 0
		if !forward {
			from = doc.CharacterCount() - 1
		}
		found = doc.FindFull(text, from, flags)
		if !found.IsNull() {
			b.win.message("Search wrapped around")
		}
	}
	b.setNotFound(found.IsNull())
	if found.IsNull() {
		b.win.message(fmt.Sprintf("%q not found", text))
		return
	}
	d.edit.SetTextCursor(found)
	d.edit.CenterCursor()
	b.showCount(d)
}

// highlight highlights the current document's matches, up to
// maxHighlights.
func (b *findBar) highlight() {
	b.clearHighlights()
	d := b.win.current()
	text := b.find.Text()
	if d == nil || text == "" || !b.visible() {
		b.setNotFound(false)
		return
	}
	doc := d.edit.Document()
	flags := b.flags()
	var cursors []*qtgui.QTextCursor
	// One more than are shown is looked for, to tell whether there are more.
	for c := doc.FindFull(text, 0, flags); !c.IsNull() && len(cursors) <= maxHighlights; c = doc.FindWithTextCursorFull(text, c, flags) {
		cursors = append(cursors, c)
	}
	b.more = len(cursors) > maxHighlights
	if b.more {
		cursors = cursors[:maxHighlights]
	}
	d.editor.setMatches(cursors)
	b.highlighted = d
	b.setNotFound(len(cursors) == 0)
	b.showCount(d)
}

// showCount shows how many matches there are, and which the selection is.
func (b *findBar) showCount(d *Doc) {
	if b.highlighted != d {
		return
	}
	current := 0
	sel := d.edit.TextCursor()
	for i, m := range d.editor.matches {
		c := m.Cursor()
		if c.SelectionStart() == sel.SelectionStart() && c.SelectionEnd() == sel.SelectionEnd() {
			current = i + 1
			break
		}
	}
	b.count.SetText(matchCount(current, len(d.editor.matches), b.more))
}

// textChanged is told that a document's text has changed: its highlights
// are found again shortly.
func (b *findBar) textChanged(d *Doc) {
	if b.visible() && d == b.win.current() && b.find.Text() != "" {
		b.refresh.Start()
	}
}

// currentChanged is told that another document is the current one.
func (b *findBar) currentChanged() {
	if b.visible() {
		b.highlight()
	} else {
		b.clearHighlights()
	}
}

// selectionMatches reports whether the selection is a match of the search,
// as Replace replaces it only then.
func (b *findBar) selectionMatches(d *Doc) bool {
	c := d.edit.TextCursor()
	if !c.HasSelection() {
		return false
	}
	// The search itself says, so that Match case and Whole words are
	// heeded as they are when finding.
	m := d.edit.Document().FindFull(b.find.Text(), c.SelectionStart(), b.flags())
	return !m.IsNull() && m.SelectionStart() == c.SelectionStart() && m.SelectionEnd() == c.SelectionEnd()
}

// replaceOne replaces the selected match and selects the next one.
func (b *findBar) replaceOne() {
	d := b.win.current()
	if d == nil || b.find.Text() == "" {
		return
	}
	if b.selectionMatches(d) {
		c := d.edit.TextCursor()
		c.InsertText(b.replace.Text())
		d.edit.SetTextCursor(c)
	}
	b.next(true)
	b.highlight()
}

// replaceAll replaces every match, as one change to undo.
func (b *findBar) replaceAll() {
	d := b.win.current()
	text, with := b.find.Text(), b.replace.Text()
	if d == nil || text == "" {
		return
	}
	doc := d.edit.Document()
	flags := b.flags()
	edit := d.edit.TextCursor()
	edit.BeginEditBlock()
	n := 0
	// Each search starts after the text put in, so that a replacement that
	// contains what is searched for is not replaced again.
	for c := doc.FindFull(text, 0, flags); !c.IsNull(); c = doc.FindWithTextCursorFull(text, c, flags) {
		c.InsertText(with)
		n++
	}
	edit.EndEditBlock()
	b.highlight()
	if n == 0 {
		b.win.message(fmt.Sprintf("%q not found", text))
		return
	}
	b.win.message(fmt.Sprintf("Replaced %s", plural(n, "occurrence", "occurrences")))
}
