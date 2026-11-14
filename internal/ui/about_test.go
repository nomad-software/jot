package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nomad-software/qute"
	"github.com/nomad-software/qute/qtcore"
	"github.com/nomad-software/qute/qtgui"
	"github.com/nomad-software/qute/qtwidgets"
	"github.com/nomad-software/qute/qutetest"
)

func TestPlainVersion(t *testing.T) {
	for version, want := range map[string]string{
		"v1.2.3":                                "v1.2.3",
		"v1.15.1-0.20261027184954-00c19a9187bb": "v1.15.0",
		"v1.15.1-0.20261027184954-00c19a9187bb+dirty": "v1.15.0",
		"v2.0.0-rc.1":                        "v2.0.0",
		"v0.0.0-20261027184954-00c19a9187bb": "dev",
	} {
		if got := plainVersion(version); got != want {
			t.Errorf("plainVersion(%q) = %q, want %q", version, got, want)
		}
	}
}

func TestAbout(t *testing.T) {
	qutetest.Run(t, func() {
		d := newAboutDialog(nil)
		defer d.DeleteLater()
		if d.version.Text() != AppVersion() {
			t.Errorf("the version shown is %q", d.version.Text())
		}
		if d.WindowTitle() != "About Jot" {
			t.Errorf("title %q", d.WindowTitle())
		}
		licences := licenceText()
		for _, want := range []string{qtcore.QLibraryInfoVersion().ToString(), "LGPLv3", "Qt SVG", "qute", "GNU General Public License"} {
			if !strings.Contains(licences, want) {
				t.Errorf("the licences do not mention %s", want)
			}
		}
		d.Layout().Activate()
		if need := d.Layout().AsQLayoutItem().HeightForWidth(d.Width()); d.Height() < need {
			t.Errorf("the about box is %d high, and needs %d", d.Height(), need)
		}
		if dir := os.Getenv("JOT_SCREENSHOTS"); dir != "" {
			d.Show()
			qutetest.Wait(50)
			qute.SaveScreenshot(d, filepath.Join(dir, "about.png"))
		}
	})
}

// TestAboutNamesEveryLibrary checks that the licences name every module the
// program is built with, as go.mod lists them, so that one added is not
// left out.
func TestAboutNamesEveryLibrary(t *testing.T) {
	mod, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var licences string
	qutetest.Run(t, func() { licences = licenceText() })
	for line := range strings.SplitSeq(string(mod), "\n") {
		fields := strings.Fields(strings.TrimPrefix(line, "require "))
		if len(fields) < 2 || !strings.HasPrefix(fields[1], "v") || !strings.Contains(fields[0], ".") {
			continue
		}
		parts := strings.Split(fields[0], "/")
		name := parts[len(parts)-1]
		if !strings.Contains(licences, ">"+name+"</a>") {
			t.Errorf("the licences do not name %s (%s)", name, fields[0])
		}
	}
}

func TestToolbarHasNoTooltips(t *testing.T) {
	qutetest.Run(t, func() {
		w := newTestWindow(t, nil)
		// tip asks the button for its tooltip and returns what is shown.
		tip := func(button *qtwidgets.QWidget) string {
			pos := qtcore.QPoint{X: 5, Y: 5}
			ev := qtgui.NewQHelpEventWithType(qtcore.QEventToolTip, pos, button.MapToGlobal(pos))
			qtcore.QCoreApplicationSendEvent(button, ev)
			return qtwidgets.QToolTipText()
		}
		// A button of its own shows its tooltip, so the check can see one.
		plain := qtwidgets.NewQToolButton(w.w)
		plain.SetToolTip("a tip")
		plain.Show()
		if tip(plain.AsQWidget()) != "a tip" {
			t.Skip("tooltips are not shown on this platform")
		}
		for _, id := range toolbarActions {
			if id == "" {
				continue
			}
			act := w.acts[id]
			if got := tip(w.toolbar.WidgetForAction(act)); got == act.ToolTip() {
				t.Errorf("the %s button shows the tooltip %q", id, got)
			}
		}
		qtwidgets.QToolTipHideText()
	})
}
