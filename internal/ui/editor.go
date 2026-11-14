package ui

import (
	"math"
	"strconv"
	"strings"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"jot/internal/config"
)

// gutterPadding is the room, in pixels, on each side of the line numbers.
const gutterPadding = 8

// textEditor is a document's text: Qt's QPlainTextEdit, which lays out and
// paints only the lines in view, so that large files stay quick, with line
// numbers in a gutter down its left side, the line the cursor is on
// highlighted, the search's matches highlighted, and Tab indenting with
// spaces or tabs as the settings say.
type textEditor struct {
	qtwidgets.QPlainTextEditBase
	edit   *qtwidgets.QPlainTextEdit
	gutter *gutterWidget

	theme         config.Theme
	showNumbers   bool
	highlightLine bool
	insertSpaces  bool
	tabWidth      int
	padding       int // pixels between the text and the editor's edges

	// matches are the search's matches, highlighted until the search
	// changes; the cursors move with edits to the text.
	matches []*qtwidgets.QTextEditExtraSelection

	// openFiles opens files dropped on the editor, which Qt would
	// otherwise insert as text.
	openFiles func(paths []string)
}

// gutterWidget is the strip down the editor's left side that shows the line
// numbers.
type gutterWidget struct {
	qtwidgets.QWidgetBase
	editor *textEditor
}

func newTextEditor(parent qtwidgets.QWidgetLike) *textEditor {
	e := &textEditor{tabWidth: 4}
	e.edit = qtwidgets.NewQPlainTextEditWith(e, parent)
	e.edit.SetObjectName("jotText")
	e.edit.SetFrameShape(qtwidgets.QFrameNoFrame)
	e.gutter = &gutterWidget{editor: e}
	qtwidgets.NewQWidgetWith(e.gutter, e.edit)
	e.gutter.SetObjectName("jotGutter")
	// Padding is made with the viewport's margins, not the document's: Qt
	// counts a change of those as a change to undo. The document's own
	// margin is taken away once, while there is nothing to undo.
	doc := e.edit.Document()
	doc.SetDocumentMargin(0)
	doc.ClearUndoRedoStacks()
	doc.SetModifiedFull(false)

	e.edit.OnBlockCountChanged(func(int) { e.updateGutterWidth() })
	e.edit.OnUpdateRequest(func(rect qtcore.QRect, dy int) {
		if !e.showNumbers {
			return
		}
		if dy != 0 {
			e.gutter.Scroll(0, dy)
		} else {
			e.gutter.UpdateRect(qtcore.QRect{X: 0, Y: rect.Y + e.padding, Width: e.gutter.Width(), Height: rect.Height})
		}
	})
	e.edit.OnCursorPositionChanged(e.updateExtraSelections)
	e.updateGutterWidth()
	return e
}

// apply gives the editor the settings' font, colours and behaviour.
func (e *textEditor) apply(cfg *config.Config) {
	e.theme = cfg.Theme()
	e.showNumbers = cfg.ShowLineNumbers
	e.highlightLine = cfg.HighlightCurrentLine
	e.insertSpaces = cfg.InsertSpaces
	e.tabWidth = cfg.TabWidth

	font := qtgui.NewQFontWithFamily(cfg.FontFamily)
	font.SetStyleHint(qtgui.QFontTypeWriter)
	font.SetPointSizeF(cfg.FontSize)
	e.edit.SetFont(font)
	e.gutter.SetFont(font)
	space := qtgui.NewQFontMetricsFWithFont(font).HorizontalAdvance(" ")
	e.edit.SetTabStopDistance(space * float64(cfg.TabWidth))

	e.padding = cfg.Padding

	if cfg.WrapText {
		e.edit.SetLineWrapMode(qtwidgets.QPlainTextEditWidgetWidth)
	} else {
		e.edit.SetLineWrapMode(qtwidgets.QPlainTextEditNoWrap)
	}

	// The text's colours are in the window's stylesheet (applyStyle); the
	// gutter, current line and matches are drawn here in the theme's.
	e.gutter.SetVisible(e.showNumbers)
	e.updateGutterWidth()
	e.gutter.Update()
	e.recolourMatches()
	e.updateExtraSelections()
}

// gutterWidth is the width the line numbers need: room for the digits of
// the last line's number, two at least, and one more on the left, as the
// numbers are right-aligned.
func (e *textEditor) gutterWidth() int {
	if !e.showNumbers {
		return 0
	}
	digits := max(2, len(strconv.Itoa(e.edit.BlockCount()))) + 1
	return 2*gutterPadding + e.gutter.FontMetrics().HorizontalAdvance("9")*digits
}

// updateGutterWidth sets the viewport's margins: the gutter and the
// padding on the left, and the padding on the other sides.
func (e *textEditor) updateGutterWidth() {
	p := e.padding
	e.SetViewportMargins(qtcore.QMargins{Left: e.gutterWidth() + p, Top: p, Right: p, Bottom: p})
	e.placeGutter()
}

