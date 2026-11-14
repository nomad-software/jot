package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"

	"jot/internal/atomicfile"
	"jot/internal/textfile"
)

// Doc is one tab: a document's text, the file it is saved to, and the
// encoding and line endings it is written in.
type Doc struct {
	win    *Window
	page   *qtwidgets.QWidget // the tab's page: the notice above the editor
	editor *textEditor
	edit   *qtwidgets.QPlainTextEdit

	// notice is the bar above the text that says the file has changed on
	// disk, with what can be done about it.
	notice       *qtwidgets.QFrame
	noticeText   *qtwidgets.QLabel
	noticeReload *qtwidgets.QPushButton

	path       string // the file's absolute path, "" until it is saved
	untitled   int    // its number while it has no path: "Untitled 2"
	encoding   textfile.Encoding
	lineEnding textfile.LineEnding
	// disk is the file as it was when last read or written, to tell its
	// own saves from changes made by others.
	disk fileState
}

// fileState is what is compared to tell whether a file has changed on disk.
type fileState struct {
	exists bool
	size   int64
	mod    time.Time
	inode  uint64
}

func statFile(path string) fileState {
	info, err := os.Stat(path)
	if err != nil {
		return fileState{}
	}
	st := fileState{exists: true, size: info.Size(), mod: info.ModTime()}
	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		st.inode = sys.Ino
	}
	return st
}

func newDoc(w *Window) *Doc {
	d := &Doc{win: w, encoding: textfile.UTF8, lineEnding: textfile.LF}
	d.page = qtwidgets.NewQWidget(nil)
	lay := qtwidgets.NewQVBoxLayout(d.page)
	lay.SetContentsMargins(qtcore.QMargins{})
	lay.SetSpacing(0)

	d.notice = qtwidgets.NewQFrame(nil)
	d.notice.SetObjectName("jotNotice")
	nl := qtwidgets.NewQHBoxLayout(d.notice)
	nl.SetContentsMargins(qtcore.QMargins{Left: 10, Top: 6, Right: 10, Bottom: 6})
	d.noticeText = qtwidgets.NewQLabel(nil)
	d.noticeText.SetWordWrap(true)
	nl.AddWidgetFull(d.noticeText, 1, 0)
	d.noticeReload = qtwidgets.NewQPushButtonWithText("Reload", nil)
	d.noticeReload.OnClicked(func(bool) {
		d.hideNotice()
		if err := d.reload(d.encoding); err != nil {
			w.warn(err.Error())
		}
	})
	nl.AddWidget(d.noticeReload)
	keep := qtwidgets.NewQPushButtonWithText("Keep Mine", nil)
	keep.OnClicked(func(bool) {
		// What is on disk now is what the next change is compared to;
		// the text stays modified, so it is still saved over it.
		d.disk = statFile(d.path)
		d.hideNotice()
	})
	nl.AddWidget(keep)
	d.notice.SetVisible(false)
	lay.AddWidget(d.notice)

	d.editor = newTextEditor(nil)
	d.edit = d.editor.edit
	d.editor.openFiles = w.openFiles
	lay.AddWidget(d.edit)
	d.editor.apply(w.cfg)

	d.edit.OnModificationChanged(func(bool) { w.docChanged(d) })
	d.edit.OnCursorPositionChanged(func() { w.cursorMoved(d) })
	d.edit.OnSelectionChanged(func() { w.cursorMoved(d) })
	d.edit.OnUndoAvailable(func(bool) { w.updateActions() })
	d.edit.OnRedoAvailable(func(bool) { w.updateActions() })
	d.edit.OnCopyAvailable(func(bool) { w.updateActions() })
	d.edit.Document().OnContentsChanged(func() { w.find.textChanged(d) })
	return d
}

// name is the document's name in its tab and the window's title.
func (d *Doc) name() string {
	if d.path == "" {
		return fmt.Sprintf("Untitled %d", d.untitled)
	}
	return filepath.Base(d.path)
}

// title is the document's name as its tab and the window's title show it,
// with a * before it while it has changes.
func (d *Doc) title() string {
	if d.modified() {
		return "*" + d.name()
	}
	return d.name()
}

// modified reports whether the text has changes not yet saved.
func (d *Doc) modified() bool { return d.edit.Document().IsModified() }

