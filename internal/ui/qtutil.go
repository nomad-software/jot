package ui

import (
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
)

// Helpers for Qt values the editor makes only to hand to Qt. qute's GC
// cleanup frees each one once it is dropped.

// setBoolProperty sets a dynamic property, which stylesheets select on.
func setBoolProperty(w qtwidgets.QWidgetLike, name string, on bool) {
	w.AsQWidget().SetProperty(name, qtcore.NewQVariantOf(on))
}

// setCursorShape sets the mouse pointer shown over w.
func setCursorShape(w qtwidgets.QWidgetLike, shape qtcore.CursorShape) {
	w.AsQWidget().SetCursor(qtgui.NewQCursorWithCursorShape(shape))
}

// keySequence returns the key sequence of a shortcut in Qt's portable text.
func keySequence(text string) *qtgui.QKeySequence {
	return qtgui.NewQKeySequenceWithKey(text)
}

// tipFilter is an event filter that takes tooltip events.
type tipFilter struct{ qtcore.QObjectBase }

// EventFilter overrides QObject::eventFilter: tooltip events are taken, and
// every other is let through. Qt's own filter is not called, as it only
// lets events through, and a button being destroyed as its window closes
// still has events filtered, which Super would refuse.
func (f *tipFilter) EventFilter(_ *qtcore.QObject, event *qtcore.QEvent) bool {
	return event.Type() == qtcore.QEventToolTip
}