// placeGutter puts the gutter down the left side, as tall as the editor,
// and as wide as the numbers and the padding beside them, which it paints
// in the text's background.
func (e *textEditor) placeGutter() {
	cr := e.edit.ContentsRect()
	width := 0
	if e.showNumbers {
		width = e.gutterWidth() + e.padding
	}
	e.gutter.SetGeometryRect(qtcore.QRect{X: cr.X, Y: cr.Y, Width: width, Height: cr.Height})
}

// ResizeEvent overrides QWidget::resizeEvent: the gutter stays as tall as
// the editor.
func (e *textEditor) ResizeEvent(ev *qtgui.QResizeEvent) {
	e.Super().ResizeEvent(ev)
	e.placeGutter()
}

// PaintEvent overrides QWidget::paintEvent for the gutter: the numbers of
// the lines in view, the cursor's line's in a stronger colour. A line that
// wraps has its number beside its first row.
func (g *gutterWidget) PaintEvent(ev *qtgui.QPaintEvent) {
	e := g.editor
	area := ev.Rect()
	qtgui.PaintOn(g, func(p *qtgui.QPainter) {
		width := e.gutterWidth()
		p.FillRect(qtcore.QRect{Width: width, Height: g.Height()}, mustColor(e.theme.Gutter))
		p.FillRect(qtcore.QRect{X: width, Width: g.Width() - width, Height: g.Height()}, mustColor(e.theme.Background))
		normal, current := mustColor(e.theme.GutterText), mustColor(e.theme.GutterCurrent)
		lineHeight := g.FontMetrics().Height()
		width -= gutterPadding
		cursorBlock := e.edit.TextCursor().BlockNumber()

		block := e.FirstVisibleBlock()
		offset := e.ContentOffset()
		geom := e.BlockBoundingGeometry(block)
		// The lines' positions are the viewport's, which begins below the
		// top padding.
		top := int(math.Round(geom.Y+offset.Y)) + e.padding
		bottom := top + int(math.Round(geom.Height))
		for block.IsValid() && top <= area.Y+area.Height {
			if block.IsVisible() && bottom >= area.Y {
				n := block.BlockNumber()
				if n == cursorBlock {
					p.SetPenColor(current)
				} else {
					p.SetPenColor(normal)
				}
				p.DrawTextIn(qtcore.QRect{X: 0, Y: top, Width: width, Height: lineHeight},
					int(qtcore.AlignRight|qtcore.AlignVCenter), strconv.Itoa(n+1))
			}
			block = block.Next()
			if !block.IsValid() {
				break
			}
			top = bottom
			bottom = top + int(math.Round(e.BlockBoundingRect(block).Height))
		}
	})
}

// updateExtraSelections highlights the cursor's line and the search's
// matches, and moves the gutter's stronger number to the cursor's line.
func (e *textEditor) updateExtraSelections() {
	sels := make([]*qtwidgets.QTextEditExtraSelection, 0, len(e.matches)+1)
	if e.highlightLine {
		sel := qtwidgets.NewQTextEditExtraSelection()
		f := qtgui.NewQTextCharFormat()
		f.SetBackground(qtgui.NewQBrushWithColor(mustColor(e.theme.CurrentLine)))
		f.SetProperty(int(qtgui.QTextFormatFullWidthSelection), qtcore.NewQVariantOf(true))
		sel.SetFormat(f)
		c := e.edit.TextCursor()
		c.ClearSelection()
		sel.SetCursor(c)
		sels = append(sels, sel)
	}
	sels = append(sels, e.matches...)
	e.edit.SetExtraSelections(sels)
	if e.showNumbers {
		e.gutter.Update()
	}
}

// matchFormat is how a search's matches look.
func (e *textEditor) matchFormat() *qtgui.QTextCharFormat {
	f := qtgui.NewQTextCharFormat()
	f.SetBackground(qtgui.NewQBrushWithColor(mustColor(e.theme.Match)))
	f.SetForeground(qtgui.NewQBrushWithColor(mustColor(e.theme.MatchText)))
	return f
}

// setMatches highlights the cursors' text as the search's matches.
func (e *textEditor) setMatches(cursors []*qtgui.QTextCursor) {
	// The old selections are let go, not just cut off: each holds a cursor
	// the document keeps moving while it lives.
	clear(e.matches)
	e.matches = e.matches[:0]
	f := e.matchFormat()
	for _, c := range cursors {
		sel := qtwidgets.NewQTextEditExtraSelection()
		sel.SetCursor(c)
		sel.SetFormat(f)
		e.matches = append(e.matches, sel)
	}
	e.updateExtraSelections()
}

// recolourMatches gives the matches the theme's colours.
func (e *textEditor) recolourMatches() {
	f := e.matchFormat()
	for _, m := range e.matches {
		m.SetFormat(f)
	}
}