// pristine reports whether the document is a new one with no text, never
// saved, which a file that is opened takes the place of. Text typed and
// deleted again leaves it pristine: there is nothing in it to lose.
func (d *Doc) pristine() bool {
	return d.path == "" && d.edit.Document().IsEmpty()
}

// text returns the document's text as it is written to its file, before
// its line endings are put in.
func (d *Doc) text() string { return plainText(d.edit.Document().ToRawText()) }

// setFile shows a file's text, starting a new history of changes.
func (d *Doc) setFile(f textfile.File) {
	d.encoding, d.lineEnding = f.Encoding, f.LineEnding
	d.edit.SetPlainText(f.Text)
	d.edit.Document().SetModifiedFull(false)
}

// setPath makes path the document's file, with f its text as read from it;
// a file that is not there yet has none, and is made when it is saved.
func (d *Doc) setPath(path string, f textfile.File) {
	d.path = path
	d.disk = statFile(path)
	if f.Encoding == "" {
		f.Encoding, f.LineEnding = textfile.UTF8, textfile.LF
	}
	d.setFile(f)
	d.hideNotice()
}

// reload reads the document's file again, in the encoding given, keeping
// the cursor where it was as near as it can.
func (d *Doc) reload(enc textfile.Encoding) error {
	if d.path == "" {
		return nil
	}
	data, err := os.ReadFile(d.path)
	if err != nil {
		return err
	}
	f, err := textfile.ParseAs(data, enc)
	if err != nil {
		return fmt.Errorf("%s could not be read as %s: %w", d.name(), enc.Name(), err)
	}
	cursor := d.edit.TextCursor()
	line, col := cursor.BlockNumber(), cursor.PositionInBlock()
	scroll := d.edit.VerticalScrollBar().Value()
	d.disk = statFile(d.path)
	d.setFile(f)
	d.hideNotice()
	d.goTo(line, col)
	d.edit.VerticalScrollBar().SetValue(scroll)
	d.win.docChanged(d)
	return nil
}

// goTo puts the cursor on line (from 0) at col, or as near as there is.
func (d *Doc) goTo(line, col int) {
	doc := d.edit.Document()
	block := doc.FindBlockByNumber(min(max(line, 0), doc.BlockCount()-1))
	c := qtgui.NewQTextCursorWithTextBlock(block)
	c.SetPosition(block.Position() + min(max(col, 0), block.Length()-1))
	d.edit.SetTextCursor(c)
}

// encode returns the document's text as its file's bytes.
func (d *Doc) encode() ([]byte, error) {
	return textfile.File{Text: d.text(), Encoding: d.encoding, LineEnding: d.lineEnding}.Bytes()
}

// saveTo writes the document to path, which becomes its file.
func (d *Doc) saveTo(path string) error {
	data, err := d.encode()
	if err != nil {
		return err
	}
	if err := atomicfile.Save(path, data); err != nil {
		return err
	}
	d.path = path
	d.disk = statFile(path)
	d.hideNotice()
	d.edit.Document().SetModifiedFull(false)
	return nil
}

// showNotice shows the bar above the text with a message, and the Reload
// button if the file can be read again.
func (d *Doc) showNotice(text string, canReload bool) {
	d.noticeText.SetText(text)
	d.noticeReload.SetVisible(canReload)
	d.notice.SetVisible(true)
}

func (d *Doc) hideNotice() { d.notice.SetVisible(false) }

// diskChanged is told that the document's file may have changed on disk.
// Changes made by the editor itself are told from others' by the file's
// size, time and inode, as they were when it was last written. A file
// with no changes of its own is read again at once; otherwise a notice
// asks what to do.
func (d *Doc) diskChanged() {
	if d.path == "" {
		return
	}
	now := statFile(d.path)
	if now == d.disk {
		return
	}
	switch {
	case !now.exists:
		d.showNotice(d.name()+" has been deleted or moved on disk. Save it to write it again.", false)
		// Saving must write it again, so the text counts as changed.
		d.edit.Document().SetModifiedFull(true)
		d.disk = now
	case !d.modified() && !d.notice.IsVisible():
		if err := d.reload(d.encoding); err != nil {
			d.showNotice(err.Error(), true)
			d.disk = now
			return
		}
		d.win.message(fmt.Sprintf("%s changed on disk and was reloaded", d.name()))
	default:
		d.showNotice(d.name()+" has changed on disk. Reload it, losing your changes, or keep your version?", true)
	}
}
