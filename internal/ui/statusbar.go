package ui

import (
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"notepad/internal/textfile"
)

// messageTime is how long, in milliseconds, a message stays in the status
// bar before the file's path shows again.
const messageTime = 5000

// statusBar is the bar along the bottom of a window: the file's path, or a
// message for a while, on the left; on the right the cursor's line and
// column with what is selected, the encoding and the line endings, the last
// two with menus that change them; and the handle to drag the window's size
// by in the corner.
type statusBar struct {
	win  *Window
	bar  *qtwidgets.QStatusBar
	file *qtwidgets.QLabel
	pos  *qtwidgets.QLabel
	enc  *qtwidgets.QLabel
	eol  *qtwidgets.QLabel
}

// menuLabel is a status bar label that opens a menu when it is clicked,
// with either button.
type menuLabel struct {
	qtwidgets.QLabelBase
	open func(at qtcore.QPoint)
}

// MousePressEvent overrides QWidget::mousePressEvent.
func (l *menuLabel) MousePressEvent(ev *qtgui.QMouseEvent) {
	if ev.Button() == qtcore.LeftButton || ev.Button() == qtcore.RightButton {
		l.open(l.MapToGlobal(qtcore.QPoint{Y: l.Height()}))
		return
	}
	l.Super().MousePressEvent(ev)
}

func newStatusBar(w *Window) *statusBar {
	s := &statusBar{win: w}
	s.bar = qtwidgets.NewQStatusBar(nil)
	s.bar.SetObjectName("notepadStatusBar")
	s.bar.SetSizeGripEnabled(true)
	s.file = qtwidgets.NewQLabel(nil)
	s.file.SetObjectName("notepadStatusFile")
	s.file.SetTextInteractionFlags(qtcore.TextSelectableByMouse)
	s.bar.AddWidgetFull(s.file, 1)

	s.pos = qtwidgets.NewQLabel(nil)
	s.pos.SetObjectName("notepadStatusPosition")
	s.bar.AddPermanentWidget(s.pos)

	label := func(name, tip string, open func(qtcore.QPoint)) *qtwidgets.QLabel {
		ml := &menuLabel{open: open}
		l := qtwidgets.NewQLabelWith(ml, nil)
		l.SetObjectName(name)
		l.SetToolTip(tip)
		l.SetContentsMargins(qtcore.QMargins{Left: 8, Right: 4})
		setCursorShape(l, qtcore.PointingHandCursor)
		s.bar.AddPermanentWidget(l)
		return l
	}
	s.enc = label("notepadStatusEncoding", "Encoding: click to change", s.encodingMenu)
	s.eol = label("notepadStatusLineEnding", "Line endings: click to change", s.lineEndingMenu)
	return s
}

// message shows a message for a while in place of the file's path.
func (s *statusBar) message(text string) { s.bar.ShowMessageFull(text, messageTime) }

// update shows the current document's path, encoding and line endings.
func (s *statusBar) update(d *Doc) {
	if d == nil {
		return
	}
	if d.path == "" {
		s.file.SetText(d.name())
	} else {
		s.file.SetText(displayPath(d.path))
	}
	s.enc.SetText(d.encoding.Name())
	s.eol.SetText(d.lineEnding.Short())
	s.updatePosition(d)
}

// updatePosition shows the cursor's line and column, and how much is
// selected. The column counts tabs to their tab stops, as they are shown.
func (s *statusBar) updatePosition(d *Doc) {
	c := d.edit.TextCursor()
	block := c.Block()
	col := visualColumn(block.Text(), c.PositionInBlock(), s.win.cfg.TabWidth)
	chars, lines := 0, 1
	if c.HasSelection() {
		start, end := c.SelectionStart(), c.SelectionEnd()
		// Counting the characters needs the text, which is copied, so a
		// very large selection is counted in Qt's units instead, which
		// differ only for characters beyond the BMP (emoji, for one).
		if end-start <= 1<<20 {
			chars = countChars(c.SelectedText())
		} else {
			chars = end - start
		}
		first, last := d.editor.selectedBlocks(c) // the lines Tab would indent
		lines = last - first + 1
	}
	s.pos.SetText(positionText(c.BlockNumber()+1, col+1, chars, lines))
}

// encodingMenu offers the encodings to save the current document in, and
// to read its file again in.
func (s *statusBar) encodingMenu(at qtcore.QPoint) {
	d := s.win.current()
	if d == nil {
		return
	}
	menu := qtwidgets.NewQMenu(s.bar)
	menu.AddSection("Save As")
	group := qtgui.NewQActionGroup(menu)
	for _, enc := range textfile.Encodings {
		act := menu.AddActionText(enc.Name())
		act.SetCheckable(true)
		act.SetChecked(enc == d.encoding)
		act.SetActionGroup(group)
		act.OnTriggered(func(bool) { s.win.setEncoding(d, enc) })
	}
	reopen := menu.AddMenuTitle("Reopen With Encoding")
	reopen.SetEnabled(d.path != "" && statFile(d.path).exists)
	for _, enc := range textfile.Encodings {
		act := reopen.AddActionText(enc.Name())
		act.OnTriggered(func(bool) { s.win.reopen(d, enc) })
	}
	s.exec(menu, at)
}

// lineEndingMenu offers the line endings to save the current document
// with.
func (s *statusBar) lineEndingMenu(at qtcore.QPoint) {
	d := s.win.current()
	if d == nil {
		return
	}
	menu := qtwidgets.NewQMenu(s.bar)
	group := qtgui.NewQActionGroup(menu)
	for _, le := range textfile.LineEndings {
		act := menu.AddActionText(le.Name())
		act.SetCheckable(true)
		act.SetChecked(le == d.lineEnding)
		act.SetActionGroup(group)
		act.OnTriggered(func(bool) { s.win.setLineEnding(d, le) })
	}
	s.exec(menu, at)
}

// exec shows a menu at a point and deletes it once it is closed.
func (s *statusBar) exec(menu *qtwidgets.QMenu, at qtcore.QPoint) {
	menu.ExecAt(at)
	if menu.Alive() {
		menu.DeleteLater()
	}
	if d := s.win.current(); d != nil {
		d.edit.SetFocus()
	}
}