// KeyPressEvent overrides QWidget::keyPressEvent: Tab indents the lines a
// selection covers, or else inserts a tab or the spaces to the next tab
// stop, and Shift+Tab takes one level of indentation off the lines.
func (e *textEditor) KeyPressEvent(ev *qtgui.QKeyEvent) {
	mods := ev.Modifiers() &^ qtcore.KeypadModifier
	switch {
	case ev.Key() == qtcore.Key_Tab && mods == qtcore.NoModifier:
		c := e.edit.TextCursor()
		if e.selectionSpansLines(c) {
			e.indentSelection(c)
			return
		}
		if e.insertSpaces {
			// The spaces take the selection's place, so they reach the tab
			// stop after where it starts.
			block := c.Block()
			col := visualColumn(block.Text(), c.SelectionStart()-block.Position(), e.tabWidth)
			c.InsertText(strings.Repeat(" ", spacesToTabStop(col, e.tabWidth)))
			e.edit.SetTextCursor(c)
			return
		}
	case ev.Key() == qtcore.Key_Backtab:
		e.unindentSelection(e.edit.TextCursor())
		return
	}
	e.Super().KeyPressEvent(ev)
}

// selectionSpansLines reports whether c's selection covers more than one
// line.
func (e *textEditor) selectionSpansLines(c *qtgui.QTextCursor) bool {
	if !c.HasSelection() {
		return false
	}
	doc := e.edit.Document()
	return doc.FindBlock(c.SelectionStart()).BlockNumber() != doc.FindBlock(c.SelectionEnd()).BlockNumber()
}

// selectedBlocks returns the numbers of the first and last lines c's
// selection covers. A selection that ends at the start of a line does not
// cover that line.
func (e *textEditor) selectedBlocks(c *qtgui.QTextCursor) (first, last int) {
	doc := e.edit.Document()
	start, end := c.SelectionStart(), c.SelectionEnd()
	first = doc.FindBlock(start).BlockNumber()
	endBlock := doc.FindBlock(end)
	last = endBlock.BlockNumber()
	if last > first && end == endBlock.Position() {
		last--
	}
	return first, last
}

// indentSelection puts one level of indentation before each line c's
// selection covers, as one step to undo, and selects those lines.
func (e *textEditor) indentSelection(c *qtgui.QTextCursor) {
	unit := indentUnit(e.insertSpaces, e.tabWidth)
	first, last := e.selectedBlocks(c)
	doc := e.edit.Document()
	c.BeginEditBlock()
	for n := first; n <= last; n++ {
		b := doc.FindBlockByNumber(n)
		if b.Length() <= 1 { // empty lines stay empty
			continue
		}
		lc := qtgui.NewQTextCursorWithTextBlock(b)
		lc.InsertText(unit)
	}
	c.EndEditBlock()
	e.selectLines(first, last)
}

// unindentSelection takes one level of indentation off each line c's
// selection covers, or off the cursor's line, as one step to undo.
func (e *textEditor) unindentSelection(c *qtgui.QTextCursor) {
	first, last := e.selectedBlocks(c)
	doc := e.edit.Document()
	c.BeginEditBlock()
	for n := first; n <= last; n++ {
		b := doc.FindBlockByNumber(n)
		if k := unindentLength(b.Text(), e.tabWidth); k > 0 {
			lc := qtgui.NewQTextCursorWithTextBlock(b)
			lc.MovePositionFull(qtgui.QTextCursorRight, qtgui.QTextCursorKeepAnchor, k)
			lc.RemoveSelectedText()
		}
	}
	c.EndEditBlock()
	if c.HasSelection() {
		e.selectLines(first, last)
	}
}

// selectLines selects lines first to last, whole.
func (e *textEditor) selectLines(first, last int) {
	doc := e.edit.Document()
	c := qtgui.NewQTextCursorWithTextBlock(doc.FindBlockByNumber(first))
	end := doc.FindBlockByNumber(last)
	c.SetPositionFull(end.Position()+end.Length()-1, qtgui.QTextCursorKeepAnchor)
	e.edit.SetTextCursor(c)
}

// localFiles returns the local files of a drop, if that is what it is.
func localFiles(source *qtcore.QMimeData) []string {
	if !source.HasUrls() {
		return nil
	}
	var paths []string
	for _, u := range source.Urls() {
		if !u.IsLocalFile() {
			return nil
		}
		paths = append(paths, u.ToLocalFile())
	}
	return paths
}

// CanInsertFromMimeData overrides QPlainTextEdit::canInsertFromMimeData:
// files dropped on the editor are taken, to be opened.
func (e *textEditor) CanInsertFromMimeData(source *qtcore.QMimeData) bool {
	if e.openFiles != nil && len(localFiles(source)) > 0 {
		return true
	}
	return e.Super().CanInsertFromMimeData(source)
}

// InsertFromMimeData overrides QPlainTextEdit::insertFromMimeData: files
// dropped on the editor, or pasted from a file manager, are opened rather
// than their names inserted.
func (e *textEditor) InsertFromMimeData(source *qtcore.QMimeData) {
	if paths := localFiles(source); e.openFiles != nil && len(paths) > 0 {
		open := e.openFiles
		// Opened once the drop is over: opening may show a dialog.
		qute.Post(func() { open(paths) })
		return
	}
	e.Super().InsertFromMimeData(source)
}
